# Reactogenic (.rtsx) for VS Code

Language support for Reactogenic's `.rtsx` files — extension id
`reactogenic.rtsx`. Spec: [`specs/phase01/ide.md`](../../specs/phase01/ide.md).

Two halves: the declarative one (RGP1-109) — the language `rtsx`, its
TextMate grammar and its language configuration — and the client (RGP1-110)
that runs `reactogenic lsp --stdio` for the window.

The package is named `rtsx`, without the `@reactogenic` scope every other
package of this repo has: `vsce` rejects scoped names, and the Marketplace id
is `publisher.name`. It is `private` — it goes to the Marketplace and Open
VSX as a `.vsix`, never to npm.

## What it contributes

| | |
| --- | --- |
| language `rtsx` | `.rtsx`; TSX's language configuration |
| language `rtsx-tags` | inside tags: `{/* */}` comments, indent after `<$Slot>` |
| grammar `source.tsx.rtsx` | VS Code's TSX grammar plus the `.rtsx` forms |
| Markdown | ```` ```rtsx ```` fences |
| as for TSX | snippets, breakpoints, Emmet, semantic-token fallback colours |
| commands | *Reactogenic: Restart Server*, *Reactogenic: Show Transpiled TSX* |
| task `reactogenic: check` | the server's binary, `check --pretty=false`; matcher `$reactogenic`, closed documents only |
| settings | `reactogenic.server.path`, `reactogenic.autoClosingTags`, `reactogenic.trace.server` |

Scopes of the `.rtsx` forms; everything else keeps its TSX scope, so themes
apply unchanged:

| Construct | Scope |
| --- | --- |
| slot tag `$Icon` | `entity.name.function.slot.rtsx` |
| `&` / `&&` | `storage.modifier.slot-arg.rtsx` |
| arg name | `entity.other.attribute-name.slot-arg.rtsx` |
| params `{ size }` | `meta.slot-params.rtsx`, TSX's parameter scopes inside |
| segment root `#about-us` | `support.class.component.segment.rtsx` |

## The client

`src/`, bundled by esbuild into one CommonJS file, `dist/extension.js`.

| | |
| --- | --- |
| `resolve.ts` | which binary runs — ide.md's table: the setting, `$REACTOGENIC_BINARY`, the workspace's `@reactogenic/cli` (walking up from the first `.rtsx` document; version ≥ `MIN_CLI_VERSION`), the bundled one. No `vscode` import |
| `server.ts` | the one server of the window, its restarts, the status item |
| `autoInsert.ts` | `>` typed → `textDocument/_vs_onAutoInsert` → the closing tag as a snippet |
| `transpiled.ts` | *Show Transpiled TSX*: `reactogenic/transpiled`, read-only beside the source |
| `task.ts` | the `check` task |
| `protocol.ts` | the requests beyond standard LSP. No `vscode` import |

- An untrusted workspace gets highlighting only: no process is started.
- The server restarts when a lockfile or `reactogenic.server.*` changes, when
  the workspace becomes trusted, and on *Restart Server*.
- The status item (the `{}` in the status bar, on an `.rtsx` document) names
  the binary, its version and where it was found.
- `MIN_CLI_VERSION` in `resolve.ts` is the first `@reactogenic/cli` with
  `lsp`: an older workspace CLI is skipped for the bundled binary, with a
  warning in the status item.

```sh
pnpm build            # dist/extension.js
pnpm typecheck
pnpm test             # unit tests: the grammar, binary resolution, the problem matcher
pnpm test:editor      # the editor suite: opens two VS Code windows
pnpm package          # dist/vsix/rtsx-<target>-<version>.vsix
```

**The editor suite** (`scripts/test-editor.mjs`, `test/editor/`) runs the
extension in a real VS Code with its own profile, against a copy of
`test/fixture`: once trusted, once untrusted. It tests this checkout with
`$REACTOGENIC_BINARY` (unset: built from `go/`), or, with `--vsix file.vsix`,
a package as it ships, with its bundled binary. `$VSCODE_EXECUTABLE` picks the
VS Code; unset, one is downloaded into `.vscode-test/`. On Linux without a
display: `xvfb-run -a pnpm test:editor` (CI's `vscode` job).

**Packaging** (`scripts/package.mjs [--pre-release] [target ...]`): one
`.vsix` per platform of `@reactogenic/cli`, each with the binary from
`dist/npm/cli-<target>/bin` (`scripts/build-binaries.sh`) and the licences of
what is in it — ours, tsgo's `LICENSE` and `NOTICE`, the grammar's and the
bundled npm packages' (`ThirdPartyNotices.txt`) — plus `universal`, without a
binary. Each is staged in `dist/stage/<target>`: what is there is what ships.
Nothing is published.

## Generated files

Everything below is written by `grammar/generate.mjs` from the unmodified
upstream copies in `grammar/upstream/`. Never edit them by hand.

```
syntaxes/rtsx.tmLanguage.json            the grammar
syntaxes/rtsx.markdown.tmLanguage.json   the Markdown fence injection
language-configuration.json              rtsx
tags-language-configuration.json         rtsx-tags
snippets/typescript.code-snippets        TSX's snippets
ThirdPartyNotices.txt                    upstream licences; the .vsix ships it, with the bundle's appended
```

```sh
pnpm generate         # write them
pnpm generate:check   # fail if any is out of date (CI)
pnpm test             # scopes, equality with source.tsx, the repo's .rtsx files, …
```

The grammar is VS Code's TSX grammar with three patches: the attribute list
(args, segment roots, params), the tag-name pattern (`$name`), the root scope.
The generator asserts the shape of every upstream rule it patches.

## Updating upstream

1. Copy the files listed in `grammar/upstream/UPSTREAM` from a newer VS Code
   and update that note (version, commits, dates, checksum).
2. `pnpm generate`. An assertion that fails names the upstream rule that
   changed: re-read it and adjust the patch.
3. `pnpm test`. The `manifest` tests compare `package.json` with VS Code's own
   TSX entry (`unbalancedBracketScopes`, `tokenTypes`, `semanticTokenScopes`,
   snippets).

## Known limits

- A regex cannot look at the next line: for a `{` that ends its line, the
  first thing inside that is not a comment decides between params and a
  spread.
- `<$Icon{ size }>` — params directly after the tag name, no space — is not
  read as a tag with attributes (TSX has the same limit for `<B{...p}>`).
- Half-typed forms that TSX's grammar does not recover from either: `x=`
  directly before `>`, an unclosed `{`.
- One intended difference from `source.tsx` on valid TSX: an element as an
  attribute value (`footer=<b>…</b>`) is tokenized, not marked illegal.
