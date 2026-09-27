// Package syntax names the .rtsx forms in tsgo's AST. The parser patches
// (go/patches/) store them in existing node kinds; this package is the one
// place that knows how.
package syntax

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// SlotParams returns the ObjectBindingPattern of slot params (`{ size }`)
// when attr is one. The parser stores params as a JsxSpreadAttribute whose
// expression is that pattern.
func SlotParams(attr *rtsx.Node) (*rtsx.Node, bool) {
	if attr.Kind != rtsx.KindJsxSpreadAttribute {
		return nil, false
	}
	if e := attr.Expression(); e != nil && e.Kind == rtsx.KindObjectBindingPattern {
		return e, true
	}
	return nil, false
}

// SegmentRoot returns the segment name of a `#name` attribute (`about-us`
// for `#about-us`). TypeScript's parser already reads it as a JsxAttribute
// named "#about-us" with no value.
func SegmentRoot(attr *rtsx.Node) (string, bool) {
	if attr.Kind != rtsx.KindJsxAttribute || attr.Initializer() != nil {
		return "", false
	}
	name := attr.Name()
	if name == nil || name.Kind != rtsx.KindIdentifier {
		return "", false
	}
	text := rtsx.NodeText(name)
	if !strings.HasPrefix(text, "#") || len(text) == 1 {
		return "", false
	}
	return text[1:], true
}

// ArgKind tells a slot arg on an attachment from a plain attribute.
type ArgKind int

const (
	NotArg  ArgKind = iota
	Arg             // `&name`: an arg of the slot's body only
	ArgProp         // `&&name`: an arg and a prop of the element
)

// SlotArg returns the name and kind of an `&name` / `&&name` attribute. The
// parser stores it as a JsxAttribute whose name is spelled with its prefix.
func SlotArg(attr *rtsx.Node) (string, ArgKind) {
	if attr.Kind != rtsx.KindJsxAttribute || attr.Name().Kind != rtsx.KindIdentifier {
		return "", NotArg
	}
	switch text := rtsx.NodeText(attr.Name()); {
	case strings.HasPrefix(text, "&&"):
		return text[2:], ArgProp
	case strings.HasPrefix(text, "&"):
		return text[1:], Arg
	}
	return "", NotArg
}

// SlotAttachment returns the `$` reference of an element's `slot={$X}`
// (syntax.md, *Slots → Attachment*): an identifier starting with `$`, or a
// property-access chain ending in one. Any other `slot` keeps its HTML
// meaning. opening is a JSX opening or self-closing element.
func SlotAttachment(opening *rtsx.Node) (ref, attr *rtsx.Node) {
	for _, a := range opening.Attributes().Properties() {
		if a.Kind != rtsx.KindJsxAttribute || a.Name().Kind != rtsx.KindIdentifier || rtsx.NodeText(a.Name()) != "slot" {
			continue
		}
		init := a.Initializer()
		if init == nil || init.Kind != rtsx.KindJsxExpression || init.Expression() == nil {
			continue
		}
		if e := init.Expression(); isSlotReference(e) {
			return e, a
		}
	}
	return nil, nil
}

func isSlotReference(n *rtsx.Node) bool {
	switch n.Kind {
	case rtsx.KindIdentifier:
		return strings.HasPrefix(rtsx.NodeText(n), "$")
	case rtsx.KindPropertyAccessExpression:
		return strings.HasPrefix(rtsx.NodeText(n.Name()), "$") && isPlainReference(n.Expression())
	}
	return false
}

func isPlainReference(n *rtsx.Node) bool {
	switch n.Kind {
	case rtsx.KindIdentifier, rtsx.KindThisKeyword:
		return true
	case rtsx.KindPropertyAccessExpression:
		return n.AsPropertyAccessExpression().QuestionDotToken == nil && isPlainReference(n.Expression())
	}
	return false
}
