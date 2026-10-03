# Diagnostics: TS errors on `.rtsx`

TS7 checks the emitted `.tsx`. Every error it finds is reported **on the
`.rtsx` the author wrote**, in terms of what they wrote: never a position in
emitted code, never a generated name.

## Where errors come from

| Source | Examples | Reported by |
| --- | --- | --- |
| transpiler | orphan-slot, arg-without-slot, segment-not-found, segment-in-loop | the Vite transform and `reactogenic check` |
| TS on the emitted `.tsx`, rewritten into slot terms | undeclared-slot, missing-slot, switch-missing-case | `reactogenic check` |
| TS on the emitted `.tsx`, as is | any other `TSxxxx` | `reactogenic check` |
| transpiler notes joined across files: by the checker; by following mounts | slot-conditional; segment-self through another file | `reactogenic check` |

The errors marked *types* in the *Compile errors* tables of
[syntax.md](syntax.md) are the middle row: TS already finds them on the
emitted code; the mapping only moves and rewords them (*Rewrites* below).

Everything `reactogenic check` reports comes from one **reporting layer**,
which the language server shares ([ide.md](ide.md), *Diagnostics*): the editor
shows the same lines.

## `reactogenic check`

The phase 1 counterpart of `tsc --noEmit`: a CLI with an exit code, for the
terminal and CI.

```json
// package.json
"scripts": {
  "check": "reactogenic check",
  "build": "reactogenic check && vite build"
}
```

1. Load `tsconfig.json`. `.rtsx` files are picked up by the same `include`
   and `exclude` globs as `.tsx`.
2. Transpile every `.rtsx` file. `Button.rtsx` is a module of the program
   under its own name; TS parses and checks its emitted TSX — never written
   to disk — and a span map carries positions back ([ide.md](ide.md), *The
   engine*). Imports resolve with or without the extension: `./button` finds
   `button.rtsx` after the built-in extensions; a segment import names its
   file (`./about-us.rtsx`). A tsconfig `contentMappers` entry changes
   nothing: the transform is built in.
3. Check the program. Diagnostics in `.rtsx` files are mapped back
   (*Mapping*); diagnostics in `.ts` / `.tsx` files pass through, with any
   related information that points into an `.rtsx` file mapped too.
4. Print in `tsc`'s format, with the `.rtsx` path and a code frame of the
   `.rtsx` source. Transpiler errors use their name as the code:

```
src/Page.rtsx:12:5 - error TS2322: Type 'number' is not assignable to type 'string'.
src/Page.rtsx:18:3 - error missing-slot: `Card` requires `$Title`.
```

**What a file reports**, besides its type errors:

| The file | Reports |
| --- | --- |
| has a syntax error | its syntax errors, nothing else: the passes do not run (the build is strict). Its module still exports what it declares, so its importers are checked, and the rest of the program is |
| has code the transpiler left out — an expression or a component inside an orphaned slot element, or inside a `Switch` or `Match` that cannot be lowered (`null` in the emitted TSX) | the transpiler's errors only. TS's would be about code the author did not write: every name used only there reads as unused ([ide.md](ide.md), *Tolerance*, rule 4) |
| has code that is emitted twice (an attachment and its fallback) | each mistake once ([ide.md](ide.md), *Diagnostics*) |
| is `name.rtsx` next to `name.tsx` | ambiguous-module, and its own errors: both files are modules of the program |

**References.** The projects a tsconfig references are checked first, each
with its own options, then the tsconfig's own files — so a tsconfig with
`references` and no files of its own (Vite's template) prints what its
projects print. A referenced project's modules are read from source (an
`.rtsx` file has no build output). Each file is reported once, by the first
project that holds it.

`--watch` re-checks on change, as `tsc --watch` does: an edited file, and one
created or deleted — a segment's file, an import's target — in the directory
of the tsconfig or of a project it references.

The editor shows the same diagnostics, from the same code, through
`reactogenic lsp` ([ide.md](ide.md), *Diagnostics*).

