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
  no `--runExternalCode`.
- **One program model** for `reactogenic check` and `reactogenic lsp`.
  `check` no longer serves `Button.rtsx` as an in-memory `Button.tsx`: the
  `.rtsx` file is the module. A segment import `./about-us.rtsx` resolves as
  written — no alias, no duplicate module.
- **Built in, not configured.** Every project `check` or the server opens has
  the `.rtsx` mapper: a tsconfig project, a referenced one (Vite's
  `tsconfig.app.json`), and a loose file without a tsconfig. A tsconfig
  `contentMappers` entry is ignored by both — the built-in mapper wins,
  nothing is spawned.
- **Extensionless imports resolve** — `import { Button } from "./button"`
  finds `button.rtsx` (vite.md, *Module resolution*): the fork's resolver
  tries mapped extensions after the built-in ones, `paths` aliases included.
  An explicit `./button.rtsx` is equally valid everywhere.
- **Siblings.** `name.tsx` next to `name.rtsx` stays `ambiguous-module`,
  reported by the transpiler on the `.rtsx` (Vite, `check` and the editor
  alike). Other built-in siblings (`.ts`, `.d.ts`, `.js`, `.jsx`) win an
  extensionless import silently, as in Vite, where `.rtsx` is last in
  `resolve.extensions`.
  > OPEN: make every built-in sibling `ambiguous-module`? It would forbid
  > `intro.rtsx` next to `intro.ts`, which the segment lookup order allows.
- **Notes travel with the parsed file.** The transpiler's notes and generated
  names (what the rewrites need) are attached to the program's source file,
  not kept in a table beside it: the server holds several versions of a file
  at once.
- **Virtual text is always plain TSX.** The rtsx grammar is a parse option
  set only by the transpiler's own source parse, so the checker never sees an
  rtsx-shaped tree.

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

`emit.Map` converts segment by segment; every emitted piece carries the set
of editor features it may answer.

| Our segment | Span-map segment | Features |
| --- | --- | --- |
| copied | verbatim: positions map 1:1 | all, except formatting |
| synthesized, with origin | atom over the origin span | none — diagnostics only |

```tsx
// .rtsx
<Button size><$Icon className="i" { size }>{size}</$Icon></Button>

// virtual TSX — C copied, · synthesized (atom on its construct)
<Button size={size} $Icon={{ className: "i", children: ({ size }) => size }} />
 CCCCCC CCCC CCCC   CCCCC    CCCCCCCCC  CCC               CCCC       CCCC
        │    │      └ the tag name `$Icon`, copied from `<$Icon`
        │    └ value copy of `size`
        └ name copy of `size`
```

- **No gaps.** Every synthesized run has an origin, so a TS error in
  generated code lands on the construct that produced it (as in `check`),
  never on virtual coordinates.
- **Atoms answer nothing.** An origin is often a whole opening tag; an atom
  with features would answer hover, completion and rename for unrelated
  generated code.
- **Names are copied, not synthesized**, so TS's own features reach them:

  | Source token | Emitted as | Gives |
  | --- | --- | --- |
  | slot tag name `<$Icon` | the prop name `$Icon=` / the key `$Icon:` | hover, definition, references on the slot |
  | attribute names of a slot element | the object keys | hover, completion of slot props |
  | arg names `&size`, `&&value` | the arg keys | hover, definition |
  | closing tag name `</Button>` | the rebuilt closing tag, when one is emitted | hover, definition there |
  | whitespace before an attribute or `>` of a rebuilt tag or slot tag | the same position in the emitted tag | prop completion on an empty position |

  These copies carry no semantic tokens: tag and attribute names keep the
  grammar's colours with or without a server. The emitted text is unchanged;
  three diagnostic columns move from the `<` / `&` to the name.
- **Several copies of one token.**
  - *Identical copies* — an attachment emitted in both branches of its
    ternary, the `$X` of `slot={$X}`, a `Switch` subject repeated per case:
    the first copy in virtual order has the features; the others have none.
  - *Shorthand* — `<Input value />`, `&size`, a slot's bare attribute — where
    the two copies are different symbols (the prop and the binding): both
    answer, as TS does on `{ value }`.

    | Copy | Features |
    | --- | --- |
    | name (the prop / arg key) | hover, completion, definition, type definition, references, highlights |
    | value (the binding) | hover, definition, references, highlights, rename, semantic tokens, inlay hints |
