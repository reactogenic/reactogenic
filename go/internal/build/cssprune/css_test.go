package cssprune

import (
	"fmt"
	"strings"
	"testing"
)

// dump prints the tree the reader built, one rule per line.
func dump(rules []*rule, indent string, b *strings.Builder) {
	for _, r := range rules {
		switch r.kind {
		case kComment:
			fmt.Fprintf(b, "%scomment %s\n", indent, r.raw)
		case kStyle:
			if r.opaque {
				fmt.Fprintf(b, "%sstyle %s OPAQUE %s\n", indent, r.head, r.body)
				continue
			}
			var ds []string
			for _, d := range r.decls {
				ds = append(ds, d.name+"="+trim(d.value))
			}
			fmt.Fprintf(b, "%sstyle %s | %s\n", indent, r.head, strings.Join(ds, " | "))
		case kGroup, kLayer:
			fmt.Fprintf(b, "%s%s @%s [%s]\n", indent, map[kind]string{kGroup: "group", kLayer: "layer"}[r.kind], r.name, r.prelude)
			dump(r.rules, indent+"  ", b)
		case kKeyframes:
			fmt.Fprintf(b, "%skeyframes %s\n", indent, keyframesName(r.prelude))
		case kAt:
			fmt.Fprintf(b, "%sat @%s %s\n", indent, r.name, r.raw)
		case kRaw:
			fmt.Fprintf(b, "%sraw %s\n", indent, r.raw)
		}
	}
}

