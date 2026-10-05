package cssprune

import (
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/markup"
)

// own is the driver's part for a page's own `<style>` elements: every one
// that is CSS and not the builder's — here, one that has text — with its
// text, which the tests write flat.
func own(doc *html.Node) (styles []*Style, builder []*html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" && markup.CSS(n) {
			if n.FirstChild == nil {
				builder = append(builder, n)
			} else {
				styles = append(styles, &Style{Node: n, CSS: n.FirstChild.Data})
			}
		}
		if n.Type == html.ElementNode && n.Data == "link" {
			if href, _ := markup.Attr(n, "href"); strings.HasPrefix(href, "/_rg/") {
				builder = append(builder, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return styles, builder
}

// TestPruneOwnStyles: a `<style>` of the page's own is CSS of the page
// (builder.md, *The builder's own elements*) — pruned as the sheet is, and
// with it: what one names another may define, and a layer is ordered where
// its name first occurs in the document. An empty `<style>` in these pages
// is the builder's: where the sheet stands.
func TestPruneOwnStyles(t *testing.T) {
	for _, tt := range []struct {
		name, css, page string
		script          string
		want            string   // the sheet
		styles          []string // each `<style>` of the page's own, in document order
		why             string
	}{
		{
			name: "pruned as the sheet is: a rule stays only if an element may match",
			css:  `.a{x:y}.gone{x:y}`,
			page: `<head><style>.a{color:red}.gone{color:red}p:hover{x:y}@media print{.gone{x:y}}</style><style></style></head><p class="a"></p>`,
			want: `.a{x:y}`, styles: []string{`.a{color:red}p:hover{x:y}`},
		},
		{
			name: "a page without a sheet of the builder's",
			css:  ``,
			page: `<head><style>.a{color:red}.gone{color:red}</style></head><p class="a"></p><style>.gone,p{x:y}</style>`,
			want: ``, styles: []string{`.a{color:red}`, `p{x:y}`},
		},
		{
			name:   "what the page's script names is maybe there too",
			css:    `.is-open{x:y}`,
			page:   `<head><style>.is-open .a{x:y}.gone .a{x:y}</style><style></style></head><p class="a"></p>`,
			script: `function m(e){e.classList.add("is-open")}`,
			want:   `.is-open{x:y}`, styles: []string{`.is-open .a{x:y}`},
		},
		{
			name: "a custom property the sheet declares and a <style> reads stays; one only a dropped rule reads goes",
			css:  `:root{--read:1;--unread:2;--by-gone:3}`,
			page: `<head><style></style></head><p class="a"></p><style>.a{color:var(--read)}.gone{color:var(--by-gone)}</style>`,
			want: `:root{--read:1}`, styles: []string{`.a{color:var(--read)}`},
		},
		{
			name: "… and the other way: a <style> declares, the sheet reads",
			css:  `.a{color:var(--mine)}`,
			page: `<head><style>:root{--mine:1;--nobody:2}</style><style></style></head><p class="a"></p>`,
			want: `.a{color:var(--mine)}`, styles: []string{`:root{--mine:1}`},
		},
		{
			name: "@keyframes and @position-try, named across the sheets",
			css:  `@keyframes spin{to{x:y}}@keyframes never{to{x:y}}.a{position-try-fallbacks:--mine}`,
			page: `<head><style></style></head><p class="a"></p><style>.a{animation:spin 1s}.gone{animation:never 1s}@position-try --mine{top:0}@position-try --nobody{top:0}</style>`,
			want: `@keyframes spin{to{x:y}}.a{position-try-fallbacks:--mine}`, styles: []string{`.a{animation:spin 1s}@position-try --mine{top:0}`},
		},
		{
			// The <style> in the head precedes the sheet: the statement its
			// emptied block leaves orders `reset`, and the sheet's own
			// emptied block of that layer has nothing left to say.
			name: "a layer is ordered where its name first occurs: a <style> before the sheet",
			css:  `@layer base{.a{x:y}}@layer reset{.gone{x:y}}`,
			page: `<head><style>@layer reset{.gone{x:y}}@layer base{.gone{x:y}}</style><style></style></head><p class="a"></p>`,
			want: `@layer base{.a{x:y}}`, styles: []string{`@layer reset;@layer base;`},
		},
		{
			// The <style> in the body follows the sheet: the sheet's
			// statement has ordered the layer, and the block goes.
			name: "… and one after it",
			css:  `@layer reset{.gone{x:y}}@layer base{.a{x:y}}`,
			page: `<head><style></style></head><p class="a"></p><style>@layer reset{.gone{x:y}}@layer late{.gone{x:y}}</style>`,
			want: `@layer reset;@layer base{.a{x:y}}`, styles: []string{`@layer late;`},
		},
		{
			// The builder's `<link>` is the sheet as well as its `<style>`:
			// here it is the first of the page's sheets.
			name: "the sheet as a file: its <link> is its place",
			css:  `@layer reset{.gone{x:y}}`,
			page: `<head><link rel="stylesheet" href="/_rg/page-1a2b3c4d.css"><style>@layer reset{.gone{x:y}}</style></head><p class="a"></p><style>@layer reset{.gone{x:y}}</style>`,
			want: `@layer reset;`, styles: []string{``, ``},
		},
		{
			// Not named, the sheet is the last of the head — where
			// packaging writes it: after the <style> of the head, before
			// the one of the body.
			name: "the sheet's place when its element is not named",
			css:  `@layer reset{.gone{x:y}}@layer base{.gone{x:y}}`,
			page: `<head><style>@layer reset{.gone{x:y}}</style></head><p class="a"></p><style>@layer base{.gone{x:y}}@layer late{.gone{x:y}}</style>`,
			want: `@layer base;`, styles: []string{`@layer reset;`, `@layer late;`},
		},
		{
			// What a <style media> declares is declared when the medium
			// applies: it orders nothing for the sheets after it.
			name: "a <style media> is a sheet under a condition",
			css:  `@layer print{.gone{x:y}}`,
			page: `<head><style media="print">@layer print{.gone{x:y}}.a{x:y}.gone{x:y}</style><style></style></head><p class="a"></p>`,
			want: `@layer print;`, styles: []string{`@layer print;.a{x:y}`},
		},
		{
			name: "an @import in a <style> is a sheet that is not here: every custom property stays",
			css:  `:root{--unread:1}.gone{x:y}`,
			page: `<head><style>@import "/theme.css";.gone{x:y}:root{--mine:2}</style><style></style></head><p class="a"></p>`,
			want: `:root{--unread:1}`, styles: []string{`@import "/theme.css";:root{--mine:2}`},
		},
		{
			name: "in SVG it is CSS of the page too",
			css:  `.a{x:y}`,
			page: `<head><style></style></head><p class="a"></p><svg><style>circle{fill:red}rect{fill:red}.a{x:y}</style><circle r="1"/></svg>`,
			want: `.a{x:y}`, styles: []string{`circle{fill:red}.a{x:y}`},
		},
		{
			// The page-level guards hold: a page that is not pruned keeps
			// every sheet whole.
			name: "a page that is not pruned keeps its <style> too",
			css:  `.gone{x:y}`,
			page: `<head><style>.gone{color:red}</style><style></style></head><p class="a" contenteditable></p>`,
			want: `.gone{x:y}`, styles: []string{`.gone{color:red}`},
			why: "the page has an element the user edits (`contenteditable`)",
		},
		{
			name: "one that does not read is left as it is, and what it names stays",
			css:  `:root{--named:1;--unread:2}@keyframes named{to{x:y}}`,
			page: `<head><style>.gone{color:var(--named);animation:named 1s</style><style></style></head><p class="a"></p>`,
			want: `:root{--named:1}@keyframes named{to{x:y}}`, styles: []string{`.gone{color:var(--named);animation:named 1s`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := parsePage(t, tt.page)
			styles, builder := own(doc)
			out, stats, err := PruneWith(tt.css, doc, Options{Script: tt.script, Builder: builder, Styles: styles})
			if err != nil {
				t.Fatal(err)
			}
			if out != tt.want || stats.Why != tt.why || stats.Unpruned != (tt.why != "") {
				t.Errorf("the sheet:\n got %s\nwant %s\n(%+v)", out, tt.want, stats)
			}
			if len(styles) != len(tt.styles) {
				t.Fatalf("%d <style> elements of the page's own, want %d", len(styles), len(tt.styles))
			}
			for i, style := range styles {
				if style.Out != tt.styles[i] {
					t.Errorf("<style> %d:\n got %s\nwant %s", i, style.Out, tt.styles[i])
				}
			}
		})
	}
}

// What is said of the page's own `<style>` elements: a text that does not
// read is the element's own mistake, not the sheet's; the rules read and
// dropped are counted, in a row of their own.
func TestPruneOwnStyleStats(t *testing.T) {
	doc := parsePage(t, `<head><style>.a{x:y}.gone{x:y}@media print{.gone{x:y}}</style><style></style></head><p class="a"></p><style>.gone{x:y</style>`)
	styles, builder := own(doc)
	out, stats, err := PruneWith(`.a{x:y}.gone{x:y}`, doc, Options{Builder: builder, Styles: styles})
	if err != nil || out != `.a{x:y}` {
		t.Fatalf("%q, %v", out, err)
	}
	if styles[0].Err != nil || styles[1].Err == nil || !strings.Contains(styles[1].Err.Error(), "cssprune: ") || styles[1].Out != styles[1].CSS {
		t.Errorf("errors: %v, %v; the second: %q", styles[0].Err, styles[1].Err, styles[1].Out)
	}
	// Two rules of the sheet and three of the first <style>; one and two dropped.
	if stats.Rules != 5 || stats.RulesDropped != 3 || stats.BytesIn != 17 || stats.BytesOut != 7 {
		t.Errorf("stats: %+v", stats)
	}
	want := Source{Name: OwnStyle, Rules: 3, RulesDropped: 2, BytesIn: len(styles[0].CSS), BytesOut: len(`.a{x:y}`)}
	if len(stats.Sources) != 1 || stats.Sources[0] != want {
		t.Errorf("sources: %+v, want %+v", stats.Sources, want)
	}
}

// A `<style>` that the driver did not give — a type that is not CSS, one it
// could not place — is read for the names it uses, and nothing more.
func TestPruneStyleNotGiven(t *testing.T) {
	doc := parsePage(t, `<head><style type="text/less">.x{color:var(--named)}</style><style></style></head><p class="a"></p>`)
	_, builder := own(doc)
	// `own` gives only what is CSS: the first is not.
	out, _, err := PruneWith(`:root{--named:1;--unread:2}`, doc, Options{Builder: builder})
	if err != nil || out != `:root{--named:1}` {
		t.Errorf("%q, %v", out, err)
	}
}

// TestPruneLinks: a `<link>` of the page's own is a stylesheet the builder
// did not bundle by its `rel`, and by nothing else (builder.md, *Checks on
// the page*, the table of `<link>`): every custom property and @keyframes
// stays with one, and a hint or a relation changes nothing.
func TestPruneLinks(t *testing.T) {
	const css = `.a{x:y}.gone{x:y}:root{--unread:1}@keyframes never{to{x:y}}`
	const pruned, kept = `.a{x:y}`, `.a{x:y}:root{--unread:1}@keyframes never{to{x:y}}`
	for name, tt := range map[string]struct{ page, want string }{
		"a stylesheet":                          {`<head><link rel="stylesheet" href="/theme.css"></head><p class="a"></p>`, kept},
		"whatever its case, among other tokens": {`<head><link rel="Alternate  StyleSheet" title="Dark" href="/dark.css"></head><p class="a"></p>`, kept},
		"in the body":                           {`<p class="a"></p><link rel="stylesheet" href="/theme.css">`, kept},
		"disabled":                              {`<head><link rel="stylesheet" href="/theme.css" disabled></head><p class="a"></p>`, kept},
		"a preloaded style is a hint":           {`<head><link rel="preload" as="style" href="/theme.css"></head><p class="a"></p>`, pruned},
		"modulepreload":                         {`<head><link rel="modulepreload" href="/x.js"></head><p class="a"></p>`, pruned},
		"an icon, a manifest, a canonical":      {`<head><link rel="icon" href="/favicon.svg"><link rel="manifest" href="/m.json"><link rel="canonical" href="/"></head><p class="a"></p>`, pruned},
		"an alternate that is no stylesheet":    {`<head><link rel="alternate" type="application/rss+xml" href="/feed.xml"></head><p class="a"></p>`, pruned},
		"no rel, and one nobody knows":          {`<head><link href="/theme.css"><link rel="stylesheets" href="/theme.css"></head><p class="a"></p>`, pruned},
	} {
		if out, stats := mustPrune(t, css, parsePage(t, tt.page)); out != tt.want || stats.Unpruned {
			t.Errorf("%s:\n got %s\nwant %s", name, out, tt.want)
		}
	}
	// The builder's own link is the sheet that is pruned, not another.
	doc := parsePage(t, `<head><link rel="stylesheet" href="/_rg/page-1a2b3c4d.css"></head><p class="a"></p>`)
	_, builder := own(doc)
	if out, _, err := PruneWith(css, doc, Options{Builder: builder}); err != nil || out != pruned {
		t.Errorf("the builder's own link: %q, %v", out, err)
	}
}
