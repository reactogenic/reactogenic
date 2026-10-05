// Package markup says what an element of a page is to the builder, where
// more than one stage has to agree on it (specs/phase02/builder.md): what
// runs — a `<script>` of the page's own, an event handler attribute, a
// `javascript:` URL — which is shell-script for the page checks and "not
// pruned" for the pruner; a `<link>`, by its `rel`; and a `<style>` whose
// text is CSS.
//
// It knows nothing of who wrote an element: that an element is the builder's
// own is the caller's to say.
package markup

import (
	"strings"

	"golang.org/x/net/html"
)

// Attr finds an attribute by name. HTML's parser lower-cased the names of
// HTML elements and camel-cased SVG's (viewBox), so the comparison folds.
func Attr(el *html.Node, name string) (string, bool) {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Key, name) || a.Namespace != "" && strings.EqualFold(a.Namespace+":"+a.Key, name) {
			return a.Val, true
		}
	}
	return "", false
}

// Runs reports whether a `<script>` element is one a browser executes: a
// classic script, a module — not a data block (`application/json`,
// `application/ld+json`, an import map, speculation rules: any type that is
// not JavaScript's), and not an element with no source and nothing in it.
func Runs(script *html.Node) bool {
	if kind, typed := Attr(script, "type"); typed {
		switch kind = strings.ToLower(strings.TrimSpace(kind)); {
		case kind == "" || kind == "module":
		case strings.Contains(kind, "javascript") || strings.Contains(kind, "ecmascript") ||
			strings.Contains(kind, "jscript") || strings.Contains(kind, "livescript"):
		default:
			return false
		}
	}
	// `xlink:href`, of SVG's script, among them.
	for _, a := range script.Attr {
		if k := strings.ToLower(a.Key); k == "src" || k == "href" {
			return true
		}
	}
	for c := script.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) != "" {
			return true
		}
	}
	return false
}

// Handler reports whether an attribute may be an event handler: `on` and a
// name. More than the handlers there are — an attribute `once` of an
// author's own is taken for one — and that is the safe side: HTML, SVG and
// MathML have no attribute that starts with `on` and is not a handler.
func Handler(key string) bool {
	return len(key) > 2 && (key[0] == 'o' || key[0] == 'O') && (key[1] == 'n' || key[1] == 'N')
}

// The attributes that hold a URL a browser may run.
var urlAttribute = map[string]bool{
	"href": true, "src": true, "action": true, "formaction": true, "data": true, "ping": true,
	"poster": true, "background": true, "cite": true, "longdesc": true, "manifest": true,
}

// JavaScriptURL reports whether an attribute holds a `javascript:` URL, as a
// URL parser reads it: leading spaces and controls dropped, and tabs and
// line breaks wherever they are.
func JavaScriptURL(key, value string) bool {
	if !urlAttribute[strings.ToLower(key)] {
		return false
	}
	value = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, value)
	value = strings.TrimLeftFunc(value, func(r rune) bool { return r <= ' ' })
	const scheme = "javascript:"
	return len(value) >= len(scheme) && strings.EqualFold(value[:len(scheme)], scheme)
}

// LinkKind is what a `<link>` is, by its `rel` (builder.md, *Checks on the
// page*, the table of `<link>`): the one classification the pruner, the page
// checks and the link check follow.
type LinkKind int

const (
	// LinkUnknown: a `rel` the table does not name, or none.
	LinkUnknown LinkKind = iota
	// LinkStylesheet: `stylesheet`, also with `alternate` — a stylesheet.
	// The builder's own, which the caller knows, is the page's sheet; any
	// other is one the builder did not bundle.
	LinkStylesheet
	// LinkHint: `preload`, `modulepreload`, `prefetch`, `preconnect`,
	// `dns-prefetch` — it fetches, and applies and runs nothing.
	LinkHint
	// LinkRelation: `icon`, `manifest`, `canonical`, `alternate` and the
	// like — it names another resource.
	LinkRelation
)

var (
	hints     = map[string]bool{"preload": true, "modulepreload": true, "prefetch": true, "preconnect": true, "dns-prefetch": true}
	relations = map[string]bool{
		"icon": true, "apple-touch-icon": true, "manifest": true, "canonical": true, "alternate": true,
		"prev": true, "next": true, "author": true, "license": true, "help": true, "search": true, "me": true,
	}
)

// Rel classifies the value of a `rel` attribute: its tokens are separated
// by HTML's whitespace and compared without their case. `stylesheet` among
// them decides — `alternate stylesheet` is a stylesheet a reader may turn
// on; then a hint; then a relation. `rel="preload" as="style"` is a hint: it
// applies nothing.
func Rel(rel string) LinkKind {
	kind := LinkUnknown
	for _, token := range strings.FieldsFunc(rel, func(r rune) bool { return strings.ContainsRune(" \t\n\f\r", r) }) {
		switch token = strings.ToLower(token); {
		case token == "stylesheet":
			return LinkStylesheet
		case hints[token]:
			kind = LinkHint
		case relations[token] && kind == LinkUnknown:
			kind = LinkRelation
		}
	}
	return kind
}

// Link classifies a `<link>` element, wherever it stands — in the head or
// in the body — and whatever else it carries: a stylesheet that is
// `disabled` is one a script, or the reader, may turn on.
func Link(link *html.Node) LinkKind {
	rel, _ := Attr(link, "rel")
	return Rel(rel)
}

// Effect is a row of the table: what a kind of `<link>` means to each stage.
type Effect struct {
	// Unbundled: it is a stylesheet the builder did not bundle — unless it
	// is the builder's own — which may read any custom property and name
	// any animation or `@position-try`: the pruner drops none.
	Unbundled bool
	// Checked: a root-relative `href` of it is checked by link-not-found.
	Checked bool
	// Runs: it is a script of the page's own (shell-script). None is: a
	// `modulepreload` fetches a module and runs nothing.
	Runs bool
}

// Effect is the kind's row.
func (k LinkKind) Effect() Effect {
	return Effect{Unbundled: k == LinkStylesheet, Checked: true}
}

// CSS reports whether a `<style>` element holds CSS a browser applies: its
// `type` is absent, empty, or `text/css` — compared without its case, and
// not trimmed, as HTML compares it.
func CSS(style *html.Node) bool {
	kind, typed := Attr(style, "type")
	return !typed || kind == "" || strings.EqualFold(kind, "text/css")
}
