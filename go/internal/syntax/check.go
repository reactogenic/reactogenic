package syntax

import (
	"fmt"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// Error is a transpiler error found on the .rtsx tree. Pos and End are byte
// offsets into the source.
type Error struct {
	Pos, End int
	Code     string
	Message  string
}

// Check reports the errors of the .rtsx attribute forms that the parser
// itself accepts (RGP1-022):
//
//   - params-on-html: params on an intrinsic element (`<div { size }>`);
//   - duplicate-params: a second params pattern on one element;
//   - segment-syntax: a `#name` with a value or a namespace;
//   - segment-id: a second `#name` on one element.
//
// Every element is checked, so one bad attribute does not hide the others.
func Check(file *rtsx.SourceFile) []Error {
	var errs []Error
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxOpeningElement || n.Kind == rtsx.KindJsxSelfClosingElement {
			errs = append(errs, checkElement(file, n)...)
		}
		n.ForEachChild(visit)
		return false
	}
	file.AsNode().ForEachChild(visit)
	return errs
}

func checkElement(file *rtsx.SourceFile, element *rtsx.Node) []Error {
	var (
		errs     []Error
		params   int
		segments []string
	)
	report := func(n *rtsx.Node, code, format string, args ...any) {
		errs = append(errs, Error{rtsx.TokenStart(file, n), n.End(), code, fmt.Sprintf(format, args...)})
	}
	tag := element.TagName()
	for _, attr := range element.Attributes().Properties() {
		if _, ok := SlotParams(attr); ok {
			params++
			switch {
			case rtsx.IsIntrinsicTag(tag):
				report(attr, "params-on-html", "Params are only allowed on components and slot elements")
			case params > 1:
				report(attr, "duplicate-params", "An element takes one params pattern")
			}
			continue
		}
		if name, ok := SegmentRoot(attr); ok {
			segments = append(segments, name)
			if len(segments) > 1 {
				report(attr, "segment-id", "An element has one id: `#%s` is already on it", segments[0])
			}
			continue
		}
		if name, ok := malformedSegmentRoot(attr); ok {
			report(attr, "segment-syntax", "`#%s` is a segment root: it takes no value and no namespace", name)
		}
	}
	return errs
}

// malformedSegmentRoot recognises an attribute that starts with `#` but is
// not a plain `#name`: `#name="x"`, `#name:x`.
func malformedSegmentRoot(attr *rtsx.Node) (string, bool) {
	if attr.Kind != rtsx.KindJsxAttribute {
		return "", false
	}
	name := attr.Name()
	if name.Kind == rtsx.KindJsxNamespacedName {
		name = name.AsJsxNamespacedName().Namespace
	}
	text := rtsx.NodeText(name)
	if !strings.HasPrefix(text, "#") || len(text) == 1 {
		return "", false
	}
	return text[1:], true
}
