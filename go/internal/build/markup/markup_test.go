package markup

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// first is the first element named tag of a parsed page.
func first(t *testing.T, page, tag string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var found *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found == nil && n.Type == html.ElementNode && n.Data == tag {
			found = n
		}
		for c := n.FirstChild; c != nil && found == nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == nil {
		t.Fatalf("no <%s> in %s", tag, page)
	}
	return found
}

// The table of `<link>` (builder.md, *Checks on the page*): by `rel`, its
// tokens whatever their case and their order.
func TestRel(t *testing.T) {
	for rel, want := range map[string]LinkKind{
		"stylesheet":             LinkStylesheet,
		"StyleSheet":             LinkStylesheet,
		"alternate stylesheet":   LinkStylesheet, // one the reader may turn on
		"stylesheet\talternate":  LinkStylesheet,
		" preload  stylesheet\n": LinkStylesheet,
		"preload":                LinkHint,
		"MODULEPRELOAD":          LinkHint,
		"prefetch":               LinkHint,
		"preconnect":             LinkHint,
		"dns-prefetch":           LinkHint,
		"icon preload":           LinkHint,
		"icon":                   LinkRelation,
		"shortcut icon":          LinkRelation,
		"apple-touch-icon":       LinkRelation,
		"manifest":               LinkRelation,
		"canonical":              LinkRelation,
		"alternate":              LinkRelation, // without `stylesheet`: a feed, a translation
		"prev":                   LinkRelation,
		"next":                   LinkRelation,
		"author":                 LinkRelation,
		"license":                LinkRelation,
		"help":                   LinkRelation,
		"search":                 LinkRelation,
		"me":                     LinkRelation,
		"":                       LinkUnknown,
		"pingback":               LinkUnknown,
		"stylesheets":            LinkUnknown, // a token is a whole word
		"style sheet":            LinkUnknown,
	} {
		if got := Rel(rel); got != want {
			t.Errorf("rel=%q: %v, want %v", rel, got, want)
		}
	}
}

// A `<link>` is classified wherever it stands and whatever else it carries.
func TestLink(t *testing.T) {
	for page, want := range map[string]LinkKind{
		`<head><link rel="stylesheet" href="/a.css"></head>`:                   LinkStylesheet,
		`<body><p>x</p><link REL="Stylesheet" href="/a.css"></body>`:           LinkStylesheet, // in the body: it applies there too
		`<head><link rel="stylesheet" href="/a.css" disabled></head>`:          LinkStylesheet, // off until something turns it on
		`<head><link rel="alternate stylesheet" title="Dark" href="/d.css">`:   LinkStylesheet,
		`<head><link rel="preload" as="style" href="/a.css"></head>`:           LinkHint, // fetched, not applied
		`<head><link rel="modulepreload" href="/a.js"></head>`:                 LinkHint, // fetched, not run
		`<head><link rel="icon" href="/favicon.svg"></head>`:                   LinkRelation,
		`<head><link rel="alternate" type="application/rss+xml" href="/feed">`: LinkRelation,
		`<head><link href="/a.css"></head>`:                                    LinkUnknown,
		`<head><link rel="import" href="/a.html"></head>`:                      LinkUnknown,
		`<body><svg><link rel="stylesheet" href="/a.css"></link></svg></body>`: LinkStylesheet,
	} {
		if got := Link(first(t, page, "link")); got != want {
			t.Errorf("%s: %v, want %v", page, got, want)
		}
	}
	// The table's rows: only a stylesheet keeps the pruner's hands off the
	// custom properties; every kind's link is checked; none runs.
	for kind, want := range map[LinkKind]Effect{
		LinkStylesheet: {Unbundled: true, Checked: true},
		LinkHint:       {Checked: true},
		LinkRelation:   {Checked: true},
		LinkUnknown:    {Checked: true},
	} {
		if got := kind.Effect(); got != want {
			t.Errorf("%v: %+v, want %+v", kind, got, want)
		}
	}
}

// What runs, and what is data.
func TestRuns(t *testing.T) {
	for page, want := range map[string]bool{
		`<script>go()</script>`:                                         true,
		`<script src="/theme.js"></script>`:                             true,
		`<script type="module">import "/x.js"</script>`:                 true,
		`<script type="Text/JavaScript">go()</script>`:                  true,
		`<script type="">go()</script>`:                                 true,
		`<svg><script href="/x.js"></script></svg>`:                     true,
		`<script></script>`:                                             false, // nothing to run
		`<script>  </script>`:                                           false,
		`<script type="application/json">{"a":1}</script>`:              false,
		`<script type="application/ld+json">{"@type":"x"}</script>`:     false,
		`<script type="importmap">{"imports":{}}</script>`:              false,
		`<script type="speculationrules">{"prerender":[]}</script>`:     false,
		`<script type="text/template"><b>x</b></script>`:                false,
		`<script type="application/json" src="/data.json"></script>`:    false,
		`<script type="module" src="/_rg/page-1a2b3c4d.js"></script>`:   true, // the builder's own is the caller's to tell
		`<script type=" Module ">import "/x.js"</script>`:               true,
		`<script type="text/ecmascript">go()</script>`:                  true,
		`<script type="application/x-javascript">go()</script>`:         true,
		`<script type="text/plain">go()</script>`:                       false,
		`<script language="javascript" type="text/babel">go()</script>`: false,
	} {
		if got := Runs(first(t, page, "script")); got != want {
			t.Errorf("%s: runs %v, want %v", page, got, want)
		}
	}
	for key, want := range map[string]bool{"onclick": true, "ONLOAD": true, "onbeforetoggle": true, "on": false, "one": true, "open": false, "data-onclick": false, "role": false} {
		if got := Handler(key); got != want {
			t.Errorf("handler %q: %v, want %v", key, got, want)
		}
	}
	for _, tt := range []struct {
		key, value string
		want       bool
	}{
		{"href", "javascript:go()", true},
		{"href", " JavaScript:go()", true},
		{"action", "java\nscript:go()", true},
		{"HREF", "\tjavascript:void 0", true},
		{"href", "/javascript:x", false},
		{"href", "https://example.com/?javascript:1", false},
		{"title", "javascript:go()", false}, // no URL there
		{"data-href", "javascript:go()", false},
	} {
		if got := JavaScriptURL(tt.key, tt.value); got != tt.want {
			t.Errorf("%s=%q: %v, want %v", tt.key, tt.value, got, tt.want)
		}
	}
}

// A `<style>` is CSS unless its type says otherwise.
func TestCSS(t *testing.T) {
	for page, want := range map[string]bool{
		`<style>a{}</style>`:                               true,
		`<style type="">a{}</style>`:                       true,
		`<style type="text/css">a{}</style>`:               true,
		`<style type="TEXT/CSS" media="print">a{}</style>`: true,
		`<style type="text/less">a{}</style>`:              false,
		`<style type=" text/css">a{}</style>`:              false, // not trimmed: a browser applies none of it
		`<svg><style>a{}</style></svg>`:                    true,
	} {
		if got := CSS(first(t, page, "style")); got != want {
			t.Errorf("%s: CSS %v, want %v", page, got, want)
		}
	}
}
