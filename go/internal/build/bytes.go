package build

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/reactogenic/reactogenic/go/internal/build/behaviors"
	"github.com/reactogenic/reactogenic/go/internal/build/cssprune"
)

// Report is the byte report of a build (builder.md, *The report*): what each
// page ships, how, and why each byte is there. It is written to
// `<out>/_rg/report.json`, and `--report` prints it.
//
// It keeps two things apart (builder.md, *Analysis and packaging*). What a
// page **needs** is analysis, per page and exact: its HTML, its CSS — the
// rules that can match on it — its script — the behaviours it mounted —
// with why (Styles, Modules, Mounts, Classes). What it **fetches** is
// packaging: the document as written, the files it links (Fetches, and how
// each blob is delivered). Packaging may make a page fetch what another
// needs; it never changes what a page needs. As the builder packages today
// — a blob is shared only when it is the same bytes — the two are equal.
//
// Nothing in it names the machine: a file of the project is named from the
// project directory, a file of a package by the package (naming).
type Report struct {
	Base       string       `json:"base"`
	Inline     string       `json:"inline"`
	Specialize bool         `json:"specialize"` // false: the control (`--no-specialize`)
	Pages      []PageReport `json:"pages"`
	Blobs      []BlobReport `json:"blobs"`
	Public     []string     `json:"public"` // the files copied from `public/`
}

// Size is a number of bytes as written, and after gzip: the gzip stream Go's
// compress/gzip writes at its level 9. That is DEFLATE by another encoder
// than zlib's, so not to the byte what `gzip -9` gives — within 1%, and on
// either side: on the docs site from 76 B fewer, on its largest page's
// HTML, to 3 B more, on a 252 B script (builder.md, *The report*, gzip;
// `bench/site.mjs` prints the range). The measuring scripts have the
// reference numbers. Brotli is not here: Go has no encoder for it, and the
// builder takes no dependency to count with — they have that too (plan.md,
// RGP2-050).
type Size struct {
	Raw  int `json:"raw"`
	Gzip int `json:"gzip"`
}

// PageReport is one page: a variant of a route.
type PageReport struct {
	Pathname string `json:"pathname"` // the route
	Variant  string `json:"variant"`  // "index": the one a static host serves at the pathname
	// Path names the document wherever the report names one — a blob's
	// pages, the printed rows: its URL path on a static host, the pathname
	// for `index`, the file for any other variant ("/account/guest.html").
	Path   string `json:"path"`
	File   string `json:"file"`   // the variant's module
	Output string `json:"output"` // its document among the artifacts
	// Document is the file as written — what the request for the page
	// carries: the HTML and whatever is inlined in it.
	Document Size `json:"document"`
	// Fetches is what a cold load of the page fetches of the build's own:
	// the document and the files it links. Packaging's.
	Fetches Fetches `json:"fetches"`
	// HTML is the page alone, without what packaging adds to it; CSS and JS
	// are what the page needs — the sizes are of its own sheet and script,
	// as analysis left them. The three do not depend on how they are
	// delivered.
	HTML Size   `json:"html"`
	CSS  *Asset `json:"css,omitempty"` // absent: the page has no `<style>`
	JS   *Asset `json:"js,omitempty"`  // absent: the page has no `<script>`
	// Components are the function components React called for the page.
	Components []Component `json:"components"`
	// Classes are the classes that `variants()` resolved for the page, in
	// the order first resolved, each with the components that resolved it.
	Classes []Class `json:"classes"`
	// Mounts are the page's behaviours, in the order they were first
	// mounted: a module and the element it is mounted on, once.
	Mounts []Mount `json:"mounts"`
	// Modules are the inputs of the page's script and their bytes in it:
	// they add up to JS.Raw. Absent for the control.
	Modules []Module `json:"modules,omitempty"`
	// Styles is what pruning did to the page's CSS, per source file, before
	// it was minified. Absent for the control.
	Styles *Styles `json:"styles,omitempty"`
}

