# Phase 1 decisions

One section per decision, newest last. A decision changes only by a new
section that supersedes it.

## RGP1-001 — Checker and implementation language

**Decision.** The transpiler is written in Go from day 1, as a fork of
TypeScript 7 (tsgo). TS7's parser, binder and checker run **in-process** with
the transpiler. Phase 1 is the first slice of the long-term compiler, not a
prototype to be rewritten later.

**Why.**

- The architecture already fixes a Go compiler forked from tsgo. A TS5
  prototype would be thrown away, and the fixtures would be the only thing
  carried over.
- In the fork, the transform, the list-slot query and `reactogenic check` all
  use the same parser and checker. What is checked is what runs, with no
  second implementation to keep in step.
- The fork can reach tsgo's internal packages directly. Phase 1 needs no
  stable public TS7 API.

**Consequences.**

| Area | Consequence |
| --- | --- |
| Parser (RGP1-002) | Extending tsgo's own JSX parser becomes the natural option. Masking the reserved forms before parsing, or using Babel, would mean a second parser next to the one the checker uses. Still decided in RGP1-002 |
| Type query (RGP1-051) | A call into tsgo's checker from Go: the props type of the tag, then `$X` in it, minus `undefined` / `null`, then array or tuple |
| Program (RGP1-050) | tsgo's compiler host with an in-memory overlay: each `Foo.rtsx` is served as `Foo.tsx` |
| `reactogenic check` (M6) | The Go binary itself, installed through an npm `bin` shim |
| Vite plugin (M5) | The plugin runs in Node, so it drives a Go process (*Node ↔ Go* below) |
| Semantics | The project is type-checked with TS7 semantics, including its `.ts` / `.tsx` files. Differences from the user's `tsc` 5.x are TS7's differences, not ours |
| Fork upkeep | tsgo is pinned to one commit and rebased on a regular schedule. Our changes live in their own files where possible (see RGP1-003 for the mechanics) |
| License | tsgo is Apache-2.0: keep its LICENSE and NOTICE, and mark files we modify |

**Node ↔ Go.**

- The plugin spawns **one long-lived Go process** per Vite server or build,
  so the program stays warm between transforms (RGP1-052).
- It talks to the process over stdio with framed request/response messages:

| Message | Direction | Payload |
| --- | --- | --- |
| `open` | Node → Go | tsconfig path |
| `transform` | Node → Go | file path, contents → `.tsx`, v3 source map, transpiler errors |
| `change` | Node → Go | file path, contents or deleted → the `.rtsx` files to re-emit (RGP1-052) |
| `close` | Node → Go | — |

- The binary is shipped the way esbuild ships its own: one npm package per
  platform, installed as an `optionalDependency` of `@reactogenic/cli`
  (see RGP1-005 for the package names).

**Rejected.**

- **TS5 JS API:** its checker and parser would be replaced in the long-term
  compiler, so everything built on them would be built twice.
- **WASM build of the Go transpiler:** slower and larger than a native
  binary. Reconsider later as a fallback for platforms without a binary.
- **One process per transform:** it would lose the warm program, and with
  it every file's type information, on every call.

> OPEN: the message encoding — JSON (simple, easy to debug) or a binary
> format (faster for large source maps). Measure in RGP1-052 before choosing.

## RGP1-002 — Parser strategy

**Decision.** Extend tsgo's own JSX parser in the fork. `.rtsx` gets its own
script kind: the extensions are parsed **only** in `.rtsx` files, so
`{ size }` and `#name` stay syntax errors in `.tsx`.

**What changes in the fork.**

| Package | Change |
| --- | --- |
| script kinds | `.rtsx` → a new script kind: TSX plus the extensions |
| AST | two node kinds: `JsxSlotParams` (wraps an `ObjectBindingPattern`) and `JsxSegmentRoot` (`#` + `JsxIdentifier`), both allowed in `JsxAttributes` |
| parser | in attribute position, `{` not followed by `...` → `JsxSlotParams`; an attribute name starting with `#` → `JsxSegmentRoot`; the parse-level errors of RGP1-022 |

