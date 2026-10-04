# Phase 2 research — the existing code, mapped for the builder

Key: `codebase`. Date: 2026-10-04. Repo at `cf80ce6` (branch
`rgp2-000-builder-research`), read only. Paths are relative to
`/Users/msnitkina/code/reactogenic/reactogenic` unless absolute. Experiments:
`/private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-codebase/`
(`$E` below). Machine: M2 Pro, macOS 27.0, go1.27.1, node v25.2.1.

## Summary of what was measured

| # | Experiment | Result |
| --- | --- | --- |
| E1 | `$E/probe`: mapped program of a 4-file docs page (`$E/site`), no patch, no new bridge function | program 71 files in 29.7 ms; module graph from the page in 73 µs; types, literal values, declaring files, contextual slot types all answered (14 ms, first check of the file) |
| E2 | `$E/constprobe`: read a constant's *value* off the checker | `const current = "/guide/slots"` → `"/guide/slots"`; `[...] as const` → full structure; `readonly NavItem[]` (no `as const`) → **type only, no value** |
| E3 | `$E/buildprobe`: esbuild Go API + a plugin whose `OnLoad` runs `transpiler.Transpile`, JSX automatic with a 26-line React-free runtime, run in Node | the page (slots, `Each`, `Match`, 2 segment roots) → 533 bytes of HTML, **no `react` module in the bundle**; esbuild build 13.2 ms, of which 3.9 ms the transpiler (3 `.rtsx` files) |
| E4 | `$E/tsemit`: tsgo's own emitter on the emitted TSX through one experimental bridge function (25-line file) (in a *copy* of the vendored tree) | `.tsx` → `.js` with `jsxImportSource`, 0.47–2.4 ms per file, 0 diagnostics — JS without esbuild is possible; bundling is not |
| E5 | binary size, stripped, `CGO_ENABLED=0`, darwin-arm64 (`$E/sizes.txt`) | `reactogenic` today 27.34 MB; program+checker only 18.03 MB; the same + esbuild `api.Build` 23.63 MB → **esbuild costs +5.6 MB (≈ +20% of the release binary)**; cross-compiles to linux/amd64, linux/arm64, windows/amd64, windows/arm64, darwin/amd64 (22.9–25.0 MB) |
| E6 | esbuild in the module graph | `go get github.com/evanw/esbuild@latest` → v0.28.2 (Go proxy, 2026-10-04); its only requirement is `golang.org/x/sys v0.0.0-20220715151400`; MVS keeps tsgo's `v0.48.0` — no conflict |

## 1. The pipeline today

### 1.1 Inputs, passes, outputs

`go/internal/transpiler/transpiler.go`

```go
type Input struct {
    Files     map[string]string                     // entry (and others), by path
    Entry     string
    UntilPass int                                   // 0: all
    ReadFile  func(path string) (string, bool)      // siblings: segment files, the ambiguous .tsx
    Tolerant  bool                                  // editor: recovered tree, never fails
}
func Transpile(in Input) (Output, error)            // error = transpiler bug, never a user error
```

Passes (`var passes`, transpiler.go:150): each parses its input with
`rtsx.ParseRTSX`, binds (`rtsx.Bind`), returns `[]emit.Edit`; `emit.Apply`
writes the next text and a map; `Map.Then` composes it back to the source.
Repeating passes run until they return no edits (`maxRuns = 1000`).

| # | name | func (file) | what |
| --- | --- | --- | --- |
| 0 | checks | `checks` (transpiler.go:516) → `syntax.Check`, `CheckFlowAsValue`, `CheckSlotTags`, `CheckSegments`, `c.checkSegmentFiles`, `c.checkAmbiguousModule` | errors on the source tree; no edits |
| 1 | shorthand props | `shorthand` (shorthand.go) | `value` → `value={value}` when `syntax.Binding` finds a value binding |
| 2 | flow lowering (repeat) | `flow` (flow.go) → `lowerMatch`, `lowerSwitch` (switch.go) | `Match`/`Switch` → `?:`; drops their import (`dropImports`) |
| 3 | slot hoisting (repeat) | `slots` (slots.go) → `renderSlots`/`renderSlot` (slotrender.go), `removeOrphans`, `hoist`, `coreImport` | attachments → `_isAssigned(...) ? <el {..._slotProps}>{_renderSlot}</el> : fallback`; slot elements → `$X={{…}}` props |
| 4 | segment roots | `segments` (segments.go) | `#name` → `id="name"`, default import of the file found (`SegmentExtensions`), nested element |

`Output` (transpiler.go:68) — everything the builder gets per file:

