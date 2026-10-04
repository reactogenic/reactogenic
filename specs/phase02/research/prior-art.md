# Prior art: compilers that know their components

Phase 2 research, key `prior-art`. Written 2026-10-04. All measurements were run on 2026-10-04;
experiment files are in `exp-prior-art/` next to this report.

## 0. Findings in short

1. **Nobody wrote their own JS linker.** Astro, Marko, Qwik, SvelteKit, SolidStart and Fresh all keep
   a general bundler (Rollup/Rolldown/esbuild) for module linking, tree-shaking, splitting and
   minifying. What they add is a per-file compiler **in front** and a build driver **around** it.
2. **A transform plugin alone was never enough.** Every one of them needs (a) several ordered builds,
   because the client entry points are only known after the server side has been compiled or
   rendered, and (b) to emit the HTML itself and inject asset URLs from a manifest.
3. **The best existing result for our test page is Marko 6: 3.7 kB raw / 2.0 kB brotli of JS per
   page**, with a stateful dropdown, sidebar and dialog. A hand-specialised version of the same
   behaviour is 0.4 kB raw / 0.23 kB brotli. A platform-only version (popover, invoker commands,
   `<details>`) is 0 B, and Marko 6 also emits 0 B for it.
4. **So the bet holds, with a caveat about size.** Against React-based docs sites the gap is two to
   three orders of magnitude (Docusaurus 241 kB, react.dev 229 kB, nextjs.org/docs 517 kB brotli of
   eager JS). Against the best compilers the remaining JS headroom is about 1.7 kB brotli per page.
5. **The larger untapped saving is CSS.** Real docs sites ship 24–80% CSS that matches nothing on the
   page (5–25 kB brotli per page). No framework I found prunes CSS per page at design-system
   variant granularity.

## 1. Method

- Web sources: fetched 2026-10-04; URL and publication date given with each claim.
- Own builds: the same 3-page docs site (sidebar on every page, dropdown on every page, modal
  dialog only on `/`) written idiomatically in Marko 6, Astro 7, SvelteKit 3 / Svelte 5, Qwik 1.20,
  Eleventy 3 + WebC, plus two hand-written floors.
- Measuring tool: `exp-prior-art/measure.mjs`. It sums external scripts, inline scripts,
  `modulepreload`s and the static import graph of modules. gzip level 9 and brotli quality 11 are
  computed per file and summed. **Dynamic `import()` targets are not followed**, so figures are the
  eager lower bound.
- Not done: no page was opened in a browser (no browser automation available). Sizes are measured;
  **runtime behaviour of every build, including the two floors, is UNVERIFIED.**
- Not built: Fresh (no Deno on the machine), SolidStart, Enhance, Leptos, Dioxus.

Versions used: marko 6.4.1, @marko/run 0.11.13, astro 7.3.5 (@astrojs/compiler-rs 0.5.1),
svelte 5.57.1, @sveltejs/kit 3.0.0, @builder.io/qwik 1.20.1, @11ty/eleventy 3.1.6,
@11ty/eleventy-plugin-webc 0.11.2, vite 8.3.2 (rolldown 1.2.12; Qwik starter pins vite 7.3.1),
esbuild 0.28.2, purgecss 8.0.0.

## 2. Measured: the same page in six implementations

Page `/` = layout (dropdown + sidebar) + modal. Page `/guide` = layout only. JS = external + inline.
Commands: `pnpm run build` in each directory, then
`node measure.mjs "<dist>::/index.html" "<dist>::/guide.html"`.

| Implementation | JS on `/` raw / gz / br | JS on `/guide` raw / br | HTML `/` raw | What the JS is |
| --- | --- | --- | --- | --- |
| Platform only, hand-written (`floor/dist-platform`) | **0** | 0 | 2,158 | none: `popover` + anchor positioning, `commandfor`/`command`, `<details>` |
| Marko 6, same platform-only components (`marko/dist-platform`) | **0** | 0 | 1,473 | none; no `<script>` tag emitted |
| Hand-specialised JS (`floor/dist-specialised`) | **413 / 311 / 227** | 272 / 170 | 2,213 | 249 B layout chunk + 164 B modal chunk, `getElementById` + handlers |
| Astro 7, hand-written `<script>` per component | 720 / 535 / 402 | 485 / 290 | 3,688 | three inlined module scripts, `querySelectorAll` loops |
| Eleventy + WebC, custom elements (unminified) | 1,136 / 396 / 299 | 813 / 255 | 3,866 | per-page bundle of the components used |
| **Marko 6, state-driven components** | **3,683 / 2,170 / 1,972** | 3,574 / 1,904 | 2,015 | 3,119 B shared chunk (runtime + dropdown + sidebar effects), 191 B modal chunk, 373 B inline resume data |
| Qwik 1.20 (Qwik City, SSG) | 57,876 / 24,690 / 22,139 | 57,860 / 22,141 | 5,401 | loader + preloader + 50 kB core `modulepreload`ed; handler chunks load on interaction |
| SvelteKit 3 + Svelte 5 (prerendered, default `csr`) | 85,069 / 33,404 / 30,131 | 84,337 / 29,782 | 3,107 | router + runtime + every component re-created for hydration |

Observations taken from the emitted files:

- **Marko ships behaviour only.** Its 3,119 B shared chunk contains no markup and no static text.
  The dropdown's entire client code is:
  `var N=T(8,e=>{k(e.a,"aria-expanded",e.i),k(e.c,"hidden",!e.i)});E("c0",e=>{b(e.a,"click",function(){N(e,!e.i)}),…})`.
  The `label="Version"` prop appears nowhere in JS: the compiler saw that it is never stateful.
- **Marko splits per page by use.** `index-*.js` (191 B) holds the two modal handlers; `/guide`
  loads an 82 B entry that only starts the resume. Modal CSS (100 B) is a separate file linked only
  from `/`.
