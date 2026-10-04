# Patches to tsgo

`go/third_party/tsgo` is a vendored copy of the `tsc/` Go module of
[microsoft/TypeScript](https://github.com/microsoft/TypeScript), at the commit
recorded in `go/third_party/tsgo/UPSTREAM`, with these patches applied in
order. The patched tree is committed; the patches are the source of truth for
every change we make to it.

| Patch | Why |
| --- | --- |
| `0001-rtsx-bridge.patch` | adds `rtsx/`, the public bridge our module imports tsgo's `internal/` packages through (decisions.md, RGP1-003) |
| `0002-rtsx-parser.patch` | slot params `{ size }` in JSX attribute position, under the parse option `RTSX` (decisions.md, RGP1-020) |
| `0003-rtsx-program.patch` | adds `rtsx/program.go`: the program — a referenced project's modules read from source, as in the server — and the checker bridge (RGP1-050/051); a tsconfig read before its program is built: the files it lists, the projects it references; the program's diagnostics in `tsc`'s steps, without the syntax errors of a mapped file's virtual text and with the declaration errors of any project that emits them, and one file's (RGP1-106); `PropsAt`: the props that the component of a tag, or the value of an attribute or property, takes — the slots of an owner (RGP1-108) |
| `0004-rtsx-mapper.patch` | a content mapper built into the binary (`contentmapper/builtin.go`, `rtsx/mapper.go`, `rtsx/spanmap.go`): tsconfig parsing, the session and inferred projects use it without `runExternalCode`; its file system, per-file identity and `Extra`; the registered mapper is one value (a project compares mappers by identity); a position at the end of verbatim text maps back exactly, and, in generated text that precedes the whole source, one that the mapper names (`MapperResult.StatementStarts`: where a statement can be inserted) is the source's start (specs/phase01/ide.md, *The engine*; RGP1-102/103/105); in the session (`project/mapped.go`): diagnostics refreshed when a mapped document is opened or closed with a text that is not the file's on disk, and, among a tsconfig and the projects it references, a mapped file's default project is the one that lists it before one that only imports it (ide.md, *Diagnostics*; RGP1-107) |
| `0005-rtsx-resolver.patch` | an extensionless import finds a content-mapped file, after every built-in extension |
| `0006-rtsx-lsp.patch` | `lsp.Embedder` and `rtsx/server`: the language server for a host with a built-in mapper — static capabilities (the configuration watcher only for a client that declares it, not awaited), no formatting, code lens or upstream-extension requests, its own server info, the parent-process hook (RGP1-103); `ls/syntactic.go`: document symbols, folding, selection ranges and closing tags on a source tree that is in no program; `ApplyChange`: a ranged document change as the server applies it; `Embedder.Owns`: workspace symbols (`ls/symbols.go`) and file-rename edits (`ls/file_rename.go`) narrowed to the host's files inside the server, and `.rtsx` and folders in the rename filters; a file rename's specifiers written for the files as they will be, a generated import left out of it; a generated import is no existing import to an import fix (`ls/autoimport/fix.go`); an import inserted at the top of a text that is only comments (`ls/change/tracker.go`: upstream indexes past the text — for a mapped file on every completion request); inlay hints once each and none on a generated call (`ls/inlay_hints.go`); no organize-imports action with nothing in it (`ls/codeactions.go`); definition on a non-relative specifier of a mapped module (`ls/definition.go`) (RGP1-105); `Embedder.Diagnostics` and `ls/host_diagnostics.go`: a mapped document's diagnostics are the host's — it is given the program and the file, and the server makes the LSP diagnostics of what it returns: a name or TS's number as the code, TS's tags and severity, a one-character range for a span of no length, related locations (RGP1-107); `Embedder.Requests`: the host's own methods, each given the program and the file of the document it names (`reactogenic/transpiled`); `Embedder.Rename` and `ls/host_rename.go`: a rename's occurrences in content-mapped files are handed to the host as the language service found them — the file, the virtual range, whether it lies in one verbatim span, the new text, gathered across projects — and the host returns the edits of those files or refuses the rename; `prepareRename` asks the same, the name unchanged; a rename that would edit nothing is not offered (an intrinsic tag), nor one with an occurrence in `node_modules` (upstream's check lets the prop of a generic component through, and edits its declaration) or in the library; what upstream renames in part is refused, in any file — a string, the prop that a body is the value of (`checker/rtsx.go`: its name) when an element has a body, a tag across component and intrinsic — an attribute that nothing declares is not offered, and the quoted key of a binding pattern is an occurrence; `Syntactic`: the tree, and offsets to and from LSP positions (specs/phase01/ide.md, *Rename*, *Commands*; RGP1-108) |
| `0007-rtsx-specifiers.patch` | the minimal module-specifier ending drops a content-mapped extension (`./button`), as the resolver finds the file without it — unless a built-in sibling of the module (looked up by its absolute path) would win the import |

## Changing tsgo

1. Edit files under `go/third_party/tsgo`.
2. Write the patch from the working tree — offline; it never re-vendors:

   ```sh
   go/scripts/regen-patch.sh 0002-rtsx-parser.patch                      # its files changed
   go/scripts/regen-patch.sh 0002-rtsx-parser.patch internal/ast/x.go    # it gains a file
   go/scripts/regen-patch.sh 0004-resolver.patch internal/module/resolver.go   # a new patch
   ```

   The script diffs the files against pristine upstream: empty for a file
   that is not in `UPSTREAM.sha256` (the manifest `sync-tsgo.sh` writes — the
   file is ours), else `HEAD`'s copy minus the committed patch, which must
   hash as the manifest says. It refuses a path that exists nowhere and a
   file that does not differ from upstream, and checks that pristine plus the
   new patch is the working tree.
3. `go/scripts/check-patches.sh` (run by the script, and by CI) checks the
   whole tree offline: every file is upstream's, byte for byte, or listed in
   exactly one patch. A new fork file that no patch lists fails here — the
   next re-vendor would delete it.
4. Commit the tree and the patch together. CI's `vendor` job re-vendors and
   compares (`pnpm vendor:check`, which fetches upstream).

**One patch owns a given upstream file.** A change to a file that an existing
patch touches is folded into that patch; the script refuses otherwise. (Two
patches on one file cannot be regenerated independently.)

Prefer new files over edits to upstream files; keep edits to upstream files
to the few lines that call into them.

## Moving to a newer upstream commit

```sh
go/scripts/sync-tsgo.sh <commit>
```

A patch that no longer applies stops the script; fix it against the new tree
and regenerate it as in step 2, after committing the re-vendored tree.

Tests (`*_test.go`), `testdata/` and test-only packages are not vendored
(`go/scripts/tsgo-exclude.txt`): upstream's CI runs them. Our changes are
covered by our own tests.

## Known upstream defects

Defects of the pinned upstream commit that our tests meet and do not patch.
RGP1-114 (re-vendor) re-checks each: when one is gone, its tolerance goes.

| Defect | Seen | Tolerated by |
| --- | --- | --- |
| `textDocument/signatureHelp` panics — `Debug failure. False expression: Not a subspan. Child: KindLessThanEqualsToken, parent: KindJsxSelfClosingElement` — right after a `<` typed behind a JSX attribute name (`<Action variant<="solid">`; `<` is a signature-help trigger character). The request is answered with an InternalError; the server goes on | the same in a plain `.tsx` file: not ours | `TestTypingNeverFails` (exactly this message, on signature help only); pinned by `TestUpstreamSignatureHelpPanic` (`go/internal/lsp/features_test.go`) |