| Field | Type | Meaning |
| --- | --- | --- |
| `TSX` | `string` | the emitted `.tsx` |
| `Diagnostics` | `[]Diagnostic{File, Line, Col, Span, Severity, Code, Message}` | transpiler errors on the source; `TSnnnn` for syntax errors of the source parse |
| `Map` | `*emit.Map` | every byte of `TSX` → source: copied (exact) or synthesized (its origin's span). `Map.Source(out)`, `Map.Output(pos)`, `Map.Spans()`, `Map.SourceMapV3(...)` (emit/sourcemap.go) |
| `Notes` | `[]Note{Span, Kind, Name, Detail, Tag}` | what was synthesized where. Kinds: `slot-prop`, `slot-body`, `slot-params`, `slot-args`, `slot-arg`, `slot-conditional`, `slot-key`, `slot-entry-key`, `attachment` (Detail `fallback` or not), `no-match`, `segment`, `shorthand-true` |
| `Generated` | `map[string]string` | generated name → what the author wrote (`_Section_intro` → `#intro`) |
| `Shorthands` | `[]Shorthand{Name, Kind}` | bare names emitted twice (rename) |
| `SlotGroups` | `[]SlotGroup{Name, Owner, Tags}` | per owner + slot name, every slot element's tag spans |
| `Stopped` | `string` | tolerant only: the pass that ended the run |
| `Dropped`, `Unlowered` | `bool` | TSX is not the author's code (left-out code; a construct left as written) |
| `Unparsed` | `[]emit.Span` | tolerant: top-level statements with a syntax error |

For a page the Notes are already a component-aware index. E1 on
`$E/site/src/pages/guide/slots.rtsx`:

```
note slot-body   name=$Title    span=257..262      note slot-prop name=$Title   detail=DocsLayout
note slot-params name=$NavLink  span=288..304      note slot-prop name=$NavLink detail=DocsLayout
note slot-body   name=$NavLink  detail=params      note slot-prop name=$Aside   detail=DocsLayout
note slot-body   name=$Aside                       note segment   name=intro    detail=section
generated: map[_Section_examples:#examples _Section_intro:#intro]; slot groups: 3
```

### 1.2 How a program is built

- `mapper.RegisterStrict(version)` / `mapper.Register(version)`
  (`go/internal/mapper/mapper.go`) → `rtsx.RegisterMapper(&rtsx.Mapper{Name, Version, Extension: ".rtsx", Transform, Depends})`
  (`go/third_party/tsgo/rtsx/mapper.go`) → `contentmapper.RegisterBuiltIn`
  (patch 0004). Process-global, must precede any program.
- `mapper.transform` calls `transpiler.Transpile` with
  `ReadFile: func(p) { return "", req.FileExists(p) }` — a transform sees its
  own file and **which siblings exist, never their contents** (cache key =
  content + `depends` existence bits). Result: `rtsx.MapperResult{Text, Spans, StatementStarts, Extra: *mapper.File}`.
- `rtsx.LoadProject(configPath, cwd, fs)` → `*rtsx.Project` (`Files()`,
  `References()`, `Program()`); `rtsx.NewProgram(configPath, cwd, fs)` is both
  (`rtsx/program.go`). `Program()` is `compiler.NewProgram{UseSourceOfProjectReference: true}`.
- In the program every `.rtsx` file is a module **under its own name**; its
  `file.Text()` is the emitted TSX, `rtsx.MappedFile(file)` returns
  `(source, *SpanMap, extra, ok)`, and `mapper.Of(file)` returns the
  `*mapper.File{transpiler.Output; Stopped bool; Err error}`.
- `./button` resolves to `button.rtsx` after the built-in extensions (patch
  0005); a segment import names its file (`./intro.rtsx`).
- Strict (`check`, and what a build wants): a file with a syntax error is not
  lowered; its source stands in as virtual text and `File.Stopped` is true
  (mapper.go, `transform`/`identity`).

`check.Run(configPath)` (`go/internal/check/check.go`): loads the tsconfig and
the projects it references, builds one program at a time, calls
`report.Program(program, configDiagnostics)`, de-duplicates by owning project.
E1: the whole of `reactogenic check` on `$E/site` takes **45 ms** wall.

### 1.3 What the checker can be asked today

The bridge's named functions are few (`rtsx/rtsx.go`: `ParseTSX`, `ParseRTSX`,
`Bind`, `ResolveValue`, `TokenStart`, `NodeText`, `IsIntrinsicTag`,
`Precedence`, …; `rtsx/program.go`: `LoadProject`, `NewProgram`,
`AllDiagnostics`, `FileDiagnostics`, `GetChecker`, `TokenAt`, `FileOf`,
`SlotDeclaration`, `PropsAt`, `IsError`). But `SourceFile`, `Node`, `Symbol`,
`Program`, `Checker`, `Type` are **type aliases** of tsgo's internal types, so
every exported method and field of those types is callable from our module
without a patch — our code already does this (`program.GetSourceFiles()` in
check.go, `p.GetSourceFile` in report/crossfile.go, `n.ForEachChild`
everywhere). Verified by running, from a module outside the fork (E1, E2):

| Question | How, today (no patch) | E1/E2 output |
| --- | --- | --- |
| module graph from a page | `file.Imports()` + `p.GetResolvedModuleFromModuleSpecifier(file, spec).ResolvedFileName` + `p.GetSourceFile(name)` | page → `ui/layout.rtsx` → `packages/core/src/index.ts` → `slots.ts`/`each.ts`/`flow.ts`, `nav.ts`, `intro.rtsx`, `examples.tsx`; 73 µs |
| is a file `.rtsx` / external | `mapper.Of(file)`, `p.IsSourceFileFromExternalLibrary(file)` | `mapped=true external=false` |
| which component a tag is, and where declared | `c.GetSymbolAtLocation(tag)`, `c.ResolveAlias`, `sym.ValueDeclaration`, `rtsx.FileOf(decl).FileName()` | `<DocsLayout>` declared in `…/site/src/ui/layout.rtsx` |
| type of an expression | `c.GetTypeAtLocation(expr)`, `c.TypeToString(t)` | `nav`: `readonly NavItem[]`; `$Title`: `{ children: string; }` |
| declared (contextual) type of a prop / slot | `c.GetContextualType(expr, 0)` | `$NavLink`: `Slot<DetailedHTMLProps<AnchorHTMLAttributes<…>>, {…}> \| undefined` |
| literal value | `t.IsStringLiteral()` / `IsNumberLiteral()`, `t.AsLiteralType().Value()` | `current`: `"/guide/slots"` |
| structured constant | walk `c.GetTypeArguments(tuple)`, `c.GetPropertiesOfType`, `c.GetTypeOfSymbol` | only for `as const` data (E2); `NAV: readonly NavItem[]` gives no value |
| props of a tag / slot | `rtsx.PropsAt(c, file, pos)`, `rtsx.SlotDeclaration(c, tag, slot)` | (used by lsp and report/crossfile.go) |
| virtual ↔ source position | `mapper.Of(file).Map.Source(span)`, `.Output(pos)`; `rtsx.SpanSource` | tag at virtual → source 213..223 |

What needs a **new bridge function** (a new file under `rtsx/`, listed in a
patch; no upstream file edited):

