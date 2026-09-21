# Rules of layout

Not syntax: a discipline the compiler enforces on where `.rtsx` constructs may
appear. It follows from how a Reactogenic page is built and served.

## The model

Reactogenic is a compiled framework.

| | Compile time | Runtime |
| --- | --- | --- |
| **Shell** | compiled **once** per pathname into plain, precise HTML + CSS + raw JS. Layout components are executed by the compiler and leave no React behind | served as static resources |
| **Island** | its root element is emitted into the shell **empty**; the island is bundled separately | React + one JS bundle per island; each island is mounted as a **separate React app** |

Navigation is strictly server-side: every navigation is a new document. The
shell is identical across documents, so the browser's cross-document View
Transitions give flicker-free navigation.

Two facts drive every rule below:

1. **The shell has no React.** It is raw HTML/CSS/JS, so nothing in it may
   depend on the React runtime.
2. **The shell compiles only once.** There is no request, no user and no data
   at that moment, and the result is the page for everyone. So nothing in it
   may be conditional.

## Two kinds of code

| | Shell code | Island code |
| --- | --- | --- |
| runs | once, in the compiler | in the browser, as React |
| may use | HTML elements, design-system components, shell-safe reusable components, slots, segment roots, `Dynamic`, compile-time constants | everything React and `.rtsx` allow |
| may not use | anything in *Shell rules* below | props or context from the shell or from another island |

The design system is authored in plain `.tsx` for exactly this reason: the
same component is *executed* by the compiler in shell code and *rendered* by
React in island code.

**The boundary is explicit, never inferred.** Code is shell code unless it is
inside a marked island. Reasons:

- The consequences are large — pre-built HTML vs an empty root, a separate
  bundle, React on the page, no props, no context. They must not hinge on a
  hook appearing in some component three imports down.
- Inference makes the shell rules unenforceable: code that breaks them would
  silently become an island instead of being an error. With an explicit
  boundary, `useState` in shell code is a mistake the compiler can name.
- An island needs a bundle entry point; an explicit boundary is one.
- Same conclusion elsewhere: Astro `client:*`, Next.js `"use client"`, Qwik `$`.

## `Dynamic`

The marker is a wrapper, `Dynamic`, imported from the framework package and
recognised by import origin. **One `Dynamic` = one island = one React app.**
In the page it reads like any React component, and it has two forms:

```tsx
import { Dynamic } from "reactogenic";

<Dynamic><Counter start={5} /></Dynamic>   // wraps other components

<Dynamic #counter />                       // is a segment root: mounts +counter.rtsx
```

Both can be combined with everything a component allows — for instance
`<Dynamic><section #cart /></Dynamic>`: inside an island a segment root is
plain composition, so this is an island whose content is `section#cart`.

What the compiler does (logical form; details in `route-table.md`):

```tsx
// shell HTML — the host element, empty
<div id="counter"></div>
```

```tsx
// generated island entry, bundled separately
import { createRoot } from "react-dom/client";
import { Counter } from "./Counter";

createRoot(document.getElementById("counter")!).render(<Counter start={5} />);
```

- The children of `Dynamic` are lifted into the entry module together with
  the imports they use. They are written in a shell file, so they may only
  mention imports and compile-time values (S3): `start={5}` is inlined into
  the entry, nothing is serialised at runtime.
- `<Dynamic #counter />` follows *Segment roots* unchanged: `Dynamic` is a root
  that accepts `id` and `children`; the entry renders the segment's default
  export. Children next to `#counter` get the usual "contents will be
  overwritten" warning.
- A segment root **outside** any `Dynamic` is static: the compiler executes
  the `+file`, which must be shell-safe, and inlines its HTML into the shell.
- `Dynamic` inside island code is an error — an island cannot start another
  app — with one exception: inside a slot of a shell component (*Shell components*).

> OPEN: the host element. `Dynamic` has to emit one (the app needs a DOM node
> to mount into), which is the one place the HTML is not the author's own.
> Which tag (`div`? an `as` prop?), and how the inline form gets its id.

