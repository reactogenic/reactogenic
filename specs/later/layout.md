# Rules of layout

Not syntax: a discipline the compiler enforces on where `.rtsx` constructs may
appear. It follows from how a Reactogenic page is built and served.

## The model

Reactogenic is a compiled framework.

| | Compile time | Runtime |
| --- | --- | --- |
| **Shell** | compiled **once** per pathname into plain, precise HTML + CSS + raw JS. Layout components are executed by the compiler and leave no React behind | served as static resources |
| **Dynamic segment** | its root element is emitted into the shell **empty**; the dynamic segment is bundled separately | React + one JS bundle per dynamic segment; each dynamic segment is mounted as a **separate React app** |

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

| | Shell code | Dynamic segment code |
| --- | --- | --- |
| runs | once, in the compiler | in the browser, as React |
| may use | HTML elements, design-system components, shell-safe reusable components, slots, segment roots, `Dynamic`, compile-time constants | everything React and `.rtsx` allow |
| may not use | anything in *Shell rules* below | props or context from the shell or from another dynamic segment |

The design system is authored in plain `.tsx` for exactly this reason: the
same component is *executed* by the compiler in shell code and *rendered* by
React in dynamic segment code.

> Phase 2: `@reactogenic/ui` is authored in `.rtsx` — plain `.tsx` once
> phase 1's transpiler has run, which is what the builder executes
> ([phase02/decisions.md](../phase02/decisions.md), 5: **for review**).

**The boundary is explicit, never inferred.** Code is shell code unless it is
inside a marked dynamic segment. Reasons:

- The consequences are large — pre-built HTML vs an empty root, a separate
  bundle, React on the page, no props, no context. They must not hinge on a
  hook appearing in some component three imports down.
- Inference makes the shell rules unenforceable: code that breaks them would
  silently become a dynamic segment instead of being an error. With an explicit
  boundary, `useState` in shell code is a mistake the compiler can name.
- A dynamic segment needs a bundle entry point; an explicit boundary is one.
- Same conclusion elsewhere: Astro `client:*`, Next.js `"use client"`, Qwik `$`.

## `Dynamic`

The marker is a wrapper, `Dynamic`, imported from the framework package and
recognised by import origin. **One `Dynamic` = one dynamic segment = one React app.**
In the page it reads like any React component, and it has two forms:

```tsx
import { Dynamic } from "reactogenic";

<Dynamic><Counter start={5} /></Dynamic>   // wraps other components

<Dynamic #counter />                       // is a segment root: mounts +counter.rtsx
```

> Note: there is no package `"reactogenic"`. Phase 1's runtime is
> `@reactogenic/core`; which package exports `Dynamic` is settled with
> dynamic segments.

Both can be combined with everything a component allows — for instance
`<Dynamic><section #cart /></Dynamic>`: inside a dynamic segment a segment root is
plain composition, so this is a dynamic segment whose content is `section#cart`.

What the compiler does (logical form; details in `route-table.md`):

```tsx
// shell HTML — the host element, empty
<div id="counter"></div>
```

