# Phase 2 research — compile-time evaluation of layout components

Key: `evaluation`. Date: 2026-10-04. Machine: darwin/arm64, Go 1.27.1, Node 25.2.1,
esbuild 0.28.2, react/react-dom 19.3.0, tsgo fork at upstream `4f5ddae` (2026-09-25).

Question: how does the builder *execute* layout components at compile time and turn a
page tree into HTML, from inside (or driven by) the Go binary — and what does each way
give the compiler in "component awareness"?

Everything below marked **measured** was run today; commands and files are in
`phase2-research/exp-evaluation/` (index at the end). Nothing in the repository was changed.

## Summary

| | A. Go evaluator over tsgo AST | B. Embedded JS engine | C. Node subprocess |
| --- | --- | --- | --- |
| Works on the fixture | yes, byte-identical HTML | yes (goja, modernc quickjs, wazero qjs), byte-identical | yes |
| Binary growth (real CLI, 27.36 MiB) | ~0 (1,455 lines of Go, no deps) — estimate | quickjs **+2.57 MiB**; qjs +3.37; goja **+24.5 MiB**; esbuild to bundle: +4.68 more | 0 |
| Cost, 101 components | ~1.25 ms eval+render | quickjs 13 ms; goja 29–32 ms; qjs 13 ms + **330–400 ms** VM start | 2.3 ms cold + **27 ms** process start |
| Language covered | a subset we define and maintain | ES2023 (quickjs) | everything |
| Shell-rule violations (7 test pages) | all 7 reported, exact `.rtsx` line:col + component stack (one of them, a pure loop, arguably too strictly) | 3 of the 6 real violations run **silently** unless sandboxed/instrumented | same as B |
| Error positions | span map directly | bundle line:col → esbuild map → span map | same as B |
| Trace (D) | native, with AST provenance | JSON from the runtime; component identity must be injected | same as B |
| Needs at build time | nothing new | esbuild (or tsgo emit) to produce JS | Node on the machine |

Recommendation in one line: **A, behind a narrow interface, with the 73-line JS runtime
under Node as a differential-test oracle in CI, and modernc.org/quickjs named as the
fallback** — conditions for switching are listed under *Recommendation*.

## The fixture

A docs site written as the design system would be, in `.rtsx`, transpiled by the
**shipped** `reactogenic serve` (alpha.1 binary), so every option consumes real phase-1 output.

- `ds/SideMenu.rtsx` — `$Brand`, `$Footer` (fallback children), nested `.map` over a constant nav tree, active link by `href === current`.
- `ds/Dialog.rtsx` — `$Title`, `$Actions`, `size` variant with default, `style={{ maxWidth: SIZES[size] }}`, derived id.
- `ds/Dropdown.rtsx` — `$Trigger`, function slot `$Item` with args `&value &selected`, popover attributes.
- `ds/DocsLayout.rtsx` composes the three; `nav.ts` holds constants; `ds/cx.ts` is the class-name helper.
- Pages: `intro`, `slots`, and `stress` (96 extra `Dialog`s → 101 component calls).

Caveat: I wrote the fixture and the evaluator. It was written first and in ordinary React
style, but it is three components, not a design system.

## What real layout components need (measured on the fixture)

Expression kinds the evaluator met rendering `intro` + `slots`
(`shelleval -kinds`, counts are evaluations):

```
Identifier 410 · CallExpression 126 · PropertyAccessExpression 99 · StringLiteral 76
JsxElement 55 · ObjectLiteralExpression 45 · ParenthesizedExpression 41
ConditionalExpression 40 · BinaryExpression 33 · TemplateExpression 18 · ArrowFunction 17
ArrayLiteralExpression 9 · NumericLiteral 9 · NullKeyword 7 · JsxFragment 3
ElementAccessExpression 3 · AsExpression 2
```

In words — the subset the three components and their pages actually used:

| Needed | Where |
| --- | --- |
| destructured params with defaults | `{ id, size = "md", $Title }` |
| object / array literals, object spread, JSX spread | slot values, `{..._slotProps($X)}` |
| template strings, `+` concatenation | `` `rg-dialog--${size}` ``, `BASE + "/flow"` |
| calls to pure helpers, rest params, `.filter(Boolean).join(" ")` | `cx(...)` |
| `?:`, `&&` on compile-time values | active link, optional badge |
| `.map` over constant arrays, block-bodied arrows with `const` + `return` | nav tree |
| **immediately-invoked arrows** | emitted by the transpiler for an attachment with args |
| element access with a compile-time key | `SIZES[size]` |
| ES module imports of constants and components, `import type` skipped | everywhere |
| `@reactogenic/core` helpers (`isAssigned`, `renderSlot`, `slotProps`, `slotArgs`, `slotEntry`, `slotKey`) | every attachment |

Not needed by the fixture, likely soon: `let` + loops and `switch` in helpers, regex
(slugs for heading ids), `Array.sort` / `reduce`, `Object.fromEntries`, JSON imports,
date/number formatting. Never: classes, `React.Children` / `cloneElement` (slots replace them).

## A. Go-native evaluator over the tsgo AST

**Prototype** (`realbin/internal/shelleval/`, built inside a scratch copy of the repo's Go
module so it calls the real `transpiler.Transpile` and `emit.Map`):