| Need | Why not reachable | Size |
| --- | --- | --- |
| TSX → JS by tsgo's emitter | `transpile.TranspileModule` takes `transpile.Options{CompilerOptions *core.CompilerOptions}` — internal named types cannot be written outside the module | a 25-line file, verified (E4): `$E/tsgo-copy/rtsx/transpile_exp.go` |
| more node kinds | `rtsx.go` re-exports 35 `Kind*` constants; an evaluator over the AST needs `KindObjectLiteralExpression`, `KindArrayLiteralExpression`, `KindVariableDeclaration`, `KindNumericLiteral`, `KindTemplateExpression`, … | one line each (rtsx.go is patch 0001's) |
| flags and enum constants | `ast.SymbolFlags*`, `checker.SignatureKind*`, `core.ResolutionMode*` are typed constants of internal packages | one line each |
| constant folding | `internal/evaluator` (`NewEvaluator`) and `c.GetConstantValue` (enum members only) | wrapper, if wanted |
| printing an AST | `internal/printer` | wrapper, only if the builder rewrites trees instead of text (`emit.Edit` is text) |

What would need a **patch to an upstream file**: nothing found for a builder
that reads the program. The one trap: `Program.Emit` skips content-mapped
files — `sourceFileMayBeEmitted` (`internal/compiler/emitter.go:506-510` in the vendored tree:
"Runtime output for content-mapped files is owned by the external content
mapper or build tool"). Do not patch that; transpile the virtual text
(`file.Text()`) instead, as E4 does.

## 2. Reusable as is, and the gaps

| Piece | Where | Reuse |
| --- | --- | --- |
| `.rtsx` → `.tsx`, strict | `transpiler.Transpile`, or `file.Text()` of the mapped program | as is. Taking it from the program guarantees "what is checked is what is built" |
| type check + source-term errors | `report.Program`, `check.Run`, `check.Print`, `check.Errors` | as is — `build` should refuse to build when `check.Errors(reports) > 0` |
| positions | `emit.Map`, `emit.LineCol`, `Map.SourceMapV3` | as is, for builder errors and (later) source maps of client JS |
| import-origin recognition | `syntax.FrameworkExport(ident)` (`go/internal/syntax/imports.go`): resolves the identifier through the binder (`valueBinding` → `rtsx.ResolveValue`), requires an `ImportSpecifier` whose declaration's specifier is `syntax.Package = "@reactogenic/core"`, returns the *imported* name (`importedName`), so aliases work and a local `Switch` does not match. Used by `flowKind` (flow.go), `inLoop` (segments.go, `Each`), `CheckFlowAsValue` | as is, **on the source tree only** (after pass 2 `Match`/`Switch` are gone, their import dropped). For names from other packages (`@reactogenic/ui`'s `Dialog`), the same function with the package as a parameter, or the checker (`GetSymbolAtLocation` → declaring file, E1) |
| segment lookup | `c.segmentFile`, `SegmentExtensions` (segments.go); in the program, the `segment` notes + `report.mounted` | as is |
| cross-file rules pattern | `report/crossfile.go`: `slotConditionals`, `segmentSelf` join transpiler notes across files through the checker | the model for shell rules: a new file in `report` |
| module graph walking | — | **gap, small**: no function today; E1's 20-line walk is all it takes |
| route table (page → pathname) | — | **gap**: `route-table.md` is referenced by layout.md and does not exist |
| HTML / CSS / client-JS emit | — | **gap**: nothing in the repo emits HTML, CSS or JS. No CSS convention is specified anywhere (`grep -i css specs` finds only "per-route CSS") |
| JS engine | — | **gap**: the Go binary cannot execute a component |

### `packages/core` at runtime

Imports, exactly (`grep -rn 'from "react' packages/core/src`):

```
src/slots.ts:1:  import type { Key, ReactNode } from "react";
src/flow.ts:1:   import type { Key, ReactNode } from "react";
src/each.ts:1:   import type { ReactNode } from "react";
src/runtime.test.tsx: import { renderToStaticMarkup } from "react-dom/server";   (test only)
src/types.check.tsx:  import type { ComponentProps, ReactNode } from "react";    (type test only)
```

- **React-free at runtime**: all three are `import type` (and
  `verbatimModuleSyntax` is on). `packages/core/dist/*.js` has no `import`
  statement at all. E3's esbuild metafile lists `slots.ts`, `each.ts`,
  `flow.ts`, `index.ts`, the static runtime and the site files — no `react`.
- The helpers are plain JS: `renderSlot` (calls a function body with args,
  else returns the body, else the fallback), `slotProps` (drops
  `NOT_ASSIGNED` entries), `isAssigned`, `slotEntry` (`KEYED`), `slotKey`
  (`SLOT_KEY`), `slotArgs` (identity), `Each` (`items.map(children)`),
  `noMatch` (throws), `Match`/`Switch` (throw: compile-time only). Symbols are
  `Symbol.for("reactogenic.…")`.
- React is only in the **types** (`ReactNode`, `Key`,
  `ComponentProps<"h1">` in user slot contracts) and in
  `package.json`: `"peerDependencies": { "react": ">=19" }`. A zero-React docs
  site still needs `@types/react` to type-check, and gets a peer warning
  unless the peer is made optional — or core's types move off React.

**What executing a component at build time means.** The emitted TSX is an
ordinary function of props returning JSX; slots are plain objects; the
helpers above are plain JS. So "execute" is: compile JSX against a runtime
whose `jsx(type, props)` calls function components and serialises intrinsic
elements. E3 does exactly that with `$E/static-runtime/jsx-runtime.js`
(26 lines) and gets, for the docs page:

```html
<div class="rg-layout"><nav class="rg-sidemenu" aria-label="Docs"><ul><li><a href="/">Introduction</a></li><li><a href="/guide/slots" aria-current="page"><b>›</b>Slots</a></li>…</ul></nav><main><h1 class="rg-title">Slots</h1><section id="intro"><p>A slot is a prop value.</p></section><section id="examples" class="band"><pre>&lt;$Title&gt;Slots&lt;/$Title&gt;</pre></section></main><aside class="rg-aside">On this page</aside></div>
```

No React, no checker involvement, no change to the transpiler. What this
route does **not** give: it needs a JS engine at build time (Node ≥ 22 is
already required by every package's `engines`; an embedded pure-Go engine is
UNVERIFIED here), and the executing runtime sees values, not components — it
cannot by itself know which design-system components were used, so
"component-aware" output (per-page CSS/JS selection) needs either the runtime
to record what it rendered or the Go side to read it off the program (E1).

A design note that binds the catalog: CLAUDE.md says the design system is
authored in plain `.tsx`, but `slot={$X}` attachments exist only in `.rtsx`
(syntax.md:329, "a container is an `.rtsx` file"). A container written in
`.tsx` must call `renderSlot`/`isAssigned`/`slotProps` by hand.

## 3. The builder's input: emitted `.tsx`

Produced by the built binary (`$E/reactogenic serve`, method `transform`;
5 files in 50 ms wall including process start). Files in `$E/emitted/`.

**A page** (`$E/site/src/pages/guide/slots.rtsx` → `page-slots.tsx`):

```tsx
// .rtsx
import { Match } from "@reactogenic/core";
import { DocsLayout } from "../../ui/layout";
import { NAV } from "../../nav.ts";
const current = "/guide/slots";
export default function SlotsPage() {
  return (
    <DocsLayout nav={NAV} current>
      <$Title>Slots</$Title>
      <$NavLink { item, active }>
        <Match on={active}><b>›</b></Match>
        {item.label}
      </$NavLink>
      <$Aside>On this page</$Aside>
      <section #intro />
      <section #examples className="band" />
    </DocsLayout>
  );
}
```

```tsx
// .tsx
import { DocsLayout } from "../../ui/layout";
import { NAV } from "../../nav.ts";
import _Section_intro from "./intro.rtsx";
import _Section_examples from "./examples.tsx";
const current = "/guide/slots";
export default function SlotsPage() {
  return (
    <DocsLayout nav={NAV} current={current} $Title={{ children: "Slots" }} $NavLink={{ children: ({ item, active }) => <>
        {active ? <b>›</b> : null}
        {item.label}
      </> }} $Aside={{ children: "On this page" }}>
      <section id="intro" ><_Section_intro /></section>
      <section id="examples" className="band" ><_Section_examples /></section>
    </DocsLayout>
  );
}
```

**A container** (`$E/site/src/ui/layout.rtsx` → `layout.tsx`), the sidebar
part:

```tsx
// .rtsx
<Each items={nav} { item }>
  <li key={item.href}>
    <a slot={$NavLink} href={item.href} aria-current={item.href === current ? "page" : undefined} &item &active={item.href === current}>
      {item.label}
    </a>
  </li>
</Each>
…
<h1 slot={$Title} className="rg-title" />
```

```tsx
// .tsx
import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotArgs as _slotArgs, slotKey as _slotKey, slotProps as _slotProps } from "@reactogenic/core";
…
<Each items={nav}>{({ item }) => <li key={item.href}>
    {((_args) => _isAssigned($NavLink) ? <a key={_slotKey($NavLink, _args)} href={item.href} aria-current={…} {..._slotProps($NavLink)}>{_renderSlot($NavLink, _args, item.label)}</a> : <a href={item.href} aria-current={…}>
      {item.label}
    </a>)(_slotArgs($NavLink, { item: item, active: item.href === current }))}
  </li>}</Each>
…
{_isAssigned($Title) ? <h1 className="rg-title" {..._slotProps($Title)}>{_renderSlot($Title, {})}</h1> : null}
```

**Segments** (`fixtures/segments/mount/input.rtsx` → `fixture-segments-mount.tsx`):

```tsx
import _Section_intro1 from "./intro.rtsx";          // `_Section_intro` was taken by the author
import _Card_pricing from "./pricing.rtsx";
import _UiPanel_faqList from "./faq-list.rtsx";
export const a = <section id="intro" ><_Section_intro1 /></section>;
export const b = <Card id="pricing" className="band" ><_Card_pricing /></Card>;
export const c = <Ui.Panel id="faq-list"><_UiPanel_faqList /></Ui.Panel>;
```

**Keyed slots** (`fixtures/slots/keyed/input.rtsx` → `fixture-slots-keyed.tsx`):

```tsx
<Table data $Column={{ [_KEYED]: true, "email": { ...emailColumn }, [nameKey]: { width: 2, children: "Name" }, ...(showAge ? { "age": {} } : {}) }} />
…
<Each items={columns}>{({ item: col }) => ((_entry, _args) => _isAssigned(_entry) ? <th key={_slotKey(_entry, _args, col.name)} {..._slotProps(_entry)}>{_renderSlot(_entry, _args, col.label)}</th> : <th key={col.name}>{col.label}</th>)(_slotEntry($Column, col.name), _slotArgs($Column, { col: col }))}</Each>
```

What the shapes mean for a builder:

- The emitted TSX is **runtime-shaped**: a slot attachment is an IIFE with a
  ternary and a spread, `Each` is still a component with a function child,
  `Match` is already `?:`. A static analyser that wants to *prove* S2/S5 or
  to unroll loops sees none of the author's constructs here — it must work
  on the **source tree** (`rtsx.ParseRTSX` + `go/internal/syntax`) or use the
  Notes. An executor does not care.
- Segment imports carry the extension (`./intro.rtsx`); other imports are as
  written (`../../ui/layout`). A bundler must resolve `.rtsx` both ways
  (E3: `ResolveExtensions` with `.rtsx` + an `OnLoad` filter `\.rtsx$`).
- After pass 2 the `Match`/`Switch` import is gone; `Each` stays imported.

## 4. Where `reactogenic build` slots in

**`go/cmd/reactogenic/main.go`** is a `switch os.Args[1]` over `check`,
`serve`, `lsp`, `content-mapper`, version, help; each command is a function
`runX(args []string) int` with its own `flag.NewFlagSet` (`runCheck`,
`runLSP`). `build` is one more case, one more `runBuild`, two more lines in
the `usage` constant. `runCheck` shows the preamble to copy: resolve `-p` to a
tsconfig, `mapper.RegisterStrict(version)`, `check.Run(config)`,
`check.Print(os.Stdout, reports, cwd, pretty, readFile)`, exit 1 when
`check.Errors(reports) > 0`.

**Diagnostics through `report`.** `report.Report{File, Span, Line, Col, Severity, Code, Message, Related, TS}`
is already host-neutral and printed by `check.Print` in `tsc`'s format with a
code frame of the `.rtsx` source. Three kinds of producer exist, and builder
errors fit the third:

1. transpiler diagnostics (`mapper.File.Diagnostics`) — per file, syntactic;
2. TS diagnostics mapped and reworded (`report.fromTS`, `report.rewrite`);
3. cross-file rules computed from notes + checker (`report.slotConditionals`,
   `report.segmentSelf` in `report/crossfile.go`, appended in
   `report.fileReports`).

A shell rule (`shell-loop`, `shell-handler`, `shell-dynamic-value`, …;
layout.md, *Enforcement*) is a rule of kind 3: given `(program, file,
mapper.File, source)` return `[]Report` with `Span` in the source
(`emit.LineCol(source, pos)` for line/col). Put there, `check` prints it and
`lsp` shows it with no further work (`lsp/diagnostics.go` → `report.File`),
which is what layout.md asks for ("compiler error, language-service
diagnostic"). One thing `report` lacks: **which files are shell code** is a
fact about entry points, not about a file — the rule needs the set of pages
(or, in phase 2, "every module reachable from a page").

Errors found only while building (a component threw at build time, a value
was not serialisable) have a JS stack, not a span. E3's bundle had no source
map; with `emit.Map.SourceMapV3` (used by `server.transform`) an executor's
stack can be mapped to `.rtsx` — to be built.

**The Vite plugin** (`packages/vite/src/index.ts`, `server.ts`): `transform`
hook → `Server.transform(file, code)` → one long-lived `reactogenic serve`
(`go/internal/server/server.go`, method `transform`, JSON lines) →
`transformWithOxc`. It holds no program and no types. It is the React path:
a builder that emits zero-React pages does not go through it, and it stays
what it is for apps (and later for islands). Dev server is out of scope, so
phase 2 need not touch `packages/vite`.

**The CLI package** (`packages/cli/bin/reactogenic.js`): `spawnSync(binary,
process.argv.slice(2), { stdio: "inherit" })` — any new subcommand works with
no change to the package. `binary.js` finds
`@reactogenic/cli-<platform>-<arch>/bin/reactogenic` or
`$REACTOGENIC_BINARY`. Note the shim is itself a Node process: a builder that
needs a JS engine has Node at hand (`process.execPath`), but the Go binary
would have to be told where it is — today nothing passes it.

## 5. Constraints that bind the design

| Constraint | Source | Consequence for the builder |
| --- | --- | --- |
| **Fork-patch policy**: every change under `go/third_party/tsgo` is in exactly one patch; one patch owns a given upstream file; prefer new files; CI's `vendor` job re-vendors and compares, `check-patches.sh` fails a fork file no patch lists | `go/patches/README.md`; `.github/workflows/ci.yml` (`go`, `vendor`) | new bridge functions go in a **new file under `rtsx/`** in a new patch (`go/scripts/regen-patch.sh 0008-… rtsx/build.go`); `Kind*` additions fold into 0001 (owner of `rtsx/rtsx.go`). No upstream file needs editing for a read-only builder (§1.3) |
| Pin frozen until RGP1-114 (re-vendor at TS 7.1) | decisions.md, *IDE support*, decision 11; `UPSTREAM`: commit `4f5ddae`, 2026-09-25 | anything that leans on tsgo internals (emitter, printer, evaluator) is re-checked at the re-vendor; the bridge is the only place to fix |
| **`CGO_ENABLED=0`, six targets** (darwin/linux/win32 × arm64/x64) | `scripts/build-binaries.sh` | every Go dependency must be pure Go. esbuild is (E5: five cross-targets built). A cgo JS engine (QuickJS/V8 bindings) is excluded |
| **Binary size**: "a growth of more than 10% over the previous release needs a note in decisions.md"; 19.4 MB (alpha.0) → 26.4–28.6 MB (alpha.1, the language service) | `scripts/build-binaries.sh` (comment), plan.md RGP1-113, decisions.md *Cost accepted* | esbuild linked in: **+5.6 MB ≈ +20%** (E5) → needs the note. The same binary is bundled in the seven `.vsix` (plan.md: 9.5 MB packed) — builder code rides into the editor extension too |
| Go modules: `go.work` uses `./go` and `./go/third_party/tsgo`; `go/go.mod` `replace`s tsgo and must build with `GOWORK=off` (CI step "Build without the workspace") | `go.work`, `go/go.mod`, ci.yml | a new dependency goes into `go/go.mod` + `go/go.sum` (not only `go.work.sum`). esbuild v0.28.2 requires only `golang.org/x/sys` (2022); tsgo requires `v0.48.0`; MVS keeps `v0.48.0` — E6's `go mod tidy` adds exactly one line |
| Go commands name the module (`github.com/reactogenic/reactogenic/go/...`), never `./go/...` | CLAUDE.md, decisions.md RGP1-003 | — |
| CI: `go` (build tsgo, vet, `go test -race`, gofmt of `go/cmd go/internal`, check-patches, GOWORK=off build), `vendor`, `js` (required by the `main` ruleset); `vscode` (not required); `package.yml` on tags | ci.yml | a builder with end-to-end tests that need Node belongs in `js` (as the Vite tests, which build the binary in `packages/vite/test/setup.ts`) or needs `setup-node` in `go` |
| `go/internal/*` is internal to our module | Go | an out-of-tree prototype must copy the packages (E1 did) |
| Disk | `GOCACHE` is 97 GB on this machine; free space fell 61 → 50 GB during this session while five cross-builds of tsgo ran (other agents were building too) | cross-compiling tsgo is the expensive step; keep prototypes to one target |
| Mapper cache contract: a transform reads its file and sibling *existence* only | `mapper.transform`, `mapper.depends`, ide.md | anything cross-file (what a component renders, whether it is shell-safe) cannot live in the transpiler; it is a `report`-style rule or the builder's own pass |
| Transpiler is purely syntactic; types never change emitted code | CLAUDE.md, vite.md *No types in the transform* | a builder may *use* types (it is not the transpiler), but if build output depended on types while `.tsx` emit does not, the two must not disagree — keep type-directed decisions in the HTML/JS emit, never in the `.tsx` |

## 6. Shell rules S1–S5 against a 4-page docs site

Rules: layout.md, *Shell rules*. Phase 2 has no islands, so every construct
below is shell code.

| Docs-site need | Construct | Rule hit | layout.md says |
| --- | --- | --- | --- |
| sidebar from a nav list | `<Each items={NAV}>` / `NAV.map(...)` | **S2** (`shell-loop`) | `> OPEN: loops over constants. <Each items={NAV_LINKS}> is deterministic and could be unrolled by the compiler. Allow Each / .map() in shell code when items is a compile-time constant (S3), or keep S2 absolute?` — **must be decided first**; with S2 absolute the nav is four hand-written `<li>` per page |
| nav link as a slot per item | `<a slot={$NavLink} …>` inside the `Each` | **S5** ("containers unconditional and unlooped at their position") + S2 | same OPEN; note the transpiler emits a `?:` for every attachment, so S2 cannot be checked on emitted TSX (§3) |
| "current page" highlight | `aria-current={item.href === current ? "page" : undefined}`; `<Match on={active}>` in the `$NavLink` body | `?:` yielding an *attribute value* is not S2 by its letter ("that produce elements"); `Match` **is** S2 (`shell-conditional`: "make it two routes or move it into an island") | no OPEN covers **conditionals over compile-time constants** — only loops. Per pathname `current` is a constant, so this is the same question; it needs the same answer |
| `current` itself | `const current = "/guide/slots"` in the page, passed down | S3 allows it ("module-level constants… props handed down from shell code") | fine. But where the pathname comes from is `route-table.md`, which does not exist |
| nav data | `export const NAV = [...]` in `nav.ts` | S3 allows it | E2: the checker gives its *value* only if declared `as const`; otherwise the builder must evaluate the initializer or execute the module |
| optional aside / title | `<aside slot={$Aside} />`, slot left empty | sanctioned variance ("an optional slot being left empty") | fine |
| slot params | `<$NavLink { item, active }>` | allowed: "the container calls the slot function at compile time, with compile-time args" | fine — and it presupposes the compiler *calls* functions at compile time |
| sidemenu toggle, dropdown open/close, dialog open | event handling | **S1** (`shell-handler`: no `onClick={…}`) | `> OPEN: can page authors contribute shell behaviour without React, or is raw JS reserved to the design system?` |
| dialog popup on a static page | `Dialog` used directly in shell code | — | `> OPEN: a shell component used directly in shell code (a static dialog in a page) — who opens it, with no island around?` — **this is exactly the phase 2 case**; the specified `open/update/close` bridge is driven from an island's effects, and there are none |
| how a component says it is a shell component, and its JS API | — | — | `> OPEN: how the design system declares a component as a shell component, and the API it must implement` (→ `slot-contract.md`, not written) |
| dialog/dropdown markup delivery | `<template id="Dialog-a1">` per use site + `mountDialog(root, slots)` in a shared runtime | — | `> OPEN: confirm <template> delivery`. For a static page with no holes a `<template>` + clone is not obviously needed (the markup can be in the document) — undecided |
| dates, build stamps | `new Date()` in a footer | **S4** (`shell-nondeterministic`) | no OPEN; an executor must enforce it (static check or a frozen realm) |
| shell identical across documents (View Transitions) | per-page `aria-current` makes the sidebar differ per page | fact, not a rule: CLAUDE.md "Identical across documents" | View Transitions are out of phase 2; the tension stays for later |

Run against E3: an executor produced the right HTML for all of the above
(loop unrolled, `Match` decided, slot function called, attachment resolved)
without any rule being relaxed in code — because at build time every value
*is* a constant. So the rules are not what makes the output static; they are
what makes the build **refuse** code that would not be. With an executor,
S2/S3/S5 reduce to "everything evaluated at build time was computable from
constants" (true by construction, no request exists), and only S1 (handlers,
hooks) and S4 (non-determinism) need a check of their own.

Other spec drift to fix when phase 2 specs are written: layout.md imports
`Dynamic` from `"reactogenic"`, the package is `@reactogenic/core`
(`syntax.Package`); vite.md's export table still lists `FnSlot` (folded into
`Slot<P, A>` by RGP1-043).

