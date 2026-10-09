# The builder: `reactogenic build`

Pages in `.rtsx` — the variants of the routes — + framework-owned layout
components → per-page plain HTML, CSS and minimal raw JS. **No React in the output.** Why it is shaped this way:
[research.md](research.md).

Phase 2 has no dynamic segments and no `Dynamic`: every page is shell
([later/layout.md](../later/layout.md)).

```
reactogenic build [-p tsconfig.json] [--pages <dir>] [--out <dir>] [--base <path>]
                  [--inline auto|always|never] [--no-specialize] [--report]
```

| Flag | Default | |
| --- | --- | --- |
| `-p` | `tsconfig.json` in the working directory | the project, as for `check`: with the projects it `references` (*The project*) |
| `--pages` | `pages` next to the tsconfig | the root of the routes (*Routes*) |
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
   ├─ routes            pages/**/*.rtsx that nothing imports → the variants of each route
   ├─ render bundle     esbuild: every variant + React's static renderer, in memory
   ├─ execute           embedded engine, a runtime per page: render(variant) → HTML + record
   ├─ check the page    ids, references, commands, links
   ├─ JS                the record's behaviours → generated entry → esbuild with the page's flags     ┐ analysis:
   ├─ CSS               esbuild: the page's CSS in import order → pruned against the page as it is      ┘ per artifact, exact
   │                    served, and its script
   └─ package           dedupe by content, inline or file, write, report                                ← packaging
```

| Stage | Owner | |
| --- | --- | --- |
| program, diagnostics | phase 1 | `mapper.RegisterStrict`, `check.Program`: what `check` prints, `build` prints, and stops; the program of the project that lists the pages is kept (*The project*) |
| bundling, minifying, lowering | esbuild `pkg/api`, in-process, public API only | never forked; `Splitting` off; `Write: false` |
| execution | `modernc.org/quickjs` embedded in the binary | no Node at build time |
| serialisation | React's own `renderToStaticMarkup`, from the project's `react-dom` | shell HTML is what React renders for the same component, under `TZ=UTC` |
| everything component-aware | ours | the record, the checks, CSS pruning, behaviour selection, packaging |

### Analysis and packaging

Two things, kept apart (the owner's rules, 2026-10-06: decisions.md, K,
rules 1–3):

| | Analysis | Packaging |
| --- | --- | --- |
| asks | what does this artifact **need**? | how does what the artifacts need **reach the browser**? |
| unit | one artifact — a page — at a time | the site |
| answer | exact: the CSS rules that can match on that page (*CSS*); the behaviours it mounted, with its flags and each mount's data (*Behaviours*) | each piece inlined in the document, a file of the page's own, or factored into files that several artifacts share (*Packaging*) |
| for | CSS and JS alike | CSS and JS alike |
| may | — | make an artifact **fetch** what another needs: a shared file may hold more than one page uses |
| may not | take another page into account — a page's needs depend on its own source alone — nor become a site-wide union because packaging shares files | change what analysis says an artifact needs |
| in the report | `html`, `css`, `js`, with why: `styles`, `modules`, `mounts`, `classes` | `document`, `fetches`, each blob's delivery (*The report*) |

```
analysis    /guide/ needs    CSS: 69 of the bundle's 89 rules — these —  JS: overlays
            /syntax/ needs   CSS: 83 of them — these —  JS: overlays, menu-keys (RG_MENU_TYPEAHEAD on), invokers
packaging   today            each page's sheet and script in its document; a blob is shared only where two
                             pages' are the same bytes
            could be         the rules both need as one file and the rest in each page — or any other cut
```

As the builder packages today, **what a page fetches is what it needs**:
a blob is one page's own sheet or script, shared only where another page's
is the same bytes. How much further packaging should factor is open, with
the owner (*Packaging*, OPEN: the factoring policy) — and whatever is
decided there, the first column does not move.

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

The owner's rule (2026-10-05; decisions.md):

| | |
| --- | --- |
| a directory under `--pages` | a **route**: its pathname is the directory, with a trailing slash |
| an `.rtsx` file of it that nothing of the project mounts or imports | a **variant** of the route: built to a document — a page. `index.rtsx` is the one a static host serves at the pathname |
| any other file of it | a segment (mounted by `#name`, syntax.md) or a module of a variant |
| what the build writes | the **artifacts**: the output files — a variant's `.html`, the `.css` and `.js` under `_rg/` (*Packaging*) |
| its `server.ts` | the middleware that will select among the route's artifacts for a request. Out of scope: every variant is built, and the builder does not read `server.ts` — to the type checker it is a module like any other |

```
pages/index.rtsx                 /            a variant  → index.html
pages/guide/index.rtsx           /guide/      a variant  → guide/index.html
pages/guide/install.rtsx         mounted by index.rtsx as `#install`: a segment, no artifact of its own
pages/account/index.rtsx         /account/    a variant  → account/index.html
pages/account/guest.rtsx         /account/    a variant  → account/guest.html
pages/account/copy.ts            a module of the two
pages/account/server.ts          chooses between the two documents — not read yet
pages/gate/closed.rtsx           /gate/       a variant  → gate/closed.html: a route without an `index`
public/favicon.svg               /favicon.svg: copied as it is
```

| | |
| --- | --- |
| a variant is found | in the program's graph — never by a name, never by executing the file. A segment root `#name` is an import in the emitted TSX, so "no module of the program imports it" covers mounts |
| an import | any the program resolved, from any module that is not a declaration: `import`, `export … from`, `import()`, and one of types alone. A module that imports itself does not make itself a segment |
| a stray `.rtsx` file | a variant, and fails as one — a segment whose mount was deleted is page-not-document, which names it |
| only `.rtsx` | `index.tsx` makes no page; beside `index.rtsx` it is `check`'s ambiguous-module — an import of `./index` could mean either — which stops the build as any diagnostic does |
| the variant's name | the file's, without `.rtsx`: it names the document — `<out>/<pathname><name>.html`, preceded by `<!doctype html>` |
| `pathname()` | the route's, in every variant of it: `/account/` in `guest.rtsx` too |
| a document's path | where a document has to be named apart from its route — a diagnostic, the report, the control's table — it is the URL path a static host gives it: the pathname for `index` (`/account/`), the file for any other variant (`/account/guest.html`) |
| a route without `index.rtsx` | has no document a static host serves at its pathname; it is still a route: a link to `/gate/` is not link-not-found |
| a directory without a variant | holds modules, or nothing: it has no artifact, the route table does not have it, and a link to it is link-not-found |

- A variant is its module's **default export**, a component without props that
  renders the whole document, from `<html>`. A shared layout is an ordinary
  component (`<DocsLayout title="…">`), as in React.
- No route table file in phase 2: the table is derived, and still gives the
  check (*Checks*: link-not-found).

> OPEN: a file stops being a variant, silently, when anything of the project
> imports it — a test beside the page (`index.test.tsx`, if the tsconfig
> includes it), a second variant that borrows a component from the first, a
> `server.ts` that imports the variants it selects among. Its document is
> then not built, and nothing says so: a route may have no `index`.
> Recommended: a warning for an `index.rtsx` that is no variant.

**Rejected:** a variant is whatever renders `<html>` (decided by executing
every file; a mistake is silent); segments leave route directories
(syntax.md: a segment is the sibling of what mounts it); `index.tsx` as a
page (the first implementation: the rule names `.rtsx` files, and no other).

| Code | Condition | Reported at |
| --- | --- | --- |
| page-no-default | a variant has no default export that is a component: the module's `default` is not a function. For a variant other than `index`: *`parts.rtsx` is a variant of the route /guide/ — nothing mounts or imports it — and has no default export that is a component* | the file |
| page-not-document | a variant's root element is not `<html>`: what it rendered does not start with `<html`. For a variant other than `index` the message says what made it one: *`intro.rtsx` is a variant of the route /guide/ — nothing mounts or imports it — and its root element is not `<html>`* | the variant's `export default` — found in the syntax, so not one in a comment or a template; for `export { Page as default }`, the `Page as default` |
| pages-not-found | `--pages` is not a directory, or holds no `.rtsx` file — found before the project is checked: without one there is nothing to build; or every `.rtsx` file under it is mounted or imported by another module — found once the program is | |
| public-conflict | a file of `public/` — the directory next to `--pages` — is where the build writes one of its own: a variant's document (`index.html`, `account/guest.html`), anything under `_rg/`; or is a file where the build makes a directory (`public/guide` beside the route `/guide/`, `public/_rg`), or under what the build writes as a file (`public/index.html/x`). Found before anything is written | the file of `public/` |

```
public/guide: error public-conflict: The page /guide/ is written to `guide/index.html`: `guide` is a directory of the output, and cannot be a file of `public/`
public/account/guest.html: error public-conflict: The page /account/guest.html is written to `account/guest.html`: a file of `public/` cannot be there
```

## Shell code in phase 2

Shell code is **executed once per variant, at build time** — each page in a
runtime of its own. The render bundle is compiled once and loaded anew for
every variant, so nothing a page leaves behind is there for the next: a
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

### Variants

The owner's rule (decisions.md, 2026-10-05):

> Shell output has no browser-time variance. Every possible shell variant is
> materialized at build time. Runtime routing may select between those
> prebuilt variants, but does not render or modify them.

```
                 BUILD
                   │
          ┌────────┴────────┐
          ▼                 ▼
     public.html       private.html
          ▲                 ▲
          └────────┬────────┘
                   │
             SERVER ROUTER
                   │
          isAuthenticated()
```

