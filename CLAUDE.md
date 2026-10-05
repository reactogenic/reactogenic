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

The builder: `reactogenic build` compiles pages in `.rtsx` plus the layout
components of `@reactogenic/ui` (`SideMenu`, `Dialog`, `DropdownMenu`) into
per-page plain HTML + CSS + minimal JS, **no React in the output**; the proof
is a four-page docs site (`site/`) and a measured bet — thresholds T1–T8 in
`plan.md` (RGP2-050), the result in `specs/phase02/bet.md`: **undecided**.
Zero React and every refuting threshold hold; what component awareness adds
over the control (`--no-specialize`) is small on a site of three components
in one layout, and over a four-page visit the control transfers less.
No islands, no dev server, no view transitions.

- `research.md` — esbuild is the linker (public Go API, in-process, never
  forked); our compiler sits in front and executes pages in an embedded
  engine (`modernc.org/quickjs`) with React's own static renderer.
- `builder.md` — the pipeline: routes, the record of execution, page checks,
  CSS pruned per page, behaviours (`mount()` + `Define` flags), packaging,
  the report, the control (`--no-specialize`).
- `components.md` — the three components (and `Button`) on platform
  primitives.
- `decisions.md` — what was decided, what is **for review**, what waits
  *For the owner* (A–M), the binary's size.
- `plan.md` — tasks `RGP2-xxx`, each with what was measured, what was not
  done and what was not verified.

What exists:

- `go/internal/build` — the driver: `build.Main` is the command, `build.Run`
  the build. A stage per package under it: `render` (the render bundle;
  each page executed in a runtime of its own), `pagecheck` (ids, references,
  commands, links), `cssprune` (a page's CSS against the page as served),
  `behaviors` (a page's script from its `mount()`s; the control). Each
  package's doc comment says what it guarantees.
- Its tests build fixture sites (`testdata/`: the whole output golden in six
  modes — `-update` rewrites it, and cssprune's corpus reads it) and `site/`
  itself. They need `pnpm install` at the root and `node`.
- `packages/core` — `pathname`, `useShellId`, `mount`: what a component asks
  the builder. `packages/ui` — the components in `.rtsx`, their CSS, the
  behaviours (`src/behaviors/`); private, not published. Browser suites
  (Playwright, Chromium and WebKit): `pnpm --filter @reactogenic/ui
  test:browser`, `pnpm --filter @reactogenic/site test:browser`.
- `site/` — built by `reactogenic build` (`$REACTOGENIC_BINARY`:
  `site/README.md`); CI's `site` job builds it, fails on any diagnostic and
  keeps the byte report. No `<script>`, no hand-written JS, no list of
  styles or behaviours in its source (T6).
- `bench/measure.mjs` — what a page costs the browser; `bench/baselines/`.
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

Out of phase 1: rules of layout, shell / islands / `Dynamic`, shell
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
- **Shell**: compiled **once** per pathname into plain, precise HTML + CSS +
  raw JS. **No React in the shell**, nothing conditional in it. Identical
  across documents so cross-document View Transitions handle navigation.
- **Islands**: marked explicitly with `<Dynamic>` — never inferred. One
  `Dynamic` = one island = one separate React root/app with its own bundle.
  Islands take only compile-time values from the shell (exception: `Form`
  handlers) and share state through the URL or the persistent-state layer
  (`sessionStorage` + own reactive store + `useSyncExternalStore`).
- **Slots** are the organizing concept: a slot is data plus an optional render
  function (`{ ...options, children }`), never a component. Rule of thumb:
  bare name = pass my value in; braces = give me your value out.
- **Shell components** (`Dialog`, `Form`, …): React-less HTML/CSS/JS with an
  imperative API, usable from islands via ordinary slot syntax. Their slot
  bodies compile to `<template>`s with **holes**; `{expr}` in such a body is a
  hole (`≡ <Dynamic>{expr}</Dynamic>`): primitive → text, JSX → React portal
  (from an island) or root (from the base layout).
- **Compiler**: Go, forked from the tsgo (TypeScript 7) parser **and
  type-aware** (embeds the checker). Emits plain `.tsx` for TS7 to type-check,
  Go templates for shells, per-route CSS; embedded esbuild; Go server. Types
  drive diagnostics and the language service; the transpiler itself is purely
  syntactic — types never change emitted code.
- The design system's core catalog is authored in plain `.tsx`, executed by
  the compiler in shell code and rendered by React in islands.
- Islands own their data via `useQuery(key, queryFn)` — React Query
  embedded, exposed as that hook. **No `Await` or other data-flow element**:
  `<Switch on={query.status} exhaustive>` with `$Case`s is all the flow
  control there is. No "empty" state — emptiness is the user's business.

## Open with the owner

Phase 2 was decided while the owner was away
(`specs/phase02/decisions.md`), and some of it reads against this file.
Nothing here is rewritten for it: the statements of this file stand as
written, the phase 2 code is as decisions.md has it, and which side gives
way is the owner's to rule on — the rows marked **for review**, and the
list *For the owner*:

- **"The design system's core catalog is authored in plain `.tsx`"** —
  `@reactogenic/ui` is authored in `.rtsx` (decision 5, **for review**).
- **"Go templates for shells"** — no `text/template` in the binary; HTML is
  written as strings (decision 6, **for review**).
- **"Loop-produced slot items (`Each` around slot elements — phase 2)"**
  (*Deferred*) — no task of `plan.md` has them; the site writes its menu out
  (*For the owner*, A).
- **"Loops over constants in the shell"** (*Deferred*) — phase 2 executes
  shell code, so they work: S2 reads "no *runtime* variance" (decision 4,
  **for review**).
- Also for review: 2 (the embedded engine), 11 (the browser floor), 16
  (`pathname()` is public), 17 (a dialog in shell code is a live `<dialog>`,
  not a `<template>`). *For the owner*, B–J: `children` vs `$Contents`, what
  islands need, integer-like keys, a page's own `<script>`, T2's budget, the
  close button's name, a dialog under a popover, disabled menu items, links
  into the current page. K–M, from the measurement: a shared sheet plus each
  page's rest, T5 and the verdict, state nothing can reach.

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
   selected at the attachment by its `key`. A function slot run per item is
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

Parked in `specs/later/`: `layout.md` (shell vs island rules, `Dynamic`,
shell components, `Form` / `$Field`; notes mark where phase 2 reads it
otherwise), `persistent-state.md`.

## Rejected (do not propose again)

Svelte's `{value}` shorthand; slot *components*; a parent `Switch` around
`Match`; `#about-us.rtsx` file names; `"use client"` / `"use dynamic"`
directives; `DynamicForm`; inferring the shell/island boundary; an `Await`
element or any semantic wrapper around query state (`Switch` is enough); an `empty`
state / `$Empty` slot / `isEmpty` rule anywhere in the syntax — "empty" is
opinionated (`[]`? `""`? `{ items: [] }`?) and stays in user code.

## Deferred / roadmap

Loop-produced slot items (`Each` around slot elements — phase 2); lazily
loaded segments; loops over constants in the shell; page-author raw JS.

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
| `phase02/plan.md`, `phase02/decisions.md` | RGP2-001–060 done. Decisions **for review** and *For the owner* (A–M): not ruled on |
| `phase02/bet.md` | the measured bet: undecided (T5 fails); re-run with `node bench/site.mjs`, `bench/delta.mjs`, `bench/verify.mjs` |
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
