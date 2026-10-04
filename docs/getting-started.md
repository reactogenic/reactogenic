# Getting started with `.rtsx`

`.rtsx` is TSX with a few additions for components that have slots, flow
control and page segments. Phase 1 runs it in an ordinary Vite + React app and
type-checks it with TypeScript 7, reporting every error on the `.rtsx` line you
wrote. Phase 2 adds a builder that turns pages into plain HTML, CSS and
minimal JS, with no React in the output — [Build](#build), not released yet.

## Install

```sh
pnpm add @reactogenic/core@alpha
pnpm add -D @reactogenic/vite@alpha @reactogenic/cli@alpha
```

Reactogenic is in alpha: the packages are published under the `alpha` tag.

`@reactogenic/cli` brings the `reactogenic` binary for your platform (macOS,
Linux and Windows, arm64 and x64).

## Configure Vite

```ts
// vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import reactogenic from "@reactogenic/vite";

export default defineConfig({
  plugins: [reactogenic(), react()],
});
```

`.tsx` and `.rtsx` live side by side and import each other without
extensions (`import { Page } from "./page"` finds `page.rtsx`). Don't keep a
`page.tsx` and a `page.rtsx` next to each other: the import would be ambiguous.

Editing an `.rtsx` file reloads the page; Fast Refresh for `.rtsx` comes with
Reactogenic's own dev server.

## Type-check

Vite does not type-check. `reactogenic check` does, for `.ts`, `.tsx` and
`.rtsx`:

```json
{
  "scripts": {
    "check": "reactogenic check",
    "build": "reactogenic check && vite build"
  }
}
```

```
src/page.rtsx:6:26 - error slot-type: `$Field` does not match its declaration in `Card`: Type 'number' is not assignable to type 'boolean | undefined'.

6     <$Field name="email" required={1} />
                           ^
```

An error about a slot or a segment is reported in its terms, as this one is
— also ``undeclared-slot: `$Nope` is not declared in `Card` ``,
``slot-args-missing: `$Icon` needs `&size` ``. Any other error is
TypeScript's own, with its code: the same `1` given to a plain prop is
`error TS2322`. Use `reactogenic check --watch` while you work, and
`--pretty=false` for `file(line,col)` output in CI.

## Editor

**VS Code** (and editors built on it): install *Reactogenic (.rtsx)* —
`reactogenic.rtsx` on the Marketplace and on Open VSX. It brings

- highlighting for `.rtsx`, with your theme's TSX colours and the slot,
  arg, param and segment forms on top;
- the language server: errors as you type — the same ones as `reactogenic
  check`, in slot terms — hover, completion, go to definition, references,
  rename, auto-import, quick fixes, closing tags;
- `.rtsx` modules in your `.ts` and `.tsx` files: imports resolve, and
  definitions and references reach into `.rtsx`.

The server is the `reactogenic` binary of your project's `@reactogenic/cli`
(0.1.0-alpha.1 or later), so the editor and `reactogenic check` always agree;
without one, the extension runs the binary it ships. The `{}` item in the
status bar says which one runs. *Reactogenic: Show Transpiled TSX* opens the
`.tsx` a file becomes, beside it.

| What | Where |
| --- | --- |
| a slot, from its tag | go to definition on `<$Icon>` → the `$Icon` declaration; rename renames every tag and the declaration |
| a segment, from its root | go to definition on `#about-us` → `about-us.rtsx` |
| every project error | the task *reactogenic: check* (Terminal → Run Task) |

Rename is whole or refused: where a rename cannot be carried through — a
name inside a string, a `.ts` file with unsaved changes — the server says
why and changes nothing.

**Other editors**: `reactogenic lsp --stdio` is a standard language server —
point Neovim, Zed, Helix or a JetBrains IDE at it for `*.rtsx`.

**Without our server**, from TypeScript 7.1: `@reactogenic/cli` is also a
TypeScript *content mapper*, so plain `tsc` and the TS 7.1 language server
can read `.rtsx` — experimental, with TypeScript's own messages:

```jsonc
// tsconfig.json
"contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]
```

## The syntax in five minutes

**Shorthand props.** A bare attribute passes the variable of the same name
when there is one in scope; otherwise it means `true`, as in TSX.

```tsx
const value = "hello";
<Input value />        // value={value}
<button disabled />    // disabled={true}: no `disabled` in scope
```

**Slots.** A component declares slots as `$`-props; the caller fills them with
`$` elements; the component attaches each one to the element it stands for.

```tsx
// Button.rtsx
import type { Slot } from "@reactogenic/core";

interface ButtonProps {
  $Label?: Slot<{ className?: string; children?: ReactNode }>;
  $Icon?: Slot<ComponentProps<"span">, { size: Size }>;
  size: Size;
}

function Button({ $Label, $Icon, size }: ButtonProps) {
  return (
    <button>
      <span slot={$Icon} &size />
      <b slot={$Label} className="label">Button</b>
    </button>
  );
}
```

```tsx
// the caller
<Button size="lg">
  <$Icon { size }><Plus size /></$Icon>
  <$Label>Save</$Label>
</Button>
```

- A slot's props replace the attachment's, prop by prop; the attachment's
  children are the fallback (`Button` above when `$Label` is not given).
- `&size` hands `size` to a function slot's body, which receives it as
  `{ size }`; `&&value={x}` also sets `value` on the element.
- A slot is one value: writing `<$Label>` twice keeps the last one. To render
  something once per item, attach the slot inside an `Each` in the component.
- Many values of one kind — table columns, form fields — are a `KeyedSlot`:
  the caller writes `<$Column key="email" …/>` per entry, and the component's
  `<th key={col.name} slot={$Column} />` renders the entry of each key.
- A function slot the component attaches once per item is keyed by you when
  only you know the items' identity: `<$Row key={({ row }) => row.id} { row }>`
  — an inline function of the args.

**Flow control.**

```tsx
import { Each, Match, Switch } from "@reactogenic/core";

<Match on={user}>
  <Avatar src={user.avatarUrl} />
</Match>

<Switch on={query.status} exhaustive>
  <$Case is="pending"><Spinner /></$Case>
  <$Case is="error"><Oops /></$Case>
  <$Case is="success"><List /></$Case>
</Switch>

<Each items={users} { item: user }>
  <li key={user.id}>{user.name}</li>
</Each>
```

`exhaustive` makes a missing case a type error: `Missing "success"`.

**Segments.** Split a long page into files next to it, and mount them by name:

```tsx
<section #pricing />   // mounts the default export of ./pricing.rtsx
```

## Moving a `.tsx` file to `.rtsx`

Renaming is the whole migration, with one thing to look for: a bare boolean
attribute that has a variable of the same name in scope changes meaning.

```tsx
const disabled = false;
<Button disabled />    // TSX: disabled={true}. .rtsx: disabled={disabled}
```

## Build

`reactogenic build` is the other way to run `.rtsx`: without Vite, and
without React in the browser. It executes every page once, at build time, and
writes it as plain HTML, with the CSS that page can use and the JavaScript of
the behaviours its components asked for.

> **Not released yet.** The builder is phase 2 and lives in the repository:
> the published `@reactogenic/cli` (0.1.0-alpha.1) has no `build` command,
> and `@reactogenic/ui` — the components — is not on npm. To try it, work in
> a checkout: [site/](../site/) is this documentation as a site, built this
> way.

```sh
pnpm install
go build -o /tmp/reactogenic github.com/reactogenic/reactogenic/go/cmd/reactogenic
cd site && /tmp/reactogenic build --report      # → site/dist
```

**Pages.** `index.rtsx` of a directory under `pages/`, next to the tsconfig,
is a page; the directory is its pathname.

```
tsconfig.json                    "include": ["layout.rtsx", "pages"]
layout.rtsx                      an ordinary module
site.css
pages/index.rtsx                 /
pages/guide/index.rtsx           /guide/
pages/guide/install.rtsx         not a page: a segment or a module of /guide/
public/favicon.svg               /favicon.svg: copied as it is
```

A page is the module's default export: a component without props that
renders the whole document, from `<html>`. There is no layout file to name:
**the layout is an ordinary component**, imported and rendered as in React.

```tsx
// layout.rtsx
import type { ReactNode } from "react";
import "./site.css";

export function Layout({ title, children }: { title: string; children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <title>{title}</title>
      </head>
      <body>
        <nav>
          <a href="/">Home</a> <a href="/guide/">Guide</a>
        </nav>
        <main>{children}</main>
      </body>
    </html>
  );
}
```

```tsx
// pages/index.rtsx
import { Dialog } from "@reactogenic/ui";
import { Layout } from "../layout";

export default function Page() {
  return (
    <Layout title="Home">
      <h1>Hello</h1>
      <Dialog>
        <$Trigger>Install</$Trigger>
        <$Title>Install</$Title>
        <p>pnpm add @reactogenic/core@alpha</p>
        <$Action key="close">Close</$Action>
      </Dialog>
    </Layout>
  );
}
```

CSS is plain `.css`, imported by the module that needs it. TypeScript 7
checks that import too: a project declares `*.css` once (`declare module
"*.css";`), and `@reactogenic/ui` does it for a project that imports it.

**Components.** `@reactogenic/ui` has `Button`, `Dialog`, `DropdownMenu` and
`SideMenu`, each the platform's own element (`<dialog>`, `popover`,
`<details>`) filled through slots. The import is all there is to write: no
stylesheet to add, no script to register, no handler. A component imports
its CSS and mounts its behaviour, and the builder ships both to the pages
that rendered it — and to no other.