| | |
| --- | --- |
| a variant | a module of the route that nothing imports (*Routes*), executed once, in a runtime of its own; its output is a file: `<out>/<pathname><name>.html` |
| today | the author writes each variant, a file of the route's directory: `index.rtsx`, `guest.rtsx`. Shell code reads nothing that differs between two requests — `pathname()` is the route's — so one module is one variant |
| who selects | the server's router, among files. There is no server in phase 2, and nothing in a built page chooses, renders or changes a shell |
| what the builder owes | every variant there is. A build that left one to be rendered at request time would break the rule |

> OPEN: how the router selects — the route's `server.ts`: its signature, how
> it names an artifact, the table the server reads (`route-table.md`). And
> whether shell code may read a dimension itself (`isAuthenticated()`), one
> module then yielding several variants: the record can keep that cost down
> — a page that never read a dimension has one file for all its values. Not
> designed: it arrives with the server.

That settles how layout.md's shell rules read:

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
| shell-script | The shell cannot run a script of the page's own: `<script src="/theme.js">` | the owner's rule (2026-10-05): a page with a script of its own is an error; whether page-author JS is ever allowed is deferred. Found on the page's HTML, with the page checks (*A script of the page's own*, below) |

Every one but shell-script is found by **executing** the page, and ends it:
one error per page, the first. A hook in a module that a page imports and does not render
— a barrel that also exports a dynamic segment's component — is not an error: as for
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
`lazy()` makes it: a dynamic segment's, exported from the same barrel, is nobody's
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

### A script of the page's own

The JS of a page is the behaviours its components mounted, and nothing else
(*Behaviours*): a page that carries what runs and the builder did not make
is **shell-script**, and the build stops (the owner, 2026-10-05; page-author
raw JS is deferred — CLAUDE.md).

| On the page as rendered | |
| --- | --- |
| a `<script>` that runs: no `type`, `module`, a JavaScript type — with a `src` or with text; SVG's too | shell-script |
| an `on…` attribute: `onclick="go()"` — any attribute whose name starts with `on`, of which HTML, SVG and MathML have no other | shell-script |
| a `javascript:` URL, in an attribute that holds a URL (`href`, `action`, `formaction`, `src`, …) | shell-script |
| a data block: `<script type="application/json">`, `application/ld+json`, `importmap`, `speculationrules`, any `type` that is not JavaScript's; a `<script>` with no source and no text | allowed: nothing runs |
| `<link rel="modulepreload">`, `rel="preload" as="script"` | allowed: a hint fetches, and runs nothing (*Checks on the page*, the table of `<link>`) |
| in a `<template>` or a `<noscript>` | not of the page: not looked at, as for the page checks |
| a document of the site in a frame (`<iframe src="/demo.html">`, `srcdoc`, `<object>`, `<embed>`) | not covered by the rule: built, and not pruned (*CSS*) |

```tsx
<script dangerouslySetInnerHTML={{ __html: `document.documentElement.classList.add("dark")` }} />
<a href="javascript:void 0">Top</a>
```

```
pages/theme/index.rtsx: error shell-script: Page /theme/: The shell cannot run a script of the page's own: `<script>`
pages/theme/index.rtsx: error shell-script: Page /theme/: The shell cannot run a script of the page's own: `href="javascript:throw new Error('React has blocked a javascript: URL as a security precaution.')"` on `<a>`
```

- Reported as the page checks are — at the page, quoting what was found,
  once per page for the same text — and before any CSS is built. The
  builder's own `<script>` is written by packaging, after: it is not there
  to be found.
- One notion of "runs" for the check and for the pruner (`markup`, in the
  builder): the pruner keeps its guard — a page with a script of its own is
  not pruned — and `reactogenic build` can no longer get there.
- A function-valued prop is shell-handler, when the element is made; this
  is what reaches the HTML as text: `dangerouslySetInnerHTML`, an attribute
  of a custom element, a `<script>` element.

