# Phase 2 research — components: `SideMenu`, `Dialog`, `DropdownMenu`, and the docs site

2026-10-04. Researcher key: `components`. Experiments: `exp-components/` next to this file
(nothing in the repository was changed). Web facts carry their URL and were fetched today
unless a date is given. "Measured" means a command in `exp-components/` produced it.

## 0. Summary

- All three components can be emitted as **plain HTML + CSS with zero JS** on today's platform:
  `<dialog>` + `command`/`commandfor`, `popover` + CSS anchor positioning (implicit anchor), `<details>`,
  `aria-current`. Measured with 22 behaviour checks and no `<script>` on any page: all pass in Chrome 154;
  Chromium 143, WebKit 26.0 and a WebKit 26.6 build fail only what section 6 lists (`closedby`, `focusgroup`,
  and WebKit not focusing a clicked button). Firefox could not be started here: UNVERIFIED locally.
- The proposed `.rtsx` API **type-checks under phase 1 as shipped** (`reactogenic check`, exit 0) and, run
  through the phase 1 transpiler + React's static renderer, yields the HTML proposed below. So the contract
  needs no new syntax. It did surface 7 phase 1 gaps (section 7).
- Sizes for a 4-page site (brotli): HTML 767–832 B per page, component CSS 1,072 B (shared), JS 0 B.
  Optional fallbacks for older engines: 402 B for all three. The same page hydrated by React: 61,326 B.
- Honest reading of the bet from this angle: here, component awareness buys **checks and exactness**
  (id references, routes, current page, which fallback a page needs), not bytes. Per-page CSS pruning found
  1 unused rule of 41, because all three components sit in the shared layout.
- Natural authoring of the site violates **no shell rule in page code** if nav items are written as slot
  elements. The violations are inside the design-system components (loops over slot entries, conditional
  parts) and in anything driven by a nav constant. Section 5 lists the relaxations.

## 1. Platform state (checked 2026-10-04)

| Primitive | Used for | Status | Source |
| --- | --- | --- | --- |
| `<dialog>`, `showModal`, `::backdrop` | Dialog | Baseline widely available (2022) | developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/dialog |
| `command` / `commandfor` (buttons only) | opening a dialog from any button, closing it | "Baseline 2025, newly available … since December 2025": Chrome 135, Firefox 144, Safari 26.2; 84.72 % global | MDN Invoker_Commands_API; caniuse.com/mdn-html_elements_button_commandfor |
| `<dialog closedby>` | light dismiss | Chrome 134, Firefox 141; **Safari: Technology Preview only**; 73.39 % global | caniuse.com/mdn-html_elements_dialog_closedby. Measured: WebKit 26.0 no, WebKit build 26.6 yes |
| `popover`, `popovertarget` | menu, drawer | Baseline; measured in all four engines | MDN Popover_API |
| CSS anchor positioning | menu placement | Chrome 125, Safari 26, Firefox 147 (2026-01-13); 85.92 % global; caniuse marks Chrome and Firefox "partial", Safari 27 "full" | caniuse.com/css-anchor-positioning |
| implicit anchor (popover → its invoker) | no `anchor-name` per use site | "an explicit association does not need to be made using the anchor-name and position-anchor properties" | MDN Popover_API/Using. Measured: works in all four engines |
| `<details>`, `name`, `::details-content` | nested nav groups | measured present in all four engines | `test-results.json` |
| `focusgroup` | arrow keys in a menu | **Shipped in Chrome 150** (Intent to Ship 2026-04-24, origin trial 146–149); Mozilla position positive; WebKit no signal | mail-archive.com/blink-dev@chromium.org/msg16361.html; developer.chrome.com/blog/focusgroup-rfc (2026-03-05). Measured: Chrome 154 yes; Chromium 143, WebKit no |
| `<menubar>`, `<menulist>`, `<menuitem>`, `<menuitemcheckbox>`, `<menuitemradio>`, `<submenu>` | a native menu | Proposal. Open UI explainer last updated 2026-10-01; WHATWG draft PR #12011; TAG review #1242; Chrome Intent to Prototype. Measured: not in Chrome 154. **`<a>` is disallowed in a menulist; application menus only** | open-ui.org/components/menu.explainer/ |
| `<navigationbar>` | site navigation | Open UI explainer, created 2025-11-07, updated 2026-06-04, "very early stage" | open-ui.org/components/navigation-bar.explainer/ |
| `interpolate-size`, `interestfor` | height animation of `<details>`; hover popovers | Chromium only (measured) | not needed for phase 2 |

Consequence: the floor for "zero JS" is **Chrome 135 / Firefox 147 / Safari 26.2**. Two features are
progressive on top: `closedby` (no Safari release) and `focusgroup` (Chrome 150+ only).

## 2. Survey: the anatomy mature systems agree on

Apple and Material rows are from knowledge of the documentation (their sites are script-rendered and only
`UIMenu` and `NavigationSplitView` could be fetched as JSON); the rest was fetched today.

### Sidebar

| System | Parts |
| --- | --- |
| UIKit | `UISplitViewController` columns (primary / supplementary / secondary); sidebar list with section headers and outline disclosure; `displayModeButtonItem` (the toggle, owned by the container, placed in the bar); collapses to a stack in compact width |
| SwiftUI | `NavigationSplitView { sidebar } detail: { }`; `List` + `Section` + `DisclosureGroup` + `NavigationLink`; `columnVisibility`; "collapses all columns into a single stack" on narrow sizes (developer.apple.com/documentation/swiftui/navigationsplitview) |
| Material 3 | navigation drawer, standard or modal: sheet, headline, section label, item (active indicator, icon, label, badge), divider, scrim. M3 Expressive deprecates it for the expanded navigation rail (9to5google.com/2025/05/14/material-3-expressive-navigation/) |
| Radix / Base UI / React Aria | no sidebar. Radix `NavigationMenu` is a horizontal bar (`Link` has `active`) |
| Web Awesome | `wa-page`: slots `navigation-header`, `navigation`, `navigation-footer`, `navigation-toggle`; `mobile-breakpoint` (768px), `nav-open`; `data-toggle-nav` on any button; "collapses into a drawer" (webawesome.com/docs/components/page/) |
| Starlight / VitePress | a config array: `{ label, items \| link \| slug, collapsed, badge }` (starlight.astro.build/guides/sidebar/) |
| Platform | `<nav aria-label>`, lists, `<details>`, `aria-current="page"` (APG disclosure navigation example) |

