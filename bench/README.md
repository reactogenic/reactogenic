# bench: what a page costs the browser

`measure.mjs` loads each page of a static site cold in headless Chrome from a
local server and sizes what was fetched: HTML, CSS, JS — raw, gzip -9,
brotli -q 11 — requests, JS to parse, and a warm 4-page session. No
dependencies (Node ≥ 20); its header has the method and the options.

```sh
node bench/measure.mjs reactogenic=site/dist --pages /,/guide/,/syntax/,/reference/cli/ --md out.md
```

`baselines/2026-10-04.md`: the same script on a 4-page docs site built 13
ways during the phase 2 research (Vite SPA, Next.js static export, Astro with
React islands — each with Radix and with Base UI — Astro with hand-written
scripts, Starlight, and a hand-written floor in four packagings). The sites
themselves are not in the repository; how they were built and what the
numbers mean is in `specs/phase02/research/baselines.md`.

## The bet

Three scripts measure Reactogenic's own docs site (`site/`) against the
thresholds of `specs/phase02/plan.md`, RGP2-050. The conclusion, with what
the numbers do not show, is `specs/phase02/bet.md`.

```sh
pnpm install --frozen-lockfile   # once, at the repository root
node bench/site.mjs     # bytes: T1, T2, T3, T5, T6, T8   → results/site.md
node bench/delta.mjs    # precision: T4                   → results/delta.md
node bench/verify.mjs   # behaviour, in browsers: T7      → results/verify.md
```

All three, as one line — the hint for a `bench` script, should the root
`package.json` get one:

```sh
node bench/site.mjs && node bench/delta.mjs && node bench/verify.mjs
```

Each builds the `reactogenic` binary from `go/` (a Go toolchain), or takes
one: `--binary <path>` or `$REACTOGENIC_BINARY`. Each builds the site into a
temporary directory (`--keep <dir>` to look at it), prints its report, writes
it to `results/` (`--md <file>` for elsewhere), and exits non-zero when a
threshold that refutes the bet fails.

| Script | Does | Needs |
| --- | --- | --- |
| `site.mjs` | builds the site four ways — as it ships, `--inline always`, the control `--no-specialize`, and `--inline never` — and runs `measure.mjs` on each; then, from the builds' files and `_rg/report.json`: what each page is whatever its delivery, the control's script split into behaviours and pathname table, the default against the control page by page, every page's script row by row, and a grep of the site's source | Node ≥ 22. Chrome for the cold loads (`$CHROME`); without it, or with `--static`, the pages' own `<link>` and `<script>` are followed instead — the same bytes on this site, one request fewer per page: the favicon |
| `delta.mjs` | copies the site, deletes the *Install* dialog from `/`, rebuilds, and diffs: the markup, the CSS selectors and the behaviours that left, and that no other page changed. Then the cheat-sheet dialog of `/syntax/` — alone (the build must refuse it), and with the menu item that opens it — and the menu's `typeahead` | Node ≥ 22; `site/node_modules` (the copy links to it) |
| `verify.mjs` | the built site and the control in Chromium and WebKit, at 1200 and 400 px: the components' behaviour on real output, the computed style of every element in the two builds — at rest, dark, reduced motion, hover, keyboard focus, each overlay open — the same comparison on the builder's fixture for what a page's script takes away (`go/internal/build/testdata/served`: `/toggle/` before and after its behaviour runs, `/frame/`), which the site cannot show: its behaviours write nothing — and the probe of the pruner's selector tables (`go/internal/build/cssprune/selector.go`) | Playwright and its browsers, which are `packages/ui`'s: `pnpm --filter @reactogenic/ui exec playwright install chromium webkit`. `--engines chromium`, `--inline never`, `--shots <dir>` |

`lib.mjs` is what they share: the binary, a build, a page of a build taken
apart, a page's script read statement by statement (T1), a sheet read as
its selectors.

`site.mjs` and `delta.mjs` write the same bytes on every run of the same
tree: nothing in their reports is a path, a date or a version.
`verify.mjs`'s report names the engines it ran.

`site/test/browser.mjs` (`pnpm --filter @reactogenic/site test:browser`) is
the site's own suite, with `--base` and screenshots of whole pages;
`verify.mjs` is the measurement of T7: it adds Back, hover, dark, reduced
motion, the menu's Tab, the probe, and a check that the style comparison
sees a missing rule.

## The catalog

The docs site has three components in one layout; what component awareness
is worth where a page uses a few of many is measured on a **fixture**
(`specs/phase02/bet.md`, *The catalog*; decisions.md, *For the owner*, L):

| | |
| --- | --- |
| `catalog/` | `@reactogenic/bench-catalog`: twenty components — the four of `@reactogenic/ui` and sixteen written for the measurement, in `.rtsx`, each with its CSS, four with a behaviour. **Not the design system** (`catalog/README.md`) |
| `catalog-site/` | ten pages of the kinds a product site has, each on three to five components beyond the layout's — one on eight, one on none. Which page uses what, and why, was written down before anything was measured (`catalog-site/README.md`) |

Both are private workspace packages (`pnpm-workspace.yaml`): the site
resolves the catalog as a site resolves a design system.

```sh
node bench/catalog.mjs          # bytes: T5 in brotli, T1, T2, T6, T8, a delta in T4's style → results/catalog.md
node bench/catalog-verify.mjs   # browsers: the pages load, and pruning changes nothing      → results/catalog-verify.md
```

| Script | Does | Needs |
| --- | --- | --- |
| `catalog.mjs` | type-checks the site (`reactogenic check`), builds it three ways — as it ships, the control `--no-specialize`, `--inline never` — each of which must print no diagnostic, and runs `measure.mjs` on each. Then: what each component weighs, what each page is, the default against the control page by page (CSS, JS, CSS + JS, and as a share of the page), the session of ten pages with its running total, **T5 page by page in brotli bytes** with gzip and raw beside, and T1, T2, T6, T8 as each can be read on a catalog. Last, one component deleted from one page, twice (the FAQ accordion of `/pricing/`, the toast of `/settings/`): what leaves, and that no other page changes. The same options as `site.mjs`; the same bytes on every run | Node ≥ 22; Chrome for the cold loads, or `--static` |
| `catalog-verify.mjs` | the site and the control in Chromium and WebKit, at 1200 and 400 px: every page loads with no console error and no failed request; and in each of 244 states per engine — at rest, dark, reduced motion, forced colours, keyboard focus, the layout's menu and drawer, and what the catalog's own behaviours write (a tab selected by a click, a key and the hash, a field counted to its limit, a password shown, a toast, a sample copied, …) — the computed style of every element is the same in the two builds, **and no selector that the page's sheet lacks matches an element**: that one names what was wrongly dropped. `--engines`, `--inline never`, `--shots <dir>` | Playwright and its browsers, as `verify.mjs` |

`compare.mjs` is the comparison of two builds of one page, lifted from
`verify.mjs` — which keeps its own copy, so that the measurement of the docs
site is the script that was reviewed.
