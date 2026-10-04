// Package behaviors builds the JS of a page: the behaviours its components
// mounted, and nothing else (specs/phase02/builder.md, *Behaviours*) — and
// the one script of the control, which knows nothing of the page it runs on
// (*The control*).
//
// esbuild is the linker, through its public API, in memory. What is ours is
// what it cannot know: which modules a page mounted, on which elements, and
// which of their flags the page turned on.
package behaviors

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// Options is the site the pages are of.
type Options struct {
	// Dir is the project directory, absolute: the specifier of a `mount()`
	// is resolved from it, by esbuild — as an import written in a file of
	// that directory would be.
	Dir string
	// Base is `--base`, the path the site is served under: the control
	// looks a page up by `location.pathname`, which carries it. "" is "/".
	Base string
	// Cache keeps what is learned of a behaviour module for the next page
	// that mounts it. Optional: without one, each call learns it again.
	Cache *Cache
}

// ModuleBytes is one input of a page's script and what it costs there: the
// *why is this byte here* of the report (builder.md, *The report*).
type ModuleBytes struct {
	// Module is the specifier of the `mount()` that brought the file, for a
	// mounted module's own file; "" for a file one of them imports, and for
	// the generated entry.
	Module string
	// Path is the file, from the project directory, as esbuild's metafile
	// names it; Entry for the generated entry.
	Path string
	// Bytes are the file's bytes in the script; 0 for a file that is in the
	// graph and left nothing — a feature the page's flags turned off.
	Bytes int
}

// Entry is the Path of the generated entry among a script's ModuleBytes: the
// mount calls.
const Entry = "<entry>"

// Build returns the script of one page — "" when the page mounted nothing:
// no `<script>` — and what each input costs in it, the mounted modules
// first, in the order of the entry, then what they import, then the entry.
//
// The reports are mount-not-found, mount-no-element (an id is looked up in
// page.HTML), mount-flag, mount-side-effect and mount-error; with any, there
// is no script.
func Build(page render.Page, opts Options) (js string, modules []ModuleBytes, reports []report.Report) {
	p := planOf(page)
	if len(p.specs) == 0 {
		return "", nil, nil
	}
	mods := opts.modules(p.specs)
	if reports = mistakes(page, p, mods, opts); len(reports) > 0 {
		return "", nil, reports
	}
	// Every flag is defined: one the builder did not define would be a
	// ReferenceError when the page loads. On only when a mount of this page
	// turned it on — the union over the page's mounts.
	define := map[string]string{}
	for _, m := range mods {
		for flag := range m.flags {
			define[flag] = strconv.FormatBool(p.on[flag])
		}
	}
	js, meta, errs := opts.bundle(p.entry(), define)
	if len(errs) > 0 {
		return "", nil, opts.buildErrors(page, p.specs, errs)
	}

	// The bytes, by input. Nothing may be in the script that no mounted
	// module brought: a module nobody mounted is not among the inputs.
	mounted := map[string]string{} // a file → the specifier, or "" for an imported one
	for i, m := range mods {
		for _, file := range m.files {
			if _, ok := mounted[file]; !ok {
				mounted[file] = ""
			}
		}
		mounted[m.file] = p.specs[i]
	}
	bytes := meta.bytes()
	for _, file := range meta.files() {
		if _, ok := mounted[file]; !ok {
			reports = append(reports, report.Page(page.File, page.Pathname, "internal", fmt.Sprintf("`%s` is in the page's script and no mounted module imports it", file)))
		}
	}
	if len(reports) > 0 {
		return "", nil, reports
	}
	var imported []string
	for _, m := range mods {
		modules = append(modules, ModuleBytes{Module: mounted[m.file], Path: m.file, Bytes: bytes[m.file]})
		for _, file := range m.files {
			if mounted[file] == "" && !slices.Contains(imported, file) {
				imported = append(imported, file)
			}
		}
	}
	slices.Sort(imported)
	for _, file := range imported {
		modules = append(modules, ModuleBytes{Path: file, Bytes: bytes[file]})
	}
	modules = append(modules, ModuleBytes{Path: Entry, Bytes: bytes[stdin]})
	return js, modules, nil
}

