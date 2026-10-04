# Reactogenic (.rtsx)

Language support for [Reactogenic](https://github.com/reactogenic/reactogenic)'s
`.rtsx` files: TSX with slots, flow control, shorthand props and page
segments.

```tsx
<Button size onClick={save}>
  <$Icon { size }><Check size /></$Icon>
  Save
</Button>
```

## What you get

- **Highlighting** — your theme's TSX colours, with slot tags (`<$Icon>`),
  args (`&size`, `&&value`), params (`{ size }`) and segment roots
  (`#about-us`) on top. Also in Markdown ```` ```rtsx ```` fences.
- **Errors as you type** — the same ones as `reactogenic check`, on the line
  you wrote and in slot terms: ``undeclared-slot: `$Nope` is not declared in
  `Card` ``.
- **TypeScript's language features in `.rtsx`** — hover, completion,
  signature help, go to definition, references, rename, auto-import, quick
  fixes, inlay hints, folding, closing tags.
- **Slots and segments** — go to definition on `<$Icon>` finds its
  declaration, on `#about-us` its file; rename a slot from any of its tags.
- **`.rtsx` from `.ts` and `.tsx`** — imports of `.rtsx` modules resolve in
  VS Code's own TypeScript; definitions and references reach into them.

## Requirements

None for highlighting. The language server is the `reactogenic` binary:

1. your project's `@reactogenic/cli` (0.1.0-alpha.1 or later), so the editor
   and `reactogenic check` agree — `pnpm add -D @reactogenic/cli@alpha`;
2. otherwise the binary this extension ships (macOS, Linux and Windows,
   arm64 and x64).

The `{}` item in the status bar, on an `.rtsx` file, says which one runs and
why. In an untrusted workspace only highlighting works.

## Commands

| | |
| --- | --- |
| *Reactogenic: Show Transpiled TSX* | the `.tsx` this file becomes, beside it |
| *Reactogenic: Restart Server* | |
| task *reactogenic: check* | every error of the project, in the Problems panel |

## Settings

| | |
| --- | --- |
| `reactogenic.server.path` | a `reactogenic` binary to run instead of the project's or the bundled one |
| `reactogenic.autoClosingTags` | insert the closing tag when `>` is typed (on) |
| `reactogenic.trace.server` | log the messages between VS Code and the server |

## Known limits

- No formatter yet: Prettier does not know the `.rtsx` forms.
- A rename that cannot be carried through whole — into a string, into a
  `.ts` file with unsaved changes — is refused with the reason.
- With the "TypeScript 7 Nightly" extension and a `contentMappers` entry for
  `.rtsx`, set `"js/ts.contentMappers.enabled": false`: two servers would
  answer in `.rtsx` files.

[Getting started](https://github.com/reactogenic/reactogenic/blob/main/docs/getting-started.md) ·
[the syntax](https://github.com/reactogenic/reactogenic/blob/main/specs/phase01/syntax.md) ·
[issues](https://github.com/reactogenic/reactogenic/issues)
