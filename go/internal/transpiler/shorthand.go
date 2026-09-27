package transpiler

import (
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// shorthand is pass 1 (syntax.md, *Shorthand props*): a bare attribute whose
// name has a value binding in scope becomes `name={name}` (case A); any
// other bare attribute keeps React's meaning, `true` (case B).
//
// Names that can never be bindings — `aria-label`, `xlink:href`, reserved
// words, segment roots — resolve to nothing and so stay case B.
func shorthand(c *passContext) []emit.Edit {
	var edits []emit.Edit
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxAttribute && n.Initializer() == nil {
			if name := n.Name(); name.Kind == rtsx.KindIdentifier {
				text := rtsx.NodeText(name)
				_, isSegment := syntax.SegmentRoot(n)
				_, arg := syntax.SlotArg(n)
				if !isSegment && arg == syntax.NotArg && syntax.Binding(n, text) == nil {
					c.note(c.span(n), "shorthand-true", text, "")
				}
				if !isSegment && arg == syntax.NotArg && syntax.Binding(n, text) != nil {
					attr := emit.Span{Pos: rtsx.TokenStart(c.file, n), End: n.End()}
					nameSpan := emit.Span{Pos: rtsx.TokenStart(c.file, name), End: name.End()}
					edits = append(edits, emit.Edit{
						Span: emit.Span{Pos: nameSpan.End, End: nameSpan.End},
						Pieces: []emit.Piece{
							emit.Synth("={", attr),
							emit.Copy(c.text, nameSpan),
							emit.Synth("}", attr),
						},
					})
				}
			}
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	return edits
}
