package pagecheck

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

var (
	routes = []render.Route{
		{Pathname: "/", File: "/site/pages/index.rtsx"},
		{Pathname: "/guide/", File: "/site/pages/guide/index.rtsx"},
		{Pathname: "/reference/cli/", File: "/site/pages/reference/cli/index.rtsx"},
	}
	files = map[string]bool{"/favicon.svg": true, "/demo/index.html": true, "/_rg/site-1a2b.css": true}
)

// run checks a page's body and returns `code: message` per report, the
// page's prefix removed.
func run(t *testing.T, o Options, body string) []string {
	t.Helper()
	page := render.Page{Route: routes[1], HTML: "<html><head><title>t</title></head><body>" + body + "</body></html>"}
	doc, err := html.Parse(strings.NewReader(page.HTML))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range o.Check(page, doc, routes, files) {
		message, ok := strings.CutPrefix(r.Message, "Page /guide/: ")
		if !ok || r.File != page.File || r.Line != 0 || r.Severity != report.Error {
			t.Errorf("not a report of the page: %+v", r)
		}
		got = append(got, r.Code+": "+message)
	}
	return got
}

func TestCheck(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       []string
	}{
		{"a page of the design system", `
			<button command="show-modal" commandfor="d1">Open</button>
			<dialog id="d1" aria-labelledby="d1-t"><h2 id="d1-t">Title</h2>
				<button command="close" commandfor="d1">Close</button>
				<button command="request-close" commandfor="d1">Cancel</button></dialog>
			<button popovertarget="m1">Menu</button>
			<div id="m1" popover anchor="d1"><a href="/guide/">Guide</a></div>
			<label for="q">Search</label><input id="q" aria-describedby="hint d1-t"><p id="hint">…</p>`, nil},

		// id-duplicate
		{"an id twice", `<nav id="nav"></nav><p id="other"></p><div id="nav"></div>`,
			[]string{"id-duplicate: `id=\"nav\"` is on 2 elements: `<nav>`, `<div>`"}},
		{"an id three times: one report", `<i id="x"></i><b id="x"></b><u id="x"></u>`,
			[]string{"id-duplicate: `id=\"x\"` is on 3 elements: `<i>`, `<b>`, `<u>`"}},
		{"ids are case-sensitive; an empty id is none", `<i id="x"></i><b id="X"></b><u id=""></u><s id=""></s>`, nil},
		{"a reference to a duplicated id is found", `<i id="x"></i><b id="x"></b><a href="#x">x</a>`,
			[]string{"id-duplicate: `id=\"x\"` is on 2 elements: `<i>`, `<b>`"}},

		// idref-not-found: each attribute that names an id
		{"commandfor", `<button command="show-modal" commandfor="install">Install</button>`,
			[]string{"idref-not-found: `commandfor=\"install\"` on `<button>` names no element of the page"}},
		{"popovertarget", `<button popovertarget="m9">Menu</button>`,
			[]string{"idref-not-found: `popovertarget=\"m9\"` on `<button>` names no element of the page"}},
		{"popoverTarget, as React prints it", `<button popoverTarget="m9">Menu</button>`,
			[]string{"idref-not-found: `popovertarget=\"m9\"` on `<button>` names no element of the page"}},
		{"label for", `<label for="q">Search</label>`,
			[]string{"idref-not-found: `for=\"q\"` on `<label>` names no element of the page"}},
		{"output for is a list", `<input id="a"><output for="a b c"></output>`,
			[]string{"idref-not-found: `for=\"a b c\"` on `<output>` names no element of the page: `b`, `c`"}},
		{"for elsewhere is not a reference", `<script for="window" event="onload"></script><div for="x"></div>`, nil},
		{"anchor", `<div popover anchor="trigger"></div>`,
			[]string{"idref-not-found: `anchor=\"trigger\"` on `<div>` names no element of the page"}},
		{"aria-labelledby: one id", `<dialog aria-labelledby="t"></dialog>`,
			[]string{"idref-not-found: `aria-labelledby=\"t\"` on `<dialog>` names no element of the page"}},
		{"aria-describedby: a list, the missing ones named", "<p id=\"a\"></p><input aria-describedby=\"a  b\tc\">",
			[]string{"idref-not-found: `aria-describedby=\"a  b\\tc\"` on `<input>` names no element of the page: `b`, `c`"}},
		{"aria-controls", `<button aria-controls="panel tabs"></button><div id="tabs"></div>`,
			[]string{"idref-not-found: `aria-controls=\"panel tabs\"` on `<button>` names no element of the page: `panel`"}},
		{"an empty list names nothing", `<div aria-labelledby="" aria-describedby=" "></div>`, nil},
		{"an empty reference to one id is a mistake", `<button commandfor="" command="--x"></button>`,
			[]string{"idref-not-found: `commandfor=\"\"` on `<button>` names no element of the page"}},
		{"other ARIA references are not in the set", `<div aria-owns="x" aria-activedescendant="y" data-for="z"></div>`, nil},

		// href="#x"
		{"a fragment", `<a href="#install">Install</a>`,
			[]string{"idref-not-found: `href=\"#install\"` on `<a>` names no element of the page"}},
		{"a fragment that is there", `<a href="#install">Install</a><h2 id="install">Install</h2>`, nil},
		{"the top of the document", `<a href="#">Top</a><a href="#top">Top</a><a href="#TOP">Top</a>`, nil},
		{"a percent-encoded fragment", `<a href="#%C3%BCber">x</a><h2 id="über">x</h2>`, nil},
		{"a named anchor", `<a href="#old">x</a><a name="old"></a>`, nil},
		{"a text directive is not an id", `<a href="#:~:text=slots">x</a><a href="#nope:~:text=slots">y</a>`,
			[]string{"idref-not-found: `href=\"#nope:~:text=slots\"` on `<a>` names no element of the page"}},
		{"an svg reference", `<svg><symbol id="icon"></symbol><use href="#icon"></use><use href="#none"></use></svg>`,
			[]string{"idref-not-found: `href=\"#none\"` on `<use>` names no element of the page"}},

		// command-target
		{"show-modal on a div", `<button command="show-modal" commandfor="x">Open</button><div id="x"></div>`,
			[]string{"command-target: `command=\"show-modal\"` on `<button>` needs a `<dialog>`: `commandfor=\"x\"` is a `<div>`"}},
		{"close and request-close on a popover", `<button command="close" commandfor="x"></button><button command="request-close" commandfor="x"></button><div id="x" popover></div>`,
			[]string{
				"command-target: `command=\"close\"` on `<button>` needs a `<dialog>`: `commandfor=\"x\"` is a `<div>`",
				"command-target: `command=\"request-close\"` on `<button>` needs a `<dialog>`: `commandfor=\"x\"` is a `<div>`",
			}},
		{"popover commands on a dialog without popover", `
			<button command="show-popover" commandfor="x"></button><button command="hide-popover" commandfor="x"></button>
			<button command="toggle-popover" commandfor="x"></button><dialog id="x"></dialog>`,
			[]string{
				"command-target: `command=\"show-popover\"` on `<button>` needs an element with `popover`: `commandfor=\"x\"` is a `<dialog>` without it",
				"command-target: `command=\"hide-popover\"` on `<button>` needs an element with `popover`: `commandfor=\"x\"` is a `<dialog>` without it",
				"command-target: `command=\"toggle-popover\"` on `<button>` needs an element with `popover`: `commandfor=\"x\"` is a `<dialog>` without it",
			}},
		{"popover commands on a popover, of any kind", `
			<button command="toggle-popover" commandfor="a"></button><div id="a" popover></div>
			<button command="show-popover" commandfor="b"></button><div id="b" popover="manual"></div>
			<button command="show-modal" commandfor="c"></button><dialog id="c" popover></dialog>`, nil},
		{"popovertarget needs a popover", `<button popovertarget="x">Menu</button><div id="x"></div>`,
			[]string{"command-target: `popovertarget=\"x\"` on `<button>` needs an element with `popover`: it names a `<div>` without it"}},
		{"a command is a keyword: case-insensitive", `<button command="Show-Modal" commandfor="x"></button><p id="x"></p>`,
			[]string{"command-target: `command=\"Show-Modal\"` on `<button>` needs a `<dialog>`: `commandfor=\"x\"` is a `<p>`"}},
		{"a custom command and an unknown one are the page's own", `<button command="--rotate" commandfor="x"></button><button command="zoom" commandfor="x"></button><p id="x"></p>`, nil},
		{"a command without a target", `<button command="show-modal">Open</button><button command="--custom"></button>`,
			[]string{"command-target: `command=\"show-modal\"` on `<button>` has no `commandfor`"}},
		{"a missing target is one report, not two", `<button command="show-modal" commandfor="gone"></button>`,
			[]string{"idref-not-found: `commandfor=\"gone\"` on `<button>` names no element of the page"}},

		// link-not-found
		{"links to pages", `<a href="/">Home</a><a href="/guide/">Guide</a><a href="/reference/cli/">CLI</a>`, nil},
		{"a link to no page", `<a href="/guide/slot/">Slots</a>`,
			[]string{"link-not-found: `href=\"/guide/slot/\"` on `<a>` is neither a page nor a file of the output"}},
		{"query and fragment are ignored", `<a href="/guide/?tab=2#install">x</a><a href="/guide/#nope">y</a><a href="/gone/?a#b">z</a>`,
			[]string{"link-not-found: `href=\"/gone/?a#b\"` on `<a>` is neither a page nor a file of the output"}},
		{"a missing trailing slash", `<a href="/guide">x</a><a href="/reference/cli?x#y">y</a><a href="/reference">z</a>`,
			[]string{"link-not-found: `href=\"/reference\"` on `<a>` is neither a page nor a file of the output"}},
		{"the file a page is written to", `<a href="/guide/index.html">x</a><a href="/index.html">y</a><a href="/gone/index.html">z</a>`,
			[]string{"link-not-found: `href=\"/gone/index.html\"` on `<a>` is neither a page nor a file of the output"}},
		{"files of the output", `<link rel="icon" href="/favicon.svg"><link rel="stylesheet" href="/_rg/site-1a2b.css"><a href="/demo/">demo</a><a href="/demo">demo</a><a href="/logo.png">logo</a>`,
			[]string{"link-not-found: `href=\"/logo.png\"` on `<a>` is neither a page nor a file of the output"}},
		{"external and relative links are not checked", `
			<a href="https://example.com/gone/">a</a><a href="//example.com/gone/">b</a><a href="mailto:x@example.com">c</a>
			<a href="slot/">d</a><a href="../gone/">e</a><a href="?tab=2">f</a><a href="">g</a><a href="/\example.com/">h</a>
			<a>i</a><a href="javascript:void 0">j</a>`, nil},
		{"dot segments and percent-encoding, as the browser resolves them", `<a href="/x/../guide/">a</a><a href="/%67uide/">b</a><a href="/guide/../gone/">c</a>`,
			[]string{"link-not-found: `href=\"/guide/../gone/\"` on `<a>` is neither a page nor a file of the output"}},
		{"whitespace around a URL", `<a href=" /guide/ ">a</a><a href=" #nope ">b</a>`,
			[]string{"idref-not-found: `href=\" #nope \"` on `<a>` names no element of the page"}},
		{"an svg link", `<svg><a href="/gone/"><text>x</text></a></svg>`,
			[]string{"link-not-found: `href=\"/gone/\"` on `<a>` is neither a page nor a file of the output"}},

		// the whole page
		{"the same mistake rendered twice is one report", `<a href="/gone/">a</a><a href="/gone/">b</a><area href="/gone/">`,
			[]string{
				"link-not-found: `href=\"/gone/\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/gone/\"` on `<area>` is neither a page nor a file of the output",
			}},
		{"document order", `<a href="/gone/">a</a><i id="x"></i><label for="q"></label><b id="x"></b><button popovertarget="x"></button>`,
			[]string{
				"link-not-found: `href=\"/gone/\"` on `<a>` is neither a page nor a file of the output",
				"idref-not-found: `for=\"q\"` on `<label>` names no element of the page",
				"id-duplicate: `id=\"x\"` is on 2 elements: `<i>`, `<b>`",
				"command-target: `popovertarget=\"x\"` on `<button>` needs an element with `popover`: it names a `<i>` without it",
			}},
		{"a template's content is not of the page", `
			<template><i id="t"></i><i id="t"></i><a href="/gone/">x</a><label for="nope"></label></template>
			<label for="t"></label>`,
			[]string{"idref-not-found: `for=\"t\"` on `<label>` names no element of the page"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, Options{}, tt.body); !slices.Equal(got, tt.want) {
				t.Errorf("got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(tt.want, "\n  "))
			}
		})
	}
}

