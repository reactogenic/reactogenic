# Phase 2 decisions

Taken on 2026-10-04 from the research ([research.md](research.md)), while the
owner was away: each is the recommended choice, with what would reverse it.
**For review** marks the ones that touch something CLAUDE.md fixes or that
are a matter of taste.

| # | Decision | Why | Reverse if |
| --- | --- | --- | --- |
| 1 | **esbuild is the linker, through its public Go API, in-process. No fork, no new linker.** `Splitting` off | the only seam it has is enough; nothing in phase 2 needs AST access or a chunk hook; a fork carries ~87k lines | an optimisation must see linked code, or sharing below module granularity is needed |
| 2 | **Pages are executed in an embedded engine (`modernc.org/quickjs`), serialised by React's own static renderer** | shell HTML equals what React renders, by construction; a Go evaluator was silently wrong on 19 of 36 ordinary cases; pure Go, six targets, +2.6 MB | the engine's limits bite (no `Intl`, no timers), or build time matters at thousands of pages |
| 3 | **React is a build-time dependency of a project**, never of its output | the components are React components; islands will render the same code | — |
| 4 | **Shell rule S2 reads "no *runtime* variance"** (builder.md, *Shell code in phase 2*) — **for review** | execution makes compile-time loops and conditionals free and deterministic; the design system's own components need them; forbidding `NAV.map(…)` over a constant is a fake constraint | the owner wants S2 absolute in page code — then the design system becomes an exempt third kind of code |
| 5 | **The design system is authored in `.rtsx`** — **for review** | CLAUDE.md says "plain `.tsx`"; that predates the attachment syntax (`slot={$X}`), which exists for containers. `.rtsx` *is* plain `.tsx` after phase 1's transpiler, and the builder reads it from the program | the catalog must be consumable without our toolchain — then it is published transpiled |
| 6 | **No `text/template` in the binary**: HTML is written as strings — **for review** | one `template.Execute` costs 18.6 MB: reflection stops the linker pruning tsgo. CLAUDE.md's "Go templates for shells" needs another carrier | a server in a separate binary |
| 7 | **Component awareness comes from the record of execution**, not the import graph | the graph over-reports (a layout that can attach a dialog imports it on every page) | — |
| 8 | **CSS: plain files, pruned per page against the emitted HTML, with runtime state as "maybe"** | sound by construction, and the largest untapped saving; no vendored CSS parser — the public API's flat output is regular | pruning proves unsound in the browser comparison |
| 9 | **JS: `mount(module, id, flags)`; flags are `Define`d per page; one build per page** | −45% from flags alone, with no AST work; `mountX(root)` is layout.md's own design | — |
| 10 | **Packaging by content hash; inline unless shared enough to pay for a request** | a small blob used once is cheaper inline; one shared by many pages is cheaper as a file | hosting cannot cache `/_rg/*` forever |
| 11 | **Browser floor: Chrome/Edge 135, Firefox 147, Safari 26.2**; the `commandfor` fallback ships with every dialog — **for review** | zero JS for open / close / focus / placement at the floor; below it a dialog button would be dead for ≈15% of usage | a lower floor is wanted: +601 B of tested shims reaches Baseline 2024 |
| 12 | **Routes are files: `index.rtsx` of a directory under `pages/`**; the layout is an ordinary component; the page renders from `<html>` | the minimum; segments stay next to the page that mounts them | a route table arrives with the server (`route-table.md`) |
| 13 | **The current page is the builder's**: `SideMenu` compares `href` with `pathname()` | no `current` prop to get wrong | — |
| 14 | **Theme follows the OS; no search; code samples are plain `<pre>`** | each of the others needs JS that no layout component owns, and phase 2 has no author JS | page-author raw JS is decided (layout.md, OPEN) |
| 15 | **`@reactogenic/ui` and the site are private workspace packages** | publishing a design system is its own decision | — |

## Binary size

With `reactogenic build` in the binary (RGP2-030) — esbuild, the engine, the
HTML parser and the builder's own packages — `scripts/build-binaries.sh
0.0.0-dev`, stripped, `CGO_ENABLED=0`:

| Target | Phase 1 | With the builder | |
| --- | --- | --- | --- |
| darwin-arm64 | 27.3 MB | 35.1 MB | +7.8, +29% |
| darwin-x64 | 28.6 MB | 37.1 MB | +8.5, +30% |
| linux-arm64 | 26.4 MB | 34.3 MB | +7.9, +30% |
| linux-x64 | 27.9 MB | 36.5 MB | +8.6, +31% |
| win32-arm64 | 26.5 MB | 34.3 MB | +7.8, +29% |
| win32-x64 | 28.2 MB | 36.9 MB | +8.7, +31% |

- Over the 10% line of `scripts/build-binaries.sh`; accepted for the builder:
  it is the linker and the engine, and neither can be had smaller (decisions
  1, 2). The research measured darwin-arm64 at ≈ 34.7 MB.
- Of the growth, ≈ 7.2 MB came with the engine and esbuild (plan.md,
  RGP2-011: 34.5 MB on darwin-arm64); the driver, the pruner, the page checks
  and the behaviours add ≈ 0.6 MB.
- No `text/template`, no `html/template`: the binary has no symbol of
  either (decision 6).
- Measured again after the review of RGP2-030 (plan.md): the six sizes are
  the same to 0.1 MB, and there is still no symbol of either package.
