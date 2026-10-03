package lsp

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// sourceTree is an .rtsx text as its author wrote it: what TypeScript cannot be
// asked, because the virtual text has no token for it (ide.md, *Span map*) —
// which tags are one slot, which two names are one element's, which bare
// names stand for `name={name}`. The front answers requests from it, and
// rename writes its edits by it.
type sourceTree struct {
	text string
	file *rtsx.SourceFile
	// out is the transform's output for text; out.Map is nil when the text
	// stands in as its own virtual text.
	out transpiler.Output

	tags  []tagName
	attrs []attribute
}

// tagName is the name of a JSX tag.
type tagName struct {
	span    emit.Span
	element *rtsx.Node // the JsxElement, or the self-closing element
	closing bool
	// partner is the index of the element's other tag name, -1 when it has
	// none — a self-closing element, or a closing tag that names another.
	partner int
}

// attribute is a JSX attribute that the passes give a meaning of their own.
type attribute struct {
	node *rtsx.Node
	// name is the name without its sigil: `size` of `&&size`.
	name emit.Span
	arg  syntax.ArgKind
	bare bool // no value: `size`, `&size`
}

// analyse reads a source text. out is the transform's output for it.
func analyse(text string, out transpiler.Output) *sourceTree {
	s := &sourceTree{text: text, file: rtsx.ParseRTSX("/document.rtsx", text), out: out}
	rtsx.Bind(s.file)
	span := func(n *rtsx.Node) emit.Span { return emit.Span{Pos: rtsx.TokenStart(s.file, n), End: n.End()} }
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		switch n.Kind {
		case rtsx.KindJsxSelfClosingElement:
			s.tags = append(s.tags, tagName{span: span(n.TagName()), element: n, partner: -1})
		case rtsx.KindJsxElement:
			el := n.AsJsxElement()
			open := tagName{span: span(el.OpeningElement.TagName()), element: n, partner: -1}
			at := len(s.tags)
			s.tags = append(s.tags, open)
			if closing := el.ClosingElement; closing != nil && closing.TagName() != nil {
				// A closing tag that recovery made up has no name; one that
				// names another element is not this one's.
				if name := span(closing.TagName()); name.Len() > 0 && s.slice(name) == s.slice(open.span) {
					s.tags[at].partner = at + 1
					s.tags = append(s.tags, tagName{span: name, element: n, closing: true, partner: at})
				}
			}
		case rtsx.KindJsxAttribute:
			if name := n.Name(); name != nil && name.Kind == rtsx.KindIdentifier {
				written, kind := syntax.SlotArg(n)
				if kind == syntax.NotArg {
					written = rtsx.NodeText(name)
				}
				if _, segment := syntax.SegmentRoot(n); !segment && written != "" {
					s.attrs = append(s.attrs, attribute{node: n, name: emit.Span{Pos: name.End() - len(written), End: name.End()}, arg: kind, bare: n.Initializer() == nil})
				}
			}
		}
		return n.ForEachChild(visit)
	}
	s.file.AsNode().ForEachChild(visit)
	// The closing name of an element is found after its children: in source
	// order the tags are not sorted by position, and nothing needs them to be.
	return s
}

func (s *sourceTree) slice(at emit.Span) string { return s.text[at.Pos:at.End] }

// within reports whether inner lies in outer.
func within(inner, outer emit.Span) bool { return outer.Pos <= inner.Pos && inner.End <= outer.End }

// tagAt is the index of the tag name that holds at, -1 when none does. A
// position at the end of a name is in it: where the cursor is after typing.
func (s *sourceTree) tagAt(at emit.Span) int {
	for i, t := range s.tags {
		if within(at, t.span) {
			return i
		}
	}
	return -1
}

// isSlotTag: the tag of a slot element, `$Icon`.
func (s *sourceTree) isSlotTag(t tagName) bool { return strings.HasPrefix(s.slice(t.span), "$") }

// group is the slot group that holds the tag name span, and the tag's index
// in it; nil for a tag that is not a slot's (or a slot that no pass
// gathered: an orphan, a `$Case`).
func (s *sourceTree) group(tag emit.Span) (*transpiler.SlotGroup, int) {
	for g := range s.out.SlotGroups {
		for i, t := range s.out.SlotGroups[g].Tags {
			if t == tag {
				return &s.out.SlotGroups[g], i
			}
		}
	}
	return nil, -1
}

