package transpiler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

type passContext struct {
	file *rtsx.SourceFile
	text string
	// report records a diagnostic; false: dropped, as part of a half-typed
	// construct (tolerant mode).
	report func(s emit.Span, severity Severity, code, message string) bool
	// note and generated record Output.Notes and Output.Generated.
	note    func(s emit.Span, kind, name, detail string)
	noteTag func(s, tag emit.Span, kind, name string)
	// shorthandSite and slotGroup record Output.Shorthands and SlotGroups.
	shorthandSite func(name emit.Span, kind string)
	slotGroup     func(name string, owner emit.Span, tags []emit.Span)
	generated     func(local, written string)
	// dropped records Output.Dropped: code of the author's is left out.
	dropped func()
	// names holds every identifier of the original source, so generated
	// names can never capture or shadow the author's.
	names map[string]bool
	// imports added by this run of the pass, to add each once.
	imports map[string]bool
	// coreNames are the helpers of @reactogenic/core this run named through
	// core(), export → local; coreImport() imports them in one declaration.
	coreNames  map[string]string
	coreOrigin emit.Span
	// entry and readFile reach the other files: segments (Input).
	entry    string
	readFile func(path string) (string, bool)
}

// span is n's source range from its first token (without leading trivia).
func (c *passContext) span(n *rtsx.Node) emit.Span {
	return emit.Span{Pos: rtsx.TokenStart(c.file, n), End: n.End()}
}

func (c *passContext) copy(n *rtsx.Node) emit.Piece {
	return emit.Copy(c.text, c.span(n))
}

// copyValue copies an expression. A string in JSX attribute position
// (`title="…"`) is not a JS string: a backslash is a backslash, it may span
// lines, and character references are decoded. Where the value moves to a JS
// position — an object property, a test, an argument — a string that holds
// one of those is rewritten as the JS literal of its value (an atom on the
// string). Any other string means the same in both grammars and is copied as
// written, so hover and completion keep working inside it (`is="loading"`).
func (c *passContext) copyValue(n *rtsx.Node) emit.Piece {
	if n.Kind == rtsx.KindStringLiteral && n.Parent != nil && n.Parent.Kind == rtsx.KindJsxAttribute {
		if raw := rtsx.NodeText(n); strings.ContainsAny(raw, "\\&\r\n") {
			return emit.Synth(jsxStringLiteral(raw), c.span(n))
		}
	}
	return c.copy(n)
}

// The two copies of a shorthand (`value` → `value={value}`) are different
// symbols — the prop and the binding — and answer different features, as TS
// does on `{ value }` (ide.md, *Span map*). The masks are what each lacks.
const (
	nameCopyHas  = emit.FeatureHover | emit.FeatureCompletion | emit.FeatureDefinition | emit.FeatureTypeDefinition | emit.FeatureReferences | emit.FeatureDocumentHighlights
	valueCopyHas = emit.FeatureHover | emit.FeatureDefinition | emit.FeatureReferences | emit.FeatureDocumentHighlights | emit.FeatureRename | emit.FeatureSemanticTokens | emit.FeatureInlayHints
	nameCopy     = emit.AllFeatures &^ nameCopyHas
	valueCopy    = emit.AllFeatures &^ valueCopyHas
)

// copyName copies a name the author wrote — a slot tag, a slot attribute,
// an arg — to where it is a prop name or an object key, so TS's features
// reach it. It keeps the grammar's colour: no semantic tokens.
func (c *passContext) copyName(s emit.Span) emit.Piece {
	return emit.Copy(c.text, s).Lacking(emit.FeatureSemanticTokens)
}

// closingName is the name for a rebuilt closing tag: the closing tag the
// author wrote, or — the element was self-closing — one more copy of the
// opening name.
func (c *passContext) closingName(el, opening *rtsx.Node) emit.Piece {
	if el.Kind == rtsx.KindJsxElement {
		if closing := el.AsJsxElement().ClosingElement; closing != nil && closing.TagName().End() > closing.TagName().Pos() {
			return c.copy(closing.TagName())
		}
	}
	return c.copy(opening.TagName()).Lacking(emit.AllFeatures)
}