> OPEN: a file-level directive next to the wrapper (`"use dynamic"`). The
> wrapper says *where an app starts*; a directive would say *what kind of code
> a file holds*, making "may this file use hooks?" a per-file check (works in
> ESLint, no transitive analysis) — a file without it is shell-safe and may
> not render a component from a file that has it, except through `Dynamic`.
> Without the directive the same check is done by following imports from
> shell code. Not the literal `"use client"`: React and bundlers already give
> it a meaning (RSC) that Reactogenic does not have.

## Shell rules

Shell code must be **shell-safe**:

| # | Rule | Why |
| --- | --- | --- |
| S1 | **No React runtime**: no hooks, state, effects, refs, context, portals, `Suspense`, no event-handler props (`onClick={…}`) | fact 1 |
| S2 | **No variance**: no `Match`, `Switch`, `Each`, and no `?:`, `&&`, `\|\|`, `??`, `.map()` that produce elements | fact 2 |
| S3 | **Compile-time values only**: literals, module-level constants and imports of them, props handed down from shell code | fact 2 |
| S4 | **Deterministic**: no `Date`, `Math.random`, environment or I/O | the shell must be reproducible per pathname |
| S5 | **Containers are unconditional and unlooped at their position** | S2, stated for the design system: a container is always there |

Sanctioned variance, and nothing else:

- the **router** choosing a shell by pathname;
- an **optional slot** being left empty.

Escape hatch: a page that needs two layouts is two routes.

Slot params in shell code are fine: the container calls the slot function at
compile time, with compile-time args.

> OPEN: loops over constants. `<Each items={NAV_LINKS}>` is deterministic and
> could be unrolled by the compiler. Allow `Each` / `.map()` in shell code when
> `items` is a compile-time constant (S3), or keep S2 absolute?

The shell's raw JS comes from the design system's shell components — see
*Shell components*.

> OPEN: can page authors contribute shell behaviour without React, or is raw
> JS reserved to the design system?

## Island rules

| # | Rule | Why |
| --- | --- | --- |
| I1 | **An island takes nothing from the shell at runtime**: no context, and only compile-time values as props (inlined into its entry). One exception: handlers from `Form` (*Forms*) | it is a separate React app, bundled separately; the shell has no runtime values to give |
| I2 | **Islands share no React state or context** with each other | each island is a separate React root and tree |
| I3 | **Shared state has two channels**: the URL (query string and hash are in-page state), and persistent state — `usePersistentState`, `usePersistentContext`, see [persistent-state.md](persistent-state.md) | the URL is what the server and a link can see; persistent state is what the tab keeps across islands and navigations |
| I4 | **An island cannot change the shell** around it | the shell is static |
| I5 | Inside an island all of `.rtsx` is allowed: `Match`, `Switch`, `Each`, slots with runtime params | it is ordinary React |

Segment roots inside island code are plain composition — import and nest, as
in their logical desugaring. They do not start a new React app.

| Segment root inside… | |
| --- | --- |
| shell code, outside `Dynamic` | allowed; static; unconditional by S2 |
| `Dynamic` itself (`<Dynamic #cart />`) | allowed; the island |
| `Match` / `Switch` in island code | allowed |
| `Each` / `.map()` anywhere | error — the id would repeat |

## Shell components

Reactogenic ships its own design system. Some of its components are **shell
components**: React-less HTML + CSS + raw JS with an imperative API, living in
the shell. `Dialog` is one. They can be **used from any island**, with the
ordinary slot syntax — there is no new syntax here.

```tsx
// .rtsx — inside an island
<Dialog onClose={() => setOpen(false)}>
  <$Title>{title}</$Title>
  <$Contents>
    <Dynamic><CustomComponent /></Dynamic>
  </$Contents>
</Dialog>
```

Roughly: the markup becomes a `<template>` at compile time, and at runtime

