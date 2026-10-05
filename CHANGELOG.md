# Changelog

The npm packages (`@reactogenic/core`, `@reactogenic/vite`, `@reactogenic/cli`
and its six platform packages) share one version. The VS Code extension has
its own: [packages/vscode/CHANGELOG.md](packages/vscode/CHANGELOG.md).

## Unreleased

The builder (specs/phase02/builder.md). In the repository; not published.

**New**

- `reactogenic build` — the pages of a site become, per page, plain HTML, the
  CSS that page can use and the JS of the behaviours its components mounted.
  No React in the output. A directory under `pages/` is a **route**; an
  `.rtsx` file of it that nothing of the project mounts or imports is a
  **variant** of the route, rendering its document from `<html>`:
  `index.rtsx` is built to `index.html`, `guest.rtsx` to `guest.html`
  beside it. Any other file there is a segment or a module (`index.tsx`
  makes no page). `--out`, `--pages`, `--base /docs/`, `--inline
  auto|always|never`; `--report` prints the bytes of every page, always
  written to `<out>/_rg/report.json`; `--no-specialize` builds the control
  of the measurement. Any error of `check` stops the build, and so does
  what only a built page shows: a handler, a hook of state or an effect, the
  clock, a script of the page's own (`shell-*`), an id or a link that names
  nothing (`idref-not-found`, `link-not-found`, …). A `<style>` element of
  a page is pruned as the page's CSS is; a blob that pages share is a file
  from 4096 B. A rule on state that only a script can write — `aria-*`,
  `disabled`, `inert`, `data-state`, `checked`, `selected`, `value` — is
  kept for a page only if the page has the state or its script names it
  (the attribute, or the property that reflects it: `ariaExpanded`); what
  the browser writes by itself (`open`, `hidden`, `style`) and every
  pseudo-class always may match. Docs: *Build* in docs/getting-started.md.
- `@reactogenic/core`: `pathname()`, `useShellId(prefix?)` and
  `mount(module, id?, flags?, data?)` — what a component asks the builder:
  the route being built, an id that reads well in view-source (`d1`, `m2`),
  a behaviour for the page. `mount`'s fourth argument is the use site's
  **data**: a plain object of JSON values that only the behaviour reads,
  handed to it as the second argument of that mount's call
  (`behaviour(root, data)`) — where `flags` say whether the code is in the
  page's script at all. Types: `MountData`, `MountValue`. In React (Vite) they are `location.pathname`,
  `useId()` and nothing. With `useShellId` the package imports `react` at
  run time, no longer its types alone; it was a peer dependency already.
- `@reactogenic/ui` — `Button`, `Dialog`, `DropdownMenu`, `SideMenu`
  (specs/phase02/components.md). `closeLabel` on `Dialog` and `SideMenu`
  names their close button (default "Close"). Not published yet: `"private": true` in
  its manifest only keeps it off npm. It is to be public, under a branded
  name the owner will pick — `@reactogenic/ui` is a working name; the
  components live in `packages/ui`.

**Fixed**

- **Keyed slots keep the order written** (specs/phase01/syntax.md, *Keyed
  slots*). A container that renders every entry of a `KeyedSlot` — a menu's
  items, a table's columns — got integer-like keys first, ascending: written
  `key="10"`, `key="9"`, `key="2"`, rendered `2`, `9`, `10`. That is
  JavaScript's order of an object's property names, and it is not `.rtsx`'s.
  An entry's property name is now its key encoded so that it is never
  integer-like (`"10"` → `"#10"`, `"#x"` → `"##x"`, any other key as it is),
  and `key={expr}` is emitted as `[slotEntryName(expr)]`. **0.1.0-alpha.1
  has the bug** — in the transpiler and in `@reactogenic/core`: both are
  fixed together, so the binary and the runtime are to be updated together.
  A container reads the keys with `slotKeys($X)`, never `Object.keys($X)`;
  attachments (`key=… slot={$X}`) are unchanged.
