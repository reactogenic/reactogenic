// Package render executes the pages of a site at build time and returns what
// each one rendered and recorded (specs/phase02/builder.md, *The record*).
package render

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
	Flags  map[string]bool // the `RG_…` flags this use site turns on
}

// Page is the record of one executed page.
type Page struct {
	Route
	HTML       string         // as rendered, without the doctype
	Mounts     []Mount        // in render order, duplicates kept
	Components map[string]int // function components rendered, by name
}
