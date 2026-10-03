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
| `0003-rtsx-program.patch` | adds `rtsx/program.go`: the program, file-system overlay and checker bridge (RGP1-050/051) |
| `0004-rtsx-mapper.patch` | a content mapper built into the binary (`contentmapper/builtin.go`, `rtsx/mapper.go`, `rtsx/spanmap.go`): tsconfig parsing, the session and inferred projects use it without `runExternalCode`; its file system, per-file identity and `Extra`; a position at the end of verbatim text maps back exactly (specs/phase01/ide.md, *The engine*; RGP1-102/103/105) |
| `0005-rtsx-resolver.patch` | an extensionless import finds a content-mapped file, after every built-in extension |
| `0006-rtsx-lsp.patch` | `lsp.Embedder` and `rtsx/server`: the language server for a host with a built-in mapper — static capabilities (the configuration watcher only for a client that declares it, not awaited), no formatting, code lens or upstream-extension requests, its own server info, the parent-process hook (RGP1-103); `ls/syntactic.go`: folding, selection ranges and closing tags on a source tree that is in no program; `ApplyChange`: a ranged document change as the server applies it (RGP1-105) |
| `0007-rtsx-specifiers.patch` | the minimal module-specifier ending drops a content-mapped extension (`./button`), as the resolver finds the file without it |

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
