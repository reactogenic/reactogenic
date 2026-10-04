# Platform: how little JavaScript a side menu, a modal dialog and a dropdown need

Researcher key: `platform`. Date: 2026-10-04. Experiments: `exp-platform/` next to this file
(`/private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-platform/`).

## Answer in five lines

1. At a floor of **Chrome/Edge 135, Firefox 147, Safari 26.2** (= Baseline 2025 plus Firefox's anchor positioning,
   all shipped by 2026-01-13) the three components need **0 bytes of JavaScript** on a docs site: side menu with a
   mobile drawer, modal dialog with light dismiss, dropdown of links. Measured in Chrome 154 and two WebKit builds.
2. The only thing on such a site that still costs script is a dropdown of **actions** (`role="menu"`):
   **414 B** minified for the keys APG requires, **801 B** with the optional ones.
3. Going one year lower (Baseline 2024: Chrome 120, Firefox 130, Safari 18.2) costs **601 B** (open a modal +
   position a dropdown) and buys about **5 points** of global usage (84.7 % → 89.5 %).
4. The same three components with React 19 + Radix: **318 307 B minified, 101 556 B gzip** (measured).
   The zero-JS page is 2 512 B HTML + 2 604 B CSS (982 B + 1 035 B gzip).
5. Most of the saving comes from decisions only a component-aware compiler can make: links vs actions, modal vs
   not, which closing modes are used, whether the menu ever collapses, which page is current.

## 0. Method, sources, limits

| Source | Version / date | Used for |
|---|---|---|
| `web-features` npm, `data.json` | 3.40.1, published 2026-10-01 (`https://unpkg.com/web-features/data.json`) | Baseline status and dates, per-key support, browser release dates |
| `@mdn/browser-compat-data` npm | 8.1.4, published 2026-10-01 (`https://unpkg.com/@mdn/browser-compat-data/data.json`) | per-key versions, notes, partial implementations |
| caniuse `data-2.0.json` | updated 2026-09-30 (`raw.githubusercontent.com/Fyrd/caniuse/main/fulldata-json/data-2.0.json`) | usage share per browser version |
| Real engines, driven with playwright-core | Chrome **154.0.8037.93** (the installed stable), WebKit builds labelled **26.6**, **26.0**, **18.2**, Chromium **143** and **131** | behaviour, focus, accessibility tree |
| W3C APG, WebKit blog, Chrome blog, Open UI, Adrian Roselli, hidde.blog, bram.us | fetched 2026-10-04, URLs inline | patterns, statuses not yet in the data |

Scripts: `status.js` (Baseline table), `bcd.js` (compat keys), `usage.js` (usage share), `run.js` (behaviour
matrix), `measure.js` (bytes), `exit.js`/`exit2.js` (exit animations), `fg2.js`/`fg3.js` (focusgroup),
`backdrop.js` (zero-JS light dismiss). Outputs are in `exp-platform/out/`.

Limits, stated once:

- **Firefox was not run locally.** Its test build cannot start inside the sandbox ("Could not find profile
  folder"), and running outside the sandbox was denied. Every Firefox statement below comes from BCD or a cited
  page and is marked **UNVERIFIED locally**.
- **Playwright's WebKit is not Safari.** Its "26.0" build already has `command`/`commandfor` (Safari got them in
  26.2) and its "26.6" build already has `closedby` and `popover=hint` (Safari 27.0 has neither, per BCD). Treat
  the WebKit columns as "WebKit trunk around that date". Safari 27.0.1 is installed but could not be automated.
- Headless macOS uses overlay scrollbars, so layout shift from scroll locking was not measured.
- The downloaded browser builds (about 700 MB) were deleted after the runs; `PLAYWRIGHT_BROWSERS_PATH=… npx playwright-core install webkit` restores them.

## 1. Status of the platform (2026-10-04)

Current stable: Chrome 154 (2026-09-22), Firefox 157 (2026-09-29), Safari 27 (2026-09-14) — web-features 3.40.1.
"Baseline" = web-features status; "low" = newly available date, "wide" = widely available date (low + 30 months).

### Dialog

| Feature | Chrome | Firefox | Safari | Baseline | Note |
|---|---|---|---|---|---|
| `<dialog>`, `showModal()`, `close()`, `returnValue`, `cancel`/`close` events | 37 | 98 | 15.4 | wide (low 2022-03-14, wide 2024-09-14) | |
| `::backdrop` | 37 | 47 | 15.4 | wide | inherits from the dialog since Chrome 122 / Fx 120 / Safari 17.4 |
| `:modal` | 105 | 103 | 15.6 | wide (2025-03-02) | |
| `autofocus` on any element | 79 | 110 | 15.4 | wide | iOS: no effect without a hardware keyboard (BCD note) |
| initial focus, inert background, focus return | — | — | — | part of `<dialog>` | measured: Chrome 154/143/131, WebKit 26.6/26.0/18.2 |
| `requestClose()` | 134 | 139 | 18.4 | — | |
| **`closedby`** (`any` / `closerequest` / `none`) | 134 (2025-03-04) | 141 (2025-07-22) | **no** (Technology Preview 249, 2026-07-29) | **not Baseline** | Interop 2026 focus area (webkit.org/blog/17818, 2026-02-12). Not in the Safari 27.0 feature post (webkit.org/blog/18325). |
| `toggle` / `beforetoggle` on dialog | 132 | 133 | 26 | low | |
| `:open` | 133 | 136 | 26.5 | low 2026-05-11 | use `[open]` instead |

### Invoker commands, popover, interest

| Feature | Chrome | Firefox | Safari | Baseline | Note |
|---|---|---|---|---|---|
| **`command` / `commandfor`** (`show-modal`, `close`, `show-popover`, `hide-popover`, `toggle-popover`, `--custom` + `command` event) | 135 (2025-04-01) | 144 (2025-10-14) | 26.2 (2025-12-12) | **low 2025-12-12** | `request-close`: Chrome 139, Fx 144, Safari 26.2 |
| **`popover`** (`auto`, `manual`), `popovertarget`, `popovertargetaction`, light dismiss, top layer, `:popover-open` | 114 (2023-05-30) | 125 (2024-04-16) | 17 (2023-09-18) | **low 2025-01-27** (iOS 18.3 completed it); wide 2027-07-27 | |
| `popover="hint"` | 151 (133–150 old spec) | 153 (149–152 old spec) | no (TP 249) | not Baseline | |
| `interestfor` | 142 (2025-10-28) | no | no | not Baseline, non-standard | Mozilla neutral, WebKit waiting for re-evaluation (web search, 2026-10-04) |
| `showPopover({source})` | 137 | 144 | 26 | low | |

