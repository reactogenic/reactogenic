# Phase 2 plan: the builder

Specs: [research.md](research.md) (why), [builder.md](builder.md) (the
pipeline), [components.md](components.md) (the three components),
[decisions.md](decisions.md), [bet.md](bet.md) (the measurement). Tasks are
`RGP2-xxx`; sizes S / M / L as in phase 1.

**Outcome.** `reactogenic build` compiles a documentation site for
Reactogenic — four pages using `SideMenu`, `Dialog`, `DropdownMenu` — into
plain HTML + CSS + minimal JS per page, with no React in the output; and a
measurement that confirms or refutes the bet: *if the compiler is aware of
components it can ridiculously optimise the output build*.

Out of scope: a dev server, view transitions, dynamic segments.

> OPEN (for the owner, decisions.md): loop-produced slot items (`Each`
> around slot elements) are "phase 2" in CLAUDE.md and in
> phase01/syntax.md's roadmap note; no task here has them, and the site's
> menu is written out as slot elements.

What each task waits for — the *Depends on* lines, in one place:

```
001 ─▶ 002 ─▶ 010, 012, 020, 021, 022
010, 012 ─▶ 011              012 ─▶ 025
011, 020, 021, 022 ─▶ 030
025, 030 ─▶ 040 ─▶ 050 ─▶ 060
```

## M0 — Research and spec

### RGP2-001 — Research · M · done
Nine investigations, each independently re-verified; research.md and
`research/`.

### RGP2-002 — Spec · M · done
builder.md, components.md, this plan, decisions.md. The dependencies are in
`go/go.mod`: esbuild 0.28.2 (public API), `modernc.org/quickjs` 0.25.0,
`golang.org/x/net/html`, cascadia (tests only).

## M1 — The engine

Go packages live under `go/internal/build/`. The contract between them:

```go
// render — RGP2-010, 011
type Route struct{ Pathname, File string }        // "/guide/", "<abs>/pages/guide/index.rtsx"
type Mount struct {
	Module string           // "@reactogenic/ui/behaviors/menu-keys"
	ID     string           // "" for a page-level behaviour
	Flags  map[string]bool
}
type Page struct {
	Route
	HTML       string         // as rendered, without the doctype
	Mounts     []Mount        // in render order, duplicates kept
	Components map[string]int // function components React called, by the function's name
}
type Options struct {
	Dir     string        // the project's directory: react and react-dom are resolved from it; "": the program's
	Timeout time.Duration // of one page; 0: 30 s
	Memory  uintptr       // of one page's runtime, in bytes; 0: 1 GiB
}
func Render(program *rtsx.Program, routes []Route, opts Options) ([]Page, []report.Report)
// Pages: those that rendered, in the order of routes. Reports: errors (the caller
// stops on one) and warnings (shell-console). Mount.Flags holds what the use site
// passed, `false` included; nil when it passed none.

// cssprune — RGP2-020
type Options struct {
	Script  string       // the page's built script (behaviors.Build): what it names is "maybe"
	Builder []*html.Node // the elements of doc that packaging wrote: its <script>, its <style> or <link>
}
func PruneWith(css string, doc *html.Node, opts Options) (out string, stats Stats, err error)
func Prune(css string, doc *html.Node) (out string, stats Stats, err error)   // PruneWith for a page with nothing of the builder's
func Whole(css string) (Stats, error)                                         // the stats of a sheet a caller keeps whole; it says Why
// doc: the page as it is served, with `<!doctype html>` — the base in its links, the builder's elements in it.
// A <script> that Builder does not name is the page's own, and such a page is not pruned (Stats.Unpruned,
// Stats.Why); a <link rel=stylesheet> that it does not name is a sheet nobody bundled

// pagecheck — RGP2-021
func Check(page render.Page, doc *html.Node, routes []render.Route, files map[string]bool) []report.Report
// files: the output's other files, by their path from its root ("/favicon.svg")
// --base is not its business: the page is checked as rendered, before packaging prefixes its links —
// another document than the one the pruner is given

// behaviors — RGP2-022
type Options struct {
	Dir   string // the project directory: `mount()` specifiers resolve from it; it may be a symbolic link
	Base  string // --base, for the control's table
	Cache *Cache // optional: a module is read once per site (&behaviors.Cache{}, shared by the pages' builds)
}
type ModuleBytes struct{ Module, Path string; Bytes int } // an input of the script: the mount's specifier, the file, its bytes;
                                                          // Path behaviors.Entry, behaviors.Runtime for what is of no file: the rows add up to len(js)
func Build(page render.Page, opts Options) (js string, modules []ModuleBytes, reports []report.Report)
func BuildControl(pages []render.Page, opts Options) (js string, reports []report.Report)   // --no-specialize
// mount-no-element reads the ids off page.HTML: no parsed document is passed

// report — a diagnostic of a page as a whole: its file, no position; check.Print writes
// `pages/guide/index.rtsx: error idref-not-found: Page /guide/: …`
func Page(file, pathname, code, message string) Report

// build — RGP2-030: the driver
func Main(args []string, cwd string, stdout, stderr io.Writer) int // `reactogenic build`: the exit status
func CheckOut(opts Options) error                                  // may --out be emptied
func Run(opts Options) (reports []report.Report, bytes *Report, err error)
// bytes: the byte report, nil when an error among the reports stopped the build (nothing written)

// check — phase 1's, extended for the builder
func Program(config string, files []string) ([]report.Report, *rtsx.Program) // as `check`, through `references`:
                                                                             // the program of the project that lists files

// what the driver's CSS build and packaging share with the stages
func render.Source(program, path) *rtsx.SourceFile          // the one resolver, for an esbuild plugin:
func render.Resolve(program, importer, specifier) string    //   where the program resolved an import, "" → esbuild's
func render.Load(program, path) (*rtsx.SourceFile, api.OnLoadResult, error)
func render.Message(program, dir, code string, message api.Message) report.Report // at the author's position
func pagecheck.RootRelative(href string) (path string, ok bool) // the links Check reads, and --base prefixes
```

**The build-time protocol** between the engine and `@reactogenic/core`: while
a page renders, the engine provides

```ts
globalThis.__reactogenic_build = {
  pathname: string,                                   // "/guide/"
  id(prefix: string): string,                         // "d1"
  mount(module: string, id: string | undefined, flags: Record<string, boolean> | undefined): void,
}
```

and `pathname()`, `useShellId()`, `mount()` of `@reactogenic/core` use it when
it is there (builder.md, *What shell code can ask the builder*). Its absence
means React: `location.pathname`, `React.useId()`, nothing.

- `id(prefix)` is the prefix and a counter per prefix, per page, from 1:
  `d1`, `d2`, `m1`. No prefix (`""`, `undefined`): `r`. A prefix that ends
  in a digit throws (shell-error): `d1` + `1` is also the eleventh `d`.
- It is there only while a page renders: not while a module loads.
- The render bundle's own globals: `__reactogenic_render(pathname)` returns
  `{ html, mounts, components }` or throws; `__reactogenic_render_json` is
  the same as JSON, for the engine; `__reactogenic_console()` hands over what
  `console` collected.

