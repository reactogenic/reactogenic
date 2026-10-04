package behaviors

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// The fixture: a project whose node_modules holds the package @fixture/ui —
// three behaviour modules written as builder.md prescribes (overlays and
// invokers page-level, menu-keys per root with a feature module and a flag
// in a module it imports), and the ones that break a rule. The package is
// committed under testdata/ui and copied into place: a committed
// node_modules is ignored by the repository.
//
// The directory is what t.TempDir gives — on macOS a path through a link
// (/var), as a project directory may be (TestSymlinks).
func project(t *testing.T) Options {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "node_modules", "@fixture", "ui"), os.DirFS("testdata/ui")); err != nil {
		t.Fatal(err)
	}
	return Options{Dir: dir}
}

const (
	ui  = "@fixture/ui/behaviors/"
	src = "node_modules/@fixture/ui/src/"
)

// mount is a record of `mount(module, id, flags)`: "RG_X" turns a flag on,
// "!RG_X" passes it off.
func mount(module, id string, flags ...string) render.Mount {
	m := render.Mount{Module: ui + module, ID: id}
	for _, flag := range flags {
		if m.Flags == nil {
			m.Flags = map[string]bool{}
		}
		name, off := strings.CutPrefix(flag, "!")
		m.Flags[name] = !off
	}
	return m
}

func page(pathname, body string, mounts ...render.Mount) render.Page {
	return render.Page{
		Route:  render.Route{Pathname: pathname, File: "/site/pages" + pathname + "index.rtsx"},
		HTML:   "<html><head><title>t</title></head><body>" + body + "</body></html>",
		Mounts: mounts,
	}
}

// The site: what each page mounted is what its components rendered.
var (
	home   = page("/", `<nav id="s1" popover></nav>`, mount("overlays", ""), mount("overlays", ""), mount("overlays", ""))
	guide  = page("/guide/", `<nav id="s1" popover></nav><div id="m1"></div>`, mount("overlays", ""), mount("menu-keys", "m1", "!RG_MENU_TYPEAHEAD"), mount("overlays", ""))
	syntax = page("/syntax/", `<nav id="s1" popover></nav><div id="m1"></div><div id="m2"></div><dialog id="d1"></dialog>`,
		mount("overlays", ""), mount("menu-keys", "m1"), mount("overlays", ""), mount("menu-keys", "m2", "RG_MENU_TYPEAHEAD", "RG_MENU_WRAP"), mount("invokers", ""), mount("overlays", ""))
	cli = page("/reference/cli/", `<p>nothing opens</p>`)
	// Pages whose pathname a browser encodes: location.pathname is
	// /se%C3%B1or/ and /a%20b%25/.
	senor   = page("/señor/", `<div id="m1"></div>`, mount("menu-keys", "m1"))
	percent = page("/a b%/", `<div id="m1"></div>`, mount("menu-keys", "m1"))
)

// A marker is a string literal of one module, or of one feature of it: it
// is in a script exactly when that code is.
var markers = []string{`"fx:overlays"`, `"fx:invokers"`, `"fx:invokers-popover"`, `"fx:menu-keys"`, `"fx:typeahead:"`, `"fx:wrap"`}

func holds(js string) []string {
	var got []string
	for _, m := range markers {
		if strings.Contains(js, m) {
			got = append(got, m)
		}
	}
	return got
}

// validESM parses a script back as an ES module.
func validESM(t *testing.T, js string) {
	t.Helper()
	if r := api.Transform(js, api.TransformOptions{Loader: api.LoaderJS, Format: api.FormatESModule}); len(r.Errors) > 0 {
		t.Errorf("not a module: %v\n%s", r.Errors, js)
	}
	if strings.Contains(js, "RG_") {
		t.Errorf("a flag is left in the script: a ReferenceError when it runs\n%s", js)
	}
}

func TestBuild(t *testing.T) {
	opts := project(t)
	for _, tt := range []struct {
		page    render.Page
		holds   []string      // the markers in the script
		modules []ModuleBytes // Bytes: 1 for "some", 0 for none
		calls   int           // of document.getElementById
	}{
		// A page-level module mounted three times: one import, one call.
		{home, []string{`"fx:overlays"`}, []ModuleBytes{{ui + "overlays", src + "behaviors/overlays.ts", 1}, {"", Entry, 1}}, 0},
		// A flag passed off is off: the feature's module is in the graph
		// and leaves nothing; so does the branch of a flag nobody passed.
		{guide, []string{`"fx:overlays"`, `"fx:menu-keys"`}, []ModuleBytes{
			{ui + "overlays", src + "behaviors/overlays.ts", 1}, {ui + "menu-keys", src + "behaviors/menu-keys.ts", 1},
			{"", src + "lib/step.ts", 1}, {"", src + "lib/typeahead.ts", 0}, {"", Entry, 1}}, 1},
		// Two mounts of one module: one copy of it, two calls; a flag one
		// of them turned on is on for the page (the union).
		{syntax, []string{`"fx:overlays"`, `"fx:invokers"`, `"fx:menu-keys"`, `"fx:typeahead:"`, `"fx:wrap"`}, []ModuleBytes{
			{ui + "overlays", src + "behaviors/overlays.ts", 1}, {ui + "menu-keys", src + "behaviors/menu-keys.ts", 1}, {ui + "invokers", src + "behaviors/invokers.ts", 1},
			{"", src + "lib/step.ts", 1}, {"", src + "lib/typeahead.ts", 1}, {"", Entry, 1}}, 2},
	} {
		t.Run(tt.page.Pathname, func(t *testing.T) {
			js, modules, reports := Build(tt.page, opts)
			if len(reports) > 0 {
				t.Fatalf("reports: %+v", reports)
			}
			validESM(t, js)
			if got := holds(js); !slices.Equal(got, tt.holds) {
				t.Errorf("the script holds %v, want %v\n%s", got, tt.holds, js)
			}
			if got := strings.Count(js, "document.getElementById("); got != tt.calls {
				t.Errorf("%d calls with an element, want %d\n%s", got, tt.calls, js)
			}
			for _, m := range tt.holds {
				if strings.Count(js, m) != 1 {
					t.Errorf("%s is in the script %d times: a module is there once\n%s", m, strings.Count(js, m), js)
				}
			}
			// The sizes: every input of the metafile, and they add up to
			// the script.
			sum := 0
			var got []ModuleBytes
			for _, m := range modules {
				sum += m.Bytes
				got = append(got, ModuleBytes{m.Module, m.Path, min(m.Bytes, 1)})
			}
			if !slices.Equal(got, tt.modules) {
				t.Errorf("modules:\n  %+v\nwant:\n  %+v", modules, tt.modules)
			}
			if sum != len(js) {
				t.Errorf("the modules' bytes add up to %d, the script is %d", sum, len(js))
			}
			t.Logf("%s: %d B %+v\n%s", tt.page.Pathname, len(js), modules, js)
		})
	}

	// A page that mounted nothing has no script.
	if js, modules, reports := Build(cli, opts); js != "" || modules != nil || reports != nil {
		t.Errorf("no mounts: %q %v %v", js, modules, reports)
	}
}

