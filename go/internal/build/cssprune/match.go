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

// dynamic reports whether an attribute is runtime state whoever could write
// it: one the browser writes by itself, with no script — so a selector on it
// never decides anything (builder.md, CSS, *Runtime state*).
//
//   - `open`: a `<details>` the reader opens (and those of its `name` the
//     browser closes for it, and one that find-in-page opens); a `<dialog>`
//     that a `command` button, Esc or `closedby` opens and closes.
//   - `hidden`: `hidden="until-found"`, which find-in-page and a fragment
//     take away.
//   - `style`: an element with `resize` gets its `width` and `height` there
//     when the reader drags it.
//
// Every other attribute is written by a script or by nobody: `inert`,
// `disabled`, `aria-*`, `data-state` have no writer in the browser; what the
// reader does to a control changes its state — `:checked`, `:disabled`, the
// value — and never `checked`, `selected` or `value`, which are its defaults
// (a reset reads them); a popover and a modal dialog reflect nothing but
// `open`. Those are decided on the page unless the page's script names them
// (names.attribute) — the owner's ruling on M, decisions.md. An element the
// reader edits, where the browser writes what it likes, leaves its page
// unpruned (pruner.page).
func dynamic(name string) bool {
	switch name {
	case "open", "hidden", "style":
		return true
	}
	return false
}