- **Slot groups.** An owner has one `$X` prop however many `<$X>` elements
  fill it (repeated: last wins; keyed: one per key), so only one tag can be
  copied — the first, where diagnostics already land. The transpiler exports,
  per owner and name, the tag-name spans (opening and closing) of every
  element. A request on any other span of the group is answered at the copied
  one, with the answer's own range set back to the requested token.
- **Syntactic features run on the source tree**, not the virtual text:
  folding, selection ranges, closing-tag insertion. A slot element has no
  element in the virtual text, and the rtsx parser yields standard node
  kinds, so the fork's providers run on it directly.

### Tolerance

A file being typed rarely parses. The transform never fails on user input:

1. Passes run on TS's recovered tree (`Input.Tolerant`). A pass that fails
   keeps the previous pass's text and map; the result is marked *stopped*.
2. Syntax errors are the **source** parse's only; TS's syntactic diagnostics
   of the virtual text are never shown.
3. A transpiler diagnostic is dropped when its node, or the nearest JSX
   element or fragment enclosing it, contains a parse error (no
   `case-no-test` on a half-typed `<$Case`; no `orphan-slot` under an element
   whose attribute is half-typed).
4. For a *stopped* file — up to the last resort, the source as virtual text
   mapped 1:1 — every TS diagnostic of the file is dropped (unlowered
   constructs would produce false ones). Source parse errors and transpiler
   diagnostics of the completed passes remain; hover and completion keep
   working.
5. A panic inside the transform is caught at the mapper boundary: step 4,
   plus one `internal` diagnostic on the first line naming the pass. The
   server keeps running.

When the source parses clean, nothing is hidden: a pass that fails, or
emitted text that does not parse, is `internal` at 1:1, as in `check`, and
tolerant output equals strict output byte for byte.

The build paths stay strict: Vite and `reactogenic check` fail on a syntax
error as today.

## `reactogenic lsp`

`reactogenic lsp --stdio`: the fork's TS7 language server with the mapper
built in. Standard LSP with **static capabilities only** — nothing is
registered dynamically — so any client that attaches it to `*.rtsx` works.
It advertises what this table lists, and no formatting.

| Feature | `.rtsx` support | How |
| --- | --- | --- |
| diagnostics | **the same as `reactogenic check`** | *Diagnostics* |
| hover, signature help | ✓ | TS on virtual text, mapped back |
| go to definition, type definition, references, highlights | ✓ | TS; `#name` → *Segments* |
| completion, auto-import | ✓ | TS; slot names → *Slots* |
| rename | correct or refused | *Rename* |
| document symbols, semantic tokens, inlay hints | ✓ | TS |
| folding, selection ranges | ✓ | the source tree |
| closing-tag insertion | ✓, `$` tags included | *Tags* |
| code actions, organize imports | only when every edit maps exactly | TS drops the rest |
| workspace symbols | symbols declared in `.rtsx` files | the rest is the user's TypeScript's |
| file rename | a renamed `.rtsx`: edits in every importer; a renamed `.ts` / `.tsx`: edits in `.rtsx` importers only | no other server knows `.rtsx` modules |
| formatting | ✗ — not advertised | *Not in the first release* |

`.ts` and `.tsx` files of the same project are in the server's program (for
types across the boundary), but the client attaches it to `.rtsx` documents
only (*The `.ts` side*). It reads them from disk: an unsaved `.ts` edit
reaches `.rtsx` files on save.

### Diagnostics

The editor shows what `reactogenic check` prints — same codes, same messages,
same positions. One **reporting layer**, shared: given the program and a
file, it returns the reports for that file.

- Transpiler errors and warnings (`orphan-slot`, `segment-children`, …), with
  their names as codes and their severities. They travel beside TS's
  diagnostics, never through the mapper result: there, any one of them would
  hide every type error of the program.