// The generated entry: one import per distinct module; a per-root module is
// called once per element, a page-level one once per page.
func TestEntry(t *testing.T) {
	for _, tt := range []struct {
		page render.Page
		want string
	}{
		{home, "import m0 from \"@fixture/ui/behaviors/overlays\";\nm0();\n"},
		{syntax, `import m0 from "@fixture/ui/behaviors/overlays";
import m1 from "@fixture/ui/behaviors/menu-keys";
import m2 from "@fixture/ui/behaviors/invokers";
m0();
m1(document.getElementById("m1"));
m1(document.getElementById("m2"));
m2();
`},
		// One module on one element twice is one call; an id is a string of
		// the page, whatever is in it.
		{page("/x/", "", mount("menu-keys", "m1"), mount("menu-keys", "m1", "RG_MENU_WRAP"), mount("menu-keys", `a"</script>`)),
			"import m0 from \"@fixture/ui/behaviors/menu-keys\";\nm0(document.getElementById(\"m1\"));\nm0(document.getElementById(\"a\\\"\\u003c/script\\u003e\"));\n"},
	} {
		if got := planOf(tt.page).entry(); got != tt.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tt.page.Pathname, got, tt.want)
		}
	}
}

// A flag off removes its code, on keeps it; a flag no mount passed is
// defined all the same — off.
func TestFlags(t *testing.T) {
	opts := project(t)
	build := func(flags ...string) string {
		t.Helper()
		js, _, reports := Build(page("/x/", `<div id="m1"></div>`, mount("menu-keys", "m1", flags...)), opts)
		if len(reports) > 0 {
			t.Fatalf("%v: %+v", flags, reports)
		}
		validESM(t, js)
		return js
	}
	none, off, on, wrap, both := build(), build("!RG_MENU_TYPEAHEAD", "!RG_MENU_WRAP"), build("RG_MENU_TYPEAHEAD"), build("RG_MENU_WRAP"), build("RG_MENU_TYPEAHEAD", "RG_MENU_WRAP")
	if none != off {
		t.Errorf("a flag passed off and a flag not passed differ:\n%s\n%s", none, off)
	}
	for _, tt := range []struct {
		name, js string
		holds    []string
	}{
		{"off", off, []string{`"fx:menu-keys"`}},
		{"typeahead", on, []string{`"fx:menu-keys"`, `"fx:typeahead:"`}},
		{"wrap", wrap, []string{`"fx:menu-keys"`, `"fx:wrap"`}},
		{"both", both, []string{`"fx:menu-keys"`, `"fx:typeahead:"`, `"fx:wrap"`}},
	} {
		if got := holds(tt.js); !slices.Equal(got, tt.holds) {
			t.Errorf("%s holds %v, want %v\n%s", tt.name, got, tt.holds, tt.js)
		}
	}
	if !(len(off) < len(on) && len(off) < len(wrap) && len(on) < len(both) && len(wrap) < len(both)) {
		t.Errorf("sizes: off %d, typeahead %d, wrap %d, both %d", len(off), len(on), len(wrap), len(both))
	}

	// The union: on when any mount of the page turned it on, in any order.
	a, _, _ := Build(page("/x/", `<i id="m1"></i><i id="m2"></i>`, mount("menu-keys", "m1", "RG_MENU_TYPEAHEAD"), mount("menu-keys", "m2", "!RG_MENU_TYPEAHEAD")), opts)
	b, _, _ := Build(page("/x/", `<i id="m1"></i><i id="m2"></i>`, mount("menu-keys", "m1", "!RG_MENU_TYPEAHEAD"), mount("menu-keys", "m2", "RG_MENU_TYPEAHEAD")), opts)
	if a != b || !strings.Contains(a, `"fx:typeahead:"`) {
		t.Errorf("the union:\n%s\n%s", a, b)
	}
}

