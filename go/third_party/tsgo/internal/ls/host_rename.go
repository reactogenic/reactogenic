// Reactogenic: a rename whose occurrences in content-mapped files are written
// back by the host of the built-in mapper. Not part of upstream: added by
// go/patches/0006-rtsx-lsp.patch.

package ls

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/locale"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsutil"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// HostRenameOccurrence is one place a rename changes, as the language
// service found it in the text it checks — before it is written back to the
// text the author wrote.
type HostRenameOccurrence struct {
	// File holds the occurrence; Pos and End are offsets in its text, which
	// for a content-mapped file is the virtual text.
	File     *ast.SourceFile
	Pos, End int
	// Node is the name there, in File's tree; nil for an occurrence that is
	// only a range.
	Node *ast.Node
	// OriginalPos and OriginalEnd are what Pos and End map to in the
	// original text of a content-mapped file. Exact: the occurrence lies in
	// one verbatim span, so it is that text — upstream writes such an
	// occurrence back, and drops any other.
	OriginalPos, OriginalEnd int
	Exact                    bool
	// NewText replaces the occurrence: the new name, or what the language
	// service makes of it there (`size: dim` for a shorthand property).
	NewText string

	location lsproto.Location // of an occurrence in a file that is not content mapped
	library  bool             // in a file of the default library
}

// HostRenameEdit is one edit of a rename, in the original text of a
// content-mapped file.
type HostRenameEdit struct {
	FileName string
	Pos, End int
	NewText  string
}

// HostRenameRequest is a rename for the host to write back.
type HostRenameRequest struct {
	// Name is the name as it is, NewName what it becomes.
	Name, NewName string
	// Prepare: nothing is renamed. The question is whether a rename here
	// would be refused; NewName is Name.
	Prepare bool
	// Occurrences are those in content-mapped files, each once.
	Occurrences []HostRenameOccurrence
	// Files are the content-mapped files of the program the rename started
	// in: code the mapper left out of a virtual text has occurrences that
	// nothing found.
	Files []*ast.SourceFile
}

// HostRename writes a rename's occurrences in content-mapped files back:
// the complete edits of those files, or an error — the rename is then
// refused as a whole, with that message.
type HostRename func(ctx context.Context, request HostRenameRequest) ([]HostRenameEdit, error)

// hostRenameOccurrences is one project's answer.
type hostRenameOccurrences struct {
	renamed     bool // the symbol can be renamed
	name        string
	occurrences []HostRenameOccurrence
	// notOffered: what is renamed is no name that anything declares — a
	// rename of it would be of that one place.
	notOffered bool
	// refused: the occurrences are not all of them, or would not mean what
	// they mean now. The rename is refused whatever the host makes of them.
	refused error
}

