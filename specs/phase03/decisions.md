# Phase 3 decisions — the Iota Viewport Model

A review of [ivm.md](ivm.md) (the owner's draft, v0.1, 2026-10-11) against
what phase 2 built and measured, made on 2026-10-11. A numbered decision
below is **for review** — proposed here, ruled by nobody — unless its row
carries a ruling: the owner ruled the draft's §16 the same day (*The draft's
§16, ruled*). Where a decision goes against something the draft marks
**Agreed**, the row says so and says on what evidence.

What this review rests on, and what it does not:

- Read: the draft, whole; `specs/phase02/` and `specs/later/layout.md`;
  `packages/ui`, `packages/core`, the builder's `cssprune`, `pagecheck`,
  `behaviors`; `bench/results/`.
- Looked up: CSS Anchor Positioning 1 (the editor's draft, 2026-10-11),
  Floating UI's docs, bundlephobia for `@floating-ui/dom` 1.8.0.
- **Not inspected: the playground.** Its URL answers 401. Every *Prototype
  policy* is known from the draft's prose only.
- **Nothing was run in a browser.** Each claim about a browser that phase 2
  did not already measure is in *Not verified*, at the end, and is a claim
  until it is.

## Ruled by the owner (2026-10-11)

| | Ruling | Here |
| --- | --- | --- |
| the design system's name | **Iota** — "Small by design" | phase 2's decision 15 ("a branded name the owner will pick") is answered |
| its layers | `core` — design tokens; `bits` — internal building blocks and utils; `ui` — React components | *For the owner*, J: `core` is also `@reactogenic/core` |
| its layout backbone | the Iota Viewport Model, [ivm.md](ivm.md) | this file |
| this review | may recommend against an item the draft marks Agreed, where the repo's evidence contradicts it; the owner rules on each | decisions 2, 3, 4, 6 |
| the draft's §16, all fourteen rows | ruled, after this review was read: *The draft's §16, ruled*, below | 9 (in part), 13, 14: replaced; 10, 12, 16: in part; G10's recommendation: replaced; five questions follow from them (*For the owner*, M–Q) |
| those five, M–Q | ruled, the same day: *For the owner* | 12 (M), 9 (N), 2 in part (O), a spec of its own for interaction (P), 14 again (Q: physical names only, for now) |

## The finding: three choices, not one

The draft describes one thing — a runtime that clones templates into a layer
and places them with Floating UI. It is three choices that do not depend on
each other, and phase 2's evidence says something different about each.

| | The choice | The draft (Agreed) | The evidence | Recommended |
| --- | --- | --- | --- | --- |
| 1 | **Where an overlay is rendered** | in `StackingLayer`, never in the browser's top layer (§2.2) | settles nothing either way: *What the exclusion buys and costs*, below | not Agreed yet: decided by a measured comparison (decision 4; *For the owner*, A) |
| 2 | **Who computes its geometry** | script: Floating UI for collisions (§5.3), a layout transaction for the rest (§13) | against. At the floor CSS anchor positioning flips, shifts and matches sizes with no script, and phase 2 checks it in two engines (`packages/ui/test/browser/run.mjs`). `@floating-ui/dom` 1.8.0 is 21,331 B minified, 8,209 B gzip, whole (bundlephobia, 2026-10-11; the subset a page needs: not measured) — T2 refutes above 5 KB brotli, "a micro-runtime, not compilation" (`phase02/plan.md`, RGP2-050) | **CSS first**; script for the cases named below, and no others (decision 2) |
| 3 | **How its content reaches the page** | a `<template>`, cloned per open (§11.1) | against, for shell pages. A page with a `<template>` is not pruned (`cssprune/prune.go:455`); a script that names `cloneNode`, `append` or `remove` is not either (`cssprune/match.go:524`). And what is in a template is not on the page: not painted first, not found, not crawled | **live elements, put in the layer at build time**; a template only where there are holes (decision 3) |

The model — two layers' worth of vocabulary, nine and twelve anchors, main
and cross, thresholds, reservation, one ordered stack — survives all three
choices. It is the contract; how a use of it reaches the browser is a
realization, chosen per use by what that use needs. This is phase 2's rule
for CSS and JS (`phase02/builder.md`, *Analysis and packaging*) said of
layout.

### What the exclusion buys and costs (choice 1)

| | With `StackingLayer` | With the top layer (as built) |
| --- | --- | --- |
| paint order | later sibling above earlier | later shown above earlier — the same rule |
| an entry above a modal that is not its descendant (a toast) | interactive | inert: a modal dialog blocks everything but its own subtree |
| leaving animation | every engine: the element is an ordinary one | Chromium only (`phase02/research/platform.md`, *Entry / exit animation*) |
| a dialog under a closed popover (phase 2, *For the owner*, H) | cannot happen | a documented limit |
| Tab leaves the open drawer; a drawer left open when the window widens (`phase02/components.md`, *Known limits*) | closed by the model | limits |
| light dismiss | the model's own, one rule | per element: `closedby` is not in Safari, a popover's backdrop lets the tap through — two scrim buttons (`dialog.rtsx`, `side-menu.rtsx`) |
| opening, closing, Esc, focus in and back | **script, always** | the browser: 0 B at the floor (phase 2, decision 11) |
| a reader without script | nothing opens: no drawer on a small screen | everything opens |
| modality | `inert` written by script, and tested by us | the browser's |
| a popup's place in Tab order; `aria-expanded` on its invoker | ours to rebuild (A2, A3) | the browser's |
| what the page's own content does | an author's `<dialog>`, a fullscreen element, a `<select>`'s picker are in the top layer all the same: **above every Iota overlay**, and a modal one makes the whole layer inert | one order for all |
| what phase 2 has | every browser probe is native state (`:modal`, `:popover-open`: `run.mjs:91`): rewritten | stands |

Three things are bought — toasts above a modal, leaving animations
everywhere, one rule for dismissal — and the price is that script becomes
the precondition of every overlay. Neither column is free, and no number in
this repo says which is smaller: that is what decision 4 asks to be measured.

### The model in CSS at the floor (choice 2)

Floor: Chrome/Edge 135, Firefox 147, Safari 26.2 (phase 2, decision 11).

| § | Rule | CSS at the floor | Script when |
| --- | --- | --- | --- |
| 4 | inset placement, offsets | `position: fixed` and insets; against an element, `anchor()` | — |
| 5.1 | outset placement | `anchor()` insets, as `dropdown-menu.css` does | the reference is a point or a range (a context menu) |
| 5.2 | cross matching | `anchor-size()` | — |
| 5.3 | preferred with fallback; automatic | `position-try-fallbacks`; `position-try-order` | more than five fallbacks: Chromium tries five (`phase02/components.md`, *DropdownMenu*) |
| 6.2 | a threshold | a query, when intended size and threshold are compile-time values: `intended / parent ≥ t` ⇔ `parent ≤ intended / t` — the builder writes the breakpoint, exact at `>=` (ruled: *For the owner*, M) | intended size is measured from content — then with enter and leave thresholds |
| 6.3 | outset → inset when it fills | the same query swaps the rule set | — |
| 7.2 | grant or decline, one side | a query at `panel + base minimum + margins`; the base's inset under `:root:has(…)` of the open state | a size measured at run time; a panel the reader resizes |
| 7.3 | the shared corner | a grid — **with a fixed edge priority only** | opening order (decision 13) |
| 8 | modality | with the top layer: nested `showModal()` *is* the counted claim. Without it: nothing | without the top layer, always |
| 9.3, 9.4 | Slide; Scale-Up with no trigger | `@starting-style` and a transition; the leaving half in every engine once the element is not in the top layer | — |
| 9.2 | Scale-Up from the trigger | `anchor()` and `anchor-size()` in `@starting-style` — if they interpolate: **not verified** | otherwise: sample the rectangle, set four custom properties |
| 10 | the trigger inside its viewport | `<details>` holds the state | Esc to collapse; modal when expanded |
| 11 | templates | not needed | holes |
| 13 | the layout transaction | the browser's layout | — |

What cannot be met without script, in full: opening order as a policy (§7.3);
a threshold or a reservation whose input is measured at run time; an
unrelated interactive entry above a modal; a leaving animation in the top
layer outside Chromium; a pointer or range as reference; hover intent; state
kept across documents; a toast's life; menu keys (shipped: `menu-keys`).

One line of the draft forecloses the table. §2.2 makes a floating root
`position: absolute` in a `position: fixed` layer. An anchor must have the
positioned element's containing block among its own (CSS Anchor
Positioning 1, *acceptable anchor element*); the layer is not an ancestor of
anything in the base, so **no element of the base can anchor a floating
root**. A root that is `position: fixed` itself has the viewport for
containing block, and the base's elements are acceptable. Not verified
(*Not verified*, 1); decision 5 whichever way choice 1 goes.

