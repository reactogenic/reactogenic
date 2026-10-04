package build

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/reactogenic/reactogenic/go/internal/build/behaviors"
)

// Report is the byte report of a build (builder.md, *The report*): what each
// page ships, how, and why each byte is there. It is written to
// `<out>/_rg/report.json`, and `--report` prints it.
//
// Nothing in it names the machine: files are named from the project
// directory, as esbuild names them.
type Report struct {
	Base       string       `json:"base"`
	Inline     string       `json:"inline"`
	Specialize bool         `json:"specialize"` // false: the control (`--no-specialize`)
	Pages      []PageReport `json:"pages"`
	Blobs      []BlobReport `json:"blobs"`
	Public     []string     `json:"public"` // the files copied from `public/`
}

// Size is a number of bytes as written, and after gzip (level 9). Brotli is
// not here: Go has no encoder for it, and the builder takes no dependency to
// count with — the measuring scripts have it (plan.md, RGP2-050).
type Size struct {
	Raw  int `json:"raw"`
	Gzip int `json:"gzip"`
}

// PageReport is one page.
type PageReport struct {
	Pathname string `json:"pathname"`
	File     string `json:"file"`   // the page's module
	Output   string `json:"output"` // its file in the output
	// Document is the file as written — what the request for the page
	// carries: the HTML and whatever is inlined in it.
	Document Size `json:"document"`
	// HTML is the page alone, without what packaging adds to it; CSS and JS
	// are its blobs. The three do not depend on how the blobs are delivered.
	HTML Size   `json:"html"`
	CSS  *Asset `json:"css,omitempty"` // absent: the page has no `<style>`
	JS   *Asset `json:"js,omitempty"`  // absent: the page has no `<script>`
	// Components are the function components React called for the page.
	Components []Component `json:"components"`
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

// Asset is a page's CSS or JS: a blob, and how the page gets it.
type Asset struct {
	Size
	Delivery string `json:"delivery"`       // "inline", "file"
	URL      string `json:"url,omitempty"`  // of a file, under the base
	Pages    int    `json:"pages"`          // the pages the blob serves
	Blob     string `json:"blob,omitempty"` // its content hash
}

type Component struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Mount struct {
	Module string          `json:"module"`
	ID     string          `json:"id,omitempty"`
	Flags  map[string]bool `json:"flags,omitempty"`
}

// Module is behaviors.ModuleBytes.
type Module struct {
	Module string `json:"module,omitempty"` // the specifier of the mount that brought the file
	Path   string `json:"path"`             // the file; `<entry>`, `<runtime>`
	Bytes  int    `json:"bytes"`
}

// Styles is cssprune.Stats.
type Styles struct {
	Unpruned         bool     `json:"unpruned,omitempty"` // the page has a `<template>` or the like: its CSS is whole
	Rules            int      `json:"rules"`
	RulesDropped     int      `json:"rulesDropped"`
	Selectors        int      `json:"selectors"`
	SelectorsDropped int      `json:"selectorsDropped"`
	Properties       int      `json:"customPropertiesDropped"`
	Keyframes        int      `json:"keyframesDropped"`
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
	Pages    []string `json:"pages"`
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
			out.Pages = append(out.Pages, site[page].page.Pathname)
		}
		r.Blobs = append(r.Blobs, out)
	}
	asset := func(at int) *Asset {
		if at < 0 {
			return nil
		}
		b := blobs[at]
		a := &Asset{Size: sizes[at], Delivery: delivery(b), Pages: len(b.pages), Blob: b.hash}
		if b.file {
			a.URL = opts.Base + b.path(opts.NoSpecialize)
		}
		return a
	}
	for i, p := range site {
		file := p.page.File
		if rel, err := filepath.Rel(opts.dir(), filepath.FromSlash(file)); err == nil {
			file = filepath.ToSlash(rel)
		}
		page := PageReport{
			Pathname: p.page.Pathname, File: file, Output: output(p.page.Pathname),
			Document: sizeOf(documents[i]), HTML: sizeOf(bare[i]),
			CSS: asset(of[i][0]), JS: asset(of[i][1]),
			Components: []Component{}, Mounts: []Mount{},
		}
		for _, name := range slices.Sorted(maps.Keys(p.page.Components)) {
			page.Components = append(page.Components, Component{name, p.page.Components[name]})
		}
		// A module on an element once, however often the page's components
		// mounted it — every Dialog mounts `overlays` — with the flags any
		// of those mounts turned on, as the page's script has it.
		for _, m := range p.page.Mounts {
			at := slices.IndexFunc(page.Mounts, func(had Mount) bool { return had.Module == m.Module && had.ID == m.ID })
			if at < 0 {
				at = len(page.Mounts)
				page.Mounts = append(page.Mounts, Mount{Module: m.Module, ID: m.ID})
			}
			for flag, on := range m.Flags {
				if page.Mounts[at].Flags == nil {
					page.Mounts[at].Flags = map[string]bool{}
				}
				page.Mounts[at].Flags[flag] = page.Mounts[at].Flags[flag] || on
			}
		}
		for _, m := range p.modules {
			page.Modules = append(page.Modules, Module{m.Module, m.Path, m.Bytes})
		}
		if s := p.styles; s != nil {
			page.Styles = &Styles{
				Unpruned: s.Unpruned, Rules: s.Rules, RulesDropped: s.RulesDropped,
				Selectors: s.Selectors, SelectorsDropped: s.SelectorsDropped,
				Properties: s.Properties, Keyframes: s.Keyframes, Sources: []Source{},
			}
			for _, src := range s.Sources {
				// Before the first source comment there is nothing of a file.
				if src.Name == "" && src.Rules == 0 {
					continue
				}
				page.Styles.Sources = append(page.Styles.Sources, Source{src.Name, src.Rules, src.RulesDropped, src.BytesIn, src.BytesOut})
			}
		}
		r.Pages = append(r.Pages, page)
	}
	return r
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
// HTML, CSS and JS, raw and gzip, and how each is delivered; the components
// it rendered; the behaviours it mounted, with their flags; the bytes of its
// script by module; the rules of its CSS kept and dropped, by source file.
func (r *Report) Print(w io.Writer) {
	for _, p := range r.Pages {
		fmt.Fprintf(w, "%s  %s\n", p.Pathname, p.File)
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
		var mounts []string
		for _, m := range p.Mounts {
			line := m.Module
			if m.ID != "" {
				line += " #" + m.ID
			}
			for _, flag := range slices.Sorted(maps.Keys(m.Flags)) {
				line += fmt.Sprintf(" %s=%v", flag, m.Flags[flag])
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
				sources = append(sources, "not pruned: the page has a <template>, a <noscript> or markup in a <select>")
			}
			for _, src := range s.Sources {
				sources = append(sources, fmt.Sprintf("%3d kept, %3d dropped  %s", src.Rules-src.RulesDropped, src.RulesDropped, src.File))
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