**Rejected:** building such a page and leaving its CSS unpruned (the
builder until 2026-10-05: sound, and it broke "the JS of a page is the
behaviours its components mounted" and T1, silently).

### The engine

Shell code runs in `modernc.org/quickjs`: ES2023, and nothing of a host.
What that means for the code of a page:

| | In the engine |
| --- | --- |
| the time zone | **UTC, on every machine**: the build machine's zone is not in the page. `new Date(2026, 9, 4)` is `2026-10-04T00:00:00.000Z`; `getHours()`, `getDay()`, `setMonth()` are their `UTC` counterparts; `getTimezoneOffset()` is `0`; `toString()` is what V8 writes under `TZ=UTC` |
| date strings | ISO 8601 as ECMAScript defines it — `2026-10-04`, `2026-10-04T12:00` (no zone: UTC), `2026-10-04T12:00+02:00` — and what `toString()` and `toUTCString()` write. Any other string is parsed as each engine likes, and in local time: shell-nondeterministic |
| `Intl` | **there is none**: `Intl.NumberFormat` is a `ReferenceError`. The methods that would answer without it — a number unformatted, strings compared by code unit — throw shell-error instead: `toLocaleString`, `toLocaleDateString`, `toLocaleTimeString` (of `Number`, `BigInt`, `Date`), `localeCompare`, `toLocaleUpperCase`, `toLocaleLowerCase`. Format in code: `toFixed`, `padStart`, a table of month names |
| not there (a `ReferenceError`) | `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`, `structuredClone`, `atob`, `btoa`, `queueMicrotask`, `setTimeout`, `setInterval`, `fetch`, `Temporal`, `process`, `require`; no `Array.fromAsync`. `WeakRef` and `FinalizationRegistry`: when an object is freed is the collector's business, not the page's |
| a runtime per page | the bundle's bytecode, loaded anew: 2.9 ms a page on the fixture site (27.6 from its text). A page has the timeout (30 s) and 1 GiB; the bundle's own loading, the same. The 30 s are time the page **ran** — the builder's own timer, on the monotonic clock, which interrupts the engine — not the engine's own timeout, whose deadline is a moment of the wall clock: a machine that slept while a page rendered, or whose clock was set, ended that page with the timeout's message after a fraction of a second (plan.md, RGP2-071) |
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

Four functions of `@reactogenic/core`, usable in any component:

```ts
pathname(): string                       // the route being rendered: "/guide/"
useShellId(prefix?: string): string      // "d1", "m1", "m2": per page, in render order
mount(module: string, id?: string, flags?: Record<string, boolean>, data?: MountData): void
variants<M>(base: string, map: M, choice?: { [D in keyof M]?: keyof M[D] }): string   // "rg-menu rg-menu-end"
```

| | At build time | In React (a dynamic segment, Vite) |
| --- | --- | --- |
| `pathname()` | the route being rendered — in every variant of it (*Routes*): from the site's root, whatever `--base` | `location.pathname` |
| `useShellId(p)` | `p` + a counter per prefix, per page; no prefix is the prefix `r`: `r1`, `r2`. A prefix that ends in a digit is shell-error — the counter follows it, and the first `d1` would be the eleventh `d` | `React.useId()` |
| `mount(m, id, flags, data)` | recorded for the page (*Behaviours*): `flags` are the page's, `data` this use site's | nothing in phase 2 |
| `variants(base, map, choice)` | the string — and each class of it recorded for the page, with the component that called (*The record*) | the string |

`useShellId` exists because React's `useId` gives `_R_3e_`: valid, and
unreadable in view-source. Ids are stable while the page's tree is.

**`variants()`** resolves a component's options into classes: **a variant
is one class, unique to it** (the owner's rule, decisions.md, K; the
convention is components.md's, *CSS convention*).

```ts
const buttonVariants = { size: { sm: "rg-button-sm", md: "" }, look: { ghost: "rg-button-ghost" } } as const;
variants("rg-button", buttonVariants, { look: "ghost", size: "sm" })   // "rg-button rg-button-sm rg-button-ghost"
variants("rg-button", buttonVariants, { size: "md" })                  // "rg-button": "" is the default — no class
variants("rg-button", buttonVariants, { size: "xl" })                  // error TS2322: no such value
```

| | |
| --- | --- |
| the result | `base`, then the class of each chosen value, in the order **the map** names its dimensions — the same element has the same `class` however a use site wrote its props. A dimension without a choice (`undefined`) adds nothing; nor does a value whose class is `""` |
| the types | `map` is read as written (`as const`, or a literal at the call): a dimension or a value it does not have is a type error. At run time such a choice adds nothing: no class is made up |
| it is a pure function | of its arguments, in both worlds: no hook, callable anywhere a string is — a module's top level too, where nothing is recorded (no page renders) |
| the record | at build time each class of the result is noted for the page, once, with the components whose calls resolved it — the report's `classes` (*The report*) |
| **what the record decides** | **nothing.** A page's CSS is pruned against the page as it is served (*CSS*): the class is on an element, and a class is matched exactly. A rule for a variant is kept on exactly the pages with an element of that variant — as a rule on `[data-variant="ghost"]` was. What the class adds is a name: for the rule, for the report, and for a packaging step that may one day want to say which component a rule is of |

**Rejected:** the record of resolved classes as the *input* of pruning — a
sheet for the whole site, chosen by the classes any page resolved (built on
the owner's first ruling on K, measured, and reversed: decisions.md): it is
analysis turned into a site-wide union because packaging wanted one file.

## The record

What makes the builder component-aware is not the import graph (it
over-reports: a layout that *can* attach a dialog imports it on every page) —
it is what executing the page **recorded**:

| | Source | Used for |
| --- | --- | --- |
| the HTML | React's renderer | the page; CSS pruning; the checks |
| mounts: `(module, id, flags, data)` | `mount()` | the page's JS |
| components rendered, with counts | the builder's JSX runtime | the report |
| classes resolved, each with the components that resolved it | `variants()` | the report |

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
| a root-relative link | `href` of any element but `<base>`, starting with one `/`. It is a route — `/guide/`, also written `/guide` (the host redirects); a route without an `index` too (*Routes*) — a variant's document, by its file — `/guide/index.html`, `/account/guest.html` — or another file of the output: `/favicon.svg`, `/demo/` for `demo/index.html` |
| its path | what a browser requests, in the names of the site's directories: tabs and line breaks dropped, `\` a `/`, dot segments resolved (also `%2e`), then each segment percent-decoded (`/se%C3%B1or/` is `pages/señor/`). Not Go's `url.Parse`: `/guide//` is not `/guide/`, `%2F` is not a separator, and a `%` that encodes nothing stands for itself — `/100%` is checked, not passed |
| not checked | a URL with a scheme or a host (`https:`, `mailto:`, `//host`, `/\host`); a relative one (`slot/`, `../x/`, `?tab=2`) |
| `--base /docs/` | not seen by the check. A page links to the site from its root, whatever the base — `/guide/`, as `pathname()` names it — and is checked as rendered, before packaging prefixes the link (*Packaging*). So `/docs/guide/` is link-not-found: it is no route, and would be served as `/docs/docs/guide/` |
| `<template>` | its content is not of the page: neither its ids nor its references. The element itself is: its `id` counts, here and for mount-no-element |
| `<noscript>` | the same: to a browser that runs scripts — the ones a behaviour is for — its content is text. However the page was parsed |
| a script of the page's own | shell-script, found here with the checks (*Shell code in phase 2*, *A script of the page's own*) |
| repeated | one report per page for the same attribute on the same element name: a link of the layout is on every item of a list |

**`<link>`, by its `rel`.** The owner's rule (2026-10-05): a `<link>` is
what its `rel` says — its tokens separated by spaces, compared without their
case — and by nothing else: not by `as`, not by where it stands, not by
`disabled`. One table, which the pruner, the page checks and the link check
follow:

| `rel` has | It is | Effect |
| --- | --- | --- |
| `stylesheet` (also with `alternate`), and it is not the builder's | a stylesheet the builder did not bundle | the pruner drops no custom property, `@keyframes` or `@position-try` (*CSS*); link-not-found checks a root-relative `href` |
| `preload`, `modulepreload`, `prefetch`, `preconnect`, `dns-prefetch` | a hint: it fetches, and applies and runs nothing | none on pruning; a root-relative `href` is checked by link-not-found; not a script of the page's own |
| `icon`, `apple-touch-icon`, `manifest`, `canonical`, `alternate` (without `stylesheet`), `prev`, `next`, `author`, `license`, `help`, `search`, `me` | a relation | none on pruning; a root-relative `href` is checked |
| anything else, or no `rel` | unknown | none on pruning; a root-relative `href` is checked; no error |

```html
<link rel="Alternate StyleSheet" title="Dark" href="/dark.css">   <!-- a stylesheet: every custom property stays -->
<link rel="preload" as="style" href="/dark.css">                  <!-- a hint: it applies nothing, and the page is pruned as without it -->
<link rel="stylesheet" href="/dark.css" disabled>                 <!-- a stylesheet: something may turn it on -->
<link rel="stylesheet" href="/_rg/page-8a8c3c5b.css">             <!-- the builder's own: the sheet that is pruned -->
```

With several tokens, `stylesheet` decides, then a hint, then a relation.

> Later: `rel=preload` could be classified further by `as` (a preloaded
> style, script, font) — the owner: not now.

> Ruled by the owner (decisions.md, H): no fifth check. A `<dialog>` inside
> a `[popover]` that a `commandfor` outside that popover names opens modal
> and unseen (components.md, *Dialog*, delivery); that is the design system's
> to solve, later — `dialog-in-popover` is not built.

> OPEN: a link to another page is checked for the page, not for the place in
> it: `/syntax/#slots` passes whether or not `/syntax/` has an element
> `slots`. The builder has every page's ids; the docs site's side menu is
> made of such links, and a renamed section breaks them silently (plan.md,
> RGP2-040). Recommended: check the fragment against the target page's ids,
> by the rule of `href="#x"`.

They are reported at the page — its file, and its path: the pathname, or
`/account/guest.html` for a variant that is not `index` (*Routes*) — with the
offending attribute's text; mapping an attribute back to its `.rtsx` position needs
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
`<link>`, and a `<script>`. (A `<style>` of the page's own is pruned too: its
text selects nothing.)

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
| `<script type="module">`, inline or `src` | a script of the page's own: its text is the page's built script, which the pruner reads (*The page's script*) | a script of the page's own: shell-script, and no page is built (*Shell code in phase 2*). The pruner would leave such a page unpruned |
| `<link rel="stylesheet">` to the page's sheet | a stylesheet the builder did not bundle: it is the sheet that is pruned | one, when its `rel` has `stylesheet` (*Checks on the page*, the table of `<link>`): every custom property, `@keyframes` and `@position-try` stays |
| `<style>` with the page's sheet | pruned with what it holds: a sheet is not pruned against itself | CSS of the page: pruned, as below |

An element is the builder's when it is what packaging wrote: that name,
those attributes — and, in the document the pruner is given, no text: an
inlined blob's element is empty there. An element of the author's that is
exactly that delivers the same blob under the same URL, or nothing (an empty
`<script>` does not run), and is rightly taken for one.

```html
<!-- /dialog/ as served: pruned — the one <script> is the builder's -->
…<button command="show-modal" commandfor="d1">Open</button>…<script type="module" src="/_rg/page-8a8c3c5b.js"></script></body>
```

**A `<style>` of the page's own** (the owner, 2026-10-05: always allowed,
and pruned) is CSS of that page as its sheet is. Its text is pruned against
the page as served — the same rules, the same "maybe", the same script —
and written back into the element in place, minified as the sheet is:

```tsx
<style>{`
  .note { color: var(--note); & b { font-weight: 600 } }
  .never { color: red }
  :root { --note: teal; --unread: 0 }
`}</style>
<p className="note">A note, <b>bold</b>.</p>
```

```html
<style>.note{color:var(--note)}.note b{font-weight:600}:root{--note: teal}</style><p class="note">A note, <b>bold</b>.</p>
```

| | |
| --- | --- |
| one cascade | the page's own `<style>` elements and its sheet are the sheets of one document, in document order: one in the head precedes the sheet (packaging writes that at the end of `<head>`), one in the body follows it. What one names another may define — a custom property the sheet declares stays when a `<style>` reads it; a `@keyframes` or a `@position-try` of a `<style>`, when the sheet names it — and a layer is ordered where its name first occurs in any of them (*Layers*) |
| what is CSS | a `<style>` whose `type` is absent, empty or `text/css`. With a `media` attribute it is pruned too — what it declares counts as inside `@media`. Any other `type` is not CSS: left as it is, and what it names stays |
| nesting | lowered, as the bundled sheet's is: the text is given to esbuild first, and what esbuild says of it is css-warning, at the page |
| in `<svg>` | CSS of the page: pruned |
| in a `<template>` or a `<noscript>` | the page is not pruned at all (the table below), and its `<style>` elements are as they were written — wherever they stand |
| a page that is not pruned, for any reason | every `<style>` of it is byte for byte what React rendered |
| a text that does not read | an unclosed string, an unbalanced bracket: kept byte for byte, read for the names it uses, and said — css-warning: *a `<style>` of the page is not pruned: …* |
| an `@import` in it | kept; it is a stylesheet the builder did not bundle |
| the control | prunes nothing: every `<style>` as written |
| the report | a row of its own under *CSS rules*: `<style>`, the rules of the page's own elements kept and dropped. Their bytes are in the page's HTML |

**Rejected:** pruning against the page before packaging, with the script
beside it (the reconcile's first form: `script { display: block }` and
`a[href^="/docs/"]` are then matched against a page that is not the one
served); taking every `<script>` of the served page for the page's own (the
two fixes together, unreconciled: every page with a behaviour came back
unpruned); a `<style>` of the page's own read for the names it uses and left
whole (the builder until 2026-10-05: its rules for elements no page has
shipped on every page).

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
| runtime state is "maybe" | every pseudo-class except `:root`, `:is()`, `:where()`, `:not()`, `:has()` (which are evaluated on their arguments), a `&` that lowering left, and every attribute selector on what **the browser writes by itself**: `open`, `hidden`, `style` — and, on the one kind of element whose menu has it, `dir` (a text control), `controls` and `loop` (a player) (*Runtime state*). The negation of "maybe" is "maybe" |
| state only a script writes | `inert`, `disabled`, `checked`, `selected`, `value`, `aria-*`, `data-state` — as any attribute: decided on the page, **unless the page's script names it** (the row below; *Runtime state*). On a page without a script they are all the page's |
| what the page's script names | "maybe" too (*The page's script*): a class, an id, an attribute — by its name or by the property that reflects it — on an element that has it as on one that has not: the script takes away as well as it adds, and `:not(.collapsed)` is "maybe" then; a custom property or an animation it names is read. A script that sets an element's text: what an element *has* (`:has()`) is "maybe". A script that may change the tree: the page is not pruned |
| case | as HTML: tags and attribute names fold, classes and ids do not — the page starts with `<!doctype html>`, as the builder writes it. Given a page without exactly that doctype, which may be in quirks mode, classes and ids fold too; attribute values are case-sensitive except with the `i` flag and for the attributes HTML compares case-insensitively (`type`, `rel`, `lang`, `dir`, `method`, …) |
| pseudo-elements | ignored for matching (`::backdrop`, `::before`, `::details-content`). Whatever follows one — `::before:hover`, `::before::marker` — is "maybe", and never safe to drop on its own (*Lists*) |
| a selector list | pruned per selector; the rule goes when none is left. **Kept whole** if a selector that would go is one some browser of the floor may reject (*Lists*). Only the rule's own list: the arguments of `:is()`, `:where()`, `:not()`, `:has()` are never trimmed — `:is(.btn, #never)` has the specificity of `#never`, matched or not |
| a selector the pruner cannot parse | kept, and its list with it, untouched (a namespace, a column combinator, a comment inside, a combinator after a pseudo-element): what does not parse may not be a selector at all |
| `@media`, `@supports`, `@container`, `@scope`, `@starting-style` | pruned inside; dropped when empty. Inside `@scope` a selector is matched against the whole page (the scope's limits are not evaluated), and what esbuild leaves there is kept as it is: a rule that still nests, a declaration between the rules |
| `@layer` | a block is pruned inside; an emptied block stays as `@layer name;` unless something before it already orders that layer. **Dropping it would reorder the cascade** (*Layers*). A block whose prelude is not one layer name (`@layer a, b { … }`) is no layer: kept as it is |
| `@keyframes` | kept iff its name occurs as a word in a kept `animation` or `animation-name` value, in a kept custom property's value (`--a: spin` for `animation: var(--a)`), in a `style` attribute of the page, in a kept rule of a `<style>` of its own, or in the page's script |
| `@position-try --x` | kept iff `--x` occurs elsewhere, as for a custom property: in a kept `position-try-fallbacks` or `position-try`, in a kept custom property's value, in a `style` attribute of the page, in a kept rule of a `<style>` of its own, or in the page's script. Its own prelude does not count. One whose prelude is not one dashed name is kept as it is |
| custom properties | a `--x` declaration is dropped iff `--x` occurs nowhere else: in no kept CSS — of the sheet or of a `<style>` of the page's own (values, at-rule preludes, `@property`, `@keyframes`) — in no attribute of the page (a `style`, SVG's `stroke="var(--x)"`) and not in the page's script (`getPropertyValue("--x")`). Its own value does not count (`--x: var(--x)`). Repeated to a fixed point, with the at-rules it empties |
| a stylesheet the builder did not bundle | a `<link>` whose `rel` has `stylesheet` (*Checks on the page*, the table of `<link>`: not a hint, not a relation), an `@import` in a `<style>` or left in the sheet: it may read any custom property and name any animation or `@position-try`, and nothing here sees it. Every custom property, every `@keyframes` and every `@position-try` stays; rules are pruned as ever. The `<link>` or `<style>` packaging writes for the page's own sheet is not one (*The builder's own elements*) |
| `@font-face`, `@property`, `@import`, `@namespace`, anything unknown | kept, byte for byte |
| an `@import` or `@namespace` after a rule | everything before it is kept, byte for byte (*Out of place*) |
| a page with a script of its own | not built: shell-script (*Shell code in phase 2*, *A script of the page's own*). The pruner's own guard stays — given such a page it does not prune it: what the script writes is not known, and `/theme.js` adding `dark` to the root would find `.dark .a` gone — and the builder no longer gets there. The `<script>` packaging writes for the page's behaviours is not one: it is the script the pruner reads (*The builder's own elements*) |
| a page with a document of the site in a frame | legal, and not pruned — the owner's ruling "for the first iteration" (decisions.md, E) — for the same reason: the document is of the page's origin, and its script writes to the page (`parent.document.body.classList.add("lit")`) as one of the page's own would. An `<iframe>` with `srcdoc`; an `<iframe>`, `<frame>` or `<embed>` whose `src`, an `<object>` whose `data`, is written without a scheme and a host (`/demo.html`, `demo/`) — the site's own, as for a link (*Checks on the page*). Not one: a URL in full (`https://…`, `//host/…`: another site's — the builder does not know where this one is served, so a frame of the site's own is written from the root); `data:`, `about:blank`, no URL at all; an `<iframe sandbox>` that does not allow both `allow-scripts` and `allow-same-origin`. **Not seen:** a document that reaches the page another way — one that opened it (`window.open`), or frames it |
| a page that contains `<template>`, `<noscript>`, `<selectedcontent>`, a `<select>` holding more than `<option>`, `<optgroup>`, `<hr>` and text, or an element with `contenteditable` (any value but `false`) | not pruned: the tree a browser matches against is not the one written (the design system emits none). Cloned content would need "maybe" relations — a template's wherever a script puts it, the selected `<option>`'s inside `<selectedcontent>` at load; `<noscript>` is elements or text depending on the browser; markup inside a `<select>` is kept by a parser that knows the customizable select and dropped by one that does not, where `<select><div><option>` makes `select > option` match; and what the user edits gets elements no script wrote — Bold a `<b>`, Enter a `<div>`, a paste whatever was copied — which `.editor b` then matches. The report says which (*The report*) |
| CSS that does not read | an error, and nothing is pruned: an unbalanced bracket, an unterminated string or comment |

