package syntax

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// SegmentRoots returns the `#name` attributes of file, in order.
func SegmentRoots(file *rtsx.SourceFile) []*rtsx.Node {
	var roots []*rtsx.Node
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if _, ok := SegmentRoot(n); ok {
			roots = append(roots, n)
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return roots
}

// CheckSegments reports the source-level errors of segment roots and
// segment files (syntax.md, *Segment roots*); the file must be bound:
//
//   - segment-id: `#name` with an explicit `id` or a spread on the element;
//   - segment-duplicate: one name mounted twice in the file;
//   - segment-in-loop: a root inside `.map()` or an `Each` body;
//   - segment-children (warning): the root has children, which the segment
//     overwrites.
func CheckSegments(file *rtsx.SourceFile) []Error {
	var errs []Error
	report := func(n *rtsx.Node, warning bool, code, format string, args ...any) {
		errs = append(errs, Error{Pos: rtsx.TokenStart(file, n), End: n.End(), Code: code, Message: fmt.Sprintf(format, args...), Warning: warning})
	}
	seen := map[string]bool{}
	for _, attr := range SegmentRoots(file) {
		name, _ := SegmentRoot(attr)
		element := attr.Parent.Parent // JsxAttributes → opening or self-closing element
		for _, other := range attr.Parent.Properties() {
			switch {
			case other.Kind == rtsx.KindJsxSpreadAttribute:
				if _, isParams := SlotParams(other); !isParams {
					report(other, false, "segment-id", "A spread could carry an `id`; `#%s` already sets it", name)
				}
			case other.Kind == rtsx.KindJsxAttribute && other.Name().Kind == rtsx.KindIdentifier && rtsx.NodeText(other.Name()) == "id":
				report(other, false, "segment-id", "An element has one id: `#%s` already sets it", name)
			}
		}
		if seen[name] {
			report(attr, false, "segment-duplicate", "`#%s` is already mounted", name)
		}
		seen[name] = true
		if inLoop(attr) {
			report(attr, false, "segment-in-loop", "`#%s` would be mounted more than once", name)
		}
		if element.Kind == rtsx.KindJsxOpeningElement && hasContent(element.Parent) {
			report(attr, true, "segment-children", "Contents will be overwritten by the segment `%s`", name)
		}
	}
	return errs
}

func hasContent(el *rtsx.Node) bool {
	for _, ch := range el.Children().Nodes {
		if !BlankText(ch) && !(ch.Kind == rtsx.KindJsxExpression && ch.Expression() == nil) {
			return true
		}
	}
	return false
}

// inLoop: n is inside a callback passed to `.map()` / `.flatMap()`, or in the
// body of an `Each`.
func inLoop(n *rtsx.Node) bool {
	for child, p := n, n.Parent; p != nil; child, p = p, p.Parent {
		switch p.Kind {
		case rtsx.KindCallExpression:
			callee := p.Expression()
			if callee.Kind == rtsx.KindPropertyAccessExpression && child != callee {
				if m := rtsx.NodeText(callee.Name()); m == "map" || m == "flatMap" {
					return true
				}
			}
		case rtsx.KindJsxElement:
			if child != p.AsJsxElement().OpeningElement && FrameworkExport(p.AsJsxElement().OpeningElement.TagName()) == "Each" {
				return true
			}
		}
	}
	return false
}
