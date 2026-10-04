# esbuild seams — esbuild's Go API as the back end of the Reactogenic builder

Researcher key: `esbuild-seams`. Date: 2026-10-04.
Version examined: **esbuild v0.28.2**, released 2026-08-08 — the latest
(`curl https://proxy.golang.org/github.com/evanw/esbuild/@latest` →
`{"Version":"v0.28.2","Time":"2026-08-08T19:46:44Z"}`;
`gh api repos/evanw/esbuild/releases/latest` → `v0.28.2 2026-08-08`).

Everything marked "measured" was run today against that version with Go 1.27.1
on darwin/arm64. Experiments, sources and raw outputs:
`/private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-esbuild-seams/`
(`./exp a|b|c|c2|d|e|f`; outputs in `a.out` … `f.out`). Source line numbers
refer to the module at `~/go/pkg/mod/github.com/evanw/esbuild@v0.28.2`.

## Answer in short

esbuild has a clean seam, and it is **the public `pkg/api`, used in-process
as a linker + minifier behind our own compiler**. It has no seam *inside*
(no AST, nothing between linking and output), and the maintainer rules one
out. That is acceptable, because none of the component-aware work belongs
inside a bundler:

| Job | Who does it |
| --- | --- |
| Know which components, variants and slots a page uses | our compiler (tsgo AST + checker, already in the binary) |
| Emit per-page HTML, the per-page list of CSS, the per-page entry module | our compiler |
| Specialise a component's behaviour to its use site | our compiler (esbuild can do the constant-flag part: `Define` / `Transform`) |
| Resolve, tree-shake, scope-hoist, minify, lower, hash, source-map JS and CSS | esbuild |
| Decide the chunk plan | our compiler; esbuild's automatic splitting is an option, not the plan |

The numbers that carry this:

- Component-aware specialisation of one dialog module: **1116 B → 367 B**
  (−67 %) when the compiler knows the flags; the same component written with
  a runtime options object stays at **1214 B** — esbuild propagates nothing
  through `opts.closable` (experiment C).
- For a 4-page site, esbuild's `Splitting` makes every page **larger and
  slower to load**: e.g. the smallest page 532 B / 1 request → 751 B / 3
  requests (experiment A). One self-contained file per page is the better
  default at this size.
- The 4-page build takes **under 2 ms** in-process with every module virtual
  (experiment F1); 50 pages × 301 modules, 1.5 MB of output, ~90 ms (E3).

## 1. Extension points of `pkg/api` (v0.28.2)

Source: `pkg/api/api.go` (747 lines, read in full) plus the docs
(<https://esbuild.github.io/api/>, <https://esbuild.github.io/plugins/>,
fetched 2026-10-04). "Verified" = exercised in an experiment here.

### Plugins (`BuildOptions.Plugins`, Go: `api.Plugin{Name, Setup func(PluginBuild)}`)

| Hook | What it gives | Notes |
| --- | --- | --- |
| `OnStart` | runs at the start of each build / rebuild | may return errors/warnings |
| `OnResolve{Filter, Namespace}` | map an import path to `{Path, Namespace, External, SideEffects, Suffix, PluginData, WatchFiles, WatchDirs}` | Filter is a Go regexp. Receives `Importer`, `Namespace`, `ResolveDir`, `Kind` (entry point, import statement, dynamic import, CSS `@import`, `composes`, `url()`), `With` (import attributes). Verified |
| `OnLoad{Filter, Namespace}` | supply `Contents` + `Loader` + `ResolveDir` for a path | this is the **virtual module** seam: nothing has to exist on disk. Verified: all experiments run on an in-memory table in namespace `rg` |
| `OnEnd` | receives `*BuildResult` after linking and writing | "can modify the build result before returning" (docs). Verified: contents of `OutputFiles` can be replaced (E2). It is the only post-link hook and it sees whole files, after hashing |
| `OnDispose` | cleanup for a `Context` | |
| `PluginBuild.Resolve` | run esbuild's resolver from a plugin | |
| `PluginBuild.InitialOptions` | read/modify the options before the build | |
| namespaces | every module is `(namespace, path)`; default `file` | shown in the metafile as `rg:rt/dom.js` (D2) |
| `PluginData` | opaque value passed resolve → load | Verified (C6) |
| `Suffix` | `?query` kept on the path; gives two module identities for one source | Verified: `dialog.js?1111` and `dialog.js?0100` in one bundle (C6) |
| `WatchFiles` / `WatchDirs` | extra inputs for `Context.Watch` | dev-server territory, out of phase 2 |

"In Go, each callback may be run on a separate goroutine" (docs) — plugin
state needs synchronisation.

`OnLoad` results are **not cached** across `Rebuild`: 347–350 calls on each of
three rebuilds (E3, measured).

### Inputs

| Option | Use for us |
| --- | --- |
| `EntryPoints` / `EntryPointsAdvanced{InputPath, OutputPath}` | one entry per page with a chosen output name; the input may be virtual. Verified |
| `Stdin{Contents, ResolveDir, Sourcefile, Loader}` | a single in-memory entry; works with plugins (E5). One per build, so `EntryPointsAdvanced` + plugin is the general form |
| `Loader map[ext]Loader` / `OnLoadResult.Loader` | `js jsx ts tsx json css local-css global-css text base64 binary dataurl file copy empty default` |
| `Alias`, `External`, `Packages`, `Conditions`, `MainFields`, `ResolveExtensions`, `NodePaths`, `Tsconfig`, `TsconfigRaw`, `PreserveSymlinks` | ordinary resolution control; only relevant once user code imports npm packages |
| `Inject` | auto-import a file's exports wherever a global name is used |

### Outputs