What is kept is kept byte for byte, in order: nothing is rewritten, so the
result can be minified again.

**Runtime state.** What can change after the page has loaded never decides
a selector. Who can change it is what the list is made by (the owner's
ruling on M, decisions.md: option 2):

| State | Who writes it | A selector on it is |
| --- | --- | --- |
| a pseudo-class — `:hover`, `:focus-visible`, `:checked`, `:disabled`, `:popover-open`, `:modal`, `:first-child`, … | the reader and the browser, on every page | "maybe", always. Not narrowed: `:disabled` and `:checked` could be decided from the attributes on a page whose script names none of them — option 3 of the question, not taken: a model of each pseudo-class to keep sound |
| `open` | the browser: a `<details>` the reader opens, the others of its `name`, which it closes, one that find-in-page opens; a `<dialog>` that a `command` button, Esc or `closedby` opens and closes | "maybe", always |
| `hidden` | the browser: `hidden="until-found"` is taken away by find-in-page and by a fragment | "maybe", always |
| `style` | the browser: an element with `resize` gets `width` and `height` there when the reader drags its corner | "maybe", always |
| `dir`, on an `<input>` or a `<textarea>` | the browser: the reader may turn a text control around (HTML, *The `dir` attribute*: the user agent sets the attribute) | "maybe" on those elements; on any other, the page's |
| `controls`, `loop`, on a `<video>` or an `<audio>` | the browser: "Show controls" and "Loop" of the player's own menu set them and take them away | "maybe" on those elements; on any other, the page's |
| `inert`, `disabled`, `aria-*`, `data-state` | a script, or nobody: no browser writes them. A popover and a modal dialog reflect nothing into attributes but `open`; what is inert under a modal dialog has no `inert` | the page's, unless the page's script names it |
| `checked`, `selected`, `value` | a script, or nobody: these are a control's **defaults**. What the reader does changes the control's state — `:checked`, the value — and never the attribute; a reset reads them and writes none | the page's, unless the page's script names it |
| anything, in an element the reader edits (`contenteditable`) | the browser, as it likes | the page is not pruned (the table above) |

```css
/* /guide/ of the docs site: its menu marks no item, and its script — `overlays` — names no `aria-current` */
:is(.rg-menu>:is(a,button),.rg-menu>li>a)[aria-current] { font-weight: 600 }   /* dropped: nobody can write it        */
.rg-sidemenu>:is(section,details) a[aria-current] { … }                        /* kept: the side menu has one         */
.rg-button:is(:disabled,[aria-disabled=true]) { … }                            /* kept: `:disabled` is a pseudo-class */
.rg-dialog[open] { … }                                                         /* on `/`, which has a dialog: kept — the browser writes `open` */
```

A page whose script is absent — it mounts nothing — decides them all on
the page. A page with a script decides those its script does not name: by
the attribute's name, or by a property that reflects it (*The page's
script*, the table of names). Until this ruling every attribute of the list
was "maybe" whoever could write it — the list predates the pruner reading
the page's script — and each sheet of the docs site kept a rule on
`[aria-current]` for a menu no page marks an item of: 72 B a page, raw,
of the 301 B bet.md counts (*State nobody can reach*). The other 229 B are
the two rules that hold `:disabled`, and stay.

