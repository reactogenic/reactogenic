package cssprune

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// show prints a parsed selector unambiguously.
func show(s *selector) string {
	var b strings.Builder
	for i, c := range s.parts {
		if s.comb[i] != 0 {
			fmt.Fprintf(&b, " %c ", s.comb[i])
		}
		b.WriteString(c.tag)
		for _, sm := range c.simples {
			switch sm.kind {
			case sClass:
				b.WriteString("." + sm.name)
			case sID:
				b.WriteString("#" + sm.name)
			case sAttr:
				b.WriteString("[" + sm.name)
				if sm.op != 0 {
					fmt.Fprintf(&b, " %c= %q", sm.op, sm.value)
				}
				if sm.flag != 0 {
					fmt.Fprintf(&b, " %c", sm.flag)
				}
				b.WriteString("]")
			case sRoot:
				b.WriteString(":root")
			case sMaybe:
				b.WriteString("?" + sm.name)
			default:
				b.WriteString(map[simpleKind]string{sIs: ":is(", sNot: ":not(", sHas: ":has("}[sm.kind])
				for j, a := range sm.args {
					if j > 0 {
						b.WriteString(", ")
					}
					b.WriteString(show(a))
				}
				b.WriteString(")")
			}
		}
	}
	return b.String()
}

func TestParseSelector(t *testing.T) {
	tests := []struct {
		in       string
		relative bool
		want     string
		unknown  bool // some browser of the floor may reject it
		bad      bool // it does not parse
	}{
		{in: `div`, want: `div`},
		{in: `DIV`, want: `div`},
		{in: `*`, want: `*`},
		{in: `.a.b#c`, want: `.a.b#c`},
		{in: `a.b > c + d ~ e f`, want: `a.b > c + d ~ e   f`},
		{in: `a>b+c~d`, want: `a > b + c ~ d`},
		{in: ` a  >  b `, want: `a > b`},
		// Attribute selectors: every operator, both quotes, the flags.
		{in: `[hidden]`, want: `[hidden]`},
		{in: `[data-x=y]`, want: `[data-x == "y"]`},
		{in: `[ data-x = "a b" ]`, want: `[data-x == "a b"]`},
		{in: `[data-x~='y']`, want: `[data-x ~= "y"]`},
		{in: `[lang|=en]`, want: `[lang |= "en"]`},
		{in: `a[href^=http]`, want: `a[href ^= "http"]`},
		{in: `a[href$=".pdf"]`, want: `a[href $= ".pdf"]`},
		{in: `[class*=col-]`, want: `[class *= "col-"]`},
		{in: `[data-v=Primary i]`, want: `[data-v == "Primary" i]`},
		{in: `[data-v="Primary"I]`, want: `[data-v == "Primary" i]`},
		{in: `[data-v=x s]`, want: `[data-v == "x" s]`, unknown: true},
		{in: `[DATA-V=x]`, want: `[data-v == "x"]`},
		{in: `[data-x="a\"b\5c c"]`, want: `[data-x == "a\"b\\c"]`},
		// Pseudo-classes: runtime state, except :root and the logical ones.
		{in: `a:hover`, want: `a?hover`},
		{in: `:root`, want: `:root`},
		{in: `:ROOT`, want: `:root`},
		{in: `li:nth-child(2n + 1)`, want: `li?nth-child`},
		{in: `li:nth-child(2n+1 of .x)`, want: `li?nth-child`, unknown: true},
		{in: `:lang(en)`, want: `?lang`, unknown: true},
		{in: `:open`, want: `?open`, unknown: true},
		{in: `:-moz-focusring`, want: `?-moz-focusring`, unknown: true},
		{in: `.a:is(.x,.y)`, want: `.a:is(.x, .y)`},
		{in: `:is(.E .F,#G) .h`, want: `:is(.E   .F, #G)   .h`},
		{in: `:where(a, b > c)`, want: `:is(a, b > c)`},
		{in: `:is()`, want: `:is()`},
		{in: `:is(.a, ::before, .b)`, want: `:is(.a, ??, .b)`}, // a forgiving list
		{in: `:not(.a, [open])`, want: `:not(.a, [open])`},
		{in: `:not(.a .b)`, want: `:not(.a   .b)`},
		{in: `.l:has(.row)`, want: `.l:has(.row)`},
		{in: `.l:has(> .row + .x, ~ .y)`, want: `.l:has( > .row + .x,  ~ .y)`},
		{in: `:root:has(.rg-dialog:modal)`, want: `:root:has(.rg-dialog?modal)`},
		{in: `:not(:has(.row))`, want: `:not(:has(.row))`},
		{in: `:not(:foo)`, want: `:not(?foo)`, unknown: true},
		{in: `:has(:is(:has(a)))`, want: `:has(:is(??))`}, // invalid inside, and forgiven
		// Pseudo-elements are not matched; what follows one is state.
		{in: `.a::before`, want: `.a`},
		{in: `.a:after`, want: `.a`},
		{in: `::selection`, want: ``},
		// Nothing follows a pseudo-element in every browser of the floor.
		{in: `.a::backdrop:hover`, want: `.a?hover`, unknown: true},
		{in: `.a:before:first-child`, want: `.a?first-child`, unknown: true},
		{in: `.a::before::marker`, want: `.a`, unknown: true},
		{in: `.a::before:after`, want: `.a`, unknown: true},
		{in: `.a::file-selector-button:is(:hover)`, want: `.a?is`, unknown: true},
		{in: `.a::before > .b`, bad: true},
		{in: `.a::before .b`, bad: true},
		{in: `.a:after + .b`, bad: true},
		{in: `.a::before ~ .b`, bad: true},
		{in: `.l:has(> .a::before)`, bad: true},
		{in: `.a::-webkit-scrollbar-thumb`, want: `.a`, unknown: true},
		{in: `.a::part(x)`, want: `.a`, unknown: true},
		{in: `details::details-content`, want: `details`},
		// `&` where esbuild leaves it, and relative selectors in @scope.
		{in: `&`, want: `?&`},
		{in: `.a&:hover`, want: `.a?&?hover`},
		{in: `> img`, relative: true, want: ` > img`},
		{in: `&>img`, want: `?& > img`},
		// Escapes.
		{in: `.sm\:flex`, want: `.sm:flex`},
		{in: `.\31 0`, want: `.10`},
		{in: `#\#x`, want: `##x`},
		{in: `.a\,b`, want: `.a,b`},
		{in: `.\--x`, want: `.--x`},
		{in: `.é`, want: `.é`},
		// Not understood: kept by the pruner.
		{in: `> img`, bad: true},
		{in: ``, bad: true},
		{in: `.a,`, bad: true},
		{in: `a b >`, bad: true},
		{in: `.`, bad: true},
		{in: `.1a`, bad: true},
		{in: `svg|a`, bad: true},
		{in: `*|a`, bad: true},
		{in: `[svg|href]`, bad: true},
		{in: `[a=1]`, bad: true},
		{in: `[a=b x]`, bad: true},
		{in: `[a`, bad: true},
		{in: `a || b`, bad: true},
		{in: `a/**/b`, bad: true},
		{in: `:not()`, bad: true},
		{in: `:not(::before)`, bad: true},
		{in: `:has(:has(a))`, bad: true},
		{in: `a::before.x`, bad: true},
		{in: `a!`, bad: true},
		{in: `:is(`, bad: true},
	}
	for _, tt := range tests {
		sel, err := parseSelector(tt.in, tt.relative)
		if err != nil {
			if !tt.bad {
				t.Errorf("%q: %v", tt.in, err)
			}
			continue
		}
		if got := show(sel); got != tt.want || tt.bad {
			t.Errorf("%q: got %q, want %q (an error: %v)", tt.in, got, tt.want, tt.bad)
		}
		if sel.known == tt.unknown {
			t.Errorf("%q: known = %v", tt.in, sel.known)
		}
	}
}

