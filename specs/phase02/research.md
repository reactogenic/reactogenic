# Phase 2 research: where the builder hooks in

The question: **does esbuild have a clean seam for Reactogenic to hook into, or
does it need a new type of compiler?**

**Answer.** esbuild has exactly one clean seam, and it is enough: its public
Go package (`pkg/api`), used in-process as the **linker and minifier behind
our own compiler**. It has no seam *inside* — no AST access, no hook on
chunks, no manual chunks, no HTML entries — and its maintainer rules those
out. Nothing in phase 2 needs them. What is new is not a bundler: it is the
**component-aware compiler in front** (it executes the page, records what was
rendered, and decides what each page ships) and the **build driver around**.
No fork of esbuild, no linker of our own.

Nine investigations (2026-10-04), each re-checked by an independent skeptic
who re-ran the experiments. The reports, with their verification sections,
are in [research/](research/); numbers below are measured unless marked.

| Report | Question | Result |
| --- | --- | --- |
| [esbuild-seams](research/esbuild-seams.md) | every seam esbuild has and lacks | `pkg/api` in memory is sufficient; nothing inside is reachable; a fork would carry ~87k lines |
| [prior-art](research/prior-art.md) | where component-aware compilers sit | Astro, Marko, Qwik, SvelteKit, Fresh: compiler in front, own driver, a general bundler as linker. Nobody wrote a linker; nobody lived in a transform plugin alone |
| [evaluation](research/evaluation.md) | executing components at build time | an embedded JS engine running React's own static renderer; a Go evaluator was refuted as the first choice |
| [platform](research/platform.md) | how little JS the three components need | none for open / close / focus / placement at Chrome 135, Firefox 147, Safari 26.2 |
| [js-behaviours](research/js-behaviours.md) | per-page JS | per-page `Define` flags + top-level-function authoring lands ≈19% above hand-written code in sum — 10–37% per page, at most 148 B |
| [css](research/css.md) | per-page CSS | esbuild bundles and minifies, never prunes; pruning against the emitted HTML is the builder's step |
| [components](research/components.md) | the three components in slot terms | contracts type-check under phase 1 today; no new syntax |
| [codebase](research/codebase.md) | what phase 1 gives the builder | the mapped program is the front half; nothing emits HTML, CSS or client JS yet |
| [baselines](research/baselines.md) | what the bet is measured against | a 4-page site built 13 ways; a hand-written floor; thresholds |

## What esbuild is, for us