```tsx
dialog.open("Dialog-a1", {
  holes: { _dynamic_b1: title, _dynamic_b2: <CustomComponent /> },
  onClose,
});
```

React never renders `Dialog`. The element in the island is a **remote
control** for DOM that belongs to the shell.

Render must stay pure (and runs twice under StrictMode), so the call is not
made during render. The compiler emits a bridge component that drives the
shell API from effects:

| In the island | Call on the shell component |
| --- | --- |
| element mounts | `dialog.open(template, holes)` |
| hole values or options change | `dialog.update(holes)` |
| element unmounts | `dialog.close()` |
| the user closes it (Esc, backdrop) | the shell calls `onClose`; the island unmounts the element |

So open/closed is ordinary island state: `<Match on={open}><Dialog …/></Match>`.

### Slot bodies are shell code, compiled ahead of time

The static HTML of a shell component can only be generated at compile time.
So a slot body of a shell component is **shell code**, even when it is written
inside an island: the compiler renders it, once, into a `<template>` for that
use site. Everything that is only known at runtime becomes a **hole** in the
template. Braces are the marker: in these slot bodies

```
{content}  ≡  <Dynamic>{content}</Dynamic>
```

so `<Dynamic>` only has to be written around JSX elements.

```tsx
// .rtsx — inside an island
<Dialog>
  <$Contents>
    Are you sure you want to remove user {userToRemove}
  </$Contents>
</Dialog>
```

```html
<!-- compile time: literal HTML in the page's shell -->
<template id="Dialog-a1">
  <dialog>
    <div data-slot="$Contents">Are you sure you want to remove user <span id="_dynamic_b2"></span></div>
  </dialog>
</template>
```

| In a slot body | Becomes |
| --- | --- |
| text, shell-safe JSX | literal HTML in the template |
| `{expr}` whose value is known at compile time | literal HTML too — the hole is optimised away; same result |
| `{expr}` of a primitive type | a hole; the bridge sets it as **text** on `open` and on every change. **No React** |
| `{expr}` of any other type (`{cond ? <b>x</b> : null}`), or an explicit `<Dynamic>` around JSX | a hole that React renders into |
| a JSX element that is not shell-safe, **outside** `Dynamic` (`<CustomComponent />`) | error — shell-slot-element: wrap it in `<Dynamic>` |

React is needed only when a hole holds JSX. The compiler is type-aware, so it
tells a primitive hole from a JSX hole by the type of the expression.

The shorthand is for braces only. An *element* is never made dynamic
implicitly: deciding that from what the component uses would be the
inference the explicit boundary exists to avoid.

The equivalence holds only here. In page-level shell code there is no island
to supply a value, so a runtime `{expr}` stays shell-dynamic-value.

Attributes on the shell component itself are not markup: runtime data and
callbacks (`onClose={…}`) are passed to `open` as they are.

At runtime the island only supplies what goes into the holes:

```tsx
// island side, roughly — driven from effects by the bridge component
dialog.open("Dialog-a1", { holes: { _dynamic_b2: userToRemove }, onClose });
```

- In which form the shell component itself reaches the page: *Delivery* below.
- In island code a layout component of the design system is rendered by
  React as usual. Only shell components are driven remotely.

### Delivery

A shell component is three artifacts, like the rest of the shell:

| Artifact | Shipped as |
| --- | --- |
| HTML | a **`<template>` element in the page's shell**, one per **use site**: the component's own markup with the slot bodies already rendered in — `<template id="Dialog-a1">…</template>` at the end of `<body>` |
| CSS | part of the per-route CSS |
| JS (`open` / `update` / `close`) | raw JS in the shared runtime, React-free |

So yes, a `DocumentFragment` — but the browser builds it, not our JS:
`template.content` *is* an inert fragment, parsed with the document, costing
nothing until used (no scripts run, no images load, no styles apply).