| File | Lines | What |
| --- | --- | --- |
| `eval.go` | 814 | modules, imports, binding patterns, calls, statements, expressions, JSX |
| `builtins.go` | 435 | JS conversions, 11 array + 9 string methods, `Object.keys/values/entries`, `Math`, core intrinsics |
| `render.go` | 206 | HTML serializer (React's escaping and attribute rules), trace |
| total | **1,455** | after `gofmt` |

**Correctness, measured.** For all three pages the output is byte-identical to the JS
runtime's output in Node, goja and quickjs, and equal to `react-dom/server`'s
`renderToStaticMarkup` except for the `<input>` tag (React reorders `name` last and writes `/>`):

```
intro  tiny runtime == react-dom/server (ignoring <input>): true | go evaluator == tiny runtime: true | bytes 2161
slots  … true | … true | bytes 2434
stress … true | … true | bytes 35903
```

**Speed, measured** (`shelleval -n 200`, a fresh evaluator per run):

```
intro   1.71 ms   of which read+transpile+parse 1.84 ms on the last run (i.e. ~all of it)
slots   1.76 ms   front end 1.58 ms
stress  5.74 ms   front end 4.49 ms  → ~1.25 ms for 101 component calls / 219 closure calls
whole process, 3 pages: median 14.7 ms (process floor `reactogenic --version`: 7.6 ms)
```

**How it says "not known at compile time"** — measured on seven violating pages
(`site/src/bad/*.rtsx`); positions are in the `.rtsx` the author wrote, through the phase-1 span map:

```
src/bad/handler.rtsx:8:17 shell-handler: The shell cannot handle events; move this into an island
src/bad/random.rtsx:2:20 shell-nondeterministic: Math.random() makes the shell irreproducible
src/bad/window.rtsx:5:44 shell-dynamic-value: `window` is not known at compile time
src/bad/date.rtsx:3:20 shell-unsupported: NewExpression cannot run at compile time
src/bad/loop.rtsx:3:3 shell-unsupported: ForOfStatement cannot run at compile time
src/bad/state.rtsx:1:1 shell-dynamic-code: "react" is a package: its code cannot run at compile time
    in Counter (src/bad/state.rtsx:12:7)
    in DocsLayout (src/bad/state.rtsx:11:5)
src/ds/Dropdown.rtsx:19:10 shell-type: cannot read `map` of undefined
    in Dropdown (src/bad/typeerror.rtsx:7:5)
```

It **fails closed**: anything outside the subset is an error naming the construct, never a
guess. `Match` / `Switch` over compile-time values just work (`bad/flow.rtsx` →
`<article><p class="note">Experimental</p><b>beta</b></article>`), because the transpiler
has already lowered them to conditionals.

**Failure modes.**

1. *Too strict*: a pure helper with `let` + a loop is rejected (`loop.rtsx`), though it is deterministic. Each such construct is more interpreter.
2. *Closed world*: no npm code at compile time — `clsx`, a markdown renderer, a highlighter cannot run. Framework-owned needs must become Go intrinsics.
3. *Silent divergence where a feature is implemented but differs from JS*. Known in the prototype: `Number → string` in exponent form (tsgo's `jsnum` has the exact algorithm, one bridge alias away), string methods count runes not UTF-16 units, integer-like keys are not enumerated first, 6 of 253 HTML entities (the transpiler's `entities.go` has the table), ~8 of ~45 unitless CSS properties. Each needs a differential test.
4. Runaway recursion: a step budget (5M closure calls) is in; Go stack depth is not guarded.

**How big is a correct one.** My estimate, not measured: 3–4k lines of Go plus a
conformance suite — core ~1.5–2k, builtins ~1k, HTML/attribute rules ~0.6k. The
attribute/style table is needed by **every** option that does not use `react-dom/server`.

**Can the checker already do part of it? Measured** (`cmd/typeprobe`, 16 probes):

| Expression | Type the checker reports |
| --- | --- |
| `BASE`, `` `${BASE}/intro` `` | `"/docs"`, `"/docs/intro"` |
| `SITE_C.name`, `NAV_C[0].href` (`as const` data) | `"Reactogenic"`, `"/docs/intro"` |
| `size === "lg" ? "wide" : "narrow"` | `"narrow" \| "wide"` |
| `BASE + "/flow"`, `SITE.name` (no `as const`) | `string` |
| `` `rg-dialog--${size}` ``, `SIZES[size]`, `1 + 2` | `string`, `number`, `number` |
| `cx("rg-sidemenu", collapsible && "…")`, `NAV.map(…)` | `string`, `string[]` |

`GetConstantValue` returned nil for all 16, including the enum members `E.B` and `E.C`
(UNVERIFIED why; I did not dig). `internal/evaluator` is 168 lines: numbers and strings in
enum initialisers only. So the checker is a **classifier, not an evaluator**: it can tell a
primitive from JSX (needed later for holes) and prove a prop is a literal union; it cannot
produce the value of a helper call. The binder's `NameResolver` (already bridged as
`rtsx.ResolveValue`) is what an evaluator uses.

**Bridge cost.** The prototype found 67 `Kind`s by name because the bridge exports 35;
the real thing is one patch adding aliases — within the bridge's "aliases and one-line
wrappers" rule. Node accessors (`n.Parameters()`, `n.AsBinaryExpression()`, …) are already
reachable through `rtsx.Node`.

**Spec consequence worth a decision.** With evaluation, S2 ("no `?:`, `.map`") collapses
into S3 ("compile-time values only"): a conditional or loop over compile-time values is
deterministic and simply evaluates; over a runtime value it is an error at that expression.
The side menu needs exactly this (`> OPEN: loops over constants` in `layout.md`).

