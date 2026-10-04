package transpiler

import (
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// flow is pass 2 (syntax.md, *Flow control*): `Match` and `Switch` become
// conditional expressions. The pass repeats: each run lowers the elements
// that hold no other `Match` / `Switch`, so nested ones are lowered inside
// out and edits never overlap. When none are left, the `Switch` / `Match`
// import is dropped.
func flow(c *passContext) []emit.Edit {
	var edits []emit.Edit
	found := false
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if kind := flowKind(n); kind != "" {
			found = true
			if !containsFlow(n) {
				if kind == "Match" {
					edits = append(edits, c.lowerMatch(n)...)
				} else {
					edits = append(edits, c.lowerSwitch(n)...)
				}
				return false
			}
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	if !found {
		return c.dropImports(syntax.Package, map[string]bool{"Switch": true, "Match": true})
	}
	return edits
}

// flowKind is "Match" or "Switch" for an element of the framework's.
func flowKind(n *rtsx.Node) string {
	var tag *rtsx.Node
	switch n.Kind {
	case rtsx.KindJsxElement:
		tag = n.AsJsxElement().OpeningElement.TagName()
	case rtsx.KindJsxSelfClosingElement:
		tag = n.TagName()
	default:
		return ""
	}
	if name := syntax.FrameworkExport(tag); name == "Match" || name == "Switch" {
		return name
	}
	return ""
}

func containsFlow(el *rtsx.Node) bool {
	found := false
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if flowKind(n) != "" {
			found = true
			return true
		}
		return n.ForEachChild(visit)
	}
	el.ForEachChild(visit)
	return found
}

// replace lowers el to expr, wrapped for its position.
func (c *passContext) replace(el *rtsx.Node, origin emit.Span, expr []emit.Piece) []emit.Edit {
	return []emit.Edit{{Span: c.span(el), Pieces: inPosition(el, origin, expr)}}
}

// failed replaces an element that cannot be lowered with null. Its errors
// are already reported and stop the build — or, in the editor, were dropped
// with a half-typed tag; either way its code is no longer in the output.
func (c *passContext) failed(el *rtsx.Node, origin emit.Span) []emit.Edit {
	c.leftOut(el)
	return c.replace(el, origin, []emit.Piece{emit.Synth("null", origin)})
}

// subject reads `on`; it reports flow-no-subject when it is missing.
func (c *passContext) subject(el *rtsx.Node, a attributes, tag string) *rtsx.Node {
	on := a.named["on"]
	if on == nil || value(on) == nil {
		c.errorAt(el, "flow-no-subject", "`%s` requires `on`", tag)
		return nil
	}
	return value(on)
}

// flowAttributes reports every attribute but the allowed ones
// (flow-attribute); `key` is never allowed on `Switch` or `Match`.
func (c *passContext) flowAttributes(a attributes, allowed ...string) bool {
	ok := true
	allow := map[string]bool{}
	for _, name := range allowed {
		allow[name] = true
	}
	for _, attr := range a.order {
		if !allow[rtsx.NodeText(attr.Name())] && c.invalid(attr, "flow-attribute", "`%s` is not an attribute of this element", rtsx.NodeText(attr.Name())) {
			ok = false
		}
	}
	for _, attr := range a.other {
		if c.invalid(attr, "flow-attribute", "Only the documented attributes are allowed here") {
			ok = false
		}
	}
	return ok
}

// lowerMatch: `<Match on={x}>body</Match>` → `x ? body : null`; with params,
// `(({ value: v }) => v ? body : null)({ value: x })`.
func (c *passContext) lowerMatch(el *rtsx.Node) []emit.Edit {
	origin := c.openingSpan(el)
	a := readAttributes(el)
	ok := c.flowAttributes(a, "on")
	on := c.subject(el, a, "Match")
	if !ok || on == nil {
		return c.failed(el, origin)
	}
	body := c.body(el, origin)
	if a.params == nil {
		expr := append(c.operand(on, rtsx.PrecedenceConditional), emit.Synth(" ? ", origin))
		expr = append(append(expr, body...), emit.Synth(" : null", origin))
		return c.replace(el, origin, expr)
	}
	pattern := c.copy(a.params)
	arg := append(append([]emit.Piece{emit.Synth(")({ value: ", origin)}, c.operand(on, rtsx.PrecedenceComma)...), emit.Synth(" })", origin))
	if name := valueName(a.params); name != nil {
		// The test is the name `value` is bound to, so TS narrows it in the body.
		expr := []emit.Piece{emit.Synth("((", origin), pattern, emit.Synth(") => ", origin), c.copy(name), emit.Synth(" ? ", origin)}
		expr = append(append(expr, body...), emit.Synth(" : null", origin))
		return c.replace(el, origin, append(expr, arg...))
	}
	// `value` is destructured further (`{ value: { name } }`): test the whole
	// value, then destructure it for the body.
	v := c.unique("_value", c.sourceText(on))
	expr := []emit.Piece{emit.Synth("(("+v+") => "+v+" ? ((", origin), pattern, emit.Synth(") => ", origin)}
	expr = append(append(expr, body...), emit.Synth(")({ value: "+v+" }) : null)(", origin))
	expr = append(append(expr, c.operand(on, rtsx.PrecedenceComma)...), emit.Synth(")", origin))
	return c.replace(el, origin, expr)
}

// valueName is the identifier a params pattern binds `value` to, if it binds
// it to a plain name (`{ value }`, `{ value: user }`).
func valueName(pattern *rtsx.Node) *rtsx.Node {
	for _, el := range pattern.Elements() {
		prop := el.PropertyName()
		if prop == nil {
			prop = el.Name()
		}
		if prop != nil && rtsx.NodeText(prop) == "value" && el.Name().Kind == rtsx.KindIdentifier {
			return el.Name()
		}
	}
	return nil
}