// ProvideHostRename is ProvideRename with the occurrences in content-mapped
// files written back by host: upstream maps each back by position and drops
// those it cannot map, which renames a part. An occurrence in any other file
// is written as upstream writes it. The occurrences are gathered across
// projects first, so the host decides on all of them at once.
func (l *LanguageService) ProvideHostRename(ctx context.Context, params *lsproto.RenameParams, orchestrator CrossProjectOrchestrator, host HostRename, prepare bool) (lsproto.WorkspaceEditOrNull, error) {
	found, err := l.handleCrossProject(
		ctx,
		params,
		orchestrator,
		func(l *LanguageService, ctx context.Context, params *lsproto.RenameParams, data SymbolAndEntriesData, _ symbolEntryTransformOptions) (hostRenameOccurrences, error) {
			return l.symbolAndEntriesToOccurrences(ctx, params, data, prepare), nil
		},
		func(results iter.Seq[hostRenameOccurrences]) hostRenameOccurrences {
			var all hostRenameOccurrences
			for result := range results {
				all.renamed = all.renamed || result.renamed
				if all.name == "" {
					all.name = result.name
				}
				all.occurrences = append(all.occurrences, result.occurrences...)
				all.notOffered = all.notOffered || result.notOffered
				if all.refused == nil {
					all.refused = result.refused
				}
			}
			return all
		},
		true,  /*isRename*/
		false, /*implementations*/
		symbolEntryTransformOptions{},
		nil, /*defaultProjectData*/
	)
	if err != nil || !found.renamed && !found.notOffered {
		return lsproto.WorkspaceEditOrNull{}, err
	}
	if found.refused != nil {
		return lsproto.WorkspaceEditOrNull{}, found.refused
	}
	if len(found.occurrences) == 0 || found.notOffered {
		// The tag of an intrinsic element that an index signature declares:
		// a symbol, and nothing to edit. A rename of nothing is not offered.
		if prepare {
			return lsproto.WorkspaceEditOrNull{}, errors.New(diagnostics.You_cannot_rename_this_element.Localize(locale.FromContext(ctx)))
		}
		return lsproto.WorkspaceEditOrNull{}, nil
	}

	request := HostRenameRequest{Name: found.name, NewName: params.NewName, Prepare: prepare}
	if prepare {
		request.NewName = found.name
	}
	type occurrenceKey struct {
		fileName string
		pos, end int
		newText  string
	}
	seen := map[occurrenceKey]bool{}
	files := map[string]*ast.SourceFile{}
	var mappedEdits []mappedRenameEdit
	for _, occurrence := range found.occurrences {
		if occurrence.File.SpanMap() == nil {
			// Upstream refuses a symbol declared in the library or in
			// node_modules by its declarations. A string has none — it is
			// found wherever a string of its type is written — and the prop
			// of a generic component has passed that check instantiated: the
			// library's file, or the declaration, is edited all the same.
			if occurrence.library {
				return lsproto.WorkspaceEditOrNull{}, errors.New(diagnostics.You_cannot_rename_elements_that_are_defined_in_the_standard_TypeScript_library.Localize(locale.FromContext(ctx)))
			}
			if isInsideNodeModules(occurrence.File.FileName()) {
				return lsproto.WorkspaceEditOrNull{}, errors.New(diagnostics.You_cannot_rename_elements_that_are_defined_in_a_node_modules_folder.Localize(locale.FromContext(ctx)))
			}
			mappedEdits = append(mappedEdits, mappedRenameEdit{uri: occurrence.location.Uri, edit: &lsproto.TextEdit{Range: occurrence.location.Range, NewText: occurrence.NewText}})
			continue
		}
		// A file of several projects is found once per project.
		key := occurrenceKey{occurrence.File.OriginalFileName(), occurrence.Pos, occurrence.End, occurrence.NewText}
		if !seen[key] {
			seen[key] = true
			files[key.fileName] = occurrence.File
			request.Occurrences = append(request.Occurrences, occurrence)
		}
	}
	for _, file := range l.GetProgram().SourceFiles() {
		if file.SpanMap() != nil {
			request.Files = append(request.Files, file)
			if files[file.OriginalFileName()] == nil {
				files[file.OriginalFileName()] = file
			}
		}
	}
	edits, err := host(ctx, request)
	if err != nil {
		return lsproto.WorkspaceEditOrNull{}, err
	}
	for _, edit := range edits {
		file := files[edit.FileName]
		if file == nil {
			return lsproto.WorkspaceEditOrNull{}, errors.New("a rename edit in a file the rename does not reach: " + edit.FileName)
		}
		document := hostScript{fileName: file.OriginalFileName(), text: file.OriginalText()}
		lspRange, _ := l.converters.ToLSPRange(document, core.NewTextRange(edit.Pos, edit.End))
		mappedEdits = append(mappedEdits, mappedRenameEdit{uri: lsconv.FileNameToDocumentURI(document.fileName), edit: &lsproto.TextEdit{Range: lspRange, NewText: edit.NewText}})
	}
	changes, ok := deduplicateRenameEdits(mappedEdits)
	if !ok {
		return lsproto.WorkspaceEditOrNull{}, errors.New("the rename gives one place two different texts")
	}
	return lsproto.WorkspaceEditOrNull{WorkspaceEdit: &lsproto.WorkspaceEdit{Changes: &changes}}, nil
}

