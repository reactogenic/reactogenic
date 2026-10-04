package render

import (
	"bytes"
	"flag"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/TypeScript/tsc/rtsx"
	"modernc.org/quickjs"

	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// The transform is registered once for the process, as the command does.
func TestMain(m *testing.M) {
	mapper.RegisterStrict("test")
	os.Exit(m.Run())
}

// fixture is a site of testdata: a project on disk, because its pages render
// with the repository's React — react, react-dom and @reactogenic/core are
// resolved from the root's node_modules, as a project resolves them.
type fixture struct {
	dir     string
	program *rtsx.Program
	routes  []Route // pages/**/index.rtsx and index.tsx, by pathname
}

// load builds the program of testdata/<name>. The fixtures are projects that
// `check` passes: what is built is what was checked.
func load(t *testing.T, name string) fixture {
	t.Helper()
	repo, _ := filepath.Abs("../../../..")
	if _, err := os.Stat(filepath.Join(repo, "node_modules", "react-dom", "package.json")); err != nil {
		t.Fatalf("the fixtures render with the repository's React, and %s has no node_modules/react-dom: run `pnpm install` there", repo)
	}
	dir, _ := filepath.Abs(filepath.Join("testdata", name))
	dir, _ = filepath.EvalSymlinks(dir)
	f := fixture{dir: filepath.ToSlash(dir)}
	var diagnostics []*rtsx.Diagnostic
	f.program, diagnostics = rtsx.NewProgram(f.dir+"/tsconfig.json", f.dir, rtsx.OSFS())
	if f.program == nil {
		t.Fatalf("%s: no program", name)
	}
	for _, r := range report.Program(f.program, diagnostics) {
		t.Errorf("%s does not check: %s", name, line(f.dir, r))
	}
	filepath.WalkDir(dir+"/pages", func(p string, d fs.DirEntry, err error) error {
		if base := filepath.Base(p); base == "index.rtsx" || base == "index.tsx" {
			p = filepath.ToSlash(p)
			f.routes = append(f.routes, Route{Pathname: strings.TrimPrefix(filepath.ToSlash(filepath.Dir(p)), f.dir+"/pages") + "/", File: p})
		}
		return nil
	})
	slices.SortFunc(f.routes, func(a, b Route) int { return strings.Compare(a.Pathname, b.Pathname) })
	return f
}

// line is a report as the tests state it: `pages/index.rtsx:6:29 error shell-handler`.
func line(dir string, r report.Report) string {
	return strings.TrimPrefix(r.File, dir+"/") + ":" + strconv.Itoa(r.Line) + ":" + strconv.Itoa(r.Col) + " " + r.Severity.String() + " " + r.Code
}

func lines(dir string, reports []report.Report) []string {
	var out []string
	for _, r := range reports {
		out = append(out, line(dir, r))
	}
	return out
}

func pathnames(pages []Page) []string {
	var out []string
	for _, p := range pages {
		out = append(out, p.Pathname)
	}
	return out
}

// golden compares got with testdata/golden/<name>; `go test -update`
// rewrites it.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", filepath.FromSlash(name))
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("no golden output %s: run `go test -update`", path)
		return
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

// printed is the whole of the reports as `reactogenic check` prints them —
// messages and related lines — for the goldens.
func printedReports(dir string, reports []report.Report) string {
	var b bytes.Buffer
	check.Print(&b, reports, dir, false, func(string) (string, bool) { return "", false })
	return b.String()
}

// The fixture site (RGP2-011, *Done when*): a shared layout, slots, `Each`,
// `Match` / `Switch`, segment roots, the three intrinsics — and a page of
// attribute cases. The HTML of each page is its golden file, byte for byte.
func TestRender(t *testing.T) {
	f := load(t, "site")
	pages, reports := Render(f.program, f.routes, Options{})
	if len(reports) != 0 {
		t.Fatalf("reports: %q", lines(f.dir, reports))
	}
	overlays := Mount{Module: "ui/behaviors/overlays"}
	dialog := func(id string, modal bool) Mount {
		return Mount{Module: "ui/behaviors/invokers", ID: id, Flags: map[string]bool{"RG_DIALOG_MODAL": modal}}
	}
	want := []struct {
		pathname, golden string
		mounts           []Mount // in render order
		components       map[string]int
		html             []string // what each feature leaves in the page
	}{
		{"/", "index", []Mount{overlays, dialog("d1", true)},
			map[string]int{"HomePage": 1, "DocsLayout": 1, "Each": 1, "Dialog": 1},
			[]string{
				`<nav id="m1" aria-label="Docs">`,                                           // useShellId("m")
				`<a href="/" aria-current="page">Introduction</a>`,                          // pathname()
				`<h1 class="hero">Reactogenic — docs</h1>`,                                  // $Title: its prop replaces the attachment's, its body the fallback
				`<p class="version">v0.1.0</p><button`,                                      // Match with params; the Match on undefined renders nothing
				`<button type="button" popoverTarget="d1" class="primary">Install</button>`, // $Trigger, one level down
				`<dialog id="d1" aria-labelledby="r1" aria-modal="true"><h2 id="r1">`,       // useShellId(): r1
				`<title>Reactogenic</title>`,                                                // the shorthand prop `title`
			}},
		{"/attrs/", "attrs", nil, map[string]int{"AttrsPage": 1},
			[]string{`aria-expanded="false"`, `<div aria-hidden="true">`, `stroke-width="2"`, `--rg-gap:4;width:50%`, `tabindex="0"`, `<textarea rows="3">text</textarea>`}},
		// A barrel: `Stepper`, which has state, is in the page's closure and
		// is not rendered — nothing to report, nothing in the record.
		{"/barrel/", "barrel", []Mount{overlays},
			map[string]int{"BarrelPage": 1, "DocsLayout": 1, "Each": 1, "Card": 1},
			[]string{`<article><h2>Card</h2><p>Only Card is rendered.</p></article>`}},
		// The shell's time zone is UTC, whatever the machine's (TestTimeZone).
		{"/dates/", "dates", nil, map[string]int{"DatesPage": 1},
			[]string{
				`<time dateTime="2026-10-04T09:30:00.000Z">0</time>`,                    // new Date(2026, 9, 4, 9, 30), getTimezoneOffset()
				`<p>2026-10-04T23:30:00.000Z`,                                           // a date-time string without a zone
				`<p>1791072000000 `,                                                     // Date.parse of one
				`<p>Sun Oct 04 2026 09:30:00 GMT+0000 (Coordinated Universal Time)</p>`, // as V8 writes it under TZ=UTC
				`<p>2026-03-04T02:01:00.000Z 1999-01-01T00:00:00.000Z `,                 // setMonth, setHours; a two-digit year
				`<p>parses its own</p><p>parses toUTCString</p>`,
			}},
		{"/guide/", "guide", []Mount{overlays},
			map[string]int{"GuidePage": 1, "DocsLayout": 1, "Each": 1, "Intro": 1, "Examples": 1},
			[]string{
				`<a href="/guide/" aria-current="page"><b>›</b>Guide</a>`,                  // a function slot, per item, with its args
				`<a href="/">Introduction</a>`,                                             // …and Match on its `active`
				`<h1 class="title">Guide</h1>`,                                             // $Title not filled: the fallback
				`<section id="intro"><p>A slot is a prop value, on /guide/.</p></section>`, // a segment root; pathname() inside it
				`<section id="examples" class="band"><pre>&lt;$Title&gt;Slots&lt;/$Title&gt;</pre></section>`,
				`<aside class="aside">On this page</aside>`,
			}},
		{"/reference/cli/", "reference_cli", []Mount{overlays, dialog("d1", false), dialog("d2", true)},
			map[string]int{"CliPage": 1, "DocsLayout": 1, "Each": 2, "Badge": 3, "Dialog": 2},
			[]string{
				`<ul id="r1"><li data-index="0"><code>check</code> <b>stable</b></li>`, // Each, Switch
				`<li data-index="2"><code>serve</code> </li></ul>`,                     // no $Case matched: nothing
				`<dialog id="d2" aria-labelledby="r3" aria-modal="true">`,              // ids count per prefix, per page
				`<button type="button" popoverTarget="d1">Open</button>`,               // $Trigger not filled
			}},
		// An element's `type` is the component: `Tabs` finds its five `Tab`s,
		// however each was made, and `Other`, which it drops, is not counted.
		// A `memo` and a `forwardRef` are counted by their function.
		{"/types/", "types", nil,
			map[string]int{"TypesPage": 1, "Tabs": 1, "Tab": 5, "Badge": 1, "Field": 1},
			[]string{
				`<ul data-tabs="5" data-role="tab" data-name="Tab"><li>one</li><li>two</li><li>by hand</li><li>cloned</li><li>spread</li></ul>`,
				`<p>true true</p><b>memo</b><input name="q"/>`,
			}},
		// JavaScript behind declarations, through an alias; a compiled
		// package's components are in the record; a JSX pragma.
		{"/vendor/", "vendor", nil,
			map[string]int{"VendorPage": 1, "Note": 1, "VendorBadge": 1, "Dot": 1},
			[]string{`<p>legacy js</p><span class="badge" data-id="string"><i class="dot"></i>compiled</span>`}},
	}
	if got := pathnames(pages); len(got) != len(want) {
		t.Fatalf("pages: %q", got)
	}
	for i, w := range want {
		page := pages[i]
		if page.Pathname != w.pathname || page.File != f.routes[i].File {
			t.Errorf("page %d is %s (%s)", i, page.Pathname, page.File)
		}
		golden(t, "site/"+w.golden+".html", page.HTML)
		for _, html := range w.html {
			if !strings.Contains(page.HTML, html) {
				t.Errorf("%s: no %s in\n%s", w.pathname, html, page.HTML)
			}
		}
		if !reflect.DeepEqual(page.Mounts, w.mounts) {
			t.Errorf("%s: mounts\n got  %+v\n want %+v", w.pathname, page.Mounts, w.mounts)
		}
		if !maps.Equal(page.Components, w.components) {
			t.Errorf("%s: components\n got  %v\n want %v", w.pathname, page.Components, w.components)
		}
	}
}

// The render bundle (RGP2-010): the pages' module closure from the program,
// React's production code, a stylesheet as nothing.
func TestBundle(t *testing.T) {
	f := load(t, "site")
	b, reports := build(f.program, f.routes, f.dir, variant{platform: api.PlatformBrowser})
	if b == nil {
		t.Fatalf("no bundle: %q", lines(f.dir, reports))
	}
	// The files of the project in the closure; @reactogenic/core's follow.
	// `~/shell.ts` is a `paths` alias and `../ui/layout` an .rtsx module:
	// only the program resolves them — esbuild is told of no tsconfig.
	var own []string
	core := 0
	for _, file := range b.files {
		if name, ok := strings.CutPrefix(file.FileName(), f.dir+"/"); ok {
			own = append(own, name)
		} else if strings.Contains(file.FileName(), "/packages/core/src/") {
			core++
		} else {
			t.Errorf("in the bundle: %s", file.FileName())
		}
	}
	// `legacy.js` and `vendor/badge/index.js` are in the bundle and not in
	// the program: their declarations are.
	want := []string{
		"kit/card.rtsx", "kit/index.ts", "kit/stepper.rtsx", "nav.ts", "pages/attrs/index.tsx", "pages/barrel/index.rtsx",
		"pages/dates/index.tsx", "pages/guide/examples.tsx", "pages/guide/index.rtsx", "pages/guide/intro.rtsx",
		"pages/index.rtsx", "pages/reference/cli/index.rtsx", "pages/types/index.tsx", "pages/vendor/index.tsx",
		"shell.ts", "ui/dialog.rtsx", "ui/layout.rtsx",
	}
	if !slices.Equal(own, want) {
		t.Errorf("the closure\n got  %q\n want %q", own, want)
	}
	if core == 0 {
		t.Error("no module of @reactogenic/core in the closure")
	}
	sources := strings.Join(b.sites.sources, "\n")
	for _, production := range []string{staticRenderer, "react-jsx-runtime.production.js", "legacy.js", "vendor/badge/index.js", namespace + ":sandbox", namespace + ":jsx", namespace + ":react"} {
		if !strings.Contains(sources, production) {
			t.Errorf("not in the bundle: %s", production)
		}
	}
	for _, absent := range []string{".development.", "react-dom-server.browser", "layout.css"} {
		if strings.Contains(sources, absent) {
			t.Errorf("in the bundle: %s\n%s", absent, sources)
		}
	}
	if strings.Contains(b.code, "only-in-the-stylesheet") {
		t.Error("the stylesheet's text is in the bundle")
	}
}

// Every shell rule, and the two rules of a page, at the line and column of
// the .rtsx (or .ts) the author wrote (builder.md, *Shell code in phase 2*,
// *Routes*); the golden file holds the messages and the component stacks.
func TestShellErrors(t *testing.T) {
	f := load(t, "bad")
	pages, reports := Render(f.program, f.routes, Options{})
	want := []string{
		"pages/by-hand/index.tsx:6:3 error shell-handler", // `createElement("button", { onClick })`: its statement
		// React's own exception — an object as a child — has no frame of
		// the project: the element of the component called last.
		"pages/child/index.rtsx:15:7 error shell-error",
		"pages/clone/index.tsx:8:14 error shell-handler",                 // `cloneElement(button, { onClick })`
		"pages/commented/index.rtsx:12:1 error page-not-document",        // the `export default` that is code, not the one in a comment or a template
		"pages/console/index.rtsx:6:11 warning shell-console",            // `log` of console.log(…)
		"pages/crypto/index.rtsx:6:18 error shell-nondeterministic",      // `getRandomValues`
		"pages/export-as/index.rtsx:5:10 error page-not-document",        // `ExportAsPage as default`
		"pages/handler/index.rtsx:6:29 error shell-handler",              // the attribute `onClick`
		"pages/key-spread/index.rtsx:6:41 error shell-error",             // `Error`; the golden has `in Row` at its element
		"pages/lib-handler/index.tsx:7:9 error shell-handler",            // a package's `jsx("button", { onClick })`: the element of its component
		"pages/lib-state/index.tsx:7:9 error shell-react",                // a package's `useState`: the same
		"pages/locale/index.tsx:6:28 error shell-error",                  // `toLocaleString`: no Intl
		"pages/new-date/index.rtsx:9:28 error shell-nondeterministic",    // `Date` of `new Date()`; `new Date(2026, 9, 4)` above it is fine
		"pages/no-default/index.rtsx:1:1 error page-no-default",          //
		"pages/not-document/index.rtsx:6:1 error page-not-document",      // `export default`
		"pages/performance/index.rtsx:6:34 error shell-nondeterministic", // `now`
		"pages/pragma/index.tsx:7:31 error shell-handler",                // under `@jsxImportSource react`
		"pages/random/index.rtsx:4:25 error shell-nondeterministic",      // `random`
		"pages/recursion/index.rtsx:4:10 error shell-error",              // the call that never returns
		"pages/shell-id/index.tsx:6:14 error shell-error",                // `useShellId` of useShellId("d1"): `d11` would be two ids
		"pages/spread/index.rtsx:6:22 error shell-handler",               // `<input {...props}>`: the element
		"pages/throws/index.rtsx:8:15 error shell-error",                 // `Error` of `throw new Error(…)`
		"pages/type-error/index.rtsx:10:27 error shell-error",            // after three characters outside the BMP: columns are characters
		"stamp.ts:4:19 error shell-nondeterministic",                     // `now` of Date.now(), in the helper a component called
		"ui/counter.rtsx:8:19 error shell-react",                         // `useCount` of useCount(0): `useState`, renamed, at the call
	}
	if got := lines(f.dir, reports); !slices.Equal(got, want) {
		t.Errorf("reports\n got  %q\n want %q", got, want)
	}
	golden(t, "bad.txt", printedReports(f.dir, reports))
	// A page that threw has no record.
	if got := pathnames(pages); !slices.Equal(got, []string{"/console/"}) {
		t.Errorf("pages: %q", got)
	}
}

// shell-react is the hook, however shell code reached it (builder.md, *Shell
// code in phase 2*): re-exported, renamed, through a namespace, destructured,
// computed. One module rendered at a pathname per form; `useId` is not state.
func TestShellReact(t *testing.T) {
	f := load(t, "bad")
	file := f.dir + "/forms/hooks.tsx"
	var routes []Route
	for _, form := range []string{"reexported", "renamed", "destructured", "namespace", "computed", "layout-effect", "plain"} {
		routes = append(routes, Route{Pathname: "/" + form + "/", File: file})
	}
	pages, reports := Render(f.program, routes, Options{})
	want := []string{
		"forms/hooks.tsx:12:18 error shell-react: The shell cannot use React state or effects: `useState`",   // export { useState } from "react"
		"forms/hooks.tsx:14:7 error shell-react: The shell cannot use React state or effects: `useEffect`",   // export { useEffect as useFx } from "react"
		"forms/hooks.tsx:17:25 error shell-react: The shell cannot use React state or effects: `useRef`",     // const { useRef } = React
		"forms/hooks.tsx:19:20 error shell-react: The shell cannot use React state or effects: `useReducer`", // export * as R from "react"
		"forms/hooks.tsx:21:34 error shell-react: The shell cannot use React state or effects: `useState`",   // React["useState"]
		"forms/hooks.tsx:23:13 error shell-react: The shell cannot use React state or effects: `useLayoutEffect`",
	}
	var got []string
	for _, r := range reports {
		got = append(got, line(f.dir, r)+": "+r.Message)
	}
	if !slices.Equal(got, want) {
		t.Errorf("reports\n got  %q\n want %q", got, want)
	}
	if len(pages) != 1 || pages[0].Pathname != "/plain/" || !strings.Contains(pages[0].HTML, "<b>useId is not state</b>") {
		t.Errorf("pages: %+v", pages)
	}
}

// A package that throws while it loads has no frame of the project and no
// component: reported at the project's import that leads to it, the
// package's own position as a related line.
func TestLoadError(t *testing.T) {
	f := load(t, "loads")
	pages, reports := Render(f.program, f.routes, Options{})
	if got := lines(f.dir, reports); !slices.Equal(got, []string{"ids.ts:1:21 error shell-nondeterministic"}) || len(pages) != 0 {
		t.Fatalf("reports %q, pages %q", got, pathnames(pages))
	}
	const want = "ids.ts(1,21): error shell-nondeterministic: Math.random() makes the shell irreproducible\n  vendor/seed/index.js:3:19 - thrown here\n"
	if got := printedReports(f.dir, reports); got != want {
		t.Errorf("printed\n got  %s want %s", got, want)
	}
}

// A loop without an end is ended, reported — at the loop: the column is
// wherever the timeout struck — and the next page renders. The timeout is
// also the bundle's, to load: three seconds are some sixty times what that
// takes, for a race-detector build.
func TestTimeout(t *testing.T) {
	f := load(t, "runaway")
	slices.Reverse(f.routes) // the loop first
	pages, reports := Render(f.program, f.routes, Options{Timeout: 3 * time.Second})
	if len(reports) != 1 || reports[0].Code != "shell-error" || reports[0].File != f.dir+"/pages/loop/index.rtsx" || reports[0].Line != 2 ||
		reports[0].Message != "Rendering did not end in 3s: a loop without an end?" {
		t.Errorf("reports: %+v", reports)
	}
	if len(pages) != 1 || pages[0].Pathname != "/" || pages[0].HTML != `<html lang="en"><head></head><body>fine</body></html>` {
		t.Errorf("pages: %+v", pages)
	}
}

// What a module does when it loads is no page's: reported once, and nothing
// renders.
func TestTopLevel(t *testing.T) {
	f := load(t, "toplevel")
	pages, reports := Render(f.program, f.routes, Options{})
	want := []string{"build-id.ts:2:9 warning shell-console", "build-id.ts:3:30 error shell-nondeterministic"}
	if got := lines(f.dir, reports); !slices.Equal(got, want) || len(pages) != 0 {
		t.Errorf("reports %q, pages %q", got, pathnames(pages))
	}
	golden(t, "toplevel.txt", printedReports(f.dir, reports))
}

// What esbuild cannot bundle is reported in the .rtsx: its position is in
// the emitted TSX, two lines further down.
func TestBundleError(t *testing.T) {
	f := load(t, "unbundled")
	pages, reports := Render(f.program, f.routes, Options{})
	want := []string{"pages/index.rtsx:2:18 error render-bundle"}
	if got := lines(f.dir, reports); !slices.Equal(got, want) || len(pages) != 0 {
		t.Errorf("reports %q, pages %q", got, pathnames(pages))
	}
	if len(reports) == 1 && reports[0].Message != `No loader is configured for ".svg" files: logo.svg` {
		t.Errorf("message: %s", reports[0].Message)
	}
}

// The sandbox (js/sandbox.js): the clock and the dice throw; a date that is
// given is a value, and `Date` is still `Date`.
func TestSandbox(t *testing.T) {
	f := load(t, "runaway")
	b, reports := build(f.program, f.routes, f.dir, variant{platform: api.PlatformBrowser})
	if b == nil {
		t.Fatalf("no bundle: %q", lines(f.dir, reports))
	}
	e, thrown, err := start(b.code, 30*time.Second)
	if err != nil || thrown != nil {
		t.Fatalf("the engine: %v, %+v", err, thrown)
	}
	defer e.close()
	const irreproducible = " makes the shell irreproducible"
	for _, c := range []struct{ expression, want string }{
		{`Date.now()`, "shell-nondeterministic: Date.now()" + irreproducible},
		{`new Date()`, "shell-nondeterministic: new Date()" + irreproducible},
		{`Date()`, "shell-nondeterministic: Date()" + irreproducible},
		{`new (new Date(0).constructor)()`, "shell-nondeterministic: new Date()" + irreproducible}, // not a way round
		{`Math.random()`, "shell-nondeterministic: Math.random()" + irreproducible},
		{`crypto.getRandomValues(new Uint8Array(1))`, "shell-nondeterministic: crypto.getRandomValues()" + irreproducible},
		{`crypto.randomUUID()`, "shell-nondeterministic: crypto.randomUUID()" + irreproducible},
		{`performance.now()`, "shell-nondeterministic: performance.now()" + irreproducible},
		{`new Date(0).toISOString()`, "1970-01-01T00:00:00.000Z"},
		{`new Date(Date.UTC(2026, 9, 4)).getUTCDay()`, "0"},
		{`Date.parse("2026-10-04T00:00:00Z")`, "1791072000000"},
		{`new Date(0) instanceof Date`, "true"},
		{`Object.prototype.toString.call(new Date(0))`, "[object Date]"},
		{`class Day extends Date {}; new Day(0).getTime()`, "0"},
		{`Math.max(1, 2)`, "2"},
		{`typeof __reactogenic_build`, "undefined"}, // only while a page renders

		// The shell's time zone is UTC, whatever the machine's: a date given
		// in local terms is a value (TestTimeZone runs this under others).
		{`new Date(2026, 9, 4).toISOString()`, "2026-10-04T00:00:00.000Z"},
		{`new Date(2026, 9, 4, 23, 59, 59, 999).getTime() === Date.UTC(2026, 9, 4, 23, 59, 59, 999)`, "true"},
		{`new Date(2026, 9, 4).getTimezoneOffset()`, "0"},
		{`[new Date(0).getFullYear(), new Date(0).getMonth(), new Date(0).getDate(), new Date(0).getDay(), new Date(0).getHours(), new Date(0).getYear()].join()`, "1970,0,1,4,0,70"},
		{`const d = new Date(0); d.setHours(5); d.setDate(2); d.setFullYear(2000, 1); d.toISOString()`, "2000-02-02T05:00:00.000Z"},
		{`new Date("2026-10-04T12:00").toISOString()`, "2026-10-04T12:00:00.000Z"}, // no zone: local, which is UTC
		{`Date.parse("2026-10-04T12:00:00.5")`, "1791115200500"},
		{`Date.parse("2026-10-04T12:00+02:00")`, "1791108000000"},
		{`Date.parse("2026-10-04")`, "1791072000000"},
		{`Date.parse("2026-10")`, "1790812800000"},
		{`new Date(0).toString()`, "Thu Jan 01 1970 00:00:00 GMT+0000 (Coordinated Universal Time)"},
		{`"" + new Date(0)`, "Thu Jan 01 1970 00:00:00 GMT+0000 (Coordinated Universal Time)"},
		{`new Date(0).toDateString() + "|" + new Date(0).toTimeString()`, "Thu Jan 01 1970|00:00:00 GMT+0000 (Coordinated Universal Time)"},
		{`Date.parse(new Date(5000).toString()) + Date.parse(new Date(5000).toUTCString())`, "10000"},
		{`new Date(new Date(7)).getTime() + new Date({ valueOf: () => 1 }).getTime()`, "8"},
		{`new Date(NaN).toString() + new Date(NaN).getHours() + new Date("2026-13-45").getTime()`, "Invalid DateNaNNaN"},
		// A date string of no standard format is parsed as its engine likes,
		// and in local time: refused.
		{`Date.parse("Oct 4 2026")`, `shell-nondeterministic: A date string that is not ISO 8601 ("Oct 4 2026")` + irreproducible},
		{`new Date("10/04/2026")`, `shell-nondeterministic: A date string that is not ISO 8601 ("10/04/2026")` + irreproducible},
		{`new Date("2026-10-04 12:00")`, `shell-nondeterministic: A date string that is not ISO 8601 ("2026-10-04 12:00")` + irreproducible},

		// No Intl in the engine: what would format without it is refused.
		{`(1234567.891).toLocaleString("en-US")`, "shell-error: Number.prototype.toLocaleString() needs Intl, which the builder's engine does not have"},
		{`10n.toLocaleString()`, "shell-error: BigInt.prototype.toLocaleString() needs Intl, which the builder's engine does not have"},
		{`new Date(0).toLocaleDateString("en-US")`, "shell-error: Date.prototype.toLocaleDateString() needs Intl, which the builder's engine does not have"},
		{`new Date(0).toLocaleTimeString()`, "shell-error: Date.prototype.toLocaleTimeString() needs Intl, which the builder's engine does not have"},
		{`new Date(0).toLocaleString()`, "shell-error: Date.prototype.toLocaleString() needs Intl, which the builder's engine does not have"},
		{`"a".localeCompare("B")`, "shell-error: String.prototype.localeCompare() needs Intl, which the builder's engine does not have"},
		{`"istanbul".toLocaleUpperCase("tr")`, "shell-error: String.prototype.toLocaleUpperCase() needs Intl, which the builder's engine does not have"},
		{`"I".toLocaleLowerCase()`, "shell-error: String.prototype.toLocaleLowerCase() needs Intl, which the builder's engine does not have"},
		{`[1.5].toLocaleString()`, "shell-error: Number.prototype.toLocaleString() needs Intl, which the builder's engine does not have"},
		{`["b", "a"].toLocaleString() + ({}).toLocaleString()`, "b,a[object Object]"}, // nothing to format
		// What the engine does not have, it does not pretend to have
		// (builder.md, *The engine*): a ReferenceError says so.
		{`["Intl", "URL", "URLSearchParams", "TextEncoder", "TextDecoder", "structuredClone", "atob", "btoa", "queueMicrotask", "setTimeout", "setInterval", "fetch", "Temporal", "process", "require"].filter((name) => name in globalThis).join()`, ""},
		{`new Intl.NumberFormat("en-US")`, "ReferenceError: 'Intl' is not defined"},
	} {
		got, err := e.vm.Eval(`(() => { try { return String(eval(`+strconv.Quote(c.expression)+`)); } catch (error) { return String(error); } })()`, quickjs.EvalGlobal)
		if err != nil || got != c.want {
			t.Errorf("%s: %v, %v; want %s", c.expression, got, err, c.want)
		}
	}
	// `console` takes any method, and collects the six that print.
	if _, err := e.vm.Eval(`console.group("g"); console.error("no", { a: 1 }, new TypeError("t")); console.table([1])`, quickjs.EvalGlobal); err != nil {
		t.Fatal(err)
	}
	if printed := e.console(); len(printed) != 1 || printed[0].Level != "error" || printed[0].Text != `no {"a":1} TypeError: t` {
		t.Errorf("console: %+v", printed)
	}
}

// A page that the tsconfig leaves out was not checked: it is not built.
func TestPageOutsideProgram(t *testing.T) {
	f := load(t, "runaway")
	outside := f.dir + "/elsewhere/index.tsx"
	pages, reports := Render(f.program, append(f.routes, Route{Pathname: "/elsewhere/", File: outside}), Options{})
	if len(pages) != 0 || len(reports) != 1 || reports[0].Code != "render-bundle" || reports[0].File != outside ||
		reports[0].Message != "The page is not a module of the project: the tsconfig does not include it" {
		t.Errorf("pages %q, reports: %+v", pathnames(pages), reports)
	}
}

// Without React there is no renderer: one report — not one per module of the
// builder's that imports it — of no file, no page.
func TestNoReact(t *testing.T) {
	f := load(t, "runaway")
	dir := t.TempDir()
	pages, reports := Render(f.program, f.routes, Options{Dir: dir})
	want := "react and react-dom are not installed in " + dir + ": the builder renders pages with the project's React"
	if len(pages) != 0 || len(reports) != 1 || reports[0].Code != "render-bundle" || reports[0].Severity != report.Error ||
		reports[0].File != "" || reports[0].Line != 0 || reports[0].Message != want {
		t.Errorf("pages %q, reports: %+v", pathnames(pages), reports)
	}
}

// The one change to React's static renderer (bundle.go, hooked): every call
// of a component goes through the builder, on its line; a renderer that
// calls its components otherwise is an error, not a page without a record.
func TestHooked(t *testing.T) {
	dir := t.TempDir()
	known, unknown := filepath.Join(dir, "known.js"), filepath.Join(dir, "unknown.js")
	const text = "\"use strict\";\nfor (request = Component(props, secondArg); again; )\n  request = Component(props, secondArg);\nrenderWithHooks(request, task, keyPath, type, props, void 0);\n"
	os.WriteFile(known, []byte(text), 0o644)
	os.WriteFile(unknown, []byte("\"use strict\";\nrequest = render(Component, props);\n"), 0o644)
	const want = "\"use strict\";\nfor (request = __reactogenic_call(Component, props, secondArg); again; )\n  request = __reactogenic_call(Component, props, secondArg);\nrenderWithHooks(request, task, keyPath, type, props, void 0);\n" +
		"\nvar __reactogenic_call = require(\"reactogenic:jsx\").call;\n"
	if got, err := hooked(known); err != nil || got != want {
		t.Errorf("hooked: %v\n%s", err, got)
	}
	if _, err := hooked(unknown); err == nil || !strings.Contains(err.Error(), "is not one the builder knows") {
		t.Errorf("a renderer of another shape: %v", err)
	}
	// The project's React is one the builder knows: TestRender's records.
}

// zoneEnv names, for the test binary that TestTimeZone runs, the offset its
// engine has when the zone reached it.
const zoneEnv = "REACTOGENIC_TEST_ZONE_OFFSET"

// The machine's time zone is not in the page (builder.md, *The engine*): the
// same goldens and the same sandbox under three zones — this test binary
// again, with TZ set. The engine by itself is in the machine's zone
// (TestEngineZone); on a platform where TZ does not reach it, that is said,
// and the zone is still tested against V8's (TestDifferential).
func TestTimeZone(t *testing.T) {
	if os.Getenv(zoneEnv) != "" {
		t.Skip("run by TestTimeZone")
	}
	for zone, offset := range map[string]string{"Asia/Tokyo": "-540", "America/New_York": "240", "UTC": "0"} {
		cmd := exec.Command(os.Args[0], "-test.run=^(TestEngineZone|TestRender|TestSandbox)$", "-test.v")
		cmd.Env = append(os.Environ(), "TZ="+zone, zoneEnv+"="+offset)
		out, err := cmd.CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte("--- PASS: TestRender")) || !bytes.Contains(out, []byte("--- PASS: TestSandbox")) {
			t.Errorf("TZ=%s: %v\n%s", zone, err, out)
		}
		if !bytes.Contains(out, []byte("--- PASS: TestEngineZone")) {
			t.Logf("TZ=%s did not reach the engine on this platform: nothing was proved for it", zone)
		}
	}
}

// The engine without the sandbox is in the machine's zone: what TestTimeZone
// sets is what it runs under.
func TestEngineZone(t *testing.T) {
	want := os.Getenv(zoneEnv)
	if want == "" {
		t.Skip("TestTimeZone runs it")
	}
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Close()
	if got, err := vm.Eval(`String(new Date(2026, 9, 4).getTimezoneOffset())`, quickjs.EvalGlobal); err != nil || got != want {
		t.Skipf("the engine's offset under TZ=%s: %v, %v; want %s", os.Getenv("TZ"), got, err, want)
	}
}
