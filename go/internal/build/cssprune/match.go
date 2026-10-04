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

// What the memo holds for an element and a position in a selector.
type memoKind uint8

const (
	mUp     memoKind = iota // upTo
	mDown                   // down
	mAbove                  // the best upTo of its ancestors
	mBefore                 // … of the siblings before it
	mAfter                  // the best down of the siblings after it
	mBelow                  // … of its descendants
)

type memoKey struct {
	s    *selector
	i    int
	el   *html.Node
	kind memoKind
}

// matcher matches selectors against one page.
type matcher struct {
	root *html.Node   // the document element: what :root is
	els  []*html.Node // every element, in document order
	// quirks: the page may be in quirks mode, where classes and ids match
	// whatever their case.
	quirks bool
	// script: what the page's script names (PruneWith); nil without one.
	script *names
	memo   map[memoKey]tri
	work   int // compounds tried, for the tests: matching must stay linear in the page
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
		rel = m.chain(el, s, i-1, mAbove)
	case '>':
		if a := parent(el); a != nil {
			rel = m.upTo(a, s, i-1)
		}
	case '+':
		if a := prev(el); a != nil {
			rel = m.upTo(a, s, i-1)
		}
	case '~':
		rel = m.chain(el, s, i-1, mBefore)
	}
	r = min(r, rel)
	m.memo[k] = r
	return r
}

// chain is the best answer over the elements one step leads to from el, again
// and again: its ancestors (mAbove), the siblings before it (mBefore) or after
// it (mAfter). The answer is kept per element, so a row of n siblings costs n
// and not n²: `.a ~ li ~ li` on a list of 20 000 took seconds without it.
func (m *matcher) chain(el *html.Node, s *selector, i int, kind memoKind) tri {
	step, f := parent, m.upTo
	switch kind {
	case mBefore:
		step = prev
	case mAfter:
		step, f = next, m.down
	}
	// Out to the first element whose answer is known, or to the end …
	var todo []*html.Node
	r := no
	for a := el; a != nil; a = step(a) {
		if v, ok := m.memo[memoKey{s, i, a, kind}]; ok {
			r = v
			break
		}
		todo = append(todo, a)
	}
	// … and back: the answer of a is that of step(a), and step(a) itself.
	for n := len(todo) - 1; n >= 0; n-- {
		if b := step(todo[n]); b == nil {
			r = no
		} else if r != yes {
			r = max(r, f(b, s, i))
		}
		m.memo[memoKey{s, i, todo[n], kind}] = r
	}
	return r
}

// has: s, relative to el, matches something — `:has(> .a .b)`.
func (m *matcher) has(el *html.Node, s *selector) tri {
	return m.related(el, s, 0)
}

// down: x matches parts[i], and parts[i+1:] match what stands after it.
func (m *matcher) down(x *html.Node, s *selector, i int) tri {
	r := m.compound(x, &s.parts[i])
	if r == no || i == len(s.parts)-1 {
		return r
	}
	k := memoKey{s: s, i: i, el: x, kind: mDown}
	if v, ok := m.memo[k]; ok {
		return v
	}
	r = min(r, m.related(x, s, i+1))
	m.memo[k] = r
	return r
}

// related is the best answer of down(·, s, i) over the elements the
// combinator before parts[i] leads to from el: its descendants, children,
// next sibling, following siblings.
func (m *matcher) related(el *html.Node, s *selector, i int) tri {
	r := no
	switch s.comb[i] {
	case '>':
		for c := el.FirstChild; c != nil && r != yes; c = c.NextSibling {
			if c.Type == html.ElementNode {
				r = max(r, m.down(c, s, i))
			}
		}
	case '+':
		if n := next(el); n != nil {
			r = m.down(n, s, i)
		}
	case '~':
		r = m.chain(el, s, i, mAfter)
	default: // a descendant
		r = m.below(el, s, i)
	}
	return r
}

// below is the best answer of down(·, s, i) over the descendants of el, kept
// per element as chain keeps its answers: `div:has(.x)` asks it of every div,
// and each subtree is walked once.
func (m *matcher) below(el *html.Node, s *selector, i int) tri {
	k := memoKey{s, i, el, mBelow}
	if v, ok := m.memo[k]; ok {
		return v
	}
	r := no
	for c := el.FirstChild; c != nil && r != yes; c = c.NextSibling {
		if c.Type == html.ElementNode {
			if r = max(r, m.down(c, s, i)); r != yes {
				r = max(r, m.below(c, s, i))
			}
		}
	}
	m.memo[k] = r
	return r
}