### CSS anchor positioning

| Feature | Chrome | Firefox | Safari | Note |
|---|---|---|---|---|
| `anchor-name`, `anchor()`, `anchor-size()`, `@position-try`, `anchor-center` | 125 (2024-05-14) | **147 (2026-01-13)** | 26 (2025-09-15) | |
| `position-area` | 129 | 147 | 26 | a single keyword (`block-end` alone) can overflow the viewport in Chrome 129–143 and Firefox 147 (BCD notes): use the two-keyword form (`block-end span-inline-end`, measured fine in Chromium 131 and 143) and set `margin: 0` (auto margins are zeroed only from Chrome 143 / Fx 148 / Safari 26.2) |
| `position-try-fallbacks` (`flip-block`, `flip-inline`) | 128 | 147 | 26 | |
| `position-anchor` | 125 | 147 | 26 | initial value differs per engine until Chrome 151 / Fx 151 / Safari 27 → always set it explicitly |
| implicit anchor from `popovertarget` | **133** | 147 | 26 | |
| implicit anchor from `commandfor` | 135 | 147 | 26.2 | |
| `position-visibility: anchor-valid / anchor-visible` | no | no | 27 | the reason web-features still lists the whole feature as "not Baseline" |

web-features: the feature as a whole is `baseline: false`; every key we need is `low` in `by_compat_key`. In
practice: usable in all three engines since Firefox 147. Interop 2026 lists it for reliability work.

Measured: WebKit "26.0" places the popover correctly but does **not** apply `flip-block` (popover rendered
below the viewport edge); WebKit "26.6" does. Chromium 131 has `position-area` but no implicit anchor: the
popover lands 34 px up and 120 px left of where it should; with an explicit `anchor-name` / `position-anchor`
pair it is exact (`out/b2024-chromium-explicit-anchor.json`).

### Entry / exit animation, sizes

| Feature | Chrome | Firefox | Safari | Baseline |
|---|---|---|---|---|
| `@starting-style` | 117 | 129 | 17.5 | low 2024-08-06 |
| `transition-behavior: allow-discrete` (parses) | 117 | 129 | 17.4 | low 2024-08-06 |
| transition of `display` | 117 | **no** (bug 1882408) | 18 | not Baseline |
| `overlay` (stay in the top layer while leaving) | 117 | no | no | Chrome only |
| `interpolate-size`, `calc-size()` | 129 | no | no | Chrome only; Mozilla position positive; WebKit patches in progress (PRs 74157, 74267) |

Measured exit animation of a top-layer element (`exit.js`, `exit2.js`, 200 ms transition, frames still rendered
after `close()` / `hidePopover()`):

| Engine | dialog | where it is painted while leaving | popover |
|---|---|---|---|
| Chrome 154 | 12 of 21 frames | in place (y 252 → 268) | 12 of 21 |
| WebKit "18.2", "26.0" | 12–13 of 21 | **out of the top layer at once**: y jumps 252 → 2295 (its in-flow position) | 13 of 21 |
| WebKit "26.6" | **0 of 20** | — | 0 of 20 (a plain element still transitions: 13 of 20) |
| Firefox | none (BCD) | UNVERIFIED locally | none (BCD) |

Entry animations are dependable everywhere; **exit animations of top-layer elements work only in Chromium**.
`CSS.supports('transition-behavior','allow-discrete')` is true in Firefox and proves nothing
(GoogleChrome/modern-web-guidance-src issue 1623).

### Details, selectors, the rest

| Feature | Chrome | Firefox | Safari | Baseline |
|---|---|---|---|---|
| `<details>` / `<summary>` | 12 | 49 | 6 | wide |
| `<details name>` (exclusive) | 120 | 130 | 17.2 | low 2024-09-03 |
| `::details-content` | 131 | 143 | 18.4 | low 2025-09-16 |
| find-in-page opens a closed `<details>` | 97 | 148 | 26.2 (partial) | not Baseline |
| `:has()` | 105 | 121 | 15.4 | wide 2026-06-19 |
| `inert` | 102 | 112 | 15.5 | wide 2025-10-11 |
| `:focus-visible` | 86 | 85 | 15.4 | wide |
| **`focusgroup`** | **150 (2026-06-30)** | no | no | not Baseline; not yet in the HTML spec (whatwg/html PR 11723). Mozilla positive, WebKit "concerns". No BCD key yet (BCD issue 30468). |
| `scrollbar-gutter` | 94 | 97 | 18.2 | low 2024-12-11 |
| `overscroll-behavior` on a non-scrollable box (the CSS-only modal scroll lock) | 144 | 150 | no | not Baseline |
| container size queries | 105 | 110 | 16 | wide 2025-08-14 |
| container style queries | 111 | 151 | 18 | low 2026-05-19 |
| `@scope` | 118 | 146 | 26.4 (17.4–26.3 buggy on inputs) | low 2026-03-24 |
| CSS nesting | 120 | 117 | 17.2 | wide 2026-06-11 |
| `light-dark()` | 123 | 120 | 17.5 | low 2024-05-13 (wide 2026-11-13) |
| `@layer` | 99 | 97 | 15.4 | wide |
| `scroll-initial-target` | 133 | no | no | Chrome only |

focusgroup, measured in Chrome 154 (`out/focusgroup.txt`): the IDL property is `focusGroup`. On plain buttons
`focusgroup="menu"` gives ArrowDown/ArrowUp with wrap, Home/End, one tab stop and last-focused memory, and infers
`menu`/`menuitem` roles. It gives **no type-ahead**, does not close anything on Tab, and **ignores items with
`tabindex="-1"`** (arrows do nothing). On a `[popover]` container the roles are not inferred (container stays
`group`), so they must be explicit. Adrian Roselli's tests (2026-07-05) list screen-reader problems and ask for
review before the design ossifies.

