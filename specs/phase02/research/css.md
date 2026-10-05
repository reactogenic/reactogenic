# Phase 2 research: CSS for a compiled design system

Researcher key: `css`. Date: 2026-10-04. Experiments: `phase2-research/exp-css/` (paths below are relative to it).
All byte counts are measured by the scripts named next to them; gzip is level 9, brotli is quality 11 (Node 25.2.1 `zlib`).

## Summary

- **esbuild does bundling, nesting lowering and minification well, and nothing else.** It emits one full CSS bundle per entry point, duplicates shared CSS in every entry, and never removes an unused rule. Measured with esbuild 0.28.2.
- **Component-aware pruning works and is cheap.** A 715-line Go pruner on esbuild's own CSS AST (reached through an 80-line bridge package) gives per-page CSS that is 38–73% of the one-bundle file (brotli), and 23–43% smaller than esbuild's per-entry output. Pruning takes under 1 ms per page after a 1.2 ms parse.
- **The absolute saving is small on this site.** The whole modelled design system is 4.9 KB brotli, so pruning saves 1.3–3.0 KB per page. For a visitor who opens three pages, one cached bundle costs fewer bytes in total (9.6 KB against 13.8 KB inlined).
- **Soundness is the real argument against generic tools.** PurgeCSS 8.0.0 on the same pages dropped the global `:focus-visible` rule and a runtime-state rule, and kept SideMenu and Dialog rules on a page that only mentions those words in its text.
- **Nearly all of the gain comes from variant, slot and element pruning.** Exact DOM-relation matching adds 1.6%, unused tokens 5.7%, impossible states 3.2%.

## 1. What I built

| Piece | File | Size |
|---|---|---|
| Design-system CSS: tokens, base, Page, SideMenu, Dialog, Dropdown, Button, Card, Prose, Callout, Tabs, Pager | `ds/*.css` | 12 files, 563 lines, 32.1 KB source |
| Four pages as the builder would emit them: landing, guide, API reference, changelog | `genpages.mjs` → `pages/*.html` | 2.3–9.1 KB |
| Bridge into esbuild's internal CSS parser, AST and printer | `esbuild-src/rgcss/rgcss.go` | 80 lines |
| Pruner | `prune/main.go` | 715 lines |
| Measurements | `measure.mjs` → `out/measure.md` | |
| Comparisons | `minicmp/` (tdewolff, esbuild API), `purge/` (PurgeCSS, cross-check, leak count), `atoms.mjs`, `baseline.mjs` | |

The CSS uses native nesting under one root class per component, variants as `data-<prop>` on the root, `data-slot` / `data-part` for slot attachments and internal parts, and runtime state only through pseudo-classes and ARIA / `open` / `hidden` attributes.

Page content:

| Page | Components |
|---|---|
| `index` | Page (landing), Dropdown (sm, bottom-end), Button (primary lg, ghost lg), 3 Cards |
| `guide` | Page (docs), SideMenu (rail, collapsible) + SideMenu (drawer), Dropdown, Button, Prose, Tabs, 2 Callouts, Pager, toc, Dialog (command) with a `<template>` row |
| `api` | Page (docs), SideMenu (rail, compact) + drawer, 2 Dropdowns, Prose with table / dl / details, Callout, Pager, toc, Dialog (command), Dialog (lg modal) |
| `changelog` | Page (plain), Dropdown, Prose |

The library is tailored to the site: the four pages together use 87% of it (site union 4454 B brotli against 5134 B, both with nesting lowered). A general-purpose library would be several times larger, which grows the one-bundle file and leaves the pruned pages unchanged. I did not measure that case.

## 2. What esbuild's CSS support does and does not do

