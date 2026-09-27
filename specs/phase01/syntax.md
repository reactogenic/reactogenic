# `.rtsx` syntax

`.rtsx` is a superset of `.tsx`. This document lists everything `.rtsx` adds.
Each extension is shown as a desugaring `.rtsx` → `.tsx`; the emitted `.tsx` is
what TS7 type-checks and what Vite runs (see [vite.md](vite.md)). Errors on
the emitted `.tsx` are reported on the `.rtsx` (see [diagnostics.md](diagnostics.md)).

Governing rule: new meaning goes only to forms that are syntax errors in
today's TSX. A syntax error today is good news: no existing program uses the
form, so it is safe to reserve. Forms that already parse keep their React
meaning; exceptions are called out explicitly in the section that makes them.

| Reserved form (syntax error today) | Used by |
| --- | --- |
| `{ a, b }` / `{}` in attribute position | slot params |
| `#name` in attribute position | segment roots — a syntax error for esbuild and Babel; TypeScript's parser accepts it (see *Segment roots*) |
| `&name`, `&&name` in attribute position | slot args (see *Slots → Attachment*) |

| Extension | Status |
| --- | --- |
| [Shorthand props](#shorthand-props) | Draft |
| [Slots](#slots) — slot elements + slot params | Draft |
| [Flow control](#flow-control-switch-and-match) — `Switch`, `Match`, `$Case` | Draft |
| [Segment roots](#segment-roots) — `<section #about-us />` | Draft |
| [Iteration](#iteration-each) — `Each` | Draft |

## Compilation passes

The desugarings are not one transform but a sequence of small ones. Each pass
sees the output of the previous one, so features compose without special
cases — conditional slots exist only because pass 3 runs after pass 2.

| # | Pass | Input → output |
| --- | --- | --- |
| 0 | checks on the source | segment files, `Each` keys — reported against what the author wrote |
| 1 | shorthand props | bare attribute → `name={name}`; resolved first, while the source scopes (params included) are intact |
| 2 | flow lowering | `Match`, `Switch` → conditional expressions |
| 3 | slots | attachments (`slot={$X}`) → conditional elements; slot elements, and conditional expressions of slot elements → `$X` props; params → callbacks |
| 4 | segment roots | `#name` → `id`, import, nested default export |

The `.rtsx` → `.tsx` examples in this document show the final output unless
they say otherwise.

> ROADMAP: phase 2 (`Each` around slot elements, see [Iteration](#iteration-each))
> is the same idea once more — lower `Each` to `.map()`, then hoist a `.map()`
> of slot elements into an array prop.

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

The rule is **scope-directed**, not type-directed: the transpiler needs only the
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
An `.rtsx` file is always a module, as under Vite: its top-level declarations
count even when it has no `import` or `export`.

Resolution is ordinary lexical lookup — the nearest binding wins — and it is
**tsgo's binder**, not a resolver of our own. Since the emitted identifier is
resolved by TS the same way, shadowing needs no special handling.

### Typing behaviour

None of its own. After desugaring, TS7 checks `value={value}` or `value`
(= `true`) against the prop type as usual.

- `boolean | X` props need no special rule: scope alone picks A or B.
- A same-named binding of another type is an ordinary TS error, not a
  transpiler rule:

  ```tsx
  const disabled = 42;
  <Button disabled />   // TS: Type 'number' is not assignable to type 'boolean'
  ```

  > ROADMAP: a friendlier hint on that error, "Did you mean
  > `disabled={true}`?"
- Case B on a non-boolean prop is an ordinary TS error
  (`Type 'true' is not assignable to type 'string'`), mapped back to the bare
  attribute.

The mapped error carries "no `value` in scope" as related information — it
explains the most likely cause, a typo or a deleted variable (see
[diagnostics.md](diagnostics.md)).

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

> ROADMAP: a warning in the `.tsx` → `.rtsx` migration path on bare
> attributes that resolve as case A (the silent flip). Belongs with editor
> tooling ([../later/tooling.md](../later/tooling.md)).

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

A container has named places (icon, title, option). In plain React each place
is a prop; once a place needs props *and* values from the container, the call
site turns inside-out:

```tsx
<Button size="lg" $IconStart={{ className: "icon", children: ({ size }) => <Icon name="plus" size={size} /> }}>
```

A slot element writes the same thing top-down, as markup. A slot is **a prop
value** — props for one element the container renders, plus its body — and
the container decides where that element goes.

`$` is reserved for slots ("$ stands for $lot, not $ystem"). Rule of thumb:
**bare name = pass my value in; braces = give me your value out.**

| Term | Where | Part | Direction |
| --- | --- | --- | --- |
| slot **props** | caller | `className="icon"` in `<$IconStart className="icon">` | caller → element |
| slot **body** | caller | the children of `<$IconStart>` | caller → element |
| slot **params** | caller | `{ size }` in `<$IconStart { size }>` | container → caller |
| **attachment** | container | `<span slot={$IconStart} />` | where the slot renders |
| **args** | container | `&size` on the attachment | what the params receive |

### Declaration: `Slot<P>` and `FnSlot<P, A>`

A slot is a `$`-named prop. Its type is the **complete prop contract** of the
element it stands for:

```tsx
type Slot<Props> = Props;

// A function slot: its body receives the attachment's args.
type FnSlot<Props, Args> = Omit<Props, "children"> & { children?: (args: Args) => ReactNode };
```

```tsx
interface ButtonProps {
  $IconStart: Slot<ComponentProps<"span">>;   // required
  $IconEnd?: Slot<ComponentProps<"span">>;    // optional
  $Badge?: FnSlot<ComponentProps<"span">, { size: ButtonSize }>;
  size?: ButtonSize;
  children: ReactNode;
}
```

- **Required or optional** is ordinary TypeScript optionality of the prop.
- **The contract is complete.** `$Box: Slot<{ color: string }>` takes no
  body: `<$Box color="red" />` is fine, `<$Box color="red">Hi</$Box>` is an
  error — unless `children` is in the contract, and then its optionality
  decides whether a body is required.
- **Slots are singular.** A slot is one prop value, never a list; a slot typed
  as an array (`Slot<P>[]`) is an error at its attachment (below).

### Attachment: `slot={$X}`

A container attaches a slot to the element it stands for:

```tsx
// Button.rtsx
function Button({ $IconStart, $Badge, size, children }: ButtonProps) {
  return (
    <button>
      <span slot={$IconStart} className="icon" />
      {children}
      <span slot={$Badge} &size>new</span>
    </button>
  );
}
```

```tsx
// Button.tsx
import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotProps as _slotProps } from "@reactogenic/core";

function Button({ $IconStart, $Badge, size, children }: ButtonProps) {
  return (
    <button>
      {_isAssigned($IconStart) ? <span className="icon" {..._slotProps($IconStart)}>{_renderSlot($IconStart, {})}</span> : null}
      {children}
      {_isAssigned($Badge) ? <span {..._slotProps($Badge)}>{_renderSlot($Badge, { size: size }, "new")}</span> : <span>new</span>}
    </button>
  );
}
```

- **Slot props replace the attachment's props, per prop.** The attachment's
  props are defaults: `<span slot={$IconStart} className="icon" />` with
  `<$IconStart className="override" />` renders `className="override"`, not
  both; with `<$IconStart id="x" />` it keeps `className="icon"` and adds
  `id="x"`.
- **A missing slot renders nothing** — no empty element; `NOT_ASSIGNED` is
  missing too — unless the attachment has children: then they are the
  **fallback**, rendered in place
  of the slot's body whenever there is none (no slot, or a slot without a
  body). Other attachment props do not establish a fallback.
- **`key`** on an attachment is React's key on the element, never a slot prop
  or an arg.
- **Args.** Attributes with `&` are args for a function slot's body; with `&&`
  they are args *and* props of the element:

  | Attribute | Element prop | Arg |
  | --- | --- | --- |
  | `value={x}` | ✓ | |
  | `&value={x}` | | ✓ |
  | `&&value={x}` | ✓ | ✓ |
  | `&value`, `&&value` | — | shorthand for `={value}`: always the binding, never `true` — an arg is a function argument, not a prop; an unbound name is TS's own error |

  `&&` props are attachment props like any other: the slot's props replace
  them, and the args still carry the attachment's value.
  `<$Option { value } value="42" />` is valid — it replaces the rendered
  `value`; `<$Option { value } value={value + "!"} />` is not — params are not
  in scope in the slot element's own attributes (a reference error).
- **Args follow function-call rules.** They go through `renderSlot`, typed by
  the slot: a function slot rendered without one of its args, and a plain slot
  rendered with any, are ordinary TS errors — as `f()` for `f(x)`, and
  `f({ x: 1 })` for `f()`. A slot typed as an array is rejected there too.
- `slot={$X}` exists only in `.rtsx`: a container is an `.rtsx` file.

> **Exception to the governing rule.** `slot` is an HTML attribute (Web
> Components), and `<div slot={x}>` parses today with that meaning. `.rtsx`
> claims the form only when the value is a `$` reference — an identifier
> starting with `$`, or a property-access chain ending in one
> (`props.$Label`). `slot="header"` and any other value keep their meaning.

**One slot, many executions.** A singular slot may be attached where it runs
many times; slot cardinality and attachment cardinality are different things:

```tsx
// Select.rtsx
function Select({ options, $Option }: SelectProps) {
  const [selected, setSelected] = useState<string>();
  return (
    <select>
      <Each items={options} { item: option }>
        <option key={option.value} slot={$Option}
          &&value={option.value} &label={option.label} &&selected={selected === option.value}>
          {option.label}
        </option>
      </Each>
    </select>
  );
}
```

```tsx
// .rtsx
<Select options>
  <$Option { label, selected }>
    <Text>{label}</Text>
    <Match on={selected}><Icon name="check" /></Match>
  </$Option>
</Select>
```

There is one `$Option` value; its attachment runs once per option.

> OPEN (#4): keys when an attachment runs per item. The report of a missing
> key belongs on the slot call (`<$Option { label }>`), not on the
> attachment or on `Each`. How a per-item key reaches the rendered element is
> not decided: `key` among the args, or params in scope in the slot
> element's own attributes (`<$Option { value } key={value}>`). Until then
> `key` on a slot element stays slot-key.

### Usage and desugaring

`<$X>` constructs the value of the prop `$X`:

```tsx
// .rtsx
<Button size="lg">
  <$IconStart className="icon" { size }>
    <Icon name="plus" size />
  </$IconStart>
  Add
</Button>
```

```tsx
// .tsx
<Button size="lg"
  $IconStart={{
    className: "icon",
    children: ({ size }) => <Icon name="plus" size={size} />,
  }}>
  Add
</Button>
```

Note `<Icon size />`: the params put `size` in scope, so *Shorthand props*
case A applies.

A text body is a string:

```tsx
// .rtsx
<Button>
  <$Label a="b">Text</$Label>
</Button>
```

```tsx
// .tsx
<Button
  $Label={{
    a: "b",
    children: "Text",
  }}
/>
```

Params are optional. Without them the body is passed as plain content:

```tsx
// .rtsx
<Button size="md">
  <$IconStart className="icon">
    <Icon name="plus" />
  </$IconStart>
  Add
</Button>
```

```tsx
// .tsx
<Button size="md"
  $IconStart={{
    className: "icon",
    children: <Icon name="plus" />,
  }}>
  Add
</Button>
```

For each component element `<P>`:

1. Every slot element placed as below (*Placement*) is taken out of the
   children and becomes the attribute `$Name={…}`, appended after all written
   attributes, in order of first appearance.
2. Slot attributes become object properties, order preserved:

   | Slot attribute | Property |
   | --- | --- |
   | `className="icon"` | `className: "icon"` |
   | `gap={2}` | `gap: 2` |
   | `wide` | `wide: wide` or `wide: true` (*Shorthand props*) |
   | `aria-label="x"` | `"aria-label": "x"` |
   | `{...rest}` | `...rest` |

3. Slot elements inside a slot element are **its** slots: they become
   properties of its value (*Recursive slots*).
4. The rest of the body becomes the `children` property — purely syntactic:

   | Slot element | Property |
   | --- | --- |
   | params + body | `children: (params) => body` |
   | body only | `children: body` |
   | no body (`<$X … />`) | no `children` property |

   `body` — the **body rule**, shared with `Match` and `Switch`:

   | Body | Emitted |
   | --- | --- |
   | one element | the element |
   | one `{expr}` | `expr` |
   | text alone | a string literal, as React renders it: whitespace collapsed by JSX's rules (`"Save"`); text with an HTML entity stays `<>…</>` |
   | several children | `<>…</>` |
5. Whatever is left in `<P>` stays as `children`.

The emitted value is always an object, even with no props: `{}`, or
`{ children: "Text" }`.

### Recursive slots

A slot's contract can contain slots, and slot elements inside a slot element
fill them:

```tsx
// .rtsx
<Dialog>
  <$Action variant="solid">
    <$IconStart>
      <Icon name="close" />
    </$IconStart>
    Close
  </$Action>
</Dialog>
```

```tsx
// .tsx
<Dialog
  $Action={{
    variant: "solid",
    $IconStart: { children: <Icon name="close" /> },
    children: "Close",
  }}
/>
```

with `DialogProps { $Action: Slot<ButtonProps> }`: the attachment
`<Button slot={$Action} />` spreads `$IconStart` onto `Button`, which attaches
it in turn. A slot element belongs to its **nearest** parent element: in
`<$Action><Button><$IconStart /></Button></$Action>` it fills `Button`'s
`$IconStart`, not `$Action`'s.

### Placement

A slot element must be:

- a **direct child** of a component element or of a slot element, or
- inside **`Match`, `Switch` or `Each`**, recognised by import origin — a
  custom component that wraps them is never inferred (`const MyMatch = Match`
  is not flow control).

Anywhere else — inside an intrinsic element, a fragment, `{c && <$X />}`,
`.map()` — it is orphan-slot.

### Repeated slots: last assignment wins

A slot is one prop value. Assigning it again replaces it, as a repeated key in
an object literal does:

```tsx
// .rtsx
<Select>
  <$Option value="1" />
  <$Option value="2" />
</Select>
```

```tsx
// .tsx
<Select $Option={{ value: "2" }} />
```

A slot element also replaces an explicit `$X={…}` attribute on the same
element: slot elements come after written attributes.

Conditional assignments follow the same rule: an assignment that does not
happen changes nothing. `<$X a /><Match on={c}><$X b /></Match>` →
`$X={c ? { b } : { a }}`; with no earlier assignment, the false branch is
`NOT_ASSIGNED` (*Conditional slots*).

> OPEN (#7):
> `Each` around slot elements, and `Match` / `Switch` with params around them,
> are allowed by *Placement* but have no semantics yet: orphan-slot until
> decided.

### Conditional slots

A slot may be filled conditionally. It falls out of compiling in passes (see
*Compilation passes*):

```tsx
// .rtsx
<Input value={value} onChange={onChange}>
  <Match on={invalid}>
    <$Hint>{renderErrors(errors)}</$Hint>
  </Match>
</Input>
```

```tsx
// after pass 2 — Match → ternary
<Input value={value} onChange={onChange}>
  {invalid ? <$Hint>{renderErrors(errors)}</$Hint> : null}
</Input>
```

```tsx
// after pass 3 — a ternary of slot elements → a ternary prop
import { NOT_ASSIGNED as _NOT_ASSIGNED } from "@reactogenic/core";

<Input value={value} onChange={onChange}
  $Hint={invalid ? { children: renderErrors(errors) } : _NOT_ASSIGNED} />
```

- Each branch is desugared as a slot element on its own. A `null` branch is
  an assignment that did not happen: it keeps the value before it, or is
  **`NOT_ASSIGNED`** when there is none — never `undefined`. With per-prop
  replacement an `undefined` would *replace* a default (a nested
  `$IconStart: undefined` spread onto `Button` overrides the attachment's own
  `$IconStart`); `NOT_ASSIGNED` never does: an attachment treats it as no slot
  (`isAssigned`), and a spread of slot props skips it (`slotProps`).
- `Slot<P>` and `FnSlot<P, A>` include `NotAssigned`, so an optional slot
  takes it.
  > OPEN: a *required* slot filled only conditionally is therefore no longer
  > a type error (it was: `undefined` is not assignable). Keep the sentinel
  > out of required slots — a separate type for optional ones — or accept?
- Narrowing works: the branch is inline in the ternary.

| Not supported | Error |
| --- | --- |
| branches that fill **different** slots (`c ? <$IconStart /> : <$IconEnd />`) | mixed-conditional-slot |
| a branch that mixes a slot element with other children, or holds several | mixed-conditional-slot |

### Params on a component: `children` is the default slot

The rule is one and the same everywhere: **wherever the transpiler sees params,
it wraps the body in a callback.** On a slot element the callback is the
slot's `children`; on a component element it is the component's `children`.

```tsx
// .rtsx
const items = getItems();
<Each items { item, index }>
  <p key={item}>{item} is {index + 1}</p>
</Each>
```

```tsx
// .tsx
const items = getItems();
<Each items={items}>
  {({ item, index }) => (
    <p key={item}>{item} is {index + 1}</p>
  )}
</Each>
```

The component is an ordinary runtime component whose `children` is a
function. With slot elements present, they are hoisted to attributes first
and the callback wraps what is left: the params are in scope in the
component's children only — not in its slot elements, and not in its own
attributes.

### Grammar

Three parts.

**Slot elements — no grammar change.** `<$IconStart>` parses today as a
component reference. `.rtsx` keeps React's rule for every plain-identifier
tag and takes only the ones starting with `$`:

| Tag | TSX today | `.rtsx` |
| --- | --- | --- |
| `div`, `my-element` — starts with a lowercase letter, or contains `-` | intrinsic element | unchanged |
| `Button`, `_Button`, `Ärger` — anything else | component | unchanged |
| `$IconStart` — starts with `$` | component | slot element |

A `$` tag is never resolved as an identifier, so nothing needs importing and
two containers can both have a `$Title` without colliding. Member-expression
tags (`<motion.div>`, `<Icons.Plus>`) are references, as today.

> **Exception to the governing rule.** A tag starting with `$` parses today
> as a component. In `.rtsx` it is a slot element. Never silent: a TSX
> program that renders `<$Modal>` has a `$Modal` binding in scope, and that
> is component-name. Rename at the import: `import { $Modal as Modal }`,
> then `<Modal>`.

**Slot params.**

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
and rest are copied to the emitted parameter unchanged. Renaming is the way
out of a name collision with the outer scope:

```tsx
// .rtsx
const options = getOptions();
const label = "Country";
<Select options>
  <$Option { label: optionLabel, value: optionValue }>
    {label}: {optionLabel} ({optionValue})
  </$Option>
</Select>
```

```tsx
// .tsx
const options = getOptions();
const label = "Country";
<Select options={options}
  $Option={{
    children: ({ label: optionLabel, value: optionValue }) =>
      <>{label}: {optionLabel} ({optionValue})</>,
  }} />
```

**Args: `&name` and `&&name`.**

```
JsxAttribute ::= … | ("&" | "&&") JsxAttributeName [ "=" JsxAttributeValue ]
```

`&` and `&&` cannot start an attribute in today's TSX (TypeScript, esbuild
and Babel all reject them), so the forms are free to reserve. No whitespace
after `&` / `&&`. Only on an attachment (`slot={$X}`); anywhere else is
arg-without-slot.

### Typing behaviour

**The transpiler is purely syntactic**: it never reads a type to decide what
to emit. TS7 checks the emitted `.tsx`, and that is what types slots:

- Param names are contextually typed from the slot's function body:
  `{ colour }` on a slot that only hands out `size` → error.
- Slot attributes are checked as a fresh object literal: unknown prop →
  excess-property error; missing required prop → missing-property error; a
  body on a contract without `children` → excess-property error.
- Args are checked by `renderSlot` (*Attachment*).

### Compile errors

| Code | Message | Condition | Needs |
| --- | --- | --- | --- |
| undeclared-slot | `$X` is not declared in `P` | parent has no `$X` prop | types |
| missing-slot | `P` requires `$X` | required slot not filled | types |
| no-values | `$X` provides no values | params on a slot whose body is not a function | types |
| content-not-allowed | `$X` takes no body | a body on a slot whose contract has no `children` | types |
| content-required | `$X` requires content | `<$X … />`, `children` required | types |
| orphan-slot | Slot must be immediate child of the component | not placed as in *Placement* | syntax |
| mixed-conditional-slot | A conditional slot fills one slot | see *Conditional slots* | syntax |
| params-on-html | Params are only allowed on components and slot elements | `<div { size }>` — an intrinsic element by React's rule (lowercase, `-`, or `a:b`) | syntax |
| duplicate-params | An element takes one params pattern | `<$X { a } { b }>` | syntax |
| slot-children-conflict | | `children=` attribute on a slot element that also has a body | syntax |
| slot-key | Slots are not elements | `key` on a slot element (see OPEN #4) | syntax |
| arg-without-slot | `&size` is an arg of a slot attachment | `&` / `&&` on an element without `slot={$X}` | syntax |
| component-name | `$Modal` is a slot tag; rename the component where it is imported | a `$` tag while a value binding of the same name is in scope — reported instead of the slot errors | syntax |

### Edge cases

- **Params scope** covers the slot body only, not the slot element's own
  attributes: in `<$X spacing { spacing }>` the attribute `spacing` resolves
  in the outer scope.
- **Shadowing** — a param shadows outer names inside the body. To keep the
  outer name reachable, rename the param: `{ label: optionLabel }`.
- **Shorthand after renaming** — *Shorthand props* looks up the local name:
  with `{ value: optionValue }`, `<Input value />` no longer sees the param.
- **Spread on the parent** — the slot attribute is appended last, so it wins
  over a `$X` inside the spread.
- **Whitespace** — JSX drops whitespace-only lines, so removing a slot element
  leaves no stray text in `children`.
- **Eager vs lazy** — a body without params is evaluated at the call site like
  ordinary `children`; a body with params runs only when the attachment
  renders.
- **`&&` evaluates its expression twice** — once as a prop, once as an arg.
- **Identity** — the emitted object (and closure, if any) is new on every
  render, so a memoised container re-renders. Same as any render prop.
- **Hooks** — with params, the body is called as a function, not rendered as a
  component: a hook call written directly in it runs inside the container's
  render.

> TECH DEBT: reachability. A supplied slot is useful only if its attachment is
> reachable in the effective render tree. A declared slot the container never
> attaches, and an attachment removed by an ancestor slot's body (a `$List`
> whose body replaces the `Each` that attached `$Option`), drop the slot
> silently. Phase 1 does not diagnose either.

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

Neither exists at runtime. The transpiler replaces them with conditional
expressions, so a body that is not chosen is **never evaluated** — no elements
created, no embedded expressions run.

### Grammar

No new grammar: elements, slot elements and slot params are already defined.

```tsx
import { Switch, Match } from "@reactogenic/core";
```

`Switch` and `Match` are recognised by **import origin, not by name**
(`import { Switch as Choose }` works). `$Case` is a slot tag and is never
imported. The package declares `Switch` as a container with a `$Case` slot, so
the slot errors (undeclared-slot, orphan-slot) work exactly as for any
container. Both are lowered away, so the emitted `.tsx` drops their import.

`$Case` repeats, unlike any other slot (see *Slots → Repeated slots*): the
transpiler consumes the `$Case` elements itself in pass 2, before slots are
hoisted, so they never become a prop. Two more things are sanctioned here and
nowhere else, for the same reason:

- `$Case` accepts `key` (see *State across branches*); on any other slot it is slot-key.
- Params on `Match` and `Switch` are consumed by the transpiler instead of
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
  {a % 3 === 0 ? "Fizz" : null}
  {a % 5 === 0 ? "Buzz" : null}
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
import { noMatch as _noMatch } from "@reactogenic/core";

{((_on) =>
  _on === "loading" ? "…"
  : _on === "error" ? "…"
  : _noMatch(_on)                  // noMatch(value: never): never — throws
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
  n % 15 === 0 ? "FizzBuzz"
  : n % 5 === 0 ? "Buzz"
  : n % 3 === 0 ? "Fizz"
  : n
)({ value: getNumber() })}
```

The container hands out `{ value }`; the pattern is ordinary slot params, so
renaming works. Its scope is the whole element: every `is` and every body.

A dynamic `Switch` cannot be `exhaustive`: arbitrary conditions cannot be proven.

Bodies follow the body rule of *Slots → Usage and desugaring*: one
element as it is, the expression of one `{…}` child, text alone as a string
literal, `<>…</>` for several children. No body → `null`.

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
- **Slot elements inside `Match` / `Switch`** — `<Button><Match …><$IconStart /></Match></Button>`
  fills an optional slot conditionally; see *Conditional slots* in [Slots](#conditional-slots).
- **Reference subjects are re-read** at each comparison (see static `Switch`).
  A getter with side effects should be assigned to a `const` first.
- **`default` is a reserved word**, so *Shorthand props* never rewrites it.
  Bare `exhaustive` would be rewritten if an `exhaustive` variable were in
  scope — then it is flow-attribute, since it takes no value.
- **Inside `.map()`** — the emitted expression has no key; key the element in
  the body, as with any ternary.
- **Hooks** — nothing special; bodies are inline in the enclosing component.

### State across branches

All branches of a `Switch` share one child position, and the transpiler adds no
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
<Switch on={mode}>
  <$Case is="login" key="login"><Input name="email" /></$Case>
  <$Case is="signup" key="signup"><Input name="email" /></$Case>
</Switch>
```

```tsx
// .tsx
import { Fragment as _Fragment } from "react";

{mode === "login" ? <_Fragment key="login"><Input name="email" /></_Fragment>
  : mode === "signup" ? <_Fragment key="signup"><Input name="email" /></_Fragment>
  : null}
```

Generated names (`_on`, `_noMatch`, `_Fragment`) never collide with the
author's: a name used anywhere in the source gets a number (`_on1`).

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

Real grammar change. In today's TSX the form is a syntax error for the
compilers that build it: esbuild (and so Vite) and Babel reject `#` in
attribute position. No program that builds today contains it, so it is safe
to reserve.

TypeScript's own parser (5.x and tsgo) is lenient here: it reads `#about-us`
as an ordinary attribute *named* `"#about-us"` and reports nothing; at most
the checker later finds no such prop. `.rtsx` gives the form its meaning in
`.rtsx` files only; in `.tsx` TypeScript's reading is unchanged.

```
JsxAttributes   ::= … | JsxSegmentRoot
JsxSegmentRoot  ::= '#' JsxIdentifier          // JsxIdentifier allows hyphens: about-us
```

No whitespace between `#` and the name (tsgo's scanner already rejects a
lone `#`). At most one per element (segment-id), and no value or namespace:
`#about-us="x"` and `#about:us` are segment-syntax. Unrelated to
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
import _Section_aboutUs from "./+about-us";

<section id="about-us" className="band">
  <_Section_aboutUs />
</section>
```

A root may be any element, HTML or component — the transpiler only nests:

```tsx
// page.rtsx
<Section #about-us />
```

```tsx
// page.tsx
import _Section_aboutUs from "./+about-us";

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

A segment root is syntactic sugar and nothing more: the `.tsx` above is its
whole meaning — what TS7 checks and what Vite runs. The import is
extensionless; resolving it to `+about-us.rtsx` is the plugin's job
([vite.md](vite.md#module-resolution)).

**Deferred:** lazily loaded segments.

Children of a segment root are overwritten, with a warning:

```tsx
// .rtsx
<section #about-us>something here</section>   // Warning: contents will be overwritten by +about-us.rtsx
```

```tsx
// .tsx
import _Section_aboutUs from "./+about-us";

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

`+` is safe where `#` is not: it needs no quoting in a shell, is not a comment
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
| segment-id | An element has one id: `#about` is already on it | explicit `id` attribute (or a spread), or a second `#name`, together with `#name` | syntax |
| segment-syntax | `#about-us` is a segment root: it takes no value and no namespace | `#about-us="x"`, `#about:us` | syntax |
| segment-duplicate | `#about-us` is already mounted | same name twice in one page | syntax (per file), route table (per page) |
| segment-root-props | `Card` must accept `id` and `children` to be a segment root | `<Card #about-us />` where `CardProps` lacks either | types |
| segment-self | | a segment that mounts itself, directly or through other segments | files |
| segment-in-loop | `#about-us` would be mounted more than once | segment root inside `.map()` or the body of `Each` | syntax |
| segment-children (warning) | Contents will be overwritten by `+about-us.rtsx` | root element has children | syntax |

### Edge cases

- **Spread** — `<section {...p} #about-us />` is segment-id: the spread could
  carry an `id` and the transpiler cannot see it.
- **Loops** — a root inside `.map()` or `Each` would repeat the id:
  segment-in-loop. Inside `Match` / `Switch` a root is fine: it is mounted at
  most once.
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
  scroll to; the transpiler does not check.

**Rejected:** `#about-us.rtsx` as the file name. `#` is the fragment delimiter
in URLs (`import "./#about-us.jsx"` would need `%23`), a comment character in
bash, `.gitignore`, YAML and Makefiles, and Node's subpath-import prefix.

### Prior art

| | Form | Note |
| --- | --- | --- |
| Pug / Slim / Emmet | `section#about-us` | `#` = id, from CSS selectors; the source of the notation |
| SvelteKit | `+page.svelte`, `+layout.svelte` | `+` prefix marks files the framework owns; the source of the file convention |
| Next.js | `@modal/` parallel routes | file-system convention that fills a named place in a layout |
| SSI / Rails partials | `<!--#include file="…" -->`, `render "about_us"` | include by file name |
| Vue | `<template #name>` | **different meaning**: `#` is the slot shorthand there |
| Svelte, Solid | — | explicit import and element |

## Iteration: `Each`

### Motivation

`{items.map((item, index) => <p key={…}>…</p>)}` is the third brace-expression
that markup keeps falling into, after `&&` and `?:`. It nests badly, the
closing `)}` is noise.

`Each` needs **no syntax of its own**. It is an ordinary runtime component
whose `children` is a slot function; params on a component
(see [Slots](#params-on-a-component-children-is-the-default-slot)) do the rest.

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
const items = getItems();
<Each items { item, index }>
  <p key={item.id}>{item.name} is {index + 1}</p>
</Each>
```

```tsx
// .tsx
const items = getItems();
<Each items={items}>
  {({ item, index }) => (
    <p key={item.id}>{item.name} is {index + 1}</p>
  )}
</Each>
```

Both steps are general rules; nothing here is specific to `Each`:

1. `items` → `items={items}` — *Shorthand props*.
2. params → the body becomes the `children` callback — *Slots*.

The `key` is written where React wants it: on the root element of the body.
It may use the params. **`Each` does not check it**, as a `for` loop would
not: a missing key is React's runtime warning. (Where a missing key is
reported for slots, see *Slots → Attachment*, OPEN #4.)

`key` on `<Each>` itself keeps its React meaning: it keys the `Each` element,
not the iterations.

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
| params-required | `Each` requires params | body without params (general slot error; write `{}` to ignore the values) | types |

### Edge cases

- **`key={index}`** is legal and explicit — the author has decided the list is
  static. The transpiler does not second-guess it.
- **Hooks** are not allowed directly in the body: it is a callback run inside
  `Each`'s render.
- **Slot elements in the body** — `<Select><Each …><$Option /></Each></Select>`
  is allowed by *Slots → Placement* but has no semantics yet (OPEN #7):
  orphan-slot until decided. A slot is singular; a container that renders many
  items attaches it inside its own `Each` (*Slots → Attachment*).
- **Segment roots in the body** would repeat an id — segment-in-loop (see
  [Segment roots](#compile-errors-3)).
- **Nested `Each`** — inner params shadow outer ones; rename to reach both.

**Rejected:** an empty-list branch (`$Empty`, Svelte's `{:else}`, Solid's
`fallback`). "Empty" is opinionated — `[]`, but also `""`, `{ items: [] }`,
"only archived" — so the syntax does not define it. Write
`<Match on={items.length === 0}>` next to the `Each`.

**Rejected:** iterables (`Set`, `Map`, generators) as `items`. `Each` takes
arrays only, so that `index` has one stable meaning and type, `number`: the
position in `items`. Convert at the call site: `items={[...set]}`.

### Prior art

| | Form | Note |
| --- | --- | --- |
| Solid | `<For each={items}>{(item, index) => …}</For>` | the same runtime shape — a component with a render-prop child — written by hand |
| Svelte 5 | `{#each items as item, index (item.id)}…{:else}…{/each}` | key in parentheses, optional; empty branch built in |
| Vue | `<li v-for="(item, index) in items" :key="item.id">` | directive on the element; key is a lint rule, not a compile error |
| Astro / React | `{items.map(…)}` | still valid in `.rtsx` |
