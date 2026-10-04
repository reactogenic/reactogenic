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
| `-p` | `tsconfig.json` in the working directory | the project, as for `check`: with the projects it `references` (*The project*) |
| `--pages` | `pages` next to the tsconfig | the root of the routes |
| `--out` | `dist` next to the tsconfig | emptied when the build writes — only what a build made; refused unless it is the builder's to empty (*The output directory*) |
| `--base` | `/` | the path the site is served under: prefixes the URLs of the build's own files and the pages' root-relative links (*Packaging*). `docs`, `/docs` and `/docs/` are one base. A usage error: a URL with a scheme, a host, a query or a fragment; a `%` that encodes nothing (`/100%/`) |
| `--inline` | `auto` | *Packaging* |
| `--no-specialize` | | the control of the bet: one unpruned CSS bundle, and one script — every behaviour the site mounts, every flag on (*The control*) |
| `--report` | | print the byte report (*The report*); it is always written to `<out>/_rg/report.json` |

Exit status as `check`: 0, 1 with diagnostics, 2 on a usage error. A path
given to a flag is relative to the working directory.

| Printed | Where |
| --- | --- |
| diagnostics, as `check --pretty=false` writes them, by file and position; a warning does not stop the build | stdout |
| with `--report`, the byte report | stdout, after them |
| `7 pages written to dist` | stdout, last |
| a usage error; an output that cannot be written (status 1) | stderr |

### The project

What `check -p` checks, `build -p` checks — and prints, and stops on. That
includes the projects the tsconfig `references`: Vite's template has a
`tsconfig.json` with no files of its own.

```
tsconfig.json        { "files": [], "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }] }
tsconfig.app.json    { …, "include": ["pages"] }         ← the pages are rendered from this project
tsconfig.node.json   { …, "include": ["vite.config.ts"] }  checked; an error in it stops the build
```

| | |
| --- | --- |
| checked | every project, each file reported once, by its own project: `check`'s rule |
| rendered from | **one** project: the one that lists the pages, in `files` or through `include`. Several list them: the first that is checked — a project comes after those it references. None does: the `-p` tsconfig's own |
| a page that project does not hold | render-bundle (*The pipeline*): pages of two projects are not one site |
| the project directory | the `-p` tsconfig's, whichever project lists the pages: `pages`, `public` and `dist` are next to it, `react` and a `mount()`'s module are resolved from it, the report names files from it |

`pages` and `public` may be symbolic links, each of its own — `public` is the
directory beside the `pages` that was named, not beside what that leads to. A
page is then the module the program has for that file, under whichever name
the tsconfig reached it by. A directory *inside* them that is a link is not
followed.

### The output directory

The build **empties** `--out`, so it has to be the builder's to empty: it
deletes only what a build wrote. Checked before anything is built; a refusal
is a usage error (exit 2). Asked again when the build writes, where it is
done — whoever called, and whatever became of the directory while the site
was built.

| `--out` is… | |
| --- | --- |
| missing, or an empty directory | used |
| a directory with `_rg/report.json` in it | a build's output: emptied, used |

