# Reactogenic

A compiled React framework. Phase 1 is `.rtsx` — TSX with slots, flow control,
shorthand props and page segments — running in Vite, type-checked by
TypeScript 7 with errors on the lines you wrote, and supported in the editor
by its own language server. Phase 2, not released yet, is the builder:
`reactogenic build` turns pages in `.rtsx` into plain HTML, CSS and minimal
JS per page, with no React in the output.

- **Use it:** [docs/getting-started.md](docs/getting-started.md)
- **The design:** [specs/phase01/](specs/phase01/) — syntax, the Vite plugin,
  diagnostics, IDE support, decisions, and the plan (`RGP1-xxx`);
  [specs/phase02/](specs/phase02/) — the builder, the components, the
  research behind them, decisions, and the plan (`RGP2-xxx`)
- **What changed:** [CHANGELOG.md](CHANGELOG.md)

| Path | What |
| --- | --- |
| `go/` | the transpiler, the builder (`go/internal/build`) and the `reactogenic` CLI, in Go, on a vendored TypeScript 7 (`go/third_party/tsgo`, patched through `go/patches/`) |
| `packages/core` | `@reactogenic/core` — the runtime: slot types, `Each`, `Switch` / `Match`, and what a component asks the builder (`pathname`, `useShellId`, `mount`) |
| `packages/vite` | `@reactogenic/vite` — the Vite plugin |
| `packages/cli` | `@reactogenic/cli` — the `reactogenic` command (`check`, `build`, `lsp`, `content-mapper`) and platform binaries |
| `packages/ui` | `@reactogenic/ui` — the first components of the design system: `Button`, `Dialog`, `DropdownMenu`, `SideMenu`, their CSS and behaviours. Not published |
| `packages/vscode` | the VS Code extension `reactogenic.rtsx` — grammar, language client, TS server plugin |
| `site/` | the documentation site: four pages in `.rtsx`, built by `reactogenic build` ([site/README.md](site/README.md)) |
| `bench/` | what a page costs the browser: the measuring script, and the baselines the builder is measured against |
| `fixtures/` | conformance cases, run with the examples of the spec |

```sh
pnpm install
pnpm test          # Go and JS
pnpm typecheck
pnpm --filter rtsx test:editor   # the extension in a real VS Code
pnpm --filter @reactogenic/site build   # the docs site → site/dist ($REACTOGENIC_BINARY: site/README.md)
```