func TestValidNth(t *testing.T) {
	for in, want := range map[string]bool{
		"even": true, "ODD": true, "3": true, "+3": true, "-3": true, "n": true, "2n": true, "-n": true,
		"2n+1": true, "2n + 1": true, "-n+3": true, "2N-1": true,
		"2n+ 1": true, "2n -1": true, " 2n ": true, "+n": true,
		"": false, "foo": false, "2n+": false, "2n1": false, "n+n": false, "2n+1 of .x": false, "++2": false,
		"2 n": false, "+ 3": false, "- n": false, "2n + 1 2": false, "n n": false,
	} {
		if got := validNth(in); got != want {
			t.Errorf("validNth(%q) = %v", in, got)
		}
	}
}

// parsePage parses a page as the builder writes it: with the doctype
// (builder.md, Routes). TestPruneQuirks has the pages without one.
func parsePage(t testing.TB, src string) *html.Node {
	t.Helper()
	if len(src) < 9 || !strings.EqualFold(src[:9], "<!doctype") {
		src = "<!doctype html>" + src
	}
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestMatchLinear: a selector costs a number of steps proportional to the
// page. A row of siblings is not walked again from each of them (`~`), nor a
// subtree from each of its ancestors (`:has()`), nor the ancestors from each
// descendant: on a list of 20 000 items `.gone~li~li` took 6.5 s that way.
func TestMatchLinear(t *testing.T) {
	const wide, deep = 4000, 400
	pages := map[string]*html.Node{
		"wide": parsePage(t, `<ul>`+strings.Repeat(`<li class="r"></li>`, wide)+`</ul>`),
		"deep": parsePage(t, strings.Repeat(`<div class="r">`, deep)),
	}
	for _, tt := range []struct {
		page, sel string
		want      bool
	}{
		{"wide", `.gone~li~li`, false},
		{"wide", `li~li~li~.gone`, false},
		{"wide", `li~li~li~li`, true},
		{"wide", `li:hover~li:hover~.gone`, false},
		{"wide", `ul:has(li~li~.gone)`, false},
		{"wide", `li:has(~.gone)`, false},
		{"wide", `li:has(~li~li~.gone)`, false},
		{"wide", `li:not(:has(~li:hover~.gone))`, true},
		{"wide", `:is(.gone~li,li~.gone)~li`, false},
		{"deep", `.gone div div`, false},
		{"deep", `.gone .r .r .r`, false},
		{"deep", `div:hover div:hover .gone`, false},
		{"deep", `div:has(.gone)`, false},
		{"deep", `div:has(div div .gone)`, false},
		{"deep", `div:not(:has(div:hover .gone)) div`, true},
		{"deep", `:is(.gone div,div .gone) div`, false},
	} {
		var p pruner
		p.page(pages[tt.page])
		sel, err := parseSelector(tt.sel, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.m.may(sel); got != tt.want {
			t.Errorf("%s: may(%q) = %v", tt.page, tt.sel, got)
		}
		if n := len(p.m.els); p.m.work > 12*n {
			t.Errorf("%s: %q tried %d compounds on %d elements", tt.page, tt.sel, p.m.work, n)
		}
	}
}

// TestMatch: the three values, on the element with id="t".
func TestMatch(t *testing.T) {
	const page = `<!doctype html><html lang="en"><body>
<main class="A x" data-v="primary" data-size="lg">
  <p class="first">one</p>
  <p id="t" class="B  b2	b3" data-variant="ghost" data-list="a b c" lang="en-GB" title="Hello">two</p>
  <input type="checkbox" id="in">
  <div class="list"><span class="row"></span><em></em></div>
  <svg viewBox="0 0 1 1"><foreignObject><i class="in-svg"></i></foreignObject></svg>
</main></body></html>`
	tests := []struct {
		sel  string
		want tri
	}{
		{`p`, yes}, {`P`, yes}, {`*`, yes}, {`div`, no},
		{`.B`, yes}, {`.b2`, yes}, {`.b3`, yes}, {`.b`, no}, {`.B.b2`, yes}, {`.B.x`, no},
		{`#t`, yes}, {`#T`, no}, {`p#t.B`, yes},
		// Static attributes are exact.
		{`[data-variant]`, yes}, {`[data-variant=ghost]`, yes}, {`[data-variant=Ghost]`, no}, {`[data-variant=solid]`, no},
		{`[data-variant=Ghost i]`, yes}, {`[data-variant=Ghost s]`, no},
		{`[DATA-VARIANT=ghost]`, yes},
		{`[data-list~=b]`, yes}, {`[data-list~="a b"]`, no}, {`[data-list~=d]`, no}, {`[data-list~=""]`, no},
		{`[lang|=en]`, yes}, {`[lang|=EN]`, yes}, {`[lang|=e]`, no}, // lang is one of HTML's case-insensitive ones
		{`[title^=He]`, yes}, {`[title^=he]`, no}, {`[title^=he i]`, yes}, {`[title^=""]`, no},
		{`[title$=lo]`, yes}, {`[title$=LO]`, no}, {`[title*=ell]`, yes}, {`[title*=xyz]`, no}, {`[title*=""]`, no},
		{`[data-missing]`, no},
		// Runtime state never decides.
		{`:hover`, maybe}, {`p:focus-visible`, maybe}, {`div:hover`, no}, {`:nth-child(2)`, maybe}, {`:first-child`, maybe},
		{`[hidden]`, maybe}, {`[open]`, maybe}, {`[style*=color]`, maybe}, {`.B[open]`, maybe}, {`.gone[open]`, no},
		// … and what only a script writes is the page's, when there is no script (TestPruneState).
		{`[aria-current=page]`, no}, {`[data-state=open]`, no}, {`.B[aria-busy=true]`, no}, {`:not([aria-busy])`, yes},
		{`[disabled]`, no}, {`[value=x]`, no}, {`[inert]`, no}, {`[checked]`, no}, {`[selected]`, no},
		{`&`, maybe}, {`&.B`, maybe}, {`&.gone`, no},
		// Pseudo-elements are ignored.
		{`.B::before`, yes}, {`.gone::before`, no}, {`::selection`, yes}, {`.B::backdrop:hover`, maybe},
		// :root.
		{`:root`, no}, {`:root p`, yes}, {`:root > p`, no}, {`:not(:root)`, yes},
		// Combinators against the real tree.
		{`main p`, yes}, {`main > p`, yes}, {`body > p`, no}, {`body p`, yes}, {`html body main p`, yes},
		{`.first + p`, yes}, {`.first ~ p`, yes}, {`input + p`, no}, {`input ~ p`, no}, {`.list p`, no},
		{`.A.x > .B`, yes}, {`.A.y > .B`, no},
		// "maybe" next to a combinator stays "maybe".
		{`.first:hover + p`, maybe}, {`.first:hover ~ p`, maybe}, {`main:hover p`, maybe}, {`main[hidden] > p`, maybe},
		{`.gone:hover + p`, no}, {`input:checked ~ p`, no}, {`.first[open] + .B`, maybe},
		// :is(), :where(): on their arguments.
		{`:is(.B, .gone)`, yes}, {`:is(.gone, .gone2)`, no}, {`:is()`, no}, {`:where(p)`, yes},
		{`:is(.gone, :hover)`, maybe}, {`:is(main p)`, yes}, {`:is(.list p)`, no},
		{`:is(.A, .Z) .B`, yes}, {`.A:is(.x, .y) > p`, yes}, {`.A:is(.p, .q) > p`, no},
		{`:is(.B, ::before)`, yes}, {`:is(.gone, ::before)`, maybe}, {`:is(.B:open)`, maybe}, {`:is([data-variant=ghost s])`, maybe},
		// :not(): the negation of "maybe" is "maybe".
		{`:not(.B)`, no}, {`:not(.gone)`, yes}, {`:not(:hover)`, maybe}, {`:not([hidden])`, maybe},
		{`:not(.B:hover)`, maybe}, {`:not(.gone:hover)`, yes}, {`:not(.gone, .B)`, no}, {`:not(.gone, :hover)`, maybe},
		{`p:not(.first)`, yes}, {`:not(:not(.B))`, yes}, {`:not(:not(:hover))`, maybe}, {`:not(main p)`, no}, {`:not(.list p)`, yes},
		// :has(): relative to the element, on the real tree.
		{`main:has(.row) p`, yes}, {`main:has(.gone) p`, no}, {`main:has(> .row) p`, no}, {`main:has(> .list > .row) p`, yes},
		{`main:has(.row:hover) p`, maybe}, {`main:not(:has(.row)) p`, no}, {`main:not(:has(.gone)) p`, yes},
		{`main:not(:has(.row:hover)) p`, maybe}, {`:has(+ input)`, yes}, {`:has(+ .list)`, no}, {`:has(~ .list)`, yes},
		{`:has(~ .list .row)`, yes}, {`:has(~ .list > em)`, yes}, {`:has(~ .list > .gone)`, no}, {`:has(~ input:checked)`, maybe},
		{`.first:has(+ .B)`, no}, {`:has(.row)`, no}, {`:root:has(.row) p`, yes}, {`:has(main .row)`, no},
		{`:has(~ .list .row + em)`, yes}, {`:has(~ .list em + .row)`, no},
	}
	doc := parsePage(t, page)
	var p pruner
	p.page(doc)
	var target *html.Node
	for _, el := range p.m.els {
		if id, _ := attribute(el, "id"); id == "t" {
			target = el
		}
	}
	for _, tt := range tests {
		sel, err := parseSelector(tt.sel, false)
		if err != nil {
			t.Errorf("%q: %v", tt.sel, err)
			continue
		}
		clear(p.m.memo)
		if got := p.m.upTo(target, sel, len(sel.parts)-1); got != tt.want {
			t.Errorf("%q: got %v, want %v", tt.sel, got, tt.want)
		}
	}

	// Elements the HTML parser treats specially.
	for sel, want := range map[string]bool{
		`input[type=CHECKBOX]`:   true, // HTML: type compares case-insensitively
		`input[type=CHECKBOX s]`: false,
		`input[type=radio]`:      false,
		`svg[viewbox]`:           true, // the parser camel-cases it; a browser matches it too
		`foreignobject .in-svg`:  true, // a superset: a browser would want foreignObject
		`foreignObject > i`:      true,
		`html[lang=EN]`:          true,
		`:root > body`:           true,
		`:root:has(.row)`:        true,
		`:root:has(.gone)`:       false,
		`:root:not(:has(.gone))`: true,
		`body:root`:              false,
		`.list > .row ~ em`:      true,
		`.list > em ~ .row`:      false,
		`.list > .row + em`:      true,
		`.list:has(.row)`:        true,
		`.list:not(:has(.row))`:  false,
	} {
		s, err := parseSelector(sel, false)
		if err != nil {
			t.Errorf("%q: %v", sel, err)
		} else if got := p.m.may(s); got != want {
			t.Errorf("may(%q) = %v", sel, got)
		}
	}
}
