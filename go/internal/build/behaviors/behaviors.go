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
	"net/url"
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
	// that directory would be. It may be reached through a symbolic link.
	Dir string
	// Base is `--base`, the path the site is served under: the control
	// looks a page up by `location.pathname`, which carries it. "" is "/".
	// Percent-encoded or not: the table is keyed by the names of the
	// directories, as the script decodes the pathname.
	Base string
	// Cache keeps what is learned of a behaviour module for the next page
	// that mounts it. Optional: without one, each call learns it again.
	Cache *Cache

	// root is Dir with its symbolic links resolved (resolved): the directory
	// esbuild names files from — it resolves the links of its working
	// directory, and a path of its metafile is from there.
	root string
}

// resolved is the options of a build that has started.
func (o Options) resolved() Options {
	o.root = o.Dir
	if real, err := filepath.EvalSymlinks(o.Dir); err == nil {
		o.root = real
	}
	return o
}

// ModuleBytes is one input of a page's script and what it costs there: the
// *why is this byte here* of the report (builder.md, *The report*).
type ModuleBytes struct {
	// Module is the specifier of the `mount()` that brought the file — the
	// first, when mounts spell one file differently — for a mounted module's
	// own file; "" for a file one of them imports, for the generated entry
	// and for esbuild's helpers.
	Module string
	// Path is the file, from the project directory, as esbuild's metafile
	// names it; Entry for the generated entry, Runtime for the helpers.
	Path string
	// Bytes are the file's bytes in the script; 0 for a file that is in the
	// graph and left nothing — a feature the page's flags turned off.
	Bytes int
}

// Entry is the Path of the generated entry among a script's ModuleBytes: the
// mount calls.
const Entry = "<entry>"

// Runtime is the Path of what is in a script and of no input: the helpers
// esbuild adds for what a module's syntax needs — a dynamic `import()` in a
// bundle that is not split. A script without them has no such row.
const Runtime = "<runtime>"

