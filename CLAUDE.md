# Reactogenic — context for working in this repo

You are working in `reactogenic/reactogenic`, a monorepo for a React-based UI
framework. Current task: phase 2 (the builder), following
`specs/phase02/plan.md` task by task (`RGP2-xxx`); phase 1 is released
(`specs/phase01/plan.md`, `RGP1-xxx`). Specs come first: settle a task's
spec, then implement.

Repo layout (see `specs/phase01/decisions.md`, RGP1-003): `go/` — our Go
module (`go/internal/`, `go/cmd/reactogenic/`), vendored tsgo in
`go/third_party/tsgo` (never edit without adding a patch to `go/patches/`);
`packages/` — pnpm workspace, every package scoped `@reactogenic/*` except
`packages/vscode` (named `rtsx`, private: `vsce` rejects scoped names — the
extension `reactogenic.rtsx`); `site/` — the docs site, a private workspace
package; `bench/` — the measuring script and its baselines; `go.work` and
`package.json` at the root tie them together. Run Go commands on
`github.com/reactogenic/reactogenic/go/...`, not `./go/...`. tsgo is reached
only through its `rtsx` bridge package.
Conformance: `go/internal/conformance` runs the syntax.md examples plus
`fixtures/`, with a ratchet in `testdata/passing.txt` (`fixtures/README.md`).

The specs are the source of truth. When this file and a spec disagree, the
spec wins; update this file.

## Current phase: phase 2 (`specs/phase02/`)

The builder: `reactogenic build` compiles pages in `.rtsx` — the variants of
the routes: a directory under `pages/` is a route, an `.rtsx` file of it that
nothing mounts or imports is a variant, built to a document (`index.rtsx` →
`index.html`, `guest.rtsx` → `guest.html`); the output files are the
artifacts — plus the layout components of `@reactogenic/ui` (`SideMenu`, `Dialog`, `DropdownMenu`) into
per-page plain HTML + CSS + minimal JS, **no React in the output**; the proof
is a four-page docs site (`site/`) and a measured bet — thresholds T1–T8 in
`plan.md` (RGP2-050), the result in `specs/phase02/bet.md`: **it holds for
a page loaded cold, and does not hold over a visit.** Zero React and every
refuting threshold hold. What component awareness adds over the control
(`--no-specialize`) is small on the docs site (three components in one
layout) and large on a catalog of twenty (`bench/catalog-site`, a fixture:
CSS 42–73% and JS 42–92% smaller per page, brotli) — but per-page sheets
share nothing, so over a visit the control transfers less from the second or
third page (*For the owner*, K: open). The owner has not chosen the
verdict's word.
No dynamic segments, no dev server, no view transitions.

- `research.md` — esbuild is the linker (public Go API, in-process, never
  forked); our compiler sits in front and executes pages in an embedded
  engine (`modernc.org/quickjs`) with React's own static renderer.
- `builder.md` — the pipeline: routes and variants, the record of
  execution, page checks (a script of the page's own is `shell-script`; a
  `<link>` is what its `rel` says), CSS pruned per page — its own `<style>`
  elements with it — behaviours (`mount()`: `Define` flags for the page,
  data for the use site), packaging (a shared blob is a file from 4096 B),
  the report, the control (`--no-specialize`).
- `components.md` — the three components (and `Button`) on platform
  primitives.
- `decisions.md` — what was decided, what is **for review**, the list *For
  the owner* (A–M) with its rulings — A and K are open — the binary's size.
- `plan.md` — tasks `RGP2-xxx`, each with what was measured, what was not
  done and what was not verified.

What exists:

- `go/internal/build` — the driver: `build.Main` is the command, `build.Run`
  the build. A stage per package under it: `render` (the render bundle;
  each page executed in a runtime of its own), `pagecheck` (ids, references,
  commands, links, scripts of the page's own), `cssprune` (a page's CSS
  against the page as served), `behaviors` (a page's script from its
  `mount()`s; the control), `markup` (what runs, a `<link>` by its `rel`:
  the one notion the stages share). Each package's doc comment says what it
  guarantees.