### RGP2-010 — The render bundle · M · done
One esbuild build, in memory: a generated entry that imports every page and
React's static renderer and exposes `render(pathname)`.
- The plugin: `OnResolve` answers from the program for every import a program
  file makes (one resolver); `OnLoad` returns `file.Text()` — emitted TSX for
  `.rtsx` — with the `tsx` loader, automatic JSX, `jsxImportSource` the
  builder's own runtime (a virtual module wrapping `react/jsx-runtime`:
  raises shell-handler, notes where an element was written). `react` and
  `react/jsx-runtime` are the builder's for **every** module of the bundle
  but React's own: a compiled package's elements and hooks are under the
  same rules (builder.md, *The record*).
- `.css` imports load as empty here; `process.env.NODE_ENV` is
  `"production"`; an external source map, kept in memory.
- React's renderer: `renderToStaticMarkup` of the project's `react-dom`. The
  legacy static build runs in the engine with no polyfill (research); the
  public `react-dom/server` entry needs `MessageChannel` and `TextEncoder`
  stubs. Either, as long as the output equals Node's.
- shell-react: the hooks of the bundle's `react` throw when a page calls
  them (builder.md: not from the imports — that missed re-exports and
  compiled packages, and failed pages that render none of it).
- **Done when:** the bundle of a three-page fixture loads in Node and renders
  the same HTML as `react-dom/server` there.
- **Done:** `go/internal/build/render/bundle.go`. The renderer is the legacy
  static build, reached by path in the project's react-dom
  (`cjs/react-dom-server-legacy.browser.production.js`: not in its `exports`
  — a React that moves it is a render-bundle error, not a wrong page). The
  JSX is compiled with `jsxDev`: esbuild hands each element's position to the
  runtime, which is how a component stack has positions; React's production
  `jsx` does the work. The entry and the builder's three modules are virtual
  (`reactogenic:entry`, `:sandbox`, `:jsx`, `:react`). The pages' module
  closure is the program files esbuild loaded.
  - **Elements are React's, untouched.** Components are counted, and an
    exception gets its component stack, because React's static renderer
    calls a function component through the builder: `bundle.go`, `hooked`
    rewrites `Component(props, secondArg)` in the renderer's text (two
    places, `renderWithHooks`) as it is loaded. A renderer without that call
    is a render-bundle error. The published react-dom 19.0.0, 19.1.0, 19.2.0
    and 19.3.0 all have the file and the two calls (read, not rendered:
    only 19.3.0 is tested); 18.3.1 ships the file minified under another
    name — render-bundle. The first implementation put a wrapper function
    in `element.type`: `child.type === Tab` was false, and no fixture read
    `element.type`.
  - The metafile is kept: who imports what is how a package that throws as
    it loads is reported at the project's import.

### RGP2-011 — Execution · L · done
`modernc.org/quickjs`, **a runtime per page**: the bundle — the sandbox
prelude first — is compiled to the engine's bytecode once, and each route
gets a new runtime that loads it and calls `render(pathname)`. Module-level
state does not cross pages (builder.md, *Shell code in phase 2*).
- The prelude makes `Date.now`, argument-less `new Date()`, `Date()`,
  `Math.random`, `crypto.getRandomValues`, `crypto.randomUUID`,
  `performance.now` throw shell-nondeterministic; `WeakRef` and
  `FinalizationRegistry` are removed; `console` is collected and printed as
  warnings.
- A page has a timeout (30 s) and a memory limit (1 GiB): shell-error.
- Nothing suspends and nothing swallows an error: `Suspense`, `lazy`, `use`
  of a promise and an `async` component are shell-react; a render that ends
  after a component threw is that component's error. A class component with
  an effect (`componentDidMount`, …) is shell-react too.
- The prelude pins the time zone to UTC and makes what needs `Intl` throw
  (builder.md, *The engine*).
- An exception becomes a diagnostic at the source position: the engine's
  stack → the bundle's source map → the program file's position → the span
  map for `.rtsx` (`emit.Map`) — `report`'s output format, with the component
  stack as related lines.
- Differential test, in CI: every fixture page rendered by the engine equals
  the same bundle rendered by Node with `react-dom/server` — including the
  36 variant cases of research/evaluation.md (*Variant cases*: a
  hand-written serializer got 16 of them wrong, the Go evaluator 19).
- The six release targets build with `CGO_ENABLED=0`
  (`scripts/build-binaries.sh`).
- **Done when:** fixture pages with slots, `Each`, `Match`, segments and the
  three build-time functions (`pathname`, `useShellId`, `mount`: RGP2-012)
  render; each shell-* error is reported at its `.rtsx` line and column; the
  differential test passes.
- Depends on: 010, 012.
- **Done:** `engine.go`, `position.go`, `js/`. Measured, darwin-arm64: the
  four-page fixture is checked, bundled and rendered in ≈ 0.1 s.
  - A runtime per page costs 2.9 ms from the bytecode (27.6 ms from the
    bundle's text, 388 kB); compiling it, once, 29 ms. The eight pages of
    the fixture site render in 80 ms, the bundle's build included.
  - The differential test compares three renderings of every fixture page:
    the engine; the same bundle in Node under `TZ=Asia/Tokyo`; and an oracle
    in Node under `TZ=UTC` — the same pages with the public
    `react-dom/server` and React itself: nothing of the builder's is in it
    (no JSX runtime, no `react` wrapper, no sandbox, no changed renderer).
    Equal to the byte, on Node 25. It needs `node` and the root's
    `node_modules`: CI's `go` job installs them. Among the pages: one that
    reads `element.type`, one of dates, one with a compiled package.
  - `TestTimeZone` runs the goldens and the sandbox's cases again under
    `TZ=Asia/Tokyo`, `America/New_York` and `UTC` (the test binary, with
    `TZ` set): the engine by itself does follow `TZ`.
  - **The engine's call depth has to be bounded by us** (10 000): with the
    engine's default a function that recurses without end overflows the Go
    stack, which is fatal to the process — no `recover`. Bounded, it is
    shell-error at the call.
  - The six targets, `CGO_ENABLED=0`, stripped, with the package linked
    (`scripts/build-binaries.sh`, which prints "MB" for bytes / 2²⁰: MiB):
    darwin-arm64 27.3 → 34.5 MiB, darwin-x64 28.6 → 36.5, linux-arm64 26.4 →
    33.7, linux-x64 27.9 → 35.9, win32-arm64 26.5 → 33.7, win32-x64 28.2 →
    36.3 (+26–29%).
  - Not verified: Windows paths (the six targets build; the tests ran on
    macOS); React other than 19.3 — its static renderer reads no clock, so
    the sandbox has no exemption for React's own code, and the builder
    depends on its text in one place (above); whether `Math`'s
    transcendental functions give the same last digit on amd64 as on arm64
    (Go may fuse a multiply-add on arm64; the tests ran on arm64 only).

### RGP2-012 — `@reactogenic/core`: `pathname`, `useShellId`, `mount` · S · done
In `packages/core`, with the protocol above and the React fallbacks; unit
tests for both sides.
- **2026-10-05, the owner's rule on behaviours** (RGP2-050, *The rulings,
  built*): `mount(module, id?, flags?, data?)` — the fourth argument is the
  use site's data; the protocol's `mount` takes it, and `pathname` is the
  route's in every variant. Types `MountData`, `MountValue`.

## M2 — What the builder decides

### RGP2-020 — CSS pruning · L · done
`go/internal/build/cssprune`: builder.md, *CSS*, on the flat, minified CSS
esbuild's public API produces.
- A CSS reader for that form (rules, at-rules with blocks and statements,
  strings, comments, escapes); a selector parser (compounds, the four
  combinators, attribute selectors with operators and the `i` / `s` flags,
  `:is` / `:where` / `:not` / `:has` with selector lists, `&`, nesting already
  lowered); three-valued matching against `*html.Node`.