```js
// dialog.open(slots), roughly
const node = document.getElementById("Dialog-a1").content.cloneNode(true).firstElementChild;
fill(node, holes);              // primitive hole → textContent; JSX hole → React renders there
document.body.append(node);
node.showModal();
```

- **A fresh clone per `open`**, removed on `close`: no state left over from
  the previous use, and two instances can be open at once (a confirm inside a
  dialog).
- Holes are looked up **inside the clone** (`node.querySelector`), so the
  same hole id in two open clones does not clash.
- The HTML stays a compile-time artifact — plain, precise, inspectable in
  view-source — and the compiler knows which templates a page needs from its
  islands' imports (`route-table.md`).
- Ids inside a template would repeat across clones; `open` generates them
  per instance (needed for `aria-labelledby`).

**No `<script>` inside the template.** It would work natively — a script in
template content is inert, and runs when a clone is inserted into the
document — but with the wrong semantics:

| Native behaviour | Problem |
| --- | --- |
| runs again on **every** clone | top-level `const` / `let` in a classic script throw "already declared" on the second open |
| classic scripts run in the global scope; the only link to their instance is `document.currentScript` | no instance state, no clean `update` / `close` |
| module and `src` scripts run **asynchronously** after insertion | the instance is not ready when `open()` returns |
| inline script | needs a CSP hash or a per-response nonce — and the shell is static; not bundled, minified or cached with the rest |

Instead the template is markup only, and the behaviour is a function in the
runtime, instantiated per clone — the small piece of JS "magic" on top:

```js
// runtime, one module per shell component — bundled, cached, CSP-clean
export function mountDialog(root, slots) {        // root: the fresh clone
  root.querySelector("[data-close]").addEventListener("click", () => api.close());
  const api = {
    update(next) { fill(root, next); },
    close() { root.close(); root.remove(); slots.onClose?.(); },
  };
  fill(root, slots);
  return api;
}
```

`open()` is then: clone the template, append it, call `mountDialog`, keep the
returned handle for `update` / `close`. Loaded once, run per instance, ready
synchronously.

Considered and not chosen:

| Alternative | Why not |
| --- | --- |
| HTML built in JS (strings, `createContextualFragment`, `createElement`) | the markup moves into the JS bundle: bigger JS, parsed on the main thread at open time, no longer a plain HTML artifact |
| one live, hidden `<dialog>` in the shell | single instance, and it keeps the previous use's state |
| fetch the fragment on first open | cacheable across routes, but adds latency to the first open; possible later as an optimisation for rarely used, heavy shells |
| custom elements + declarative shadow DOM | native `<slot>` would map onto `$Title` nicely, but shadow roots cut the component off from the per-route CSS |

> OPEN: confirm `<template>` delivery.

> OPEN: how the design system declares a component as a shell component, and
> the API it must implement (`open` / `update` / `close`). Belongs in
> `slot-contract.md`. Framework-only for now; third-party shell components
> are not planned.

> OPEN: is a **JSX hole** — a `Dynamic` around JSX in a shell-component slot —
> a **separate root**, or a **portal** of the island that opened it?
> (Primitive holes need no React, so the question does not arise for them.)
>
> Booting a root is not the concern: React is already on the page, and
> `createRoot` costs well under a millisecond (a fiber root, plus React's
> event listeners attached to the container) and a few KB. The costs of a
> separate root are semantic:
>
> - **no runtime values** can cross (dynamic-props), so dialog contents cannot
>   simply use the state of the island that opened the dialog — unless the
>   parent drives the child root by calling `root.render` again on every change;
> - the first render of a new root is **scheduled**, so the dialog opens
>   empty and fills in a tick later (avoidable only with `flushSync`);
> - no context from the opener; a separate entry and chunk per `Dynamic`.
>
> `createPortal` into the slot's host element has none of these: no new root,
> same commit as the opener, runtime props and context flow, events bubble
> through the React tree. It reads as "this is still the island that opened
> the dialog, rendering into DOM the shell owns".
>
> Recommended: a **top-level** `Dynamic` (in shell code) is a separate root —
> one island, one app. A `Dynamic` **nested** in a shell-component slot is a
> portal of its opener, and may take runtime values. The wrapper stays in
> the source either way: it marks where React starts inside shell-owned DOM.