func TestReports(t *testing.T) {
	opts := project(t)
	bad := page("/bad/", `<div id="m1"></div><template><div id="m3"></div></template>`,
		mount("nope", ""),
		mount("menu-keys", "m9", "RG_MENU_TYPEHEAD", "RG_MENU_WRAP"), // a typo; a flag of a module it imports
		mount("menu-keys", "m9", "RG_MENU_TYPEHEAD"),                 // said once
		mount("menu-keys", "m3"),                                     // in a template: not an element of the page
		mount("overlays", "", "!RG_MENU_TYPEAHEAD"),                  // another module's flag, even off
		mount("side-effect", "m1"),
		mount("broken-import", "m1"),
		mount("nope", ""),
	)
	want := `pages/bad/index.rtsx: error mount-not-found: Page /bad/: ` + "`mount(\"@fixture/ui/behaviors/nope\")`: the module does not resolve from the project directory" + `
pages/bad/index.rtsx: error mount-no-element: Page /bad/: ` + "`mount(\"@fixture/ui/behaviors/menu-keys\")`: no element of the page has `id=\"m9\"`" + `
pages/bad/index.rtsx: error mount-flag: Page /bad/: ` + "`mount(\"@fixture/ui/behaviors/menu-keys\")`: the module has no flag `RG_MENU_TYPEHEAD`" + `
pages/bad/index.rtsx: error mount-no-element: Page /bad/: ` + "`mount(\"@fixture/ui/behaviors/menu-keys\")`: no element of the page has `id=\"m3\"`" + `
pages/bad/index.rtsx: error mount-flag: Page /bad/: ` + "`mount(\"@fixture/ui/behaviors/overlays\")`: the module has no flag `RG_MENU_TYPEAHEAD`" + `
` + opts.Dir + `/node_modules/@fixture/ui/src/behaviors/side-effect.ts: error mount-side-effect: A behaviour module cannot run code when it is imported, only when it is mounted: ` + "`side-effect.ts`" + `
  what runs: ` + "`var reduce = matchMedia(\"(prefers-reduced-motion: reduce)\");`" + `
  mounted on the page /bad/
` + opts.Dir + `/node_modules/@fixture/ui/src/lib/registers.ts: error mount-side-effect: A behaviour module cannot run code when it is imported, only when it is mounted: ` + "`registers.ts`" + `
  what runs: ` + "`customElements.define(\"fx-registered\", class extends HTMLElement {`" + `
  mounted on the page /bad/
` + opts.Dir + `/node_modules/@fixture/ui/src/behaviors/broken-import.ts(2,25): error mount-error: Could not resolve "../lib/missing"
  mounted on the page /bad/
`
	printed := func(reports []report.Report) string {
		var out bytes.Buffer
		check.Print(&out, reports, "/site", false, nil)
		return out.String()
	}
	js, modules, reports := Build(bad, opts)
	if got := printed(reports); got != want || js != "" || modules != nil {
		t.Errorf("Build: %q %v\n%s\nwant:\n%s", js, modules, got, want)
	}
	// The control reports the same, page by page.
	js, reports = BuildControl([]render.Page{home, bad, cli}, opts)
	if got := printed(reports); got != want || js != "" {
		t.Errorf("BuildControl: %q\n%s\nwant:\n%s", js, got, want)
	}

	// A module that resolves and is not a behaviour: the build's own error,
	// as the mount's.
	want = "pages/x/index.rtsx: error mount-error: Page /x/: `mount(\"@fixture/ui/behaviors/no-default\")`: No matching export in \"node_modules/@fixture/ui/src/behaviors/no-default.ts\" for import \"default\"\n"
	x := page("/x/", `<div id="m1"></div>`, mount("overlays", ""), mount("no-default", "m1"))
	_, _, reports = Build(x, opts)
	if got := printed(reports); got != want {
		t.Errorf("Build:\n%s\nwant:\n%s", got, want)
	}
	// In the control it is the error of the first page that mounts the module.
	if _, reports = BuildControl([]render.Page{cli, home, x, x}, opts); printed(reports) != want {
		t.Errorf("BuildControl:\n%s\nwant:\n%s", printed(reports), want)
	}
}

// What is read of a module: where it resolves, the files it reaches, the
// flags of all of them — and, shared through a Cache, read once.
func TestModules(t *testing.T) {
	opts := project(t)
	for _, tt := range []struct {
		module string
		want   module
	}{
		{"menu-keys", module{file: src + "behaviors/menu-keys.ts", files: []string{src + "behaviors/menu-keys.ts", src + "lib/step.ts", src + "lib/typeahead.ts"},
			flags: map[string]bool{"RG_MENU_TYPEAHEAD": true, "RG_MENU_WRAP": true}}},
		{"overlays", module{file: src + "behaviors/overlays.ts", files: []string{src + "behaviors/overlays.ts"}, flags: map[string]bool{}}},
		{"invokers", module{file: src + "behaviors/invokers.ts", files: []string{src + "behaviors/invokers.ts"}, flags: map[string]bool{"RG_INVOKERS_POPOVER": true}}},
		// `sideEffects: false` of the package's package.json is not taken
		// at its word.
		{"side-effect", module{file: src + "behaviors/side-effect.ts", files: []string{src + "behaviors/side-effect.ts", src + "lib/registers.ts"}, flags: map[string]bool{},
			effects: []effect{
				{file: src + "behaviors/side-effect.ts", runs: `var reduce = matchMedia("(prefers-reduced-motion: reduce)");`},
				{file: src + "lib/registers.ts", runs: `customElements.define("fx-registered", class extends HTMLElement {`},
			}}},
		{"nope", module{notFound: true}},
	} {
		if got := *opts.resolved().read(ui + tt.module); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n  %+v\nwant:\n  %+v", tt.module, got, tt.want)
		}
	}

	// A cache gives what no cache gives, to concurrent builds too.
	want := map[string]string{}
	pages := []render.Page{home, guide, syntax, cli}
	for _, p := range pages {
		want[p.Pathname], _, _ = Build(p, opts)
	}
	opts.Cache = &Cache{}
	var wg sync.WaitGroup
	for range 4 {
		for _, p := range pages {
			wg.Go(func() {
				if js, _, reports := Build(p, opts); js != want[p.Pathname] || len(reports) > 0 {
					t.Errorf("%s with a cache: %q %v", p.Pathname, js, reports)
				}
			})
		}
	}
	wg.Wait()
	if len(opts.Cache.modules) != 3 {
		t.Errorf("the cache holds %d modules, want 3", len(opts.Cache.modules))
	}
}

