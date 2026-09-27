package check

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/project"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// slotConditionals reports a required slot filled only conditionally when
// the component attaches it without a fallback (syntax.md, *Conditional
// slots*): the false branch is NOT_ASSIGNED, and nothing would render.
//
// The transpiler notes both sides — slot-conditional at the caller, with the
// component's tag; attachment in the component, with or without fallback —
// and the checker joins them: which component the tag is, and whether it
// declares the slot as optional.
func slotConditionals(p *project.Project) []Report {
	if p.Program == nil {
		return nil
	}
	var reports []Report
	var c *rtsx.Checker
	for src, out := range p.Outputs() {
		for _, n := range out.Notes {
			if n.Kind != "slot-conditional" {
				continue
			}
			if c == nil {
				checker, release := rtsx.GetChecker(p.Program)
				defer release()
				c = checker
			}
			if !requiredWithoutFallback(p, c, src, out, n) {
				continue
			}
			text, _ := p.Source(src)
			line, col := emit.LineCol(text, n.Span.Pos)
			reports = append(reports, Report{File: src, Line: line, Col: col, Error: true, Code: "slot-conditional",
				Message: fmt.Sprintf("Required slot without fallback cannot be conditional: `%s` of `%s`", n.Name, text[n.Tag.Pos:n.Tag.End])})
		}
	}
	return reports
}

// requiredWithoutFallback reports whether the component at note.Tag declares
// note.Name as required, and attaches it only without a fallback. Anything it
// cannot tell — a component not written in .rtsx, a slot never attached —
// is not an error.
func requiredWithoutFallback(p *project.Project, c *rtsx.Checker, src string, out transpiler.Output, note transpiler.Note) bool {
	file := p.Program.GetSourceFile(tsxPath(src))
	if file == nil || out.Map == nil {
		return false
	}
	pos, ok := out.Map.Output(note.Tag.End - 1)
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
	declFile := rtsx.FileOf(decl)
	if declFile == nil {
		return false
	}
	container, ok := p.SourceOf(declFile.FileName())
	if !ok {
		return false
	}
	cout, _ := p.Output(container)
	if cout.Map == nil {
		return false
	}
	span := cout.Map.Source(emit.Span{Pos: decl.Pos(), End: decl.End()})
	attached := false
	for _, n := range cout.Notes {
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

func tsxPath(rtsxPath string) string {
	return rtsxPath[:len(rtsxPath)-len(".rtsx")] + ".tsx"
}