| `--out` is refused when | |
| --- | --- |
| it holds the project (the tsconfig's directory), `--pages` or `public/`: it is one of them, or an ancestor | the build would delete its own source — whatever else the directory holds |
| it is inside `--pages` or `public/` | the next build would read this one's output |
| it is not a directory | |
| it is not empty and has no `_rg/report.json` | it is not a previous output of the builder — `_rg/` and an `index.html` are not enough: only the builder writes the report. Empty it yourself, or name another |

```
$ reactogenic build --out .
reactogenic build: --out /site holds the project (/site): the build empties its output directory
```

```
reactogenic build --out public        # refused: the project's own public/
reactogenic build --out ../shared     # refused: not empty, and no build made it
reactogenic build --out dist          # the second time: dist/_rg/report.json is there
```

- Directories are compared by what they are, not by their names: through a
  symbolic link, or in another case on a file system that folds it, the
  project is still the project.
- It is emptied **when the build writes**, not before: a build that stops on
  a diagnostic leaves the last output as it was.
- So does a build that **cannot write** — a file of `public/` it cannot
  read, a full disk, two names the file system takes for one (`public/Guide`
  and the page `/guide/`, where case is folded). The new output is written
  into a directory of its own inside `--out` (`.rg-…`) first; only when all
  of it is there is the old one removed and the new one moved up.
- From the first file written, `--out` holds `_rg/report.json` — the old one
  or the new: a build that is killed leaves a directory still known as the
  builder's, and the next build empties it.
- The directory itself stays, and a `.git` in it: an output that is a
  checkout of the branch it is published from. A directory that holds
  nothing else counts as empty.

**Rejected:** emptying whatever `--out` names unless it holds the tsconfig
or the pages (the first draft: `--out src`, `--out ~/Documents/site` were
wiped).

## The pipeline

```
tsconfig ─▶ program (phase 1: mapper + tsgo) ─▶ diagnostics ─ any error stops the build
   │
   ├─ routes            pages/**/index.rtsx → pathnames
   ├─ render bundle     esbuild: every page + React's static renderer, in memory
   ├─ execute           embedded engine, a runtime per page: render(pathname) → HTML + record
   ├─ check the page    ids, references, commands, links
   ├─ JS                the record's behaviours → generated entry → esbuild with the page's flags
   ├─ CSS               esbuild: the page's CSS in import order → pruned against the page as it is served, and its script
   └─ package           dedupe by content, inline or file, write, report
```

| Stage | Owner | |
| --- | --- | --- |
| program, diagnostics | phase 1 | `mapper.RegisterStrict`, `check.Program`: what `check` prints, `build` prints, and stops; the program of the project that lists the pages is kept (*The project*) |
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
binary**: one `template.Execute` costs 18.6 MiB (research.md). HTML is written
as strings.

## Routes

`index.rtsx` or `index.tsx` of a directory under `--pages` is a page; its
pathname is the directory, with a trailing slash. Both side by side is
`check`'s ambiguous-module — an import of `./index` could mean either — and
stops the build as any diagnostic does.

```
pages/index.rtsx                 /
pages/guide/index.rtsx           /guide/
pages/guide/install.rtsx         not a page: a segment or a module of /guide/
pages/reference/cli/index.rtsx   /reference/cli/
public/favicon.svg               /favicon.svg: copied as it is
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
| pages-not-found | `--pages` is not a directory, or holds no page. Found before the project is checked: without a page there is nothing to build | |
| public-conflict | a file of `public/` — the directory next to `--pages` — is where the build writes one of its own: a page's `index.html`, anything under `_rg/`; or is a file where the build makes a directory (`public/guide` beside the page `/guide/`, `public/_rg`), or under what the build writes as a file (`public/index.html/x`). Found before anything is written | the file of `public/` |

```
public/guide: error public-conflict: The page /guide/ is written to `guide/index.html`: `guide` is a directory of the output, and cannot be a file of `public/`
```

## Shell code in phase 2

Shell code is **executed once per pathname, at build time** — each page in a
runtime of its own. The render bundle is compiled once and loaded anew for
every pathname, so nothing a page leaves behind is there for the next: a
module's top-level state, a global, a patched prototype.

```ts
const seen = new Map<string, number>();          // slug.ts, imported by every page
export const slug = (t: string) => { const n = seen.get(t) ?? 0; seen.set(t, n + 1); return n ? `${t}-${n}` : t; };
```

```html
<h2 id="install">   <!-- on / and on /guide/, in whatever order they are built -->
```

A page's bytes depend on its own source alone; adding a page does not change
another. **Rejected:** one runtime per build (the first implementation:
`/guide/` got `id="install-1"` because `/` was rendered before it);
resetting only the builder's own state between pages (the record, the id
counters: a module's state is not the builder's to find).

That settles how layout.md's shell rules read for now:

| | layout.md | Phase 2 |
| --- | --- | --- |
| S2 no variance | no `Match`, `Switch`, `Each`, `?:`, `.map()` | **no *runtime* variance.** Anything computed while the page is executed is a compile-time value — loops over constants, conditionals on props and on the pathname. Nothing conditional is left *in the output* |
| S3 compile-time values | literals, constants, props | whatever execution yields; there is nothing else to read |
| S4 deterministic | no `Date`, `Math.random`, environment or I/O | enforced by the engine: the clock and the dice throw, the time zone is UTC on every machine, the collector cannot be watched, and a page has thirty seconds and one gibibyte (*The engine*) |
| S1 no React runtime | no hooks, state, effects, refs, context, portals, `Suspense`, no event-handler props | no handlers, state, effects or refs, and nothing that suspends (below) — in the project's code and in a package's alike. Pure render-time React — components, `children`, `Children`, `cloneElement`, `useMemo`, `useId`'s replacement, and **context** (`createContext`, `useContext`, `use` of one): it is resolved while the page executes — is the same code React runs, on React's own elements |
| S5 containers unlooped | | folded into S2 |

This is the one reading under which the design system's own components
(`Each` over slot entries, a footer only when `$Action` is filled) are legal
shell code with no special case, and under which an author may loop over a
constant — to produce elements, not slot elements: `Each` or `.map()` around
`<$Item>` is still orphan-slot (phase01/syntax.md, *Placement*).

> OPEN (for the owner, decisions.md): loop-produced slot items are "phase 2"
> in CLAUDE.md and in syntax.md's roadmap note, and no task of plan.md has
> them. Until then a `SideMenu`'s items are written out, one `<$Item>` each.

**Rejected:** S2 absolute with the design system exempt — a third
kind of code, and `NAV.map(…)` over a constant is deterministic, so
forbidding it is a fake constraint.

| Code | Message | How it is found |
| --- | --- | --- |
| shell-handler | The shell cannot handle events: `onClick` on `<button>` | a function-valued prop on a host element, when the element is created — by JSX, `createElement` or `cloneElement`, by the project or by a package |
| shell-react | The shell cannot use React state or effects: `useState` | `useState`, `useReducer`, `useEffect`, `useLayoutEffect`, `useInsertionEffect`, `useRef`, `useImperativeHandle`, `useSyncExternalStore`, `useTransition`, `useDeferredValue`, `useOptimistic`, `useActionState` throw when a page **calls** them — however the hook was reached: imported, re-exported, renamed, `React.useState`, `React["useState"]`, `const { useRef } = React`, in a compiled package. And a class component with an effect — `componentDidMount`, `componentDidUpdate`, `componentWillUnmount`, `getSnapshotBeforeUpdate` — when its element is made: *`componentDidMount` of `Ticker`*. A class that only renders is a component like any other: its state is a constant, and an error boundary catches nothing in a static render |
| | The shell cannot suspend: `<Suspense>` | what waits, and what would hide an error (*Nothing swallows an error*): an element of `Suspense` or of a `lazy` component, when it is made — *a `lazy` component*; `use` of a promise — *`use` of a promise*; a component that returns one — *`Feed` is an async component* |
| shell-nondeterministic | `Date.now()` makes the shell irreproducible | `Date.now`, `new Date()` without arguments, `Date()`, `Math.random`, `crypto.getRandomValues`, `crypto.randomUUID`, `performance.now` throw in the engine; so does a date string that is not ISO 8601 — *A date string that is not ISO 8601 ("Oct 4 2026") makes the shell irreproducible*. `new Date(2026, 9, 4)` is a compile-time value: midnight, UTC |
| shell-error | the message of the exception, after its name when it is not a plain `Error`: `TypeError: cannot read property 'map' of undefined` | anything else thrown while a page renders. Among it, what the engine itself ends: `InternalError: stack overflow` (a call depth of 10 000), a page that does not finish in 30 s — *Rendering did not end in 30s: a loop without an end?* — or takes more than 1 GiB — *Rendering took more than 1024 MiB of memory: a loop that keeps what it makes?*; and what the engine lacks — *Number.prototype.toLocaleString() needs Intl, which the builder's engine does not have* |
| shell-console (warning) | `console.log: ` and what was printed | `console` is collected: nobody reads the console of a build. At the call |

Every one is found by **executing** the page, and ends it: one error per
page, the first. A hook in a module that a page imports and does not render
— a barrel that also exports an island's component — is not an error: as for
the record, what counts is what ran, not the import graph. **Rejected:**
shell-react from the imports of the pages' module closure — it missed
re-exports, `const { useRef } = React` and compiled packages, and failed
pages that render none of it.

**Nothing swallows an error.** React's static renderer renders the fallback
of a `<Suspense>` boundary for whatever its content throws, and tells
nobody: every rule above, broken inside a boundary, would build a page
without what the boundary wraps.

```tsx
<Suspense fallback={<p>Loading</p>}>
  <h1>Changelog</h1>
  <Clock />                 {/* Date.now(): shell-nondeterministic */}
</Suspense>
```

```
React alone:    <p>Loading</p>, exit 0
the builder:    pages/changelog/index.rtsx(10,9): error shell-react: The shell cannot suspend: `<Suspense>`
```

So there is no boundary in the shell — with or without anything in it that
suspends — and nothing to wait for: `lazy`, `use` of a promise, an `async`
component. A `lazy` component is an error where it is rendered, not where
`lazy()` makes it: an island's, exported from the same barrel, is nobody's
mistake. And a boundary that was made past the builder's runtime — an
element written out as an object, by a package with a JSX runtime of its
own — still hides nothing: a render that ends after a component threw is
that component's error, the first one.

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
| not there (a `ReferenceError`) | `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`, `structuredClone`, `atob`, `btoa`, `queueMicrotask`, `setTimeout`, `setInterval`, `fetch`, `Temporal`, `process`, `require`; no `Array.fromAsync`. `WeakRef` and `FinalizationRegistry`: when an object is freed is the collector's business, not the page's |
| a runtime per page | the bundle's bytecode, loaded anew: 2.9 ms a page on the fixture site (27.6 from its text). A page has the timeout (30 s) and 1 GiB; the bundle's own loading, the same |
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
| `pathname()` | the route being rendered: from the site's root, whatever `--base` | `location.pathname` |
| `useShellId(p)` | `p` + a counter per prefix, per page; no prefix is the prefix `r`: `r1`, `r2`. A prefix that ends in a digit is shell-error — the counter follows it, and the first `d1` would be the eleventh `d` | `React.useId()` |
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
- By name — the function's own, whatever the bundler renamed it to
  (`KeepNames`): `function Page` of two modules is one entry, so the count
  is of a name, not of a module's export. It is for the report. A `memo` and a
  `forwardRef` are counted by the function they wrap; an element made by
  hand or by a package counts as any other. A class is rendered and not
  counted (one with an effect is shell-react).
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

| | Rule |
| --- | --- |
| an id | compared as written (case-sensitive); `id=""` is none |
| one id | `commandfor`, `popovertarget`, `anchor`, `for` on a `<label>`; empty, it names nothing: reported |
| a list of ids | `aria-labelledby`, `aria-describedby`, `aria-controls`, `for` on an `<output>`: space-separated, each checked; an empty list is no mistake |
| `href="#x"` | an id — as written or percent-decoded — or an `<a name>`. `#`, `#top` and a text directive (`#:~:text=…`) are not references |
| a command | the six keywords, case-insensitive; a custom command (`--x`) is the page's own. A keyword without `commandfor` commands nothing: command-target |
| a missing target | idref-not-found alone, not command-target as well |
| a root-relative link | `href` of any element but `<base>`, starting with one `/`. It is a route — `/guide/`, also written `/guide` (the host redirects) or `/guide/index.html` (the file) — or a file of the output: `/favicon.svg`, `/demo/` for `demo/index.html` |
| its path | what a browser requests, in the names of the site's directories: tabs and line breaks dropped, `\` a `/`, dot segments resolved (also `%2e`), then each segment percent-decoded (`/se%C3%B1or/` is `pages/señor/`). Not Go's `url.Parse`: `/guide//` is not `/guide/`, `%2F` is not a separator, and a `%` that encodes nothing stands for itself — `/100%` is checked, not passed |
| not checked | a URL with a scheme or a host (`https:`, `mailto:`, `//host`, `/\host`); a relative one (`slot/`, `../x/`, `?tab=2`) |
| `--base /docs/` | not seen by the check. A page links to the site from its root, whatever the base — `/guide/`, as `pathname()` names it — and is checked as rendered, before packaging prefixes the link (*Packaging*). So `/docs/guide/` is link-not-found: it is no route, and would be served as `/docs/docs/guide/` |
| `<template>` | its content is not of the page: neither its ids nor its references. The element itself is: its `id` counts, here and for mount-no-element |
| `<noscript>` | the same: to a browser that runs scripts — the ones a behaviour is for — its content is text. However the page was parsed |
| repeated | one report per page for the same attribute on the same element name: a link of the layout is on every item of a list |

They are reported at the page (file and pathname) with the offending
attribute's text; mapping an attribute back to its `.rtsx` position needs
provenance the renderer does not carry (*Not in phase 2*).

```
pages/guide/index.rtsx: error idref-not-found: Page /guide/: `commandfor="install"` on `<button>` names no element of the page
pages/guide/index.rtsx: error command-target: Page /guide/: `command="show-modal"` on `<button>` needs a `<dialog>`: `commandfor="nav"` is a `<nav>`
pages/guide/index.rtsx: error link-not-found: Page /guide/: `href="/guide/slot/"` on `<a>` is neither a page nor a file of the output
```

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
page entry ─ esbuild ─▶ flat CSS ─ prune against the page as served, and its script ─▶ esbuild minify ─▶ the page's CSS
```

The page is the page **as it is served**: the file that is written, not what
React rendered. Packaging changes two things a rule can select on
(*Packaging*): the links get the base, and the page gets a `<style>` or a
`<link>`, and a `<script>`.

```css
/* --base /docs/: the page's link is written <a href="/docs/guide/"> */
a[href^="/docs/"] { … }      /* kept: it matches the file that is served        */
a[href="/guide/"] { … }      /* dropped: it matched what was rendered, no more */
style, script { display: block }   /* kept, each if packaging wrote one        */
```

Which element, and under which URL, depends on the blob — on the pruned
sheet itself. So the builder guesses (the sheet is inlined), prunes, packages
the site, and prunes again each page whose file is another than the one it
was pruned against, until none is: a round or two more, for a page whose
sheet is a file. A page that does not settle in 8 rounds — a rule selects on
the hash in its stylesheet's own URL, and the hash changes with the rule —
gets its whole sheet, and the report says so.

**The builder's own elements.** The `<style>` or `<link>` and the `<script>`
that packaging writes are elements of the page like any other — a rule may
select them — and nothing more. The driver tells the pruner which they are:

| The builder's | Is not | An author's is |
| --- | --- | --- |
| `<script type="module">`, inline or `src` | a script of the page's own: its text is the page's built script, which the pruner reads (*The page's script*) | a script of the page's own: the page is not pruned |
| `<link rel="stylesheet">` to the page's sheet | a stylesheet the builder did not bundle: it is the sheet that is pruned | one: every custom property, `@keyframes` and `@position-try` stays |
| `<style>` with the page's sheet | read for the names it uses: a sheet is not pruned against itself | read: what it names stays |

```html
<!-- /dialog/ as served: pruned — the one <script> is the builder's -->
…<button command="show-modal" commandfor="d1">Open</button>…<script type="module" src="/_rg/page-8a8c3c5b.js"></script></body>
<!-- /theme/: not pruned — "the page has a script of its own" -->
…<script src="/theme.js"></script>…<script type="module" src="/_rg/page-8a8c3c5b.js"></script></body>
```

**Rejected:** pruning against the page before packaging, with the script
beside it (the reconcile's first form: `script { display: block }` and
`a[href^="/docs/"]` are then matched against a page that is not the one
served); taking every `<script>` of the served page for the page's own (the
two fixes together, unreconciled: every page with a behaviour came back
unpruned).

**CSS is not rewritten for the base**: a selector on `href` is matched
against the link as served. `a[href="/guide/"]` selects nothing under
`--base /docs/`, in any build; write what holds under every base
(`a[href$="/guide/"]`), or mark the link with a class.

The first step lowers nesting and does not minify: esbuild then writes
`/* path/to/file.css */` before each file's rules, which is where the
report's numbers per source file come from (*The report*). The pruner reads
either form.

The first step is **one esbuild build for the site**: every page's module is
an entry, and esbuild gives each entry the CSS its modules import, in import
order (the JS it makes of them is thrown away).

| | In the CSS build |
| --- | --- |
| the modules | the program's, as in the render bundle: its resolutions, its texts (*One resolver*) |
| what is in a page's sheet | every stylesheet a module of the page imports — also of a module the page uses nothing of: a package's `sideEffects` is not asked. `import { Button } from "@reactogenic/ui"` brings the whole design system's CSS, and pruning takes out what the page has no element for |
| `react`, `react-dom` | not read: they hold no stylesheet |
| `url(…)` | left as written: nothing is processed (*Not in phase 2*). Write it from the root, to a file of `public/` |

| Code | Condition |
| --- | --- |
| css-bundle | what esbuild cannot make a sheet of: a stylesheet that is not there, at the import that names it (TypeScript does not say so — `*.css` is declared — and the render bundle reads no CSS); CSS that does not read |
| css-warning (warning) | what esbuild says of a stylesheet, at its position: `"widht" is not a known CSS property`, with its note (`Did you mean "width" instead?`) |

| | Rule |
| --- | --- |
| a selector | kept iff some element of the page **may** match. Type, class, id and static attributes are matched exactly; combinators against the real tree |
| runtime state is "maybe" | every pseudo-class except `:root`, `:is()`, `:where()`, `:not()`, `:has()` (which are evaluated on their arguments), a `&` that lowering left, and every attribute selector on `open`, `hidden`, `inert`, `disabled`, `checked`, `selected`, `value`, `style`, `aria-*`, `data-state` — the one list of what the browser and a behaviour write without saying. The negation of "maybe" is "maybe" |
| what the page's script names | "maybe" too (*The page's script*): a class, an id, an attribute; a custom property or an animation it names is read. A script that changes the tree: the page is not pruned |
| case | as HTML: tags and attribute names fold, classes and ids do not — the page starts with `<!doctype html>`, as the builder writes it. Given a page without exactly that doctype, which may be in quirks mode, classes and ids fold too; attribute values are case-sensitive except with the `i` flag and for the attributes HTML compares case-insensitively (`type`, `rel`, `lang`, `dir`, `method`, …) |
| pseudo-elements | ignored for matching (`::backdrop`, `::before`, `::details-content`). Whatever follows one — `::before:hover`, `::before::marker` — is "maybe", and never safe to drop on its own (*Lists*) |
| a selector list | pruned per selector; the rule goes when none is left. **Kept whole** if a selector that would go is one some browser of the floor may reject (*Lists*). Only the rule's own list: the arguments of `:is()`, `:where()`, `:not()`, `:has()` are never trimmed — `:is(.btn, #never)` has the specificity of `#never`, matched or not |
| a selector the pruner cannot parse | kept, and its list with it, untouched (a namespace, a column combinator, a comment inside, a combinator after a pseudo-element): what does not parse may not be a selector at all |
| `@media`, `@supports`, `@container`, `@scope`, `@starting-style` | pruned inside; dropped when empty. Inside `@scope` a selector is matched against the whole page (the scope's limits are not evaluated), and what esbuild leaves there is kept as it is: a rule that still nests, a declaration between the rules |
| `@layer` | a block is pruned inside; an emptied block stays as `@layer name;` unless something before it already orders that layer. **Dropping it would reorder the cascade** (*Layers*). A block whose prelude is not one layer name (`@layer a, b { … }`) is no layer: kept as it is |
| `@keyframes` | kept iff its name occurs as a word in a kept `animation` or `animation-name` value, in a kept custom property's value (`--a: spin` for `animation: var(--a)`), in a `style` attribute or `<style>` element of the page, or in the page's script |
| `@position-try --x` | kept iff `--x` occurs elsewhere, as for a custom property: in a kept `position-try-fallbacks` or `position-try`, in a kept custom property's value, in a `style` attribute or `<style>` element of the page, or in the page's script. Its own prelude does not count. One whose prelude is not one dashed name is kept as it is |
| custom properties | a `--x` declaration is dropped iff `--x` occurs nowhere else: in no kept CSS (values, at-rule preludes, `@property`, `@keyframes`), in no attribute or `<style>` element of the page (a `style`, SVG's `stroke="var(--x)"`) and not in the page's script (`getPropertyValue("--x")`). Its own value does not count (`--x: var(--x)`). Repeated to a fixed point, with the at-rules it empties |
| a stylesheet the builder did not bundle | `<link rel="stylesheet">` to a file of `public/`, an `@import` in a `<style>` or left in the sheet: it may read any custom property and name any animation or `@position-try`, and nothing here sees it. Every custom property, every `@keyframes` and every `@position-try` stays; rules are pruned as ever. The `<link>` or `<style>` packaging writes for the page's own sheet is not one (*The builder's own elements*) |
| `@font-face`, `@property`, `@import`, `@namespace`, anything unknown | kept, byte for byte |
| an `@import` or `@namespace` after a rule | everything before it is kept, byte for byte (*Out of place*) |
| a page with a script of its own | not pruned: a `<script>` that runs — not a data block (`application/ld+json`, an import map), not an empty one — an `on…` attribute, a `javascript:` URL. What it writes is not known: `/theme.js` adding `dark` to the root would find `.dark .a` gone. The report says so (*The report*). The `<script>` packaging writes for the page's behaviours is not one: it is the script the pruner reads (*The builder's own elements*) |
| a page that contains `<template>`, `<noscript>`, `<selectedcontent>`, a `<select>` holding more than `<option>`, `<optgroup>`, `<hr>` and text, or an element with `contenteditable` (any value but `false`) | not pruned: the tree a browser matches against is not the one written (the design system emits none). Cloned content would need "maybe" relations — a template's wherever a script puts it, the selected `<option>`'s inside `<selectedcontent>` at load; `<noscript>` is elements or text depending on the browser; markup inside a `<select>` is kept by a parser that knows the customizable select and dropped by one that does not, where `<select><div><option>` makes `select > option` match; and what the user edits gets elements no script wrote — Bold a `<b>`, Enter a `<div>`, a paste whatever was copied — which `.editor b` then matches. The report says which (*The report*) |
| CSS that does not read | an error, and nothing is pruned: an unbalanced bracket, an unterminated string or comment |

What is kept is kept byte for byte, in order: nothing is rewritten, so the
result can be minified again.

**Lists.** One selector a browser cannot parse makes it drop the whole rule,
so trimming a list around such a selector would bring a dead rule to life
there:

```css
.a, .gone:open { … }      /* Safari 26.2 has no :open: the rule is dead there */
.a { … }                  /* trimmed: now it applies — not what was written    */
```

A selector is safe to drop on its own only if every browser of the floor
(decisions.md, 11) parses it: the pseudo-classes and pseudo-elements that
have been everywhere for years, `:nth-*(An+B)`, `:is()`, `:not()`, `:has()`.
Anything else — prefixed, newer, the `s` flag — goes with its whole list or
not at all.

So does anything that goes on after a pseudo-element. The grammar allows a
pseudo-class there, but which pairs are valid is each browser's own list:
Chromium and WebKit reject `::before:hover`, `::backdrop:first-child`,
`::before::marker` and most others, and esbuild prints them all without a
warning.

```css
.Other::after:hover, .Btn { … }   /* dead in Chromium and WebKit            */
.Btn { … }                        /* trimmed: alive — so it is kept whole   */
.Other::after > b, .Btn { … }     /* not a selector anywhere: kept as it is */
```

**The page's script.** A behaviour changes the page after it has loaded:
what it writes is state the HTML does not show. The pruner is given the
page's built script, and a name that is in it never decides anything.

```js
function mount(root) {                       // the page's script, minified or not
  root.classList.add("is-open");             //   .is-open            "maybe"
  root.tabIndex = 0;                         //   [tabindex="0"]      "maybe": the property reflects the attribute
  root.dataset.placement = "top";            //   [data-placement]    "maybe"
  root.setAttribute("data-side", "left");    //   [data-side=left]    "maybe"
  getComputedStyle(root).getPropertyValue("--rg-breakpoint");   // the property is read
}
```

| | |
| --- | --- |
| a name | a word of the script's text — an identifier, a property, a word of a string — whatever it is there: `dark` of `classList.add("dark")` and a variable `dark`. More is "maybe" than is written; never less |
| a class, an id | "maybe" when it is a word of the script, whatever its case; a name that is no word (`sm:flex`), when the text holds it |
| an attribute | "maybe" when the script names it as it is written (`data-side`), as the property that reflects it (`tabIndex`, `htmlFor`; `className` and `classList` for `class`), or as `dataset`'s key (`fooBar` for `data-foo-bar`) |
| the tree | a script that names an API that adds, moves or removes an element — `createElement`, `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `cloneNode`, `appendChild`, `insertBefore`, `replaceChildren`, `replaceWith`, `append`, `prepend`, `before`, `after`, `remove`, … — changes what stands next to what: the page is not pruned |
| not seen | a name the script computes (`"is-" + state`): a behaviour names what it writes, in full (*Behaviours*, what a behaviour writes) |

The three behaviours of `@reactogenic/ui` write nothing to the page and
name no such API (a test of the pruner builds them and says so).

**Rejected:** a rule alone — "a behaviour writes only the attributes that
are *maybe*" — with nothing that reads the script (the first draft:
`mount()` takes any module, and `item.tabIndex = 0` in one silently lost
`.menu [tabindex="0"]`); a check of each module's writes (`classList`,
`setAttribute` with a literal, …: what is written is known only with types,
and a property that reflects an attribute looks like any other).

**Layers.** A layer's place in the cascade is where its name first occurs.

```css
/* the bundle */                          /* a page without .gone */
@layer reset { .gone { … } }              @layer reset;
@layer components { .a { color: red } }   @layer components { .a { color: red } }
@layer reset { .a { color: blue } }       @layer reset { .a { color: blue } }
```

Without the statement `components` would come first, and `.a` turn blue.

| | |
| --- | --- |
| already ordered | by an earlier `@layer a, b;`, an earlier block that is kept, or the statement an earlier emptied block left |
| a prelude that is not layer names | a browser ignores the rule, so it orders nothing: `@layer a, 1;` does not order `a`, and `@layer a, b { … }` is kept as it is — as a statement it would order `a` and `b`, which the block never did. A name is identifiers joined by dots, none of them a CSS-wide keyword |
| nested | `@layer a { @layer b { … } }` is `a.b`; `a.b` orders `a` too |
| inside `@media` and the other conditional at-rules | what is declared there is ordered only when the condition holds: it does not count outside, and the at-rule stays to carry the statement — `@media print { @layer p; }` |
| anonymous (`@layer { … }`) | dropped once it holds no rule: nothing can name it again |

**Out of place.** A browser ignores an `@import` or a `@namespace` that
follows a rule — and reads it once the rule is gone, or once `@layer a;`
stands where a block was, since a statement may precede both.

```css
/* the bundle */                  /* with the statement */
@layer a { .gone { … } }          @layer a;
@namespace url(…/2000/svg);       @namespace url(…/2000/svg);   /* read now: the default is SVG */
p { color: red }                  p { color: red }              /* … and matches no <p>         */

.gone { … }                       /* the same with a rule dropped */
@import url(theme.css);           @import url(theme.css);       /* loads now */
```

So when an `@import` or `@namespace` has anything before it but `@charset`,
`@import`, `@namespace`, `@layer` statements and comments, the sheet up to
the last such rule is kept as it is. It takes an authoring mistake; esbuild
passes it through with a warning (`All "@import" rules must come first`).

Soundness is the requirement: a dropped rule must never have matched. The
pruner is tested against an independent selector matcher — which knows
nothing of HTML's case rules, of SVG's names or of `<noscript>`, so those
rows have tests of their own, and the tests' cases were run in a browser
once (computed styles before and after pruning: the package's doc has the
engines and the count) — and the built docs site by the same comparison
(pruned vs unpruned CSS, every page; plan.md). The comparison cannot see a selector wrongly
taken for valid — it only shows on a page that lacks the rest of the list —
so the tables of what every browser parses are probed apart (plan.md,
RGP2-050).

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
  // the flag: is the code in the page; the attribute: did this menu ask
  if (RG_MENU_TYPEAHEAD && root.hasAttribute("data-typeahead")) root.addEventListener("keydown", onType);
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
import m0 from "@reactogenic/ui/behaviors/menu-keys";
import m1 from "@reactogenic/ui/behaviors/overlays";
m0(document.getElementById("m1"));
m0(document.getElementById("m2"));
m1();
```

| | Rule |
| --- | --- |
| module | the default export is `(root: HTMLElement) => void`, or `() => void` for a page-level behaviour (mounted without an id; run once per page however often it is mounted). One or the other: a module that a page mounts both ways is mount-kind — its page-level call would hand the function no root |
| the entry | one import per distinct module, in the order they are first mounted, resolved by esbuild from the project directory (through a symbolic link too: files are named from the real one); one call per mount, in render order. One module on one element twice is one call. A module is the **file** it resolves to: two spellings of it are one import, and one page-level call |
| flags | bare `declare const RG_…: boolean` identifiers — never an options object, an imported constant or a class member: esbuild removes code only on parser-time constants (research.md) |
| a flag | an identifier that starts with `RG_` and that the module reads **free** — nothing binds it; `declare` binds nothing. Exactly what `Define` replaces, so esbuild is asked: each file is transformed with every `RG_…` name of its text defined as a marker, and the names whose marker comes out are its flags. `RG_lower`, `RG_$` are flags; a name in a comment, a string, a property, or `const RG_X = false` of the module's own is not |
| a page's flags | one `Define` map per page: the **union** over the page's mounts. A flag's name is the page's, not a module's — on, in every module that reads it, once one mount turned it on; two modules that read one name share it. So a flag gates **code**, never a use site: whether a root uses a feature is an attribute the component emits and the behaviour reads at run time (`data-typeahead`, above). A root behaves the same whatever else is mounted on the page — and the same under `--no-specialize` |
| every flag is defined | the builder defines each flag of the module and of every module it imports — `false` unless a mount set it. A flag the builder did not see would be a `ReferenceError` at run time, so the built script is asked the same question: a free `RG_…` left in it (one written `RG_\u0041` in the source) is mount-flag, and no script |
| no top-level side effects | the builder **reads** each module once per site: it bundles `import "<module>"` alone — nothing of it used — ignoring `sideEffects` and `@__PURE__` annotations. Whatever is left in that output ran at the top level: mount-side-effect. State and constants are not code that runs (`let typed = ""`, `new WeakMap()`, a class, an enum); a call is (`["a", "b"].join(",")`, `matchMedia(…)`, `"command" in HTMLButtonElement.prototype`) — it belongs in the function. The same read resolves the module and lists the files whose flags are its own |
| … of a module with flags | read as a page builds it — every flag defined: once all on, once all off, and what is left of either counts. `const DELAY = RG_SLOW ? 500 : 100` is a constant; `if (RG_X) document.title = "x"` runs. What runs only under a mix (`RG_A && !RG_B`) is not seen |
| what a behaviour writes | state, and it **names** it: the attributes that are "maybe" (*CSS*), and any class, id or attribute written by its name in full — a literal, not `"is-" + state`. The pruner reads the page's script for those names (*CSS*, *The page's script*); a name it cannot read is a rule it may drop. No element is added, moved or removed: a script that does leaves its page unpruned. The same for a custom property or an animation it uses: named in full |
| build | one `api.Build` per page: generated entry, `Bundle`, minify, ES modules, `Define` = the page's flags, `Metafile`. No splitting. Every input of its metafile is a file some mounted module reaches |

The `(root)` signature is layout.md's `mountX(root, …)` for clones opened
from islands. That much carries over, and no more: the generated entry
mounts by id when the page loads (a clone exists only after `open`, with ids
made per instance); a behaviour returns nothing (layout.md's handle has
`update` and `close`); and the record is of shell code — what an island
mounts happens at run time (*Not in phase 2*).

| Code | Condition |
| --- | --- |
| mount-not-found | the module of a `mount()` does not resolve |
| mount-no-element | the id of a `mount()` is not on the page (a `<template>`'s content is not, nor a `<noscript>`'s) |
| mount-kind | a module is mounted on an element by one `mount()` of the page and without an id by another — however the two spell it. Per page: one that is of an element on `/` and of the page on `/guide/` is not seen, each page being built on its own |
| mount-flag | a flag passed to `mount()` — on or off — that is no flag of the module or of a module it imports; a flag the page's script reads and the builder did not find |
| mount-side-effect | a module, or one it imports, runs code when it is imported; reported at that file, with the statement that stays, once per file however the mounts spell the module |
| mount-error | anything else esbuild says of a module — an import of its own that does not resolve, no default export: its message, at its position |

The first four are reported as the page checks are — at the page, naming
the mount: ``Page /syntax/: `mount("@reactogenic/ui/behaviors/menu-keys")`:
no element of the page has `id="m9"` ``.

## Packaging

Per page: its HTML, its pruned CSS, its JS. Identical CSS or JS on several
pages is **one blob**, by content hash.

| `--inline` | A blob is… |
| --- | --- |
| `auto` | a file when it serves two or more pages **and** is over 250 B gzipped; otherwise inlined |
| `always` | inlined: one request per page |
| `never` | a file |

- Files: `<out>/_rg/<name>-<hash>.css` / `.js`, the hash from the content
  (SHA-256, eight hex digits), so they can be cached forever. The name is
  `page` — `site` for the control's two files — and never a page's own: a
  blob's URL depends on its content alone, so what one page ships does not
  change another page's bytes.
- Inlined: `<style>` at the end of `<head>`; `<script type="module">` at the
  end of `<body>`. A linked script is `<script type="module" src>`; a linked
  stylesheet `<link rel="stylesheet">`.
- A page with no behaviours has no `<script>`. A page whose CSS is empty has
  no `<style>`.
- The page is React's HTML, to the byte, but for what packaging writes into
  it: the two elements above, and the base. It is not parsed and serialised
  again, and nothing lower-cases an attribute: `popoverTarget="m1"` is
  `popovertarget` to every HTML parser, the page checks' among them.
- A blob that cannot stand inside its element is a file, whatever `--inline`
  says: CSS that holds `</style`, a script that holds `</script` or `<!--`
  (esbuild escapes the first two in what it prints; `<!--` it does not).
- A site behind a `Content-Security-Policy` without `'unsafe-inline'` is
  built with `--inline never`: no inline `<style>` or `<script>` is written.
  Hashes of the inlined blobs for the policy (`sha256-…`, per page) are *Not
  in phase 2*.

```html
<!-- /guide/, --inline auto: its CSS is /guide/more/'s too, and over 250 B gzipped; it mounts nothing -->
<!doctype html><html lang="en"><head>…<link rel="stylesheet" href="/_rg/page-e28e8351.css"></head><body>…</body></html>
<!-- /dialog/: its CSS and its script are its own -->
<!doctype html><html lang="en"><head>…<style>…</style></head><body>…<script type="module">…</script></body></html>
```

**`auto`.** The rule is about caching, not size. A file costs a request —
≈180–330 B of headers; 250 B in the research's tables — so a blob of one
page is always cheaper inline, and a shared one is cheaper as a file once a
visitor opens a second page that has it: `(k − 1) × S > 250` for the `k`
pages of a visit (research/js-behaviours.md), taken at `k = 2` — the builder
knows the site's pages, not a visitor's.

```
a script that is `overlays` alone (252 B raw, under 250 B gzipped), on all four pages     inlined in each: the request costs more than the copy
the layout's CSS (over 250 B gzipped), on all four pages                                  /_rg/page-<hash>.css, once
```

- `S` is the blob alone, gzipped by the builder (`compress/gzip`, level 9):
  the size a build can measure, and the one the report prints. The
  research's tables are in brotli, which Go's standard library does not
  have.
- Sharing couples pages: when a change makes one page's CSS equal to
  another's, the blob they now share may become a file, and the other
  page's HTML changes with it. A page's own bytes — its HTML as rendered,
  its CSS, its script — depend on no other page; how they are delivered
  does (plan.md, T4).

**Rejected:** `(pages − 1) × bytes > 1024` (the first draft: no unit, and
`pages` was the site's — the 176 B of `overlays` were a file on a 200-page
site and inline on a 4-page one, for the same visitor).

**`--base /docs/`.** The URLs of the files above carry it, and so does every
root-relative `href` of the page — the links the page check read, after it
read them: `/guide/` → `/docs/guide/`. The source is written once, for any
base. `//host` and a URL with a scheme are left alone: a link to another
site of the origin is written in full.

- One rule for both: what the check takes for a root-relative link is what
  gets the base — `<base href>` is neither. The rest of the attribute
  stays as written: `/guide/more/?from=plain#top` →
  `/docs/guide/more/?from=plain#top`, `/guide` → `/docs/guide`.
- `public/` is under the base because the site is: its files are written
  where they were (`<out>/favicon.svg`, served as `/docs/favicon.svg`).
- CSS is not rewritten for it: a selector on `href` is matched against the
  link as served (*CSS*).

> OPEN: the other root-relative URLs of a page (`src`, `srcset`, `action`,
> `url()` in CSS) get the base by the same rule; which attributes, when the
> asset pipeline is specified (*Not in phase 2*).

> OPEN (for the owner, decisions.md): what islands need from the builder —
> `pathname()` under a base in React (`location.pathname` carries it), the
> record, the mount entry, pruning (*Not in phase 2*, the first row).

**Rejected:** the base written into the links by the author (the source is
then built for one base, and shell code has no way to learn it: `pathname()`
is the route); checking links with the base stripped and passing those
outside it as "another site of the origin" (with links written from the
root, that is every link: the check is off whenever `--base` is set).

## The report

`<out>/_rg/report.json`, and `--report` prints it: per page, the bytes of
HTML, CSS and JS (raw and gzip — brotli is the bench's, plan.md RGP2-050: Go
has none), how each blob is delivered, the
components rendered, the behaviours mounted with their flags, and per
behaviour module its bytes in the page's script (from esbuild's metafile) —
*why is this byte here*. The rows add up to the script: the mounted modules,
the files they import, the generated entry (`<entry>`), and what is of no
input — the helpers esbuild adds for a dynamic `import()` — as `<runtime>`.
CSS: rules kept and dropped per source file; for a page that was not
pruned, why (*the page has a script of its own*).

