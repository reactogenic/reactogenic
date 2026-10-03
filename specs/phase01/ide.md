# IDE support

Editor support for `.rtsx`: a language server, syntax highlighting and a VS
Code extension. The language is in [syntax.md](syntax.md); the errors and
their rewrites in [diagnostics.md](diagnostics.md).

Three deliverables, one engine:

| Deliverable | What | Where |
| --- | --- | --- |
| `reactogenic lsp` | language server (LSP over stdio) | the `reactogenic` binary, `@reactogenic/cli` |
| grammar | TextMate grammar `source.tsx.rtsx`, language id `rtsx` | `packages/vscode` |
| extension | VS Code extension: grammar + client that runs the server | `packages/vscode`, one `.vsix` per platform |

## The engine: one transform, one program model

Everything the editor knows comes from TypeScript 7 checking the emitted TSX,
with positions mapped back. The mechanism is TS7's own **content mapper**
model, which the vendored fork already contains: a mapped file stays
`Button.rtsx` in the program; its **virtual text** (our emitted TSX) is what
TS parses and checks; a **span map** carries every position back.

```
Button.rtsx ──transform──▶ virtual TSX + span map + notes
                              │
        TS7 program (.ts, .tsx and .rtsx as modules) ── checker
                              │
     reactogenic check        reactogenic lsp        (same diagnostics)
```

- **One transform** for every host: `transpiler.Transpile` plus the
  conversion of its map (*Span map*). It runs in-process — no child process,
  no `--runExternalCode`, no tsconfig entry.
- **One program model** for `reactogenic check` and `reactogenic lsp`.
  `check` no longer serves `Button.rtsx` as an in-memory `Button.tsx`
  (diagnostics.md, *reactogenic check*): the `.rtsx` file is the module. A
  segment import `./about-us.rtsx` resolves as written — no alias, no
  duplicate module.
- **Extensionless imports resolve** — `import { Button } from "./button"`
  finds `button.rtsx` (vite.md, *Module resolution*): a patch to the fork's
  resolver tries mapped extensions after the built-in ones. Built-in
  extensions win, as in Vite, where `.rtsx` is last in `resolve.extensions`.
  An explicit `./button.rtsx` is equally valid everywhere.
- **Built in, not configured**: every project the server or `check` opens
  has the `.rtsx` mapper — configured, referenced (Vite's
  `tsconfig.app.json`) and inferred alike.

**Rejected:** building on the *stock* TS7 language server with an external
content mapper. Content mappers ship in TypeScript 7.1 (stable planned for
2026-11-24; 7.0.x ignores the option), need `--runExternalCode`, a tsconfig
entry and the separate "TypeScript 7" extension, do not resolve extensionless
imports of mapped files (upstream: working as intended), and have no hook to
rewrite TS errors into slot terms. We keep the *contract* — our transform is a
content mapper — and host it ourselves (*Stock TypeScript 7.1*, below).

**Rejected:** a proxy in front of the stock server (no structured
diagnostics, no checker); a second, IDE-only emission (two maps to keep
type-equivalent); mapping `.rtsx` to the language id `typescriptreact`
(TS 7.0's server then parses raw `.rtsx` as TSX; 7.1's finds no project).

### Span map

`emit.Map` converts segment by segment:

| Our segment | Span-map segment | Features |
| --- | --- | --- |
| copied | verbatim: positions map 1:1 | all, except formatting |
| synthesized, with origin | atom over the origin span | folding only — otherwise diagnostics only |

- **No gaps.** Every synthesized run has an origin, so a TS error in
  generated code lands on the construct that produced it (as in `check`),
  never on virtual coordinates.
- **Atoms answer nothing.** An origin is often a whole opening tag; an atom
  with features would answer hover, completion and rename for unrelated
  generated code. The one feature they keep is folding, so a block that
  contains an rtsx construct still folds — except a hoisted import, whose
  origin is far from where it is emitted.
- **One primary copy.** Where one source token is copied to several virtual
  places (`<Input value />` → `value={value}`; an attachment emitted in both
  branches of its ternary), one copy is primary and carries the features; the
  others are diagnostics-only. Primary: the *value* of a shorthand prop (the
  binding the author refers to), the *assigned* branch of an attachment.
- **Names are copied, not synthesized**, so TS's own features reach them:

  | Source token | Emitted as | Gives |
  | --- | --- | --- |
  | slot tag name `<$Icon` | the prop name `$Icon=` / the key `$Icon:` | hover, definition, references on the slot |
  | attribute names of a slot element | the object keys | hover, completion of slot props |
  | arg names `&size`, `&&value` | the arg keys | hover, definition |
  | closing tag name `</Button>` | the rebuilt closing tag | rename keeps tags paired |

  The emitted text is unchanged; three diagnostic columns move from the `<` /
  `&` to the name.

### Tolerance

A file being typed rarely parses. The transform never fails on user input:

1. Passes run on TS's recovered tree (`Input.Tolerant`); a pass that fails
   keeps the previous pass's text and map.
