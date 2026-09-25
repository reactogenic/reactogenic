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
| Fork upkeep | tsgo is pinned to one commit and rebased on a regular schedule. Our changes live in their own files where possible, and tsgo's own test suite stays green in our CI |
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
  platform, installed as an `optionalDependency` of `reactogenic`.

**Rejected.**

- **TS5 JS API:** its checker and parser would be replaced in the long-term
  compiler, so everything built on them would be built twice.
- **WASM build of the Go transpiler:** slower and larger than a native
  binary. Reconsider later as a fallback for platforms without a binary.
- **One process per transform:** it would lose the warm program, and with
  it every file's type information, on every call.

> OPEN: the message encoding — JSON (simple, easy to debug) or a binary
> format (faster for large source maps). Measure in RGP1-052 before choosing.
