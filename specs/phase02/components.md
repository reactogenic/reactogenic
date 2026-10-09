# Layout components: `SideMenu`, `Dialog`, `DropdownMenu`

`@reactogenic/ui` — the first components of the design system. Authored in
`.rtsx` with typed slots; executed by the builder in shell code
([builder.md](builder.md)); no new syntax. Evidence for every choice here:
[research/components.md](research/components.md),
[research/platform.md](research/platform.md).

Phase 2 has no dynamic segments: all three are layout components, executed in shell
code. What `Dialog` is when a dynamic segment drives it — `onClose`, open by being
mounted, a `<template>` with holes — is layout.md's and is not specified here.

Rules shared by all three:

1. **Platform first.** Each component is the platform's own element: state is
   the platform's (`dialog[open]`, `:popover-open`, `details[open]`), and
   opening, closing, focus and placement are the browser's. JS is added per
   feature, by the use site (*Behaviours*).
2. **Behaviour is declarative.** `href`, `command` + `commandfor`,
   `popovertarget`. No handlers. **A slot's contract omits what the component
   wires**: a slot's props replace its attachment's (syntax.md), so a
   `$Trigger` with a `commandfor` of its own would be a button that opens
   something else, or nothing. (`id` is the exception: rule 6.)
3. **Option or content.** An option selects among variants of the emitted
   HTML / CSS / JS, so it is a value of a closed union, known when the page
   is executed (builder.md, S3): `align="end"`, and equally
   `align={wide ? "end" : "start"}`. Content flows into the HTML (labels,
   slot bodies).
4. **Repeated things are a `KeyedSlot`**, rendered in the order written —
   whatever the keys look like: a component iterates `slotKeys($X)`
   (phase01/syntax.md, *Keyed slots*), never `Object.keys($X)`.
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
`variant?: "solid" | "ghost"` — a class of its own, `rg-button-ghost`, only
when it is not the default (*CSS convention*, variants).

```ts
type ButtonProps = ButtonLinkProps | ButtonActionProps;   // one or the other
// both: id, variant, disabled, aria-label, aria-current, children
interface ButtonLinkProps   { href: string }              // <a>: nothing of a button's
interface ButtonActionProps {                             // <button type="button">
  command?: Command; commandfor?: string;                 // a dialog's
  popoverTarget?: string; popoverTargetAction?: "show" | "hide" | "toggle";   // React's spelling of `popovertarget`
  "aria-haspopup"?: …;
}
interface TriggerProps {}   // the shared props only: a button whose action is its component's (rule 2)
type Command = "show-modal" | "close";
```

A link takes nothing of a button's: `command`, `commandfor` and
`popovertarget` do nothing on an `<a>`.

```tsx
<Button href="/" command="show-modal" commandfor="x">…</Button>
//               ^ error TS2322 — a link commands nothing
```

A link that is `disabled` is HTML's placeholder link — `<a>` without `href`:
not a link, not focusable — and says so with `aria-disabled`. The same holds
for a disabled link item of a menu.

```tsx
// .rtsx
<Button href="/guide/" variant="ghost">Guide</Button>
<Button href="/next/" disabled>Next</Button>
```

```html
<a class="rg-button rg-button-ghost" href="/guide/">Guide</a>
<a class="rg-button" aria-disabled="true">Next</a>
```

`Command` is what every browser of the floor has, and what `invokers` covers
below it: a dialog's `show-modal` and `close`.

| Not a `Command` | Why |
| --- | --- |
| `request-close` | Chrome has it from 139; the floor is 135. Chrome 135–138 have `command`, so `invokers` adds no listener there — and the button would be dead |
| `show-popover`, `hide-popover`, `toggle-popover` | a popover is targeted with `popoverTarget`, which is older than `command` |

The JSX augmentation (*Types*) gives a plain `<button>` every HTML command.

## `Dialog`

