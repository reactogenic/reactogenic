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
  | slot tag name `<$Icon` | the prop name `$Icon=` / the key `$Icon:` (in quotes when not an identifier: `"$sub-item":`) | hover, definition, references on the slot |
  | attribute names of a slot element | the object keys | hover, completion of slot props |
  | arg names `&size`, `&&value` | the arg keys | hover, definition |
  | closing tag name `</Button>` | the rebuilt closing tag, when one is emitted | hover, definition there |

  These copies carry no semantic tokens: tag and attribute names keep the
  grammar's colours with or without a server. The emitted text is unchanged;
  three diagnostic columns move from the `<` / `&` to the name.
- **Several copies of one token.**
  - *Identical copies* — an attachment emitted in both branches of its
    ternary, the `$X` of `slot={$X}`, a `Switch` subject repeated per case, a
    slot's previous value kept by each null branch of a conditional: the
    first copy has the features; the others have none — names and both
    copies of a shorthand included.
  - *Shorthand* — `<Input value />`, `&size`, a slot's bare attribute — where
    the two copies are different symbols (the prop and the binding): both
    answer, as TS does on `{ value }`.

    | Copy | Features |
    | --- | --- |
    | name (the prop / arg key) | hover, completion, definition, type definition, references, highlights |
    | value (the binding) | hover, definition, references, highlights, rename, semantic tokens, inlay hints |
- **Attribute strings.** A string attribute whose value moves to a JS
  position — a slot prop, `is=`, `on=`, an arg, a key — is copied when it
  reads the same as a JS string, so hover and completion work inside
  `is="loading"`. One with a backslash, `&` or a line break does not (JSX has
  no escapes, allows line breaks and decodes `&amp;`): it is written as the
  JS literal of its value, an atom on the string.

  ```tsx
  <$Label title="Tom &amp; Jerry" path="C:\new" />   // .rtsx
  $Label={{ title: "Tom & Jerry", path: "C:\\new" }}  // virtual TSX
  ```
- **Slot groups.** An owner has one `$X` prop however many `<$X>` elements
  fill it (repeated: last wins; keyed: one per key), so only one tag can be
  copied — the first, where diagnostics already land. The transpiler exports,
  per owner and name, the tag-name spans (opening and closing) of every
  element — once, like every note and diagnostic, however often the owner's
  text is emitted (an attachment's fallback is emitted twice). A request on
  any other span of the group is answered at the copied one, with the
  answer's own range set back to the requested token: hover, definition,
  type definition, implementation, references, highlights, completion,
  `prepareRename` and rename. The front moves the request and sets the range
  back; it reads the groups off the document's text with the same transform.
  - **Closing tags** likewise. An element whose children are all slots is
    emitted self-closing, so its `</Card>` has no copy: it is answered at
    the opening tag name, paired on the source tree. The range is set back
    part by part: on `</Kit.Panel>` it is the closing tag's `Kit`, or its
    `Panel`. Completion is the exception: the name of a closing tag is not
    chosen, and one that has no copy lists nothing.
  - **References and highlights** add what TS cannot list: the other tags
    of a group, an element's other tag name — references in every `.rtsx`
    file of the answer, whichever document asks (the container's declaration
    of a slot lists every tag in the pages; a file that is not open is read
    from disk, as the server reads it). On a tag TS highlights the
    element's two tags, whole; of a rebuilt element neither is source text,
    and the front highlights its names.
- **Syntactic features run on the source tree**, not the virtual text:
  folding, selection ranges, closing-tag insertion. A slot element has no
  element in the virtual text, and the rtsx parser yields standard node
  kinds, so the fork's providers run on it directly.

### Tolerance

A file being typed rarely parses. The transform never fails on user input:

1. Passes run on TS's recovered tree (`Input.Tolerant`). A pass that fails
   keeps the previous pass's text and map; the result is marked *stopped*,
   naming the pass.
2. Syntax errors are the **source** parse's only, each once (the parser
   reports a missing `</` at the end of the text once per element still
   open); TS's syntactic diagnostics of the virtual text are never shown.
