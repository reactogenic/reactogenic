# Running `.rtsx` with Vite

Phase 1 goal: an ordinary Vite + React project can contain `.rtsx` files and
run them in dev and in `vite build`, with nothing else changing.

## Setup

```ts
// vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import reactogenic from "@reactogenic/vite";

export default defineConfig({
  plugins: [reactogenic(), react()],
});
```

- `reactogenic()` runs first (`enforce: "pre"`) and compiles `.rtsx` itself:
  the Go transpiler turns it into TSX, and Vite's own `transformWithOxc`
  turns that into JS, with one source map back to the `.rtsx` (*Findings*
  below). `.tsx` files go through Vite and plugin-react as usual.
- It drives one `reactogenic serve` process per build or dev server: the
  binary from the `binary` option, else `$REACTOGENIC_BINARY`, else
  `reactogenic` on `PATH` (the npm package with platform binaries is
  RGP1-092).
- `.tsx` and `.rtsx` live side by side and import each other. Renaming a
  `.tsx` file to `.rtsx` is the whole migration (mind the *silent flip* in
  [syntax.md](syntax.md#edge-cases)).

## The transform

| Plugin hook | Does |
| --- | --- |
| `config` | adds `.rtsx` to `resolve.extensions` |
| `transform` | for an id ending in `.rtsx` (query ignored): the passes of [syntax.md](syntax.md#compilation-passes) in Go, then Oxc; returns JS (`moduleType: "js"`) + source map |
| `closeBundle`, dev server close | ends the Go process |

- **One transpiler.** The transform and `reactogenic check`
  ([diagnostics.md](diagnostics.md)) run the same code on the same input, so
  what is type-checked is exactly what runs.
- **Source map.** Vite chains it with the maps of later plugins, so devtools,
  stack traces and the error overlay show `.rtsx` positions.
- **Errors.** The transform throws the transpiler's own errors — the ones the
  *Compile errors* tables in [syntax.md](syntax.md) mark *syntax* or *files* —
  with their `.rtsx` location: Vite's overlay in dev, a
  failed `vite build`. Errors marked *types* are TS errors in slot terms and
  are reported by `reactogenic check`, like every other type error.

**Findings** (Vite 8.3 — Rolldown and Oxc — with plugin-react 6.1, probed
2026-09-26 on a scratch project):

| Approach | `vite build` | dev server |
| --- | --- | --- |
| transform returns TSX with `moduleType: "tsx"` | works: Rolldown compiles it | fails: Vite's Oxc plugin picks files by extension and never sees `.rtsx`; the module is served as raw TSX, with `?import` appended because `.rtsx` is not a script extension |
| add `.rtsx` to `oxc.include` | — | fails: `transformWithOxc` takes the language from the extension, and `rtsx` is not one |
| resolve to `…/page.rtsx?lang.tsx` (Vue's pattern) | works | fails: when a Fast Refresh filter matches a file whose real extension is not a script type, Vite forces `lang: "js"`, and the TSX does not parse |
| the plugin compiles `.rtsx` → TSX → JS itself, with Vite's exported `transformWithOxc(…, { lang: "tsx", jsx })` | expected to work | expected to work; Fast Refresh needs plugin-react to include `.rtsx` — its refresh wrapper filters by its own `include` option, which another plugin cannot extend |

**Decided** (phase 1): the plugin compiles `.rtsx` itself (the last row),
and there is **no Fast Refresh for `.rtsx`**: edits reload the page. Fast
Refresh belongs with Reactogenic's own dev server in the next phase. The
options considered for later:
>
> - (a) documented setup: `plugins: [reactogenic(), react({ include:
>   /\.(rtsx|[jt]sx?)$/ })]` — explicit, one more thing to get right;
> - (b) `reactogenic()` returns plugin-react configured for `.rtsx`, so the
>   setup is `plugins: [reactogenic()]` — changes the *Setup* section above;
> - (c) no Fast Refresh for `.rtsx` in phase 1: edits reload the page.

## Module resolution

- `import Button from "./Button"` finds `Button.rtsx` as it finds
  `Button.tsx`. Both files side by side is an error: the import would be
  ambiguous.
- Segment imports name the file the transpiler found, extension included
  (`import … from "./about-us.rtsx"`; syntax.md, *Segment files*): Vite
  resolves them as they are written.
- The type checker resolves identically; see
  [diagnostics.md](diagnostics.md#reactogenic-check).

## No types in the transform

The transpiler is purely syntactic (syntax.md, *Slots → Typing behaviour*):
the transform of one file reads that file, and the segment files next to it,
and nothing else. It holds no checker and no program; types are TS7's job, in
`reactogenic check`. (Until the slot model of RGP1-043, list slots made the
emit type-directed; they are gone.)

## Runtime

All packages are scoped to the `@reactogenic` npm org. Phase 1 ships:

| Package | Contents |
| --- | --- |
| `@reactogenic/core` | the runtime, below |
| `@reactogenic/vite` | the plugin |
| `@reactogenic/cli` | the `reactogenic` command (a shim that finds the binary) |
| `@reactogenic/cli-<os>-<arch>` | the Go binary per platform, `optionalDependencies` of `@reactogenic/cli` |

`@reactogenic/vite` depends on `@reactogenic/cli` for the binary it drives.

`@reactogenic/core` exports:

| Export | Kind | Note |
| --- | --- | --- |
| `Slot`, `FnSlot`, `SlotFn`, `ArgsOf`, `NoArgs` | types | slot declarations |
| `renderSlot` | function | imported by emitted attachments, under a generated name |
| `Each` | component | an ordinary runtime component |
| `Switch`, `Match` | declarations only | lowered away; the emitted `.tsx` drops their import |
| `noMatch` | function | imported by emitted code for `exhaustive`, under a generated name |

## Not in phase 1

- Type-checking inside the dev server. Vite does not type-check `.tsx`
  either; `reactogenic check` does.
- Editor support (completion, hover, errors as you type) — see
  [../later/tooling.md](../later/tooling.md). Until then, `.rtsx` errors come
  from Vite and from `reactogenic check`.