**The output.**

```
$ reactogenic build
2 pages written to dist
```

```
dist/index.html            the page, its CSS in a <style>, its script in a <script type="module">
dist/guide/index.html      no dialog here: none of its CSS, and no <script> at all
dist/favicon.svg
dist/_rg/report.json       what every page ships, in bytes
```

```html
<!-- dist/index.html: what <Dialog> became. No React, and nothing to hydrate -->
<button type="button" class="rg-button" command="show-modal" commandfor="d1">Install</button>
<dialog id="d1" class="rg-dialog" closedby="any" aria-labelledby="d1-t">…</dialog>
```

- CSS or JS that two or more pages share is a file under `dist/_rg/`, named
  by its content, when that is cheaper than a copy in each page: `--inline
  auto` (the default), `always` or `never`.
- Links are written from the site's root (`/guide/`); `--base /docs/` builds
  the site for a path.
- A build empties `dist`, so `--out` takes only a directory that is empty or
  holds a previous build.
- Any error stops the build and nothing is written: what `check` reports,
  and what only the whole page shows.

```
pages/guide/index.rtsx: error link-not-found: Page /guide/: `href="/guide/instal/"` on `<a>` is neither a page nor a file of the output
pages/guide/index.rtsx: error idref-not-found: Page /guide/: `commandfor="install"` on `<button>` names no element of the page
```