- `@reactogenic/core`: two new exports. `slotKeys(slot)` — the keys of a
  keyed slot as the caller wrote them, in order (`[]` for a slot that is not
  there). `slotEntryName(key)` — the property name of an entry, for a keyed
  slot written by hand in `.tsx`: `{ [KEYED]: true, [slotEntryName(id)]: … }`.
  `slotEntry(slot, key)` finds an entry under its encoded name, and under a
  raw integer-like one still; a name the object only inherits (`toString`)
  is no entry any more.
- `<$X key="__proto__">` is an entry: it was read as the object's prototype.
- An explicit `$X={…}` attribute beside keyed slot elements type-checks: the
  `KEYED` marker is written after the attribute's spread, where it was
  reported as overwritten (TS2783).
- `slot-key-inline` — a key that is neither a string nor a number — is
  reported on the `key`'s value (before: on the attribute).

**The binary** is 29–31% larger: it holds esbuild and a JavaScript engine,
which execute the pages. Stripped, in MiB (specs/phase02/decisions.md,
*Binary size*):

| | 0.1.0-alpha.1 | with `build` |
| --- | --- | --- |
| darwin-arm64 | 27.3 | 35.1 |
| darwin-x64 | 28.6 | 37.1 |
| linux-arm64 | 26.4 | 34.3 |
| linux-x64 | 27.9 | 36.5 |
| win32-arm64 | 26.5 | 34.3 |
| win32-x64 | 28.2 | 36.9 |

## 0.1.0-alpha.1 — 2026-10-04

IDE support (specs/phase01/ide.md).

**New**

- `reactogenic lsp --stdio` — a language server for `.rtsx`: TypeScript 7's,
  with the transform built in. Diagnostics are those of `reactogenic check`.
- The VS Code extension `reactogenic.rtsx`: highlighting, the server, and
  `.rtsx` modules in VS Code's own TypeScript.
- `reactogenic content-mapper`, declared by `@reactogenic/cli`: `.rtsx` in
  stock TypeScript 7.1 (`contentMappers` in tsconfig). Experimental.
- `reactogenic --version`.

**`reactogenic check`** now type-checks `.rtsx` files under their own names
(before: a `.rtsx.tsx` copy beside each). What that changes in its output:

- Each mistake is reported once: an error in a mounted segment, or in a
  module imported as `./b.rtsx`, is no longer printed a second time, and the
  two TS2339 of one attachment are one.
- Related locations name the `.rtsx` file (`props.rtsx:1:32`, not
  `props.rtsx.tsx:1:32`).
- A file with a syntax error no longer turns into an empty module: its
  importers are checked, without a TS2306 each.
- A file from which the transpiler had to leave code out (an orphaned slot,
  `arg-without-slot`, `params-on-html`) reports the transpiler's error and
  no TypeScript error until it is fixed.
- `segment-not-found` comes without TypeScript's TS2307 on the same `#name`.
- Project references: referenced projects are checked first, each file is
  reported once, by the project that lists it and with that project's
  options. A missing reference is one line (TS6053).
- `include` keeps TypeScript's meaning: a pattern that names extensions
  (`src/**/*.tsx`) lists no `.rtsx` file — write `src/**/*` or add
  `src/**/*.rtsx`. Such a file is still checked when an import reaches it.
- Declaration errors (TS4094, …) are reported for projects that emit
  declarations.
- `Foo.tsx` beside `Foo.rtsx` is still `ambiguous-module`, and the `.rtsx`
  is now checked too.
- A segment under `moduleResolution: node16` / `nodenext` resolves (before:
  TS2307).
- `--watch` follows referenced projects, and `.jsx` / `.js` files.

**Fixed**

- `&&name={expr}`: a type error in `expr` was dropped (`check` exited 0).
- `<section#intro />` and `<section hidden#intro />` — no space before `#` —
  emitted invalid TSX.
- `&#name` was accepted as an arg; it is a syntax error.
- An arg of the wrong type was reported as `slot-no-args` ("takes no args");
  it is TypeScript's TS2322.

## 0.1.0-alpha.0 — 2026-09-28

The first release: `.rtsx` in Vite (`@reactogenic/vite`), the runtime
(`@reactogenic/core`), `reactogenic check` (`@reactogenic/cli`).