Agreed: header · sections with a label · items (icon, label, badge, link) · one or two levels of nesting with
disclosure · footer · one current item · on compact widths a drawer with a scrim, whose toggle the
container owns.

### Dialog

| System | Parts |
| --- | --- |
| UIKit | `UIAlertController`: title, message, actions with a style (`default`, `cancel`, `destructive`); sheets with detents |
| SwiftUI | `.alert(_:isPresented:actions:message:)`, `.confirmationDialog`, `.sheet`; `Button(role:)` |
| Material 3 | container, optional icon, headline, supporting text, actions, scrim; full-screen variant with a close icon |
| Radix | Root, Trigger, Portal, Overlay, Content, Title, Description, Close |
| Base UI | Root, Trigger, Portal, Backdrop, Popup, Title, Description, Close |
| React Aria | DialogTrigger, Modal, Dialog, `Heading slot="title"`, `Button slot="close"` |
| Web Awesome | slots default, `label`, `header-actions`, `footer`; `light-dismiss`, `without-header`; **`data-dialog="open <id>"` on any button, `data-dialog="close"` inside** (webawesome.com/docs/components/dialog/) |
| Platform | `<dialog>`, `command="show-modal" \| "close"`, `closedby`, `autofocus` |

Agreed: a trigger outside the surface, linked by identity · backdrop · surface · title (the accessible
name) · body · actions with a role · a close affordance · options: how it may be dismissed.

### Menu

| System | Parts |
| --- | --- |
| UIKit | `UIMenu(title, image, options, children)`; options `displayInline` (a section), `destructive`, `singleSelection`; children are actions or menus: **one ordered list** (developer.apple.com/documentation/uikit/uimenu) |
| SwiftUI | `Menu { Button; Divider; Section; Menu; Toggle; Picker }` |
| Material 3 | container, list item (leading icon, label, trailing icon or text), divider, submenu |
| Radix | Root, Trigger, Portal, Content, Label, Item, Group, CheckboxItem, RadioGroup, RadioItem, ItemIndicator, Sub, SubTrigger, SubContent, Separator, Arrow; a link is `asChild` (radix-ui.com/primitives/docs/components/dropdown-menu) |
| Base UI | Root, Trigger, Portal, Backdrop, Positioner, Popup, Arrow, Item, **LinkItem**, Separator, Group, GroupLabel, RadioGroup, RadioItem, CheckboxItem, SubmenuRoot, SubmenuTrigger (base-ui.com/react/components/menu) |
| React Aria | MenuTrigger, Button, Popover, Menu, MenuItem (`href` makes a link), MenuSection, Header, Separator, SubmenuTrigger, `Text slot="label" \| "description"`, Keyboard (react-aria.adobe.com/Menu) |
| Web Awesome | `wa-dropdown` (slot `trigger`; `placement`, `distance`), `wa-dropdown-item` (`value`, `type="checkbox"`, `variant="danger"`, `disabled`, `href`; slots `icon`, `details`, `submenu`) |
| Platform | popover + anchor positioning; `focusgroup="menu"`; APG: site navigation is a disclosure, not `role="menu"` — "it does not provide the complex functionality that assistive technologies expect in a widget that has the menu role" (w3.org/WAI/ARIA/apg/patterns/disclosure/examples/disclosure-navigation/) |

Agreed: trigger · popup anchored to it (side, alignment, flips) · items (icon, label, trailing detail,
disabled, destructive) · sections and separators · checkable items · submenus.
**Not agreed: links.** Every library allows a link item; the platform proposal forbids `<a>` in a menu and
APG keeps navigation out of `role="menu"`.

## 3. Proposed components

Names are the owner's working names. The contract below is the file set `exp-components/src/ds/*.rtsx`,
which type-checks (`reactogenic check -p src` → exit 0, one warning, see 7.4).

The HTML shown as "emitted" is what the stand-in builder of section 6 produced, with three spellings
normalised: React prints `popoverTarget`, `popover="auto"`, `open=""` and ids like `_R_3e_`.

Rules shared by the three:

1. **Option or content.** An option selects among variants of the emitted HTML/CSS/JS, so it is a literal
   of a closed union (`align="end"`, `closedby="closerequest"`, `collapsed`). Content flows into the HTML
   (`title`, `label`, slot bodies). The builder may reject a non-literal option.
2. **Repeated things are a `KeyedSlot`, rendered in the order written.**
3. **A container takes items or sections, never both at one level.** Slot elements are hoisted into one
   prop per slot name, so the order between an `<$Item>` and a `<$Section>` is lost.
4. **Behaviour is declarative**: `href`, `command` + `commandfor`, `popovertarget`. No handlers (S1 holds).
5. **State is the platform's**: `dialog[open]`, `:popover-open`, `details[open]`. No JS state.

### 3.1 Identity: who names a dialog

| Case | Who names it | Written as |
| --- | --- | --- |
| trigger next to the dialog | the compiler; the author never sees an id | `<$Trigger>` slot |
| a button elsewhere on the page, or two buttons | the author, as in HTML | `<Dialog id="shortcuts">` + `<Button command="show-modal" commandfor="shortcuts">` |
| the dialog's body is long | the author; one token names the id and the file | `<Dialog #install>` — phase 1 segment root, works today (7.4) |
| a reusable component with a dialog inside, used twice | the compiler | needs a compile-time `useId()` or an equivalent; not needed by the docs site |

