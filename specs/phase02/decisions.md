# Phase 2 decisions

Taken on 2026-10-04 from the research ([research.md](research.md)), while the
owner was away: each is the research's recommended choice — except 16 to
20, which the specs, their review and the integration took against it or
without it — with what would reverse it. **For review** marks the ones that
touch something CLAUDE.md fixes or that are a matter of taste. What was
*not* decided is at the end: *For the owner*.

| # | Decision | Why | Reverse if |
| --- | --- | --- | --- |
| 1 | **esbuild is the linker, through its public Go API, in-process. No fork, no new linker.** `Splitting` off | the only seam it has is enough; nothing in phase 2 needs AST access or a chunk hook; a fork carries ~87k lines | an optimisation must see linked code, or sharing below module granularity is needed |
| 2 | **Pages are executed in an embedded engine (`modernc.org/quickjs`), serialised by React's own static renderer** — **for review**: the research's verification left it to the owner whether shell HTML must equal React's, and whether docs content is `.rtsx` or Markdown | shell HTML equals what React renders, by construction of the serializer (the engine's own JavaScript differs from V8's where ICU is involved — no `Intl`, `localeCompare` — and the builder refuses those); a Go evaluator was silently wrong on 19 of 36 ordinary cases; pure Go, six targets, +2.6 MiB | the engine's limits bite (no `Intl`, no timers), or build time matters at thousands of pages (a runtime per page: 2.9 ms each) |
| 3 | **React is a build-time dependency of a project**, never of its output | the components are React components; islands will render the same code | — |
| 4 | **Shell rule S2 reads "no *runtime* variance"** (builder.md, *Shell code in phase 2*), and **S1 admits context**: `createContext` / `useContext` are resolved while the page executes. `Suspense`, `lazy`, `use` of a promise and `async` components stay out, as layout.md has them — **for review** | execution makes compile-time loops and conditionals free and deterministic; the design system's own components need them; forbidding `NAV.map(…)` over a constant is a fake constraint. Context at build time is a value like any other | the owner wants S2 absolute in page code — then the design system becomes an exempt third kind of code. S1's context: a provider in shell code does not reach an island (layout.md, I1) |
| 5 | **The design system is authored in `.rtsx`** — **for review** | CLAUDE.md says "plain `.tsx`"; that predates the attachment syntax (`slot={$X}`), which exists for containers. `.rtsx` *is* plain `.tsx` after phase 1's transpiler, and the builder reads it from the program | the catalog must be consumable without our toolchain — then it is published transpiled |
| 6 | **No `text/template` in the binary**: HTML is written as strings — **for review** | one `template.Execute` costs 18.6 MiB: reflection stops the linker pruning tsgo. CLAUDE.md's "Go templates for shells" needs another carrier | a server in a separate binary |
| 7 | **Component awareness comes from the record of execution**, not the import graph | the graph over-reports (a layout that can attach a dialog imports it on every page) | islands: what an island renders is known only from its imports (layout.md, *Delivery*) — the record covers shell code |
| 8 | **CSS: plain files, pruned per page against the page as it is served and the page's script, with runtime state as "maybe"** | sound by construction, and the largest untapped saving; no vendored CSS parser — the public API's flat output is regular | pruning proves unsound in the browser comparison; islands: the HTML is no longer the whole DOM |
| 9 | **JS: `mount(module, id, flags)`; flags are `Define`d per page; one build per page** | −45% from flags alone, with no AST work; the `(root)` signature is layout.md's own | islands: a clone has no id at load, and its mount returns a handle |
| 10 | **Packaging by content hash; inline unless shared and larger than a request (250 B gzipped)** | a blob of one page is cheaper inline; one that several pages share is cheaper as a file from a visitor's second page | hosting cannot cache `/_rg/*` forever |
| 11 | **Browser floor: Chrome/Edge 135, Firefox 147, Safari 26.2**; the `commandfor` fallback ships with every dialog — **for review** | no JS to open / close / focus / place at the floor (`overlays` per page with an overlay all the same — 176 B in the research, for Back; 252 B as built, since it also closes on a link into the page); below it a dialog button would be dead for ≈15% of usage | a lower floor is wanted: +411 B (anchored placement) on top of the fallback already shipped reaches Baseline 2024 — on iOS, 18.3. The shims were measured by their author only |
| 12 | **Routes are files: `index.rtsx` of a directory under `pages/`**; the layout is an ordinary component; the page renders from `<html>` | the minimum; segments stay next to the page that mounts them | a route table arrives with the server (`route-table.md`) |
| 13 | **The current page is the builder's**: `SideMenu` compares `href` with `pathname()` | no `current` prop to get wrong | — |
| 14 | **Theme follows the OS; no search; code samples are plain `<pre>`** | each of the others needs JS that no layout component owns, and phase 2 has no author JS | page-author raw JS is decided (layout.md, OPEN) |
| 15 | **`@reactogenic/ui` and the site are private workspace packages** | publishing a design system is its own decision | — |
| 16 | **`pathname()` is public**: usable in any component — **for review** | `SideMenu` needs it; an author's own nav or breadcrumb will. The research recommended hiding it in phase 2 (research/components.md, D11) | the owner wants the pathname the framework's own |
| 17 | **A dialog written in shell code is a live `<dialog>`, opened by `command` / `commandfor`** — **for review** | no JS, no holes: it answers layout.md's OPEN "a shell component used directly in shell code — who opens it", with the alternative layout.md lists as considered and not chosen (one live, hidden `<dialog>`). `<template>` + clone stays for dialogs with holes | islands need one delivery for both |
| 18 | **Every page is rendered in a runtime of its own; `Suspense`, `lazy`, `use` of a promise, `async` components are shell-react** | a page's bytes must not depend on the pages built before it; a boundary renders its fallback for any error of its content, silently | — |
| 19 | **A page with a script of its own is built, and not pruned** — until the owner decides (below) | sound either way; forbidding it is a rule about page-author JS, which CLAUDE.md defers | — |
| 20 | **The page is pruned as it is served, and the builder's own `<script>`, `<style>` and `<link>` are named to the pruner** (taken with the integration, RGP2-040) | the review of the driver wanted the first, the review of the specs the rule that a page's own script turns pruning off; together and unreconciled, every page with a behaviour was unpruned, silently. An element packaging wrote is one a rule may select, and nothing more | pruning before packaging, with the script beside it — if nothing may select on what packaging writes: `--base` in a link is the counter-example |

