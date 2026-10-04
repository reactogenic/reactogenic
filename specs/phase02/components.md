# Layout components: `SideMenu`, `Dialog`, `DropdownMenu`

`@reactogenic/ui` — the first components of the design system. Authored in
`.rtsx` with typed slots; executed by the builder in shell code
([builder.md](builder.md)); no new syntax. Evidence for every choice here:
[research/components.md](research/components.md),
[research/platform.md](research/platform.md).

Rules shared by all three:

1. **Platform first.** Each component is the platform's own element: state is
   the platform's (`dialog[open]`, `:popover-open`, `details[open]`), and
   opening, closing, focus and placement are the browser's. JS is added per
   feature, by the use site (*Behaviours*).
2. **Behaviour is declarative.** `href`, `command` + `commandfor`,
   `popovertarget`. No handlers.
3. **Option or content.** An option selects among variants of the emitted
   HTML / CSS / JS, so it is a literal of a closed union (`align="end"`).
   Content flows into the HTML (labels, slot bodies).
4. **Repeated things are a `KeyedSlot`**, rendered in the order written.
5. **A container takes items or sections, never both at one level**: slot
   elements become one prop per slot name, so the order *between* two slot
   names is lost.

Browser floor: **Chrome/Edge 135, Firefox 147, Safari 26.2**. Below it
nothing breaks hard: a dialog still opens (the `invokers` behaviour), a menu
opens unanchored.

`Button` is the fourth export: `<button>` — or `<a>` when it has `href` —
with `variant?: "solid" | "ghost"`; it passes `command`, `commandfor`,
`popovertarget` through.

## `Dialog`

```tsx
interface DialogProps {
  id?: string;                                  // a name for a button elsewhere; generated when absent
  closedby?: "any" | "closerequest" | "none";   // option; HTML's attribute and meaning; default "any"
  $Trigger?: Slot<ButtonProps>;                 // a button rendered where the dialog is written
  $Title: Slot<ComponentProps<"h2">>;           // required: the accessible name
  $Action?: KeyedSlot<ButtonProps>;             // footer buttons, in order
  children?: ReactNode;                         // the body
}
```

```tsx
// .rtsx
<Dialog closedby="closerequest">
  <$Trigger>Delete project…</$Trigger>
  <$Title>Delete the project?</$Title>
  This cannot be undone.
  <$Action key="cancel" variant="ghost">Cancel</$Action>
  <$Action key="delete" href="/">Delete</$Action>
</Dialog>
```

```html
<!-- emitted, in place -->
<button type="button" class="rg-button" command="show-modal" commandfor="d1">Delete project…</button>
<dialog id="d1" class="rg-dialog" closedby="closerequest" aria-labelledby="d1-t">
  <div data-part="panel">
    <header>
      <h2 id="d1-t">Delete the project?</h2>
      <button type="button" data-part="close" command="close" commandfor="d1" aria-label="Close">✕</button>
    </header>
    <div data-part="body">This cannot be undone.</div>
    <footer>
      <button type="button" class="rg-button" data-variant="ghost" command="close" commandfor="d1">Cancel</button>
      <a class="rg-button" href="/">Delete</a>
    </footer>
  </div>
</dialog>
```

