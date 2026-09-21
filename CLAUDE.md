# Reactogenic — context for working in this repo

You are working in `reactogenic/reactogenic`, a monorepo for a React-based UI
framework. Current task: write the syntax specification in `specs/`. No
implementation code yet — Markdown only. Ask before creating anything outside `specs/`.

## What Reactogenic is

A framework with four tightly coupled pillars: a compiler, a design system
(UIKit-inspired: containers with fixed roles + controls), a server-side router,
and an extended TSX syntax with the `.rtsx` file extension.

Positioning: "Reactogenic apps are built from framework-owned components with
typed slots. Your code lives in the slots; the compiler owns everything around
them, so each screen ships exactly the HTML, CSS and JS it needs."

## Architecture decisions (fixed — do not relitigate)

- Navigation is server-only. The pathname defines the page. Query string and
  hash are in-page state, never navigation.
- Each page has a static, deterministic **shell** (pre-generated per pathname,
  emitted as a Go template) and **islands** (client-side React, hydrated only
  inside explicit dynamic boundaries). Shell is identical across documents so
  the browser's cross-document View Transitions API handles flicker-free navigation.
- Slots are the organizing concept. Custom/plain React code is allowed ONLY as
  slot content. Every slot boundary is also a hydration, chunking and async boundary.
- Islands own their data via a resource abstraction (React Query embedded, not
  exposed) with states: idle, loading, reloading, empty, error, success.
- Compiler: Go, forked from the tsgo (TypeScript 7) parser. Emits plain `.tsx`
  for TS7 to type-check, Go templates for shells, per-route CSS. Bundling via
  embedded esbuild. Server is also Go.
- The design system's core catalog is authored in plain `.tsx` (not `.rtsx`) to
  avoid a compiler/design-system bootstrap cycle.

## Governing syntax rule

New meaning may only be given to forms that are syntax errors in today's TSX.
Any form that already parses keeps its React meaning exactly. Prefer extensions
that are valid TSX and need only a transform; real grammar changes only when unavoidable.

## Agreed extensions (specify these)

1. **Slot elements** `<$IconStart>…</$IconStart>` inside `<Button>` compiles to
   `<Button $IconStart={…}/>`. Valid TSX already; transform only. `$` is reserved
   for slots ("$ stands for $lot, not $ystem"). Same slot twice = compile error.
   Unslotted children go to `children`.
2. **Type-directed shorthand props.** Bare attribute on a boolean prop = `true`
   (as React). Bare attribute on a non-boolean prop = same-named variable in
   scope. No same-named variable = compile error, never silent `true`.
   Define `boolean | X` union behaviour.
3. **Slot prop binding.** A bare destructuring pattern in attribute position,
   `<$Cell column="name" { cell, row }>`, binds parameters flowing OUT of the
   container into the slot body. First real grammar change. Rule of thumb:
   bare name = pass my value in; braces = give me your value out.
4. **`Match`** — independent conditional; lazy children (evaluated only when
   the condition holds); value narrowing inside.
5. **`Case`** — adjacent `Case` siblings form a group; first true wins;
   `<Case default>` fallback; no wrapper element; no match at runtime throws;
   compiler proves exhaustiveness where it can. `Match`/`Case` are ordinary
   imports from the framework package, recognised by import origin, not name.
6. **Rules of layout** — containers (anything with slots) must be unconditional
   and unlooped at their position. `Match`/`Case`/loops are allowed only inside
   slots. The only sanctioned shell variance: the router choosing a shell, and
   optional slots being empty. Enforced as compiler error, language-service
   diagnostic, and ESLint rule. Escape hatch: dynamic shells become two routes.

## Proposed, not yet approved (spec as "Candidate" sections)

- `For` with `each` + binding `{ item, index }`, compile-time key enforcement.
- `Await` resolving a resource, with `$Loading`/`$Error`/`$Empty`/`$Reloading` slots.
- `$Error` slot on every container (compiler inserts the boundary).
- Two-way binding on design-system inputs, candidate form `value={=name}`.

## Deliverables for this task

Create `specs/` with:

- `syntax.md` — the six extensions above, each with: motivation, grammar
  (what parses today vs what we add), desugaring to plain TSX, typing behaviour,
  compile errors, edge cases, prior art (Vue, Svelte 5, Solid, Astro).
- `slot-contract.md` — how a component declares slots and their types.
- `route-table.md` — pathname → shell + islands + chunks.
- `resource.md` — the resource state machine and the `isEmpty` rule.

Style: compact, precise, examples over prose. Every desugaring shown as
before/after `.rtsx` → `.tsx`. Mark open questions inline as `> OPEN:`.
Do not invent extensions beyond the lists above. Do not write parser code.