`:is()` does not change with it: its arguments are never trimmed (the row
above), so `.rg-button:is(:disabled,[aria-disabled=true])` is kept whole for
`:disabled`, and on `/guide/` `.rg-menu>:is(a,button)` stays inside `:is(…)`
where it went from its own list — an `:is()` has the specificity of its
arguments, matched or not.

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
  root.classList.toggle("collapsed");        //   .collapsed          "maybe", on the element that has it too
  root.tabIndex = 0;                         //   [tabindex="0"]      "maybe": the property reflects the attribute
  root.dataset.placement = "top";            //   [data-placement]    "maybe"
  root.setAttribute("data-side", "left");    //   [data-side=left]    "maybe"
  root.ariaExpanded = "true";                //   [aria-expanded]     "maybe": state only a script writes, and this one names it
  root.toggleAttribute("inert");             //   [inert]             "maybe" — and `:not([inert])`
  root.firstElementChild.disabled = true;    //   [disabled]          "maybe"
  root.lastElementChild.textContent = "ok";  //   :has(…)             "maybe" where it was "yes"
  getComputedStyle(root).getPropertyValue("--rg-breakpoint");   // the property is read
}
```

A script **takes away** as well as it adds, so the script is asked before
the page is: what the page has when it loads decides nothing once the script
names it.

```html
<div id="c9" class="card collapsed"><p class="body">…</p></div>
```

```css
.card:not(.collapsed) > .body { … }   /* matches nothing as written — and the body, after a click */
```

| | |
| --- | --- |
| a name | a word of the script's text — an identifier, a property, a word of a string — whatever it is there: `dark` of `classList.add("dark")` and a variable `dark`. More is "maybe" than is written; never less |
| a class, an id | "maybe" when it is a word of the script, whatever its case; a name that is no word (`sm:flex`), when the text holds it. On every element — one that has it (`classList.toggle("collapsed")`, `classList.remove("active")`: `:not(.collapsed)` is "maybe") and one that has not |
| … the page has, and the script does not name | "maybe" when the script may write the attribute whole: for a class, it names `className`, the word `class` (`setAttribute("class", …)`, `removeAttribute("class")`) or `classList` beside `value`; for an id, the word `id` (`label.id = "second"` — `:not(#first)` is "maybe"). `classList` alone names what it adds and removes |
| an attribute | "maybe" when the script names it as it is written (`data-side` — in `setAttribute`, `toggleAttribute`, `removeAttribute`, or anywhere else: a word is a word), or as a property that reflects it — the table below |
| the text | a script that names `textContent`, `innerText` or `text` — read or written: `Object.assign(el, { textContent })` is no assignment to look for — may put text where an element held elements. They go; none comes, none moves. So `:has()` is "maybe" where it was "yes" (`.status:not(:has(b))` stays), "no" is still "no", and the page is pruned |
| the tree | a script that names an API that adds, moves or removes an element — `createElement`, `innerHTML`, `outerHTML`, `outerText`, `insertAdjacentHTML`, `cloneNode`, `appendChild`, `insertBefore`, `replaceChildren`, `replaceWith`, `append`, `prepend`, `before`, `after`, `remove`, a table's `insertRow`, a range's `insertNode`, `execCommand`, `contentEditable`, … — may change what stands next to what: the page is not pruned, and the report names the word (*the page's script may change the tree: it names `append`*). The word is enough, whatever it is there (a `URLSearchParams`'s `append`) — but for a `remove` written `classList.remove` (`classList?.remove`): that takes a class away, and is the commonest write a behaviour makes. A list held in a variable (`const l = e.classList; l.remove("x")`) is not seen to be one |
| not seen | a name the script computes (`"is-" + state`): a behaviour names what it writes, in full (*Behaviours*, what a behaviour writes) |

A property that reflects an attribute names it. The script's words are
compared in lower case with the hyphens taken out, so one rule covers most
of them, and the rest are few:

| The attribute | Is named by | |
| --- | --- | --- |
| any: `disabled`, `inert`, `tabindex`, `readonly`, `aria-expanded`, `aria-haspopup`, … | its own name, whatever the case and the hyphens: `disabled`, `inert`, `tabIndex`, `readOnly`, `ariaExpanded`, `ariaHasPopup` | `e.disabled = true` writes `disabled`; `e.ariaExpanded = "true"` writes `aria-expanded` |
| `value`, `checked`, `selected` | their own names too — `e.value`, `e.checked`, `e.selected` | these properties do not reflect on a text field, a checkbox, an option — and `value` does on a button, an option, a list item, a hidden input. Named is named: the pruner does not know the element |
| `value`, `checked`, `selected`, `muted` | `default` before the name: `defaultValue`, `defaultChecked`, `defaultSelected`, `defaultMuted` | the properties that do reflect them |
| `aria-controls`, `aria-labelledby`, `aria-describedby`, `aria-details`, `aria-errormessage`, `aria-flowto`, `aria-owns`, `aria-activedescendant`, `popovertarget`, `commandfor` | `Element` or `Elements` after the name: `ariaControlsElements`, `ariaActiveDescendantElement`, `popoverTargetElement`, `commandForElement` | setting one writes the attribute, empty |
| `data-foo-bar` | the word after `data-`: `fooBar` of `dataset.fooBar`, `state` of `dataset.state` | |
| `class` | `className`, `classList` | |
| `for` | `htmlFor` | |
| `rel` | `relList` | |

Each row is a test of the pruner (`TestPruneReflects`, `TestPruneState`:
dropped when nothing names it, kept when the attribute or the property is
named, and the negation kept when a script takes the state away).

The three behaviours of `@reactogenic/ui` write nothing to the page, name
no such API and neither `class` nor `id` (a test of the pruner builds them
and says so — and pins the state of the design system's CSS that each of
them names: `menu-keys` names `aria-disabled`, in the selector of the items
it moves between, so a page that mounts it keeps the rules on
`[aria-disabled]`; `overlays` and `invokers` name none). `menu-keys` reads
`textContent`, for typeahead: on a page that mounts it `:has()` is never
"yes" — which keeps no rule of the design system or of the docs site that
would have gone.

**Rejected:** a rule alone — "a behaviour writes only the attributes that
are *maybe*" — with nothing that reads the script (the first draft:
`mount()` takes any module, and `item.tabIndex = 0` in one silently lost
`.menu [tabindex="0"]`); a check of each module's writes (`classList`,
`setAttribute` with a literal, …: what is written is known only with types,
and a property that reflects an attribute looks like any other); the page
asked before the script — "yes" for a class the element has, the script
consulted only for one it has not (as first built: `:not(.collapsed)` was
"no", and three rules of a card that collapses were gone when it opened);
`remove` as a word like the others (`classList.remove("x")` left its page
unpruned, with "changes the tree" for a reason); an assignment to
`textContent` taken for a change of the tree (the page loses its pruning for
"Copied", and a write that is not an assignment is missed); every
attribute a behaviour might write "maybe" on every page — `aria-*`,
`disabled`, `data-state`, … whoever could write them (the builder until the
owner's ruling on M: a rule for a state nobody on the page can reach
shipped with it).

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

The owner's rule (2026-10-05; decisions.md): *the builder creates one
behavior entrypoint per page. Page-wide behavior capabilities are passed to
esbuild via Define, allowing unused behavior branches to be eliminated;
instance-specific flags remain in generated mount data/calls.*

The JS of a page is the behaviours its components **mounted**, and nothing
else. A behaviour is a TypeScript module, owned by the design system:

```ts
// @reactogenic/ui/behaviors/menu-keys.ts
declare const RG_MENU_TYPEAHEAD: boolean;

export default function mountMenuKeys(root: HTMLElement, own?: { typeahead?: boolean }): void {
  root.addEventListener("keydown", onKey);
  // the flag: is the code in the page; the data: did this menu ask
  if (RG_MENU_TYPEAHEAD && own?.typeahead) root.addEventListener("keydown", onType);
}
function onKey(e: KeyboardEvent) { … }
function onType(e: KeyboardEvent) { … }
```

```tsx
// the component: an action menu needs arrow keys, a menu of links does not.
// The flag says the page needs the typeahead code; the data, that this menu uses it
const id = useShellId("m");
if (hasAction) mount("@reactogenic/ui/behaviors/menu-keys", id, { RG_MENU_TYPEAHEAD: typeahead }, typeahead ? { typeahead: true } : undefined);
```

```js
// generated entry for a page whose record holds two such mounts, the second with typeahead
import m0 from "@reactogenic/ui/behaviors/menu-keys";
import m1 from "@reactogenic/ui/behaviors/overlays";
m0(document.getElementById("m1"));
m0(document.getElementById("m2"), {"typeahead":true});
m1();
```

| What a use site says | It is | Passed by | Decides |
| --- | --- | --- | --- |
| the page needs a capability | a **flag**: `RG_…`, the third argument of `mount()` | `Define`, one map per page | whether the code is in the page's script |
| this root uses it, or anything else only the script reads | the mount's **data**: the fourth argument of `mount()` | the second argument of that mount's call in the entry | what the behaviour does on this root |

The page's HTML does not carry what only the script reads:

```html
<div id="m2" class="rg-menu" data-typeahead popover role="menu">   <!-- before: an attribute the behaviour read -->
<div id="m2" class="rg-menu" popover role="menu">                  <!-- now: the same HTML with and without typeahead -->
```

**Only what the script alone reads moves** (the owner, 2026-10-05: an
attribute may carry CSS state rather than JS — `open` → `data-open`). What
a rule selects is the page's, not the mount's:

| A use site's value is read by… | It is… | |
| --- | --- | --- |
| the behaviour only | the mount's data: an argument of its call | `typeahead` |
| CSS, at build time (an option) | a class of its own on the element the rule selects, through `variants()` (*What shell code can ask the builder*; components.md, *CSS convention*) | `rg-menu-end`, `rg-button-ghost` |
| CSS, at run time (state) | the platform's state, or an attribute the behaviour writes and names in full — by its name or by the property that reflects it | `[open]`, `:popover-open`, `data-open="true"`, `aria-expanded` |
| CSS **and** the behaviour | the attribute, once: the behaviour reads it there — never a second copy in the mount's call that could disagree | |

