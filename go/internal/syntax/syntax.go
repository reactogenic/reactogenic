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