- Its tests build fixture sites (`testdata/`: the whole output golden in six
  modes — `-update` rewrites it, and cssprune's corpus reads it) and `site/`
  itself. They need `pnpm install` at the root and `node`.
- `packages/core` — `pathname`, `useShellId`, `mount`: what a component asks
  the builder. `packages/ui` — the components in `.rtsx`, their CSS, the
  behaviours (`src/behaviors/`). Not published yet: `"private": true` in its
  manifest is the flag that keeps it off npm, no more — the aim is public,
  under a branded name the owner will pick (`@reactogenic/ui` is a working
  name). Browser suites
  (Playwright, Chromium and WebKit): `pnpm --filter @reactogenic/ui
  test:browser`, `pnpm --filter @reactogenic/site test:browser`.
- `site/` — built by `reactogenic build` (`$REACTOGENIC_BINARY`:
  `site/README.md`); CI's `site` job builds it, fails on any diagnostic and
  keeps the byte report. No `<script>`, no hand-written JS, no list of
  styles or behaviours in its source (T6).
- `bench/` — `measure.mjs` (what a page costs the browser), `baselines/`;
  the bet's scripts: `site.mjs`, `delta.mjs`, `verify.mjs` on the docs site,
  `catalog.mjs`, `catalog-verify.mjs` on `bench/catalog` (twenty components,
  sixteen of them measurement fixtures — not the design system) and
  `bench/catalog-site` (ten pages); results in `bench/results/`.
- Never link `text/template` / `html/template` into the binary: +18.6 MiB.
- Docs for users: *Build* in `docs/getting-started.md`; `CHANGELOG.md`,
  *Unreleased*.

## Phase 1 (`specs/phase01/`) — released

Scope, and nothing else:

1. **`.rtsx` runnable by Vite** — a plugin transpiles `.rtsx` → `.tsx`; the
   rest is Vite + plugin-react as usual.
2. **TS errors mapped back to `.rtsx`** — `reactogenic check` type-checks the
   emitted `.tsx` and reports every error on the source, in slot terms.

3. **IDE support** — language server, syntax highlighting, VS Code
   extension (`specs/phase01/ide.md`).

Out of phase 1: rules of layout, shell / dynamic segments / `Dynamic`, shell
components, `Form`, persistent state, shell compilation, server, routing. The transpiler itself **is** Go from day 1: a tsgo fork
with TS7's checker in-process, which the Vite plugin drives as a long-lived
process (`specs/phase01/decisions.md`, RGP1-001). Those specs are parked in `specs/later/`; do not pull them
into `specs/phase01/`. The architecture below is the long-term target.

## What Reactogenic is

A **compiled** framework with four tightly coupled pillars: a compiler, a
design system (UIKit-inspired: containers with fixed roles + controls), a
server-side router, and an extended TSX syntax with the `.rtsx` file extension.

Positioning: "Reactogenic apps are built from framework-owned components with
typed slots. Your code lives in the slots; the compiler owns everything around
them, so each screen ships exactly the HTML, CSS and JS it needs."

## Architecture decisions (fixed — do not relitigate)

- **Navigation is server-only.** The pathname defines the page; every
  navigation is a new document. Query string and hash are in-page state.
- **Shell**: compiled at build time into plain, precise HTML + CSS + raw JS.
  **No React in the shell**, and **no browser-time variance**: every
  possible shell variant is materialized at build time; the server's router
  may select between prebuilt variants (`isAuthenticated()` → `public.html`
  / `private.html`) and neither renders nor changes them. Identical across
  documents so cross-document View Transitions handle navigation.
- **Dynamic segments**: marked explicitly with `<Dynamic>` — never inferred. One
  `Dynamic` = one dynamic segment = one separate React root/app with its own bundle.
  Dynamic segments take only compile-time values from the shell (exception: `Form`
  handlers) and share state through the URL or the persistent-state layer
  (`sessionStorage` + own reactive store + `useSyncExternalStore`).
- **Slots** are the organizing concept: a slot is data plus an optional render
  function (`{ ...options, children }`), never a component. Rule of thumb:
  bare name = pass my value in; braces = give me your value out.
- **Shell components** (`Dialog`, `Form`, …): React-less HTML/CSS/JS with an
  imperative API, usable from dynamic segments via ordinary slot syntax. Their slot
  bodies compile to `<template>`s with **holes**; `{expr}` in such a body is a
  hole (`≡ <Dynamic>{expr}</Dynamic>`): primitive → text, JSX → React portal
  (from a dynamic segment) or root (from the base layout).
- **Compiler**: Go, forked from the tsgo (TypeScript 7) parser **and
  type-aware** (embeds the checker). Emits plain `.tsx` for TS7 to type-check,
  Go templates for shells, per-route CSS; embedded esbuild; Go server. Types
  drive diagnostics and the language service; the transpiler itself is purely
  syntactic — types never change emitted code.
- The design system's core catalog is authored in `.rtsx` — its first real
  production test — executed by the compiler in shell code and rendered by
  React in dynamic segments. If there is a real need, a transpiled `.tsx`
  version can be published beside it.
- Dynamic segments own their data via `useQuery(key, queryFn)` — React Query
  embedded, exposed as that hook. **No `Await` or other data-flow element**:
  `<Switch on={query.status} exhaustive>` with `$Case`s is all the flow
  control there is. No "empty" state — emptiness is the user's business.

## Open with the owner

Phase 2 was decided while the owner was away
(`specs/phase02/decisions.md`). Ruled since (2026-10-05, *Ruled by the
owner* there), and built: esbuild stays; pages are executed in an embedded
engine, each in a runtime of its own; React is a build-time dependency for
the foreseeable future; the shell has no browser-time variance (above); an
island is called a **dynamic segment**; the design system is authored in
`.rtsx`, to be public under a branded name the owner will pick; awareness is
the record of execution; CSS is pruned per page, a page's own `<style>`
with it, and a `<link>` is classified by its `rel`; routes are directories
and their variants `.rtsx` files nothing imports; a use site's own values
are the mount's data; a shared blob is a file from 4096 B; a page with a
script of its own is an error; the browser floor; the current page is
marked by the design system, the compiler only offers `pathname()`; theme,
search and code samples as they are. Still the owner's to rule on — the
rows marked **for review**, and of the list *For the owner* A and K:

- **"Go templates for shells"** (*Compiler*, above) — no `text/template` in
  the binary; HTML is written as strings. Agreed; what carries shells on a
  server is too early to decide (decision 6).
- **Packaging** — blobs that differ by a rule share nothing: a discussion of
  its own (`builder.md`, *Packaging*, OPEN; *For the owner*, K).
- **"Loop-produced slot items (`Each` around slot elements — phase 2)"**
  (*Deferred*) — no task of `plan.md` has them; the site writes its menu out
  (*For the owner*, A).
- **The router** — a route's `server.ts` selects among its artifacts; out of
  scope, and not designed: its signature, how it names an artifact, whether
  shell code may read a dimension itself (`builder.md`, *Variants*, OPEN).
  It arrives with the server.
- **Page-author JS** — deferred: until it is decided a script of the page's
  own does not build (decision 19).
- Not final: 16 (`pathname()` is public: "common sense says yes"), 17 (a
  dialog in shell code is a live `<dialog>`: for test purposes only — later
  the design system's decision, and the owner will bring a generic "portal"
  idea). For review: 4's other half (context in shell code).
- *For the owner*, A–M: ruled (2026-10-05) but for **A** (above: the owner
  asked back) and **K** (packaging, above). Built: D — keyed slots keep the
  written order, an entry's property name is its key encoded and a container
  iterates `slotKeys($X)`, never `Object.keys` (`phase01/syntax.md`, *Keyed
  slots*); G — `closeLabel` on `Dialog` and `SideMenu`; M — state only a
  script can write (`aria-*`, `disabled`, `data-state`, …) is decided on the
  page unless the page's script names it (`builder.md`, *CSS*, *Runtime
  state*). The design system's, later, with no compiler change: B
  (`children` or `$Contents`), G (a prop or a slot), H (a dialog under a
  popover), I (disabled menu items). Out of scope: C (dynamic segments fall
  back to their imports), J (links into the current page). As built: E (a
  frame of the site: legal, unpruned), F (T2 is an absolute budget). L (T5):
  measured again on a catalog of ~20 components — `plan.md`, RGP2-050,
  `bet.md`.

