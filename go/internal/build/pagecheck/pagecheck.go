// Package pagecheck checks what only something that sees a whole page can:
// its ids, the references to them, the targets of its commands and its links
// to the site's other pages (specs/phase02/builder.md, *Checks on the page*)
// — and that nothing in it runs: a script of the page's own is shell-script
// (*Shell code in phase 2*), by the one notion of "runs" the pruner has too
// (package markup).
//
// It works on the parsed HTML of the rendered page, so a report is about the
// page — its file and pathname — and quotes the attribute; the renderer
// carries nothing that maps an attribute back to the `.rtsx`.
package pagecheck

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/markup"
	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// Check returns the id-duplicate, idref-not-found, command-target,
// link-not-found and shell-script reports of page, in document order. doc is the page's HTML,
// parsed; routes are the site's documents — the variants of its routes;
// files are the other files of the output, by their path from its root
// ("/favicon.svg").
//
// The page is checked as it is rendered, before it is packaged: its links
// name the site from its root, whatever `--base` the build then prefixes
// them with (builder.md, *Packaging*).
//
// The same mistake is reported once per page however often it is rendered: a
// link of the layout is on every item of a list.
func Check(page render.Page, doc *html.Node, routes []render.Route, files map[string]bool) []report.Report {
	c := &checker{page: page, routes: map[string]bool{}, documents: map[string]bool{}, files: files, ids: map[string][]*html.Node{}, names: map[string]bool{}, seen: map[string]bool{}}
	for _, r := range routes {
		c.routes[r.Pathname] = true
		c.documents["/"+r.Output()] = true
	}
	var elements []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			elements = append(elements, n)
			if id, _ := attr(n, "id"); id != "" { // `id=""` is no id
				c.ids[id] = append(c.ids[id], n)
			}
			if name, _ := attr(n, "name"); name != "" && n.Data == "a" {
				c.names[name] = true
			}
			// A template's content is not of the page: its ids are not in
			// the document, and its references are resolved where it is
			// cloned (phase 2 emits none).
			// Nor is a noscript's, to a browser that runs scripts — those
			// the page's behaviours are for: it is text there, whether the
			// parser that made doc kept it as text or as elements.
			if (n.Data == "template" || n.Data == "noscript") && n.Namespace == "" {
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	for _, n := range elements {
		c.element(n)
	}
	return c.reports
}

type checker struct {
	page      render.Page
	routes    map[string]bool // the routes, by pathname: a route with a variant, whichever
	documents map[string]bool // the variants' documents, by their file: "/guide/index.html", "/account/guest.html"
	files     map[string]bool
	ids       map[string][]*html.Node
	names     map[string]bool // <a name>: what a fragment names when no id does
	seen      map[string]bool
	reports   []report.Report
}

func (c *checker) report(code, format string, args ...any) {
	r := report.Page(c.page.File, c.page.Path(), code, fmt.Sprintf(format, args...))
	if key := code + "\x00" + r.Message; !c.seen[key] {
		c.seen[key] = true
		c.reports = append(c.reports, r)
	}
}

// lists are the attributes whose value is a space-separated list of ids; the
// others name one.
var lists = map[string]bool{"aria-labelledby": true, "aria-describedby": true, "aria-controls": true}

// The commands whose target has to be something (builder.md): a custom
// command (`--x`) and one the builder does not know are the page's own.
var (
	dialogCommands  = map[string]bool{"show-modal": true, "close": true, "request-close": true}
	popoverCommands = map[string]bool{"show-popover": true, "hide-popover": true, "toggle-popover": true}
)

// script reports what of an element runs (shell-script): page-author JS is
// deferred, so a page has no script but the builder's — which packaging
// writes after these checks. What runs is package markup's to say, for the
// pruner too: a `<script>` that is not a data block, an event handler
// attribute, a `javascript:` URL. A `<link>` runs nothing, whatever its
// `rel` (the table of `<link>`): `modulepreload` fetches.
func (c *checker) script(n *html.Node) {
	const cannot = "The shell cannot run a script of the page's own: "
	switch tag := strings.ToLower(n.Data); {
	case tag == "script" && markup.Runs(n):
		written := "<script"
		for _, key := range []string{"type", "src", "href"} {
			if value, ok := markup.Attr(n, key); ok {
				written += fmt.Sprintf(" %s=%q", key, value)
			}
		}
		c.report("shell-script", cannot+"`%s>`", written)
	case tag == "link" && markup.Link(n).Effect().Runs:
		c.report("shell-script", cannot+"`<link>`")
	}
	for _, a := range n.Attr {
		if markup.Handler(a.Key) || markup.JavaScriptURL(a.Key, a.Val) {
			c.report("shell-script", cannot+"`%s=%q` on `<%s>`", a.Key, a.Val, n.Data)
		}
	}
}

func (c *checker) element(n *html.Node) {
	c.script(n)
	for _, a := range n.Attr {
		if a.Namespace != "" {
			continue
		}
		written := fmt.Sprintf("`%s=%q` on `<%s>`", a.Key, a.Val, n.Data)
		switch a.Key {
		case "id":
			// Reported where the id is used a second time, once.
			if all := c.ids[a.Val]; len(all) > 1 && all[1] == n {
				tags := make([]string, len(all))
				for i, e := range all {
					tags[i] = "`<" + e.Data + ">`"
				}
				c.report("id-duplicate", "`id=%q` is on %d elements: %s", a.Val, len(all), strings.Join(tags, ", "))
			}
		case "commandfor":
			target := c.one(written, a.Val)
			command, _ := attr(n, "command")
			if target == nil {
				break
			}
			switch keyword := strings.ToLower(command); { // an enumerated attribute: ASCII case-insensitive
			case dialogCommands[keyword] && !(target.Data == "dialog" && target.Namespace == ""):
				c.report("command-target", "`command=%q` on `<%s>` needs a `<dialog>`: `commandfor=%q` is a `<%s>`", command, n.Data, a.Val, target.Data)
			case popoverCommands[keyword] && !has(target, "popover"):
				c.report("command-target", "`command=%q` on `<%s>` needs an element with `popover`: `commandfor=%q` is a `<%s>` without it", command, n.Data, a.Val, target.Data)
			}
		case "command":
			// A command with nothing to command does nothing in the browser.
			if command := strings.ToLower(a.Val); (dialogCommands[command] || popoverCommands[command]) && !has(n, "commandfor") {
				c.report("command-target", "%s has no `commandfor`", written)
			}
		case "popovertarget":
			if target := c.one(written, a.Val); target != nil && !has(target, "popover") {
				c.report("command-target", "%s needs an element with `popover`: it names a `<%s>` without it", written, target.Data)
			}
		case "anchor":
			c.one(written, a.Val)
		case "for":
			// `for` is an id on a label and a list of ids on an output;
			// elsewhere it is not HTML's.
			switch {
			case n.Namespace != "":
			case n.Data == "label":
				c.one(written, a.Val)
			case n.Data == "output":
				c.list(written, a.Val)
			}
		case "href":
			// What a URL parser strips around a URL: spaces and controls.
			switch value := strings.TrimFunc(a.Val, func(r rune) bool { return r <= ' ' }); {
			case n.Data == "base" && n.Namespace == "":
			case n.Data == "link" && n.Namespace == "" && !markup.Link(n).Effect().Checked:
				// The table of `<link>`: a link of every `rel` is checked.
			case strings.HasPrefix(value, "#"):
				if !c.fragment(value[1:]) {
					c.report("idref-not-found", "%s names no element of the page", written)
				}
			case !c.link(value):
				c.report("link-not-found", "%s is neither a page nor a file of the output", written)
			}
		default:
			if lists[a.Key] {
				c.list(written, a.Val)
			}
		}
	}
}

// one checks a reference to one id and returns the element it names.
func (c *checker) one(written, id string) *html.Node {
	if all := c.ids[id]; len(all) > 0 {
		return all[0]
	}
	c.report("idref-not-found", "%s names no element of the page", written)
	return nil
}

// list checks a space-separated list of ids. An empty list names nothing,
// and that is not a mistake.
func (c *checker) list(written, value string) {
	ids := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune(asciiSpace, r) })
	var missing []string
	for _, id := range ids {
		if len(c.ids[id]) == 0 {
			missing = append(missing, "`"+id+"`")
		}
	}
	switch {
	case len(missing) == 0:
	case len(ids) == 1:
		c.report("idref-not-found", "%s names no element of the page", written)
	default:
		c.report("idref-not-found", "%s names no element of the page: %s", written, strings.Join(missing, ", "))
	}
}