- **Marko's remaining cost is generic machinery**: a scheduler with a binary heap, an event
  delegation table, a `createTreeWalker` pass over `<!--M_$3 a-->` comment markers, and a serialised
  scope table (`M._.r=[…{i:!1}…]`). Measured split of the 3,081-character chunk: 2,753 generic
  runtime, 299 for the dropdown and the sidebar together, 29 for the export list.
- **Svelte duplicates the page in JS.** `nodes/2.*.js` contains
  ``o(`<h1>Introduction</h1> <p>Reactogenic apps are built from…`)`` — the same text that is already
  in the HTML. Hydration re-creates every component, static or not.
- **Qwik's HTML carries the bookkeeping**: 5,401 B against 2,015–2,213 B, from `q:id`, `on:click`
  QRLs, `<!--qv …-->` comments and a `qwik/json` block. (Qwik 2.0 RC, 2026-10-01, replaces the
  comments with "a single encoded string at the end of the HTML" —
  https://next.qwik.dev/blog/qwik-2-rc/.)
- **Astro inlined everything**: zero JS requests, zero CSS requests on both pages.

## 3. Measured: live documentation sites

`node measure.mjs <url>` on 2026-10-04. Eager first-party JS (external + inline), third-party
analytics excluded. Raw output: `exp-prior-art/live.txt`.

| Site (page) | Built with | Eager JS raw | Eager JS br | CSS raw / br |
| --- | --- | --- | --- | --- |
| nuejs.org/docs/ | Nue 2 | 5.8 kB | 2.4 kB | 7.6 / 2.2 kB |
| starlight.astro.build/getting-started/ | Astro 7.2.10 + Starlight 0.42.5 | 18.3 kB | 7.7 kB | 91.3 / 17.0 kB |
| markojs.com/docs/explanation/fine-grained-bundling | Marko 6 | 26.1 kB | 10.8 kB | 44.5 / 7.7 kB |
| 11ty.dev/docs/ | Eleventy | 34.7 kB | 9.5 kB | 79.7 / 17.3 kB |
| brisa.build/getting-started/quick-start | Brisa | 67.0 kB | 20.6 kB | 24.1 / 5.7 kB |
| qwik.dev/docs/advanced/optimizer/ | Qwik | 73.9 kB (+123 kB inline state) | 28.0 kB (+18.8 kB) | 76.4 / 16.1 kB |
| docs.astro.build/en/concepts/islands/ | Astro + Starlight | 138.0 kB | 32.2 kB | 107.9 / 20.2 kB |
| vitepress.dev/guide/what-is-vitepress | VitePress | 265.6 kB | 72.1 kB | 199.8 / 33.8 kB |
| svelte.dev/docs/svelte/overview | SvelteKit | 278.2 kB | 84.7 kB | 111.9 / 19.3 kB |
| docs.solidjs.com/concepts/intro-to-reactivity | SolidStart | 357.7 kB | 82.2 kB | 113.8 / 16.4 kB |
| react.dev/learn | Next.js | 858.9 kB | 228.8 kB | 108.6 / 16.2 kB |
| docusaurus.io/docs | Docusaurus | 1,358 kB | 240.9 kB | 173.5 / 32.3 kB |
| nextjs.org/docs | Next.js | 2,065 kB | 516.7 kB | 484.8 / 61.7 kB |

Notes:

- Starlight is the closest existing analogue of the phase 2 target: sidebar (`<details>` groups, 6
  on the page), a search modal (`<dialog>` in a `<site-search>` custom element), dropdowns
  (`<select>` in `<starlight-theme-select>`). Its 18.3 kB excludes the Pagefind UI chunk that the
  search element imports on idle: 68.1 kB raw / 16.7 kB br.
- 45% of Starlight's 2,938 B search chunk (1,323 B) is Vite's generic dynamic-import preload helper,
  not Starlight code (`exp-prior-art/starlight-search.js`).
- enhance.dev/docs/ measured 1.73 MB of first-party JS, but 1.54 MB is a video player and 183 kB is
  DocSearch; Enhance's own element scripts on the page are 6.2 kB. Not representative.
- usefresh.dev/docs/concepts/islands measured 187 kB raw / 53 kB br. Whether that domain is the
  official Fresh site is UNVERIFIED (fresh.deno.dev/docs returned 404).

### CSS that matches nothing on the page

`exp-prior-art/purge/purge.mjs`: download the page's stylesheets and inline styles, run PurgeCSS 8
against that page's own HTML, minify both with Lightning CSS. This is a static approximation: rules
for classes that JS adds later are counted as unused, and PurgeCSS's word-based extractor keeps
anything whose tokens appear anywhere in the HTML.

| Site | CSS br before | After purge | Unused (br) |
| --- | --- | --- | --- |
| vitepress.dev | 33.5 kB | 7.6 kB | 77% |
| docusaurus.io | 32.0 kB | 8.8 kB | 73% |
| react.dev | 15.9 kB | 5.2 kB | 67% |
| docs.solidjs.com | 15.9 kB | 7.3 kB | 54% |
| svelte.dev | 15.6 kB | 8.7 kB | 44% |
| starlight.astro.build | 14.6 kB | 9.5 kB | 35% |
| markojs.com | 6.9 kB | 5.3 kB | 24% |
| qwik.dev | 13.4 kB | 11.7 kB | 12% |
| nuejs.org | 2.1 kB | 1.8 kB | 12% |

On Starlight the unused CSS (5.1 kB br) is two thirds of all the eager JS on the page (7.7 kB br).

### Published numbers

I found no primary-source figure for "JS per page on a docs page with sidebar, modal and dropdown".
What exists is for other workloads:

| Source, date | Workload | Numbers |
| --- | --- | --- |
| Loren Stewart, "I Built the Same App 10 Times", 2025-10-28, https://www.lorenstew.art/blog/10-kanban-boards/ | kanban app, Lighthouse, gzip | homepage raw / compressed: Marko 12.4 / 6.8 kB; Astro + HTMX 86.9 / 21.5; SolidStart 83.9 / 29.8; Qwik City 86.5 / 42.5; SvelteKit 103.4 / 47.8; Next.js 16 486.1 / 150.9 |
| Leptos book, islands chapter, https://book.leptos.dev/islands.html (undated) | wasm binary, uncompressed | static page 24 kb with islands vs 274 kb without; one counter island 166 kb vs 355 kb hydrated |
| Brisa announcement, 2024-10-05, https://brisa.build/blog/introducing-brisa | framework's own claim | 0 B by default; 3 kB once a web component is used; 2 kB RPC for server actions |
| Meta, 2020-05-08, https://engineering.fb.com/2020/05/08/web/facebook-redesign/ | facebook.com homepage CSS | "more than 400 KB of compressed CSS… only 10 percent of that was actually used"; atomic CSS: "less than 20 percent of the CSS the old site downloaded" |
| Tamagui docs, https://tamagui.dev/docs/intro/why-a-compiler (undated) | own site | "30-50% of components typically flatten"; Lighthouse about 80 to 95 with the compiler on |

## 4. Per framework

### What the compiler knows, and what follows

| Framework | Knows that a bundler cannot | Optimisations that follow | Status |
| --- | --- | --- | --- |
| **Astro 7** | which components are server-only (everything not marked `client:*`); each component's `<style>` and `<script>` | server-only components leave no JS; styles scoped by `data-astro-cid-*`; scripts bundled, deduped per page, inlined when small; CSS chunked per page, inlined under 4 kB | verified in my build and https://docs.astro.build/en/guides/styling/, /guides/client-side-scripts/ |
| **Marko 6** | which values are stateful, across templates (child-template analysis); which DOM nodes a state change touches | only handlers and state-to-DOM effects ship; non-stateful props generate no client code; layouts with no interactivity stay out of the client bundle; resumes instead of hydrating; no client assets at all for a page without interactivity | verified in my build; https://markojs.com/docs/explanation/fine-grained-bundling, newsletters May/June/August 2026 |
| **Qwik** | closure boundaries at every `$`; which variables each closure captures | each handler extracted to a lazily loaded symbol; captures serialised into HTML; no hydration | verified in my build; https://qwik.dev/docs/advanced/optimizer/ |
| **Svelte 5** | the template structure and which expressions are reactive; which selectors a component's markup can match | templates hoisted to HTML strings and cloned; fine-grained updates without a VDOM; scoped styles with a hash class | verified in my build output; https://svelte.dev/docs/svelte/scoped-styles. Does **not** skip static subtrees at hydration |
| **Solid** | same as Svelte, from JSX | `template()` + `cloneNode`, event delegation, separate DOM and SSR outputs | dom-expressions README, https://github.com/ryansolid/dom-expressions. SolidStart: `experimental.islands` "only accepts false" (https://docs.solidjs.com/solid-start/v2/reference/config/solid-start, via search, 2026) — not built here |
| **Fresh 2** | nothing from a compiler: islands are found by directory convention (`islands/`) | only island modules are bundled for the client; props serialised; "Passing functions to an island is not supported" | https://usefresh.dev/docs/concepts/islands; source `packages/fresh/src/dev/builder.ts` |
| **Enhance** | custom-element templates expanded on the server | HTML-first; JS only for elements that define a client script | https://enhance.dev/docs/ — details of delivery UNVERIFIED |
| **Eleventy + WebC** | which components were rendered on this page | per-page CSS and JS bundles of only those components; a component with no style or script leaves no host tag ("zero overhead HTML") | verified in my build; https://www.11ty.dev/docs/languages/webc/ |
| **Leptos** | `#[island]` macro marks what compiles to wasm | binary grows "as a function of the amount of interactivity", not app size | https://book.leptos.dev/islands.html |
| **Tamagui** | a design system's components and tokens | flattens styled components to `div`, partial evaluation, atomic CSS extraction, `useMedia`/`useTheme` turned into media queries and CSS variables | https://tamagui.dev/docs/intro/why-a-compiler |
| **StyleX** | style objects are statically resolvable by construction | atomic classes, deduplication, CSS size plateau | https://engineering.fb.com/2025/11/11/web/stylex-a-styling-library-for-css-at-scale/ (2025-11-11) |

Newcomers checked (2025–2026):

- **Nue 2.0** (2025-10-14, https://nuejs.org/blog/2.0/): Bun-only; a file "becomes dynamic" if it
  "has event handlers or imports", otherwise renders server-side with no JS. Smallest live docs
  page measured (5.8 kB JS, 7.6 kB CSS). It gets there with a small global design system and
  hand-written CSS, not with compiler analysis.
- **Brisa** (0.2.x): server components by default, web components with signals when marked;
  claims key-level i18n pruning for client components.
- **Remix 3** (beta 5, 2026-07-01, via https://www.infoq.com/news/2026/07/remix-3-beta-preview/):
  explicitly no custom bundler or compiler. A counter-example, not prior art for us. Secondary
  source only.
- **Vue 3.6 Vapor mode**: still a release candidate in September 2026 according to secondary
  sources; same family as Svelte/Solid (no VDOM, still hydrates everything). UNVERIFIED from
  primary sources.
- **shadcn-style libraries without React** using invoker commands (for example
  `supermomonga/shadcnui-hono-jsx`, issue about fallbacks, 2026) and **daisyUI**
  (https://daisyui.com/components/dropdown/: three no-JS dropdown methods — `<details>`, popover +
  anchor positioning, CSS focus). These are hand-authored platform-only components, not compilers.
- I found **no 2025–2026 newcomer that executes a closed component set at build time and emits
  specialised framework-free JS**. The nearest are Marko 6 (generic runtime, 3 kB) and Astro
  (author writes the vanilla JS).

### Where each sits relative to the bundler

| Framework | In front (per-file) | Around (driver) | After (on bundler output) | Bundler |
| --- | --- | --- | --- | --- |
| Astro 7 | `.astro` → JS module by `@astrojs/compiler-rs` (Rust, native binaries, wasm fallback) inside a Vite transform | `buildApp` runs three environments in order: prerender, ssr, client; "MUST be built after SSR/prerender because client inputs are discovered during those builds"; then `generatePages` writes the HTML | `generateBundle` walks chunk parents "until you find a page" to assign CSS; manifest injected into chunks after the build | Vite 8 / Rolldown |
| Marko 6 | each `.marko` compiled three ways: `output: "html"`, `"dom"`, `"hydrate"` | linked mode: "automatically discovers all of the entry `.marko` files while compiling the server, and tells Vite which modules to load in the browser"; uses `buildApp`; server manifest handed to the client build | `generateBundle` repairs Vite's CSS chunk pruning for lazily loaded entries; overrides tree-shaking so that "Marko is the only source of side effects" | Vite / Rolldown |
| Qwik 1.x | Rust optimizer (`transform_modules`) called from `transform`; each extracted segment becomes a virtual module via `resolveId` / `load` | client build first, then SSR: "The SSR build requires the manifest generated during the client build" | `emitFile` per segment, `manualChunks` to group symbols, manifest and bundle graph written in `generateBundle` | Vite / Rollup |
| SvelteKit 3 | `.svelte` → JS in a transform plugin | server build, client build, then prerender | — | Vite 8 / Rolldown |
| Fresh 2 (built-in builder) | none | crawl `islands/`, build `entryPoints: Record<name, spec>`, call `esbuild.build({ entryPoints, splitting: true, metafile: true })`, map entries to chunks from the metafile | — | esbuild (or the optional Vite plugin) |
| Eleventy + WebC | WebC renders the page and collects component CSS/JS as it goes | Eleventy writes HTML with `getBundle("css")` / `getBundle("js")` | — | none: concatenation |

Sources: `withastro/astro` `packages/astro/src/core/build/static-build.ts` and `plugins/plugin-css.ts`
(main, fetched 2026-10-04); `marko-js/vite` `src/index.ts` (main) and README;
`@builder.io/qwik@1.20.1/dist/optimizer.mjs` (lines 1452, 2436–2705, 2792–2871, 3012) and
https://qwik.dev/docs/advanced/vite/; `denoland/fresh` `packages/fresh/src/dev/esbuild.ts` and
`builder.ts` (main). Copies in `exp-prior-art/src-refs/`.

### What a bundler plugin could not do

| Need | Who | How they solved it |
| --- | --- | --- |
| Entry points of build B come from the result of build A | Astro, Marko, Qwik, SvelteKit, SolidStart | own driver; since Vite 6 the Environment API's `buildApp` |
| The same source compiled to different outputs for server and browser | Marko (three outputs), Svelte, Solid, Qwik | query suffixes or per-environment transforms |
| Emit HTML with the right asset URLs | all | render pages after bundling, read a manifest; none use the bundler's HTML handling |
| Assign CSS to pages | Astro, Marko | post-processing in `generateBundle`; Astro's plugin is 676 lines of module-graph walking and deduplication |
| Create chunks the source never names as modules | Qwik | `emitFile` + `manualChunks` — Rollup-only hooks |
| Runtime that finds code for an event | Qwik (loader + manifest), Marko (resume ids) | shipped runtime |

esbuild's plugin API is `onResolve`, `onLoad`, `onStart`, `onEnd`, `onDispose`; "It's not possible
to hook into every part of the bundling process. For example, it's not currently possible to modify
the AST directly" (https://esbuild.github.io/plugins/). Qwik's approach therefore has no esbuild
equivalent as a plugin. Fresh shows the form that does fit esbuild: generate entry points, call the
build API, read the metafile.

## 5. "Ridiculous" optimisations that need a closed component set

### Proven in the wild

| Optimisation | Where | Evidence |
| --- | --- | --- |
| Zero JS for a page or component with no interactivity | Astro, Marko 6, Fresh, Brisa, WebC, Leptos islands | my Marko platform build: no script tag; Marko May 2026 newsletter: "A page that renders no interactive components now emits no client assets at all" |
| Ship handlers and effects, never templates | Marko 6 | my build: 3,119 B chunk without markup |
| Props that are never stateful produce no client code | Marko 6 | my build: `label` absent from JS |
| Behaviour chunked per page by actual use | Marko 6, Astro, WebC | my builds: modal code only on `/` |
| CSS per page by component use | Astro, Marko, WebC | my builds: modal CSS only on `/` |
| Unused-selector removal inside a component | Svelte | Svelte docs and compiler warnings |
| Usage-only atomic CSS that plateaus | StyleX, Tailwind 4, Panda ("named classes only for the variants it can see") | Meta 2020 and 2025 posts: 80% reduction |
| Flattening design-system components and partially evaluating their styles | Tamagui | "30-50% of components typically flatten" |
| Inlining below a size threshold | Astro | docs: styles under 4 kB inlined; my build: 0 requests |
| Handlers as lazily loaded chunks | Qwik | my build; costs a 50 kB core |
| Platform-only dialog, popover, disclosure with no JS | daisyUI and similar, hand-authored | daisyUI docs |
| Only the i18n keys used by client components shipped | Brisa | framework's own claim, not measured |

Platform status on 2026-10-04 (secondary sources, see open questions): invoker commands
(`commandfor` / `command`) Baseline after Chrome 135, Firefox 144, Safari 26.2
(https://www.infoq.com/news/2026/01/html-invoker-commands); CSS anchor positioning Baseline newly
available since Firefox 147, January 2026 (https://web.dev/blog/web-platform-01-2026);
`<details name>` Baseline 2024; `<dialog closedby>` still limited availability.

### Speculative (not found in any framework)

| Optimisation | Measured headroom | Risk |
| --- | --- | --- |
| **Per-page CSS at variant granularity**: emit only the rules of the variants, sizes and states the page's component instances use | 24–80% of CSS on real docs sites, 5–25 kB br per page | needs CSS authored so variant → rules is mechanical; JS-toggled states must be declared |
| **No resume data and no DOM markers**: the shell has no conditionals, so every node address is a compile-time constant | Marko pays 373 B inline plus comment markers and a tree walk; Qwik's HTML is 5,401 B against 2,213 B for the specialised floor | none found; follows from shell rules S2 and S5 |
| **Behaviour specialised per use site**: no generic runtime, handlers bound to known nodes | 3,683 B → 413 B raw (9×), but only 1.7 kB br absolute | per-instance code grows with instance count; a tiny shared helper wins past a few instances |
| **Feature-level elimination by use-site props**: a `Dialog` without `onClose` or focus options drops to platform-only | 0 B vs 164–191 B for the modal | the design system has to be written feature by feature |
| Class-name mangling across HTML, CSS and JS | small after brotli | hurts view-source readability, which layout.md values |

## 6. Lessons for Reactogenic, ranked

1. **Use the bundler as a linker, driven through its API; do not write a linker and do not try to
   live inside a plugin.** Evidence: all six frameworks; Fresh's builder is the minimal shape and
   already uses esbuild that way; esbuild's plugin API has five callbacks and no chunk hooks.
   For us: the Go compiler renders pages, writes one JS entry per page, calls esbuild's Go API with
   `splitting` and `metafile`, then writes the script tags itself.
2. **Derive each page's assets from what was rendered, not from the import graph.** Evidence: WebC
   does this in a few lines; Astro reconstructs it from chunks with 676 lines of heuristics and
   Starlight still ships 35% unused CSS. Our compiler executes the layout components, so it can
   record per page: component, variant, which slots were filled.
3. **Shell components must separate markup from behaviour, and only behaviour may reach the
   browser.** Evidence: Marko 3.7 kB vs SvelteKit 85 kB on the same page; Svelte's page text is in
   both HTML and JS. `layout.md`'s `mountDialog(root, slots)` is already this shape.
4. **Write the three components platform-first.** Evidence: 0 B measured for popover + invoker
   commands + `<details>`; all three reached Baseline by January 2026. JS becomes an option per
   feature, which is where compiler knowledge of use-site props pays.
5. **Skip resumability machinery.** Evidence: 2,753 of the 3,081 characters in Marko's shared chunk
   are scheduler, marker walk and delegation; our shell has no conditionals, so ids or child paths
   fixed at compile time replace all of it (413 B floor).
6. **Put the phase 2 "ridiculous" budget into CSS.** Evidence: the purge table; CSS exceeds JS on
   every lean docs site measured (Starlight 17.0 kB vs 7.7 kB br). Constraint borrowed from StyleX:
   design-system styles must be statically resolvable.
7. **Inline under a threshold and skip shared chunks at this scale.** Evidence: Astro inlines under
   4 kB; our whole per-page JS is under 0.5 kB, smaller than one request's overhead.
8. **Do not adopt lazy handler loading for the shell.** Evidence: Qwik 22 kB br eager against
   Marko's 2 kB on this page; it pays off for large apps, not for three small components.
9. **Watch for generic bundler boilerplate in tiny chunks.** Evidence: 45% of Starlight's search
   chunk is Vite's preload helper. Check what esbuild adds around `import()` and split chunks before
   relying on it for sub-kilobyte outputs.
10. **Go is not the problem Astro had.** Astro's Go compiler was distributed as wasm; Astro 7
    (2026-06-22) moved to Rust "native binaries… with a WASM fallback" on oxc and Lightning CSS, and
    reports the compiler alone as "roughly 6%" of the build-time gain. We already ship native Go
    binaries. The post does not state why Go was dropped, so treat this lesson as weak.

## Recommendation

Build the phase 2 builder as **compiler in front, own driver around, esbuild as linker in the
middle** — the shape every prior framework converged on — with one difference that our constraints
allow: the compiler executes the closed component set and therefore decides HTML, CSS and JS per
page directly, with no runtime and no resume data.

Concretely:

1. The Go compiler renders each page and records which components, variants and features were used.
2. It emits HTML and per-page CSS itself. Nothing in prior art suggests handing either to a bundler.
3. Behaviour is a small raw-JS module per shell component feature; the compiler writes a per-page
   entry that imports only what the page's instances need, and esbuild's Go API links and minifies
   it. Under a size threshold the result is inlined.
4. The three components are designed platform-first, so the base case is 0 B of JS.

Targets to hold the bet against, from the measurements: **JS per page ≤ 0.5 kB raw for the stateful
variant and 0 B for the platform variant; no unused CSS rules on any page.** The reference points
to beat are Marko 6 (3.7 kB raw JS) and Starlight (18.3 kB JS, 35% unused CSS).

State the bet honestly in the phase 2 write-up: compared with React-based docs tooling the saving
is 100× or more, but Astro and Marko already capture most of that. What component ownership adds on
top is roughly 2 kB of JS and 5–25 kB of CSS per page (brotli), plus React-style authoring.

## Open questions

1. Do the two floor variants behave correctly in browsers? Nothing here was run in a browser. In
   particular: anchor-positioned popover placement, `closedby` (limited availability), focus
   handling, and `ariaExpanded` reflection.
2. How is design-system CSS authored so that variant → rules is mechanical: atomic classes
   (StyleX-style), one rule block per variant, or `@scope`? This decides whether lesson 6 is cheap.
3. Which component states are toggled by JS at runtime (`.collapsed`, `[open]`)? They must be
   declared, or per-page CSS pruning removes rules that are needed after interaction.
4. Per-use-site code or a tiny shared helper? Specialised code grows with instance count (a page
   with 20 dropdowns). Where is the crossover, and does the compiler choose per page?
5. Does esbuild add wrapper code to split chunks or `import()` that matters at sub-kilobyte sizes?
   Lesson 9 was observed on Vite/Rolldown only.
6. Platform-first accessibility: do popover + `role="menu"` and `<details>` as a sidebar meet the
   keyboard behaviour we want (arrow keys, typeahead), or is JS needed for that in every case?
7. Baseline dates for invoker commands and anchor positioning came from secondary sources and
   should be confirmed against MDN/web.dev before the design system depends on them.
8. Why Astro left Go is not stated in the release post; if it matters for our own choice, ask in the
   Astro repository rather than infer.
9. Fresh, SolidStart and Enhance were not built; their rows rest on documentation and source only.

## Verification (independent)

Skeptic pass, 2026-10-04. Files: `exp-prior-art-verify/` next to this report. The author's text
above is unchanged. Everything below was re-run or re-fetched by me on 2026-10-04.

**Outcome: 9 claims confirmed, 6 partly, 0 fully refuted. The recommendation holds with five
amendments** (end of this section). The measured numbers all reproduce. The corrections are about
what the numbers mean.

### What I ran

| Check | Command / source | Result |
| --- | --- | --- |
| Rebuild Marko from the author's sources in a separate directory | `marko-rebuild/`: `pnpm run build` | byte-identical output: 3,119 + 191 + 82 + 80 B JS, 778 + 100 B CSS |
| Re-measure all local builds | `node measure.mjs …` → `local1.txt`, `local2.txt` | every figure in section 2 reproduces |
| Re-measure 8 live sites | `node measure.mjs <urls>` → `live-rerun.txt` | identical to `live.txt` |
| Cited sources vs GitHub `main` | `curl raw.githubusercontent.com/…`, `cmp` with `src-refs/` | all 5 files identical; quoted lines exist (Astro `static-build.ts:136`, `plugin-css.ts:341,365`, 676 lines; Marko README:75; Fresh `esbuild.ts` 49–75, 121–137) |
| **Browser behaviour** (the report's main UNVERIFIED item) | `browser/test.mjs`, playwright-core 1.63 → `browser/run3.log` | Chrome 154.0.8037.93 (installed) and Playwright WebKit 26.6: 39 of 52 checks pass per browser, same 13 failures in both; details below. Firefox 155 build would not start in this sandbox ("Could not find profile folder"): **Firefox behaviour still UNVERIFIED** |
| Platform status from primary data | `web-platform-dx/web-features` `main` (`*.yml.dist`), `mdn/browser-compat-data` `main`, `Fyrd/caniuse` (data updated 2026-09-30) → `web/` | below, claim 14 |
| A dropdown with production behaviour, specialised | `realistic/build.mjs`, esbuild 0.28.2 | 776 B raw / 377 B br (layout), 944 / 472 with the modal |
| esbuild splitting and side-effect order | `esbuild-order/build.mjs`, esbuild 0.28.2 | source order `init1,shared`; with `splitting: true` → `shared,init1` |
| What the "unused" CSS is | `css-cache/what-is-unused.mjs` | Starlight 35% → 17% once the search UI's rules are kept |
| Per-page CSS over a 4-page visit | `css-cache/split.mjs` | Starlight: shared file 14.8 kB br, per-page pruned 37.8 kB br |

### Verdict per claim

| # | Claim (short) | Verdict | What I found |
| --- | --- | --- | --- |
| 1 | No surveyed framework wrote its own JS linker | **confirmed** | Sources match `main`. Two notes. SolidStart is named in finding 1 but was neither built nor source-checked. Outside the survey the picture differs: Vite 8 itself replaced esbuild + Rollup with Rolldown ("a Rust-based bundler that replaces both esbuild and Rollup", Astro 7 post, 2026-06-22), so the largest esbuild consumer left it. |
| 2 | A transform plugin alone was insufficient; HTML is emitted by the framework, not the bundler | **partly** | Build ordering: confirmed (Astro line 136, Marko README line 75, https://qwik.dev/docs/advanced/vite/). The HTML half is wrong for Marko: `@marko/vite` feeds virtual `.html` entries through Vite's HTML pipeline and parses Vite's emitted HTML into its asset manifest (`src/index.ts:1349` "read the final generated `.html` from vite", 1447–1463 `generateDocManifest(basePath, chunk.source)`). The table row "none use the bundler's HTML handling" should read "Astro, Qwik, SvelteKit, Fresh do not; Marko does, for asset tags". |
| 3 | Qwik needs Rollup-only hooks; esbuild has five callbacks | **confirmed** | `optimizer.mjs` 1.20.1: 5× `emitFile`, `manualChunks` 2792–2871, `generateBundle` 3012. https://esbuild.github.io/plugins/ lists exactly `onResolve`, `onLoad`, `onStart`, `onEnd`, `onDispose` and has the quoted sentence. |
| 4 | Fresh's builder is prior art for esbuild driven through its API | **partly** | The source is as described. Missing: Fresh's own docs now call it the "legacy `Builder` class"; `jsr:@fresh/init` generates a Vite setup and the docs describe migrating *from* the Builder (https://usefresh.dev/docs/advanced/vite, fetched 2026-10-04; Vite support announced 2025-09-02, https://deno.com/blog/fresh-and-vite). Reasons given are HMR, the plugin ecosystem and boot time — dev-server concerns, out of phase 2 scope, but they return with the dev server. |
| 5 | Marko 6.4.1: 3,683 B raw / 1,972 B br on `/`, 3,574 B elsewhere; modal code and CSS only on `/` | **confirmed** | Rebuilt, byte-identical. Detail: about 80 B of the 3,683 are two `//# sourceMappingURL` comments. |
| 6 | Marko chunk: 2,753 generic, 299 component-specific | **confirmed** | Recomputed: 3,081 = 2,753 + 299 + 29. |
| 7 | Hand-specialised 413 B / 227 B br; platform-only 0 B; Marko platform-only 0 B | **partly** | Sizes confirmed. Behaviour, now tested: everything the floors implement works in Chrome 154 and WebKit 26.6. But the 413 B dropdown is a toy: it does not close on outside click, Escape works only while focus is inside the list, focus does not return to the trigger, no arrow keys. The Marko stateful version has the same gaps (it mirrors the same source), plus `aria-expanded` renders as absent / `""` instead of `"false"` / `"true"`. The platform version does more at 0 B (light dismiss, Escape, top layer, anchored at `menuTop = btnBottom`, `menuRight = btnRight`; `commandfor`, `closedby=any` and `<details>` all pass), but `role=menu` on a `<ul>` of links is invalid ARIA and there is no arrow-key navigation. |
| 8 | SvelteKit 85.1 kB / 30.1 kB br, Qwik 57.9 kB / 22.1 kB br | **confirmed** | 84,537 + 532 and 57,150 + 726 re-measured; page text found in `nodes/2.*.js`; `modulepreload` of the 50,321 B core is in `qwik/dist/index.html`. |
| 9 | Live sites, eager first-party JS | **confirmed** | Identical on re-run. Two caveats on reading it. "Inline JS" includes serialized data: nextjs.org 227.8 kB raw inline and svelte.dev 48.8 kB raw inline are mostly payload, not code. And Starlight's 18.3 kB is not three components: it is theme switching, search, mobile menu, sidebar state and scroll restore, both tables of contents, synced tabs and code-block scripts (18 `<script>` tags on the page). It is not a like-for-like "reference point to beat". |
| 10 | Unused CSS: VitePress 77%, Docusaurus 73%, … Starlight 35% | **partly** | Reproduced exactly (Starlight 14,620 → 9,539 br; VitePress 33,503 → 7,598). The interpretation is too strong. On Starlight, 105 of the 226 rejected selectors are `.pagefind-ui__*`: the search UI that JS inserts when search opens. Keeping them: **17% unused, not 35%**. VitePress with DocSearch / local search kept: 58%, not 77%. The report names this limitation but still uses the raw figures in findings, lessons and targets. |
| 11 | Astro assigns CSS by walking the module graph; WebC collects what was rendered | **confirmed** | Comments at `plugin-css.ts:341` and `:365`; WebC docs quote verified; WebC build: `/` 1,136 B JS and 1,036 B CSS, `/guide` 813 and 909. |
| 12 | No framework emits per-page CSS at variant granularity or per-use-site specialised JS | **partly** | The cited sources say what is claimed (StyleX 2025-11-11 "reduced CSS size by 80%", app-wide sheet; Tamagui "30-50% of components typically flatten"). The CSS half of the absence claim overlooks SSR critical-CSS extraction in CSS-in-JS libraries, which has derived a page's CSS from what was rendered, at class (= variant) granularity, for years: Emotion `extractCritical` "pulls out Emotion rules that are actually used in the rendered HTML" (https://emotion.sh/docs/ssr); styled-components' `ServerStyleSheet` and Stitches' `getCssText` work the same way (UNVERIFIED in detail; Stitches' page says only "all the CSS you need to server-side render"). They ship a runtime and hydrate, so the output is not what we want, but the technique in recommendation 1 is not new. The JS half (no framework emits specialised framework-free JS per use site) is **unverifiable**; I found no counter-example. |
| 13 | 45% of Starlight's search chunk is Vite's preload helper | **confirmed** | 1,323 of 2,938. It answers the report's open question 5 in esbuild's favour: with esbuild 0.28.2 a split entry is `import"./TMTRNFJZ.js";` (23 B) and a dynamic import is a bare `import("./GFWODUZV.js")`. No helper, 0 B of wrapper (`realistic/build.log`). |
| 14 | Platform features are Baseline as of 2026 | **partly** | Dates confirmed from primary data: invoker commands Baseline *newly available* 2025-12-12 (Chrome 135, Firefox 144, Safari 26.2); `<details name>` 2024-09-03; `closedby` not Baseline (Chrome 134, Firefox 141, Safari "preview" only in BCD). Anchor positioning: the core properties used here (`anchor-name`, `position-anchor`, `anchor()`) are newly available 2026-01-13 (Chrome 125, Firefox 147, Safari 26), but web-features marks the feature as a whole `baseline: false`, and caniuse marks every Chrome and Firefox version partial. Two corrections: (a) "newly available" is not "widely available" — that comes 30 months later, mid-2028; (b) from caniuse usage data of 2026-09-30, about 85% of global users have invoker commands and 86% have anchor positioning. Roughly one visitor in seven gets a "Give feedback" button that does nothing unless a fallback ships. |
| 15 | Astro 7 replaced the Go/wasm compiler with Rust; ~6% of build time | **confirmed** | https://astro.build/blog/astro-7/, dated June 22, 2026: "In isolation, the Rust compiler showed a roughly 6% improvement in build times". Nuance: the post does give a motive in one sentence — "Moving to Rust allowed us to ship native binaries for supported platforms, with a WASM fallback" — and lists the Go compiler's HTML correction as removed behaviour. It supports lesson 10 slightly better than the report says. |

### New measurements that change conclusions

**1. esbuild `splitting` reorders side-effect modules (reproduced on 0.28.2).**

```
src/a.js:  import "./init1.js"; import "./shared.js";     src/b.js:  import "./init2.js"; import "./shared.js";
node src/a.js              -> a: init1,shared
splitting: true, dist/a.js -> a: shared,init1      (import"./Y6ZMRSVZ.js";globalThis.order=…"init1"…)
splitting: false           -> a: init1,shared
```

esbuild's docs still say "Code splitting is still a work in progress. It currently only works with
the `esm` output format. There is also a known ordering issue with `import` statements across code
splitting chunks" (https://esbuild.github.io/api/#splitting, issue #399, open since 2020). The
report's `floor/src/layout.js` is exactly this kind of module: top-level `getElementById(…).onclick
= …`. Recommendation 3 ("splitting plus metafile") therefore needs a rule: behaviour modules export
functions and have no top-level side effects, and the generated per-page entry calls them in order.
Or do not split at all, which lesson 7 already argues for at this size. Recommendation 3 and
lesson 7 contradict each other as written.

**2. The 0.5 kB JS target rests on a dropdown without the expected behaviour.**

`realistic/src/dropdown.js` adds what the platform popover gives for free (outside click, Escape
from anywhere, focus return) plus arrow keys, Home and End; the modal gets backdrop-click close.

| Variant | Layout pages | Page with modal |
| --- | --- | --- |
| Report's floor | 272 B raw / 170 br | 413 B raw / 227 br |
| Same, production behaviour | 776 B raw / 377 br | 944 B raw / 472 br |
| Marko 6 | 3,574 B raw / 1,904 br | 3,683 B raw / 1,972 br |

Still about 4× under Marko, but over the proposed "≤ 0.5 kB raw". Behaviour of this variant was
not browser-tested.

**3. Per-page pruned CSS costs more than a shared stylesheet once the visitor opens a second page.**

Same PurgeCSS method, four pages of one site, brotli, warm HTTP cache between pages:

| Site, 4-page visit | A. one shared file (as shipped) | B. per-page pruned, nothing shared | C. common chunk + per-page delta | First page only: A / B / C |
| --- | --- | --- | --- | --- |
| Starlight | **14.8 kB** | 37.8 kB | 22.2 kB (6.4 + 15.8) | 14.6 / 9.5 / 10.4 kB |
| VitePress | 33.5 kB | 36.5 kB | **19.1 kB** (6.8 + 12.3) | 33.5 / 7.6 / 8.2 kB |

C is overstated: PurgeCSS trims selector lists per page, so the same rule can serialize differently
on two pages and land in the delta. The direction is not in doubt. "No unused CSS rules on any
page" optimises the first view and forfeits the cache; the saving of "5–25 kB br per page" in the
report is a first-view number. At phase 2 scale (about 1 kB of CSS per page, inlined) none of this
matters in bytes. It matters as a principle if lesson 6 is where the "ridiculous" budget goes.

### What the report missed

1. **The bet is not testable by a platform-first design alone.** Marko 6 already emits 0 B and no
   script tag for the platform-only components, and so would Astro or plain HTML. A 0 B result
   proves that the platform is good, not that component awareness pays. Against the best existing
   compiler the whole JS headroom is 1.7 kB br, once, cached. Phase 2 needs a control: build the
   same site, same platform-first components, with Astro or Marko, and state in advance which
   differences count (CSS bytes per visit, HTML bytes, JS for the non-platform features).
2. **Platform-first needs a support floor and a fallback decision.** About 15% of global users
   lack invoker commands or anchor positioning today, and `closedby` is not in Safari stable. A
   fallback is JS, loaded conditionally — which is itself a per-feature behaviour module, so it
   fits the design, but the "0 B base case" then holds only for supporting browsers.
3. **"Dropdown menu" has two meanings with different costs.** A list of links behind a button is a
   disclosure: popover, no JS, no `role=menu`. An ARIA menu needs roving focus and arrow keys: JS
   in every browser (about 0.5 kB raw by the measurement above). The docs site needs the first.
4. **Nobody in the survey executes components inside a non-JS compiler.** Every framework that
   renders at build time does it in a JS runtime (Astro's Go compiler compiled `.astro` to JS and
   Node rendered it). Recommendation 1, "the Go compiler executes the layout components", has no
   prior art here and is the riskiest sentence in the recommendation. It is the subject of
   `evaluation.md`, not of this report, but this report should not present it as following from
   prior art.
5. **CSS goes through the bundler in every Vite-based framework surveyed.** Recommendation 2 says
   "Nothing in prior art suggests handing either to a bundler". Astro, Marko, SvelteKit and Qwik
   all import CSS as modules and let Vite chunk and minify it; Astro's 676-line plugin
   post-processes Vite's CSS chunks, and my Marko build's `sidemenu-MQQ02yZc.css` is a Vite chunk
   holding layout, dropdown and sidemenu rules together. The design conclusion (emit CSS from the
   render record) stands, on the opposite argument: bundler-graph assignment is what produces the
   coarse result, and WebC, which uses no bundler, is the positive example.
6. **esbuild writes a 23 B entry file that only imports the shared chunk.** With splitting on,
   each layout-only page costs two requests for 776 B. The driver should point the script tag at
   the chunk, inline it, or not split.

### Does the recommendation hold?

Yes: compiler in front, own driver around, esbuild as linker. Nothing I found argues for writing a
linker or for living inside a bundler plugin, and esbuild adds no boilerplate to tiny outputs.
Amendments:

1. Recommendation 3: drop `splitting`, or require behaviour modules without top-level side
   effects. Resolve the contradiction with lesson 7.
2. Targets: JS ≤ 1 kB raw / 0.5 kB br per page for the JS variant with full behaviour; 0 B only
   for browsers that have the platform features, with the fallback's size reported separately.
3. Replace "no unused CSS rules on any page" with a visit-level measure (bytes over a 3–4 page
   visit, warm cache), and decide between inlined per-page CSS and a common file plus deltas on
   that number.
4. Use 17% (not 35%) as Starlight's unused-CSS figure, and do not use its 18.3 kB of JS as a
   like-for-like reference.
5. Add the control build from "missed" item 1, so the bet can fail.

Not verified by me: Firefox behaviour of any build; Safari stable (WebKit 26.6 is Playwright's
build and reports `closedBy` as present, while BCD lists Safari support as "preview"); the Brisa,
Leptos, Nue, Remix 3 and Vue Vapor rows; the Marko newsletter quote (seen only in a search
snippet from https://markojs.com/docs/newsletter/may-2026).
