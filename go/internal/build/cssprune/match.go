package cssprune

import (
	"strings"

	"golang.org/x/net/html"
)

// tri is the answer to "does this element match?" when part of a selector is
// runtime state: no, maybe, yes. `and` is min, `or` is max, `not` mirrors —
// so the negation of "maybe" is "maybe" (builder.md, CSS).
type tri uint8

const (
	no tri = iota
	maybe
	yes
)

// dynamic reports whether an attribute is runtime state: a selector on it
// never decides anything (builder.md, CSS; components.md, CSS convention).
func dynamic(name string) bool {
	switch name {
	case "open", "hidden", "inert", "disabled", "checked", "selected", "value", "style", "data-state":
		return true
	}
	return strings.HasPrefix(name, "aria-")
}

// The attributes whose values HTML compares case-insensitively in selectors
// (HTML, "Case-sensitivity of selectors"): `[type="CHECKBOX"]` matches
// type="checkbox".
var caseInsensitiveValue = set(
	"accept", "accept-charset", "align", "alink", "axis", "bgcolor", "charset", "checked", "clear", "codetype",
	"color", "compact", "declare", "defer", "dir", "direction", "disabled", "enctype", "face", "frame", "hreflang",
	"http-equiv", "lang", "language", "link", "media", "method", "multiple", "nohref", "noresize", "noshade",
	"nowrap", "readonly", "rel", "rev", "rules", "scope", "scrolling", "selected", "shape", "target", "text",
	"type", "valign", "valuetype", "vlink",
)

type memoKey struct {
	s   *selector
	i   int
	el  *html.Node
	fwd bool
}

// matcher matches selectors against one page.
type matcher struct {
	root *html.Node   // the document element: what :root is
	els  []*html.Node // every element, in document order
	memo map[memoKey]tri
}

// may reports whether some element of the page may match s.
func (m *matcher) may(s *selector) bool {
	clear(m.memo)
	for _, el := range m.els {
		if m.upTo(el, s, len(s.parts)-1) != no {
			return true
		}
	}
	return false
}

func parent(el *html.Node) *html.Node {
	if p := el.Parent; p != nil && p.Type == html.ElementNode {
		return p
	}
	return nil
}

func prev(el *html.Node) *html.Node {
	for s := el.PrevSibling; s != nil; s = s.PrevSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}

func next(el *html.Node) *html.Node {
	for s := el.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}

// upTo: el matches parts[i], and parts[:i] match what stands before it.
func (m *matcher) upTo(el *html.Node, s *selector, i int) tri {
	r := m.compound(el, &s.parts[i])
	if r == no {
		return no
	}
	if i == 0 {
		if s.comb[0] != 0 { // relative to a scope root, which is not resolved
			return maybe
		}
		return r
	}
	k := memoKey{s: s, i: i, el: el}
	if v, ok := m.memo[k]; ok {
		return v
	}
	rel := no
	switch s.comb[i] {
	case ' ':
		for a := parent(el); a != nil && rel != yes; a = parent(a) {
			rel = max(rel, m.upTo(a, s, i-1))
		}
	case '>':
		if a := parent(el); a != nil {
			rel = m.upTo(a, s, i-1)
		}
	case '+':
		if a := prev(el); a != nil {
			rel = m.upTo(a, s, i-1)
		}
	case '~':
		for a := prev(el); a != nil && rel != yes; a = prev(a) {
			rel = max(rel, m.upTo(a, s, i-1))
		}
	}
	r = min(r, rel)
	m.memo[k] = r
	return r
}

// has: s, relative to el, matches something — `:has(> .a .b)`.
func (m *matcher) has(el *html.Node, s *selector) tri {
	return m.related(el, s.comb[0], func(x *html.Node) tri { return m.down(x, s, 0) })
}

// down: x matches parts[i], and parts[i+1:] match what stands after it.
func (m *matcher) down(x *html.Node, s *selector, i int) tri {
	r := m.compound(x, &s.parts[i])
	if r == no || i == len(s.parts)-1 {
		return r
	}
	k := memoKey{s: s, i: i, el: x, fwd: true}
	if v, ok := m.memo[k]; ok {
		return v
	}
	r = min(r, m.related(x, s.comb[i+1], func(y *html.Node) tri { return m.down(y, s, i+1) }))
	m.memo[k] = r
	return r
}

