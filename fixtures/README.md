# Conformance fixtures

Hand-written cases for the transpiler, run by
`go/internal/conformance` together with the examples extracted from
`specs/phase01/syntax.md` (RGP1-004).

## Two sources

| Source | Checks | Where |
| --- | --- | --- |
| spec examples | output only | every `// .rtsx` code block in syntax.md, paired with the `// .tsx` or `// after pass N` blocks right after it; extracted at test time, never copied |
| fixtures | output and diagnostics | this directory |

Spec examples do not assert diagnostics: their `// Error: …` annotations mix
transpiler errors with type errors that only `reactogenic check` reports. Every
error code therefore gets a fixture here.

## A fixture

A directory that holds an `input.rtsx`:

```
fixtures/<area>/<name>/
├── input.rtsx      the entry file
├── output.tsx      expected output (optional)
├── errors.txt      expected diagnostics (optional; absent = none)
├── output.passN.tsx  expected output after pass N (optional; output only)
├── check.txt       expected `reactogenic check` output (from RGP1-070)
└── *.rtsx, *.tsx   any other files: segments (`+name.rtsx`), containers
```

`errors.txt`, one diagnostic per line, `#` for comments:

```
LINE[:COL] error|warning [CODE] ["message substring"]

3:5 error orphan-slot
7 warning segment-children "overwritten"
```

Line and severity must match; column, code and message are checked when
given. A diagnostic that no line expects fails the case.

## How output is compared

Both sides are parsed as TSX and compared as trees, so a case asserts meaning,
not formatting:

- whitespace, comments and parentheses are ignored;
- JSX text is compared as React sees it (lines trimmed, empty lines dropped);
- `{…}` shown on its own counts as the expression inside it, because the
  spec writes a lone JSX child that way.

## Spec examples

- A case's id is `syntax.md#<heading anchor>/<n>`; the anchor is the one
  GitHub gives the nearest heading.
- `// page.rtsx` makes the entry `page.rtsx`; otherwise it is `input.rtsx`.
- Every `#name` in the input gets a stub `+name.rtsx`, so segment roots
  resolve.
- Spec examples leave out imports. An example that does not import from
  `@reactogenic/core` gets `import { Switch, Match, Each } from
  "@reactogenic/core";` prepended, and its expected output gets what pass 2
  leaves of it: `import { Each } from "@reactogenic/core";`.
- A trailing `// …` after two or more spaces, and a whole-line `// …`, are
  annotations for the reader and are removed before parsing.
- Every expected `.tsx` must parse; `TestSpecOutputsParse` fails on a broken
  example.

## The ratchet

`go/internal/conformance/testdata/passing.txt` lists the cases that pass.

- A listed case that fails is a regression: the test fails.
- A case that passes but is not listed also fails the test, until it is added:

  ```sh
  go test github.com/reactogenic/reactogenic/go/internal/conformance -run TestConformance -update
  ```

- Everything else is not implemented yet; `go test -v` counts it.