// fragment says whether `#fragment` names something of the page, as the
// browser looks it up: the id as written, then percent-decoded, then an
// `<a name>`. The empty fragment and `top` are the top of the document; a
// text directive (`#:~:text=…`) is not an id.
func (c *checker) fragment(fragment string) bool {
	fragment, _, _ = strings.Cut(fragment, ":~:")
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		decoded = fragment
	}
	return fragment == "" || len(c.ids[fragment]) > 0 || len(c.ids[decoded]) > 0 || c.names[decoded] || strings.EqualFold(decoded, "top")
}

// link says whether an href may stand: one that is not root-relative — a URL
// with a scheme or a host, a relative path, a query alone — is not checked;
// a root-relative one has to be a page of the site or a file of the output,
// its query and fragment aside.
//
// The href is read as a browser's URL parser reads it, not as Go's: that one
// refuses what a browser requests (`/100%`) and decodes what a browser
// leaves alone (`%2F`).
func (c *checker) link(href string) bool {
	href, ok := RootRelative(href)
	if !ok {
		return true
	}
	p := pathOf(href)
	file := func(p string) bool { return c.files[p] || c.files[strings.TrimPrefix(p, "/")] }
	// A route — also one without an `index`: it is the server's to say what
	// is served there (builder.md, *Routes*) — or the file a variant is
	// written to: `/guide/index.html`, `/account/guest.html`.
	if c.routes[p] || c.documents[p] || file(p) {
		return true
	}
	if strings.HasSuffix(p, "/") {
		return file(p + "index.html") // a directory of `public/` with an index
	}
	// `/guide` is `/guide/` after the host's redirect.
	return c.routes[p+"/"] || file(p+"/index.html")
}

