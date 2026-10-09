package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"modernc.org/quickjs"
)

// A page is rendered in a realm of its own (builder.md, *Shell code in phase
// 2*): what a module keeps at its top level, what is put on the global
// object or on a prototype while one page renders, is not there for the
// next. So a page's bytes do not depend on which pages were rendered before
// it — the same in either order of the routes.
func TestRealmPerPage(t *testing.T) {
	f := load(t, "state")
	want := map[string]string{
		"/":       `<html lang="en"><head></head><body><h2 id="install" data-visits="1" data-patched="no">Install</h2></body></html>`,
		"/guide/": `<html lang="en"><head></head><body><h2 id="install" data-visits="1" data-patched="no">Install</h2><a href="#install">Install</a></body></html>`,
	}
	for _, order := range []string{"as routed", "reversed"} {
		pages, reports := Render(f.program, f.routes, Options{})
		if len(reports) != 0 || len(pages) != len(want) {
			t.Fatalf("%s: pages %q, reports %q", order, pathnames(pages), lines(f.dir, reports))
		}
		for _, page := range pages {
			if page.HTML != want[page.Pathname] {
				t.Errorf("%s: %s\n got  %s\n want %s", order, page.Pathname, page.HTML, want[page.Pathname])
			}
		}
		slices.Reverse(f.routes)
	}
}

// What the collector does is not the page's to see: a WeakRef and a
// FinalizationRegistry tell when an object was freed (builder.md, *The
// engine*).
func TestNoCollector(t *testing.T) {
	f := load(t, "state")
	b, reports := build(f.program, f.routes, f.dir, variant{platform: api.PlatformBrowser})
	if b == nil {
		t.Fatalf("no bundle: %q", lines(f.dir, reports))
	}
	e, thrown, err := start(b.code, 30*time.Second)
	if err != nil || thrown != nil {
		t.Fatalf("the engine: %v, %+v", err, thrown)
	}
	defer e.close()
	got, err := e.vm.Eval(`["WeakRef", "FinalizationRegistry"].filter((name) => name in globalThis).join()`, quickjs.EvalGlobal)
	if err != nil || got != "" {
		t.Errorf("in the engine: %v, %v", got, err)
	}
}