// copyChild copies a JSX child. JSX text starts at its first byte: its
// leading whitespace is content, not trivia.
func (c *passContext) copyChild(n *rtsx.Node) emit.Piece {
	if n.Kind == rtsx.KindJsxText {
		return emit.Copy(c.text, emit.Span{Pos: n.Pos(), End: n.End()})
	}
	return c.copy(n)
}

// errorAt reports an error on n; false: it was dropped, n being part of a
// half-typed construct (ide.md, *Tolerance*).
func (c *passContext) errorAt(n *rtsx.Node, code, format string, args ...any) bool {
	s := c.span(n)
	if n.Kind == rtsx.KindJsxText && s.Len() <= 0 {
		// Whitespace text has no token: the error is on the text itself, not
		// an empty span at whatever follows it.
		s = emit.Span{Pos: n.Pos(), End: n.End()}
	}
	return c.report(s, Error, code, fmt.Sprintf(format, args...))
}

// invalid reports an error on n — an attribute or a child that its construct
// does not take — and says whether the construct fails with it. When the
// error is dropped, n being half-typed, the construct is lowered without n
// instead (ide.md, *Tolerance*): one `$Case` being typed does not take the
// `Switch` out of the virtual text.
func (c *passContext) invalid(n *rtsx.Node, code, format string, args ...any) bool {
	if c.errorAt(n, code, format, args...) {
		return true
	}
	c.leftOut(n)
	return false
}

// leftOut: n is not in the output. If it holds code — an expression, a
// component — TS no longer sees what the author wrote (Output.Dropped).
func (c *passContext) leftOut(n *rtsx.Node) {
	if holdsCode(n) {
		c.dropped()
	}
}

// holdsCode: n uses a name — in a `{…}` expression, a spread or params, or
// as a component tag. Text, intrinsic elements, slot tags and string
// attributes use none.
func holdsCode(n *rtsx.Node) bool {
	found := false
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		switch n.Kind {
		case rtsx.KindJsxExpression:
			found = n.Expression() != nil
		case rtsx.KindJsxSpreadAttribute:
			found = true
		case rtsx.KindJsxOpeningElement, rtsx.KindJsxSelfClosingElement:
			tag := n.TagName()
			name := rtsx.NodeText(tag)
			found = !rtsx.IsIntrinsicTag(tag) && !(tag.Kind == rtsx.KindIdentifier && (name == "" || strings.HasPrefix(name, "$")))
		}
		return found || n.ForEachChild(visit)
	}
	visit(n)
	return found
}