## Recommendation

1. **Build on the mapped program, not on a second pipeline.** `reactogenic
   build` = `mapper.RegisterStrict` → `rtsx.NewProgram` → `report.Program`
   (stop on errors) → for each page, walk `file.Imports()` → take each
   `.rtsx` module's `file.Text()` as its TSX. Everything in that chain exists
   and is measured (30 ms program, 45 ms check for the docs page). The
   transpiler needs no change for phase 2.
2. **The seam for esbuild is real and small** — an `OnLoad` for `\.rtsx$`
   calling `transpiler.Transpile`, plus `ResolveExtensions` (E3, 13 ms). It
   costs +5.6 MB (+20%) and one line in `go/go.mod`; no dependency conflict;
   cross-compiles CGO-free. But note what it is a seam *for*: bundling,
   splitting and minifying JS. It does not make HTML, and tsgo's own emitter
   already turns our TSX into JS through one bridge function (E4). So esbuild
   earns its 5.6 MB only for the **client JS per page** (and for bundling the
   build-time module graph if components are executed); decide on that
   ground, not for TSX→JS.
3. **Decide "execute or evaluate" explicitly — it is the fork in the road.**
   - *Execute* (E3): JSX runtime that serialises; correct HTML today for
     slots, `Each`, `Match`, segments; needs a JS engine at build time and
     gives component awareness only if the runtime or the Go side records it.
   - *Evaluate in Go* over the source tree with the checker: no JS engine,
     full component awareness, but the checker yields values only for literal
     and `as const` types (E2), so it needs an AST evaluator of its own — a
     second semantics for the same code.
   The codebase favours a hybrid: Go decides *what* is on the page from the
   program (which components, which slots filled — Notes + checker, E1), and
   execution produces the markup.