> OPEN: a shell component used directly in **shell code** (a static dialog in
> a page) — who opens it, with no island around?

> OPEN: mount/unmount as open/close is inferred from the sketch. The
> alternative is an explicit `open` prop on an always-mounted element.

## Forms

`Form` is compiled: the compiler sees every input and every validation rule,
links them to the form, and can derive from them everything that needs the
**closed set of fields** — the shell's validation JS, the typed payload, the
server-side check. A `Dynamic` inside breaks exactly that: what it renders is
only known at runtime.

```tsx
<Form>
  <Input name="email" required />
  <Dynamic><Input name="nickname" /></Dynamic>   // error: form-dynamic-field
</Form>
```

So there is no hybrid form. There is `Form`, and there is what React already has:

| | `Form` | a React form in an island |
| --- | --- | --- |
| what it is | shell component: rendered fully statically, **no React**, native submission | today's React `<form>`, nothing more; island code only |
| fields | closed set, known at compile time | whatever React renders: conditional sections, repeating groups |
| inputs and validation | linked by the compiler | the framework does nothing; entirely the author's business |
| may contain `Dynamic` | yes, as long as the fields stay static (below) | it already *is* dynamic |

There is no `DynamicForm` component to specify: the framework neither links
nor controls a React form.

The rule is "**the fields of a `Form` are static**", not "no `Dynamic` in a
`Form`". The compiler rejects the inputs it can see inside a `Dynamic`
(design-system inputs, recognised by import origin). An input it cannot see —
rendered deep inside a custom component — still lands in the form's DOM and
would be submitted; the server drops fields the form did not declare, with a
warning in development.

### `$Field`: the bridge to every input

A field is a slot of `Form`. Its options declare the field; its params are the
bridge between the form's store and whatever renders the input.

```tsx
// .rtsx
<Form>
  <$Field name="age" type="number" validation { value, onChange, invalid, errors, nativeType }>
    <Input type={nativeType} value onChange invalid>
      <Match on={invalid}>
        <$Hint>{renderErrors(errors)}</$Hint>
      </Match>
    </Input>
  </$Field>
</Form>
```

| Options (in) | |
| --- | --- |
| `name` | the field; part of the form's closed set |
| `type` | the field's value type (`"number"`), not the HTML input type |
| `validation` | the rules — here *Shorthand props*: a `validation` variable in scope |

| Params (out) | |
| --- | --- |
| `value`, `onChange` | the field's value in the form's store, and its setter |
| `invalid`, `errors` | validity from the store |
| `nativeType` | the HTML input type the form chose for `type` |

`<Input type={nativeType} value onChange invalid>` is three shorthand props:
the params line up with the input's prop names on purpose.