### Delivery (choice 3)

`later/layout.md` (*Delivery*) already has the template, cloned per open,
for one case: a shell component driven from a dynamic segment, its slot
bodies holding holes. Phase 2's decision 17 kept the live element for a
dialog written in shell code — provisional, "for test purposes only", until
the owner's "portal". The draft's §11.1 makes the template the only
delivery. What that costs a shell page:

| | With a template | With a live element in the layer |
| --- | --- | --- |
| CSS pruning | off for the page, twice over: the element, and the script that clones it | on: the layer is part of the page as served |
| first paint | no sidebar, no reserved margin; then the page shifts by the panel's width | as built |
| before the module runs; with no script | every opener is dead; no navigation | as the realization allows |
| find-in-page, `#fragment`, a crawler | the drawer's links are not in the document | they are |
| a cross-document view transition | captures "no sidebar", then it appears — against "identical across documents" (`CLAUDE.md`, *Architecture decisions*) | the shell is the same in both documents |
| ids | repeated per clone, or rewritten — and every reference with them (F6) | the page's own, checked by `pagecheck` |
| `mount()` | cannot reach it: an id in a template is `mount-no-element` (`behaviors.go:447`) | by id, as built |

So "portal" is a **build-time** move for shell code: the compiler puts the
element in the layer; nothing is cloned in the browser. That is a builder
feature — today a page's HTML is "React's, to the byte" (`build/html.go`) —
and the first task the rulings would open.

## Decisions

**For review**, but where a row says *replaced by the owner* or *ruled so*.
*Against Agreed* marks a row that reverses or narrows an item the draft
agreed.