// Nothing suspends in the shell, and nothing swallows what shell code throws
// (builder.md, *Shell code in phase 2*): a `<Suspense>` boundary renders its
// fallback for any exception of its content, silently — a shell rule broken
// inside one would build a page without what it wraps.
func TestSuspends(t *testing.T) {
	f := load(t, "suspends")
	pages, reports := Render(f.program, f.routes, Options{})
	want := []string{
		// A boundary the builder's runtime did not see (/by-hand/): what it
		// swallowed, where it was thrown.
		"clock.tsx:3:22 error shell-nondeterministic: Date.now() makes the shell irreproducible",
		// The component, at its element: what it returned is a promise.
		"pages/async/index.tsx:16:9 error shell-react: The shell cannot suspend: `Feed` is an async component",
		// A class with an effect is the hook's mistake in the older form.
		"pages/class/index.tsx:23:9 error shell-react: The shell cannot use React state or effects: `componentDidMount` of `Ticker`",
		"pages/lazy/index.tsx:9:9 error shell-react: The shell cannot suspend: a `lazy` component", // where it is rendered, not where `lazy()` made it
		"pages/quiet/index.tsx:9:16 error shell-react: The shell cannot suspend: `<Suspense>`",     // `createElement(React.Suspense, …)`, with nothing in it that suspends
		"pages/suspense/index.tsx:10:9 error shell-react: The shell cannot suspend: `<Suspense>`",  // the element — not the fallback, silently
		"pages/use/index.tsx:6:14 error shell-react: The shell cannot suspend: `use` of a promise", // `use` of use(items)
	}
	var got []string
	for _, r := range reports {
		got = append(got, line(f.dir, r)+": "+r.Message)
	}
	if !slices.Equal(got, want) {
		t.Errorf("reports\n got  %s\n want %s", strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}
	// Context is render-time React, by `use` too; a `lazy` component that
	// nothing renders is nothing; a class that only renders is a component.
	if len(pages) != 1 || pages[0].Pathname != "/context/" || !strings.Contains(pages[0].HTML, `<section data-tone="plain"><p>dark dark</p></section>`) {
		t.Errorf("pages: %+v", pages)
	}
}

// A page that allocates without end is ended as one that loops without end
// is (builder.md, *Shell code in phase 2*, shell-error): at 300 MB a second,
// the thirty seconds of the timeout are the machine's memory. The page after
// it renders: its runtime is its own.
func TestMemory(t *testing.T) {
	f := load(t, "memory")
	pages, reports := Render(f.program, f.routes, Options{Memory: 64 << 20})
	if len(reports) != 1 || reports[0].Code != "shell-error" || reports[0].File != f.dir+"/pages/index.tsx" ||
		reports[0].Message != "Rendering took more than 64 MiB of memory: a loop that keeps what it makes?" {
		t.Errorf("reports: %+v", reports)
	}
	if len(pages) != 1 || pages[0].Pathname != "/fine/" || pages[0].HTML != `<html lang="en"><head></head><body>fine</body></html>` {
		t.Errorf("pages: %+v", pages)
	}
}

// The timeout is time the page ran, not a moment of the wall clock: a
// machine that sleeps while a page renders, or whose clock is set, moves the
// wall clock and not the page. The engine's own timeout is such a moment, and
// ended TestMemory's page with the timeout's message after a fraction of a
// second (plan.md, RGP2-071) — so the test is run again under a wall clock
// that steps a minute at every reading. On macOS, where a library can stand
// in for the system's clock; elsewhere the Go runtime reads it past libc.
func TestTimeoutClock(t *testing.T) {
	if os.Getenv("RG_CLOCK_STEPS") != "" {
		// The child: is the clock the stepping one?
		if first, second := time.Now().Unix(), time.Now().Unix(); second-first < 60 {
			t.Fatalf("the wall clock does not step: %d, %d", first, second)
		}
		return
	}
	cc, err := exec.LookPath("cc")
	if runtime.GOOS != "darwin" || err != nil {
		t.Skip("needs macOS and a C compiler: the wall clock is replaced by a library")
	}
	library := filepath.Join(t.TempDir(), "steps.dylib")
	if out, err := exec.Command(cc, "-dynamiclib", "-o", library, filepath.Join("testdata", "clock", "steps.c")).CombinedOutput(); err != nil {
		t.Skipf("the stepping clock does not compile: %v\n%s", err, out)
	}
	child := exec.Command(os.Args[0], "-test.run", "^(TestTimeoutClock|TestMemory|TestRealmPerPage)$", "-test.v")
	child.Env = append(os.Environ(), "DYLD_INSERT_LIBRARIES="+library, "RG_CLOCK_STEPS=1")
	out, err := child.CombinedOutput()
	if !strings.Contains(string(out), "--- PASS: TestTimeoutClock") {
		t.Skipf("the stepping clock was not loaded: %v\n%s", err, out)
	}
	if err != nil || !strings.Contains(string(out), "--- PASS: TestMemory") || !strings.Contains(string(out), "--- PASS: TestRealmPerPage") {
		t.Errorf("under a wall clock that steps: %v\n%s", err, out)
	}
}

// What `variants()` of @reactogenic/core resolves is in the page's record
// (builder.md, *What shell code can ask the builder*): each class
// once, in the order first resolved, with the components that resolved it —
// per page, and only while a page renders.
func TestClasses(t *testing.T) {
	f := load(t, "classes")
	pages, reports := Render(f.program, f.routes, Options{})
	if len(reports) != 0 || len(pages) != 2 {
		t.Fatalf("pages %q, reports %q", pathnames(pages), lines(f.dir, reports))
	}
	home, other := pages[0], pages[1]
	if want := []string{"button", "button-sm", "toolbar", "button-ghost"}; !slices.Equal(home.Classes, want) {
		t.Errorf("/: classes %q, want %q", home.Classes, want)
	}
	for class, by := range map[string][]string{"button": {"Button", "Toolbar"}, "button-sm": {"Button"}, "toolbar": {"Toolbar"}, "button-ghost": {"Toolbar", "Button"}} {
		if !slices.Equal(home.Resolved[class], by) {
			t.Errorf("/: %s was resolved by %q, want %q", class, home.Resolved[class], by)
		}
	}
	// The call of a module's top level was not a page's: `loaded`, and the
	// class of the value it chose, are in no record — the HTML has what was
	// written (`data-loaded`), and a class written on an element is there.
	if len(home.Resolved) != 4 || !strings.Contains(home.HTML, `<body data-loaded="loaded button-lg"><button class="button button-sm">Small</button><button class="button">Medium</button><div class="toolbar button button-ghost"></div><button class="button button-sm button-ghost">Ghost</button><p class="literal">`) {
		t.Errorf("/: resolved %v in\n%s", home.Resolved, home.HTML)
	}
	// A page's record is its own.
	if !slices.Equal(other.Classes, []string{"button"}) || !slices.Equal(other.Resolved["button"], []string{"Button"}) || len(other.Resolved) != 1 {
		t.Errorf("/other/: classes %q, resolved %v", other.Classes, other.Resolved)
	}
}
