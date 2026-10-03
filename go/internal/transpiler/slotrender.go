package transpiler

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// renderSlots lowers the container side of slots (syntax.md, *Slots →
// Attachment*):
//
//	<span slot={$Badge} className="b" &size &&value={v}>new</span>
//	→ {$Badge
//	    ? <span className="b" value={v} {...$Badge}>{_renderSlot($Badge, { size: size, value: v }, "new")}</span>
//	    : <span className="b" value={v}>new</span>}
//
// The attachment's props are defaults the slot's props replace; its children
// are the fallback. Attachments are lowered innermost first, so an attachment
// inside another one's fallback is lowered before it.
func (c *passContext) renderSlots() []emit.Edit {
	var edits []emit.Edit
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if ref, attr := attachment(n); ref != nil && !hasAttachmentBelow(n) {
			edits = append(edits, c.renderSlot(n, ref, attr)...)
			return false
		}
		return n.ForEachChild(visit)
	}
	c.file.AsNode().ForEachChild(visit)
	return edits
}

// attachment returns the `$` reference and the `slot` attribute of an
// element that attaches a slot.
func attachment(el *rtsx.Node) (*rtsx.Node, *rtsx.Node) {
	switch el.Kind {
	case rtsx.KindJsxElement:
		return syntax.SlotAttachment(el.AsJsxElement().OpeningElement)
	case rtsx.KindJsxSelfClosingElement:
		return syntax.SlotAttachment(el)
	}
	return nil, nil
}

func hasAttachmentBelow(el *rtsx.Node) bool {
	found := false
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if ref, _ := attachment(n); ref != nil {
			found = true
			return true
		}
		return n.ForEachChild(visit)
	}
	el.ForEachChild(visit)
	return found
}

