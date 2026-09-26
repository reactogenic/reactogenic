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
| M4 | Type service | 050–059 | list slots emit correctly from real props types; the Go process serves the plugin |
| M5 | Vite plugin | 060–069 | example app runs in `vite dev` and `vite build` |
| M6 | `reactogenic check` | 070–089 | every *types* error in syntax.md is reported on the `.rtsx` |
| M7 | Conformance and release | 090–099 | spec ↔ tests complete, packages ready to publish |

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

### RGP1-010 — Slot types and `renderSlot` · S
`SlotFn`, `OptionalSlotFn` and `renderSlot`, exactly as in
[syntax.md](syntax.md#declaration-plain-tsx-no-extension).
- **Done when:** a container written in plain `.tsx` against these types
  type-checks, and so does the whole *Declaration matrix*. Each "error" cell
  in the matrix is a type-level test.

### RGP1-011 — `Each` · S
The component from [syntax.md](syntax.md#the-component). No fragment, no
wrapper.
- **Done when:** unit tests render it with React, and generic inference gives
  `item: T`.

### RGP1-012 — `Switch`, `Match`, `$Case`, `noMatch` declarations · S
- Declarations only: `Switch` is a container with a `$Case` list slot, so the
  slot errors work on it.
- If they are ever rendered at runtime (the transform was bypassed), they
  throw a clear error.
- `noMatch(value: never): never` throws, with the value in the message.
- **Done when:** the declarations drive undeclared-slot and orphan-slot in
  the RGP1-043 fixtures.

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
### RGP1-038 — List slots (emit) · M · done (type answer stubbed until RGP1-051)
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
  `+` file; `import type` and all-`type` named imports are fine),
  each-no-key (the params form of `Each` only).
- Files: segment-not-found and segment-self (followed through other
  segments), via `Input.Files` and `Input.ReadFile`.
- Spec: the component-root and overwritten-children examples now show the
  generated import.
- **M3 complete: 55 of 55 conformance cases pass** — every example of
  syntax.md, and every fixture.

## M4 — Type service

### RGP1-050 — Project program with virtual `.tsx` · M
One tsgo compiler host with an in-memory overlay, shared by the plugin (via
RGP1-053) and the CLI.
- It loads `tsconfig.json` and expands its `include` / `exclude` globs to
  `.rtsx` as well, because TS ignores unknown extensions.
- Each `Foo.rtsx` is served as an in-memory `Foo.tsx`, so TS resolves
  extensionless imports and segment imports on its own.
- **Done when:** a project that mixes `.tsx` and `.rtsx` type-checks, with
  imports going both ways.
- Depends on: 001, 030.

### RGP1-051 — The list-slot query · S
Implements `isListSlot` against the program.
- It resolves the container tag's props type, looks up `$X`, removes
  `undefined` / `null`, and checks for an array or a tuple.
- It records each query as a dependency: this file asked about container `P`,
  which is declared in file `F`.
- **Done when:** the RGP1-038 fixtures pass with the real query.
  Containers declared in `.tsx`, in `.rtsx`, and imported from a package are
  all covered.
- Depends on: 038, 050.

### RGP1-052 — Incremental updates · M
- A changed file updates the program without a full rebuild.
- The dependencies recorded by RGP1-051 answer "which `.rtsx` files must be
  re-emitted?".
- Set a budget and measure it: the transform of one file on a warm program,
  on the example app (RGP1-064).
- **Done when:** a benchmark exists, and editing `FormProps` re-emits exactly
  the files that use `<Form>`.
- Depends on: 051.

### RGP1-053 — Stdio server · M
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

### RGP1-060 — Plugin skeleton and `config` · S
- `enforce: "pre"`.
- Spawns the Go process (RGP1-053) once per server or build, and stops it
  when Vite closes.
- Add `.rtsx` to `resolve.extensions` and to plugin-react's `include`. This
  resolves the OPEN in vite.md as option (a), or records why not.
- **Done when:** a hand-written `.rtsx` with no extensions renders in
  `vite dev`, with Fast Refresh working.
- Depends on: 003, 030.

### RGP1-061 — `transform` · M
- `.rtsx` goes in; `.tsx` and a v3 source map come out.
- Transpiler errors are thrown with `loc`, so they appear in Vite's overlay
  in dev and fail `vite build`.
- Each transform is one `transform` request to the Go process.
- **Done when:** an error in the browser shows `.rtsx` lines in devtools and
  in the stack trace, and a transpiler error shows in the overlay with a code
  frame.
- Depends on: 060, 053, all of M3.

### RGP1-062 — Resolution · S
- Extensionless imports, including segment imports, go through Vite's
  resolver.
- The `Foo.tsx` + `Foo.rtsx` ambiguity error from RGP1-040 is raised at the
  import.
- **Done when:** `.tsx` → `.rtsx` and `.rtsx` → `.tsx` imports both work in
  dev and in build.
- Depends on: 061.

### RGP1-063 — HMR across type dependencies · M
When a container's declaration changes, invalidate the `.rtsx` modules that
queried it (RGP1-052), not only the modules that import it.
- **Done when:** in a running dev server, turning `$Field` from an array
  into an object re-emits the page that uses it, with no restart.
- Depends on: 052, 061.

### RGP1-064 — Example app and end-to-end tests · M
Needs your approval first: it creates files outside `specs/`.
- A small Vite app that uses every extension: shorthand props, slots with
  params, list and conditional slots, `Match`, every `Switch` mode, segment
  roots, `Each`.
- Browser tests run it in `vite dev` and against the `vite build` output.
- **Done when:** end-to-end tests are green in CI in both modes.
- Depends on: 062.

## M6 — `reactogenic check`

### RGP1-070 — CLI skeleton · S
- `reactogenic check [-p tsconfig]`: the Go binary, run on the RGP1-050
  program, and reached from npm through the `@reactogenic/cli` package and
  its `reactogenic` bin shim, added here.
- Exit code 0 means no errors, 1 means errors.
- `--pretty` / `--no-pretty`, as in `tsc`.
- **Done when:** on a clean project it prints nothing and exits 0.
- Depends on: 050.

### RGP1-071 — Position mapping · M
- Copied span → the exact `.rtsx` span. Synthesized span → its origin, per
  the table in [diagnostics.md](diagnostics.md#mapping).
- Related information and message chains are mapped as well.
- Diagnostics in real `.ts` / `.tsx` files pass through unchanged.
- No error may ever point into a virtual file. An unmapped span is a bug and
  fails a test.
- **Done when:** one fixture per row of the origins table passes.
- Depends on: 030c, 070.

### RGP1-072 — Generated-name replacement · S
Replace generated names in messages with what the author wrote: `_on` with
the `on` expression, `_Section_aboutUs` with `#about-us`, the `noMatch` alias
with nothing the author would see.
- **Done when:** no fixture's expected output contains a generated name.
- Depends on: 071.

### RGP1-073 — Rewrites · M
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

Rules match on origin and TS error code, never on message text. This also
resolves the OPEN on exact TS codes.
- **Done when:** all eleven fixtures pass, and an error that matches no rule
  falls back to TS's own message at the origin.
- Depends on: 071, 072.

### RGP1-074 — Output format · S
- `tsc`-style lines. Transpiler errors use their name as the code
  (`error missing-slot:`).
- Pretty mode prints a code frame of the `.rtsx` source.
- **Done when:** snapshot tests of both formats pass.
- Depends on: 071.

### RGP1-075 — Transpiler errors in `check` · S
`check` reports the same transpiler errors the Vite transform throws, with
one code path and no duplicates.
- **Done when:** a file with both a transpiler error and a TS error reports
  both, once each.
- Depends on: 061, 071.

### RGP1-076 — `--watch` · S
Re-check on change, using the incremental program (RGP1-052).
- **Done when:** an edit re-reports within the budget set in RGP1-052.
- Depends on: 052, 074.

## M7 — Conformance and release

### RGP1-090 — Spec conformance gate · S
CI fails if:
- any code-block pair in syntax.md has no passing fixture;
- any error code in the *Compile errors* tables of syntax.md has no fixture;
- any row of the diagnostics.md tables has no fixture.
- **Done when:** the gate is on, and green.
- Depends on: 004, M3, M6.

### RGP1-091 — Getting-started docs · S
- Install, `vite.config.ts`, `reactogenic check` in `package.json`.
- Migrating from `.tsx`: a warning about the silent flip.
- **Done when:** a new project reaches a rendering `.rtsx` page by following
  only the docs.

### RGP1-092 — Platform binaries · M
- Cross-compile the Go binary for each platform.
- Publish one npm package per platform, as an `optionalDependency` of
  `@reactogenic/cli` (the esbuild model).
- The shim and the plugin find the binary for the current platform, or fail
  with a clear message.
- **Done when:** a clean install on macOS, Linux and Windows runs
  `reactogenic check` and `vite build` from CI.
- Depends on: 003, 053.

### RGP1-093 — Publish · S
Versioning, changelog, and publishing to npm. Publishing is outward-facing,
so it happens only on your go-ahead.
- Depends on: 090, 091, 092.

## Not in phase 1

Language service and editor support ([../later/tooling.md](../later/tooling.md));
type errors in the Vite dev overlay; everything in `specs/later/`.