```
/actions/  pages/actions/index.rtsx
                  raw     gzip
  HTML           1411      634
  CSS            3668     1237   inline
  JS             1420      695   inline
  document       6545     2455   actions/index.html
  components ActionsPage ×1, Button ×1, Dialog ×1, DropdownMenu ×1, Each ×1, Layout ×1, MenuItem ×3
  behaviours @reactogenic/ui/behaviors/overlays
             @reactogenic/ui/behaviors/menu-keys #actions RG_MENU_TYPEAHEAD=true
             @reactogenic/ui/behaviors/invokers
  JS bytes      194 B  @reactogenic/ui/src/behaviors/overlays.ts
                850 B  @reactogenic/ui/src/behaviors/menu-keys.ts
                330 B  @reactogenic/ui/src/behaviors/invokers.ts
                 46 B  <entry>  the mount calls
  CSS rules    1 kept,   0 dropped  @reactogenic/ui/src/tokens.css
               3 kept,   1 dropped  @reactogenic/ui/src/button.css
              12 kept,   1 dropped  @reactogenic/ui/src/dialog.css
               6 kept,   1 dropped  @reactogenic/ui/src/dropdown-menu.css
               0 kept,  22 dropped  @reactogenic/ui/src/side-menu.css
               5 kept,  10 dropped  site.css

css  3ed21ca5     3668     1237   inline                   /actions/
css  e28e8351     1074      502   _rg/page-e28e8351.css    /guide/ /guide/more/
```