// symbolAndEntriesToOccurrences is symbolAndEntriesToRename up to the point
// where that writes each occurrence back: the same checks, the same text
// for each occurrence. And it is where a rename that the occurrences do not
// make is stopped — what upstream renames in part, in a .tsx file as well:
//
//   - a string: found wherever a string of its type is written — another
//     function's, the library's — and not always in the type that declares
//     it. Refused;
//   - a key written as a string in a binding pattern, `{ "sub-item": sub }`,
//     is no reference to upstream's search: added here;
//   - the prop that an element's body is the value of: the body has no name.
//     Refused when an element of that prop's type has a body;
//   - a tag that would change between a component's and an intrinsic
//     element's (`Box` to `box`): it would no longer be a reference. Refused;
//   - the name of an attribute that nothing declares (`data-tone`): renamed,
//     it is another attribute. Not offered.
func (l *LanguageService) symbolAndEntriesToOccurrences(ctx context.Context, params *lsproto.RenameParams, data SymbolAndEntriesData, prepare bool) hostRenameOccurrences {
	if !nodeIsEligibleForRename(data.OriginalNode) {
		return hostRenameOccurrences{}
	}
	program := l.GetProgram()
	sourceFile := ast.GetSourceFileOfNode(data.OriginalNode)
	newName := params.NewName
	if prepare {
		newName = data.OriginalNode.Text()
	}
	if info, ok := l.getRenameInfoForNode(ctx, newName, data.OriginalNode, sourceFile, program); !ok || !info.CanRename {
		return hostRenameOccurrences{}
	}
	ch, done := program.GetTypeChecker(ctx)
	defer done()
	quotePreference := lsutil.GetQuotePreference(sourceFile, l.UserPreferences())
	useAliasesForRename := l.UserPreferences().UseAliasesForRename.IsTrueOrUnknown()

	name := data.OriginalNode.Text()
	result := hostRenameOccurrences{renamed: true, name: name}
	entries := core.FlatMap(data.SymbolsAndEntries, func(s *SymbolAndEntries) []*ReferenceEntry { return s.references })
	entries = append(entries, quotedBindingKeys(program, ch, name, entries)...)
	undeclared := len(entries) > 0
	for _, entry := range entries {
		if l.UserPreferences().AllowRenameOfImportPath != core.TSTrue && entry.node != nil && ast.IsStringLiteralLike(entry.node) && ast.TryGetImportFromModuleSpecifier(entry.node) != nil {
			continue
		}
		l.resolveEntrySource(entry)
		occurrence := HostRenameOccurrence{
			File:    entry.sourceFile,
			Pos:     entry.textRange.Pos(),
			End:     entry.textRange.End(),
			Node:    entry.node,
			NewText: l.getTextForRename(data.OriginalNode, entry, newName, ch, quotePreference, useAliasesForRename),
			library: program.IsSourceFileDefaultLibrary(entry.sourceFile.Path()) && tspath.IsDeclarationFileName(entry.sourceFile.FileName()),
		}
		undeclared = undeclared && entry.node != nil && ast.IsJsxAttribute(entry.node.Parent) && entry.node.Parent.Name() == entry.node && strings.Contains(entry.node.Text(), "-")
		if spans := entry.sourceFile.SpanMap(); spans != nil {
			original, fidelity := spans.VirtualToOriginalSpan(*entry.textRange)
			occurrence.OriginalPos, occurrence.OriginalEnd, occurrence.Exact = original.Pos(), original.End(), fidelity.IsExact()
		} else {
			lspRange, ok := l.renameEditRange(entry)
			if !ok {
				continue // as upstream: behind a declaration map, with no place in its source
			}
			occurrence.location = lsproto.Location{Uri: l.getFileNameOfEntry(entry), Range: lspRange}
		}
		// A tag is a component or an intrinsic element by its first letter.
		if node := entry.node; result.refused == nil && !prepare && node != nil && ast.IsIdentifier(node) && ast.IsJsxTagName(node) &&
			scanner.IsIntrinsicJsxName(node.Text()) != scanner.IsIntrinsicJsxName(newName) {
			what := "an intrinsic element: a tag that starts with a lower-case letter is one"
			if scanner.IsIntrinsicJsxName(node.Text()) {
				what = "a component: a tag that starts with a capital letter is one"
			}
			result.refused = hostRefusal(entry.sourceFile, entry.textRange.Pos(), "`<%s>` would become %s.", node.Text(), what)
		}
		result.occurrences = append(result.occurrences, occurrence)
	}
	switch {
	case undeclared:
		return hostRenameOccurrences{notOffered: true, name: name}
	case core.Some(data.SymbolsAndEntries, func(s *SymbolAndEntries) bool {
		return s.definition != nil && s.definition.Kind == definitionKindString
	}):
		result.refused = fmt.Errorf("Rename refused: \"%s\" is a string, not a name. TypeScript finds the strings of its type wherever they are written, and not always the type that declares them: the rename would be partial.", name)
	case result.refused == nil:
		result.refused = bodyIsRenamedProp(program, ch, name, data.OriginalNode, entries)
	}
	return result
}