Author ids stay plain strings, as in HTML and React. What the compiler adds is the check, which a bundler
cannot do: it sees every id and every reference on a page.

| Code (proposed) | Condition |
| --- | --- |
| idref-not-found | `commandfor` / `popovertarget` / `aria-labelledby` / `href="#x"` names no element of the page |
| id-duplicate | two elements of one page share an id (extends segment-duplicate) |
| command-target | `command="show-modal"` on something that is not a dialog, `toggle-popover` on a non-popover |

Generated ids: per page, in document order, short (`d1`, `m1`). React's `useId` gave `_R_3e_` in the
experiment: stable only while the tree does not change.

### 3.2 `Dialog`

```tsx
interface DialogProps {
  id?: string;                                  // name for a button elsewhere; generated when absent
  closedby?: "any" | "closerequest" | "none";   // option; default "any"
  $Trigger?: Slot<ButtonProps>;                 // a button rendered where the dialog is written
  $Title: Slot<ComponentProps<"h2">>;           // required: the accessible name
  $Action?: KeyedSlot<ButtonProps>;             // footer buttons, in order
  children?: ReactNode;                         // the body
}
```

| Part | Kind | Note |
| --- | --- | --- |
| `id` | content (a name) | or `#name` |
| `closedby` | option | the HTML attribute, unchanged |
| `$Trigger` | slot, `P = ButtonProps` | fallback: none — no trigger is rendered |
| `$Title` | slot, required | missing → `missing-slot: Dialog requires $Title` (measured, phase 1 reports it) |
| `$Action` | keyed slot | with `href` it is a link; otherwise it closes the dialog |
| args (`A`) | none | nothing varies per render in a static dialog |

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
<button type="button" class="rg-btn" command="show-modal" commandfor="d1">Delete project…</button>
<dialog id="d1" class="rg-dialog" closedby="closerequest" aria-labelledby="d1-t">
  <header>
    <h2 id="d1-t">Delete the project?</h2>
    <button type="button" class="rg-x" command="close" commandfor="d1" aria-label="Close">✕</button>
  </header>
  <div class="rg-dialog-body">This cannot be undone.</div>
  <footer>
    <button type="button" class="rg-btn rg-btn-ghost" command="close" commandfor="d1">Cancel</button>
    <a class="rg-btn" href="/">Delete</a>
  </footer>
</dialog>
```

- **CSS**: `css/dialog.css`, 975 B minified, 390 B brotli. Surface, `::backdrop`, header, close button, body,
  enter/exit transition (`@starting-style`, `allow-discrete`), scroll lock
  (`:root:has(.rg-dialog:modal) { overflow: hidden }`). The `> footer` rule only when a use site has `$Action`.
- **JS**: none. Focus moves into the dialog and returns to the invoker natively (measured, A13).
- **Opened from elsewhere**: `<Button command="show-modal" commandfor="shortcuts">` in the header and in a
  page body both open the layout's `<Dialog id="shortcuts">` (measured, A4 and A8).
- **Delivery**: a live `<dialog>` where it is written. The top layer makes its position irrelevant.
  `layout.md` chose a `<template>` cloned per `open` for dialogs driven by islands, which have holes; a
  static dialog has none, so the "one live `<dialog>`" alternative it lists costs no JS at all. This answers
  its `OPEN: a shell component used directly in shell code — who opens it`: a button with `commandfor`.

### 3.3 `DropdownMenu`

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
  align?: "start" | "end";             // option
  $Trigger: Slot<ButtonProps>;         // required
  $Item: KeyedSlot<MenuItemProps>;     // in order
}
```

An item without a handler can do two things: navigate, or command another element. That is the whole
action vocabulary of a React-less page.

```tsx
// .rtsx — items are links
<DropdownMenu align="end">
  <$Trigger variant="ghost">0.2 alpha</$Trigger>
  <$Item key="v0.2" href="/" current>0.2 alpha</$Item>
  <$Item key="v0.1" href="https://v0-1.reactogenic.dev/">0.1</$Item>
</DropdownMenu>
```

```html
<button type="button" class="rg-btn rg-btn-ghost" popovertarget="m1">0.2 alpha</button>
<ul id="m1" class="rg-menu rg-menu-end" popover>
  <li><a href="/" aria-current="true">0.2 alpha</a></li>
  <li><a href="https://v0-1.reactogenic.dev/">0.1</a></li>
</ul>
```

```tsx
// .rtsx — an item is an action
<DropdownMenu>
  <$Trigger>Actions</$Trigger>
  <$Item key="shortcuts" command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</$Item>
  <$Item key="source" href="https://github.com/reactogenic/reactogenic">Source</$Item>
  <$Item key="print" disabled>Print</$Item>
</DropdownMenu>
```

```html
<button type="button" class="rg-btn" popovertarget="m2">Actions</button>
<div id="m2" class="rg-menu" popover role="menu" focusgroup="menu">
  <button type="button" role="menuitem" command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>
  <a href="https://github.com/reactogenic/reactogenic" role="menuitem">Source</a>
  <button type="button" role="menuitem" disabled>Print</button>
</div>
```

- **Links or actions decide the semantics, at compile time.** Every item a link → a disclosure of links, no
  `menu` role, Tab moves through it (APG). Any action item → `role="menu"`, which promises arrow keys.
- **CSS**: `css/menu.css`, 824 B minified, 329 B brotli, and **no rule per use site**: the popover's implicit
  anchor is its invoker, so `position-area: block-end span-inline-end` and
  `position-try-fallbacks: flip-block, flip-inline` on the class are enough. Measured: 4px below the trigger,
  right edges equal with `align="end"`, flips above when there is no room (A9, B5), in all four engines.
  The rules sit in `@supports (position-area: block-end)`; without it the UA's centred popover remains.
- **JS**: none for a menu of links. For an action menu, arrow keys come from `focusgroup` in Chrome 150+
  (measured, B3); elsewhere Tab still works and a 185 B fallback adds the arrows.