- Known traps, each with a test (research/css.md, *Verification*): a
  referenced `@keyframes` must stay; `&:is(…)` under lowered nesting; the `i`
  flag and HTML's case-insensitive attribute values; a token read only by
  `@container style()`; an emptied `@layer` block; `:has()`, `+`, `~` next
  to "maybe".
- Soundness test: for a corpus of pages and stylesheets, every selector the
  pruner drops matches no element under cascadia once its "maybe" parts are
  removed; and pruning is idempotent. cascadia knows nothing of HTML's case
  rules, SVG's names or `<noscript>`: each of those has a test of its own.
- The page's script (builder.md, *CSS*): what it names is "maybe"; a page
  with a script of its own, or whose script changes the tree, is not pruned;
  with a stylesheet the builder did not bundle, no custom property and no
  `@keyframes` is dropped. A test builds `@reactogenic/ui`'s behaviours and
  finds no tree-changing API in them.
- **Done when:** the corpus passes, with the design system's CSS and a page
  of each kind among it.
- **Done:** the corpus passes on the two modelled design systems of the
  research (`testdata/docs`, `testdata/components`), four adversarial sheets,
  and — added with RGP2-030 — the design system itself: `packages/ui`'s CSS
  (the files, not a copy: `testdata/ui/all.css`) against six pages the
  builder built, one of each kind (the golden output of `internal/build`'s
  fixture site), each given as the driver gives it — with its built script,
  and the elements of packaging named as the builder's. Of the bundle
  (7 532 B minified, 2 096 gzip) a page keeps 24–61% (gzip): 2 180 selectors
  dropped over the corpus, every one checked by cascadia.
  - Seen on the built pages, and fixed with the review of RGP2-030:
    `@position-try --rg-menu-edge` (of `dropdown-menu.css`) was on pages
    that have no menu, as any at-rule the pruner did not know. It goes when
    nothing kept names it, as a custom property does: −73 B on `/`,
    `/dialog/` and `/plain/`.
  - With the same review: a page with a `contenteditable` element is not
    pruned (what the user's editing creates is of no page as written); the
    corpus has such a page, and three pages as they are served — under a
    base, with the `<link>` and `<script>` of packaging, against a sheet
    that selects on them (`internal/build/testdata/served`).
  - With the integration of the two (RGP2-040): the page is given as
    served **and** the builder's own elements are named
    (`Options.Builder`), so that the `<script>` packaging writes is not "a
    script of the page's own" and its `<link>` not "a stylesheet the builder
    did not bundle" (builder.md, *CSS*, *The builder's own elements*).
    Unreconciled, every page that mounts a behaviour came back unpruned,
    with no test failing. The `served` fixture has a fourth page, with an
    author's script beside the builder's: not pruned.
  - With the review of the integrated tree: **the script is asked before
    the page** (builder.md, *The page's script*). A class or an id the page
    had was "yes" without the script being asked, so `:not(.collapsed)` was
    "no" and the rule gone when `classList.toggle("collapsed")` made it
    match; a text set over an element (`textContent`) removed what
    `:not(:has(b))` waited for; a document of the site in an `<iframe>`
    wrote to a page that had been pruned; and `classList.remove("x")` left
    its page unpruned, as "changes the tree". All four reproduced in
    Chromium 153 and WebKit 26.6 against `--no-specialize` (204, 51 and 51
    computed-style differences in Chromium after a click, or as loaded), and
    equal after the fix, in four packagings. The `served` fixture has two
    more pages for it (`/toggle/`, `/frame/`), and the corpus test looks at
    every page with a script a second time — with what the script names
    taken away.
    **Not seen**, and said so in builder.md: a name the script computes; a
    document that reaches the page by opening it or by framing it.
- **2026-10-05, the owner's rule on `<style>` and `<link>`** (RGP2-050,
  *The rulings, built*): a `<style>` of the page's own is pruned with the
  sheet (`Options.Styles`) — one cascade, in document order — and a
  `<link>` is a stylesheet by its `rel` (`markup.Link`). The guard for a
  script of the page's own stays, as defence in depth: the driver stops on
  such a page before it (shell-script).

### RGP2-021 — Page checks · S · done
`pagecheck`: id-duplicate, idref-not-found, command-target, link-not-found
(builder.md, *Checks on the page*), on `golang.org/x/net/html`. The content
of a `<template>` and of a `<noscript>` is not of the page, however the
document was parsed.
- **2026-10-05, the owner's rules** (RGP2-050, *The rulings, built*):
  shell-script — a script of the page's own is an error, found here; a
  link is checked whatever its `rel` (the table of `<link>`); a route is
  linked to by its pathname, a variant by its file.

### RGP2-022 — Behaviours: the page's JS · M · done
`behaviors`: the generated entry from a page's mounts, one `api.Build` per
page with the page's `Define`s, the metafile's bytes per module.
- Every free `RG_…` identifier of a mounted module's source, and of the
  modules it imports, is defined — found by a `Transform` of each file with
  the names defined as markers; the built script is asked again, as the
  backstop; the union over mounts; mount-not-found, mount-no-element,
  mount-kind, mount-flag.
- The read of a module (builder.md, *Behaviours*): a build of
  `import "<module>"` alone, before the page's — `Define` is fixed when a
  build starts. It gives the files, and mount-side-effect: what is left of a
  module nobody uses — for a module with flags, of two more builds, every
  flag on and every flag off.
- A module is the file it resolves to; the project directory's symbolic
  links are resolved before esbuild is given it.
- The metafile check: every input of a page's script is a file a mounted
  module reaches.
- `--no-specialize` (builder.md, *The control*): one script for the site.
- The fixture's scripts run in Node against a fake `document`
  (`go test`), and in Chrome with `RG_TEST_CHROME=<binary>`.
- **Done when:** a fixture with three behaviour modules gives, per page, a
  script that holds only what was mounted, and a flag off removes its code.
- **2026-10-05, the owner's rule on behaviours** (RGP2-050, *The rulings,
  built*): a mount's data is the second argument of its call in the entry,
  and the third member of its row in the control's table; mount-data. The
  control's table is keyed by a document's path (`/account/guest.html`).