| Option | Use for us |
| --- | --- |
| `Write: false` + `BuildResult.OutputFiles[]{Path, Contents, Hash}` | everything comes back in memory; nothing is written (F1: no `out/` directory). Lets the compiler **inline** a small script or stylesheet into the HTML instead of linking it |
| `Outdir`, `Outfile`, `Outbase`, `EntryNames`, `ChunkNames`, `AssetNames` (`[dir] [name] [hash] [ext]`), `OutExtension`, `PublicPath`, `AllowOverwrite` | naming by template only. `[hash]` is content-derived and stable across identical builds (E8) |
| `Format`: `iife` / `cjs` / `esm`; `GlobalName`; `Platform` | `iife` for a classic or inline `<script>`; `esm` for `<script type=module>` and for `Splitting` |
| `Splitting` | automatic shared chunks; **esm only** (A4: `Splitting currently only works with the "esm" format`) |
| `Banner` / `Footer` per type (`js`, `css`) | Verified (E5) |
| `Sourcemap` (inline / linked / external / both), `SourceRoot`, `SourcesContent` | Verified: an inline map carried by a virtual module is **composed**, so the final map points at the `.rtsx` (F2: `sources=[src/pages/index.rtsx]`) |
| `LegalComments` | none / inline / eof / linked / external |
| `Metafile` + `AnalyzeMetafile` | see §3 (d) |

### Optimisation and lowering

| Option | What it does (measured where noted) |
| --- | --- |
| `MinifyWhitespace`, `MinifyIdentifiers`, `MinifySyntax` | the minifier; `MinifySyntax` also does DCE of constant branches |
| `TreeShaking`, `IgnoreAnnotations`, `/* @__PURE__ */`, `sideEffects` (package.json or `OnResolveResult.SideEffects`) | top-level-statement tree shaking per file |
| `Define map[ident]expr` | **global per build**; replaces free identifiers / member chains before DCE. The one esbuild mechanism that removes whole features (C1) |
| `DropLabels` | removes labelled statements; same effect as `Define` (C5: 1116 → 367 B) |
| `Drop` | `console`, `debugger` |
| `Pure []string` | mark calls to named functions as removable when unused |
| `MangleProps`, `ReserveProps`, `MangleQuoted`, `MangleCache` | regex-driven property renaming; `MangleCache` keeps names consistent across builds (C8: `{open_, toggle_}` → `{t, s}`) |
| `KeepNames`, `Charset`, `LineLimit` | |
| `Target`, `Engines`, `Supported map[feature]bool` | JS and CSS lowering (F4: CSS nesting flattened, `?.` lowered) |
| `JSX*` | not needed: the output has no JSX |

### Other entry points

