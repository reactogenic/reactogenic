# Getting started with `.rtsx`

`.rtsx` is TSX with a few additions for components that have slots, flow
control and page segments. Phase 1 runs it in an ordinary Vite + React app and
type-checks it with TypeScript 7, reporting every error on the `.rtsx` line you
wrote.

## Install

```sh
pnpm add @reactogenic/core
pnpm add -D @reactogenic/vite @reactogenic/cli
```

`@reactogenic/cli` brings the `reactogenic` binary for your platform (macOS,
Linux and Windows, arm64 and x64).

## Configure Vite

```ts
// vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import reactogenic from "@reactogenic/vite";

export default defineConfig({
  plugins: [reactogenic(), react()],
});
```

`.tsx` and `.rtsx` live side by side and import each other without
extensions (`import { Page } from "./page"` finds `page.rtsx`). Don't keep a
`page.tsx` and a `page.rtsx` next to each other: the import would be ambiguous.

Editing an `.rtsx` file reloads the page; Fast Refresh for `.rtsx` comes with
Reactogenic's own dev server.

## Type-check

Vite does not type-check. `reactogenic check` does, for `.ts`, `.tsx` and
`.rtsx`:

```json
{
  "scripts": {
    "check": "reactogenic check",
    "build": "reactogenic check && vite build"
  }
}
```

```
src/page.rtsx:4:26 - error TS2322: Type 'number' is not assignable to type 'boolean | undefined'.

4     <$Field name="email" required={1} />
                           ^
```

Errors about slots and segments are reported in their terms —
``undeclared-slot: `$Nope` is not declared in `Card` ``, ``slot-args-missing:
`$Icon` needs `&size` ``. Use `reactogenic check --watch` while you work, and
`--pretty=false` for `file(line,col)` output in CI.

## The syntax in five minutes

**Shorthand props.** A bare attribute passes the variable of the same name
when there is one in scope; otherwise it means `true`, as in TSX.

```tsx
const value = "hello";
<Input value />        // value={value}
<button disabled />    // disabled={true}: no `disabled` in scope
```

**Slots.** A component declares slots as `$`-props; the caller fills them with
`$` elements; the component attaches each one to the element it stands for.

```tsx
// Button.rtsx
import type { FnSlot, Slot } from "@reactogenic/core";

interface ButtonProps {
  $Label?: Slot<{ className?: string; children?: ReactNode }>;
  $Icon?: FnSlot<ComponentProps<"span">, { size: Size }>;
  size: Size;
}

function Button({ $Label, $Icon, size }: ButtonProps) {
  return (
    <button>
      <span slot={$Icon} &size />
      <b slot={$Label} className="label">Button</b>
    </button>
  );
}
```

```tsx
// the caller
<Button size="lg">
  <$Icon { size }><Plus size /></$Icon>
  <$Label>Save</$Label>
</Button>
```

- A slot's props replace the attachment's, prop by prop; the attachment's
  children are the fallback (`Button` above when `$Label` is not given).
- `&size` hands `size` to a function slot's body, which receives it as
  `{ size }`; `&&value={x}` also sets `value` on the element.
- A slot is one value: writing `<$Label>` twice keeps the last one. To render
  something once per item, attach the slot inside an `Each` in the component.

**Flow control.**

```tsx
import { Each, Match, Switch } from "@reactogenic/core";

<Match on={user}>
  <Avatar src={user.avatarUrl} />
</Match>

<Switch on={query.status} exhaustive>
  <$Case is="pending"><Spinner /></$Case>
  <$Case is="error"><Oops /></$Case>
  <$Case is="success"><List /></$Case>
</Switch>

<Each items={users} { item: user }>
  <li key={user.id}>{user.name}</li>
</Each>
```

`exhaustive` makes a missing case a type error: `Missing "success"`.

**Segments.** Split a long page into files next to it, and mount them by name:

```tsx
<section #pricing />   // mounts the default export of ./+pricing.rtsx
```

## Moving a `.tsx` file to `.rtsx`

Renaming is the whole migration, with one thing to look for: a bare boolean
attribute that has a variable of the same name in scope changes meaning.

```tsx
const disabled = false;
<Button disabled />    // TSX: disabled={true}. .rtsx: disabled={disabled}
```

## Reference

- [specs/phase01/syntax.md](../specs/phase01/syntax.md) — every extension, with
  its desugaring and errors.
- [specs/phase01/vite.md](../specs/phase01/vite.md) — the Vite plugin.
- [specs/phase01/diagnostics.md](../specs/phase01/diagnostics.md) —
  `reactogenic check` and how errors are mapped back.