| | |
| --- | --- |
| used | `api.Build` with virtual modules (a plugin's `OnResolve` / `OnLoad`), `Write: false`, `Define`, `Metafile`; `api.Transform`. 2–9 ms per page in-process |
| does well | resolve, bundle, tree-shake, scope-hoist, minify, lower syntax and CSS nesting, per-entry CSS bundles in import order |
| does not do | AST access (out of scope for good, per the maintainer); a hook between linking and output; manual chunks; HTML; CSS pruning; constants across a call or module boundary in one pass (a second `api.Build` over the linked output folds `export const` flags: 1083 → 367 B) |
| `Splitting` | **off**: at this size every page gets larger and needs more requests (532 B / 1 request → 751 B / 3), a chunk is made per set of entry points and never splits a file, and the evaluation-order bug (#399) still reproduces on 0.28.2 |
| cost | +4.7 MiB on the release binary (darwin-arm64: 27.4 → 32.0 MiB); no dependency conflict with tsgo; cross-compiles for the six targets without cgo |

Binary sizes here are MiB (bytes / 2²⁰), as `scripts/build-binaries.sh`
prints them under the label "MB": the phase 1 binary for darwin-arm64,
rebuilt, is 28,685,106 B = 27.36 MiB (28.7 MB).

**Specialisation needs no AST work.** A dialog behaviour whose features are
free identifiers set by `Define` goes from 1116 B to 367 B; the same module
taking an options object stays at 1214 B. esbuild removes code only on
parser-time constants, so behaviours are *authored for this linker*: flags
are bare identifiers, each feature is a top-level function, no classes, no
options objects (builder.md, *Behaviours*).

That number is not where the bet shows: it measures esbuild's minifier as
much as component awareness. terser `unsafe` reaches 339 B from the options
form with one call site, knowing nothing of components — and gives up at
two call sites (1297 B). The bet shows where generic tools stop: a
component used several times with different props, per-page CSS, the HTML.

**One resolver.** esbuild's own resolution picked `widget.rtsx` where the
checker had checked `widget.jsx`. The plugin answers resolution from the
program, so what is built is what was checked.

## What executes the page

Fixed already: layout components are *executed by the compiler*. Open was the
engine.

| Option | Verdict |
| --- | --- |
| a Go evaluator over the tsgo AST for a shell-safe subset | **not first.** A 1,455-line prototype was byte-identical on its own fixture, but on 36 ordinary cases it matched React on 12, refused 5 and was **silently wrong on 19** (`aria-expanded`, `aria-hidden`, `aria-modal` among them). It is a second implementation of JS semantics, of React DOM's attribute rules and of `@reactogenic/core` |
| an embedded engine with our own serializer | the serializer is the same liability: 11 of 12 trees differ from `react-dom/server` |
| **an embedded engine running React's static renderer** | **chosen.** `modernc.org/quickjs` (pure Go, +2.6 MiB) runs the esbuild bundle of the page with React's legacy static renderer unmodified, no polyfills: HTML equal to Node's `react-dom/server` on 36 of 36 cases; 101 components in 20 ms. Shell HTML equals what React renders for the same component by construction of the serializer — which islands will need. The engine's JavaScript is not V8's where ICU is involved: no `Intl`, and `localeCompare` sorts `["b","a","C"]` as `Cab` (V8: `abC`). The builder refuses those (builder.md, *The engine*) |
| a Node subprocess | the test oracle. Node is not guaranteed next to the binary (the extension's bundled one, a Go server later) |

React is a **build-time** dependency of a project; no byte of it reaches the
output.

Found on the way, and binding: **`text/template` costs this binary
18.6 MiB** (27.4 → 46.0 MiB; 28.7 → 48.2 MB). Its reflective method lookup stops the linker pruning
tsgo's exported methods — the mechanism behind goja's +24.5 MB. The builder
writes HTML as strings. CLAUDE.md's long-term "Go templates for shells" needs
another carrier if it means `text/template` in this binary.

## What the platform does

At Chrome/Edge 135, Firefox 147, Safari 26.2 (invoker commands: Baseline
since 2025-12; anchor positioning's core: 2026-01):

| Component | HTML | JS at the floor, to open / close / focus / place |
| --- | --- | --- |
| Dialog | `<dialog>` + `command="show-modal"` / `commandfor` | none |
| DropdownMenu of links | `popover` + `popovertarget` + anchor positioning | none |
| DropdownMenu with an action | the same, `role="menu"` | 414 B: the keys APG requires |
| SideMenu | `<nav popover>` (a drawer below the breakpoint, a column above), `<details>`, `aria-current` written per page | none |
| any page with one of them | | 176 B (`overlays`); with a dialog, + 190 B (`invokers`) |

"Zero JS" does not survive the last row. Three things the reports' first
versions missed, found by the skeptics and now part of the contract
(components.md):

- **Back restores open overlays** (bfcache): 176 B per page that has one.
- **Below the floor a dialog button is dead** (≈15% of global usage): the
  190 B feature-detected `commandfor` fallback ships with every dialog.
- **The scrim takes focus** unless it comes after the panel (and, when the
  panel has nothing focusable, the dialog itself has `autofocus`); **the
  drawer's backdrop lets a tap through**; `position-area` does not flip in
  the WebKit "26.0" build on a page taller than the viewport — the "26.6"
  build and Safari 27.0.1 flip; Safari 26.2–26.6, the floor, is not
  verified — while `anchor()` insets + `flip-block` flip in all that were
  run.

Not measured: Firefox, Safari 26.x (the floor), real key and pointer input
in Safari (27.0.1 was run script-driven), and every touch device. WebKit
builds and compat data stand in.

## What the bet is worth

Same 4-page docs site, means per page, brotli:

| Build | JS | Total | Requests |
| --- | ---: | ---: | ---: |
| hand-written floor (platform primitives) ¹ | 358 B | 7.5 KB | 3 |
| the same at behaviour parity ² | 571 B | — | 3 |
| Astro + hand-written scripts | — | 7.7 KB | 2 |
| Starlight | — | 44.0 KB | — |
| Astro + React islands + Radix (the best React build) | 90.3 KB | 99.7 KB | 15 |
| Vite SPA + Radix | 103.4 KB | 105.5 KB | — |
| Next.js static export + Radix | 146.7 KB | 158.6 KB | — |

¹ 76% of its JS is theme, copy and search, which the phase 2 site does not
have (decisions.md, 14). ² With what components.md specifies and the first
floor lacked — typeahead, Tab closes, focus return, `aria-haspopup`, a
backdrop fallback: 897 → 1,584 B raw, 358 → 571 B brotli.

- Dropping React is worth **250–470×** in JS (against the first floor) — and
  is available today to anyone who hand-writes platform code. Astro and
  Marko already get most of it. So the bet is not "fewer bytes than React";
  it is **who writes the platform code**: pages written with `SideMenu`,
  `Dialog`, `DropdownMenu` and no hand-written JS must land near the
  hand-written floor **at parity**.
- What component ownership adds on top, measured: JS −45% from flags alone in
  the same authoring style (≈−70% with the platform contract); CSS 39–76% of
  the one-bundle file per page. CSS is the larger untapped saving — docs
  sites ship 17–58% of CSS that matches nothing on the page — and no
  framework found prunes at design-system variant granularity without a
  runtime: CSS-in-JS critical extraction (Emotion's `extractCritical`) has
  derived a page's CSS from what was rendered for years, and hydrates.
- It also buys what bytes do not show: ids, anchor pairs and
  `aria-current` generated per page; id references, command targets and
  links checked per page.

The thresholds are in plan.md (RGP2-050). The control that keeps the bet
falsifiable is an ablation of our own build: the same site with awareness off
(one CSS bundle, one script with every behaviour the site mounts and every
flag on).

## Not answered

- Markdown content and highlighted code samples: a second front end;
  third-party JS at build time would run in the same engine.
- A catalog of 20 components where a page uses 3–5: the realistic test of
  per-page precision. Three components in a shared layout barely differ per
  page.
- Islands: how holes and `Dynamic` meet the executor's record.