## B. Embedded JS engine

Pipeline tested: esbuild bundles the emitted TSX with a custom automatic JSX runtime
(`site/shell-runtime/jsx-runtime.ts`, 73 lines: lazy VDOM → HTML string + trace) into one
27 KB IIFE; the engine evaluates it and calls `rgRender(page)`.

**Which engines, today.**

| Engine | State (source, date) | ES level, measured (`cmd/features`) |
| --- | --- | --- |
| `github.com/dop251/goja` @ 2026-10-02 | pure Go. README: "Full ECMAScript 5.1 support", "Most of ES6 functionality, still work in progress" (github.com/dop251/goja, read 2026-10-04) | far beyond the README: classes, private fields, `?.`/`??`, async/await, BigInt, `toSorted`, `Set.union`, iterator helpers all pass. Fails: async generators, `Object.groupBy`, `Intl` |
| `modernc.org/quickjs` v0.25.0 (2026-10-01) | pure Go (ccgo translation of QuickJS 2026-06-04), BSD-3; "ES2023 … modules, asynchronous generators, proxies and BigInt"; "has no Intl"; "VM is not safe for concurrent use" (pkg.go.dev/modernc.org/quickjs, read 2026-10-04) | all 18 language probes pass; `Intl` missing |
| `github.com/fastschema/qjs` v0.0.6 | QuickJS-NG compiled to wasm, run by wazero; no cgo (github.com/fastschema/qjs, read 2026-10-04) | same engine family; not probed separately |
| v8go forks | cgo + prebuilt static V8. `tommie/v8go` merged Windows amd64 on 2026-10-02, no Windows arm64; other forks claim it (search results, 2026-10-04, UNVERIFIED beyond that) | ruled out: the release build is `CGO_ENABLED=0` for six targets (`scripts/build-binaries.sh:23`) |
| `grafana/sobek` | goja fork with ESM for k6 (github.com/grafana/sobek, read 2026-10-04); not tested | — |

None of the three has `console`, `setTimeout`, `queueMicrotask`, `TextEncoder`, `URL`,
`structuredClone` (measured: all `undefined`).

**Binary size, measured** on the real CLI (`realbin/`, same flags as
`build-binaries.sh`, darwin/arm64; the rebuilt baseline is 28,685,106 B vs 28,668,594 B shipped):

| Build | Size | Growth |
| --- | --- | --- |
| reactogenic | 27.36 MiB | — |
| + modernc quickjs | 29.93 MiB | **+2.57** |
| + fastschema qjs (wazero) | 30.73 MiB | +3.37 |
| + esbuild (`pkg/api`) | 32.03 MiB | +4.68 |
| + esbuild + quickjs | 34.46 MiB | +7.10 |
| + goja | 51.85 MiB | **+24.49** |

goja alone adds 8.45 MiB to an empty program (9.95 vs 1.50 MiB) but 24.5 MiB to ours.
My reading — inferred, not proven: goja's reflection over methods stops the linker pruning
unused methods across the whole binary, and tsgo has very many.

All three cross-compile with `CGO_ENABLED=0` for the six release targets (measured;
quickjs 4.7–5.1 MB, qjs 5.7–6.1 MB, goja 9.8–10.6 MB standalone).

**Time, measured** (`gob/cmd/*`, shell bundle, es2022; es2015 gave the same picture):

| | new VM | eval 27 KB bundle | `intro` (5 comps) first / median | `stress` (101 comps) first / median |
| --- | --- | --- | --- | --- |
| goja | 0.03–0.09 ms | 1.2–2.0 ms | 1.5–1.8 / 0.81 ms | 30.8–42.6 / 28.6–31.9 ms |
| modernc quickjs | 0.2–0.44 ms | 2.7–3.2 ms | 1.0 / 0.80 ms | 12.7–13.7 / 13.0–13.2 ms |
| qjs (wazero) | **332–399 ms** | 3.0–3.5 ms | 1.1–1.5 / 0.85 ms | 13.1–13.6 / 13.9 ms |

qjs paid its start on every one of four process runs; I did not explore its cache options.
For a 4-page site every engine is fast enough; quickjs is ~10× slower per component than
the Go evaluator and it does not matter.

**What breaks, measured.**

- `react-dom/server` does not load in any of the three: `ReferenceError: MessageChannel is not defined` at module init. The engine route means **our own JSX→HTML runtime**, not React's.
- A bundle that merely imports `react` fails at load with `'console' is not defined` until a stub is installed.
- `localeCompare` sort of `["b","a","C"]`: quickjs `Cab`, goja and V8 `abC`. No ICU — the same component can order differently at compile time and in a browser island.
- **Shell-rule violations run silently.** Same seven pages, quickjs, plain:

  | Page | Go evaluator | JS engine |
  | --- | --- | --- |
  | `onClick={…}` | shell-handler | **renders, handler dropped** |
  | `new Date()` | error | **renders** |
  | `Math.random()` | shell-nondeterministic | **renders `id="x0.38226…"`** |
  | `window.location` | shell-dynamic-value | `ReferenceError: 'window' is not defined` |
  | `useState` | shell-dynamic-code | `TypeError: cannot read property 'useState' of null` |
  | `undefined.map` | shell-type, 19:10 | `TypeError … at Dropdown (<eval>:1292:120)` |
  | `let` + loop | rejected | renders `<p>6</p>` — correctly; it is pure |

  With a sandbox prelude (a throwing `Proxy` for `Date`, a throwing `Math.random`) the two
  nondeterminism cases become errors (measured). Handlers need a check in the runtime.
  So B needs the rules enforced *beside* execution: sandboxing plus a static pass.