// "No top-level side effects" is what esbuild leaves of a module nobody
// uses: state and constants go, a call stays — also one annotated pure. A
// module with flags is asked with every flag on, and off.
func TestSideEffects(t *testing.T) {
	opts := project(t).resolved()
	for _, tt := range []struct {
		name, source, kept string
	}{
		{"state", "const seen = new WeakMap<Element, number>();\nlet n = 0;\nconst ITEM = '[role=\"menuitem\"]';\nexport default function m(root: HTMLElement) { seen.set(root, n++); root.querySelector(ITEM); }\n", ""},
		{"set", "const KEYS = new Set([\"ArrowUp\", \"ArrowDown\"]);\nexport default function m(root: HTMLElement) { root.dataset.k = String(KEYS.size); }\n", ""},
		{"folded", "const A = \"a\";\nconst SEL = `${A},b`;\nconst OBJ = { a: 1, b: [1, 2] };\nexport default function m(root: HTMLElement) { root.querySelector(SEL); root.dataset.x = String(OBJ.a); }\n", ""},
		{"enum", "enum Key { Up, Down }\nexport default function m(root: HTMLElement) { root.dataset.k = String(Key.Up); }\n", ""},
		{"class", "class Menu { static all = new Set<Menu>(); constructor(public root: HTMLElement) {} }\nexport default function m(root: HTMLElement) { new Menu(root); }\n", ""},
		{"join", "const SEL = [\"a\", \"b\"].join(\",\");\nexport default function m(root: HTMLElement) { root.querySelector(SEL); }\n",
			`var SEL = ["a", "b"].join(",");`},
		{"in", "const supported = \"command\" in HTMLButtonElement.prototype;\nexport default function m() { if (!supported) addEventListener(\"click\", () => {}); }\n",
			`var supported = "command" in HTMLButtonElement.prototype;`},
		{"pure", "const x = /* @__PURE__ */ matchMedia(\"(min-width: 1px)\");\nexport default function m() { x.matches; }\n",
			`var x = matchMedia("(min-width: 1px)");`},
		// A flag is defined when a page is built: a constant made of one is
		// a constant…
		{"flag constants", "declare const RG_SLOW: boolean;\nconst DELAY = RG_SLOW ? 500 : 100;\nconst slow = RG_SLOW;\nconst options = { slow: RG_SLOW };\nexport default function m(root: HTMLElement) { root.dataset.d = String(DELAY) + slow + options.slow; }\n", ""},
		// … and a statement under one runs, with the flag on or with it off.
		{"statement", "declare const RG_X: boolean;\nif (RG_X) document.title = \"x\";\nexport default function m() {}\n",
			`document.title = "x"; with every flag on`},
		{"statement off", "declare const RG_X: boolean;\nconst title = RG_X ? \"\" : document.title;\nexport default function m() { return title; }\n",
			`var title = document.title; with every flag off`},
		{"call with a flag", "declare const RG_X: boolean;\nconst q = matchMedia(RG_X ? \"print\" : \"screen\");\nexport default function m() { return q; }\n",
			`var q = matchMedia("print"); with every flag on`},
	} {
		file := src + "behaviors/" + strings.ReplaceAll(tt.name, " ", "-") + ".ts"
		if err := os.WriteFile(filepath.Join(opts.Dir, file), []byte(tt.source), 0o644); err != nil {
			t.Fatal(err)
		}
		m := opts.read(ui + strings.ReplaceAll(tt.name, " ", "-"))
		var got string
		for _, e := range m.effects {
			got += e.runs + e.when
		}
		if got != tt.kept || len(m.errors) > 0 || m.unread != "" {
			t.Errorf("%s: kept %q, want %q (effects %+v, errors %v, unread %q)", tt.name, got, tt.kept, m.effects, m.errors, m.unread)
		}
	}
}

