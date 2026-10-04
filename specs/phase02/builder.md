# The builder: `reactogenic build`

Pages in `.rtsx` + framework-owned layout components → per-page plain HTML,
CSS and minimal raw JS. **No React in the output.** Why it is shaped this way:
[research.md](research.md).

Phase 2 has no islands and no `Dynamic`: every page is shell
([later/layout.md](../later/layout.md)).

```
reactogenic build [-p tsconfig.json] [--pages <dir>] [--out <dir>] [--base <path>]
                  [--inline auto|always|never] [--no-specialize] [--report]
```

| Flag | Default | |
| --- | --- | --- |
| `-p` | `tsconfig.json` in the working directory | the project, as for `check` |
| `--pages` | `pages` next to the tsconfig | the root of the routes |
| `--out` | `dist` next to the tsconfig | emptied first; refused if it holds the tsconfig or the pages |
| `--base` | `/` | the path the site is served under; prefixes asset URLs |
| `--inline` | `auto` | *Packaging* |
| `--no-specialize` | | the control of the bet: one unpruned CSS bundle, every behaviour with every flag on (*The control*) |
| `--report` | | print the byte report (*The report*); it is always written to `<out>/_rg/report.json` |

Exit status as `check`: 0, 1 with diagnostics, 2 on a usage error.

## The pipeline

```
tsconfig ─▶ program (phase 1: mapper + tsgo) ─▶ diagnostics ─ any error stops the build
   │
   ├─ routes            pages/**/index.rtsx → pathnames
   ├─ render bundle     esbuild: every page + React's static renderer, in memory
   ├─ execute           embedded engine: render(pathname) → HTML + record, per page
   ├─ check the page    ids, references, commands, links
   ├─ CSS               esbuild: the page's CSS in import order → pruned against its HTML
   ├─ JS                the record's behaviours → generated entry → esbuild with the page's flags
   └─ package           dedupe by content, inline or file, write, report
```

| Stage | Owner | |
| --- | --- | --- |
| program, diagnostics | phase 1 | `mapper.RegisterStrict`, `rtsx.NewProgram`, `report.Program`: what `check` prints, `build` prints, and stops |
| bundling, minifying, lowering | esbuild `pkg/api`, in-process, public API only | never forked; `Splitting` off; `Write: false` |
| execution | `modernc.org/quickjs` embedded in the binary | no Node at build time |
| serialisation | React's own `renderToStaticMarkup`, from the project's `react-dom` | shell HTML is what React renders for the same component |
| everything component-aware | ours | the record, the checks, CSS pruning, behaviour selection, packaging |

