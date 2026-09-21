# `.rtsx` syntax

`.rtsx` is a superset of `.tsx`. This document lists everything `.rtsx` adds.
Each extension is shown as a desugaring `.rtsx` → `.tsx`; the emitted `.tsx` is
what TS7 type-checks.

Governing rule: new meaning goes only to forms that are syntax errors in
today's TSX. A syntax error today is good news: no existing program uses the
form, so it is safe to reserve. Forms that already parse keep their React
meaning; exceptions are called out explicitly in the section that makes them.

| Reserved form (syntax error today) | Used by |
| --- | --- |
| `{ a, b }` / `{}` in attribute position | slot params |
| `#name` in attribute position | segment roots |

| Extension | Status |
| --- | --- |
| [Shorthand props](#shorthand-props) | Draft |
| [Slots](#slots) — slot elements + slot params | Draft |
| [Flow control](#flow-control-switch-and-match) — `Switch`, `Match`, `$Case` | Draft |
| [Segment roots](#segment-roots) — `<section #about-us />` | Draft |
| [Iteration](#iteration-each) — `Each` | Draft |
| Rules of layout — not syntax; see [layout.md](layout.md) | Draft |

## Shorthand props

### Motivation

`value={value}`, `user={user}`, `onChange={onChange}` is the most common
repetition in JSX. JS already solved it for objects (`{ value }`); `.rtsx`
does the same for attributes. Rule of thumb: **bare name = pass my value in.**

### Grammar

No grammar change. A bare attribute (`JsxAttribute` without initializer)
already parses. Only the desugaring differs.

> **Exception to the governing rule.** Case A below changes the meaning of a
> form that parses today (`value` → `true`). Accepted because it is the only
> way to get the shorthand without new punctuation.

### Desugaring

The rule is **scope-directed**, not type-directed: the compiler needs only the
file's binder, never the checker.

**A. Same-named value binding in scope** → pass it.

```tsx
// .rtsx
const value = useDraft();
<Input value />
```

```tsx
// .tsx
const value = useDraft();
<Input value={value} />
```

**B. No such binding** → unchanged, React meaning (`true`).

```tsx
// .rtsx
<Input disabled />
```

```tsx
// .tsx
<Input disabled />
```

Mixed:

```tsx
// .rtsx
function Field({ value, onChange }: FieldProps) {
  return <Input value onChange required />;
}
```

```tsx
// .tsx
function Field({ value, onChange }: FieldProps) {
  return <Input value={value} onChange={onChange} required />;
}
```

### What "in scope" means

Scope is the **ES module's scope chain** at the attribute's position: the
module scope (imports and top-level declarations) plus the nested function and
block scopes enclosing the element. Nothing outside the module is ever
consulted.

| Counts | Does not count |
| --- | --- |
| `const` / `let` / `var` | the global object: `window.name`, `window.open`, `status`, `top`, `event`, … |
| parameters, destructured names | ambient declarations: `lib.d.ts`, `declare global`, `declare const` |
| function and class declarations | type-only bindings: `interface`, `type`, `import type` |
| value imports | |
| slot params (see [Slots](#slots)) | |

So `<Input name />` is always case B unless the module itself declares `name`.

Resolution is ordinary lexical lookup — the nearest binding wins. Since the
emitted identifier is resolved by TS the same way, shadowing needs no special
handling.

### Typing behaviour

None of its own. After desugaring, TS7 checks `value={value}` or `value`
(= `true`) against the prop type as usual.

- `boolean | X` props need no special rule: scope alone picks A or B.
- Case B on a non-boolean prop is an ordinary TS error
  (`Type 'true' is not assignable to type 'string'`), mapped back to the bare
  attribute.

> OPEN: for that error the language service could append "no `value` in scope"
> as related information. Cheap, and it explains the most likely cause (typo or
> deleted variable).

### Compile errors

None. Every bare attribute desugars to A or B.

### Edge cases

- **Forcing `true`** when a same-named binding exists: write `disabled={true}`.
  Explicit initializers are never rewritten.
- **Not an identifier** → always B: `aria-label`, `data-id`, `xlink:href`.
- **Reserved words** → always B, since they can never be bindings: `default`,
  `class`, `for`. This is what keeps `<$Case default>` unambiguous.
- **TDZ.** `<Input value />` above `const value = …` in the same block is
  case A; TS reports the use-before-assignment.
- **Intrinsic elements** follow the same rule: with `disabled` in scope,
  `<button disabled />` → `<button disabled={disabled} />`.
- **Slot elements** follow the same rule for their own attributes.
- **Spread** is unaffected: `<Input {...props} value />` → `<Input {...props} value={value} />`,
  order preserved.
- **Silent flip.** Adding or removing a binding named like a bare boolean
  attribute changes that attribute's meaning with no error when the types
  happen to agree (`const disabled = false; … <Button disabled />`).

> OPEN: mitigate the silent flip with an editor inlay hint (`disabled`⟨`={disabled}`⟩)
> on every case-A attribute? No effect on the language, only on tooling.

> OPEN: pasting existing TSX into `.rtsx` can hit the silent flip. Should the
> `.tsx` → `.rtsx` migration path warn on bare attributes that resolve as case A?

### Prior art

| | Form | Note |
| --- | --- | --- |
| JS | `{ value }` | object property shorthand; the model for this rule |
| Svelte 5 | `<Input {value} />` | purely syntactic: always expands to `value={value}`, no scope lookup; an undeclared name is an ordinary unresolved identifier. Bare `disabled` is always `true` |
| Svelte 5 | `<input bind:value />` | ≡ `bind:value={value}`. A bare name with no braces, closest to `.rtsx`; also expands unconditionally |
| Astro | `<Input {value} />` | same as Svelte |
| Vue 3.4+ | `<Input :value />` | same-name shorthand on `v-bind`; bare `value` stays `true` |
| Solid | — | plain JSX, no shorthand |

All four keep bare `attr` = `true` and spend punctuation on the shorthand, so
each form has one meaning and no scope lookup is needed. `.rtsx` spends no
punctuation: one form, two meanings, scope decides. The scope rule and the
silent flip are the price.

**Rejected:** Svelte's `{value}` spelling. Bare braces in attribute position
mean the opposite direction — see [Slots](#slots): bare name = pass my value
in; braces = give me your value out.

## Slots

### Motivation

A container has named places (icon, title, cell). In plain React each place is
a prop; once a place needs options *and* values from the container, the call
site turns inside-out:

```tsx
<Button size="lg" $IconStart={{ spacing: "tight", children: ({ size }) => <Icon name="plus" size={size} /> }}>
```

A slot element writes the same thing top-down, as markup. There is no slot
component: a slot is **data plus a render function**, and the container owns
all markup around it.

`$` is reserved for slots ("$ stands for $lot, not $ystem"). Rule of thumb:
**bare name = pass my value in; braces = give me your value out.**

Terms, for `<$IconStart spacing="tight" { size }>…</$IconStart>`:

| Term | Part | Direction |
| --- | --- | --- |
| slot **options** | `spacing="tight"` | user → container |
| slot **params** | `{ size }` | container → user |
| slot **body** | `…` | |

The container side passes **args** (`renderSlot(children, { size })`); the user
side declares **params** — same split as any JS function. "Binding" is kept
for two-way binding; "slot props" is avoided because in React it would read as
the options.

### Declaration (plain TSX, no extension)

A slot is a `$`-named prop whose type is an object: the slot's own options,
plus `children`. The framework package provides:

```tsx
type SlotFn<Params> = (params: Params) => ReactNode;          // params required
type OptionalSlotFn<Params> = ReactNode | SlotFn<Params>;  // params optional

function renderSlot<Params>(children: OptionalSlotFn<Params>, args: Params): ReactNode {
  return typeof children === "function" ? children(args) : children;
}
```

```tsx
interface ButtonProps {
  children: ReactNode;
  size: ButtonSize;
  $IconStart: {
    spacing: IconSpacing;
    children: OptionalSlotFn<{ size: ButtonSize }>;
  };
}

function Button({ children, size, $IconStart, ...props }: ButtonProps) {
  const iconClassName = getIconClassName($IconStart, size);
  return (
    <button {...props}>
      <span className={iconClassName}>
        {renderSlot($IconStart.children, { size })}
      </span>
      {children}
    </button>
  );
}
```

### Declaration matrix

Three independent axes, all plain TS. The compiler reads them from the
parent's props type (see *Typing behaviour*).

**1. Is the slot required?**

| Declaration | `<Button>` without `<$IconStart>` |
| --- | --- |
| `$IconStart: {…}` | error: `Button` requires `$IconStart` |
| `$IconStart?: {…}` | fine; container reads `$IconStart?.…` |

**2. What may the body be?**

| `children` type | body only | params + body |
| --- | --- | --- |
| `ReactNode` — container hands nothing out | ✓ | error: `$IconStart` provides no values |
| `OptionalSlotFn<Params>` | ✓ | ✓ |
| `SlotFn<Params>` — container needs a function (lazy or per-item calls) | error: `$IconStart` requires params | ✓ |

A user who needs none of the values of a `SlotFn` slot writes empty
params: `<$Row {}>…</$Row>` → `children: () => …`.

**3. Is the body required?**

| Declaration | `<$IconStart spacing="tight" />` |
| --- | --- |
| `children: …` | error: `$IconStart` requires content |
| `children?: …` | fine; `renderSlot(undefined, args)` renders nothing |

The emitted value is **always an object**, even for a slot with no options:
the simplest slot is `$Title: { children: ReactNode }`, never `$Title: ReactNode`.
Otherwise the shape would depend on whether the user happened to pass options.

Fallback content for an optional slot is the container's business:
`renderSlot($IconStart?.children, { size }) ?? <DefaultIcon />`.

### Usage and desugaring

```tsx
// .rtsx
<Button size="lg">
  <$IconStart spacing="tight" { size }>
    <Icon name="plus" size />
  </$IconStart>
  Add
</Button>
```

```tsx
// .tsx
<Button size="lg"
  $IconStart={{
    spacing: "tight",
    children: ({ size }) => <Icon name="plus" size={size} />,
  }}>
  Add
</Button>
```

Note `<Icon size />`: the params put `size` in scope, so *Shorthand props*
case A applies.

Params are optional. Without them the body is passed as plain content:

```tsx
// .rtsx
<Button size="md">
  <$IconStart spacing="tight">
    <Icon name="plus" />
  </$IconStart>
  Add
</Button>
```

```tsx
// .tsx
<Button size="md"
  $IconStart={{
    spacing: "tight",
    children: <Icon name="plus" />,
  }}>
  Add
</Button>
```

For each component element `<P>`:

1. Every **immediate child** element whose tag starts with `$` is a slot element.
2. It is removed from the children and appended to `<P>` as the attribute
   `$Name={{ … }}`, after all written attributes, in source order.
3. Slot attributes become object properties, order preserved:

   | Slot attribute | Property |
   | --- | --- |
   | `spacing="tight"` | `spacing: "tight"` |
   | `spacing={x}` | `spacing: x` |
   | `spacing` | `spacing: spacing` or `spacing: true` (*Shorthand props*) |
   | `aria-label="x"` | `"aria-label": "x"` |
   | `{...rest}` | `...rest` |

4. The slot body becomes the `children` property. The choice is purely
   syntactic — the compiler never looks at the declared type:

   | Slot element | Property |
   | --- | --- |
   | params + body | `children: (params) => body` |
   | body only | `children: body` |
   | no body (`<$X … />`) | no `children` property |

   `body` is the single child expression, or `<>…</>` when there are several
   children or text.
5. Whatever is left in `<P>` stays as `children`.

### Params on a component: `children` is the default slot

The rule is one and the same everywhere: **wherever the compiler sees params,
it wraps the body in a callback.** On a slot element the callback is the
slot's `children`; on a component element it is the component's `children`.

```tsx
// .rtsx
<Each items { item, index }>
  <p key={item}>{item} is {index + 1}</p>
</Each>
```

```tsx
// .tsx
<Each items={items}>
  {({ item, index }) => (
    <p key={item}>{item} is {index + 1}</p>
  )}
</Each>
```

The component is an ordinary runtime component that declares
`children: SlotFn<…>` (or `OptionalSlotFn<…>`) and calls it; the whole
*Declaration matrix* applies to `children` as to any slot. Nothing is compiled
away and no wrapper is added.

With slot elements present, they are hoisted to attributes first and the
callback wraps what is left. The params are therefore in scope in the
component's children only — not in its slot elements, and not in its own
attributes.

### One slot, many items

A slot is filled **once**. When a container renders many items, the data goes
in as a prop and the slot is the per-item template; the container calls it
once per item and hands the item out through the params.

```tsx
interface SelectProps {
  options: Option[];
  $Option: { children: SlotFn<Option> };
}

function Select({ options, $Option }: SelectProps) {
  return (
    <ul>
      {options.map((option) => (
        <li key={option.value}>{$Option.children(option)}</li>
      ))}
    </ul>
  );
}
```

```tsx
// .rtsx
<Select options>
  <$Option { label, value }>
    <Flag code={value} />
    {label}
  </$Option>
</Select>
```

```tsx
// .tsx
<Select options={options}
  $Option={{
    children: ({ label, value }) => <><Flag code={value} />{label}</>,
  }} />
```

Keys, item markup and iteration stay inside the container; the call site has
no loop. Per-item variation is written inside the body.

**Deferred:** repeatable slots (`$Option: {…}[]`, several `<$Option>` elements,
`{xs.map(() => <$Option …>)}`). Filling a slot twice is duplicate-slot.
Phase 2 brings them back through `Each` — see the roadmap note in
[Iteration](#iteration-each).

### Grammar

Two parts.

**Slot elements — no grammar change.** `<$IconStart>` parses today as a
component reference. `.rtsx` reserves it: a `$`-tag is never resolved as an
identifier, so nothing needs importing and two containers can both have a
`$Title` without colliding.

> **Exception to the governing rule.** A `$`-tag parses today with React
> meaning. `.rtsx` claims the whole `$` tag namespace.

**Slot params — the first real grammar change.**

```
JsxAttributes  ::= … | JsxSlotParams
JsxSlotParams ::= ObjectBindingPattern        // `{` not followed by `...`
```

In today's TSX, `{` in attribute position must be followed by `...`; anything
else is a syntax error. So:

| Form | Meaning |
| --- | --- |
| `{...rest}` | spread attribute, as today |
| `{ size }`, `{ size: s }`, `{ size = "md" }`, `{ row: { id } }` | params |
| `{}` | empty params: `() => body` |
| `{ size, ...rest }` | params with rest (starts with a name, so unambiguous) |
| `{ ...rest }` alone | spread attribute, never params |

Slot params are a normal JS destructuring pattern: renaming, defaults, nesting
and rest all work and are copied to the emitted parameter unchanged. Renaming
is the way out of a name collision with the outer scope:

```tsx
// .rtsx
const label = "Country";
<Select options>
  <$Option { label: optionLabel, value: optionValue }>
    {label}: {optionLabel} ({optionValue})
  </$Option>
</Select>
```

```tsx
// .tsx
const label = "Country";
<Select options={options}
  $Option={{
    children: ({ label: optionLabel, value: optionValue }) =>
      <>{label}: {optionLabel} ({optionValue})</>,
  }} />
```

No type annotation inside the pattern; types come from the declaration.

### Typing behaviour

**The compiler is type-aware.** It embeds the checker (tsgo fork) and, before
desugaring a container element, resolves the props type of its tag. That query
depends only on the container's declaration, never on the call site being
desugared, so there is no cycle. Types never change the emitted code — the
desugaring stays purely syntactic. They are used for:

- **Diagnostics** — every error below is reported on the `.rtsx` source in
  slot terms, not as an assignability error on emitted code.
- **Language service** — see *Tooling*.

The emitted `.tsx` is still fully checked by TS7, which is what types the rest:

- Param names are contextually typed from `SlotFn<Params>`. `{ colour }` on a
  slot that only hands out `size` → error.
- Slot attributes are checked as a fresh object literal: unknown option →
  excess-property error; missing required option → missing-property error.
- Optional slot: `$IconStart?: {…}`. The container then writes
  `renderSlot($IconStart?.children, { size })`.

> OPEN: a helper such as `Slot<Children, Options = {}>` to shorten
> declarations — belongs in `slot-contract.md`.

### Compile errors

| Code | Message | Condition | Needs |
| --- | --- | --- | --- |
| undeclared-slot | `$X` is not declared in `P` | parent has no `$X` prop | types |
| missing-slot | `P` requires `$X` | required slot not filled | types |
| params-required | `$X` requires params | body only, `children: SlotFn` | types |
| no-values | `$X` provides no values | params, `children: ReactNode` | types |
| content-required | `$X` requires content | `<$X … />`, `children` not optional | types |
| orphan-slot | Slot must be immediate child of the component | parent is an intrinsic element, fragment, expression or the file root | syntax |
| duplicate-slot | `$X` is already filled | same slot element twice in one parent, or slot element plus explicit `$X={…}` attribute | syntax |
| params-on-html | Params are only allowed on components and slot elements | `<div { size }>` | syntax |
| duplicate-params | A slot takes one params pattern | `<$X { a } { b }>` | syntax |
| slot-children-conflict | | `children=` attribute on a slot element that also has a body | syntax |
| slot-key | Slots are not elements | `key` on a slot element | syntax |

```tsx
<Button>
  <$TableCell><Icon /></$TableCell>  // Error: $TableCell is not declared in Button
  Click me
</Button>

<Button>
  <div>
    <$IconStart><Icon /></$IconStart>  // Error: Slot must be immediate child of the component
  </div>
  Click me
</Button>
```

### Edge cases

- **Params scope** covers the slot body only, not the slot element's own
  attributes: in `<$X spacing { spacing }>` the attribute `spacing` resolves
  in the outer scope.
- **Shadowing** — a param shadows outer names inside the body, like any
  parameter. To keep the outer name reachable, rename the param:
  `{ label: optionLabel }`.
- **Shorthand after renaming** — *Shorthand props* looks up the local name:
  with `{ value: optionValue }`, `<Input value />` no longer sees the param;
  write `<Input value={optionValue} />`.
- **Not an immediate child** — `{cond && <$X />}`, `.map(...)`, inside a
  fragment or an intrinsic element → orphan-slot.
- **Spread on the parent** — the slot attribute is appended last, so it wins
  over a `$X` inside the spread.
- **Whitespace** — JSX already drops whitespace-only lines, so removing a slot
  element leaves no stray text in `children`.
- **Nesting** — a slot element belongs to its nearest parent element. A slot
  body may contain containers with their own slots.
- **Eager vs lazy** — a body without params is evaluated at the call site
  like ordinary `children`; a body with params runs only when the container
  calls `renderSlot`.
- **Calling `children` directly** is safe only for `SlotFn`. For
  `OptionalSlotFn` always go through `renderSlot`.
- **Identity** — the emitted object (and closure, if any) is new on every
  render, so a memoised container re-renders. Same as any render prop.
- **Hooks** — with params, the body is called as a function, not rendered
  as a component: a hook call written directly in it runs inside the
  container's render.

### Tooling

The language service answers from the same props-type query:

- Typing `$` (or `<`) as a child of `<Button>` completes the slots `Button`
  declares. Slots that are already filled are left out; required ones sort
  first.
- Inside a slot tag: completion and checking of the slot's options.
- Inside the params braces: completion of the names the container hands out.
- Hover on a slot tag shows the slot's declared type; go-to-definition jumps
  to the `$X` member of the container's props.

### Prior art

| | Form | Note |
| --- | --- | --- |
| Vue | `<template #cell="{ row }">` | scoped slots: the closest match — named slot + destructured params out; compiled to a function |
| Svelte 5 | `{#snippet cell(row)}…{/snippet}` inside the component tag | becomes a function prop of the parent |
| Svelte 4 | `<div slot="cell" let:row>` | `let:` = value flowing out; replaced by snippets |
| Astro | `<span slot="icon">` | static named slots; no params |
| Solid / React | `children={(row) => …}` | render props; what `.rtsx` emits |

## Flow control: `Switch` and `Match`

### Motivation

JSX conditionals are expressions in braces: `&&` leaks `0` and `""` into the
output, nested ternaries are unreadable, and a multi-way choice has no markup
form at all. `.rtsx` adds both shapes:

| | Renders | Shape |
| --- | --- | --- |
| `Match` | its body, if the subject is truthy. Every `Match` decides alone | `if` |
| `Switch` + `$Case` | at most one `$Case` — first match wins | `switch` |

Neither exists at runtime. The compiler replaces them with conditional
expressions, so a body that is not chosen is **never evaluated** — no elements
created, no embedded expressions run.

### Grammar

No new grammar: elements, slot elements and slot params are already defined.

```tsx
import { Switch, Match } from "reactogenic";  // package name is a placeholder
```

`Switch` and `Match` are recognised by **import origin, not by name**
(`import { Switch as Choose }` works). `$Case` is a slot tag and is never
imported. The package declares `Switch` as a container with a `$Case` slot, so
completion, hover and orphan-slot work exactly as for any container.

Three things are sanctioned here and nowhere else, because the compiler
consumes these elements itself:

- `$Case` may be filled **many times** (repeatable slots are otherwise deferred).
- `$Case` accepts `key` (see *State across branches*); on any other slot it is slot-key.
- Params on `Match` and `Switch` are consumed by the compiler instead of
  becoming a `children` callback: the bodies must stay inline for narrowing,
  and `Switch` params must reach the `is` of every `$Case`.

| Attribute | On | Meaning |
| --- | --- | --- |
| `on={expr}` | `Match` | the subject; evaluated once, tested for truthiness like `?:` |
| `{ value }` | `Match` | hands the subject out |
| `on={expr}` | `Switch` | the subject; evaluated once |
| `{ value }` | `Switch` | hands the subject out; switches to **dynamic** mode |
| `exhaustive` | `Switch` | the cases must cover the subject's type |
| `is={expr}` | `$Case` | static mode: a value compared with the subject by `===`. Dynamic mode: a condition, tested for truthiness |
| `default` | `$Case` | fallback; bare only, must be last |
| `key` | `$Case` | remount on switch; wraps the body in a keyed fragment |

`Switch` mode is syntactic: params present → dynamic, absent → static.

### Desugaring: `Match`

```tsx
// .rtsx
<p>
  <Match on={a % 3 === 0}>Fizz</Match>
  <Match on={a % 5 === 0}>Buzz</Match>
</p>
```

```tsx
// .tsx
<p>
  {a % 3 === 0 ? <>Fizz</> : null}
  {a % 5 === 0 ? <>Buzz</> : null}
</p>
```

A ternary, not `&&`: `<Match on={items.length}>` renders nothing for
`0`, never the text `0`.

The body is inline, so TS7 narrows a reference used as the subject:

```tsx
// .rtsx
<Match on={user}>
  <Avatar src={user.avatarUrl} />   // user: User, not User | null
</Match>
```

```tsx
// .tsx
{user ? <Avatar src={user.avatarUrl} /> : null}
```

A computed subject has no reference to narrow; params hand its value out:

```tsx
// .rtsx
<Match on={getUser()} { value: user }>
  <Avatar src={user.avatarUrl} />
</Match>
```

```tsx
// .tsx
{(({ value: user }) => user ? <Avatar src={user.avatarUrl} /> : null)({ value: getUser() })}
```

### Desugaring: static `Switch`

```tsx
// .rtsx
<Switch on={getStatus()}>
  <$Case is="loading"><Spinner /></$Case>
  <$Case is="error"><Oops /></$Case>
</Switch>
```

```tsx
// .tsx
{((_on) =>
  _on === "loading" ? <Spinner />
  : _on === "error" ? <Oops />
  : null
)(getStatus())}
```

No match and no `default` renders nothing, like a JS `switch`. Only
`exhaustive` ends in a throw.

When `on` is a **reference** — an identifier or a property-access chain — no
wrapper is emitted and the reference is repeated, so that TS7 narrows it
inside each body:

```tsx
// .rtsx — state: { status: "loading" } | { status: "error"; error: Error } | …
<Switch on={state.status}>
  <$Case is="loading"><Spinner /></$Case>
  <$Case is="error"><ErrorText error={state.error} /></$Case>
</Switch>
```

```tsx
// .tsx
{state.status === "loading" ? <Spinner />
  : state.status === "error" ? <ErrorText error={state.error} />
  : null}
```

### Desugaring: `exhaustive`

```tsx
// .rtsx — getStatus(): "loading" | "error" | "success"
<Switch on={getStatus()} exhaustive>
  <$Case is="loading">…</$Case>
  <$Case is="error">…</$Case>
</Switch>                          // Error: Missing "success"
```

```tsx
// .tsx
{((_on) =>
  _on === "loading" ? <>…</>
  : _on === "error" ? <>…</>
  : noMatch(_on)                   // noMatch(value: never): never — throws
)(getStatus())}
```

The proof is TS7's own: after every comparison the subject must have narrowed
to `never`. What is left over is the list of missing cases. The throw stays in
the output even when the proof succeeds — data can lie.

### Desugaring: dynamic mode

With params, `is` is a condition over the handed-out value.

```tsx
// .rtsx
<Switch on={getNumber()} { value: n }>
  <$Case is={n % 15 === 0}>FizzBuzz</$Case>
  <$Case is={n % 5 === 0}>Buzz</$Case>
  <$Case is={n % 3 === 0}>Fizz</$Case>
  <$Case default>{n}</$Case>
</Switch>
```

```tsx
// .tsx
{(({ value: n }) =>
  n % 15 === 0 ? <>FizzBuzz</>
  : n % 5 === 0 ? <>Buzz</>
  : n % 3 === 0 ? <>Fizz</>
  : <>{n}</>
)({ value: getNumber() })}
```

The container hands out `{ value }`; the pattern is ordinary slot params, so
renaming works. Its scope is the whole element: every `is` and every body.

A dynamic `Switch` cannot be `exhaustive`: arbitrary conditions cannot be proven.

Bodies follow the slot-body rule: the single child expression, or `<>…</>` for
several children or text. No body → `null`.

### Position

| Position | Emitted |
| --- | --- |
| JSX child | `{…}` |
| expression — `return <Switch …>`, arrow body, attribute value, root of a slot body | `(…)` |

### Typing behaviour

- Bodies are **inline**, not closures. The only function emitted is an
  immediately-invoked arrow, which TS's control-flow analysis sees through, so
  narrowing from the enclosing code and from each test reaches the body.
- In a `Switch`, each branch also sees every earlier test as false.
- Static mode: `is` is checked against the subject by TS7's comparison rules —
  `is="lodaing"` is "no overlap", and a repeated `is` is unreachable for the
  same reason.
- Dynamic mode: `value` has the type of `on`; `is={value}` narrows `value`
  inside that body.
- `Match` params: `value` has the type of `on`, narrowed to its truthy
  part inside the body.

### Compile errors

| Code | Message | Condition | Needs |
| --- | --- | --- | --- |
| switch-missing-case | Missing `"success"` | `exhaustive`, subject not narrowed to `never` | types |
| switch-dynamic-exhaustive | Dynamic switch cannot be exhaustive | `exhaustive` together with params | syntax |
| switch-exhaustive-default | Exhaustive switch takes no `default` | `exhaustive` together with `<$Case default>` | syntax |
| flow-no-subject | `Switch` / `Match` requires `on` | `on` missing | syntax |
| case-no-test | `$Case` requires `is` | `$Case` with neither `is` nor `default` | syntax |
| case-both | `$Case` takes `is` or `default`, not both | | syntax |
| case-default-value | `default` takes no value | `<$Case default={x}>` | syntax |
| case-default-not-last | `default` must be the last `$Case` | also covers two defaults | syntax |
| case-params | `$Case` provides no values | params on `$Case`; they go on the container | syntax |
| switch-children | Only `$Case` is allowed here | any other child of `Switch` (whitespace and comments excepted) | syntax |
| flow-attribute | | any other attribute or spread on `Switch`, `Match`, `$Case`; `key` on `Switch` or `Match` | syntax |
| flow-as-value | `Switch` / `Match` exist only as elements | `const S = Switch`, `as={Match}`, `createElement(Switch, …)`, re-export | syntax |

`$Case` outside `Switch` — including inside `Match` — is already
undeclared-slot or orphan-slot.

### Edge cases

- **Lazy means lazy** — a body that is not chosen never runs, including
  `{expensive()}` inside it.
- **`on` runs once**, before any `is`. Tests run top to bottom and stop at
  the first match.
- **Zero or more** is several `Match` elements side by side; each is its own
  child position, so React state in one is unaffected by the others.
- **Slot elements inside `Match`** — `<Button><Match …><$IconStart /></Match></Button>`
  is orphan-slot. Conditions go *inside* the slot body.
- **Reference subjects are re-read** at each comparison (see static `Switch`).
  A getter with side effects should be assigned to a `const` first.
- **`default` is a reserved word**, so *Shorthand props* never rewrites it.
  Bare `exhaustive` would be rewritten if an `exhaustive` variable were in
  scope — then it is flow-attribute, since it takes no value.
- **Inside `.map()`** — the emitted expression has no key; key the element in
  the body, as with any ternary.
- **Hooks** — nothing special; bodies are inline in the enclosing component.
- **Where they are allowed** is a layout question — see *Rules of layout*.

### State across branches

All branches of a `Switch` share one child position, and the compiler adds no
keys. React therefore **re-renders, not remounts**, when two branches render
the same component type — exactly as the hand-written ternary would:

```tsx
<Switch on={mode}>
  <$Case is="login"><Input name="email" /></$Case>
  <$Case is="signup"><Input name="email" /></$Case>
</Switch>
```

Switching `mode` keeps the one `Input` instance: typed text, focus, internal
`useState`, and no mount effects (autofocus, enter animation, fetch-on-mount).

To remount, the consumer sets `key` — on the body element, or on `$Case`,
which wraps the body in a keyed fragment:

```tsx
// .rtsx
<$Case is="login" key="login"><Input name="email" /></$Case>
```

```tsx
// .tsx
mode === "login" ? <Fragment key="login"><Input name="email" /></Fragment>
```

None of this is new: a hand-written ternary behaves identically, including the
single-element vs `<>…</>` distinction, which the author of the ternary would
have to write too. `.rtsx` does not fix problems it does not introduce.

### Prior art

| | Form | Note |
| --- | --- | --- |
| JS | `switch (x) { case "a": … }` / `switch (true) { case cond: … }` | static and dynamic mode are exactly these two idioms |
| Solid | `<Switch fallback><Match when>` | same wrapper shape, conditions only, no subject and no exhaustiveness. Naming clash: Solid's `Match` is our `$Case` |
| Vue | `v-if` / `v-else-if` / `v-else` | grouping by adjacency, no wrapper; branches auto-keyed |
| Svelte 5 | `{#if}…{:else if}…{:else}…{/if}` | block syntax |
| Rust / TS-pattern | `match x { … }` | exhaustive by default; here it is opt-in via `exhaustive` |
| React / Astro | `&&`, `?:` | what `.rtsx` emits |

## Segment roots

### Motivation

Two kinds of component should look different at the point of use:

| Kind | Used as | Example |
| --- | --- | --- |
| reusable component | imported, as in React | `<Button>`, `<Select>` |
| **segment** — a part of one page or feature, split out so that `index.rtsx` does not pile up | mounted by name into a **segment root** | `<section #about-us />` |

One token names the DOM id, the file (`+about-us.rtsx` — "plus this file") and
the URL fragment. `#` already means
"id" everywhere on the web (CSS, URLs), so the notation needs no explanation.
No import, no wrapper component, and `/#about-us` scrolls to it for free.

### Grammar

Real grammar change. `#` cannot start an attribute in today's TSX, so the form
is a syntax error there.

```
JsxAttributes   ::= … | JsxSegmentRoot
JsxSegmentRoot  ::= '#' JsxIdentifier          // JsxIdentifier allows hyphens: about-us
```

No whitespace between `#` and the name. At most one per element. Unrelated to
class private names (`#x`), which never occur in attribute position.

Only `#name` mounts a segment. A plain `id` keeps its React meaning and mounts
nothing:

```tsx
<section id="about-us" />   // just an id — +about-us.rtsx is not involved
```

### Desugaring

```tsx
// page.rtsx
<section #about-us className="band" />
```

```tsx
// page.tsx
import _Section_aboutUs from "./+about-us.jsx";
…
<section id="about-us" className="band">
  <_Section_aboutUs />
</section>
```

A root may be any element, HTML or component — the compiler only nests:

```tsx
// page.rtsx
<Section #about-us />
```

```tsx
// page.tsx
<Section id="about-us">
  <_Section_aboutUs />
</Section>
```

Two constraints, both checked by TS7 on the emitted code:

1. the root accepts `id` and `children`;
2. the segment file has a default export that is a component.

Rules:

- `#name` → `id="name"`, in the position where it was written.
- The segment is the **default export** of `+name.rtsx` (else `+name.tsx`) in
  the same directory as the file that mentions it. The name is used verbatim;
  there is no `name/index.rtsx` lookup. No such file → compile error.
- The import is **static**. A segment is part of the page, not a lazy chunk.
- The generated identifier is `_<Tag>_<camelCasedName>`; it is not nameable
  from user code.
- The segment is rendered with **no props**: a segment owns its data. Other
  attributes on the root go to the root element.

The `.tsx` above is the logical form, and what TS7 checks. It is the same
whichever way the page is produced, and the transform runs before either:

| Mode | Root | Segment |
| --- | --- | --- |
| pre-rendered + hydrated | emitted into the shell with the segment's HTML inside | hydrated in place |
| client-side render | emitted into the shell, empty | rendered into it on the client |

Which mode a page uses is not a syntax question — see `route-table.md`.

**Deferred:** lazily loaded segments.

Children of a segment root are overwritten, with a warning:

```tsx
// .rtsx
<section #about-us>something here</section>   // Warning: contents will be overwritten by +about-us.rtsx
```

```tsx
// .tsx
<section id="about-us">
  <_Section_aboutUs />
</section>
```

### Segment files

The `+` prefix marks a file as a segment and keeps the two kinds of component
disjoint:

| Rule | Error |
| --- | --- |
| a `+` file default-exports a component with no required props — checked for every `+` file, mounted or not | segment-not-component, segment-props |
| a `+` file is never imported by hand; `#name` is the only way in | segment-import |
| `#name` mounts only `+` files; a plain `about-us.rtsx` is not a candidate | segment-not-found |

Other exports of a `+` file are unreachable and therefore pointless; types
may still be exported and imported with `import type`.

`+` is safe where `#` is not: it needs no quoting in shells, is not a comment
character anywhere, and is literal in URL paths.

### Typing behaviour

TS7 checks the emitted import and element:

- the root does not accept `id` or `children` → segment-root-props;
- no default export, or the default export is not a component → segment-not-component;
- the component has required props → segment-props.

### Compile errors

| Code | Message | Condition | Needs |
| --- | --- | --- | --- |
| segment-not-found | No segment `+about-us.rtsx` or `+about-us.tsx` next to `page.rtsx` | file missing | files |
| segment-not-component | `+about-us.rtsx` has no default component | | types |
| segment-import | Segments are mounted with `#about-us`, not imported | value import of a `+` file | syntax |
| segment-props | A segment takes no props | required props on the default export | types |
| segment-id | | explicit `id` attribute (or a spread) together with `#name` | syntax |
| segment-duplicate | `#about-us` is already mounted | same name twice in one page | syntax (per file), route table (per page) |
| segment-root-props | `Card` must accept `id` and `children` to be a segment root | `<Card #about-us />` where `CardProps` lacks either | types |
| segment-self | | a segment that mounts itself, directly or through other segments | files |
| segment-children (warning) | Contents will be overwritten by `+about-us.rtsx` | root element has children | syntax |

### Edge cases

- **Spread** — `<section {...p} #about-us />` is segment-id: the spread could
  carry an `id` and the compiler cannot see it.
- **Loops and conditions** — a root inside `.map()` would repeat the id; inside
  `Match` / `Switch` it would make the shell vary. Both are ruled out in
  *Rules of layout*.
- **Nesting** — a segment may contain segment roots of its own; names resolve
  relative to the file they are written in.
- **Case** — the name is the file name, so `#AboutUs` and `#about-us` are
  different segments; kebab-case is the convention because it is also the
  fragment in the URL.
- **Shorthand props** never applies: `#name` is not a bare attribute.

- **Not reusable** — a segment is mounted once per page (segment-duplicate).
  Anything used twice is a reusable component and is imported.

- **Component roots** — what the component does with `id` is its own
  business. If it does not put it on a DOM node, `/#about-us` has nothing to
  scroll to; the compiler does not check.

**Rejected:** `#about-us.rtsx` as the file name. `#` is the fragment delimiter
in URLs (`import "./#about-us.jsx"` would need `%23`), a comment character in
bash, `.gitignore`, YAML and Makefiles, and Node's subpath-import prefix.

### Tooling

- After `#`: completion of sibling `+` files not yet mounted on the page.
- Cmd-click (go-to-definition) on `#counter` opens `+counter.rtsx`, or
  `+counter.tsx`. Renaming either side renames the other.
- Quick fix for segment-not-found: create the file with an empty default component.

### Prior art

| | Form | Note |
| --- | --- | --- |
| Pug / Slim / Emmet | `section#about-us` | `#` = id, from CSS selectors; the source of the notation |
| Astro | `<AboutUs client:visible />` | islands by directive on an imported component; no id, no file convention |
| Qwik | `component$` | boundaries by `$` marker, resumable instead of hydrated |
| SvelteKit | `+page.svelte`, `+layout.svelte` | `+` prefix marks files the framework owns; the source of the file convention |
| Next.js | `@modal/` parallel routes | file-system convention that fills a named place in a layout |
| SSI / Rails partials | `<!--#include file="…" -->`, `render "about_us"` | include by file name |
| Vue | `<template #name>` | **different meaning**: `#` is the slot shorthand there |
| Svelte, Solid | — | explicit import and element |

## Iteration: `Each`

### Motivation

`{items.map((item, index) => <p key={…}>…</p>)}` is the third brace-expression
that markup keeps falling into, after `&&` and `?:`. It nests badly, the
closing `)}` is noise, and a forgotten `key` is a runtime warning instead of a
compile error.

`Each` needs **no syntax of its own**. It is an ordinary runtime component
whose `children` is a slot function; params on a component
(see [Slots](#params-on-a-component-children-is-the-default-slot)) do the rest.
The only thing the compiler adds is key enforcement.

`.map()` keeps working — it parses today, so it keeps its meaning.

### The component

```tsx
// framework package — plain TSX
interface EachProps<T> {
  items: readonly T[];
  children: SlotFn<{ item: T; index: number }>;
}

function Each<T>({ items, children }: EachProps<T>) {
  return items.map((item, index) => children({ item, index }));
}
```

No fragment, no wrapper: `Each` returns the array of whatever the body returns.

### Desugaring

```tsx
// .rtsx
<Each items key={item.id} { item, index }>
  <p>{item.name} is {index + 1}</p>
</Each>
```

```tsx
// .tsx
<Each items={items}>
  {({ item, index }) => (
    <p key={item.id}>{item.name} is {index + 1}</p>
  )}
</Each>
```

Three steps, two of them general:

1. `items` → `items={items}` — *Shorthand props*.
2. params → the body becomes the `children` callback — *Slots*.
3. **`key` on `Each` moves onto the root of the body.** `Each` is recognised by
   import origin for this step only. The key expression may use the params.

A key written directly on the body root is simply left there:

```tsx
// .rtsx
<Each items={users} { item: user }>
  <UserRow key={user.id} user />
</Each>
```

```tsx
// .tsx
<Each items={users}>
  {({ item: user }) => <UserRow key={user.id} user={user} />}
</Each>
```

**The key is enforced at compile time**: one of the two forms must be present.

If the body is not a single element (several children, text), the slot-body
rule already turns it into `<>…</>`; a key moved from `Each` lands on that
fragment (`<Fragment key={…}>`). A key cannot be *written* on such a body, so
there `key` on `Each` is the only form.

### Typing behaviour

All TS7, from `EachProps<T>`:

- `item` is the element type of `items`, `index` is `number`. The params are
  an ordinary destructuring of `{ item, index }`: renaming and nesting work,
  and an unknown name (`{ itme }`) is a TS error.
- `items` that may be `undefined` is an ordinary TS error (`items ?? []`).
- The body is a real callback. Narrowing of `const`s and of the params works;
  narrowing of outer `let`s does not cross it, exactly as in `.map()`.

### Compile errors

| Code | Message | Condition | Needs |
| --- | --- | --- | --- |
| each-no-key | Each iteration needs a `key` | no `key` on `Each`, and the body is not a single element with `key` | syntax |
| each-double-key | | `key` on `Each` and on the body root | syntax |
| params-required | `Each` requires params | body without params (general slot error; write `{}` to ignore the values) | types |

### Edge cases

- **`key={index}`** is legal and explicit — the author has decided the list is
  static. The compiler does not second-guess it.
- **`key` on `Each` never reaches `Each`** — it is always moved, so `Each`
  itself cannot be keyed. Wrap it if that is ever needed.
- **Hooks** are not allowed directly in the body: it is a callback run inside
  `Each`'s render.
- **Slot elements in the body** — `<Select><Each …><$Option /></Each></Select>`
  is orphan-slot **in phase 1**. A container that renders many items takes the
  data as a prop and one slot as the template (*One slot, many items*).
  Phase 2 lifts this — see *Roadmap*.
- **Segment roots in the body** would repeat an id — ruled out in *Rules of layout*.
- **Nested `Each`** — inner params shadow outer ones; rename to reach both.

> OPEN: empty list. `Each` is a real container now, so an optional
> `$Empty: { children: ReactNode }` slot costs nothing in syntax. Svelte has
> `{:else}`, Solid has `fallback`.

> OPEN: iterables (`Set`, `Map`, generators) — a library decision only:
> `items: Iterable<T>` and `Array.from` inside `Each`.

### Roadmap

> ROADMAP (phase 2): slots inside `Each`.
>
> ```tsx
> <Select>
>   <Each items={options} { item: { value, label } }>
>     <$Option value>{label}</$Option>
>   </Each>
> </Select>
> ```
>
> Goal: an `Each` that is an immediate child of a container may produce that
> container's slot elements, one per item. `<$Option value>` picks `value` up
> from the params through *Shorthand props*.
>
> Not designed yet. It depends on:
>
> - **repeatable slots** (deferred in [Slots](#one-slot-many-items)): `$Option`
>   declared as an array, and the type-directed array-vs-object emit that was
>   drafted and dropped;
> - an exception to **orphan-slot**: a slot element whose parent is an `Each`
>   that is itself an immediate child of the container;
> - `Each` being **compiled** in this position rather than rendered — the
>   result is data for a prop, roughly
>   `$Option={options.map(({ value, label }) => ({ value, children: <>{label}</> }))}`,
>   not elements, so the key rule does not apply;
> - mixing static `<$Option>` elements with an `Each` in one container, and
>   the static-shell question a data-driven slot list raises in *Rules of layout*.
>
> Phase 1 must not close this door: keep `Each` recognised by import origin,
> and keep orphan-slot an error (not a silent pass-through to `children`).

### Prior art

| | Form | Note |
| --- | --- | --- |
| Solid | `<For each={items}>{(item, index) => …}</For>` | the same runtime shape — a component with a render-prop child — written by hand |
| Svelte 5 | `{#each items as item, index (item.id)}…{:else}…{/each}` | key in parentheses, optional; empty branch built in |
| Vue | `<li v-for="(item, index) in items" :key="item.id">` | directive on the element; key is a lint rule, not a compile error |
| Astro / React | `{items.map(…)}` | still valid in `.rtsx` |
