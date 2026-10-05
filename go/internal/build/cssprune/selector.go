package cssprune

import (
	"fmt"
	"strings"
)

// The selector parser. It is strict: what it does not understand is an error,
// and a selector that does not parse is kept (builder.md, CSS).

type simpleKind uint8

const (
	sClass simpleKind = iota
	sID
	sAttr
	sRoot  // :root
	sMaybe // runtime state: any other pseudo-class, and a `&` that survived lowering
	sIs    // :is(), :where()
	sNot
	sHas
)

type simple struct {
	kind  simpleKind
	name  string      // the class, the id, the attribute (lower-cased), the pseudo-class
	op    byte        // sAttr: 0 for [name], else '=', '~', '|', '^', '$', '*'
	value string      // sAttr
	flag  byte        // sAttr: 'i', 's' or 0
	args  []*selector // sIs, sNot, sHas
}

type compound struct {
	tag     string // lower-cased; "" and "*" are any element
	simples []simple
	pseudo  bool // it ends in a pseudo-element: the selector's last compound
}

// selector is a complex selector: compounds joined by combinators.
type selector struct {
	parts []compound
	// comb[i] stands before parts[i]: ' ', '>', '+', '~'. comb[0] is 0 unless
	// the selector is relative (`> a` in :has() and in @scope).
	comb []byte
	// known: every browser of the floor accepts it. One selector a browser
	// rejects makes it reject the whole rule, so a selector that is not known
	// is never dropped on its own: trimming the list could bring a dead rule
	// to life.
	known bool
}

// Pseudo-classes and pseudo-elements every browser of the floor parses
// (decisions.md, 11). Anything else — prefixed, newer, misspelt — is matched
// the same way ("maybe"), but makes its selector not known.
var knownPseudoClass = set(
	"active", "any-link", "autofill", "checked", "default", "defined", "disabled", "empty", "enabled",
	"first-child", "first-of-type", "focus", "focus-visible", "focus-within", "fullscreen", "host", "hover",
	"in-range", "indeterminate", "invalid", "last-child", "last-of-type", "link", "modal", "only-child",
	"only-of-type", "optional", "out-of-range", "placeholder-shown", "popover-open", "read-only", "read-write",
	"required", "root", "scope", "target", "user-invalid", "user-valid", "valid", "visited",
)

var knownPseudoElement = set(
	"after", "backdrop", "before", "cue", "details-content", "file-selector-button", "first-letter", "first-line",
	"marker", "placeholder", "selection",
)

// CSS 2's pseudo-elements, which may be written with one colon.
var legacyPseudoElement = set("after", "before", "first-letter", "first-line")

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// maxSelectorDepth bounds :is(:not(:has(…))).
const maxSelectorDepth = 16

type selParser struct {
	s     string
	i     int
	known bool
	depth int
	inHas bool
}

// parseSelector parses one complex selector. relative: it may start with a
// combinator.
func parseSelector(text string, relative bool) (*selector, error) {
	return parseSelectorAt(text, relative, 0, false)
}

func parseSelectorAt(text string, relative bool, depth int, inHas bool) (*selector, error) {
	if depth > maxSelectorDepth {
		return nil, fmt.Errorf("selector nested deeper than %d", maxSelectorDepth)
	}
	p := &selParser{s: text, known: true, depth: depth, inHas: inHas}
	sel, err := p.complex(relative)
	if err != nil {
		return nil, fmt.Errorf("selector %q: %w", clip(text), err)
	}
	sel.known = p.known
	return sel, nil
}

func (p *selParser) ws() bool {
	start := p.i
	for p.i < len(p.s) && isSpace(p.s[p.i]) {
		p.i++
	}
	return p.i > start
}

func (p *selParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *selParser) ident() (string, error) {
	if !identStart(p.s, p.i) {
		return "", fmt.Errorf("an identifier expected at %d", p.i)
	}
	var name string
	name, p.i = readName(p.s, p.i)
	return name, nil
}

func (p *selParser) complex(relative bool) (*selector, error) {
	sel := &selector{}
	p.ws()
	comb := byte(0)
	if c := p.peek(); relative && (c == '>' || c == '+' || c == '~') {
		comb = c
		p.i++
		p.ws()
	}
	for {
		cp, err := p.compound()
		if err != nil {
			return nil, err
		}
		sel.comb, sel.parts = append(sel.comb, comb), append(sel.parts, cp)
		space := p.ws()
		switch c := p.peek(); {
		case p.i >= len(p.s):
			return sel, nil
		case cp.pseudo:
			// `.a::before > .b`: no browser reads it, and the rule dies with
			// it. Not a selector to reason about — kept, with its list.
			return nil, fmt.Errorf("a pseudo-element is not last, at %d", p.i)
		case c == '>' || c == '+' || c == '~':
			comb = c
			p.i++
			p.ws()
		case space:
			comb = ' '
		default:
			return nil, fmt.Errorf("unexpected %q at %d", c, p.i)
		}
	}
}

func (p *selParser) compound() (compound, error) {
	var cp compound
	start := p.i
	if p.peek() == '*' {
		cp.tag = "*"
		p.i++
	} else if identStart(p.s, p.i) {
		name, _ := p.ident()
		cp.tag = strings.ToLower(name)
	}
	if p.peek() == '|' {
		return cp, fmt.Errorf("namespaces are not supported")
	}
	for {
		c := p.peek()
		if cp.pseudo && c != ':' {
			break // only pseudo-classes may follow a pseudo-element
		}
		switch c {
		case '.', '#':
			p.i++
			name, err := p.ident()
			if err != nil {
				return cp, err
			}
			k := sClass
			if c == '#' {
				k = sID
			}
			cp.simples = append(cp.simples, simple{kind: k, name: name})
		case '[':
			a, err := p.attribute()
			if err != nil {
				return cp, err
			}
			cp.simples = append(cp.simples, a)
		case '&':
			p.i++
			cp.simples = append(cp.simples, simple{kind: sMaybe, name: "&"})
		case ':':
			s, pe, err := p.pseudo(cp.pseudo)
			if err != nil {
				return cp, err
			}
			if pe {
				cp.pseudo = true
			} else {
				cp.simples = append(cp.simples, s)
			}
		default:
			if p.i == start {
				return cp, fmt.Errorf("a selector expected at %d", p.i)
			}
			return cp, nil
		}
	}
	return cp, nil
}