> OPEN: type errors in the Vite dev overlay (what `vite-plugin-checker` does
> for `tsc`). Useful, but a separate process; after phase 1's core.

## Mapping

The transform's source map records, for every emitted node, either the span it
was **copied** from, or the construct it was **synthesized** for (its
*origin*). Two cases:

| Error span in the emitted code | Reported at | Message |
| --- | --- | --- |
| inside copied code — a user expression, a body, an attribute value | the exact `.rtsx` span | TS's, unchanged |
| inside synthesized code | the origin | rewritten if a rule below matches; else TS's, unchanged |

**Names are copied**: a slot's prop name is its tag name (`<$Icon` →
`$Icon=`), a slot object's keys are the attribute names, an arg key is the
arg's name. An error on one of them is reported at the name the author wrote
([ide.md](ide.md), *Span map*).

Origins of synthesized code:

| Emitted | Origin |
| --- | --- |
| `value={value}` / bare `value` | the bare attribute |
| `$X={{ … }}`, an item of `$X={[ … ]}` | the slot element's tag |
| a property of the slot object | the slot attribute it came from |
| `children: (params) => …` | the params pattern |
| ternary chain, `_on`, IIFE of `Match` / `Switch` | the `Match` / `Switch` tag |
| `_on === value` | the `is` attribute |
| `noMatch(_on)` | the `Switch` tag |
| segment import, `id="…"`, `<_Section_x />` | `#name` |

Generated names in a message are replaced by what the author wrote: `_on` by
the `on` expression (`getStatus()`), `_Section_aboutUs` by `#about-us`.

## Rewrites

Each error the author would otherwise read as an assignability error on code
they never wrote:

| Code | TS reports on the emitted code | Reported at |
| --- | --- | --- |
| undeclared-slot | property `$X` does not exist on the props type (TS2322 › TS2339) | slot tag |
| missing-slot | property `$X` is missing | container tag |
| params-required | a body is not assignable to a function slot's `(args) => …` | slot tag |
| content-not-allowed | property `children` does not exist in the slot's contract | slot tag: "`$X` takes no body" |
| no-values | the params get no contextual type (TS7031, implicit `any`) | params pattern |
| content-required | property `children` is missing in the slot object | slot tag — **not specific yet**: with `NotAssigned` in `Slot`, TS reports a union mismatch; reported as slot-type, "`$X` does not match its declaration in `P`: …" |
| switch-missing-case | `noMatch`'s argument is not assignable to `never` | `Switch` tag: "Missing `"success"`" — the leftover type, printed as cases |
| segment-not-component | the module has no default export, or it is not a component | `#name` |
| segment-props | required props missing on `<_Section_x />` | `#name` |
| segment-root-props | the root does not accept `id` or `children` | `#name` |
| slot-args-missing | a property of `renderSlot`'s args is missing (a function slot attached without its args) | the attachment: "`$Icon` needs `&size`" |
| slot-no-args | a property of `renderSlot`'s args is not assignable to `never` (args to a slot whose body is not a function) | that arg: "`$Label` takes no args: its body is not a function" |
| slot-list | `renderSlot`'s args are `never` (a slot typed as an array) | the attachment: "`$List` is a list; a slot is one value" |
| slot-key-no-args | the `[SLOT_KEY]` property is excess (TS2353): a key function on a slot without args | the `key` attribute |
| slot-key-inline | a computed entry key is not a string or number (TS2464): a key function passed by reference | the `key` value; TS's other errors inside that slot element are dropped |

Rewrites are matched by the error's origin and its TS code, never by message
text. An error that matches no rule keeps TS's message, at the origin.

Shorthand props case B on a non-boolean prop is **not** rewritten — TS's
"Type `true` is not assignable to type `string`" is accurate — but it gets
related information: "no `value` in scope".

The exact TS7 code behind each rule is fixed by that rule's test in
RGP1-073 (TS7 is the checker: [decisions.md](decisions.md#rgp1-001--checker-and-implementation-language)).
