# Reactogenic

A compiled React framework. Phase 1 is `.rtsx` — TSX with slots, flow control,
shorthand props and page segments — running in Vite, type-checked by
TypeScript 7 with errors on the lines you wrote, and supported in the editor
by its own language server.

- **Use it:** [docs/getting-started.md](docs/getting-started.md)
- **The design:** [specs/phase01/](specs/phase01/) — syntax, the Vite plugin,
  diagnostics, IDE support, decisions, and the plan (`RGP1-xxx`)
- **What changed:** [CHANGELOG.md](CHANGELOG.md)

| Path | What |
| --- | --- |
| `go/` | the transpiler and `reactogenic` CLI, in Go, on a vendored TypeScript 7 (`go/third_party/tsgo`, patched through `go/patches/`) |
| `packages/core` | `@reactogenic/core` — the runtime: slot types, `Each`, `Switch` / `Match` |
| `packages/vite` | `@reactogenic/vite` — the Vite plugin |
| `packages/cli` | `@reactogenic/cli` — the `reactogenic` command (`check`, `lsp`, `content-mapper`) and platform binaries |
| `packages/vscode` | the VS Code extension `reactogenic.rtsx` — grammar, language client, TS server plugin |
| `fixtures/` | conformance cases, run with the examples of the spec |

```sh
pnpm install
pnpm test          # Go and JS
pnpm typecheck
pnpm --filter rtsx test:editor   # the extension in a real VS Code
```