- **Positions** arrive in bundle coordinates. Two hops work (measured, `mapback.mjs`):
  `bad.js:1292:120` → esbuild map → emitted TSX `Dropdown.rtsx:20:10` → phase-1 map →
  `.rtsx` line 19. My quick lookup lost the column on the second hop; in Go the span map
  would give it exactly. A v3 source-map *reader* is new code (phase 1 only writes them).
- **Component identity.** The bundler renamed colliding components — the trace says
  `Page3`, `Page5`, `Page6` for three files' `Page`. Names are not identities; the compiler
  must inject one (`file#export`) per component.

## C. Node subprocess

Same bundle, run by `node` (measured, `bench-spawn.mjs`, 20 runs each):

```
node -e 0 (bare startup)           median 27.2 ms
node run shell bundle (3 pages)    median 34.7 ms     (bundle eval 1.6 ms; stress 2.3 ms cold, 0.74 ms warm)
node run react bundle (3 pages)    median 42.5 ms     (bundle eval 5.7–7.2 ms; stress 2.9 ms cold, 0.43 ms warm)
```

- `react-dom/server` works here and is the natural **reference**: it is what an island's React will agree with. Its bundle is 632 KB (231 KB minified) against 27 KB for the tiny runtime.
- Gotcha, measured: the browser build of `react-dom/server` kept the process alive after rendering (the run hit the 120 s timeout) until I added `process.exit(0)`.
- Determinism: output is a function of the Node version (ICU data, V8). Pinning is the user's problem, not ours.
- Availability: Node exists wherever `@reactogenic/cli` came from npm. It is not guaranteed next to the binary inside the VS Code extension, nor for the long-term Go server. So C cannot back as-you-type diagnostics in the language server.
- Errors: as in B (stack in bundle coordinates, two hops), plus `--enable-source-maps` does the first hop for free.
- esbuild's `jsxDev: true` passes each element's `{fileName, lineNumber, columnNumber}` to the runtime (measured: `Dialog` at `src/pages/slots.rtsx:8:7`), in emitted-TSX coordinates — a cheap use-site for traces in B and C.

## D. The trace

What the bet needs is not the HTML but: which components, with which compile-time
props / slots / variants, at which DOM path, producing what.

Measured from the Go evaluator (`out/goeval.slots.trace.json`):

```
Slots @ src/pages/slots.rtsx:1:1 -> /0 (2434 B)
  DocsLayout @ src/pages/slots.rtsx:6:5 -> /0 (2434 B)
    SideMenu @ src/ds/DocsLayout.rtsx:10:7 -> /0/0 (754 B)
    Dropdown @ src/ds/DocsLayout.rtsx:15:9 -> /0/1/1 (520 B)
    Dialog   @ src/pages/slots.rtsx:8:7    -> /0/2/1 (416 B)   props {id:"example", size:"sm", $Title:{className:"custom-title", children:<Fragment>}}
    Dialog   @ src/ds/DocsLayout.rtsx:24:7 -> /0/3   (520 B)   props {id:"search",  size:"lg", $Title:{…}, $Actions:{…}}
```

That already answers "this page uses `Dialog` in sizes `sm` and `lg`, `Dropdown` once
aligned `end`, `SideMenu` collapsible" — the input for per-page CSS and JS.

| | How the trace is exposed | What it can hold beyond (component, props, path) |
| --- | --- | --- |
| A | a Go struct built during evaluation; use sites already in `.rtsx` coordinates; declaring file known | per-attribute and per-text **provenance** (every value carries its AST node), branches taken, props read, which slot filled which attachment — all without touching the code |
| B | the JS runtime returns JSON next to the HTML (measured: 101 entries on `stress`, identical in goja/quickjs/Node) | the same only by **instrumenting** the code (coverage-style transform); identity must be injected |
| C | as B, over stdout | as B; real React could add its own component stack |

For phase 2 (everything shell, no holes) all three give enough to test the bet. The
difference appears with islands: a slot body with runtime values needs **partial**
evaluation. In A an unknown is a first-class value — at a JSX child it becomes a hole, in a
condition it is an error at that node, and "known at compile time → hole optimised away"
is the default. In B/C the compiler must decide binding times statically beforehand and
rewrite unknown `{expr}` into hole markers before running the code.

## Per-option assessment

| | Correctness risk | Effort | Component awareness | Errors in source terms | Islands later |
| --- | --- | --- | --- | --- | --- |
| **A** | drift from JS where we implement a feature; none where we refuse it | ~3–4k lines + conformance suite (estimate) | deepest: trace + provenance | native, exact, with component stack; runs in the language server on the tree it already has | React renders the same `.tsx`; evaluator handles holes natively. Must keep its HTML rules equal to React DOM's |
| **B** quickjs | none for language semantics; no `Intl`, different collation; ccgo-translated C, one maintainer, manual value lifetimes | runtime ~75 lines TS, embed ~300 Go, source-map reader, sandbox, static rule pass, esbuild or tsgo emit to produce JS — my estimate ~1–1.5k | trace by cooperation; no provenance | two hops; rules not enforced by execution | same code in both places; holes need a static pre-pass |
| **C** Node | lowest; can use real React | smallest | as B | as B | as B; unavailable to the binary alone |