// Asset is a page's CSS or JS: what the page needs of it — the Size — and
// how packaging delivers that: the blob that holds it.
type Asset struct {
	Size
	Delivery string `json:"delivery"`       // "inline", "file"
	URL      string `json:"url,omitempty"`  // of a file, under the base
	Pages    int    `json:"pages"`          // the pages the blob serves
	Blob     string `json:"blob,omitempty"` // its content hash
}

// Fetches is what packaging makes a cold load of a page fetch of the build's
// own files: Size is all of it — the document and each file it links, each
// compressed on its own — and CSS and JS the styles and the script among
// it, wherever they are: inlined or in a file, the page's own or shared.
// Against the page's CSS and JS — what it needs — they say what a page
// fetches for another's sake: nothing, while a blob is shared only when it
// is the same bytes.
type Fetches struct {
	Size
	Requests int      `json:"requests"` // the document and each file
	Files    []string `json:"files"`    // the files, by URL under the base
	CSS      Size     `json:"css"`
	JS       Size     `json:"js"`
}

type Component struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Class is a class that `variants()` of @reactogenic/core resolved while the
// page rendered (render.Page.Classes): a component's root, or a variant's
// own class. By are the components whose calls resolved it, in the order of
// their first call; none, for a call outside any component.
type Class struct {
	Name string   `json:"name"`
	By   []string `json:"by"`
}

type Mount struct {
	Module string          `json:"module"`
	ID     string          `json:"id,omitempty"`
	Flags  map[string]bool `json:"flags,omitempty"`
	// Data is what the use site hands the behaviour: the second argument of
	// the mount's call in the page's script.
	Data json.RawMessage `json:"data,omitempty"`
}

// Module is behaviors.ModuleBytes.
type Module struct {
	Module string `json:"module,omitempty"` // the specifier of the mount that brought the file
	Path   string `json:"path"`             // the file; `<entry>`, `<runtime>`
	Bytes  int    `json:"bytes"`
}

// Styles is cssprune.Stats: of the page's sheet and of its own `<style>`
// elements, whose row among Sources is `<style>`.
type Styles struct {
	Unpruned         bool     `json:"unpruned,omitempty"` // the page's CSS is whole, and Why says why: "the page has a <template>"
	Why              string   `json:"why,omitempty"`
	Rules            int      `json:"rules"`
	RulesDropped     int      `json:"rulesDropped"`
	Selectors        int      `json:"selectors"`
	SelectorsDropped int      `json:"selectorsDropped"`
	Properties       int      `json:"customPropertiesDropped"`
	Keyframes        int      `json:"keyframesDropped"`
	PositionTries    int      `json:"positionTryDropped"`
	Sources          []Source `json:"sources"`
}

// Source is the part of a page's CSS that one file wrote.
type Source struct {
	File         string `json:"file"`
	Rules        int    `json:"rules"`
	RulesDropped int    `json:"rulesDropped"`
	BytesIn      int    `json:"bytesIn"`  // before pruning, not minified
	BytesOut     int    `json:"bytesOut"` // after
}

// BlobReport is one blob of the site.
type BlobReport struct {
	Kind     string   `json:"kind"` // "css", "js"
	Hash     string   `json:"hash"`
	Size     Size     `json:"size"`
	Delivery string   `json:"delivery"`
	File     string   `json:"file,omitempty"` // in the output, when it is one
	Pages    []string `json:"pages"`          // the documents it serves, by path
}

func sizeOf(content string) Size {
	var b bytes.Buffer
	w, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
	io.WriteString(w, content)
	w.Close()
	return Size{Raw: len(content), Gzip: b.Len()}
}

