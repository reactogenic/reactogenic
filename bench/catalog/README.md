# The catalog: a measurement fixture

`@reactogenic/bench-catalog`. **Not the design system** — that is
`packages/ui` (`@reactogenic/ui`), with its spec
(`specs/phase02/components.md`), its contract tests and its browser suite.
This is twenty components written for one measurement: what component
awareness is worth where a page uses a few components of many
(`specs/phase02/bet.md`, *The catalog*; plan.md, RGP2-050). They have no
spec, no contract tests and no API anyone should build on. What checks them
is `bench/catalog-verify.mjs`: every page of `bench/catalog-site` in two
engines, and the pruned sheets against the whole one.

A private workspace package, so that the site resolves it as a site resolves
a design system: `import { Tabs } from "@reactogenic/bench-catalog"` through
the program, `mount("@reactogenic/bench-catalog/behaviors/tabs", …)` through
esbuild from the project directory (builder.md, *Behaviours*).

## The twenty

Four are `@reactogenic/ui`'s own, re-exported: `Button`, `Dialog`,
`DropdownMenu`, `SideMenu`. Sixteen are here — what a docs, marketing or app
site has beside them:

| Component | Is | JS | Options a rule selects | Slots |
| --- | --- | --- | --- | --- |
| `Accordion` | `<details>` in a column; `exclusive` is the platform's `<details name>` | none | `variant` (flush) | `$Item` (keyed) |
| `Avatar` | a picture, or the initials of the name | none | `size`, `shape` | — |
| `Badge` | a label on something | none | `tone` (4), `variant` (2), `dot` | — |
| `Breadcrumbs` | `<nav><ol>`; the last crumb is the page | none | — | `$Crumb` (keyed) |
| `Callout` | a note beside the text | none | `tone` (3) | `$Title` |
| `Card` | a surface; with `href` the card is one link | none | `variant` (2), link | `$Media`, `$Eyebrow`, `$Title`, `$Footer` |
| `Checkbox` | the platform's, restyled; `variant="switch"` is `role="switch"` | none | `variant` | — |
| `CodeBlock` | `<pre><code>`, a title, and — with `copy` — a button | `copy`, only with `copy` | `wrap`, `lines` | — |
| `Field` | a labelled input or textarea, hint, error | `field`, only with `counter` or `reveal` | invalid | `$Prefix` |
| `Pagination` | page numbers as links | none | `compact` | — |
| `Progress` | the platform's `<progress>`; indeterminate without a value | none | `tone` (3), `size` | — |
| `Select` | the platform's `<select>`, options only | none | `size` | `$Option` (keyed) |
| `Table` | static rows in a box that scrolls sideways | none | `density`, `striped`, `sticky`, column `align` | `$Column` (keyed) |
| `Tabs` | a tab list and its panels | `tabs` | `variant` (pills) | `$Tab` (keyed) |
| `Toast` | a manual popover its trigger shows | `toast`, unless `timeout={0}`; `overlays` | `tone` (2) | `$Trigger` |
| `Tooltip` | a description shown on hover and focus | none: CSS | `side` | — |

Three have a feature behind a flag (builder.md, *Behaviours*: a flag says
the code is in the page's script, the mount's data that this root uses it):

| Behaviour | Flag | Feature |
| --- | --- | --- |
| `tabs` | `RG_TABS_HASH` | the tab the URL's hash names is shown, and follows it (`hash`) |
| `field` | `RG_FIELD_COUNT` | "12 / 160" under the input (`counter`) |
| `field` | `RG_FIELD_REVEAL` | a button that shows the password (`reveal`) |
| `menu-keys` (of `@reactogenic/ui`) | `RG_MENU_TYPEAHEAD` | typeahead |

## What was followed

`specs/phase02/components.md`, *CSS convention*, and builder.md's authoring
style for behaviours — they are what the pruner and the linker rely on, so a
fixture that broke them would measure something else.