- TS errors, rewritten into slot terms (diagnostics.md, *Rewrites*). The
  layer sees the structured diagnostic — message arguments and chain — and
  the file's notes; a TS error that is not rewritten keeps its numeric code,
  so TS's quick fixes still find it. Suggestions pass through untouched.
- The cross-file rules: `slot-conditional` and `segment-self`.
- TS5097 on a segment import of a `.tsx` / `.ts` file is dropped: the import
  is the transpiler's (syntax.md, *Segment files*).
- **Each mistake once.** Code copied to several virtual places is checked in
  each. Diagnostics with the same range, code and message are merged; a
  diagnostic in a secondary copy is dropped when another copy of the same
  source text has one with the same code at the same range (branches narrow
  differently, so the messages may differ). `check` gains this too: it
  printed such errors twice.

In the server the layer sits where the diagnostic is still structured, before
it becomes an LSP message. Diagnostics are pulled per open document; after a
change to any file the server asks the client to pull again.

Project-wide errors stay with `reactogenic check`. The extension contributes
the task *reactogenic: check* — the binary the server runs, `check
--pretty=false` — with the problem matcher `$reactogenic`, applied to closed
documents only (an open document's problems come from the server).

### Slots

- After `<$` inside a component's children: its declared slots — TS's
  property completion at the copied name, filtered to `$` names, with the
  `$` names already written under the same owner appended last (TS omits
  props that are already there; a keyed slot is filled many times).
- Inside a slot tag: completion and checking of the slot's props (free: the
  attribute names are the object's keys).
- Hover on a slot tag — any element of its group — shows the declared slot
  type; go to definition jumps to the `$X` member of the container's props.

### Segments

- Go to definition on `#about-us` opens the mounted file (the lookup of
  syntax.md, *Segment files*).
- After `#`: the sibling modules not yet mounted in the file.
- A segment file created or deleted updates its mounters' errors without an
  edit to them. The transform's only file-system input is the **names** in
  the file's directory, read through the server's file system (unsaved
  buffers included); the directory's listing is part of the transform's cache
  key. `segment-self`, which follows mounts through other files' contents, is
  a cross-file rule of the reporting layer for that reason.

### Tags

TS cannot answer these: a slot element has no element in the virtual text,
and a half-typed tag is where the map is least exact. The server answers
from the tolerant source parse: typing `>` after `<div` or `<$Icon { size }`
inserts the closing tag (`textDocument/_vs_onAutoInsert`, which the extension
sends itself — the language client has no support for it).

### Rename

**A rename started in an `.rtsx` document is correct or refused — never
partial.** TS finds the occurrences on the virtual text; the server builds
the edits itself, from each occurrence's *virtual* position (after mapping,
the two copies of a shorthand are one source range):

| Site | The server |
| --- | --- |
| an occurrence in plain copied text | the mapped edit |
| shorthand `<Button size>` | expands: the binding renamed gives `size={dim}`; the prop renamed gives `scale={size}`; both renamed, the plain token |
| arg shorthand `&size` | the same: `&size={dim}`, `&scale={size}` |
| `&&size` | the binding renamed gives `&&size={dim}`; the arg or the prop renamed: refused |
| a tag name with a closing tag | every edit inside an opening tag name is repeated at the same offset in its closing tag name (on the source tree: components, intrinsics, slot elements) |
| a slot tag | every `<$X` / `</$X>` of its slot group; refused unless the new name starts with `$` |
| an occurrence that cannot be written back | refuses the whole rename, as an error naming the place |

Then a post-check: the edits are applied in memory to each touched `.rtsx`
and passes 0–1 re-run; the rename is refused if a file gains a parse error,
or a bare attribute outside the edits flips between bound and `true` (the
new name captured it).

### Commands

`reactogenic/transpiled` (custom request): the emitted TSX of a document and
the tolerance step that produced it. *Show transpiled TSX* opens it read-only
beside the source and refreshes it on every edit.

### Not in the first release

