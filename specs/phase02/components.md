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
6. **An author's `id` wins.** A slot's props replace its attachment's
   (syntax.md), `id` among them. Where a component names an element itself —
   a dialog's title, an action menu's trigger — and the slot brings an `id`,
   that is the id the component refers to (`aria-labelledby`).
7. **A slot's content is the author's.** A component's CSS reaches its own
   structure only (*CSS convention*): another component in a slot or a body —
   a `Dialog` in `SideMenu`'s `$Header` — looks and works as it does anywhere.

Browser floor: **Chrome/Edge 135, Firefox 147, Safari 26.2**. Below it
nothing breaks hard: a dialog still opens (the `invokers` behaviour), a menu
opens unanchored.

## `Button`

The fourth export: `<button>` — or `<a>` when it has `href` — with
`variant?: "solid" | "ghost"` (`data-variant`, only when it is not the
default); it passes `command`, `commandfor`, `popoverTarget` (React's
spelling of `popovertarget`), `popoverTargetAction`, `id`, `disabled`,
`aria-label`, `aria-haspopup`, `aria-current` through.

A link that is `disabled` is HTML's placeholder link — `<a>` without `href`:
not a link, not focusable — and says so with `aria-disabled`. The same holds
for a disabled link item of a menu.

```tsx
// .rtsx
<Button href="/guide/" variant="ghost">Guide</Button>
<Button href="/next/" disabled>Next</Button>
```

```html
<a class="rg-button" data-variant="ghost" href="/guide/">Guide</a>
<a class="rg-button" aria-disabled="true">Next</a>
```

```ts
type Command = "show-modal" | "close" | "request-close";   // a dialog's: what `invokers` covers below the floor
```

A popover is targeted with `popoverTarget`, which is older than `command` —
so `Command` has no `toggle-popover`. The JSX augmentation (*Types*) gives a
plain `<button>` every HTML command.

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

```tsx
// .rtsx — the default: light dismiss
<Dialog>
  <$Trigger>Install</$Trigger>
  <$Title>Install</$Title>
  pnpm add @reactogenic/core@alpha
</Dialog>
```

```html
<!-- the second dialog of the page -->
<button type="button" class="rg-button" command="show-modal" commandfor="d2">Install</button>
<dialog id="d2" class="rg-dialog" closedby="any" aria-labelledby="d2-t">
  <div data-part="panel">
    <header>
      <h2 id="d2-t">Install</h2>
      <button type="button" data-part="close" command="close" commandfor="d2" aria-label="Close">✕</button>
    </header>
    <div data-part="body">pnpm add @reactogenic/core@alpha</div>
  </div>
  <button type="button" data-part="scrim" command="close" commandfor="d2" tabindex="-1" aria-hidden="true"></button>
</dialog>
```

| Part | |
| --- | --- |
| identity | `$Trigger`: the compiler names the dialog (`useShellId("d")`). A button elsewhere: the author's `id` — or a segment root, `<Dialog #install>` — and `<Button command="show-modal" commandfor="install">`; the builder checks the reference (idref-not-found, command-target) |
| its name | `aria-labelledby` the title: `id="<dialog>-t"` — or the `id` the author gave `$Title` |
| `$Action` | with `href` a link; otherwise it closes the dialog |
| `<footer>` | only when `$Action` is filled |
| the surface | `<dialog>` is the whole viewport, transparent; `[data-part="panel"]` is what is seen. So everything around the panel can be the scrim |
| size | the panel never leaves the viewport: at most `32rem` wide, never wider or taller than the viewport less the dialog's padding. Header and footer stay; **the body scrolls**, both ways (a `<pre>` wider than the panel); a word wider than the panel breaks. The `<dialog>` itself never scrolls — that would carry the close button and the scrim out of view. (CSS: the open dialog is a grid of one `minmax(0, 1fr)` cell — in an `auto` track the panel's percentages resolve against its own content — with `overflow: hidden`) |
| light dismiss (`closedby="any"`) | a **scrim**: one `<button command="close">` that covers the dialog under the panel, emitted **after** the panel — `closedby` itself is not in Safari. The close button in the header is the first focusable element, so that is where focus goes on opening, never to the scrim (`tabindex="-1"` does not keep a dialog from focusing an element; its place in the document does) |
| delivery | a live `<dialog>` where it is written — anywhere flow content may be, the author's own `<form>` included: every button the dialog emits is `type="button"`. The top layer makes its position irrelevant. (layout.md's `<template>` + clone is for dialogs with holes, opened from islands.) |
| CSS | surface, `::backdrop`, header, body, footer, entry transition (`@starting-style`, `allow-discrete`), scroll lock: `:root:has(.rg-dialog:modal) { overflow: hidden }` |
| JS | `invokers` (the `commandfor` fallback) and `overlays` — both page-level, *Behaviours* |