**`--report`** prints what `_rg/report.json` holds — for every page, why
each byte is there:

```
/  pages/index.rtsx
                  raw     gzip
  HTML            825      390
  CSS            2533      965   inline
  JS              563      328   inline
  document       3967     1608   index.html
  components Action ×1, Button ×2, Dialog ×1, Each ×1, Layout ×1, Page ×1
  behaviours @reactogenic/ui/behaviors/overlays
             @reactogenic/ui/behaviors/invokers
  JS bytes      248 B  @reactogenic/ui/src/behaviors/overlays.ts
                307 B  @reactogenic/ui/src/behaviors/invokers.ts
                  8 B  <entry>  the mount calls
  CSS rules    1 kept,   0 dropped  @reactogenic/ui/src/tokens.css
               3 kept,   1 dropped  @reactogenic/ui/src/button.css
              14 kept,   0 dropped  @reactogenic/ui/src/dialog.css
               0 kept,   7 dropped  @reactogenic/ui/src/dropdown-menu.css
               0 kept,  25 dropped  @reactogenic/ui/src/side-menu.css
               2 kept,   1 dropped  site.css

/guide/  pages/guide/index.rtsx
                  raw     gzip
  HTML            238      182
  CSS              79       93   inline
  JS                -        -   none
  document        332      243   guide/index.html
  components Layout ×1, Page ×1
  CSS rules    2 kept,   1 dropped  site.css
```

**What a page may do.** A page is executed once, for everyone: loops and
conditionals over constants, components, slots, `Each`, `Match`, segments —
and no event handler, no state or effect, no clock and no dice.

```tsx
<ol>{PAGES.map((name) => <li key={name}>{name}</li>)}</ol>   // fine: a constant
<button onClick={() => alert("hi")}>Hello</button>
```

```
pages/guide/index.rtsx(15,15): error shell-handler: The shell cannot handle events: `onClick` on `<button>`
  pages/guide/index.rtsx:6:1 - in Page
```

**Browsers.** Chrome and Edge 135, Firefox 147, Safari 26.2: from there on,
opening, closing, focus and placement are the browser's own.

**What phase 2 does not have.**

| | |
| --- | --- |
| islands | no React in the page at all, so nothing interactive beyond what the components bring: `<Dynamic>` is specified ([later/layout.md](../specs/later/layout.md)) and not built |
| a dev server | no watch mode, no HMR: run `build` again |
| Markdown | pages are `.rtsx`; code samples are plain `<pre>`, not highlighted |
| assets | `public/` is copied as it is; images, fonts and `url()` in CSS are not processed |
| the page's rules in the editor | handlers, hooks and the clock are found by executing the page: `build` reports them, `check` and the language server do not |
| every browser | the built site was run in Chromium 153 and WebKit 26.6; not in Firefox, and not at the floor's own versions |

## Reference

- [specs/phase01/syntax.md](../specs/phase01/syntax.md) — every extension, with
  its desugaring and errors.
- [specs/phase01/vite.md](../specs/phase01/vite.md) — the Vite plugin.
- [specs/phase01/diagnostics.md](../specs/phase01/diagnostics.md) —
  `reactogenic check` and how errors are mapped back.
- [specs/phase01/ide.md](../specs/phase01/ide.md) — the language server, the
  grammar and the VS Code extension.
- [specs/phase02/builder.md](../specs/phase02/builder.md) — `reactogenic
  build`: routes, what shell code may do, CSS pruning, behaviours, the
  report.
- [specs/phase02/components.md](../specs/phase02/components.md) —
  `@reactogenic/ui`: `Button`, `Dialog`, `DropdownMenu`, `SideMenu`.