### RGP2-025 — `@reactogenic/ui` · L · done
`packages/ui`: `Button`, `Dialog`, `DropdownMenu`, `SideMenu` in `.rtsx`,
their CSS, the three behaviours, the JSX augmentation (components.md).
- **2026-10-05, the owner's rule on behaviours** (RGP2-050, *The rulings,
  built*): `typeahead` is the mount's data — `menu-keys` takes `(root,
  own?)` — and no attribute: the menu's HTML is the same with and without.
  No rule of the package's CSS selected `data-typeahead`. The package is to
  be public under a branded name (decisions.md, 15).
- It type-checks under `reactogenic check`.
- HTML contract tests (vitest): each example of components.md, rendered
  through the phase 1 transpiler and `react-dom/server` with a stand-in for
  the build-time protocol, gives the HTML the spec shows.
- Behaviour tests in a browser (Playwright: Chromium and WebKit): the
  research's checks — a dialog opens from a button elsewhere, Esc and the
  scrim close it and focus returns; a menu is anchored and flips; arrow keys
  in an action menu; the drawer opens, closes on its backdrop without
  activating what is behind, and is a column above the breakpoint; Back does
  not restore an open overlay. The behaviours are bundled for them by a test
  helper that defines every `RG_…` flag — an undefined one is a
  `ReferenceError` in the browser, not a build error (esbuild is a
  devDependency); the builder's own bundling is RGP2-022's.
- Private for now: publishing it is a decision of its own.
- Depends on: 012.
- **Done:** `packages/ui`. On the integrated tree (RGP2-040): `reactogenic
  check` on the package is silent; vitest: 4 files, 70 tests; the browser
  suite (`pnpm --filter @reactogenic/ui test:browser`): 72 passed in
  Chromium 153, and 65 passed with 7 known in WebKit 26.6. What the suite
  calls *known* is components.md's *Known limits* — a button that is clicked
  is not focused there — and that Playwright's WebKit has no page cache.
  Not run: Firefox.

## M3 — The build

### RGP2-030 — `reactogenic build` · L · done
`go/internal/build`: routes, the pipeline of builder.md, packaging, the
report, the command in `go/cmd/reactogenic`.
- **2026-10-05, the owner's rules** (RGP2-050, *The rulings, built*):
  routes and variants (`routes.go`: `candidates`, then `variants` from the
  program's graph); the inline threshold of 4096 B (`pack.go`); a page's own
  `<style>` pruned and written back (`html.go`: `ownStyles`, `stylesOf`);
  `markup`, a package for what the stages have to agree on.
- The page's JS first (`behaviors.Build`), then its CSS: one esbuild build
  with every page as an entry (CSS in import order per entry; the JS
  outputs are discarded), then `cssprune.PruneWith` — the page as served,
  its script, and which of its elements are the builder's.
- `--out`: only a build's own output is emptied (builder.md, *The output
  directory*).
- `--inline`, `--base`, `--no-specialize`, `--report`; `public/` copied.
  `--inline auto` measures a blob with `compress/gzip`; the report has raw
  and gzip.
  `--base`: `pagecheck.Check` first, on the page as rendered; then the base
  is prefixed to the root-relative `href`s (builder.md, *Packaging*).
- Golden tests: a fixture site's whole `dist/`, both modes. Among the
  fixture's pages: one with nothing that opens — no `<script>` in it.
- The binary's size is printed by `build-binaries.sh`; the growth (esbuild +
  the engine: +7.10 MiB on 27.36 in the research, measured together) goes
  into decisions.md.
- Depends on: 011, 020, 021, 022.
- **Done:** `go/internal/build` (`build.Main` is the command, `build.Run`
  the build), `go/cmd/reactogenic`. The fixture site
  (`testdata/site`, seven pages on the real `@reactogenic/ui`) builds in
  ≈ 0.1 s; its whole output is golden in six modes (`testdata/golden`:
  `auto`, `always`, `never`, `control`, `base`, `base-control`), built twice
  and compared, and built once more by the compiled binary (`TestBinary`).
  - What each page ships, on the fixture (raw bytes, minified):

    | Page | Uses | CSS | JS | Control: CSS / JS |
    | --- | --- | --- | --- | --- |
    | `/plain/` | a `Button` that is a link | 1 033 | — | 7 531 / 1 644, on every page |
    | `/guide/`, `/guide/more/` | text | 1 074, one file for both | — | |
    | `/links/` | a menu of links | 2 306 | 252 | |
    | `/` | a `SideMenu` | 2 934 | 252, the same script | |
    | `/dialog/` | a `Dialog` | 2 803 | 563 | |
    | `/actions/` | an action menu with typeahead, a `Dialog` | 3 845 | 1 451 | |

    (As of the integration, RGP2-040: `overlays` also listens to `navigate`,
    and the components' CSS has the rules of components.md's review.)

  - Exported for it, and nothing else: `render.Source`, `render.Resolve`,
    `render.Load` (the one resolver, for the CSS build's plugin),
    `render.Message` (an esbuild message at the author's position),
    `pagecheck.RootRelative` (one rule for the links that are checked and
    the links that get the base).
  - The report has gzip and no brotli: no encoder in Go's standard library,
    and no dependency taken to count with. `bench/` measures it (RGP2-050).
  - In a browser, once, by hand (a script that is not in the repository:
    RGP2-050 owns the browser checks): the fixture built by the compiled
    binary in four modes — default, `--inline never`, `--base /docs/`, the
    control under `/docs/` — served over HTTP, in Chrome 154 (headless, by
    `packages/ui`'s Playwright). 32 of 32 checks: `/plain/` has no script;
    the dialog of `/dialog/` opens from its button, is modal, Esc closes it
    and focus returns — also with the engine's `command` taken away, so by
    the page's own script; the action menu of `/actions/` opens, arrow keys
    move the focus and wrap, Home, End, typeahead, an item opens the dialog;
    the menu of links has no arrow keys; a link leads to a page under the
    base, with its linked stylesheet applied. Not run: WebKit, Firefox.
  - Not verified: Windows (paths, the `--out` checks, the links).
  - **After the review** (ten findings, each with a test that fails without
    its fix):
    - `-p` is the project as for `check`: `check.Program` walks
      `references` and keeps the program of the project that lists the
      pages (builder.md, *The project*). Vite's template builds.
    - A page's CSS is pruned against the page **as it is served** — the
      base in its links, the `<style>` / `<link>` and `<script>` of
      packaging — in rounds, since how a sheet is delivered depends on the
      sheet (builder.md, *CSS*). Fixture `testdata/served`, golden in three
      modes; the JS is built before the CSS for it.
    - The new output is written beside the old one and takes its place when
      it is whole; `CheckOut` is asked again there, so `Run` is safe for any
      caller. The tests that must be refused build a copy of the fixture in
      a directory of their own; `/` and the repository's fixture are asked
      of `CheckOut` alone.
    - public-conflict also for a file where the build makes a directory.
    - `pages` and `public` may be symbolic links.
    - The report names a package's file by its package
      (`@reactogenic/ui/src/dialog.css`), and counts no byte of esbuild's
      source comments: the same report on any checkout — a copy of the
      fixture that reaches its packages through a link out of the project
      builds the golden output, report included.
    - `--base`: decoded for the keys of the control's table; a `%` that
      encodes nothing is a usage error.
    - Specified as it is: `index.rtsx` beside `index.tsx` is `check`'s
      ambiguous-module; gzip in the report is Go's DEFLATE, not `gzip -9`.
    - Exported for it: `check.Program`; `cssprune.Whole`, and `Why` and
      `PositionTries` in its `Stats` (`Why` was `Because` until the
      integration: one name for the reason, `styles.why` in the report).
  - In a browser again, on the output of the fixed binary (Chrome 154,
    headless, by `packages/ui`'s Playwright; the script is still not in the
    repository — RGP2-050): the fixture in seven modes, 164 of 164 checks
    (the list above, per mode); under `--base /caf%C3%A9/`, the default
    build 23 of 23 and the control 24 of 24 — it mounted nothing before;
    computed styles of every element, pruned against the control: the
    fixture's pages with their menus and dialogs open (13 comparisons), and
    `testdata/served` in five modes — `/`, `/guide/`, and `/edit/` before
    and after the user makes a word bold and adds a line (25 comparisons) —
    all equal. Not run: WebKit, Firefox.

### RGP2-040 — The docs site · M · done
`site/`: a private workspace package. `layout.rtsx` and four pages under
`pages/`, content from `docs/getting-started.md` and the phase 1 specs:

| Page | Components beyond the layout's |
| --- | --- |
| `/` | an *Install* dialog (`$Trigger`) |
| `/guide/` | — |
| `/syntax/` | an action menu that opens a cheat-sheet dialog |
| `/reference/cli/` | — |

The layout carries the `SideMenu` (nested groups, the current page marked)
and a `DropdownMenu` of links in the header. So the pages differ in what they
ship: `/guide/` and `/reference/cli/` need `overlays` alone, `/` adds
`invokers`, `/syntax/` adds `menu-keys`.
- **The authoring rule:** no `<script>`, no hand-written JS, no per-page
  list of styles or behaviours in the site's source. A script in a page
  would also turn its CSS pruning off (builder.md, *CSS*).
- CI builds it (`reactogenic build`) and fails on any diagnostic: RGP2-060.
- Depends on: 025, 030.
- **Done:** `site/` — `layout.rtsx`, `code.rtsx` (a sample: plain
  `<pre><code>`), `site.css`, `public/favicon.svg`, and the four pages, whose
  long bodies are segments (`pages/syntax/slots.rtsx` mounted by
  `<section #slots />`, ten in all) — built by `reactogenic build`, on the
  tree that has the driver, the reconciled specs and the reviewed components
  together. It was written before the driver existed and against
  `@reactogenic/ui` before its types were narrowed: it type-checks and
  builds on the merged tree as it was written.
  - `reactogenic build --report` in `site/`: 4 pages, exit 0, nothing
    reported — also with `--inline always`, `--inline never`,
    `--no-specialize` and `--base /reactogenic/`. `go test` builds it in six
    modes (`internal/build`, `TestDocsSite`): four pages, no diagnostic, the
    mounts of the table above, one `<script>` per page — the builder's — and
    every page pruned.
  - What each page ships, raw / gzip bytes as the report prints them
    (`--inline auto`; the HTML is without what packaging adds):

    | Page | HTML | CSS | JS | Control: CSS / JS |
    | --- | --- | --- | --- | --- |
    | `/` | 9 689 / 3 454 | 9 041 / 2 423 | 563 / 328 | 9 352 / 2 477 and 1 645 / 824, on every page |
    | `/guide/` | 10 548 / 3 681 | 7 224 / 2 093 | 252 / 179 | |
    | `/reference/cli/` | 21 503 / 6 932 | 7 179 / 2 084 | 252 / 179, the same script | |
    | `/syntax/` | 36 287 / 9 992 | 8 914 / 2 398 | 1 446 / 704 | |

    Under `auto` nothing is a file: the one blob two pages share, the 252 B
    script, is 179 B gzipped — under a request. So the default build is
    `--inline always`'s, to the byte but for the report's `inline`. With
    `--inline never` a page is its HTML plus 112 B of `<link>` and
    `<script>`; under `--base /reactogenic/` the HTML grows by the base in
    its links (240–348 B) and the CSS and JS are the same bytes.
  - In a browser, on the built output: `site/test/browser.mjs` (`pnpm
    --filter @reactogenic/site test:browser`) builds the site as it ships
    and with `--no-specialize` — the same HTML, no pruning — serves both, and
    runs Chromium 153 (the full browser) and WebKit 26.6 at 1200 and 400 px:
    326 checks pass, 2 are known, none fails. Run in four modes — default,
    `INLINE=always`, `INLINE=never`, `BASE=/reactogenic/` — with the same
    result in each.
    - Every page: no sideways scroll; one script and no React; no page
      error, console error or failed request; the side menu marks the page
      (`aria-current="page"` on its link, styled) and opens its group alone.
    - The Install dialog opens modal with its panel in the viewport, and
      closes by Esc, its close button, its scrim and its *Close* action,
      focus back on *Install* each time; with the engine's `command` taken
      away, the page's own script opens and closes it.
    - The action menu of `/syntax/` opens anchored below its trigger with
      focus on its first item; ArrowDown and ArrowUp move and wrap, Home,
      End, typeahead (`s`, `g`, `c`); its item opens the cheat sheet and
      closes the menu, and Esc returns focus to the menu's trigger.
    - The links menu opens anchored below its trigger, end-aligned, in the
      viewport, at both widths. The drawer at 400 px opens from its toggle,
      closes on its scrim without activating what is behind, and on its
      close button; at 1200 px the same `<nav>` is a sticky column beside
      the content, with no toggle.
    - **Pruning changes nothing that is seen**: the computed style of every
      element of every page — and of its `::before`, `::after`, `::marker`
      and `::backdrop` — is the same in the two builds: as loaded, with the
      links menu open, the drawer open, a group of it toggled, keyboard
      focus on an element, each dialog open, the action menu open: 80
      comparisons per mode, 1 060 to 4 365 computed styles each, no difference, custom
      properties included. The comparison finds a rule that is broken on
      purpose (`[aria-current]` and `:popover-open` misspelt in one page's
      sheet: 91 differences).
    - *Known*, 2 of the checks: in WebKit a dialog opened **by a click**
      leaves focus on `<body>` when it closes (components.md, *Known
      limits*); opened from the keyboard, focus returns.
  - Screenshots of those runs (`SHOTS=<dir>`), read: the top of every page
    at 1200 and 400 px in both engines, and the Install dialog, the cheat
    sheet, the action menu, the links menu and the open drawer in both; the
    `build` section of `/reference/cli/` at both widths. Nothing wrong in
    them. The full-page captures were not read through.
  - The side menu's groups each start with the page's own link: a link to a
    section (`/guide/#install`) is never the current page (components.md,
    *SideMenu*). The page check does not look at the fragment of a link to
    another page (builder.md, *Checks on the page*, OPEN): that every such
    link names an element of its page was checked on the built output by a
    throwaway script, once — 63 links with a fragment, none broken.
  - Found on the way: the example added to components.md for those groups
    was a fragment the HTML contract test cannot render (orphan-slot) — it
    is a whole `SideMenu` now, and the eighth contract example.
  - **Not done:** CI (RGP2-060). `pnpm -r typecheck` does not reach the
    site — it has `check`, not `typecheck`: the js job would need the
    binary.
  - **Not verified:** Firefox (it does not start in the sandbox the suite
    was run in); Safari proper; a touch device; the floor's own versions;
    dark mode and the widths 800, 640 and 320 px on the *built* site — they
    were looked at on the stand-in build only, before the driver existed
    (Chromium 153 light and dark, WebKit 26.6 light: no page scrolls
    sideways, both dialogs stay in the viewport); Windows.
  - For RGP2-050, as measured here and not judged: against the control the
    per-page CSS is 3%, 23%, 23% and 5% smaller raw (2%, 16%, 16%, 3%
    gzipped) — the site's own sheet, which every page uses nearly whole, is
    most of it — and the JS 66%, 85%, 85% and 12% (`/syntax/` mounts all
    three behaviours: the control has nothing more than its table).
  - Playwright's headless shell paints the page beside the open drawer
    wrongly (scrolled, without the backdrop); the full Chromium
    (`channel: "chromium"`, as both browser suites launch it) and WebKit
    paint it right.