// BuildControl returns the one script of a site built with --no-specialize:
// every module mounted on any page, every flag on, and a table from pathname
// to that page's mounts, applied to `location.pathname`. It is what a build
// that does not know what a page rendered ships: the same behaviour on every
// page, at the cost the per-page scripts are measured against.
//
// The reports are those of Build, for every page. A site that mounts nothing
// has no script.
func BuildControl(pages []render.Page, opts Options) (js string, reports []report.Report) {
	var site plan           // the modules of the site, in the order they are first mounted
	var first []render.Page // per module, the first page that mounts it
	type row struct {
		pathname string
		calls    []call
	}
	var table []row
	for _, page := range pages {
		p := planOf(page)
		if len(p.specs) == 0 {
			continue
		}
		reports = append(reports, mistakes(page, p, opts.modules(p.specs), opts)...)
		r := row{pathname: strings.TrimSuffix(base(opts.Base), "/") + page.Pathname}
		for _, c := range p.calls {
			i := slices.Index(site.specs, p.specs[c.module])
			if i < 0 {
				i = len(site.specs)
				site.specs, first = append(site.specs, p.specs[c.module]), append(first, page)
			}
			r.calls = append(r.calls, call{i, c.id})
		}
		table = append(table, r)
	}
	if len(site.specs) == 0 || len(reports) > 0 {
		return "", reports
	}
	define := map[string]string{}
	for _, m := range opts.modules(site.specs) {
		for flag := range m.flags {
			define[flag] = "true"
		}
	}

	// import m0 from "…";
	// …
	// const m = [m0, …], t = {"/guide/": [[0, "m1"], [1]], …};
	// for (const [i, id] of t[<pathname>] || []) id ? m[i](document.getElementById(id)) : m[i]();
	var entry strings.Builder
	site.imports(&entry)
	entry.WriteString("const m = [")
	for i := range site.specs {
		fmt.Fprintf(&entry, "m%d, ", i)
	}
	entry.WriteString("], t = {\n")
	for _, r := range table {
		fmt.Fprintf(&entry, "  %s: [", quote(r.pathname))
		for _, c := range r.calls {
			if c.id == "" {
				fmt.Fprintf(&entry, "[%d], ", c.module)
			} else {
				fmt.Fprintf(&entry, "[%d, %s], ", c.module, quote(c.id))
			}
		}
		entry.WriteString("],\n")
	}
	entry.WriteString("};\nfor (const [i, id] of t[" + controlPathname + "] || []) id ? m[i](document.getElementById(id)) : m[i]();\n")

	js, _, errs := opts.bundle(entry.String(), define)
	for _, e := range errs {
		// An error of a module's import is of the first page that mounts it.
		page := first[0]
		if l := e.Location; l != nil && l.File == stdin && l.Line >= 1 && l.Line <= len(first) {
			page = first[l.Line-1]
		}
		reports = append(reports, opts.buildErrors(page, site.specs, []api.Message{e})...)
	}
	if len(reports) > 0 {
		return "", reports
	}
	return js, nil
}

// plan is a page's mounts as its script runs them.
type plan struct {
	specs []string        // the distinct modules, in the order they are first mounted: one import each
	calls []call          // in render order
	on    map[string]bool // the flags a mount turned on
}

// call is one mount call of the entry: a per-root module once per element it
// is mounted on; a page-level module — mounted without an id — once per
// page, however often it is mounted.
type call struct {
	module int    // index in specs
	id     string // "" for a page-level module
}

func planOf(page render.Page) plan {
	p := plan{on: map[string]bool{}}
	for _, m := range page.Mounts {
		i := slices.Index(p.specs, m.Module)
		if i < 0 {
			i = len(p.specs)
			p.specs = append(p.specs, m.Module)
		}
		// Two mounts of one module on one element are one: its listeners
		// would be there twice.
		if c := (call{i, m.ID}); !slices.Contains(p.calls, c) {
			p.calls = append(p.calls, c)
		}
		for flag, on := range m.Flags {
			p.on[flag] = p.on[flag] || on
		}
	}
	return p
}

