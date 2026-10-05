# The catalog site: a measurement fixture

**Not a product, not the docs site, not the design system.** Ten pages of
the kinds a product site has, written on the twenty components of
`bench/catalog` (`@reactogenic/bench-catalog`: itself a fixture — the real
design system is `packages/ui`), built by `reactogenic build`, and measured
by `bench/catalog.mjs` and `bench/catalog-verify.mjs`. It exists for one
question of `specs/phase02/bet.md`: what component awareness is worth where
a page uses a few components of many (plan.md, RGP2-050, T5; decisions.md,
*For the owner*, L).

```sh
pnpm install                                   # once, at the repository root
node bench/catalog.mjs                         # bytes → bench/results/catalog.md
node bench/catalog-verify.mjs                  # browsers → bench/results/catalog-verify.md
reactogenic check -p bench/catalog-site/tsconfig.json
reactogenic build -p bench/catalog-site/tsconfig.json --out /tmp/catalog --report
```

| Path | What |
| --- | --- |
| `pages/**/index.rtsx` | a page: the route is the directory |
| `layout.rtsx` | the document from `<html>`: header, side menu, `<main>`, footer |
| `site.css` | the site's own CSS — page grid, text, and what a page lays out itself (the home page's hero, the pricing grid, the dashboard's grid). Unlayered. Imported once, by the layout: every page's sheet starts from all of it, as it does from all of the catalog, and pruning takes out what the page has no element for |
| `public/` | copied as it is |

## The pages, and what each uses — written before anything was measured

The selection was fixed before the first build was measured, from what such
a page has on real sites, and was not changed afterwards (the section *What
changed after the first measurement* below says what was, if anything).
Every component of the catalog is used by at least one page: the control's
sheet is "everything any page imports", and a component no page uses would
weigh on the control alone.

**The layout**, on every page: `SideMenu` (the site's navigation: a column
above 50rem, a drawer below), `DropdownMenu` (a menu of links, *Resources*,
in the header), `Button` (that menu's trigger, and the *Sign in* link). It
mounts `overlays`, so every page ships a script.

| # | Page | Kind | Beyond the layout | Why these | Behaviours beyond `overlays` |
| --- | --- | --- | --- | --- | --- |
| 1 | `/` | marketing home | `Card`, `Badge`, `Avatar` | a hero (the page's own markup), a grid of feature cards, a "New" pill, two testimonials with their authors | — |
| 2 | `/pricing/` | pricing | `Tabs`, `Card`, `Badge`, `Tooltip`, `Accordion` | monthly / yearly as tabs over the plan cards, "Most popular", a hint on a plan's limit, the FAQ | `tabs` |
| 3 | `/docs/` | docs article | `Breadcrumbs`, `Tabs`, `CodeBlock`, `Callout` | where the article is, the install command per package manager (the tab follows the URL's hash: a link can name it), samples with a copy button, a note and a warning | `tabs` with `RG_TABS_HASH`, `copy` |
| 4 | `/docs/api/` | API reference | `Breadcrumbs`, `Table`, `Badge`, `CodeBlock` | tables of options, each with its stability, and signatures to copy | `copy` |
| 5 | `/changelog/` | changelog | `Badge`, `Callout`, `Pagination` | releases with *Added* / *Fixed* / *Breaking* labels, a note on a breaking change, older releases | — |
| 6 | `/blog/` | blog post | `Avatar`, `Badge`, `Callout`, `CodeBlock` | the author, the post's tags, a pull-out, one sample (shown, not copied: no copy button) | — |
| 7 | `/dashboard/` | app dashboard | `Card`, `Progress`, `Table`, `Badge`, `Dialog`, an action `DropdownMenu` | figures in cards, quota bars, recent deployments with their status, a row's actions (typeahead), a confirmation | `menu-keys` with `RG_MENU_TYPEAHEAD`, `invokers` |
| 8 | `/settings/` | settings form — **the page that uses many** | `Tabs`, `Field`, `Select`, `Checkbox`, `Avatar`, `Callout`, `Toast`, `Dialog` | profile / notifications / security as tabs; text fields (a bio with a counter, a password that can be shown), a time zone, switches, the profile picture, the danger zone and its confirmation, "Saved" | `tabs`, `field` with `RG_FIELD_COUNT` and `RG_FIELD_REVEAL`, `toast`, `invokers` |
| 9 | `/contact/` | contact form | `Field`, `Select`, `Checkbox` | three plain fields, a topic, a consent box: no counter, no password — so no `field` behaviour | — |
| 10 | `/404/` | not found — **the page that uses almost none** | — | a heading and a link home (a `Button`, which the layout has already) | — |

Three to five components on seven pages, six on one (two of them of
`@reactogenic/ui`), eight on one, none on one.

The session — the order `bench/catalog.mjs` visits them in, with a warm
cache — is the table's order: a visitor who lands on the home page, looks
at the price, reads the docs, and signs in. Ten pages is a long visit; the
report has the running total after each page, so a visit of two, three or
five pages can be read off it.

## What is not realistic, and is said

- Every page has the side menu. A marketing page seldom has one; an app or a
  docs site does. `@reactogenic/ui` has one layout component for navigation,
  and the fixture uses it as the docs site does.
- The forms post nowhere, and *Save changes* is the `Toast`'s own trigger: a
  static page has no handler to say "Saved" after a request.
- No theme switch, no search, no syntax highlighting (decisions.md, 14).
- **The pages are short.** The text is filler, and there is less of it than
  on a real site: a page's HTML is 2.5–9.4 KB here, 9.7–36 KB on the docs
  site. So CSS and JS are a larger share of a page than on a real one, and
  every "of the page" percentage in `bench/results/catalog.md` flatters the
  default build. T5 compares CSS with CSS and JS with JS, and does not
  depend on it; the bytes saved per page do not either.
- **The catalog has options no page uses** (a flush accordion, a compact
  pagination, a card's media, …: `bench/results/catalog.md` lists them). A
  design system does; the control carries them and the default build does
  not, which is part of what is measured.

## What changed after the first measurement

The page list, and what each page uses, did not. Five things in the CSS
did, all found by the first browser pass (`catalog-verify.mjs`) or by
looking at its screenshots — none was made for a number, and each moves a
sheet by a few hundred bytes at most:

| | Was | Now |
| --- | --- | --- |
| `Tooltip` at 400 px | a bubble centred on its word, which left the screen on `/pricing/` (sideways scroll) | below 30rem it is a line under the text, margin to margin |
| `Table` with `sticky` | `overflow: visible` at every width: the table left the screen on `/docs/api/` at 400 px | sticky from 64rem up; below, the box scrolls |
| `Field`, `Select` in a row | stretched to the row's height, so a field without a hint stood lower than its neighbour | `align-content: start` |
| `site.css`, `.grid p` | greyed a card's figure and a plan's price too | `.grid p:not([class])` |
| `Avatar`'s picture | `loading="lazy"`: whether a cold load fetched it would depend on the viewport | loaded with the page — one request, on `/` |