Prior art, for calibration: StyleX's compiler uses "a forked version of `path.evaluate()`
from Babel" and can evaluate "simple pure arrow functions" (facebook/stylex issue #726,
via search 2026-10-04) — option A in a narrow domain. Tamagui "evaluates styled components
away using both the AST and Node VM code evaluation", bundling with esbuild to a temp file
(tamagui.dev/docs/intro/why-a-compiler, via search 2026-10-04) — A with C as fallback.
Prepack, the general JS partial evaluator, is archived (github.com/facebookarchive/prepack)
— the warning against letting A grow into "all of JavaScript".

## Recommendation

**Phase 2: option A — a Go evaluator for the shell-safe subset, in `go/internal/`.**

1. It is the only option where "the compiler is aware of components" is literal: values carry their AST nodes, so the trace, the provenance and the diagnostics are the same data structure. That is what the bet is about.
2. Measured: 1,455 lines render SideMenu / Dialog / Dropdown pages byte-identical to JS, in ~1.25 ms per 100 components, with no dependency and no binary growth.
3. Measured: it reports all shell-rule violations at the exact `.rtsx` position; a JS engine ran three of them silently. B and C therefore need a separate static pass anyway — and that pass plus the binding-time analysis islands will need is a large part of A.
4. It runs inside the language server, which C cannot and B can only through a bundle.

Guardrails that make this safe rather than a tar pit:

- **One interface**: `Evaluate(page) → (tree, trace, diagnostics)`. Input is the emitted TSX, output is data. B can replace the implementation without touching the rest of the builder.
- **Fail closed**: outside the subset is `shell-unsupported` naming the construct. Never approximate.
- **Differential conformance**: keep the 73-line JS runtime; CI renders every fixture with Node (and `react-dom/server` as the reference for attribute rules) and compares bytes — the same ratchet idea as `testdata/passing.txt`. Node is a test dependency only.
- **Framework needs are Go intrinsics** (`@reactogenic/core` helpers already are in the prototype), not interpreted library code.

What would change my mind — switch to **B with `modernc.org/quickjs`** (+2.57 MiB, pure Go, six targets, ES2023; not goja: +24.5 MiB; not qjs: 330–400 ms start) if any of these happens:

- the docs site or the catalog needs **third-party JS at compile time** (markdown, syntax highlighting, `clsx`-style helpers) and Go-native replacements are not acceptable;
- the evaluator passes ~4k lines, or needs mutation-heavy code, regex or classes to render the framework's own components;
- differential tests keep finding divergences in features we chose to implement;
- shell code must run components we do not own.

C stays what it is good at: the oracle.

## Open questions

1. **Docs content**: are pages hand-written `.rtsx`, or Markdown/MDX with highlighted code? The second needs either Go-native intrinsics (a markdown and a highlighting library in the binary) or a JS engine — this alone can flip the recommendation.
2. **S2 vs S3**: accept "conditionals and loops over compile-time values are allowed in shell code" (the `> OPEN: loops over constants` note)? The side menu needs it, and evaluation enforces it for free.
3. **Local mutation**: is `let` + a loop inside a pure helper shell-safe? JS engines accept it; the prototype rejects it. Allowing it is more interpreter (assignment, loops, `switch`).
4. **Reference for HTML rules**: is `react-dom/server`'s output the definition of "correct" (so shell and island markup agree), including its quirks (`<input … name="q"/>`, attribute order)? Or do we define our own serializer and test islands against it?
5. **Component identity in the trace**: `file#export`? Needed by every option; B/C must inject it (bundlers rename).
6. Does a builder that embeds esbuild anyway (per the long-term architecture) change the size argument? esbuild + quickjs is +7.10 MiB together.
7. UNVERIFIED: whether tsgo's own emit (type stripping + JSX transform, already in the binary) plus quickjs's ES-module loader could feed B without esbuild. Not tested.
8. UNVERIFIED: why `Checker.GetConstantValue` returned nil for enum member accesses in the probe.
9. Not measured: A's exact binary growth (estimated negligible), Go stack behaviour on deep recursion, qjs with a compilation cache, Linux/Windows timings.

## Experiment index

All under `phase2-research/exp-evaluation/`.

| Path | What |
| --- | --- |
| `site/src/` | the fixture (`.rtsx`), `bad/` = seven rule violations + `flow.rtsx` |
| `site/transpile.mjs` | runs the shipped `reactogenic serve` over the fixture → `X.rtsx.tsx` + `.map` |
| `site/shell-runtime/jsx-runtime.ts` | the tiny JSX → HTML + trace runtime (B, C) |
| `site/build.mjs`, `build-bad.mjs` | esbuild bundles (shell es2022/es2015/dev, react) |
| `site/run-node.mjs`, `bench-spawn.mjs`, `run-bad.mjs`, `mapback.mjs` | option C runs, spawn cost, errors, two-hop mapping |
| `site/out/` | bundles, HTML per option, traces |
| `gob/cmd/{goja,quickjs,qjs}` + `internal/harness` | option B timing harness |
| `gob/cmd/features`, `gob/cmd/qjserr` | ES feature probes; errors plain vs sandboxed |
| `realbin/` | scratch copy of `go/cmd` + `go/internal` (tests removed), tsgo by `replace` to the repo; engine-linked CLI builds for size |
| `realbin/internal/shelleval/` | **the option A prototype** |
| `realbin/cmd/shelleval`, `realbin/cmd/typeprobe` | its driver (`-n`, `-trace`, `-kinds`); the checker probe over `probe/probe.ts` |