2. Syntax errors are reported from the **source** parse only; an error TS
   finds in emitted text that the source parse did not report is ours and is
   dropped.
3. Transpiler diagnostics on a construct that contains a parse error are
   suppressed (no `case-no-test` on a half-typed `<$Case`).
4. Last resort — nothing usable: the source as virtual text, mapped 1:1, with
   TS diagnostics ignored for the file. Completion and hover keep working.

The build paths stay strict: Vite and `reactogenic check` fail on a syntax
error as today.

## `reactogenic lsp`

`reactogenic lsp --stdio`: the fork's TS7 language server, with the mapper
built in and four additions (below). Standard LSP; any client works.

| Feature | `.rtsx` support | How |
| --- | --- | --- |
| diagnostics | **the same as `reactogenic check`** | *Diagnostics* |
| hover, signature help | ✓ | TS on virtual text, mapped back |
| go to definition, type definition, references, highlights | ✓ | TS; `#name` → *Segments* |
| completion, auto-import | ✓ | TS; slot names → *Slots* |
| rename | correct or refused | *Rename* |
| document symbols, folding, selection ranges, semantic tokens, inlay hints | ✓ | TS |
| code actions, organize imports | only when every edit maps exactly | TS drops the rest |
| formatting | ✗ | *Not in the first release* |

`.ts` and `.tsx` files of the same project are in the server's program (for
types across the boundary), but the client attaches it to `.rtsx` documents
only (*The `.ts` side*).

### Diagnostics

The editor shows what `reactogenic check` prints — same codes, same messages,
same positions. One implementation, shared:

- Transpiler errors and warnings (`orphan-slot`, `segment-children`, …), with
  their names as codes and their severities.
- TS errors, rewritten into slot terms (diagnostics.md, *Rewrites*). The
  rewrites need the structured diagnostic — message arguments and chain — and
  the transpiler's notes: a hook in the server, before the diagnostic becomes
  an LSP message.
- `slot-conditional`, the cross-file rule.
- **Each mistake once.** Code copied to several virtual places is checked in
  each; diagnostics with the same range and message are merged. (`check`
  gains this too: it printed such errors twice.)

Diagnostics are pulled per open document. Project-wide errors stay with
`reactogenic check`; the extension offers it as a task with a problem matcher.

### Slots

- After `<$` inside a component's children: its declared slots — the `$`
  props of the owner, from TS's own property completion at the copied name,
  filtered to `$` names; filled slots sort last.
- Inside a slot tag: completion and checking of the slot's props (free: the
  attribute names are the object's keys).
- Hover on a slot tag shows the declared slot type; go to definition jumps to
  the `$X` member of the container's props.

### Segments

- Go to definition on `#about-us` opens the mounted file (the lookup of
  syntax.md, *Segment files*).
- After `#`: the sibling modules not yet mounted in the file.
- A segment file created, renamed or deleted updates its mounters' errors
  without an edit to them: the transform reads siblings through the server's
  file system (unsaved buffers included), and a created or deleted file
  re-transforms the `.rtsx` files of its directory.

### Rename

**A rename is correct or refused — never partial.** TS computes the edits on
the virtual text; the server completes them:

| Site | Problem | The server |
| --- | --- | --- |
| shorthand `<Button size>` | one token, two meanings: prop and binding | expands: renaming the binding gives `size={dim}`; renaming the prop gives `scale={size}` |
| arg shorthand `&size` | the same | the same: `&size={dim}` |
| slot elements | TS sees one prop per owner; the closing tag and repeated or keyed elements have no virtual token | renames every `<$X` / `</$X>` of that name under the same owner |
| an occurrence that cannot be written back | — | refuses the whole rename, naming the place |

### Commands

`reactogenic/transpiled` (custom request): the emitted TSX of a document —
the extension shows it beside the source ("Show transpiled TSX").

### Not in the first release

| | |
| --- | --- |
| formatting | through the virtual text it re-indents moved slot bodies. Needs its own implementation; Prettier does not know `.rtsx` |
| shorthand inlay hint | `disabled`⟨`={disabled}`⟩ on every bound bare attribute (syntax.md, *Silent flip*) |
| quick fix for `segment-not-found` | create the file with an empty default component |
| hover docs on `Switch` / `Match` / `$Case` | they are lowered away; static text |
| param completion in `{ }` of a slot without a body | needs params emitted without a body, which changes diagnostics |
| push diagnostics | for clients without pull support |

> OPEN: auto-import writes `./button.rtsx` (TS's specifier for a mapped
> file). Valid everywhere; the convention is extensionless. Strip it?

## Syntax highlighting

A TextMate grammar **generated** from VS Code's TSX grammar by a small patch
script — scope `source.tsx.rtsx`, language id `rtsx`.

Under the unmodified TSX grammar, slot tags and params tokenize acceptably,
but `&name`, `&&name={x}` and `#name` fall to a catch-all "illegal attribute"
rule that also swallows a following `>`: the rest of the file is painted as
attributes. So highlighting needs its own rules, not just a language id.