| | |
| --- | --- |
| HTML, CSS, JS | the page alone — with its doctype and the base, without what packaging puts in it — and its two blobs. They do not depend on `--inline` |
| document | the file as written: what the request for the page carries. With a blob inlined, the page and the blob |
| a behaviour | a module and the element it is mounted on, once, however many components mounted it, with the flags any of them turned on |
| the last lines | the site's blobs: kind, hash, raw, gzip, `inline` or its file, the pages it serves |
| CSS rules | of the sheet before it is minified; a file whose rules all went is a row: that is the saving. A file's bytes (`bytesIn`, `bytesOut` in the JSON) are its rules', before and after pruning |
| a page that is not pruned | one line in place of the rows, with the reason: ``not pruned: the page has an element the user edits (`contenteditable`)`` (*CSS*) |
| files | the project's own, from the project directory: `site.css`. A package's, by the package and the path in it: `@reactogenic/ui/src/dialog.css` — whether the install put it in `node_modules/`, in pnpm's store or behind a link out of the project. Nothing in the report is the machine's: two builds of one input write the same report, on any checkout |
| a file of no package, outside the project | where it is, from the project directory: `../shared/tokens.css` |
| gzip | DEFLATE at level 9, as Go's `compress/gzip` writes it: 0–5 B above what `gzip -9` gives on a page's files, under 1%. The measuring scripts have the reference numbers |
| the control | no rows per module or per source file: its two files are the site's |
| brotli | not in the report: Go's standard library has no encoder, and the builder takes no dependency to count with. The measuring scripts have it (plan.md, RGP2-050) |