// readerWrites: an attribute the browser writes on this element alone, for
// the reader, from the element's own menu:
//
//   - `dir` on a text control, which the reader may turn around (HTML, *The
//     dir attribute*: the user agent sets the attribute);
//   - `controls` and `loop` on a `<video>` or an `<audio>`: "Show controls"
//     and "Loop" set them and take them away.
func readerWrites(el *html.Node, name string) bool {
	switch name {
	case "dir":
		return el.Data == "input" || el.Data == "textarea"
	case "controls", "loop":
		return el.Data == "video" || el.Data == "audio"
	}
	return false
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
		// The script is asked first: a name that is in it never decides
		// anything (builder.md, CSS, *The page's script*). A class it names
		// may be added — and taken away, from an element that has it: "yes"
		// there would make `:not(.collapsed)` "no", and the rule would be
		// gone when `classList.toggle("collapsed")` makes it match.
		switch {
		case m.script.value(s.name):
			return maybe
		case !hasClass(el, s.name, m.quirks):
			return no
		case m.script.writesClass():
			return maybe // `className = ""`: every class the page has may go
		}
		return yes
	case sID:
		v, ok := attribute(el, "id")
		switch {
		case m.script.value(s.name):
			return maybe
		case !ok || v != s.name && !(m.quirks && strings.EqualFold(v, s.name)):
			return no
		case m.script.writesID():
			return maybe // `e.id = "second"`: the id it had is not named
		}
		return yes
	case sAttr:
		if dynamic(s.name) || readerWrites(el, s.name) || m.script.attribute(s.name) {
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
		// A script that writes an element's text takes away what the
		// element held: what it has is not sure to stay, and what it has
		// not, it does not get — "no" is still "no".
		if r == yes && m.script.writesText() {
			return maybe
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
	// tree: the first name of the script that is one of treeWriters — but a
	// `remove` that is `classList.remove`; "" when it names none.
	tree string
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
		word := script[i:j]
		n.words[word] = true
		n.folded[fold(word)] = true
		if n.tree == "" && treeWriters[word] && !(word == "remove" && ofClassList(script[:i])) {
			n.tree = word
		}
		i = j
	}
	return n
}

// ofClassList reports whether the name that follows before is a member of
// `classList`, written so: `e.classList.remove`, `e.classList?.remove`. A
// token list's `remove` takes a class away, not an element — and it is the
// commonest write a behaviour makes. A list reached another way (`const l =
// e.classList; l.remove("x")`, `classList["remove"]`) is not seen to be
// one: that `remove` is the DOM's, which is the safe side.
func ofClassList(before string) bool {
	before = strings.TrimRight(before, " \t\r\n")
	before, dot := strings.CutSuffix(before, ".")
	before = strings.TrimRight(strings.TrimSuffix(before, "?"), " \t\r\n")
	const list = "classList"
	return dot && strings.HasSuffix(before, list) && (len(before) == len(list) || !nameChar(before[len(before)-len(list)-1]))
}

// writesClass reports whether the script may write `class` whole, so that a
// class the page has may go without being named: `className = ""`,
// `setAttribute("class", …)`, `removeAttribute("class")` — the word `class`,
// whatever it is there — and `classList.value = ""`. `classList` alone names
// what it adds and removes.
func (n *names) writesClass() bool {
	return n != nil && (n.words["className"] || n.folded["class"] || n.words["classList"] && n.words["value"])
}

// writesID: so for `id` — `e.id = "second"`, `removeAttribute("id")`.
func (n *names) writesID() bool {
	return n != nil && n.folded["id"]
}

// writesText reports whether the script names a property that sets an
// element's text, and so removes the elements it held: `textContent`,
// `innerText`, and `text` — of a link, an option, a title. Read or written:
// `Object.assign(e, { textContent })` is no assignment to look for.
// (`outerText` replaces the element itself: treeWriters.)
func (n *names) writesText() bool {
	return n != nil && (n.words["textContent"] || n.words["innerText"] || n.words["text"])
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
// (`setAttribute("data-open")`, `toggleAttribute("inert")`,
// `removeAttribute("aria-disabled")`), or as a property that reflects it
// (builder.md, CSS, *The page's script*, the table):
//
//   - its own name, whatever the case and the hyphens: `disabled`, `inert`,
//     `tabIndex` for tabindex, `readOnly`, `ariaExpanded` for aria-expanded
//     — and `value`, `checked`, `selected`, whose properties reflect on some
//     elements and on others do not: named is named;
//   - `default` before it: `defaultValue`, `defaultChecked`,
//     `defaultSelected`, `defaultMuted`;
//   - `Element` or `Elements` after it: `ariaControlsElements`,
//     `ariaActiveDescendantElement`, `popoverTargetElement`,
//     `commandForElement` — setting one writes the attribute, empty;
//   - `dataset.fooBar` for data-foo-bar: the word after `data-`;
//   - `className` and `classList` for class, `htmlFor` for for, `relList`
//     for rel.
func (n *names) attribute(name string) bool {
	if n == nil {
		return false
	}
	name = strings.ToLower(name)
	switch folded := fold(name); {
	case n.folded[folded], n.folded["default"+folded], n.folded[folded+"element"], n.folded[folded+"elements"]:
		return true
	case name == "class":
		return n.words["className"] || n.words["classList"]
	case name == "for":
		return n.words["htmlFor"]
	case name == "rel":
		return n.words["relList"]
	}
	rest, data := strings.CutPrefix(name, "data-")
	return data && n.folded[fold(rest)]
}

// treeWriters are the DOM's ways to add, move or remove an element. A script
// that names one changes what stands next to what: no selector with a
// combinator is decided by the page as it was written.
//
// Setting an element's text (`textContent`, `innerText`) is not among them:
// it removes what the element held and leaves every other element where it
// stood, so only `:has()` is touched (writesText). `outerText` removes the
// element itself, from between its siblings.
var treeWriters = set(
	"createElement", "createElementNS", "innerHTML", "outerHTML", "insertAdjacentHTML", "insertAdjacentElement",
	"setHTMLUnsafe", "setHTML", "createContextualFragment", "DOMParser", "parseHTMLUnsafe", "cloneNode", "importNode",
	"adoptNode", "appendChild", "insertBefore", "replaceChild", "removeChild", "replaceChildren", "replaceWith",
	"append", "prepend", "before", "after", "remove", "moveBefore", "attachShadow", "write", "writeln",
	"outerText",
	// A table's own, a range's, a selection's, and the editor's.
	"insertRow", "deleteRow", "insertCell", "deleteCell", "createTHead", "deleteTHead", "createTFoot", "deleteTFoot",
	"createTBody", "createCaption", "deleteCaption", "insertNode", "surroundContents", "extractContents",
	"deleteContents", "deleteFromDocument", "execCommand", "contentEditable", "designMode",
)

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