func (c *passContext) renderSlot(el, ref, slotAttr *rtsx.Node) []emit.Edit {
	origin := c.openingSpan(el)
	opening := el
	if el.Kind == rtsx.KindJsxElement {
		opening = el.AsJsxElement().OpeningElement
	}
	var props [][]emit.Piece // the element's own props, in order
	var args [][]emit.Piece  // the body's args
	var key *rtsx.Node
	for _, attr := range opening.Attributes().Properties() {
		if attr == slotAttr {
			continue
		}
		if attr.Kind == rtsx.KindJsxAttribute && attr.Name().Kind == rtsx.KindIdentifier && rtsx.NodeText(attr.Name()) == "key" {
			key = attr
			continue
		}
		name, kind := syntax.SlotArg(attr)
		if kind == syntax.NotArg {
			props = append(props, []emit.Piece{c.copy(attr)})
			continue
		}
		argKey := name
		if !identifierName.MatchString(name) {
			argKey = `"` + name + `"`
		}
		c.note(c.span(attr), "slot-arg", c.sourceText(ref), name)
		// The arg key is the name the author wrote. A bare `&size` is a
		// shorthand: the key and the value are two copies of one token.
		key := []emit.Piece{emit.Synth(argKey+": ", c.span(attr))}
		bare := attr.Initializer() == nil
		if argKey == name && name != "" {
			keyCopy := c.argNameCopy(attr, name).Lacking(emit.FeatureSemanticTokens)
			if bare {
				keyCopy = keyCopy.Lacking(nameCopy)
			}
			key = []emit.Piece{keyCopy, emit.Synth(": ", c.span(attr))}
		}
		val := c.argValue(attr, name)
		if bare {
			site := "arg"
			if kind == syntax.ArgProp {
				site = "arg-prop"
			}
			c.shorthandSite(c.argNameCopy(attr, name).From, site)
			val = []emit.Piece{c.argNameCopy(attr, name).Lacking(valueCopy)}
		}
		args = append(args, append(key, val...))
		if kind == syntax.ArgProp {
			// The element's prop is one more copy; the arg answers.
			prop := []emit.Piece{emit.Synth(name+"=", c.span(attr))}
			if init := attr.Initializer(); init != nil {
				prop = append(prop, c.copy(init).Lacking(emit.AllFeatures))
			} else { // `&&selected` → selected={selected}
				prop = append(prop, emit.Synth("{", c.span(attr)), c.argNameCopy(attr, name).Lacking(emit.AllFeatures), emit.Synth("}", c.span(attr)))
			}
			props = append(props, prop)
		}
	}
	fallback := meaningfulChildren(childrenOf(el))
	c.note(origin, "slot-args", c.sourceText(ref), "")
	if len(fallback) > 0 {
		c.note(origin, "attachment", c.sourceText(ref), "fallback")
	} else {
		c.note(origin, "attachment", c.sourceText(ref), "")
	}

	render := c.core("renderSlot", origin)
	assigned := c.core("isAssigned", origin)
	spread := c.core("slotProps", origin)

	// With `key`, the attachment renders one entry: of a keyed slot, the
	// entry of that key; of a singular slot, the slot itself (syntax.md,
	// *Keyed slots*). The lookup runs once, through an inline function.
	slot := func() emit.Piece { return c.copy(ref) }
	var entry string
	if key != nil && value(key) != nil {
		entry = c.fresh("_entry")
		slot = func() emit.Piece { return emit.Synth(entry, c.span(ref)) }
	}
	// With args, the slot may carry a key function of them (syntax.md, *Key
	// functions*): the args are built once, for the key and for the body.
	argsValue := func() []emit.Piece { return objectLiteral(args, origin) }
	var argsName string
	if len(args) > 0 {
		argsName = c.fresh("_args")
		argsValue = func() []emit.Piece { return []emit.Piece{emit.Synth(argsName, origin)} }
	}

	tag := emit.Span{Pos: c.span(opening.TagName()).Pos, End: opening.Attributes().Pos()}
	tag.End = tag.Pos + len(strings.TrimRight(c.text[tag.Pos:tag.End], " \t\r\n"))
	element := func(withSlot bool) []emit.Piece {
		out := []emit.Piece{emit.Synth("<", origin), emit.Copy(c.text, tag)}
		switch {
		case withSlot && argsName != "":
			out = append(out, emit.Synth(" key={"+c.core("slotKey", origin)+"(", origin), slot(), emit.Synth(", "+argsName, origin))
			if key != nil && value(key) != nil {
				out = append(append(out, emit.Synth(", ", origin)), c.operand(value(key), rtsx.PrecedenceComma)...)
			}
			out = append(out, emit.Synth(")}", origin))
		case key != nil:
			out = append(out, emit.Synth(" ", origin), c.copy(key))
		}
		for _, p := range props {
			out = append(append(out, emit.Synth(" ", origin)), p...)
		}
		if withSlot {
			out = append(out, emit.Synth(" {..."+spread+"(", origin), slot(), emit.Synth(")}>{"+render+"(", origin), slot(), emit.Synth(", ", origin))
			out = append(out, argsValue()...)
			if len(fallback) > 0 {
				var all []emit.Piece
				for _, ch := range childrenOf(el) {
					all = append(all, c.copyChild(ch))
				}
				out = append(append(out, emit.Synth(", ", origin)), c.bodyOf(fallback, all, origin)...)
			}
			out = append(out, emit.Synth(")}", origin))
		} else {
			out = append(out, emit.Synth(">", origin), emit.Copy(c.text, c.childrenSpan(el)))
		}
		return append(out, emit.Synth("</", origin), c.closingName(el, opening), emit.Synth(">", origin))
	}
	expr := append([]emit.Piece{emit.Synth(assigned+"(", origin), slot(), emit.Synth(") ? ", origin)}, element(true)...)
	expr = append(expr, emit.Synth(" : ", origin))
	if len(fallback) > 0 {
		expr = append(expr, element(false)...)
	} else {
		expr = append(expr, emit.Synth("null", origin))
	}
	// The entry and the args are bound once, by an inline function.
	var params []string
	var values [][]emit.Piece
	if entry != "" {
		lookup := append([]emit.Piece{emit.Synth(c.core("slotEntry", origin)+"(", origin), c.copy(ref), emit.Synth(", ", origin)}, c.operand(value(key), rtsx.PrecedenceComma)...)
		params, values = append(params, entry), append(values, append(lookup, emit.Synth(")", origin)))
	}
	if argsName != "" {
		// slotArgs types the args as renderSlot does, at the object literal,
		// so an error points at the arg.
		typed := []emit.Piece{emit.Synth(c.core("slotArgs", origin)+"(", origin), c.copy(ref), emit.Synth(", ", origin)}
		typed = append(append(typed, objectLiteral(args, origin)...), emit.Synth(")", origin))
		params, values = append(params, argsName), append(values, typed)
	}
	if len(params) > 0 {
		wrapped := append([]emit.Piece{emit.Synth("(("+strings.Join(params, ", ")+") => ", origin)}, expr...)
		wrapped = append(wrapped, emit.Synth(")(", origin))
		for i, v := range values {
			if i > 0 {
				wrapped = append(wrapped, emit.Synth(", ", origin))
			}
			wrapped = append(wrapped, v...)
		}
		expr = append(wrapped, emit.Synth(")", origin))
	}
	return c.replace(el, origin, expr)
}

func childrenOf(el *rtsx.Node) []*rtsx.Node {
	if el.Kind != rtsx.KindJsxElement {
		return nil
	}
	return el.Children().Nodes
}

// argValue is an arg's value: its expression, or — `&size` — the binding of
// its name, always (an arg is a function argument, never `true`).
func (c *passContext) argValue(attr *rtsx.Node, name string) []emit.Piece {
	switch init := attr.Initializer(); {
	case init == nil:
		return []emit.Piece{c.argNameCopy(attr, name)}
	case init.Kind == rtsx.KindJsxExpression && init.Expression() == nil:
		return []emit.Piece{emit.Synth("undefined", c.span(attr))}
	case init.Kind == rtsx.KindJsxExpression:
		return c.operand(init.Expression(), rtsx.PrecedenceComma)
	default:
		return []emit.Piece{c.copy(init)}
	}
}

// argNameCopy copies the name of `&size` / `&&size` without its prefix, so
// an error on it points at the name the author wrote.
func (c *passContext) argNameCopy(attr *rtsx.Node, name string) emit.Piece {
	end := attr.Name().End()
	return emit.Copy(c.text, emit.Span{Pos: end - len(name), End: end})
}
