package cssprune

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// naive is the matcher without what it keeps: every walk — the ancestors, a
// row of siblings, a subtree — is done again from every element, as the
// definition of each combinator reads. What chain and below remember must be
// what this finds.
type naive struct{ m *matcher }

func (n naive) upTo(el *html.Node, s *selector, i int) tri {
	r := n.compound(el, &s.parts[i])
	if r == no {
		return no
	}
	if i == 0 {
		if s.comb[0] != 0 {
			return maybe
		}
		return r
	}
	rel := no
	switch s.comb[i] {
	case ' ':
		for a := parent(el); a != nil; a = parent(a) {
			rel = max(rel, n.upTo(a, s, i-1))
		}
	case '>':
		if a := parent(el); a != nil {
			rel = n.upTo(a, s, i-1)
		}
	case '+':
		if a := prev(el); a != nil {
			rel = n.upTo(a, s, i-1)
		}
	case '~':
		for a := prev(el); a != nil; a = prev(a) {
			rel = max(rel, n.upTo(a, s, i-1))
		}
	}
	return min(r, rel)
}

// down: x matches parts[i], and the rest matches what the combinators lead to.
func (n naive) down(x *html.Node, s *selector, i int) tri {
	r := n.compound(x, &s.parts[i])
	if r == no || i == len(s.parts)-1 {
		return r
	}
	return min(r, n.related(x, s, i+1))
}

func (n naive) related(el *html.Node, s *selector, i int) tri {
	r := no
	switch s.comb[i] {
	case '>':
		for c := el.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				r = max(r, n.down(c, s, i))
			}
		}
	case '+':
		if x := next(el); x != nil {
			r = n.down(x, s, i)
		}
	case '~':
		for x := next(el); x != nil; x = next(x) {
			r = max(r, n.down(x, s, i))
		}
	default:
		var walk func(*html.Node)
		walk = func(p *html.Node) {
			for c := p.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					r = max(r, n.down(c, s, i))
					walk(c)
				}
			}
		}
		walk(el)
	}
	return r
}

func (n naive) compound(el *html.Node, c *compound) tri {
	if c.tag != "" && c.tag != "*" && !strings.EqualFold(c.tag, el.Data) {
		return no
	}
	r := yes
	for i := range c.simples {
		s := &c.simples[i]
		v := no
		switch s.kind {
		case sIs:
			for _, a := range s.args {
				x := n.upTo(el, a, len(a.parts)-1)
				if !a.known {
					x = min(x, maybe)
				}
				v = max(v, x)
			}
		case sNot:
			for _, a := range s.args {
				v = max(v, n.upTo(el, a, len(a.parts)-1))
			}
			v = yes - v
		case sHas:
			for _, a := range s.args {
				v = max(v, n.related(el, a, 0))
			}
		default:
			v = n.m.simple(el, s)
		}
		r = min(r, v)
	}
	return r
}

// TestMatchAgainstNaive: on random pages and selectors the matcher answers,
// for every element, what the naive one does.
func TestMatchAgainstNaive(t *testing.T) {
	rnd := rand.New(rand.NewPCG(20, 26))
	pick := func(of ...string) string { return of[rnd.IntN(len(of))] }

	var tree func(depth int, left *int) string
	tree = func(depth int, left *int) string {
		var b strings.Builder
		for *left > 0 && rnd.IntN(4) != 0 {
			*left--
			tag := pick("div", "p", "span")
			fmt.Fprintf(&b, `<%s class="%s %s">`, tag, pick("a", "b", "c", ""), pick("a", "b", ""))
			if depth < 6 && rnd.IntN(3) != 0 {
				b.WriteString(tree(depth+1, left))
			}
			b.WriteString("</" + tag + ">")
		}
		return b.String()
	}

	var complex func(depth int, inHas, relative bool) string
	compound := func(depth int, inHas bool) string {
		s := pick("", "", "div", "p", "span", "*")
		for k := rnd.IntN(3); k > 0 || s == ""; k-- {
			switch c := rnd.IntN(12); {
			case c < 5:
				s += "." + pick("a", "b", "c", "gone")
			case c < 7:
				s += pick(":hover", "[open]", ":first-child", ":root")
			case depth >= 2:
				s += ".a"
			case c < 9:
				s += ":not(" + complex(depth+1, inHas, false) + pick("", ","+complex(depth+1, inHas, false)) + ")"
			case c < 11:
				s += ":is(" + complex(depth+1, inHas, false) + pick("", ","+complex(depth+1, inHas, false), ",:open") + ")"
			case !inHas:
				s += ":has(" + complex(depth+1, true, true) + pick("", ","+complex(depth+1, true, true)) + ")"
			default:
				s += ".b"
			}
		}
		return s
	}
	complex = func(depth int, inHas, relative bool) string {
		s := ""
		if relative {
			s = pick("", "", "> ", "+ ", "~ ")
		}
		s += compound(depth, inHas)
		for k := rnd.IntN(4 - depth); k > 0; k-- {
			s += pick(" ", " ", ">", "+", "~", "~") + compound(depth, inHas)
		}
		return s
	}

	answers := map[tri]int{}
	for round := 0; round < 400; round++ {
		left := 4 + rnd.IntN(40)
		src := tree(0, &left)
		var p pruner
		p.page(parsePage(t, src))
		for k := 0; k < 25; k++ {
			text := complex(0, false, false)
			sel, err := parseSelector(text, false)
			if err != nil {
				t.Fatalf("%q: %v", text, err)
			}
			clear(p.m.memo)
			some := false
			for _, el := range p.m.els {
				got, want := p.m.upTo(el, sel, len(sel.parts)-1), naive{&p.m}.upTo(el, sel, len(sel.parts)-1)
				if got != want {
					t.Fatalf("%q on <%s class=%q> of\n%s\ngot %v, want %v", text, el.Data, el.Attr, src, got, want)
				}
				answers[got]++
				some = some || got != no
			}
			if got := p.m.may(sel); got != some {
				t.Fatalf("may(%q) = %v on\n%s", text, got, src)
			}
		}
	}
	// The test is only worth what it reaches: all three answers, often.
	if answers[no] < 1000 || answers[maybe] < 1000 || answers[yes] < 1000 {
		t.Errorf("answers compared: %d no, %d maybe, %d yes", answers[no], answers[maybe], answers[yes])
	}
}