// imports writes one import per module, each on a line of its own: line i+1
// of an entry is the import of specs[i] (buildErrors).
func (p plan) imports(entry *strings.Builder) {
	for i, spec := range p.specs {
		fmt.Fprintf(entry, "import m%d from %s;\n", i, quote(spec))
	}
}

// entry is the generated entry of a page (builder.md, *Behaviours*):
//
//	import m0 from "@reactogenic/ui/behaviors/menu-keys";
//	import m1 from "@reactogenic/ui/behaviors/overlays";
//	m0(document.getElementById("m1"));
//	m0(document.getElementById("m2"));
//	m1();
func (p plan) entry() string {
	var entry strings.Builder
	p.imports(&entry)
	for _, c := range p.calls {
		if c.id == "" {
			fmt.Fprintf(&entry, "m%d();\n", c.module)
		} else {
			fmt.Fprintf(&entry, "m%d(document.getElementById(%s));\n", c.module, quote(c.id))
		}
	}
	return entry.String()
}

// controlPathname is the page the control's script is on, as the table
// names it: a page answers to `/guide/`, to `/guide` on a host that does not
// redirect, and to the file it is written to, `/guide/index.html`.
const controlPathname = `location.pathname.replace(/(\/index\.html|\/)?$/, "/")`

// mistakes returns what is wrong with a page's mounts, in render order.
func mistakes(page render.Page, p plan, mods []*module, opts Options) []report.Report {
	var reports []report.Report
	ids := elementIDs(page.HTML)
	said := map[string]bool{}
	once := func(key string) bool {
		first := !said[key]
		said[key] = true
		return first
	}
	for _, m := range page.Mounts {
		mod := mods[slices.Index(p.specs, m.Module)]
		mount := fmt.Sprintf("`mount(%q)`", m.Module)
		if once(m.Module) {
			switch {
			case mod.notFound:
				reports = append(reports, report.Page(page.File, page.Pathname, "mount-not-found", mount+": the module does not resolve from the project directory"))
			case len(mod.errors) > 0:
				reports = append(reports, opts.buildErrors(page, nil, mod.errors)...)
			case len(mod.effects) > 0:
				reports = append(reports, opts.sideEffects(page, mod)...)
			}
		}
		if m.ID != "" && !ids[m.ID] && once(m.Module+"\x00id\x00"+m.ID) {
			reports = append(reports, report.Page(page.File, page.Pathname, "mount-no-element", fmt.Sprintf("%s: no element of the page has `id=%q`", mount, m.ID)))
		}
		if mod.flags == nil {
			continue // not read: its flags are not known
		}
		for _, flag := range sortedKeys(m.Flags) {
			if !mod.flags[flag] && once(m.Module+"\x00flag\x00"+flag) {
				reports = append(reports, report.Page(page.File, page.Pathname, "mount-flag", fmt.Sprintf("%s: the module does not declare the flag `%s`", mount, flag)))
			}
		}
	}
	return reports
}

// elementIDs are the ids of a page's elements, a template's content aside:
// what `document.getElementById` finds when the script runs.
func elementIDs(src string) map[string]bool {
	ids := map[string]bool{}
	z := html.NewTokenizer(strings.NewReader(src))
	template := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ids
		case html.EndTagToken:
			if name, _ := z.TagName(); string(name) == "template" && template > 0 {
				template--
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, more := z.TagName()
			if string(name) == "template" {
				template++
				continue
			}
			for more {
				var key, value []byte
				if key, value, more = z.TagAttr(); string(key) == "id" && len(value) > 0 && template == 0 {
					ids[string(value)] = true
				}
			}
		}
	}
}

