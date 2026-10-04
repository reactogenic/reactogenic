package cssprune

import (
	"fmt"
	"strings"
	"testing"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
)

// lower is what the driver feeds Prune: esbuild's output with nesting
// lowered (builder.md, CSS), minified or printed.
func lower(t testing.TB, css string, minify bool) string {
	t.Helper()
	r := api.Transform(css, api.TransformOptions{
		Loader: api.LoaderCSS, MinifyWhitespace: minify, MinifySyntax: minify,
		Supported: map[string]bool{"nesting": false},
	})
	if len(r.Errors) > 0 {
		t.Fatalf("esbuild: %v", r.Errors)
	}
	return strings.TrimSpace(string(r.Code))
}

func mustPrune(t testing.TB, css string, doc *html.Node) (string, Stats) {
	t.Helper()
	out, stats, err := Prune(css, doc)
	if err != nil {
		t.Fatal(err)
	}
	return out, stats
}

// TestPrune: the table of builder.md, CSS, row by row, and the traps of
// plan.md, RGP2-020 (research/css.md, Verification). css is flat, as esbuild
// prints it; nested is authored CSS, lowered by esbuild in the test.
func TestPrune(t *testing.T) {
	tests := []struct {
		name   string
		css    string
		nested string
		page   string
		want   string
	}{
		// ---- a selector; a selector list ----
		{
			name: "a rule stays only if an element may match",
			css:  `.a{x:y}.gone{x:y}p{x:y}div{x:y}#i{x:y}#gone{x:y}[data-v=ghost]{x:y}[data-v=solid]{x:y}.a>b{x:y}.a>i{x:y}`,
			page: `<p class="a" id="i" data-v="ghost"><b></b></p>`,
			want: `.a{x:y}p{x:y}#i{x:y}[data-v=ghost]{x:y}.a>b{x:y}`,
		},
		{
			name: "a selector list is pruned per selector",
			css:  `.a,.gone{x:y}.gone,.a:hover,.gone2{x:y}.gone,.gone2{x:y}.a,.gone:nth-child(2n+1){x:y}`,
			page: `<p class="a"></p>`,
			want: `.a{x:y}.a:hover{x:y}.a{x:y}`,
		},
		{
			// One selector a browser rejects kills the rule there. Trimming
			// the list around it would bring the rule to life.
			name: "a list is kept whole when a selector to drop may be invalid somewhere",
			css:  `.a,.gone::-webkit-scrollbar{x:y}.a,.gone:open{x:y}.a,.gone:nth-child(2 of .x){x:y}.a,[data-v=x s]{x:y}.gone::-moz-x,.gone2{x:y}`,
			page: `<p class="a"></p>`,
			want: `.a,.gone::-webkit-scrollbar{x:y}.a,.gone:open{x:y}.a,.gone:nth-child(2 of .x){x:y}.a,[data-v=x s]{x:y}`,
		},
		{
			// Chromium and WebKit reject almost everything that follows a
			// pseudo-element — a pseudo-class, a second pseudo-element, a
			// combinator — and esbuild prints it without a warning.
			name: "a list is kept whole when a selector to drop goes on after a pseudo-element",
			css: `.a,.gone::before:hover{x:y}.a,.gone:after:focus{x:y}.a,.gone::before::marker{x:y}.a,.gone::backdrop:first-child{x:y}` +
				`.a,.gone::selection:root{x:y}.a,.gone::before > .b{x:y}.a,.gone::before .b{x:y}.gone::before > .b{x:y}` +
				`.gone::before:hover,.gone2{x:y}.a::before:hover,.gone{x:y}`,
			page: `<p class="a"></p>`,
			want: `.a,.gone::before:hover{x:y}.a,.gone:after:focus{x:y}.a,.gone::before::marker{x:y}.a,.gone::backdrop:first-child{x:y}` +
				`.a,.gone::selection:root{x:y}.a,.gone::before > .b{x:y}.a,.gone::before .b{x:y}.gone::before > .b{x:y}` +
				`.a::before:hover{x:y}`,
		},
		{
			name:   "…as esbuild prints it",
			nested: `.Other::after:hover, .Btn { color: red } .Btn { .Other::before:hover, & { color: blue } }`,
			page:   `<button class="Btn"></button>`,
			want:   `.Other:after:hover,.Btn{color:red}.Btn .Other:before:hover,.Btn{color:#00f}`,
		},
		{
			// What does not parse as a selector may not be one. Trimming
			// `.gone` off the last rule would make a real @font-face of what
			// a browser reads as one rule with a bad selector.
			name: "a list with a selector that does not parse is kept as it is",
			css:  `.a,svg|a{x:y}.gone , svg|a{x:y}.gone,a||b,.gone2{x:y}.gone,@font-face{font-family:x}`,
			page: `<p class="a"></p>`,
			want: `.a,svg|a{x:y}.gone , svg|a{x:y}.gone,a||b,.gone2{x:y}.gone,@font-face{font-family:x}`,
		},
		{
			name: "runtime state is maybe: pseudo-classes and the attributes behaviours write",
			css: `.a:hover{x:y}.a:focus-visible{x:y}.a:popover-open{x:y}.a[open]{x:y}.a[hidden]{x:y}.a[inert]{x:y}.a[disabled]{x:y}` +
				`.a[aria-expanded=true]{x:y}.a[data-state=open]{x:y}.a[style*=color]{x:y}.a[value=""]{x:y}.a:first-child{x:y}` +
				`.gone:hover{x:y}.gone[open]{x:y}.a[data-other=open]{x:y}.a[title]{x:y}:focus-visible{x:y}`,
			page: `<p class="a"></p>`,
			want: `.a:hover{x:y}.a:focus-visible{x:y}.a:popover-open{x:y}.a[open]{x:y}.a[hidden]{x:y}.a[inert]{x:y}.a[disabled]{x:y}` +
				`.a[aria-expanded=true]{x:y}.a[data-state=open]{x:y}.a[style*=color]{x:y}.a[value=""]{x:y}.a:first-child{x:y}:focus-visible{x:y}`,
		},
		{
			name: "pseudo-elements are ignored for matching",
			css:  `.a::before{x:y}.gone::after{x:y}::selection{x:y}.dlg::backdrop{x:y}.gone::backdrop{x:y}.a::first-line{x:y}.a:after{x:y}.gone:before{x:y}details::details-content{x:y}`,
			page: `<p class="a"></p><dialog class="dlg"></dialog>`,
			want: `.a::before{x:y}::selection{x:y}.dlg::backdrop{x:y}.a::first-line{x:y}.a:after{x:y}`,
		},
		{
			name: ":root is the document element, not a state",
			css:  `:root{x:y}:root .a{x:y}:root>.a{x:y}div:root{x:y}:root:has(.a){x:y}:root:has(.gone){x:y}:root:has(.a:modal){x:y}.a :root{x:y}:not(:root){x:y}:root:not(html){x:y}`,
			page: `<div class="a"></div>`,
			want: `:root{x:y}:root .a{x:y}:root:has(.a){x:y}:root:has(.a:modal){x:y}:not(:root){x:y}`,
		},
		{
			name: "the negation of maybe is maybe",
			css: `.a:not(:hover){x:y}.a:not([hidden]){x:y}.a:not(.b){x:y}.a:not(.b:hover){x:y}.a:not(.c){x:y}:not(.a,.c,html,head,body){x:y}` +
				`.c:not(.a){x:y}.a:not(:not(.b)){x:y}.a:not(:not(:hover)){x:y}.a:not(p){x:y}`,
			page: `<p class="a b"></p><p class="c"></p>`,
			want: `.a:not(:hover){x:y}.a:not([hidden]){x:y}.a:not(.b:hover){x:y}.a:not(.c){x:y}.c:not(.a){x:y}.a:not(:not(.b)){x:y}.a:not(:not(:hover)){x:y}`,
		},
		{
			name: ":has(), + and ~ next to maybe",
			css: `.list:has(.msg:hover){x:y}.list:has(.row){x:y}.list:has(>.foot[hidden]){x:y}.list:not(:has(.msg:focus)) .foot{x:y}` +
				`.list:not(:has(.msg)) .foot{x:y}.d[open]+.after{x:y}.d:not([open])+.after{x:y}.d[open]~.after{x:y}.d[open]+.list{x:y}` +
				`.msg:hover~.foot{x:y}.foot:hover~.msg{x:y}.d:has(+.after:hover){x:y}.d:has(~.gone:hover){x:y}`,
			page: `<div class="list"><p class="msg">none</p><div class="foot"></div></div><details class="d"><summary>s</summary></details><div class="after"></div>`,
			want: `.list:has(.msg:hover){x:y}.list:has(>.foot[hidden]){x:y}.list:not(:has(.msg:focus)) .foot{x:y}` +
				`.d[open]+.after{x:y}.d:not([open])+.after{x:y}.d[open]~.after{x:y}.msg:hover~.foot{x:y}.d:has(+.after:hover){x:y}`,
		},
		{
			// research/css.md: the prototype read `:is()` under a nested rule
			// as a descendant of the parent, and dropped `.A:is(.x,.y)`.
			name: "what lowered nesting becomes",
			nested: `.A { color: black; &:is(.x, .y) { color: red } &:is(.p, .q) { color: blue } }
				.B { :is(&.x, .q) { color: red } }
				.C, .D { & .e { color: red } > .f { color: blue } }
				.T { &[data-on] { color: red } &[data-off] { color: blue } }
				.E .F, #G { .h { color: red } }
				.H .I, #J { .h { color: red } }
				ul { li& { color: red } }
				.L::before { &:hover { color: red } }
				.Q { :not(&) { color: red } }`,
			page: `<div class="A x"></div><div class="B x"></div><div class="D"><i class="e"></i></div><div class="T" data-on></div><p id="G"><b class="h"></b></p><div class="Q"></div>`,
			want: `.A{color:#000}.A:is(.x,.y){color:red}:is(.B.x,.q){color:red}:is(.C,.D) .e{color:red}.T[data-on]{color:red}:is(.E .F,#G) .h{color:red}:not(.Q){color:red}`,
		},
		{
			name: "the i flag, and the attribute values HTML compares case-insensitively",
			css:  `[data-v=Primary i]{x:y}[data-v=Primary]{x:y}input[type=CHECKBOX]{x:y}input[type=RADIO]{x:y}[data-v=PRIMARY s]{x:y}a[rel=NoFollow]{x:y}a[href="/A"]{x:y}`,
			page: `<div data-v="primary"></div><input type="checkbox"><a rel="nofollow" href="/a"></a>`,
			want: `[data-v=Primary i]{x:y}input[type=CHECKBOX]{x:y}a[rel=NoFollow]{x:y}`,
		},
		{
			name: "tags and attribute names fold; classes and ids do not",
			css:  `DIV{x:y}Button{x:y}[POPOVERTARGET]{x:y}.card{x:y}.Card{x:y}#Main{x:y}#main{x:y}`,
			page: `<div class="Card" id="main"><button popoverTarget="m"></button></div>`,
			want: `DIV{x:y}Button{x:y}[POPOVERTARGET]{x:y}.Card{x:y}#main{x:y}`,
		},
		{
			name: "strings, urls and escapes are not structure",
			css:  `.gone{x:y}.s{background:url(data:image/svg+xml;charset=utf8,%3Csvg{}%3E) no-repeat,url(a;b.png);content:'}"{'}.sm\:flex,.\31 0,#\#x,.gone\,.s{x:y}.t{x:y}`,
			page: `<p class="s sm:flex"></p><p class="t"></p>`,
			want: `.s{background:url(data:image/svg+xml;charset=utf8,%3Csvg{}%3E) no-repeat,url(a;b.png);content:'}"{'}.sm\:flex{x:y}.t{x:y}`,
		},

		// ---- conditional at-rules ----
		{
			name: "at-rules are pruned inside and dropped when empty",
			css:  `@supports (x:y){.gone{x:y}}@container (width>1px){.gone{x:y}}@scope (.a){.gone{x:y}}@starting-style{.gone{x:y}}@media print{.gone{x:y}}.a{x:y}`,
			page: `<p class="a"></p>`,
			want: `.a{x:y}`,
		},
		{
			name: "nested at-rules",
			css: `@media (width>=1px){@supports (display:grid){.gone{x:y}}}` +
				`@media (width>=2px){@supports (display:grid){.a{x:y}.gone{x:y}}.gone{x:y}@container c (width>1px){.gone{x:y}}}` +
				`@starting-style{.a[open]{opacity:0}.gone[open]{opacity:0}}`,
			page: `<p class="a"></p>`,
			want: `@media (width>=2px){@supports (display:grid){.a{x:y}}}@starting-style{.a[open]{opacity:0}}`,
		},
		{
			// esbuild leaves @scope as it is: relative selectors, `&`, and
			// nested rules survive in it.
			name:   "@scope: relative selectors, and nesting esbuild does not lower",
			nested: `@scope (.a) to (.b) { .a { x: y } :scope { x: y } > p { x: y } > .gone { x: y } & > p { y: z } .gone { x: y } .q { .r { x: y } } }`,
			page:   `<div class="a"><p></p></div>`,
			want:   `@scope(.a)to (.b){.a{x:y}:scope{x:y}>p{x:y}>p{y:z}.q{.r{x:y}}}`,
		},
		{
			// A declaration between rules ends at its `;`, which must stay:
			// without it the next rule would be read into the value.
			name: "@scope: declarations between the rules",
			css:  `@scope (.a){color:red;.gone{x:y}--x: {a:b};.a{x:y}--y: 1;.gone{x:y}margin:0}:root{--x: 1;--z: 2}`,
			page: `<div class="a"></div>`,
			want: `@scope (.a){color:red;--x: {a:b};.a{x:y}--y: 1;margin:0;}:root{--x: 1}`,
		},

		// ---- @layer ----
		{
			// research/css.md: without the statement the layer order flips,
			// and `.A` goes from red to blue.
			name: "an emptied @layer block stays as a statement",
			css:  `@layer reset{.gone{margin:0}}@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`,
			page: `<div class="A"></div>`,
			want: `@layer reset;@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`,
		},
		{
			name: "…unless an earlier statement already orders it",
			css:  `@layer reset,components;@layer reset{.gone{margin:0}}@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`,
			page: `<div class="A"></div>`,
			want: `@layer reset,components;@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`,
		},
		{
			name: "…or an earlier block that is kept",
			css:  `@layer a{.A{x:y}}@layer b{.A{x:y}}@layer a{.gone{x:y}}@layer c{.gone{x:y}}@layer c{.gone{x:y}}`,
			page: `<div class="A"></div>`,
			want: `@layer a{.A{x:y}}@layer b{.A{x:y}}@layer c;`,
		},
		{
			name: "nested and dotted layer names",
			css:  `@layer a{@layer b{.gone{x:y}}}@layer a.c{.A{x:y}}@layer a.b{.A{x:z}}@layer d.e{.gone{x:y}}@layer d{.gone{x:y}}@layer d{@layer e{.gone{x:y}}}`,
			page: `<div class="A"></div>`,
			want: `@layer a{@layer b;}@layer a.c{.A{x:y}}@layer a.b{.A{x:z}}@layer d.e;`,
		},
		{
			// What a conditional at-rule declares is ordered only when the
			// condition holds: it does not count outside, and the at-rule
			// must stay to carry the statement.
			name: "a layer inside a conditional at-rule",
			css: `@media print{@layer p{.gone{x:y}}}@layer q{.A{x:y}}@layer p{.A{x:y}}` +
				`@layer r;@media print{@layer r{.gone{x:y}}}` +
				`@media print{@layer s{.A{x:y}}}@layer s{.gone{x:y}}@layer t{.A{x:y}}`,
			page: `<div class="A"></div>`,
			want: `@media print{@layer p;}@layer q{.A{x:y}}@layer p{.A{x:y}}@layer r;@media print{@layer s{.A{x:y}}}@layer s;@layer t{.A{x:y}}`,
		},
		{
			name: "an anonymous layer orders nothing once it is empty",
			css:  `@layer{.gone{x:y}}@layer{@layer z{.gone{x:y}}}@layer{.A{x:y}}`,
			page: `<div class="A"></div>`,
			want: `@layer{.A{x:y}}`,
		},
		{
			name: "a layer emptied by its custom properties",
			css:  `@layer t{:root{--unused: 1}}@layer u{.A{x:y}}@layer t{.A{y:z}}`,
			page: `<div class="A"></div>`,
			want: `@layer t;@layer u{.A{x:y}}@layer t{.A{y:z}}`,
		},
		{
			// A browser ignores a @layer rule whose prelude it cannot read: it
			// orders nothing, and a statement in its place would.
			name: "a @layer block that names no single layer is kept as it is",
			css:  `@layer a, b{.gone{x:y}}@layer c{.A{color:red}}@layer b{.A{color:#00f}}@layer 1{.gone{x:y}}@layer a . b{.gone{x:y}}@layer initial{.gone{x:y}}`,
			page: `<div class="A"></div>`,
			want: `@layer a, b{.gone{x:y}}@layer c{.A{color:red}}@layer b{.A{color:#00f}}@layer 1{.gone{x:y}}@layer a . b{.gone{x:y}}@layer initial{.gone{x:y}}`,
		},
		{
			name: "…and a statement with a name that is not one orders nothing",
			css:  `@layer a,1;@layer a{.gone{x:y}}@layer c{.A{x:y}}@layer a{.A{y:z}}@layer d,e;@layer d{.gone{x:y}}`,
			page: `<div class="A"></div>`,
			want: `@layer a,1;@layer a;@layer c{.A{x:y}}@layer a{.A{y:z}}@layer d,e;`,
		},

		// ---- @import and @namespace out of place ----
		{
			// A browser ignores an @import or a @namespace that follows a rule,
			// but not one that follows `@layer a;`: the statement would make
			// the default namespace SVG's, and `p` match nothing.
			name: "an emptied @layer block before a @namespace is not made a statement",
			css:  `@layer a{.gone{x:y}}@namespace url(http://www.w3.org/2000/svg);p{color:red}.gone{x:y}@layer a{.gone{x:y}}`,
			page: `<p class="A"></p>`,
			want: `@layer a{.gone{x:y}}@namespace url(http://www.w3.org/2000/svg);p{color:red}`,
		},
		{
			name: "nothing is dropped before an @import that is out of place",
			css: `:root{--unused: 1}.gone{x:y}@media print{.gone{x:y}}@keyframes k{to{x:y}}@layer l{.gone{x:y}}` +
				`@import url(x.css);.gone{x:y}@layer l{.gone{x:y}}@layer m{.gone{x:y}}.A{x:y}`,
			page: `<p class="A"></p>`,
			want: `:root{--unused: 1}.gone{x:y}@media print{.gone{x:y}}@keyframes k{to{x:y}}@layer l{.gone{x:y}}` +
				`@import url(x.css);@layer m;.A{x:y}`,
		},
		{
			name: "…and a sheet that starts as it should is pruned as any other",
			css: `@charset "UTF-8";/*! legal */@layer a,b;@import url(x.css);@layer c;@namespace svg url(http://www.w3.org/2000/svg);` +
				`.gone{x:y}@layer a{.gone{x:y}}@layer d{.gone{x:y}}.A{x:y}`,
			page: `<p class="A"></p>`,
			want: `@charset "UTF-8";/*! legal */@layer a,b;@import url(x.css);@layer c;@namespace svg url(http://www.w3.org/2000/svg);@layer d;.A{x:y}`,
		},

		// ---- @keyframes ----
		{
			// research/css.md: the prototype looked for the name among
			// identifier tokens of `animation-name`, and dropped `spin`.
			name: "a referenced @keyframes stays",
			css: `@keyframes spin{to{rotate:360deg}}@keyframes fade{to{opacity:0}}@keyframes "quoted"{to{opacity:0}}` +
				`@keyframes viaprop{to{opacity:0}}@keyframes attr{to{opacity:0}}@keyframes zip{to{opacity:0}}@-webkit-keyframes old{to{opacity:0}}` +
				`@keyframes linear{to{opacity:0}}@keyframes sp\69n2{to{opacity:0}}` +
				`.Button[aria-busy=true]::after{animation:spin 1s linear infinite}.Button{animation-name:"quoted",spin2;--a: viaprop;-webkit-animation:old 1s}` +
				`.Button:hover{animation:var(--a) 1s}.gone{animation:zip 1s}.Button:active{transition:fade 1s}`,
			page: `<button class="Button" style="animation: attr 2s">go</button>`,
			want: `@keyframes spin{to{rotate:360deg}}@keyframes "quoted"{to{opacity:0}}` +
				`@keyframes viaprop{to{opacity:0}}@keyframes attr{to{opacity:0}}@-webkit-keyframes old{to{opacity:0}}` +
				`@keyframes linear{to{opacity:0}}@keyframes sp\69n2{to{opacity:0}}` +
				`.Button[aria-busy=true]::after{animation:spin 1s linear infinite}.Button{animation-name:"quoted",spin2;--a: viaprop;-webkit-animation:old 1s}` +
				`.Button:hover{animation:var(--a) 1s}.Button:active{transition:fade 1s}`,
		},
		{
			name: "@keyframes inside an at-rule; a name only a dropped rule used",
			css:  `@media (prefers-reduced-motion:no-preference){@keyframes in{to{opacity:1}}@keyframes out{to{opacity:0}}.a{animation:in 1s}.gone{animation:out 1s}}@layer m{@keyframes out2{to{opacity:0}}}`,
			page: `<p class="a"></p>`,
			want: `@media (prefers-reduced-motion:no-preference){@keyframes in{to{opacity:1}}.a{animation:in 1s}}@layer m;`,
		},

		// ---- @position-try ----
		{
			// The design system's menu: a page without one kept its fallback.
			name: "a @position-try stays only if something names it",
			css: `@position-try --edge{left:.5rem;right:auto}@position-try --viaprop{top:0}@position-try --attr{top:0}@position-try --styled{top:0}` +
				`@position-try --gone{left:var(--only-here)}@position-try --unnamed{top:0}@position-try --esc\61ped{top:0}@position-try --Case{top:0}` +
				`:root{--only-here: 1px;--fallback: --viaprop}` +
				`.menu{position-try-fallbacks:flip-block,--edge}.menu:hover{position-try:var(--fallback)}.menu:focus{position-try-fallbacks:--escaped,--case}` +
				`.gone{position-try-fallbacks:--gone}`,
			page: `<div class="menu" style="position-try-fallbacks: --attr"></div><style>.y{position-try: --styled}</style>`,
			want: `@position-try --edge{left:.5rem;right:auto}@position-try --viaprop{top:0}@position-try --attr{top:0}@position-try --styled{top:0}` +
				`@position-try --esc\61ped{top:0}` +
				`:root{--fallback: --viaprop}` +
				`.menu{position-try-fallbacks:flip-block,--edge}.menu:hover{position-try:var(--fallback)}.menu:focus{position-try-fallbacks:--escaped,--case}`,
		},
		{
			name: "@position-try inside an at-rule; one whose prelude is no name is kept as it is",
			css:  `@layer c{@position-try --in{top:0}.gone{position-try-fallbacks:--in}}@media print{@position-try --p{top:0}}@position-try edge{top:0}@position-try --a,--b{top:0}@position-try{top:0}`,
			page: `<p class="a"></p>`,
			want: `@layer c;@position-try edge{top:0}@position-try --a,--b{top:0}@position-try{top:0}`,
		},

		// ---- custom properties ----
		{
			name: "a custom property nothing reads is dropped, to a fixed point",
			css: `:root{--a: var(--b);--b: 1px;--c: var(--d);--d: 2px;--e: 3px;--self: var(--self);--attr: 4px;--svg: red;--styled: 5px;--kf: 6px}` +
				`.x{width:var(--a)}.gone{height:var(--c)}@media (width>1px){:root{--e: 9px;--d: 8px}}@keyframes k{to{width:var(--kf)}}.x:hover{animation:k 1s}`,
			page: `<div class="x" style="margin: var(--attr)"><svg><rect fill="var(--svg)"/></svg></div><style>.y{padding:var(--styled)}</style>`,
			want: `:root{--a: var(--b);--b: 1px;--attr: 4px;--svg: red;--styled: 5px;--kf: 6px}.x{width:var(--a)}@keyframes k{to{width:var(--kf)}}.x:hover{animation:k 1s}`,
		},
		{
			// research/css.md: the prototype dropped --theme and kept the query.
			name: "a custom property read only by @container style()",
			css:  `:root{--theme: dark;--used: 1px;--unused: 2px}@container style(--theme: dark){.A{color:#fff}}.A{padding:var(--used)}`,
			page: `<div class="A"></div>`,
			want: `:root{--theme: dark;--used: 1px}@container style(--theme: dark){.A{color:#fff}}.A{padding:var(--used)}`,
		},
		{
			name: "…and not kept by a query that was emptied",
			css:  `:root{--theme: dark}@container style(--theme: dark){.gone{color:#fff}}.A{color:red}`,
			page: `<div class="A"></div>`,
			want: `.A{color:red}`,
		},
		{
			name: "names in other places keep a custom property: @property, a dashed ident, a fallback",
			css:  `:root{--reg: 1px;--anchor: 2px;--fb: 3px;--dead: 4px}@property --reg{syntax:"<length>";inherits:false;initial-value:0}.a{anchor-name:--anchor;width:var(--none,var(--fb));transition:--reg 1s}`,
			page: `<p class="a"></p>`,
			want: `:root{--reg: 1px;--anchor: 2px;--fb: 3px}@property --reg{syntax:"<length>";inherits:false;initial-value:0}.a{anchor-name:--anchor;width:var(--none,var(--fb));transition:--reg 1s}`,
		},
		{
			name: "a custom property that only a dropped custom property read",
			css:  `.a{--x: var(--y);--y: var(--z);--z: 1px;--w: {a:b};color:red}`,
			page: `<p class="a"></p>`,
			want: `.a{color:red}`,
		},

		// ---- everything else is kept ----
		{
			name: "at-rules the pruner does not know are kept as they are",
			css: `@charset "UTF-8";@import url(x.css) layer(l);@namespace svg url(http://www.w3.org/2000/svg);@font-face{font-family:X;src:url(x.woff2)}` +
				`@property --x{syntax: "<length>"; inherits: false; initial-value: 0px}@page{margin:1cm;@top-left{content:"x"}}` +
				`@counter-style c{system:cyclic;symbols:"*"}@position-try --t{inset:auto}@view-transition{navigation:auto}` +
				`@unknown foo{bar {baz: 1}}@unknown2 foo;/*! legal */.gone{x:y}`,
			// A @position-try is known by now (above): this one stays
			// because the page names it.
			page: `<p style="position-try-fallbacks: --t"></p>`,
			want: `@charset "UTF-8";@import url(x.css) layer(l);@namespace svg url(http://www.w3.org/2000/svg);@font-face{font-family:X;src:url(x.woff2)}` +
				`@property --x{syntax: "<length>"; inherits: false; initial-value: 0px}@page{margin:1cm;@top-left{content:"x"}}` +
				`@counter-style c{system:cyclic;symbols:"*"}@position-try --t{inset:auto}@view-transition{navigation:auto}` +
				`@unknown foo{bar {baz: 1}}@unknown2 foo;/*! legal */`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			css := tt.css
			if tt.nested != "" {
				css = lower(t, tt.nested, true)
			}
			doc := parsePage(t, tt.page)
			out, _ := mustPrune(t, css, doc)
			if out != tt.want {
				t.Errorf("\n css: %s\n got: %s\nwant: %s", css, out, tt.want)
			}
			checkPruned(t, css, out, doc)

			// The same sheet as esbuild prints it without minifying prunes
			// to the same rules.
			if tt.nested != "" {
				printed, _ := mustPrune(t, lower(t, tt.nested, false), doc)
				if got := lower(t, printed, true); got != lower(t, out, true) {
					t.Errorf("printed input:\n got: %s\nwant: %s", got, lower(t, out, true))
				}
			}
		})
	}
}