// byteReport makes the report of a packaged site: documents are the pages as
// written, bare the same without what packaging put in them.
func byteReport(opts Options, site []built, blobs []*blob, of [][2]int, documents, bare []string, static []string) *Report {
	r := &Report{Base: opts.Base, Inline: opts.Inline, Specialize: !opts.NoSpecialize, Public: slices.Clone(static)}
	if r.Public == nil {
		r.Public = []string{}
	}
	sizes := make([]Size, len(blobs))
	delivery := func(b *blob) string {
		if b.file {
			return "file"
		}
		return "inline"
	}
	for i, b := range blobs {
		sizes[i] = sizeOf(b.content)
		out := BlobReport{Kind: b.kind, Hash: b.hash, Size: sizes[i], Delivery: delivery(b)}
		if b.file {
			out.File = b.path(opts.NoSpecialize)
		}
		for _, page := range b.pages {
			out.Pages = append(out.Pages, site[page].page.Path())
		}
		r.Blobs = append(r.Blobs, out)
	}
	// What a page needs is its own sheet and script, as analysis left them
	// (built.css, built.js); the blob says how that is delivered.
	needs := map[string]Size{}
	for i, b := range blobs {
		needs[b.content] = sizes[i]
	}
	asset := func(at int, own string) *Asset {
		if at < 0 {
			return nil
		}
		need, known := needs[own]
		if !known {
			need = sizeOf(own)
			needs[own] = need
		}
		b := blobs[at]
		a := &Asset{Size: need, Delivery: delivery(b), Pages: len(b.pages), Blob: b.hash}
		if b.file {
			a.URL = opts.Base + b.path(opts.NoSpecialize)
		}
		return a
	}
	names := naming{dir: opts.dir(), packages: map[string]string{}}
	for i, p := range site {
		file := p.page.File
		if rel, err := filepath.Rel(opts.dir(), filepath.FromSlash(file)); err == nil {
			file = filepath.ToSlash(rel)
		}
		page := PageReport{
			Pathname: p.page.Pathname, Variant: p.page.Name(), Path: p.page.Path(), File: file, Output: p.page.Output(),
			Document: sizeOf(documents[i]), HTML: sizeOf(bare[i]),
			CSS: asset(of[i][0], p.css), JS: asset(of[i][1], p.js),
			Components: []Component{}, Classes: []Class{}, Mounts: []Mount{},
		}
		// What the page fetches is read off its blobs — packaging's — not
		// off what it needs.
		page.Fetches = Fetches{Size: page.Document, Requests: 1, Files: []string{}}
		for k, at := range of[i] {
			if at < 0 {
				continue
			}
			kind := [2]*Size{&page.Fetches.CSS, &page.Fetches.JS}[k]
			kind.Raw, kind.Gzip = kind.Raw+sizes[at].Raw, kind.Gzip+sizes[at].Gzip
			if b := blobs[at]; b.file {
				page.Fetches.Requests++
				page.Fetches.Files = append(page.Fetches.Files, opts.Base+b.path(opts.NoSpecialize))
				page.Fetches.Raw, page.Fetches.Gzip = page.Fetches.Raw+sizes[at].Raw, page.Fetches.Gzip+sizes[at].Gzip
			}
		}
		for _, name := range slices.Sorted(maps.Keys(p.page.Components)) {
			page.Components = append(page.Components, Component{name, p.page.Components[name]})
		}
		for _, name := range p.page.Classes {
			by := p.page.Resolved[name]
			if by == nil {
				by = []string{}
			}
			page.Classes = append(page.Classes, Class{name, by})
		}
		// A module on an element once, however often the page's components
		// mounted it — every Dialog mounts `overlays` — with the flags any
		// of those mounts turned on and the data they hand it, as the
		// page's script has it.
		for _, m := range p.page.Mounts {
			at := slices.IndexFunc(page.Mounts, func(had Mount) bool { return had.Module == m.Module && had.ID == m.ID })
			if at < 0 {
				at = len(page.Mounts)
				page.Mounts = append(page.Mounts, Mount{Module: m.Module, ID: m.ID, Data: json.RawMessage(m.Data)})
			}
			for flag, on := range m.Flags {
				if page.Mounts[at].Flags == nil {
					page.Mounts[at].Flags = map[string]bool{}
				}
				page.Mounts[at].Flags[flag] = page.Mounts[at].Flags[flag] || on
			}
		}
		for _, m := range p.modules {
			page.Modules = append(page.Modules, Module{m.Module, names.of(m.Path), m.Bytes})
		}
		if s := p.styles; s != nil {
			page.Styles = &Styles{
				Unpruned: s.Unpruned, Why: s.Why, Rules: s.Rules, RulesDropped: s.RulesDropped,
				Selectors: s.Selectors, SelectorsDropped: s.SelectorsDropped,
				Properties: s.Properties, Keyframes: s.Keyframes, PositionTries: s.PositionTries, Sources: []Source{},
			}
			for _, src := range s.Sources {
				// Before the first source comment there is nothing of a file.
				if src.Name == "" && src.Rules == 0 {
					continue
				}
				page.Styles.Sources = append(page.Styles.Sources, Source{names.of(src.Name), src.Rules, src.RulesDropped, src.BytesIn, src.BytesOut})
			}
		}
		r.Pages = append(r.Pages, page)
	}
	return r
}