```tsx
// generated dynamic segment entry, bundled separately
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
- `Dynamic` inside dynamic segment code is an error — a dynamic segment cannot start another
  app — with one exception: inside a slot of a shell component, where it is a
  **portal** of the dynamic segment, not a new app (*Shell components*).

> OPEN: the host element. `Dynamic` has to emit one (the app needs a DOM node
> to mount into), which is the one place the HTML is not the author's own.
> Which tag (`div`? an `as` prop?), and how the inline form gets its id.

**Rejected:** a file-level directive (`"use dynamic"` / `"use client"`). The
wrapper is the only marker. Whether a file may use hooks follows from who
renders it: the compiler follows imports from shell code and reports
shell-react at the use site (shell-dynamic-code). No per-file annotation.

## Shell rules

> The owner's rule since 2026-10-05: **shell output has no browser-time
> variance; every possible shell variant is materialized at build time; the
> server's router may select between prebuilt variants and neither renders
> nor changes them** ([phase02/builder.md](../phase02/builder.md),
> *Variants*). So S2 reads "no **runtime** variance": the builder executes
> shell code once per variant, and what that computes — a loop over a
> constant, a conditional on the pathname — is in the output as plain HTML.
> S3 reads "whatever execution yields", S4 is enforced by the engine. S1
> admitting context (resolved while the page executes) is still the owner's
> to rule on ([phase02/decisions.md](../phase02/decisions.md), 4: **for
> review**). The table below is the original statement.

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
>
> Phase 2 allows them: the page is executed, so the loop is a compile-time
> value (decisions.md, 4: **for review**). Not what it produces as *slot*
> elements: `Each` around `<$Item>` is still orphan-slot (decisions.md, *For
> the owner*, A).

The shell's raw JS comes from the design system's shell components — see
*Shell components*.

> OPEN: can page authors contribute shell behaviour without React, or is raw
> JS reserved to the design system?
>
> Phase 2 does not answer it. A behaviour is a module a component `mount()`s
> (builder.md, *Behaviours*), and the design system's are the only ones
> specified; a page's own `<script>` is built as written and turns that
> page's CSS pruning off (decisions.md, 19; *For the owner*, E).

## Dynamic segment rules

| # | Rule | Why |
| --- | --- | --- |
| I1 | **A dynamic segment takes nothing from the shell at runtime**: no context, and only compile-time values as props (inlined into its entry). One exception: handlers from `Form` (*Forms*) | it is a separate React app, bundled separately; the shell has no runtime values to give |
| I2 | **Dynamic segments share no React state or context** with each other | each dynamic segment is a separate React root and tree |
| I3 | **Shared state has two channels**: the URL (query string and hash are in-page state), and persistent state — `usePersistentState`, `usePersistentContext`, see [persistent-state.md](persistent-state.md) | the URL is what the server and a link can see; persistent state is what the tab keeps across dynamic segments and navigations |
| I4 | **A dynamic segment cannot change the shell** around it | the shell is static |
| I5 | Inside a dynamic segment all of `.rtsx` is allowed: `Match`, `Switch`, `Each`, slots with runtime params | it is ordinary React |

Segment roots inside dynamic segment code are plain composition — import and nest, as
in their logical desugaring. They do not start a new React app.

| Segment root inside… | |
| --- | --- |
| shell code, outside `Dynamic` | allowed; static; unconditional by S2 |
| `Dynamic` itself (`<Dynamic #cart />`) | allowed; the dynamic segment |
| `Match` / `Switch` in dynamic segment code | allowed |
| `Each` / `.map()` anywhere | error — the id would repeat |

## Shell components

Reactogenic ships its own design system. Some of its components are **shell
components**: React-less HTML + CSS + raw JS with an imperative API, living in
the shell. `Dialog` is one. They can be **used from any dynamic segment**, with the
ordinary slot syntax — there is no new syntax here.

```tsx
// .rtsx — inside a dynamic segment
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

React never renders `Dialog`. The element in the dynamic segment is a **remote
control** for DOM that belongs to the shell.

> OPEN (for the owner, [phase02/decisions.md](../phase02/decisions.md), B):
> phase 2's `Dialog` takes its body as `children`
> ([phase02/components.md](../phase02/components.md), *Dialog*), not as
> `<$Contents>`, and holes are defined below for slot bodies only.
> Recommended there: `children` stays, and is a body with holes too when
> dynamic segments arrive — this section is amended then.

Render must stay pure (and runs twice under StrictMode), so the call is not
made during render. The compiler emits a bridge component that drives the
shell API from effects:

| In the dynamic segment | Call on the shell component |
| --- | --- |
| element mounts | `dialog.open(template, holes)` |
| hole values or options change | `dialog.update(holes)` |
| element unmounts | `dialog.close()` |
| the user closes it (Esc, backdrop) | the shell calls `onClose`; the dynamic segment unmounts the element |

So open/closed is ordinary dynamic segment state: `<Match on={open}><Dialog …/></Match>`.

### Slot bodies are shell code, compiled ahead of time

The static HTML of a shell component can only be generated at compile time.
So a slot body of a shell component is **shell code**, even when it is written
inside a dynamic segment: the compiler renders it, once, into a `<template>` for that
use site. Everything that is only known at runtime becomes a **hole** in the
template. Braces are the marker: in these slot bodies

```
{content}  ≡  <Dynamic>{content}</Dynamic>
```

so `<Dynamic>` only has to be written around JSX elements.

```tsx
// .rtsx — inside a dynamic segment
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