- Opening a modal dialog from an item closes the menu natively (measured, B4).
- Not in phase 2: sections and separators, checkable items (state: needs JS or a form), submenus, icons.

### 3.4 `SideMenu`

```tsx
interface SideMenuItemProps {
  href?: string;                              // a link, or — with nested $Item and no href — a disclosure
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
<SideMenu id="nav" label="Documentation">
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
  <$Section key="reference" title="Reference" collapsed={false}>
    <$Item key="components" href="/components/">Components</$Item>
  </$Section>
  <$Footer>MIT licensed</$Footer>
</SideMenu>
```

```html
<!-- emitted for the page /guide/slots/ -->
<button type="button" class="rg-sm-toggle" popovertarget="nav" aria-label="Documentation">☰</button>
<nav id="nav" class="rg-sm" popover aria-label="Documentation">
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
  <details open><summary>Reference</summary><ul><li><a href="/components/">Components</a></li></ul></details>
  <div class="rg-sm-footer">MIT licensed</div>
</nav>
```

- **The current page is the builder's, not the author's.** The item whose `href` is the pathname being
  compiled gets `aria-current="page"`, and every disclosure around it gets `open`. The only bytes that
  differ between two pages' menus are that attribute and `open` on its ancestors (on `/` the "Syntax" group
  is emitted closed). CSS styles `[aria-current]`: no class, no JS. There is no `current` prop to get wrong.
- **Drawer on small screens: the same `<nav>` is a popover.** Above the breakpoint, CSS overrides the UA's
  `display: none` for a closed popover and the menu is a sticky column; below it, the toggle opens it in the
  top layer with a backdrop, light dismiss, Esc, and focus return — all native.
  Measured (A2, C1–C4, `edge.mjs`): in flow at 1200px (256px wide, toggle hidden, links work, exposed as
  `navigation`); hidden at 400px; opens to 288 × 800; a click outside closes it.
  One HTML copy of the menu, no JS.
- **CSS**: `css/sidemenu.css`, 1,556 B minified, 519 B brotli. `<details>` rules only when a use site has a
  disclosure; `.rg-sm-footer` only with `$Footer`.
- **JS**: none.
- Known limits, both cosmetic: a drawer left open while the window is widened stays in the top layer until
  the next click (measured); a popover is not modal, so Tab can leave the open drawer.
- Not in phase 2: icons and badges on items, an icon-only rail, groups that remember being closed across
  pages (needs storage, so JS).

### 3.5 The awkward cases

| Case | Answer | Evidence |
| --- | --- | --- |
| dialog opened from a button elsewhere | author id + `commandfor`, checked per page by the compiler | A4, A8 |
| who generates ids | the compiler, for `$Trigger` and for menus | B1 |
| dropdown of links | list of `<a>` in a popover, no `menu` role | A9–A12 |
| dropdown of actions | `role="menu"`; items are `command` buttons | B3, B4 |
| nested groups | recursive `$Item`; `<details>` | C3 |
| current page marked | from the pathname, per page, at compile time | A3, C3 |
| drawer on small screens | the `<nav>` is a popover; CSS decides | A2, C1–C4 |

## 4. The docs site

```
site/
  layout.rtsx                 DocsLayout: header (version menu, shortcuts button), SideMenu, <main>, shortcuts Dialog
  pages/
    index.rtsx                /
    install.rtsx              a segment of "/": the body of <Dialog #install>
    guide/slots/index.rtsx    /guide/slots/
    guide/flow/index.rtsx     /guide/flow/
    components/index.rtsx     /components/
```

This is `exp-components/src/site/`, built by `build.mjs` into `dist/<pathname>/index.html`.