// naming names the files of the report: the inputs of a script, the source
// files of a sheet (builder.md, *The report*: files).
//
// esbuild names a file from the project directory, by where it is — links
// resolved. That is the file's name while it is the project's own
// (`site.css`). A package's file is somewhere of the install's choosing:
// `../../packages/ui/src/…` through a workspace's link,
// `node_modules/.pnpm/…` in pnpm's store, and through a link out of the
// project a chain of `../` to a path of this machine. So it is named by
// what it is on every machine: its package, and its path in the package —
// `@reactogenic/ui/src/dialog.css`.
type naming struct {
	dir      string            // the project directory
	packages map[string]string // a directory → the name of the package it is the root of; "": of none
}

// of is the name of a file that esbuild names name.
func (s *naming) of(name string) string {
	if name == behaviors.Entry || name == behaviors.Runtime || name == cssprune.OwnStyle {
		return name
	}
	file := filepath.FromSlash(name)
	if !filepath.IsAbs(file) {
		file = filepath.Join(s.dir, file)
	}
	rel, err := filepath.Rel(s.dir, file)
	if err != nil {
		rel = file // another volume
	}
	rel = filepath.ToSlash(rel)
	if filepath.IsLocal(filepath.FromSlash(rel)) && !slices.Contains(strings.Split(rel, "/"), "node_modules") {
		return rel // the project's own
	}
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		pkg, known := s.packages[dir]
		if !known {
			var manifest struct {
				Name string `json:"name"`
			}
			if text, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil && json.Unmarshal(text, &manifest) == nil {
				pkg = manifest.Name
			}
			s.packages[dir] = pkg
		}
		if pkg != "" {
			inside, _ := filepath.Rel(dir, file)
			return pkg + "/" + filepath.ToSlash(inside)
		}
		if filepath.Dir(dir) == dir {
			// Of no package, and not the project's: where it is.
			return rel
		}
	}
}

// JSON is the report as `<out>/_rg/report.json` holds it.
func (r *Report) JSON() ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false) // `<entry>` is a name, not markup
	e.SetIndent("", "  ")
	if err := e.Encode(r); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Print writes the report as `--report` shows it: per page, the bytes of its