## M4 — The bet

### RGP2-050 — Measure · M · done
`bench/`: the measuring scripts of the research (bytes raw / gzip / brotli
per page, requests, JS to parse, a warm 4-page session), run on three builds
of the site: the default; `--inline always`; and the control,
`--no-specialize` — its pathname table's bytes reported apart from its
behaviours'. Brotli is measured here (Node's zlib): the builder's own report
has raw and gzip. Browser checks of the built site (Playwright): the
behaviour list of RGP2-025 on real output, and the computed-style comparison
of pruned against unpruned CSS on every page. With it, the validity probe of
RGP2-020 in Chromium, Firefox and WebKit: for every name of
`knownPseudoClass` and `knownPseudoElement` (`cssprune/selector.go`) and the
forms `validNth` accepts, insert `SEL, p {}` and count `cssRules` — a name a
browser of the floor rejects must leave the table. Checked so far in
Chromium 153 and WebKit 26.6 only; never in Firefox, nor at the floor's
versions.

The bet is **confirmed** if all hold, **refuted** if any of the marked ones
fails:

| | Threshold | |
| --- | --- | --- |
| T1 runtime | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report — a behaviour mounted on that page, a file it imports, or the generated entry (`<entry>`: the mount calls) — and there is no `<runtime>` row | refutes |
| T2 JS per page | ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page | refutes above 5 KB brotli — a micro-runtime, not compilation |
| T3 against React | ≥ 100× below the best React build of an equivalent site (Astro + React islands + Radix: 317 KB raw, measured in research/baselines.md) | |
| T4 precision | deleting the *Install* dialog from `/` — it has its own `$Trigger`: nothing else refers to it — removes its markup, the CSS rules only it matched and the `invokers` behaviour from that page, and nothing else of it; and every other page's HTML as rendered, CSS and script are the same bytes. Compared under `--inline always`: sharing couples how pages are delivered, not what they are (builder.md, *Packaging*) | refutes |
| T5 awareness | against the control, **in brotli bytes** — what a page transfers: `brotli -q 11`, each blob on its own, as `bench/measure.mjs` does; raw and gzip are reported beside it — per-page CSS ≥ 20% smaller on at least two pages (of a catalog: on at least half of its pages), JS ≥ 30% smaller on every page that ships one. The unit is the owner's ruling (2026-10-05; decisions.md, L): T5 gave none until it was measured, and was first read raw | |
| T6 authoring | the authoring rule of RGP2-040: no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | refutes |
| T7 behaviour | the browser checks pass on the built site | refutes |
| T8 requests | ≤ 3 per page, cold | |