No scanner change is needed. Found while scaffolding (RGP1-003): tsgo's
scanner already reads `#about-us` in attribute position as one JSX
identifier, and its parser builds a `JsxAttribute` named `"#about-us"` with
no diagnostic, as TypeScript 5.x does. In `.rtsx` that attribute becomes a
`JsxSegmentRoot`; in `.tsx` nothing changes. `{ size }` is a parse error in
both, as expected.

**The checker never sees the new nodes.** The passes lower them away, and
the checker type-checks the emitted `.tsx`, which is parsed again as plain
TSX (RGP1-050). So the fork changes the parser and the AST, not the checker
or the emitter.

**Why.**

- One parser. The spans the passes and the diagnostics use come straight
  from it, with no offset map in between.
- The AST is tsgo's own, so the passes work on the same nodes the checker
  uses.
- A small surface: two node kinds and two branches in attribute parsing, all
  in the JSX part of the parser.

**Consequences.**

- Rebasing on tsgo means keeping these parser changes applied. The changes
  live in separate files, called from as few places in tsgo's code as
  possible.
- The parser tests for RGP1-020–022 are tsgo-style tests in the fork, next to
  tsgo's own.
- Scope analysis for shorthand props (RGP1-032) needs slot params declared
  as bindings over their body. That is either a binder change in the fork or
  a resolver of our own over the `.rtsx` AST; RGP1-032 decides which.

**Rejected.**

- **(b) Mask the reserved forms, then parse with tsgo unmodified:** it adds an
  offset map in front of the source map. Every masked form has to stay valid
  and unambiguous in every context, and tsgo's parse errors would point at
  the mask, not at what the author wrote.
- **(c) Babel:** a second parser next to tsgo's, with its own AST and its
  own spans.

## RGP1-003 — Repository layout and vendoring