func (m *matcher) compound(el *html.Node, c *compound) tri {
	m.work++
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
		if hasClass(el, s.name, m.quirks) {
			return yes
		}
		return may(m.script.value(s.name)) // a class the script may add
	case sID:
		if v, ok := attribute(el, "id"); ok && (v == s.name || m.quirks && strings.EqualFold(v, s.name)) {
			return yes
		}
		return may(m.script.value(s.name))
	case sAttr:
		if dynamic(s.name) || m.script.attribute(s.name) {
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

func may(b bool) tri {
	if b {
		return maybe
	}
	return no
}

// names is what a page's script names: the words of its text. A behaviour
// writes state the HTML does not show, and names what it writes — a class
// in `classList.add("dark")`, an attribute in `setAttribute("data-x", …)`,
// a property that reflects one in `e.tabIndex = 0` — so a selector on a name
// that is in the script never decides anything (builder.md, CSS). A word
// that is only a variable or a key of the script makes a selector "maybe"
// for nothing: that is the safe side. What the script computes is not seen.
type names struct {
	text   string          // the script, for a name that is not a word (`sm:flex`)
	words  map[string]bool // as written: for a custom property, an animation
	folded map[string]bool // lower-cased, hyphens dropped: `tabIndex`, `data-foo-bar`, `fooBar`
}

func nameChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '$' || c >= 0x80
}

func fold(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "")
}

func scriptNames(script string) *names {
	n := &names{text: script, words: map[string]bool{}, folded: map[string]bool{}}
	for i := 0; i < len(script); {
		if !nameChar(script[i]) {
			i++
			continue
		}
		j := i
		for j < len(script) && nameChar(script[j]) {
			j++
		}
		n.words[script[i:j]] = true
		n.folded[fold(script[i:j])] = true
		i = j
	}
	return n
}

// value reports whether the script names a class or an id — whatever its
// case: quirks mode folds it, and folding matches more.
func (n *names) value(name string) bool {
	if n == nil {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !nameChar(name[i]) { // not one word of the script: anywhere in its text
			return strings.Contains(strings.ToLower(n.text), strings.ToLower(name))
		}
	}
	return n.folded[fold(name)]
}

// attribute reports whether the script names an attribute: as it is written
// (`setAttribute("data-open")`), or as the property that reflects it —
// `tabIndex` for tabindex, `dataset.fooBar` for data-foo-bar, `className`
// and `classList` for class, `htmlFor` for for.
func (n *names) attribute(name string) bool {
	if n == nil {
		return false
	}
	switch name = strings.ToLower(name); {
	case n.folded[fold(name)]:
		return true
	case name == "class":
		return n.words["className"] || n.words["classList"]
	case name == "for":
		return n.words["htmlFor"]
	}
	rest, data := strings.CutPrefix(name, "data-")
	return data && n.folded[fold(rest)]
}

// treeWriters are the DOM's ways to add, move or remove an element. A script
// that names one changes what stands next to what: no selector with a
// combinator is decided by the page as it was written.
var treeWriters = []string{
	"createElement", "createElementNS", "innerHTML", "outerHTML", "insertAdjacentHTML", "insertAdjacentElement",
	"setHTMLUnsafe", "setHTML", "createContextualFragment", "DOMParser", "parseHTMLUnsafe", "cloneNode", "importNode",
	"adoptNode", "appendChild", "insertBefore", "replaceChild", "removeChild", "replaceChildren", "replaceWith",
	"append", "prepend", "before", "after", "remove", "moveBefore", "attachShadow", "write", "writeln",
}

func (n *names) changesTree() bool {
	for _, name := range treeWriters {
		if n.words[name] {
			return true
		}
	}
	return false
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

// hasClass: classes are separated by HTML's whitespace, not Unicode's. fold:
// whatever their case, as in quirks mode — which folds ASCII only; folding
// more matches more, and that is the safe side.
func hasClass(el *html.Node, class string, fold bool) bool {
	v, _ := attribute(el, "class")
	for v != "" {
		i := 0
		for i < len(v) && !isSpace(v[i]) {
			i++
		}
		if i > 0 && (v[:i] == class || fold && strings.EqualFold(v[:i], class)) {
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
