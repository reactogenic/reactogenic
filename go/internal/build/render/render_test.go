package render

import (
	"bytes"
	"flag"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/TypeScript/tsc/rtsx"

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
	want := []string{
		"nav.ts", "pages/attrs/index.tsx", "pages/guide/examples.tsx", "pages/guide/index.rtsx", "pages/guide/intro.rtsx",
		"pages/index.rtsx", "pages/reference/cli/index.rtsx", "shell.ts", "ui/dialog.rtsx", "ui/layout.rtsx",
	}
	if !slices.Equal(own, want) {
		t.Errorf("the closure\n got  %q\n want %q", own, want)
	}
	if core == 0 {
		t.Error("no module of @reactogenic/core in the closure")
	}
	sources := strings.Join(b.sites.sources, "\n")
	for _, production := range []string{staticRenderer, "react-jsx-runtime.production.js", namespace + ":sandbox", namespace + ":jsx"} {
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
		// React's own exception — an object as a child — has no frame of
		// the project: the element of the component called last.
		"pages/child/index.rtsx:15:7 error shell-error",
		"pages/console/index.rtsx:6:11 warning shell-console",         // `log` of console.log(…)
		"pages/crypto/index.rtsx:6:18 error shell-nondeterministic",   // `getRandomValues`
		"pages/handler/index.rtsx:6:29 error shell-handler",           // the attribute `onClick`
		"pages/new-date/index.rtsx:9:28 error shell-nondeterministic", // `Date` of `new Date()`; `new Date(2026, 9, 4)` above it is fine
		"pages/no-default/index.rtsx:1:1 error page-no-default",
		"pages/not-document/index.rtsx:6:1 error page-not-document",      // `export default`
		"pages/performance/index.rtsx:6:34 error shell-nondeterministic", // `now`
		"pages/random/index.rtsx:4:25 error shell-nondeterministic",      // `random`
		"pages/recursion/index.rtsx:4:10 error shell-error",              // the call that never returns
		"pages/spread/index.rtsx:6:22 error shell-handler",               // `<input {...props}>`: the element
		"pages/throws/index.rtsx:8:15 error shell-error",                 // `Error` of `throw new Error(…)`
		"pages/type-error/index.rtsx:10:27 error shell-error",            // after three characters outside the BMP: columns are characters
		"stamp.ts:4:19 error shell-nondeterministic",                     // `now` of Date.now(), in the helper a component called
		"ui/counter.rtsx:2:17 error shell-react",                         // `useState`, in the import
		"ui/counter.rtsx:9:9 error shell-react",                          // `useEffect` of React.useEffect
	}
	if got := lines(f.dir, reports); !slices.Equal(got, want) {
		t.Errorf("reports\n got  %q\n want %q", got, want)
	}
	golden(t, "bad.txt", printedReports(f.dir, reports))
	// A page that threw has no record. shell-react is in the text: the page
	// renders — React's hooks run once, on the server's terms — and `build`
	// stops on the report.
	if got := pathnames(pages); !slices.Equal(got, []string{"/console/", "/state/"}) {
		t.Errorf("pages: %q", got)
	}
}

// A loop without an end is ended, reported, and the next page renders.
func TestTimeout(t *testing.T) {
	f := load(t, "runaway")
	slices.Reverse(f.routes) // the loop first
	pages, reports := Render(f.program, f.routes, Options{Timeout: 2 * time.Second})
	if len(reports) != 1 || reports[0].Code != "shell-error" || reports[0].File != f.dir+"/pages/loop/index.rtsx" || reports[0].Line != 2 ||
		reports[0].Message != "Rendering did not end in 2s: a loop without an end?" {
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

// Without react-dom there is no renderer: one report, no page.
func TestNoReact(t *testing.T) {
	f := load(t, "runaway")
	pages, reports := Render(f.program, f.routes, Options{Dir: t.TempDir()})
	if len(pages) != 0 || !slices.ContainsFunc(reports, func(r report.Report) bool {
		return r.Code == "render-bundle" && r.Severity == report.Error && strings.Contains(r.Message, "react-dom is not installed in ")
	}) {
		t.Errorf("pages %q, reports: %+v", pathnames(pages), reports)
	}
}