**Upstream has moved.** `microsoft/typescript-go` is archived; the Go port now
lives in [microsoft/TypeScript](https://github.com/microsoft/TypeScript) as
the `tsc/` module, `github.com/microsoft/TypeScript/tsc`. "tsgo" in these
specs means that module.

**Layout.** Go and TypeScript are split at the root; the root only ties them
together.

```
/
├── go.work                 use ./go ./go/third_party/tsgo
├── package.json            private; scripts run both sides
├── pnpm-workspace.yaml     packages/*
├── go/
│   ├── go.mod              github.com/reactogenic/reactogenic/go
│   ├── cmd/reactogenic/    `check` and the stdio server
│   ├── internal/           the transpiler
│   ├── third_party/tsgo/   vendored tsc/ module, patched
│   ├── patches/            our changes to tsgo, as patches
│   └── scripts/            sync-tsgo.sh
├── packages/
│   ├── core/               @reactogenic/core — the runtime
│   └── vite/               @reactogenic/vite — the plugin
└── specs/
```

- The closest model is microsoft/TypeScript itself: `go.work` at the root,
  the Go module in `tsc/`, npm packages in `packages/`. The Rust + pnpm
  projects we looked at (Biome, Turborepo, oxc, Rolldown) keep both
  manifests at the root instead.
- A root `go.work` lets `go` and gopls work from the repo root.
  `go/go.mod` also `replace`s tsgo with `./third_party/tsgo`, so `go/` builds
  on its own too.
- Go commands name our module explicitly
  (`go test github.com/reactogenic/reactogenic/go/...`): `./go/...` would
  include the vendored module.

**Vendoring.**

- `go/scripts/sync-tsgo.sh <commit>` copies `tsc/` at that commit into
  `go/third_party/tsgo`, records it in `UPSTREAM`, and applies
  `go/patches/*.patch` in order. The patched tree is committed.
- Not vendored: `*_test.go`, `testdata/`, and the test-only packages
  (`fourslash`, `testrunner`, `testutil`, `execute/tsctests`). That brings
  the tree from 393 MB to 23 MB. Upstream's CI runs upstream's tests; our
  changes are covered by our own.
- CI re-vendors and fails if the committed tree differs from upstream plus
  patches, so no change to tsgo bypasses `go/patches/`.
- LICENSE and NOTICE are kept (Apache-2.0).

**Reaching `internal/`.** Go allows tsgo's `internal/` packages to be
imported only from inside tsgo's module. Patch `0001-rtsx-bridge.patch` adds
a public package `rtsx` inside the vendored module that re-exports what the
transpiler needs: type aliases and one-line wrappers.

**Rejected.**

- **Git submodule** (tsgolint, rslint): a clone step before every build, and
  patches that exist only in a working tree.
- **Generated shim modules under Microsoft's module path plus
  `//go:linkname`** (tsgolint, rslint): a code generator to maintain, and
  linkname breaks silently across rebases. We patch tsgo anyway, so a
  hand-written bridge in the patch set costs nothing extra.
- **Rewriting tsgo's import paths into our module:** every upstream sync
  would touch every file.
- **`go/` + `ts/`:** no project we looked at splits the JS side that way;
  `packages/` is the convention.

**Toolchain.** tsgo requires Go 1.27. `go.mod` says so, and Go 1.21+
downloads that toolchain on demand (`GOTOOLCHAIN=auto`, the default).

## RGP1-005 — Phase 1 open questions

| Question | Decision | Recorded in |
| --- | --- | --- |
| Which tags are components | React's rule, unchanged; only tags starting with `$` are taken (slots). No invented naming constraint | syntax.md, *Slots → Grammar* |
| Warning on the silent flip when migrating | deferred to editor tooling | syntax.md, *Shorthand props* |
| `Slot<Children, Options>` helper | deferred | syntax.md, *Slots → Typing behaviour* |
| Iterables in `Each` | rejected: arrays only, so `index` is always a `number` position | syntax.md, *Iteration* |
| Scope for shorthand props | tsgo's binder, no resolver of our own; a same-named binding of the wrong type is an ordinary TS error | syntax.md, *Shorthand props*; RGP1-032 |
| Package names | all scoped to the `@reactogenic` npm org: `core`, `vite`, `cli`, `cli-<os>-<arch>` | vite.md, *Runtime* |

## RGP1-020 — No new node kinds, no new script kind

Supersedes two rows of RGP1-002: *script kinds* and *AST*.

**Decision.** The `.rtsx` forms are stored in node kinds tsgo already has,
and `.rtsx` is recognised by its file extension:

| Form | Stored as | Why it cannot be confused with TSX |
| --- | --- | --- |
| slot params `{ size }` | `JsxSpreadAttribute` whose expression is an `ObjectBindingPattern` | the TSX grammar only ever puts an expression there |
| segment root `#about-us` | `JsxAttribute` named `"#about-us"`, no value — what TypeScript's parser already builds | only `.rtsx` gives it a meaning |
| `.rtsx` | TSX script kind; the parser enables slot params when the file name ends in `.rtsx` | TypeScript itself derives script kinds from extensions |

`go/internal/syntax` is the one place that reads these shapes
(`SlotParams`, `SegmentRoot`).

**Why.**

- New kinds mean editing tsgo's generated files (`kind_generated.go`, its
  stringer, `ast_generated.go`). Their generator is not vendored, and the
  files change with most upstream commits, so every rebase would conflict.
- A new script kind means changing `core.ScriptKind` and every switch over
  it.
- The checker never sees `.rtsx` trees, so reusing kinds cannot confuse it.
  The transpiler lowers every form before TS7 type-checks the output.

**Result.** Patch `0002-rtsx-parser.patch`: one new file,
`internal/parser/rtsx.go`, and three lines in `parseJsxAttribute`.

## RGP1-043 — The slot model: singular slots, attachments, args

Supersedes the slot parts of RGP1-005 and RGP1-042 (syntax.md, *Slots*).