| Construct | Scope | Reads as |
| --- | --- | --- |
| slot tag `$Icon` | `entity.name.function.slot.rtsx` | function colour |
| `&` / `&&` | `storage.modifier.slot-arg.rtsx` | keyword-like |
| arg name | `entity.other.attribute-name.slot-arg.rtsx` | attribute |
| params `{ size }` | `meta.slot-params.rtsx`, TSX's parameter scopes inside | parameters |
| segment root `#about-us` | `support.class.component.segment.rtsx` | component colour |

- The generator patches three things: the attribute list (args, segment
  roots, params), the tag-name pattern (`$name`), the root scope. It asserts
  the shape of each rule it patches, so an upstream change fails loudly.
- The root scope is `source.tsx.rtsx`: editors find injections (JSDoc,
  styled-components, …) by scope prefix; `source.rtsx` would lose them.
- Plain `.tsx` code tokenizes identically to `source.tsx` — tested on the
  upstream grammar's own test inputs.
- A sigil without a name yet (`&`, `#` right after typing) must not derail
  the file: the name is optional in the rules.
- Language configuration: TSX's, plus `$` allowed in tag names (indent after
  `<$Slot>`) and in the word pattern (completing `<$Ic` replaces the `$`).

**Rejected:** a thin grammar that includes `source.tsx` plus injections
(cannot recolour slot tags, needs a selector that enumerates nesting depths,
fails inside `{…}` expressions); semantic tokens alone (TS does not token
JSX tag or attribute names, and nothing colours without a server).

**Deferred:** a tree-sitter grammar (Zed, Neovim, Helix); GitHub Linguist
(needs ~2,000 public files); Shiki (the grammar loads there as is — export it
from a package when the docs site needs it).

## VS Code extension

`packages/vscode` — marketplace id `reactogenic.rtsx`.

- **Contributes**: the language `rtsx` (`.rtsx`), the grammar, the language
  configuration, Emmet as for TSX, a Markdown fence injection (` ```rtsx `),
  the commands *Restart server* and *Show transpiled TSX*, a `reactogenic
  check` task with a problem matcher.
- **Client**: `vscode-languageclient` over stdio, selector `rtsx`.
- **Which binary runs**, first match:

  | | |
  | --- | --- |
  | 1 | the setting `reactogenic.server.path` |
  | 2 | `$REACTOGENIC_BINARY` |
  | 3 | the workspace's `@reactogenic/cli`, resolved from the document's folder upward — if it has `lsp` (version ≥ the extension's minimum) |
  | 4 | the binary bundled in the `.vsix` |

  The workspace's own CLI comes first so the editor and `reactogenic check`
  agree. A status item shows which binary runs and its version.
- **Trust**: an untrusted workspace gets highlighting only — the server reads
  tsconfig and runs a binary from `node_modules`.
- **Packaging**: one `.vsix` per platform (the six of `@reactogenic/cli`),
  each with its binary, plus the licences of the code in it.

### The `.ts` side

`src/main.tsx` imports `./page`, and VS Code's built-in TypeScript — a
different server — does not know `.rtsx`: it reports TS2307 there.

| The user's TypeScript server | How it learns `.rtsx` |
| --- | --- |
| built-in `tsserver`, TS ≤ 6 (VS Code's default) | a **TS server plugin** the extension contributes: resolves `.rtsx` imports and serves the emitted TSX as the file's text, positions mapped back |
| TS 7.1+ native server | the **stock content mapper** (below) |

The plugin serves saved files: an unsaved `.rtsx` edit reaches `.ts` files on
save (as in Svelte's plugin).

### Stock TypeScript 7.1

The same transform, as a standard content mapper: `reactogenic content-mapper`
speaks the mapper protocol, and `@reactogenic/cli` declares it
(`typescript.contentMapper` in its `package.json`). A project that lists it in
tsconfig `contentMappers` gets `.rtsx` in plain `tsc --runExternalCode` and in
the TS 7.1 server. **Experimental** until 7.1 is stable: raw TS messages (no
rewrites), explicit `.rtsx` in imports written in `.ts` files.

## Other editors

`reactogenic lsp --stdio` is a standard server: Neovim (`vim.lsp`), Zed,
Helix and JetBrains can run it for `*.rtsx`. Highlighting there needs the
deferred tree-sitter grammar (JetBrains takes the TextMate one).

## Testing

| Layer | Test |
| --- | --- |
| transform | conformance corpus: the span map validates; every virtual node maps to the same source span as `emit.Map`; tolerant mode over typing-like mutants of the fixtures never panics and never loses the file |
| server | a stdio client in Go tests drives the real binary: one scenario per feature row above, on a fixture project |
| `check` | its existing suite, unchanged in expectations, on the new program model |
| grammar | scope assertions per construct; equality with `source.tsx` on plain TSX; no `invalid.*` token in any `.rtsx` of the repo |
| extension | binary resolution unit tests; an editor suite (open a file, assert diagnostics, hover, definition) run in an isolated VS Code profile |
