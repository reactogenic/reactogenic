// Package pagecheck checks what only something that sees a whole page can:
// its ids, the references to them, the targets of its commands and its links
// to the site's other pages (specs/phase02/builder.md, *Checks on the page*).
//
// It works on the parsed HTML of the rendered page, so a report is about the
// page — its file and pathname — and quotes the attribute; the renderer
// carries nothing that maps an attribute back to the `.rtsx`.
package pagecheck

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// Options is what the checks need of the build beside the page.
type Options struct {
	// Base is `--base`, the path the site is served under: "/docs/". A link
	// under it is the site's and is matched without it; "" is "/".
	Base string
}

// Check is Options{}.Check: the checks of a site served from "/".
func Check(page render.Page, doc *html.Node, routes []render.Route, files map[string]bool) []report.Report {
	return Options{}.Check(page, doc, routes, files)
}

// Check returns the id-duplicate, idref-not-found, command-target and
// link-not-found reports of page, in document order. doc is the page's HTML,
// parsed; routes are the site's pages; files are the other files of the
// output, by their path from its root ("/favicon.svg").
//
// The same mistake is reported once per page however often it is rendered: a
// link of the layout is on every item of a list.
func (o Options) Check(page render.Page, doc *html.Node, routes []render.Route, files map[string]bool) []report.Report {
	c := &checker{page: page, base: base(o.Base), routes: map[string]bool{}, files: files, ids: map[string][]*html.Node{}, names: map[string]bool{}, seen: map[string]bool{}}
	for _, r := range routes {
		c.routes[r.Pathname] = true
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
			if n.Data == "template" && n.Namespace == "" {
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
	page    render.Page
	base    string // "/", "/docs/"
	routes  map[string]bool
	files   map[string]bool
	ids     map[string][]*html.Node
	names   map[string]bool // <a name>: what a fragment names when no id does
	seen    map[string]bool
	reports []report.Report
}

func (c *checker) report(code, format string, args ...any) {
	r := report.Page(c.page.File, c.page.Pathname, code, fmt.Sprintf(format, args...))
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

func (c *checker) element(n *html.Node) {
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
			switch value := strings.Trim(a.Val, asciiSpace); {
			case n.Data == "base" && n.Namespace == "":
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
func (c *checker) link(href string) bool {
	// `//host` and, as browsers read it, `/\host` are another origin.
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") || strings.HasPrefix(href, `/\`) {
		return true
	}
	u, err := url.Parse(href)
	if err != nil {
		return true // not a URL we can read: the browser's to resolve
	}
	p := path.Clean(u.Path)
	if strings.HasSuffix(u.Path, "/") && p != "/" {
		p += "/"
	}
	// What is not under the base is another site of the same origin.
	switch {
	case p+"/" == c.base:
		p = "/"
	case strings.HasPrefix(p, c.base):
		p = "/" + p[len(c.base):]
	default:
		return true
	}
	file := func(p string) bool { return c.files[p] || c.files[strings.TrimPrefix(p, "/")] }
	if c.routes[p] || file(p) {
		return true
	}
	if strings.HasSuffix(p, "/") {
		return file(p + "index.html") // a directory of `public/` with an index
	}
	// `/guide` is `/guide/` after the host's redirect; `/guide/index.html` is
	// the file the page is written to.
	dir, index := strings.CutSuffix(p, "/index.html")
	return c.routes[p+"/"] || file(p+"/index.html") || index && c.routes[dir+"/"]
}

// base normalises `--base`: "", "docs", "/docs" and "/docs/" → "/", "/docs/".
func base(b string) string {
	return strings.TrimSuffix("/"+strings.Trim(b, "/"), "/") + "/"
}

// HTML's ASCII whitespace: what separates the ids of a list and what is
// stripped around a URL.
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