// hostRefusal is a refusal that names a place: the position pos of file's
// text — for a content-mapped file, where that is in what its author wrote.
func hostRefusal(file *ast.SourceFile, pos int, format string, args ...any) error {
	text := file.Text()
	if spans := file.SpanMap(); spans != nil {
		original, _ := spans.VirtualToOriginalSpan(core.NewTextRange(pos, pos))
		text, pos = file.OriginalText(), original.Pos()
	}
	line, column := 1, 1
	for i, r := range text {
		if i >= pos {
			break
		}
		if r == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return fmt.Errorf("Rename refused at %s:%d:%d: %s", tspath.GetBaseFileName(file.OriginalFileName()), line, column, fmt.Sprintf(format, args...))
}

// renamedDeclarations are the declarations that the entries rename: those
// whose name is an entry.
func renamedDeclarations(entries []*ReferenceEntry) map[*ast.Node]bool {
	declarations := map[*ast.Node]bool{}
	for _, entry := range entries {
		if node := entry.node; node != nil && node.Parent != nil && ast.IsDeclaration(node.Parent) && node.Parent.Name() == node {
			declarations[node.Parent] = true
		}
	}
	return declarations
}

// declares reports whether symbol is declared by one of declarations.
func declares(symbol *ast.Symbol, declarations map[*ast.Node]bool) bool {
	return symbol != nil && core.Some(symbol.Declarations, func(d *ast.Node) bool { return declarations[d] })
}

// quotedBindingKeys are the keys of binding patterns that name a renamed
// property as a string — `{ "sub-item": sub }`, the only way to destructure
// a property whose name is no identifier. Upstream's reference search takes
// a string for a property's name where it declares or indexes one, and not
// here: the rename would leave the pattern reading a property that is gone.
func quotedBindingKeys(program *compiler.Program, ch *checker.Checker, name string, entries []*ReferenceEntry) []*ReferenceEntry {
	declarations := renamedDeclarations(entries)
	if len(declarations) == 0 {
		return nil
	}
	found := map[*ast.Node]bool{}
	for _, entry := range entries {
		found[entry.node] = true
	}
	var keys []*ReferenceEntry
	for _, file := range program.SourceFiles() {
		// The name in quotes: few files have it, the library's seldom.
		if text := file.Text(); !strings.Contains(text, `"`+name+`"`) && !strings.Contains(text, "'"+name+"'") && !strings.Contains(text, "`"+name+"`") {
			continue
		}
		for _, node := range getPossibleSymbolReferenceNodes(file, name, nil /*container*/) {
			if found[node] || !ast.IsStringLiteralLike(node) || node.Text() != name || !ast.IsBindingElement(node.Parent) || node.Parent.PropertyName() != node || !ast.IsObjectBindingPattern(node.Parent.Parent) {
				continue
			}
			found[node] = true
			if t := ch.GetTypeAtLocation(node.Parent.Parent); t != nil && declares(ch.GetPropertyOfType(t, name), declarations) {
				keys = append(keys, newNodeEntry(node))
			}
		}
	}
	return keys
}

// bodyIsRenamedProp refuses the rename of the prop that an element's body
// is the value of (`children`), when an element with a body has that prop:
// the body is an occurrence with no name, and renamed without it the
// element passes a prop that is gone.
func bodyIsRenamedProp(program *compiler.Program, ch *checker.Checker, name string, location *ast.Node, entries []*ReferenceEntry) error {
	if name == "" || ch.JsxChildrenPropertyName(location) != name {
		return nil
	}
	declarations := renamedDeclarations(entries)
	if len(declarations) == 0 {
		return nil
	}
	var refused error
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsJsxElement(node) && len(ast.GetSemanticJsxChildren(node.Children().Nodes)) > 0 {
			opening := node.AsJsxElement().OpeningElement
			if t := ch.GetContextualType(opening.Attributes(), checker.ContextFlagsNone); t != nil && declares(ch.GetPropertyOfType(t, name), declarations) {
				tag := opening.TagName()
				file := ast.GetSourceFileOfNode(node)
				refused = hostRefusal(file, scanner.GetTokenPosOfNode(tag, file, false /*includeJsDoc*/), "`<%s>` has a body, which is its `%s`: a body has no name to rename. Write it as an attribute first.", scanner.GetTextOfNode(tag), name)
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	for _, file := range program.SourceFiles() {
		if !file.IsDeclarationFile && file.AsNode().ForEachChild(visit) {
			break
		}
	}
	return refused
}

// HostFile is the program and the file of a document; the file is nil when
// the program does not hold it.
func (l *LanguageService) HostFile(uri lsproto.DocumentUri) (*compiler.Program, *ast.SourceFile) {
	return l.tryGetProgramAndFile(uri.FileName())
}