| | Rule |
| --- | --- |
| module | the default export is `(root: HTMLElement, data?) => void`, or `() => void` for a page-level behaviour (mounted without an id; run once per page however often it is mounted). One or the other: a module that a page mounts both ways is mount-kind — its page-level call would hand the function no root |
| the entry | one import per distinct module, in the order they are first mounted, resolved by esbuild from the project directory (through a symbolic link too: files are named from the real one); one call per mount, in render order. One module on one element twice is one call. A module is the **file** it resolves to: two spellings of it are one import, and one page-level call |
| data | a plain object of JSON values — strings, finite numbers, booleans, `null`, arrays and plain objects of them. It is recorded with the mount and written into the entry as the literal it is, its keys sorted: the same data is the same script, in whatever order a use site wrote it. A key whose value is `undefined` is left out, as `JSON.stringify` leaves it; `undefined` or `null` for the whole is no data, and that mount keeps the call of one argument. Anything a literal cannot say is mount-data: a function, a `Date` or any other instance, `NaN`, a symbol, an object that holds itself, a key `__proto__` (in a literal it is the prototype) |
| … of one element | an element gets one call of a module, with one data: two mounts of one module on one element hand it the same, or it is mount-data — also when one of them hands none |
| … of the page | a page-level behaviour is called once, for everyone who mounted it: there is no use site to hand it anything of. Data without an id is mount-data |
| flags | bare `declare const RG_…: boolean` identifiers — never an options object, an imported constant or a class member: esbuild removes code only on parser-time constants (research.md) |
| a flag | an identifier that starts with `RG_` and that the module reads **free** — nothing binds it; `declare` binds nothing. Exactly what `Define` replaces, so esbuild is asked: each file is transformed with every `RG_…` name of its text defined as a marker, and the names whose marker comes out are its flags. `RG_lower`, `RG_$` are flags; a name in a comment, a string, a property, or `const RG_X = false` of the module's own is not |
| a page's flags | one `Define` map per page: the **union** over the page's mounts. A flag's name is the page's, not a module's — on, in every module that reads it, once one mount turned it on; two modules that read one name share it. So a flag gates **code**, never a use site: whether a root uses a feature is its mount's data (`typeahead`, above). A root behaves the same whatever else is mounted on the page — and the same under `--no-specialize` |
| every flag is defined | the builder defines each flag of the module and of every module it imports — `false` unless a mount set it. A flag the builder did not see would be a `ReferenceError` at run time, so the built script is asked the same question: a free `RG_…` left in it (one written `RG_\u0041` in the source) is mount-flag, and no script |
| no top-level side effects | the builder **reads** each module once per site: it bundles `import "<module>"` alone — nothing of it used — ignoring `sideEffects` and `@__PURE__` annotations. Whatever is left in that output ran at the top level: mount-side-effect. State and constants are not code that runs (`let typed = ""`, `new WeakMap()`, a class, an enum); a call is (`["a", "b"].join(",")`, `matchMedia(…)`, `"command" in HTMLButtonElement.prototype`) — it belongs in the function. The same read resolves the module and lists the files whose flags are its own |
| … of a module with flags | read as a page builds it — every flag defined: once all on, once all off, and what is left of either counts. `const DELAY = RG_SLOW ? 500 : 100` is a constant; `if (RG_X) document.title = "x"` runs. What runs only under a mix (`RG_A && !RG_B`) is not seen |
| what a behaviour writes | state, and it **names** it: any class, id or attribute — `aria-*`, `disabled`, `data-state` among them: nothing a script writes is "maybe" unsaid (*CSS*, *Runtime state*) — written by its name in full, a literal, not `"is-" + state`, or through the property that reflects it (`e.ariaExpanded`). The pruner reads the page's script for those names (*CSS*, *The page's script*); a name it cannot read is a rule it may drop. What it takes away it names too — `classList.remove("active")`, not `classList.remove(state)`. No element is added, moved or removed: a script that may leaves its page unpruned — but for an element's text, which may be set (`status.textContent = "Copied"`). The same for a custom property or an animation it uses: named in full |
| build | one `api.Build` per page: generated entry, `Bundle`, minify, ES modules, `Define` = the page's flags, `Metafile`. No splitting. Every input of its metafile is a file some mounted module reaches |

The `(root, data)` signature is layout.md's `mountX(root, …)` for clones opened
from dynamic segments. That much carries over, and no more: the generated entry
mounts by id when the page loads (a clone exists only after `open`, with ids
made per instance); a behaviour returns nothing (layout.md's handle has
`update` and `close`); and the record is of shell code — what a dynamic segment
mounts happens at run time (*Not in phase 2*).

| Code | Condition |
| --- | --- |
| mount-not-found | the module of a `mount()` does not resolve |
| mount-no-element | the id of a `mount()` is not on the page (a `<template>`'s content is not, nor a `<noscript>`'s) |
| mount-kind | a module is mounted on an element by one `mount()` of the page and without an id by another — however the two spell it. Per page: one that is of an element on `/` and of the page on `/guide/` is not seen, each page being built on its own |
| mount-flag | a flag passed to `mount()` — on or off — that is no flag of the module or of a module it imports; a flag the page's script reads and the builder did not find |
| mount-data | the data of a `mount()` is not a plain object of JSON values, or is given without an id — found where `mount()` is called; a module is mounted on one element twice with different data |
| mount-side-effect | a module, or one it imports, runs code when it is imported; reported at that file, with the statement that stays, once per file however the mounts spell the module |
| mount-error | anything else esbuild says of a module — an import of its own that does not resolve, no default export: its message, at its position |

The first four are reported as the page checks are — at the page, naming
the mount: ``Page /syntax/: `mount("@reactogenic/ui/behaviors/menu-keys")`:
no element of the page has `id="m9"` ``. So is mount-data for two mounts
that disagree; data that is not JSON is reported as a shell rule is — at the
`.rtsx` position of the call, and it ends the page:

