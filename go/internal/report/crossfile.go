package report

import (
	"context"
	"fmt"
	"path"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// The cross-file rules (ide.md, *Diagnostics*): what the transpiler noted in
// one file, joined with the program's other files. A transform sees its own
// file and the names of its siblings, never their contents — its result is
// cached by exactly that — so these cannot be the transpiler's.

// slotConditionals reports a required slot filled only conditionally when
// the component attaches it without a fallback (syntax.md, *Conditional
// slots*): the false branch is NOT_ASSIGNED, and nothing would render.
//
// The transpiler notes both sides — slot-conditional at the caller, with the
// component's tag; attachment in the component, with or without fallback —
// and the checker joins them: which component the tag is, and whether it
// declares the slot as optional.
func slotConditionals(ctx context.Context, p *rtsx.Program, file *rtsx.SourceFile, f *mapper.File, source string) []Report {
	var reports []Report
	var c *rtsx.Checker
	for _, n := range f.Notes {
		if n.Kind != "slot-conditional" {
			continue
		}
		if c == nil {
			checker, release := rtsx.GetChecker(ctx, p, file)
			defer release()
			c = checker
		}
		if !requiredWithoutFallback(c, file, f, n) {
			continue
		}
		line, col := emit.LineCol(source, n.Span.Pos)
		reports = append(reports, Report{File: file.FileName(), Span: n.Span, Line: line, Col: col, Code: "slot-conditional",
			Message: fmt.Sprintf("Required slot without fallback cannot be conditional: `%s` of `%s`", n.Name, source[n.Tag.Pos:n.Tag.End])})
	}
	return reports
}

// requiredWithoutFallback reports whether the component at note.Tag declares
// note.Name as required, and attaches it only without a fallback. Anything it
// cannot tell — a component not written in .rtsx, or in a file that is
// stopped; a slot never attached — is not an error.
func requiredWithoutFallback(c *rtsx.Checker, file *rtsx.SourceFile, f *mapper.File, note transpiler.Note) bool {
	if f.Map == nil {
		return false
	}
	pos, ok := f.Map.Output(note.Tag.End - 1)
	if !ok {
		return false
	}
	tag := rtsx.TokenAt(file, pos)
	if tag == nil {
		return false
	}
	decl, declared, optional := rtsx.SlotDeclaration(c, tag, note.Name)
	if !declared || optional || decl == nil {
		return false
	}
	container, ok := mapper.Of(rtsx.FileOf(decl))
	if !ok || container.Stopped || container.Map == nil {
		return false
	}
	span := container.Map.Source(emit.Span{Pos: decl.Pos(), End: decl.End()})
	attached := false
	for _, n := range container.Notes {
		if n.Kind != "attachment" || n.Name != note.Name || n.Span.Pos < span.Pos || n.Span.End > span.End {
			continue
		}
		if n.Detail == "fallback" {
			return false
		}
		attached = true
	}
	return attached
}

// segmentSelf reports a segment root that mounts the segment it is written
// in through other files (syntax.md, *Segment roots*): `#outro` in
// intro.rtsx, when outro.rtsx — or a segment it mounts — mounts `#intro`.
// The loop is followed over the program's mapped files and their notes. A
// root that names its own file is the transpiler's to report.
func segmentSelf(p *rtsx.Program, file *rtsx.SourceFile, f *mapper.File, source string) []Report {
	var reports []Report
	for _, n := range f.Notes {
		if n.Kind != "segment" {
			continue
		}
		next := mounted(p, file, n)
		if next == nil || next == file || !mounts(p, next, file, map[*rtsx.SourceFile]bool{}) {
			continue
		}
		line, col := emit.LineCol(source, n.Span.Pos)
		reports = append(reports, Report{File: file.FileName(), Span: n.Span, Line: line, Col: col, Code: "segment-self",
			Message: fmt.Sprintf("`#%s` mounts the segment it is written in", n.Name)})
	}
	return reports
}

// mounted is the .rtsx file that a segment root of file mounts: `name.rtsx`
// next to it — the first of the lookup (syntax.md, *Segment files*), and in
// the program through the import the root emits. nil: the root mounts
// another kind of file, which has no roots of its own, or none.
func mounted(p *rtsx.Program, file *rtsx.SourceFile, note transpiler.Note) *rtsx.SourceFile {
	next := p.GetSourceFile(path.Join(path.Dir(file.FileName()), note.Name+".rtsx"))
	if _, ok := mapper.Of(next); !ok {
		return nil
	}
	return next
}

// mounts follows the segment roots of from, depth first, and reports whether
// they lead to target.
func mounts(p *rtsx.Program, from, target *rtsx.SourceFile, seen map[*rtsx.SourceFile]bool) bool {
	if from == target {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	f, _ := mapper.Of(from)
	for _, n := range f.Notes {
		if n.Kind != "segment" {
			continue
		}
		if next := mounted(p, from, n); next != nil && mounts(p, next, target, seen) {
			return true
		}
	}
	return false
}