// fresh returns base, or base1, base2, … — the first not used in the source.
func (c *passContext) fresh(base string) string {
	name := base
	for i := 1; c.names[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	return name
}

// unique is fresh, and reserves the name, so the next one differs: for a
// name that stands for one construct (`_on` of one Switch), which a message
// must be able to trace back.
func (c *passContext) unique(base, written string) string {
	name := c.fresh(base)
	c.names[name] = true
	c.generated(name, written)
	return name
}

// sourceText is n as written in this pass's input.
func (c *passContext) sourceText(n *rtsx.Node) string {
	s := c.span(n)
	return c.text[s.Pos:s.End]
}

// identifiers collects every identifier in file.
func identifiers(file *rtsx.SourceFile) map[string]bool {
	names := map[string]bool{}
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindIdentifier {
			names[rtsx.NodeText(n)] = true
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return names
}

// core names a helper of @reactogenic/core in the emitted code, under a
// fresh name; coreImport() imports it.
func (c *passContext) core(export string, origin emit.Span) string {
	if c.coreNames == nil {
		c.coreNames = map[string]string{}
		c.coreOrigin = origin
	}
	local := c.fresh("_" + export)
	c.coreNames[export] = local
	c.generated(local, export)
	return local
}

// coreImport imports, in one declaration after the last import, the helpers
// named through core() that the file does not import yet.
func (c *passContext) coreImport() []emit.Edit {
	var specs []string
	for export, local := range c.coreNames {
		if !c.imports["@reactogenic/core|"+export+"|"+local] && !c.hasImport("@reactogenic/core", export, local) {
			specs = append(specs, export+" as "+local)
		}
	}
	if len(specs) == 0 {
		return nil
	}
	sort.Strings(specs)
	// Extend an import of the package this pass added before, if there is one.
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind != rtsx.KindImportDeclaration || rtsx.NodeText(stmt.AsImportDeclaration().ModuleSpecifier) != "@reactogenic/core" {
			continue
		}
		named := namedImports(stmt)
		if len(named) == 0 || !strings.HasPrefix(rtsx.NodeText(named[0].Name()), "_") {
			continue // the author's own import: leave it as written
		}
		end := named[len(named)-1].End()
		return []emit.Edit{{Span: emit.Span{Pos: end, End: end}, Pieces: []emit.Piece{emit.Synth(", "+strings.Join(specs, ", "), c.coreOrigin)}}}
	}
	text := fmt.Sprintf("import { %s } from %q;", strings.Join(specs, ", "), "@reactogenic/core")
	var last *rtsx.Node
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind == rtsx.KindImportDeclaration {
			last = stmt
		}
	}
	if last != nil {
		return []emit.Edit{{Span: emit.Span{Pos: last.End(), End: last.End()}, Pieces: []emit.Piece{emit.Synth("\n"+text, c.coreOrigin)}}}
	}
	return []emit.Edit{{Span: emit.Span{}, Pieces: []emit.Piece{emit.Synth(text+"\n", c.coreOrigin)}}}
}

// hasImport: the file imports `exported as local` from module.
func (c *passContext) hasImport(module, exported, local string) bool {
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind != rtsx.KindImportDeclaration || rtsx.NodeText(stmt.AsImportDeclaration().ModuleSpecifier) != module {
			continue
		}
		for _, spec := range namedImports(stmt) {
			if importedAs(spec) == exported && rtsx.NodeText(spec.Name()) == local {
				return true
			}
		}
	}
	return false
}

// ensureImport returns an edit that adds `import { exported as local } from
// module` after the file's last import — or nil if the file (or this run of
// the pass) already has it.
func (c *passContext) ensureImport(module, exported, local string, origin emit.Span) []emit.Edit {
	key := module + "|" + exported + "|" + local
	if c.imports[key] {
		return nil
	}
	c.imports[key] = true
	var last *rtsx.Node
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind != rtsx.KindImportDeclaration {
			continue
		}
		last = stmt
		if rtsx.NodeText(stmt.AsImportDeclaration().ModuleSpecifier) != module {
			continue
		}
		for _, spec := range namedImports(stmt) {
			if importedAs(spec) == exported && rtsx.NodeText(spec.Name()) == local {
				return nil
			}
		}
	}
	text := fmt.Sprintf("import { %s as %s } from %q;", exported, local, module)
	if last != nil {
		return []emit.Edit{{Span: emit.Span{Pos: last.End(), End: last.End()}, Pieces: []emit.Piece{emit.Synth("\n"+text, origin)}}}
	}
	return []emit.Edit{{Span: emit.Span{}, Pieces: []emit.Piece{emit.Synth(text+"\n", origin)}}}
}

func namedImports(decl *rtsx.Node) []*rtsx.Node {
	clause := decl.AsImportDeclaration().ImportClause
	if clause == nil {
		return nil
	}
	bindings := clause.AsImportClause().NamedBindings
	if bindings == nil || bindings.Kind == rtsx.KindNamespaceImport {
		return nil
	}
	return bindings.Elements()
}

func importedAs(spec *rtsx.Node) string {
	if p := spec.PropertyName(); p != nil {
		return rtsx.NodeText(p)
	}
	return rtsx.NodeText(spec.Name())
}

