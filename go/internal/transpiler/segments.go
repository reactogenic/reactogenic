package transpiler

import (
	"fmt"
	"path"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// segments is pass 4 (syntax.md, *Segment roots*):
//
//	<section #about-us className="band" />
//	→ import _Section_aboutUs from "./+about-us";
//	  <section id="about-us" className="band"><_Section_aboutUs /></section>
//
// A root's children are overwritten, roots inside them included (pass 0
// warned: segment-children).
func segments(c *passContext) []emit.Edit {
	var edits []emit.Edit
	mounted := map[*rtsx.Node]bool{} // elements: one root each (a second is segment-id)
	for _, attr := range syntax.SegmentRoots(c.file) {
		if insideRootChildren(attr) || mounted[attr.Parent] {
			continue
		}
		mounted[attr.Parent] = true
		name, _ := syntax.SegmentRoot(attr)
		opening := attr.Parent.Parent
		origin := c.span(attr)
		local := c.fresh("_" + tagIdentifier(c.tagText(opening)) + "_" + camel(name))

		edits = append(edits, emit.Edit{Span: origin, Pieces: []emit.Piece{emit.Synth(fmt.Sprintf("id=%q", name), origin)}})
		mount := emit.Synth("<"+local+" />", origin)
		if opening.Kind == rtsx.KindJsxSelfClosingElement {
			end := emit.Span{Pos: opening.End() - 2, End: opening.End()} // `/>`
			edits = append(edits, emit.Edit{Span: end, Pieces: []emit.Piece{
				emit.Synth(">", origin), mount, emit.Synth("</", origin), c.copy(opening.TagName()), emit.Synth(">", origin),
			}})
		} else {
			edits = append(edits, emit.Edit{Span: c.childrenSpan(opening.Parent), Pieces: []emit.Piece{mount}})
		}
		edits = append(edits, c.ensureDefaultImport("./+"+name, local, origin)...)
	}
	return edits
}

// insideRootChildren: attr's element sits in the children of another
// segment root, which overwrites it.
func insideRootChildren(attr *rtsx.Node) bool {
	for child, p := attr.Parent.Parent, attr.Parent.Parent.Parent; p != nil; child, p = p, p.Parent {
		if p.Kind != rtsx.KindJsxElement || child == p.AsJsxElement().OpeningElement {
			continue
		}
		for _, a := range p.AsJsxElement().OpeningElement.Attributes().Properties() {
			if _, ok := syntax.SegmentRoot(a); ok {
				return true
			}
		}
	}
	return false
}

// tagIdentifier: `section` → `Section`, `Ui.Card` → `UiCard`.
func tagIdentifier(tag string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(tag, func(r rune) bool { return r == '.' || r == ':' || r == '-' }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// camel: `about-us` → `aboutUs`.
func camel(name string) string {
	parts := strings.Split(name, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// ensureDefaultImport adds `import local from "module";` after the last
// import, once.
func (c *passContext) ensureDefaultImport(module, local string, origin emit.Span) []emit.Edit {
	key := module + "|default|" + local
	if c.imports[key] {
		return nil
	}
	c.imports[key] = true
	text := fmt.Sprintf("import %s from %q;", local, module)
	var last *rtsx.Node
	for _, stmt := range c.file.Statements.Nodes {
		if stmt.Kind == rtsx.KindImportDeclaration {
			last = stmt
		}
	}
	if last != nil {
		return []emit.Edit{{Span: emit.Span{Pos: last.End(), End: last.End()}, Pieces: []emit.Piece{emit.Synth("\n"+text, origin)}}}
	}
	return []emit.Edit{{Span: emit.Span{}, Pieces: []emit.Piece{emit.Synth(text+"\n\n", origin)}}}
}

// checkSegmentFiles reports segment-not-found (no `+name.rtsx` or
// `+name.tsx` next to the file) and segment-self (a segment that mounts
// itself, directly or through other segments).
func (c *passContext) checkSegmentFiles() {
	dir := path.Dir(c.entry)
	for _, attr := range syntax.SegmentRoots(c.file) {
		name, _ := syntax.SegmentRoot(attr)
		file, ok := c.segmentFile(dir, name)
		if !ok {
			c.errorAt(attr, "segment-not-found", "No segment `+%s.rtsx` or `+%s.tsx` next to `%s`", name, name, path.Base(c.entry))
			continue
		}
		if c.mountsEntry(file, map[string]bool{}) {
			c.errorAt(attr, "segment-self", "`#%s` mounts the segment it is written in", name)
		}
	}
}

func (c *passContext) segmentFile(dir, name string) (string, bool) {
	for _, ext := range []string{".rtsx", ".tsx"} {
		p := path.Join(dir, "+"+name+ext)
		if _, ok := c.readFile(p); ok {
			return p, true
		}
	}
	return "", false
}

// mountsEntry follows the segment roots of file, depth first, and reports
// whether they lead back to the entry.
func (c *passContext) mountsEntry(file string, seen map[string]bool) bool {
	if file == c.entry {
		return true
	}
	if seen[file] {
		return false
	}
	seen[file] = true
	text, _ := c.readFile(file)
	parsed := rtsx.ParseRTSX("/"+file, text)
	for _, attr := range syntax.SegmentRoots(parsed) {
		name, _ := syntax.SegmentRoot(attr)
		if next, ok := c.segmentFile(path.Dir(file), name); ok && c.mountsEntry(next, seen) {
			return true
		}
	}
	return false
}
