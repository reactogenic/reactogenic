# Phase 2 plan: the builder

Specs: [research.md](research.md) (why), [builder.md](builder.md) (the
pipeline), [components.md](components.md) (the three components),
[decisions.md](decisions.md). Tasks are `RGP2-xxx`; sizes S / M / L as in
phase 1.

**Outcome.** `reactogenic build` compiles a documentation site for
Reactogenic — four pages using `SideMenu`, `Dialog`, `DropdownMenu` — into
plain HTML + CSS + minimal JS per page, with no React in the output; and a
measurement that confirms or refutes the bet: *if the compiler is aware of
components it can ridiculously optimise the output build*.

Out of scope: a dev server, view transitions, islands.

```
001 ─▶ 002 ─┬─▶ 010 ─▶ 011 ─┐
            ├─▶ 012 ────────┤
            ├─▶ 020 ────────┼─▶ 030 ─▶ 040 ─▶ 050 ─▶ 060
            ├─▶ 021 ─ 022 ──┤
            └─▶ 025 ────────┘
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
	Components map[string]int // function components rendered, by name
}
type Options struct {
	Dir     string        // the project's directory: react and react-dom are resolved from it; "": the program's
	Timeout time.Duration // of one page; 0: 30 s
}
func Render(program *rtsx.Program, routes []Route, opts Options) ([]Page, []report.Report)
// Pages: those that rendered, in the order of routes. Reports: errors (the caller
// stops on one) and warnings (shell-console). Mount.Flags holds what the use site
// passed, `false` included; nil when it passed none.

// cssprune — RGP2-020
func Prune(css string, doc *html.Node) (out string, stats Stats, err error)

// pagecheck — RGP2-021
func Check(page render.Page, doc *html.Node, routes []render.Route, files map[string]bool) []report.Report
// files: the output's other files, by their path from its root ("/favicon.svg")
// --base is not its business: the page is checked as rendered, before packaging prefixes its links

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
  `d1`, `d2`, `m1`. No prefix (`""`, `undefined`): `r`.
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
`modernc.org/quickjs`, one runtime per build: the sandbox prelude, the
bundle, then `render(pathname)` per route.
- The prelude makes `Date.now`, argument-less `new Date()`, `Math.random`,
  `crypto.getRandomValues`, `performance.now` throw shell-nondeterministic;
  `console` is collected and printed as warnings.
- The prelude pins the time zone to UTC and makes what needs `Intl` throw
  (builder.md, *The engine*).
- An exception becomes a diagnostic at the source position: the engine's
  stack → the bundle's source map → the program file's position → the span
  map for `.rtsx` (`emit.Map`) — `report`'s output format, with the component
  stack as related lines.
- Differential test, in CI: every fixture page rendered by the engine equals
  the same bundle rendered by Node with `react-dom/server` — including the
  36 attribute cases of research/evaluation.md that a hand-written serializer
  got wrong.
- The six release targets build with `CGO_ENABLED=0`
  (`scripts/build-binaries.sh`).
- **Done when:** fixture pages with slots, `Each`, `Match`, segments and the
  three intrinsics render; each shell-* error is reported at its `.rtsx`
  line and column; the differential test passes.
- Depends on: 010.
- **Done:** `engine.go`, `position.go`, `js/`. Measured, darwin-arm64: the
  four-page fixture is checked, bundled and rendered in ≈ 0.1 s.
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
    (`scripts/build-binaries.sh`): darwin-arm64 27.3 → 34.5 MB, darwin-x64
    28.6 → 36.5, linux-arm64 26.4 → 33.7, linux-x64 27.9 → 35.9, win32-arm64
    26.5 → 33.7, win32-x64 28.2 → 36.3 (+26–29%).
  - Not verified: Windows paths (the six targets build; the tests ran on
    macOS); React other than 19.3 — its static renderer reads no clock, so
    the sandbox has no exemption for React's own code, and the builder
    depends on its text in one place (above); whether `Math`'s
    transcendental functions give the same last digit on amd64 as on arm64
    (Go may fuse a multiply-add on arm64; the tests ran on arm64 only).

### RGP2-012 — `@reactogenic/core`: `pathname`, `useShellId`, `mount` · S
In `packages/core`, with the protocol above and the React fallbacks; unit
tests for both sides.

## M2 — What the builder decides

### RGP2-020 — CSS pruning · L
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
  removed; and pruning is idempotent.
- **Done when:** the corpus passes, with the design system's CSS and a page
  of each kind among it.
- **State:** built and passing on the two modelled design systems of the
  research (`testdata/docs`, `testdata/components`) and four adversarial
  sheets. Left for RGP2-025: add `packages/ui`'s CSS and one built page of
  each kind to `corpus()` in `corpus_test.go` — the task is done then.

### RGP2-021 — Page checks · S · done
`pagecheck`: id-duplicate, idref-not-found, command-target, link-not-found
(builder.md, *Checks on the page*), on `golang.org/x/net/html`.

### RGP2-022 — Behaviours: the page's JS · M · done
`behaviors`: the generated entry from a page's mounts, one `api.Build` per
page with the page's `Define`s, the metafile's bytes per module.
- Every free `RG_…` identifier of a mounted module's source, and of the
  modules it imports, is defined — found by a `Transform` of each file with
  the names defined as markers; the built script is asked again, as the
  backstop; the union over mounts; mount-not-found, mount-no-element,
  mount-flag.
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

### RGP2-025 — `@reactogenic/ui` · L
`packages/ui`: `Button`, `Dialog`, `DropdownMenu`, `SideMenu` in `.rtsx`,
their CSS, the three behaviours, the JSX augmentation (components.md).
- It type-checks under `reactogenic check`.
- HTML contract tests (vitest): each example of components.md, rendered
  through the phase 1 transpiler and `react-dom/server` with a stand-in for
  the build-time protocol, gives the HTML the spec shows.
- Behaviour tests in a browser (Playwright: Chromium and WebKit): the
  research's checks — a dialog opens from a button elsewhere, Esc and the
  scrim close it and focus returns; a menu is anchored and flips; arrow keys
  in an action menu; the drawer opens, closes on its backdrop without
  activating what is behind, and is a column above the breakpoint; Back does
  not restore an open overlay.
- Private for now: publishing it is a decision of its own.
- Depends on: 012.

## M3 — The build

### RGP2-030 — `reactogenic build` · L
`go/internal/build`: routes, the pipeline of builder.md, packaging, the
report, the command in `go/cmd/reactogenic`.
- The page's CSS: one esbuild build with every page as an entry (CSS in
  import order per entry; the JS outputs are discarded), then `cssprune`.
- `--inline`, `--base`, `--no-specialize`, `--report`; `public/` copied.
  `--base`: `pagecheck.Check` first, on the page as rendered; then the base
  is prefixed to the root-relative `href`s (builder.md, *Packaging*).
- Golden tests: a fixture site's whole `dist/`, both modes.
- The binary's size is printed by `build-binaries.sh`; the growth (esbuild +
  the engine, measured ≈ +7.3 MB on 27.4) goes into decisions.md.
- Depends on: 011, 020, 021, 022.

### RGP2-040 — The docs site · M
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
- No `<script>`, no hand-written JS, no per-page list of styles or
  behaviours in the site's source.
- CI builds it (`reactogenic build`) and fails on any diagnostic.
- Depends on: 025, 030.

## M4 — The bet

### RGP2-050 — Measure · M
`bench/`: the measuring scripts of the research (bytes raw / gzip / brotli
per page, requests, JS to parse, a warm 4-page session), run on three builds
of the site: the default; `--inline always`; and the control,
`--no-specialize`. Browser checks of the built site (Playwright): the
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
| T1 runtime | 0 bytes of React or of any generic runtime: every JS byte belongs to a behaviour mounted on that page | refutes |
| T2 JS per page | ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page; a page with nothing that opens ships no script | refutes above 5 KB brotli — a micro-runtime, not compilation |
| T3 against React | ≥ 100× below the best React build of an equivalent site (Astro + React islands + Radix: 317 KB raw, measured in research/baselines.md) | |
| T4 precision | deleting the dialog from `/syntax/` removes exactly its markup, its CSS rules and its behaviours from that page, and changes no other page's bytes | refutes |
| T5 awareness | against the control: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one | |
| T6 authoring | T-authoring of RGP2-040: no hand-written JS, no per-page asset lists | refutes |
| T7 behaviour | the browser checks pass on the built site | refutes |
| T8 requests | ≤ 3 per page, cold | |

The result, with the tables, is written to `specs/phase02/bet.md`.
- Depends on: 040.

### RGP2-060 — CI and docs · S
A `site` job (build + the byte report as an artifact); getting-started gains
*Build*; CHANGELOG; CLAUDE.md.

## Later, noted here so it is not lost

- Keyed slots and integer-like keys (components.md, *Known limits*); the
  false `segment-children` warning on `<Dialog #install><$Title>…`; the
  opaque TS2559 for a keyed slot element without `key`
  (research/components.md, section 7).
- A catalog of ~20 components where a page uses 3–5: the scale test of
  per-page precision.
- Firefox and Safari proper, and touch devices: nobody has run them.
- Phase 1's leftover: the server's intermittent crash while pushing tsconfig
  diagnostics (phase01/plan.md, RGP1-113).
