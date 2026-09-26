package transpiler

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
)

// renderSlots lowers the container side of slots (syntax.md, *Rendering a
// slot*):
//
//	<div slot={$IconEnd} size />
//	→ {$IconEnd ? <div {...$IconEnd}>{_renderSlot($IconEnd.children, { size: size })}</div> : null}
//
// A slot-rendering element has no children, so these never nest; pass 3
// lowers them all in its first run.
func (c *passContext) renderSlots() []emit.Edit {
	var edits []emit.Edit
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if ref, slotAttr := slotRender(n); ref != nil {
			edits = append(edits, c.renderSlot(n, ref, slotAttr)...)
			return false
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	return edits
}

// slotRender returns the `$` reference of an element's `slot={$X}`, and
// the attribute. Any other `slot` keeps its HTML meaning.
func slotRender(el *rtsx.Node) (*rtsx.Node, *rtsx.Node) {
	var opening *rtsx.Node
	switch el.Kind {
	case rtsx.KindJsxElement:
		opening = el.AsJsxElement().OpeningElement
	case rtsx.KindJsxSelfClosingElement:
		opening = el
	default:
		return nil, nil
	}
	for _, attr := range opening.Attributes().Properties() {
		if attr.Kind != rtsx.KindJsxAttribute || attr.Name().Kind != rtsx.KindIdentifier || rtsx.NodeText(attr.Name()) != "slot" {
			continue
		}
		if v := value(attr); v != nil && isSlotReference(v) {
			return v, attr
		}
	}
	return nil, nil
}

// isSlotReference: `$X`, or a property-access chain ending in `$X`.
func isSlotReference(n *rtsx.Node) bool {
	switch n.Kind {
	case rtsx.KindIdentifier:
		return strings.HasPrefix(rtsx.NodeText(n), "$")
	case rtsx.KindPropertyAccessExpression:
		return isReference(n) && strings.HasPrefix(rtsx.NodeText(n.Name()), "$")
	}
	return false
}

func (c *passContext) renderSlot(el, ref, slotAttr *rtsx.Node) []emit.Edit {
	origin := c.openingSpan(el)
	opening := el
	if el.Kind == rtsx.KindJsxElement {
		opening = el.AsJsxElement().OpeningElement
		if len(jsxChildren(el)) > 0 {
			c.errorAt(jsxChildren(el)[0], "slot-render-children",
				"The slot's body is its content; `%s` takes no children here", c.tagText(el))
		}
	}
	var args []*rtsx.Node
	var key *rtsx.Node
	for _, attr := range opening.Attributes().Properties() {
		switch {
		case attr == slotAttr:
		case attr.Kind == rtsx.KindJsxAttribute && attr.Name().Kind == rtsx.KindIdentifier && rtsx.NodeText(attr.Name()) == "key":
			key = attr
		default:
			args = append(args, attr)
		}
	}
	render := c.fresh("_renderSlot")
	edits := c.ensureImport("@reactogenic/core", "renderSlot", render, origin)

	tag := emit.Span{Pos: c.span(opening.TagName()).Pos, End: opening.Attributes().Pos()}
	tag.End = tag.Pos + len(strings.TrimRight(c.text[tag.Pos:tag.End], " \t\r\n"))
	expr := append([]emit.Piece{}, c.copy(ref))
	expr = append(expr, emit.Synth(" ? <", origin), emit.Copy(c.text, tag), emit.Synth(" {...", origin), c.copy(ref), emit.Synth("}", origin))
	if key != nil {
		expr = append(expr, emit.Synth(" ", origin), c.copy(key))
	}
	expr = append(expr, emit.Synth(">{"+render+"(", origin), c.copy(ref), emit.Synth(".children, ", origin))
	expr = append(expr, objectLiteral(c.attributeProps(args), origin)...)
	expr = append(expr, emit.Synth(")}</", origin), c.copy(opening.TagName()), emit.Synth("> : null", origin))
	return append(edits, c.replace(el, origin, expr)...)
}