```
ui/menu.rtsx(14,3): error mount-data: `mount("@reactogenic/ui/behaviors/menu-keys")`: the data is not JSON: `data.onPick` is a function
pages/syntax/index.rtsx: error mount-data: Page /syntax/: `mount("@reactogenic/ui/behaviors/menu-keys")`: the module is mounted on `#m2` twice, with `{"typeahead":true}` and with no data: an element gets one call, with one data
```

## Packaging

How what the artifacts need reaches the browser (*Analysis and packaging*):
each piece inlined in its document, a file of its own, or factored into
files several artifacts share — for CSS and JS by the same rules. It may
make a page fetch what another needs; it never changes what a page needs,
and the report has both (*The report*).

**Today** packaging factors nothing below a page's whole sheet or script:
per page its HTML, its pruned CSS, its JS; identical CSS or JS on several
pages is **one blob**, by content hash. So a page fetches exactly what it
needs. The documents and the blobs that are files are the build's artifacts
(*Routes*). What it should factor beyond that is the OPEN below.

| `--inline` | A blob is… |
| --- | --- |
| `auto` | a file when it serves two or more pages **and** is 4096 B or more as written; otherwise inlined |
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
  it: the two elements above, the base — and the text of a `<style>` element
  of the page's own, which is CSS of the page and is written as pruning left
  it (*CSS*, *The builder's own elements*): the one place where text React
  rendered is written anew. It is not parsed and serialised again, and nothing lower-cases an attribute: `popoverTarget="m1"` is
  `popovertarget` to every HTML parser, the page checks' among them.
- A blob that cannot stand inside its element is a file, whatever `--inline`
  says: CSS that holds `</style`, a script that holds `</script` or `<!--`
  (esbuild escapes the first two in what it prints; `<!--` it does not).
- A site behind a `Content-Security-Policy` without `'unsafe-inline'` is
  built with `--inline never`: no inline `<style>` or `<script>` is written.
  Hashes of the inlined blobs for the policy (`sha256-…`, per page) are *Not
  in phase 2*.

```html
<!-- /guide/, --inline auto: its CSS is /guide/more/'s too, and 1,074 B: inlined in both; it mounts nothing -->
<!doctype html><html lang="en"><head>…<style>…</style></head><body>…</body></html>
<!-- the same page of the control: the site's one sheet, 7,523 B, is a file; its one script, 1,730 B, is in every page -->
<!doctype html><html lang="en"><head>…<link rel="stylesheet" href="/_rg/site-a05e499c.css"></head><body>…<script type="module">…</script></body></html>
```

**`auto`.** Agreed with the owner (2026-10-05): the threshold is the
ecosystem's, **4096 B** of the blob as written.

| A blob of… | on one page | on two or more |
| --- | --- | --- |
| under 4096 B | inlined | inlined in each |
| 4096 B or more | inlined | a file, fetched once: `_rg/page-<hash>.css` |

```
a script that is `overlays` alone (252 B), on all four pages of the docs site     inlined in each
a sheet that is the same 1,074 B on two pages                                     inlined in each
a sheet of 7,523 B that every page has (the control's)                            /_rg/site-<hash>.css, once
a sheet of 9 KB that differs from every other by one selector                     inlined: it is one page's
```

- The size is the blob's own bytes as it is written — not compressed: what
  Vite and Astro count, and a number no encoder decides. The report prints
  it in its `raw` column.
- A blob of one page is never a file: a request for what no other page has
  buys nothing.
- How many pages share it beyond two does not count: the builder knows the
  site's pages, not a visitor's.
- Sharing couples pages: when a change makes one page's CSS equal to
  another's, the blob they now share may become a file, and the other
  page's HTML changes with it. A page's own bytes — its HTML as rendered,
  its CSS, its script — depend on no other page; how they are delivered
  does (plan.md, T4).

| Where the numbers come from | |
| --- | --- |
| 4096 B | what others use: Vite inlines an asset under `assetsInlineLimit`, 4096 B by default, and Astro's `build.inlineStylesheets: "auto"` (its default since 3.0) inlines a stylesheet under that same limit |
| 250 B gzipped — the rule it replaces | the byte break-even of one request, from our own measurement, not from a published figure: a cached file costs its bytes once and one request's headers (180–330 B over HTTP/2, measured in research/js-behaviours.md; 228 B on GitHub Pages), an inlined blob costs its bytes on every page — so for a visitor's second page a file is cheaper once the blob is larger than a request: `(k − 1) × S > 250` for the `k` pages of a visit, taken at `k = 2`. It counts bytes only: a linked stylesheet also costs the first view a round trip, which no byte rule sees |
| 14 KB | web.dev's guidance for the first view is a budget, not a threshold: keep what the first render needs within the first round trip, about 14 KB compressed (ten TCP segments) |

On the docs site nothing is a file under `auto`: a layout's CSS is one blob
only on pages whose pruned sheets are equal to the byte, and the site's four
are not — each is inlined, and nothing of them is cached from page to page
(the OPEN below).

> **Deferred by the owner until the design system is ready** (2026-10-09:
> "I don't want to fine-tune it until the design system is ready. For now I
> just need a solution that works"). Packaging stays as this section has
> it — a blob is shared where two pages' are the same bytes — and the
> study below is kept for the day the proper test can be made: with all the
> components in place.
>
> OPEN, then (decisions.md, K): **the factoring policy.** Sharing
> is by content hash, so two pages whose sheets share 99% are two blobs,
> each inlined, and nothing is cached from page to page: over a visit the
> control, whose one sheet is a file, transfers less (bet.md). The owner's
> rules (2026-10-06) settle whose problem it is — packaging's, for CSS and
> JS alike, with analysis left exact — and not the policy. The study for it
> is `bench/factor.mjs` → `bench/results/factor.md`: the same analysis under
> ten packagings, on both sites.
>
> What the numbers show (brotli; `bench/results/factor.md` has raw and gzip,
> CSS alone and JS alone, and every table whole). **Cold and the visit pull
> apart**: today's packaging is the lightest cold page, in one request, and
> leaves nothing in the cache; every shared file costs the cold page a
> request and, unless the file is exact for it, bytes. **Over a visit that
> reaches every page one sheet of the union is the floor, and the control's
> sheet is that union but for the rules no page uses** (4 of its 111 units
> on the docs site, 16 of 387 on the catalog): on the catalog nothing but
> (e) — the control without those rules — transfers as little as the
> control, and the candidates that keep T5 lose the visit by 17% to 45%; on
> the docs site (b), (c), (d, whole) and (e) do transfer less, by 0.1% to
> 6.9%, because the control's script is under 4096 B and so in every page.
> **No candidate meets T5, T8 and the visit together, on either site** —
> awareness can win a long visit only where the design system is larger
> than what the site uses, and neither fixture is that. And only (b′), (d)
> and (e) keep the cascade by construction: (b) and (c) put a rule after
> ones it preceded, and need the pruner to say that no element matches both
> — it reports only kept or dropped today.
>
> | Packaging — CSS and JS alike | A page fetches only what it needs | The cascade | Docs site: cold page, mean | requests | session | Catalog: cold page, mean | requests | session | T5, docs / catalog | T8 | The visit |
> | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |
> | (a) today: everything inlined | yes | as written | 7,585 | 2 | 29,871 | 4,454 | 2–3 | 43,305 | fail / pass | pass / pass | fails / fails: the control transfers 12.7% / 44.7% less |
> | (b′) the rules every sheet starts with, as one file | yes | by construction: a prefix | 7,830 | 4 | 29,446 | 4,606 | 4–5 | 42,349 | fail / pass | fail / fail | fails / fails: 11.5% / 43.4% |
> | (b) needed by every page → one sheet, one script; the rest inlined | yes | **needs a proof** (45 and 53 pairs of rules) | 7,803 | 4 | 24,992 | 4,842 | 4–5 | 33,290 | fail / pass | fail / fail | **holds**, 4.1% less / fails: 28.0% |
> | (c) needed by ≥ 2 pages | no: 0.9 KB and 9.1 KB of CSS a page, mean | **needs a proof** | 7,918 | 4 | 24,256 | 6,588 | 4–5 | 26,348 | fail / fail (JS) | fail / fail | **holds**, 6.9% / fails: 9.1% |
> | (c) needed by ≥ 3 | no: 3.4 KB on the catalog (on the docs site it is (b)) | **needs a proof** | 7,803 | 4 | 24,992 | 5,432 | 4–5 | 28,917 | fail / pass | fail / fail | **holds**, 4.1% / fails: 17.2% |
> | (c) needed by ≥ half | no: 0.9 KB and 0.5 KB | **needs a proof** | 7,918 | 4 | 24,256 | 4,950 | 4–5 | 31,298 | fail / pass | fail / fail | **holds**, 6.9% / fails: 23.5% |
> | (d) a file per component and per module, whole | no: 0.2 KB and 4.3 KB | by construction: source order | 8,471 | 8–11 | 25,149 | 6,820 | 8–20 | 29,394 | fail / fail (JS) | fail / fail | **holds**, 3.5% / fails: 18.5% |
> | (d) … each page's own part, a file where two pages' are the same bytes | yes | by construction | 8,285 | 7–8 | 28,148 | 5,191 | 5–9 | 38,688 | fail / fail (JS) | fail / fail | fails / fails: 7.4% / 38.1% |
> | (e) one sheet — the union — and one script | no: 1.0 KB and 20.3 KB | by construction: source order | 8,210 | 3 | 26,032 | 8,858 | 4–5 | 23,771 | fail / fail | pass / fail | **holds**, 0.1% / **holds**, 0.8% |
> | (f) the control | — | — | 8,242 | 3 | 26,064 | 9,041 | 4–5 | 23,953 | — | pass / fail | — |
>
> With the scripts left inlined — every script (b) and (c) would share is
> under the 4096 B of `auto` — each of (b′), (b) and (c) is one request
> fewer: 3 on the docs site, where T8 then passes, and 3 on the catalog but
> on `/`, whose picture makes it 4. Their sessions move by under 4%, and no
> verdict on the visit changes. (e) with each page's own script inlined is
> 24,282 B on the docs site and 25,355 B on the catalog — 5.5% more than
> the control there, whose script is a file.
>
> The thresholds are plan.md's as written (T5: what a page fetches cold
> against the control's, each blob on its own; T8: ≤ 3 requests cold). "The
> visit" — no more than the control over the session — is not a threshold
> of plan.md: the owner named it as a target (decisions.md, K, rule 4), and
> whether it becomes one is open (plan.md, RGP2-050).
>
> Not designed here, and nothing of it is built: the policy is the owner's
> next decision.

**Rejected:** `(pages − 1) × bytes > 1024` (the first draft: no unit, and
`pages` was the site's — the 176 B of `overlays` were a file on a 200-page
site and inline on a 4-page one, for the same visitor); *over 250 B gzipped*
(the rule until 2026-10-05: a break-even of our own measuring, in bytes
alone — a sheet of 1,074 B that two pages share was a file, and its request
a round trip of the first view).

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

> Ruled by the owner (decisions.md, C): dynamic segments fall back to their
> imports — for CSS and JS, as layout.md's *Delivery* has it — and are
> otherwise out of scope here: `pathname()` under a base in React
> (`location.pathname` carries it), the record, the mount entry, pruning
> (*Not in phase 2*, the first row) wait for them.

**Rejected:** the base written into the links by the author (the source is
then built for one base, and shell code has no way to learn it: `pathname()`
is the route); checking links with the base stripped and passing those
outside it as "another site of the origin" (with links written from the
root, that is every link: the check is off whenever `--base` is set).

## The report

`<out>/_rg/report.json`, and `--report` prints it: per page — a variant of a
route: `pathname`, `variant`, `path`, its module and its document — **what it
needs**, the bytes of its HTML, CSS and JS (raw and gzip — brotli is the
bench's, plan.md RGP2-050: Go has none), and **what it fetches**: the
document as written and the files it links, with how each blob is delivered
(*Analysis and packaging*); the components rendered and the classes their
`variants()` resolved, the behaviours mounted with their flags and their data, and per
behaviour module its bytes in the page's script (from esbuild's metafile) —
*why is this byte here*. The rows add up to the script: the mounted modules,
the files they import, the generated entry (`<entry>`), and what is of no
input — the helpers esbuild adds for a dynamic `import()` — as `<runtime>`.
CSS: rules kept and dropped per source file, and of the page's own
`<style>` elements; for a page that was not pruned, why (*the page has a
`<template>`*).

```
/account/guest.html  pages/account/guest.rtsx
                  raw     gzip
  HTML           1129      554
  CSS            2635     1001   inline
  JS              563      328   inline, the same on 2 pages
  document       4373     1809   account/guest.html
  fetches        4373     1809   1 request: what the page needs, and no more
  …

/actions/  pages/actions/index.rtsx
                  raw     gzip
  HTML           1393      628
  CSS            3773     1276   inline
  JS             1448      708   inline
  document       6660     2510   actions/index.html
  fetches        6660     2510   1 request: what the page needs, and no more
  components ActionsPage ×1, Button ×1, Dialog ×1, DropdownMenu ×1, Each ×1, Layout ×1, MenuItem ×3
  classes    DropdownMenu: rg-menu
             Button: rg-button
  behaviours @reactogenic/ui/behaviors/overlays
             @reactogenic/ui/behaviors/menu-keys #actions RG_MENU_TYPEAHEAD=true {"typeahead":true}
             @reactogenic/ui/behaviors/invokers
  JS bytes      248 B  @reactogenic/ui/src/behaviors/overlays.ts
                832 B  @reactogenic/ui/src/behaviors/menu-keys.ts
                307 B  @reactogenic/ui/src/behaviors/invokers.ts
                 61 B  <entry>  the mount calls
  CSS rules    1 kept,   0 dropped  @reactogenic/ui/src/tokens.css
               3 kept,   1 dropped  @reactogenic/ui/src/button.css
              13 kept,   1 dropped  @reactogenic/ui/src/dialog.css
               5 kept,   2 dropped  @reactogenic/ui/src/dropdown-menu.css
               0 kept,  25 dropped  @reactogenic/ui/src/side-menu.css
               5 kept,  10 dropped  site.css

css  6c082bb9     3773     1276   inline                   /actions/
js   560fac5c      563      328   inline                   /account/guest.html /dialog/
css  e28e8351     1074      502   inline                   /guide/ /guide/more/
```

```
/links/  pages/links/index.rtsx                 ← --inline never: the same needs, fetched in three requests
                  raw     gzip
  HTML            814      441
  CSS            2302      888   /_rg/page-d236f53c.css
  JS              252      179   /_rg/page-2f3112a3.js, the same on 2 pages
  document        926      502   links/index.html
  fetches        3480     1569   3 requests: what the page needs, and no more
  components Button ×1, DropdownMenu ×1, Each ×1, Layout ×1, LinksPage ×1, MenuItem ×3
  classes    DropdownMenu: rg-menu
             Button: rg-button rg-button-ghost
  …

/links/  pages/links/index.rtsx                 ← --no-specialize: the control decides nothing per page
                  raw     gzip
  HTML            814      441
  CSS            7523     2082   /_rg/site-a05e499c.css, the same on 10 pages
  JS             1730      855   inline, the same on 10 pages
  document       2628     1295   links/index.html
  fetches       10151     3377   2 requests
  …
```

| | |
| --- | --- |
| **needs** — HTML, CSS, JS | analysis: the page alone — with its doctype and the base, without what packaging puts in it — its own sheet and its own script. They do not depend on `--inline`, nor on any other page |
| **fetches** | packaging: what a cold load of the page fetches of the build's own files — the document and each file it links, each compressed on its own — and the number of requests. In the JSON, `fetches`: `raw`, `gzip`, `requests`, `files` (the URLs), and `css` and `js`: the styles and the script among it, inlined or in a file, the page's own or shared |
| needs against fetches | the row says what a page fetches over what it needs: *what the page needs, and no more* — always, as the builder packages today (*Packaging*: a blob is shared only where it is the same bytes) — or *N B of CSS and M B of JS more than the page needs*, once a shared file holds another page's rules. The control decides nothing per page: its row has the requests alone |
| classes | the classes the page's `variants()` calls resolved (*What shell code can ask the builder*), by the components that resolved them, in the order first resolved: a component's root and its variants' own classes. In the JSON, `classes`: `name`, `by`. A class written as a literal is not there: the HTML has it |
| document | the file as written: what the request for the page carries. With a blob inlined, the page and the blob |
| the first line | the document's path and the variant's module: the route's pathname for `index`, the file for another variant (*Routes*). In the JSON: `pathname` (the route), `variant`, `path`, `file`, `output` |
| a behaviour | a module and the element it is mounted on, once, however many components mounted it, with the flags any of them turned on and the data its call is given (`mounts[].data` in the JSON) |
| the last lines | the site's blobs: kind, hash, raw, gzip, `inline` or its file, the documents it serves, by path |
| CSS rules | of the sheet before it is minified; a file whose rules all went is a row: that is the saving. A file's bytes (`bytesIn`, `bytesOut` in the JSON) are its rules', before and after pruning. The last row, `<style>`, is the page's own `<style>` elements, together: their bytes are in the page's HTML, not in its CSS |
| a page that is not pruned | one line in place of the rows, with the reason: ``not pruned: the page has an element the user edits (`contenteditable`)``, ``not pruned: the page has a document of the site in a frame (`<iframe>`)``, ``not pruned: the page's script may change the tree: it names `append` `` — the word to look for in the behaviour (*CSS*). In the JSON: `styles.unpruned`, and the reason in `styles.why` |
| a blob's delivery | `inline`, or its file. Under `auto` a blob that pages share is a file when its raw column is 4096 or more: the script of `/account/guest.html` and `/dialog/` above and the sheet of the two guide pages are under it; the control's one sheet — 7,523 B, on every page — is over (*Packaging*) |
| files | the project's own, from the project directory: `site.css`. A package's, by the package and the path in it: `@reactogenic/ui/src/dialog.css` — whether the install put it in `node_modules/`, in pnpm's store or behind a link out of the project. Nothing in the report is the machine's: two builds of one input write the same report, on any checkout |
| a file of no package, outside the project | where it is, from the project directory: `../shared/tokens.css` |
| gzip | DEFLATE at level 9, as Go's `compress/gzip` writes it: within 1% of what `gzip -9` (zlib) gives — on the docs site from 76 B below, on its largest page's HTML, to 3 B above, on a 252 B script (`bench/site.mjs`). The measuring scripts have the reference numbers |
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
| CSS | per page, pruned against the page as served and its script | one sheet for the site: the unpruned bundle of everything any page imports |
| JS | per page: the modules it mounted, its flags, each mount's data in its call | one script for the site: every behaviour mounted on any page, every flag on, and a table from a document's path to that document's mounts — module, element, data |

```js
// the control's entry: every flag is defined `true`
import m0 from "@reactogenic/ui/behaviors/overlays";
import m1 from "@reactogenic/ui/behaviors/menu-keys";
const m = [m0, m1], t = {
  "/": [[0]],
  "/syntax/": [[0], [1, "m1"], [1, "m2", {"typeahead":true}]],
  "/account/guest.html": [[0]],
};
for (const [i, id, d] of t[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/|(\.html))?$/, (all, index, file) => file || "/")] || [])
  id ? m[i](document.getElementById(id), d) : m[i]();
