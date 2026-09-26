package syntax

import (
	"fmt"
	"path"
	"strings"

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
//     overwrites;
//   - segment-import: a value import of a `+` file.
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
			report(attr, true, "segment-children", "Contents will be overwritten by `+%s.rtsx`", name)
		}
	}
	for _, stmt := range file.Statements.Nodes {
		if stmt.Kind != rtsx.KindImportDeclaration || !importsValues(stmt) {
			continue
		}
		spec := stmt.AsImportDeclaration().ModuleSpecifier
		if base := path.Base(rtsx.NodeText(spec)); strings.HasPrefix(base, "+") && strings.HasPrefix(rtsx.NodeText(spec), ".") {
			report(stmt, false, "segment-import", "Segments are mounted with `#%s`, not imported", strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(base, "+"), ".rtsx"), ".tsx"))
		}
	}
	return errs
}

// importsValues: the import brings in values or runs the module — anything
// but `import type …` or named imports that are all `type`.
func importsValues(decl *rtsx.Node) bool {
	clause := decl.AsImportDeclaration().ImportClause
	if clause == nil {
		return true // import "./+x" runs it
	}
	if clause.IsTypeOnly() {
		return false
	}
	c := clause.AsImportClause()
	if c.Name() != nil || c.NamedBindings == nil || c.NamedBindings.Kind == rtsx.KindNamespaceImport {
		return true
	}
	for _, spec := range c.NamedBindings.Elements() {
		if !spec.IsTypeOnly() {
			return true
		}
	}
	return false
}

func hasContent(el *rtsx.Node) bool {
	for _, ch := range el.Children().Nodes {
		if !(ch.Kind == rtsx.KindJsxText && ch.AsJsxText().ContainsOnlyTriviaWhiteSpaces) && !(ch.Kind == rtsx.KindJsxExpression && ch.Expression() == nil) {
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

// CheckEachKeys reports each-no-key: the body of an `Each` written with
// params must be a single element that carries `key` (syntax.md,
// *Iteration*). A hand-written render prop is plain React and is left
// alone. The file must be bound.
func CheckEachKeys(file *rtsx.SourceFile) []Error {
	var errs []Error
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxElement && FrameworkExport(n.AsJsxElement().OpeningElement.TagName()) == "Each" && hasParams(n.AsJsxElement().OpeningElement) {
			var body []*rtsx.Node
			for _, ch := range n.Children().Nodes {
				if !(ch.Kind == rtsx.KindJsxText && ch.AsJsxText().ContainsOnlyTriviaWhiteSpaces) && !(ch.Kind == rtsx.KindJsxExpression && ch.Expression() == nil) {
					body = append(body, ch)
				}
			}
			at := n.AsJsxElement().OpeningElement
			if len(body) == 1 && (body[0].Kind == rtsx.KindJsxElement || body[0].Kind == rtsx.KindJsxSelfClosingElement) {
				if hasKey(body[0]) {
					return n.ForEachChild(visit)
				}
				at = body[0]
			}
			errs = append(errs, Error{Pos: rtsx.TokenStart(file, at), End: at.End(), Code: "each-no-key",
				Message: "Each iteration needs a `key` on the root element of the body"})
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return errs
}

func hasParams(opening *rtsx.Node) bool {
	for _, attr := range opening.Attributes().Properties() {
		if _, ok := SlotParams(attr); ok {
			return true
		}
	}
	return false
}

func hasKey(el *rtsx.Node) bool {
	opening := el
	if el.Kind == rtsx.KindJsxElement {
		opening = el.AsJsxElement().OpeningElement
	}
	for _, attr := range opening.Attributes().Properties() {
		if attr.Kind == rtsx.KindJsxAttribute && attr.Name().Kind == rtsx.KindIdentifier && rtsx.NodeText(attr.Name()) == "key" {
			return true
		}
	}
	return false
}