3. A transpiler diagnostic is dropped when its node, or a JSX element or
   fragment enclosing it, contains a parse error (no `case-no-test` on a
   half-typed `<$Case`; no `orphan-slot` under an element whose attribute is
   half-typed, or after a tag that is not closed yet).
   - *Contains*: the parser flagged a node there, or the range of one of its
     errors lies there — an unclosed tag (TS17008) leaves no flag.
   - Decided on the **source** parse: a diagnostic of a later pass is judged
     where the author wrote the construct. That pass's own parse can only
     add to it — recovery may re-parent lowered text differently.
   - Every element around it counts, not the nearest alone: what is in a
     loop, or under which owner, is read off the elements above, and an
     unterminated string re-parents what follows it (`#intro` "would be
     mounted more than once", under an `Each` it is not in). The price: a
     true diagnostic elsewhere in that JSX tree waits until the tree parses.
     Nothing outside the tree counts: a syntax error elsewhere in the file
     hides nothing.
   - A node with no element around it (a root element) is judged by the
     **statement** it is written in: recovery turns the children of an owner
     whose opening tag is half-typed (`<Button variant=>`, a deleted `>`)
     into top-level expressions of that statement. An error in another
     statement or another function hides nothing. A file-level diagnostic
     (`ambiguous-module`) is never dropped.
   - A diagnostic decided by the names in scope — `component-name` — is
     judged by its **top-level statement** (rule 4's unit): an unclosed
     brace declares the next component inside this one, where `$Title` is a
     binding.
   - What the dropped error was about is left out, and the construct is
     lowered around it: a `$Case` being typed does not take its `Switch` —
     the subject, the other cases — out of the virtual text.
4. TS says nothing about a **top-level statement that does not parse** — a
   component being typed. What the passes lowered there was a recovered
   tree, and the virtual text is broken in its own way: TS's errors about it
   land on lines the author is not touching, reworded in slot terms that are
   not true or naming generated helpers.

   ```tsx
   <section>
     <$Title c                                               // being typed
     <h1 slot={$Title} className="default">Untitled</h1>     // untouched
   </section>
   // shown: the syntax errors
   // not: `$Title` is a list; a slot is one value      (TS2345, reworded: slot-list)
   //      Binding element 'isAssigned' implicitly has an 'any' type.
   ```

   - *Does not parse*: the statement holds a syntax error of the source
     parse (*contains*, as in rule 3) — or the statement of the virtual
     text that TS's diagnostic is in holds one of TS's own parse: the two
     recover differently, and TS may take in statements that the source
     parse left whole.
   - The unit is the top-level statement, not the innermost: an unclosed
     brace or string moves what follows into another scope, where a
     statement can parse and still not be what the author wrote.
   - The other top-level statements are checked as ever: a type error in
     another component stays while this one is typed. The price: a true
     type error in the statement being typed waits until it parses.
   - `slot-conditional` there is dropped too: it asks the checker.

   For a *stopped* file — up to the last resort, the source as virtual text
   mapped 1:1 — every TS diagnostic of the file is dropped (unlowered
   constructs would produce false ones). The same when code was **left
   out**: a `Switch` or `Match` that cannot be lowered and an orphaned slot
   element become `null`; a half-typed `$Case` with a body, a conditional
   child that mixes slot elements with anything else, and a `children`
   attribute next to a body are skipped — whether their error was shown or
   dropped, every name used only there would read as unused. (The children
   of a segment root are overwritten by design, with a warning: the file is
   not marked.) And when a construct **stays as written**: an arg on an
   element without `slot={$X}` (`arg-without-slot`) and params on an
   intrinsic element (`params-on-html`) are errors that no pass lowers, so
   the virtual text is not TSX — TS reads `<option &size />` as `<option`
   `& size / >`, and reports on that. Source parse errors and transpiler
   diagnostics of the completed passes remain; hover and completion keep
   working.
5. A panic inside a pass is caught there, whether the source parses or not:
   step 4 from the last good text, plus one `internal` diagnostic on the
   first line naming the pass. A panic anywhere else in the transform is
   caught at the mapper boundary, with the source as virtual text. The
   server keeps running.

When the source parses clean, nothing is hidden: a pass that fails, or
emitted text that does not parse, is `internal` at 1:1, as in `check`, and
tolerant output equals strict output byte for byte — the map and everything
exported beside it included.

The build paths stay strict: Vite and `reactogenic check` fail on a syntax
error as today. `check` registers the same mapper with the passes strict: a
file with a syntax error reports its syntax errors and is not lowered. Its
source stands in as its virtual text — the last resort of rule 4, so the file
is stopped and its importers still find what it exports. Rules 2 and 4 are
the reporting layer's (*Diagnostics*), the same in both hosts.

## `reactogenic lsp`

`reactogenic lsp --stdio`: the fork's TS7 language server with the mapper
built in. Standard LSP with **static capabilities**: no feature is
registered dynamically, so any client that attaches it to `*.rtsx` files
works; configuration and file watching are, when the client supports it —
and initialization does not wait for the client's answer. It advertises
exactly what this table lists: no formatting, no code lens.

A **front** in the same process sits between the client and the fork's
server. It keeps the text of each open `.rtsx` document (the server asks for
whole documents on change; a client that sends a range anyway is served — the
change is applied as the server applies it, and one with a position no
document has goes no further) and answers the source-tree features itself.
It never blocks on writing to the client: the client may be blocked writing
to it. A failure inside the front answers that request with an error; the
process goes on.

The workspace-wide answers are narrowed to what is ours **in the server**
(the fork's `Embedder.Owns`), where the choice is still per symbol and per
import — not on the finished answer.

**How the process ends** — the front's too, so that it holds in any state
of the server:

| | Exit status |
| --- | --- |
| `exit` after `shutdown` | 0 |
| `exit` without `shutdown` — also before `initialized`, or before `initialize` | 1 |
| the input ends | 0 |
| input that is not LSP (no `Content-Length`, a message cut short) | 1, the reason on stderr |
| the client's process is gone: the `processId` of `initialize`, or `--clientProcessId <pid>`, probed every 5 s (a killed editor whose pipe another process still holds); answers nobody reads are given one second, not waited for | 1 |

| Feature | `.rtsx` support | How |
| --- | --- | --- |
| diagnostics | **the same as `reactogenic check`** | *Diagnostics* |
| hover, signature help | ✓ | TS on virtual text, mapped back |
| go to definition, type definition, implementation, references, highlights | ✓ — on a module specifier: the module's file, a `paths` alias included | TS; `#name` → *Segments* |
| call hierarchy | ✓ — a call in a slot body is the enclosing component's; generated calls are not listed | TS |
| completion, auto-import | ✓ — also in a file whose only import is generated (*Specifiers the server writes*) | TS; slot names → *Slots* |
| rename | correct or refused | *Rename* |
| semantic tokens | ✓ | TS |
| inlay hints | ✓ — each once; none on a generated call (`slot:` before the `$X` of `slot={$X}`); a slot's params get their type | TS |
| document symbols, folding, selection ranges | ✓ — a declaration's range is its own, whatever it holds; slot attributes, args and params are not symbols | the source tree |
| closing-tag insertion | ✓, `$` tags included | *Tags* |
| linked editing | plain tag pairs; none — rather than a wrong pair — on a tag that has slots, or on a slot tag | TS; *Not in the first release* |
| code actions | quick fixes: only when every edit maps exactly | TS drops the rest |
| organize / sort / remove unused imports | where every import is the author's (a file that only fills slots). **Not offered** in a file with a generated import — never an action with no edit | TS; *Not in the first release* |
| workspace symbols | symbols declared in `.rtsx` files — chosen before TS cuts to its best 256 | the rest is the user's TypeScript's |
| file rename | per import: an edit in an `.rtsx` document, or of an import of an `.rtsx` module, is ours. So: a renamed `.rtsx` — every importer; a renamed `.ts` / `.tsx` — its `.rtsx` importers; several files, or a folder — each import by that rule; a moved `.rtsx` file's own imports follow it (not a generated one: *Not in the first release*, renaming a segment) | no other server knows `.rtsx` modules |
| formatting | ✗ — not advertised | *Not in the first release* |

`.ts` and `.tsx` files of the same project are in the server's program (for
types across the boundary), but the client attaches it to `.rtsx` documents
only (*The `.ts` side*). It reads them from disk: an unsaved `.ts` edit
reaches `.rtsx` files on save, and a rename's edit of such a file is made
for the saved text (*Rename*: refused while the file has unsaved changes).

**A document without a file name** — an untitled one, `untitled:Untitled-1` —
has no extension to find the mapper by. The language id of `didOpen` decides:
a document that is not a `file:` and is opened as `rtsx` is a mapped `.rtsx`
file. The front serves it to the fork under a name ending in `.rtsx` and
translates that name in every message, both ways; nothing is decoded while no
such document is open. A name that ends in `.rtsx` already
(`untitled:/tmp/new.rtsx`) is mapped by its extension whatever it is opened
as, and has the one served name: closed and opened again as another language,
it is one document reopened. It belongs to no project: relative imports do not
resolve, and there are no JSX types — as for an untitled TSX document.

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
- The cross-file rules: `slot-conditional` (the caller's note, the
  container's notes, joined by the checker) and `segment-self` through
  another file (the mounts of the program's `.rtsx` files, followed from
  their notes; a root that names its own file is the transpiler's).
- On the import a segment root emits — the transpiler's, not the author's
  (syntax.md, *Segment files*) — two TS errors are dropped: TS5097, for a
  `.tsx` / `.ts` file named with its extension; and TS2307 / TS2792, no such
  module, when the root has `segment-not-found`: one mistake, one line.
- For a **stopped** file, no TS diagnostic at all, and none in a top-level
  statement that does not parse (*Tolerance*, rule 4); TS's syntax errors of
  a virtual text, never (rule 2).
- **Each mistake once.** Code copied to several virtual places is checked in
  each. Diagnostics with the same range, code and message are merged; a
  diagnostic in a secondary copy — one that answers no feature (*Span map*,
  *Several copies of one token*) — is dropped when another copy of the same
  source text has one with the same code at the same range (branches narrow
  differently, so the messages may differ). Nothing else is merged: two
  diagnostics on synthesized text share their construct's range without
  being one mistake, and both copies of a shorthand answer. `check` gains
  this too: it printed such errors twice.

  ```tsx
  <b slot={$Label} title={$Label.nope}>Button</b>   // emitted in both branches of a ternary
  // TS2339 … on type '{ title?: string; … }'       the first copy: reported
  // TS2339 … on type 'unique symbol'               the fallback's copy: dropped
  // TS18048 '$Label' is possibly 'undefined'       the fallback's copy only: reported
  ```

  A mistake is never dropped altogether. The secondary copies yield first;
  what is left is then merged — in that order, because the copy that answers
  need not come first:

  ```tsx
  <tr slot={$Row} &&className={row.nope} />   // the element's prop, then the arg — which answers
  // TS2339 … 'nope' …   the prop's copy, secondary: dropped
  // TS2339 … 'nope' …   the arg's copy, the same message: reported
  ```

A report carries its source range, severity, code, message, related
information and — when it came from TS — the structured diagnostic.

Two differences between the hosts, both TypeScript's own:

- `check` reports in `tsc`'s steps (diagnostics.md, step 3), a document's
  reports have none. While a `.ts` / `.tsx` file has a syntax error, or — in
  a project that emits declarations — next to a type error, the editor shows
  the type or declaration errors that `check` does not print yet.
- TypeScript's style checks (`noUnusedLocals`, `noUnusedParameters`, …) are
  errors in `check` and warnings in the editor, as in VS Code's own
  TypeScript: the user's `reportStyleChecksAsWarnings`, on by default. The
  codes are TypeScript's list (TS6133, not TS6198); with the setting off the
  editor shows `check`'s lines.

  ```
  src/page.rtsx(3,9): error TS6133: 'unused' is declared but its value is never read.   check, and the task's problem while the document is closed
  3:9 warning TS6133                                                                    the document, open
  ```
  > OPEN: keep TypeScript's default, or errors unless the user sets
  > `reportStyleChecksAsWarnings`? Today an `.rtsx` document reads like the
  > `.tsx` next to it, and not like `check`'s line for it. Not decided: the
  > server does for an `.rtsx` document what TypeScript's does for a `.tsx`
  > one (RGP1-107).

(And the builds are strict where the editor is tolerant: a file with a syntax
error — *Tolerance*.)

In the server the layer sits where the diagnostic is still structured — the
fork's `Embedder.Diagnostics`, asked for the program and the file in place of
TS's own path, which would map each diagnostic back by position and gather
those in generated code at the top of the file. The server makes the LSP
diagnostic from the report:

| A report | As an LSP diagnostic |
| --- | --- |
| the transpiler's, a cross-file rule's, a TS error reworded | `code`: its name (`"undeclared-slot"`); `source`: `reactogenic` |
| a TS diagnostic that is not reworded; a syntax error of the source | `code`: TS's number (`2322`); `source`: `ts` — what TS's quick fixes match on |
| a warning | a warning |
| a TS suggestion — an unused name, a deprecated one | a hint with its tag (faded, struck through): passed through |
| a span of no length | the character after it, a line break too (both of a CRLF); empty at the end of the text |
| related information | a location in the source of the file it names — an `.rtsx` file's, mapped; without a file, the diagnostic's own range |

`validate.enable: false` (TypeScript's setting) turns an `.rtsx` document's
diagnostics off, the transpiler's with TypeScript's.
> OPEN: keep the transpiler's — syntax errors, `orphan-slot`: what fails the
> build — when TypeScript's validation is off? Not decided: one switch for
> the whole answer, as TypeScript's for a `.tsx` document (RGP1-107).

Diagnostics are pulled per open document. A client pulls the document that
changed; the others follow because the server asks it to pull again
(`workspace/diagnostic/refresh`) after:

- an edit of any open `.rtsx` document;
- a watched file created, changed or deleted — a `.ts` file, a segment file,
  a tsconfig;
- an `.rtsx` document opened or closed with a text that is not the file's on
  disk: a file never saved, a buffer the editor restored, one closed without
  saving. Opening a saved file asks for nothing.

**A document's project is its lister**, as in `check` (diagnostics.md,
*References*): of the projects that hold an `.rtsx` file, the one whose
tsconfig lists it — the first, when several do — not the first that reaches
it through an import, which is TypeScript's choice for its own files. With a
test project referenced before the app it tests, a file of the app is checked
under the app's options, whichever document was opened first; hover and
completion are that project's too.

The rule is **among a tsconfig and the projects it references** — what
`check -p` reads — starting, as TypeScript does, from the nearest
`tsconfig.json` above the file. When none of them lists the file, it is the
first of them that imports it; a tsconfig further up is asked only when none
of them holds the file at all, as for a `.tsx` file.

```jsonc
// tsconfig.json — a base the packages extend; by default it lists every file under it
{ "compilerOptions": { "strict": true } }
// packages/app/tsconfig.json — main.tsx imports ./page
{ "extends": "../../tsconfig.json", "compilerOptions": { "jsx": "preserve", "paths": { "@/*": ["./src/*"] } },
  "include": ["src/**/*.ts", "src/**/*.tsx"] }
// packages/app/src/page.rtsx is the package's — its `paths`, its `jsx` — as `check -p packages/app` reports it
```

Project-wide errors stay with `reactogenic check`. The extension contributes
the task *reactogenic: check* — the binary the server runs, `check
--pretty=false` — with two problem matchers, applied to closed documents
only. An open document's problems come from a server and replace the task's
for that file, so a line's problem has the owner of the server that reports
the file once it is open:

| Matcher | Lines | Owner |
| --- | --- | --- |
| `$reactogenic` | `.rtsx` files | `reactogenic`: our server |
| `$reactogenic-ts` | every other file (`.ts`, `.tsx`, …) | `typescript`, as `$tsc`: VS Code's TypeScript — ours is not attached there |

The extension's collection for the owner `reactogenic` lives as long as the
window: a server restart removes the open documents' problems only, never
what the task left for closed ones.

### Slots

- After `<$` inside a component's children: **exactly its `$` props**. The
  front asks the checker for them at the owner's tag (`reactogenic/slots`,
  its own request: the props of a component at its tag name, of a slot's
  value at the slot's name), so the list does not depend on what the
  half-typed tag did to the tree:

  | `<$` typed | Lists |
  | --- | --- |
  | under a component, or a slot element (recursive slots) | that owner's `$` props, written or not — a keyed slot is filled many times |
  | under a `Match`, in a `$Case` | the same: the owner is the component above them, also while the tag is still open (`<$`, `<$T`, `<$>`) and TS has no token to answer at |
  | next to a slot element with slots of its own | the owner's, not the nested component's |
  | under a `Switch`, which is lowered away | `$Case` |
  | in a closing tag | nothing |

  TS's property completion at the copied name supplies the items — with
  their documentation — for the props not written yet; the front makes the
  others, last, with the slot's type as their detail. An optional slot is
  labelled `$Icon?`, as TS labels it; the inserted text is the bare name.
  When the owner has no type (it is being typed too): TS's `$` items, and
  the `$` names written under the owner.
- Inside a slot tag: completion and checking of the slot's props (free: the
  attribute names are the object's keys).
- Hover on a slot tag — any element of its group, either tag — shows the
  declared slot type; go to definition jumps to the `$X` member of the
  container's props. (Type definition answers nothing there: to TS the type
  of a prop written in JSX is its value's, and a slot's value is the object
  the transform builds.)

### Segments

- Go to definition on `#about-us` opens the mounted file (the lookup of
  syntax.md, *Segment files*).
- After `#`: the sibling modules not yet mounted in the file — each name
  once, with the file the lookup would take; not the file itself, a
  declaration file, or a name that is no segment's (`404.tsx`).
- Both read the names next to the document from the server
  (`reactogenic/siblings`, the front's own request): its file system, so a
  buffer never saved is a sibling.
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
sends itself — the language client has no support for it). With several
cursors it asks once per cursor and each gets its own tag; a cursor that is
not in a tag gets none.

### Rename

**A rename started in an `.rtsx` document is correct or refused — never
partial.** TS finds the occurrences on the virtual text; the fork hands them
over before any is written back (`Embedder.Rename`: the file, the virtual
range, whether it lies in one verbatim span, the new text — gathered across
projects), and the server builds the edits itself, from each occurrence's
*virtual* position (after mapping, the two copies of a shorthand are one
source range):

| Site | The server |
| --- | --- |
| an occurrence in plain copied text | the mapped edit; identical copies are one edit |
| shorthand `<Button size>` | expands: the binding renamed gives `size={dim}`; the prop renamed gives `scale={size}`; both renamed, the plain token |
| arg shorthand `&size` | the same: `&size={dim}`, `&scale={size}` |
| `&&size`, `&&size={x}` | the binding renamed gives `&&size={dim}`; the arg or the prop renamed: refused |
| a tag name with a closing tag | every edit inside a tag name is repeated at the same offset in the element's other tag name (on the source tree: components, intrinsics, slot elements; `<UI.Button>`) |
| a slot tag | every `<$X` / `</$X>` of its slot group; refused unless the new name starts with `$` |
| any other tag; the name in `slot={…}` | refused when the rename would add or take its `$`: the `$` makes the slot element, and the attachment |
| any tag, in any file | refused when the rename would move it between a component and an intrinsic element (`Box` → `box`, `my-box`): the first letter, or a hyphen, decides what a tag refers to |
| code that no virtual text holds — the children a segment root overwrites, a slot element that a later one replaces — in every mapped file that holds the name, with or without an occurrence | a binding read in an expression, and the tag of a component (both its tags), are renamed with their declaration, which the source's own scopes find. Any other name there refuses: an attribute's (a bare one too: the prop, the binding, or both), a slot's or an intrinsic tag, a member, a type, a name that nothing in the file declares |
| an occurrence that cannot be written back | refuses the whole rename, as an error naming the place — generated code: the `children` that a slot's body becomes, the prop of `&&name` |
| a file that is stopped or has a syntax error, and holds the name | refuses: what is left out of its virtual text may be an occurrence |
| the key of a binding pattern written as a string — `{ "$sub-item": $sub }`, how a container destructures a slot whose name is no identifier | an occurrence: TS's search does not list it, the server adds it (in `.tsx` files too) |
| a string (`is="loading"`, `mode === "a"`) | refused: TS finds the strings of its type wherever they are written — another function's, the library's — and not always the type that declares them |
| the prop `children` | refused when an element of that prop's type has a body, naming the element: the body is the prop's value and has no name. Without such an element, a rename like any other |
| an attribute that nothing declares (`data-tone`, `aria-label` on an element) | not offered: renamed, it is another attribute |
| an occurrence in a file of the standard library | refused, as one in `node_modules` |
| a `.ts` / `.tsx` file with unsaved changes in the editor | the extension refuses, naming the file: the server reads it from disk, and its edit is for the saved text |

Then a post-check: the edits are applied in memory to each touched `.rtsx`
and the result is transpiled; the rename is refused if the file gains a
syntax error or a transpiler error, or a bare attribute flips between bound
and `true` (the new name captured it).

A refusal is an LSP error (`RequestFailed`) that names the place.
`prepareRename` runs the same rename with the name unchanged, in the
document's project, and refuses where that refuses — every row above that
does not depend on the new name — and where a rename would edit nothing (an
intrinsic tag), or a declaration in `node_modules` (TS's own check lets the
prop of a generic component through). The name it shows is the name as it
is written: `$sub-item`, not the quoted key TS has for it.

```tsx
<tr slot={$Row} &row &&selected />       // .rtsx — `row`, the arg of $Row, renamed to `line`
<tr slot={$Row} &line={row} &&selected />
// `selected`, the arg, renamed: Rename refused at table.rtsx:21:30: `&&selected` names an arg and a prop at once; …
```

TypeScript's own, the same in a `.tsx` file:
- what is no name to it is not offered — a `Switch` or `Match` under any
  name (the tag is lowered away, its import dropped), a string in the type
  that declares it, the quoted key of a binding pattern where it stands;
- the new name is not checked against the names in scope: a binding renamed
  to the name of another one leaves TS2300, or captures its references.
  (The generated test of *Testing* renames to names that are not taken.)

**Rejected:** refusing on any new diagnostic of the program after the edits
— it would cover the collision too, at the price of a second check of the
whole program per rename.

### Commands

`reactogenic/transpiled` (custom request, `{ textDocument: { uri } }` →
`{ text, step }`): the emitted TSX of a document — the virtual text of the
program it is in, the unsaved buffer included — and the tolerance step that
produced it: `lowered`; `stopped: pass 3 (slot hoisting)`, the text of the
last pass that held; `source`, the source standing in. A document that is
not an `.rtsx` one is an error. The server's own methods are answered inside
the fork's server (`Embedder.Requests`), from the document's program.
*Show transpiled TSX* opens it read-only beside the source and
refreshes it on every edit; with a server that does not have the request, it
says the server is too old. The document is named `page.transpiled.rtsx` and
its language is `rtsx`, not TSX: the grammar is a superset, and no server
reports on it. As a `.tsx` document it would get VS Code's TypeScript, with
syntax errors in Problems whenever the text is not TSX — a stopped file's is
the source.

### Not in the first release

| | |
| --- | --- |
| formatting | through the virtual text it re-indents moved slot bodies. Needs its own implementation; Prettier, Biome and ESLint do not parse `.rtsx`, so format-on-save leaves the file untouched |
| rename from a `.ts` file into `.rtsx` | refused there (*The `.ts` side*); start it from the `.rtsx` file |
| renaming a segment | `#about-us` has no virtual token and its import is generated: the `#name` rename is refused; renaming the file — or moving the mounter away from it — leaves `segment-not-found` on the mounter |
| unsaved `.ts` edits | reach `.rtsx` files on save |
| unsaved new files in a project | an `untitled:` document is served (*A document without a file name*) but belongs to no project: relative imports do not resolve and there are no JSX types until it is saved |
| linked editing of slot tag pairs, and of a tag that has slots | plain pairs work; `editor.linkedEditing` is off by default |
| code lens (references, implementations) | not advertised: a lens's range is wrong on a declaration that holds rtsx constructs, and its command is filled in by the TypeScript extension's own client. Off by default in VS Code |
| organize imports next to a generated import | a segment mounter, a container (`slot={$X}`), an exhaustive `Switch`: the generated import sits on the last import's line end, so the edit of that line has no source range. Needs the generated import on a line of its own (a transpiler change); until then the three import actions are not offered there |
| auto-import of a dependency's `.rtsx` exports | a package under `node_modules` that ships `.rtsx` modules imports and checks, but its exports are not offered before the first import: TS's index of dependencies parses their files itself, with its own resolver and host — the mapper would have to be built into that index too |
| shorthand inlay hint | `disabled`⟨`={disabled}`⟩ on every bound bare attribute (syntax.md, *Silent flip*) |
| quick fix for `segment-not-found` | create the file with an empty default component |
| hover docs on `Switch` / `Match` / `$Case` | they are lowered away; static text |
| param completion in `{ }` of a slot without a body | needs params emitted without a body, which changes diagnostics |
| prop completion on an empty position of a rebuilt tag or slot tag | `<$Icon ▮>`: the whitespace there is generated. Completion works once a letter is typed |
| Problems for closed files | open documents only; run the *reactogenic: check* task. A background `--watch` matcher comes later |
| push diagnostics | for clients without pull support |
| a server per workspace folder or CLI version | one server per window |
| extensions keyed on `typescriptreact` | e.g. Tailwind CSS IntelliSense needs `"tailwindCSS.includeLanguages": { "rtsx": "typescriptreact" }`; `[typescriptreact]` settings must be repeated for `[rtsx]` |

**Specifiers the server writes** (auto-import, file rename) are extensionless
for an `.rtsx` module — `./button`, as the convention is — unless a built-in
sibling would win the import: next to `button.ts` or `button.d.ts` the
specifier is `./button.rtsx`, decided on the module's own path (relative,
`paths` and package specifiers alike) and, for a rename, on the files as
they will be: `util.ts` renamed to `util.rtsx` is not its own sibling
(`./util` stays); `button.ts` in a renamed folder still is one. One
exception: in a file that mounts a
segment, auto-import follows that file's existing imports, and the generated
segment import is explicit (`./intro.rtsx`); both forms are valid everywhere.

**In a file whose only import is generated** (a segment mounter or a
container without imports of its own) a new import goes to line 1 — where the
transform puts its own, above a leading comment or directive — whichever way
its module sorts against the generated one. The span map names the places
before the source where a statement can go (the line starts of the generated
text); any other position there is an atom, and an edit in it is dropped.

**A generated import is not one to add to.** A name its module exports —
`Each` or `Slot` in a container, a mounted module's export — gets an import
declaration of its own, in any file: TS would add it to the generated
import, which has no source text.

```tsx
// .rtsx — `Each` accepted from the completion list
export function List({ $Row }: Props) { return <ul><li slot={$Row} />{Eac▮}</ul>; }

// the edit: line 1, a whole declaration — never `, Each`
import { Each } from "@reactogenic/core";
```

**A cursor at the end of a copied expression** (`on={getSta▮}`, an identifier
being typed in a slot body) is that expression's end, though generated text
starts there: the span map's position lookup prefers the copied text, so
completion — auto-import included — works at the normal typing position.

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
  roots, params; what ends an attribute name), the tag-name pattern
  (`$name`), the root scope. It asserts the shape of each rule it patches, so
  an upstream change fails loudly.
- The root scope is `source.tsx.rtsx`: editors find injections (JSDoc,
  styled-components, …) by scope prefix; `source.rtsx` would lose them.
- Plain `.tsx` code tokenizes identically to `source.tsx` — tested on the
  repo's `.tsx` files, and measured once on the upstream grammar's own test
  inputs (plan.md, RGP1-109). Three exceptions on valid TSX, each tested:
  - a tag that starts with `$` (`<$Modal>`) is a slot tag — by design, the
    exception of syntax.md: `entity.name.function.slot.rtsx` on its name,
    where TSX has `support.class.component.tsx`; nothing else differs.
    `<$ns.Comp>` stays a component;
  - an element as an attribute value (`footer=<Match …>`) is tokenized: the
    TSX grammar marks it illegal;
  - an attribute name directly before a spread (`<A x{...p}>`) is a name and
    a spread: the TSX grammar derails on it.
- Params whose `{` ends its line (comments aside) keep TSX's brace scopes,
  `punctuation.section.embedded.begin/end.tsx`, and only what is inside
  carries `meta.slot-params.rtsx`: a regex cannot look at the next line, so
  such a `{` is TSX's `{…}` until the first thing inside that is not a
  comment decides — `...` makes it a spread, anything else params. On one
  line the braces are `punctuation.definition.binding-pattern.object.tsx`
  inside `meta.slot-params.rtsx`.
- No whitespace is needed between two forms, as in the transpiler: a name —
  of an attribute, an arg or a segment root — ends before `{`, `&` and `#`
  (`items{ item }`, `value&size`, `&size{...p}`). So a name typed directly
  in front of existing params, a spread, an arg or a segment root never
  derails the file.
- A sigil without a name yet (`&`, `#` right after typing) must not derail
  the file: the name is optional in the rules, also directly before `{`. A
  sigil followed by something that is not a name (`&a:b`, `#404`, `&&&x`) is
  `invalid.illegal`, up to the tag end or a `{` and not through it.
- ` ```rtsx ` fences in Markdown: the rule VS Code's Markdown grammar has for
  ` ```tsx `, with our language, as an injection — generated from the
  vendored copy of that grammar like everything else.
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

`packages/vscode` — marketplace id `reactogenic.rtsx`. The package is named
`rtsx`, unscoped and private: `vsce` rejects scoped names, and it is never
published to npm.

- **Contributes**: the languages `rtsx` (`.rtsx`) and `rtsx-tags` (inside
  tags: `{/* */}` comments, indent after `<$Slot>`); the grammar, with what
  VS Code's own TSX entry carries (`embeddedLanguages`,
  `unbalancedBracketScopes`, `tokenTypes`, `semanticTokenScopes`);
  breakpoints for `rtsx`; TSX's snippets; Emmet as for TSX; a Markdown fence
  injection (` ```rtsx `); the commands *Restart server* and *Show
  transpiled TSX*; the *reactogenic: check* task and the matchers
  `$reactogenic` and `$reactogenic-ts` (*Diagnostics*);
  the settings `reactogenic.server.path`, `reactogenic.autoClosingTags`
  (*Tags*; on) and `reactogenic.trace.server`.
- **Client**: `vscode-languageclient` over stdio; selector language `rtsx`,
  schemes `file` and `untitled` (not `git:` — the left side of a diff). An
  untitled document is `rtsx` by its language id alone: the server maps it
  (*`reactogenic lsp`*, *A document without a file name*).
- **Which binary runs**, first match:

  | | |
  | --- | --- |
  | 1 | the setting `reactogenic.server.path` |
  | 2 | `$REACTOGENIC_BINARY` |
  | 3 | the workspace's `@reactogenic/cli` — the nearest one, found by walking up from the folder of an `.rtsx` document (below) — if its version is ≥ the extension's minimum (the first version with `lsp`; pre-release tags compared numerically) and its platform package is installed |
  | 4 | the binary bundled in the `.vsix` |

  The workspace's own CLI comes first so the editor and `reactogenic check`
  agree. **Where the walk of row 3 starts** is decided at each start of the
  server, *Restart server* included:

  | The window | The walk starts at |
  | --- | --- |
  | has folders, and an `.rtsx` file inside one of them is open | that document's folder — the active document first, then a visible one, then any open one |
  | has folders, none such is open | the first workspace folder; then the first such document opened decides, restarting the server if the binary differs |
  | has no folder | the folder of the first `.rtsx` file opened |

  A document outside every workspace folder never decides in a window that
  has folders: the binary — which the workspace's *check* task runs too —
  would come from a `node_modules` the workspace's trust does not cover. Once
  a document has decided, opening another does not move the server.

  One server per window. A status item names the binary, its version
  and where it was found, and warns when the workspace CLI was skipped as too
  old — the editor then runs a newer transpiler than that project's `check`.
  A path from row 1 or 2 that does not exist is an error, not a
  fall-through; so is a chosen binary that does not start (one from before
  `lsp`), or that runs and has not answered `initialize` within 10 s — its
  process is killed. A restart never waits behind a start under way: that
  start is given up. When the server has crashed too often for the language
  client to restart it, the status item says that it stopped.

  The server restarts when a lockfile or a `reactogenic.server.*`
  setting changes, and when the workspace becomes trusted. The lockfiles are
  those inside the workspace folders, and — the opened folder may be a
  package of a monorepo — above them: in the directory whose `node_modules`
  holds the CLI, and in the nearest directory at or above the walk's start
  that has a lockfile. On Windows a
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
speaks the mapper protocol on stdin and stdout, and `@reactogenic/cli`
declares it (`typescript.contentMapper` in its `package.json`). A project that
lists it in tsconfig `contentMappers` gets `.rtsx` in plain
`tsc --runExternalCode` and in the TS 7.1 server — for editors without our
server.

```jsonc
// tsconfig.json — `reactogenic check` and `reactogenic lsp` ignore the entry
"contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]
```

The stock host has none of our hooks, so the mapper does in the result what
the fork does around it:

| | The mapper |
| --- | --- |
| extensionless imports | writes the extension into the virtual text, for an import — or the name of a module augmentation — written in an `.rtsx` file that our resolver would resolve to an `.rtsx` file: relative or through `paths`, after every built-in extension, a file before a directory's `index.rtsx` (a directory whose `package.json` names an entry is TypeScript's). A post-pass over the emitted text, its map composed with the passes'; the inserted text is an atom on the specifier. An alias that would no longer match its pattern with the extension on — an exact one, a pattern with text after its `*` — is replaced by the relative path of the file, the whole of it an atom on the specifier |
| module-ness | appends `export {}` to a file without imports or exports: an `.rtsx` file is always a module |
| transpiler errors | sends them as mapper diagnostics, source `reactogenic`: a number from a stable table (below 1000, where TS has none), the name leading the message — `error reactogenic101: orphan-slot: …`. The contract makes them syntax errors of the file: while one stands, `tsc` prints no type error of the program |
| syntax errors | **one mistake, one report.** While the virtual text has a syntax error, TypeScript reports the mistake there and none of the source parse's is sent (the two rarely agree on place or code, so they cannot be matched). The source parse's — under TS's own number, `reactogenic1003` — are sent when the virtual text parses: the passes lowered the broken code away (the children of a segment root) |
| TS5097 on a segment import | never arises: the import generated for a `.tsx` / `.ts` segment is written without its extension, so every other error TS has for that module still shows (TS2306, a file that is not a module). Where a sibling would win the extensionless import — `intro.ts` next to the segment `intro.tsx` — the extension stays, under an ignore directive over that specifier; a directive has no codes, so there it drops whatever TS reports on the specifier |
| a stopped file, a panic | *Tolerance* 4–5: an ignore directive over the whole virtual text; `internal` on the first line. No transform ends the process — the host never starts a mapper twice |

```tsx
// page.rtsx                        // virtual TSX
import { Button } from "./button";  import { Button } from "./button.rtsx";
import { Card } from "@/card";      import { Card } from "@/card.rtsx";
import { Menu } from "@menu";       import { Menu } from "./menu.rtsx";     // "@menu": ["./src/menu"]
<section #intro />                  import _Section_intro from "./intro";   // intro.tsx
```

**Experimental**, with the limits of the contract:

- raw TS messages (no rewrites); a message that names a rewritten specifier
  names what the mapper wrote (`'./button.rtsx'`), and a replaced alias
  answers no hover or definition;
- no `slot-conditional`: it needs the checker, which a mapper runs before. A
  project can pass `tsc --runExternalCode` and fail `reactogenic check`;
- transpiler warnings not shown, and in `tsc` a transpiler error hides the
  project's type errors until fixed;
- in a file with a syntax error, TypeScript reports on the virtual text: at
  the construct that generated the text when the mistake is not in copied
  code, and sometimes more than once (it also parses what a broken file left
  unlowered);
- explicit `.rtsx` in imports written in `.ts` files, and in imports of an
  `.rtsx` file inside a `node_modules` package (`ui-kit/button.rtsx`);
- rename not fixed up;
- the files around a file are the disk's, and a created or deleted segment
  file or import target is seen when the file that names it next changes;
- no `segment-self` for a loop through another file (a segment that mounts
  itself directly is still reported);
- `.rtsx` sources are valid UTF-8: the host re-encodes a file that is not,
  and then rejects the answer (TS18069 on its first line);
- until 7.1 is stable, the "TypeScript 7 Nightly" extension and a trusted
  workspace.

**Not combined with our extension in VS Code**: the TS 7.1 server would also
answer inside `.rtsx` documents — each hover and each TS error twice.
`"js/ts.contentMappers.enabled": false` in the workspace restores one server.

## Other editors

`reactogenic lsp --stdio` is a standard server: Neovim (`vim.lsp`), Zed,
Helix and JetBrains can run it for `*.rtsx` files (scheme `file`). A client
that declares no capability at all is served, and asked nothing;
`--clientProcessId <pid>` names the editor's process where `initialize`
does not (*How the process ends*). Highlighting there needs the deferred
tree-sitter grammar (JetBrains takes the TextMate one).

## Testing

| Layer | Test |
| --- | --- |
| transform | conformance corpus: the span map validates; virtual nodes map to the same source span as `emit.Map`; no source offset has two projections that answer the same feature, except shorthand; the output of a source that parses is TSX, or marked as holding a construct as written; tolerant mode over typing-like mutants of the fixtures never panics and never loses the file |
| server | a Go test client runs the server in-process over a pipe (race-instrumented), one scenario per feature row above on a fixture project, plus the binary itself: a session and its exit statuses. The client behaves as VS Code does: UTF-16, pull diagnostics with refresh, watched-file events where the server registered a watcher, and the capabilities that change answers — hierarchical symbols, line folding, completion items resolved, code actions as literals, edits as document changes; it answers `workspace/configuration` from settings a test supplies, applies the edits it is given, sends `exit` with its pipes still open, and fails the test on a server request it did not answer. The advertised capabilities are compared, key for key, with the feature table; the registrations, with the two kinds of watching. A bare connection drives what a client does wrong: an exit without shutdown, input that is not LSP, documents and positions that should not be sent. Diagnostics: every project of `check`'s goldens that has no syntax error, and the Vite test app — both hosts run on one directory, and each `.rtsx` document's pulled errors and warnings are `check`'s lines for the file (severity, code, message, position, related locations); each row of the table of *Diagnostics*; the tolerance rules as pulled, and over the Vite test app being typed — an attribute at the end of every opening tag, a child under it, character by character: off the typed line a state shows syntax errors, the file's one mistake in another statement, and nothing else; a document's project in nested tsconfigs — one above lists the file, nobody lists it — as `check -p` on the nearest; a refresh request and the changed answer after each kind of change, with no edit to the document — another document edited, opened or closed unsaved, a `.ts` file, a tsconfig, a segment file and the `.tsx` sibling created and deleted, a segment's content changed (`segment-self`). Slots and segments: `<$` in a component with props that are not slots, after a keyed slot's first entry, with every slot written, under a `Switch`, after a slot filled under a `Match` or in the cases of a `Switch`, typed under a `Match` and in a `$Case` (open, with a letter, closed), before a slot element with slots of its own, in a closing tag; hover, definition, references, highlights and `prepareRename` from every tag of a group, each with its own range, and from each part of a member tag's closing tag; references asked from the container — the slot's declaration, the component's — with the page open and closed; `#name` for each extension of the lookup, and the siblings after `#` — a buffer never saved among them. Rename: one case per row of the table — the exact edits, applied, and no diagnostic in the project afterwards; each refusal as an error with its place, and from `prepareRename` where it does not depend on the new name; and **every word** of three fixture projects (`.ts`, `.tsx` and `.rtsx`; strings and texts included; every element and attribute declared, so that a renamed attribute is an error) renamed to a fresh name, applied, the diagnostics of all files pulled, put back: renamed without a diagnostic, or refused by `prepareRename`. One of the three has code that no virtual text holds: after each rename what hides that code is taken away, and there is no diagnostic then either |
| reporting layer | on programs built with the transform tolerant and strict: a stopped file reports no TS diagnostic and its importers are still checked — code left out, and a construct left as written; a file with a syntax error in each mode, and a statement that does not parse next to one that does; a failure of the transpiler; for every file of a project, the per-file form equals the whole-program form (suggestions aside), declaration errors of a project that emits included; a report's range is source text; the merge rule, case by case, and an error in `&&name` through the whole layer |
| `check` | golden output recorded from the overlay model before the migration; reproduced on the mapped program except the listed differences (plan.md, RGP1-106). Project shapes as goldens: `.rtsx` only, Vite's template, a file of two projects, a cross-project import, references in either order, a missing reference, an `include` that names extensions, a project that emits declarations, `paths`, a `contentMappers` entry, a segment under `node16` / `nodenext`, a segment loop, a segment without a file, a syntax error in an `.rtsx` (one the parser reports twice included) and in a `.tsx` file, an unlowered construct, a warning, TypeScript's style checks; `--watch`: an edit, a segment file created and deleted, a referenced project's directory |
| grammar | scope assertions per construct, each also directly before `>` and as a bare sigil; equality with `source.tsx` on plain TSX; no `invalid.*` token in any `.rtsx` of the repo; regenerating changes nothing |
| extension | binary resolution unit tests; an editor suite in an isolated VS Code profile: language id, one slot-term diagnostic, exactly one hover and one definition result, an untitled document, the server's process and its exact command line through restarts, crashes and a binary that never answers; a second, untrusted window: no server process; a third, a package of a monorepo: which CLI runs, and its lockfile above the folder; a fourth, *Show transpiled TSX*: the emitted text beside the source, refreshed on an edit, a file being typed — and, against a stand-in server without the request, "too old"; a rename into a `.ts` file with unsaved changes: refused, and after a revert both files edited. The suite also runs against a packaged `.vsix` and its bundled binary |
| plugin | `tsserver` driven over stdio: no TS2307, references at source positions, rename refused |
| stock mapper | a Go test host speaks the protocol over pipes: the handshake, a slot and a shorthand through a valid map, every import form, a broken file, a construct left as written, a panic, concurrent transforms, the end of input; the corpus through it, and its typing-like mutants — no mistake reported twice or not at all, no panic behind an answer. `scripts/e2e-stock-mapper.sh` runs `typescript@next`'s `tsc --runExternalCode` on a project, each run with exactly its expected errors, and `reactogenic check` on the same tsconfig — by hand: it needs the network |
