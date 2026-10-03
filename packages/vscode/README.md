# Reactogenic (.rtsx) for VS Code

Language support for Reactogenic's `.rtsx` files — extension id
`reactogenic.rtsx`. Spec: [`specs/phase01/ide.md`](../../specs/phase01/ide.md).

This package holds the declarative half (RGP1-109): the language `rtsx`, its
TextMate grammar and its language configuration. The client that runs
`reactogenic lsp` is RGP1-110.

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

Scopes of the `.rtsx` forms; everything else keeps its TSX scope, so themes
apply unchanged:

| Construct | Scope |
| --- | --- |
| slot tag `$Icon` | `entity.name.function.slot.rtsx` |
| `&` / `&&` | `storage.modifier.slot-arg.rtsx` |
| arg name | `entity.other.attribute-name.slot-arg.rtsx` |
| params `{ size }` | `meta.slot-params.rtsx`, TSX's parameter scopes inside |
| segment root `#about-us` | `support.class.component.segment.rtsx` |

## Generated files

Everything below is written by `grammar/generate.mjs` from the unmodified
upstream copies in `grammar/upstream/`. Never edit them by hand.

```
syntaxes/rtsx.tmLanguage.json            the grammar
syntaxes/rtsx.markdown.tmLanguage.json   the Markdown fence injection
language-configuration.json              rtsx
tags-language-configuration.json         rtsx-tags
snippets/typescript.code-snippets        TSX's snippets
ThirdPartyNotices.txt                    upstream licences; ships in the .vsix
```

```sh
pnpm generate         # write them
pnpm generate:check   # fail if any is out of date (CI)
pnpm test             # scopes, equality with source.tsx, the repo's .rtsx files
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
