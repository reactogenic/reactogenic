# The documentation site

Reactogenic's own docs, built by `reactogenic build`
([specs/phase02/builder.md](../specs/phase02/builder.md)): four pages in
`.rtsx`, the components of `@reactogenic/ui`, one plain CSS file. No
`<script>`, no hand-written JS, no list of styles or behaviours anywhere
here — what a page ships is what its components asked the builder for
(plan.md, RGP2-040, T6).

```sh
pnpm install                      # at the repository root
pnpm --filter @reactogenic/site check   # type-check: reactogenic check
pnpm --filter @reactogenic/site build   # reactogenic build → site/dist
pnpm --filter @reactogenic/site test:browser   # the built site in Chromium and WebKit (test/browser.mjs)
```

In this repository the `reactogenic` command is the binary that
`$REACTOGENIC_BINARY` names — build it from `go/cmd/reactogenic`:

```sh
go build -trimpath -o /tmp/reactogenic github.com/reactogenic/reactogenic/go/cmd/reactogenic
REACTOGENIC_BINARY=/tmp/reactogenic pnpm --filter @reactogenic/site build
```

| Path | What |
| --- | --- |
| `pages/**/index.rtsx` | a page; its pathname is the directory: `pages/guide/index.rtsx` → `/guide/` |
| `pages/**/<name>.rtsx` | a segment of the page next to it, mounted by `<section #name />` — which is also the section's `id` |
| `layout.rtsx` | the document from `<html>`: header, side menu, `<main>`, footer |
| `code.rtsx` | `Code`: a sample as plain `<pre><code>`; the text is a template-literal constant of the module that shows it |
| `site.css` | page grid, text, code, tables. Unlayered, so it wins over the design system's layers |
| `public/` | copied to the output as it is |
| `test/browser.mjs` | builds the site as it ships and with `--no-specialize`, and checks both in a browser: the components work, and pruning changes no computed style |
| `dist/` | the output; not in the repository |

| Page | Beyond the layout | Ships |
| --- | --- | --- |
| `/` | an *Install* dialog with its `$Trigger` | `overlays`, `invokers` |
| `/guide/` | — | `overlays` |
| `/syntax/` | an action menu (typeahead) that opens the cheat-sheet dialog | `overlays`, `invokers`, `menu-keys` |
| `/reference/cli/` | — | `overlays` |

Writing a page:

- **Links** are written from the site's root (`/guide/#install`), whatever
  `--base` the site is built with.
- **The side menu** is written once, in `layout.rtsx`. It marks the current
  page itself; a link into a page (`/guide/#install`) is never the current
  page, so each group starts with the page's own link. A new section of a
  page needs its `<$Item>` there — the build checks that a link's page
  exists, not its fragment.
- **Text styles** reach the children of `<main>` and of its sections, not
  every `h2` or `a`: `site.css` is unlayered, and a bare element selector
  would restyle the components' own titles and links.
- **Shell code** runs once, at build time: no handlers, no state, no
  `Date.now()`, no `Intl`. Loops and conditionals over constants are fine.