## 2. What the platform does without script (measured)

`pages/zero.html` + `pages/style.css`, no `<script>`. Full matrices: `out/matrix-zero.md`, `out/matrix-2024.md`.

| Check | Chrome 154 | WebKit "26.6" | WebKit "26.0" | Chromium 131 | WebKit "18.2" |
|---|---|---|---|---|---|
| Enter on `<button commandfor command="show-modal">` opens a modal | yes | yes | yes | **no** | **no** |
| Initial focus on the `autofocus` element | yes | yes | yes | — | — |
| Background not focusable | yes | yes | yes | — | — |
| Esc closes, focus returns to the button | yes | yes | yes | — | — |
| Click on the backdrop closes (`closedby="any"`) | yes | yes | **no** | — | — |
| `closedby="none"`: Esc does not close | yes | yes | **no** | — | — |
| `<form method="dialog">` button closes, sets `returnValue` | yes | yes | yes | yes | yes |
| Page behind does not scroll: `html:has(dialog:modal){overflow:hidden}` | locked | locked | locked | locked* | locked* |
| …with `overscroll-behavior: contain` instead | locked | scrolls | scrolls | scrolls | scrolls |
| `popovertarget` opens; focus on the `autofocus` item | yes | yes | yes | yes | yes |
| Popover under its button (implicit anchor) | exact | exact | exact | **off by 34/120 px** | **centred (no anchor positioning)** |
| Flips above when there is no room | yes | yes | **no** | yes | — |
| Esc closes + focus returns; click outside closes; a second popover closes the first | yes | yes | yes | yes | yes |
| Arrow keys / Home / End / type-ahead in `role="menu"` | **no** | **no** | **no** | **no** | **no** |
| Sidebar at 1200 px: visible, toggle hidden | yes | yes | yes | yes | yes |
| Sidebar at 400 px: hidden; toggle opens a drawer; Esc and outside tap close it; focus returns | yes | yes | yes | yes | yes |
| `<summary>` toggles a group; `name` closes the other | yes | yes | yes | yes | yes |
| Group height animates (`interpolate-size`) | yes | no (jumps) | no | yes | no |

\* measured with the modal opened by the 190 B shim (`pages/enh.html`).

Accessibility tree, Chrome 154 through CDP (`out/ax-chrome154.txt`), no ARIA written for these:
`button "v0.2" expanded=false` → `expanded=true` when its popover opens; `dialog "Search" modal=true` and the
rest of the page gone from the tree while it is open; `navigation "Docs"` present at desktop width although the
element carries `popover`. hidde.blog (updated 2025-03-15) reports the implicit `aria-expanded` in Chrome, Edge,
Firefox and Safari; Firefox and Safari are UNVERIFIED locally.

Not a defect: WebKit's Tab key skips links and buttons by default (a macOS Safari setting), so its Tab sequences
differ from Chrome's.

## 3. The components

### 3.1 Modal dialog — APG "Dialog (Modal)"

APG asks for: focus moves inside on open; Tab stays inside; Esc closes; focus returns to the invoker;
`role="dialog"`, `aria-modal="true"`, a label (w3.org/WAI/ARIA/apg/patterns/dialog-modal, fetched 2026-10-04).

**(a) Zero JavaScript**

```html
<button commandfor="search" command="show-modal">Search</button>

<dialog id="search" aria-labelledby="search-t">
  <form method="dialog"><button class="scrim" tabindex="-1" aria-hidden="true"></button></form>
  <div class="panel">
    <h2 id="search-t">Search</h2>
    <input type="search" aria-label="Query" autofocus>
    <form method="dialog"><button>Close</button></form>
  </div>
</dialog>
```
```css
html:has(dialog:modal) { overflow: hidden }      /* scroll lock */
html { scrollbar-gutter: stable }                /* no shift when the scrollbar goes */
dialog { opacity: 0; translate: 0 1rem; transition: opacity .2s, translate .2s, overlay .2s allow-discrete, display .2s allow-discrete }
dialog[open] { opacity: 1; translate: 0 0 }
@starting-style { dialog[open] { opacity: 0; translate: 0 1rem } }
```

The platform supplies role, modality, inert background, initial focus, Esc, focus return and the top layer.
The compiler supplies `aria-labelledby` (it knows the `$Title` slot) and the ids.

Light dismiss without `closedby`: the dialog is a transparent full-viewport layer, `.scrim` is a `position:fixed;
inset:0` submit button of a `method="dialog"` form, `.panel` is the visible box. Measured in Chrome 154,
Chromium 131, WebKit "26.6" and "18.2" (`out/backdrop.txt`): click inside keeps it open, click outside closes,
Esc closes, Enter in the input does not close. About 120 B of markup per dialog; no script, no Safari gap.

What zero JS gives up:

| Given up | Where | Matters? |
|---|---|---|
| Exit animation | everything but Chromium | cosmetic |
| `closedby="none"` (Esc must not close) | Safari ≤ 27.0 | only for a dialog that demands a decision |
| Opening by anything but a button in the same document (keyboard shortcut, URL hash, on load) | everywhere | not an APG requirement |
| The dialog at all | Chrome < 135, Firefox < 144, Safari < 26.2 | this is the floor decision |
| Tab cycles through the browser's own UI once per lap | everywhere (Chrome: `q > Close > BODY > q`) | by design of `<dialog>` |

**(b) Minimal JS for the gaps** (`exp-platform/js/`, sizes are esbuild 0.28.2 `--minify`; terser 5.51.2 gives the same ±5 B)

| Gap | File | min B | gzip B | Verified in |
|---|---|---:|---:|---|
| Open a modal where `command` is missing | `invoker-show-modal.js` | 190 | 169 | Chromium 131, WebKit "18.2" |
| Every built-in command + custom ones (when the compiler does not know which are used) | `invoker-full.js` | 580 | 333 | not run |
| `closedby="any"` by script instead of the scrim | `dialog-light-dismiss.js` | 270 | 215 | WebKit "26.0", "18.2", Chromium 131 |
| `closedby="none"` | `dialog-no-esc.js` | 139 | 142 | WebKit "26.0", "18.2", Chromium 131 |