Note for whoever continues: the shared Go build cache was at 105 GB and free disk fell
from 61 GB to 48 GB during this session (nine researchers building in parallel). I deleted
only my own scratch binaries.

## Verification (independent)

Skeptic pass, 2026-10-04, same machine (darwin/arm64, Go 1.27.1, Node 25.2.1). Everything
below was re-run from the author's sources copied to `phase2-research/exp-evaluation-verify/`
(binaries rebuilt from source, bundles rebuilt with esbuild 0.28.2, `.rtsx` re-transpiled
with the shipped alpha.1 binary). The author's text above is unchanged.

**Bottom line.** The measurements reproduce, almost to the byte. Two conclusions drawn
from them do not survive, and both were arguments for A over B:

1. **React's own static renderer runs in the embedded engines with no polyfills.** The
   `MessageChannel` failure comes from the streaming half of the `react-dom/server` entry.
   Bundling `react-dom/cjs/react-dom-server-legacy.browser.production.js` instead gives
   `renderToStaticMarkup` in modernc quickjs and goja, byte-identical to Node. So "the
   engine route means owning the JSX-to-HTML serializer" is wrong.
2. **A JS engine reports all six real violations with about eight lines** (the author's
   5-line sandbox prelude plus a 3-line check in `jsxDEV`). No static pass is needed for them.

And the "byte-identical" result is specific to a fixture written by the same author as both
renderers: on 36 one-line cases of ordinary design-system markup the Go evaluator matched
React on 12, refused 5, and produced **different HTML silently on 19**.

### Verdict per claim

| # | Claim | Verdict | What I ran and found |
| --- | --- | --- | --- |
| 1 | Go evaluator == JS runtime on the fixture, byte-identical | **confirmed, narrowly** | Rebuilt `shelleval` and the shell bundle: `cmp` equal for intro / slots / stress (2161 / 2434 / 35903 B). `wc -l` = 1455. Does not generalise: see *Variant cases*. |
| 2 | Tiny runtime == `renderToStaticMarkup` except `<input>` | **confirmed on the fixture** | Same diff as reported, react-dom 19.3.0. The fixture avoids every attribute class where the runtime is wrong; on the variant cases it matches React on 20 of 36. |
| 3 | Evaluator cost: ~1.25 ms / 101 components, 1.7 ms per page, 14.7 ms per process | **confirmed** | 3 runs of `-n 200`: intro 1.67–1.76, slots 1.72–1.77, stress 5.56–5.59 ms, front end 4.2–4.6. 20 process runs: median 14.6 ms; `reactogenic --version` 7.6 ms. The 1.25 ms is a subtraction of one run's front end from a 200-run mean (range 0.9–1.4 ms). |
| 4 | Violations reported at exact `.rtsx` positions; fails closed | **partly** | All seven positions reproduce, and a position inside a slot body is right (`slotpos.rtsx:6:33`). Columns count runes, as in phase 1. Syntax outside the subset is refused (16 of 16 unsupported constructs tried, `src/fc/`). But: unimplemented *semantics* are silent, not refused (below); `ref={fn}` on an element is dropped without a diagnostic; `useState` is reported at `1:1` (the import), not at the call; and unbounded recursion ends in Go's `fatal error: stack overflow`, which kills the process (the author lists this as unguarded — in the language server it would take the server down). |
| 5 | Engine runs 3 of 6 violations silently, so B and C need sandboxing plus a static pass | **partly — the conclusion is refuted** | Plain run reproduces (handler, `Date`, `Math.random` render). With the prelude plus `if (typeof type === "string") for (k in props) if (typeof props[k] === "function") throw …` in `jsxDEV`, quickjs reports all six; the handler error carries `handler.rtsx:5:70` (emitted-TSX coordinates) from `jsxDEV`'s source argument. |
| 6 | modernc quickjs: pure Go, ES2023, six targets, +2.57 MiB | **confirmed** | Rebuilt: 31,380,386 vs 28,685,106 B. Six `CGO_ENABLED=0` cross-builds of `only-quickjs` succeed (4.9–5.3 MB with my flags). New: the **real CLI** with `-tags quickjs` builds for windows/arm64, 30,264,320 B vs 27,755,520 B shipped. pkg.go.dev read 2026-10-04: v0.25.0, 2026-10-01, BSD-3, "ES2023 … modules, asynchronous generators, proxies and BigInt", no `Intl`, VM not goroutine-safe, QuickJS 2026-06-04. |
| 7 | goja +24.5 MiB in the real CLI | **confirmed, and the cause is now shown** | Rebuilt: 54,368,834 B. goja calls `reflect.Type.Method(i)` / `Value.Method(idx)` (`object_goreflect.go:215,596`). Control: the real CLI importing only `text/template` grows to 48,232,914 B (**+18.64 MiB**) — the same linker effect without goja. See *Missed* 1. |
| 8 | Engine timings | **confirmed** | quickjs stress 12.2–12.8 ms, goja 28.1–30.2 ms, qjs new VM 331–343 ms. New: with `qjs.Option{CacheDir}` the start is 452 ms on the first run and **79 ms** after, with a 5.8 MB cache directory. Still far behind quickjs's 0.2 ms; the rejection stands. |
| 9 | `react-dom/server` unusable in an embedded engine without polyfills; engine route = own serializer | **refuted** | The literal error reproduces for the `react-dom/server` browser entry (and with `MessageChannel` stubbed it then needs `TextEncoder`). The legacy static build needs nothing: 331 KB bundle (128 KB minified), no stubs at all. quickjs: eval 17–24 ms, stress 20 ms; goja: eval 9 ms, stress 49 ms. Output `cmp`-equal to Node's `react-dom/server` on all three pages, and equal on 36 of 36 variant cases. With a tracing `jsx` wrapper over `react/jsx-runtime`: 133 KB minified, trace 101 on stress, same HTML. Caveat: the file is not in react-dom's `exports` map, so it is reached by path/alias — a private layout that can change between React versions. |
| 10 | Node: ~27 ms start + ~8 ms; react bundle keeps the process alive | **confirmed** | 27.2 / 36.1 / 43.2 ms medians. Without `process.exit` the react bundle is still alive after 6 s; the legacy-build bundle exits by itself in 0.08 s. |
| 11 | Engine errors need two mapping hops; a source-map reader is new code | **confirmed, small** | `mapback.mjs 1292 120` → `Dropdown.rtsx 20:10` → line 19. `go/internal/emit/sourcemap.go` (137 lines) only writes. The second hop needs no reader in Go (the span map is in memory); the first is a VLQ decoder of about 20 lines, and `jsxDEV` positions avoid it for element-level errors. |
| 12 | Every option delivers (component, props, path); bundlers rename components | **confirmed** | `goeval.slots.trace.json`: 6 entries with use site, declaring file, props, path, bytes. `Page3` / `Page5` / `Page6` reproduce in Node and quickjs. |
| 13 | The checker cannot be the evaluator | **confirmed** | `typeprobe` output is identical to the table. The count is 6 single literals + 1 union + 2 enum-literal types rather than "7", which changes nothing. The author's UNVERIFIED item is resolved: `GetConstantValue` (`internal/checker/services.go:870`) returns a value only for an `EnumMember` node or an access to a **`const enum`** member; the probe's enum is not `const`, so nil is expected. `evaluator.go` is 168 lines. |
| 14 | v8go ruled out by `CGO_ENABLED=0` | **confirmed** | `scripts/build-binaries.sh:23`. github.com/tommie/v8go README, read 2026-10-04: cgo required; prebuilt V8 for Linux, macOS, Android; "Windows amd64 is supported with Go 1.27 or newer"; no Windows arm64. |
| 15 | A production evaluator is 3–4k lines of Go | **unverifiable; likely low if React defines "correct"** | An estimate. For scale: React's own host-element serialisation in the legacy build is about 2,300 lines of JS (lines 468–2,787, `pushStyleAttribute` to `createRenderState`; not all of it attribute rules), against the "~0.6k" budgeted for HTML rules. The estimate also omits package/module resolution, re-exports (`export { x } from`, `export { X as default }`, `import * as` are all refused today), and the Go copies of `@reactogenic/core`'s 201 lines. |