What the table cannot measure on this site, and where it is tested instead:

| | |
| --- | --- |
| "a page with nothing that opens ships no script" | every page of the site has the layout's `SideMenu` and `DropdownMenu`, so each mounts `overlays`. RGP2-030's golden fixture has such a page |
| T4 as first written — deleting the dialog of `/syntax/` | its menu item commands it (`commandfor`): deleting the dialog alone is idref-not-found, and deleting the item with it also takes `menu-keys` and the menu's `role` away |
| T5 where a page uses a few components of many | the docs site has three components in one layout, and one page that mounts everything. The owner's ruling (decisions.md, L): measured again on a catalog of about twenty — `bench/catalog` and `bench/catalog-site`, **measurement fixtures**, not the design system and not a product (their READMEs; bet.md, *The catalog*). T5's "at least two pages" reads "at least half of the pages" there; T2's 1.5 KB — a budget for the docs site's three behaviours — is not carried over: its bound (5 KB brotli) is, and the heaviest page is reported; T3 has no React build to stand against |
| T2 as a multiple of the hand-written floor | none is claimed. The research's floors are of a site with theme, copy and search — 76% of the first floor's JS (research/baselines.md, *Verification*) — which this site does not have (decisions.md, 14); at behaviour parity that floor is 1,584 B raw, 571 B brotli, mean per page. T2 is a budget for this site's three behaviours |