```js
// invoker-show-modal.js — 190 B minified
'command' in HTMLButtonElement.prototype || addEventListener('click', (e) => {
  const b = e.target.closest('[command=show-modal]');
  b && document.getElementById(b.getAttribute('commandfor')).showModal();
});
```

For comparison, APG's own `dialog.js` is 10 756 B source, 4 611 B minified (`out/apg-sizes.md`).

**(c) Compile-time decisions that change the cost**

| Decision | Cost |
|---|---|
| Opened by a static button | 0 (`commandfor`) |
| Close buttons | 0 — emit `<form method="dialog">`, not `command="close"`: it works wherever `<dialog>` does (Baseline 2022) |
| Light dismiss wanted | 0 — emit the scrim only for dialogs that ask for it |
| `closedby="none"` used on this page | 139 B until Safari ships `closedby`; 0 otherwise |
| No form in the dialog | no `returnValue` plumbing; on a static page nothing reads it anyway |
| Not modal (a search panel that need not block the page) | `<dialog popover>` + `popovertarget`: 0 B and the floor drops to Baseline 2024; gives up the inert background |
| Opened from an island later (`specs/later/layout.md`, the `<template>` + `dialog.open()` bridge) | a different, scripted path; out of phase 2 |

### 3.2 Dropdown — APG "Disclosure Navigation Menu" or "Menu Button"

APG's navigation example says it "does not use the menu role because it does not provide the complex
functionality that assistive technologies expect"; arrow keys there are optional
(w3.org/WAI/ARIA/apg/patterns/disclosure/examples/disclosure-navigation, fetched 2026-10-04). The Menu Button
pattern requires `aria-haspopup`, `aria-expanded`, `role="menu"` and, inside the menu, ArrowDown/ArrowUp,
Home/End, Esc, Tab closing it, Enter activating and closing; type-ahead and arrows on the button are optional.

**(a) Zero JavaScript — a dropdown of links (version switcher, language, "more")**

```html
<button popovertarget="ver" style="anchor-name:--ver">v0.2</button>
<div popover id="ver" class="menu" style="position-anchor:--ver">
  <a href="/v0.2/" autofocus>v0.2 (current)</a>
  <a href="/v0.1/">v0.1</a>
</div>
```
```css
.menu { margin: 0; position-area: block-end span-inline-end; position-try-fallbacks: flip-block, flip-inline }
```

The platform supplies: open/close on Enter, Space and click; `aria-expanded` on the button; focus on the first
link (`autofocus`); the popover placed next in tab order; Esc with focus return; click-outside; one open at a
time; placement and flipping. The explicit anchor pair costs about 40 B per menu and removes the dependency on
implicit anchors (Chrome 133) and on `position-anchor`'s per-engine initial value.

What zero JS gives up: arrow keys between links (optional in APG); the popover stays open when Tab leaves it
(measured; the disclosure pattern does not require closing); opening on hover (`interestfor` is Chrome-only);
correct placement below Chrome 125 / Firefox 147 / Safari 26.

**(b) A dropdown of actions is `role="menu"` and needs script.** Emitting `role="menu"` without the keys
announces a widget that then does not behave like one, which is worse than a plain disclosure.

| Variant | File | min B | gzip B | Verified in |
|---|---|---:|---:|---|
| APG-required keys: arrows with wrap, Home/End, Tab closes, activation closes | `menu-keys-required.js` | 414 | 273 | Chrome 154 |
| + type-ahead, + ArrowDown/ArrowUp on the button | `menu-keys.js` | 801 | 452 | Chrome 154, Chromium 143/131, WebKit "26.6"/"26.0"/"18.2" |
| If `focusgroup="menu"` could be relied on: Tab closes, activation closes | `menu-keys-with-focusgroup.js` | 200 | 139 | Chrome 154 |
| Placement where anchor positioning is missing | `anchor-fallback.js` | 411 | 316 | WebKit "18.2" |

```js
// menu-keys-required.js — 414 B minified
addEventListener('keydown', (e) => {
  const m = e.target.closest('[role=menu]');
  if (!m) return;
  const items = [...m.querySelectorAll('[role^=menuitem]')];
  const i = items.indexOf(e.target);
  const n = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: -1 }[e.key];
  if (e.key == 'Tab') m.hidePopover();
  else if (n != null) { e.preventDefault(); items.at(n % items.length).focus(); }
});
addEventListener('click', (e) => {
  const m = e.target.closest('[role^=menuitem]')?.closest('[role=menu]');
  m && m.hidePopover();
});
```

Opening, Esc, light dismiss, focus return, `aria-expanded` and "focus the first item" stay the platform's; the
script owns only the keys. APG's `menu-button-actions.js` is 7 944 B source, 4 335 B minified.

focusgroup does not pay yet: it is Chrome-only, so the script ships anyway, and a page that uses both must drop
`tabindex="-1"` from the items (focusgroup ignores them) — which makes every item a tab stop in the other
engines. Gating the arrows on focusgroup saved 107 B (801 → 694) in the measurement.

**(c) Compile-time decisions**

| Decision | Cost |
|---|---|
| Items are links | 0 B, disclosure semantics |
| Items are actions | 414–801 B once per page that has such a menu; or 0 B if the design system declares it a disclosure of buttons (no `role="menu"`) |
| The trigger sits in a fixed header slot | placement can be plain `position: fixed` CSS computed from the layout: no anchor positioning, floor drops to Baseline 2024 |
| The trigger can be anywhere | anchor positioning; 411 B fallback below Firefox 147 / Safari 26 |
| Submenus | more script; keep them out of phase 2 |
| Open on hover | not available cross-browser; click only |

### 3.3 Side menu — a `<nav>` of links with disclosure groups

No APG pattern is named "sidebar". It is a navigation landmark with links, `aria-current="page"` on the current
one, and groups that follow the Disclosure pattern; `<summary>` exposes the expanded state itself.