| | |
| --- | --- |
| formatting | through the virtual text it re-indents moved slot bodies. Needs its own implementation; Prettier, Biome and ESLint do not parse `.rtsx`, so format-on-save leaves the file untouched |
| rename from a `.ts` file into `.rtsx` | refused there (*The `.ts` side*); start it from the `.rtsx` file |
| renaming a segment | `#about-us` has no virtual token and its import is generated: the `#name` rename is refused; renaming the file leaves `segment-not-found` on the mounter |
| unsaved `.ts` edits | reach `.rtsx` files on save |
| linked editing of slot tag pairs | `editor.linkedEditing` is off by default |
| shorthand inlay hint | `disabled`⟨`={disabled}`⟩ on every bound bare attribute (syntax.md, *Silent flip*) |
| quick fix for `segment-not-found` | create the file with an empty default component |
| hover docs on `Switch` / `Match` / `$Case` | they are lowered away; static text |
| param completion in `{ }` of a slot without a body | needs params emitted without a body, which changes diagnostics |
| Problems for closed files | open documents only; run the *reactogenic: check* task. A background `--watch` matcher comes later |
| push diagnostics | for clients without pull support |
| a server per workspace folder or CLI version | one server per window |
| extensions keyed on `typescriptreact` | e.g. Tailwind CSS IntelliSense needs `"tailwindCSS.includeLanguages": { "rtsx": "typescriptreact" }`; `[typescriptreact]` settings must be repeated for `[rtsx]` |

> OPEN: auto-import writes `./button.rtsx` (TS's specifier for a mapped
> file). Valid everywhere; the convention is extensionless. Strip it?

> OPEN: at the end of a copied expression followed by generated text
> (`on={getSta|}`) the position maps forward but not back exactly, so
> auto-import items are missing there. A small patch to the span map's
> position lookup; decide by a test in RGP1-105.

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