The result, with the tables, is written to `specs/phase02/bet.md`.
- Depends on: 040.
- **Done:** `bench/site.mjs`, `bench/delta.mjs`, `bench/verify.mjs`
  (`bench/lib.mjs` is what they share), their reports in `bench/results/`,
  and [bet.md](bet.md). **The bet holds for a page loaded cold, and does
  not hold over a visit** — bet.md's verdict, in two parts. Cold: every
  threshold that refutes holds; T5 fails on the docs site and holds on the
  catalog (*T5's unit, and the catalog*, below). A visit: the build without
  awareness transfers less from the second page of the docs site and from
  the third of the catalog (decisions.md, K: open). By the rule above —
  all eight — that reads "confirmed" if T5 is taken where ruling L sent
  it, and "undecided" if T5 must also hold on the docs site: the owner has
  not chosen the word.

  The docs site, as measured last — 2026-10-05, on the tree with every
  ruling built (`bench/results/site.md`, `delta.md`):

  | | Measured | |
  | --- | --- | --- |
  | T1 | the report's rows add up to each page's script (563, 252, 1,443, 252 B); no `<runtime>` row; the only statements that run are the mount calls | pass |
  | T2 | `/syntax/`: 1,443 B raw, 588 B brotli | pass |
  | T3 | 219× raw, 152× brotli, at the worst pairing — against another site of the same shape | pass |
  | T4 | the *Install* dialog deleted from `/`: 1,070 B of markup in one span, 14 selectors that name `.rg-dialog` (1,752 B), `invokers`; the other pages the same bytes | pass |
  | T5 | in brotli: CSS 2.6%, 16.1%, 3.6%, 16.5% — no page at 20% (raw: 23.5% and 24.0% on two); JS 17.2% on `/syntax/`, which mounts everything the site has | **fail** |
  | T6 | 21 source files; nothing found | pass |
  | T7 | 702 checks pass, 8 known, none fails: Chromium 153.0.8010.12 (355), WebKit 26.6 (347 and the 8 known); 688 of the site, 14 of the pruner's fixture; 256 comparisons of computed styles, all equal | pass |
  | T8 | 2 requests per page: the document and the favicon | pass |

  The notes below are in the order things were measured: the numbers in
  each are of its time.

  - Four builds, not three: `--inline never` too — the default's blobs as
    files. Under `auto` the default build *is* `--inline always`'s on this
    site.
  - The control's script is 1,405 B of behaviours and 240 B of its own:
    the table is reported apart, and T5 is given against both.
  - Found by measuring, and the owner's (decisions.md, K, L): over a
    four-page visit the control transfers 19.8% less than the default
    build — per-page sheets share nothing; T5's JS half cannot hold on a
    page that mounts everything.
  - The validity probe: each of the 119 selectors `cssprune` takes for
    known — its three tables of names, and 16 forms of each `:nth-*()` —
    parses in Chromium 153 and WebKit 26.6. No name leaves the table.
  - The computed-style comparison: 248 of the site, all equal — at rest,
    dark, reduced motion, hover, keyboard focus, each overlay open — and it
    sees a rule taken out of one page.
  - Corrected on the way: builder.md's "gzip: 0–5 B above `gzip -9`" (from
    76 B below to 3 B above, on the docs site) — and, with the final round,
    the comment in `bytes.go` that still said "a few bytes more".
  - **Measured again on `rgp2-final`**, after the review's fixes to the
    pruner (RGP2-020): `site.mjs` and `delta.mjs` write the same bytes as
    before — the site's behaviours write nothing, so nothing of it was
    wrongly dropped, and nothing is kept now that went before. No verdict
    changes. `verify.mjs` gained the comparison the site cannot make: the
    `served` fixture's `/toggle/`, as loaded and after its behaviour took a
    class, an id and an element away, and its `/frame/` — 6 comparisons
    and 4 checks that they can fail, all passing; with the pruner as it
    was, the frame's fail.
  - Found by the same measurement, and the owner's (decisions.md, M): 301 B
    of each page's sheet are rules for a state no element of the page can
    reach. (Ruled since — option 2 — and built: 72 B of it went, the rule
    on an attribute; 229 B stay, the rules on `:disabled`.)
  - **Not run:** Firefox — the checks and the probe; Safari proper; the
    floor's versions (Chrome 135, Firefox 147, Safari 26.2); a touch
    device; Windows; `--base` (the site's own suite builds under one);
    `--no-specialize --inline always`; time (CPU, parse, paint).
  - **Not built**, so not measured: the React baseline and a hand-written
    floor *of this site* — T3 is against the research's site, and no ratio
    to a floor is claimed; a catalog of ~20 components (*Later*) — built
    since, as a fixture: *T5's unit, and the catalog*, below.
- **The rulings, built (2026-10-05).** The owner ruled on the decisions
  (decisions.md, *Ruled by the owner*); five rulings changed the builder,
  and the bet was measured again on the result. What each touched — the
  specs have the rules; the tasks above point here:

  | Ruling | Built | Tasks |
  | --- | --- | --- |
  | routes (12) | a directory under `--pages` is a route; an `.rtsx` file of it that no module of the program imports is a variant, built to `<pathname><name>.html`; `index.tsx` makes no page. A document is named by route and variant in the render bundle, the record, the checks, the report (`pathname`, `variant`, `path`) and the control's table. `server.ts` is not read | 010, 011, 021, 022, 030 |
  | behaviours (9) | `mount(module, id?, flags?, data?)`: the data is JSON, recorded with the mount, written into the generated entry as that mount's second argument; mount-data. `DropdownMenu` passes `{ typeahead: true }` and emits no `data-typeahead` | 011, 012, 022, 025 |
  | packaging (10) | `--inline auto`: a file when two or more documents share a blob of 4096 B or more as written; otherwise inlined | 030 |
  | a page's own script (19) | shell-script, an error of the page checks: a `<script>` that runs, an `on…` attribute, a `javascript:` URL | 021, 020 |
  | `<style>` and `<link>` (20) | a `<style>` of the page's own is pruned with the page's sheet and written back in place; a `<link>` is classified by `rel` in one table (`markup`) | 020, 021, 030 |

  - **Tests.** Each rule has tests that fail on the tree before it. The
    fixture `site` gained `/account/` (two variants, a segment, a module, a
    `server.ts`) and `/gate/` (a variant and no `index`); `bad` gained a
    stray segment (page-not-document), two `--pages` roots without a page,
    and a page with scripts of its own; `served` lost `/own/` and gained
    `/styled/`. The goldens were rewritten, and their diff read: new pages;
    `variant` and `path` in every report; `/actions/` without
    `data-typeahead` and with its data in the entry; the sheet two guide
    pages share and the control's script inlined (under 4096 B); `/own/`
    gone, `/styled/` there. The docs site builds its four pages in all six
    modes, and no sheet of it changed by a byte.
  - **Measured again** (bet.md): no threshold's outcome changes — T5
    fails. `/syntax/`'s script is 1,443 B (was 1,446), its HTML 18 B
    smaller; the control's script is 1,667 B (was 1,645) and, under the new
    threshold, inlined in every page — so over a four-page visit the
    control now transfers 12.9% less than the default build, not 19.8%.
    T7: 702 checks pass, 8 known, none fails (Chromium 153, WebKit 26.6):
    the 688 of the site as before, and 14 of the fixture — its `/styled/`
    is compared too, pruned `<style>` elements against the control's whole
    ones.
  - **Decided where the rulings were silent** — each in builder.md:
    an import of types alone counts as an import (the program's graph, as
    it is); a directory without a variant is no route for the link check;
    a document is named by its path on a static host
    (`/account/guest.html`) wherever one name is needed; data that is not
    JSON is reported where `mount()` is called, two mounts that disagree at
    the page; a key whose value is `undefined` is left out; the control
    prunes no `<style>`; a `<style>` that cannot be paired with the page's
    text, or does not read, is left as it is; any attribute that starts
    with `on` is a handler; `stylesheet` decides a `rel` of several tokens.
  - **Found on the way.** `bench/site.mjs` took the word `javascript:` in a
    page's text for a `javascript:` URL (T1's check): it reads attributes
    now. `bench/measure.mjs` once counted three requests for `/` in the
    `--inline never` build — the favicon's was missed; the run was repeated
    and is the same bytes twice. A paused animation in the fixture hung
    `verify.mjs`, which waits for animations to end: the fixture's runs.
  - **Not done.** A warning for an `index.rtsx` that is no variant because
    something imports it (builder.md, *Routes*, OPEN). Two outputs that
    collide — `a/b.rtsx` and a route `a/b.html/` — are not reported: the
    write fails, and the old output stays. The render package's fixtures
    still have `index.tsx` pages: render is given its routes.
  - **Not verified.** Firefox, Safari proper and the floor's versions, as
    before. `verify.mjs --inline never` was run, and gives the same 702, 8 and 0;
    `--base` was not, here. The binary's size was
    not measured again. An SVG `<style>` whose text holds markup: such a
    page's `<style>` elements are left as they are, by reasoning and one
    unit test of the pairing's refusal — none in a browser.