The equivalence holds only here. In page-level shell code there is no dynamic segment
to supply a value, so a runtime `{expr}` stays shell-dynamic-value.

Attributes on the shell component itself are not markup: runtime data and
callbacks (`onClose={…}`) are passed to `open` as they are.

At runtime the dynamic segment only supplies what goes into the holes:

```tsx
// dynamic segment side, roughly — driven from effects by the bridge component
dialog.open("Dialog-a1", { holes: { _dynamic_b2: userToRemove }, onClose });
```

- In which form the shell component itself reaches the page: *Delivery* below.
- In dynamic segment code a layout component of the design system is rendered by
  React as usual. Only shell components are driven remotely.

### Delivery

A shell component is three artifacts, like the rest of the shell:

| Artifact | Shipped as |
| --- | --- |
| HTML | a **`<template>` element in the page's shell**, one per **use site**: the component's own markup with the slot bodies already rendered in — `<template id="Dialog-a1">…</template>` at the end of `<body>` |
| CSS | part of the per-route CSS |
| JS (`open` / `update` / `close`) | raw JS in the shared runtime, React-free |

> Phase 2 has no shared runtime: a page's JS is the behaviours its
> components mounted, built per page (builder.md, *Behaviours*). Of that,
> the `(root)` signature of `mountDialog` below carries over to dynamic segments; the
> entry that mounts by id at load, a behaviour that returns nothing and the
> record of execution do not (decisions.md, *For the owner*, C).

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
  dynamic segments' imports (`route-table.md`).
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

**JSX holes are portals or roots depending on where the shell component is
used.** A `Dynamic` around JSX in a shell-component slot is:

| The shell component is used from… | The JSX hole is… |
| --- | --- |
| a dynamic segment (`Dialog` opened by dynamic segment code) | a **portal** of that dynamic segment: `createPortal` into the hole's element. No new root, same commit as the opener, runtime props and context flow, events bubble through the React tree. It is still the dynamic segment that opened the dialog, rendering into DOM the shell owns |
| the base layout (shell code) | a **root** — there is no React app around it to portal from. One dynamic segment, one app, as for any top-level `Dynamic` |

The wrapper is written the same way in both cases: it marks where React
starts inside shell-owned DOM. Primitive holes need no React either way.

A portal hole may therefore take **runtime values** from its opener — the
dynamic-props rule applies to roots, not portals.