Two builds of the same input are the same bytes: the pages, the blobs and
their names, the report.

## The control

`--no-specialize` builds the same pages and decides nothing per page: every
page ships the whole site's CSS and the whole site's script, and the script
looks up in the browser what its page mounted. Same HTML, and

| | Default | `--no-specialize` |
| --- | --- | --- |
| CSS | per page, pruned against the page as served and its script | one file for the site: the unpruned bundle of everything any page imports |
| JS | per page: the modules it mounted, its flags | one file for the site: every behaviour mounted on any page, every flag on, and a table from pathname to that page's mounts |

```js
// the control's entry: every flag is defined `true`
import m0 from "@reactogenic/ui/behaviors/overlays";
import m1 from "@reactogenic/ui/behaviors/menu-keys";
const m = [m0, m1], t = {
  "/": [[0]],
  "/syntax/": [[0], [1, "m1"], [1, "m2"]],
};
for (const [i, id] of t[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/)?$/, "/")] || [])
  id ? m[i](document.getElementById(id)) : m[i]();
```

- The table's keys carry `--base` (`/docs/syntax/`), in the names of the
  directories: `location.pathname` is what the browser encodes of them
  (`/se%C3%B1or/` for `pages/señor/`), so it is decoded (a pathname that is
  not valid percent-encoding throws: it is no page of the site). The base
  is decoded for the keys too: `--base /caf%C3%A9/` and `--base /café/` are
  one directory, written into the links as given and keyed `/café/…`; a
  base that does not decode is a usage error. A page
  answers to `/syntax/`, `/syntax` and `/syntax/index.html` — as its own
  script does, which is in the page wherever it is served. A page that
  mounts nothing is not in the table.