| API | Use |
| --- | --- |
| `api.Transform(src, TransformOptions)` | single-file, no resolution: minify/lower a JS or CSS **string**. Verified as (1) the minifier for an inline `<script>` (E9), (2) a CSS minifier/lowerer (B4: 539 → 440 B), (3) a per-variant specialiser with its own `Define` (C6b) |
| `api.Context(opts)` → `Rebuild / Watch / Serve / Cancel / Dispose` | incremental. With virtual modules a rebuild is not meaningfully faster (93 → 73–84 ms, E3) because `OnLoad` reruns; irrelevant for a one-shot `build` command |
| `api.FormatMessages`, `Message{Location, Notes, Detail}` | diagnostics in esbuild's format, or map them into ours |
| `pkg/cli`: `Run`, `RunWithPlugins`, `ParseBuildOptions` | CLI-flag parsing for Go embedders (since 2024, changelog #3539) |

Cost of linking esbuild into a Go binary: **+10.1 MB** unstripped
(12 548 178 B vs 2 413 202 B hello-world), **+7.2 MB** with `-ldflags "-s -w"`
(measured). esbuild's `go.mod` says `go 1.13` and pins an old `golang.org/x/sys`;
as a dependency, MVS picks ours (`x/sys v0.48.0` in `go/go.mod`) — the
experiment module builds with Go 1.27.1 without a `replace`.

## 2. What esbuild does not offer

| Missing | Evidence / maintainer's position |
| --- | --- |
| **AST access or AST transforms** | Docs, *Plugin API limitations*: "It's not possible to hook into every part of the bundling process. For example, it's not currently possible to modify the AST directly." FAQ, *Upcoming roadmap*: "I am not planning to include these features in esbuild's core itself: … An API for custom AST manipulation". [#1880](https://github.com/evanw/esbuild/issues/1880) (evanw, 2021-12-22): "AST manipulation is out of scope… You'll need to use another tool instead, either to preprocess the input to esbuild, post-process the output from esbuild, or to replace esbuild entirely." |
| **Hook after linking / per chunk** (`renderChunk`) | [#1315](https://github.com/evanw/esbuild/issues/1315), open since 2021-05-26, one maintainer question, nothing since. Only `OnEnd` exists: whole output files, after names are hashed and cross-chunk import paths are written. Editing a file there leaves `Hash` and the file name stale (E2, measured) |
| **Manual chunks** | [#207](https://github.com/evanw/esbuild/issues/207), open since 2020-06-30 (last activity 2024-11-21). Making a module an extra entry point does **not** pin its code there: the code still goes to automatic chunks and the entry becomes a re-export shell (A5, measured) |
| **Chunk naming beyond a template** | `ChunkNames` takes `[name] [hash] [ext] [dir]`; `[name]` is always `chunk` for shared chunks. No callback |
| **HTML entry points** | [#31](https://github.com/evanw/esbuild/issues/31), open since 2020-02-22; FAQ: "I may also look into adding an HTML content type". Last comment 2026-02-06 confirms it still cannot. "The bundled JavaScript generated by esbuild will not automatically import the generated CSS into your HTML page for you" (content-types docs) |
| **CSS code splitting** | One CSS bundle per entry point, shared CSS **duplicated** in each, with or without `Splitting` (B1/B2, measured). [#608](https://github.com/evanw/esbuild/issues/608): evanw sketches `@import`-based CSS chunks (2020-12-18) and "just generate a separate CSS file for each entry point without any code splitting or sharing" (2021-02-11); the latter is what ships |
| **Unused-CSS removal** | None. A never-referenced rule in a global file and a never-imported local class in a CSS module both survive (`.unused-utility`, `.neverReferenced` → `.i{…}`; B1/B3, measured). esbuild does remove *duplicate* rules across files (B5; changelog 2022, #2688) |
| **Cross-module constant propagation with DCE** | Constants are inlined only from a `const` prefix at the top of a scope, and **any `import` in the file disables it at top level** ("Import statements anywhere in the file disable top-level const local prefix because import cycles can be used to trigger TDZ", `internal/js_parser/js_parser.go:8204`). Cross-module inlining of an import-free constants file happens **in the printer, after tree shaking**: the value is substituted but the guarded code and its imports stay (C3: 1083 B vs 367 B; `if(!1)for(…)` is in the output). The feature is from 2022 (changelog, #1317, #1981: "you can trigger this optimization by just putting the constants you want inlined into a separate file"); the 2025 changelog restates the limit: "esbuild's constant inlining only happens in very restrictive scenarios to avoid issues with TDZ handling" (`CHANGELOG-2025.md:510`) |
| **Function inlining / specialisation** | Only calls to **empty** and **identity** `function` declarations are inlined (`js_parser.go:11327–11349`; C7b measured: `id(5)` → `5`, `nothing(1,2)` removed, but `always()`, `add1(1)` stay; arrow-function constants are not inlined at all, C7). No propagation of argument values into a callee: object flags (C4, 1214 B) and positional boolean flags (C4b, 1152 B) remove nothing |
| **Statement-level code splitting** | Removed in 0.11.7 (2021): "This file-splitting feature has been removed because it doesn't work well with … top-level await… reachability is still tracked at the file level" (`CHANGELOG-2021.md:3865`). A shared chunk therefore carries every live export of a file, including ones a given page never calls (A2: the changelog page downloads `rovingFocus`) |
| **Splitting for `iife` / `cjs`** | "Code splitting is still a work in progress. It currently only works with the `esm` output format" (API docs); A4 measured |
| **Correct evaluation order across chunks** | [#399](https://github.com/evanw/esbuild/issues/399), open since 2020-09-20, linked from the API docs as a "known ordering issue". **Reproduced on v0.28.2** (E1): without splitting `1 init → 2 shared → 3 a`; with splitting `2 shared sees CONFIG = undefined → 1 init → 3 a` |
| **Top-level await outside esm** | "bundling code containing top-level await is only supported when the output format is set to `esm`" (docs); E7: `Top-level await is currently not supported with the "iife" output format` |
| **Type checking, other languages, HMR, module federation** | FAQ, same out-of-scope list |
| **1.0 / stability promise** | FAQ: "I think of esbuild as a late-stage beta… still missing some significant features… (mainly that code splitting is still pretty primitive)" |

Scope hoisting, as it is: ESM inputs are concatenated into one scope with
symbols renamed to avoid collisions (all A1 outputs are flat; glue is 1 B per
file). Wrappers appear only for CommonJS inputs (`__commonJS` + interop
helpers: **555 B** of runtime for a one-line CJS module, D1) and for ESM
modules reached through `import()` when `Splitting` is off (E6: 1434 B with
the lazy `__esm` wrapper vs 212 + 972 B split). If the runtime we author is
plain ESM with static imports, neither occurs.

The maintainer's own framing (FAQ): "Think of esbuild as a 'linker' for the
web. It knows how to transform and bundle JavaScript and CSS. But the details
of how your source code ends up as plain JavaScript or CSS may need to be
3rd-party code."

## 3. Experiments

Fixture: five React-free "behaviour" modules as a design system would ship
them — `rt/dom.js` (4 helpers), `rt/focus.js` (`trapFocus`, `rovingFocus`),
`rt/sidemenu.js`, `rt/dialog.js`, `rt/dropdown.js` — and four generated page
entries: `index` (sidemenu), `guide` (sidemenu + dropdown), `api` (all three),
`changelog` (dialog). All modules are virtual (plugin, namespace `rg`). All
builds: `Bundle`, `esm`, `ES2022`, full minify.

### (a) Per-page entries, with and without `Splitting`

| | no splitting (A1) | `Splitting` (A2) | fine-grained modules + `Splitting` (F3) |
| --- | --- | --- | --- |
| output files | 4 | 9 (4 entries + 5 chunks) | 8 |
| site total | 4934 B | 3142 B | — |
| `index` first load | **532 B, 1 req** (gz 326) | 751 B, 3 req (gz 579) | 751 B, 3 req |
| `guide` | **1285 B, 1 req** (gz 621) | 2034 B, 5 req (gz 1336) | 1572 B, 4 req |
| `api` | **2096 B, 1 req** (gz 932) | 2659 B, 6 req (gz 1738) | 2500 B, 5 req |
| `changelog` | **1021 B, 1 req** (gz 581) | 1619 B, 4 req (gz 1083) | 1198 B, 3 req |

- **Without splitting, every page is tree-shaken on its own**: `rt/dom.js`
  costs 160 B on `index` (no `uid`) and 209 B elsewhere; `rt/focus.js` costs
  290 B on `guide` (only `rovingFocus`), 336 B on `changelog` (only
  `trapFocus`), 626 B on `api`. The shared helper is **duplicated**, in its
  minimal form.
- **With splitting, the shared helper lands in a shared chunk** (`dom.js` →
  `chunk-R36XDG4Z.js`, 241 B), one chunk per distinct set of entries, at file
  granularity. Each page pays for the union of exports (the whole `focus.js`,
  696 B) plus import/export glue: 28 % of all bytes are glue (D1).
- Chunk count grows with the number of distinct entry sets: 6 pages where
  each pair shares one helper → **15 chunks of 51 B**, 6 requests per page
  (A7).
- IIFE (classic / inline script) costs +11 B per file over esm (A3).
- Compiler-chosen sharing works through two builds: build the shared layer as
  an entry (`base-TEKE3KTR.js`, 910 B), then build pages with those modules
  resolved by a plugin to that URL as `External` (A6: `index` = 429 + 910 B in
  2 requests). The layer's export names stay unmangled because it is an entry.
  Cosmetic cost: esbuild does not merge repeated imports of the same external
  (5 `import … from"./base-TEKE3KTR.js"` statements in `api.js`).

Module granularity is the lever we control: one export per module turns
file-level splitting back into function-level splitting (F3).

### (b) CSS per entry

| Case | Result (measured) |
| --- | --- |
| JS entries that `import "x.css"` (B1) | one sibling `.css` per entry (`index.css` 315 B, `guide.css` 406 B, `api.css` 628 B, `changelog.css` 404 B). `base.css` (181 B) is **repeated in all four**. Identical with `Splitting: true`: no CSS chunk is ever shared. The metafile links them: `outputs["out/api.js"].cssBundle = "out/api.css"` |
| CSS entry points generated per page, `@import`-ing component CSS (B2) | same four files, byte for byte; no JS involved |
| A JS entry that imports only CSS (E4) | still emits an empty `index.js` (0 B) that we would have to drop |
| CSS modules (`local-css`), JS uses 1 of 3 classes (B3) | CSS keeps all three rules (`.d{…}.e{…}.i{…}`); with a default import the JS keeps the whole map `{card:"d",title:"e",neverReferenced:"i"}`; with a named import only the used string remains in JS — the CSS is unchanged either way |
| Same file `@import`-ed twice; identical rule in two files (B5) | deduplicated: `.y{…}.btn{…}.a{…}` |
| Unused rules | never removed (`.unused-utility` survives everywhere) |

So for CSS esbuild is a concatenator, minifier, lowerer and class-name
mangler. *Which rules a page gets* is entirely the caller's decision — which
is exactly the per-route CSS the architecture already assigns to the compiler.

### (c) `Define` + minify as a specialisation mechanism

`rt/dialog.js` with four optional features (focus trap, close buttons,
backdrop click, enter animation), importing `focus.js` and `animate.js`.

| How the flag is expressed | all on | all off | Dead imports removed? |
| --- | --- | --- | --- |
| C1 free identifiers + `BuildOptions.Define` | 1116 B | **367 B** | yes — `focus.js`, `animate.js` gone; `dom.js` 127 → 78 B |
| C5 labelled statements + `DropLabels` | 1116 B | **367 B** | yes |
| C2c `const` prefix inside the function body | — | **367 B** | yes |
| C6b `OnLoad` runs `api.Transform{Define}` per variant, several variants in one build | 1116 B | **367 B** | yes |
| C2 top-level `const` in the same module (module has imports) | 1152 B | 1152 B | no — not inlined at all |
| C3 `export const` in another (import-free) module | — | 1083 B | no — value printed as `!1`, code kept |
| C3b TS `const enum` in another module | — | 1079 B | no |
| C4 `opts.closable` etc., literal object at the call site | — | 1214 B | no |
| C4b positional boolean parameters, `false` at the call site | — | 1152 B | no |

Intermediate points with `Define`: modal + close only → 826 B; close only →
430 B.

Findings:

1. Dead code **does** go away, including the now-unused imports and their
   whole modules — but only when the constant is substituted **in the parser**
   (`Define`, `DropLabels`, a nested-scope const prefix).
2. **Nothing propagates across a call or a module boundary** in a way that
   enables DCE. A component written the React way (`props.closable`) is opaque
   to esbuild.
3. `Define` is one map per build. Two use sites with different flags in one
   build need two module identities: a plugin resolves `dialog.js?1111` /
   `dialog.js?0100` (`Suffix` + `PluginData`) and specialises the source in
   `OnLoad`. Doing that with `api.Transform{Define, MinifySyntax}` works
   (C6b: page `b` 430 B, page `d` 367 B, page `c` with both variants 1469 B);
   prepending top-level `const`s does not (C6: 1152 B each).
4. Small-function inlining is negligible (C7/C7b).

This is the bet, measured on one component: **3.3× smaller** (1214 → 367 B)
when the thing in front knows the props at compile time, and esbuild cannot
be the thing that knows.

### (d) Metafile: can every output byte be attributed?

Shape (D2, v0.28.2): `inputs[path] = {bytes, imports[{path, kind, original,
external?, with?}], format}`; `outputs[path] = {bytes, inputs[path] =
{bytesInOutput}, imports[{path, kind, external?}], exports[], entryPoint?,
cssBundle?}`. Virtual modules appear as `namespace:path`.

| Build | bytes attributed to an input |
| --- | --- |
| esm, no splitting, minified | **99.9 %** (1 B of glue per file) |
| iife, minified | 99.0 % (12 B wrapper per file) |
| esm, no splitting, not minified | 95.7 % (path comments) |
| esm, `Splitting`, minified | **72.4 %** — cross-chunk `import`/`export` statements belong to no input; entry files are 33–45 % attributed |
| one CommonJS input | 13 % — esbuild's runtime helpers (555 B) belong to no input |

- Per-page cost is computable: `entryPoint` → output, then the closure over
  `outputs[].imports` with `kind: "import-statement"` (used for the tables
  above; also the list for `<link rel=modulepreload>`).
- "Why is it here" is computable at **module** level: `inputs[].imports`
  (with the `original` specifier) is the import graph; `AnalyzeMetafile(…,
  Verbose)` prints the shortest import chain (D3).
- Not available: which **exports/statements** of a module survived (only a
  byte count), and no attribution below the module. A "why does this page
  ship this function" report needs module-per-function granularity or our own
  bookkeeping — which we have, since the compiler generated the entries.

### Extras

| | Result |
| --- | --- |
| E1 | evaluation-order bug #399 reproduced with `Splitting` (see §2) |
| E2 | `OnEnd` can replace `OutputFiles[i].Contents`; `Hash` and names are not recomputed |
| E3 | 50 entries × 301 modules: 93 ms first build, 73–84 ms rebuilds; `OnLoad` re-invoked every rebuild |
| E6 | `import()` without `Splitting` is inlined behind a lazy wrapper (1434 B); with `Splitting` it becomes its own file (212 + 972 B) — the on-demand loading seam for a rarely opened dialog |
| E8 | `[hash]` is deterministic and content-derived |
| F1 | 4-page build: best 0.67 ms, mean 0.9–1.9 ms |
| F2 | input source maps on virtual modules are composed into the output map |

## 4. The internal API and forks

- **Not importable.** `import "github.com/evanw/esbuild/internal/js_ast"` from
  another module → `use of internal package github.com/evanw/esbuild/internal/js_ast
  not allowed` (measured, `internal.log`). Go enforces `internal/`; the only
  ways in are a fork or a path-rewritten copy.
- **Maintainer on using the internals** ([#2172](https://github.com/evanw/esbuild/issues/2172),
  2022-04-11, "Forking esbuild to build an AST plugin tool"): "The internal
  AST is not designed for this use case at all, and it's not a use case that
  I'm going to spend time supporting… the AST is not cleanly abstracted and is
  only intended for use with esbuild (e.g. uses a lot of internal data
  structures, has implicit invariants regarding symbols and tree shaking,
  does some weird things for performance reasons)… keeping a hack like this
  working over time as esbuild changes might be a big pain for you."
  I found no statement against forking as such (MIT licence); the position is
  "unsupported, undocumented, will break".
- **Fork surface** (measured on v0.28.2): 87 319 lines of non-test Go under
  `internal/` (`js_parser` 26 429, `css_parser` 9 905, `linker` 7 441,
  `js_ast` 7 188, `resolver` 5 675, `js_printer` 5 025, `bundler` 3 568) plus
  53 188 lines of tests; `pkg/` is 6 588 lines.
- **Churn**: 13 tagged releases in the last 12 months (v0.25.11 2025-10-15 →
  v0.28.2 2026-08-08), on 10 distinct dates. `v0.27.0...v0.28.2` (9 months):
  119 commits, 46 files under `internal/` touched, +3049/−1306 lines,
  `js_parser.go` alone 837 changed lines
  (`gh api repos/evanw/esbuild/compare/v0.27.0...v0.28.2`). A patch set on the
  parser or linker would need rebasing at every one of those.
- **How others consume it** (checked 2026-10-04 via the GitHub API):

| Project | How |
| --- | --- |
| Hugo | public `pkg/api` only, `github.com/evanw/esbuild v0.28.2` in `go.mod`; wrappers in `internal/js/esbuild/{build,batch,resolve,options}.go` — plugins for resolution, nothing internal |
| esm.sh | `github.com/ije/esbuild-internal v0.28.2`: a mechanical mirror that moves `internal/*` and `pkg/api` to importable paths with a script (`update.ts` downloads the upstream tag and rewrites import paths). Per that script the code is exported, not modified. 12 stars; a second such mirror exists (`matthewmueller/esbuild_internal`). What esm.sh does with the internals beyond `api` is UNVERIFIED |
| Astro compiler (`withastro/compiler`) | no module dependency; a **copied subset** under `lib/esbuild/` (`css_ast`, `css_lexer`, `css_parser`, `css_printer`, `logger`, `sourcemap`, `helpers`, …) — used as a CSS parser/printer for style scoping, frozen at copy time |
| Bun | UNVERIFIED here: began as a Zig port of esbuild's parser/bundler, i.e. "replace esbuild entirely" |

Nobody in this list carries a behavioural patch on esbuild's linker. The
existing pattern for "I need the internals" is a path-rewritten mirror
(read-only use of the parser/printer), not a modified fork.

Reactogenic already maintains one vendored compiler fork (tsgo, with a patch
directory and a re-vendor task, RGP1-114). A second fork of a ~90 kLOC code
base whose author calls its AST "not cleanly abstracted" should need a
concrete, measured reason. None appeared in these experiments.

## 5. Where a compiler in front of esbuild stops being enough

Going through what phase 2 needs:

| Need | In front of esbuild? | How |
| --- | --- | --- |
| Per-page entry that mounts exactly the components on the page | yes | generated virtual module (A) |
| Per-page CSS = the rules of the components/variants on the page | yes — and *only* in front | generated CSS entry or a concatenated string through `Transform` (B2, B4). esbuild will never prune it for us |
| Per-use-site specialisation by constant flags | yes | variant module identity + `Transform{Define}` in `OnLoad` (C6b), or the compiler prints the specialised source itself |
| Specialisation beyond flags: inlining a slot body, unrolling a loop over constants, resolving `props.x` | yes, but it is **our** partial evaluator; esbuild contributes nothing (C4) | on the tsgo AST, before esbuild |
| HTML, `<template>`s, `<script>`/`<link>` tags, inlining small assets | yes | `Write: false`, read `OutputFiles` + `Metafile` |
| Chunk plan (what is shared between pages) | yes | choose per build: one file per page (default at this size), a compiler-chosen shared layer via two builds (A6), or `Splitting` with module-per-export granularity (F3) |
| "What does this page ship and why" report | yes | metafile (99.9 % attribution without splitting) + our own entry manifest |
| Source maps back to `.rtsx` | yes | inline map on the generated module (F2) |

It stops being enough only at these points, none of which phase 2 hits:

1. **A transformation that must see linked code** — e.g. cross-module
   inlining after tree shaking, merging near-identical functions across
   modules, whole-program property mangling by type. There is no hook; the
   options are post-processing whole output files in `OnEnd` (re-parse with
   our own parser, redo hashes and import paths ourselves) or doing the
   whole-program step *before* esbuild by emitting fewer, already-merged
   modules. Because the framework owns all the runtime modules, "before" is
   always available to us.
2. **Sharing below a module** between pages, with correct evaluation order:
   file-level chunks + bug #399. Avoidable by construction — side-effect-free
   modules, one export per module, all side effects in the generated entry —
   but it is a discipline we must enforce on the runtime, not something
   esbuild guarantees.
3. **Shared CSS chunks.** Not available; we would write the shared file
   ourselves (it is concatenation).
4. **Classic-script code splitting.** `Splitting` needs esm; inline or `iife`
   scripts cannot share chunks. Irrelevant if each page is one file.
5. **Many generated variants.** `OnLoad` is not cached and each variant is a
   separate module, so variant explosion is our problem to bound. At
   0.2–1 kB per variant and sub-millisecond builds this is far away.

What would actually force "a new type of compiler" is not on esbuild's side
of the seam at all: executing design-system `.tsx` at compile time,
partially evaluating components against their props, and proving shell
rules — all AST- and type-level work that sits on tsgo, which we already
have in-process. esbuild is the last stage of that compiler, not a
competitor to it.

## Recommendation

1. **Use esbuild through the public `pkg/api`, in-process, as the back end.
   Do not fork it and do not import a mirror of its internals.** Pin the
   version in `go/go.mod` (currently `v0.28.2`); cost +7–10 MB of binary.
2. **Everything goes through virtual modules**: one plugin with a namespace
   for generated entries and one for runtime modules; `Write: false`; the
   compiler writes the files (and decides what to inline) from `OutputFiles`
   and the metafile.
3. **Default chunk plan for the docs site: one self-contained JS file and one
   CSS file per page, `Splitting` off.** At 0.5–2 kB per page this is smaller
   and fewer requests than sharing. Keep the decision in the compiler (it has
   the page × component matrix) and revisit with a compiler-chosen shared
   layer (A6) when measurements say so — not esbuild's automatic chunks.
4. **Author the runtime for the linker we have**: plain ESM, static imports,
   no top-level side effects, one export per module, feature switches as
   free identifiers (`__RG_MODAL__`) or labelled statements rather than
   option objects. Then `Transform{Define}` per variant gives the measured
   3× reduction with no AST work on our side. Treat anything beyond constant
   flags as the compiler's own partial evaluation on the tsgo AST.
5. **Per-page CSS is the compiler's output**, selected by component/variant
   usage; esbuild only minifies and lowers it.
6. **Ship the report early**: metafile + the compiler's own manifest give a
   per-page "bytes, by module, and the import chain" table almost for free,
   and it is the instrument for confirming the bet.
7. Re-examine the fork question only if a measured optimisation needs linked
   code (§5, point 1). The first resort then is a second pass over
   `OutputFiles`, not a fork.

## Open questions

1. **Which engine executes design-system `.tsx` at compile time?** Not an
   esbuild question, but it decides whether esbuild is also used to bundle
   the code the compiler *runs* (a second, internal build targeting that
   engine).
2. **Module script or classic/inline script per page?** `esm` output needs
   `<script type=module>` (deferred, CORS-fetched, cannot be `file://`-opened
   in some browsers); `iife` can be inlined into the HTML and costs +11 B.
   For ~0.5–2 kB of JS, inlining may beat any request. Needs a decision and a
   CSP position (the layout spec already rejects inline scripts in
   templates for CSP reasons).
3. **How are variants keyed?** Per use site (maximal specialisation, more
   bytes when a page has two dialogs) or per distinct flag set (C6b: a page
   with two variants is 1469 B; one general copy is 1116 B plus a second
   call). The compiler needs a rule for when to fall back to the general
   module.
4. **Is "one export per module, no top-level side effects" acceptable as a
   hard rule for the framework's runtime?** It is what makes both tree
   shaking and any later splitting exact, and sidesteps #399.
5. **Shared layer threshold**: at what site size does a compiler-chosen
   shared file beat per-page duplication? Needs the real components'
   sizes; the fixture here is too small to set the number.
6. UNVERIFIED: whether esbuild's minifier output is within a few percent of
   terser/SWC/oxc on this kind of DOM-heavy code. If the gap matters, a
   second minifier pass over `OutputFiles` is possible without changing the
   seam.
7. UNVERIFIED: behaviour of `Context.Watch` with purely virtual inputs
   (`WatchFiles`) — out of phase 2 (no dev server), relevant later.
8. esbuild is pre-1.0 and has a single maintainer; 2026 has had releases on
   five dates so far (02-05, 03-12, 04-02, 06-11, 08-08) and code splitting
   has been "work in progress" since 2020. Acceptable for a pinned, in-process
   dependency behind a narrow interface, but worth isolating behind one Go
   package (`go/internal/link` or similar) so it can be swapped.

## Verification (independent)

Skeptic pass, 2026-10-04. Not written by the report's author; the text above
is unchanged. Everything here is in
`/private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-esbuild-seams-verify/`:

- `rerun/` — the author's sources copied and rebuilt from scratch
  (Go 1.27.1, esbuild v0.28.2). `a.out b.out c.out c2.out d.out` are
  **byte-identical** to the author's; `e.out`/`f.out` differ only in timings.
- `v/` — my own variants (`./v v1|v2|v4|v4b|v5|v7|v8|v9`, outputs `v*.out`),
  in a module that pins the repo's dependency versions.
- `min/` — esbuild vs terser 5.51.2, @swc/core 1.16.13, oxc-minify 0.152.0
  (`cmp*.mjs`, `cmp*.out`; `node_modules` removed, `package-lock.json` kept).

### Verdict per claim

| # | Claim (short) | Verdict | What I checked |
| --- | --- | --- | --- |
| 1 | v0.28.2 is current; builds with the repo's dependency versions | **confirmed**, evidence corrected | Proxy `@latest` and `gh releases/latest` both give v0.28.2, 2026-08-08. The author's module did **not** test the second half: its `go.mod` resolved `golang.org/x/sys` to esbuild's 2022 pseudo-version, not the repo's. My module (`v/go.mod`: `go 1.27.0`, `x/sys v0.48.0`, `x/sync v0.23.0`, `x/text v0.42.0`, `klauspost/compress v1.20.0`, all imported) builds and runs every variant with no `replace`. Cross-compiles for all six release platforms in `scripts/release.sh` (linux/windows × amd64/arm64, darwin/amd64; stripped 8.6–9.7 MB) |
| 2 | Public API drives everything from memory | **confirmed** | Rerun. Added V5: a broken `tsconfig.json` and `package.json` in `AbsWorkingDir` have no effect on a virtual `.ts` module (0 errors, 0 warnings, identical output, no `out/`). Not checked at syscall level |
| 3 | No AST access, permanently out of scope | **confirmed** | Quotes verified on the plugins page, the FAQ, #1880 and #2172 (fetched today). The FAQ adds a sentence the report omits: "I'm not doing active feature development for esbuild at the moment… I'm still actively maintaining esbuild and doing periodic esbuild releases." No seam is coming |
| 4 | No hook between linking and output; `OnEnd` leaves `Hash` stale | **confirmed** | Rerun (E2). `pkg/api/api_impl.go:1638–1680` runs `OnEnd` after the result is populated. #1315 open, 2 comments, last touched 2021-05-26 |
| 5 | Flags via `Define`: 1116 → 367 B; options object stays at 1214 B | **confirmed as measured; the interpretation needs two caveats** | Rerun identical. Gzip: 613 → 250 B (−59 %). Caveats below (V7, terser) |
| 6 | esbuild propagates no constants across call or module boundaries in a way that enables DCE | **partly** | True for one pass, every case reproduced. **Refuted for the cross-module `export const` case with a second pass** (V1): feeding the linked output back through `api.Build` gives 1083 → **367 B**, the same as `Define`. The second pass does nothing for same-module top-level consts (1152 → 1144), the options object (1214 → 1206) or positional booleans (1152 → 1144). Citation detail: `js_printer.go:981` is the decorator path; the main substitution sites are `:1769` and `:3194` |
| 7 | Several variants in one build via `Suffix` + `PluginData` + `Transform{Define}` in `OnLoad` | **confirmed** | Rerun identical (430 / 367 / 1469 B). Added V8: the source-map chain stops at the variant module (`rg-variant:rt/dialog.js?0100`) unless the `Transform` call sets `Sourcemap: inline` and `Sourcefile`; with them the final map names the original (`runtime/dialog.ts`) |
| 8 | `Splitting` makes every page larger, with more requests | **confirmed**, and it survives a warm cache | Rerun identical. Added V2, see below |
| 9 | Independent tree shaking without splitting; file-granular chunks with it | **confirmed** | Rerun identical; the changelog entry is under `## 0.11.7` |
| 10 | #399 reproduces on v0.28.2 | **confirmed** | Reran both outputs under node v25.2.1: split-true prints `2 shared sees CONFIG = undefined` first. Issue open, last updated 2025-04-15; the API docs still link it |
| 11 | Splitting is esm-only; no top-level await in iife | **confirmed** | Rerun; error text at `api_impl.go:1443`. The top-level-await sentence is on the content-types page, not the API page |
| 12 | CSS: per entry, shared CSS duplicated, no unused-rule removal | **confirmed** | Rerun identical. #608 is closed (2021-05-18). Added V4/V4b, see below |
| 13 | Metafile attribution 99.9 % / 99.0 % / 72.4 % | **confirmed** | Rerun identical |
| 14 | Internals not importable; ~87k lines; they move | **confirmed**, churn figure overstated for the part that matters | My count is 87 436 lines (author 87 319). The compare gives 46 files, +3049/−1306 — **including tests and snapshots**. Non-test code under `internal/`: 26 files, +1863/−972, of which 567 added lines are two new logger style files. `linker.go`: +45/−31. `bundler.go`: +109/−52. `js_parser.go`: +571/−266. A linker patch would have had little to rebase in those nine months; a parser patch a lot |
| 15 | Known Go consumers do not patch the linker | **confirmed**, and the UNVERIFIED part is resolved | Hugo `go.mod:30` → `evanw/esbuild v0.28.2`. esm.sh → `ije/esbuild-internal v0.28.2`; besides `api` it imports `js_parser`, `js_ast`, `logger`, `config` (`server/build_resolver.go:15–19`) and `xxhash` (`server/router.go:26`): read-only parsing. Astro compiler: no esbuild in `go.mod`, a copied subset in `lib/esbuild` (repo pushed 2026-09-17) |
| 16 | Build cost negligible; binary +10.1 / +7.2 MB | **confirmed** | Rerun: 85 / 76 / 71 ms for 50 × 301; 4 pages best 0.66–0.79 ms; `OnLoad` 348–351 calls per rebuild. Binary 12 548 258 vs 2 413 154 B, stripped 8 759 890 vs 1 571 202 B. Context the report lacks: the shipped darwin-arm64 `reactogenic` is 28.7 MB (alpha.1), so the stripped increment is about +25 %, an upper bound because the standard library is already linked |
| 17 | Source maps compose back to `.rtsx` | **confirmed** | Rerun identical; see the caveat under 7 |

No claim was refuted outright. One is partly wrong (6) and three rest on
weaker evidence than stated (1, 5, 14).

### New measurements

**V1 — a second esbuild pass is a real post-link seam.** The report says
cross-module constants never lead to DCE. After linking they are literals in
one file, and a second `api.Build` over that file removes the dead branches
and the functions only they used:

| Input to pass 2 | pass 1 | pass 2 | pass 3 |
| --- | --- | --- | --- |
| C3, `export const` flags in another module | 1083 B | **367 B** | 363 B |
| C2, top-level `const` in the same module | 1152 B | 1144 B | 1144 B |
| C4, options object | 1214 B | 1206 B | — |
| C4b, positional booleans | 1152 B | 1144 B | — |

This is not a better route than `Define`, since flags in a shared module are
still one set per build. It does make §5 point 1 concrete: "a second pass
over `OutputFiles`" works today for anything that becomes a literal after
linking, at the cost of redoing hashes.

**V7 — the 367 B depends on esbuild's purity analysis, which is conservative.**
Same all-off `Define` build, with one harmless-looking top-level statement
added to a module that should disappear:

| Change | Result |
| --- | --- |
| baseline | 367 B |
| `focus.js`: `const FOCUSABLE = [...].join(",")` | 482 B — the array and the call stay |
| `animate.js`: unused `const reduce = matchMedia(...)` | 420 B |
| `focus.js`: top-level `customElements.define(...)` | 428 B |
| `focus.js`: unused `export const registry = new WeakMap()` | 367 B |
| `focus.js`: unused class with `static all = new Set()` | 367 B |

So "no top-level side effects" in recommendation 4 is a rule that needs a
check in CI (the metafile shows a module that should be absent), not a style
preference.

**Terser — the 1214 B is an esbuild limit, not a property of options
objects.** Terser over the report's own C4 bundle (`min/cmp3.out`):

| Call sites of `mountDialog` | terser safe | terser `unsafe: true` |
| --- | --- | --- |
| one, all flags false | 1178 B | **339 B** (gz 228), focus trap and animation removed |
| two, flags `0000` and `0100` | 1297 B | 1297 B |
| two, identical flags | 1297 B | 1297 B |

A generic minifier already matches the `Define` result (367 B) when a
component is used once, with no knowledge of components. It gives up as soon
as there are two call sites, even identical ones. The "3.3×" in §3 (c)
therefore measures esbuild's minifier as much as component awareness. The
bet has to be shown where generic tools stop: a component used several times
with different props, per-page CSS selection, and HTML.

**Minifier gap (the report's open question 6), on the four A1 bundles:**

| Tool | raw total | gzip total | brotli total |
| --- | --- | --- | --- |
| esbuild 0.28.2 | 4930 B | 2443 B | 2009 B |
| terser over the unminified bundle | 4784 B (−3.0 %) | 2375 B (−2.8 %) | 1932 B |
| terser over esbuild's minified output | 4801 B | 2369 B | 1922 B |
| swc | 4899 B | 2357 B (−3.5 %) | 1915 B |
| oxc-minify | 4926 B | 2427 B | 1992 B |

esbuild is within 3–4 % of the best. None of the others can be linked into a
Go binary.

**V2 — sessions with a warm HTTP cache.** The report compares cold loads
only. Cumulative bytes fetched (raw / gzip per file), visiting
index → guide → api → changelog:

| Plan | page 1 | page 2 | page 3 | page 4 |
| --- | --- | --- | --- | --- |
| self-contained (A1) | 532 / 326, 1 req | 1817 / 947, 2 | 3913 / 1879, 3 | 4934 / 2460, 4 |
| `Splitting` (A2) | 751 / 579, 3 req | 2137 / 1443, 6 | 2971 / 1988, 8 | 3142 / 2123, 9 |
| compiler-chosen layer (A6) | 1330 / 781, 2 req | 2252 / 1243, 3 | 3735 / 1933, 4 | 4297 / 2302, 5 |

Compressed, `Splitting` is behind until the fourth and last page and then
ahead by 14 %, with nine requests against four. Two other visit orders give
the same picture (`v/v2.out`). Recommendation 3 holds at this size. The
fixture is small; the crossover moves earlier once shared modules are larger
than the import/export glue.

**V4 / V4b — modern CSS passes through intact.** A stylesheet using
`@layer`, `@property`, `@scope`, `@container`, `@starting-style`,
`@position-try`, `@view-transition`, `@function`, anchor positioning
(`anchor-name`, `position-area`, `position-try-fallbacks`, `anchor()`,
`anchor-size()`), `:popover-open`, `::details-content`, `light-dark()`,
`color-mix()`, relative colours, `interpolate-size`, `calc-size()`, `if()`
and `sibling-index()`: 2099 → 1827 B, 0 errors, 0 warnings, nothing dropped.
With nesting lowered, a nested `@starting-style` is hoisted correctly
(`@starting-style{[popover]:popover-open{opacity:0}}`). For an old target
esbuild rewrites `width<40rem` but leaves `light-dark()` and `inset`
untouched and says nothing: it lowers a subset silently, so browser support
is the compiler's job to state.

**V9 — determinism.** 30 builds of the 4-page site: one result set per
`Splitting` value, identical names and hashes.

### What the report does not cover

1. **Phase 2 has no user JavaScript.** Shell rule S1
   (`specs/later/layout.md:113`) forbids event handlers in shell code, and
   phase 2 has no islands. Every byte of JS on the docs site comes from
   framework-owned behaviour modules: a closed set of a few files. Resolution,
   npm handling and most of §1 are unused. esbuild's job reduces to
   concatenate, tree-shake and minify, which makes the choice low-risk and
   cheap to reverse, and makes "does esbuild have a seam" a smaller question
   than the report's length suggests.
2. **Recommendation 4 conflicts with how the catalog is meant to be written.**
   CLAUDE.md: the design system's core catalog is "authored in plain `.tsx`,
   executed by the compiler in shell code". Behaviour modules with
   `__RG_MODAL__` free identifiers or labelled statements are a second,
   hand-written artefact in a different style. The report does not say who
   maps a component's props to those flags, or how the `.tsx` and the
   behaviour module are kept in step. That mapping is the compiler work the
   bet is about.
3. **The baseline for the bet.** See the terser table. Comparing against
   esbuild alone flatters the result.
4. **Where the bytes are.** The fixture has 0.5–2 kB of JS per page. If
   dialog and dropdown lean on `<dialog>`, `popover` and anchor positioning,
   JS shrinks further and CSS and HTML dominate, where by the report's own
   finding esbuild contributes minification only. I did not measure this; it
   belongs to the platform and CSS reports.
5. **Minor releases can break a build.** #4436 (2026-04-02): a compat-table
   update in a release after 0.27.4 made esbuild reject destructuring for
   Safari < 14.1 targets; closed as expected behaviour. Supports pinning; it also means a version bump
   needs the conformance run.
6. **Inline module scripts.** Open question 2 implies only `iife` can be
   inlined. `<script type="module">` with inline content works for a
   self-contained esm bundle, so the 11 B wrapper is not needed for scoping.

### Does the recommendation hold?

Yes. Using `pkg/api` in-process, with virtual modules, `Write: false`, one
file per page and no fork is supported by every rerun and by my attempts to
break it. Amendments:

- Recommendation 4: keep the `Define` route, but add a build check that a
  module expected to be dropped is absent from the metafile (V7), and pass
  `Sourcemap` + `Sourcefile` to the `Transform` call (V8).
- Recommendation 7: the "second pass over `OutputFiles`" is measured and
  works (V1); name it as the standing answer to "we need linked code".
- §3 (c): do not present 3.3× as evidence for the bet. Measure against
  terser `unsafe` on a page that uses a component more than once.
- Decide how behaviour modules relate to the `.tsx` catalog before writing
  them in the esbuild-friendly style.
