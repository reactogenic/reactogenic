# Patches to tsgo

`go/third_party/tsgo` is a vendored copy of the `tsc/` Go module of
[microsoft/TypeScript](https://github.com/microsoft/TypeScript), at the commit
recorded in `go/third_party/tsgo/UPSTREAM`, with these patches applied in
order. The patched tree is committed; the patches are the source of truth for
every change we make to it.

| Patch | Why |
| --- | --- |
| `0001-rtsx-bridge.patch` | adds `rtsx/`, the public bridge our module imports tsgo's `internal/` packages through (decisions.md, RGP1-003) |

## Changing tsgo

1. Edit files under `go/third_party/tsgo`.
2. Write the change as a new patch, paths relative to the vendored module:

   ```sh
   git diff --relative=go/third_party/tsgo -- go/third_party/tsgo > go/patches/000N-short-name.patch
   ```

   For a patch that adds one whole file, such as the bridge, regenerate it
   from the file instead:

   ```sh
   go/scripts/file-patch.sh rtsx/rtsx.go 0001-rtsx-bridge.patch
   ```
3. Check that the patches still reproduce the committed tree:

   ```sh
   go/scripts/sync-tsgo.sh && git status --short go/third_party/tsgo   # must print nothing
   ```

Prefer new files over edits to upstream files; keep edits to upstream files
to the few lines that call into them.

## Moving to a newer upstream commit

```sh
go/scripts/sync-tsgo.sh <commit>
```

A patch that no longer applies stops the script; fix it against the new tree
and regenerate it as in step 2.

Tests (`*_test.go`), `testdata/` and test-only packages are not vendored
(`go/scripts/tsgo-exclude.txt`): upstream's CI runs them. Our changes are
covered by our own tests.