Missing `$Title` → phase 1's `missing-slot`. Exit animation is an
enhancement: only Chromium animates leaving the top layer.

**Rejected:** the scrim as `<form method="dialog"><button>` — it needs no
`command`, but a dialog written inside the author's `<form>` then nests a
form: the HTML parser drops the inner `<form>` tag, and its button becomes a
submit button of the author's form — visible, and one click from submitting
it. The `command` button is native at the floor and covered by `invokers`
below it, like every other button of the dialog.

## `DropdownMenu`

```tsx
interface MenuItemProps {
  href?: string;            // a link — whatever else the item has
  command?: Command;        // or a declarative action on another element
  commandfor?: string;
  current?: boolean;        // the item that stands for what is shown now
  disabled?: boolean;       // focus and the arrow keys skip it; a link loses its href
  children?: ReactNode;
}
interface DropdownMenuProps {
  id?: string;
  align?: "start" | "end";             // option; default "start"
  typeahead?: boolean;                 // option, of this menu; action menus only
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
<button type="button" class="rg-button" id="m2-t" popovertarget="m2" aria-haspopup="menu">Actions</button>
<div id="m2" class="rg-menu" popover role="menu" aria-labelledby="m2-t">
  <button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>
  <a href="https://github.com/reactogenic/reactogenic" role="menuitem">Source</a>
</div>
```

```tsx
// .rtsx — typeahead; the author's id on the trigger; disabled items
<DropdownMenu typeahead>
  <$Trigger id="more">More</$Trigger>
  <$Item key="next" href="/next/" disabled>Next page</$Item>
  <$Item key="print" disabled>Print</$Item>
  <$Item key="shortcuts" command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</$Item>
</DropdownMenu>
```

```html
<button type="button" class="rg-button" id="more" popovertarget="m3" aria-haspopup="menu">More</button>
<div id="m3" class="rg-menu" data-typeahead popover role="menu" aria-labelledby="more">
  <a role="menuitem" aria-disabled="true">Next page</a>
  <button type="button" role="menuitem" disabled>Print</button>
  <button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>
</div>
```

