package transpiler

import (
	"regexp"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// slots is pass 3 (syntax.md, *Slots*): `slot={$X}` renders a slot, slot
// elements become `$X` props of their container, and params become a
// `children` callback.
//
// Like pass 2 it repeats, inside out: a container is rewritten only once no
// container below it is left. Orphaned slot elements are reported and removed
// first, so every run makes progress.
func slots(c *passContext) []emit.Edit {
	if edits := c.renderSlots(); len(edits) > 0 {
		return edits
	}
	if edits := c.removeOrphans(); len(edits) > 0 {
		return edits
	}
	var edits []emit.Edit
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if c.isUnit(n) && !c.hasUnitBelow(n) {
			edits = append(edits, c.hoist(n)...)
			return false
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	return edits
}

// tagOf is the tag name of an element (opening, self-closing or whole).
func tagOf(el *rtsx.Node) *rtsx.Node {
	switch el.Kind {
	case rtsx.KindJsxElement:
		return el.AsJsxElement().OpeningElement.TagName()
	case rtsx.KindJsxSelfClosingElement:
		return el.TagName()
	}
	return nil
}

// isSlotElement: an element whose tag starts with `$` — unless a binding of
// that name is in scope (then it is a component, and pass 0 reported
// component-name).
func isSlotElement(n *rtsx.Node) bool {
	tag := tagOf(n)
	if tag == nil || tag.Kind != rtsx.KindIdentifier {
		return false
	}
	name := rtsx.NodeText(tag)
	return strings.HasPrefix(name, "$") && syntax.Binding(tag, name) == nil
}

// isContainer: a component element — not intrinsic, not a slot element.
func isContainer(n *rtsx.Node) bool {
	tag := tagOf(n)
	return tag != nil && !rtsx.IsIntrinsicTag(tag) && !isSlotElement(n)
}

func unwrapParens(n *rtsx.Node) *rtsx.Node {
	for n != nil && n.Kind == rtsx.KindParenthesizedExpression {
		n = n.Expression()
	}
	return n
}

// isUnit: a container with slot children (direct or conditional), or with
// params.
func (c *passContext) isUnit(n *rtsx.Node) bool {
	if !isContainer(n) {
		return false
	}
	if readAttributes(n).params != nil {
		return true
	}
	if n.Kind != rtsx.KindJsxElement {
		return false
	}
	for _, ch := range n.Children().Nodes {
		if isSlotElement(ch) || slotConditional(ch) != nil {
			return true
		}
	}
	return false
}

func (c *passContext) hasUnitBelow(el *rtsx.Node) bool {
	found := false
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if c.isUnit(n) {
			found = true
			return true
		}
		return n.ForEachChild(visit)
	}
	el.ForEachChild(visit)
	return found
}

// slotConditional returns the conditional expression of a `{…}` child whose
// branches hold slot elements (syntax.md, *Conditional slots*), or nil.
func slotConditional(ch *rtsx.Node) *rtsx.Node {
	if ch.Kind != rtsx.KindJsxExpression || ch.Expression() == nil {
		return nil
	}
	expr := unwrapParens(ch.Expression())
	if expr.Kind != rtsx.KindConditionalExpression {
		return nil
	}
	var hasSlot func(n *rtsx.Node) bool
	hasSlot = func(n *rtsx.Node) bool {
		n = unwrapParens(n)
		switch {
		case n.Kind == rtsx.KindConditionalExpression:
			ce := n.AsConditionalExpression()
			return hasSlot(ce.WhenTrue) || hasSlot(ce.WhenFalse)
		case isSlotElement(n):
			return true
		case n.Kind == rtsx.KindJsxFragment:
			for _, f := range n.Children().Nodes {
				if isSlotElement(f) {
					return true
				}
			}
		}
		return false
	}
	if hasSlot(expr) {
		return expr
	}
	return nil
}

// claimed reports whether a slot element has an owner: it is a direct child
// of a component or of a slot element, or a branch (possibly inside a
// fragment) of a conditional that is (syntax.md, *Slots → Placement*).
func claimed(slot *rtsx.Node) bool {
	n := slot.Parent
	if n.Kind == rtsx.KindJsxElement {
		return isOwner(n)
	}
	if n.Kind == rtsx.KindJsxFragment { // a fragment branch: reported as mixed-conditional-slot
		n = n.Parent
	}
	for n.Kind == rtsx.KindParenthesizedExpression || n.Kind == rtsx.KindConditionalExpression {
		n = n.Parent
	}
	return n.Kind == rtsx.KindJsxExpression && n.Parent.Kind == rtsx.KindJsxElement && isOwner(n.Parent) && slotConditional(n) != nil
}

// isOwner: an element whose slots its slot children fill — a component, or
// a slot element (*Recursive slots*).
func isOwner(n *rtsx.Node) bool {
	return isContainer(n) || isSlotElement(n)
}

// removeOrphans reports every slot element without a container
// (orphan-slot) and removes it, outermost first.
func (c *passContext) removeOrphans() []emit.Edit {
	var edits []emit.Edit
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if isSlotElement(n) && !claimed(n) {
			c.errorAt(n, "orphan-slot", "Slot must be immediate child of the component")
			edits = append(edits, emit.Edit{Span: c.span(n), Pieces: inPosition(n, c.openingSpan(n), []emit.Piece{emit.Synth("null", c.openingSpan(n))})})
			return false
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	return edits
}

// hoist rewrites one container: `<P a>…<$X o>body</$X>…</P>` →
// `<P a $X={{ o, children: body }}>…</P>`.
func (c *passContext) hoist(p *rtsx.Node) []emit.Edit {
	origin := c.openingSpan(p)
	opening := p
	if p.Kind == rtsx.KindJsxElement {
		opening = p.AsJsxElement().OpeningElement
	}
	a := readAttributes(p)
	names, values, replaced, remaining := c.assignSlots(p, a)

	// The new element: written attributes (minus params and replaced `$X=`),
	// then the slot props.
	tag := emit.Span{Pos: c.span(opening.TagName()).Pos, End: opening.Attributes().Pos()}
	tag.End = tag.Pos + len(strings.TrimRight(c.text[tag.Pos:tag.End], " \t\r\n"))
	out := []emit.Piece{emit.Synth("<", origin), emit.Copy(c.text, tag)}
	for _, attr := range opening.Attributes().Properties() {
		if _, isParams := syntax.SlotParams(attr); isParams || replaced[attr] {
			continue
		}
		out = append(out, emit.Synth(" ", origin), c.copy(attr))
	}
	for _, name := range names {
		out = append(out, emit.Synth(" "+name+"={", origin))
		out = append(append(out, values[name]...), emit.Synth("}", origin))
	}

	meaningful := meaningfulChildren(remaining)
	var children []emit.Piece
	for _, ch := range remaining {
		children = append(children, c.copyChild(ch))
	}
	open := len(meaningful) > 0
	if a.params != nil {
		// Params on the component: its children become a callback.
		body := []emit.Piece{emit.Synth("null", origin)}
		if len(meaningful) > 0 {
			body = c.bodyOf(meaningful, children, origin)
		}
		children = append([]emit.Piece{emit.Synth("{(", origin), c.copy(a.params), emit.Synth(") => ", origin)}, body...)
		children = append(children, emit.Synth("}", origin))
		open = true
	}
	if !open {
		out = append(out, emit.Synth(" />", origin))
	} else {
		out = append(out, emit.Synth(">", origin))
		out = append(out, children...)
		out = append(out, emit.Synth("</", origin), c.copy(opening.TagName()), emit.Synth(">", origin))
	}
	return []emit.Edit{{Span: c.span(p), Pieces: out}}
}

func meaningfulChildren(children []*rtsx.Node) []*rtsx.Node {
	var out []*rtsx.Node
	for _, ch := range children {
		if !(ch.Kind == rtsx.KindJsxText && ch.AsJsxText().ContainsOnlyTriviaWhiteSpaces) && !(ch.Kind == rtsx.KindJsxExpression && ch.Expression() == nil) {
			out = append(out, ch)
		}
	}
	return out
}

// assignSlots folds the slot children of an owner (a component or a slot
// element) into one value per slot, in source order: a slot element assigns
// its value; a conditional assigns in its branches and keeps the previous
// value in its null branches — the last assignment wins (syntax.md, *Slots →
// Repeated slots*). An explicit `$X={…}` attribute of the owner is the first
// assignment; when a slot element follows, the attribute is replaced.
//
// It returns the slots in order of first appearance, their values, the
// replaced attributes, and the children that are not slots.
func (c *passContext) assignSlots(owner *rtsx.Node, a attributes) ([]string, map[string][]emit.Piece, map[*rtsx.Node]bool, []*rtsx.Node) {
	var names []string
	values := map[string][]emit.Piece{}
	replaced := map[*rtsx.Node]bool{}
	var rest []*rtsx.Node
	previous := func(name string) []emit.Piece {
		if v, ok := values[name]; ok {
			return v
		}
		names = append(names, name)
		if attr := a.named[name]; attr != nil && value(attr) != nil {
			replaced[attr] = true
			return c.operand(value(attr), rtsx.PrecedenceComma)
		}
		return nil
	}
	if owner.Kind != rtsx.KindJsxElement {
		return nil, values, replaced, nil
	}
	for _, ch := range owner.Children().Nodes {
		switch {
		case isSlotElement(ch):
			name := rtsx.NodeText(tagOf(ch))
			previous(name)
			values[name] = c.slotObject(ch)
		case slotConditional(ch) != nil:
			cond := slotConditional(ch)
			if name, ok := c.conditionalSlotName(cond); ok {
				values[name] = c.conditionalValue(cond, previous(name), c.openingSpan(owner))
			}
		default:
			rest = append(rest, ch)
		}
	}
	return names, values, replaced, rest
}

// conditionalSlotName checks a conditional's branches: each must be one slot
// element of one slot, or null (mixed-conditional-slot).
func (c *passContext) conditionalSlotName(expr *rtsx.Node) (string, bool) {
	name, ok := "", true
	var walk func(n *rtsx.Node)
	walk = func(n *rtsx.Node) {
		n = unwrapParens(n)
		switch {
		case n.Kind == rtsx.KindConditionalExpression:
			walk(n.AsConditionalExpression().WhenTrue)
			walk(n.AsConditionalExpression().WhenFalse)
		case isNullish(n):
		case isSlotElement(n):
			switch tag := rtsx.NodeText(tagOf(n)); name {
			case "":
				name = tag
			case tag:
			default:
				c.errorAt(n, "mixed-conditional-slot", "A conditional slot fills one slot: `%s` and `%s`", name, tag)
				ok = false
			}
		default:
			c.errorAt(n, "mixed-conditional-slot", "A conditional slot's branches are slot elements of one slot, or null")
			ok = false
		}
	}
	walk(expr)
	return name, ok && name != ""
}

func isNullish(n *rtsx.Node) bool {
	return n.Kind == rtsx.KindNullKeyword || n.Kind == rtsx.KindIdentifier && rtsx.NodeText(n) == "undefined"
}

// conditionalValue rebuilds a conditional of slot elements as a conditional
// of slot values: a null branch keeps the previous value (undefined when
// there is none).
func (c *passContext) conditionalValue(expr *rtsx.Node, previous []emit.Piece, origin emit.Span) []emit.Piece {
	expr = unwrapParens(expr)
	switch {
	case expr.Kind == rtsx.KindConditionalExpression:
		ce := expr.AsConditionalExpression()
		out := append(c.operand(ce.Condition, rtsx.PrecedenceConditional), emit.Synth(" ? ", origin))
		out = append(out, c.conditionalValue(ce.WhenTrue, previous, origin)...)
		out = append(out, emit.Synth(" : ", origin))
		return append(out, c.conditionalValue(ce.WhenFalse, previous, origin)...)
	case isNullish(expr) && previous != nil:
		return append(append([]emit.Piece{emit.Synth("(", c.span(expr))}, previous...), emit.Synth(")", c.span(expr)))
	case isNullish(expr):
		return []emit.Piece{emit.Synth("undefined", c.span(expr))}
	}
	return c.slotObject(expr)
}

var identifierName = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func attrName(c *passContext, attr *rtsx.Node) string {
	return strings.TrimSpace(c.text[c.span(attr.Name()).Pos:attr.Name().End()])
}

// attributeProps turns JSX attributes into object properties, in order:
// `a="1"` → `a: "1"`, `a={x}` → `a: x`, bare `a` → `a: true`, `{}` →
// `undefined`, `aria-label` quoted, `{...r}` → `...r`.
func (c *passContext) attributeProps(attrs []*rtsx.Node) [][]emit.Piece {
	var props [][]emit.Piece
	for _, attr := range attrs {
		if attr.Kind == rtsx.KindJsxSpreadAttribute {
			props = append(props, []emit.Piece{emit.Synth("...", c.span(attr)), c.copy(attr.Expression())})
			continue
		}
		name := attrName(c, attr)
		key := name
		if !identifierName.MatchString(name) {
			key = `"` + name + `"`
		}
		prop := []emit.Piece{emit.Synth(key+": ", c.span(attr))}
		switch v := attr.Initializer(); {
		case v == nil:
			prop = append(prop, emit.Synth("true", c.span(attr)))
		case v.Kind == rtsx.KindJsxExpression && v.Expression() == nil:
			prop = append(prop, emit.Synth("undefined", c.span(attr)))
		case v.Kind == rtsx.KindJsxExpression:
			prop = append(prop, c.operand(v.Expression(), rtsx.PrecedenceComma)...)
		default:
			prop = append(prop, c.copy(v))
		}
		props = append(props, prop)
	}
	return props
}

// objectLiteral joins properties into `{ a, b }`, or `{}`.
func objectLiteral(props [][]emit.Piece, origin emit.Span) []emit.Piece {
	if len(props) == 0 {
		return []emit.Piece{emit.Synth("{}", origin)}
	}
	out := []emit.Piece{emit.Synth("{ ", origin)}
	for i, p := range props {
		if i > 0 {
			out = append(out, emit.Synth(", ", origin))
		}
		out = append(out, p...)
	}
	return append(out, emit.Synth(" }", origin))
}

// slotObject: `<$X a="1" {...r} { p }><$Y />body</$X>` →
// `{ a: "1", ...r, $Y: {}, children: (p) => body }` (syntax.md, *Usage and
// desugaring*, *Recursive slots*).
func (c *passContext) slotObject(el *rtsx.Node) []emit.Piece {
	origin := c.openingSpan(el)
	opening := el
	if el.Kind == rtsx.KindJsxElement {
		opening = el.AsJsxElement().OpeningElement
	}
	a := readAttributes(el)
	names, values, replaced, rest := c.assignSlots(el, a)
	body := meaningfulChildren(rest)
	var attrs []*rtsx.Node
	for _, attr := range opening.Attributes().Properties() {
		if _, ok := syntax.SlotParams(attr); ok || replaced[attr] {
			continue
		}
		if attr.Kind == rtsx.KindJsxAttribute {
			switch name := attrName(c, attr); {
			case name == "key":
				c.errorAt(attr, "slot-key", "Slots are not elements: `key` is not allowed")
				continue
			case name == "children" && len(body) > 0:
				c.errorAt(attr, "slot-children-conflict", "`children` is given twice: as an attribute and as the body")
				continue
			}
		}
		attrs = append(attrs, attr)
	}
	props := c.attributeProps(attrs)
	for _, name := range names {
		props = append(props, append([]emit.Piece{emit.Synth(name+": ", origin)}, values[name]...))
	}
	if len(body) > 0 {
		var all []emit.Piece
		for _, ch := range rest {
			all = append(all, c.copyChild(ch))
		}
		prop := []emit.Piece{emit.Synth("children: ", origin)}
		if a.params != nil {
			prop = append(prop, emit.Synth("(", origin), c.copy(a.params), emit.Synth(") => ", origin))
		}
		props = append(props, append(prop, c.bodyOf(body, all, origin)...))
	}
	return objectLiteral(props, origin)
}