**(a) Zero JavaScript**

```html
<button class="nav-toggle" popovertarget="nav" aria-label="Navigation">☰</button>
<nav id="nav" popover aria-label="Docs">
  <details name="g" open><summary>Guide</summary>
    <ul><li><a href="/intro" aria-current="page">Introduction</a></li><li><a href="/slots">Slots</a></li></ul></details>
  <details name="g"><summary>Reference</summary>
    <ul><li><a href="/cli">CLI</a></li></ul></details>
</nav>
```
```css
#nav { position: fixed; inset: 3rem auto 0 0; width: 16rem; margin: 0; border: 0; overflow: auto }
@media (min-width: 48rem) { #nav { display: block } .nav-toggle { display: none } main { margin-inline-start: 16rem } }
@media (max-width: 47.999rem) {
  #nav { inset-block-start: 0; translate: -100% 0; transition: translate .2s, display .2s allow-discrete, overlay .2s allow-discrete }
  #nav:popover-open { translate: 0 0 }
  @starting-style { #nav:popover-open { translate: -100% 0 } }
}
```

One `<nav>` serves both layouts. A closed popover is hidden only by a user-agent rule, so `display: block` at
desktop width shows it in place; on small screens it is a drawer with Esc, outside-tap close, focus return and
`aria-expanded` on the toggle. Measured in six engine builds including Chromium 131 and WebKit "18.2". A drawer
left open while the window is widened stays in the same spot (measured: `open=true x=0 y=48 w=256`).

Per page the compiler writes `aria-current="page"` on one link and `open` on the group that contains it. That
is the state a client-side router would otherwise compute at run time.

What zero JS gives up:

| Given up | Where | Matters? |
|---|---|---|
| The drawer is not modal: the page behind is not inert | everywhere | acceptable for navigation; outside tap closes it |
| Animated group height | everything but Chromium | cosmetic |
| Drawer slide-out animation | Firefox (BCD), WebKit "26.6" (measured) | cosmetic |
| Groups the reader opened, and the menu's scroll offset, are forgotten on the next page | everywhere | a choice: 270 B (`sidebar-state.js`) restores both; 80 B (`sidebar-reveal-current.js`) only scrolls the current link into view |
| The drawer itself | Chrome < 114, Firefox < 125, Safari < 17 | add `@supports not selector(:popover-open) { #nav { display: block; position: static } }` |

**(c) Compile-time decisions**

| Decision | Cost |
|---|---|
| Never collapses | no `popover`, no toggle button, less CSS |
| Collapses on small screens | 0 B (popover drawer) |
| Groups: fixed / collapsible / exclusive | none / `<details>` / `<details name>` — all 0 B |
| Current page and its open group | written into each page's HTML |
| Remember state across pages | 270 B, or the parked persistent-state layer later |

## 4. Bytes

Script per page, by floor and by what the page contains (`out/sizes.md`; minified, uncompressed):

| Page contains | Floor: Chrome 135 / Firefox 147 / Safari 26.2 | Floor: Baseline 2024 (Chrome 120 / Firefox 130 / Safari 18.2) |
|---|---:|---:|
| Side menu only | 0 | 0 |
| + dropdown of links | 0 | 0 if placed by layout CSS; 411 if anchored |
| + modal dialog (light dismiss by scrim) | 0 | 190 |
| + a `closedby="none"` dialog | 139 | 139 |
| + dropdown of actions | 414 – 801 | 414 – 801 |
| Everything above, largest variants | **940** | **1 541** |

The whole experiment page, three components and five popovers/dialogs, without script: HTML 2 512 B (982 gzip),
CSS 2 604 B minified (1 035 gzip) — side menu 1 014 B, dialog 672 B, dropdown 379 B (`out/css-sizes.md`).

React baseline for the same three components (`react-baseline/`, esbuild `--bundle --minify`, production):

| Bundle | min B | gzip B |
|---|---:|---:|
| react 19.3.0 + react-dom, "hello" | 223 500 | 69 091 |
| + @radix-ui/react-dialog 1.1.23, react-dropdown-menu 2.1.24, react-collapsible 1.1.20 | 318 307 | 101 556 |

## 5. Browser floor

Share of global usage at or above each floor (`usage.js`, caniuse data of 2026-09-30; 97.27 % of usage is
tracked). Chrome for Android is reported as one current version, so these are upper bounds; the Samsung
Internet and Opera version mapping is my assumption (UNVERIFIED).

| Floor | Share of all usage |
|---|---:|
| Chrome/Edge 135, Firefox 144, Safari 26.2 (invoker commands, Baseline 2025-12-12) | 84.69 % |
| same with Firefox 147 (anchor positioning) | 84.68 % |
| Chrome/Edge 120, Firefox 130, Safari 18.2 (Baseline 2024) | 89.50 % |
| Chrome/Edge 105, Firefox 121, Safari 15.4 (`<dialog>` + `:has()`) | 93.82 % |

Most of the 5 points between the first and third rows are iOS 18 (ios_saf 18.5–18.7 = 1.79 %) and desktop
Chrome 119–134.

## 6. What this says about the bet

The platform removed the script; the compiler's part is choosing the cheapest correct markup, and each choice
needs knowledge a bundler does not have:

| The compiler knows | So it emits |
|---|---|
| a `Dropdown`'s items are all links | a disclosure popover, no `role="menu"`, no script |
| which dialogs ask for light dismiss or forbid Esc | the scrim, or the 139 B shim, only on those pages |
| the `$Title` slot of a dialog | `aria-labelledby` and its id |
| trigger and popover belong to one component | a unique `anchor-name` / `position-anchor` pair, ids for `popovertarget` |
| the page being built | `aria-current`, the open group, per-page CSS without unused components |
| the declared floor | exactly the shims that floor needs, feature-tested in one expression each |

Every shim in `js/` is an independent top-level statement with its own feature test, so a per-page script is a
concatenation of the ones that page needs: no module graph, no runtime.

## Recommendation