| Question | Cheapest honest answer | Why |
| --- | --- | --- |
| how a page declares its pathname | file-based: **`index.rtsx` of a directory is the page**; the default export, no props | segments live next to the file that mounts them. "Every `.rtsx` under `pages/` is a page" would turn `install.rtsx` into `/install/` |
| shared layout | an ordinary component, imported and wrapped: `<DocsLayout title="Slots">…</DocsLayout>` | React semantics; no `_layout` convention to specify |
| nav model | the `SideMenu`'s slot elements in `layout.rtsx` are the model | 12 lines for 4 pages; no loop, no constant, no new rule |
| a constant array (`nav.ts`) | only when something else needs the order: prev/next links, an index page | needs loops over constants (section 5) and `Each` around slot elements (syntax.md OPEN #7) |
| a route table file | not in phase 2 | the builder derives it from `pages/`; it still gives the checks: `href="/guide/slot/"` → no such page |
| content | `.rtsx` only | Markdown is a second front end (and MDX compiles to React): a separate project |
| code samples | `<pre><code>{sample}</code></pre>`, the sample a template-literal constant, **no highlighting** | 0 B of JS and CSS; done in the experiment |
| next step for samples | token spans from the tsgo scanner already in the binary | UNVERIFIED effort; highlights TS/TSX/`.rtsx` only. Shiki with the repo's generated TextMate grammar needs Node at build time. Client-side highlighters ship JS |
| `<html>`, `<head>`, `<title>` | written in the layout, as in React 19 | the experiment does this; the alternative is a fixed document owned by the builder |
| theme | `color-scheme: light dark`, follows the OS | a toggle needs storage, so JS |
| search | out | needs an index and JS |

## 5. Shell rules: what the site violates, and the relaxations

`layout.md`: S1 no React runtime, S2 no variance (`Match`, `Switch`, `Each`, `?:`, `&&`, `.map()` producing
elements), S3 compile-time values only, S4 deterministic, S5 containers unconditional and unlooped.

| Where | Construct | Rule | |
| --- | --- | --- | --- |
| page and layout code as written in the experiment | literal slot elements, props, template-literal constants | none | **no violation** |
| inside `SideMenu`, `Dialog`, `DropdownMenu` | `<Each items={Object.keys($Section)}>`; `$Action !== undefined ? <footer>… : null`; `href === pathname() ? "page" : undefined` | S2 | every one of the three components does it |
| inside `Dialog`, `DropdownMenu` | `useId()` | S1 | only for generated ids |
| a layout that reads a nav constant | `<Each items={NAV}>` around `<$Item>`, `NAV[i + 1]` for prev/next | S2, and OPEN #7 | optional for this site |
| `` <title>{`${title} · Reactogenic`}</title> ``, a class computed from an option | an expression over compile-time values | S3 lists "literals, module-level constants, props" — it does not say expressions of them | needs one sentence |
| the same layout giving a different menu per page | the current item | sanctioned only as "the router choosing a shell by pathname" | needs the pathname to be a compile-time value |

Options, with consequences:

| | Rule | Consequence |
| --- | --- | --- |
| A | S2 stays absolute in page code; design-system code is exempt | smallest change. The site builds as written. A third kind of code ("design-system code") enters the model; authors cannot loop over a constant, and prev/next links are hand-written. By the standing rule this is a fake constraint: `NAV.map(…)` over a constant is deterministic |
| B | **Variance is allowed when it is a function of compile-time values and the pathname.** S2 becomes "no *runtime* variance" and folds into S3 + S4 | one rule for authors and the design system; reusable shell-safe components fall out. `Match` / `Switch` / `Each` / `?:` / `.map()` are fine when their inputs are compile-time; the error moves from the construct (shell-conditional, shell-loop) to the input that is not known (shell-dynamic-value). The builder must evaluate arbitrary TS expressions at build time |
| C | B, but only for `Each` / `Match` / `Switch`; `?:` and `.map()` stay errors | no saving: `items={…}` still has to be evaluated. A distinction without a reason |
| D | the pathname: (i) only framework components see it; (ii) it is a value authors can read | (i) is enough for this site. (ii) is needed the day an author writes their own nav or breadcrumb |
| E | S5 under B | a container looped over a constant is N static containers. segment-in-loop stays: the id would repeat |

Evidence for B's cost: the experiment's stand-in builder is B. It runs the transpiled `.tsx` once per
pathname and keeps the HTML; loops, conditionals and `useId` all just work, in 68 lines (`build.mjs`). S1 still needs a static check: `useState` would run and silently never update.

## 6. Measurements

Stand-in builder: `.rtsx` → `reactogenic serve` (phase 1 transpiler, 0.1.0-alpha.1) → esbuild 0.28.2 →
`renderToStaticMarkup` (React 19.3.0) per pathname. `node build.mjs && node sizes.mjs`:

| artifact | bytes | gzip -9 | brotli 11 |
| --- | ---: | ---: | ---: |
| `dist/index.html` | 2952 | 1050 | 832 |
| `dist/guide/slots/index.html` | 2373 | 1005 | 797 |
| `dist/guide/flow/index.html` | 2295 | 954 | 767 |
| `dist/components/index.html` | 3007 | 1013 | 801 |
| CSS: tokens + button + 3 components (min) | 3995 | 1261 | 1072 |
| — `sidemenu.css` | 1556 | 621 | 519 |
| — `dialog.css` | 975 | 477 | 390 |
| — `menu.css` | 824 | 431 | 329 |
| site's own CSS (min) | 640 | 382 | 303 |
| JS | 0 | 0 | 0 |
| optional: `commandfor.js` (before Chrome 135 / Firefox 144 / Safari 26.2) | 329 | 235 | 167 |
| optional: `closedby.js` (Safari releases) | 280 | 222 | 166 |
| optional: `menukeys.js` (no `focusgroup`) | 301 | 248 | 185 |
| optional: all three | 910 | 494 | 402 |
| contrast: the `/` page hydrated by React (`node baseline.mjs`) | 230525 | 71237 | 61326 |

Behaviour, `node test.mjs` (Playwright; no page has a `<script>`):

| Check | Chrome 154 | Chromium 143 | WebKit 26.0 | WebKit 26.6 build |
| --- | --- | --- | --- | --- |
| A4–A6, A8 dialog opens from a button elsewhere, Esc and `command="close"` close it | pass | pass | pass | pass |
| A7 backdrop click closes (`closedby="any"`) | pass | pass | **fail** | pass |
| B2 `closedby="closerequest"`: backdrop keeps it, Esc closes | pass | pass | pass | pass |
| A9, B5 menu anchored to its trigger, flips | pass | pass | pass | pass |
| A12, A13 keyboard only: opens, focus enters, Esc returns focus | pass | pass | pass | pass |
| A10, A11 the same after a mouse click on the trigger | pass | pass | **fail** | **fail** |
| B3 arrow keys in an action menu (`focusgroup`) | pass | **fail** | **fail** | **fail** |
| B4 a menu item opens a dialog; the menu closes | pass | pass | pass | pass |
| A2, A3 wide: menu in flow, current page marked | pass | pass | pass | pass |
| C1–C4 narrow: drawer opens from the toggle, light dismiss | pass | pass | pass | pass |

A10 and A11 fail in WebKit because it does not focus a button that is clicked, so Tab starts from the top
of the page; that is the engine's behaviour for every button, not this markup's.

With the fallback script (`dist-js/`): A7 passes in WebKit 26.0 and B3 passes in Chromium 143 and
WebKit 26.0 (`test-js.log`). Not verified: `commandfor.js` (no engine at hand lacks the feature but has
popover), anything in Firefox (Playwright's Firefox 144 and 155 fail to start in this sandbox: "Could not
find profile folder"), and Safari itself (WebKit builds stand in for it).

Per-page CSS (`node used.mjs`): of 41 component rules, 40 can match on three pages and 41 on
`/components/`. The one difference is `.rg-menu :disabled`. With the components in the shared layout,
per-page CSS splitting has nothing to remove on this site.

## 7. What the experiment found in phase 1

| # | Finding | Evidence | Effect on phase 2 |
| --- | --- | --- | --- |
| 1 | **A keyed slot does not keep the written order for integer-like keys.** Written `b, 10, a, 2, 0.2` → `Object.keys`: `2, 10, b, a, 0.2`. syntax.md notes it; all three components iterate their entries, so it now decides rendering order | `node keyorder.mjs` | a version menu with `key="2"`, `key="10"` renders reordered. Fix options in D4 |
| 2 | Order between two slot names is lost (`$Item` next to `$Section`) | desugaring: one prop per slot | contract rule 3 above |
| 3 | An attachment on a component always passes `children`, so the component must accept it | `<Section slot={$Section} />` → TS2322 until `children?` was added | design-system internals need the prop, or the transpiler omits an absent body |
| 4 | `<Dialog #install><$Title>…</$Title></Dialog>` warns segment-children, though the emitted code is right: the slots stay, the segment is the body | `check.log`; `node transform.mjs src/site/pages/index.rtsx` | the warning should skip slot elements |
| 5 | Forgetting `key` on a keyed slot's elements gives `TS2559: Type 'string' has no properties in common with type 'SideMenuItemProps'` | `src-dx/site/bad.rtsx` | needs a rewrite into slot terms; every nav author will hit it |
| 6 | A misspelt nested slot (`<$Itme>` inside `<$Section>`) is raw TS2561, not undeclared-slot | same file | minor |
| 7 | `@types/react` 19.3.0 has `popover`, `popoverTarget`, `closedby`, but not `command` / `commandfor` / `focusgroup` | `grep` in `@types/react/index.d.ts` | the design system ships a JSX augmentation (`src/ds/html.d.ts`). React's renderer passes the attributes through and prints `popoverTarget` in camelCase, which HTML accepts |

What works unchanged: required slots (`missing-slot`), option unions (TS2322 on `align="middle"`), slot
prop types (`slot-type` on `command="delete"`), recursive keyed slots, segment roots on a component.

## 8. Decisions for the owner

| # | Decision | Options | Suggested |
| --- | --- | --- | --- |
| D1 | browser floor | (a) Chrome 135 / Firefox 147 / Safari 26.2, zero JS; (b) lower, with the 402 B fallbacks | (a); emit a fallback only where a page uses the feature |
| D2 | naming a dialog | `$Trigger` (generated id) + author `id` / `#name` checked by the compiler; or handle objects; or strings unchecked | the first |
| D3 | compile-time `useId` for reusable components | support it; or defer | defer: the site does not need it |
| D4 | keyed slot order | (a) reject integer-like keys where order matters; (b) the `KEYED` marker carries the key order; (c) the builder uses source order and ignores JS enumeration | (b) or (c); (a) forbids `key="2"` |
| D5 | links and actions in one menu | one `DropdownMenu`, semantics derived from the items; or two components | one |
| D6 | action menus without `focusgroup` | Tab only; or the 185 B fallback | fallback, only on pages with an action menu |
| D7 | the drawer | (a) `<nav popover>` (measured, not modal); (b) `<dialog>` (modal, but a `dialog` role around the nav on desktop); (c) two copies of the menu | (a) |
| D8 | who places the drawer's toggle | (a) `SideMenu` renders it, fixed on small screens; (b) a `SideMenuToggle` the author places; (c) a page container that owns header, menu and main, as `UISplitViewController` and `wa-page` do | (a) now; (c) is the UIKit answer and a fourth component |
| D9 | current page | derived from `href` and the pathname; or an explicit prop | derived; exact match after normalising the trailing slash |
| D10 | shell rules | A, B or C of section 5 | B |
| D11 | the pathname as a value for authors | hidden (D-i) or exposed (D-ii) | hidden in phase 2 |
| D12 | a static dialog's delivery | a live `<dialog>` in place; or `layout.md`'s `<template>` + clone | live; templates only when there are holes |
| D13 | pages | `index.rtsx` per directory; or every `.rtsx` under `pages/`; or a route table | `index.rtsx` |
| D14 | who owns `<html>` / `<head>` | the layout writes it; or the builder | the layout, for phase 2 |
| D15 | code samples | plain `<pre>`; or scanner-based spans | plain; spans as a stretch |
| D16 | breakpoint of the drawer | one design-system token; or an option per use site | one token: a media query cannot read a custom property |

## Recommendation

1. Lift the three contracts of section 3 into the phase 2 spec as they are. They type-check today, need no
   new syntax, and their HTML is measured to work without JS on Chrome 135+, Safari 26.2+ and (per caniuse,
   not measured here) Firefox 147+.
2. Emit platform primitives, one-to-one: `Dialog` → `<dialog>` + `commandfor`; `DropdownMenu` → `popover` +
   implicit-anchor positioning; `SideMenu` → `<nav popover>` + `<details>` + `aria-current`. Ship the three
   fallbacks as per-page, feature-detected extras, not as a runtime.
3. Take shell-rule option B: variance is allowed when it is a function of compile-time values and the
   pathname. It is the only option under which the design system's own components are legal shell code
   without a special case.
4. Give the compiler the checks only it can do — id references, routes, key order, command targets — and
   count those, not bytes, as what component awareness buys on this site. The byte result is 0.8 KB of HTML
   and 1.1 KB of shared CSS per page against 61 KB for hydration, and any build that drops React would get
   most of that. Per-page CSS splitting has nothing to cut here.
5. Fix findings 1, 4 and 5 of section 7 before the site is written: each one is hit by the first nav an
   author writes.

## Open questions

- Firefox and Safari proper are unmeasured. Does the popover drawer behave the same in Firefox 147+, and
  does Safari 27 (installed here, not automatable in the sandbox) match the WebKit builds?
- `closedby` has no Safari release. Is "backdrop click does nothing in Safari" acceptable without the
  166 B fallback?
- How does a framework component read the pathname and get generated ids when the builder is not React?
  The experiment used a module variable and `useId`; both depend on the execution model another report
  decides.
- Is `role="menu"` right for a menu that mixes links and actions, given that the platform's own menu
  proposal forbids links and defers navigation to a separate element?
- Should `$Item` hand out args (`{ current }`) so an author can render the current item differently? It
  would be the first function slot evaluated at compile time; the site does not need it.
- A long menu loses its scroll position on every navigation, and groups do not remember being closed. Both
  need storage. Are they phase 2 problems for a 4-page site, or persistent-state problems for later?
- The Open UI menu elements and `<navigationbar>` would replace the emitted HTML of two components. The
  contract above does not depend on which primitive is emitted; worth re-checking when either ships.
- Does `Dialog` need a `$Description` (for `aria-describedby`), and do `$Action`s need a role
  (`cancel`, `destructive`) beyond `variant`?

## Verification (independent)

2026-10-04. Skeptic pass by a second agent. Everything below was re-run or re-fetched today in
`exp-components-verify/` (a copy of `exp-components/`; nothing in the repository or in the author's
directory was changed). "Reproduced" means the same command gave the same output.

### Verdict per claim

| # | Claim | Verdict | What I checked |
| --- | --- | --- | --- |
| 1 | 22 zero-JS checks pass in Chrome 154 | **confirmed** | Rebuilt (`node build.mjs`: the four pages are byte-identical to the author's), `TARGETS=chrome node test.mjs` → all 22 pass (`v-test-chrome.log`). No `<script>` and no `on*=` attribute in any of the four pages. Chromium 143 and WebKit 26.0 re-run too: they differ from Chrome 154 exactly where section 6 says. |
| 2 | Floor is Chrome 135 / Firefox 147 / Safari 26.2 | **partly** | Sources agree: MDN banner "Baseline 2025, newly available since December 2025"; caniuse `commandfor` 135 / 144 / 26.2, 84.72 %; caniuse anchor positioning 125 / 147 / 26, 85.92 % (0.01 % full + 85.91 % partial); BCD `popovertarget.implicit_anchor_reference` Chrome 133, Firefox 147, Safari 26; Firefox 147 released 2026-01-13. Two corrections below (C1, C2). |
| 3 | `closedby` has no Safari release, `focusgroup` is Chrome 150+; 166 B and 185 B fallbacks | **confirmed** | caniuse: Safari 26.x and 27 "not supported", TP only; BCD `dialog.closedby` safari = `preview`; Apple's Safari 27 release notes (released 2026-09-14) and the 27.2 beta notes do not contain "closedby". Intent to Ship Focusgroup, 2026-04-24, milestone 150; web.dev "New to the web platform in June" 2026 lists it in Chrome 150. Sizes reproduced to the byte. A7 / B3 with and without `rg.js` reproduced in Chromium 143 and WebKit 26.0. |
| 4 | Implicit anchor: no per-use-site CSS | **confirmed** | MDN quote is verbatim. New check (`v-extra.mjs` E3): two menus on `/components/` with the same class each anchor to their own trigger (gap 4 px), and stay anchored after scrolling with the menu open, in Chrome 154, Chromium 143, WebKit 26.0. Holds only for `popovertarget` / `commandfor` / `showPopover({source})`. Firefox: not measured. |
| 5 | One `<nav popover>` is sidebar and drawer | **confirmed, with corrections** | `edge.mjs` reproduced. The author's "exposed as `navigation`" used Playwright's DOM-computed role; I read Chrome's real accessibility tree over CDP: `navigation "Documentation"` with its links, and `button "0.2 alpha" expanded=false` (E1). Tab order at 1200 px is header → nav → main (E2). "Not modal" is not cosmetic: see C3. |
| 6 | The slot contract type-checks with the shipped binary | **confirmed** | `reactogenic check -p src --pretty=false` → exit 0, one warning; `-p src-dx` → the five diagnostics of section 7, exit 1. Binary reports `0.1.0-alpha.1`. |
| 7 | Sizes: HTML 767–832 B, CSS 1,072 B, JS 0 B; React 61,326 B | **confirmed** | `node sizes.mjs`, `node baseline.mjs`: identical numbers. The baseline counts the JS bundle only; the HTML and CSS would ship on top of it. "JS 0 B" is conditional on C1. |
| 8 | Per-page CSS splitting has nothing to remove | **partly** | `node used.mjs` reproduced (40 / 41 on three pages, 41 / 41 on `/components/`). True of this site by construction: see C4. |
| 9 | Pages violate no shell rule; the violations are in the design system; S2 must exempt or be restated | **partly** | The inventory is right (read all of `src/ds/*.rtsx` and `src/site/**`). The conclusion overstates: see C5. |
| 10 | Integer-like keys are reordered | **confirmed, and worse than stated** | `node keyorder.mjs` reproduced. Through the builder (`src/site/pages/keys/index.rtsx`): a menu written `10, 9, 2, next, 1` is emitted `1, 2, 9, 10, next`; sections keyed `2026, intro, 2025` are emitted `2025, 2026, intro`. `reactogenic check` passes that file with no diagnostic. |
| 11 | Menu elements are a proposal and disallow `<a>` | **confirmed** | Explainer: six elements, "last updated 2026-10-01", "Anchor `<a>` tags are disallowed in menu lists", "only covers application menus, not navigation menus". `<navigationbar>`: created 2025-11-07, updated 2026-06-04, "very early stage". APG sentence is verbatim. `menuElements=false` reproduced in Chrome 154. Not checked: PR #12011, TAG review #1242, the Intent to Prototype. |
| 12 | `@types/react` 19.3.0 lacks `command` / `commandfor` / `focusgroup` | **confirmed** | `npm view @types/react` → `latest: 19.3.0` (registry modified 2026-10-04). No file under `@types/react` or `@types/react-dom` contains "commandfor". `popover` at line 2940, `closedby` at 3194, `ButtonHTMLAttributes` at 3146. Emitted HTML carries `command`, `commandfor`, `focusgroup="menu"`. |
| 13 | `<Dialog #install>` transpiles correctly; false segment-children warning | **confirmed** | `node transform.mjs src/site/pages/index.rtsx` → `<Dialog id="install" $Title={{…}} $Action={{…}}><_Dialog_install /></Dialog>`; the warning is in the check output. |

### Corrections

**C1. The `commandfor` fallback is not a progressive enhancement.** 15.3 % of global usage has no invoker
commands (caniuse, 84.72 % support). I ran the zero-JS output in an engine without them (Playwright
Chromium 110, `v-old-zero.log`): A4, A6, A8, A13 and B1 fail — every dialog button on the site is dead.
With `rg.js` they pass (`v-old-js.log`). This also verifies `commandfor.js`, which the author listed as
not verified, for the dialog path; Chromium 110 has no popover, so the popover path is still unverified.
Consequence: a public docs site ships JS on every page (all four pages carry the layout's dialog), 167 B
for invoker commands plus 166 B for `closedby`. "JS 0 B" describes the floor browsers only.

**C2. Two of the three floor browsers were never measured.** Firefox does not start here (I tried once
outside the command sandbox; the same macOS sandbox-extension error; a second attempt that turned off
Firefox's own sandbox was denied by the permission system and I did not pursue it). Safari proper was
not driven. The WebKit "26.6" build is ahead of Safari 27 on `closedby` and `popover=hint` (both
`preview` in BCD), so it is not evidence for any Safari release. Also unverified: whether Firefox 147 or
148 is the floor for `@supports (position-area: block-end)`. BCD lists the `block-end` keyword as
Firefox 148 / Chrome 144, but Chromium 143 passes A9 with it, so that BCD entry is not reliable either way.
No touch device was measured, and the drawer is the small-screen feature.

**C3. The popover drawer lets clicks through its backdrop.** A popover's `::backdrop` is
`pointer-events: none`. Measured at 400 px with the drawer open (`v-scrim.mjs`): a click on the dimmed
area over the header's "Shortcuts" button closes the drawer **and opens the dialog**, in Chrome 154,
Chromium 143 and WebKit 26.0. Tab leaves the open drawer after the last link and lands on content
under the backdrop (E4). Widened while open, the drawer is a 157 px strip at 0,0 that covers the logo (E7).
A CSS-only repair exists (`v-scrim-fix.mjs`):

| Rule added while the drawer is open | click-through | Tab escapes |
| --- | --- | --- |
| none | yes (3 engines) | yes |
| `:root:has(.rg-sm:popover-open) :is(.site-header, main) { pointer-events: none }` | **no** (3 engines) | yes |
| the same with `interactivity: inert` | **no** in Chrome 154 and Chromium 143; unsupported in WebKit 26.0 | **no** in Chromium; yes in WebKit |

The rule needs to name what lies behind the drawer, which only a container that owns header, menu and
main can do. That is an argument for option (c) of D8, not only a later nicety.

**C4. "Nothing to remove" is a property of this layout, not a finding about the bet.** The layout puts a
`Dialog` and a `DropdownMenu` on every page. A page with only the `SideMenu` needs 684 B of the 1,072 B
(brotli), so 36 % would be prunable (`v-css-variant.mjs`). The CSS report (`css.md`) measured 38–73 %
on its own sample site. Recommendation 4's "per-page CSS splitting has nothing to cut" should not be
carried into the decision as a general statement.

**C5. S2 already has the special case.** `layout.md` lists "design-system components" and "shell-safe
reusable components" as two separate things shell code may use, says layout components are "executed by
the compiler", and sanctions "an optional slot being left empty" — which only works if a component may
branch on a slot. So option A is the spec as written, not a new exemption, and "B is the only option
without a special case" is too strong. B is still the better rule, for the reason `layout.md` itself
leaves open (loops over constants) and because the evaluation report reaches the same conclusion from the
other side ("S2 collapses into S3"). Small gaps in the inventory: S2 does not list `if` / early
`return`, which `Button` and `MenuItem` use; and the design system in the experiment is `.rtsx`, while
CLAUDE.md says the catalog is plain `.tsx`.

### What the report did not cover

- **Generated ids are needed in phase 2, not deferrable.** D3 defers "compile-time `useId`", but the
  layout's own `DropdownMenu` has no `id` and the `$Trigger` dialog has none: both got `_R_26_` /
  `_R_3e_` from React's `useId`. The evaluation report recommends a Go evaluator with no React, so the id
  source must be a compiler intrinsic from the first page.
- **The JS side of the builder is three fixed snippets on this site.** Per page the builder picks from
  `commandfor.js`, `closedby.js`, `menukeys.js` (910 B minified in total). Nothing here needs a bundler,
  code splitting or tree shaking, so this site cannot show whether esbuild's seam is good enough for JS.
- **Base path.** Every `href` is root-absolute and `aria-current` is an exact match on the pathname. A
  site served under a sub-path needs the builder to own the prefix; nothing in the contract says so.
- **The action menu has no accessible name.** Chrome's tree shows `menu ""` (E3c); the emitted
  `role="menu"` needs `aria-labelledby` pointing at the trigger, so the trigger needs an id too.
- **`focusgroup` and roles.** Adrian Roselli's tests of Chrome 150 (2026-07,
  adrianroselli.com/2026/07/focusgroup-tests.html) report role inference problems and advise caution.
  The emitted markup sets `role` on every item itself, and Chrome 154 exposes `menu` / `menuitem` × 3
  correctly, so it is not affected today; it is a reason to keep the explicit roles.

### On the recommendation

It holds, with four changes: treat `commandfor.js` as required output, not an extra (C1); add the
backdrop rule or choose the page container of D8 (c) before calling the drawer done (C3); drop the CSS
sentence from recommendation 4 (C4); and plan generated ids for phase 2. The contracts, the one-to-one
mapping to platform primitives, shell rule B and the three phase 1 fixes stand as written. Firefox and
Safari proper remain unmeasured.
