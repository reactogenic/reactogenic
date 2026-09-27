# Reactogenic

A compiled React framework. Phase 1 is `.rtsx` — TSX with slots, flow control,
shorthand props and page segments — running in Vite and type-checked by
TypeScript 7 with errors on the lines you wrote.

- **Use it:** [docs/getting-started.md](docs/getting-started.md)
- **The design:** [specs/phase01/](specs/phase01/) — syntax, the Vite plugin,
  diagnostics, decisions, and the plan (`RGP1-xxx`)

| Path | What |
| --- | --- |
| `go/` | the transpiler and `reactogenic` CLI, in Go, on a vendored TypeScript 7 (`go/third_party/tsgo`, patched through `go/patches/`) |
| `packages/core` | `@reactogenic/core` — the runtime: slot types, `Each`, `Switch` / `Match` |
| `packages/vite` | `@reactogenic/vite` — the Vite plugin |
| `packages/cli` | `@reactogenic/cli` — the `reactogenic` command and platform binaries |
| `fixtures/` | conformance cases, run with the examples of the spec |

```sh
pnpm install
pnpm test          # Go and JS
pnpm typecheck
```
