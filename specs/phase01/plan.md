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
- **Done when:** `<$` inside `<Button>` lists exactly its `$` props, also
  after a keyed slot's first entry; hover and rename work from the second
  `<$Column key=…>` and from `</$X>`; definition on `#intro` returns
  `intro.rtsx`; `#` lists unmounted siblings; every row of the *Rename* table
  has a test, a refusal reaches the client as an error, and no rename in the
  fixture project leaves it with a new diagnostic.
- Depends on: 107.

### RGP1-109 — Grammar · M
- Vendored TSX grammar with its notices, the generator, the generated
  `rtsx.tmLanguage.json`, the two language configurations. A `grammar` step
  in CI's `js` job.
- **Done when:** the scope assertions pass, with each form directly before
  `>` and with a bare sigil; plain TSX tokenizes as under `source.tsx`; no
  `.rtsx` file of the repo has an `invalid.*` token; regenerating changes
  nothing.
- Depends on: 100.

### RGP1-110 — VS Code extension · L
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

### RGP1-111 — The `.ts` side: TS server plugin · M
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

### RGP1-112 — Stock content mapper · S
- `reactogenic content-mapper` and the manifest in `@reactogenic/cli`
  (ide.md, *Stock TypeScript 7.1*): numeric codes for transpiler
  diagnostics, `recover` around every transform, an ignore directive on the
  specifier of a generated segment import. Experimental.
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
