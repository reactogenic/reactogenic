# Changelog

The npm packages (`@reactogenic/core`, `@reactogenic/vite`, `@reactogenic/cli`
and its six platform packages) share one version. The VS Code extension has
its own: [packages/vscode/CHANGELOG.md](packages/vscode/CHANGELOG.md).

## 0.1.0-alpha.1 — unreleased

IDE support (specs/phase01/ide.md).

**New**

- `reactogenic lsp --stdio` — a language server for `.rtsx`: TypeScript 7's,
  with the transform built in. Diagnostics are those of `reactogenic check`.
- The VS Code extension `reactogenic.rtsx`: highlighting, the server, and
  `.rtsx` modules in VS Code's own TypeScript.
- `reactogenic content-mapper`, declared by `@reactogenic/cli`: `.rtsx` in
  stock TypeScript 7.1 (`contentMappers` in tsconfig). Experimental.
- `reactogenic --version`.

**`reactogenic check`** now type-checks `.rtsx` files under their own names
(before: a `.rtsx.tsx` copy beside each). What that changes in its output:

- Each mistake is reported once: an error in a mounted segment, or in a
  module imported as `./b.rtsx`, is no longer printed a second time, and the
  two TS2339 of one attachment are one.
- Related locations name the `.rtsx` file (`props.rtsx:1:32`, not
  `props.rtsx.tsx:1:32`).
- A file with a syntax error no longer turns into an empty module: its
  importers are checked, without a TS2306 each.
- A file from which the transpiler had to leave code out (an orphaned slot,
  `arg-without-slot`, `params-on-html`) reports the transpiler's error and
  no TypeScript error until it is fixed.
- `segment-not-found` comes without TypeScript's TS2307 on the same `#name`.
- Project references: referenced projects are checked first, each file is
  reported once, by the project that lists it and with that project's
  options. A missing reference is one line (TS6053).
- `include` keeps TypeScript's meaning: a pattern that names extensions
  (`src/**/*.tsx`) lists no `.rtsx` file — write `src/**/*` or add
  `src/**/*.rtsx`. Such a file is still checked when an import reaches it.
- Declaration errors (TS4094, …) are reported for projects that emit
  declarations.
- `Foo.tsx` beside `Foo.rtsx` is still `ambiguous-module`, and the `.rtsx`
  is now checked too.
- A segment under `moduleResolution: node16` / `nodenext` resolves (before:
  TS2307).
- `--watch` follows referenced projects, and `.jsx` / `.js` files.

**Fixed**

- `&&name={expr}`: a type error in `expr` was dropped (`check` exited 0).
- `<section#intro />` and `<section hidden#intro />` — no space before `#` —
  emitted invalid TSX.
- `&#name` was accepted as an arg; it is a syntax error.
- An arg of the wrong type was reported as `slot-no-args` ("takes no args");
  it is TypeScript's TS2322.

## 0.1.0-alpha.0 — 2026-09-28

The first release: `.rtsx` in Vite (`@reactogenic/vite`), the runtime
(`@reactogenic/core`), `reactogenic check` (`@reactogenic/cli`).