- `$Field` is filled once per field, so it is a **list slot**
  ([syntax.md](syntax.md#list-slots)): `Form` declares `$Field: {…}[]`.
- Two fields with the same `name` → form-duplicate-field.

- `<Match on={invalid}><$Hint>…` is a conditional slot
  ([syntax.md](syntax.md#conditional-slots)). It tests `invalid`, not
  `errors`: an empty array is truthy.

> OPEN (blocking): **where does a `$Field` body run?** `value`, `invalid`
> and `errors` change at runtime, so the body as written is not static HTML,
> while `Form` "renders fully statically without React".
>
> 1. *In an island* there is nothing to solve: `Form` is a React component
>    over its store and the params are ordinary runtime slot params (the
>    pattern of TanStack Form's `form.Field`, react-hook-form's `Controller`).
> 2. *In shell code*, either
>    - **restricted bodies** (recommended): params may flow only into props of
>      design-system inputs — which the compiler knows how to bind to the
>      store in raw JS — and into a `Dynamic`. `{renderErrors(errors)}` or a
>      `Match` on a param is an error there; error text is the form's own
>      business, or
>    - a **reactive-template compiler**: arbitrary bodies compiled to raw JS
>      bindings over the store (what Solid and Svelte do). Powerful, and a
>      second compilation target to build and maintain.

### A dynamic widget for a static field

A date picker, a combobox, a rich-text editor: the **field** (name, rules) is
known at compile time, only its **UI** needs React. `$Field` (above) declares
the field statically and hands the island a way to report its value:

```tsx
// .rtsx
<Form>
  <$Field name="birthday" required { onChange }>
    <Dynamic><DatePicker onChange /></Dynamic>
  </$Field>
</Form>
```

- `$Field` is part of the static form: the compiler sees `name="birthday"` and
  `required`, so the set of fields stays closed and everything derived from it
  still holds. In the shell it renders the field's markup, a shell-owned
  carrier for the value, and the `Dynamic` host.
- `{ onChange }` are slot params; `<DatePicker onChange />` is
  *Shorthand props*. No new syntax.
- `onChange(value)` writes the value into the form's field and runs the
  shell's validation for it. The island never touches the form.

**`Form` is the exception to dynamic-props.** Normally nothing from the shell's
runtime may cross into a `Dynamic`. `Form` may pass handlers in, because it
has **its own state management**: a React-free store in the shell's runtime
that owns the values, validity and submission of the form. A handler handed
out by `$Field` is just a reference into that store, and the compiler knows
which one at compile time — form, field — so it can write it into the
island's entry:

```tsx
// generated island entry, roughly
render(<DatePicker onChange={formStore("Form-a1").field("birthday").set} />);
```

The exception is `Form`'s alone. No other shell component hands handlers to
an island; islands reach everything else through
[persistent state](persistent-state.md) or the URL.

- TS7 checks the fit: `DatePicker`'s `onChange` must accept what `$Field`
  hands out, `(value: string) => void`.
- Native constraint validation skips hidden inputs, so the rules of such a
  field are enforced by the shell's validation JS, not by the browser.
- Until the island has mounted, the field is empty; a `required` field
  therefore blocks submission, which is the safe direction.

> OPEN: is `Form`'s store built on the same reactive library as
> [persistent state](persistent-state.md)? It would let an island *read* form
> state (`useSyncExternalStore`) with no further machinery.

> OPEN: `id` among the params, so that the shell's `<label for>` reaches a
> dynamic widget.

> OPEN: the value type. `string` matches what a form submits; multi-value
> widgets and files need more.

## Enforcement

Every rule is reported three ways: compiler error, language-service
diagnostic (as you type), ESLint rule (for CI without a build).

| Code | Message | Rule |
| --- | --- | --- |
| shell-react | The shell cannot use React: `useState` | S1 |
| shell-handler | The shell cannot handle events; move this into an island | S1 |
| shell-conditional | The shell cannot be conditional; make it two routes or move it into an island | S2, S5 |
| shell-loop | The shell cannot loop | S2, S5 |
| shell-dynamic-value | `user` is not known at compile time | S3 |
| shell-nondeterministic | `Date.now()` makes the shell irreproducible | S4 |
| dynamic-props | `user` is not known at compile time and cannot cross into `Dynamic` | I1, S3 |
| dynamic-nested | An island cannot start another app | `Dynamic` inside island code, outside a shell component's slot |
| form-dynamic-field | The fields of a `Form` are static | an input inside a `Dynamic` that is not the body of a `$Field` |
| form-duplicate-field | `age` is already a field of this form | forms |
| shell-slot-element | A shell component cannot render React elements; wrap this in `<Dynamic>` | shell components |
| shell-dynamic-code | `Counter` uses React; wrap it in `<Dynamic>` | S1, reported at the use site in shell code |
| segment-in-loop | `#about-us` would be mounted more than once | segment roots |

The language service also shows, inside a segment file, whether it is mounted
as an island or compiled into the shell.