## For the owner

Not decided here: each would relitigate something CLAUDE.md or a phase 1
spec fixes, or is a choice between two specs — K, L and M are what the
measurement leaves open ([bet.md](bet.md)). The recommended option is
first; the place in the spec carries an `> OPEN:`.

| | Question | Options | Where |
| --- | --- | --- | --- |
| A | **Loop-produced slot items.** CLAUDE.md (*Deferred / roadmap*) and phase01/syntax.md say "phase 2"; plan.md has no task, and `<Each><$Item/></Each>` is still orphan-slot. A side menu from a constant `NAV` cannot be written | **1. later**: the site writes its menu out as slot elements; CLAUDE.md and syntax.md say "later" (the bet does not need it). 2. a task in this phase, before RGP2-040: syntax, types and the transpiler for `Each` / `.map()` around slot elements | plan.md, top; builder.md, *Shell code in phase 2* |
| B | **Dialog's body: `children`, or layout.md's `<$Contents>`.** components.md takes `children`; layout.md defines holes for slot bodies only | **1. `children` in phase 2** (nothing has holes yet), and layout.md is amended when islands arrive: a shell component's `children` is a body with holes too. 2. `<$Contents>` now, so that the island form is the phase 2 form | components.md, `Dialog` (the other track's file: the OPEN belongs there) |
| C | **What islands need from the builder.** Not carried over (builder.md, *Not in phase 2*): pruning against the page's HTML (an island's DOM is not in it; a `<template>` turns pruning off), the generated entry (mounts by id at load), the record (an island is not executed at build time), shell-handler at element creation (`<Dynamic><button onClick>`), `pathname()` under `--base` in React | **1. islands fall back to their imports** for CSS and JS, as layout.md's *Delivery* has it — every rule of a component an island imports stays, template content matches as "maybe" — and the record stays shell-only. 2. islands are executed once at build time for a first record | builder.md, *Packaging* (OPEN), *Not in phase 2* |
| D | **Keyed slots and integer-like keys.** A menu written `10, 9, 2` renders `2, 9, 10`; `check` is silent; all three components iterate keyed slots. The fix changes what phase 1 emits | **1. the `KEYED` marker carries the keys in written order** (research/components.md, recommendation 5: before the site is written), syntax.md updated. 2. a diagnostic for an integer-like `key`. 3. a documented limit, as now | plan.md, *Later*; components.md, *Known limits* |
| E | **A page's own `<script>`.** Today: built, and its CSS is not pruned (decision 19). It also breaks "the JS of a page is the behaviours its components mounted, and nothing else" and T1, silently. With it, since the review of the integrated tree: **a document of the site in a frame** (`<iframe src="/demo.html">`, `srcdoc`, `<object>`, `<embed>`) — its script writes to the page as the page's own would, so such a page is not pruned either. A frame of another site (a URL in full) or a sandboxed one is not one | **1. an error in phase 2** (`page-script`; data blocks stay legal) until page-author raw JS is specified (CLAUDE.md, *Deferred*) — and a frame of the site's own stays legal, unpruned: it is a document, not page-author JS. 2. as now, with the report saying why the page was not pruned | builder.md, *CSS* |
| F | **T2's budget** (≤ 1.5 KB raw on the heaviest page) was set against the first hand-written floor; the verified floor at behaviour parity is 1,584 B raw *mean* — of a site with theme, copy and search, which this one lacks | **1. keep it as an absolute budget for this site's three behaviours**, and say so (plan.md does now). 2. restate it as a multiple of a parity floor rebuilt for the phase 2 site | plan.md, RGP2-050 |
| G | **The close button's name** is the English `Close`, on the dialog and on the drawer | **1. `closeLabel?: string`** on `DialogProps` and `SideMenuProps` (content; default `"Close"`). 2. a `$Close` slot. 3. as now | components.md, `Dialog` |
| H | **A dialog under a closed popover** of the author's own opens modal and unseen; only `SideMenu`'s drawer is seen to | **1. a page check, `dialog-in-popover`** (no byte; the failure is silent otherwise). 2. a documented limit, as now | components.md, `Dialog`; builder.md, *Checks on the page* |
| I | **A disabled menu item** is skipped by focus, arrow keys and typeahead; APG's menu pattern keeps it focusable | **1. APG's**: `aria-disabled="true"` on a button item too, the arrow keys stop at it. 2. as now (two wave-1 decisions with tests) | components.md, `DropdownMenu` |
| J | **Links into the current page** in a `SideMenu` (`/guide/#install`) are not marked and open no group — so the docs site's groups each start with an "Overview" link | **1. such an item opens its disclosures and stays unmarked.** 2. as now | components.md, `SideMenu` |
| K | **Per-page sheets share nothing.** Measured (bet.md): the docs site's four pruned sheets are four blobs — 7,179 B of each the same rules — so each is inlined in its page, and over a four-page visit the control, with one cached sheet, transfers 19.8% less (brotli). Decision 10 shares a blob only when it is the same bytes | **1. two blobs per page: the rules every page of the site keeps, as one file, and the page's own rest** (≈ 7.2 KB once, then ≈ 1.8 KB on two pages and under 0.1 KB on two — estimated, not built; the order of rules has to survive the split: a rule that moves behind one it preceded can win where it lost). 2. as now: the cold page is what is optimised. 3. per-page pruning only where it saves more than the request it costs a visit | builder.md, *Packaging* |
| L | **T5 and the verdict.** T5's JS half ("≥ 30% smaller on every page that ships one") cannot hold on a page that mounts everything the site mounts — `/syntax/`: 12.1%, 0.0% without the control's table — and T5 gave no unit: its CSS half holds raw (22.8%, 23.2%) and not compressed (≤ 16.3%). By plan.md's rule the bet is then neither confirmed nor refuted | **1. the verdict stays "undecided" and T5 is measured again on a catalog of ~20 components** (plan.md, *Later*), where a page that uses everything is not the common case. 2. T5 restated after the fact — "on every page that mounts less than the site does", in raw bytes — under which it holds here: a threshold moved to where the number is | plan.md, RGP2-050; bet.md |
| M | **State that nothing can reach stays "maybe".** Measured (bet.md): each page's sheet of the docs site keeps 301 B (raw; 3–4%) of `.rg-button:is(:disabled,[aria-disabled=true])`, the menu's same rule and its `[aria-current]`, on pages with no such element and no behaviour that makes one. builder.md's list of runtime state — every pseudo-class, and `open`, `hidden`, `inert`, `disabled`, `checked`, `selected`, `value`, `style`, `aria-*`, `data-state` — is "what the browser and a behaviour write *without saying*", and predates the pruner reading the page's script | **1. as now**: one line of rule, sound whoever writes; the cost is 3–4% of a sheet. 2. the attributes of the list that only a script writes (all but `open`, and `hidden` for `until-found`) are decided on the page unless the page's script names them, as any other attribute is — 72 B a page here; components.md's convention then reads "a behaviour names the state it writes". 3. and `:disabled`, `:checked`, … decided from the attributes on a page whose script names none of them — the other 229 B, and a model of each pseudo-class to keep sound | builder.md, *CSS* |