// Build returns the script of one page — "" when the page mounted nothing:
// no `<script>` — and what each input costs in it, the mounted modules
// first, in the order of the entry, then what they import, then the entry,
// then esbuild's helpers if there are any: the bytes add up to the script.
//
// The reports are mount-not-found, mount-no-element (an id is looked up in
// page.HTML), mount-kind, mount-flag, mount-side-effect and mount-error; with
// any, there is no script.
func Build(page render.Page, opts Options) (js string, modules []ModuleBytes, reports []report.Report) {
	opts = opts.resolved()
	p := planOf(page)
	if len(p.specs) == 0 {
		return "", nil, nil
	}
	mods := opts.modules(p.specs)
	if reports = mistakes(page, p, mods, opts); len(reports) > 0 {
		return "", nil, reports
	}
	p, mods = p.byFile(mods)
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
	if reports = undefined(page, js); len(reports) > 0 {
		return "", nil, reports
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
	// What is of no input is esbuild's own.
	rest := len(js)
	for _, m := range modules {
		rest -= m.Bytes
	}
	switch {
	case rest < 0:
		return "", nil, []report.Report{report.Page(page.File, page.Pathname, "internal", fmt.Sprintf("the inputs of the page's script add up to %d B, the script is %d B", len(js)-rest, len(js)))}
	case rest > 0:
		modules = append(modules, ModuleBytes{Path: Runtime, Bytes: rest})
	}
	return js, modules, nil
}

// undefined reports the flags a built script reads and nothing defined
// (undefinedFlags): with one, there is no script.
func undefined(page render.Page, js string) []report.Report {
	var reports []report.Report
	for _, flag := range undefinedFlags(js) {
		reports = append(reports, report.Page(page.File, page.Pathname, "mount-flag", fmt.Sprintf("the page's script reads the flag `%s`, which is not defined: the builder finds a flag by its name in the text of a module, written without an escape", flag)))
	}
	return reports
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
	opts = opts.resolved()
	var site plan           // the modules of the site, in the order they are first mounted
	var files []string      // per module, its file: a module is one, however a page spells it
	var first []render.Page // per module, the first page that mounts it
	define := map[string]string{}
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
		mods := opts.modules(p.specs)
		if wrong := mistakes(page, p, mods, opts); len(wrong) > 0 {
			reports = append(reports, wrong...)
			continue
		}
		p, mods = p.byFile(mods)
		// The key is what the script makes of `location.pathname`: decoded.
		// So the base is — `/caf%C3%A9/` and `/café/` are one directory, and
		// the browser reports either as the first.
		under := strings.TrimSuffix(base(opts.Base), "/")
		if decoded, err := url.PathUnescape(under); err == nil {
			under = decoded
		}
		r := row{pathname: under + page.Pathname}
		for _, c := range p.calls {
			i := slices.Index(files, mods[c.module].file)
			if i < 0 {
				i = len(files)
				files, site.specs, first = append(files, mods[c.module].file), append(site.specs, p.specs[c.module]), append(first, page)
				for flag := range mods[c.module].flags {
					define[flag] = "true"
				}
			}
			r.calls = append(r.calls, call{i, c.id})
		}
		table = append(table, r)
	}
	if len(site.specs) == 0 || len(reports) > 0 {
		return "", reports
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
	if reports = undefined(first[0], js); len(reports) > 0 {
		return "", reports
	}
	return js, nil
}

// plan is a page's mounts as its script runs them.
type plan struct {
	specs []string        // the distinct modules, in the order they are first mounted: one import each. As the mounts spell them until they are read; then by file (byFile)
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

// byFile is the plan with its modules distinct by the file they resolve to,
// once they are read: two spellings of one module are one import — under the
// first — and their calls the calls of one module, so a page-level
// behaviour still runs once.
func (p plan) byFile(mods []*module) (plan, []*module) {
	q := plan{on: p.on}
	var distinct []*module
	to := make([]int, len(mods)) // an index in p.specs → in q.specs
	for i, m := range mods {
		to[i] = slices.IndexFunc(distinct, func(d *module) bool { return d.file == m.file })
		if to[i] < 0 {
			to[i] = len(distinct)
			distinct, q.specs = append(distinct, m), append(q.specs, p.specs[i])
		}
	}
	for _, c := range p.calls {
		if c.module = to[c.module]; !slices.Contains(q.calls, c) {
			q.calls = append(q.calls, c)
		}
	}
	return q, distinct
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
// redirect, and to the file it is written to, `/guide/index.html`. The table
// names a page as its directories are named, `location.pathname` as the
// browser encodes that (`/se%C3%B1or/`): it is decoded.
const controlPathname = `decodeURIComponent(location.pathname).replace(/(\/index\.html|\/)?$/, "/")`

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
	// A behaviour is of an element or of the page (builder.md, *Behaviours*,
	// module): per module — the file, however the mounts spell it — the
	// first id it is mounted on, and whether it is mounted without one.
	type kind struct {
		id   string
		page bool
	}
	kinds := map[string]*kind{}
	module := func(m render.Mount) string {
		if file := mods[slices.Index(p.specs, m.Module)].file; file != "" {
			return "\x00file\x00" + file
		}
		return m.Module
	}
	for _, m := range page.Mounts {
		k := kinds[module(m)]
		if k == nil {
			k = &kind{}
			kinds[module(m)] = k
		}
		if m.ID == "" {
			k.page = true
		} else if k.id == "" {
			k.id = m.ID
		}
	}
	for _, m := range page.Mounts {
		mod := mods[slices.Index(p.specs, m.Module)]
		mount := fmt.Sprintf("`mount(%q)`", m.Module)
		// What is wrong with a module is said once, however the mounts spell it.
		if once(m.Module) && (mod.file == "" || once("\x00file\x00"+mod.file)) {
			switch {
			case mod.notFound:
				reports = append(reports, report.Page(page.File, page.Pathname, "mount-not-found", mount+": the module does not resolve from the project directory"))
			case mod.unread != "":
				reports = append(reports, report.Page(page.File, page.Pathname, "internal", mount+": "+mod.unread))
			case len(mod.errors) > 0:
				reports = append(reports, opts.buildErrors(page, nil, mod.errors)...)
			case len(mod.effects) > 0:
				reports = append(reports, opts.sideEffects(page, mod)...)
			}
		}
		if m.ID != "" && !ids[m.ID] && once(m.Module+"\x00id\x00"+m.ID) {
			reports = append(reports, report.Page(page.File, page.Pathname, "mount-no-element", fmt.Sprintf("%s: no element of the page has `id=%q`", mount, m.ID)))
		}
		// Its page-level call would hand the function no root: a TypeError
		// that ends the script, and every mount after it.
		if k := kinds[module(m)]; k.page && k.id != "" && once(module(m)+"\x00kind") {
			reports = append(reports, report.Page(page.File, page.Pathname, "mount-kind", fmt.Sprintf("%s: the module is mounted on an element (`%s`) and without one: a behaviour is of an element or of the page", mount, k.id)))
		}
		if mod.flags == nil {
			continue // not read: its flags are not known
		}
		for _, flag := range sortedKeys(m.Flags) {
			if !mod.flags[flag] && once(m.Module+"\x00flag\x00"+flag) {
				reports = append(reports, report.Page(page.File, page.Pathname, "mount-flag", fmt.Sprintf("%s: the module has no flag `%s`", mount, flag)))
			}
		}
	}
	return reports
}

// elementIDs are the ids of a page's elements, a template's content aside —
// the template itself is an element of the page: what
// `document.getElementById` finds when the script runs.
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
			for more {
				var key, value []byte
				if key, value, more = z.TagAttr(); string(key) == "id" && len(value) > 0 && template == 0 {
					ids[string(value)] = true
				}
			}
			if string(name) == "template" {
				template++
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
		Stdin:         &api.StdinOptions{Contents: entry, ResolveDir: o.root, Sourcefile: stdin, Loader: api.LoaderJS},
		AbsWorkingDir: o.root,
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
				File: o.path(l.File), Line: l.Line, Col: l.Column + 1, Code: "mount-error", Message: e.Text,
				Related: []report.Report{{Severity: report.Message, Message: "mounted on the page " + page.Pathname}},
			})
		}
	}
	return reports
}
