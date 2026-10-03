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
| transpiler notes joined by the checker, across files | slot-conditional | `reactogenic check` |

The errors marked *types* in the *Compile errors* tables of
[syntax.md](syntax.md) are the middle row: TS already finds them on the
emitted code; the mapping only moves and rewords them (*Rewrites* below).

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
   file (`./about-us.rtsx`).
   > Until RGP1-106: `check` serves `Button.rtsx` as an in-memory
   > `Button.tsx` through an overlay, with an `about-us.rtsx.tsx` alias for
   > segment imports.
3. Check the program. Diagnostics in virtual files are mapped back (*Mapping*);
   diagnostics in real `.ts` / `.tsx` files pass through, with any related
   information that points into a virtual file mapped too.
4. Print in `tsc`'s format, with the `.rtsx` path and a code frame of the
   `.rtsx` source. Transpiler errors use their name as the code:

```
src/Page.rtsx:12:5 - error TS2322: Type 'number' is not assignable to type 'string'.
src/Page.rtsx:18:3 - error missing-slot: `Card` requires `$Title`.
```

A tsconfig with `references` and no files of its own (Vite's template) is
checked through its references, each file reported once.

`--watch` re-checks on change, as `tsc --watch` does.

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