4. **Shell rules as `report` rules** (new file beside `crossfile.go`), fed the
   set of page entry points, so `build`, `check` and the editor print the same
   lines. Check them on the source tree, never on emitted TSX.
5. **Bridge additions in one new patch** with only new files under `rtsx/`
   (`0008-rtsx-build.patch`): `TranspileTSX` if wanted, the extra `Kind*` and
   flag constants. No upstream file edited.
6. **Before any code, settle in specs**: loops and conditionals over
   compile-time constants; who opens a shell component on a page without
   islands; how a component declares its shell JS and CSS; the route
   convention (pages → pathnames); `react` as a peer of `@reactogenic/core`.

## Open questions

1. Loops over constants (layout.md OPEN) — and the unlisted twin,
   conditionals over constants (`Match on={item.href === current}`): allowed
   in shell code when decidable at build time?
2. A shell component used directly in shell code — who opens the dialog /
   dropdown / side menu with no island? Declarative triggers in HTML
   (UNVERIFIED here: `popover` / invoker attributes / `<dialog>` support —
   another researcher's topic), or design-system raw JS keyed by attributes?
3. How does a component declare its raw JS and CSS so the builder can ship
   "exactly what the page needs" — a sibling file convention, an export, a
   manifest? Nothing in the repo says; there is no CSS convention at all.
4. Build-time execution engine: Node subprocess (present wherever the npm
   package is installed, but the Go binary is not told its path), an embedded
   pure-Go engine (UNVERIFIED: language coverage, size), or none (Go
   evaluator)?
5. Is `@types/react` acceptable as the type vocabulary of a zero-React site
   (`ReactNode`, `ComponentProps<"a">`), and should `react` stay a
   non-optional peer of `@reactogenic/core`?
6. Is the design-system catalog `.tsx` (CLAUDE.md) or `.rtsx` (attachments
   exist only there)?
7. Does the builder live in the same binary (it then ships inside the VS Code
   extension, +20% with esbuild) or in a second one?
8. Route table: directory convention or explicit list, and how a page learns
   its own pathname (`current`) without writing it by hand.
9. Should `Program.Emit` ever be used (it skips mapped files by design), or is
   the virtual text always transpiled on its own — and then which emitter,
   tsgo's or esbuild's, is "the" JS semantics of a page?
10. UNVERIFIED: build behaviour on Windows paths (E3 ran on macOS only);
    esbuild's `OnLoad` with `ResolveDir` on symlinked workspaces (pnpm) beyond
    the one symlink layout tried here.

## Verification (independent)

Skeptic pass, 2026-10-04, same machine (go1.27.1, node v25.2.1), repo at
`4159fa3`, read only. Experiments in
`…/phase2-research/exp-codebase-verify/` (`$V`); the author's binaries in `$E`
were re-run, and four new probes were built (`$V/gomod/cmd/progbuild`,
`$V/gomod/cmd/emitprobe`, `$V/realmod` = a copy of `go/cmd` + `go/internal`
against the vendored tree, `$V/rt/compare.mjs`). The author's text above is
unchanged.

### Verdict per claim

| # | Claim | Verdict | What was checked; correction |
| --- | --- | --- | --- |
| 1 | The mapped program answers everything, no patch, no new bridge function | **confirmed** | Re-ran `$E/probe` 3×: program 15.8–25.5 ms, graph 39–50 µs, checker 10–16 ms, same answers. Built my own out-of-fork program (`progbuild`) on `file.Imports()`, `GetResolvedModuleFromModuleSpecifier`, `file.Text()`, `mapper.Of`: compiles and runs against the vendored tree. Caveat: `file.Imports()` includes type-only imports (`react` under `layout.rtsx` is `import type`), so it is the *checker's* graph, not the runtime graph |
| 2 | Checker yields values only for literal types and `as const` data | **partly** — true, and weaker than stated | Re-ran `$E/constprobe`; same output. Variant `$V/site/src/const2.tsx`: `as const` imported from another module, `as const satisfies readonly NavItem[]`, nested `as const`, template literal `as const`, enum member, `NAV_AS_CONST[0].href` → values. `const n = 2 + 3` → `number`, `"/guide" + "/slots"` → `string`, inferred `[{…}]` → no value. **Correction:** inside a component the prop is its declared type — `insideParam = <not a constant: string>` for `current: string` although the caller passes a literal. The loop and the `Match` of a docs sidebar sit inside `DocsLayout`, over props typed `readonly NavItem[]` / `string`, so `as const` at the data does **not** make them unrollable from the checker: of the three options listed, only an inter-procedural evaluator or execution works |
| 3 | `@reactogenic/core` is React-free at runtime; peer `react >=19` | **confirmed** | `grep` as cited; `packages/core/dist/*.js` has no `import`/`require`; `peerDependencies {react: ">=19"}`, no `peerDependenciesMeta`. Metafile inputs re-read: no `react`. Note the workspace `exports` is `./src/index.ts` (dist only via `publishConfig`), which is why the bundle shows `src/*.ts` |
| 4 | Emitted TSX executes to static HTML, zero React, 26-line runtime, 533 bytes | **partly** | Reproduced byte for byte (`cmp` with `$E/out/slots.html`). **But the runtime is not a correct serialiser for what the type checker accepts.** `$V/rt/compare.mjs` runs the same element trees through it and through `react-dom/server` 19.3.0: **11 of 12 differ** (`$V/rt/compare.out`). Load-bearing ones for dialog / dropdown / side menu: `aria-expanded={false}` is **dropped** (React: `aria-expanded="false"`); `aria-hidden={true}` → bare `aria-hidden`; `style={{…}}` → `style="[object Object]"`; `tabIndex` / `strokeWidth` not renamed; `dangerouslySetInnerHTML` (highlighted code blocks) printed as an attribute; `defaultValue` not mapped. The page proved the *route*, not the runtime: see M2 |
| 5 | esbuild has a clean seam: `OnLoad` for `\.rtsx$` + `ResolveExtensions`; 13.2 ms / 3.9 ms | **partly** | Re-ran `$E/buildprobe` 3×: 16.4 / 6.0 / 4.7 ms total, transpiler 6.0 / 2.1 / 1.5 ms — the cited number is a cold single run; warm is ~5 ms. **Correction 1:** E3's `ResolveExtensions` (`.tsx,.ts,.rtsx,.jsx,.js`) disagrees with patch 0005 (`.rtsx` after *every* built-in extension). `$V/site/src/amb`: `widget.rtsx` next to `widget.jsx`, `import "./widget"`, `allowJs` → `reactogenic check` passes having checked `widget.jsx`; E3-style build renders `from widget.rtsx`. Checked ≠ built. **Correction 2:** E3 re-transpiles from disk; it does not use the program the recommendation is about. `progbuild program` does: `OnLoad` returns the program's `file.Text()`, `OnResolve` answers from the program's resolutions — same 533-byte HTML, esbuild 2.4–9.2 ms after 34–42 ms program + all diagnostics, and the ambiguity case resolves as checked (`from widget.jsx`). So the seam is clean, and cleaner than shown: one resolver, one transpile |
| 6 | esbuild +5.6 MB (≈ +20%), no dependency conflict, CGO-free cross-compile, over the 10% threshold | **partly** — number is a proxy | The +5.6 MB was measured on a probe binary. On the **real** `cmd/reactogenic` with the release flags (`$V/realmod`, `$V/sizes.txt`): darwin-arm64 27.36 → 32.03 MB = **+4.67 MB (+17.1%)**; linux-amd64 27.86 → 33.10 MB = **+5.24 MB (+18.8%)**; gzip -9 (what the `.vsix` / tarball carries) 9.30 → 11.25 MB (+1.95 MB). Still over 10%. `go get` adds one `require` line, `x/sys` stays `v0.48.0`. v0.28.2 is latest per `proxy.golang.org/…/@latest` (tagged 2026-08-08); its `go.mod` requires only `x/sys` 2022. Threshold comment confirmed in `scripts/build-binaries.sh:45`. Windows cross-build not repeated by me |
| 7 | tsgo's emitter does TSX → JS through one new bridge file; esbuild not required for that | **confirmed, with a limit** | `diff -rq $E/tsgo-copy go/third_party/tsgo` → only `rtsx/transpile_exp.go`. Built the real binary against that copy with a `tsemit` entry: runs, and is **the same size to the byte** (28 685 106) — `internal/transpile` and the JSX transforms are already linked (`go list -deps`). The unmeasured size effect is zero. **Limit:** the emitted JS keeps `"../../ui/layout"`, `"./intro.rtsx"`, `"../../nav.ts"`; Node fails with `ERR_MODULE_NOT_FOUND` (`$V/tsemit-run`). tsgo emit alone is not executable — anything that *runs* the modules needs a bundler or a loader hook, so "esbuild only for client JS" understates it |
| 8 | `Program.Emit` skips content-mapped files | **confirmed — now by running** | `$V/emitprobe` calls `Emit` by reflection (the options type cannot be named outside the fork). `noEmit: false`: emitted `nav.js`, `examples.js`, nothing for the three `.rtsx`. With `declaration: true`: `layout.d.rtsx.ts`, `slots.d.rtsx.ts`, `intro.d.rtsx.ts`, still no `.js`. Both conditions (`emitter.go:508`, `outputpaths.go:57`) are upstream code — no patch lists those files. Confidence can be high |
| 9 | Shell rules cannot be checked on emitted TSX | **confirmed** | `$E/emitted/layout.tsx` and `$E/tsemit.out` show the IIFE + ternary; `flow.go:35` drops the `Switch`/`Match` import (`context.go:322`). Types exist only on the virtual text, so a typed rule works as `crossfile.go` does: source span → `Map.Output` → checker |
| 10 | `syntax.FrameworkExport` is import-origin recognition, hard-wired to `@reactogenic/core` | **confirmed** | `go/internal/syntax/imports.go` read in full: `const Package`, `fromPackage` compares the specifier text exactly |
| 11 | `build` is one more case in `main.go`; `packages/cli` unchanged; errors through `report` | **confirmed** | `main.go:49-71` switch, `runCheck:102`; `reactogenic.js` passes `process.argv.slice(2)`; `report.File` (`lsp/diagnostics.go:30`) and `report.Program` (`check.go:90`) both end in `fileReports`, which appends the cross-file rules. Note `check.Run` returns reports, not the program: a builder calls `rtsx.NewProgram` itself |
| 12 | Mapper cache contract: own file + sibling existence only | **confirmed** | `mapper.go:114`, `depends` at `:137`, comment in `crossfile.go:14-17` |
| 13 | A docs site hits S2/S5 and S1 at once; layout.md OPENs as listed; none on conditionals | **confirmed** | layout.md:113-117 (rules), :129, :136, :338, :340, :359 (OPENs). No OPEN mentions conditionals over constants |
| 14 | `route-table.md`, `slot-contract.md` unwritten; no CSS convention | **confirmed** | `ls specs/later`; grep finds the references at layout.md:69, 294, 342 and CLAUDE.md:164 only; "per-route CSS" is all the specs say about CSS |
| 15 | Patch policy allows cheap bridge additions | **confirmed** | `go/patches/README.md`; `rtsx/rtsx.go` is 0001's only file, `rtsx/program.go` 0003's; 35 `Kind*` constants today. Confidence can be high |

### What the report missed

- **M1. Two of its "decide first" items are already fixed decisions.**
  CLAUDE.md, *Architecture decisions (fixed — do not relitigate)*: "Go
  templates for shells, per-route CSS; **embedded esbuild**; Go server" and
  "the design system's core catalog is authored in plain `.tsx`, **executed
  by the compiler** in shell code"; layout.md:12 and :36 say the same. So
  "esbuild optional" and "execute or evaluate" are not open by the specs. What
  *is* open, and what the report leaves UNVERIFIED, is **which engine
  executes**: the Go binary has none, cgo engines are excluded, Node is not
  passed to the binary. (Another researcher's directory, `exp-evaluation/gob`,
  holds goja / QuickJS probes; this report should defer to it.)
- **M2. The type vocabulary and the serialiser disagree.** `check` types JSX
  against `@types/react` (`className`, `tabIndex`, `strokeWidth`, `style`
  objects, boolean `aria-*`), so anything that type-checks must serialise as
  React DOM would. Either the build-time runtime implements React DOM's
  attribute table (the 11 differences above are the start of that list), or
  shell code gets its own JSX namespace through `jsxImportSource` — which
  changes what `check` and the editor accept. This is a spec decision the
  report does not list.
- **M3. Two resolvers.** Shown in claim 5: esbuild's own resolution can pick a
  different file than the one checked. Fix is free (`OnResolve` from the
  program); it should be part of the design, not an option.
- **M4. Import-graph awareness over-ships; the executor's record is exact.**
  `$V/site/src/aware`: `Layout` attaches `<Dialog slot={$Search} />`; one page
  fills `$Search`, one does not. The module graph (esbuild metafile, and
  equally `file.Imports()`) lists `dialog.tsx` for **both** pages. A runtime
  that records the functions it calls (two added lines, `$V/rt-rec`) reports
  `With, Layout, Dialog` vs `Without, Layout`. Per-page JS/CSS selection — the
  bet itself — therefore comes from the execution, not from "Go reads what is
  on the page from the program (Notes + checker)"; the hybrid in
  Recommendation 3 has the roles the wrong way round for this purpose. Notes
  and checker remain right for *diagnostics*.
- **M5. Nothing here measured client output.** E3's 5 KB bundle is the
  build-time program; the page shipped 0 bytes of JS because it has no
  behaviour. How a shell component's raw JS and CSS are selected, split per
  page and minified — the actual phase 2 deliverable — is untested in this
  report.
- **M6. All timings are single runs on a 4-file page.** They show the floor
  (tens of ms), not scaling.

### Does the recommendation hold

Yes for its core, with corrections. Item 1 (build on the mapped program) is
now measured end to end, which the report had not done: program text and
program resolutions through an esbuild plugin give the same HTML in 2–9 ms.
Item 4 (shell rules in `report`), item 5 (one patch of new files) and item 6
(specs first) stand. Change item 2: esbuild is not optional once anything is
executed, and the architecture already names it; its real cost is +17–19%,
not +20%. Change item 3: the open decision is the execution engine, not
whether to execute; component awareness should come from the executor's
record. Add to item 6: the JSX attribute semantics of shell code (M2).