```

| | |
| --- | --- |
| the table's keys | a document's path on a static host (*Routes*): the route's pathname for `index` — `/account/` — the file for any other variant — `/account/guest.html`. A page that mounts nothing is not in the table |
| what a document answers to | an `index`: `/syntax/`, `/syntax` and `/syntax/index.html` — as its own script does, which is in the page wherever it is served. Any other variant: its file, `/account/guest.html`, and nothing else — what ends in `.html` and is no `index.html` is looked up as it is |
| `--base` | in the keys (`/docs/syntax/`), in the names of the directories: `location.pathname` is what the browser encodes of them (`/se%C3%B1or/` for `pages/señor/`), so it is decoded (a pathname that is not valid percent-encoding throws: it is no page of the site). The base is decoded for the keys too: `--base /caf%C3%A9/` and `--base /café/` are one directory, written into the links as given and keyed `/café/…`; a base that does not decode is a usage error |
| a mount's data | the third member of its row, handed to the behaviour as the default build's call hands it; a row without it calls with `undefined` |
| **what the control cannot do** | know its document by anything but the URL. A server that serves several artifacts of a route under one URL — the variants' router (*Routes*: `server.ts`) — is outside it: at `/account/` the control would run the mounts of `index` on whichever variant was sent. The default build has no such limit: a page's script is the page's |

- Every page gets both blobs — also a page that mounts nothing, or imports
  no stylesheet: the control does not know. They are packaged by the same
  rule (`--inline`), as `_rg/site-<hash>.css` and `.js` when they are
  files: under `auto` a sheet or a script of the whole site under 4096 B is
  inlined in every page.
- The CSS goes through the same steps but the pruning: bundled, nesting
  lowered, minified.
- The mount-* reports are those of the default build, page by page.
- The table is the control's own cost: it grows with the site and is in
  every page's script. The measurement reports its bytes apart (plan.md,
  RGP2-050), so that T5 does not divide by it.

The difference between the two builds is what component awareness is worth
(plan.md, RGP2-050).

The control is **analysis turned off**, not a packaging: its one sheet is
everything any page imports, used or not. One sheet of what the pages
*need* — the union of exact per-page analysis — is a packaging, and the
study has it as (e) (*Packaging*, OPEN).

## Not in phase 2

| | |
| --- | --- |
| dynamic segments, `Dynamic`, `<template>` delivery, holes | layout.md stands, and the engine already renders what React renders. To be redone with them, not carried over: CSS pruning (what a dynamic segment renders is not in the page's HTML, and a `<template>` turns pruning off for its page — research/css.md has the fallback: every rule of a component a dynamic segment imports stays, a template's content matches as "maybe"); the generated entry and the record (*Behaviours*); shell-handler when an element is *made* (`<Dynamic><button onClick={…}>` is made in shell code) |
| a server, and a route's `server.ts` | every variant is built; nothing selects among them at request time, and a static host serves `index.html` (*Routes*) |
| a factoring policy | packaging shares a blob only where two pages' are the same bytes; what it should factor beyond that is open with the owner (*Packaging*, OPEN), with a study and no build |
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
| page-author raw JS | a script of the page's own is shell-script; whether it is ever allowed is deferred (CLAUDE.md, *Deferred*) |
| a check of what a behaviour writes | the pruner reads the page's script for the names; a computed name is not seen (*CSS*, *The page's script*) |
| asset pipeline (images, fonts), `public/` | a `public` directory next to `pages` is copied as is — its files, a link to a file as the file; nothing is processed. A link to one of its files is a link to a file of the output (*Checks*); a `url()` in CSS is left as written |