// dropImports removes the named imports of module whose exported name is in
// drop; a declaration left with nothing is removed whole.
func (c *passContext) dropImports(module string, drop map[string]bool) []emit.Edit {
	var edits []emit.Edit
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind != rtsx.KindImportDeclaration || rtsx.NodeText(stmt.AsImportDeclaration().ModuleSpecifier) != module {
			continue
		}
		specs := namedImports(stmt)
		var keep []*rtsx.Node
		for _, spec := range specs {
			if !drop[importedAs(spec)] || spec.AsImportSpecifier().IsTypeOnly {
				keep = append(keep, spec)
			}
		}
		if len(keep) == len(specs) {
			continue
		}
		clause := stmt.AsImportDeclaration().ImportClause.AsImportClause()
		if len(keep) == 0 && clause.Name() == nil {
			whole := c.span(stmt)
			if strings.HasPrefix(c.text[whole.End:], "\r\n") {
				whole.End += 2
			} else if strings.HasPrefix(c.text[whole.End:], "\n") {
				whole.End++
			}
			edits = append(edits, emit.Edit{Span: whole})
			continue
		}
		bindings := c.span(clause.NamedBindings)
		pieces := []emit.Piece{emit.Synth("{ ", bindings)}
		for i, spec := range keep {
			if i > 0 {
				pieces = append(pieces, emit.Synth(", ", bindings))
			}
			pieces = append(pieces, c.copy(spec))
		}
		pieces = append(pieces, emit.Synth(" }", bindings))
		if len(keep) == 0 { // `import Default, {} from …` → `import Default from …`
			pieces = nil
			bindings.Pos = c.span(clause.Name()).End
		}
		edits = append(edits, emit.Edit{Span: bindings, Pieces: pieces})
	}
	return edits
}

// jsxChildren returns the meaningful children of a JSX element: without
// whitespace-only text and without empty `{/* comments */}`.
func jsxChildren(el *rtsx.Node) []*rtsx.Node {
	if el.Kind != rtsx.KindJsxElement {
		return nil
	}
	var out []*rtsx.Node
	for _, ch := range el.Children().Nodes {
		switch {
		case syntax.BlankText(ch):
		case ch.Kind == rtsx.KindJsxExpression && ch.Expression() == nil:
		default:
			out = append(out, ch)
		}
	}
	return out
}

// childrenSpan is the source between an element's opening and closing tags.
func (c *passContext) childrenSpan(el *rtsx.Node) emit.Span {
	e := el.AsJsxElement()
	return emit.Span{Pos: e.OpeningElement.End(), End: rtsx.TokenStart(c.file, e.ClosingElement)}
}

// body is the slot-body rule (syntax.md, *Usage and desugaring*): a single
// child element as it is, the expression of a single `{…}` child, and
// anything else — text, several children — in `<>…</>`; no children: null.
func (c *passContext) body(el *rtsx.Node, origin emit.Span) []emit.Piece {
	children := jsxChildren(el)
	if len(children) == 0 {
		return []emit.Piece{emit.Synth("null", origin)}
	}
	return c.bodyOf(children, []emit.Piece{emit.Copy(c.text, c.childrenSpan(el))}, origin)
}

// bodyOf applies the slot-body rule to children; all is the copied source
// of every child, used when they go in a fragment.
func (c *passContext) bodyOf(children []*rtsx.Node, all []emit.Piece, origin emit.Span) []emit.Piece {
	if len(children) == 1 {
		switch ch := children[0]; {
		case isJSXElement(ch):
			return []emit.Piece{c.copy(ch)}
		case ch.Kind == rtsx.KindJsxExpression && ch.AsJsxExpression().DotDotDotToken == nil:
			return c.operand(ch.Expression(), rtsx.PrecedenceComma)
		case ch.Kind == rtsx.KindJsxText && !strings.Contains(rtsx.NodeText(ch), "&"):
			// Text alone is a string, as React renders it: `"Text"`.
			return []emit.Piece{emit.Synth(jsString(cleanJSXText(rtsx.NodeText(ch))), c.span(ch))}
		}
	}
	return append(append([]emit.Piece{emit.Synth("<>", origin)}, all...), emit.Synth("</>", origin))
}

