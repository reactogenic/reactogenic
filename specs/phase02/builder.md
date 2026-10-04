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
| serialisation | React's own `renderToStaticMarkup`, from the project's `react-dom` | shell HTML is what React renders for the same component, under `TZ=UTC` |
| everything component-aware | ours | the record, the checks, CSS pruning, behaviour selection, packaging |

**One resolver.** Every import of a project file is resolved by the program
(the esbuild plugin's `OnResolve` asks it), and a program file's text is the
program's (`file.Text()` — the emitted TSX for an `.rtsx` module): what is
built is what was checked. esbuild resolves only what the program does not
hold (assets, behaviour modules).

| An import made by a program file | Resolved by |
| --- | --- |
| the program resolved it to a module with code (`./layout` → `layout.rtsx`, a `paths` alias, a workspace package's `.ts`) | the program |
| the program resolved it to a package's declarations (`lib` → `node_modules/lib/index.d.ts`) | esbuild: the package's JavaScript, from the importing file |
| the program resolved it to declarations of the project's own (`~/legacy.js` → `legacy.d.ts`, through a `paths` alias or not) | the program: the JavaScript beside the declarations (`legacy.js`; `.mjs` for `.d.mts`, `.cjs` for `.d.cts`) |
| `*.css` | nobody, in the render bundle: an empty module (*CSS*) |
| anything else (`./logo.svg`) | esbuild; no loader is render-bundle (below) |

| An import made by **any** module of the bundle but React's own | Resolved to |
| --- | --- |
| `react`, `react/jsx-runtime`, `react/jsx-dev-runtime` | the builder's `react` and JSX runtime, which wrap the project's React (*Shell code in phase 2*, *The record*) |

esbuild reads no `tsconfig.json`: the JSX settings are the builder's, and a
program file is compiled with esbuild's defaults for the rest
(*Not in phase 2*).

| Code | Condition |
| --- | --- |
| render-bundle | the render bundle cannot be made: an import that esbuild cannot resolve or load, at the import in the `.rtsx`; a page that is not a module of the program (the tsconfig does not include it: it was not checked); no `react` or `react-dom` in the project (one report, without a file: *react and react-dom are not installed in …*), or a `react-dom` whose static renderer the builder does not know (*The record*). No page is rendered |

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
| page-not-document | the page's root element is not `<html>`: what it rendered does not start with `<html` | the page's `export default` — found in the syntax, so not one in a comment or a template; for `export { Page as default }`, the `Page as default` |
| pages-not-found | `--pages` is not a directory, or holds no page | |

## Shell code in phase 2

Shell code is **executed once per pathname, at build time**. That settles how
layout.md's shell rules read for now:

| | layout.md | Phase 2 |
| --- | --- | --- |
| S2 no variance | no `Match`, `Switch`, `Each`, `?:`, `.map()` | **no *runtime* variance.** Anything computed while the page is executed is a compile-time value — loops over constants, conditionals on props and on the pathname. Nothing conditional is left *in the output* |
| S3 compile-time values | literals, constants, props | whatever execution yields; there is nothing else to read |
| S4 deterministic | no `Date`, `Math.random`, I/O | enforced by the engine: the clock and the dice throw, and the time zone is UTC on every machine (*The engine*) |
| S1 no React runtime | no hooks, state, effects, handlers | no handlers, state, effects or refs (below) — in the project's code and in a package's alike. Pure render-time React — components, `children`, `Children`, `cloneElement`, context, `useId`'s replacement — is the same code React runs, on React's own elements |
| S5 containers unlooped | | folded into S2 |

This is the one reading under which the design system's own components
(`Each` over slot entries, a footer only when `$Action` is filled) are legal
shell code with no special case, and under which an author may loop over a
constant. **Rejected:** S2 absolute with the design system exempt — a third
kind of code, and `NAV.map(…)` over a constant is deterministic, so
forbidding it is a fake constraint.

| Code | Message | How it is found |
| --- | --- | --- |
| shell-handler | The shell cannot handle events: `onClick` on `<button>` | a function-valued prop on a host element, when the element is created — by JSX, `createElement` or `cloneElement`, by the project or by a package |
| shell-react | The shell cannot use React state or effects: `useState` | `useState`, `useReducer`, `useEffect`, `useLayoutEffect`, `useInsertionEffect`, `useRef`, `useImperativeHandle`, `useSyncExternalStore`, `useTransition`, `useDeferredValue`, `useOptimistic`, `useActionState` throw when a page **calls** them — however the hook was reached: imported, re-exported, renamed, `React.useState`, `React["useState"]`, `const { useRef } = React`, in a compiled package |
| shell-nondeterministic | `Date.now()` makes the shell irreproducible | `Date.now`, `new Date()` without arguments, `Date()`, `Math.random`, `crypto.getRandomValues`, `crypto.randomUUID`, `performance.now` throw in the engine; so does a date string that is not ISO 8601 — *A date string that is not ISO 8601 ("Oct 4 2026") makes the shell irreproducible*. `new Date(2026, 9, 4)` is a compile-time value: midnight, UTC |
| shell-error | the message of the exception, after its name when it is not a plain `Error`: `TypeError: cannot read property 'map' of undefined` | anything else thrown while a page renders. Among it, what the engine itself ends: `InternalError: stack overflow` (a call depth of 10 000), and a page that does not finish in 30 s — *Rendering did not end in 30s: a loop without an end?*; and what the engine lacks — *Number.prototype.toLocaleString() needs Intl, which the builder's engine does not have* |
| shell-console (warning) | `console.log: ` and what was printed | `console` is collected: nobody reads the console of a build. At the call |

Every one is found by **executing** the page, and ends it: one error per
page, the first. A hook in a module that a page imports and does not render
— a barrel that also exports an island's component — is not an error: as for
the record, what counts is what ran, not the import graph. **Rejected:**
shell-react from the imports of the pages' module closure — it missed
re-exports, `const { useRef } = React` and compiled packages, and failed
pages that render none of it.

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
| the position | the innermost frame of the exception's stack that is in the project's code — not React's, not a package's: the helper that read the clock, not the component that called it; the call of the hook. Columns count characters, as in `check` |
| shell-handler | narrowed to the attribute: `onClick` of `<button onClick={…}>`. A function that arrives through a spread (`<input {...props} />`): the element. By `createElement` or `cloneElement`: the call, or its statement |
| the component stack | the **owners**, as React's own stacks have them: each component at the place its element was written, the page last, at its `export default` |
| a component whose element is not JSX with a position | `<Row {...rest} key="r" />` — a key after a spread, which esbuild compiles to `createElement`: the element all the same, from the engine's stack. Made by `React.createElement` by hand, or by a package: the line has no position (`  in Row`) |
| an exception in a package's code (its component has a handler, calls a hook, throws) | no frame is the project's: the nearest element of the component stack that the project wrote — `<LibButton>` |
| an exception of React's own (an object as a child) | no frame is the project's: the element of the component called last, its line reading `after Coordinates`. React's production build words it as an error number and a link |
| a module's top level | what a module throws or prints while it loads is no page's: reported once, and no page is rendered |
| a package's top level | no frame is the project's and there is no component: at the project's import that leads to the package, by the shortest way, with where it threw as a related line |

```
ids.ts(1,21): error shell-nondeterministic: Math.random() makes the shell irreproducible
  node_modules/seed/index.js:3:19 - thrown here
```
| an error that several pages share (a layout's) | reported once |

### The engine

Shell code runs in `modernc.org/quickjs`: ES2023, and nothing of a host.
What that means for the code of a page:

| | In the engine |
| --- | --- |
| the time zone | **UTC, on every machine**: the build machine's zone is not in the page. `new Date(2026, 9, 4)` is `2026-10-04T00:00:00.000Z`; `getHours()`, `getDay()`, `setMonth()` are their `UTC` counterparts; `getTimezoneOffset()` is `0`; `toString()` is what V8 writes under `TZ=UTC` |
| date strings | ISO 8601 as ECMAScript defines it — `2026-10-04`, `2026-10-04T12:00` (no zone: UTC), `2026-10-04T12:00+02:00` — and what `toString()` and `toUTCString()` write. Any other string is parsed as each engine likes, and in local time: shell-nondeterministic |
| `Intl` | **there is none**: `Intl.NumberFormat` is a `ReferenceError`. The methods that would answer without it — a number unformatted, strings compared by code unit — throw shell-error instead: `toLocaleString`, `toLocaleDateString`, `toLocaleTimeString` (of `Number`, `BigInt`, `Date`), `localeCompare`, `toLocaleUpperCase`, `toLocaleLowerCase`. Format in code: `toFixed`, `padStart`, a table of month names |
| not there (a `ReferenceError`) | `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`, `structuredClone`, `atob`, `btoa`, `queueMicrotask`, `setTimeout`, `setInterval`, `fetch`, `Temporal`, `process`, `require`; no `Array.fromAsync` |
| `Math`'s transcendental functions | the engine's: `Math.tan(1)` is `1.557407724654902` here and `1.5574077246549023` in V8. The standard leaves the last digit to the implementation: round what is rendered |

```tsx
const day = new Date(2026, 9, 4);
<time dateTime={day.toISOString()}>{day.getDate()}.{day.getMonth() + 1}.</time>
```

```html
<time dateTime="2026-10-04T00:00:00.000Z">4.10.</time>   <!-- in Berlin, in Tokyo, in CI -->
```

Beyond that the engine is held to V8: every fixture page is what React in
Node renders under `TZ=UTC`, to the byte (plan.md, the differential test).
**Rejected:** refusing local time (`new Date(2026, 9, 4)` an error — the
natural way to write a date); the machine's zone (a laptop and CI build
different pages from one source); an engine with `Intl` (ICU's data, or cgo).

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

In the render bundle, `react` and React's JSX runtime are the builder's own,
for every module: the project's files (`jsxImportSource`), a file with a
`@jsxImportSource` pragma, a package compiled against `react/jsx-runtime` or
`React.createElement`. They wrap the project's React. An element is made by
React and **left as React made it** — `element.type` is the component, so
`child.type === Tab`, `Children`, `cloneElement` and a component's statics
work as in React:

```tsx
function Tabs({ children }: { children: ReactNode }) {
  const tabs = Children.toArray(children).filter((child) => isValidElement(child) && child.type === Tab);
  return <ul data-tabs={tabs.length}>{tabs}</ul>;   // the Tabs's own Tab children, as in React
}
```

| What the builder adds | Where |
| --- | --- |
| shell-handler | when an element is made: `jsx`, `jsxs`, `createElement`, `cloneElement` |
| shell-react | the twelve hooks of the bundle's `react` throw |
| where an element was written, and by which component | noted beside the element (by its props object), from esbuild's `jsxDev` position: the element is not touched |
| the count, and the component stack of an exception | React's static renderer calls a function component through the builder — the **one change** made to its text, where it reads `Component(props, secondArg)` |

Type checking is untouched — it still sees React's types.

- A component is counted when React **calls** it: an element that is made
  and never rendered (an unused fallback, a slot nobody attaches, a child
  that `Tabs` drops) is not in the record.
- By name: `function Page` of two modules is one entry. A `memo` and a
  `forwardRef` are counted by the function they wrap; an element made by
  hand or by a package counts as any other. A class is rendered and not
  counted.
- The renderer is found by its path in the project's `react-dom` and changed
  by its text: a React whose static renderer is elsewhere, or calls its
  components otherwise, is render-bundle — never a page without a record.
- One React: the project's. A package that would resolve a copy of its own
  gets the project's.

**Rejected:** a function of the builder's as the element's `type`, calling
the component (the first implementation: `child.type === Tab` is false and
the statics are gone — a page silently different from React's); checking a
host element's props when it is rendered (a second change to the renderer,
and the element's position is lost).
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
| shell rules in `check` and the editor | `build` reports them: every one is found by executing the page |
| every violation of a page in one build | the first one ends the page |
| `Intl`, a time zone other than UTC | *The engine* |
| `.rtsx` positions for the page checks | needs attribute provenance through the renderer |
| Markdown, highlighted code samples | a second front end; samples are plain `<pre>` |
| source maps for the emitted JS | |
| the tsconfig's emit options in the render bundle | esbuild compiles a program file with its own defaults for `useDefineForClassFields`, `experimentalDecorators` and the like: shell code is function components |
| `async` components, `use()`, Suspense | the renderer is synchronous; the engine has no timers |
| asset pipeline (images, fonts), `public/` | a `public` directory next to `pages` is copied as is; nothing is processed |