1. **Floor for phase 2: Chrome/Edge 135, Firefox 147, Safari 26.2.** Call it "Baseline 2025 + anchor
   positioning". All three engines have been past it since 2026-01-13, it covers about 84.7 % of global usage,
   and for a documentation site read by developers the real share is higher. At this floor the phase-2 site
   ships **no JavaScript for layout**.
2. **Build the three components on `<dialog>` + `commandfor`, `popover` + `popovertarget`, `<details name>` and
   anchor positioning with explicit anchor names.** Use `popovertarget` for popovers and `commandfor` only for
   `show-modal`; close with `<form method="dialog">`; do light dismiss with the scrim, not `closedby`.
3. **Make "links or actions" a property of the dropdown component that the compiler can read.** Links compile
   to a disclosure with no script. For phase 2 ship only that; add the action menu (414–801 B) when a page
   needs one.
4. **Treat as progressive enhancement, never as a requirement:** exit animations (`overlay`, `display`
   transitions), `interpolate-size`, `::details-content` height animation, `focusgroup`, `popover="hint"`,
   `interestfor`, `scroll-initial-target`, the `overscroll-behavior` scroll lock. Use `html:has(dialog:modal)`
   for the scroll lock and `[open]` rather than `:open`.
5. **Keep the floor a compiler input.** Going down to Baseline 2024 is 190 B (open a modal) + 411 B (place an
   anchored dropdown) and about 5 points of usage; the shims are written and tested. Do not go below
   Chrome 114 / Firefox 125 / Safari 17 (no popover): that needs a different sidebar and dropdown.
6. **Before relying on it, run the matrix in Firefox 147+ and in Safari 26.2+/27 by hand** (`pages/zero.html`
   opens from disk); those two are the unverified part of this report.

## Open questions

1. Firefox: does the zero-JS page pass the same checks in Firefox 147–157 (anchored placement and flipping,
   `autofocus` inside a popover, focus return)? UNVERIFIED locally; BCD says the features are there.
2. Safari 27.0.1: is `closedby` really absent (BCD: preview only), and does a top-layer exit transition run or
   not? The WebKit "26.6" build disagrees with the "26.0" build on the second point.
3. The scrim makes the `<dialog>` a full-viewport transparent box. Does that sit well with the design system's
   dialog sizing and with a scrolling panel? Alternative: keep `closedby="any"` and ship the 270 B shim to Safari
   until it lands.
4. Is a non-modal drawer acceptable for the side menu on small screens, or must it be modal (then it is a
   `<dialog>` on mobile and a `<nav>` on desktop: two elements or a 190 B-class script)?
5. Should phase 2 remember open groups and the menu's scroll offset across pages (270 B), or is the
   compile-time state enough? It touches the parked persistent-state layer.
6. Does the action menu belong in phase 2 at all? A docs site can do without it (theme switch and "copy link"
   are single buttons).
7. Where do the shims live: inline in each page (no request; at most 1.5 KB) or one cached file? With 3–4 pages
   and "minimum JS chunked per page" inline looks right, but that is the builder's measurement to make.
8. A keyboard shortcut for search (`/`, Cmd+K) is script by nature (about 100 B, not written here);
   `accesskey` is the zero-JS alternative with browser-specific modifiers.

## Verification (independent)

Skeptic pass, 2026-10-04. Experiments: `exp-platform-verify/` next to this file (scripts at the top level,
outputs in `out/`). The author's text above is unchanged.

**What I could run.** Chrome **154.0.8037.93** (installed), Chromium **143.0.7499.4** and WebKit build **"26.0"**
(Playwright 1.57 builds already in `~/Library/Caches/ms-playwright`), and — new — the installed **Safari 27.0.1**,
by serving a self-testing page from `127.0.0.1` and opening it with `open -g -a Safari` (`self/sync.html`,
result in `out/self-safari2701-sync.json`). Safari's page was `visibilityState: hidden` (display asleep), so that
run is script-driven (`button.click()`, no real keys, no timers, no animation frames).
**What I could not run.** Firefox (the sandbox blocks its child processes; running outside the sandbox was denied
again), Chromium 131, WebKit "18.2" and "26.6" (the author deleted those builds; not re-downloaded). Safari
WebDriver is off ("Allow remote automation" disabled) and I did not change that.
Side effect to know about: four tabs on `http://127.0.0.1:479x/` were left open in Safari; they are dead and safe to close.

Data: `web-features` 3.40.1 and `@mdn/browser-compat-data` 8.1.4 re-fetched from unpkg; both are the npm `latest`
(published 2026-10-01) and byte-identical to the author's copies (same SHA-1).

### Verdict per claim

