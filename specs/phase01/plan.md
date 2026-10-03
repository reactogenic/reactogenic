# Phase 1 plan

Scope, from [vite.md](vite.md) and [diagnostics.md](diagnostics.md):

1. `.rtsx` runs in an ordinary Vite + React project.
2. TS errors on the emitted `.tsx` are reported on the `.rtsx`.

Every task below serves one of the two. Task ids are `RGP1-xxx`. They are
numbered in blocks of ten, one block per milestone, so tasks can be added
without renumbering.

Size: **S** ≤ 2 days, **M** ≤ 1 week, **L** > 1 week (split it before starting).

## Milestones

| # | Milestone | Tasks | Ends with |
| --- | --- | --- | --- |
| M0 | Decisions and scaffolding | 001–009 | repo builds, fixture harness runs, both decisions recorded |
| M1 | Runtime package | 010–019 | `@reactogenic/core` exports usable from plain `.tsx` |
| M2 | Parser | 020–029 | every `.rtsx` form parses to an AST with exact spans |
| M3 | Transpiler | 030–049 | every desugaring in [syntax.md](syntax.md) passes its fixture |
| M4 | Type service | 050–059 | `reactogenic check` has its program; the Go process serves the plugin |
| M5 | Vite plugin | 060–069 | example app runs in `vite dev` and `vite build` |
| M6 | `reactogenic check` | 070–089 | every *types* error in syntax.md is reported on the `.rtsx` |
| M7 | Conformance and release | 090–099 | spec ↔ tests complete, packages ready to publish |
| M8 | IDE support | 100–114 | `.rtsx` in VS Code: highlighting, `check`'s errors live, hover, completion, definition, rename |

M1 and M2 can run in parallel. M4 can start once RGP1-030 is done. M5 and M6
both need M3 and M4, but not each other.

Critical path: 003 → 020 → 030 → 050 → 052 → 053 → 061 → 090.

## M0 — Decisions and scaffolding