### Variant cases (new experiment)

`exp-evaluation-verify/site/src/variant/c00…c35.rtsx`: 36 one-element pages of markup a
dialog, dropdown, side menu or docs page would plausibly contain. Rendered three ways
(`run-variant.mjs`, results in `out/variant.results.json`):

```
{ cases: 36, goEqTiny: 27, tinyEqReact: 20, goEqReact: 12, goRefused: 5, goSilentDiff: 19 }
```

Silent differences from React shared by the Go evaluator **and** the 73-line runtime:

| Source | React 19.3 | Go evaluator / tiny runtime |
| --- | --- | --- |
| `aria-expanded={false}` | `aria-expanded="false"` | attribute dropped |
| `aria-hidden={true}`, `aria-modal={true}` | `="true"` | `=""` |
| `data-open={true} data-closed={false}` | `"true"` / `"false"` | `""` / dropped |
| `style={{ gridRow: 2, aspectRatio: 2, columnCount: 3, WebkitLineClamp: 3, "--rg-gap": 4 }}` | unitless | `2px`, `3px`, `4px` |
| `defaultValue`, `defaultChecked`, `<textarea defaultValue>` | `value=`, `checked=`, text child | written as `defaultValue="…"` |
| `strokeWidth`, `fillRule`, `strokeLinecap` | `stroke-width` … | camelCase kept |
| `contentEditable`, `draggable`, `spellCheck={false}` | `"true"` / `"false"` | `""` / dropped |
| `crossOrigin`, `suppressHydrationWarning` | `crossorigin`, omitted | camelCase, written |

Silent differences from **JavaScript itself**, in the Go evaluator only (the tiny runtime is right):

| Source | JS | Go evaluator |
| --- | --- | --- |
| `&copy; &mdash; &rarr; &hellip;` in JSX text | `© — → …` | `&amp;copy; &amp;mdash; …` (double-escaped) |
| `"a😀b".length` | 4 | 3 |
| `Object.keys({ b:1, 2:1, a:1, 1:1 })` | `1,2,b,a` | `b,2,a,1` |
| `{-0}` | `0` | `-0` |
| `"10" == 10` | true | false |
| `"～" < "😀"` | false | true |

The author lists three of these six as known (entities, string length, key order). The point is the category: they are
implemented-but-different, so "fail closed, never approximate" does not cover them. The
aria rows matter most for phase 2 — `aria-expanded`, `aria-hidden` and `aria-modal` are
exactly what the three components emit.