```tsx
interface DialogProps {
  id?: string;                                  // a name for a button elsewhere; generated when absent
  closedby?: "any" | "closerequest" | "none";   // option; HTML's attribute and meaning; default "any"
  $Trigger?: Slot<TriggerProps>;                // a button rendered where the dialog is written
  $Title: Slot<ComponentProps<"h2">>;           // required: the accessible name
  $Action?: KeyedSlot<ButtonLinkProps | TriggerProps>;   // footer buttons, in order: a link, or a button that closes
  closeLabel?: string;                          // content: the close button's accessible name; default "Close"
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
      <button type="button" class="rg-button rg-button-ghost" command="close" commandfor="d1">Cancel</button>
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
| identity | `$Trigger`: the compiler names the dialog (`useShellId("d")`). A button elsewhere: the author's `id`, and `<Button command="show-modal" commandfor="install">`; the builder checks the reference (idref-not-found, command-target). `<Dialog #install>` is not a way to name a dialog: it is a segment root (syntax.md) — the `id`, **and** the body is the segment `install.rtsx`; a body written inline is overwritten. For a long body, then; its slot elements stay, with phase 1's false segment-children warning (plan.md, *Later*) |
| its name | `aria-labelledby` the title: `id="<dialog>-t"` — or the `id` the author gave `$Title` |
| `$Trigger` | a button, and the dialog's: `href`, `command`, `commandfor`, `popoverTarget` are type errors at the slot element (rule 2) |
| `$Action` | with `href` a link; otherwise a button that closes the dialog — it takes no `command` of its own |
| `<footer>` | only when `$Action` is filled |
| the close button's name | `closeLabel`, printed as its `aria-label`; `"Close"` when absent. Content, not an option: a page in German writes `closeLabel="Schließen"`. The scrim has no name — it is `aria-hidden` |
| the surface | `<dialog>` is the whole viewport, transparent; `[data-part="panel"]` is what is seen. So everything around the panel can be the scrim |
| size | the panel never leaves the viewport: at most `32rem` wide, never wider or taller than the viewport less the dialog's padding. Header and footer stay; **the body scrolls**, both ways (a `<pre>` wider than the panel); a word wider than the panel breaks. The `<dialog>` itself never scrolls — that would carry the close button and the scrim out of view. (CSS: the open dialog is a grid of one `minmax(0, 1fr)` cell — in an `auto` track the panel's percentages resolve against its own content — with `overflow: hidden`) |
| light dismiss (`closedby="any"`) | a **scrim**: one `<button command="close">` that covers the dialog under the panel, emitted **after** the panel — `closedby` itself is not in Safari. The close button in the header is the first focusable element, so that is where focus goes on opening, never to the scrim (`tabindex="-1"` does not keep a dialog from focusing an element; its place in the document does) |
| delivery | a live `<dialog>` where it is written — anywhere flow content may be, the author's own `<form>` included: every button the dialog emits is `type="button"`. The top layer makes its position irrelevant, **except under a closed popover**: that is `display: none`, and nothing in a box that is not there is rendered — the dialog opens modal and unseen, the page inert until Esc. `SideMenu` sees to its own drawer (*SideMenu*, `$Header`, `$Footer`); under a popover of the author's own, a dialog may be opened only by a button inside that popover. (layout.md's `<template>` + clone is for dialogs with holes, opened from dynamic segments.) |
| the body | `children`. When a dynamic segment drives the dialog, `children` is a slot body in layout.md's sense: shell code, with holes |
| CSS | surface, `::backdrop`, header, body, footer, entry transition (`@starting-style`, `allow-discrete`; *CSS convention*, motion), scroll lock: `:root:has(.rg-dialog:modal) { overflow: hidden }` |
| JS | `invokers` (the `commandfor` fallback) and `overlays` — both page-level, *Behaviours* |

Missing `$Title` → phase 1's `missing-slot`. Exit animation is an
enhancement: only Chromium animates leaving the top layer.

> Ruled by the owner (decisions.md, G): the close button's name is a
> design-system matter, and a prop and a slot are both legitimate — neither
> needs anything of the contract or the compiler. Built: the prop,
> `closeLabel`, here and on `SideMenu`; a `$Close` slot is as valid, later.

> Ruled by the owner (decisions.md, H): a dialog under a closed popover of
> the author's own is the design system's to solve, later — no page check
> (`dialog-in-popover` is not built), and the delivery row's exception stays
> a documented limit.

> Ruled by the owner (decisions.md, B): the body as `children` or as
> layout.md's `<$Contents>` is a design-system choice too — both are to be
> supported, and the compiler treats them the same. The components keep
> `children`.

**Rejected:** the scrim as `<form method="dialog"><button>` — it needs no
`command`, but a dialog written inside the author's `<form>` then nests a
form: the HTML parser drops the inner `<form>` tag, and its button becomes a
submit button of the author's form — visible, and one click from submitting
it. The `command` button is native at the floor and covered by `invokers`
below it, like every other button of the dialog.

## `DropdownMenu`

```tsx
// An item is a link, or a declarative action on another element: one or the other.
type MenuItemProps = MenuLinkProps | MenuActionProps;
// both: current (the item that stands for what is shown now),
//       disabled (focus and the arrow keys skip it; a link loses its href), children
interface MenuLinkProps   { href: string }                              // no command
interface MenuActionProps { command?: Command; commandfor?: string }    // no href
interface DropdownMenuProps {
  id?: string;
  align?: "start" | "end";             // option; default "start"
  typeahead?: boolean;                 // option, of this menu; action menus only
  $Trigger: Slot<TriggerProps>;        // required
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
<button type="button" class="rg-button rg-button-ghost" popovertarget="m1">0.1 alpha</button>
<ul id="m1" class="rg-menu rg-menu-end" popover>
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
<div id="m3" class="rg-menu" popover role="menu" aria-labelledby="more">
  <a role="menuitem" aria-disabled="true">Next page</a>
  <button type="button" role="menuitem" disabled>Print</button>
  <button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>
</div>
```

| | |
| --- | --- |
| **links or actions decide the semantics, at compile time** | every item a link → a disclosure of links: a list, no `menu` role, Tab moves through it, **no JS of its own** (the page-level `overlays` only). Any action item → `role="menu"`, which promises arrow keys → the `menu-keys` behaviour is mounted on that menu. An item with `href` is a link — `disabled` too: it does not turn a menu of links into a menu |
| what the types exclude | `href` with `command` or `commandfor` — a link commands nothing (`Type 'string' is not assignable to type 'Command \| undefined'`, at the `command`); on `$Trigger`: `href`, `command`, `commandfor`, `popoverTarget` (rule 2). An item with neither `href` nor `command` is legal: a placeholder, usually `disabled`. A `command` without `commandfor` is the builder's command-target |
| an action menu's name and focus | `role="menu"` needs a name: the trigger gets `id="<menu>-t"` — unless the author gave `$Trigger` an `id`, which is then the one used — and `aria-haspopup="menu"`, the menu `aria-labelledby`. Its first item that is not disabled has `autofocus`: opening the menu puts focus in it, where the arrow keys work. (A disclosure of links keeps focus on its button, as APG's does.) |
| `disabled` | a button item: `disabled`. A link item: `<a>` without `href`, with `aria-disabled="true"` (`Button`). Neither takes focus, is reached by the arrow keys or typeahead, or closes the menu when clicked |
| placement | the popover's implicit anchor is its invoker: `position-anchor: auto` (its initial value differs between engines; in Chrome 151+ `anchor()` resolves against nothing without it), `top: anchor(bottom)`, `left: anchor(left)` — `right: anchor(right)` for `align="end"`, the class `rg-menu-end`; no rule per use site. Not `position-area`: on a scrolled page it did not flip in Playwright's WebKit 26.0 (research/platform.md, correction C). It does in WebKit 26.6 (measured) and in Safari 27.0.1; Safari 26.2–26.5, the floor, is not verified — so the insets stay. Inside `@supports (top: anchor(bottom))`; without anchor positioning the UA's centred popover remains |
| size, and where it goes when it does not fit | `width: max-content` (never less than the trigger), and never larger than the viewport less `1rem`, either way: a popover is fixed, so what of it is outside the viewport cannot be scrolled to. The room beside the trigger does not squeeze the menu — a menu that does not fit **moves**. `position-try-fallbacks`, in order: `flip-block` (above the trigger), `flip-inline` (the trigger's other edge — so `align` is where it goes *when there is room*), both; `--rg-menu-edge` (`@position-try`: `left: 0.5rem`), for a menu wider than the room on either side of its trigger — a phone; and last `--rg-menu-corner` (`top: 0.5rem; left: 0.5rem`), which always fits: for a menu taller than the room above and below its trigger (a phone on its side, a page zoomed to 400%), where **the menu scrolls**. Five fallbacks and no more: Chromium tries five and never a sixth (measured in 153; WebKit 26.6 tries a seventh) |
| `typeahead` | the menu mounts `menu-keys` with `RG_MENU_TYPEAHEAD` and with the data `{ typeahead: true }`: a printable key moves to the next item that starts with it. The flag is the page's (builder.md: the union of its mounts) — it puts the code in the page's script; the data is the menu's — the second argument of its mount's call: a menu without it has no typeahead, whatever else is on the page. No attribute: only the behaviour reads it (builder.md, *Behaviours*), so the menu's HTML is the same with and without |
| `current` | `aria-current="true"` on the item |
| opening a dialog from an item | `menu-keys` closes the menu first, with focus back on the trigger — so closing the dialog returns focus there. (Natively the menu closes too, but the dialog then returns focus to an item that is gone.) |
| not in phase 2 | sections and separators, checkable items, submenus, icons |

> Ruled by the owner (decisions.md, I): whether a disabled item stays
> focusable (APG's menu pattern) is the design system's to solve, later. As
> built, for the tests: it is skipped — by focus, the arrow keys and
> typeahead (the `disabled` row).

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
  $Toggle?: Slot<SideMenuToggleProps>;        // the drawer's button; fallback: the menu icon, named by `label`
  closeLabel?: string;                        // content: the drawer's close button's accessible name; default "Close"
}
// A <button>'s props without what the side menu wires (rule 2).
interface SideMenuToggleProps extends Omit<ComponentProps<"button">, "type" | "popoverTarget" | "popoverTargetAction" | "command" | "commandfor"> {}
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
| **the current page is the builder's, not the author's** | the item whose `href` is `pathname()` gets `aria-current="page"`, and every disclosure around it `open`. CSS styles `[aria-current]`. There is no `current` prop to get wrong; two pages' menus differ in exactly those attributes |
| which `href` is the page | every form the builder's link check takes for the route (builder.md, *Checks on the page*): `/guide/`, `/guide`, `/guide/index.html`, percent-encoded or not (`/%C3%BCber/` is `pages/über/`; `%2F` is no separator). **Not** an `href` with a `?query` or a `#fragment` — `/guide/#keyed`, `#keyed`: that is a state or a place of a page, not the page |
| a link into a page | `href="/guide/#install"` is not the page `/guide/`: with a fragment or a query an item is never current, and opens no group — which section is shown is run-time state, and six items of one page cannot all be the current one. So a group of a page's sections starts with the page itself (the example below; the note after it) |
| drawer on small screens | **the same `<nav>` is a popover.** Above the breakpoint CSS shows it as a sticky column and hides the toggle and the close button; below it, the toggle opens it in the top layer: backdrop, Esc, focus return (to a toggle that had focus: *Known limits*) — native. One copy of the menu in the HTML |
| `$Toggle` | `label` names the fallback, the icon: `aria-label="Documentation"`. A toggle with content of the author's is named by that content — `<$Toggle>Menu</$Toggle>` → `<button type="button" class="rg-sidemenu-toggle" popovertarget="s1">Menu</button>`, no `aria-label`: a button that shows "Menu" and is named "Documentation" answers to neither word by voice (WCAG 2.5.3, Label in Name). The `<nav>` keeps `label` |
| the close button | always emitted, first in the `<nav>`: on iOS 17–18.2 a popover does not close on an outside tap. Its name: `closeLabel` (`aria-label`; default `"Close"`), as the dialog's |
| the backdrop | a tap on it closes the drawer and activates nothing behind it. A popover's own `::backdrop` cannot do that: it lets pointer events through, and light dismiss closes the drawer on `pointerup` — *before* a tap's click is dispatched, which then lands on what was behind. (Measured in Chromium 153 and WebKit 26.6: `pointer-events: none` on `<body>` while the drawer is open stops a mouse click, not a touch tap.) So the drawer has a **scrim** of its own, as the dialog does: the last element of the `<nav>`, a button fixed over the rest of the viewport with `popovertargetaction="hide"`. A tap on it is a tap inside the popover: nothing closes until its click, and nothing behind is reached. It also closes the drawer where an outside tap does not (iOS 17–18.2). It is there only while the drawer is open. The drawer slides by its `left`, not by a transform, which would contain the fixed scrim |
| `$Header`, `$Footer` | `<div data-part="header">` before the sections, `<div data-part="footer">` after them; neither is emitted when its slot is not filled. Free content, other components included (rule 7): a `Dialog` or a `DropdownMenu` there is itself — the side menu's rules for links, lists, titles, its close button and its scrim reach `> section`, `> details` and its own children only |
| a `Dialog` in the drawer, opened from outside it | works: below the breakpoint the closed drawer is `display: none` (the user agent's rule for a closed popover), and a dialog in it would open modal and unseen — so the closed drawer has its box while a dialog in it is modal, `:not(:popover-open):has(dialog:modal) { display: block }`. That box is where a closed drawer is: beside the viewport |
| breakpoint | one design-system value, `50rem` (a media query cannot read a custom property) |
| groups | `<details>`; a collapsible section is a `<details>` too |
| what the types exclude | an item with `href` **and** nested `$Item` (`'href' does not exist in type 'SideMenuGroupProps'`, at the `href`); `collapsed` without `title` — a `<summary>` with nothing to click (`Property 'title' is missing … required in type 'SideMenuCollapsibleSectionProps'`). On `$Toggle`: `type`, `popoverTarget`, `popoverTargetAction`, `command`, `commandfor` (`'popoverTarget' does not exist in type 'SideMenuToggleProps'`). All are TS errors at the slot element, from `reactogenic check` |
| JS | `overlays` only |
| not in phase 2 | icons and badges, an icon-only rail, groups that remember being closed across pages (storage, so JS) |

```tsx
// layout.rtsx — a page and its sections
<SideMenu label="Documentation">
  <$Section key="learn">
    <$Item key="guide">
      Getting started
      <$Item key="page" href="/guide/">Overview</$Item>
      <$Item key="install" href="/guide/#install">Install</$Item>
    </$Item>
  </$Section>
</SideMenu>
```

```html
<!-- emitted for the page /guide/ — the group is open because of its first item -->
<button type="button" class="rg-sidemenu-toggle" popovertarget="s2" aria-label="Documentation">☰</button>
<nav id="s2" class="rg-sidemenu" popover aria-label="Documentation">
  <button type="button" data-part="close" popovertarget="s2" popovertargetaction="hide" aria-label="Close">✕</button>
  <section>
    <ul>
      <li>
        <details open>
          <summary>Getting started</summary>
          <ul>
            <li><a href="/guide/" aria-current="page">Overview</a></li>
            <li><a href="/guide/#install">Install</a></li>
          </ul>
        </details>
      </li>
    </ul>
  </section>
  <button type="button" data-part="scrim" popovertarget="s2" popovertargetaction="hide" tabindex="-1" aria-hidden="true"></button>
</nav>
```

> Ruled by the owner (decisions.md, J): links *into* the current page — a
> page's headings as nested items (`/guide/slots/#keyed`, `#keyed`,
> `?tab=api`) — are out of scope. As built: such an item is not marked and
> opens no disclosure, so the docs site's groups start with an "Overview"
> link to the page, as above.

## Behaviours

`@reactogenic/ui/behaviors/*` (builder.md, *Behaviours*). What a page ships
is what its components mounted:

| Module | Mounted by | Does | Flags |
| --- | --- | --- | --- |
| `overlays` (page-level) | `Dialog`, `DropdownMenu`, `SideMenu` | closes open popovers and dialogs when the reader goes elsewhere. To another page, on `pagehide`: Back would otherwise restore the page with the overlay open (bfcache). To another place of this page: a same-page link (`#keyed`) in an open drawer, menu or dialog scrolls the page under it and would leave it open over what was asked for — on `navigate`, of the Navigation API (Chrome 102, Firefox 147, Safari 26.2: the floor has it). Not `hashchange`: the second click on the same link changes no hash | — |
| `invokers` (page-level) | `Dialog`; a `Button` or a menu item with `command` | `command` / `commandfor` for `show-modal` and `close` — all of `Command` — where the browser has none; feature-detected (`"command" in HTMLButtonElement.prototype`), no listener at the floor | — |
| `menu-keys` (per menu) | `DropdownMenu` with an action item | Arrow keys with wrap, Home, End, over the items that are not disabled (`:disabled`, `aria-disabled="true"`). Tab closes the menu and moves on from the trigger. Activating an item closes it. Whenever it closes the menu, focus is put back on the trigger (the element `aria-labelledby` names) — not left to the browser: WebKit does not focus a button on click, so a menu opened with the mouse has no invoker to return to. Esc and light dismiss stay native | `RG_MENU_TYPEAHEAD`: typeahead, on the menus whose mount hands it `{ typeahead: true }` (the mount's data) |

A page with a menu of links and no dialog ships `overlays` alone; a page with
nothing that opens ships no script.

## CSS convention

What makes per-page pruning exact (builder.md, *CSS*):

| | |
| --- | --- |
| one `.css` per component, imported by the component | `import "./dialog.css"` |
| everything nested under the component's root class | `.rg-dialog { … > [data-part="panel"] { … } }`. A rule on an ancestor of the root cannot nest: it names the root in `:has()` — `:root:has(.rg-dialog:modal)`. `SideMenu` has a second root, its toggle: `.rg-sidemenu-toggle` |
| a rule reaches the component's own structure, never a slot's content | parts through child combinators from the root: `.rg-dialog > [data-part="panel"] > header > [data-part="close"]`, `.rg-sidemenu > [data-part="scrim"]`, `.rg-menu > li > a`. A descendant selector only below an element that holds nothing of the author's but a label: `.rg-sidemenu > :is(section, details) a`. **Never** `.rg-sidemenu a` or `.rg-sidemenu [data-part="close"]`: a `Dialog` in `$Header` has a `[data-part="close"]` of its own, and the rule that hides the drawer's hides the dialog's |
| compile-time options: **variants** | an option a rule selects resolves into **one class, unique to it**, through `variants()` of `@reactogenic/core` (below): `align="end"` → `rg-menu-end`, `variant="ghost"` → `rg-button-ghost` — the root's class and the variant's name. The default has none: `.rg-button` is its look. The rule is `.rg-button.rg-button-ghost`, nested as `&.rg-button-ghost`. An option only a behaviour reads is neither a class nor an attribute: it is the mount's data (`typeahead`; builder.md, *Behaviours*) |
| internal **parts**, slot attachments among them | `data-part`, **never a class**: `[data-part="footer"]` is `$Footer`'s attachment. A slot's props replace its attachment's, `className` among them (phase01/syntax.md, *Slots*) — `<$Footer className="mine">` is `<div data-part="footer" class="mine">`, and still the side menu's footer to its rules. A part that was a class would lose its rule to the first `className` an author wrote. An attachment the component's tag already names has none (`header > h2`) |
| no `!important` | in no file of the design system. Its rules are in layers (*order*, below), and with layers `!important` turns the cascade round: a layered `!important` declaration beats an unlayered one — the design system's over the project's — so one would be an override no project's CSS can answer (measured in Chromium and WebKit; decisions.md, K, rule 6). `test/convention.test.ts` fails on one |
| runtime state | a pseudo-class, or an attribute — never a class. **A behaviour names the state it writes**: the attribute's name in full (`setAttribute("aria-expanded", …)`, `toggleAttribute("inert")`), or the property that reflects it (`e.ariaExpanded`, `e.disabled`, `e.dataset.state` — builder.md, *CSS*, *The page's script*, has the table). That is what the builder reads off the page's script: state only a script can write (`aria-*`, `disabled`, `inert`, `data-state`, `checked`, `selected`, `value`) is decided on the page **unless the page's script names it** — on a page whose behaviours name no `aria-current`, a rule on `[aria-current]` with no such element is dropped; what the browser writes by itself (`open`, `hidden`, `style`) and every pseudo-class is "maybe" on every page. A name that is computed (`"aria-" + state`) is not seen, and a rule the pruner dropped for the page would start to match: that is what this rule keeps out, and an element added or moved leaves the page unpruned. **The three behaviours here write nothing at all**: they call the platform (`showModal()`, `close()`, `hidePopover()`, `focus()`); `test/convention.test.ts` holds them to it, and the builder's own test pins the state each of them names (`menu-keys` names `aria-disabled`, which it reads: its pages keep those rules) |
| motion | every `transition` is inside `@media (prefers-reduced-motion: no-preference)`: the dialog's fade and the drawer's slide are for a reader who has not asked for less (WCAG 2.3.3) |
| forced colours | a background is the canvas there, and a shadow is gone: every surface of the top layer — the dialog's panel, the menu, the drawer — has a `1px solid` border, and the current item is marked by more than a background (its weight) |
| order | `@layer rg.base, rg.components;` declared once, in the tokens file every component imports before its own (`import "./tokens.css"; import "./dialog.css"`); component rules in `rg.components`. Author CSS is unlayered, so it wins — over every normal declaration, which is all the design system has (*no `!important`*) |
| tokens | custom properties on `:root`; `color-scheme: light dark` and `light-dark()` — the theme follows the OS (a toggle needs storage, so JS) |

**Variants.** `variants(base, map, choice)` of `@reactogenic/core`
(builder.md, *What shell code can ask the builder*) is how a component
writes the `class` of an element with options: the base, then the class of
each chosen value, in the order the map names its dimensions. `DropdownMenu`'s:

```tsx
const menuVariants = { align: { start: "", end: "rg-menu-end" } } as const;
// …
<ul id={menu} className={variants("rg-menu", menuVariants, { align })} popover="">
```

```html
<ul id="m1" class="rg-menu rg-menu-end" popover>   <!-- align="end" -->
<ul id="m4" class="rg-menu" popover>               <!-- the default, or no `align` -->
```

```css
/* dropdown-menu.css */
.rg-menu { …; &.rg-menu-end { left: auto; right: anchor(right) } }
```

| | |
| --- | --- |
| one class per value | unique to it: `rg-<component>-<value>`. A page's sheet keeps `.rg-menu.rg-menu-end` exactly when an element of that page has the class (builder.md, *CSS*: a class is matched exactly) — as it kept `[data-align="end"]`; what changes is that the variant has a name of its own, which a rule, a report and a later packaging step can refer to |
| the default | `""` in the map: no class, and no rule of its own |
| a value the map does not have | a type error at the call: a variant is a value of a closed union (rule 3) |
| the record | the builder notes which classes each page's `variants()` calls resolved, and which component called: the report's `classes` (builder.md, *The report*). Nothing is decided by it — the page's HTML has the classes |
| a part | is not a variant: it stays `data-part` (the table above) |

**Rejected:** options as attributes on the root (`data-align="end"`,
`data-variant="ghost"` — the convention until 2026-10-06: sound, and a
variant had no name); parts as classes too (`rg-sidemenu-footer` — built
with the ruling that was reversed, decisions.md, K: a slot's `className`
then costs the part its rule, or stops replacing the attachment's — phase
1's semantics changed to suit the optimizer).

## Types

`@types/react` (19.x) has `popover`, `popoverTarget`, `closedby`, but not
`command` / `commandfor`. `@reactogenic/ui` ships the augmentation
(`src/html.d.ts`).

**The HTML in this file is React's output as a parser reads it.** Nothing
rewrites the markup: the builder's HTML is React's, to the byte (builder.md,
*The record*). As a string it is spelled otherwise:

| In this file | React prints | A component writes |
| --- | --- | --- |
| `popovertarget="m1"`, `popovertargetaction="hide"` | `popoverTarget="m1"`, `popoverTargetAction="hide"` — attribute names are case-insensitive in HTML | `popoverTarget`, `popoverTargetAction` |
| `popover` | `popover=""` | `popover=""` — **not** a bare `popover`: that is `true`, which React drops, and the menu would be no popover |
| `autofocus` | `autofocus=""` | `autoFocus` — a lower-case `autofocus` is dropped too |
| `open`, `disabled` | `open=""`, `disabled=""` | |
| `command`, `commandfor`, `closedby`, `tabindex="-1"` | as here | `command`, `commandfor`, `closedby`, `tabIndex` |

The contract tests compare after exactly that normalisation: names in lower
case, `x=""` as `x`.

A component imports its CSS, and TypeScript 7 checks side-effect imports
(TS2882): `@reactogenic/ui` declares `*.css` modules (`src/css.d.ts`, ambient),
which covers a site's own `.css` imports too.

## Known limits

- (Gone: a keyed slot did not keep the written order for integer-like keys —
  a menu written `b, 10, 9, a` was rendered `9, 10, b, a`. Ruled a bug by the
  owner (decisions.md, D) and fixed in phase 1's emit and in
  `@reactogenic/core`: an entry's property name is its key encoded, and the
  components iterate `slotKeys` — phase01/syntax.md, *Keyed slots*;
  `test/site.test.ts`, "keyed slots keep the order written".)
- A drawer left open while the window is widened stays in the top layer until
  the next click; a popover is not modal, so Tab can leave the open drawer.
- `closedby="none"` does not stop Esc in Safari (no `closedby` there).
- WebKit does not focus a button that is clicked. So after a dialog, a
  drawer or a menu of links was opened *with the mouse*, closing it leaves
  focus on nothing (`<body>`) there; opened from the keyboard, focus returns
  to the button. That is the engine's behaviour for every button on the web.
  (An action menu is not affected: `menu-keys` focuses the trigger itself.)
- Below the floor there is no Navigation API (Firefox before 147, Safari
  before 26.2): a same-page link in an open drawer, menu or dialog scrolls
  the page and leaves it open. Leaving the page still closes it (`pagehide`).
- A menu that fits nowhere near its trigger goes to the viewport's top left
  corner, wherever the trigger is: Chromium tries five fallbacks, and the
  fifth has to be the one that always fits.
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
- Measured in Chromium 143 / 153, Chrome 154 and Playwright's WebKit 26.0 /
  26.6 (`pnpm --filter @reactogenic/ui test:browser`). Not in Firefox
  (`ENGINES=firefox` is there; Playwright's Firefox 155 did not start in the
  sandbox the suite was run in), not in Safari proper (research/platform.md
  drove Safari 27.0.1 by script for the platform's own features, not for
  these components), on no touch device (taps are Playwright's emulation);
  and Playwright's WebKit has no page cache, so `overlays` on `pagehide` is
  measured in Chromium only. What came after the spec review — same-page
  links, a dialog in the closed drawer, a menu taller than the viewport,
  reduced motion, forced colours — was run in Chromium 153 and WebKit 26.6
  only. Forced colours are Playwright's emulation: Chromium forces the
  colours, WebKit only matches the media query. No screen reader was run.