// quote writes a JS string literal: JSON is one.
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// base normalises `--base`: "", "docs", "/docs" and "/docs/" → "/", "/docs/".
func base(b string) string {
	return strings.TrimSuffix("/"+strings.Trim(b, "/"), "/") + "/"
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// stdin is the generated entry's name in esbuild's messages and metafile.
const stdin = "<stdin>"

// bundle is the one esbuild build of a script (builder.md, *Behaviours*):
// the generated entry, bundled and minified with the page's flags defined,
// as an ES module for `<script type="module">`. No splitting, nothing
// written.
func (o Options) bundle(entry string, define map[string]string) (js string, meta metafile, errors []api.Message) {
	return o.esbuild(entry, func(b *api.BuildOptions) {
		b.MinifyWhitespace, b.MinifyIdentifiers, b.MinifySyntax = true, true, true
		b.Define = define
	})
}

// esbuild bundles a generated entry, in memory; set says what the build is
// for. The output is returned without the line break esbuild ends a file
// with: a byte of nobody's.
func (o Options) esbuild(entry string, set func(*api.BuildOptions)) (js string, meta metafile, errors []api.Message) {
	options := api.BuildOptions{
		Stdin:         &api.StdinOptions{Contents: entry, ResolveDir: o.Dir, Sourcefile: stdin, Loader: api.LoaderJS},
		AbsWorkingDir: o.Dir,
		Bundle:        true,
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		Target:        api.ES2022,
		Charset:       api.CharsetUTF8, // a module script is UTF-8, whatever the page says
		Metafile:      true,
		LegalComments: api.LegalCommentsNone,
		LogLevel:      api.LogLevelSilent,
	}
	set(&options)
	result := api.Build(options)
	if len(result.Errors) > 0 {
		return "", metafile{}, result.Errors
	}
	if err := json.Unmarshal([]byte(result.Metafile), &meta); err != nil || len(result.OutputFiles) != 1 {
		return "", metafile{}, []api.Message{{Text: fmt.Sprintf("esbuild: %d outputs, metafile: %v", len(result.OutputFiles), err)}}
	}
	return strings.TrimSuffix(string(result.OutputFiles[0].Contents), "\n"), meta, nil
}

// metafile is what is read of esbuild's.
type metafile struct {
	Inputs map[string]struct {
		Imports []struct {
			Path string `json:"path"`
		} `json:"imports"`
	} `json:"inputs"`
	Outputs map[string]struct {
		Inputs map[string]struct {
			BytesInOutput int `json:"bytesInOutput"`
		} `json:"inputs"`
	} `json:"outputs"`
}

// files are the inputs beside the entry, sorted.
func (m metafile) files() []string {
	var files []string
	for file := range m.Inputs {
		if file != stdin {
			files = append(files, file)
		}
	}
	slices.Sort(files)
	return files
}

// bytes are the bytes each input left in the output.
func (m metafile) bytes() map[string]int {
	bytes := map[string]int{}
	for _, out := range m.Outputs {
		for file, in := range out.Inputs {
			bytes[file] += in.BytesInOutput
		}
	}
	return bytes
}

// buildErrors turns esbuild's errors into reports: at the file and position
// esbuild names, with the page as a related line; an error of the generated
// entry — line i of it imports specs[i-1] — at the page, as its mount's.
func (o Options) buildErrors(page render.Page, specs []string, errors []api.Message) []report.Report {
	var reports []report.Report
	for _, e := range errors {
		l := e.Location
		switch {
		case l == nil:
			reports = append(reports, report.Page(page.File, page.Pathname, "mount-error", e.Text))
		case l.File == stdin:
			text := e.Text
			if l.Line >= 1 && l.Line <= len(specs) {
				text = fmt.Sprintf("`mount(%q)`: %s", specs[l.Line-1], text)
			}
			reports = append(reports, report.Page(page.File, page.Pathname, "mount-error", text))
		default:
			reports = append(reports, report.Report{
				File: filepath.ToSlash(filepath.Join(o.Dir, l.File)), Line: l.Line, Col: l.Column + 1, Code: "mount-error", Message: e.Text,
				Related: []report.Report{{Severity: report.Message, Message: "mounted on the page " + page.Pathname}},
			})
		}
	}
	return reports
}
