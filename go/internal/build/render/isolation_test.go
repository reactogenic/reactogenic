package render

import (
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
