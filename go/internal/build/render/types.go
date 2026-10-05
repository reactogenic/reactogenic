// Package render executes the pages of a site — the variants of its routes —
// at build time and returns what each one rendered and recorded
// (specs/phase02/builder.md, *The record*).
//
// The pages are bundled by esbuild from the program — its resolutions, its
// texts — with React's static renderer (bundle.go), and executed in an
// embedded engine, a runtime per page (engine.go): a page's bytes do not
// depend on the pages rendered before it. What a page throws is reported
// where the author wrote it (position.go).
package render

import (
	"strings"
	"time"
)

// Index is the variant of a route that a static host serves at the route's
// pathname: `index.rtsx` (builder.md, *Routes*).
const Index = "index"

// Route is one document of the site: a variant of a route (builder.md,
// *Routes*). The route is a directory under the pages root; the variant, an
// `.rtsx` file of it that nothing of the project mounts or imports.
type Route struct {
	Pathname string // the route: "/guide/" — the directory, with a trailing slash; `pathname()` in every variant of it
	File     string // the variant's module, absolute
	Variant  string // the file's name without `.rtsx`: "index", "guest"; "" is Index
}

// Name is the variant's name: Variant, or Index when none is given.
func (r Route) Name() string {
	if r.Variant == "" {
		return Index
	}
	return r.Variant
}

// Output is the variant's document among the artifacts, from the output's
// root: `guide/index.html`, `account/guest.html`.
func (r Route) Output() string {
	return strings.TrimPrefix(r.Pathname, "/") + r.Name() + ".html"
}

// Path is the URL path the document has on a static host, from the site's
// root: the route's pathname for Index ("/account/"), the file for any other
// variant ("/account/guest.html"). It is what names a document where the
// pathname names a route: in a report, in the render bundle, in the
// control's table.
func (r Route) Path() string {
	if r.Name() == Index {
		return r.Pathname
	}
	return r.Pathname + r.Variant + ".html"
}

// Mount is one call of `mount()` from @reactogenic/core while a page rendered
// (builder.md, *Behaviours*).
type Mount struct {
	Module string          // "@reactogenic/ui/behaviors/menu-keys"
	ID     string          // the root element's id; "" for a page-level behaviour
	Flags  map[string]bool // the `RG_…` flags of this use site, as passed: on or off; nil when none were
	// Data is what this use site hands its behaviour — the second argument
	// of the mount's call — as JSON text, its keys sorted: `{"typeahead":true}`.
	// "": none.
	Data string
}

// Page is the record of one executed page: a variant of a route.
type Page struct {
	Route
	HTML       string         // as rendered, without the doctype
	Mounts     []Mount        // in render order, duplicates kept
	Components map[string]int // function components rendered, by name
}

// Options are the builder's choices for Render.
type Options struct {
	// Dir is the project's directory, the tsconfig's: `react` and
	// `react-dom` are resolved from it. "": the program's own.
	Dir string
	// Timeout ends a page that does not finish rendering — a loop without an
	// end — and, the same, the bundle that does not finish loading. 0: thirty
	// seconds.
	Timeout time.Duration
	// Memory ends a page that takes more than that many bytes — a loop that
	// keeps what it makes fills the machine's memory long before the
	// timeout. 0: one gibibyte.
	Memory uintptr
}
