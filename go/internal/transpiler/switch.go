package transpiler

import (
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
)

// switchCase is one `$Case` of a `Switch`.
type switchCase struct {
	el    *rtsx.Node
	is    *rtsx.Node // the test; nil for default
	key   *rtsx.Node // the `key` attribute, if any
	isDef bool
}

// lowerSwitch (syntax.md, *Flow control*):
//
//	static:    ((_on) => _on === a ? A : _on === b ? B : null)(on)
//	reference: on === a ? A : on === b ? B : null   (so TS narrows `on`)
//	dynamic:   (({ value: n }) => c1 ? A : c2 ? B : null)({ value: on })
//
// `exhaustive` ends in `_noMatch(_on)` instead of null; a default case ends
// in its body.
func (c *passContext) lowerSwitch(el *rtsx.Node) []emit.Edit {
	origin := c.openingSpan(el)
	a := readAttributes(el)
	ok := c.flowAttributes(a, "on", "exhaustive")
	on := c.subject(el, a, "Switch")
	exhaustive := a.named["exhaustive"]
	if exhaustive != nil && exhaustive.Initializer() != nil {
		c.errorAt(exhaustive, "flow-attribute", "`exhaustive` takes no value")
		ok = false
	}
	dynamic := a.params != nil
	if exhaustive != nil && dynamic {
		c.errorAt(exhaustive, "switch-dynamic-exhaustive", "Dynamic switch cannot be exhaustive")
		ok = false
	}
	cases, casesOK := c.switchCases(el)
	ok = ok && casesOK
	if exhaustive != nil && len(cases) > 0 && cases[len(cases)-1].isDef {
		c.errorAt(cases[len(cases)-1].el, "switch-exhaustive-default", "Exhaustive switch takes no `default`")
		ok = false
	}
	if !ok || on == nil {
		return c.failed(el, origin)
	}

	var edits []emit.Edit
	reference := !dynamic && isReference(on)
	subject := func() []emit.Piece { // the value compared in each test
		if reference {
			return []emit.Piece{c.copy(on)}
		}
		return nil // filled per mode below
	}
	var onName string
	if !dynamic && !reference {
		onName = c.unique("_on", c.sourceText(on))
		subject = func() []emit.Piece { return []emit.Piece{emit.Synth(onName, origin)} }
	}

	var chain []emit.Piece
	for _, sc := range cases {
		if sc.isDef {
			break
		}
		var test []emit.Piece
		if dynamic {
			test = c.operand(sc.is, rtsx.PrecedenceConditional)
		} else {
			test = append(append(subject(), emit.Synth(" === ", origin)), c.operand(sc.is, rtsx.PrecedenceEquality)...)
		}
		chain = append(append(chain, test...), emit.Synth(" ? ", origin))
		body, e := c.caseBody(sc, origin)
		edits = append(edits, e...)
		chain = append(append(chain, body...), emit.Synth("\n  : ", origin))
	}
	switch last := len(cases) - 1; {
	case last >= 0 && cases[last].isDef:
		body, e := c.caseBody(cases[last], origin)
		edits = append(edits, e...)
		chain = append(chain, body...)
	case exhaustive != nil:
		c.note(origin, "no-match", "", "")
		noMatch := c.fresh("_noMatch")
		c.generated(noMatch, "noMatch")
		edits = append(edits, c.ensureImport("@reactogenic/core", "noMatch", noMatch, origin)...)
		chain = append(append(append(chain, emit.Synth(noMatch+"(", origin)), subject()...), emit.Synth(")", origin))
	default:
		chain = append(chain, emit.Synth("null", origin))
	}

	var expr []emit.Piece
	switch {
	case reference:
		expr = chain
	case dynamic:
		expr = append([]emit.Piece{emit.Synth("((", origin), c.copy(a.params), emit.Synth(") =>\n  ", origin)}, chain...)
		expr = append(append(append(expr, emit.Synth("\n)({ value: ", origin)), c.operand(on, rtsx.PrecedenceComma)...), emit.Synth(" })", origin))
	default:
		expr = append([]emit.Piece{emit.Synth("(("+onName+") =>\n  ", origin)}, chain...)
		expr = append(append(append(expr, emit.Synth("\n)(", origin)), c.operand(on, rtsx.PrecedenceComma)...), emit.Synth(")", origin))
	}
	return append(edits, c.replace(el, origin, expr)...)
}