## Governing syntax rule

New meaning may only be given to forms that are **syntax errors in today's
TSX** — that is good news: such a form is safe to reserve. Forms that already
parse keep their React meaning; the three exceptions (bare attribute with a
same-named binding; a tag starting with `$` is a slot, never a component;
`slot={$X}` renders a slot) are called out in the spec.
"Syntax error" means rejected by the compilers that build TSX (esbuild,
Babel): TypeScript's own parser accepts `#name` as an attribute named
`"#about-us"` — see *Segment roots* in `specs/phase01/syntax.md`.

## What is specified

`specs/phase01/syntax.md` — compilation passes, then:

1. **Shorthand props** — scope-directed: `<Input value />` → `value={value}`
   if a `value` binding is in the module's scope chain, else React's `true`.
2. **Slots** — `<$X>` constructs the prop `$X`:
   `<$IconStart className="i" { size }>…</$IconStart>` →
   `$IconStart={{ className: "i", children: ({ size }) => … }}`. Declared
   `Slot<P>` / `Slot<P, A>` (P is the complete contract, A the args of a
   function body) — one value, repeated → last wins — or `KeyedSlot<P>` /
   `KeyedSlot<P, A>`: entries by React `key` (`<$Column key="email" />`),
   in the order written (a container iterates `slotKeys($X)`), selected at
   the attachment by its `key`. A function slot run per item is
   keyed by the caller: `<$Option key={({ value }) => value} />` (inline
   arrow only; replaces the attachment's `key`). The container attaches with
   `<span slot={$X} className="default" &arg &&both={x}>fallback</span>`:
   slot props replace attachment props per prop; children are the fallback;
   `&` = arg only, `&&` = arg + prop. Recursive slots; placement: direct child
   of a component or slot element, or inside Match/Switch/Each. Params on a
   component make its `children` a callback. The transpiler is purely
   syntactic.
3. **Flow control** — `<Match on={…}>` (if; params optional) and
   `<Switch on={…} [exhaustive] [{ value }]>` with `$Case is=` / `default`
   (first match wins; no match renders nothing; `exhaustive` proven by TS7).
4. **Segment roots** — `<section #about-us />` mounts sibling `about-us`
   (first of `.rtsx`, `.tsx`, `.jsx`, `.ts`, `.js`; the import names it)
   (default export, no props). Pure syntactic sugar: an import + a nested
   element.
5. **`Each`** — plain runtime component + params; no key check (as a `for`
   loop).

`specs/phase01/vite.md` — the Vite plugin, module resolution, the one type
the transform needs no types (the transpiler is syntactic), the runtime package.
`specs/phase01/diagnostics.md` — `reactogenic check`, source-map origins,
rewrites of TS errors into slot terms.
`specs/phase01/ide.md` — IDE support: `reactogenic lsp` (the fork's TS7
language server with the `.rtsx` transform built in as a content mapper; the
same program model and diagnostics as `check`), the generated TextMate
grammar, the VS Code extension (`packages/vscode`).
`specs/phase01/plan.md` — tasks `RGP1-xxx`; `specs/phase01/decisions.md` —
one section per decided task.

`specs/phase02/` — the builder: see *Current phase* above.

Parked in `specs/later/`: `layout.md` (shell vs dynamic segment rules, `Dynamic`,
shell components, `Form` / `$Field`; notes mark where phase 2 reads it
otherwise), `persistent-state.md`.

## Rejected (do not propose again)

Svelte's `{value}` shorthand; slot *components*; a parent `Switch` around
`Match`; `#about-us.rtsx` file names; `"use client"` / `"use dynamic"`
directives; `DynamicForm`; inferring the shell/dynamic segment boundary; an `Await`
element or any semantic wrapper around query state (`Switch` is enough); an `empty`
state / `$Empty` slot / `isEmpty` rule anywhere in the syntax — "empty" is
opinionated (`[]`? `""`? `{ items: [] }`?) and stays in user code.

## Deferred / roadmap

Loop-produced slot items (`Each` around slot elements — phase 2); lazily
loaded segments; shell variants beyond the pathname (with the server);
page-author raw JS. (Loops over constants in the shell work since phase 2:
shell code is executed at build time.)

## Candidates (spec as "Candidate" sections, not yet approved)

- `$Error` slot on every container (compiler inserts the boundary).
- Two-way binding on design-system inputs, candidate form `value={=name}`.

## Deliverables

| File | Status |
| --- | --- |
| `phase01/syntax.md`, `phase01/vite.md`, `phase01/diagnostics.md` | drafted; `> OPEN:` notes inside |
| `phase01/plan.md`, `phase01/decisions.md` | RGP1-001–005 done |
| `phase01/ide.md` | implemented and released (RGP1-100–113: npm 0.1.0-alpha.1, the extension 0.1.1); 114 (re-vendor) later |
| `phase02/research.md`, `phase02/research/` | done (RGP2-001) |
| `phase02/builder.md`, `phase02/components.md` | implemented (RGP2-010–040): `reactogenic build`, `@reactogenic/ui`, `site/`; `> OPEN:` notes inside |
| `phase02/plan.md`, `phase02/decisions.md` | RGP2-001–060 done; the owner's rulings of 2026-10-05 built (plan.md, RGP2-050, *The rulings, built*). *For the owner* A–M: ruled, D, G and M built (decisions.md) — A and K open; what is **for review** or not final: not ruled on |
| `phase02/bet.md` | the measured bet, on the docs site and on a catalog of twenty components: holds for a page loaded cold, not over a visit (K, open); the verdict's word is the owner's. Re-run: `bench/README.md` |
| `later/layout.md`, `later/persistent-state.md` | parked; phase 2 reads the shell rules as in `phase02/builder.md` |
| `slot-contract.md`, `route-table.md`, `resource.md` | later |

Phase 1 open decisions: how the plugin hands TSX
to plugin-react — `phase01/vite.md`; Node ↔ Go message encoding —
`phase01/decisions.md`.

## Working style

One extension at a time, so the author can follow. Compact, precise, examples
over prose. Every desugaring as before/after `.rtsx` → `.tsx`. Open questions
inline as `> OPEN:`; rejected ideas as **Rejected:** with the reason. Do not
invent extensions beyond the lists above. Commit and
push only when asked.

Releases: `scripts/release.sh` (the order is in its header); `CHANGELOG.md`
for the npm packages, `packages/vscode/CHANGELOG.md` for the extension, whose
`README.md` is the Marketplace page (`DEVELOPMENT.md` is the developer's).

`main` is protected (ruleset `main`): changes land through pull requests —
no approvals required, squash merge only, and CI's `go`, `js` and `vendor`
must pass on a branch up to date with `main`; no force pushes or deletion.
Repository admins can bypass. Work on a branch per task (`rgp1-xxx-…`),
commit there, and open the PR when asked to push.