// twins are the other source spans that are the same name as at, which lies
// in a tag name: the same offsets in every other tag of its slot group, or —
// for any other element — in its other tag (`<UI.Button>` … `</UI.Button>`).
// TypeScript sees one of them at most: one prop for a slot however many
// elements fill it, and no closing tag for an element emitted self-closing.
func (s *sourceTree) twins(at emit.Span) []emit.Span {
	i := s.tagAt(at)
	if i < 0 {
		return nil
	}
	tag := s.tags[i]
	shift := func(to emit.Span) emit.Span {
		return emit.Span{Pos: to.Pos + at.Pos - tag.span.Pos, End: to.Pos + at.End - tag.span.Pos}
	}
	var out []emit.Span
	if group, _ := s.group(tag.span); group != nil {
		for _, other := range group.Tags {
			if other != tag.span {
				out = append(out, shift(other))
			}
		}
		return out
	}
	if tag.partner >= 0 {
		out = append(out, shift(s.tags[tag.partner].span))
	}
	return out
}

// answers reports whether the source position pos has a copy in the virtual
// text that answers feature. A text that is its own virtual text answers
// everywhere.
func (s *sourceTree) answers(pos int, feature emit.Features) bool {
	if s.out.Map == nil {
		return true
	}
	for _, segment := range s.out.Map.Segments {
		if segment.Copied && segment.In.Pos <= pos && pos < segment.In.End && segment.Without&feature == 0 {
			return true
		}
	}
	return false
}

// copied reports whether the source span at is in the virtual text at all,
// with or without features.
func (s *sourceTree) copied(at emit.Span) bool {
	if s.out.Map == nil {
		return true
	}
	for _, segment := range s.out.Map.Segments {
		if segment.Copied && within(at, segment.In) {
			return true
		}
	}
	return false
}

// retarget is where a request at the source offset pos is answered instead
// (ide.md, *Span map*, slot groups): at the copied tag of its slot group,
// or — the closing tag of an element that is emitted without one — at the
// opening tag. from is the name the request is on, to the name that answers.
func (s *sourceTree) retarget(pos int) (from, to emit.Span, ok bool) {
	i := s.tagAt(emit.Span{Pos: pos, End: pos})
	if i < 0 {
		return from, to, false
	}
	tag := s.tags[i]
	if group, n := s.group(tag.span); group != nil {
		return tag.span, group.Tags[0], n > 0
	}
	if tag.closing && !s.answers(tag.span.Pos, emit.FeatureHover) {
		to = s.tags[tag.partner].span
		if group, _ := s.group(to); group != nil {
			to = group.Tags[0]
		}
		return tag.span, to, true
	}
	return from, to, false
}

// owner is the element whose child the element of tag is: the nearest JSX
// element above it. nil at the root of a JSX tree.
func owner(element *rtsx.Node) *rtsx.Node {
	for n := element.Parent; n != nil; n = n.Parent {
		if n.Kind == rtsx.KindJsxElement {
			return n
		}
		if n.Kind != rtsx.KindJsxExpression && n.Kind != rtsx.KindConditionalExpression && n.Kind != rtsx.KindBinaryExpression &&
			n.Kind != rtsx.KindParenthesizedExpression && n.Kind != rtsx.KindJsxFragment {
			return nil
		}
	}
	return nil
}

// segmentAt is the `#name` — its span, sigil included, and the name — that
// holds the source offset pos. A `#` with no name yet is one, with "".
func (s *sourceTree) segmentAt(pos int) (at emit.Span, name string, ok bool) {
	var found *rtsx.Node
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxAttribute && n.Initializer() == nil && n.Name() != nil && n.Name().Kind == rtsx.KindIdentifier {
			if start := rtsx.TokenStart(s.file, n.Name()); start <= pos && pos <= n.Name().End() && strings.HasPrefix(s.text[start:], "#") {
				found = n.Name()
				return true
			}
		}
		return n.ForEachChild(visit)
	}
	s.file.AsNode().ForEachChild(visit)
	if found == nil {
		return at, "", false
	}
	at = emit.Span{Pos: rtsx.TokenStart(s.file, found), End: found.End()}
	return at, s.slice(at)[1:], true
}

// mounted are the segment names the file mounts.
func (s *sourceTree) mounted() map[string]bool {
	names := map[string]bool{}
	for _, root := range syntax.SegmentRoots(s.file) {
		name, _ := syntax.SegmentRoot(root)
		names[name] = true
	}
	return names
}