Verified with esbuild 0.28.2 (the CLI in the repo's `node_modules` and the Go source at commit `f6058f8`, 2026-08-09, `version.txt` = 0.28.2).

| Capability | Result | Evidence |
|---|---|---|
| Bundle `@import` | Yes | `esbuild ds/all.css --bundle` → 33 656 B, no warnings |
| Minify | Yes: 33 656 → 25 577 B | `out/all.min.css` |
| Lower nesting | Yes: `--supported:nesting=false` → 29 383 B | `out/all.flat.min.css` |
| Modern syntax passes through | `@scope`, `@layer`, `@container`, `@property`, `@position-try`, `@function`, `if()`, `:open`, `::details-content`, `@starting-style`, `anchor()` all printed unchanged, no warnings | `t/modern.css` |
| Lower `@scope` | No. With `--target=chrome100,safari15,firefox100` the `@scope` block is printed unchanged | same |
| CSS modules (`local-css`) | Renames local names. Does **not** drop a class the JS never imports: `.unused` came out as `.r{color:green}` | `t/m.module.css`, `t/o3/m.css` |
| Unused-rule elimination | None. `.never-used{color:blue}` survives `--minify` | `t/dup.css` |
| Per-entry CSS | One complete bundle per entry point | below |
| CSS code splitting | None. With `--splitting --format=esm` the shared CSS-only modules become four **0-byte** JS chunks, and each entry's `.css` still holds everything it reaches | `out/esb-js/`, `meta.json` |
| Minify custom-property values | No. `--color-bg: light-dark(#ffffff, #0f1115)` keeps its spaces and long hex | `out/all.min.css` |
| Merge same-selector rules after pruning | No. Re-minifying the pruned output changed 7466 B to 7467 B | `esbuild out/index.states.css --minify` |

Per-entry output for the four pages, each entry importing exactly the component files its page uses (`entries/*.css`):

| Entry | Files imported | Bytes (min) |
|---|---:|---:|
| index | 6 | 12 356 |
| guide | 11 | 24 764 |
| api | 10 | 23 920 |
| changelog | 5 | 13 444 |

The four files total 74 484 B. `tokens + base + Page + Dropdown` is in all four and is 9832 B minified, so 3 × 9832 B of that total is duplication.

The esbuild docs describe the same model: "esbuild will gather all CSS files referenced from a given entry point and bundle it into a sibling CSS output file" (https://esbuild.github.io/content-types/#css-from-js, fetched 2026-10-04). The v0.12.0 release notes say each CSS output file "contains the transitive set of all CSS reachable from the JS entry point" (issue https://github.com/evanw/esbuild/issues/608). The Splitting section still reads "Code splitting is still a work in progress. It currently only works with the `esm` output format" (https://esbuild.github.io/api/#splitting, fetched 2026-10-04).

**Plugin seam for CSS.** `OnLoadArgs` has `Path`, `Namespace`, `Suffix`, `PluginData`, `With` and no importer or entry point (`esbuild-src/pkg/api/api.go:679`). A file is loaded once per build and shared by every entry, so an `OnLoad` plugin cannot return different CSS for different pages. Per-page pruning has to happen outside esbuild's bundle step, or with one build per page. UNVERIFIED by running a plugin; this is read from the API type.

## 3. Component-aware pruning: measured

### The pruner

It parses the bundled, un-minified CSS with esbuild's parser, then keeps a selector only if some element of the page **may** match it. Static conditions (tag, class, id, `data-*`, combinators, `:is()`, `:has()`, nesting `&`) are evaluated exactly against the emitted HTML. Runtime conditions are treated as "maybe". Then it drops custom properties and `@keyframes` nothing refers to, and prints with esbuild's printer.

Modes, in increasing precision:

| Mode | What it knows |
|---|---|
| esbuild per-entry | which component files the page imports |
| `keys` | each compound selector must be satisfiable by some element; combinators are ignored |
| `exact` | full selector matching against the DOM, runtime conditions as "maybe" |
| `+ tokens` | also drops unreferenced custom properties and keyframes |
| `+ states` | a runtime condition is "maybe" only where it can occur (table in section 4) |

Cost: parse 1.2 ms, prune 0.2–0.8 ms per page (`out/prune.log`).

With pruning off, the bridge output is byte-identical to `esbuild --bundle --minify` except for one `✓` that the CLI escapes as `\2713` (25 574 against 25 577 B).

### Per page: raw / gzip / brotli bytes

From `node measure.mjs`. Nesting is kept unless noted.

| Variant | index | guide | api | changelog | Sum of 4 |
|---|---:|---:|---:|---:|---:|
| one bundle for all | 25577 / 5521 / 4892 | same | same | same | 25577 / 5521 / 4892 (once) |
| esbuild per-entry | 12356 / 3432 / 3008 | 24764 / 5414 / 4807 | 23920 / 5292 / 4685 | 13444 / 3677 / 3242 | 74484 / 17815 / 15742 |
| pruned: keys | 8988 / 2778 / 2415 | 18453 / 4312 / 3821 | 18802 / 4380 / 3881 | 7306 / 2454 / 2152 | 53549 / 13924 / 12269 |
| pruned: exact | 8875 / 2761 / 2395 | 17750 / 4221 / 3744 | 18242 / 4284 / 3781 | 7306 / 2454 / 2152 | 52173 / 13720 / 12072 |
| pruned: exact + tokens | 7974 / 2447 / 2113 | 17494 / 4153 / 3674 | 17885 / 4173 / 3704 | 6411 / 2168 / 1897 | 49764 / 12941 / 11388 |
| pruned: exact + tokens + states | 7466 / 2301 / 1987 | 17066 / 4041 / 3587 | 17457 / 4062 / 3594 | 6276 / 2117 / 1855 | 48265 / 12521 / 11023 |
| same, nesting lowered | 8229 / 2343 / 2030 | 19422 / 4218 / 3762 | 19700 / 4235 / 3752 | 6778 / 2150 / 1884 | 54129 / 12946 / 11428 |

Brotli size as a share of the one-bundle file:

| Variant | index | guide | api | changelog |
|---|---:|---:|---:|---:|
| esbuild per-entry | 61% | 98% | 96% | 66% |
| pruned: keys | 49% | 78% | 79% | 44% |
| pruned: exact + tokens + states | 41% | 73% | 73% | 38% |

Reading the table:

- **Against esbuild per-entry**, the best pruned variant is 34% smaller on index, 25% on guide, 23% on api and 43% on changelog (brotli).
- **Where the gain comes from** (brotli, sum of four pages): per-entry 15 742 → keys 12 269 (−22%) → exact 12 072 (−1.6%) → tokens 11 388 (−5.7%) → states 11 023 (−3.2%).
- **A docs page uses most of a docs design system.** The guide page keeps 73% of the bundle even at the most precise level.
- **Keeping nesting in the output is worth 4.7%** on the guide page (3587 against 3762 B brotli).

### Soundness check with an independent engine

`purge/crosscheck.mjs` erases runtime conditions from every selector and runs it through linkedom's `querySelector` on each page. Any selector that matches there must survive pruning.

| Variant | (page, selector) pairs | Match statically | Dropped but matching |
|---|---:|---:|---:|
| exact | 1072 | 468 | 0 |
| keys | 1072 | 468 | 0 |
| exact + states | 1072 | 468 | 26 |
| PurgeCSS 8.0.0 | 1072 | 468 | 27 |

The 26 drops in the states variant are the intended ones and rest on the state table: `.Button[aria-busy=true]`, `.Button:disabled`, `.Button[aria-disabled=true]`, `.Dropdown [data-slot=item]:disabled`, `[aria-disabled=true]`, `[aria-checked=true]`, and `[hidden]`. The check cannot see inside `<template>`, so it does not cover cloned content.

### Against a generic HTML-scanning purger

PurgeCSS 8.0.0 with `variables` and `keyframes` on, same HTML, same flat CSS (`purge/run.mjs`):

| Page | PurgeCSS brotli | Ours (flat) brotli | Difference |
|---|---:|---:|---:|
| index | 2083 | 2030 | −2.5% |
| guide | 3893 | 3762 | −3.4% |
| api | 3888 | 3752 | −3.5% |
| changelog | 2312 | 1884 | −18.5% |

On bytes a generic purger gets close when the page is fully static. The difference is in what it gets wrong (`node selset.mjs out/<p>.states.flat.css out/<p>.purgecss-vars.css`):

| PurgeCSS behaviour | Example | Consequence |
|---|---|---|
| Dropped a rule that can match | `:focus-visible` on all four pages | no keyboard focus ring |
| Dropped a runtime-state rule | `.Dialog[data-variant=command] [data-part=result][aria-selected=true]`; the row only exists in a `<template>` and the attribute is set by JS | selected search result is not highlighted |
| Kept rules of absent components | 17 `.SideMenu` / `.Dialog` selectors on changelog, because the page's text says "SideMenu, Dialog, Dropdown" | dead bytes |
| Kept wrong variants | `.Button[data-size=lg]`, `.Dialog[data-size=sm]` on api, because `lg` and `sm` occur elsewhere | dead bytes |

### Atomising

`node atoms.mjs` counts declarations in the pruned flat CSS.

| Page | Declarations | Unique | Rule-based brotli | Atomic lower bound brotli |
|---|---:|---:|---:|---:|
| guide | 467 | 260 (56%) | 3762 | 2828 |
| index | 204 | 146 (72%) | 2030 | 1802 |

The lower bound ignores every pseudo-class, state and media selector, and ignores the class lists the HTML would need (at least 467 references on the guide page). Authored content in Prose cannot be atomised at all without a class on every element. The realistic gain is well under 0.9 KB brotli on the heaviest page and costs readable output. Not worth doing in phase 2.

### Inline or external

HTML + CSS on the wire, brotli, cold cache then HTTP cache, requests in parentheses (`out/measure.md` sections D and G):

| Strategy | index only | guide only | index → guide | index → guide → api | all 4 |
|---|---:|---:|---:|---:|---:|
| S1 one `all.css`, linked | 5720 (2) | 6740 (2) | 7568 (3) | 9604 (4) | 10355 (5) |
| S5 site-union css, linked | 5283 (2) | 6301 (2) | 7130 (3) | 9163 (4) | 9919 (5) |
| S2 per-page pruned, linked | 2818 (2) | 5436 (2) | 8254 (4) | 13884 (6) | 16496 (8) |
| S3 per-page pruned, inlined | 2770 (1) | 5389 (1) | 8159 (2) | 13782 (3) | 16335 (4) |
| S4 common.css linked + rest inlined | 3359 (2) | 6111 (2) | 7651 (3) | 12139 (4) | 13303 (5) |
| S6 common.css + docs.css linked + tail inlined | 3329 (2) | 6353 (3) | 7863 (4) | 10929 (5) | 12061 (6) |

- **One page visited:** inlined pruned CSS is 2770 B and one request, against 5720 B and two requests for one bundle.
- **Two pages:** roughly even.
- **Three or more:** one cached bundle wins. At three pages inlining costs 4.2 KB more.

Rules partition naturally by the set of pages that use them (`out/sig/`, flat, minified):

| Used by exactly | Raw bytes | Brotli alone |
|---|---:|---:|
| guide + api | 9274 | 1746 |
| all four pages | 6525 | 1877 |
| api only | 2861 | 756 |
| guide only | 2520 | 673 |
| index only | 1937 | 583 |
| other 5 sets | 2185 | — |

So the pruner already produces what a chunking policy needs: a rule → page-set table. Whether to inline, link per page, or emit shared chunks is an output-stage choice.

Splitting changes rule order, which changes the cascade. It is only safe if order does not matter between chunks, which `@layer` gives. My S4 and S6 numbers ignore this; they are byte counts, not a tested page.

### Context: what docs sites ship today

`node baseline.mjs`, 2026-10-04. Linked stylesheets on one docs page, recompressed by me at brotli 11 (not the sites' real transfer sizes; font CSS included where the site links it).

| Page | CSS files | Raw | Brotli |
|---|---:|---:|---:|
| vitepress.dev/guide/getting-started | 2 | 199 842 | 33 798 |
| starlight.astro.build/getting-started/ | 3 | 79 076 | 13 980 |
| docusaurus.io/docs | 2 | 173 539 | 32 323 |
| react.dev/learn | 1 | 108 627 | 16 227 |
| svelte.dev/docs/svelte/overview | 9 | 111 651 | 19 173 |
| vite.dev/guide/ | 1 | 261 008 | 40 071 |
| tailwindcss.com/docs/installation/using-vite | 2 | 657 703 | 58 360 |

Our modelled docs page is 3.6 KB brotli pruned and 4.9 KB un-pruned. Most of the gap to these sites comes from a lean design system, not from pruning. These sites also carry features the model does not (search UI, syntax themes, i18n).

## 4. Keeping pruning sound

A rule can be dropped only if no element can ever match it on that page. Three things make that hard.

**Runtime state.** `:popover-open`, `[open]`, `[aria-expanded]`, `:hover`, `:has(...)` over state. The pruner never drops on these. To drop state rules that cannot occur, it needs one more fact per component: which attributes its behaviour writes. The table used in the experiment:

| Attribute | Written by |
|---|---|
| `aria-expanded` | Dropdown |
| `aria-checked` | Dropdown with `data-variant="select"` |
| `aria-selected` | Tabs, Dialog with `data-variant="command"` |
| `hidden` | Tabs |
| `aria-current` | Page (toc scroll-spy) |
| `open` | native on `details`, `dialog` |

Native limits are also used: `:popover-open` needs a `popover` attribute, `:disabled` needs a form control, `::backdrop` needs a dialog or popover.

**Cloned templates.** Content inside `<template>` is inserted somewhere else at runtime, so its ancestors and siblings are unknown. The matcher treats a `<template>` boundary as "any ancestor". Without this the search-result rule above would be dropped.

**Author overrides and slot content.** Both end up in the HTML the compiler emits. A `className` passed through a slot is a class on an element; authored markup in a slot is elements. Author CSS is pruned by the same matcher against the same DOM, so there is no special case. `.Prose table` is kept exactly when the page's content has a table.

What breaks it: any JS that adds a class, a `data-*` attribute or an element the compiler did not emit. In phase 2 all JS is framework-owned, so this is a rule for the framework's own behaviours. Page-author raw JS is on the roadmap and would need the same rule or an opt-out.

### Proposed convention

The compiler owns the attribute vocabulary, so the classification of every selector part is syntactic:

| Selector part | Meaning | Known at |
|---|---|---|
| `.SideMenu` (root class = file name) | component | compile time |
| `&[data-variant="drawer"]`, `&[data-collapsible]` on the root | prop value | compile time |
| `[data-slot="title"]` | slot attachment | compile time (filled or not) |
| `[data-part="close"]` | internal part | compile time |
| tag, class, any other `data-*` | emitted markup, authored content | compile time |
| pseudo-classes, `aria-*`, `open`, `hidden`, `disabled`, `inert` | state | runtime |

Rules that go with it:

1. One `.css` file per component, next to its `.tsx`, everything nested under one root class.
2. Behaviours write state only through the runtime attributes in the last row, and each behaviour declares which ones.
3. `@layer reset, tokens, base, components;` declared once. Author CSS stays unlayered, so it wins over framework CSS without specificity tricks, and chunks can be split without changing the cascade.
4. The checker validates CSS against the component's types: `[data-variant="drawr"]` is an error if `"drawr"` is not in the prop's union, `[data-slot="titel"]` if no such slot. The in-process TS7 checker already has those types. This is what makes the mapping exact rather than a naming habit.

With this convention the mapping "rule → component / variant / slot / state" does not need a side table. The `keys` mode is that mapping evaluated without DOM relations, and it came within 1.6% of the exact result (12 269 against 12 072 B brotli).

### A problem the convention does not solve: scope leak

Nested rules compile to descendant selectors, which reach into slot content. `purge/leak.mjs` found 13 (page, selector) cases where a rule of one component matches an element owned by another. One is a real bug in my own CSS: `.Prose a:not([data-part])` underlines the Pager links, because the Pager sits inside Prose. Slot names repeat across components (`title` in SideMenu, Dialog, Card and Callout; `footer` in Page, SideMenu and Card), so a Callout inside a Dialog would take the Dialog's title style.

Two fixes:

- **`@scope (.Dialog) to ([data-slot="contents"] > *)`.** Native lower boundary. Baseline newly available since Firefox 146, 2025-12-09 (https://web.dev/blog/web-platform-12-2025). esbuild parses and prints it (changelog: "Parse and print CSS `@scope` rules", #4322) and cannot lower it. In a browser without `@scope` the whole block is ignored.
- **Compiler-qualified names.** The compiler emits `data-slot="Dialog.title"` or a generated class and rewrites the selector to match. Works everywhere, costs a rewrite step.

## 5. Authoring options

| Option | What it needs from the build | Go toolchain, no Node | Status 2026 |
|---|---|---|---|
| Plain `.css` + native nesting | bundle, minify | Yes, esbuild | nesting Baseline since 2023 |
| `data-*` variants | nothing | Yes | — |
| `@layer` | nothing | Yes, esbuild handles layer order | Baseline since 2022 |
| `@scope` | nothing; cannot be lowered | Yes, pass-through | Baseline newly available 2025-12-09 |
| CSS Modules | name renaming + a name map into JS | Yes, esbuild `local-css` | stable; no unused-class removal (measured) |
| StyleX | Babel plugin, or the unofficial Rust/SWC compiler through napi | No | v0.17.1, Dec 2025, with `@stylexjs/unplugin` (https://stylexjs.com/blog/v0.17.1); `@stylexswc/rs-compiler` 0.18.6 |
| vanilla-extract | executes `.css.ts` at build time through its JS integration | No | active (https://dev.to/anber/the-state-of-zero-runtime-css-in-js-mid-2026-5ci6) |
| Panda CSS | Node CLI / PostCSS, static extraction | No | active (same source) |
| Linaria / wyw-in-js | Babel evaluation | No | maintained; new work in wyw-in-js (same source) |
| Tailwind v4 | Rust engine + JS; a standalone CLI binary exists | Only as a subprocess | 4.3.2, 2026-06-29 (Wikipedia, via search) |

CSS Modules add nothing here: the compiler already owns every name, and hashed names would get in the way of author overrides.

The typed-styles libraries exist to get two things: styles keyed by typed props, and extraction at build time. The convention above gets both from plain CSS, because the compiler executes the component and can type-check the CSS against it. UNVERIFIED: I did not build the type check.

## 6. CSS tools usable from Go

| Tool | Version checked | What it gives | Finding |
|---|---|---|---|
| esbuild public API (`api.Transform`, `api.Build`) | 0.28.2 | bundle, minify, lower, prefix. No AST | 1.3 ms for the 33 KB bundle (`minicmp`) |
| esbuild internals through a bridge package | 0.28.2 | full selector AST, parser, printer, nesting lowering | 80 lines; needs a vendored copy with one added package, the same model as the tsgo fork. Output matches the CLI |
| tdewolff/minify + parse | minify v2.24.19, parse v2.8.16 | fast minifier (0.29 ms); token-stream parser, selectors as token lists | **Breaks on nested rules that start with an attribute selector.** `.a { color: red; [data-x="y"] { margin: 0px; } .d { margin: 0px; } }` comes out with the nested blocks untouched, and the parser reported 15 of the bundle's rulesets before stopping |
| lightningcss | 1.33.0 (Rust; run through Node here) | best-in-class transforms | no better on this input: 25 637 B against esbuild's 25 577 B nested; 29 136 against 29 383 B flat |
| pgaskin/go-lightningcss | — | lightningcss wasm transpiled to Go with wasm2go; `Transform()` only, no AST | README: experimental, binaries "about 10x the size of the WebAssembly", about 12 GB RAM to compile (https://github.com/pgaskin/go-lightningcss, fetched 2026-10-04). Not tried |

Minified size of the same bundle with nesting lowered: esbuild 29 383, tdewolff 29 216, lightningcss 29 136 B. The three are within 1%.

## Recommendation

**Pipeline for phase 2**

1. Bundle each component's `.css` in a fixed layer order.
2. Parse once with esbuild's CSS parser through a bridge package in a vendored esbuild.
3. For each page, prune against the HTML the builder just emitted: exact matching, runtime conditions as "maybe", `<template>` as an open boundary. Then drop unreferenced tokens and keyframes.
4. Print with esbuild's printer, nesting kept.
5. Deliver per page: inline `<style>` by default.

**Why exact DOM matching rather than a rule → key table.** It is sound without any convention for the static part, it handles author CSS and slot content with no extra rules, and the matcher is about 360 lines, state table and multi-page modes included. The `keys` result shows a key table would be nearly as small, so if islands later hide the DOM, the fallback is cheap.

**Why inline by default.** It is the simplest output, one request, and the smallest first view (2.6–5.6 KB for HTML and CSS together). It costs about 4 KB over a three-page visit against one cached bundle. If that matters, switch the default to S5 (site-union file, 9.2 KB for three pages) or S6; the pruner's rule → page-set table supports either without new analysis.

**Authoring convention.** The one in section 4: one file per component, `data-<prop>` for variants, `data-slot` / `data-part`, state only through pseudo-classes and ARIA, `@layer` for order.

**Leave out of phase 2:** state-table pruning (3.2%, and it adds a declaration every behaviour must keep correct), atomising, CSS Modules, any CSS-in-JS library, lightningcss.

**On the bet.** For CSS it holds in relative terms and is modest in absolute terms on a site this small: 38–73% of the bundle, 1.3–3.0 KB brotli saved per page. The stronger result is correctness: the compiler can prune soundly where a generic tool demonstrably cannot.

**If vendoring esbuild is rejected** by the builder decision, the fallback is `api.Transform` to lower nesting, then a hand-written splitter and selector parser over the flat output. That loses nested output (4.7% on the guide page) and adds a selector parser to maintain. tdewolff is not a fallback for nested source.

## Open questions

1. **Scope leak.** `@scope` with a lower boundary, or compiler-qualified slot and part names? It depends on the browser floor the platform research sets; `@scope` cannot be lowered by esbuild.
2. **Delivery default.** Inline per page, or one site-union file? The numbers favour inline for a one-page visit and a shared file from three pages on.
3. **Who declares what a behaviour writes?** Needed only if state pruning is wanted. A declaration in the component, or extraction from literal arguments of a runtime `setState` helper.
4. **Is one esbuild vendored for JS anyway?** If the builder research ends with a vendored esbuild, the CSS bridge is free. If not, is an 80-line bridge worth a second vendored tree?
5. **Tokens as public API.** Dropping unreferenced custom properties (5.7%) is sound only while nothing outside the compiler's view reads them. Author raw JS or a later theme switcher would change that.
6. **`:not()` and `:is()` precision.** The pruner keeps every `:not(...)` and does not trim selector lists inside `:is()`. Both can be tightened; neither affects soundness.
7. **Browser floor for the CSS in the model.** I verified `@scope` (2025-12), anchor positioning (Firefox 147, 2026-01-13, secondary sources) and `:open` (Safari 26.5, May 2026, https://web.dev/blog/web-platform-05-2026). UNVERIFIED: `::details-content`, `closedby`, `commandfor`, `interpolate-size`.
8. **Library growth.** The claim that pruned pages stay flat while the bundle grows is reasoning, not measured; the modelled library is 87% used by the site.

## Verification (independent)

Skeptic pass, 2026-10-04. Everything below was re-run in `phase2-research/exp-css-verify/` (inputs copied from `exp-css/`, the author's `esbuild-src` used read-only through a `replace`). Tools: esbuild 0.28.2 (latest on npm, published 2026-08-08), Node 25.2.1, Go 1.27.1, PurgeCSS 8.0.0, lightningcss 1.33.0, Chrome for Testing 143.0.7499.4 (Playwright headless shell).

**Reproduction.** Regenerated pages, bundles and all 28 pruned files are byte-identical to the author's; `node measure.mjs` output is identical to `exp-css/out/measure.md` (`diff` empty). The numbers in the report are real. The corrections below are about what they mean.

### Three findings that change the report

**1. The recommended mode has a soundness bug the author's check could not see.**
`exact + tokens` drops `@keyframes Button-spin` on index, guide and api while keeping `.Button[aria-busy=true]::after{animation:Button-spin …}`:

```
index.exact      busy-rule=1 uses-anim=1 keyframes=1
index.exact-tok  busy-rule=1 uses-anim=1 keyframes=0     (same for guide, api)
```

Cause: `pruneTokens` looks for the animation name among `TIdent` tokens; esbuild does not store it as one. The linkedom cross-check skips `@keyframes` (`if (s.startsWith("@keyframes")) continue`), so "dropped but matching: 0" never covered it.

I found it with an independent check (`exp-css-verify/browser/`): one HTML file holds both the full bundle and the pruned CSS, a script applies a runtime scenario, snapshots `getComputedStyle` of every element and 6 pseudo-elements under each stylesheet, and diffs. 13 scenarios (static; dialogs, popovers and details opened; template rows cloned in; each `aria-*` value, `open`, `hidden`, `disabled` set on every element) × 2 viewports × 4 pages:

| Pruned variant | Runs | Runs with a computed-style difference |
|---|---:|---:|
| `exact` | 104 | 0 |
| `keys` | 104 | 0 |
| `exact + tokens` (the recommended one) | 104 | 6 (the spinner's `transform`, `aria-busy` scenario) |

So selector pruning is behaviour-preserving on the modelled pages in a real engine, which is stronger evidence than the linkedom check. The token/keyframes step is not.

**2. The headline numbers come from a mode the recommendation excludes.**
"38–73% of the bundle", "23–43% smaller than per-entry", the PurgeCSS comparison and table D all use `exact + tokens + states`. The recommendation leaves the state table out of phase 2. For the recommended mode, with the keyframes bug fixed (`node recm.mjs`, brotli 11):

| | index | guide | api | changelog |
|---|---:|---:|---:|---:|
| Bytes | 2138 | 3699 | 3722 | 1897 |
| Share of the one-bundle file (4892) | 44% | 76% | 76% | 39% |
| Against esbuild per-entry | −29% | −23% | −21% | −41% |

Corrected headline: **39–76% of the bundle, 21–41% smaller than per-entry.**

**3. An esbuild plugin can return different CSS per page (claim 14 is wrong as stated).**
`OnLoadArgs` indeed has no importer, but `OnResolveArgs` has `Importer` and `Kind`, and the namespace chosen in `OnResolve` reaches `OnLoad`. `exp-css-verify/plugin/main.go` puts each entry point in a namespace `page-<name>` and propagates it through `@import`:

```
index.css     12391 B  .Button-loaded-for-index
guide.css     24799 B  .Button-loaded-for-guide
api.css       23953 B  .Button-loaded-for-api
OnLoad calls for Button.css: 3  tokens.css: 3
```

One build, the same `ds/Button.css` loaded three times with different content. This does not remove the need for a selector AST (the public API still exposes none), so the bridge argument stands. It does mean "pruning has to happen outside esbuild's bundle step" is a choice, not a constraint.

### Verdict per claim

| # | Claim | Verdict | What I checked |
|---|---|---|---|
| 1 | esbuild emits one full CSS bundle per entry, no CSS splitting | Confirmed | Re-ran: 12356 / 24764 / 23920 / 13444 B, four 0-byte chunks, shared part 9832 B. Docs quote matches (fetched 2026-10-04). Issue #608 is closed, opened 2020-12-17; it is about this limitation |
| 2 | No unused-CSS elimination, CSS modules included | Confirmed | `.o{…}.r{color:green}.keep{…}`; `.never-used` survives |
| 3 | 80-line bridge, output equals the CLI | Confirmed | Differences are the `✓` escape and a trailing newline, nothing else. The bridge pulls 12 internal esbuild packages, 33 490 lines of non-test Go (`go list -deps ./rgcss`), including `js_ast`, `config`, `logger`, `fs`. It is 80 lines on top of a full vendored tree, with no API stability |
| 4 | Pruned CSS is 38–73% of the bundle, 23–43% under per-entry | Partly | Reproduced, but for the excluded `states` mode. Recommended mode: 39–76% and 21–41% (finding 2) |
| 5 | Where the gain comes from (−22% / −1.6% / −5.7% / −3.2%) | Confirmed | Reproduced. The tokens step includes the unsound keyframes drop: fixed sum is 11 456, so that step is −5.1% |
| 6 | Three pages: bundle 9604 B beats inline 13 782 B; one page: inline wins | Partly | Reproduced, with `states` CSS. Recommended mode: 14 172 against 9604. "Two pages roughly even" holds only when one page is the landing page: guide → api is 11 255 inline against 8776 linked |
| 7 | Exact pruner sound on the modelled pages for static conditions | Partly | Cross-check reproduces (1072 / 468 / 0). Chrome diff agrees for `exact` and `keys`. Not true of the recommended `exact + tokens` (finding 1), and the pruner has general bugs the model does not exercise (below) |
| 8 | PurgeCSS within 2.5–18.5%, unsound and imprecise | Partly | Reproduced. Against the recommended mode PurgeCSS is 3.6% *smaller* on index, 0.7–0.8% larger on guide and api, 17% larger on changelog. With `dynamicAttributes` (10 names) and a two-entry safelist it keeps every selector ours keeps and costs +3.5% / +4.0% / +4.5% / +22.7%. Unsound by default, not inherently. Imprecision from matching words in text is inherent |
| 9 | Parse 1.2 ms, prune 0.2–0.8 ms per page | Confirmed | Same range. Scales linearly: 1676 elements 4.4 ms, 8356 elements 15.3 ms |
| 10 | tdewolff cannot parse nested component CSS | Confirmed | Re-ran the binary: nested blocks passed through, `BeginRuleset:15`. v2.24.19 and v2.8.16 are the newest on the Go proxy today |
| 11 | lightningcss not smaller than esbuild; Go route experimental | Partly | Nested 25 637 reproduced. On flat output the report's own figure has lightningcss *smaller* (29 136 against 29 383). All within 1%, so the conclusion holds. go-lightningcss README says what is claimed; not built |
| 12 | Atomising not worth it | Confirmed | `node atoms.mjs` reproduces. It is a rough bound, as the report says |
| 13 | 13 scope leaks; `@scope` or qualified names needed | Partly | 13 reproduces, but it over-counts. `.Prose>*+*` spacing child components is intended; `.Dialog [data-slot=contents]` "leaks" only because the slot element is also the Prose root. The Pager underline is the one real bug. `@scope` facts confirmed (web.dev, Firefox 146; esbuild changelog #4322; printed unchanged under old targets) |
| 14 | An `OnLoad` plugin cannot return per-page CSS | Refuted | Finding 3 |
| 15 | No CSS-in-JS / Tailwind without Node or a subprocess | Confirmed | Versions are stale: StyleX post is dated 2025-11-25, StyleX is 0.19.1 (2026-09-15), Tailwind 4.3.3 (2026-09-25). I checked npm metadata and the StyleX post only |
| 16 | Docs sites ship 14–58 KB brotli of linked CSS | Confirmed | `node baseline.mjs` today gives identical numbers. Starlight also inlines 12 230 B of `<style>` that the table omits |

### Pruner bugs the model does not exercise

`exp-css-verify/adv/t1.css` + `t1.html`, confirmed in Chrome 143 with the same diff harness:

| Input | Pruner (`exact`) | Browser |
|---|---|---|
| `.A { &:is(.x, .y) { color: red } }` on `<div class="A x">` | dropped | matches. Arguments of `:is()` inside a nested rule are treated as descendants of the parent rule. The model's only `:is()` works by coincidence |
| `[data-v="Primary" i]` on `data-v="primary"` | dropped | matches (`i` flag ignored) |
| `input[type="CHECKBOX"]` on `type="checkbox"` | dropped | matches (HTML compares `type` values case-insensitively) |
| `.list:has(.row)`, `.list > .row ~ .foot2` where `.row` exists only in a `<template>` | dropped | match once a row is cloned into `.list` |
| `@layer reset{.gone{}} @layer components{.A{color:red}} @layer reset{.A{color:blue}}` with no order statement | first block removed | layer order flips, `.A` goes red → blue |
| `--theme: dark` read only by `@container style(--theme: dark)` | token dropped, query kept | not run in the browser |

Consequences for the design:

- **"`<template>` as an open boundary" is one-directional.** It protects the cloned element's own rules. It does not protect a static element whose match depends on cloned content (`:has()`, `+`, `~`). The rule needs to be: any selector with a compound that can match template content is "maybe" for the whole selector.
- **The `@layer` order statement is a soundness precondition**, not a style preference. The modelled design system contains no `@layer` at all (`grep layer ds/*.css` is empty), so step 1 of the pipeline and the claim that chunks can be split without changing the cascade are untested.
- `keys` mode happened to keep the `:is()` and template cases and dropped the two case-insensitive ones.

### What the report did not cover

- **Docs-to-docs navigation.** Inline is recommended as the default, but a reader moving between two docs pages already pays 2.5 KB more than with one linked file (11 255 against 8776), because each docs page needs 76% of the bundle. With revisits (guide, api, guide, api) it is 22 510 against 12 660. The amounts are small either way; the default should be chosen for simplicity, not on these bytes.
- **Name mangling was not measured.** The compiler owns every slot, part, component and token name. A crude rewrite to short names (`node mangle.mjs`; byte counts only, output not validated) takes HTML + inline CSS down another 7.6–8.6% brotli (240–460 B per page). That is more than exact matching (1.6%), tokens (5.1%) or states (3.2%) individually, and it is the kind of gain only a component-aware compiler gets. It conflicts with author overrides by plain name, which is why it needs a decision.
- **The bet is tested in its least favourable case.** The four pages use 87% of the library. The report says so (open question 8); the consequence is that phase 2 cannot confirm or refute "ridiculously optimize" for CSS from this model. A library of realistic size is needed.
- **A runtime theme switch** would write `data-theme` or a class on `<html>`. The convention classifies every `data-*` as compile-time. The model avoids this with `light-dark()` and no toggle.
- **PurgeCSS's real disqualifier is not soundness.** It is a Node/PostCSS tool that needs nesting lowered first, so it cannot live in the Go binary.
- **Cross-check independence.** The linkedom check shares the pruner's assumption about which conditions are runtime and skips keyframes and tokens. A computed-style diff in a browser (about 1 s per page and scenario) should be the conformance test for the pruner.

### Does the recommendation hold?

Yes for the core: prune as the builder's own step, after HTML emission, on esbuild's CSS AST. No other Go-reachable parser handles nested CSS, and selector pruning held up in a real browser.

With these corrections:

1. Fix the keyframes lookup, `:is()` under nesting, the `i` flag and the template rule before trusting the output; gate the pruner with the browser diff.
2. Require the `@layer` order statement, or keep emptied layer blocks.
3. Quote 39–76% and 21–41% for the recommended mode.
4. Treat inline-by-default as open; for a docs site the site-union file is at least as defensible.
5. Drop "an esbuild plugin cannot do this" as an argument. The reason to prune outside the bundle step is the AST, not the plugin API.
6. The bridge costs a second vendored tree (33 k lines of esbuild internals in its import closure). That only pays if the JS builder vendors esbuild anyway.
