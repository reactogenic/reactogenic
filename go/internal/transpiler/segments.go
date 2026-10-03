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
//	→ import _Section_aboutUs from "./about-us.rtsx";
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
		local := c.unique("_"+tagIdentifier(c.tagText(opening))+"_"+camel(name), "#"+name)
		c.note(origin, "segment", name, c.tagText(opening))

		edits = append(edits, emit.Edit{Span: origin, Pieces: []emit.Piece{emit.Synth(fmt.Sprintf("id=%q", name), origin)}})
		mount := emit.Synth("<"+local+" />", origin)
		if opening.Kind == rtsx.KindJsxSelfClosingElement && !strings.HasSuffix(c.text[:opening.End()], "/>") {
			// A recovered element still missing its `/>`: only the id.
		} else if opening.Kind == rtsx.KindJsxSelfClosingElement {
			end := emit.Span{Pos: opening.End() - 2, End: opening.End()} // `/>`
			edits = append(edits, emit.Edit{Span: end, Pieces: []emit.Piece{
				emit.Synth(">", origin), mount, emit.Synth("</", origin), c.copy(opening.TagName()).Lacking(emit.AllFeatures), emit.Synth(">", origin),
			}})
		} else {
			edits = append(edits, emit.Edit{Span: c.childrenSpan(opening.Parent), Pieces: []emit.Piece{mount}})
		}
		// The import names the file found, extension included: an
		// extensionless one would resolve by TS's and Vite's own orders.
		module := "./" + name
		if file, ok := c.segmentFile(path.Dir(c.entry), name); ok {
			module = "./" + path.Base(file)
		}
		edits = append(edits, c.ensureDefaultImport(module, local, origin)...)
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

// checkAmbiguousModule reports ambiguous-module: `Foo.tsx` next to
// `Foo.rtsx` makes an import of `./Foo` ambiguous (vite.md, *Module
// resolution*).
func (c *passContext) checkAmbiguousModule() {
	if !strings.HasSuffix(c.entry, ".rtsx") {
		return
	}
	tsx := strings.TrimSuffix(c.entry, ".rtsx") + ".tsx"
	if _, ok := c.readFile(tsx); ok {
		c.report(emit.Span{}, Error, "ambiguous-module", fmt.Sprintf("`%s` and `%s` side by side: an import of `./%s` is ambiguous", path.Base(tsx), path.Base(c.entry), strings.TrimSuffix(path.Base(tsx), ".tsx")))
	}
}

// checkSegmentFiles reports segment-not-found (no file of the name next to
// the entry, see SegmentExtensions) and segment-self (a segment that mounts
// itself, directly or through other segments).
func (c *passContext) checkSegmentFiles() {
	dir := path.Dir(c.entry)
	for _, attr := range syntax.SegmentRoots(c.file) {
		name, _ := syntax.SegmentRoot(attr)
		file, ok := c.segmentFile(dir, name)
		if !ok {
			c.errorAt(attr, "segment-not-found", "No segment `%s` next to `%s`: looked for `%s.rtsx`, `.tsx`, `.jsx`, `.ts`, `.js`", name, path.Base(c.entry), name)
			continue
		}
		if c.mountsEntry(file, map[string]bool{}) {
			c.errorAt(attr, "segment-self", "`#%s` mounts the segment it is written in", name)
		}
	}
}

// SegmentExtensions is the lookup order of `#name` (syntax.md, *Segment
// files*): the first `name<ext>` next to the file is the segment.
var SegmentExtensions = []string{".rtsx", ".tsx", ".jsx", ".ts", ".js"}

func (c *passContext) segmentFile(dir, name string) (string, bool) {
	for _, ext := range SegmentExtensions {
		p := path.Join(dir, name+ext)
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
	if seen[file] || !strings.HasSuffix(file, ".rtsx") { // only .rtsx has segment roots
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