> OPEN: a shell component used directly in **shell code** (a static dialog in
> a page) — who opens it, with no dynamic segment around?
>
> Phase 2: nobody has to. It is a live `<dialog>` in the page, opened by a
> button's `command` / `commandfor` — no JS at the browser floor, no holes —
> which is the alternative listed above as not chosen ("one live, hidden
> `<dialog>`"). `<template>` + clone stays for a dialog with holes
> (components.md, *Dialog*; decisions.md, 17: **for review**).

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

| | `Form` | a React form in a dynamic segment |
| --- | --- | --- |
| what it is | shell component: rendered fully statically, **no React**, native submission | today's React `<form>`, nothing more; dynamic segment code only |
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
  ([syntax.md](../phase01/syntax.md#list-slots)): `Form` declares `$Field: {…}[]`.

  > OPEN: syntax.md has no *List slots*: a slot typed as an array is an
  > error there (slot-list), and many values of one kind are a `KeyedSlot`
  > (syntax.md, *Keyed slots*; phase02/components.md, rule 4). So `$Field:
  > KeyedSlot<…>` — and is the entry's `key` the field's `name`, or written
  > beside it?

- Two fields with the same `name` → form-duplicate-field.

- `<Match on={invalid}><$Hint>…` is a conditional slot
  ([syntax.md](../phase01/syntax.md#conditional-slots)). It tests `invalid`, not
  `errors`: an empty array is truthy.

> OPEN (blocking): **where does a `$Field` body run?** `value`, `invalid`
> and `errors` change at runtime, so the body as written is not static HTML,
> while `Form` "renders fully statically without React".
>
> 1. *In a dynamic segment* there is nothing to solve: `Form` is a React component
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
the field statically and hands the dynamic segment a way to report its value:

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
  shell's validation for it. The dynamic segment never touches the form.

**`Form` is the exception to dynamic-props.** Normally nothing from the shell's
runtime may cross into a `Dynamic`. `Form` may pass handlers in, because it
has **its own state management**: a React-free store in the shell's runtime
that owns the values, validity and submission of the form. A handler handed
out by `$Field` is just a reference into that store, and the compiler knows
which one at compile time — form, field — so it can write it into the
dynamic segment's entry:

```tsx
// generated dynamic segment entry, roughly
render(<DatePicker onChange={formStore("Form-a1").field("birthday").set} />);
```

The exception is `Form`'s alone. No other shell component hands handlers to
a dynamic segment; dynamic segments reach everything else through
[persistent state](persistent-state.md) or the URL.

- TS7 checks the fit: `DatePicker`'s `onChange` must accept what `$Field`
  hands out, `(value: string) => void`.
- Native constraint validation skips hidden inputs, so the rules of such a
  field are enforced by the shell's validation JS, not by the browser.
- Until the dynamic segment has mounted, the field is empty; a `required` field
  therefore blocks submission, which is the safe direction.

> OPEN: is `Form`'s store built on the same reactive library as
> [persistent state](persistent-state.md)? It would let a dynamic segment *read* form
> state (`useSyncExternalStore`) with no further machinery.

> OPEN: `id` among the params, so that the shell's `<label for>` reaches a
> dynamic widget.

> OPEN: the value type. `string` matches what a form submits; multi-value
> widgets and files need more.

## Enforcement

Every rule is reported three ways: compiler error, language-service
diagnostic (as you type), ESLint rule (for CI without a build).

> Phase 2 reports shell-react, shell-handler and shell-nondeterministic —
> and shell-error, for whatever else a page throws — from `build` alone:
> each is found by executing the page, so `check` and the editor do not see
> them (builder.md, *Not in phase 2*). shell-conditional, shell-loop and
> shell-dynamic-value have nothing to report under its reading of S2 and S3.

| Code | Message | Rule |
| --- | --- | --- |
| shell-react | The shell cannot use React: `useState` | S1 |
| shell-handler | The shell cannot handle events; move this into a dynamic segment | S1 |
| shell-conditional | The shell cannot be conditional; make it two routes or move it into a dynamic segment | S2, S5 |
| shell-loop | The shell cannot loop | S2, S5 |
| shell-dynamic-value | `user` is not known at compile time | S3 |
| shell-nondeterministic | `Date.now()` makes the shell irreproducible | S4 |
| dynamic-props | `user` is not known at compile time and cannot cross into `Dynamic` | I1, S3 |
| dynamic-nested | A dynamic segment cannot start another app | `Dynamic` inside dynamic segment code, outside a shell component's slot (where it is a portal) |
| form-dynamic-field | The fields of a `Form` are static | an input inside a `Dynamic` that is not the body of a `$Field` |
| form-duplicate-field | `age` is already a field of this form | forms |
| shell-slot-element | A shell component cannot render React elements; wrap this in `<Dynamic>` | shell components |
| shell-dynamic-code | `Counter` uses React; wrap it in `<Dynamic>` | S1, reported at the use site in shell code |
| segment-in-loop | `#about-us` would be mounted more than once | segment roots |

The language service shows, inside a segment file, whether it is mounted as
a dynamic segment or compiled into the shell — there is no directive to say so.