| # | Decision | Why | Reverse if |
| --- | --- | --- | --- |
| 1 | **The model is the contract; a use of it is realized by what it needs** — CSS alone, CSS and a behaviour, never a generic runtime. Analysis says what a page's viewports are; packaging how they reach the browser | phase 2's rule for CSS and JS; T1 ("0 bytes … of any generic runtime") stays true by construction | a use turns up whose realization cannot be chosen at build time |
| 2 | **Geometry is CSS wherever its inputs are compile-time values. Floating UI is not a dependency of that path** — *against Agreed* (§5.3). Script computes geometry only for: a point or range reference, a measured intended size, a reader-resized panel | choice 2, above. The inputs *are* compile-time in shell code: "no browser-time variance". And §16's ruling 13 (`position-visibility`) works on an anchor-positioned element only: **native hiding is ruled (*For the owner*, O), and an outset viewport with an element for reference is CSS-anchored by it**; the rest of this row is for review | the subset measured (*Not verified*, 4) fits a budget the owner sets, and CSS fails a browser check that script passes |
| 3 | **In shell code a floating viewport is a live element, placed in the layer by the builder. A template only where there are holes** — *against Agreed* (§11.1) | choice 3, above | the pruner learns to match a template's content where it will be cloned (`phase02/builder.md`, *Not in phase 2*), and first paint is solved another way |
| 4 | **Whether Iota's components leave the top layer is decided by a measured comparison, not agreed beforehand** — *against Agreed* (§2.2): `Dialog`, `DropdownMenu`, `SideMenu` built both ways, with decisions 2 and 3 in both, against thresholds stated first (*For the owner*, A) | the table of choice 1: three gains, one categorical cost, no number | the owner rules without it |
| 5 | **A floating root is `position: fixed`; the layer is a direct child of `<body>`, unpositioned, `isolation: isolate`** — a change to §2.2's CSS. A page check holds the second half | the anchor rule, above; and a transform, filter or containment on an ancestor of the layer re-bases every "fixed" in it | the anchor rule is read wrongly (*Not verified*, 1) |
| 6 | **Chrome that reserves space is base content while it is docked; only its overlay form is a floating viewport** — *against Agreed* (§7.2, "granted panels remain in `StackingLayer`") | A1: in the layer, primary navigation comes after all content in Tab and reading order. F1: it must exist at first paint. Phase 2's one `<nav>`, two presentations, already is this | a repair of focus and reading order is specified, and the owner prefers it to one element in two places |
| 7 | **The document scrolls.** A side reservation is the root's padding; a top or bottom one is a fixed inset. The reserved insets are custom properties on `:root` (`--iota-inset-*`), and `scroll-padding` follows them | S1: a base that scrolls inside itself loses toolbar collapse, printing, scroll restoration. S4: without `scroll-padding` a `#fragment` lands under the strip. Custom properties are what a print rule and the base's own CSS can read | an application shell (not a document) is Iota's first target |
| 8 | **Modality is a function of resolved state**: `modal: boolean \| "when-overlay" \| "when-filled"`. So is being dismissible. A viewport that fills both axes covers the base: the base is inert under it | R1: a docked sidebar that becomes a drawer is inexpressible while `modal` cannot change (§8.3) — and it is the first component. A7 | — |
| 9 | ~~Inertness is derived, not counted: a target is blocked iff a modal is above it~~ — **replaced by the owner (2026-10-11; §16, 7 and 8): the stack is of modal viewports only, push and pop; a modal is immutable until it is at the top.** Counted claims stand, each modal owning the list of what it blocked: **a modal whose reference is lost, or that is unmounted, leaves the stack wherever it stands (*For the owner*, N)**. What is left of this row, for review: entries have a **band** (chrome, transient, always-on-top) for paint order; the **owner** of an entry is the interaction spec's (§16, 12) | M1: the count equals the comparison under LIFO and is wrong outside it; the comparison is right everywhere, for ten entries. Ownership is what close, outside-press and Esc must follow (M3). A band is what a toast needs (M2). Dissolves §16's *Stack removal* and *Modal mutation* | — |
| 10 | **Interaction state changes when closing starts**: claims released, focus returned, the closing root inert. The animation is visual only — for the reservation, **ruled so (§16, 6)**; for the claims, for review; focus is not the model's (§16, 12) | M5: as drafted (§11.2) the base is inert, and the fading dialog clickable, for the animation's length | — |
| 11 | **Size is physical, or the content's; main and cross are for offsets, thresholds and a strip's thickness.** Sizes are CSS lengths (`rem`, tokens), with `fill` as a value, and minimum and maximum | G1: a `{main: 300, cross: 200}` menu moved from `bottom` to `right` is either rotated or was never main/cross. G3: px numbers in mount data take tokens out of CSS | — |
| 12 | **One resolution order, no iteration**: thresholds once, on the requested placement's axes → a viewport that fills skips collision → collision never re-enters expansion. `match-reference` keeps fallbacks on its axis — the inset destination from the *preferred* placement: **ruled so (§16, 3)**; a threshold has hysteresis only where script evaluates it; as a query it is exact at `>=`: **ruled (§16, 2; *For the owner*, M)** | G2: §6.2, §6.3 and §13 use three placements, and "bounded" ends the loop at an arbitrary step | — |
| 13 | ~~The shared corner goes by a fixed edge priority, not opening order~~ — **replaced by the owner (2026-10-11; §16, 4): "first accepted request wins for now", reservation order kept apart from paint order.** R2 stands as a known cost | R2: opening order is not the same in the next document, so the shell is not identical across documents; and it is the one thing in §7 CSS cannot say | users prefer the order they opened things in, measured |
| 14 | ~~Logical names in both families~~ — **replaced by the owner (2026-10-11; §16, 14): placement names are physical only; logical names, direction and writing modes are deferred — "will solve later" (§16, 14, as revised in *For the owner*, Q).** The offset rule — positive is inward on an aligned axis, physical (right, down) on a centred one — **ruled so (§16, 1)** | G5: inset names are physical, outset alignments follow direction, and §6.3 maps one onto the other. §16's *writing modes* cannot stay open next to an Agreed §5.3 | — |
| 15 | **Content is laid out once, at the destination size; the viewport is revealed** (a clip, or a size with `overflow: clip`), and nothing transforms the root. Animation is chosen per effective mode; `fade` and `none` exist | N1: at the animating size text rewraps every frame; a transform makes the root the containing block of every fixed descendant (`side-menu.css` slides by `left` for this reason). N3 | — |
| 16 | **Lifecycle has `Closed` (mounted, hidden) and an initial `Open` with no entrance; `close()` is not `unmount()`**; `Closing → Opening` exists — the last **ruled so (§16, 11)**: reversal from the rendered state, the latest request wins, a superseded operation settles as cancelled | L1: the diagram ends every close in *Unmounted* while the handle has `open()` after `close()`; a docked sidebar would play its entrance on every document | — |
| 17 | **Opening is declarative**: a custom command, `command="--iota-open" commandfor="…"`, checked by the builder's `command-target` | F2: a page's author writes no script (phase 2, decision 19); the draft has an imperative API only | a custom command does not reach a target the realization needs (*Not verified*, 9) |
| 18 | **A viewport's configuration is checked by types and by the builder, not in the browser** — §12's validations ship no bytes | the values are compile-time; a check in the browser is weight, and reports to nobody | dynamic segments pass run-time values (then: in development builds only) |
| 19 | **A byte budget per capability is an acceptance criterion** (§14 has none), each capability behind a flag of `mount()`; with it: no layout shift for docked chrome, navigation without script, every page pruned, a screen-reader pass | "Small by design"; phase 2's T1–T8 were stated before they were measured | — |
| 20 | **v1's parent is the window**, less the reserved strips: the **available rect**. No element parents. A reference is an element, a point or a range | G4, G6, R3: an element parent in a window-fixed layer is unclipped; fill and a FAB's inset need the rectangle that is left | a split-pane application is a target |

## Findings

Severity: **B** blocks — the model cannot express what it must, or breaks
what is built; **M** major; **m** minor. The last column is the decision
above, or what to decide.

### Against what is built

| | § | Problem | Sev | → |
| --- | --- | --- | --- | --- |
| C1 | 11.1 | A `<template>` on a page, or a script naming `cloneNode` / `append` / `remove`, turns CSS pruning off for that page (`cssprune/prune.go:455`, `match.go:524`). Every page has navigation: every page | B | 3 |
| C2 | 11.1, 12 | "No element is added, moved or removed" by a behaviour (`phase02/builder.md`, *Behaviours*); `packages/ui/test/convention.test.ts` fails one that writes `style`, `inert`, `hidden`, an attribute or an element. The runtime writes all of them | M | restate the convention: a behaviour names what it writes, in full (rule M already reads that) |
| C3 | 11.1 | `mount()` finds its element by id when the page loads and returns nothing; a clone has no id at load and needs a handle (phase 2, decision 9, *Reverse if*) | M | 3; for holes: `layout.md`'s `mountX(root)` per clone |
| C4 | 2.2 | `pagecheck` requires a `<dialog>` behind `show-modal` and a `[popover]` behind `popovertarget` (`command-target`); ids in a template are not of the page (`idref-not-found`) | M | 17 |
| C5 | 5.3, 12 | T1 forbids "any generic runtime"; T2 has 57 B left on `/syntax/` (`phase02/bet.md`), and its refuting line is 5 KB brotli. Every page mounts its layout's overlays, so a fixed cost lands on every page | B | 2, 19; *For the owner*, C |
| C6 | — | Every browser check asks native state (`dialog.open && :modal`, `:popover-open`); the site's and the bench's too | M | part of decision 4's cost |
| C7 | 2.1 | "Layer" and "base" are cascade layers here: `@layer rg.base, rg.components` (`phase02/components.md`, *CSS convention*). "Viewport" is the browser's three | m | *For the owner*, K |
| C8 | — | Iota's `core` is tokens; `@reactogenic/core` is `mount`, `variants`, `pathname`. "`ui` — React components" beside a design system authored in `.rtsx` and executed at build time | m | *For the owner*, J |

