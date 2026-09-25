# Running `.rtsx` with Vite

Phase 1 goal: an ordinary Vite + React project can contain `.rtsx` files and
run them in dev and in `vite build`, with nothing else changing.

## Setup

```ts
// vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import reactogenic from "reactogenic/vite";

export default defineConfig({
  plugins: [reactogenic(), react()],
});
```

- `reactogenic()` runs first (`enforce: "pre"`) and turns `.rtsx` into `.tsx`.
  Everything after that — stripping types, the JSX transform, Fast Refresh —
  is Vite's and plugin-react's, unchanged.
- `.tsx` and `.rtsx` live side by side and import each other. Renaming a
  `.tsx` file to `.rtsx` is the whole migration (mind the *silent flip* in
  [syntax.md](syntax.md#edge-cases)).

## The transform

| Plugin hook | Does |
| --- | --- |
| `config` | adds `.rtsx` to `resolve.extensions` and to the files plugin-react processes |
| `transform` | for an id ending in `.rtsx`: runs the passes of [syntax.md](syntax.md#compilation-passes); returns `.tsx` source + source map |

- **The output is TSX, not JS.** The plugin neither strips types nor
  transforms JSX; the rest of the pipeline does that as for any `.tsx`.
- **One transpiler.** The transform and `reactogenic check`
  ([diagnostics.md](diagnostics.md)) run the same code on the same input, so
  what is type-checked is exactly what runs.
- **Source map.** Vite chains it with the maps of later plugins, so devtools,
  stack traces and the error overlay show `.rtsx` positions.
- **Errors.** The transform throws the transpiler's own errors — the ones the
  *Compile errors* tables in [syntax.md](syntax.md) mark *syntax* or *files*,
  plus duplicate-slot — with their `.rtsx` location: Vite's overlay in dev, a
  failed `vite build`. Errors marked *types* are TS errors in slot terms and
  are reported by `reactogenic check`, like every other type error.

> OPEN: how the output is presented as TSX to downstream plugins, which pick
> files by extension. (a) extend their `include` to `.rtsx` from the `config`
> hook, or (b) return the module under a virtual id ending in `.tsx`.
> Recommended: (a) — ids stay the real file names, which HMR and the overlay
> rely on.

## Module resolution

- `import Button from "./Button"` finds `Button.rtsx` as it finds
  `Button.tsx`. Both files side by side is an error: the import would be
  ambiguous.
- Segment imports are emitted extensionless (`import … from "./+about-us"`)
  and resolve the same way, to `+about-us.rtsx` or `+about-us.tsx`.
- The type checker resolves identically; see
  [diagnostics.md](diagnostics.md#reactogenic-check).

## Types in the transform

List slots are type-directed ([syntax.md](syntax.md#list-slots)): to emit
`<Form>`'s `$Field` as an array, the transform must know that `FormProps`
declares `$Field` as one. So the plugin holds a type checker: TS7, in the Go
transpiler, which the plugin drives as one long-lived process
([decisions.md](decisions.md#rgp1-001--checker-and-implementation-language)).

- **One program** over the project's `tsconfig.json`, kept for the life of the
  dev server or build. Every `.rtsx` file is in it as its emitted `.tsx`
  (the same virtual files as `reactogenic check`).
- **One query.** For each slot element `$X` under a container `P`: "is `$X` in
  `P`'s props type an array or a tuple?" Nothing else in the emit reads
  types.
- **No cycle.** The answer comes from `P`'s declared props type. Desugaring
  changes JSX, never declarations, so a container declared in an `.rtsx`
  file has the same props type before and after its own transform.
- **HMR.** An emit now depends on types declared in other files. The plugin
  records which containers each transform asked about; when such a
  declaration changes, the `.rtsx` files that asked are transformed again.
  Without this, editing `FormProps` would leave stale output.


## Runtime

Phase 1 ships a small `reactogenic` package:

| Export | Kind | Note |
| --- | --- | --- |
| `SlotFn`, `OptionalSlotFn` | types | slot declarations |
| `renderSlot` | function | used by containers |
| `Each` | component | an ordinary runtime component |
| `Switch`, `Match` | declarations only | lowered away; the emitted `.tsx` drops their import |
| `noMatch` | function | imported by emitted code for `exhaustive`, under a generated name |

## Not in phase 1

- Type-checking inside the dev server. Vite does not type-check `.tsx`
  either; `reactogenic check` does.
- Editor support (completion, hover, errors as you type) — see
  [../later/tooling.md](../later/tooling.md). Until then, `.rtsx` errors come
  from Vite and from `reactogenic check`.
