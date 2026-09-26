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
| M1 | Runtime package | 010–019 | `reactogenic` exports usable from plain `.tsx` |
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

### RGP1-004 — Fixture harness · M
The shared format for every conformance test from here on.
- A fixture is `name.rtsx` plus some of these:
  - `name.tsx`, the expected output;
  - `name.errors`, expected transpiler errors (code and `.rtsx` span);
  - `name.check`, expected `reactogenic check` output.
- Multi-file fixtures are a directory: segments, containers declared in other
  files.
- An extractor turns every `// .rtsx` / `// .tsx` code-block pair in
  syntax.md into a fixture. The spec's examples then *are* the tests, and a
  spec edit that breaks one fails CI.
- Output is compared after normalising the formatting, so tests assert
  meaning rather than whitespace.
- **Done when:** the extractor finds every pair in syntax.md, and each one
  runs (red is expected until M3).
- Depends on: 003.

### RGP1-005 — Close the phase 1 OPENs in syntax.md · S
Decide or defer each one, so implementation does not stall on them:
- the migration warning for the silent flip;
- the `Slot<Children, Options>` helper;
- iterables in `Each`.
- **Done when:** each OPEN is decided or marked ROADMAP.

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

### RGP1-020 — Slot params · M
`JsxSlotParams ::= ObjectBindingPattern`, for `{` not followed by `...`.
- Every row of the grammar table must parse: renaming, defaults, nesting,
  rest, `{}`. `{ ...rest }` on its own stays a spread attribute.
- **Done when:** the grammar table in syntax.md passes as parser tests with
  exact spans.
- Depends on: 003.

