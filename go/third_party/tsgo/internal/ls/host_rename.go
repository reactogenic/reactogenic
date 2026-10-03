// Reactogenic: a rename whose occurrences in content-mapped files are written
// back by the host of the built-in mapper. Not part of upstream: added by
// go/patches/0006-rtsx-lsp.patch.

package ls

import (
	"context"
	"errors"
	"iter"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/locale"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsutil"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
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
			}
			return all
		},
		true,  /*isRename*/
		false, /*implementations*/
		symbolEntryTransformOptions{},
		nil, /*defaultProjectData*/
	)
	if err != nil || !found.renamed {
		return lsproto.WorkspaceEditOrNull{}, err
	}
	if len(found.occurrences) == 0 {
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
			// Upstream refuses a symbol declared in node_modules by its
			// declarations; the prop of a generic component has passed that
			// check instantiated, and its declaration is edited all the same.
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
// for each occurrence.
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

	result := hostRenameOccurrences{renamed: true, name: data.OriginalNode.Text()}
	for _, entry := range core.FlatMap(data.SymbolsAndEntries, func(s *SymbolAndEntries) []*ReferenceEntry { return s.references }) {
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
		}
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
		result.occurrences = append(result.occurrences, occurrence)
	}
	return result
}

// HostFile is the program and the file of a document; the file is nil when
// the program does not hold it.
func (l *LanguageService) HostFile(uri lsproto.DocumentUri) (*compiler.Program, *ast.SourceFile) {
	return l.tryGetProgramAndFile(uri.FileName())
}