- **T5's unit, and the catalog (2026-10-05).** The owner's ruling on L
  (decisions.md): "fix the ambiguity around units and rerun on a catalog of
  ~20 components".
  - **The unit.** T5 is decided in brotli bytes (the table above);
    `bench/site.mjs` decides it so and prints gzip and raw beside. On the
    docs site it fails in brotli on both halves — CSS 2.6% to 16.5%, no
    page at 20%; JS 17.2% on `/syntax/` — where raw its CSS half had held.
  - **Built**, as measurement fixtures (their READMEs say so: neither is
    the design system): `bench/catalog` (`@reactogenic/bench-catalog`) —
    the four components of `@reactogenic/ui` and sixteen more, each with
    its CSS in components.md's convention, four with a behaviour in
    builder.md's authoring style, three flags of its own — and
    `bench/catalog-site`, ten pages. Two private workspace packages
    (`pnpm-workspace.yaml`, the lockfile's two importers). The pages, and
    which components each uses, were written down before the first build
    was measured (`bench/catalog-site/README.md`).
  - **Run:** `bench/catalog.mjs` → `bench/results/catalog.md` —
    `reactogenic check` and three builds (default, `--no-specialize`,
    `--inline never`) with no diagnostic; every page cold in headless
    Chrome and a warm session of ten; T5 page by page; T1, T2, T6, T8; one
    component deleted from one page, twice. The same bytes on two runs.
    `bench/catalog-verify.mjs` → `bench/results/catalog-verify.md` —
    Chromium and WebKit, 1200 and 400 px.

    | | On the catalog | |
    | --- | --- | --- |
    | T1 | the rows add up to each of the ten scripts (252 B to 2,448 B); no `<runtime>` row | pass |
    | T2 | its 1.5 KB is the docs site's budget and is not carried over; its bound is: the heaviest page, `/settings/`, is 2,448 B raw, 876 B brotli | pass on the bound |
    | T4's question | an accordion deleted from `/pricing/` (1,115 B of markup, 12 selectors), a toast from `/settings/` (387 B, 8 selectors, its behaviour: 2,448 → 1,932 B); no other page changed | pass |
    | T5 | brotli: CSS 41.7% to 73.0% smaller on ten pages of ten, JS 41.9% to 91.6% | **pass** |
    | T6 | 17 source files; nothing found | pass |
    | T7, as a sanity pass | 1,348 checks pass, none known, none fails (674 in each of Chromium 153.0.8010.12 and WebKit 26.6): 488 comparisons of computed styles, all equal, and 488 times no dropped selector matched an element | pass |
    | T8 | 2 requests a page, 3 on the one with a picture | pass |

  - **The session.** Cold, a page of the default build is 50.7% lighter
    than the control's (brotli, mean of ten). Over the ten pages the
    control transfers 44.8% less — 24,034 B against 43,505 — and is ahead
    from the third page. On the docs site: 8.0% lighter cold, 12.8% less
    for the control over four pages, ahead from the second. Per-page
    pruned sheets share nothing (decisions.md, K: open, not designed).
  - **The verdict** is the one at the head of *Done*: it holds for a page
    loaded cold, and does not hold over a visit; which word of the rule
    that is, the owner has not chosen.
  - **Decided where the plan was silent.** T5's "at least two pages" reads
    "at least half of the pages" on ten. Its JS half is judged on the
    scripts as shipped; against the control's behaviours alone — builder.md
    has the table reported apart — it is given beside, and holds too (32.6%
    at the least). T2's budget is not judged on a page that mounts five
    behaviours; per behaviour mounted the figure is printed for the
    heaviest page and for the worst (`/docs/`: 583 B, over the docs site's
    500), and not judged either. T3 is not asked: there is no React build
    of the fixture. T4 is the docs site's by name; its question is asked of
    two other components. "T7" there is the style comparison and a check
    per behaviour, not a suite.
  - **Measured twice.** First with a binary built before decision M's
    change to the pruner was in the tree; then — the numbers above, and
    bet.md's — on the tree that has M, D (`slotKeys`) and G (`closeLabel`)
    built, both sites with one binary. Between the two: every sheet lost
    the menu's `[aria-current]` rule, 72 B (the catalog's `/404/`, which
    has no current page, 230 B); `/syntax/` gained a sentence; no script
    moved. The catalog's five containers of keyed slots iterate with
    `slotKeys` now.
  - **Not done.** No contract tests and no spec for the sixteen.
  - **Not verified.** Firefox; Safari proper; the floor's versions; a
    touch device; the catalog under `--base`, `--inline always`, and its
    browser pass under `--inline never`; that the fixture's pages stand for
    a real site's — their HTML is 2.5–9.4 KB, a quarter of the docs
    site's, which flatters every share "of the page" and no part of T5.

### RGP2-060 — CI and docs · S · done
A `site` job (build + the byte report as an artifact); getting-started gains
*Build*; CHANGELOG; CLAUDE.md.
- **Done:**
  - CI, `.github/workflows/ci.yml`, job `site`: the binary of the commit,
    then `reactogenic build` in `site/`. **Any diagnostic fails it**: the
    build prints its diagnostics and one last line (`4 pages written to
    dist`), so a second line is one — the exit status would let a warning
    through (shell-console, css-warning: 0). Then `--report`; the artifact
    `site-report` is what it printed (`report.txt`) and `_rg/report.json`.
    Not a required check of the `main` ruleset, as `vscode` is not.
  - The `go` job already had Node and `pnpm install` before `go test`; it
    gains `-timeout 30m`.
  - docs/getting-started.md, *Build*: the `pages/` convention, the layout as
    a component, `@reactogenic/ui`, the output, `--report`, the browser
    floor, what phase 2 lacks. Every output shown there is a run of the
    binary on that example. The `check` sample said `error TS2322` for a
    prop of a slot element; the binary says slot-type, and so does the doc.
  - CHANGELOG.md, *Unreleased*; README.md; CLAUDE.md (*Current phase*,
    *Open with the owner*, *Deliverables*); later/layout.md: notes and OPENs
    where phase 2 reads it otherwise.
  - The extension's grammar test (`packages/vscode/test/repo-rtsx.test.mjs`:
    no `invalid.*` token in any `.rtsx` of the repository) now also walks
    `site/` and `go/internal`: 75 more files, none with a problem.
- **Not verified:**
  - GitHub Actions did not run: nothing was pushed. The `site` job's `run`
    steps were executed as its `shell: bash` runs them, with the binary
    built here: they pass, and a `console.log` in a page (a warning) and a
    `Date.now()` (an error) each fail the build step.
  - **The builder's packages have never run under `-race`**, which the `go`
    job uses: not here (the instruction of this phase: no `-race`, for the
    cache it fills), not in CI. `modernc.org/quickjs` under the race
    detector is the unknown — its time, which the timeout allows for, and
    whether its transpiled C passes `checkptr`.
  - With CI's Node: `go test` of `internal/build/...` passes with Node
    22.21.1 on the PATH, as with 25.2.1. On macOS; CI is Linux.
- The site's own pages (`site/pages/guide/`) were written from
  getting-started before it had *Build*; they have `build` on
  `/reference/cli/` only.

## Later, noted here so it is not lost

- ~~Keyed slots and integer-like keys~~: fixed (decisions.md, D — an
  entry's name is its key encoded, and a container iterates `slotKeys`:
  phase01/syntax.md, *Keyed slots*). ~~State that nothing can reach stays
  "maybe"~~: ruled and built (decisions.md, M — an attribute only a script
  writes is decided on the page unless the page's script names it:
  builder.md, *CSS*); the rules on `:disabled` and the other
  pseudo-classes stay, 229 B a page of the docs site.
- The false `segment-children` warning on `<Dialog #install><$Title>…`; the
  opaque TS2559 for a keyed slot element without `key`
  (research/components.md, section 7).
- Loop-produced slot items (above).
- ~~A catalog of ~20 components where a page uses 3–5: the scale test of
  per-page precision.~~ Built as a measurement fixture (RGP2-050, *T5's
  unit, and the catalog*). A catalog that is the design system's own — a
  spec, contract tests, a browser suite per component — is still later.
- Firefox, Safari 26.x (the floor), real key and pointer input in Safari,
  and every touch device: not run. The platform research's verifier ran
  Safari 27.0.1, script-driven (research/platform.md, *Verification*).
- A check of what a behaviour writes to the page (builder.md, *Not in phase
  2*); hashes of inlined blobs for a `Content-Security-Policy`.
- `rel=preload` could be classified further by `as` (a preloaded style,
  script, font) — the owner: not now (builder.md, *Checks on the page*, the
  table of `<link>`).
- A route's `server.ts`, and the server it is middleware of (builder.md,
  *Routes*, *Variants*).
- **Before the next release**: the notices of what the binary now links.
  `scripts/build-binaries.sh` puts tsgo's licence and notice into each
  platform package, and nothing else; since RGP2-030 the binary also holds
  esbuild, `modernc.org/quickjs` (QuickJS, with modernc's libc) and
  `golang.org/x/net`. The extension bundles the same binary
  (`packages/vscode`, `ThirdPartyNotices.txt`). Found with RGP2-060; nobody
  has read the licences for what they ask.
- Phase 1's leftover: the server's intermittent crash while pushing tsconfig
  diagnostics (phase01/plan.md, RGP1-113).