Refused cleanly (as designed): `dangerouslySetInnerHTML`, `.sort()`, `.replace(/re/)`,
`.toFixed()`, `JSON.stringify`. Heading slugs and `JSON` are ordinary docs-site needs.

### What the report missed

1. **`text/template` costs the binary 18.6 MiB.** Measured: real CLI 28,685,106 B →
   48,232,914 B from one `template.Execute`. The linker keeps every exported method once
   reflective method lookup is reachable, and tsgo has very many. The long-term
   architecture says "Go templates for shells" and "Go server": if either means
   `text/template` or `html/template` linked into this binary, it grows by two thirds.
   This is the same mechanism that makes goja cost 24.5 MiB. Any future dependency needs
   this check (esbuild and modernc quickjs pass it).
2. **Option B has two forms, and the report only tested one.** B1: own 73-line serializer
   (what was measured). B2: React's static renderer in the engine (works, above). B2 gives
   React-exact HTML with no attribute table to own, the real `@reactogenic/core` instead
   of Go copies, and hooks/context that work at build time (`useId`, context) — at the
   cost of 128 KB of JS to embed or load, ~17 ms to evaluate it, and no DOM path in the
   trace unless markers are added.
3. **Provenance is not in the prototype.** Recommendation point 1 says "values carry
   their AST nodes". In `eval.go` only `Element` has a `node`; strings, numbers and
   objects are bare Go values, and `cx(...)` returns a plain string. Per-attribute
   provenance means every string operation propagates origins — unbuilt and uncounted in
   the estimate. For phase 2 the trace (component, props) is what CSS and JS selection
   need, and B delivers the same trace (verified: 101 entries, identical).
4. **Partial evaluation for islands is asserted, not shown.** In the prototype an unknown
   is an error, not a value. It is the strongest long-term argument for A and has no
   experiment behind it; B's alternative (sentinel values or a pre-pass) is equally untested.
5. **The differential oracle must be React, not the tiny runtime.** The guardrail reads as
   "compare with the 73-line runtime, React for attribute rules". The runtime shares the
   evaluator's tables (`RENAME`, `UNITLESS` are the same lists in both files), so agreement
   between them proves little: 27 of 36 agree while only 12 of 36 are right.
6. **Sibling reports already put esbuild in the binary** (esbuild-seams.md, prior-art.md,
   css.md: `pkg/api`, +4.68 MiB measured here). With esbuild present, B's increment is
   quickjs alone (+2.57 MiB) and "needs at build time: nothing new" stops separating A from B.
   codebase.md also reports that tsgo's emitter turns TSX into JS through one bridge
   function, which answers open question 7 in part.
7. **Design-system packaging.** The evaluator refuses every non-relative import
   (`shell-dynamic-code`). Components delivered as a workspace package need module
   resolution in the evaluator, and must ship TSX source rather than compiled JS.

### Does the recommendation hold?

**Not as argued.** A works and is fast; nothing here shows it cannot be built. But the case
for A *over* B rested on four points, and after re-testing:

| Report's reason for A | After verification |
| --- | --- |
| Only option where awareness is literal (provenance) | Not implemented; the trace both options produce is the same |
| Byte-identical, 1.25 ms, no dependency | True on the fixture; speed is irrelevant at 4 pages (the report says so); 19 of 36 ordinary cases silently wrong |
| JS engine runs 3 of 6 violations silently, needs a static pass | 8 lines fix all six |
| Runs in the language server | True for A; B is also in-process |

What is left for A: default-deny globals, direct `.rtsx` positions for errors thrown inside
component code, zero dependencies, and an unproven path to partial evaluation. What it
costs: a second implementation of JS semantics, of React DOM's attribute rules and of
`@reactogenic/core`, each of which must be kept equal to the original by tests.

Two of the report's own switch conditions are already met or nearly so: "differential
tests keep finding divergences in features we chose to implement" (19 silent of 36 here), and
"the docs site needs third-party JS at compile time" (open question 1 — unanswered, and a
docs site with highlighted `.rtsx` samples usually does).

The decision turns on two questions only the owner can answer:

- Must shell HTML equal what React renders for the same component (islands later)? If yes,
  B2 gets that by construction and A must chase it.
- Is docs content hand-written `.rtsx`, or Markdown with highlighted code?

Suggested next step before committing: keep the `Evaluate(page) → (tree, trace,
diagnostics)` interface from the report, and put quickjs + React's legacy renderer behind
it as the first implementation to compare against the prototype on a fixture that includes
the variant cases. Both exist as working experiments now.

### Files

`exp-evaluation-verify/`: `site/` (rebuilt bundles; `build-legacy.mjs`, `build-trace.mjs`,
`rgtrace/jsx-runtime.ts`, `entry-react-legacy.ts`, `entry-react-trace.ts` — React in the
engine; `gen-variant.mjs`, `build-variant.mjs`, `run-variant.mjs`, `src/variant/`,
`out/variant.results.json` — variant cases; `src/fc/` — 30 fail-closed probes;
`shell-runtime2/`, `build-bad2.mjs` — handler check), `realbin/` (`build-all.sh`,
`cmd/reactogenic/x_tmpl.go` — the `text/template` control), `gob/` (`xbuild.sh`,
`cmd/variantq`, `cmd/qjs` with `CacheDir`). Size-test binaries were deleted after measuring.