func (p *selParser) attribute() (simple, error) {
	a := simple{kind: sAttr}
	p.i++
	p.ws()
	name, err := p.ident()
	if err != nil {
		return a, err
	}
	a.name = strings.ToLower(name)
	p.ws()
	switch c := p.peek(); c {
	case ']':
		p.i++
		return a, nil
	case '=':
		a.op = '='
		p.i++
	case '~', '|', '^', '$', '*':
		if p.i+1 >= len(p.s) || p.s[p.i+1] != '=' {
			return a, fmt.Errorf("unexpected %q in an attribute selector", c) // and ns|name
		}
		a.op = c
		p.i += 2
	default:
		return a, fmt.Errorf("unexpected %q in an attribute selector", c)
	}
	p.ws()
	if c := p.peek(); c == '"' || c == '\'' {
		if a.value, p.i, err = readString(p.s, p.i); err != nil {
			return a, err
		}
	} else if a.value, err = p.ident(); err != nil {
		return a, err
	}
	if p.ws(); identStart(p.s, p.i) {
		flag, _ := p.ident()
		switch flag {
		case "i", "I":
			a.flag = 'i'
		case "s", "S":
			a.flag = 's'
			p.known = false // only Firefox parses it
		default:
			return a, fmt.Errorf("unknown attribute flag %q", flag)
		}
		p.ws()
	}
	if p.peek() != ']' {
		return a, fmt.Errorf("] expected at %d", p.i)
	}
	p.i++
	return a, nil
}

// pseudo parses :name, ::name and their functional forms. element: it is a
// pseudo-element, which matching ignores. after: a pseudo-element came before.
//
// What follows a pseudo-element is never known: the grammar allows a
// pseudo-class there, but which pairs a browser accepts is its own business —
// Chromium and WebKit reject `::before:hover`, `::backdrop:first-child`,
// `::before::marker` and most other pairs of the tables above, and esbuild
// prints them all (builder.md, CSS, *Lists*).
func (p *selParser) pseudo(after bool) (s simple, element bool, err error) {
	p.known = p.known && !after
	p.i++
	if p.peek() == ':' {
		element = true
		p.i++
	}
	name, err := p.ident()
	if err != nil {
		return s, false, err
	}
	name = strings.ToLower(name)
	args, functional := "", p.peek() == '('
	if functional {
		end, err := skip(p.s, p.i, 0)
		if err != nil {
			return s, false, err
		}
		args, p.i = p.s[p.i+1:end-1], end
	}
	if !element && !functional && legacyPseudoElement[name] {
		element = true
	}
	if element {
		if p.depth > 0 {
			return s, false, fmt.Errorf("a pseudo-element inside a pseudo-class")
		}
		p.known = p.known && !functional && knownPseudoElement[name]
		return s, true, nil
	}
	s = simple{kind: sMaybe, name: name}
	switch {
	case after: // `::backdrop:hover`: state of something that is not an element
	case !functional:
		if name == "root" {
			s.kind = sRoot
		}
		p.known = p.known && knownPseudoClass[name]
	case name == "is" || name == "where":
		// A forgiving list: a browser ignores an argument it cannot parse.
		// Here such an argument may match anything.
		s.kind = sIs
		list, err := splitList(args)
		if err != nil {
			return s, false, err
		}
		for _, text := range list {
			if len(list) == 1 && text == "" {
				break // :is() matches nothing
			}
			arg, err := parseSelectorAt(text, false, p.depth+1, p.inHas)
			if err != nil {
				arg = &selector{parts: []compound{{simples: []simple{{kind: sMaybe, name: "?"}}}}, comb: []byte{0}}
			}
			s.args = append(s.args, arg)
		}
	case name == "not" || name == "has":
		s.kind = sNot
		if name == "has" {
			if s.kind = sHas; p.inHas {
				return s, false, fmt.Errorf(":has() inside :has()")
			}
		}
		list, err := splitList(args)
		if err != nil {
			return s, false, err
		}
		for _, text := range list {
			arg, err := parseSelectorAt(text, name == "has", p.depth+1, p.inHas || name == "has")
			if err != nil {
				return s, false, err
			}
			p.known = p.known && arg.known
			s.args = append(s.args, arg)
		}
	default:
		p.known = p.known && knownNth[name] && validNth(args)
	}
	return s, false, nil
}

var knownNth = set("nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type")

// validNth: An+B, `even`, `odd` — without `of S`, which is newer.
func validNth(s string) bool {
	s = strings.ToLower(trim(s))
	if s == "even" || s == "odd" {
		return true
	}
	digits := func(s string) int {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		return i
	}
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	n := digits(s)
	if n == len(s) {
		return n > 0 // B
	}
	if s[n] != 'n' {
		return false
	}
	if s = trim(s[n+1:]); s == "" {
		return true // An
	}
	if s[0] != '+' && s[0] != '-' {
		return false
	}
	s = trim(s[1:])
	return s != "" && digits(s) == len(s) // An+B, with spaces around the sign
}