| | |
| --- | --- |
| **links or actions decide the semantics, at compile time** | every item a link → a disclosure of links: a list, no `menu` role, Tab moves through it, **no JS**. Any action item → `role="menu"`, which promises arrow keys → the `menu-keys` behaviour is mounted on that menu. An item with `href` is a link, whatever else it has — `disabled` too: it does not turn a menu of links into a menu |
| an action menu's name and focus | `role="menu"` needs a name: the trigger gets `id="<menu>-t"` — unless the author gave `$Trigger` an `id`, which is then the one used — and `aria-haspopup="menu"`, the menu `aria-labelledby`. Its first item that is not disabled has `autofocus`: opening the menu puts focus in it, where the arrow keys work. (A disclosure of links keeps focus on its button, as APG's does.) |
| `disabled` | a button item: `disabled`. A link item: `<a>` without `href`, with `aria-disabled="true"` (`Button`). Neither takes focus, is reached by the arrow keys or typeahead, or closes the menu when clicked |
| placement | the popover's implicit anchor is its invoker: `position-anchor: auto` (its initial value differs between engines; in Chrome 151+ `anchor()` resolves against nothing without it), `top: anchor(bottom)`, `left: anchor(left)` — `right: anchor(right)` for `align="end"`; no rule per use site. (`position-area` does not flip in WebKit on a page taller than the viewport.) Inside `@supports (top: anchor(bottom))`; without anchor positioning the UA's centred popover remains |
| size, and where it goes when it does not fit | `width: max-content` (never less than the trigger, never more than the viewport less `1rem`): the room beside the trigger does not squeeze the menu — a menu that does not fit **moves**. `position-try-fallbacks`, in order: `flip-block` (above the trigger), `flip-inline` (the trigger's other edge — so `align` is where it goes *when there is room*), both, and last `--rg-menu-edge` (`@position-try`: `left: 0.5rem`), for a menu wider than the room on either side of its trigger — a phone |
| `typeahead` | the menu gets `data-typeahead`, and mounts `menu-keys` with `RG_MENU_TYPEAHEAD`: a printable key moves to the next item that starts with it. The flag is the page's (builder.md: the union of its mounts) — it puts the code in the page's script; the attribute is the menu's — a menu without it has no typeahead, whatever else is on the page |
| `current` | `aria-current="true"` on the item |
| opening a dialog from an item | `menu-keys` closes the menu first, with focus back on the trigger — so closing the dialog returns focus there. (Natively the menu closes too, but the dialog then returns focus to an item that is gone.) |
| not in phase 2 | sections and separators, checkable items, submenus, icons |

## `SideMenu`

```tsx
// An item is a link, or a disclosure of nested items: one or the other.
type SideMenuItemProps = SideMenuLinkProps | SideMenuGroupProps;
interface SideMenuLinkProps {
  href?: string;
  children?: ReactNode;                       // the label
  $Item?: undefined;
}
interface SideMenuGroupProps {                // no href: its label opens it
  children?: ReactNode;                       // the label: the <summary>
  $Item: KeyedSlot<SideMenuItemProps>;        // recursive
}
// A section is always open, or collapsible — then it has a title: what opens it.
type SideMenuSectionProps = SideMenuPlainSectionProps | SideMenuCollapsibleSectionProps;
interface SideMenuPlainSectionProps {
  title?: string;
  collapsed?: undefined;
  $Item: KeyedSlot<SideMenuItemProps>;
}
interface SideMenuCollapsibleSectionProps {
  title: string;
  collapsed: boolean;                         // option: false = collapsible, open
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
  <button type="button" data-part="scrim" popovertarget="s1" popovertargetaction="hide" tabindex="-1" aria-hidden="true"></button>
</nav>
```

| | |
| --- | --- |
| **the current page is the builder's, not the author's** | the item whose `href` is `pathname()` (trailing slash normalised) gets `aria-current="page"`, and every disclosure around it `open`. CSS styles `[aria-current]`. There is no `current` prop to get wrong; two pages' menus differ in exactly those attributes |
| a link into a page | `href="/guide/#install"` is not the page `/guide/`: with a fragment or a query an item is never current, and opens no group — which section is shown is run-time state, and six items of one page cannot all be the current one. So a group of a page's sections starts with the page itself (below) |
| drawer on small screens | **the same `<nav>` is a popover.** Above the breakpoint CSS shows it as a sticky column and hides the toggle and the close button; below it, the toggle opens it in the top layer: backdrop, Esc, focus return — native. One copy of the menu in the HTML |
| the close button | always emitted, first in the `<nav>`: on iOS 17–18.2 a popover does not close on an outside tap |
| the backdrop | a tap on it closes the drawer and activates nothing behind it. A popover's own `::backdrop` cannot do that: it lets pointer events through, and light dismiss closes the drawer on `pointerup` — *before* a tap's click is dispatched, which then lands on what was behind. (Measured in Chromium 153 and WebKit 26.6: `pointer-events: none` on `<body>` while the drawer is open stops a mouse click, not a touch tap.) So the drawer has a **scrim** of its own, as the dialog does: the last element of the `<nav>`, a button fixed over the rest of the viewport with `popovertargetaction="hide"`. A tap on it is a tap inside the popover: nothing closes until its click, and nothing behind is reached. It also closes the drawer where an outside tap does not (iOS 17–18.2). The drawer slides by its `left`, not by a transform, which would contain the fixed scrim |
| `$Header`, `$Footer` | `<div data-part="header">` before the sections, `<div data-part="footer">` after them; neither is emitted when its slot is not filled. Free content, other components included (rule 7): a `Dialog` or a `DropdownMenu` there is itself — the side menu's rules for links, lists, titles, its close button and its scrim reach `> section`, `> details` and its own children only |
| breakpoint | one design-system value, `50rem` (a media query cannot read a custom property) |
| groups | `<details>`; a collapsible section is a `<details>` too |
| what the types exclude | an item with `href` **and** nested `$Item` (`'href' does not exist in type 'SideMenuGroupProps'`, at the `href`); `collapsed` without `title` — a `<summary>` with nothing to click (`Property 'title' is missing … required in type 'SideMenuCollapsibleSectionProps'`). Both are TS errors at the slot element, from `reactogenic check` |
| JS | `overlays` only |
| not in phase 2 | icons and badges, an icon-only rail, groups that remember being closed across pages (storage, so JS) |

```tsx
// layout.rtsx — a page and its sections
<$Item key="guide">
  Getting started
  <$Item key="page" href="/guide/">Overview</$Item>
  <$Item key="install" href="/guide/#install">Install</$Item>
</$Item>
```

```html
<!-- emitted for the page /guide/: the group is open because of its first item -->
<details open>
  <summary>Getting started</summary>
  <ul>
    <li><a href="/guide/" aria-current="page">Overview</a></li>
    <li><a href="/guide/#install">Install</a></li>
  </ul>
</details>
```

## Behaviours

`@reactogenic/ui/behaviors/*` (builder.md, *Behaviours*). What a page ships
is what its components mounted:

| Module | Mounted by | Does | Flags |
| --- | --- | --- | --- |
| `overlays` (page-level) | `Dialog`, `DropdownMenu`, `SideMenu` | closes open popovers and dialogs on `pagehide`: Back would otherwise restore the page with the overlay open (bfcache) | — |
| `invokers` (page-level) | `Dialog`; a `Button` or a menu item with `command` | `command` / `commandfor` for `show-modal`, `close`, `request-close` where the browser has none; feature-detected (`"command" in HTMLButtonElement.prototype`), no listener at the floor | — |
| `menu-keys` (per menu) | `DropdownMenu` with an action item | Arrow keys with wrap, Home, End, over the items that are not disabled (`:disabled`, `aria-disabled="true"`). Tab closes the menu and moves on from the trigger. Activating an item closes it. Whenever it closes the menu, focus is put back on the trigger (the element `aria-labelledby` names) — not left to the browser: WebKit does not focus a button on click, so a menu opened with the mouse has no invoker to return to. Esc and light dismiss stay native | `RG_MENU_TYPEAHEAD`: typeahead, on the menus that have `data-typeahead` |

A page with a menu of links and no dialog ships `overlays` alone; a page with
nothing that opens ships no script.

## CSS convention

What makes per-page pruning exact (builder.md, *CSS*):

| | |
| --- | --- |
| one `.css` per component, imported by the component | `import "./dialog.css"` |
| everything nested under the component's root class | `.rg-dialog { … > [data-part="panel"] { … } }`. A rule on an ancestor of the root cannot nest: it names the root in `:has()` — `:root:has(.rg-dialog:modal)`. `SideMenu` has a second root, its toggle: `.rg-sidemenu-toggle` |
| a rule reaches the component's own structure, never a slot's content | parts through child combinators from the root: `.rg-dialog > [data-part="panel"] > header > [data-part="close"]`, `.rg-sidemenu > [data-part="scrim"]`, `.rg-menu > li > a`. A descendant selector only below an element that holds nothing of the author's but a label: `.rg-sidemenu > :is(section, details) a`. **Never** `.rg-sidemenu a` or `.rg-sidemenu [data-part="close"]`: a `Dialog` in `$Header` has a `[data-part="close"]` of its own, and the rule that hides the drawer's hides the dialog's |
| compile-time options | `data-<option>` on the root: `[data-align="end"]`, `[data-variant="ghost"]`; read by a behaviour too (`data-typeahead`) |
| internal parts, slot attachments | `data-part`, `data-slot` |
| runtime state | pseudo-classes and `open` / `hidden` / `aria-*` only — never a class the script toggles |
| order | `@layer rg.base, rg.components;` declared once, in the tokens file every component imports before its own (`import "./tokens.css"; import "./dialog.css"`); component rules in `rg.components`. Author CSS is unlayered, so it wins |
| tokens | custom properties on `:root`; `color-scheme: light dark` and `light-dark()` — the theme follows the OS (a toggle needs storage, so JS) |

## Types

`@types/react` (19.x) has `popover`, `popoverTarget`, `closedby`, but not
`command` / `commandfor`. `@reactogenic/ui` ships the augmentation
(`src/html.d.ts`). React's renderer prints `popoverTarget` in camelCase and
`popover=""` for the bare attribute; HTML attribute names are
case-insensitive, and the builder lower-cases them.

A component imports its CSS, and TypeScript 7 checks side-effect imports
(TS2882): `@reactogenic/ui` declares `*.css` modules (`src/css.d.ts`, ambient),
which covers a site's own `.css` imports too.

## Known limits

- A keyed slot does not keep the written order for **integer-like keys**
  (`key="2"`, `key="10"`: JavaScript enumerates them first, ascending).
  syntax.md notes it; here it decides rendering order.
  > OPEN: the `KEYED` marker carries the written order (recommended), or the
  > transpiler rejects integer-like keys where order matters.
- A drawer left open while the window is widened stays in the top layer until
  the next click; a popover is not modal, so Tab can leave the open drawer.
- `closedby="none"` does not stop Esc in Safari (no `closedby` there).
- WebKit does not focus a button that is clicked. So after a dialog or a
  drawer was opened *with the mouse*, closing it returns focus to nothing
  (`<body>`) there; opened from the keyboard, focus returns to the button.
  That is the engine's behaviour for every button on the web.
- Menus are placed with physical insets (`top`, `left`, `right`): the form
  that was measured to flip in every engine. So `align` is not mirrored in a
  right-to-left page yet, and the drawer is always on the left.
- Rule 7 is about selectors, not inheritance: a component written in a slot
  inherits what CSS inherits there — in `SideMenu`'s `$Footer`, its smaller
  font size.
- Unlayered author CSS wins over **every** rule of a component, so a bare
  element selector in it restyles the components' own elements: `h2 { … }`
  reaches the dialog's title and the side menu's section titles, `a { … }`
  every button that is a link, `ul { … }` the menus. Text styles are scoped
  to the page's content, as `site/site.css` does:

  ```css
  h2 { margin-top: 2.5rem }                             /* also the dialog's title */
  :is(main, main > section) > h2 { margin-top: 2.5rem } /* the page's headings     */
  ```
- A menu item with `href` and `command` is a link: the command is not
  emitted. The types allow the pair.
- Measured in Chromium 143 / 153, Chrome 154 and Playwright's WebKit 26.0 /
  26.6 (`pnpm --filter @reactogenic/ui test:browser`). Not in Firefox
  (`ENGINES=firefox` is there; Playwright's Firefox 155 did not start in the
  sandbox the suite was run in), not in Safari proper, on no touch device
  (taps are Playwright's emulation); and Playwright's WebKit has no page
  cache, so `overlays` is measured in Chromium only.
