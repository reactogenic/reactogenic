# Tooling (language service)

Not part of phase 1. Parked here from [syntax.md](../phase01/syntax.md).

## Shorthand props

> OPEN: mitigate the silent flip with an editor inlay hint (`disabled`⟨`={disabled}`⟩)
> on every case-A attribute? No effect on the language, only on tooling.

## Slots

The language service answers from the same props-type query the transpiler
uses for list slots:

- Typing `$` (or `<`) as a child of `<Button>` completes the slots `Button`
  declares. Slots that are already filled are left out; required ones sort
  first.
- Inside a slot tag: completion and checking of the slot's options.
- Inside the params braces: completion of the names the container hands out.
- Hover on a slot tag shows the slot's declared type; go-to-definition jumps
  to the `$X` member of the container's props.

## Segment roots

- After `#`: completion of sibling `+` files not yet mounted on the page.
- Cmd-click (go-to-definition) on `#counter` opens `+counter.rtsx`, or
  `+counter.tsx`. Renaming either side renames the other.
- Quick fix for segment-not-found: create the file with an empty default component.