### Geometry

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| G1 | 3.2, 5.2, 12 | `intendedSize` is main/cross; content has a width and a height | a 300 × 200 menu, auto-placed from `bottom-start` to `right-start`: rotated, or unchanged — the draft says "remap" | M | 11 |
| G2 | 6.2, 6.3, 13 | Thresholds are on the *effective* axes (§6.2), evaluated before collision resolves them (§13, 3–4); the fallback is keyed on the *requested* placement (§6.3) | parent 400 × 800, menu 380 × 300, `bottom-start`, `{main: 0.9}`: no fill; collision picks `right-start`, where 380 / 400 fills; the inset fallback is `bottom-left`, where it does not | M | 12 |
| G3 | 3.2, 6.1, 12 | Lengths are px numbers: no `rem`, no token, no content size, no `fill`, no minimum or maximum, no rule for content larger than its viewport (which box scrolls) | a full-height sidebar is `threshold.cross = 0`; a long menu has no maximum height | M | 11 |
| G4 | 3.1, 12 | `parent: Window \| HTMLElement`, rendered in a layer fixed to the window: an element parent neither clips nor scrolls its viewport | a viewport "filling" a pane hangs over the page when the pane scrolls | m | 20 |
| G5 | 4.1, 5.1, 6.3 | Inset names are physical; outset `-start` / `-end` follow direction (Floating UI's, read from the floating element — which is in the layer, not beside its trigger); §6.3 maps `-start` to `-left` | a right-to-left `top-start` menu that fills jumps to the other side | M | **deferred by the owner (*For the owner*, Q)**: physical names only for now; a known limit, as phase 2's menus |
| G6 | 3.1, 12 | `reference: HTMLElement` — and the type has no window, though §3.1 needs one, no point, no range | a context menu; a window-relative inset anchor does not type | M | 20 |
| G7 | 6.2 | The table keeps main "independently positioned" under a cross-only fill; the prose says an outset that expands "abandons outset anchoring" | a listbox under a wide field on a phone: stays under the field, or jumps to the window's bottom | M | decide: one-axis fill of an outset stays attached on main |
| G8 | 5.2 | `match-reference` is recalculated when placement changes axis | a select's listbox, `auto`, lands beside its field: 32 px tall | M | 12 |
| G9 | 4.1 | A corner's main axis is always vertical; `center`'s is horizontal by fiat; `top`, `left`, `right`, `bottom` are in both placement unions with two meanings | a toast at `top-right` cannot Slide in from the side | m | state the main axis; brand the unions |
| G10 | 6.4 | Hysteresis is open "for thresholds" | not needed at all if intended size is never measured from effective size (§6.1 already says so): a threshold is then a pure function of the parent | m | **ruled (§16, 2; *For the owner*, M)**: none in a query; enter and leave thresholds where script evaluates, the tolerance a system constant |
| G11 | 13 | No rule for a reference that is clipped, scrolled away or under a sticky header | a popup follows its trigger under the site's header | M | **ruled (§16, 13)**: a lost reference closes the viewport, with its exit; a reference out of sight hides it by `position-visibility`, both spellings (*For the owner*, O) |

### Scroll and the viewport

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| S1 | 2.3, 7.2 | Who scrolls is not said. The CSS implies the document | a top "reservation" as a base margin exists at scroll 0 only; an inner scroller instead loses toolbar collapse, printing of the whole document, scroll restoration | B | 7 |
| S2 | 8 | `inert` does not stop the root scrolling: no scroll lock in the model. `overflow: hidden` removes a classic scrollbar and widens the parent | the page scrolls behind a modal | M | lock as `dialog.css` does; `scrollbar-gutter: stable`; `overscroll-behavior: contain` in floating scrollers |
| S3 | 2.2, 13 | A root placed by script trails a scrolling anchor by a frame or more | every dropdown in scrolled content, worst on iOS | M | 2 |
| S4 | 7 | A reserved strip is invisible to the platform | `#fragment`, focus and find-in-page land under a top strip; `position: sticky; top: 0` slides under it | M | 7 |
| S5 | 7 | The base narrows; its content's media queries do not | a two-column page beside a granted sidebar stays two-column | M | the base is a query container; Iota's own breakpoints are container queries |
| S6 | 6.4, 7.2 | The layer's height changes as mobile toolbars retract; `3dvmin` margins with it | a 500 px sheet at threshold 0.8 flips while the reader scrolls (0.81 ↔ 0.71) | M | thresholds and margins against a stable viewport (`svh` / `lvh`), never `dv*` |
| S7 | 6.4 | The layer is fixed and does not scroll | a field in a bottom sheet stays behind the on-screen keyboard | M | geometry on the layout viewport; the keyboard's inset is an input to the available rect |
| S8 | — | Safe areas: `inset: 0` under a notch; a px offset cannot say `env()`; a filled axis drops its offset | a bottom sheet under the home indicator | M | the parent's padding is an input, read from CSS |
| S9 | — | Printing: a fixed panel repeats on every page | — | m | 7: `@media print` zeroes the insets and hides the layer |
| S10 | 3.1 | "The visible window rectangle", if read as the visual viewport | pinch zoom re-lays the UI | m | the layout viewport |

### Reservation

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| R1 | 7.4, 8.3, 12 | A grant may become an overlay on resize "without remounting"; `modal` may not change while mounted. "Independent properties" allows states no component wants and forbids the one every site needs | a docked sidebar, narrowed: an overlay on a live page, no scrim, no way to dismiss — or modal while docked | B | 8 |
| R2 | 7.3 | The corner depends on opening order | two documents of one site show two shells; not expressible in CSS | M | **kept by the owner for now (§16, 4)**: a known cost |
| R3 | 6.2, 7.3 | A filled cross "spans the parent"; the corner table gives `200 × 450`. A request carries a main extent; the strip spans the cross | an edge-centred 300 px palette that reserves: its cross size is not defined, granted or declined | M | 20 |
| R4 | 6.2, 7.4 | A main-filled panel requests the whole parent | a base with no minimum grants it and has no width | m | a filled main axis does not reserve |
| R5 | 7.4, 13 | "Bounded" is per transaction; nothing bounds across them | grant → the base wraps → a scrollbar appears → the parent is 15 px narrower → decline → … | M | `scrollbar-gutter: stable`; the parent excludes the scrollbar; hysteresis on the reservation |
| R6 | 7.3 | Two panels on one side: allowed? in which order? | a rail and a secondary panel on the left | m | decide; with 13 |
| R7 | 7.5 | The base's margins change with the grant | a layout animation of the whole page per frame; after load, a layout shift | M | 6. The animation is the ruling's (§16, 6, "mirroring entrance"): its cost is to be measured |
| R8 | 7.4 | Grant or decline, and nothing else | on decline the panel overlays; a site wants it *closed* | M | 8: a declined panel's default is its component's (`"when-overlay"` closed) |

### Stack and modality

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| M1 | 8.2, 8.3 | Counted claims assume LIFO; the Agreed parts guarantee long-lived entries at the bottom (a reserved panel, §10's button) | a sidebar toggled while a dialog is open is appended above the dialog, interactive | M | **ruled (§16, 7)**: only modals are in the stack. The case is then a non-modal entry opened by script above a modal: *For the owner*, N |
| M2 | 2.2 | Paint order is mount order: no "always on top" | a toast shown before a dialog is under its scrim and inert — silent to a screen reader; after, above it by timing | M | 9 |
| M3 | 2.1, 8.3 | "Logical ownership" has no parameter anywhere; the LIFO cascade stands in for it | Esc in a dialog closes the sidebar mounted later; a press in a submenu is "outside" its menu | M | not the model's (§16, 12): the interaction spec (*For the owner*, P) |
| M4 | 8 | No scrim in the model: an inert base swallows presses and is not dimmed | every modal | M | one scrim per modal entry, the runtime's, its preceding sibling, not an entry |
| M5 | 11.2 | Claims are released on unmount — after the leaving animation | 200 ms of inert page under a dialog that is fading and still clickable | M | 10 |
| M6 | 2.2, 14 | "No implicit native top-layer presentation" cannot be held for the page's content | an author's `showModal()` makes the layer inert; a `<select>`'s picker is above it | M | reword: "Iota's components do not use…"; a page check for `popover` and `<dialog>` if choice 1 excludes them |

### Accessibility and input

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| A1 | 7.2 | A granted panel stays in the layer: after all of the page in the DOM | navigation is last in Tab order and in reading order (WCAG 1.3.2, 2.4.3); a skip link points backwards | B | 6 |
| A2 | 8.4 | Focus is specified for modals only | a non-modal popup from a trigger mid-page: Tab goes on through the page, the popup comes after all of it. `popover` puts it after its invoker for nothing | M | not the model's (§16, 12): P |
| A3 | 2.2 | `aria-expanded` on an invoker is the browser's for a popover; without one it is written by script | — | M | named state (rule M) |
| A4 | 8.4 | "Only the active modal advertises modal semantics" — and a popup above it is its sibling | with `aria-modal="true"` a listbox opened from the dialog is outside the modal for VoiceOver | M | never `aria-modal`; `inert` is the mechanism |
| A5 | 8.4 | Close requests: Esc is `keydown` in the prototype; "top entry" is the last sibling | Android's back does nothing; Esc closes a toast; Esc during composition closes a dialog | M | not the model's (§16, 12): P. Proposed there: CloseWatcher; `dismissible` per entry; the topmost dismissible entry of the focused owner chain |
| A6 | 8.4 | Outside press needs ownership; on touch, dismissing on `pointerup` lets the click land behind (measured: `phase02/components.md`, *SideMenu*) | — | M | not the model's (§16, 12): P. Proposed there: dismiss on `click` |
| A7 | 6.2, 12 | A non-modal viewport that fills both axes covers a focusable base | Tab into what cannot be seen (WCAG 2.4.11) | M | 8 |
| A8 | 11.1 | `aria-describedby` / `-controls` / `-labelledby` into a template dangle until it is cloned; a live region cloned with its text is often not announced | a tooltip's text is absent when its button is announced | M | 3; a live region is pre-rendered, outside the claims |
| A9 | 8.4 | Return focus to the `trigger`, not to what had focus: WebKit does not focus a clicked button (`phase02/components.md`, *Known limits*). `autofocus` in an element inserted later does nothing. With the rest inert no Tab trap is needed — say so, and save its bytes | — | m | — |
| A10 | — | Hover intent — delays, the gap between a trigger in the base and a sibling in the layer, hoverable content (WCAG 1.4.13), long-press — is not modelled | tooltip, submenu, hover card | M | *For the owner*, H |
| A11 | 2.1 | `<main class="base-layer">` around a page takes the landmark role from its `<header>` and `<footer>` | — | m | a `<div>` |

### Animation and lifecycle

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| N1 | 9.1, 9.2 | "Geometry changes, content is not distorted, the mechanism is open" leaves the content's layout during B undefined | at the animating size: a rewrap per frame, and a 56 px start; under a transform: fixed descendants re-based | M | 15 |
| N2 | 9.4 | Slide reflects across the *offset origin* — defined for inset anchors only (§4.2) — and relies on something clipping the start. The prototype's lab canvas clipped | an outset menu starts on top of its trigger; an inset viewport on a card starts beside it, in view | M | Slide is for parent-edge inset and filled viewports, or a clip is specified |
| N3 | 9.3, 12 | Animation is one fixed option; the pivot is undefined for outset and after a flip; no `fade`, no `none`; `initialScalePercent` is required beside a trigger | a menu that became a bottom sheet scales up from its trigger; a centred dialog cannot fade | M | 15 |
| N4 | 9.2, 9.5, 7.5 | The trigger is sampled before the reservation moves it; on exit the reservation is still held | a sidebar grows out of a button that is sliding away | M | **ruled (§16, 6)**, as read here: the base's change is on B's timeline, both ways |
| L1 | 11.2, 12 | The diagram has no closed-but-mounted, no `Mounted → Unmounted`, no `Closing → Opening`, no open-at-load without an entrance | a form in a dialog loses its input on close; a docked sidebar animates in on every page | M | 16 |
| L2 | 10 | The self-trigger viewport is two size states of one viewport; the API has no expand or collapse, and it cannot become modal expanded | a FAB that opens into a modal sheet | M | 8, 16: named geometry states |
| L3 | 11.2 | "The page lifecycle must dispose": a navigation is a new document | disposing on `pagehide` leaves a dead page on Back (bfcache); `unload` disables bfcache | M | no disposal on page events; transient entries close on `pagehide` (`overlays` does); disposal is React's unmount only |
| L4 | 3.2 | Geometry driven by a gesture — a sheet's detents, a swipe, a resized panel — is an `update()`, a transaction and a base reflow per pointer move | — | M | out of v1, said; or a fast path that skips reservation until release |

### The framework

| | § | Problem | Breaks when | Sev | → |
| --- | --- | --- | --- | --- | --- |
| F1 | 11.1, 7 | Nothing floating exists before the module runs | first paint without the sidebar, then a shift; openers dead until then; a view transition between two documents captures "no sidebar" | B | 3, 6 |
| F2 | 12 | No way to open without script of the page's own | every trigger in shell code | M | 17 |
| F3 | — | State across documents — a collapsed sidebar, open groups, a scroll offset — is lost, or flashes, unless applied before first paint: a blocking script (the builder's is a module at the end of `<body>`) or a variant per state | — | M | *For the owner*, I |
| F4 | 8.2, 12 | A page's script and each dynamic segment's bundle would each carry the stack | two counters on one base corrupt `inert`; "preserve the application's `inert`" cannot tell a sibling copy from the application | M | one instance per document, found through the DOM, or the state kept in the DOM |
| F5 | 11, 12 | React is not covered: no `adopt(element)` for what a portal renders; unmount empties the holes before the leaving animation; StrictMode's open, close, open; a reference's identity across remounts | a re-render closes a popover (the prototype's reference-loss policy) | M | with dynamic segments; `layout.md`'s bridge is the start |
| F6 | 11.1 | "Prevent duplicate ids" by rewriting breaks every reference into and out of a clone (`aria-*`, `for`, `commandfor`, `href="#…"`, SVG `url(#…)`), the id selectors the pruner matched, and "an author's id wins" | — | M | 3; a template is single-instance unless the builder generated its ids |
| F7 | 2.1, 11.1 | Content rendered away from where it was written loses `lang`, `dir`, a theme's scope, an ancestor's container, its `<form>`, and bubbling to its logical ancestors | a combobox's listbox in the author's form; a dialog in a themed section | M | the builder copies `lang`, `dir`, the scope; `form=`; said for each realization |
| F8 | 12 | No events: a grant that flips (§7.4) notifies nobody; no close reason, which `layout.md`'s bridge needs; `requestSpace()` returns no rectangle | — | M | add: the allocation, a subscription, `onClose(reason)` |
| F9 | 12 | Validation at run time | — | m | 18 |
| F10 | — | Durations, easings, thresholds, base minimums, margins, the scale percentage are tokens: `core`'s. There is **no z-index scale** — worth saying on the tin | — | m | with `core` |
| F11 | 11.1 | A `Dialog` in `SideMenu`'s `$Header` is a template in a template | invisible to `getElementById` until the outer is cloned | m | 3 |

### Verification

| | § | Problem | → |
| --- | --- | --- | --- |
| V1 | 14 | Acceptance is geometry: no bytes, no layout shift, no script-off case, no pruning, no assistive technology | 19 |
| V2 | 1 | "Geometry and DOM-emulation checks pass" is the prototype's; no engine has rendered the model | decision 4's comparison |
| V3 | — | Firefox has never been run in this repo; no touch device; not the floor's own versions (`phase02/plan.md`, RGP2-050, *Not run*) | run them in decision 4's comparison: a model of focus, `inert` and animation needs all three engines |

## Components against the model

Can the draft, as written, express it?

| Component | | Missing |
| --- | --- | --- |
| a docked sidebar that is a drawer on a small screen | no | R1, R8, A1, F1, F3, L1 |
| a modal dialog with a scrim | partly | M4, S2, M5; a height from content with a maximum (G3); `fade` (N3) |
| a dropdown with a submenu | partly | M3, A10, A2 |
| a select, a combobox | partly | G7, G8, S7, F7 |
| a tooltip | no | A8, A10; out of Esc and LIFO (A5) |
| a stack of toasts | no | M2, A8; its timers; reflow of the stack |
| a context menu at the pointer | no | G6 |
| a bottom sheet | partly | M4, S7, S8, L4 |
| a command palette | partly | an offset in viewport units, a height that follows each keystroke (G3) |
| a popover in a dialog in a drawer | **yes** — paint and claims: the model's strength | A2, A4, F7 |
| a button that expands (§10) | partly | L2, M1, M4, R3 |

## For the owner

The recommended option is first.

| | Question | Options | Ruling | Where |
| --- | --- | --- | --- | --- |
| A | **Do Iota's components leave the top layer?** (§2.2, Agreed) | **1. Decide by a measured comparison** — `Dialog`, `DropdownMenu`, `SideMenu` built on the layer and on the top layer, both with CSS geometry and live elements; thresholds stated first: the layer's fixed cost per page with an overlay (proposed: ≤ 1 KB raw, 0 on a page with none, each capability behind a flag); phase 2's browser checks pass in Chromium, WebKit and Firefox; the accessibility tree of each open overlay equal to the native build's; no layout shift; every page pruned. The layer wins if it passes all of them. 2. The exclusion stands as agreed; the review's other decisions apply inside it. 3. The top layer stays; the layer is used only for what it cannot do (a toast above a modal) | | choice 1; decision 4 |
| B | **May script be required to open an overlay?** With the layer it is, always: phase 2's decision 11 gave "no JS to open / close / focus / place at the floor" as its reason | **1. Yes, if navigation stays reachable without it** (docked chrome in the base: decision 6; a drawer falls back to in-flow). 2. Yes, unconditionally. 3. No — which is option A.3 | | decision 6 |
| C | **Phase 2's thresholds under a design system.** T1 forbids "any generic runtime"; T2 is "an absolute budget for this site's three behaviours" (phase 2, F) with 57 B left | **1. T1 stands; T2 is restated per capability** (a budget for each of stack, modality, dismissal, placement by script…), stated before it is measured. 2. One budget per page. 3. Both stand as written, and Iota must fit | | decision 19 |
| D | **The shared corner** (§7.3) | **1. A fixed edge priority.** 2. Opening order, as drafted — then a reservation needs script, and two documents may differ | **"Keep first accepted request wins for now. Store reservation order separately from paint order."** Option 2, for now (§16, 4). Not said: the order of panels that are open when the page loads — proposed: document order | decision 13 |
| E | **Who scrolls** | **1. The document.** 2. `BaseViewport` | | decision 7 |
| F | **Placement names** | **1. Logical in both families.** 2. Physical, mirrored by a rule. 3. As drafted | **"Separate physical placement names from logical placement names. Keep left/right physical."** Neither option: two sets, one mapping (§16, 14). What is left: Q | decision 14 |
| G | **Docked chrome** (§7.2, Agreed: it stays in the layer) | **1. In the base while docked; in the layer as an overlay** — one element, as phase 2's `SideMenu`. 2. In the layer, with a specified repair of focus and reading order | | decision 6 |
| H | **Hover intent** (tooltip, submenu, hover card) | **1. Out of IVM v1, said in the spec**; the catalog's CSS tooltip stays what it is. 2. In, with a budget | | A10 |
| I | **State across documents** (a collapsed sidebar) | **1. None in v1: it resets**, as phase 2 (`phase02/components.md`, *SideMenu*: not in phase 2). 2. A variant per state, selected by the router. 3. A blocking script of the builder's, ahead of first paint — today it writes one module at the end of `<body>` | | F3 |
| J | **The packages.** `core` is tokens, and `@reactogenic/core` exists; "`ui` — React components", authored in `.rtsx` | **1. Their own scope, the owner's to name** (`@iota/…`: availability not checked); `@reactogenic/ui` moves there. 2. Under `@reactogenic/`, with other names | | C8 |
| K | **The words.** "Layer" and "base" are cascade layers in `packages/ui`; "viewport" is the browser's | **1. Keep the model's words; rename the cascade layers** (`@layer iota.reset, iota.components`). 2. Rename the model's | | C7 |
| L | **An unrelated entry above a modal** (§8.4, open) — a toast while a dialog is open | **1. Wanted, as a band of its own** — it is the one thing only the layer gives. 2. Not wanted: then A's case for the layer is two items, not three | | decision 9; N(b) |
| M | **Hysteresis (§16, 2) and a threshold written as a query.** A query has no memory: it cannot leave "slightly lower" than it enters. And in the band the result depends on history: a page loaded there differs from the same page resized into it, and from the next document | **1. Hysteresis where script evaluates the threshold** (a measured input, a size that jitters); **a compile-time threshold is a query, exact at `>=`** — its input is the window, which moves when the reader moves it, and stable units (S6) take the toolbar's jitter away. 2. Hysteresis always: every threshold needs script. Either way a page that loads has no history: the entering threshold applies | **"Agree. Queries use exact >=, with no hysteresis. Script evaluation may use separate enter/leave thresholds. That difference must be explicit in the contract."** Option 1. The contract says which of the two a threshold is — it follows from its inputs: compile-time, a query; measured, script | decision 12; choice 2 |
| N | **The stack of modals (§16, 7 and 8), read through.** (a) A modal that is not at the top and that *script* closes — a React unmount, a lost reference (§16, 13), `pagehide`: inert stops the reader, not the page. (b) A non-modal viewport is not in the stack: a modal pushed after it blocks it as it blocks the base; it may be removed at any time, its claims with it; one opened after the modal is above it and interactive — which is L. (c) `modal: "when-overlay"` (decision 8) is a push when the grant is lost and a pop when it returns, the pop waiting for the top | **1. (a) deferred until it is at the top; (b) and (c) as read.** 2. (a) refused, with an error in development. 3. (a) cascades: what is above closes first — the prototype's policy | **"Reference loss takes precedence over stack order. So unmounted viewport should forcefully remove modal viewport from stack"** — a modal whose reference is lost, or that is unmounted, leaves the stack wherever it stands (each modal owns the list of what it blocked). Otherwise **"keep immutable"**: a modal that is not at the top neither changes nor closes (§16, 8); nothing is deferred. (b) and (c): not ruled | decisions 8, 9 |
| O | **`position-visibility` (§16, 13).** It acts on an anchor-positioned element only. The spelling at the floor is `anchors-visible` (Chrome 125, Firefox 147, Safari 26.2; deprecated); `anchor-visible` is Safari 27 alone (`@mdn/browser-compat-data`, `main`, 2026-10-11). What it hides is "strongly hidden": at once, no exit, and the viewport stays open | **1. An outset viewport is placed by CSS anchor positioning** (decisions 2 and 5), both spellings written — the ruling needs it. 2. Placement by script, and Floating UI's `hide` in its place: then this ruling is not CSS | **"Agree on native hiding and dual spelling"** — `position-visibility`, written `anchors-visible` then `anchor-visible`. It entails that a viewport hidden this way is anchor-positioned in CSS: decision 2 for an outset viewport with an element for reference, and decision 5's `position: fixed` roots if the anchor rule is as read (*Not verified*, 1). The rest of decision 2 — thresholds, Floating UI as no dependency — is not ruled by it | decisions 2, 5 |
| P | **Where focus and dismissal are specified (§16, 12)**: focus in and back, a popup's place in Tab order, Esc and back, outside press, ownership, hover intent — A2, A3, A5, A6, A9, A10, M3. Modality stays the model's (§16, 7), and pushing a modal makes the focused element inert: the browser drops focus to `<body>`, and by this ruling the viewport does not move it | **1. An interaction spec of `bits`, beside ivm.md, before any component**; the model gives it two signals — the top of the stack, and "closing started" (decision 10). 2. Per component, in `ui` | **"Exactly!"** Option 1: an interaction spec of `bits`, beside ivm.md, before any component | A2–A10 |
| Q | **`-start` / `-end` under §16, 14.** An outset alignment rides on a physical side (`top-start`), and §6.3 maps it to a physical corner (`top-left`) | **1. The side is physical, the alignment logical, mapped in the one place** — CSS's and Floating UI's reading; §6.3's table goes through the mapping. 2. Two whole outset families, `top-left` beside `top-start` — and `top-left` is already an inset name | **"keep physical only (will solve later)"** — the last word of three that day: every placement name is physical. Logical names, direction and writing modes are deferred | G5 |

## The draft's §16, ruled

By the owner, 2026-10-11, after this review. *Recommended* is what this
file said before the ruling.

| | Topic | Recommended | Ruling | Here |
| --- | --- | --- | --- | --- |
| 1 | Centered-axis offsets | physical on a centred axis | **"Keep positive right/down"** | as recommended; decision 14's offset rule |
| 2 | Expansion boundary | `>=`; no hysteresis (G10) | **"Expand at >=. Add hysteresis: enter at the configured threshold, leave below a slightly lower threshold. Make that tolerance a system constant initially."** | G10's recommendation is replaced. Ruled further in *For the owner*, M: only where script evaluates |
| 3 | Expanded outset mapping | said per use (`expandTo`) | **"Expansion should derive its inset destination from the preferred outset placement, rather than the collision-resolved placement."** | derived, and from the preferred placement: decision 12's order. Left: §6.3's left and right rows drop the alignment; G7 |
| 4 | Reservation order | a fixed edge priority | **"Keep first accepted request wins for now. Store reservation order separately from paint order."** | decision 13 is replaced; R2 is a known cost. *For the owner*, D |
| 5 | Reservation gaps | extent + a gap token | **"Own size + main axis offset (including negative offset). So panel 300px with -4px offset reserves 296px"** | reserved span = main extent + main offset. The cross offset reserves nothing. A positive offset is then a gap at the parent's edge, not between the panel and the base — that one is the base's margin |
| 6 | Grant lifetime | released when closing starts; the base not animated in v1 | **"Reservation release starts when exit starts, mirroring entrance"** | decision 10, for the reservation. Read here as: the base's inset moves on B's timeline, entering and leaving — N4 is answered, R7's cost is taken and to be measured |
| 7 | Stack removal | by ownership (decision 9) | **"The stack is ONLY for modal viewports. Inert descendants cannot be closed by definition (inert) and so stack operates naturally (push/pop only)"** | decision 9's first half is replaced: counted claims, push and pop. What script may still do to what is inert — ruled in *For the owner*, N: a lost reference comes before stack order |
| 8 | Modal mutation / reorder | supported, by derivation | **"See above. Modal viewports are immutable until at the top of the stack"** | a modal changes, and closes, at the top only. Decision 8 is read on it: N(c) |
| 9 | Collision correction | fixed by the system, in CSS | **"Explicit placement uses preferred placement with fallback; automatic placement chooses available space. Apply bounded shifting afterward. Keep middleware details internal."** | the policy is the system's and nothing of it is exposed: as recommended. Who computes it is decision 2, not ruled. "Bounded" by what is not said — proposed: the viewport keeps overlapping its reference on the cross axis |
| 10 | Scale fallback | a token; zero allowed | **"Let's check what UIKit does, but 30% sounds like a good starting point"** | 30% to start with, a token. Checked: below |
| 11 | Exit / reversal | decisions 10, 16 | **"Reverse immediately from the currently rendered geometry and visibility. Latest requested state wins. Superseded operations settle as cancelled rather than hanging or reporting completion."** | decision 16's `Closing → Opening`. Proposed for the API: `open()` and `close()` resolve to `"completed"` or `"cancelled"`; they do not reject |
| 12 | Focus / dismissal | A2, A5, A6, A9 | **"Outside of viewport model/concern. Viewport only is responsible for dimensions and position."** | §8.4 leaves the model, and A2, A3, A5, A6, A9, A10, M3 with it. They are the interaction spec's: *For the owner*, P |
| 13 | Reference loss | a transient viewport closes; G11 | **"Reference loss should trigger exit animation. Visual reference loss (aka trigger scrolled out of visibility) should use CSS: `position-visibility: anchor-visible` rule"** | G11 is answered. The property needs CSS anchor positioning, and another spelling at the floor — agreed: *For the owner*, O |
| 14 | Window / writing modes | logical names in both families | **"Separate physical placement names from logical placement names. Keep left/right physical. Define window geometry and coordinate conversion centrally; treat RTL and writing-mode behavior as explicit mappings via inline-start/end and block-start/end"** | decision 14 is replaced. Revised in *For the owner*, Q: physical names only, the logical ones later. Not ruled: the visual viewport, the keyboard, safe areas, stable units (S6–S8, S10), transforms on the layer's ancestors (decision 5) |

**Checked for 10.** UIkit — the web framework, getuikit.com
(`src/less/components/animation.less`, branch `develop`, 2026-10-11):
`uk-animation-scale-up` goes from `transform: scale(0.9)` to `1` with a fade,
0.5 s, ease-out; its slide moves by 100% with a fade (the small one 10px, the
medium 50px). So **90%, not 30%** — and a transform of the whole element,
content included, which §9.2 does not want and decision 15 argues against:
at 90% a transform is a pop, at 30% the viewport visibly grows, and they are
two different effects. Apple's UIKit publishes no figure for its
presentations that could be cited: **not checked** — if that is the UIKit
meant, the number has to be measured on a device.

## Not verified

Claims of this review that no one has run. Each is a line of decision 4's
comparison, or of a spike before it.

1. An element of the base is not an acceptable anchor for an absolute child
   of a fixed layer, and is one for a fixed root under an unpositioned
   layer — in the three engines.
2. `anchor()` / `anchor-size()` inside `@starting-style` interpolate to the
   final insets; `position-try-order` at the floor (Firefox 147, Safari
   26.2); a `display` transition outside the top layer in Firefox and
   Safari.
3. `inert` on the base does not stop wheel, touch or keyboard scrolling of
   the root; whether an inert nested scroller scrolls.
4. The size of the Floating UI subset a page would need
   (`computePosition`, `flip`, `shift`, `size`, `autoUpdate`), minified and
   brotli, built with esbuild as a behaviour.
5. Floating UI reads `-start` / `-end` from the floating element's
   direction (from memory of its source; its *platform* page does not say).
6. `CloseWatcher` at the floor in Firefox and Safari.
7. VoiceOver hides a sibling popup under `aria-modal="true"`; a live region
   cloned with its text is not announced; an inert live region is silent.
8. `autofocus` is ignored on an element inserted after load.
9. A custom command's event reaches a target that is neither a `<dialog>`
   nor a popover, and bubbles, at the floor.
10. The reservation oscillation with classic scrollbars (R5); how the
    layer's height changes while a mobile toolbar retracts (S6).
11. History scroll restoration of a nested scroller; a fixed element
    repeated per printed page (S1, S9).
12. The playground: every *Prototype policy* in [ivm.md](ivm.md).
13. `position-visibility: anchors-visible` at the floor, in an engine — the
    versions in *For the owner*, O are compat data, not a run; and whether
    the browser's own overflow alignment of an anchored box (`anchor-center`,
    `position-area`) is the "bounded shifting" of §16, 9.

## What the rulings would open

No `plan.md` yet. In order, if the recommendations stand:

1. **The comparison** of *For the owner*, A — with *Not verified* 1–3, 6, 8
   and 9 as its first hour, and Firefox run in this repo for the first time.
2. **The builder's portal**: an element written in a component, emitted in
   the layer; `pagecheck` and `cssprune` told of it; `command-target` for
   `--iota-*`.
3. **[ivm.md](ivm.md), second draft**: §16's rulings and the decisions as
   ruled, each `> OPEN:` in place. Beside it, the **interaction spec**
   (*For the owner*, P): what §16, 12 took out of the model.
4. `core`: the tokens the model names (F10). Then `bits`, then `ui`.