## Binary size

With `reactogenic build` in the binary (RGP2-030) — esbuild, the engine, the
HTML parser and the builder's own packages — `scripts/build-binaries.sh
0.0.0-dev`, stripped, `CGO_ENABLED=0`. MiB throughout: `build-binaries.sh`
prints bytes / 2²⁰ as "MB".

| Target | Phase 1 | With the builder | |
| --- | --- | --- | --- |
| darwin-arm64 | 27.3 MiB | 35.1 MiB | +7.8, +29% |
| darwin-x64 | 28.6 MiB | 37.1 MiB | +8.5, +30% |
| linux-arm64 | 26.4 MiB | 34.3 MiB | +7.9, +30% |
| linux-x64 | 27.9 MiB | 36.5 MiB | +8.6, +31% |
| win32-arm64 | 26.5 MiB | 34.3 MiB | +7.8, +29% |
| win32-x64 | 28.2 MiB | 36.9 MiB | +8.7, +31% |

- Over the 10% line of `scripts/build-binaries.sh`; accepted for the builder:
  it is the linker and the engine, and neither can be had smaller (decisions
  1, 2). The research measured the two together at darwin-arm64 27.36 →
  34.46 MiB, +7.10 MiB (+26%).
- Of the growth, ≈ 7.2 MiB came with the engine and esbuild (plan.md,
  RGP2-011: 34.5 MiB on darwin-arm64); the driver, the pruner, the page checks
  and the behaviours add ≈ 0.6 MiB.
- No `text/template`, no `html/template`: the binary has no symbol of
  either (decision 6).
- Measured again after the review of RGP2-030 (plan.md): the six sizes are
  the same to 0.1 MiB, and there is still no symbol of either package.
- After the integration of the review tracks (plan.md, RGP2-040):
  darwin-arm64 is 35.15 MiB (36 854 690 B; +0.08 on the build before it — a
  runtime per page, the pruner's reading of the page's script), with no
  symbol of either package. The other five targets were not built again.