// With --base the site's links carry the base: it is stripped before the
// match, and a root-relative link outside it is another site's.
func TestBase(t *testing.T) {
	body := `<a href="/docs/">home</a><a href="/docs">home</a><a href="/docs/guide/">guide</a><a href="/docs/guide">guide</a>
		<a href="/docs/favicon.svg">icon</a><a href="/docs/gone/">gone</a><a href="/guide/">outside</a><a href="/docsx/">outside</a><a href="/">outside</a>`
	want := []string{"link-not-found: `href=\"/docs/gone/\"` on `<a>` is neither a page nor a file of the output"}
	for _, base := range []string{"/docs/", "/docs", "docs"} {
		if got := run(t, Options{Base: base}, body); !slices.Equal(got, want) {
			t.Errorf("base %q: got:\n  %s\nwant:\n  %s", base, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	}
	for _, base := range []string{"", "/"} {
		if got := run(t, Options{Base: base}, `<a href="/guide/">a</a><a href="/docs/guide/">b</a>`); !slices.Equal(got, []string{"link-not-found: `href=\"/docs/guide/\"` on `<a>` is neither a page nor a file of the output"}) {
			t.Errorf("base %q: %q", base, got)
		}
	}
}

// Files may be named from the output's root with or without the slash.
func TestFiles(t *testing.T) {
	page := render.Page{Route: routes[0], HTML: `<html><body><a href="/a.pdf">a</a><a href="/b/c.pdf">b</a><a href="/d.pdf">d</a></body></html>`}
	doc, _ := html.Parse(strings.NewReader(page.HTML))
	got := Check(page, doc, routes, map[string]bool{"a.pdf": true, "/b/c.pdf": true})
	if len(got) != 1 || got[0].Message != "Page /: `href=\"/d.pdf\"` on `<a>` is neither a page nor a file of the output" {
		t.Errorf("%+v", got)
	}
	if got := Check(page, doc, nil, nil); len(got) != 3 {
		t.Errorf("no routes, no files: %d reports", len(got))
	}
}

// The reports print as check's do: the page's file, the code, the pathname
// and the attribute.
func TestPrint(t *testing.T) {
	page := render.Page{Route: routes[1], HTML: `<html><body><button command="show-modal" commandfor="install">Install</button><a href="/guide/slot/">Slots</a></body></html>`}
	doc, _ := html.Parse(strings.NewReader(page.HTML))
	var out bytes.Buffer
	check.Print(&out, Check(page, doc, routes, files), "/site", false, nil)
	want := "pages/guide/index.rtsx: error idref-not-found: Page /guide/: `commandfor=\"install\"` on `<button>` names no element of the page\n" +
		"pages/guide/index.rtsx: error link-not-found: Page /guide/: `href=\"/guide/slot/\"` on `<a>` is neither a page nor a file of the output\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