**One resolver.** Every import of a project file is resolved by the program
(the esbuild plugin's `OnResolve` asks it), and a program file's text is the
program's (`file.Text()` — the emitted TSX for an `.rtsx` module): what is
built is what was checked. esbuild resolves only what the program does not
hold (assets, behaviour modules).

| An import made by a program file | Resolved by |
| --- | --- |
| the program resolved it to a module with code (`./layout` → `layout.rtsx`, a `paths` alias, a workspace package's `.ts`) | the program |
| the program resolved it to declarations (`react` → `@types/react/index.d.ts`) | esbuild: the package's JavaScript, from the importing file |
| `*.css` | nobody, in the render bundle: an empty module (*CSS*) |
| anything else (`./logo.svg`) | esbuild; no loader is render-bundle (below) |

esbuild reads no `tsconfig.json`: the JSX settings are the builder's, and a
program file is compiled with esbuild's defaults for the rest
(*Not in phase 2*).

| Code | Condition |
| --- | --- |
| render-bundle | the render bundle cannot be made: an import that esbuild cannot resolve or load, at the import in the `.rtsx`; no `react-dom` in the project. No page is rendered |

**No `text/template`, `html/template` or reflection-heavy package in the
binary**: one `template.Execute` costs 18.6 MB (research.md). HTML is written
as strings.

## Routes

`index.rtsx` (or `index.tsx`) of a directory under `--pages` is a page; its
pathname is the directory, with a trailing slash.

```
pages/index.rtsx                 /
pages/guide/index.rtsx           /guide/
pages/guide/install.rtsx         not a page: a segment or a module of /guide/
pages/reference/cli/index.rtsx   /reference/cli/
```

- The page is the module's **default export**, a component without props that
  renders the whole document, from `<html>`. A shared layout is an ordinary
  component (`<DocsLayout title="…">`), as in React.
- Output: `<out>/<pathname>index.html`, preceded by `<!doctype html>`.
- No route table file in phase 2: the table is derived, and still gives the
  check (*Checks*: link-not-found).

| Code | Condition | Reported at |
| --- | --- | --- |
| page-no-default | `index.rtsx` has no default export that is a component: the module's `default` is not a function | the file |
| page-not-document | the page's root element is not `<html>`: what it rendered does not start with `<html` | the page's `export default` |
| pages-not-found | `--pages` is not a directory, or holds no page | |

## Shell code in phase 2

Shell code is **executed once per pathname, at build time**. That settles how
layout.md's shell rules read for now:

| | layout.md | Phase 2 |
| --- | --- | --- |
| S2 no variance | no `Match`, `Switch`, `Each`, `?:`, `.map()` | **no *runtime* variance.** Anything computed while the page is executed is a compile-time value — loops over constants, conditionals on props and on the pathname. Nothing conditional is left *in the output* |
| S3 compile-time values | literals, constants, props | whatever execution yields; there is nothing else to read |
| S4 deterministic | no `Date`, `Math.random`, I/O | enforced by the engine: they throw |
| S1 no React runtime | no hooks, state, effects, handlers | no handlers, state, effects or refs (below). Pure render-time React — components, `children`, `useId`'s replacement — is the same code React runs |
| S5 containers unlooped | | folded into S2 |

This is the one reading under which the design system's own components
(`Each` over slot entries, a footer only when `$Action` is filled) are legal
shell code with no special case, and under which an author may loop over a
constant. **Rejected:** S2 absolute with the design system exempt — a third
kind of code, and `NAV.map(…)` over a constant is deterministic, so
forbidding it is a fake constraint.

| Code | Message | How it is found |
| --- | --- | --- |
| shell-handler | The shell cannot handle events: `onClick` on `<button>` | a function-valued prop on a host element, when the element is created |
| shell-react | The shell cannot use React state or effects: `useState` | an import of `useState`, `useReducer`, `useEffect`, `useLayoutEffect`, `useInsertionEffect`, `useRef`, `useImperativeHandle`, `useSyncExternalStore`, `useTransition`, `useDeferredValue`, `useOptimistic`, `useActionState` from `react` in a module a page reaches; reported at the import — the name in `import { useState }`, or in `React.useState` when `react` is imported whole. Found in the text: nothing runs for it, and the page still renders |
| shell-nondeterministic | `Date.now()` makes the shell irreproducible | `Date.now`, `new Date()` without arguments, `Date()`, `Math.random`, `crypto.getRandomValues`, `crypto.randomUUID`, `performance.now` throw in the engine. `new Date(2026, 9, 4)` is a compile-time value |
| shell-error | the message of the exception, after its name when it is not a plain `Error`: `TypeError: cannot read property 'map' of undefined` | anything else thrown while a page renders. Among it, what the engine itself ends: `InternalError: stack overflow` (a call depth of 10 000), and a page that does not finish in 30 s — *Rendering did not end in 30s: a loop without an end?* |
| shell-console (warning) | `console.log: ` and what was printed | `console` is collected: nobody reads the console of a build. At the call |

Every one is reported at the `.rtsx` (or `.tsx`, `.ts`) position, through the
render bundle's source map and the file's span map, with the component stack
as related lines. `build` reports them; `check` and the editor do not yet
(*Not in phase 2*).

```
pages/pricing/index.rtsx(8,15): error shell-error: no price for enterprise
  pages/pricing/index.rtsx:17:24 - in Price
  pages/pricing/index.rtsx:16:7 - in Each
  pages/pricing/index.rtsx:26:7 - in Plans
  pages/pricing/index.rtsx:23:1 - in PricingPage
```

| | Where |
| --- | --- |
| the position | the innermost frame of the exception's stack that is in the project's code — not React's, not a package's: the helper that read the clock, not the component that called it. Columns count characters, as in `check` |
| shell-handler | narrowed to the attribute: `onClick` of `<button onClick={…}>`. A function that arrives through a spread (`<input {...props} />`): the element |
| the component stack | the **owners**, as React's own stacks have them: each component at the place its element was written, the page last, at its `export default` |
| an exception of React's own (an object as a child) | no frame is the project's: the element of the component called last, its line reading `after Coordinates`. React's production build words it as an error number and a link |
| a module's top level | what a module throws or prints while it loads is no page's: reported once, and no page is rendered |
| an error that several pages share (a layout's) | reported once |

### What shell code can ask the builder

Three functions of `@reactogenic/core`, usable in any component:

```ts
pathname(): string                       // the page being rendered: "/guide/"
useShellId(prefix?: string): string      // "d1", "m1", "m2": per page, in render order
mount(module: string, id?: string, flags?: Record<string, boolean>): void
```

| | At build time | In React (an island, Vite) |
| --- | --- | --- |
| `pathname()` | the route being rendered | `location.pathname` |
| `useShellId(p)` | `p` + a counter per prefix, per page; no prefix: `r1`, `r2` | `React.useId()` |
| `mount(m, id, flags)` | recorded for the page (*Behaviours*) | nothing in phase 2 |

`useShellId` exists because React's `useId` gives `_R_3e_`: valid, and
unreadable in view-source. Ids are stable while the page's tree is.

## The record

What makes the builder component-aware is not the import graph (it
over-reports: a layout that *can* attach a dialog imports it on every page) —
it is what executing the page **recorded**:

| | Source | Used for |
| --- | --- | --- |
| the HTML | React's renderer | the page; CSS pruning; the checks |
| mounts: `(module, id, flags)` | `mount()` | the page's JS |
| components rendered, with counts | the builder's JSX runtime | the report |

The JSX runtime is the builder's own (`jsxImportSource` set for the render
bundle only): it wraps `react/jsx-runtime`, counts function components by
name, and raises shell-handler. Type checking is untouched — it still sees
React's JSX types.

- A component is counted when React **calls** it: an element that is made
  and never rendered (an unused fallback, a slot nobody attaches) is not in
  the record.
- By name: `function Page` of two modules is one entry. Classes, `memo` and
  `forwardRef` components, and elements made by `React.createElement` by
  hand are rendered and not counted.
- What React adds is the page's too — the HTML is React's, to the byte: an
  empty `<head>` when the page has none, `<link rel="preload" as="image">`
  for an `<img srcSet>`, a `<meta>` written in the body moved into the head.

## Checks on the page

On the parsed HTML of each page. Things only something that sees the whole
page can check:

| Code | Condition |
| --- | --- |
| id-duplicate | two elements of the page share an `id` |
| idref-not-found | `commandfor`, `popovertarget`, `for`, `aria-labelledby`, `aria-describedby`, `aria-controls`, `anchor`, or `href="#x"` names no element of the page |
| command-target | `command="show-modal"`, `close` or `request-close` whose target is not a `<dialog>`; `show-popover`, `hide-popover`, `toggle-popover`, or any `popovertarget`, whose target has no `popover` |
| link-not-found | a root-relative `href` (`/guide/slot/`) that is neither a page nor a file of the output; `#fragment` and query are ignored for the match |

They are reported at the page (file and pathname) with the offending
attribute's text; mapping an attribute back to its `.rtsx` position needs
provenance the renderer does not carry (*Not in phase 2*).

## CSS

**Authoring.** Plain `.css`, imported by the module that needs it
(`import "./dialog.css"`). TypeScript 7 checks side-effect imports: a project
declares the module once (`declare module "*.css";`), or `check` says TS2882.
esbuild bundles a page's CSS in import order, lowers nesting and minifies. The design system's convention
([components.md](components.md), *CSS convention*) is what makes pruning
exact rather than heuristic.

**Pruning** is the builder's step, after the HTML exists: a rule stays only if
some element of *this page* may match it.

```
page entry ─ esbuild ─▶ flat, minified CSS ─ prune against the page's HTML ─▶ esbuild minify ─▶ the page's CSS
```

| | Rule |
| --- | --- |
| a selector | kept iff some element of the page **may** match. Type, class, id and static attributes are matched exactly; combinators against the real tree |
| runtime state is "maybe" | every pseudo-class except `:root`, `:is()`, `:where()`, `:not()`, `:has()` (which are evaluated on their arguments), and every attribute selector on `open`, `hidden`, `inert`, `disabled`, `checked`, `selected`, `value`, `style`, `aria-*`, `data-state`. The negation of "maybe" is "maybe" |
| pseudo-elements | ignored for matching (`::backdrop`, `::before`, `::details-content`) |
| a selector list | pruned per selector; the rule goes when none is left |
| `@media`, `@supports`, `@container`, `@scope`, `@starting-style` | pruned inside; dropped when empty |
| `@layer` | a block is pruned inside; an emptied block stays as `@layer name;` unless an earlier statement already orders it. **Dropping it would reorder the cascade** |
| `@keyframes` | kept iff its name occurs as a word in a kept `animation` or `animation-name` value |
| custom properties | a `--x` declaration is dropped iff `--x` occurs nowhere else: in no kept CSS (values and at-rule preludes) and in no `style` attribute of the page. Repeated to a fixed point |
| `@font-face`, `@property`, `@import`, `@namespace`, anything unknown | kept |
| a page that contains `<template>` | not pruned (phase 2 emits none; cloned content would need "maybe" relations) |

Soundness is the requirement: a dropped rule must never have matched. The
pruner is tested against an independent selector matcher, and the built docs
site by a computed-style comparison in a browser (pruned vs unpruned CSS,
every page; plan.md).

**Rejected:** PurgeCSS-style text scanning (drops `:focus-visible` and
runtime-state rules, keeps rules for names that only occur in prose);
vendoring esbuild for its CSS AST (a second fork — the flat, minified output
of the public API is regular enough to parse ourselves); CSS Modules and
CSS-in-JS (nothing here needs them).

## Behaviours

The JS of a page is the behaviours its components **mounted**, and nothing
else. A behaviour is a TypeScript module, owned by the design system:

```ts
// @reactogenic/ui/behaviors/menu-keys.ts
declare const RG_MENU_TYPEAHEAD: boolean;

export default function mountMenuKeys(root: HTMLElement): void {
  root.addEventListener("keydown", onKey);
  if (RG_MENU_TYPEAHEAD) root.addEventListener("keydown", onType);
}
function onKey(e: KeyboardEvent) { … }
function onType(e: KeyboardEvent) { … }
```

```tsx
// the component: an action menu needs arrow keys, a menu of links does not
const id = useShellId("m");
if (hasAction) mount("@reactogenic/ui/behaviors/menu-keys", id, { RG_MENU_TYPEAHEAD: typeahead });
```

```js
// generated entry for a page whose record holds two such mounts
import a from "@reactogenic/ui/behaviors/menu-keys";
import b from "@reactogenic/ui/behaviors/overlays";
a(document.getElementById("m1")); a(document.getElementById("m2")); b();
```

| | Rule |
| --- | --- |
| module | the default export is `(root: HTMLElement) => void`, or `() => void` for a page-level behaviour (mounted without an id; run once per page however often it is mounted) |
| flags | bare `declare const RG_…: boolean` identifiers — never an options object, an imported constant or a class member: esbuild removes code only on parser-time constants (research.md) |
| a page's flags | per module, the **union** over the page's mounts. So a flag only *adds* behaviour. What must differ between two use sites on one page is an attribute on the root, read at run time |
| every flag is defined | the builder defines each `RG_…` identifier found in the module's source — `false` unless a mount set it. A flag the builder did not see would be a `ReferenceError` at run time |
| no top-level side effects | a module that should have been dropped must be absent from the metafile; the builder checks it |
| build | one `api.Build` per page: generated entry, `Bundle`, minify, ES modules, `Define` = the page's flags, `Metafile`. No splitting |

`mountX(root)` per use site is layout.md's own design for clones opened from
islands, so phase 2's behaviours carry over.

| Code | Condition |
| --- | --- |
| mount-not-found | the module of a `mount()` does not resolve |
| mount-no-element | the id of a `mount()` is not on the page |
| mount-flag | a flag passed to `mount()` that the module does not declare |

## Packaging

Per page: its HTML, its pruned CSS, its JS. Identical CSS or JS on several
pages is **one blob**, by content hash.

| `--inline` | A blob is… |
| --- | --- |
| `auto` | a file when it serves several pages and `(pages − 1) × bytes > 1024`; otherwise inlined |
| `always` | inlined: one request per page |
| `never` | a file |

- Files: `<out>/_rg/<name>-<hash>.css` / `.js`, the hash from the content, so
  they can be cached forever.
- Inlined: `<style>` at the end of `<head>`; `<script type="module">` at the
  end of `<body>`. A linked script is `<script type="module" src>`; a linked
  stylesheet `<link rel="stylesheet">`.
- A page with no behaviours has no `<script>`. A page whose CSS is empty has
  no `<style>`.
- The rule is about caching, not size: a file costs ≈180–330 B of headers per
  request, so a small blob used once is always cheaper inline, and one shared
  by many pages is cheaper as a file from the second page.

## The report

`<out>/_rg/report.json`, and `--report` prints it: per page, the bytes of
HTML, CSS and JS (raw, gzip, brotli), how each blob is delivered, the
components rendered, the behaviours mounted with their flags, and per
behaviour module its bytes in the page's script (from esbuild's metafile) —
*why is this byte here*. CSS: rules kept and dropped per source file.

## The control

`--no-specialize` builds the same pages as a build that does not know what a
page rendered would: same HTML, and

| | Default | `--no-specialize` |
| --- | --- | --- |
| CSS | per page, pruned against its HTML | one file for the site: the unpruned bundle of everything any page imports |
| JS | per page: the modules it mounted, its flags | one file for the site: every behaviour mounted on any page, every flag on, and a table from pathname to that page's mounts |

The difference between the two builds is what component awareness is worth
(plan.md, RGP2-050).

## Not in phase 2

| | |
| --- | --- |
| islands, `Dynamic`, `<template>` delivery, holes | layout.md stands; nothing here blocks it — the engine already renders what React renders |
| a dev server, watch mode, HMR | out of scope |
| view transitions | out of scope |
| shell rules in `check` and the editor | `build` reports them; the rules that are static (shell-react) move to `internal/report` later |
| `.rtsx` positions for the page checks | needs attribute provenance through the renderer |
| Markdown, highlighted code samples | a second front end; samples are plain `<pre>` |
| source maps for the emitted JS | |
| the tsconfig's emit options in the render bundle | esbuild compiles a program file with its own defaults for `useDefineForClassFields`, `experimentalDecorators` and the like: shell code is function components |
| `async` components, `use()`, Suspense | the renderer is synchronous; the engine has no timers |
| asset pipeline (images, fonts), `public/` | a `public` directory next to `pages` is copied as is; nothing is processed |