| Part | |
| --- | --- |
| identity | `$Trigger`: the compiler names the dialog (`useShellId("d")`). A button elsewhere: the author's `id` — or a segment root, `<Dialog #install>` — and `<Button command="show-modal" commandfor="install">`; the builder checks the reference (idref-not-found, command-target) |
| `$Action` | with `href` a link; otherwise it closes the dialog |
| `<footer>` | only when `$Action` is filled |
| light dismiss (`closedby="any"`) | a **scrim**: `<form method="dialog">` holding one full-viewport button, emitted **after** the panel — `closedby` itself is not in Safari. The close button in the header is the first focusable element, so focus never lands on the scrim |
| delivery | a live `<dialog>` where it is written; the top layer makes its position irrelevant. (layout.md's `<template>` + clone is for dialogs with holes, opened from islands.) |
| CSS | surface, `::backdrop`, header, body, footer, entry transition (`@starting-style`, `allow-discrete`), scroll lock: `:root:has(.rg-dialog:modal) { overflow: hidden }` |
| JS | `invokers` (the `commandfor` fallback) and `overlays` — both page-level, *Behaviours* |

Missing `$Title` → phase 1's `missing-slot`. Exit animation is an
enhancement: only Chromium animates leaving the top layer.

## `DropdownMenu`

```tsx
interface MenuItemProps {
  href?: string;            // a link
  command?: Command;        // or a declarative action on another element
  commandfor?: string;
  current?: boolean;        // the item that stands for what is shown now
  disabled?: boolean;
  children?: ReactNode;
}
interface DropdownMenuProps {
  id?: string;
  align?: "start" | "end";             // option; default "start"
  typeahead?: boolean;                 // option; action menus only
  $Trigger: Slot<ButtonProps>;         // required
  $Item: KeyedSlot<MenuItemProps>;     // in order
}
```

An item without a handler can do two things: navigate, or command another
element. That is the whole action vocabulary of a React-less page.

```tsx
// .rtsx — every item is a link
<DropdownMenu align="end">
  <$Trigger variant="ghost">0.1 alpha</$Trigger>
  <$Item key="alpha1" href="/" current>0.1 alpha</$Item>
  <$Item key="npm" href="https://www.npmjs.com/package/@reactogenic/cli">npm</$Item>
</DropdownMenu>
```

```html
<button type="button" class="rg-button" data-variant="ghost" popovertarget="m1">0.1 alpha</button>
<ul id="m1" class="rg-menu" data-align="end" popover>
  <li><a href="/" aria-current="true">0.1 alpha</a></li>
  <li><a href="https://www.npmjs.com/package/@reactogenic/cli">npm</a></li>
</ul>
```

```tsx
// .rtsx — an item is an action
<DropdownMenu>
  <$Trigger>Actions</$Trigger>
  <$Item key="shortcuts" command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</$Item>
  <$Item key="source" href="https://github.com/reactogenic/reactogenic">Source</$Item>
</DropdownMenu>
```

```html
<button type="button" class="rg-button" popovertarget="m2" aria-haspopup="menu">Actions</button>
<div id="m2" class="rg-menu" popover role="menu">
  <button type="button" role="menuitem" command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>
  <a href="https://github.com/reactogenic/reactogenic" role="menuitem">Source</a>
</div>
```

| | |
| --- | --- |
| **links or actions decide the semantics, at compile time** | every item a link → a disclosure of links: a list, no `menu` role, Tab moves through it, **no JS**. Any action item → `role="menu"`, which promises arrow keys → the `menu-keys` behaviour is mounted on that menu |
| placement | the popover's implicit anchor is its invoker: `anchor()` insets and `position-try-fallbacks: flip-block` on the class — no rule per use site. (`position-area` does not flip in WebKit on a page taller than the viewport.) Inside `@supports`; without anchor positioning the UA's centred popover remains |
| `typeahead` | mounts `menu-keys` with `RG_MENU_TYPEAHEAD` |
| opening a dialog from an item | closes the menu, natively |
| not in phase 2 | sections and separators, checkable items, submenus, icons |

## `SideMenu`

```tsx
interface SideMenuItemProps {
  href?: string;                              // a link — or, with nested $Item and no href, a disclosure
  children?: ReactNode;                       // the label
  $Item?: KeyedSlot<SideMenuItemProps>;       // recursive
}
interface SideMenuSectionProps {
  title?: string;
  collapsed?: boolean;                        // option: absent = not collapsible; false = collapsible, open
  $Item: KeyedSlot<SideMenuItemProps>;
}
interface SideMenuProps {
  id?: string;
  label: string;                              // the landmark's name
  $Header?: Slot<ComponentProps<"div">>;
  $Section: KeyedSlot<SideMenuSectionProps>;
  $Footer?: Slot<ComponentProps<"div">>;
  $Toggle?: Slot<ComponentProps<"button">>;   // the drawer's button; fallback: the menu icon
}
```

```tsx
// layout.rtsx
<SideMenu label="Documentation">
  <$Section key="start">
    <$Item key="intro" href="/">Introduction</$Item>
  </$Section>
  <$Section key="guide" title="Guide">
    <$Item key="syntax">
      Syntax
      <$Item key="slots" href="/guide/slots/">Slots</$Item>
      <$Item key="flow" href="/guide/flow/">Flow control</$Item>
    </$Item>
  </$Section>
  <$Footer>MIT licensed</$Footer>
</SideMenu>
```

```html
<!-- emitted for the page /guide/slots/ -->
<button type="button" class="rg-sidemenu-toggle" popovertarget="s1" aria-label="Documentation">☰</button>
<nav id="s1" class="rg-sidemenu" popover aria-label="Documentation">
  <button type="button" data-part="close" popovertarget="s1" popovertargetaction="hide" aria-label="Close">✕</button>
  <section><ul><li><a href="/">Introduction</a></li></ul></section>
  <section>
    <h2>Guide</h2>
    <ul>
      <li>
        <details open>
          <summary>Syntax</summary>
          <ul>
            <li><a href="/guide/slots/" aria-current="page">Slots</a></li>
            <li><a href="/guide/flow/">Flow control</a></li>
          </ul>
        </details>
      </li>
    </ul>
  </section>
  <div data-part="footer">MIT licensed</div>
</nav>
```

| | |
| --- | --- |
| **the current page is the builder's, not the author's** | the item whose `href` is `pathname()` (trailing slash normalised) gets `aria-current="page"`, and every disclosure around it `open`. CSS styles `[aria-current]`. There is no `current` prop to get wrong; two pages' menus differ in exactly those attributes |
| drawer on small screens | **the same `<nav>` is a popover.** Above the breakpoint CSS shows it as a sticky column and hides the toggle and the close button; below it, the toggle opens it in the top layer: backdrop, light dismiss, Esc, focus return — native. One copy of the menu in the HTML |
| the close button | always emitted: on iOS 17–18.2 a popover does not close on an outside tap |
| the backdrop | a tap on it closes the drawer and activates nothing behind it |
| breakpoint | one design-system value (a media query cannot read a custom property) |
| groups | `<details>`; a collapsible section is a `<details>` too |
| JS | `overlays` only |
| not in phase 2 | icons and badges, an icon-only rail, groups that remember being closed across pages (storage, so JS) |

## Behaviours

`@reactogenic/ui/behaviors/*` (builder.md, *Behaviours*). What a page ships
is what its components mounted:

| Module | Mounted by | Does | Flags |
| --- | --- | --- | --- |
| `overlays` (page-level) | `Dialog`, `DropdownMenu`, `SideMenu` | closes open popovers and dialogs on `pagehide`: Back would otherwise restore the page with the overlay open (bfcache) | — |
| `invokers` (page-level) | `Dialog`; an item or button with `command` | `command` / `commandfor` for `show-modal`, `close`, `request-close` where the browser has none; feature-detected, inert at the floor | — |
| `menu-keys` (per menu) | `DropdownMenu` with an action item | Arrow keys, Home, End, wrap; Tab closes; focus returns to the trigger | `RG_MENU_TYPEAHEAD` |

A page with a menu of links and no dialog ships `overlays` alone; a page with
nothing that opens ships no script.

## CSS convention

What makes per-page pruning exact (builder.md, *CSS*):

| | |
| --- | --- |
| one `.css` per component, imported by the component | `import "./dialog.css"` |
| everything nested under the component's root class | `.rg-dialog { … [data-part="panel"] { … } }` |
| compile-time options | `data-<option>` on the root: `[data-align="end"]`, `[data-variant="ghost"]` |
| internal parts, slot attachments | `data-part`, `data-slot` |
| runtime state | pseudo-classes and `open` / `hidden` / `aria-*` only — never a class the script toggles |
| order | `@layer rg.base, rg.components;` declared once, in the tokens file every component imports; component rules in `rg.components`. Author CSS is unlayered, so it wins |
| tokens | custom properties on `:root`; `color-scheme: light dark` and `light-dark()` — the theme follows the OS (a toggle needs storage, so JS) |

## Types

`@types/react` (19.x) has `popover`, `popoverTarget`, `closedby`, but not
`command` / `commandfor`. `@reactogenic/ui` ships the augmentation. React's
renderer prints `popoverTarget` in camelCase; HTML attribute names are
case-insensitive, and the builder lower-cases them.

## Known limits

- A keyed slot does not keep the written order for **integer-like keys**
  (`key="2"`, `key="10"`: JavaScript enumerates them first, ascending).
  syntax.md notes it; here it decides rendering order.
  > OPEN: the `KEYED` marker carries the written order (recommended), or the
  > transpiler rejects integer-like keys where order matters.
- A drawer left open while the window is widened stays in the top layer until
  the next click; a popover is not modal, so Tab can leave the open drawer.
- `closedby="none"` does not stop Esc in Safari (no `closedby` there).
