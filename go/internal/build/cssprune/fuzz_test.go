package cssprune

import (
	"os"
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// FuzzPrune: whatever the sheet and the page, Prune does not panic; it
// either refuses the CSS or returns CSS that reads again, that pruning again
// leaves as it is, and whose rules are rules of the input.
func FuzzPrune(f *testing.F) {
	for _, s := range corpus(f) {
		page, err := os.ReadFile(s.pages[0])
		if err != nil {
			f.Fatal(err)
		}
		f.Add(bundle(f, s.entry, true), string(page))
		f.Add(bundle(f, s.entry, false), string(page))
	}
	for _, css := range []string{
		``, `}`, `{`, `a{`, `a{}}`, `@`, `@;`, `@media`, `@media{`, `@layer;`, `@layer{}`, `@layer a.b,c;@layer a.b{}@layer c{@layer d{x{y:z}}}`,
		`a{b:url(}`, `a{b:"c}`, `a{b:c\`, `a\{b{c:d}`, `a{b:c;;d}`, `a,{b:c}`, `,a{b:c}`, `a{--x:{};--y:var(--x)}`, `a{--x:var(--x)}`,
		`.a\ {b:c}`, `.a\`, `/*`, `/**/a/**/{/**/b/**/:/**/c/**/}/**/`, `a{b:c}/*! x */d{e:f}`, `:is(:not(:has(:is(a)))){b:c}`,
		`@scope (a) to (b){>c{d:e}f{g{h:i}}j:k;l:m}`, `@keyframes "a b"{to{c:d}}e{animation:"a b" 1s}`, `@keyframes{}`, `@keyframes a`,
		`a;b{c:d}`, `@media x{a;b{c:d}}`, `@media x{;}`, `@media x{a{b:c}`, `a{b:c}@media x{d{e:f}}g{h:i}`, `[a="]"]{b:c}`, `[a=b i],[a=b s],[a|=b]{c:d}`,
		"a\n{\nb\n:\nc\n}\n", "\xef\xbb\xbfa{b:c}", "a{b:c}\x00", `a{b:(c;d);e:[f;g];h:{i;j}}`, `((((((((`, `a{b:c !important;d:e!important}`,
		`@container style(--a: b){c{d:e}}:root{--a: b;--f: g}`, `&{a:b}&&{c:d}a&{e:f}`, `a>b+c~d e{f:g}`, `:root:not(html),html:not(:root){a:b}`,
	} {
		f.Add(css, `<html class="a"><body><a b="c"><b></b><c></c><d></d><e></e></a><x style="y: var(--x)"></x></body></html>`)
	}
	f.Add(`a{b:c}`, `<template><a></a></template>`)
	f.Add(`a{b:c}`, `<noscript><a></a></noscript>`)
	f.Add(`a{b:c}`, ``)
	f.Fuzz(func(t *testing.T, css, page string) {
		doc, err := html.Parse(strings.NewReader(page))
		if err != nil {
			t.Skip()
		}
		out, stats, err := Prune(css, doc)
		if err != nil {
			if out != "" {
				t.Errorf("an error, and output: %q", out)
			}
			return
		}
		if stats.BytesIn != len(css) || stats.BytesOut != len(out) || stats.Unpruned && out != css {
			t.Errorf("stats do not describe the output: %+v", stats)
		}
		if stats.Unpruned {
			return
		}
		again, _, err := Prune(out, doc)
		if err != nil {
			t.Fatalf("the output does not read: %v\n in: %q\nout: %q", err, css, out)
		}
		if again != out {
			t.Fatalf("not idempotent:\n   in: %q\n once: %q\ntwice: %q", css, out, again)
		}
		sameDeclarations(t, css, out)
	})
}

// FuzzMatch: the soundness test on selectors nobody wrote. Whatever parses
// and is found to match nothing on the page matches nothing under cascadia
// either, once its runtime state is taken out.
func FuzzMatch(f *testing.F) {
	src, err := os.ReadFile("testdata/adversarial/selectors.html")
	if err != nil {
		f.Fatal(err)
	}
	doc := parsePage(f, string(src))
	for _, s := range corpus(f) {
		rules, err := parseRules(bundle(f, s.entry, true), false, 0)
		if err != nil {
			f.Fatal(err)
		}
		for _, r := range styleRules(rules, nil) {
			list, _ := splitList(r.head)
			for _, sel := range list {
				f.Add(sel)
			}
		}
	}
	f.Fuzz(func(t *testing.T, text string) {
		sel, err := parseSelector(text, false)
		if err != nil {
			return
		}
		var p pruner
		p.page(doc)
		if p.m.may(sel) {
			return
		}
		// The bridge to cascadia is a superset; where it is a loose one —
		// something widened beyond runtime state, a :not() whose argument
		// holds state and is left out whole — cascadia may match what
		// nothing can, and the answer proves nothing.
		alts, approx := static(sel)
		if approx || notOfMaybe(sel) {
			return
		}
		for _, alt := range alts {
			group, err := cascadia.ParseGroup(alt)
			if err != nil {
				continue
			}
			if el := cascadia.Query(doc, group); el != nil {
				t.Errorf("%q matches nothing, but cascadia finds <%s> for %q", text, el.Data, alt)
			}
		}
	})
}