// checkPruned asserts what holds for every Prune: pruning again changes
// nothing, a kept rule keeps its declarations as they were written, and
// esbuild reads the output.
func checkPruned(t testing.TB, css, out string, doc *html.Node) {
	t.Helper()
	checkPrunedWith(t, css, out, doc, Options{})
}

// checkPrunedWith is checkPruned for a page with what the builder made for
// it: its script, the elements packaging wrote.
func checkPrunedWith(t testing.TB, css, out string, doc *html.Node, opts Options) {
	t.Helper()
	again, stats, err := PruneWith(out, doc, opts)
	if err != nil {
		t.Fatalf("the output does not read: %v\n%s", err, out)
	}
	if again != out {
		t.Errorf("not idempotent:\n once: %s\ntwice: %s", out, again)
	}
	if stats.RulesDropped+stats.SelectorsDropped+stats.Properties+stats.Keyframes+stats.PositionTries != 0 {
		t.Errorf("pruning again dropped something: %+v", stats)
	}
	sameDeclarations(t, css, out)
	if r := api.Transform(out, api.TransformOptions{Loader: api.LoaderCSS}); len(r.Errors) > 0 {
		t.Errorf("esbuild rejects the output: %v", r.Errors)
	}
}

// styleRules lists the style rules of a sheet, in order, those inside
// at-rules included.
func styleRules(rules []*rule, into []*rule) []*rule {
	for _, r := range rules {
		switch r.kind {
		case kStyle:
			into = append(into, r)
		case kGroup, kLayer:
			into = styleRules(r.rules, into)
		}
	}
	return into
}