// In the repository a package is a pnpm link into the workspace: a module
// resolves to its real file, named from the project directory, and is read
// there. Skipped where `pnpm install` has not run.
func TestWorkspace(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "@reactogenic", "core")); err != nil {
		t.Skip("no node_modules at the repository's root")
	}
	m := Options{Dir: root}.resolved().read("@reactogenic/core")
	if m.notFound || len(m.errors) > 0 || !strings.HasPrefix(m.file, "packages/core/") || !slices.Contains(m.files, m.file) {
		t.Errorf("@reactogenic/core: %+v", m)
	}
	for _, file := range m.files {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

func TestElementIDs(t *testing.T) {
	got := elementIDs(`<html><body><DIV ID="a" class="x"><p id=b><br id="c"/><i id=""></i></div>
		<template id="tp"><p id="t"></p><template id="in"><p id="u"></p></template></template><svg><g id="s"/></svg><script>var x = '<p id="no">';</script></body></html>`)
	// A template is an element of the page — getElementById finds it; what
	// is in it is not.
	want := map[string]bool{"a": true, "b": true, "c": true, "s": true, "tp": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
}

// The control: one script for the site — every module mounted on any page,
// every flag on, and the table from pathname to the page's mounts.
func TestControl(t *testing.T) {
	opts := project(t)
	pages := []render.Page{home, guide, syntax, cli}
	js, reports := BuildControl(pages, opts)
	if len(reports) > 0 {
		t.Fatalf("reports: %+v", reports)
	}
	validESM(t, js)
	// Every flag: also the one no page of the site turned on.
	if got := holds(js); !slices.Equal(got, markers) {
		t.Errorf("the control holds %v, want all of %v\n%s", got, markers, js)
	}
	for _, key := range []string{`"/":`, `"/guide/":`, `"/syntax/":`} {
		if !strings.Contains(js, key) {
			t.Errorf("no %s in the table\n%s", key, js)
		}
	}
	if strings.Contains(js, "/reference/cli/") {
		t.Errorf("a page that mounts nothing is in the table\n%s", js)
	}
	// What component awareness is worth, on the fixture: every page's own
	// script is smaller than the control.
	for _, p := range pages {
		own, _, _ := Build(p, opts)
		if len(own) >= len(js) {
			t.Errorf("%s: its script is %d B, the control %d B", p.Pathname, len(own), len(js))
		}
		t.Logf("%s: %d B; the control: %d B", p.Pathname, len(own), len(js))
	}
	t.Logf("\n%s", js)

	// --base is in the table's keys: they are what location.pathname is.
	for _, base := range []string{"/docs/", "/docs", "docs"} {
		opts.Base = base
		if js, _ := BuildControl(pages, opts); !strings.Contains(js, `"/docs/":`) || !strings.Contains(js, `"/docs/guide/":`) || strings.Contains(js, `"/guide/":`) {
			t.Errorf("base %q:\n%s", base, js)
		}
	}
	if js, reports := BuildControl([]render.Page{cli}, opts); js != "" || reports != nil {
		t.Errorf("a site that mounts nothing: %q %v", js, reports)
	}
}

// write adds a behaviour module to the fixture's package.
func write(t *testing.T, opts Options, name, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(opts.Dir, src+"behaviors", name+".ts"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

// codes builds a page and returns the codes of its reports, in order.
func codes(t *testing.T, opts Options, p render.Page) (js string, modules []ModuleBytes, got []string) {
	t.Helper()
	js, modules, reports := Build(p, opts)
	for _, r := range reports {
		got = append(got, r.Code)
	}
	return js, modules, got
}

// The project directory may be reached through a symbolic link: esbuild
// names every file from the real one, and so is what is read of its metafile.
func TestSymlinks(t *testing.T) {
	// A link to the project, as /var is on macOS.
	real := project(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real.Dir, link); err != nil {
		t.Skip(err)
	}
	// A workspace: the package is linked into node_modules, and the project
	// is reached through a link elsewhere — `..` from that one is not the
	// workspace.
	tmp := t.TempDir()
	site, deep := filepath.Join(tmp, "real", "site"), filepath.Join(tmp, "home", "deep")
	if err := os.CopyFS(filepath.Join(tmp, "real", "packages", "ui"), os.DirFS("testdata/ui")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(site, "node_modules", "@fixture"), deep} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "packages", "ui"), filepath.Join(site, "node_modules", "@fixture", "ui")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(site, filepath.Join(deep, "site")); err != nil {
		t.Fatal(err)
	}

	pages := []render.Page{home, guide, syntax}
	want, wantModules, _ := Build(guide, real)
	on := page("/x/", `<div id="m1"></div>`, mount("menu-keys", "m1", "RG_MENU_TYPEAHEAD", "RG_MENU_WRAP"))
	wantOn, _, _ := Build(on, real)
	wantControl, _ := BuildControl(pages, real)
	for _, tt := range []struct {
		name string
		opts Options
		src  string // the package's sources, as a metafile names them
	}{
		{"a link to the project", Options{Dir: link}, src},
		{"a workspace", Options{Dir: site}, "../packages/ui/src/"},
		{"a link to the project of a workspace", Options{Dir: filepath.Join(deep, "site")}, "../packages/ui/src/"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			js, modules, reports := Build(guide, tt.opts)
			if js != want || len(reports) > 0 {
				t.Errorf("the script:\n%s\nwant:\n%s\n%+v", js, want, reports)
			}
			named := slices.Clone(wantModules)
			for i, m := range named {
				named[i].Path = strings.Replace(m.Path, src, tt.src, 1)
			}
			if !slices.Equal(modules, named) {
				t.Errorf("modules:\n  %+v\nwant:\n  %+v", modules, named)
			}
			// The flags are read off the files: every one is defined.
			if js, _, reports := Build(on, tt.opts); js != wantOn || len(reports) > 0 {
				t.Errorf("flags on:\n%s\nwant:\n%s\n%+v", js, wantOn, reports)
			}
			if js, reports := BuildControl(pages, tt.opts); js != wantControl || len(reports) > 0 {
				t.Errorf("the control:\n%s\nwant:\n%s\n%+v", js, wantControl, reports)
			}
			// What is wrong is said of the mount, at the page…
			_, _, reports = Build(page("/x/", `<div id="m1"></div>`, mount("nope", ""), mount("no-default", "m1")), tt.opts)
			if len(reports) != 1 || reports[0].Code != "mount-not-found" || reports[0].File != "/site/pages/x/index.rtsx" {
				t.Errorf("a module that does not resolve: %+v", reports)
			}
			_, _, reports = Build(page("/x/", `<div id="m1"></div>`, mount("no-default", "m1")), tt.opts)
			if len(reports) != 1 || reports[0].Code != "mount-error" || reports[0].File != "/site/pages/x/index.rtsx" || !strings.Contains(reports[0].Message, "`mount(\"@fixture/ui/behaviors/no-default\")`: No matching export") {
				t.Errorf("a module without a default export: %+v", reports)
			}
			// … or at a file, which is there.
			_, _, reports = Build(page("/x/", `<div id="m1"></div>`, mount("side-effect", "m1"), mount("broken-import", "m1")), tt.opts)
			if len(reports) != 3 {
				t.Fatalf("reports: %+v", reports)
			}
			for i, file := range []string{"behaviors/side-effect.ts", "lib/registers.ts", "behaviors/broken-import.ts"} {
				if _, err := os.Stat(reports[i].File); err != nil || !strings.HasSuffix(reports[i].File, "/src/"+file) {
					t.Errorf("a report at %s, want a file …/src/%s: %v", reports[i].File, file, err)
				}
			}
		})
	}
	// A file under the directory is named from it as the caller names it.
	_, _, reports := Build(page("/x/", `<div id="m1"></div>`, mount("side-effect", "m1")), Options{Dir: link})
	if len(reports) != 2 || reports[0].File != filepath.ToSlash(link)+"/"+src+"behaviors/side-effect.ts" {
		t.Errorf("reports: %+v", reports)
	}
}

// A constant made of a flag is a constant: with the flag defined it folds
// away, as it does in the page's script.
func TestFlagConstants(t *testing.T) {
	opts := project(t)
	write(t, opts, "flagconst", `declare const RG_SLOW: boolean;
const DELAY = RG_SLOW ? 500 : 100;
const slow = RG_SLOW;
const options = { slow: RG_SLOW, delay: DELAY };
export default function m(root: HTMLElement) { root.dataset.d = "fc:" + DELAY + slow + options.slow; }
`)
	for flag, want := range map[string]string{"RG_SLOW": `{slow:!0,delay:500};function o(t){t.dataset.d="fc:500true"+`, "!RG_SLOW": `{slow:!1,delay:100};function o(t){t.dataset.d="fc:100false"+`} {
		js, _, got := codes(t, opts, page("/x/", `<div id="m1"></div>`, mount("flagconst", "m1", flag)))
		if len(got) > 0 || !strings.Contains(js, want) {
			t.Errorf("%s: %v\n%s\nwant %s in it", flag, got, js, want)
		}
		validESM(t, js)
	}
	// What runs under a flag runs, whichever way the flag is; it is no
	// script on a page that leaves the flag off either.
	write(t, opts, "flagon", "declare const RG_X: boolean;\nif (RG_X) document.title = \"x\";\nexport default function m() {}\n")
	write(t, opts, "flagoff", "declare const RG_X: boolean;\nconst t = RG_X ? \"\" : document.title;\nexport default function m() { return t; }\n")
	for _, name := range []string{"flagon", "flagoff"} {
		for _, flags := range [][]string{nil, {"RG_X"}} {
			if js, _, got := codes(t, opts, page("/x/", "", mount(name, "", flags...))); js != "" || !slices.Equal(got, []string{"mount-side-effect"}) {
				t.Errorf("%s %v: %q %v", name, flags, js, got)
			}
		}
	}
}

// A flag is an identifier that starts with `RG_` and that nothing binds:
// whatever follows, every one a script reads is defined.
func TestFlagNames(t *testing.T) {
	opts := project(t)
	write(t, opts, "names", `declare const RG_lower: boolean;
declare const RG_MIX_ed: boolean;
declare const RG_$: boolean;
declare const RG_é: boolean;
declare const RG_UNREAD: boolean;
// RG_COMMENT is a flag of nobody.
const RG_BOUND = false;
export default function m(root: HTMLElement) {
  if (RG_lower) root.dataset.l = "fn:lower";
  if (RG_MIX_ed) root.dataset.m = "fn:mixed";
  if (RG_$) root.dataset.d = "fn:dollar";
  if (RG_é) root.dataset.u = "fn:unicode";
  if (RG_BOUND) root.dataset.b = "fn:bound";
  root.dataset.s = "RG_STRING" + (root as any).RG_PROPERTY;
}
`)
	body := `<div id="m1"></div>`
	for _, tt := range []struct {
		flags []string
		holds []string
	}{
		{nil, nil},
		{[]string{"RG_lower"}, []string{`"fn:lower"`}},
		{[]string{"RG_MIX_ed", "RG_$", "!RG_lower"}, []string{`"fn:mixed"`, `"fn:dollar"`}},
		{[]string{"RG_é"}, []string{`"fn:unicode"`}},
	} {
		js, _, got := codes(t, opts, page("/x/", body, mount("names", "m1", tt.flags...)))
		if len(got) > 0 {
			t.Errorf("%v: %v", tt.flags, got)
		}
		if left := undefinedFlags(js); len(left) > 0 || js == "" {
			t.Errorf("%v: left undefined: %v\n%s", tt.flags, left, js)
		}
		var holds []string
		for _, m := range []string{`"fn:lower"`, `"fn:mixed"`, `"fn:dollar"`, `"fn:unicode"`, `"fn:bound"`} {
			if strings.Contains(js, m) {
				holds = append(holds, m)
			}
		}
		if !slices.Equal(holds, tt.holds) {
			t.Errorf("%v: the script holds %v, want %v\n%s", tt.flags, holds, tt.holds, js)
		}
	}
	// What is not a flag of the module cannot be passed: a constant of its
	// own, a name in a comment, in a string, of a property, a flag it
	// declares and never reads.
	for _, flag := range []string{"RG_BOUND", "RG_COMMENT", "RG_STRING", "RG_PROPERTY", "RG_UNREAD", "!RG_BOUND"} {
		if js, _, got := codes(t, opts, page("/x/", body, mount("names", "m1", flag))); js != "" || !slices.Equal(got, []string{"mount-flag"}) {
			t.Errorf("%s: %q %v", flag, js, got)
		}
	}
	js, reports := BuildControl([]render.Page{page("/x/", body, mount("names", "m1"))}, opts)
	if len(reports) > 0 || len(undefinedFlags(js)) > 0 || !strings.Contains(js, `"fn:lower"`) || strings.Contains(js, `"fn:bound"`) {
		t.Errorf("the control: %+v\n%s", reports, js)
	}

	// A flag the builder cannot find in the text — here its name is escaped —
	// is no script: it would be a ReferenceError when the page loads.
	write(t, opts, "escaped", "export default function m(root: HTMLElement) { if (RG_\\u0041) root.dataset.a = \"fn:escaped\"; }\n")
	x := page("/x/", body, mount("escaped", "m1"))
	js, _, reports = Build(x, opts)
	if js != "" || len(reports) != 1 || reports[0].Code != "mount-flag" || !strings.Contains(reports[0].Message, "`RG_A`") {
		t.Errorf("an escaped flag: %q %+v", js, reports)
	}
	if js, reports := BuildControl([]render.Page{x}, opts); js != "" || len(reports) != 1 || reports[0].Code != "mount-flag" {
		t.Errorf("an escaped flag, the control: %q %+v", js, reports)
	}
}

// A module is a file, however a mount spells it: one import, and a
// page-level behaviour runs once.
func TestSpellings(t *testing.T) {
	opts := project(t)
	spelled := func(spec, id string) render.Mount { return render.Mount{Module: spec, ID: id} }
	plain := page("/x/", `<div id="m1"></div>`, mount("overlays", ""), mount("menu-keys", "m1"))
	p := page("/x/", `<div id="m1"></div>`,
		mount("overlays", ""), spelled("./"+src+"behaviors/overlays.ts", ""), spelled("./"+src+"behaviors/overlays", ""),
		mount("menu-keys", "m1"), spelled("./"+src+"behaviors/menu-keys", "m1"))
	want, wantModules, _ := Build(plain, opts)
	js, modules, reports := Build(p, opts)
	if js != want || len(reports) > 0 || !slices.Equal(modules, wantModules) {
		t.Errorf("the script:\n%s\nwant:\n%s\n%+v\n%+v", js, want, modules, reports)
	}
	// The control: one module of the site, whichever page spells it how.
	wantControl, _ := BuildControl([]render.Page{home, plain}, opts)
	if js, reports := BuildControl([]render.Page{home, p}, opts); js != wantControl || len(reports) > 0 {
		t.Errorf("the control:\n%s\nwant:\n%s\n%+v", js, wantControl, reports)
	}
	// A module that breaks a rule is reported once: here at each of its two
	// files that do.
	_, _, got := codes(t, opts, page("/x/", `<div id="m1"></div>`, mount("side-effect", "m1"), spelled("./"+src+"behaviors/side-effect", "m1")))
	if !slices.Equal(got, []string{"mount-side-effect", "mount-side-effect"}) {
		t.Errorf("a module with side effects, spelled twice: %v", got)
	}
}

// What esbuild adds of its own — the helpers of a dynamic import — is a row
// of the sizes too: they add up to the script.
func TestRuntime(t *testing.T) {
	opts := project(t)
	write(t, opts, "dynamic", `declare const RG_DYN: boolean;
export default function m(root: HTMLElement) {
  if (RG_DYN) import("../lib/typeahead").then((t) => root.addEventListener("keydown", t.onType));
}
`)
	for _, tt := range []struct {
		flag    string
		runtime bool
	}{{"RG_DYN", true}, {"!RG_DYN", false}} {
		js, modules, reports := Build(page("/x/", `<div id="m1"></div>`, mount("dynamic", "m1", tt.flag)), opts)
		if len(reports) > 0 {
			t.Fatalf("reports: %+v", reports)
		}
		validESM(t, js)
		sum := 0
		for _, m := range modules {
			sum += m.Bytes
		}
		last := modules[len(modules)-1]
		if sum != len(js) || (last.Path == Runtime) != tt.runtime || tt.runtime && last.Bytes == 0 {
			t.Errorf("%s: the modules' bytes add up to %d, the script is %d: %+v\n%s", tt.flag, sum, len(js), modules, js)
		}
	}
}

// ran is what testdata/run.mjs prints.
type ran struct {
	Added    []string                     `json:"added"`    // the listeners, in the order they were added
	Datasets map[string]map[string]string `json:"datasets"` // by element id, after every listener was called
	Title    string                       `json:"title"`
	Popover  map[string]string            `json:"popover"`
}

// node runs a script with Node against run.mjs's fake document.
func node(t *testing.T, js, pathname, ids string) ran {
	t.Helper()
	script := filepath.Join(t.TempDir(), "script.mjs")
	if err := os.WriteFile(script, []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "testdata/run.mjs", script, pathname, ids).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var r ran
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return r
}

// The scripts run: in Node, against a fake `document` that records the
// listeners a script adds and then calls each.
func TestRun(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	opts := project(t)
	opts.Base = "/docs"
	pages := []render.Page{home, guide, syntax, cli, senor, percent}
	control, reports := BuildControl(pages, opts)
	if len(reports) > 0 {
		t.Fatalf("reports: %+v", reports)
	}
	menu := func(at, typed string) map[string]string {
		d := map[string]string{"count": "3", "at": at, "menuKeys": "fx:menu-keys"}
		if typed != "" {
			d["typed"] = typed
		}
		return d
	}
	for _, tt := range []struct {
		page render.Page
		ids  string
		want ran // of the page's own script
		// of the control on that page: every flag is on
		control ran
	}{
		// Mounted three times, a page-level behaviour runs once.
		{home, "s1",
			ran{Added: []string{"window:pagehide"}, Popover: map[string]string{"data-closed": "fx:overlays"}},
			ran{Added: []string{"window:pagehide"}, Popover: map[string]string{"data-closed": "fx:overlays"}}},
		// The arrow stops at the last item, a letter does nothing: no wrap, no typeahead.
		{guide, "s1,m1",
			ran{Added: []string{"window:pagehide", "m1:keydown"}, Popover: map[string]string{"data-closed": "fx:overlays"},
				Datasets: map[string]map[string]string{"m1": menu("2", "")}},
			ran{Added: []string{"window:pagehide", "m1:keydown", "m1:keydown"}, Popover: map[string]string{"data-closed": "fx:overlays"}, Title: "fx:wrap",
				Datasets: map[string]map[string]string{"m1": menu("0", "fx:typeahead:a")}}},
		// The union: both menus wrap and take letters, m2 asked for it.
		{syntax, "s1,m1,m2,d1",
			ran{Added: []string{"window:pagehide", "m1:keydown", "m1:keydown", "m2:keydown", "m2:keydown", "window:click"}, Popover: map[string]string{"data-closed": "fx:overlays"}, Title: "fx:wrap",
				Datasets: map[string]map[string]string{"m1": menu("0", "fx:typeahead:a"), "m2": menu("0", "fx:typeahead:aa"), "clicked": {"count": "3", "at": "2", "invoked": "fx:invokers"}}},
			ran{Added: []string{"window:pagehide", "m1:keydown", "m1:keydown", "m2:keydown", "m2:keydown", "window:click", "window:click"}, Popover: map[string]string{"data-closed": "fx:overlays"}, Title: "fx:wrap",
				Datasets: map[string]map[string]string{"m1": menu("0", "fx:typeahead:a"), "m2": menu("0", "fx:typeahead:aa"), "clicked": {"count": "3", "at": "2", "invoked": "fx:invokers", "popover": "fx:invokers-popover"}}}},
	} {
		t.Run(tt.page.Pathname, func(t *testing.T) {
			normal := func(r ran) ran {
				if r.Datasets == nil {
					r.Datasets = map[string]map[string]string{}
				}
				if _, ok := r.Datasets["clicked"]; !ok {
					r.Datasets["clicked"] = map[string]string{"count": "3", "at": "2"} // the target of every event
				}
				return r
			}
			js, _, reports := Build(tt.page, opts)
			if len(reports) > 0 {
				t.Fatalf("reports: %+v", reports)
			}
			if got, want := node(t, js, tt.page.Pathname, tt.ids), normal(tt.want); !reflect.DeepEqual(got, want) {
				t.Errorf("the page's script:\n  %+v\nwant:\n  %+v", got, want)
			}
			// The control finds the page under the base, however it is addressed.
			at := "/docs" + tt.page.Pathname
			for _, pathname := range []string{at, at + "index.html", strings.TrimSuffix(at, "/")} {
				if got, want := node(t, control, pathname, tt.ids), normal(tt.control); !reflect.DeepEqual(got, want) {
					t.Errorf("the control at %s:\n  %+v\nwant:\n  %+v", pathname, got, want)
				}
			}
		})
	}
	// A pathname is what the browser makes of the page's: encoded.
	for _, pathname := range []string{"/docs/se%C3%B1or/", "/docs/se%c3%b1or", "/docs/señor/index.html", "/docs/a%20b%25/", "/docs/a%20b%25/index.html"} {
		if got := node(t, control, pathname, "m1"); !slices.Equal(got.Added, []string{"m1:keydown", "m1:keydown"}) {
			t.Errorf("the control at %s: %+v", pathname, got)
		}
	}
	// On a page that mounts nothing, and on one that is not the site's, the
	// control does nothing.
	for _, pathname := range []string{"/docs/reference/cli/", "/guide/", "/docs/guide/x/", "/docs/senor/"} {
		if got := node(t, control, pathname, "m1"); len(got.Added) != 0 {
			t.Errorf("the control at %s: %+v", pathname, got)
		}
	}
}

// The scripts run in a browser: inlined as the module script of the page's
// own HTML, followed by a script that presses the keys; what they did is
// read off the DOM. Run with RG_TEST_CHROME=<the Chrome binary>.
func TestChrome(t *testing.T) {
	chrome := os.Getenv("RG_TEST_CHROME")
	if chrome == "" {
		t.Skip("RG_TEST_CHROME is not set")
	}
	opts := project(t)
	site, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The pathname of a file: URL is the file's path: the site is served
	// under its directory.
	opts.Base = filepath.ToSlash(site)
	pages := []render.Page{home, guide, syntax, cli, senor, percent}
	control, reports := BuildControl(pages, opts)
	if len(reports) > 0 {
		t.Fatalf("reports: %+v", reports)
	}
	const drive = `<script type="module">
for (const el of document.querySelectorAll("#m1, #m2")) {
  el.dataset.count = "3"; el.dataset.at = "2";
  for (const key of ["ArrowDown", "a"]) el.dispatchEvent(new KeyboardEvent("keydown", { key }));
}
document.body.click();
dispatchEvent(new PageTransitionEvent("pagehide"));
for (const script of document.querySelectorAll("script")) script.remove();
</script>`
	dom := func(p render.Page, js string) string {
		t.Helper()
		file := filepath.Join(site, filepath.FromSlash(p.Pathname), "index.html")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if js != "" {
			js = `<script type="module">` + js + `</script>`
		}
		if err := os.WriteFile(file, []byte("<!doctype html>"+strings.Replace(p.HTML, "</body>", js+drive+"</body>", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		// Chrome 154 prints the DOM and does not exit: it is read up to the
		// end of the document, then stopped.
		cmd := exec.Command(chrome, "--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check",
			"--user-data-dir="+filepath.Join(site, ".profile"), "--dump-dom", (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String())
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		read := make(chan string)
		go func() {
			var out []byte
			buf := make([]byte, 4096)
			for !bytes.Contains(out, []byte("</html>")) {
				n, err := stdout.Read(buf)
				if out = append(out, buf[:n]...); err != nil {
					break
				}
			}
			read <- string(out)
		}()
		var out string
		select {
		case out = <-read:
		case <-time.After(30 * time.Second):
		}
		cmd.Process.Kill()
		cmd.Wait()
		if !strings.Contains(out, "</html>") {
			t.Fatalf("chrome printed no document: %q", out)
		}
		return out
	}
	const (
		closed  = `<nav id="s1" popover="" data-closed="fx:overlays"></nav>`
		stopped = `<div id="m1" data-menu-keys="fx:menu-keys" data-count="3" data-at="2"></div>`
		wrapped = `<div id="m1" data-menu-keys="fx:menu-keys" data-count="3" data-at="0" data-typed="fx:typeahead:a"></div>`
		second  = `<div id="m2" data-menu-keys="fx:menu-keys" data-count="3" data-at="0" data-typed="fx:typeahead:aa"></div>`
		invoked = `data-invoked="fx:invokers"`
		popover = `data-popover="fx:invokers-popover"`
		title   = `<title>fx:wrap</title>`
	)
	for _, tt := range []struct {
		page         render.Page
		own, control []string // what is in the DOM after the page's own script, and after the control
	}{
		{home, []string{closed}, []string{closed}},
		{guide, []string{closed, stopped}, []string{closed, wrapped, title}},
		{syntax, []string{closed, wrapped, second, invoked, title}, []string{closed, wrapped, second, invoked, popover, title}},
		{cli, nil, nil},
		// The browser encodes the pathname; the control finds the page.
		{senor, []string{stopped}, []string{wrapped, title}},
		{percent, []string{stopped}, []string{wrapped, title}},
	} {
		own, _, reports := Build(tt.page, opts)
		if len(reports) > 0 {
			t.Fatalf("reports: %+v", reports)
		}
		for _, run := range []struct {
			name, js string
			want     []string
		}{{"its own script", own, tt.own}, {"the control", control, tt.control}} {
			got := dom(tt.page, run.js)
			for _, s := range []string{closed, stopped, wrapped, second, invoked, popover, title} {
				if strings.Contains(got, s) != slices.Contains(run.want, s) {
					t.Errorf("%s, %s: %s in the DOM: %v\n%s", tt.page.Pathname, run.name, s, strings.Contains(got, s), got)
				}
			}
			t.Logf("%s, %s:\n%s", tt.page.Pathname, run.name, got)
		}
	}
}