func TestReader(t *testing.T) {
	tests := []struct{ name, css, want string }{
		{"rules, minified", `.a{color:red}.b,.c{margin:0;padding:0}`, `
style .a | color=red
style .b,.c | margin=0 | padding=0`},
		{"rules, printed", ".a {\n  color: red;\n}\n.b,\n.c {\n  margin: 0;\n}\n", `
style .a | color=red
style .b,
.c | margin=0`},
		{"at-rules with blocks", `@media (width>=50rem){.a{color:red}@supports (display:grid){.b{color:blue}}}@starting-style{.c{opacity:0}}`, `
group @media [(width>=50rem)]
  style .a | color=red
  group @supports [(display:grid)]
    style .b | color=blue
group @starting-style []
  style .c | opacity=0`},
		{"at-rules with statements", `@charset "UTF-8";@import url(a;b.css) layer(x);@layer a,b;@namespace svg url(http://www.w3.org/2000/svg);@unknown2 foo`, `
at @charset @charset "UTF-8";
at @import @import url(a;b.css) layer(x);
at @layer @layer a,b;
at @namespace @namespace svg url(http://www.w3.org/2000/svg);
at @unknown2 @unknown2 foo;`},
		{"at-rules kept as they are", `@font-face{font-family:X;src:url(x.woff2)}@property --x{syntax: "<length>"; inherits: false}@unknown foo{bar {baz: 1}}@page{margin:1cm;@top-left{content:"x"}}`, `
at @font-face @font-face{font-family:X;src:url(x.woff2)}
at @property @property --x{syntax: "<length>"; inherits: false}
at @unknown @unknown foo{bar {baz: 1}}
at @page @page{margin:1cm;@top-left{content:"x"}}`},
		{"layers and keyframes", `@layer a{.x{color:red}}@layer{.z{color:red}}@keyframes spin{0%{rotate:0deg}to{rotate:360deg}}@-webkit-keyframes "s p"{to{opacity:1}}`, `
layer @layer [a]
  style .x | color=red
layer @layer []
  style .z | color=red
keyframes spin
keyframes s p`},
		// What esbuild prints for a data URL and a quoted `;`: neither the
		// `;` nor the braces end anything.
		{"strings and urls", `.s{background:url(data:image/svg+xml;charset=utf8,%3Csvg{}%3E) no-repeat,url(a;b.png);content:'}"{';quotes:"\"" "}"}.t{color:red}`, `
style .s | background=url(data:image/svg+xml;charset=utf8,%3Csvg{}%3E) no-repeat,url(a;b.png) | content='}"{' | quotes="\"" "}"
style .t | color=red`},
		{"quoted url, balanced brackets", `.s{background:url( "a)b" );grid-template-areas:"a b";width:calc((1px + 2px)*3);--j: { "a": [1, 2] };--k:{a}b{c};color:red}`, `
style .s | background=url( "a)b" ) | grid-template-areas="a b" | width=calc((1px + 2px)*3) | --j={ "a": [1, 2] } | --k={a}b{c} | color=red`},
		{"escapes", `.sm\:flex,.\31 0,#\#x,.a\,b,.c\{d{color:red}.e\ {margin:0}`, `
style .sm\:flex,.\31 0,#\#x,.a\,b,.c\{d | color=red
style .e\  | margin=0`},
		{"comments", "/* ds/a.css */\n.a{color:red}/*! legal */.b{/* in */color:blue}", `
comment /* ds/a.css */
style .a | color=red
comment /*! legal */
style .b | color=blue`},
		{"declarations", `.a{color:red!important;;--empty:;--x: 1px ;COLOR:Blue;junk;-webkit-animation:a 1s}`, `
style .a | color=red!important | --empty= | --x=1px | color=Blue | = | -webkit-animation=a 1s`},
		// esbuild does not lower nesting inside @scope.
		{"what survives lowering in @scope", `@scope(.A)to (.stop){.gone2{color:red}:scope{color:green}>img{color:#00f}.q{.r{color:red}}color:red;.s{color:red;@media (x){color:blue}}--x: {a:b};.t{color:red}margin:0}`, `
group @scope [(.A)to (.stop)]
  style .gone2 | color=red
  style :scope | color=green
  style >img | color=#00f
  style .q OPAQUE .r{color:red}
  raw color:red;
  style .s OPAQUE color:red;@media (x){color:blue}
  raw --x: {a:b};
  style .t | color=red
  raw margin:0;`},
		{"a semicolon at the top level is part of the prelude", `a;b{color:red}`, `
style a;b | color=red`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := parseRules(tt.css, false, 0)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			dump(rules, "", &b)
			if got, want := strings.TrimSpace(b.String()), strings.TrimSpace(tt.want); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestReaderErrors(t *testing.T) {
	for _, css := range []string{
		`.a{color:red`,             // unclosed block
		`.a{color:red}}`,           // unbalanced
		`.a{content:"x}`,           // unterminated string
		`.a{color:red}/* open`,     // unterminated comment
		`.a{background:url(x}`,     // unterminated url
		`.a{width:calc(1px}`,       // ( closed by }
		`.a`,                       // no block
		`@media (x){.a{color:red}`, // unclosed at-rule
		strings.Repeat("(", 1000) + strings.Repeat(")", 1000) + "{}", // too deep
	} {
		if _, err := parseRules(css, false, 0); err == nil {
			t.Errorf("%q: no error", clip(css))
		}
	}
}

func TestWords(t *testing.T) {
	tests := []struct{ in, want string }{
		{`spin 1s linear infinite`, `spin s linear infinite`},
		{`var(--a, var(--b)) calc(1px*var(--c))`, `var --a var --b calc px var --c`},
		{`"spin" 'a b'`, `spin|a b`},
		{`sp\69n --\66oo`, `spin --foo`},
		{`url(data:x;--not) url("--str") #fff`, `url|url|--str|fff`},
		{`/* --c */ --d`, `--d`},
		{`1--x -2px --theme: dark`, `--x -2px --theme dark`},
	}
	for _, tt := range tests {
		var got []string
		words(tt.in, func(w string) { got = append(got, w) })
		sep := " "
		if strings.Contains(tt.want, "|") {
			sep = "|"
		}
		if g := strings.Join(got, sep); g != tt.want {
			t.Errorf("words(%q) = %q, want %q", tt.in, g, tt.want)
		}
	}
}