// sameDeclarations: every rule of out is a rule of css, in order, with a
// sub-list of its selectors and its declarations byte for byte — but for
// custom properties, which may be gone.
func sameDeclarations(t testing.TB, css, out string) {
	t.Helper()
	in, err := parseRules(css, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	pruned, err := parseRules(out, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	from := styleRules(in, nil)
	i := 0
next:
	for _, r := range styleRules(pruned, nil) {
		for ; i < len(from); i++ {
			if derives(r, from[i]) {
				i++
				continue next
			}
		}
		t.Errorf("a rule of the output is not a rule of the input: %s", clip(r.raw))
		return
	}
}

func derives(out, in *rule) bool {
	if out.opaque || in.opaque {
		return out.raw == in.head+"{"+in.body+"}"
	}
	a, _ := splitList(out.head)
	b, _ := splitList(in.head)
	j := 0
	for _, s := range a {
		for j < len(b) && b[j] != s {
			j++
		}
		if j == len(b) {
			return false
		}
		j++
	}
	j = 0
	for _, d := range in.decls {
		if j < len(out.decls) && out.decls[j].text == d.text {
			j++
		} else if !d.custom {
			return false
		}
	}
	return j == len(out.decls)
}

func TestPruneTemplate(t *testing.T) {
	const css = `.a{x:y}.gone{x:y}:root{--unused: 1}@keyframes k{to{x:y}}`
	// The content of a <template> is cloned somewhere at run time: what it
	// matches, and what matches because of it (`.list:has(.row)`), is not
	// known. Phase 2 emits none; a page that has one is not pruned. The
	// content of a <noscript> is elements in one browser and text in the
	// next, whichever way the page was parsed.
	noscript, err := html.ParseWithOptions(strings.NewReader(`<p class="a"></p><noscript><p class="gone"></p></noscript>`), html.ParseOptionEnableScripting(false))
	if err != nil {
		t.Fatal(err)
	}
	const markup = "the page has markup in a <select>"
	for name, tt := range map[string]struct {
		doc     *html.Node
		because string
	}{
		"template":            {parsePage(t, `<p class="a"></p><template><div class="row"></div></template>`), "the page has a <template>"},
		"shadow root":         {parsePage(t, `<p class="a"></p><div><template shadowrootmode="open"><slot></slot></template></div>`), "the page has a <template>"},
		"noscript, as text":   {parsePage(t, `<head><noscript><link rel="stylesheet" href="x.css"></noscript></head><p class="a"></p>`), "the page has a <noscript>"},
		"noscript, as markup": {noscript, "the page has a <noscript>"},
		// The browser clones the selected option's content into it at load:
		// `selectedcontent .desc` and `button .desc` match there, and nothing
		// in the page as it was written says so.
		"selectedcontent": {parsePage(t, `<p class="a"></p><select><button><selectedcontent></selectedcontent></button><option><span class="gone">x</span>A</option></select>`), markup},
		// A parser from before the customizable select drops what a <select>
		// holds besides its options, and their end tags: there the option
		// is a child of the select, and the next <p> is not in a <div>.
		"select, markup around the options": {parsePage(t, `<p class="a"></p><select><div><option>A</option></div></select>`), markup},
		"select, markup in an option":       {parsePage(t, `<p class="a"></p><select><optgroup label="g"><option><b>A</b></option></optgroup></select>`), markup},
		"selectedcontent, on its own":       {parsePage(t, `<p class="a"></p><selectedcontent></selectedcontent>`), "the page has a <selectedcontent>"},
		// What the user's editing creates is of no script: Bold makes a <b>,
		// Enter a <div> — `.gone` may be the class of what was pasted.
		"contenteditable":                     {parsePage(t, `<p class="a"></p><div contenteditable>notes</div>`), "the page has an element the user edits (`contenteditable`)"},
		"contenteditable, as React writes":    {parsePage(t, `<p class="a"></p><div contenteditable="true">notes</div>`), "the page has an element the user edits (`contenteditable`)"},
		"contenteditable, plain text":         {parsePage(t, `<p class="a"></p><div ContentEditable="plaintext-only">notes</div>`), "the page has an element the user edits (`contenteditable`)"},
		"contenteditable, a value of its own": {parsePage(t, `<p class="a"></p><div contenteditable="yes">notes</div>`), "the page has an element the user edits (`contenteditable`)"},
	} {
		out, stats := mustPrune(t, css, tt.doc)
		if out != css || !stats.Unpruned || stats.Why != tt.because || stats.BytesOut != len(css) || stats.Rules != 3 || stats.Selectors != 3 || stats.RulesDropped != 0 {
			t.Errorf("%s: pruned: %q %+v", name, out, stats)
		}
	}
	// An element that says it is not edited is as any other.
	for name, page := range map[string]string{
		"contenteditable=false":   `<p class="a"></p><div contenteditable="false">notes</div>`,
		"another case, and space": `<p class="a"></p><div contenteditable=" FALSE ">notes</div>`,
		"a data attribute":        `<p class="a"></p><div data-contenteditable="true">notes</div>`,
	} {
		if out, stats := mustPrune(t, css, parsePage(t, page)); out != `.a{x:y}` || stats.Unpruned || stats.Why != "" {
			t.Errorf("%s: %q %+v", name, out, stats)
		}
	}
}

// TestWhole: a sheet the driver does not prune, counted as Prune counts one
// it gives back.
func TestWhole(t *testing.T) {
	const css = `.a{x:y}.gone,.b{x:y}@media print{:root{--unused: 1}}@keyframes k{to{x:y}}`
	stats, err := Whole(css)
	if want := (Stats{BytesIn: len(css), BytesOut: len(css), Rules: 3, Selectors: 4, Unpruned: true}); err != nil || fmt.Sprint(stats) != fmt.Sprint(want) {
		t.Errorf("%+v, %v; want %+v", stats, err, want)
	}
	if _, err := Whole(`.a{x:y`); err == nil {
		t.Error("CSS that does not read is counted")
	}
}

// TestPruneQuirks: without `<!doctype html>` a browser may be in quirks mode,
// where classes and ids match whatever their case. The builder writes the
// doctype; a page given without one is pruned for either mode.
func TestPruneQuirks(t *testing.T) {
	const css = `.foo{x:y}#bar{x:y}p.FOO#BAR{x:y}.gone{x:y}#gone{x:y}[class=foo]{x:y}[id=bar]{x:y}`
	const body = `<p class="Foo" id="Bar">t</p>`
	const folded = `.foo{x:y}#bar{x:y}p.FOO#BAR{x:y}`
	for _, tt := range []struct{ name, page, want string }{
		{"no doctype", body, folded},
		{"no doctype, a document", `<html><head><title>t</title></head><body>` + body, folded},
		{"a quirks doctype", `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">` + body, folded},
		{"a doctype that is not html", `<!doctype svg>` + body, folded},
		{"standards", `<!doctype html>` + body, ``},
		{"standards, as written by hand", "<!-- c -->\n<!DOCTYPE HTML>\n" + body, ``},
	} {
		doc, err := html.Parse(strings.NewReader(tt.page))
		if err != nil {
			t.Fatal(err)
		}
		if out, _ := mustPrune(t, css, doc); out != tt.want {
			t.Errorf("%s:\n got: %s\nwant: %s", tt.name, out, tt.want)
		}
	}
}

// TestPruneSelect: a <select> as it has always been written is pruned like
// anything else; so is an <option> that is not in one.
func TestPruneSelect(t *testing.T) {
	const css = `select>option{x:y}select>optgroup>option{x:y}select>hr{x:y}select>.gone{x:y}option:checked{x:y}datalist b{x:y}.gone{x:y}`
	doc := parsePage(t, `<select><option>A</option><optgroup label="g"><option>B</option></optgroup><hr><option>C</option><script></script></select>`+
		`<datalist><option><b>D</b></option></datalist>`)
	out, stats := mustPrune(t, css, doc)
	if want := `select>option{x:y}select>optgroup>option{x:y}select>hr{x:y}option:checked{x:y}datalist b{x:y}`; out != want || stats.Unpruned {
		t.Errorf("\n got: %s\nwant: %s\n%+v", out, want, stats)
	}
}

func TestPruneStats(t *testing.T) {
	const css = `/* ds/tokens.css */
:root {
  --a: 1px;
  --unused: 2px;
}
/* ds/button.css */
.Button,
.gone {
  width: var(--a);
}
.gone {
  color: red;
}
@media print {
  .gone2 {
    color: red;
  }
}
@keyframes spin {
  to {
    rotate: 360deg;
  }
}
/* ds/empty.css */
/*! legal */
`
	out, stats := mustPrune(t, css, parsePage(t, `<button class="Button"></button>`))
	const want = `/* ds/tokens.css */:root{--a: 1px}/* ds/button.css */.Button{width: var(--a)}/* ds/empty.css *//*! legal */`
	if out != want {
		t.Errorf("\n got: %s\nwant: %s", out, want)
	}
	wantStats := Stats{
		BytesIn: len(css), BytesOut: len(want),
		Rules: 4, RulesDropped: 2, Selectors: 5, SelectorsDropped: 3, Properties: 1, Keyframes: 1,
		// A file's bytes are its rules': not the comment that names it,
		// which is as long as the path to the file on this machine.
		Sources: []Source{
			{Name: "ds/tokens.css", BytesIn: 57 - len("/* ds/tokens.css */"), BytesOut: 34 - len("/* ds/tokens.css */"), Rules: 1},
			{Name: "ds/button.css", BytesIn: 176 - len("/* ds/button.css */"), BytesOut: 43 - len("/* ds/button.css */"), Rules: 3, RulesDropped: 2},
			{Name: "ds/empty.css", BytesIn: len("/*! legal */"), BytesOut: len("/*! legal */")},
		},
	}
	if got, want := sprint(stats), sprint(wantStats); got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}

	// A minified sheet names no sources.
	_, stats = mustPrune(t, `.a{x:y}/*! legal */.gone{x:y}`, parsePage(t, `<p class="a"></p>`))
	if stats.Sources != nil || stats.Rules != 2 || stats.RulesDropped != 1 {
		t.Errorf("%+v", stats)
	}
}

func sprint(s Stats) string {
	src := s.Sources
	s.Sources = nil
	out := fmt.Sprintf("%+v", s)
	for _, x := range src {
		out += fmt.Sprintf("\n  %+v", x)
	}
	return out
}

func TestPruneErrors(t *testing.T) {
	doc := parsePage(t, `<p></p>`)
	if out, _, err := Prune(`.a{color:red`, doc); err == nil || out != "" {
		t.Errorf("unbalanced CSS: %q, %v", out, err)
	}
	if _, _, err := Prune(`.a{}`, nil); err == nil {
		t.Error("no page: no error")
	}
	// An empty sheet, and a sheet for an empty page.
	if out, _ := mustPrune(t, ``, doc); out != "" {
		t.Errorf("%q", out)
	}
	if out, _ := mustPrune(t, `.a{x:y}*{x:y}`, &html.Node{Type: html.DocumentNode}); out != "" {
		t.Errorf("%q", out)
	}
}

// TestMinifyKeepsLayerStatements: the driver minifies the pruned sheet with
// esbuild again; the statement an emptied layer left must survive that.
func TestMinifyKeepsLayerStatements(t *testing.T) {
	out, _ := mustPrune(t, `@layer reset{.gone{margin:0}}@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`, parsePage(t, `<div class="A"></div>`))
	if got, want := lower(t, out, true), `@layer reset;@layer components{.A{color:red}}@layer reset{.A{color:#00f}}`; got != want {
		t.Errorf("esbuild: %s, want %s", got, want)
	}
}
