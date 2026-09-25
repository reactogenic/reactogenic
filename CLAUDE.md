# Reactogenic — context for working in this repo

You are working in `reactogenic/reactogenic`, a monorepo for a React-based UI
framework. Current task: write the specifications in `specs/`. No
implementation code yet — Markdown only. Ask before creating anything outside `specs/`.

The specs are the source of truth. When this file and a spec disagree, the
spec wins; update this file.

## Current phase: phase 1 (`specs/phase01/`)

Scope, and nothing else:

1. **`.rtsx` runnable by Vite** — a plugin transpiles `.rtsx` → `.tsx`; the
   rest is Vite + plugin-react as usual.
2. **TS errors mapped back to `.rtsx`** — `reactogenic check` type-checks the
   emitted `.tsx` and reports every error on the source, in slot terms.

Out of phase 1: rules of layout, shell / islands / `Dynamic`, shell
components, `Form`, persistent state, shell compilation, server, routing,
language service. The transpiler itself **is** Go from day 1: a tsgo fork
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
  drive diagnostics and the language service; they change emitted code in one
  place only (list slots).
- The design system's core catalog is authored in plain `.tsx`, executed by
  the compiler in shell code and rendered by React in islands.
- Islands own their data via `useQuery(key, queryFn)` — React Query
  embedded, exposed as that hook. **No `Await` or other data-flow element**:
  `<Switch on={query.status} exhaustive>` with `$Case`s is all the flow
  control there is. No "empty" state — emptiness is the user's business.

## Governing syntax rule

New meaning may only be given to forms that are **syntax errors in today's
TSX** — that is good news: such a form is safe to reserve. Forms that already
parse keep their React meaning; the two exceptions (bare attribute with a
same-named binding; the `$` tag namespace) are called out in the spec.

## What is specified

`specs/phase01/syntax.md` — compilation passes, then:

1. **Shorthand props** — scope-directed: `<Input value />` → `value={value}`
   if a `value` binding is in the module's scope chain, else React's `true`.
2. **Slots** — `<$IconStart spacing="tight" { size }>…</$IconStart>` →
   `$IconStart={{ spacing: "tight", children: ({ size }) => … }}`. Terms:
   options (in), params (out), body. Types `SlotFn` / `OptionalSlotFn` /
   `renderSlot`. Params on a component make its `children` a callback. List
   slots (`$X: {…}[]`, type-directed array emit). Conditional slots
   (`Match` around a slot element → ternary prop).
3. **Flow control** — `<Match on={…}>` (if; params optional) and
   `<Switch on={…} [exhaustive] [{ value }]>` with `$Case is=` / `default`
   (first match wins; no match renders nothing; `exhaustive` proven by TS7).
4. **Segment roots** — `<section #about-us />` mounts sibling `+about-us.rtsx`
   (default export, no props). Pure syntactic sugar: an import + a nested
   element.
5. **`Each`** — plain runtime component + params; the transpiler only checks
   `key` on the body root.

`specs/phase01/vite.md` — the Vite plugin, module resolution, the one type
query in the transform (list slots), the runtime package.
`specs/phase01/diagnostics.md` — `reactogenic check`, source-map origins,
rewrites of TS errors into slot terms.
`specs/phase01/plan.md` — tasks `RGP1-xxx`; `specs/phase01/decisions.md` —
one section per decided task.

Parked in `specs/later/`: `layout.md` (shell vs island rules, `Dynamic`,
shell components, `Form` / `$Field`), `persistent-state.md`, `tooling.md`
(language service).

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
| `phase01/plan.md`, `phase01/decisions.md` | RGP1-001 done |
| `later/layout.md`, `later/persistent-state.md`, `later/tooling.md` | parked |
| `slot-contract.md`, `route-table.md`, `resource.md` | later |

Phase 1 open decisions: parser strategy (RGP1-002); how the plugin hands TSX
to plugin-react — `phase01/vite.md`; Node ↔ Go message encoding —
`phase01/decisions.md`.

## Working style

One extension at a time, so the author can follow. Compact, precise, examples
over prose. Every desugaring as before/after `.rtsx` → `.tsx`. Open questions
inline as `> OPEN:`; rejected ideas as **Rejected:** with the reason. Do not
invent extensions beyond the lists above. Do not write parser code. Commit and
push only when asked.
