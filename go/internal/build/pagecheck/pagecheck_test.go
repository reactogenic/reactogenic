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
func run(t *testing.T, body string) []string {
	t.Helper()
	page := render.Page{Route: routes[1], HTML: "<html><head><title>t</title></head><body>" + body + "</body></html>"}
	doc, err := html.Parse(strings.NewReader(page.HTML))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range Check(page, doc, routes, files) {
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
		// The path is the one a browser requests, not the one Go's URL parser
		// makes of it.
		{"an encoded slash is not a slash", `<a href="/guide%2F">a</a><a href="/reference%2Fcli/">b</a><a href="/reference%2fcli/">c</a>`,
			[]string{
				"link-not-found: `href=\"/guide%2F\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/reference%2Fcli/\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/reference%2fcli/\"` on `<a>` is neither a page nor a file of the output",
			}},
		{"an empty segment is a segment", `<a href="/guide//">a</a><a href="/reference//cli/">b</a>`,
			[]string{
				"link-not-found: `href=\"/guide//\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/reference//cli/\"` on `<a>` is neither a page nor a file of the output",
			}},
		{"a percent sign that encodes nothing", `<a href="/100%">a</a><a href="/gone/%zz">b</a><a href="/guide/?q=100%#%zz">c</a>`,
			[]string{
				"link-not-found: `href=\"/100%\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/gone/%zz\"` on `<a>` is neither a page nor a file of the output",
			}},
		{"a tab or a line break in a URL is not of it", "<a href=\"/go\tne/\">a</a><a href=\"/gui\nde/\">b</a><a href=\"/\t/example.com/gone/\">c</a>",
			[]string{"link-not-found: `href=\"/go\\tne/\"` on `<a>` is neither a page nor a file of the output"}},
		{"a backslash is a slash", `<a href="/reference\cli/">a</a><a href="/reference\gone/">b</a>`,
			[]string{"link-not-found: `href=\"/reference\\\\gone/\"` on `<a>` is neither a page nor a file of the output"}},
		{"dot segments, encoded and last", `<a href="/guide/.">a</a><a href="/guide/x/..">b</a><a href="/guide/%2E/">c</a><a href="/x/%2e%2E/guide">d</a><a href="/../../guide/">e</a><a href="/guide/..">f</a><a href="/guide/x/.%2e/y">g</a>`,
			[]string{"link-not-found: `href=\"/guide/x/.%2e/y\"` on `<a>` is neither a page nor a file of the output"}},
		// A link is written from the site's root, whatever --base the site is
		// built for: packaging prefixes it, after the check.
		{"a link that carries a base is to no page", `<a href="/docs/guide/">a</a><a href="/docs/">b</a>`,
			[]string{
				"link-not-found: `href=\"/docs/guide/\"` on `<a>` is neither a page nor a file of the output",
				"link-not-found: `href=\"/docs/\"` on `<a>` is neither a page nor a file of the output",
			}},
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
		// To a browser that runs scripts — every one the page's behaviours
		// are for — a <noscript> holds text.
		{"a noscript's content is not of the page", `
			<noscript><i id="n"></i><i id="n"></i><a href="/gone/">x</a><label for="nope"></label></noscript>
			<a href="#n">x</a>`,
			[]string{"idref-not-found: `href=\"#n\"` on `<a>` names no element of the page"}},
		// As behaviors' mount-no-element has it: getElementById finds it.
		{"a template itself is an element of the page", `<template id="tp" aria-controls="gone"></template><a href="#tp">x</a><i id="tp"></i>`,
			[]string{
				"idref-not-found: `aria-controls=\"gone\"` on `<template>` names no element of the page",
				"id-duplicate: `id=\"tp\"` is on 2 elements: `<template>`, `<i>`",
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.body); !slices.Equal(got, tt.want) {
				t.Errorf("got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(tt.want, "\n  "))
			}
		})
	}
}

// The names of the site are matched decoded: a page or a file whose name is
// not plain ASCII, or holds what a URL has to encode.
func TestNames(t *testing.T) {
	routes := []render.Route{{Pathname: "/"}, {Pathname: "/señor/"}, {Pathname: "/a b/"}, {Pathname: "/100%/"}, {Pathname: "/a%2Fb/"}}
	page := render.Page{Route: routes[0], HTML: `<html><body>
		<a href="/señor/">a</a><a href="/se%C3%B1or/">b</a><a href="/se%c3%b1or">c</a><a href="/a b/">d</a><a href="/a%20b/index.html">e</a>
		<a href="/100%25/">f</a><a href="/100%/">g</a><a href="/a%252Fb/">h</a><a href="/q%3F.pdf">i</a>
		<a href="/a%2Fb/">j</a><a href="/a/b/">k</a><a href="/senor/">l</a><a href="/q">m</a></body></html>`}
	doc, _ := html.Parse(strings.NewReader(page.HTML))
	var got []string
	for _, r := range Check(page, doc, routes, map[string]bool{"/q?.pdf": true}) {
		got = append(got, r.Message)
	}
	want := []string{
		"Page /: `href=\"/a%2Fb/\"` on `<a>` is neither a page nor a file of the output",
		"Page /: `href=\"/a/b/\"` on `<a>` is neither a page nor a file of the output",
		"Page /: `href=\"/senor/\"` on `<a>` is neither a page nor a file of the output",
		"Page /: `href=\"/q\"` on `<a>` is neither a page nor a file of the output",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
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

// RootRelative is the links Check reads, and so the ones packaging gives the
// base (builder.md, *Packaging*): its path, as a URL parser reads the href.
func TestRootRelative(t *testing.T) {
	for href, want := range map[string]string{
		"/":                 "/",
		"/guide/":           "/guide/",
		"/guide/?tab=2#x":   "/guide/",
		"/guide#x":          "/guide",
		" /guide/ ":         "/guide/",
		"\n/gui\tde/":       "/guide/",
		`\guide\slot`:       "/guide/slot",
		"/a?b=//c":          "/a",
		"/100%":             "/100%",
		"/favicon.svg":      "/favicon.svg",
		"/se%C3%B1or/":      "/se%C3%B1or/",
		"/a//b":             "/a//b",
		"/#top":             "/",
		"/?q":               "/",
		"/guide/index.html": "/guide/index.html",
	} {
		if got, ok := RootRelative(href); !ok || got != want {
			t.Errorf("%q: %q, %v; want %q", href, got, ok, want)
		}
	}
	for _, href := range []string{"", "#x", "?q", "guide/", "./guide/", "../", "//host/x", `/\host`, `\\host`, "/\t/host", "https://example.com/", "mailto:a@b", "javascript:void(0)", "data:,x"} {
		if got, ok := RootRelative(href); ok {
			t.Errorf("%q is taken for a root-relative link: %q", href, got)
		}
	}
}

// A <noscript> is read the same however the page was parsed: with scripting
// off — as a pruner that wants its markup parses it — its content is
// elements in the tree, and still not of the page.
func TestNoscript(t *testing.T) {
	page := render.Page{Route: routes[1], HTML: `<html><head></head><body><noscript><i id="n"></i><i id="n"></i><a href="/gone/">x</a></noscript><a href="#n">x</a></body></html>`}
	doc, err := html.ParseWithOptions(strings.NewReader(page.HTML), html.ParseOptionEnableScripting(false))
	if err != nil {
		t.Fatal(err)
	}
	reports := Check(page, doc, routes, files)
	if len(reports) != 1 || reports[0].Code != "idref-not-found" || !strings.Contains(reports[0].Message, "`href=\"#n\"`") {
		t.Errorf("reports: %+v", reports)
	}
}