### RGP1-021 — Segment roots `#name` · S
`JsxSegmentRoot ::= '#' JsxIdentifier`.
- tsgo already parses `#about-us` as a `JsxAttribute` named `"#about-us"`.
  In `.rtsx`, the parser turns it into a `JsxSegmentRoot`; the scanner is
  untouched ([decisions.md](decisions.md#rgp1-002--parser-strategy)).
- No whitespace after `#`, hyphens allowed, at most one per element.
- **Done when:** parser tests pass, including `#name` next to spreads and
  other attributes.
- Depends on: 003.

### RGP1-022 — Parse-level errors · S
- params-on-html, duplicate-params, and a second `#name` on one element.
- Error recovery: one bad attribute must not hide the errors in the rest of
  the file.
- **Done when:** each error has a fixture with its span.
- Depends on: 020, 021.

## M3 — Transpiler

### RGP1-030 — Pipeline, emitter and origin-tracking source map · L
The base everything else builds on. Split it into these subtasks:
- **030a — pass pipeline.** Passes 0–4 as separate transforms over one AST,
  each seeing the previous one's output
  ([syntax.md](syntax.md#compilation-passes)).
- **030b — emitter.** Edits the source text rather than reprinting it, so
  copied code keeps its exact bytes and spans.
- **030c — source map with origins.** Every emitted span is either *copied*
  (it has a source span) or *synthesized* (it has an origin construct), as in
  [diagnostics.md](diagnostics.md#mapping). This is a standard v3 map, which
  Vite needs, plus an origin side table, which M6 needs.
- **Done when:** a file with no `.rtsx` extensions passes through byte for
  byte with an identity map.
- Depends on: 020, 021.

### RGP1-031 — Import-origin recognition · S
`Switch`, `Match` and `Each` are recognised by the module they are imported
from, including aliases (`import { Switch as Choose }`).
- flow-as-value: `const S = Switch`, `as={Match}`, `createElement(Switch, …)`,
  a re-export.
- **Done when:** the alias and flow-as-value fixtures pass.

### RGP1-032 — Pass 1: shorthand props · M
- Lookup follows the ES module's scope chain: imports and top-level
  declarations, enclosing functions and blocks, and slot params.
- Excluded: globals, ambient declarations, type-only bindings.
- Always case B: reserved words and non-identifiers (`aria-label`).
- Intrinsic elements and slot elements follow the same rule; spread order is
  preserved.
- **Done when:** every example and edge case in *Shorthand props* is a
  passing fixture, including TDZ and shadowing.
- Depends on: 030.

### RGP1-033 — Pass 2: `Match` · S
- Lower to a ternary, never `&&`.
- Params form: an immediately-invoked arrow, as in the spec.
- Position: `{…}` as a JSX child, `(…)` in expression position.
- **Done when:** the fixtures in *Desugaring: `Match`* and *Position* pass.
- Depends on: 030, 031.

### RGP1-034 — Pass 2: `Switch` · M
Four shapes:
- static;
- a reference subject, repeated with no wrapper so TS can narrow it;
- `exhaustive`, ending in `noMatch` under a generated name;
- dynamic (params).

Plus: `$Case key` becomes a keyed `Fragment`, bodies follow the slot-body
rule, and the `Switch` / `Match` import is dropped.
- **Done when:** every desugaring fixture in *Flow control* passes, and every
  *syntax* error in its *Compile errors* table has a fixture.
- Depends on: 033.

### RGP1-035 — Pass 3: slot hoisting (object form) · M
- Remove slot elements from `children` and append them as attributes, in
  order.
- Property mapping follows the table in *Usage and desugaring*.
- `children` is chosen syntactically: params + body, body only, or none.
- Body: the single child expression, or `<>…</>`.
- Errors: orphan-slot; duplicate-slot (slot element plus an explicit
  attribute); slot-key; slot-children-conflict.
- **Done when:** the *Slots* fixtures pass, apart from list and conditional
  slots.
- Depends on: 030, 032.

### RGP1-036 — Params on a component · S
When a component element has params, what is left of its children becomes
its `children` callback. This happens after slot hoisting, and the params
are not in scope in the slot elements.
- **Done when:** the `Each` desugaring fixture passes through the general
  rule, with nothing specific to `Each`.
- Depends on: 035.

### RGP1-037 — Conditional slots · S
- A ternary whose branches are slot elements of one slot, or `null`, becomes
  a ternary prop; `null` becomes `undefined`.
- Errors: mixed-conditional-slot, orphan-slot (for `&&`, and for
  `Match` / `Switch` with params).
- **Done when:** the three-stage example in *Conditional slots* passes at
  each stage.
- Depends on: 034, 035.

### RGP1-038 — List slots (emit) · M
- Collect same-named slot elements in source order into one array attribute,
  at the position of the first.
- A conditional item is spread in: `...(c ? [{…}] : [])`.
- Errors: duplicate-slot (types case).
- The type query is an injected interface, `isListSlot(container, name)`, so
  this task is tested with a stub before M4 exists.
- **Done when:** the *List slots* fixtures pass with the stub.
- Depends on: 035, 037.

### RGP1-039 — Pass 4: segment roots · M
- `#name` becomes `id="name"` in place, plus an extensionless import of the
  segment, plus its default export nested inside the root.
- Generated identifier: `_<Tag>_<camelName>`.
- Errors on the source: segment-id, segment-children (warning),
  segment-duplicate (per file), segment-in-loop, segment-import.
- **Done when:** the *Segment roots* fixtures pass.
- Depends on: 030.

### RGP1-040 — Checks needing the file system · S
- segment-not-found (a sibling `+name.rtsx` or `+name.tsx`);
- segment-self (a cycle through segments);
- `Foo.tsx` and `Foo.rtsx` side by side ([vite.md](vite.md#module-resolution)).
- This goes behind a file-system interface, so the Vite plugin and the CLI
  supply their own.
- **Done when:** the multi-file fixtures pass.
- Depends on: 039.

### RGP1-041 — `Each` key check · S
each-no-key: the body of an `Each` (recognised by import origin) must be a
single element with `key`.
- **Done when:** the *Iteration* fixtures pass.
- Depends on: 031, 036.

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
  program, and reached from npm through a `bin` shim added to the
  `reactogenic` package here.
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
  `reactogenic` (the esbuild model).
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