**Decision.** A slot is one prop value, `Slot<P>` (P is the complete prop
contract) or `FnSlot<P, A>`. `<$X>` constructs it; repeating it replaces it
(last assignment wins). The container attaches it with `<El slot={$X} …>`:
the attachment's props are defaults the slot's props replace, per prop; its
children are the fallback; `&name` is an arg of a function slot's body,
`&&name` an arg and a prop. Slots nest (a slot element inside a slot element
fills that slot's slots). An attachment may run many times — inside an `Each`
— while the slot stays one value.

**Why.** Lists of slots (`Slot<P>[]`) could not say how static items and
data-driven ones mix, nor which props come from the container and which from
the caller. A singular slot attached inside the container's own loop answers
both: repetition belongs to the attachment, and `&` / `&&` say which values
are args and which are props.

**Consequences.**

- **The transpiler is purely syntactic.** List slots were its only
  type-directed step. Gone with them: `Input.ListSlot`, the checker's
  `IsListSlot`, the two-round program build (RGP1-051), `list-slots.txt`.
  The Vite transform needs no checker (vite.md).
- **Args are typed by `renderSlot`**, not by the transpiler: a function slot
  rendered without its args, a plain slot rendered with any, and a slot typed
  as an array are TS errors at the attachment.
- The parser patch (0002) adds `&name` / `&&name`, stored as a `JsxAttribute`
  named `"&name"` — no new node kind, as for `#name`.
- `Each` no longer checks keys; where a missing key is reported for slots is
  open (syntax.md, OPEN #4).
- Open: last-wins with conditional assignments (OPEN #7, implemented
  provisionally in source order); reachability of attachments (tech debt).

**`NOT_ASSIGNED`** (#7). The false branch of a conditional `<$X>` with no
earlier assignment is the sentinel `NOT_ASSIGNED`, never `undefined`: with
per-prop replacement an `undefined` replaces a default, the sentinel never
does. Attachments test `isAssigned($X)` and spread `slotProps($X)`, which
drops sentinel entries (a nested slot that was not assigned keeps the
attachment's default). `Slot` / `FnSlot` include `NotAssigned`; open whether a
required slot should exclude it (syntax.md).

**Keyed slots** (#4). Many values of one kind are a `KeyedSlot<P>` /
`KeyedSlot<P, A>`, keyed by React's `key` at the call site
(`<$Column key="email" />`) and selected by the attachment's `key`
(`<th key={col.name} slot={$Column} />`). `key` keeps the transpiler
syntactic (a prop chosen as the key would need types on both sides) and
answers #4: the key of a slot call is the React key of what it renders. The
value carries the `KEYED` brand in its object literal — entries stay
type-checked as slot values — and `slotEntry` returns the entry of a keyed
slot, or a singular slot as it is. `FnSlot<P, A>` is folded into `Slot<P, A>`.

**Required slots and `NOT_ASSIGNED`.** `Slot<P>` keeps one type for required
and optional slots; no separate type keeps the sentinel out of required ones.
A required slot filled only conditionally is an error — "Required slot without
fallback cannot be conditional" (slot-conditional) — unless the component
attaches it with a fallback. `reactogenic check` joins the caller's
conditional (a transpiler note with the component's tag) with the component's
attachments (notes with or without fallback) through the checker.

**Key functions.** A singular `Slot<P, A>` attached per item is keyed by the
caller: `<$Option key={({ value }) => value} />`, a function of the args. It
is told from an entry key by syntax — an inline arrow or function expression;
`key={getKey}` is an entry key and is reported as slot-key-inline. Requiring
the inline form is intended: a reference would be ambiguous. The slot's key
replaces the attachment's `key`, as slot props replace attachment props. The
function is held under `SLOT_KEY`, since `P` already has React's `key`. An
attachment with args binds them once through `slotArgs`, which types them as
`renderSlot` does, so arg errors still point at the arg.

**Segment files by name.** `#about-us` mounts `about-us` — the first of
`.rtsx`, `.tsx`, `.jsx`, `.ts`, `.js` next to the file. The `+` prefix is
dropped: a segment is an ordinary module, made a segment by being mounted, so
segment-import goes with it. The emitted import names the file found,
extension included, so Vite and TS7 load the file the lookup chose rather than
resolving by their own orders (TS: `.ts` before `.tsx`). `reactogenic check`
resolves `./x.rtsx` through an `x.rtsx.tsx` alias of the virtual `x.tsx`, and
drops TS5097 on segment imports.