- **Contributes**: the languages `rtsx` (`.rtsx`) and `rtsx-tags` (inside
  tags: `{/* */}` comments, indent after `<$Slot>`); the grammar, with what
  VS Code's own TSX entry carries (`embeddedLanguages`,
  `unbalancedBracketScopes`, `tokenTypes`, `semanticTokenScopes`);
  breakpoints for `rtsx`; TSX's snippets; Emmet as for TSX; a Markdown fence
  injection (` ```rtsx `); the commands *Restart server* and *Show
  transpiled TSX*; the *reactogenic: check* task and `$reactogenic` matcher.
- **Client**: `vscode-languageclient` over stdio; selector language `rtsx`,
  schemes `file` and `untitled` (not `git:` — the left side of a diff).
- **Which binary runs**, first match:

  | | |
  | --- | --- |
  | 1 | the setting `reactogenic.server.path` |
  | 2 | `$REACTOGENIC_BINARY` |
  | 3 | the workspace's `@reactogenic/cli`, found by walking up from the folder of the first `.rtsx` document opened — if its version is ≥ the extension's minimum (the first version with `lsp`; pre-release tags compared numerically) |
  | 4 | the binary bundled in the `.vsix` |

  The workspace's own CLI comes first so the editor and `reactogenic check`
  agree. One server per window. A status item names the binary, its version
  and where it was found, and warns when the workspace CLI was skipped as too
  old — the editor then runs a newer transpiler than that project's `check`.
  A path from row 1 or 2 that does not exist is an error, not a
  fall-through. The server restarts when a lockfile or a `reactogenic.server.*`
  setting changes, and when the workspace becomes trusted. On Windows a
  workspace binary runs from a copy, so `pnpm install` can replace it.
- **Trust**: an untrusted workspace gets highlighting only — the server reads
  tsconfig and runs a binary from `node_modules`.
- **Packaging**: one `.vsix` per platform (the six of `@reactogenic/cli`),
  each with its binary and the licences of the code in it, plus one universal
  `.vsix` without a binary (highlighting anywhere; the server if rows 1–3
  find one). `engines.vscode: ^1.91.0`, the language client's floor.

### The `.ts` side

`src/main.tsx` imports `./page`, and VS Code's TypeScript — a different
server — does not know `.rtsx`: it reports TS2307 there.

| The user's TypeScript server | How it learns `.rtsx` |
| --- | --- |
| built-in `tsserver`, TS ≤ 6 (VS Code's default; Cursor, VSCodium) | the **TS server plugin** the extension contributes |
| the "TypeScript 7" extension with a 7.0.x server | nothing: plugins are not loaded and 7.0 has no mappers. `.ts` files report TS2307 on `.rtsx` imports; `.rtsx` files are unaffected. That extension warns that our plugin will not load and offers *Disable Native Preview in Workspace* — the remedy until 7.1 |
| TS 7.1+ native server | the stock content mapper (below) |

**The plugin** — a real folder in the `.vsix`, CommonJS, no dependencies,
loaded by `tsserver` in every TS project, so it costs nothing until an
`.rtsx` import appears:

- *Resolution*: TypeScript's own first; on failure, `x.rtsx` — so built-in
  extensions win, and `paths`, `index.rtsx` and an explicit `./x.rtsx`
  behave as in the server.
- *Text*: the file's text, for `tsserver`, is the emitted TSX (tolerant
  mode: a saved syntax error does not empty the module; a file that yields
  nothing keeps its last good text).
- *Transform*: synchronous — `spawnSync` of the binary's `serve`, one spawn
  per batch of files, cached by mtime and size. `serve` returns the span
  tuples of *Span map*.
  > OPEN: spawn cost on Windows is unmeasured (5 ms warm on macOS). If it
  > is slow, one long-lived `serve` child behind `Atomics.wait`.
- *Answers*: every `tsserver` answer that points into an `.rtsx` file is
  mapped to source positions — definition, type definition, implementation,
  references, call hierarchy, navigate-to, the related information of
  diagnostics, file-rename edits; a location without an exact source span is
  dropped. **A rename that reaches an `.rtsx` file is refused**, naming the
  file: it is started from the `.rtsx` side, where the *Rename* table applies.
- It serves saved files: an unsaved `.rtsx` edit reaches `.ts` files on save
  (as in Svelte's plugin).

### Stock TypeScript 7.1

The same transform, as a standard content mapper: `reactogenic content-mapper`
speaks the mapper protocol, and `@reactogenic/cli` declares it
(`typescript.contentMapper` in its `package.json`). A project that lists it in
tsconfig `contentMappers` gets `.rtsx` in plain `tsc --runExternalCode` and in
the TS 7.1 server — for editors without our server.

**Experimental**, with the limits of the contract: raw TS messages (no
rewrites); explicit `.rtsx` in imports written in `.ts` files; transpiler
warnings not shown, and a transpiler error hides the project's type errors
until fixed; rename not fixed up; a created or deleted segment file is seen
when its mounter next changes; until 7.1 is stable, the "TypeScript 7 Nightly"
extension and a trusted workspace.

**Not combined with our extension in VS Code**: the TS 7.1 server would also
answer inside `.rtsx` documents — each hover and each TS error twice.
`"js/ts.contentMappers.enabled": false` in the workspace restores one server.

## Other editors

`reactogenic lsp --stdio` is a standard server: Neovim (`vim.lsp`), Zed,
Helix and JetBrains can run it for `*.rtsx`. Highlighting there needs the
deferred tree-sitter grammar (JetBrains takes the TextMate one).

## Testing

| Layer | Test |
| --- | --- |
| transform | conformance corpus: the span map validates; virtual nodes map to the same source span as `emit.Map`; no source offset has two projections that answer the same feature, except shorthand; tolerant mode over typing-like mutants of the fixtures never panics and never loses the file |
| server | a Go test client runs the server in-process over a pipe (race-instrumented), one scenario per feature row above on a fixture project, plus one smoke scenario through the built binary. The client behaves as VS Code does: UTF-16, pull diagnostics with refresh, watched-file events; it fails the test on a server request it did not answer |
| `check` | golden output recorded from the overlay model before the migration; reproduced on the mapped program except the listed differences |
| grammar | scope assertions per construct; equality with `source.tsx` on plain TSX; no `invalid.*` token in any `.rtsx` of the repo |
| extension | binary resolution unit tests; an editor suite in an isolated VS Code profile: language id, one slot-term diagnostic, exactly one hover and one definition result |
| plugin | `tsserver` driven over stdio: no TS2307, references at source positions, rename refused |