// switchCases reads and checks the `$Case` children of a `Switch`. A child
// whose error is dropped — it is half-typed (ide.md, *Tolerance*) — is left
// out, and the `Switch` is lowered around it.
func (c *passContext) switchCases(el *rtsx.Node) ([]switchCase, bool) {
	ok := true
	var cases []switchCase
	for _, ch := range jsxChildren(el) {
		if !isJSXElement(ch) || ch.Kind == rtsx.KindJsxFragment || c.tagText(ch) != "$Case" {
			if c.invalid(ch, "switch-children", "Only `$Case` is allowed here") {
				ok = false
			}
			continue
		}
		a := readAttributes(ch)
		sc := switchCase{el: ch, key: a.named["key"]}
		is, def := a.named["is"], a.named["default"]
		if a.params != nil {
			c.errorAt(a.params, "case-params", "`$Case` provides no values; params go on the `Switch`")
			ok = false
		}
		for _, attr := range a.order {
			if name := rtsx.NodeText(attr.Name()); name != "is" && name != "default" && name != "key" {
				c.errorAt(attr, "flow-attribute", "`%s` is not an attribute of `$Case`", name)
				ok = false
			}
		}
		for _, attr := range a.other {
			c.errorAt(attr, "flow-attribute", "Only `is`, `default` and `key` are allowed on `$Case`")
			ok = false
		}
		switch {
		case is != nil && def != nil:
			c.errorAt(ch, "case-both", "`$Case` takes `is` or `default`, not both")
			ok = false
		case def != nil && def.Initializer() != nil:
			c.errorAt(def, "case-default-value", "`default` takes no value")
			ok = false
		case def != nil:
			sc.isDef = true
		case is == nil || value(is) == nil:
			if !c.invalid(ch, "case-no-test", "`$Case` requires `is`") {
				continue // being typed: not a case yet
			}
			ok = false
		default:
			sc.is = value(is)
		}
		if len(cases) > 0 && cases[len(cases)-1].isDef {
			c.errorAt(ch, "case-default-not-last", "`default` must be the last `$Case`")
			ok = false
		}
		cases = append(cases, sc)
	}
	return cases, ok
}

// caseBody is a case's body; with `key`, `<_Fragment key=…>children</_Fragment>`.
func (c *passContext) caseBody(sc switchCase, origin emit.Span) ([]emit.Piece, []emit.Edit) {
	if sc.key == nil {
		return c.body(sc.el, origin), nil
	}
	fragment := c.fresh("_Fragment")
	edits := c.ensureImport("react", "Fragment", fragment, origin)
	pieces := []emit.Piece{emit.Synth("<"+fragment+" ", origin), c.copy(sc.key), emit.Synth(">", origin)}
	if sc.el.Kind == rtsx.KindJsxElement {
		pieces = append(pieces, emit.Copy(c.text, c.childrenSpan(sc.el)))
	}
	return append(pieces, emit.Synth("</"+fragment+">", origin)), edits
}

// isReference: an identifier, `this`, or a property-access chain of them —
// a subject that can be repeated so TS narrows it in each branch.
func isReference(n *rtsx.Node) bool {
	switch n.Kind {
	case rtsx.KindIdentifier, rtsx.KindThisKeyword:
		return true
	case rtsx.KindPropertyAccessExpression:
		return n.AsPropertyAccessExpression().QuestionDotToken == nil && isReference(n.Expression())
	}
	return false
}