### RGP1-001 — Decide the checker · S · done
TS7, in-process: the transpiler is written in Go as a fork of tsgo from
day 1. The Vite plugin drives it as one long-lived process over stdio. See
[decisions.md](decisions.md#rgp1-001--checker-and-implementation-language).

### RGP1-002 — Decide the parser strategy · S · done
Extend tsgo's JSX parser in the fork, behind a `.rtsx` script kind. See
[decisions.md](decisions.md#rgp1-002--parser-strategy). The spike this task
originally included is RGP1-020 and RGP1-021 themselves, once the fork
exists (RGP1-003).

### RGP1-003 — Repository scaffolding · M · done
- Go and TypeScript are split at the root: `go/` and `packages/`. tsgo is
  vendored from microsoft/TypeScript's `tsc/` into `go/third_party/tsgo`,
  with our changes kept in `go/patches/`. See
  [decisions.md](decisions.md#rgp1-003--repository-layout-and-vendoring).
- CI runs the Go build, vet, tests and gofmt; a vendor check; and the pnpm
  typecheck and tests.
- The `bin` shim for `check` moves to RGP1-070.
- Found on the way: TypeScript's parser does not reject `#name`. The spec is
  corrected, and RGP1-021 needs no scanner change.

### RGP1-004 — Fixture harness · M · done
`go/internal/conformance`, with the formats in
[fixtures/README.md](../../fixtures/README.md).
- Spec examples are extracted from syntax.md at test time: 24 cases, one of
  them an intermediate stage (`after pass 2`). They check output only; the
  diagnostics are checked by fixtures.
- Output is compared as a normalised TSX tree, not as text.
- A ratchet file (`testdata/passing.txt`) makes CI fail on regressions and
  on passes nobody recorded. Unimplemented cases are only counted, so CI
  stays green while M2–M3 are built.
- `TestSpecOutputsParse` checks that every expected `.tsx` in the spec
  parses. It found two broken examples on its first run; both are fixed.
- Deferred to RGP1-051: spec examples that need types. The *List slots*
  example needs `Form` declared, not only `FormProps`.

### RGP1-005 — Close the phase 1 OPENs in syntax.md · S · done
See [decisions.md](decisions.md#rgp1-005--phase-1-open-questions).

## M1 — Runtime package

### RGP1-010 — Slot types and `renderSlot` · S · done
### RGP1-011 — `Each` · S · done
### RGP1-012 — `Switch`, `Match`, `$Case`, `noMatch` declarations · S · done
`packages/core/src`: `slots.ts` (`SlotFn`, `OptionalSlotFn`, `renderSlot`),
`each.ts`, `flow.ts`. React 19 is a peer dependency.
- `Switch` / `Match` are declarations with their slots (`$Case` a list
  slot) that throw a clear error if rendered — i.e. if the transform did
  not run. `noMatch` throws with the value.
- Tests: `runtime.test.tsx` (vitest: `renderSlot`, `Each` with no wrapper
  and with no items, the throws); `types.check.tsx` (checked by `tsc`: the
  *Declaration matrix* — required slot, unknown option, unknown param,
  params on a `ReactNode` body, a `SlotFn` without params, optional body —
  and `Each`'s item inference).

## M2 — Parser

### RGP1-020 — Slot params · M · done
Patch `0002-rtsx-parser.patch`. In `.rtsx`, a `{` in attribute position
that is not followed by `...` is parsed as an `ObjectBindingPattern` and
stored in a `JsxSpreadAttribute` — no new node kind
([decisions.md](decisions.md#rgp1-020--no-new-node-kinds-no-new-script-kind)).
`go/internal/syntax.SlotParams` reads it.
- Tests (`go/internal/syntax`): every row of the grammar table, with exact
  spans and bound names; `.tsx` still rejects the form; params next to
  other attributes on a component; malformed params are parse errors.

### RGP1-021 — Segment roots `#name` · S · done
No parser patch: tsgo already reads `#about-us` as a `JsxAttribute` named
`"#about-us"`, and its scanner rejects `# about`.
`go/internal/syntax.SegmentRoot` recognises it.
- Tests: names with hyphens and capitals, open/close and self-closing
  elements, next to spreads, slot params and other attributes, exact spans;
  `id="…"`, bare `about-us`, `#x="…"` and `#a:b` are not segment roots.

### RGP1-022 — Parse-level errors · S · done
`go/internal/syntax.Check`: params-on-html (by the checker's own
intrinsic-tag rule), duplicate-params, segment-id (a second `#name`), and the
new segment-syntax (`#name="x"`, `#name:x`). Each element is checked, so one
bad attribute does not hide the others.
- Tests with exact spans in `go/internal/syntax`; fixtures in
  `fixtures/parse/`, which join the ratchet once the transpiler reports them
  (RGP1-030).
- Spec gaps closed: segment-id now covers a second `#name`, and
  segment-syntax is new.

## M3 — Transpiler

### RGP1-030 — Pipeline, emitter and origin-tracking source map · L · done
- **030a — pass pipeline** (`go/internal/transpiler`). Text to text: each
  pass parses its input, returns edits, and the edits make the next pass's
  input — the same shape as the spec's "after pass N" examples. Pass 0 runs
  `syntax.Check`; passes 1–4 are empty until their tasks. A syntax error
  stops the pipeline and is reported as `TS<code>`.
- **030b — emitter** (`go/internal/emit`). Output is the input plus edits;
  every run of output is either *copied* (exact input span) or
  *synthesized* (with an origin span). Untouched code keeps its bytes.
- **030c — source map with origins.** `emit.Map` maps output spans to the
  source: copied text exactly, synthesized text to its origin. Per-pass maps
  compose (`Then`), so diagnostics from any pass land on the `.rtsx` the
  author wrote. `SourceMapV3` encodes the map for Vite, with UTF-16 columns.
- Done: a file with no extensions passes through byte for byte with an
  identity map; transpiler errors are reported with source line and column.
  6 of 29 conformance cases now pass, the RGP1-022 fixtures among them.

### RGP1-031 — Import-origin recognition · S · done
`syntax.FrameworkExport` resolves a tag through tsgo's binder to its import
specifier: a named value import from `@reactogenic/core`, however aliased.
A local or parameter named `Switch`, another package's `Switch`, and
`import type` are not the framework's.
- `syntax.CheckFlowAsValue` (pass 0): any value reference to `Switch` or
  `Match` other than a tag — `const S = Switch`, `as={Match}`,
  `createElement(Switch, …)`, `export { Match }`, re-exports from the
  package (including `export *`), and `R.Switch` through a namespace
  import. Property and member names are not references.
- Fixtures `fixtures/flow/as-value`, `fixtures/flow/re-export`. The alias
  case is unit-tested here and joins the conformance suite with the
  `Switch` lowering (RGP1-034).

### RGP1-032 — Pass 1: shorthand props · M · done
- Scope comes from tsgo's binder, not a resolver of our own (RGP1-005):
  `syntax.Binding` asks the binder's `NameResolver`, with globals excluded
  and ambient and type-only declarations filtered out. Slot params are the
  one scope the binder cannot know; `Binding` finds them by walking up to
  the elements whose body holds the attribute, and the nearer of the two
  wins. No binder patch, no reordering of passes (done with RGP1-031).
- Lookup follows the ES module's scope chain: imports and top-level
  declarations, enclosing functions and blocks, and slot params.
- Excluded: globals, ambient declarations, type-only bindings.
- Always case B: reserved words and non-identifiers (`aria-label`).
- Intrinsic elements and slot elements follow the same rule; spread order is
  preserved.
- **Done when:** every example and edge case in *Shorthand props* is a
  passing fixture, including TDZ and shadowing.
- Done: `transpiler/shorthand.go` inserts `={name}` after the name; the name
  inside the braces is copied, so TS errors on it map exactly. All three
  spec examples pass, plus fixtures for case A, names that can never be
  bindings, globals / `import type` / `declare`, intrinsic elements and
  spreads, TDZ, and slot params (as a pass-1 stage).
- Found: a file with no `import`/`export` is a script to TypeScript, and
  its top-level declarations become globals. `.rtsx` is now always parsed as
  a module, as under Vite; syntax.md says so.
- Depends on: 030.

### RGP1-033 — Pass 2: `Match` · S · done
### RGP1-034 — Pass 2: `Switch` · M · done
`transpiler/flow.go`, `transpiler/switch.go`, with shared helpers in
`transpiler/context.go`.
- Pass 2 **repeats** until it makes no edits: each run lowers the `Match` /
  `Switch` elements that hold no other one, so nesting lowers inside out
  and edits never overlap. The maps compose across runs.
- `Match`: ternary; with params, an immediately-invoked arrow that tests the
  name `value` is bound to (narrowing works). When `value` is destructured
  further (`{ value: { name } }`), the whole value is tested and then
  destructured for the body.
- `Switch`: static, reference (the subject repeated, no wrapper), dynamic,
  `exhaustive` (`_noMatch`, imported), `$Case key` (`_Fragment`, imported
  from React). Parentheses only where an operand needs them (tsgo's
  precedence).
- Generated names avoid every identifier of the source (`_on1`, …). When no
  `Switch` / `Match` is left, their import is dropped; a declaration left
  empty goes with its line.
- Errors, each at its source position: flow-no-subject, flow-attribute
  (including `key`, spreads, and `exhaustive={…}` after shorthand),
  switch-children, case-no-test, case-both, case-default-value,
  case-default-not-last, case-params, switch-dynamic-exhaustive,
  switch-exhaustive-default. An element with errors becomes `null`.
- Spec: every *Flow control* example passes. Two examples now show the
  generated import (`_noMatch`, `_Fragment`); the *Conditional slots* input
  no longer relies on bindings from another section. Spec examples get an
  implicit `import { Switch, Match, Each }` (fixtures/README.md).
- Fixtures: alias, nested, fresh-names, imports-kept, attribute-position,
  match-nested-pattern, precedence, errors, exhaustive-binding.
  34 of 46 conformance cases pass.

### RGP1-035 — Pass 3: slot hoisting (object form) · M · done
### RGP1-036 — Params on a component · S · done
### RGP1-037 — Conditional slots · S · done
### RGP1-038 — List slots (emit) · M · superseded by RGP1-043 (slots are singular)
`transpiler/slots.go`. One pass rewrites a container in one edit — its
attributes, its slot props and its children — so the four tasks share one
implementation.
- Repeats inside out, like pass 2. Orphaned slot elements are reported
  (orphan-slot) and removed first, so every run makes progress.
- Slot element → object: attributes in order (`"aria-label"` quoted, bare →
  `true`, spreads, `{}` → `undefined`), `children` by the body rule, params →
  `children: (params) => body`. A container left without children becomes
  self-closing.
- Params on a component: its remaining children become
  `{(params) => body}`.
- Conditional slots: a `{…}` child whose conditional chain ends in slot
  elements of one slot or `null` → a conditional prop (`null` →
  `undefined`); `&&`, and `Match` / `Switch` with params, stay orphans; a
  reference `Switch` gives a chain and works.
- List slots: same-named elements → one array; a conditional item is
  spread (`...(c ? [{…}] : [])`). The type answer is `Input.ListSlot`;
  fixtures declare it in `list-slots.txt`, spec examples declare
  `Form.$Field`. RGP1-051 replaces the stub with the checker.
- Errors: orphan-slot, mixed-conditional-slot, duplicate-slot (both kinds),
  slot-key, slot-children-conflict; component-name in pass 0.
- Body rule aligned with its own text: a single `{expr}` child is `expr`
  (the dynamic-`Switch` example showed `<>{n}</>`; fixed).
- Spec examples that used `<Each items>` / `<Select options>` without a
  binding now declare it. 49 of 52 conformance cases pass; the three left
  are segment roots (RGP1-039).

### RGP1-039 — Pass 4: segment roots · M · done
### RGP1-040 — Checks needing the file system · S · done
### RGP1-041 — `Each` key check · S · done
`transpiler/segments.go`, `syntax/segments.go`.
- Pass 4: `#name` → `id="name"` in place; the segment's default export,
  imported as `_<Tag>_<camelName>` (fresh against the source), nested as the
  only child. A root inside another root's children is overwritten with
  them; a second `#name` on one element is segment-id and not mounted.
- Pass 0 on the source: segment-id (explicit `id`, a spread), segment-
  duplicate, segment-in-loop (`.map()` / `.flatMap()` callbacks, `Each`
  bodies), segment-children (warning), segment-import (value imports of a
  `+` file; `import type` and all-`type` named imports are fine — removed
  with the `+` convention, see decisions.md),
  each-no-key (the params form of `Each` only).
- Files: segment-not-found and segment-self (followed through other
  segments), via `Input.Files` and `Input.ReadFile`.
- Spec: the component-root and overwritten-children examples now show the
  generated import.
- **M3 complete: 55 of 55 conformance cases pass** — every example of
  syntax.md, and every fixture.

### RGP1-042 — Rendering a slot: `Slot<Props, Children>`, `slot={$X}` · S · done
Added after the slot design was completed (syntax.md, *Declaration* and
*Rendering a slot*). `transpiler/slotrender.go`, first in pass 3: `<El
slot={$X} args… />` → `{$X ? <El {...$X}>{_renderSlot($X.children, {
args })}</El> : null}`; `key` stays on the element; `slot="…"` and non-`$`
values keep their HTML meaning; slot-render-children. `Slot` is in
`@reactogenic/core`. A function slot rendered without its args is a TS
error on the `.rtsx` line (tested through `reactogenic check`).

### RGP1-043 — The slot model · L · done
syntax.md, *Slots*, rewritten (decisions.md, RGP1-043). Parser patch 0002:
`&name` / `&&name`. Pass 3: attachments (`slot={$X}` with defaults, `&` /
`&&` args, fallback children, `key` kept), recursive slots, last-wins (with an
explicit `$X={…}` and conditionals, provisionally), placement through slot
elements. Removed: list slots, `Input.ListSlot`, `IsListSlot`, the two-round
program build, the `Each` key check, slot-render-children. Core: `Slot<P>`,
`FnSlot<P, A>`, `renderSlot(slot, args, fallback?)` with function-call
typing and arrays rejected. Found on the way: JSX text children were copied
without their leading whitespace (`{a} (` lost its space) — fixed.
- `NOT_ASSIGNED` (#7): conditional slots end in the sentinel, not
  `undefined`; attachments use `isAssigned` / `slotProps`; the helpers a run
  needs are imported in one declaration.

### RGP1-044 — Keyed slots; `Slot<P, A>` · M · done
`KeyedSlot<P>` / `KeyedSlot<P, A>` (syntax.md, *Keyed slots*): slot elements
with React's `key` build a branded object literal (`[_KEYED]: true`, entries
by key; conditional entries spread in; keyed-slot-mixed); an attachment with
`key` renders `_slotEntry($X, key)` — the entry of a keyed slot, a singular
slot as it is — evaluated once. `FnSlot<P, A>` folded into `Slot<P, A>`.
Resolves #4 (keys of slots attached per item). Tests: keyed fixture, spec
examples, a check test (entry props stay type-checked), a runtime render of
a keyed table. The `@reactogenic/core` helper imports of a file are merged
into one declaration.

## M4 — Type service

### RGP1-050 — Project program with virtual `.tsx` · M · done
### RGP1-051 — The list-slot query · S · removed by RGP1-043 (the transpiler needs no types)
`go/internal/project`, over the bridge `rtsx/program.go` (patch 0003).
- An overlay file system (tsgo's `wrapvfs`) serves `Foo.tsx` for every
  `Foo.rtsx` and lists it in directory entries, so tsconfig's own `include`
  globs and extensionless module resolution find it — no glob expansion of
  our own. `.rtsx` is transpiled lazily, on first read.
- List slots: the program is built once with no answers; its checker is
  asked (`rtsx.IsListSlot`: the tag's call signatures → props type →
  property, minus `undefined` / `null` → array or tuple); files whose output
  changes are re-transpiled and the program rebuilt once. The answer depends
  only on declarations, so one round settles it.
- Tests on real temporary projects: `.tsx` ↔ `.rtsx` imports both ways
  with no diagnostics; a type error inside `.rtsx` is reported on its
  virtual `.tsx`; a list slot answered by the checker gives a clean program.
- The conformance harness keeps its `list-slots.txt` stub: fixtures have no
  tsconfig.
- Found: a text-only slot body was an element (`<>text</>`) and could not
  fill `children: string`. Resolved: text alone is a string literal (body
  rule, syntax.md); this test now fills `children: string` with text.

### RGP1-052 — Incremental updates · M · superseded by RGP1-103 (the server's project system is the incremental program)
- A changed file updates the program without a full rebuild.
- The dependencies recorded by RGP1-051 answer "which `.rtsx` files must be
  re-emitted?".
- Set a budget and measure it: the transform of one file on a warm program,
  on the example app (RGP1-064).
- **Done when:** a benchmark exists, and editing `FormProps` re-emits exactly
  the files that use `<Form>`.
- Depends on: 051.

### RGP1-053 — Stdio server · M · done
The Go side of the plugin: a long-lived process that serves `open`,
`transform`, `change` and `close` over stdio
([decisions.md](decisions.md#rgp1-001--checker-and-implementation-language)).
- Choose the message encoding (the OPEN in decisions.md) using the RGP1-052
  benchmark.
- A crash or a protocol error must reach Vite as an error. It must never
  hang the dev server.
- **Done when:** a Node test client can open a project, transform a file and
  receive invalidations.
- Depends on: 050, 052.

## M5 — Vite plugin

### RGP1-060 — Plugin skeleton and `config` · S · done
- Unblocked: the plugin compiles `.rtsx` itself with Vite's
  `transformWithOxc`; no Fast Refresh for `.rtsx` in phase 1 (vite.md).
- `enforce: "pre"`.
- Spawns the Go process (RGP1-053) once per server or build, and stops it
  when Vite closes.
- Add `.rtsx` to `resolve.extensions` and to plugin-react's `include`. This
  resolves the OPEN in vite.md as option (a), or records why not.
- **Done when:** a hand-written `.rtsx` with no extensions renders in
  `vite dev`; an edit reloads the page (no Fast Refresh in phase 1).
- Depends on: 003, 030.

### RGP1-061 — `transform` · M · done
- `.rtsx` goes in; `.tsx` and a v3 source map come out.
- Transpiler errors are thrown with `loc`, so they appear in Vite's overlay
  in dev and fail `vite build`.
- Each transform is one `transform` request to the Go process.
- **Done when:** an error in the browser shows `.rtsx` lines in devtools and
  in the stack trace, and a transpiler error shows in the overlay with a code
  frame.
- Depends on: 060, 053, all of M3.

### RGP1-062 — Resolution · S · done (extensionless `.rtsx` and segment imports; the `.tsx` + `.rtsx` ambiguity error is not raised yet)
- Extensionless imports, including segment imports, go through Vite's
  resolver.
- The `Foo.tsx` + `Foo.rtsx` ambiguity error from RGP1-040 is raised at the
  import.
- **Done when:** `.tsx` → `.rtsx` and `.rtsx` → `.tsx` imports both work in
  dev and in build.
- Depends on: 061.

### RGP1-063 — HMR across type dependencies · M · removed by RGP1-043 (no type dependencies; an edit reloads the page)
When a container's declaration changes, invalidate the `.rtsx` modules that
queried it (RGP1-052), not only the modules that import it.
- **Done when:** in a running dev server, turning `$Field` from an array
  into an object re-emits the page that uses it, with no restart.
- Depends on: 052, 061.

### RGP1-064 — Example app and end-to-end tests · M · done (runtime; a browser run is left)
`packages/vite/test/render.test.ts`: an app built with the plugin for Node
(`vite build --ssr`), imported and rendered by React. It checks what the
compiled code does: per-prop replacement and fallbacks, `&&` props and `&`
args on an attachment run per option, `Match` in a slot body, `Switch`, a
segment root. A real-browser run (Playwright) is left.
Needs your approval first: it creates files outside `specs/`.
- A small Vite app that uses every extension: shorthand props, slots with
  params, list and conditional slots, `Match`, every `Switch` mode, segment
  roots, `Each`.
- Browser tests run it in `vite dev` and against the `vite build` output.
- **Done when:** end-to-end tests are green in CI in both modes.
- Depends on: 062.

**M4/M5 so far.** `reactogenic serve` (`go/internal/server`): stateless,
newline-delimited JSON, `transform` → TSX + v3 map + diagnostics.
`@reactogenic/vite`: `.rtsx` → TSX (Go) → JS (`transformWithOxc`, lang
`tsx`, the incoming map chained); transpiler errors fail the transform at
their `.rtsx` position, warnings go to `this.warn`; one Go process per build
or dev server. Tested end to end with the real binary (built in the test
setup): `vite build` of an app with extensionless `.rtsx` imports, a segment
root and a slot attachment; the dev server's transform (JS, map back to the
`.rtsx`); a transpiler error failing the build at `bad.rtsx:2:7`. Left:
RGP1-052 (incremental program, for `check --watch`), RGP1-064 (a browser
e2e run).

## M6 — `reactogenic check`

### RGP1-070 — CLI skeleton · S · done (npm `bin` shim with RGP1-092)
- `reactogenic check [-p tsconfig]`: the Go binary, run on the RGP1-050
  program, and reached from npm through the `@reactogenic/cli` package and
  its `reactogenic` bin shim, added here.
- Exit code 0 means no errors, 1 means errors.
- `--pretty` / `--no-pretty`, as in `tsc`.
- **Done when:** on a clean project it prints nothing and exits 0.
- Depends on: 050.

### RGP1-071 — Position mapping · M · done
- Copied span → the exact `.rtsx` span. Synthesized span → its origin, per
  the table in [diagnostics.md](diagnostics.md#mapping).
- Related information and message chains are mapped as well.
- Diagnostics in real `.ts` / `.tsx` files pass through unchanged.
- No error may ever point into a virtual file. An unmapped span is a bug and
  fails a test.
- **Done when:** one fixture per row of the origins table passes.
- Depends on: 030c, 070.

### RGP1-072 — Generated-name replacement · S · done
Replace generated names in messages with what the author wrote: `_on` with
the `on` expression, `_Section_aboutUs` with `#about-us`, the `noMatch` alias
with nothing the author would see.
- **Done when:** no fixture's expected output contains a generated name.
- Depends on: 071.

### RGP1-073 — Rewrites · M · done
One subtask and one fixture per row of the rewrites table in
[diagnostics.md](diagnostics.md#rewrites):
- **073a** undeclared-slot
- **073b** missing-slot
- **073c** params-required
- **073d** no-values
- **073e** content-required
- **073f** a required slot filled conditionally
- **073g** switch-missing-case: prints the leftover type as the missing cases
- **073h** segment-not-component
- **073i** segment-props
- **073j** segment-root-props
- **073k** shorthand case B: adds related information, "no `value` in scope"
- **073l** slot-args-missing: a function slot rendered without its args
- **073m** slot-no-args: args to a slot whose body is not a function

Rules match on origin and TS error code, never on message text. This also
resolves the OPEN on exact TS codes.
- **Done when:** all thirteen fixtures pass, and an error that matches no rule
  falls back to TS's own message at the origin.
- Depends on: 071, 072.

### RGP1-074 — Output format · S · done
- `tsc`-style lines. Transpiler errors use their name as the code
  (`error missing-slot:`).
- Pretty mode prints a code frame of the `.rtsx` source.
- **Done when:** snapshot tests of both formats pass.
- Depends on: 071.

### RGP1-075 — Transpiler errors in `check` · S · done
`check` reports the same transpiler errors the Vite transform throws, with
one code path and no duplicates.
- **Done when:** a file with both a transpiler error and a TS error reports
  both, once each.
- Depends on: 061, 071.

### RGP1-076 — `--watch` · S · done (full re-check per change; incremental is RGP1-052)
`reactogenic check --watch`: polls the project's source files (skipping
`node_modules` and hidden directories) and re-checks on a change.
Re-check on change, using the incremental program (RGP1-052).
- **Done when:** an edit re-reports within the budget set in RGP1-052.
- Depends on: 052, 074.

**M6 so far** (`go/internal/check`, `go/cmd/reactogenic`): `reactogenic check
[-p tsconfig|dir] [--pretty=false]` opens the project (RGP1-050), reports
the transpiler's errors once per file, and maps every TS diagnostic in a
virtual `.tsx` back through the transpiler's map — related information
too; diagnostics in real files pass through. Output in `tsc`'s two formats,
with a code frame when pretty; exit status 1 on errors. Tested on temporary
projects: a TS error on a shorthand-rewritten attribute lands on the
attribute the author wrote, and one on a list-slot item's option lands on
that option. Left: generated-name replacement (072), rewrites into slot
terms (073), `--watch` (076).

**RGP1-072/073.** The transpiler records `Output.Notes` — what it
synthesized for which source span (slot prop, body, params, args, arg,
no-match, segment, a bare attribute left `true`) — and `Output.Generated`
(`_Div_num` → `#num`, `_on` → `getStatus()`; per-construct names are unique).
`check` rewrites by note kind, TS code and structured message arguments,
never message text; generated names are replaced in every message. Finer
origins: `$X={` points at its slot element, `children:` at the body. Tested
on one project covering every rule (`internal/check/rewrite_test.go`).
Found: `content-required` cannot be specific — `NotAssigned` in `Slot` turns
TS's "children is missing" into a union mismatch, reported as slot-type; and
tsgo parses files in parallel, so the project's overlay raced on its maps —
fixed with a mutex, `-race` clean.

## M7 — Conformance and release

### RGP1-090 — Spec conformance gate · S · done
`go/internal/conformance/coverage_test.go`: every syntax.md example passes
(strictly); every transpiler code of the *Compile errors* tables is expected
by a fixture; every *types* code and diagnostics.md rewrite is asserted by a
`reactogenic check` test — except rows marked "not specific yet"
(content-required), a documented gap. On its first run it found three gaps,
now filled (arg-without-slot and segment-self fixtures; fixtures may name
their entry in an `entry` file). A mutation check confirmed it fails when a
code loses its fixture.
CI fails if:
- any code-block pair in syntax.md has no passing fixture;
- any error code in the *Compile errors* tables of syntax.md has no fixture;
- any row of the diagnostics.md tables has no fixture.
- **Done when:** the gate is on, and green.
- Depends on: 004, M3, M6.

### RGP1-091 — Getting-started docs · S · done
`docs/getting-started.md` (install, Vite, `check`, the syntax in five minutes,
migrating), and a root `README.md`. The `Button` example was type-checked with the
real binary and React's types: clean, and a broken variant reports
`params-required`.
- Install, `vite.config.ts`, `reactogenic check` in `package.json`.
- Migrating from `.tsx`: a warning about the silent flip.
- **Done when:** a new project reaches a rendering `.rtsx` page by following
  only the docs.

### RGP1-092 — Platform binaries · M · done (CI cross-platform install check left)
`scripts/build-binaries.sh [version] [targets…]` cross-compiles (`CGO_ENABLED=0`,
`-trimpath -s -w`, ~20 MB) into `dist/npm/cli-<platform>-<arch>/` with a generated
`package.json` (`os`, `cpu`, `preferUnplugged`); binaries are never committed.
`@reactogenic/cli`: the `reactogenic` shim and `binaryPath()` (platform package,
or `$REACTOGENIC_BINARY`), with the six platforms as `optionalDependencies`. The
Vite plugin gets its binary from it. Verified: darwin-arm64, linux-x64 (ELF) and
win32-x64 (PE32+) built; the shim ran `check` from a scratch `node_modules`.
- Cross-compile the Go binary for each platform.
- Publish one npm package per platform, as an `optionalDependency` of
  `@reactogenic/cli` (the esbuild model).
- The shim and the plugin find the binary for the current platform, or fail
  with a clear message.
- **Done when:** a clean install on macOS, Linux and Windows runs
  `reactogenic check` and `vite build` from CI.
- Depends on: 003, 053.

### RGP1-093 — Publish · S · done (0.1.0-alpha.0, 2026-09-28)
`scripts/release.sh pack <version>` builds the six binaries and packs all nine
packages into `dist/release/`; `scripts/release.sh publish <otp>` publishes
those tarballs, binaries first, under the prerelease's dist-tag (`alpha`).
`core` and `vite` build to `dist/` (`.js`, `.d.ts`, maps, with `src`);
`publishConfig.exports` points the published packages there while the repo
keeps using `src`. MIT (`LICENSE`); the binary packages carry tsgo's
Apache-2.0 license and notice too. Smoke-tested from the tarballs in a fresh
Vite + React app (`check`, `vite build`, a dev-transform render with
plugin-react), which found `.rtsx` inheriting plugin-react's Fast Refresh
(`$RefreshReg$ is not defined` outside the browser) — fixed, with a test.
Published all nine under `alpha`, then the same smoke test from the registry
(`pnpm add …@alpha` as the docs say), with plugin-react 6.1, TypeScript 7.0
and Vite 8.3: clean. `release.sh publish` resumes across one-time passwords:
a new package's public view lags its publish by minutes, so a name the org
owns with no public versions counts as published. Left: the CI install check
on Linux and Windows (RGP1-092).
- **Next release:** bumping the version also bumps `cli`'s platform
  `optionalDependencies`, which CI's `pnpm install --frozen-lockfile` rejects
  until the lockfile is updated — and pnpm resolves them only once they are
  published. Run `pnpm install` after publishing and commit the lockfile.
- **Before publishing (resolved):** `@reactogenic/core` (and `vite`, `cli`) ship TypeScript
  source that imports with `.ts` extensions — a consumer's `tsc` rejects that
  without `allowImportingTsExtensions` (found checking the docs). Add a build
  step that emits `.js` + `.d.ts`, and point `exports` at it. The packages are
  `private: true` until then.
Versioning, changelog, and publishing to npm. Publishing is outward-facing,
so it happens only on your go-ahead.
- Depends on: 090, 091, 092.

## M8 — IDE support

Spec: [ide.md](ide.md). Research (2026-10-03): ten investigations, then a
five-lens review of the spec, every finding checked by a second pass;
summarized in decisions.md, *IDE support*.

```
100 ─▶ 101 ─▶ 102 ─▶ 103 ─▶ 105 ─▶ 106 ─▶ 107 ─▶ 108 ─┐
               └──▶ 104 ──────┘                        ├─▶ 113 ─▶ 114
100 ─▶ 109 ───────────────▶ 110 ◀── 107 ; 110 ─▶ 111 ──┘
               104 ─▶ 112
```

The fork's pin is frozen through M8 (RGP1-114 re-vendors). No release before
108 and 111.

### RGP1-100 — Research and spec · M · done
ide.md, this milestone, decisions.md.

### RGP1-101 — Patch tooling · S · done
Revised after review: abc39c1 committed two fork files that were in no
patch — regenerating 0006 from `HEAD`'s file list had dropped them, and the
script said nothing. The script now also reads the working-tree patch's file
list, knows which files are ours from a manifest of upstream
(`UPSTREAM.sha256`), refuses a mistyped path and a file with no difference,
and `check-patches.sh` verifies the whole tree offline (CI runs it).
M8 raises the patched upstream files from one to about a dozen; the scripts
were built for patches that add files.
- `regen-patch.sh`: file list from the patch's own headers plus arguments;
  the pristine side rebuilt offline (HEAD, minus the patch); never deletes
  `third_party/tsgo`; a missing patch means a new one, untracked files
  included.
- Rule (go/patches/README.md): one patch owns a given upstream file.
- **Done when:** regenerating a multi-file patch with a file unlisted loses
  nothing; regenerating a patch whose file a later patch also touches is
  refused or succeeds without loss; `pnpm vendor:check` is green.
- Depends on: 100.

### RGP1-102 — The transform as a content mapper · M · done
Three `check` expectations moved by one column, as predicted; the emitted
text of all 67 conformance cases is byte-identical. Over the corpus (3,439
virtual nodes) the span map validates and agrees with `emit.Map`, except two
spans that run backwards in the source: a conditional slot's whole prop,
whose name is copied from after its origin — TS reports on the name or the
value, never on that span. Whitespace is not copied (it would change the
emitted text): prop completion on an empty position is in ide.md's *Not in
the first release*. `oneCopyAnswers` marks later copies per construct.
Revised after review (findings 2.3–2.10, 3.14):
- A diagnostic's span never runs backwards. A zero-length syntax error keeps
  the parser's position; it was moved to the next token, often the next
  line (`check` and Vite printed it there too). An error on whitespace text
  is on the text, not an empty span at what follows.
- A JSX attribute string moved to a JS position is the JS literal of its
  value when it holds `\`, `&` or a line break (ide.md, *Span map*). It was
  copied raw: a multi-line `className` on a slot element was an internal
  error, `&amp;` and `C:\new` changed value. Line breaks are kept, as
  TypeScript and esbuild do (Babel folds them into a space). Fixture
  `slots/attribute-strings`.
- A slot tag that is not an identifier (`<$sub-item>`) is a slot — the `$`
  decides, as syntax.md's grammar table has it. On a component it was a
  valid attribute name; inside a slot it is now a quoted key, the name still
  copied. Pass 0 did not have it for params: the hyphen made the tag
  intrinsic, and `<$sub-item { size }>` was `params-on-html` next to correct
  output. It takes params like any slot element. Fixture
  `slots/hyphen-names`.
- A masked copy emitted twice answers once (a slot's previous value under
  two null branches; fixture `slots/conditional-previous`). The corpus test
  checks each shorthand for exactly its two copies with the table's
  features, and compares all twenty feature bits with the fork's
  declaration.
- The spans that run backwards are the props of a slot first filled by a
  lowered `Match` / `Switch` — with or without a null branch, so a
  `slot-conditional` note does not identify them; the test allows a
  collapsed span on that attribute shape only (five in the corpus now).
- Diagnostics, notes and slot groups found in text that is emitted twice
  (an attachment's fallback) are recorded once.
- "Produced code that does not parse" names the pass that wrote the text.
- Copy names instead of synthesizing them (ide.md, *Span map*): slot tag
  names, attribute names of slot elements, arg keys, closing tag names where
  one is emitted. The emitted text does not
  change. Three `check` expectations move by one column, from `<` / `&` to
  the name (`undeclared-slot`, `slot-type`, `slot-no-args`); diagnostics.md
  *Mapping* and *Rewrites* follow.
- `emit`: every piece carries a feature mask; `Map.Spans()` returns the
  span-map tuples — copied → verbatim; synthesized → atom on its origin,
  no features. No gaps.
- Exports beside the map: slot groups (per owner and name, every tag-name
  span), shorthand sites (the name copy and the value copy), bound bare
  attributes (pass 1, case A).
- Transpiler diagnostics gain a source span (start and end).
- **Done when:** over the conformance corpus the span map validates against
  the fork's `spanmap`; every virtual node of non-zero length maps to the
  same source span as `emit.Map.Source`; no source offset has two
  projections answering the same feature, except the two copies of a
  shorthand.
- Depends on: 101.

### RGP1-103 — Mapped program and a bare server · L · done
The seam is `contentmapper.BuiltIn` (one registered mapper, served by a
`Project` and a `Host` that spawn nothing) plus one-line hooks where the fork
gathers mappers: tsconfig parsing, the session's host, inferred projects.
`lsp.Embedder` gives the server static capabilities and our server info. All
three project shapes passed on the first run. `GOWORK=off` builds (CI checks
it); the binary is 39.6 MB unstripped. Patches 0004–0006.
Review (4.8, 4.9, 3.6, 3.12): the ends of the process are the front's —
`exit` in any state, LSP's exit statuses, input that is not LSP reported, a
watchdog on the client's process (`--clientProcessId`), no goroutine left
behind. The configuration watcher is registered only with a client that
declares it, and never awaited. Code lens, `_vs_references` and
`experimental` are taken out in the bridge; a test holds the capability
keys to ide.md's table. The test client is VS Code's in what changes a
server's answers, and a bare connection drives the bad endings.
Second pass: stopped by the watchdog, the server gives its output queue one
second and ends — a killed editor's unread answers kept it alive for good
when another process held the pipe. The test client reports a change on
disk only where a watcher the server registered matches the path, and
compares URIs as paths (a server writes `,` as `%2C`).
The least-proven bet first: the built-in mapper inside the fork's project
system.
- Patches (decisions.md, *Patched upstream files*): built-in mappers in
  tsconfig parsing (no `runExternalCode` gate, a user `.rtsx` entry
  dropped); the same for inferred projects; a built-in mapper host; `Extra`
  carried with the parsed file; the resolver's extensionless lookup; the
  rtsx grammar as a parse option; a mapped `.rtsx` is always a module.
- The server bridge is its own package, linked only into the `reactogenic`
  binary and the server tests. `go/` still builds with `GOWORK=off`.
- `reactogenic lsp --stdio`, bare: no rtsx-specific feature yet. Static
  capabilities only; no formatting; type acquisition off.
- The Go LSP test client (ide.md, *Testing*) and a fixture project.
- **Done when:** a program of `.ts` / `.tsx` / `.rtsx` importing each other —
  with and without extensions, through a `paths` alias — builds with one
  module per file; and in the server, hover on a copied expression and one
  TS diagnostic land at `.rtsx` positions for (i) a tsconfig with no
  `contentMappers`, (ii) a root tsconfig with `files: []` and `references`,
  (iii) a loose `.rtsx` without a tsconfig; the client receives no
  `client/registerCapability` for `.rtsx`.
- Depends on: 102.

### RGP1-104 — Tolerant transform · M · done
25,640 typing-like mutants of the corpus (17,640 with syntax errors): no
panic, no hang, a valid map every time, and — after a guard in the segment
pass and skipping a nameless `&` — not one stopped pass. The mapper is
tolerant and recovers from a panic.
Revised after review (findings 2.1, 2.2, 2.5, 2.9, 3.9, 1.5); the mutant
test now types Enter too, fails on every error and checks every span:
26,600 mutants, 17,853 with syntax errors.
- Rule 3 was wrong in both directions. An unclosed tag leaves no flag in
  the tree, so valid `$Case`s after one got `orphan-slot`; and any syntax
  error anywhere dropped the diagnostics of root elements and
  `ambiguous-module`. It is now decided on the source parse, by flags and
  by the parser's error ranges, up to the nearest element and no further.
  Patch 0002: the flag of a nameless or detached `&` reaches the attribute.
- Whitespace text with a line break is formatting whatever the parser's flag
  says: after recovery the flag is stale, and the text was reported as
  `switch-children`.
- A `$Case` being typed took the whole `Switch` out of the virtual text: no
  completion in `<$Case is={Status.}`, and with `noUnusedLocals` an error on
  every name used only there. A child or attribute whose error is dropped is
  now left out and the `Switch` / `Match` lowered around it. Where code is
  left out all the same — `null` for a construct that cannot be lowered or
  an orphaned slot, a skipped case with a body — the file says so
  (`Output.Dropped`; the mapper's `File.Stopped`), and rule 4 applies. Both
  server scenarios are tests (`internal/lsp`).
- A panic in a pass is recovered in `Transpile` for a clean source too: the
  last good text, the pass named, the `internal` diagnostic added there. The
  mapper's own `recover` is the last resort.
- The mapper tells the transform which siblings exist, never what they hold
  — what its cache key covers. `segment-self` through another file is
  therefore not reported by the mapped transform (a direct self-mount still
  is); RGP1-106 reports it as a cross-file rule. `check` is unchanged.

Revised again after the fixes were verified (2.1, 2.5):
- Rule 3 over-corrected: a node with no element around it was judged alone,
  and recovery puts the children of an owner whose opening tag is half-typed
  (`<Button variant=>`, `<Switch on=>`, a deleted `>`) at the top of their
  statement — `orphan-slot` on untouched lines. Such a node is now judged
  by its statement. Exhaustive mutants (341,956; 250,901 with syntax
  errors), diagnostics with a code the unmutated case lacks: `orphan-slot`
  15,799 → 314, `flow-as-value` 941 → 0, `flow-no-subject` 60 → 1. What
  remains has no owner tag left (its `<` deleted or misread).
- `Output.Dropped` was not set for a conditional child of an owner that is
  neither a slot's value nor a child — `mixed-conditional-slot`, reported or
  dropped (an unclosed `Match` above a slot element and a child) — nor for a
  `children` attribute that loses to a body.
- Known, not marked: the children of a segment root (overwritten by design);
  an earlier `<$X>` that a later one replaces (last wins); the subject of a
  `Switch` on a reference with no tested case (`<Switch on={s}>` whose first
  `<$Case` is being typed emits `null`).
  > OPEN: emit the subject of such a `Switch` (hover and completion in
  > `on={…}` of an empty `Switch` have nothing to answer). It changes emitted
  > code for valid input, so it is not done here.
- Three pieces of the first revision had no test that failed without them;
  each has one now: `syntax.BlankText` (whitespace after a mismatched closing
  tag gave the slot around it `children: ""`), the span of an error on
  whitespace text, and a plain attribute string staying a copy (hover and
  completion inside `is="loading"`).
- `Input.Tolerant` (ide.md, *Tolerance*): passes on the recovered tree,
  per-pass fallback and the *stopped* mark, suppression under a broken JSX
  element, identity as the last resort, `recover` at the mapper boundary.
  Strict mode unchanged.
- **Done when:** typing-like mutants of every fixture (delete, insert,
  truncate at each token) never panic, never hang and always yield a virtual
  text with a valid map; over the unmutated corpus tolerant and strict
  outputs are byte-identical, diagnostics included; `orphan-slot` under
  `<Button size={>` is suppressed.
- Depends on: 102.

### RGP1-105 — Server features · M · done
A front in the server's process (`internal/lsp/front.go`) answers folding,
selection ranges and closing tags from the source tree — through
`ls.Syntactic`, a new file in the fork — and narrows workspace symbols and
file-rename edits. Its first version deadlocked with the test client (each
blocked writing to the other): writes to the client are now queued. Both
OPENs of ide.md are settled by patches: specifiers are extensionless
(0007), and a cursor at the end of copied text maps back exactly, which
restores auto-import where identifiers are typed. 1,308 requests over typing
mutants: none fails.
Review (4.1, 4.2, 4.6, 4.7): the front's parse takes no file name — a
document's URI is not one, and `untitled:new.rtsx` killed the server — and
a panic in the front is an error answer. `untitled:` leaves the client's
selector. An awaited request is forgotten on any answer. A ranged change
goes through `rtsx/server.ApplyChange`, the server's own line map; one with
a position no document has is dropped before the server (upstream dies of a
line beyond int32). Implementation, call hierarchy and linked editing have
rows in ide.md's table, and scenarios.
Review, the fork (1.3–1.8, 3.1–3.5, 3.10, 4.3–4.5):
- *Specifiers*: the sibling guard probed a relative path against the
  server's directory, so `./button` was written next to `button.ts`. It is
  decided on the module's absolute path now, for every kind of specifier.
- *Auto-import beside a generated import*: the import lands on line 1 in a
  mounter or container without imports. The first fix mapped every position
  in generated text before the source to the source's start — so a name
  that TS adds *into* the generated import (`Each`, `Slot`, a mounted
  module's export) was written as `, Each` at 1:1. Now the mapper names the
  positions where a statement can go (the line starts there,
  `MapperResult.StatementStarts`); any other stays an atom. And a generated
  import is no existing import to TS's import fixes (`ls/autoimport/fix.go`):
  such a name gets a declaration of its own — also in a container with
  imports of its own, where it was never offered.
- *Document symbols* moved to the source tree, with folding: on the virtual
  text a declaration that holds any rtsx construct had its name as its
  range, and generated object keys were symbols.
- *Inlay hints*: once each (a range maps to many copied runs; each run gave
  the enclosing function's hints again); none on a generated call.
- *Organize imports* is not offered next to a generated import, instead of
  an action with no edit. Deferred — ide.md, *Not in the first release*:
  making it work needs the generated import on a line of its own
  (transpiler).
- *Definition* on a `paths` specifier of an `.rtsx` module.
- *Workspace symbols and file rename* are narrowed in the fork
  (`Embedder.Owns`): symbols before the cut to 256; rename per import, so a
  mixed batch and a folder are right, and the client is asked about folders.
  The front's two rewrites are gone; `textDocument/rename` on a specifier
  (a client without `willRenameFiles`) stays whole.
- *File rename, second pass*: specifiers are written for the files as they
  will be (`renamedHost`). `util.ts` → `util.rtsx` was its own sibling —
  still on disk when the server is asked — and every importer got
  `./util.rtsx`; a folder renamed with `button.rtsx` and `button.ts` in it
  got `./kit/button`, the sibling's. A generated import is left out of the
  rewrite: it made every edit of a moved mounter unmappable, so its own
  imports did not follow it.
- `BuiltInMappers` is one slice per registration: the inferred project was
  rebuilt on every open and close. `rtsx.NewProgram` reads a referenced
  project from source.
- Deferred — ide.md, *Not in the first release*: auto-import of a
  dependency's `.rtsx` exports (TS's dependency index has its own host and
  resolver).
Review, the missing tests (3.8, 3.11, 3.13, 3.15): project shapes (a
`contentMappers` entry, `.rtsx` only, `paths`, two projects, composite
references, `extends`, a loose file) and text shapes (CRLF, a BOM,
non-ASCII and astral characters, a utf-8 client) as tables; semantic tokens
by type and none on copied names; a shorthand answering as both symbols;
arg and slot-attribute names; a built-in sibling winning the import; a
`.ts` change on disk; a segment file's rename (no edit); a pass made to
fail keeps the text before it (`transpiler.TestStoppedKeepsPreviousPass`),
and a panic is caught at the mapper boundary. The typing test runs over the
conformance corpus with signature help — 15,020 requests — tolerating one
upstream panic (go/patches/README.md, *Known upstream defects*). Run over
the transform branch's new fixtures it met a second one, which is patched:
completion in a file that is only comments (or comments under an import the
transform drops) was an error — upstream computes an auto-import's edit for
every item of a mapped file, and indexed past a text with no statement
(`ls/change/tracker.go`).
- Folding and selection ranges from the source tree; closing-tag insertion
  (*Tags*); workspace symbols and file rename as in ide.md's table;
  `reactogenic --version`, the same string in `serverInfo` and in the
  transform's identity.
- The auto-import boundary case (ide.md, second OPEN) decided by a test.
- **Done when:** one scenario per row of ide.md's feature table except
  diagnostics and rename; completion right after `iconSize.` in a half-typed
  file; a multi-line `<Button>` with slots, `<$Icon>` and `<Switch>` each
  fold; hover and completion over the mutants of 104 never panic; a smoke
  scenario through the built binary.
- Depends on: 103, 104.

### RGP1-106 — `reactogenic check` on the mapped program · M
- First, golden `check` output from today's overlay: every project of the
  check suite, the Vite test app, one project per row of diagnostics.md
  *Rewrites*.
- The reporting layer (ide.md, *Diagnostics*): `Report(program, file)`,
  written against both hosts. Positions through the file's span map; notes
  looked up with `emit.Map.Source`; `segment-self` as a cross-file rule;
  the TS5097 drop; the merge rule.
- The overlay file system and the `.rtsx.tsx` alias go, with their
  `ambiguous-module` copy (the transpiler's report remains).
- A tsconfig with `references` and no files of its own: each referenced
  project is checked, each file reported once.
- syntax.md *Segment files*, diagnostics.md and decisions.md lose their
  overlay text.
- **Done when:** the goldens reproduce except the merged duplicates; an
  error inside a mounted segment is reported once; an `.rtsx`-only project
  reports no TS18003; the Vite template layout prints what
  `-p tsconfig.app.json` prints; `--watch` picks up an `.rtsx` edit; a
  tsconfig with a `contentMappers` entry changes nothing.
- Depends on: 103, 105.

### RGP1-107 — Diagnostics in the server · M
- The reporting layer behind a hook in the server's diagnostics path, before
  synthesized diagnostics are aggregated; it produces the LSP diagnostics
  itself (string codes, severities). A refresh request after a change to any
  file.
- The directory listing in the transform's cache key; the transform reads
  names through the snapshot's file system (ide.md, *Segments*).
- **Done when:** for the projects of 106's goldens without syntax errors,
  the errors and warnings pulled for each `.rtsx` document equal `check`'s
  lines for that file (code, message, line, column); creating and deleting
  `intro.rtsx` — on disk, and as an unsaved buffer — changes `page.rtsx`'s
  pulled diagnostics with no edit to it.
- Depends on: 106.

### RGP1-108 — Slots, segments, rename · L
- A request hook in the server's dispatch; rename locations exported from the
  fork before write-back.
- Slot-name completion; requests on any element of a slot group; `#name`
  definition and completion; the *Rename* table and its post-check;
  `reactogenic/transpiled`.
- A request on the closing tag name of a component emitted self-closing
  (all its children are slots: there is no closing name to copy) is answered
  at its opening tag name, the answer's range set back — the pairing on the
  source tree that rename needs anyway. Today `</Card>` there answers
  nothing: pinned by `TestClosingTagOfComponent` (review 3.13).
- **Done when:** `<$` inside `<Button>` lists exactly its `$` props, also
  after a keyed slot's first entry; hover and rename work from the second
  `<$Column key=…>` and from `</$X>`; definition on `#intro` returns
  `intro.rtsx`; `#` lists unmounted siblings; every row of the *Rename* table
  has a test, a refusal reaches the client as an error, and no rename in the
  fixture project leaves it with a new diagnostic.
- Depends on: 107.

### RGP1-109 — Grammar · M · done
`packages/vscode` (package `rtsx`, private): the TSX grammar of VS Code
1.140.0 (TypeScript-TmLanguage `48f6086`) vendored with its notices, and
`grammar/generate.mjs`, which writes the grammar (6 rules added, 3 changed),
the Markdown fence injection, both language configurations, the snippets and
`ThirdPartyNotices.txt`; 17 upstream changes are tested to fail it. Beyond
the research: a sigil followed by a non-name stops before the tag end; a
line comment after `=` and a comment line before a multi-line spread
tokenize as in TSX.
*Tests* (316): 101 scope cases, each checking that the next line is code
again; equality with `source.tsx`, whitespace included, on the repo's 29
`.tsx` files and 34 snippets; no `invalid.*` token and no open tag in the 58
`.rtsx` files and 29 syntax.md examples (20 of the 87 fail under the TSX
grammar).
*Measured once, outside the suite*: none of the upstream grammar's 466 test
inputs differs; of 1,073 local `.tsx` files only an intermediate-pass
fixture that still holds `.rtsx` forms; of 360 TypeScript test files 4 —
element-valued attributes (2), invalid TSX (1), a generic arrow the TSX
grammar itself reads as a tag (1). In a running VS Code 1.140.0 (isolated
profile): language `rtsx`, the scopes with bare sigils, Emmet, a TSX
snippet, the Markdown fence; `vsce package` accepts the manifest.
*Left*: Enter between slot tags and comment toggling were not re-run in the
editor (its window had no focus; the research ran both on the same two
configurations) — 110's editor suite has the comment toggle, not yet the
Enter. `.vscodeignore` and the manifest's commands, task and matcher are
110's. `x=` typed directly before `>`, and an unclosed `{`, derail as in TSX.
- Vendored TSX grammar with its notices, the generator, the generated
  `rtsx.tmLanguage.json`, the two language configurations. A `grammar` step
  in CI's `js` job.
- **Done when:** the scope assertions pass, with each form directly before
  `>` and with a bare sigil; plain TSX tokenizes as under `source.tsx`; no
  `.rtsx` file of the repo has an `invalid.*` token; regenerating changes
  nothing.
- Depends on: 100.

### RGP1-110 — VS Code extension · L · done (the slot-term diagnostic test waits for 107; *Show transpiled TSX* for 108)
Built ahead of 107 and 108, against the server of 105. `src/` (eight files,
one 450 KiB CommonJS bundle): the client over stdio, attached to `rtsx` on
`file` and `untitled`; the resolver of ide.md's table, without a `vscode`
import; the status item; restarts; closing tags; the two commands; the
`check` task and `$reactogenic`. The server's pull diagnostics and the
matcher share one owner, so an opened document's diagnostics replace what
the task left for it. No binary found (the universal `.vsix` in a project
without the CLI) is a quiet state — the status item says so, no notification
— and was not run in the editor.
*Unit tests* (61, in `pnpm test`): the order of the table; a missing explicit
path is an error; the walk up through npm's and pnpm's layouts and this
repository's own install; the version gate (`alpha.10` > `alpha.9`, 19
pairs); trust; the Windows copy (platform injected, on macOS); the matcher
against the output of `reactogenic check --pretty=false` on `test/fixture`,
built from the checkout. Eight mutants of the resolver: all killed.
*Editor suite* (`pnpm test:editor`; VS Code 1.140.0, macOS arm64, isolated
profile, two windows, about 10 s): trusted — 13 pass, 1 skipped: language id;
one server process (counted with `ps`), named by the status item; exactly one
hover and one definition at `<$Icon`; `>` after `<$Icon { size }` inserts
`</$Icon>` with the cursor between; nothing with the setting off; Toggle Line
Comment writes `{/* some text */}`; the task reports `undeclared-slot` at 9:10
of a closed document; TS2322 from the server at its `.rtsx` position; a
setting change, a lockfile change and the command each restart; a missing
path, and a binary without `lsp`, are errors. Untrusted — 3 pass: restricted
mode, language id `rtsx`, no server process, also after *Restart Server*.
The same suite passes against the packaged darwin-arm64 `.vsix` with its
bundled binary.
*Packaging* (`pnpm package`): built and inspected for darwin-arm64 (9.5 MB;
the binary 28.3 MB, executable bit kept, `TargetPlatform` set) and universal
(135 KB, no binary); each staged in a clean folder with `LICENSE`, the
grammar's and the bundle's notices (nine npm packages), and — with a binary —
tsgo's `LICENSE` and `NOTICE`.
*Found on the way*: the fork answers an unknown method with InvalidRequest
(-32600), not MethodNotFound — the client reads both as "too old";
`@vscode/test-electron`'s runner always disables workspace trust, so the
suite launches VS Code itself; after a snippet inserted at the cursor,
`editor.selection` in the extension host lags, so the test types the next
character instead.
*Left*: the skipped test `TODO(RGP1-107)` (the server reports TS2322 at
`$Badge` today, not `undeclared-slot`); the "shown" branch of *Show
transpiled TSX*, never run (108); the `vscode` CI job and Linux under `xvfb`
were never run — nothing is pushed; the other five platform `.vsix` were not
built; the Windows copy was not run on Windows; the restart on a trust grant
is wired and not tested (no API grants trust); `MIN_CLI_VERSION` is
`0.1.0-alpha.1`, to be the first release with `lsp` (113); the Go modules'
notices are missing from the binary's licences, as in the npm packages; the
Marketplace page (README for users, icon, changelog) is 113's.
*After review* (one major, nine minor findings; each reproduced by a test
that failed, then fixed): an untitled `rtsx` document was parsed as plain
TypeScript — the server's front now serves a document that is `rtsx` by
language id alone under a name ending in `.rtsx` (`internal/lsp/alias.go`,
no patch to the fork); a restart cleared the task's problems of closed
documents — the collection of the owner `reactogenic` now lives as long as
the window; a `.ts` line of `check` showed twice once the file was open —
a second matcher, `$reactogenic-ts`, owner `typescript`; a document outside
the workspace chose the binary, for the *check* task too — only a document
inside a workspace folder decides now, and each start decides again; a
lockfile above the opened folder was not watched; a binary that never
answers `initialize` blocked every later restart — the extension now starts
the process itself, gives a start 10 s, and a restart gives up a start under
way; the status item named a server that had crashed for good; `--stdio`
was passed twice; several cursors got one cursor's closing tag — one
request per cursor, inserted as snippet edits of one workspace edit; the
transpiled document was TSX, so VS Code's TypeScript reported on it — it is
`page.transpiled.rtsx`, language `rtsx`. The editor suite is four windows
now (trusted 21 pass and 1 skipped, untrusted 3, `monorepo` 6, `transpiled`
4 — the last against a stand-in server with `reactogenic/transpiled`), and
passes against the packaged darwin-arm64 `.vsix`; 397 unit tests. *Still left*: an untitled document belongs to no project (TS7026,
TS2875 on its JSX; relative imports do not resolve); the client's own
"couldn't create connection" notification still shows beside ours when a
start fails; a server that spawns children is killed without them; the
no-folder window and the multi-root case have unit tests only; nothing ran
on Linux or Windows.
- `packages/vscode`: manifest (ide.md, *Contributes*), client, binary
  resolution, status item, restart triggers, commands, closing-tag
  insertion, the `check` task and matcher, packaging per platform plus
  universal. Its `test` script runs unit tests; the editor suite is
  `test:editor`, in a `vscode` CI job under `xvfb`.
- **Done when:** resolution-order unit tests pass (versions compared with
  pre-release tags, `alpha.10` > `alpha.9`); in an untrusted workspace no
  server process starts; the editor suite, in an isolated profile: language
  id `rtsx`, a slot-term diagnostic at its position, exactly one hover and
  one definition result, `>` after `<$Icon { size }` inserts the closing
  tag, Toggle Line Comment inside a JSX child writes `{/* */}`.
- Depends on: 107, 109 (*Show transpiled TSX*: 108).

### RGP1-111 — The `.ts` side: TS server plugin · M · done
`serve` has a second request, `virtual` (`internal/server/virtual.go`): a
batch of files, each with its text, through the tolerant transform of
`reactogenic lsp` (`mapper.Transform`); for each, the virtual text and the
span tuples in UTF-16 offsets, `export {}` appended to a file that is no
module. The files of a batch are transformed on every processor. The Vite
plugin's `transform` is untouched.
The plugin is `packages/vscode/src/plugin` (five files, no `vscode` import,
`resolve.ts` bundled in): built by `scripts/build.mjs` into
`node_modules/reactogenic-typescript-plugin`, where `tsserver` finds it by
name — from the checkout and, at the same path, in every `.vsix`. `vsce`
ignores `node_modules` unless it packages dependencies: the staged manifest
names the plugin as its one dependency, and `package.mjs` fails if the
`.vsix` lacks it. 20 KiB minified. The extension sends the setting and the
workspace's trust (`src/tsPlugin.ts`).
*How*: `tsserver` keeps the `.rtsx` source as the file's text — it reads it,
watches it, and counts lines in it. The plugin replaces what the project
hands the program (snapshot, version, script kind TSX) and wraps the
language service: positions out through the span map, positions in the
other way, line and column from the source. Not Svelte's way (the text
replaced at `readFile`, the line maps patched after it).
*Decided here* (each in ide.md): `spawnSync` per batch — the numbers below;
a batch is what an importer resolves to plus the `.rtsx` files beside
those; a text is cached by the source `tsserver` holds, not by mtime and
size, since it is `tsserver` that reads the file; workspace symbols and
file-rename edits of `.rtsx` are **left out**, not mapped as the spec had
it — `reactogenic lsp` answers both, and an edit made by both servers would
be applied twice; the plugin finds its binary itself and takes the
workspace's CLI only on the extension's word that the workspace is trusted
(it is loaded in restricted mode too, and before the extension); a tsconfig
`plugins` entry configures nothing; an import `tsserver` writes for an
`.rtsx` module loses its extension where that is the same module.
*Measured* (M2 Pro, macOS 27.0, the release build, 28.5 MB): one `serve`
process, `virtual` with 1 / 10 / 50 / 200 files of 8 KB — 11 / 22 / 65 /
200 ms warm (a process that does nothing: 6 ms); 300–520 ms the first time
a freshly copied binary runs. A project without `.rtsx` whose imports all
resolve: nothing. One with 600 files and 13 unresolved imports in each:
400 → 470 ms to load — each failed import is looked up a second time; 890 ms
before the lookups of one program build shared a cache.
*Tests* (`pnpm test`; 463 in the package, the plugin's about 20 s of it on
an idle machine): 15 on the mapping; 22 scenarios in `tsserver` 5.9.3 and
6.0.3, loaded as VS Code loads it — beyond the Done when: related
information, call hierarchy both ways, `references-full`, highlights, file
references, aliased and `node16` specifiers, auto-import, a file created
and deleted, a binary that fails, is replaced, is too old, the process
counts, an untrusted workspace's CLI and a tsconfig entry that run nothing.
21 mutants of the plugin: 19 killed; two cannot be reached through
`tsserver` (its `rename` never asks for locations after a refusal; the
files an importer resolves to are also found beside each other). Editor
suite `typescript` (VS Code 1.140.0, built-in TypeScript 6.0.3): `main.tsx`
before the extension is activated and after — 2 pass, from the checkout and
from the packaged darwin-arm64 `.vsix` with its bundled binary. The other
four suites once: 21 + 1 skipped, 3, 4 pass; `monorepo` — its window closed
after 3 of 6 with no failure reported; alone, 6 pass.
**Not explained**: of nine runs of the `typescript` suite, three in a row
ended the same way — the window closed a second into the first test, no
test reported as failed, the extension host's log saying "received
terminate message from renderer"; the runs before and after passed, with
the same code. Other tasks ran editor suites on this machine at the time.
A suite that fails now waits a second before it ends, so that its report
is not lost with the window (`test/editor/harness.ts`).
*Found on the way*: TypeScript's *Move to file* fails in any program that
holds a file of an extension it does not know ("has unknown extension") —
the plugin hides the `.rtsx` files for that call; `tsserver`'s language
service takes its program from the project (`updateFromProject`), not from
the host's versions, so making a text again needs the project marked as
changed — `markAsDirty`, not in the public types; while a project loads, VS
Code's syntax server answers go-to-definition with the import itself.
*Left*: Windows — spawn cost unmeasured, nothing run there (the OPEN
stays); Linux not run; TypeScript < 5.0 has no `resolveModuleNameLiterals`:
the plugin does nothing; a rename is refused on the locations of the
file's own project — with several projects loaded, another's are not looked
at; a segment file created or deleted changes its mounter's text at the
mounter's next save (its exports do not depend on it); a call in a slot's
body is listed under the component, with no caller of its own; with the
universal `.vsix`, no binary before the extension is activated
(`workspaceContains:**/*.rtsx` would activate it at startup — and start the
server with no `.rtsx` document open); `MIN_CLI_VERSION` must be the first
release with `virtual` (113); no `vscode` CI run.
- `serve` gains tolerant mode and span tuples. The plugin (ide.md, *The
  `.ts` side*), added to the extension's manifest and `.vsix`.
- **Done when:** `tsserver` driven over stdio reports no TS2307 for
  `import { Page } from "./page"` in a `.tsx` file; go-to-definition and
  references land in `page.rtsx` at source positions; no response contains a
  virtual offset in an `.rtsx` file; F2 on a prop used as shorthand and on a
  `$Slot` member is refused and changes no file; a saved `.rtsx` with a
  syntax error keeps its importer free of "no exported member"; a project
  without `.rtsx` spawns nothing.
- Depends on: 104, 110.

### RGP1-112 — Stock content mapper · S · done
`internal/stockmapper`: the protocol on the standard library; the built-in
transform plus a post-pass that writes `.rtsx` into extensionless imports
and module augmentations (relative and `paths`; an alias that cannot carry
the extension becomes a relative path) and `export {}` into a file that is
not a module; 28 numbered codes below 1000 (a syntax error keeps TS's number,
and is sent only when the virtual text parses: while it does not, TypeScript
reports the mistake itself); a `.tsx` / `.ts` segment import written without
its extension; two ignore directives. Measured with
`typescript@7.1.0-dev.20261003.1` (`scripts/e2e-stock-mapper.sh`, twelve
`tsc --runExternalCode` runs, each with exactly its expected errors): a clean
project with a `.tsx` segment, three kinds of alias and an augmentation exits
0; type errors at `page.rtsx(11,15)`, `(16,14)` and `button.rtsx(15,7)`;
`reactogenic101` at `(16,7)` hides the type error next to it; TS2306 for a
`.ts` segment that is not a module; 0.044 s to start through the Node
launcher, 0.003 s for four transforms. By hand, the same project in
`tsc --lsp`: hover, definition, references, and completion after `status.`,
at source positions.
Reviewed and fixed (2026-10-03): a syntax error was reported twice, by
TypeScript and by the mapper at another place (4,296 of 17,640 broken
mutants; now none, by a test); exact aliases, directory aliases and patterns
with a suffix stayed unresolved; the segment directive hid TS2306;
`reactogenic check` reported TS18068 on the documented tsconfig — it now
drops it, ahead of 106.
Left: Windows and VS Code's own client were not run; in the stock server,
go to definition on a `paths` specifier of a mapped file returns nothing —
also for one written in a `.tsx` file; on that project `check` prints
TS5097 under `page.rtsx.tsx` (the overlay's alias of a module a `.tsx` file
imports by its full name — gone with 106).
- `reactogenic content-mapper` and the manifest in `@reactogenic/cli`
  (ide.md, *Stock TypeScript 7.1*): numeric codes for transpiler
  diagnostics, `recover` around every transform, a generated segment import
  that raises no TS5097. Experimental.
- **Done when:** `typescript@next`'s `tsc --runExternalCode` checks a project
  with `.rtsx` files and reports at `.rtsx` positions.
- Depends on: 104.

### RGP1-113 — Docs and release · S
- getting-started (*Editor*), README, a changelog line for the `check`
  changes (moved columns, merged duplicates, project references).
- Versions: npm `0.1.0-alpha.N`; the extension `0.1.N`, published as a
  pre-release (the Marketplace has no pre-release tags). The build stamps
  the version into the binary; it prints the six stripped sizes, and a
  growth of more than 10% over the previous release needs a note in
  decisions.md.
- Order: platform packages and `@reactogenic/cli`; lockfile; the `.vsix`
  files from the same binaries, never rebuilt; Marketplace and Open VSX. The
  `typescript.contentMapper` manifest is not published before the binary
  that has `content-mapper`.
- **Done when:** CI builds seven `.vsix` files and each platform one starts
  its bundled binary and answers `initialize`; a fresh project on the
  published alpha shows "workspace" in the status item, and one on
  0.1.0-alpha.0 shows "bundled" with the reason. Publishing — npm, the
  Marketplace, Open VSX — happens on your go-ahead.
- Depends on: 108, 110, 111.

### RGP1-114 — Re-vendor · M
The fork at TypeScript 7.1's beta or later; patches rebased.
- Depends on: 113.

## Not in phase 1

Type errors in the Vite dev overlay; everything in `specs/later/`; from
ide.md: formatting, a tree-sitter grammar, the *Not in the first release*
table.