| | |
| --- | --- |
| one `.css` per component, imported by the component | after the tokens: `@reactogenic/ui/tokens.css` (which orders the layers), then `./tokens.css` (the fixture's tones) |
| everything nested under the root class | `.bc-<name>`, in `@layer rg.components` |
| parts through child combinators | `.bc-field > [data-part="control"] > [data-part="reveal"]`; a slot's content is never reached |
| options a rule selects are classes | a **variant**: one class per value, unique to it, resolved by `variants()` of `@reactogenic/core` on the element the rule selects — `variants("bc-badge", badgeVariants, { tone, variant, dot: dot ? "on" : undefined })` → `bc-badge bc-badge-info bc-badge-dot`. The default has none. A switch (`dot`, `wrap`, `striped`) is a dimension with one value, `on`. Parts stay `data-part`: a slot's `className` replaces its attachment's (`Card`'s `$Footer`, `Callout`'s `$Title`) |
| no `!important` | as `@reactogenic/ui`: the rules are in layers, and a layered `!important` beats a project's unlayered CSS. `bench/catalog.mjs` fails on one, in either package |
| runtime state | pseudo-classes (`:checked`, `:popover-open`, `:indeterminate`, `:focus-within`), platform state (`[open]`), `aria-*` — and one named attribute: `data-full`, which `field` writes by that name |
| motion | every `transition` and `animation` inside `@media (prefers-reduced-motion: no-preference)` |
| forced colours | a border where a background or a shadow carried the meaning |
| a behaviour | a default export `(root, data?)`, bare `declare const RG_…` flags, top-level functions, state at module level, nothing that runs at import |
| what a behaviour writes | state, named in full: `aria-selected`, `tabIndex`, `hidden` (`tabs`); `data-full`, `aria-pressed`, the input's `type`, a button's and an output's text (`field`); a button's text (`copy`). No class, no element. `toast` writes nothing |

## What was chosen, and what it costs

- **`Tooltip` is CSS**, shown on `:hover` and `:focus-within`, with
  `aria-describedby`. `popover="hint"` with `interestfor` is Chromium's
  alone, below the floor (decisions.md, 11), and a scripted tooltip is a
  behaviour per use site. The price: Esc does not dismiss it (WCAG 1.4.13).
- **`Tabs` needs its script**: the platform has no tabs. Without it the
  first panel is shown and the others cannot be reached.
- **`Table` is static.** Sorting would be a behaviour that moves rows — a
  script that changes the tree leaves its page unpruned (builder.md, *The
  page's script*) — and the fixture has no use for it.
- **`Toast`'s trigger is a button**: on a static page nothing else can say
  "show it". It stands for what a form's handler would do.
- **No `Hero`, `Footer`, `Stepper`, `Kbd`, `Banner`, `Skeleton`.** The home
  page's hero and the footer are the site's own markup and CSS
  (`bench/catalog-site/site.css`), as on the docs site; the rest had no page
  that would use them, and a component no page uses weighs on the control
  alone.
- **Keyed slots are iterated with `slotKeys`** and read with `slotEntry`
  (syntax.md, *Keyed slots*), as `@reactogenic/ui` does; every key here is a
  word.

## How heavy

Sizes are `bench/results/catalog.md`'s (*The catalog*), written by
`bench/catalog.mjs`. The sixteen were sized against two references, not
against the result:

| | Minified CSS per component |
| --- | --- |
| `@reactogenic/ui`'s four | 0.4–2.5 KB, 1.5 KB on average |
| this fixture's sixteen | 0.5–2.0 KB, 1.2 KB on average |
| Bootstrap 5.3.3, the same sixteen kinds (`bootstrap.min.css`, the rules whose selector starts with the component's class; measured once, 2026-10-05, not kept) | badge 0.5, breadcrumb 0.9, progress 1.4, toast 1.9, alert 2.5, pagination 2.6, tooltip 3.1, tabs and nav 3.5, select 3.4, card 4.4, checks 4.4, accordion 4.5, table 5.2, text input 6.1 KB: 3.2 KB on average |

So a component here is about what one of `@reactogenic/ui` is, and a bit
over a third of Bootstrap's — which carries a custom-property API, every
colour variant and RTL. A catalog of Bootstrap's weight would make the
control's sheet about three times larger and every saving with it. The
behaviours are 360–800 B in a page's script; `@reactogenic/ui`'s are
250–830 B.