func isJSXElement(n *rtsx.Node) bool {
	switch n.Kind {
	case rtsx.KindJsxElement, rtsx.KindJsxSelfClosingElement, rtsx.KindJsxFragment:
		return true
	}
	return false
}

// operand copies expr, in parentheses when its precedence is not above min.
func (c *passContext) operand(expr *rtsx.Node, min int) []emit.Piece {
	if rtsx.Precedence(expr) > min {
		return []emit.Piece{c.copyValue(expr)}
	}
	s := c.span(expr)
	return []emit.Piece{emit.Synth("(", s), c.copy(expr), emit.Synth(")", s)}
}

// inPosition wraps a lowered expression for where the element stood: `{…}`
// as a JSX child or attribute value, `(…)` anywhere else (syntax.md,
// *Position*).
func inPosition(el *rtsx.Node, origin emit.Span, expr []emit.Piece) []emit.Piece {
	open, close := "(", ")"
	switch el.Parent.Kind {
	case rtsx.KindJsxElement, rtsx.KindJsxFragment, rtsx.KindJsxAttribute:
		open, close = "{", "}"
	}
	return append(append([]emit.Piece{emit.Synth(open, origin)}, expr...), emit.Synth(close, origin))
}

// attributes of a JSX element (opening or self-closing), by kind of use.
type attributes struct {
	named  map[string]*rtsx.Node // JsxAttribute by name
	order  []*rtsx.Node          // named attributes, in source order
	params *rtsx.Node            // the params pattern, if any
	other  []*rtsx.Node          // spreads, namespaced names
}

func readAttributes(el *rtsx.Node) attributes {
	opening := el
	if el.Kind == rtsx.KindJsxElement {
		opening = el.AsJsxElement().OpeningElement
	}
	a := attributes{named: map[string]*rtsx.Node{}}
	for _, attr := range opening.Attributes().Properties() {
		if pattern, ok := syntax.SlotParams(attr); ok {
			a.params = pattern
			continue
		}
		if attr.Kind == rtsx.KindJsxAttribute && attr.Name().Kind == rtsx.KindIdentifier {
			a.named[rtsx.NodeText(attr.Name())] = attr
			a.order = append(a.order, attr)
			continue
		}
		a.other = append(a.other, attr)
	}
	return a
}

// value is an attribute's value as an expression: `x={expr}` or `x="lit"`;
// nil for a bare attribute or `x={}`.
func value(attr *rtsx.Node) *rtsx.Node {
	init := attr.Initializer()
	switch {
	case init == nil:
		return nil
	case init.Kind == rtsx.KindJsxExpression:
		return init.Expression()
	}
	return init
}

// tagText is a JSX tag name as written: `Switch`, `$Case`, `Icons.Plus`.
func (c *passContext) tagText(el *rtsx.Node) string {
	if el.Kind == rtsx.KindJsxElement {
		el = el.AsJsxElement().OpeningElement
	}
	return strings.TrimSpace(c.text[c.span(el.TagName()).Pos:el.TagName().End()])
}

// openingSpan is the span of an element's opening tag — the origin of code
// synthesized for the element (diagnostics.md, *Mapping*).
func (c *passContext) openingSpan(el *rtsx.Node) emit.Span {
	if el.Kind == rtsx.KindJsxElement {
		return c.span(el.AsJsxElement().OpeningElement)
	}
	return c.span(el)
}

// cleanJSXText applies React's whitespace rule to JSX text: lines are
// trimmed (except the start of the first and the end of the last), empty
// lines dropped, the rest joined with one space.
func cleanJSXText(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var kept []string
	for i, line := range lines {
		if i > 0 {
			line = strings.TrimLeft(line, " \t")
		}
		if i < len(lines)-1 {
			line = strings.TrimRight(line, " \t")
		}
		if line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, " ")
}

// jsString is s as a JavaScript string literal.
func jsString(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