// related is the best answer of f over the elements a combinator leads to
// from el: its descendants, children, next sibling, following siblings.
func (m *matcher) related(el *html.Node, comb byte, f func(*html.Node) tri) tri {
	r := no
	switch comb {
	case '>':
		for c := el.FirstChild; c != nil && r != yes; c = c.NextSibling {
			if c.Type == html.ElementNode {
				r = max(r, f(c))
			}
		}
	case '+':
		if n := next(el); n != nil {
			r = f(n)
		}
	case '~':
		for n := next(el); n != nil && r != yes; n = next(n) {
			r = max(r, f(n))
		}
	default: // a descendant
		var walk func(n *html.Node)
		walk = func(n *html.Node) {
			for c := n.FirstChild; c != nil && r != yes; c = c.NextSibling {
				if c.Type == html.ElementNode {
					if r = max(r, f(c)); r != yes {
						walk(c)
					}
				}
			}
		}
		walk(el)
	}
	return r
}

func (m *matcher) compound(el *html.Node, c *compound) tri {
	// Tag names compare case-insensitively: exact for HTML elements, and a
	// superset of what matches for SVG's camel-cased ones.
	if c.tag != "" && c.tag != "*" && !strings.EqualFold(c.tag, el.Data) {
		return no
	}
	r := yes
	for i := range c.simples {
		if r = min(r, m.simple(el, &c.simples[i])); r == no {
			return no
		}
	}
	return r
}

func (m *matcher) simple(el *html.Node, s *simple) tri {
	switch s.kind {
	case sClass:
		return is(hasClass(el, s.name))
	case sID:
		v, ok := attribute(el, "id")
		return is(ok && v == s.name)
	case sAttr:
		if dynamic(s.name) {
			return maybe
		}
		v, ok := attribute(el, s.name)
		return is(ok && attributeMatches(s, v))
	case sRoot:
		return is(el == m.root)
	case sIs:
		r := no
		for _, a := range s.args {
			v := m.upTo(el, a, len(a.parts)-1)
			if !a.known { // a browser that cannot parse the argument ignores it
				v = min(v, maybe)
			}
			if r = max(r, v); r == yes {
				break
			}
		}
		return r
	case sNot:
		r := no
		for _, a := range s.args {
			if r = max(r, m.upTo(el, a, len(a.parts)-1)); r == yes {
				break
			}
		}
		return yes - r
	case sHas:
		r := no
		for _, a := range s.args {
			if r = max(r, m.has(el, a)); r == yes {
				break
			}
		}
		return r
	}
	return maybe
}

func is(b bool) tri {
	if b {
		return yes
	}
	return no
}

// attribute finds an attribute by name. HTML's parser lower-cased the names
// of HTML elements and camel-cased SVG's (viewBox), so the comparison folds.
func attribute(el *html.Node, name string) (string, bool) {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Key, name) || a.Namespace != "" && strings.EqualFold(a.Namespace+":"+a.Key, name) {
			return a.Val, true
		}
	}
	return "", false
}

// hasClass: classes are separated by HTML's whitespace, not Unicode's.
func hasClass(el *html.Node, class string) bool {
	v, _ := attribute(el, "class")
	for v != "" {
		i := 0
		for i < len(v) && !isSpace(v[i]) {
			i++
		}
		if v[:i] == class && i > 0 {
			return true
		}
		for i < len(v) && isSpace(v[i]) {
			i++
		}
		v = v[i:]
	}
	return false
}

func attributeMatches(s *simple, v string) bool {
	want := s.value
	if s.flag == 'i' || s.flag == 0 && caseInsensitiveValue[s.name] {
		v, want = strings.ToLower(v), strings.ToLower(want)
	}
	switch s.op {
	case '=':
		return v == want
	case '~':
		if want == "" || strings.ContainsAny(want, " \t\n\r\f") {
			return false
		}
		for v != "" {
			i := 0
			for i < len(v) && !isSpace(v[i]) {
				i++
			}
			if v[:i] == want {
				return true
			}
			for i < len(v) && isSpace(v[i]) {
				i++
			}
			v = v[i:]
		}
		return false
	case '|':
		return v == want || strings.HasPrefix(v, want+"-")
	case '^':
		return want != "" && strings.HasPrefix(v, want)
	case '$':
		return want != "" && strings.HasSuffix(v, want)
	case '*':
		return want != "" && strings.Contains(v, want)
	}
	return true // [name]
}