- Every page gets both files — also a page that mounts nothing, or imports
  no stylesheet: the control does not know. They are packaged by the same
  rule (`--inline`), as `_rg/site-<hash>.css` and `.js`.
- The CSS goes through the same steps but the pruning: bundled, nesting
  lowered, minified.
- The mount-* reports are those of the default build, page by page.
- The table is the control's own cost: it grows with the site and is in
  every page's script. The measurement reports its bytes apart (plan.md,
  RGP2-050), so that T5 does not divide by it.

The difference between the two builds is what component awareness is worth
(plan.md, RGP2-050).

## Not in phase 2

| | |
| --- | --- |
| islands, `Dynamic`, `<template>` delivery, holes | layout.md stands, and the engine already renders what React renders. To be redone with them, not carried over: CSS pruning (what an island renders is not in the page's HTML, and a `<template>` turns pruning off for its page — research/css.md has the fallback: every rule of a component an island imports stays, a template's content matches as "maybe"); the generated entry and the record (*Behaviours*); shell-handler when an element is *made* (`<Dynamic><button onClick={…}>` is made in shell code) |
| a dev server, watch mode, HMR | out of scope |
| view transitions | out of scope |
| shell rules in `check` and the editor | `build` reports them: every one is found by executing the page |
| every violation of a page in one build | the first one ends the page |
| `Intl`, a time zone other than UTC | *The engine* |
| `.rtsx` positions for the page checks | needs attribute provenance through the renderer |
| Markdown, highlighted code samples | a second front end; samples are plain `<pre>` |
| source maps for the emitted JS | |
| the tsconfig's emit options in the render bundle | esbuild compiles a program file with its own defaults for `useDefineForClassFields`, `experimentalDecorators` and the like: shell code is function components |
| `async` components, `use()` of a promise, `lazy`, Suspense | shell-react: the renderer is one synchronous pass, the engine has no timers, and a boundary hides errors |
| hashes of inlined blobs for a `Content-Security-Policy` | `--inline never` needs none |
| a check of what a behaviour writes | the pruner reads the page's script for the names; a computed name is not seen (*CSS*, *The page's script*) |
| asset pipeline (images, fonts), `public/` | a `public` directory next to `pages` is copied as is — its files, a link to a file as the file; nothing is processed. A link to one of its files is a link to a file of the output (*Checks*); a `url()` in CSS is left as written |