| # | Claim | Verdict | What I checked |
|---|---|---|---|
| 1 | Invoker commands Baseline 2025-12-12 (135 / 144 / 26.2) | **confirmed** | web-features `invoker-commands`: `low 2025-12-12`, 135/144/26.2; release dates 2025-04-01 / 2025-10-14 / 2025-12-12 from `browsers.*.releases`. MDN "Firefox 144 for developers" lists `command`/`commandfor` (2025-10-14). Safari 27.0.1: `'command' in HTMLButtonElement.prototype` true and a click on the `command="show-modal"` button opens a `:modal` dialog. Chromium 131 / WebKit 18.2 absence not re-run. |
| 2 | `closedby` not Baseline, absent from Safari 27.0 | **confirmed, now measured** | BCD `safari=preview`, `safari_ios=no`; TP 249 post (2026-07-29) says "Added support for close watchers, including the `closedby` attribute"; the Safari 27.0 post (2026-09-17) does not mention it. **Safari 27.0.1: `'closedBy' in HTMLDialogElement.prototype === false`** — the part the author marked "not checked". Confidence can go from medium to high. |
| 3 | Scrim button gives light dismiss with no script | **partly** | The author's page reproduces in Chrome 154, Chromium 143, WebKit "26.0" (`out/backdrop-rerun.txt`) and in Safari 27.0.1 (script click on the scrim closes; focus returns to the invoker). **But it depends on the dialog having an `autofocus` element.** See correction A. |
| 4 | Zero-JS `<dialog>` + `command` meets the APG modal pattern | **confirmed** (Firefox unverifiable) | `run.js zero.html` re-run: identical results in Chrome 154 (including the AX tree `dialog "Search" modal=true`), Chromium 143, WebKit "26.0". Safari 27.0.1: opens, initial focus on the `autofocus` input, focus returns to the invoker (Esc, inertness and the AX tree not testable there). |
| 5 | `html:has(dialog:modal){overflow:hidden}` locks scroll; `overscroll-behavior` only Chrome 144+ | **confirmed** | Re-run: locked in Chrome 154, Chromium 143, WebKit "26.0"; with `overscroll-behavior` Chrome 154 locked, Chromium 143 and WebKit scroll to 400. Extra variants (`lock.js`, `out/lock.txt`): wheel over the dialog itself, PageDown/End/ArrowDown/Space, and a page already scrolled to 500 — no movement, position kept. Safari 27.0.1: computed `overflow: hidden` on `<html>` while modal. Touch scrolling on iOS and Firefox: not tested by anyone. |
| 6 | Core anchor positioning in all three engines since Firefox 147 | **partly** | Data confirmed (keys `low 2026-01-13`, 125–129 / 147 / 26; MDN "Firefox 147": "CSS anchor positioning is now enabled by default", 2026-01-13). Two corrections, B and C below. Chromium 131 numbers not re-run. |
| 7 | Dropdown of links needs zero JS | **confirmed** (Firefox unverifiable) | Re-run identical in Chrome 154, Chromium 143, WebKit "26.0"; Chrome AX `expanded=false → true`. Safari 27.0.1: opens, focus on the `autofocus` link, placed exactly (`dTop=0 dLeft=0`) with explicit and with implicit anchors. hidde.blog (updated 2025-03-15) does list Edge, Chrome, Firefox and Safari for the implicit `aria-expanded`. Caveat: correction D. |
| 8 | Action menu is the only scripted component: 414 B / 801 B | **confirmed** | Re-minified with esbuild 0.28.2: 414 B (273 gzip), 801 B (452 gzip), all other sizes equal, gzip within 3 B. The 414 B script, which the author ran only in Chrome 154, also passes arrows, wrap, Home/End, Tab-closes and activation-closes in WebKit "26.0" and Chromium 143 (`out/enhreq-*.json`). Without script no arrow key moves focus in any of the three engines. |
| 9 | `focusgroup`: Chrome 150 only, gaps as listed | **confirmed** | Chrome blog 2026-06-30; web-features issue 4330 "focusgroup shipped in Chromium 150"; Roselli post dated 2026-07-05 (updated 07-14). `fg2.js` / `fg3.js` re-run in Chrome 154: same output (no type-ahead, `tabindex="-1"` items ignored, popover container stays `group`). `focusGroup` absent in Chromium 143 and in Safari 27.0.1. HTML-spec status not checked. |
| 10 | Side menu needs zero JS (`<nav popover>` + `<details name>`) | **partly** | The mechanics reproduce in Chrome 154, Chromium 143, WebKit "26.0"; Safari 27.0.1 shows the closed `nav[popover]` with `display:block` (256 px wide box) and `<details name>` exclusivity. Chrome AX at 400 px with the drawer open: `navigation "Docs"`, toggle `expanded=true` (`out/ax.txt`). "Zero JS" does not survive corrections D and E. |
| 11 | Exit animations of top-layer elements work only in Chromium | **partly — too strong** | See correction F. BCD facts (`overlay`, `interpolate-size` Chrome-only; Firefox no `display` transition; bug 1882408 still open per GoogleChrome/modern-web-guidance-src issue 1623, 2026-10-01) confirmed; Safari 27.0.1 `CSS.supports('overlay','auto')` false. |
| 12 | Baseline 2024 floor = 601 B for about 5 points | **partly** | 190 + 411 B re-measured; `usage.js` re-run gives 84.69 % and 89.50 %. Cross-check: caniuse's own `css-anchor-positioning` is 85.92 % (full + partial). The shims were not re-run (no Chromium 131 / WebKit 18.2). The floor itself is mis-stated for iOS: correction E. |
| 13 | React 19.3 + Radix = 318 307 B / 101 556 B gzip | **confirmed** | Rebuilt with the same command: 318 354 B / 101 546 B gzip (47 B off); react-only 222 767 / 68 881 (report: 223 500 / 69 091). Versions equal npm `latest` today. `zero.html` 2 512 B / 982 gzip, CSS 2 603 B / 1 032 gzip. What the number proves is limited: see "Missed" 1. |
| 14 | Popover Baseline 2025-01-27; `hint` and `interestfor` not usable | **confirmed** | web-features `popover` `low 2025-01-27`; BCD `popover.hint` 151 / 153 / preview, `interestfor` 142 / no / no. Safari 27.0.1: `popover="hint"` rejected, no `interestForElement`. Detail: web-features gives Chrome **116** for the feature (the `togglePopover()` return value); the attribute itself is 114 as the report says. |

### Corrections

**A. The scrim takes initial focus when the dialog has no `autofocus` element.** `scrim.js` → `out/scrim.txt`;
same in Chrome 154, Chromium 143, WebKit "26.0", Safari 27.0.1:

| Variant | Initial focus | Space right after opening | Outside click closes |
|---|---|---|---|
| report: scrim first, `autofocus` on the input | input | stays open | yes |
| scrim first, no `autofocus` | **the scrim** (`aria-hidden`, invisible) | **closes the dialog** | yes |
| scrim first, no focusable content | **the scrim** | **closes** | yes |
| **scrim last in the dialog**, panel `z-index: 1`, no `autofocus` | first real control | (activates that control) | yes |
| scrim `inert` | first real control | — | **no** (defeats the purpose) |

`<dialog>` focuses its first focusable descendant, and `tabindex="-1"` does not exclude one. Chrome's AX tree then
shows `button "" focused=true` inside the dialog (`out/ax.txt`). Rule for the compiler: emit the scrim **after**
the panel, and put `autofocus` on the dialog itself when the panel has nothing focusable. Dragging a text
selection from the panel out over the scrim does not close the dialog (measured).

