// Package render executes the pages of a site at build time and returns what
// each one rendered and recorded (specs/phase02/builder.md, *The record*).
//
// The pages are bundled by esbuild from the program — its resolutions, its
// texts — with React's static renderer (bundle.go), and executed in an
// embedded engine, one per build (engine.go). What a page throws is reported
// where the author wrote it (position.go).
package render

import "time"

// Route is a page: `index.rtsx` (or `index.tsx`) of a directory under the
// pages root.
type Route struct {
	Pathname string // "/guide/": the directory, with a trailing slash
	File     string // the page's module, absolute
}

// Mount is one call of `mount()` from @reactogenic/core while a page rendered
// (builder.md, *Behaviours*).
type Mount struct {
	Module string          // "@reactogenic/ui/behaviors/menu-keys"
	ID     string          // the root element's id; "" for a page-level behaviour
	Flags  map[string]bool // the `RG_…` flags of this use site, as passed: on or off; nil when none were
}

// Page is the record of one executed page.
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
	// end. 0: thirty seconds.
	Timeout time.Duration
}