// RootRelative says whether href — the value of an `href`, as written — is a
// link to the site from its root, and returns its path, query and fragment
// aside, a backslash a slash. These are the links Check reads, and so the
// ones packaging prefixes with `--base` (builder.md, *Packaging*): one rule
// for both. A URL with a scheme or a host (`//host`, and so `/\host`), a
// relative one and a fragment alone are not.
func RootRelative(href string) (path string, ok bool) {
	// What a URL parser strips around a URL: spaces and controls. A tab or a
	// line break is not of the URL, wherever it is.
	href = strings.TrimFunc(href, func(r rune) bool { return r <= ' ' })
	href = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, href)
	href, _, _ = strings.Cut(href, "#")
	href, _, _ = strings.Cut(href, "?")
	href = strings.ReplaceAll(href, `\`, "/") // in a path, a backslash is a slash
	// `//host` — and so `/\host` — is another origin.
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") {
		return "", false
	}
	return href, true
}

// pathOf is the path a root-relative href requests, in the names of the
// site's directories: dot segments resolved (`.`, `..`, also written `%2e`),
// then each segment percent-decoded. An empty segment is a segment — `/a//b`
// is not `/a/b` — and an encoded slash is no separator: `%2F` is decoded to
// a NUL, which is in no name. A `%` that encodes nothing stands for itself.
func pathOf(href string) string {
	var segments []string
	all := strings.Split(href[1:], "/")
	for i, segment := range all {
		last := i == len(all)-1
		switch strings.ReplaceAll(strings.ToLower(segment), "%2e", ".") {
		case "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
			}
			if last {
				segments = append(segments, "")
			}
		case ".":
			if last {
				segments = append(segments, "")
			}
		default:
			segments = append(segments, decoded(segment))
		}
	}
	return "/" + strings.Join(segments, "/")
}

func decoded(segment string) string {
	if !strings.Contains(segment, "%") {
		return segment
	}
	var b strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if c == '%' && i+2 < len(segment) && isHex(segment[i+1]) && isHex(segment[i+2]) {
			if c = unhex(segment[i+1])<<4 | unhex(segment[i+2]); c == '/' {
				c = 0
			}
			i += 2
		}
		b.WriteByte(c)
	}
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c >= 'a':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}

// HTML's ASCII whitespace: what separates the ids of a list.
const asciiSpace = " \t\n\f\r"

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key && a.Namespace == "" {
			return a.Val, true
		}
	}
	return "", false
}

func has(n *html.Node, key string) bool {
	_, ok := attr(n, key)
	return ok
}
