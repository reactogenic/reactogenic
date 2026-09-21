# Persistent state: communication between islands

Every `Dynamic` island is a separate React root and tree
(see [layout.md](layout.md)). React state and React context stop at the root,
so two islands cannot share either. Reactogenic therefore ships its own layer.

It solves a second problem with the same mechanism: navigation is server-side,
every navigation is a new document, and all React state dies with the old one.
Persistent state survives it.

| Hook | Replaces | Shared across islands | Survives navigation |
| --- | --- | --- | --- |
| `usePersistentState` | `useState` | ✓ | ✓ |
| `usePersistentContext` | `useContext` | ✓ | ✓ |

## Architecture

Three layers, each usable without the one above it:

| Layer | Role |
| --- | --- |
| **`sessionStorage`** | persistence: per tab, survives navigation and reload, gone when the tab closes |
| **reactive store** (Reactogenic's own subscription library, no React in it) | the source of truth within a document: an in-memory cache of parsed values plus subscriptions per key |
| **`useSyncExternalStore`** | binds a key of the store to a React component, in any island |

```
island A ─┐                                   ┌─ sessionStorage  (next document reads it)
island B ─┼─ useSyncExternalStore ─ store ────┤
shell JS ─┘   (subscribe / getSnapshot)       └─ subscribers     (every island, same tick)
```

- **One store per document.** It ships in the shared runtime chunk, next to
  React, never inside an island's own bundle — otherwise each island would
  get a private copy and nothing would be shared.
- **Read path.** On first access of a key the store parses the
  `sessionStorage` entry once and caches the value. `getSnapshot` returns the
  cached reference, so it is stable between changes, as
  `useSyncExternalStore` requires.
- **Write path.** `set` replaces the cached value, notifies the key's
  subscribers synchronously, and writes the JSON to `sessionStorage`. Memory
  is the truth; storage is write-behind.
- **No hydration mismatch.** Island roots are empty in the shell and rendered
  on the client only, so the first render may read storage directly; no
  server snapshot is needed.
- The store has no React in it, so the shell's raw JS can subscribe to the
  same keys.

## `usePersistentState`

```tsx
// state/cart.ts — imported by every island that needs it
import { persistentState } from "reactogenic";

export const cartState = persistentState<CartItem[]>("cart", []);
```

```tsx
// +cart-button.rtsx — island A
const [cart, setCart] = usePersistentState(cartState);
<Button onClick={() => setCart((items) => [...items, item])}>Add</Button>
```

```tsx
// +cart-badge.rtsx — island B, another React root
const [cart] = usePersistentState(cartState);
<Badge count={cart.length} />
```

- Same return shape as `useState`; the setter takes a value or an updater.
- Island B re-renders in the same tick as island A's `setCart`, and both show
  the same cart after the next navigation.
- The value must be JSON-serialisable; the type parameter should say so.

> OPEN: handle or string key? Above, the key and its type are declared once
> (`persistentState<T>(key, initial)`) and islands share the **handle**, so
> two islands cannot disagree about the type. The alternative,
> `usePersistentState<T>("cart", [])` at every use site, is shorter and lets
> them disagree silently. Recommended: handle.

## `usePersistentContext`

Context without a tree: a named value with **one provider and many readers**.

```tsx
// state/theme.ts
export const ThemeContext = persistentContext<Theme>("theme", "light");
```

```tsx
// island A — provides
<PersistentProvider context={ThemeContext} value={theme}>…</PersistentProvider>
```

```tsx
// island B — reads; no setter
const theme = usePersistentContext(ThemeContext);
```

- A reader gets the provider's value from whichever island provides it; the
  default when nobody does (or has, in this tab).
- There is no tree between islands, so there is **no scoping**: one value per
  context per document. Two providers of one context is an error.
- Inside the providing island it also behaves as ordinary React context.

> OPEN: this section is inferred from the name. Is "one writer, many readers,
> no scoping" the intended difference from `usePersistentState`, or is
> `usePersistentContext` meant to be something else (e.g. a bag of several
> persistent values)?

## Which state goes where

| State | Home | Why |
| --- | --- | --- |
| belongs in a link: filters, tab, page, selected item | URL — query string and hash | shareable, bookmarkable, visible to the server |
| belongs to the visit: cart draft, dismissed banners, wizard progress, theme | persistent state | per tab, survives navigation, invisible to the server |
| belongs to one component: open/closed, hover, input draft | `useState` | dies with the document, as it should |
| comes from the server | a resource (see `resource.md`) | not client state at all |

## Edge cases

- **Tabs are isolated.** `sessionStorage` is per tab; there is no cross-tab
  sync, and none is wanted. A duplicated tab starts with a copy.
- **Storage unavailable or full** (private mode, quota): the store keeps
  working in memory; the state is shared between islands but no longer
  survives navigation. One console warning, no throw.
- **Corrupt or unparsable entry** → treated as absent; the initial value is used.
- **Not for secrets**: any script on the origin can read `sessionStorage`.
- **Keys are global per origin.** Declaring the same key twice with
  `persistentState` is a compile error where the compiler can see both.

> OPEN: stale shapes. A tab can outlive a deploy, so a stored value may have
> yesterday's shape. Options: a `version` on the handle (mismatch → initial
> value), or a `validate` function.

> OPEN: selectors — `usePersistentState(cartState, (c) => c.length)` to
> re-render only when the selected part changes.