**B. `position-anchor` is not a "125–129 / 147 / 26" key.** web-features lists `css.properties.position-anchor`
as `low 2026-09-14` (Chrome 151, Firefox 151, Safari 27); earlier versions are "partial" because the initial
value differs. The report's table says so and the recommended markup always sets it, so nothing breaks — but
the sentence "every needed key is `low` 125–129 / 147 / 26" is wrong for this one key.

**C. The WebKit "26.0" flip failure is narrower than stated, and has a workaround.** `flip.js` → `out/flip.txt`:

| Placement CSS | WebKit "26.0", page taller than the viewport | WebKit "26.0", page not scrollable | Chrome 154 | Safari 27.0.1 |
|---|---|---|---|---|
| `position-area: block-end span-inline-end` + `position-try-fallbacks: flip-block` (also with `@position-try`, also with an explicit fallback area) | **overflows** (menu 696–784 in a 700 px viewport) | flips | flips | flips |
| `top: anchor(bottom); left: anchor(left)` + `position-try-fallbacks: flip-block` | flips | flips | flips | flips |
| `flip-inline` at the right edge, `position-area` | flips | — | flips | — |

So the bug is "block-axis fallback of a `position-area` box is not tried when the document scrolls", which is
every docs page. Safari 27.0.1 does not have it. Whether Safari 26.2–26.6 (the floor) have it is **UNVERIFIED**:
either emit the `anchor()` inset form, which flips in every build tested, or test a real Safari 26.x first.

**D. Back/forward cache restores open overlays.** `bf/run.js` → `out/bfcache.txt`, Chrome 154 with bfcache on,
pages served over HTTP: open the drawer, follow a link inside it, press Back →
`{"persisted":true,"nav":true}`: the previous page comes back **with the drawer open**. Same for the dropdown
(`"ver":true`) and for a modal dialog (`"dlg":true,"modal":true,"htmlOverflow":"hidden"`). Every navigation on
the docs site starts inside one of these. The fix is script: closing them on `pagehide` is **176 B minified
(140 gzip)** and was verified (all three `false` after Back, the reader's opened `<details>` still open).
Playwright's WebKit does not use the page cache (`persisted:false`), so Safari and Firefox are UNVERIFIED; both
ship a back/forward cache. Also measured: a same-page `#hash` link inside the open drawer scrolls the page and
leaves the drawer open (Chrome 154, WebKit "26.0").

**E. On iOS 17.0–18.2 a popover does not close on an outside tap** (BCD note on `api.HTMLElement.popover`,
webkit.org/b/267688; it is why web-features dates popover to iOS 18.3). The report's "Baseline 2024" floor is
"Safari 18.2", and its drawer has no close button and covers its own toggle (`inset-block-start: 0`): on iOS
18.0–18.2 the only way out is to follow a link. The Baseline-2024 floor must read iOS **18.3**, and the drawer
should carry a `<button popovertarget="nav" popovertargetaction="hide">` at any floor (0 B; also what a
touch screen-reader user needs).

**F. Exit animations.** `self/test.html` in the WebKit "26.0" build (`out/self-webkit260.json`):

| Element | Frames still rendered after hide | Painted where |
|---|---|---|
| popover | 13 of 21 | in place (y 334) |
| `<dialog>`, default styles | 12 of 20 | jumps to y 2264 |
| `<dialog>` with `position: fixed` | 12 of 20 | **in place (y 322)** |

The jump is not the top layer as such: the UA sheet makes only `dialog:modal` fixed, and a closed dialog falls
back to `position: absolute`. With `position: fixed` on the dialog (the scrim design is a full-viewport box
anyway) the WebKit "18.2"/"26.0" behaviour is a correct fade in place; what is lost without `overlay` is top-layer
stacking and the `::backdrop` during those 200 ms. The author's own `out/exit.txt` already shows the drawer
sliding out in place in those two builds. The claim therefore reduces to: Firefox no; WebKit "26.6" build 0
frames (not re-run); **shipping Safari 27 unknown** — I could not measure it either (hidden page, no frames).
The recommendation "treat exit animations as enhancement" is unaffected.

### Missed

1. **The React + Radix number is not the baseline that tests the bet.** The zero-JS markup is plain HTML; a
   hand-written page or any static generator can emit it. What this report measures the *compiler* contributing
   is the selection of at most about 1.5 KB of shims, ids and anchor pairs, `aria-current`, and per-page CSS.
   The comparison that decides "ridiculously optimize" is against multi-page docs generators, not against a
   client-rendered React app.
2. **Below the floor the dialog fails hard**, not gracefully: the button does nothing (about 15 % of global
   usage by the report's own figures). The 190 B shim is feature-tested and inert at the floor, so "floor as a
   compiler input" should default to shipping it.
3. **Inline `style="anchor-name:…"` needs `style-src-attr 'unsafe-inline'`** under a strict CSP, and inline
   shims need hashes. The compiler can emit the anchor pairs as id rules in the page's CSS file instead.
4. Hidde's page says the "popover follows its invoker in tab order" behaviour applies to desktop browsers, not
   mobile; with a touch screen reader the drawer's content is wherever it sits in the DOM. Put the `<nav>` right
   after its toggle.
5. `anchor-fallback.js` positions once, in page coordinates: under the report's own sticky header the menu
   scrolls away from its button. Acceptable for a fallback, but it is not "placement" parity.

### Does the recommendation hold?

Yes for the floor, the element choices and the list of enhancements — with these amendments before it becomes a spec:

1. Scrim **after** the panel; `autofocus` on the dialog when the panel has no focusable element (A).
2. Budget **176 B per page** that has a drawer, dropdown or dialog for the `pagehide` close, or decide explicitly
   that Back may show an open overlay (D). "No JavaScript for layout" is true for opening, closing, focus and
   placement, not for this.
3. Place menus with `anchor()` insets + `flip-block`, or verify `position-area` flipping in Safari 26.2–26.6 (C).
4. A close button inside the drawer; Baseline-2024 floor means iOS 18.3 (E).
5. Still unverified after this pass: **every Firefox behaviour**, Safari 26.2–26.6, real key/pointer input and
   exit animations in Safari 27, touch scroll lock on iOS, bfcache in Safari and Firefox.