// HTML, CSS and JS — what it needs — raw and gzip, and how each is
// delivered; what it fetches; the components it rendered and the classes
// their variants resolved; the behaviours it mounted, with their flags and
// their data; the bytes of its script by module; the rules of its CSS kept
// and dropped, by source file.
func (r *Report) Print(w io.Writer) {
	for _, p := range r.Pages {
		fmt.Fprintf(w, "%s  %s\n", p.Path, p.File)
		fmt.Fprintf(w, "  %-10s %8s %8s\n", "", "raw", "gzip")
		row := func(name string, size Size, note string) {
			fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("  %-10s %8d %8d   %s", name, size.Raw, size.Gzip, note), " "))
		}
		row("HTML", p.HTML, "")
		for _, a := range []struct {
			name  string
			asset *Asset
		}{{"CSS", p.CSS}, {"JS", p.JS}} {
			if a.asset == nil {
				fmt.Fprintf(w, "  %-10s %8s %8s   none\n", a.name, "-", "-")
				continue
			}
			note := "inline"
			if a.asset.Delivery == "file" {
				note = a.asset.URL
			}
			if a.asset.Pages > 1 {
				note += fmt.Sprintf(", the same on %d pages", a.asset.Pages)
			}
			row(a.name, a.asset.Size, note)
		}
		row("document", p.Document, p.Output)
		// Packaging's side: what a cold load fetches, against what analysis
		// says the page needs. The control has no analysis to compare with.
		fetched := fmt.Sprintf("%d request", p.Fetches.Requests)
		if p.Fetches.Requests != 1 {
			fetched += "s"
		}
		if r.Specialize {
			var css, js int
			if p.CSS != nil {
				css = p.CSS.Raw
			}
			if p.JS != nil {
				js = p.JS.Raw
			}
			if more := [2]int{p.Fetches.CSS.Raw - css, p.Fetches.JS.Raw - js}; more == [2]int{} {
				fetched += ": what the page needs, and no more"
			} else {
				fetched += fmt.Sprintf(": %d B of CSS and %d B of JS more than the page needs", more[0], more[1])
			}
		}
		row("fetches", p.Fetches.Size, fetched)

		list := func(name string, lines []string) {
			for i, line := range lines {
				if i > 0 {
					name = ""
				}
				fmt.Fprintf(w, "  %-10s %s\n", name, line)
			}
		}
		var components []string
		for _, c := range p.Components {
			components = append(components, fmt.Sprintf("%s ×%d", c.Name, c.Count))
		}
		if len(components) > 0 {
			list("components", []string{strings.Join(components, ", ")})
		}
		// The classes `variants()` resolved, by who resolved them.
		var classes, callers []string
		for _, c := range p.Classes {
			by := strings.Join(c.By, ", ")
			at := slices.Index(callers, by)
			if at < 0 {
				at = len(callers)
				callers, classes = append(callers, by), append(classes, by+":")
			}
			classes[at] += " " + c.Name
		}
		list("classes", classes)
		var mounts []string
		for _, m := range p.Mounts {
			line := m.Module
			if m.ID != "" {
				line += " #" + m.ID
			}
			for _, flag := range slices.Sorted(maps.Keys(m.Flags)) {
				line += fmt.Sprintf(" %s=%v", flag, m.Flags[flag])
			}
			if len(m.Data) > 0 {
				line += " " + string(m.Data)
			}
			mounts = append(mounts, line)
		}
		list("behaviours", mounts)
		var modules []string
		for _, m := range p.Modules {
			line := fmt.Sprintf("%6d B  %s", m.Bytes, m.Path)
			if m.Path == behaviors.Entry {
				line += "  the mount calls"
			} else if m.Path == behaviors.Runtime {
				line += "  esbuild's helpers"
			}
			modules = append(modules, line)
		}
		list("JS bytes", modules)
		if s := p.Styles; s != nil {
			var sources []string
			if s.Unpruned {
				sources = append(sources, "not pruned: "+s.Why)
			}
			for _, src := range s.Sources {
				line := fmt.Sprintf("%3d kept, %3d dropped  %s", src.Rules-src.RulesDropped, src.RulesDropped, src.File)
				if src.File == cssprune.OwnStyle {
					line += "  the page's own, in its HTML"
				}
				sources = append(sources, line)
			}
			list("CSS rules", sources)
		}
		fmt.Fprintln(w)
	}
	for _, b := range r.Blobs {
		where := "inline"
		if b.File != "" {
			where = b.File
		}
		fmt.Fprintf(w, "%-4s %s %8d %8d   %-24s %s\n", b.Kind, b.Hash, b.Size.Raw, b.Size.Gzip, where, strings.Join(b.Pages, " "))
	}
}
