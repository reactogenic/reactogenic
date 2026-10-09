# The bet, measured

RGP2-050 ([plan.md](plan.md)). The numbers are of 2026-10-05, on the tree
that has the builder, the components and the docs site together — measured
on `rgp2-integrate` (afc30d7), again on `rgp2-final` after the review of
that tree changed the pruner, and **again after the owner's rulings of
2026-10-05 were built** ([decisions.md](decisions.md), *Ruled by the
owner*; plan.md, RGP2-050): routes and variants, a mount's data, the inline
threshold of 4096 B, a script of the page's own as an error, a page's own
`<style>` pruned. What moved with them, and no verdict did:

| | Before | Now | Why |
| --- | ---: | ---: | --- |
| `/syntax/`, HTML | 36,287 | 36,269 | `data-typeahead=""` is gone: the option is the mount's data |
| `/syntax/`, JS | 1,446 | 1,443 | `own?.typeahead` in place of `hasAttribute("data-typeahead")`; the data in the mount's call |
| the control's JS | 1,645 | 1,667 | its table carries a mount's data, and looks a document up by its file too |
| the control's script, delivered | a file | inlined in each page | 1,667 B is under 4096 B |
| the session, control against default (brotli) | 19.8% smaller | 12.9% smaller | the control's script is no longer fetched once |
| `/` and `/reference/cli/`, HTML | 9,689, 21,503 | 9,698, 23,331 | the pages' text: "dynamic segments"; the `build` reference describes routes, variants and the new rules |

With those rulings no sheet of the docs site changed by a byte, and no other
script.

**And once more, on the tree that has the rest of the rulings built** —
T5's unit and the catalog (decisions.md, L), state that only a script
writes (M), keyed slots in the written order (D: `slotKeys`), the close
button's name (G: `closeLabel`) — both sites with one binary. Every number
below is of that tree. What moved:

| | Before | Now | Why |
| --- | ---: | ---: | --- |
| the docs site's four sheets | 9,041, 7,224, 8,914, 7,179 | 8,969, 7,152, 8,842, 7,107 | M: 72 B each — the menu's `[aria-current]` rule is decided on the page now: no item of it is current, and no script names the attribute |
| selectors of the control's sheet that are on no page | 3 of 94 | 4 of 94 | that rule |
| `/syntax/`, HTML | 36,269 | 36,542 | a sentence of the page's text (`slots.rtsx`) |
| a page, cold, default against control (brotli, mean) | 7.9% smaller | 8.0% smaller | the sheets |
| the session, control against default (brotli) | 12.9% smaller | 12.8% smaller | the sheets |
| T5 on the docs site | read raw: its CSS half held on two pages | read in brotli: no page at 20% | L: the unit |
| the catalog's ten sheets, first measured before M was built | 8,888 to 16,551; `/404/` 6,082 | 8,816 to 16,479; `/404/` 5,852 | M: 72 B each, and 230 B on `/404/`, which has no current page at all |

No script changed, of either site, and no HTML but `/syntax/`'s: D and G
are no byte at the defaults.

**And on the branch of the owner's seven rules** (2026-10-06;
decisions.md, *K, ruled and reversed*; plan.md, RGP2-071) — an option a
rule selects is a class of its own, through `variants()`
(`data-variant="ghost"` → `rg-button-ghost`, `data-align="end"` →
`rg-menu-end`, and every option of the catalog fixture); nothing else that
ships changed: parts are `data-part` and pruning is what it was. Both sites
with one binary, built from that tree; every number below is of it. What
moved:

| | Before | Now | Why |
| --- | ---: | ---: | --- |
| the docs site's four sheets | 8,969, 7,152, 8,842, 7,107 | 8,961, 7,144, 8,834, 7,099 | two selectors, 4 B each: `[data-variant=ghost]` → `.rg-button-ghost`, `[data-align=end]` → `.rg-menu-end` |
| the control's sheet | 9,352 | 9,344 | the same two |
| the docs site's HTML | 9,698, 10,548, 36,542, 23,331 | 9,678, 10,538, 36,532, 23,321 | a class is shorter than its attribute: 5 B a ghost button or an end-aligned menu |
| the catalog's ten sheets; the control's | 8,816 to 16,479, `/404/` 5,852; 31,961 | 8,798 to 16,449, `/404/` 5,844; 31,883 | its options — tone, size, variant, … — as classes |
| a page, cold, default against control (brotli, mean) | 8.0% smaller; 50.7% | 8.0%; 50.7% | |
| the session, control against default (brotli) | 12.8% smaller; 44.8% | 12.7%; 44.7% | |
| T5 on the docs site, CSS (brotli) | 2.6%, 16.1%, 3.6%, 16.5% | 2.5%, 16.2%, 3.6%, 16.8% | |

No script changed, of either site; no threshold's outcome did.

<!-- The numbers of this file are copied from bench/results/site.md,
     delta.md, catalog.md and factor.md, and the counts of the browser runs
     from verify.md and catalog-verify.md — all six written on this tree,
     with one binary. -->

**Verdict: holds for a page loaded cold; does not hold over a visit.**

| | The docs site | The catalog |
| --- | --- | --- |
| what it is | `site/`: four pages, three components in one shared layout | `bench/catalog-site`: ten pages on twenty components, three to five to a page — **a fixture**, written for this measurement |
| the thresholds that can refute | hold: T1, T2, T4, T6, T7 (702 browser checks pass, 8 known, none fails) | hold, as far as each can be asked there: T1, T2's bound, T6, T4's question twice — and the pruned pages against the unpruned in two engines (1,348 checks pass, none known, none fails) |
| **T5**, in brotli | **fails**, on both halves: no page's CSS is 20% smaller than the control's (2.5% to 16.8%); `/syntax/`'s script is 17.2% smaller | **holds**, on every page: CSS 41.8% to 72.9% smaller, JS 41.9% to 91.6% |
| a page, cold | 8.0% lighter than the control's | 50.7% lighter |
| a visit | the control transfers 12.7% less over four pages, and is ahead from the second | the control transfers 44.7% less over ten, and is ahead from the third |

**Cold.** Every threshold that can refute the bet holds, on both sites.
T5 — what awareness adds over the same build without it — fails on the
docs site: three components in one layout, and one page that mounts
everything. It holds on every page of the catalog. A page of the catalog is half the
control's bytes — of a fixture whose pages are short (*The catalog*); a
page of the docs site is 0.2–0.9 KB lighter.

**A visit.** The build without awareness transfers less from the second
page of the docs site (9,580 B against 10,232 after two, in brotli; 26,064
against 29,871 over four) and from the third of the catalog (12,870
against 13,748 after three; 23,953 against 43,305 over ten), because
per-page pruned sheets share nothing. What packaging could share instead —
with analysis left as it is — is the factoring policy: open with the owner
(decisions.md, K), with a study (*The factoring study*, below).

**The word.** The owner's, since 2026-10-06 (decisions.md, K, rule 4;
plan.md, RGP2-050): the thresholds stay as written; "current
implementation fails visit/request targets; cold T5 remains undecided
because the fixture doesn't exercise the stated condition." The visit: the
paragraph above — and plan.md has no threshold for it yet. Requests: none
fails on this build (T8: 2 a page); 4 a page was the build of the ruling
that was reversed (*The ruling that was reversed*, below). Cold T5: it
fails on the docs site and holds on the catalog, and neither is a design
system larger than what its site uses — the docs site uses three
components of three, the catalog site all twenty.

## The question

*If the compiler is aware of components it can ridiculously optimise the
output build.* Made testable by [research.md](research.md): pages written
with `SideMenu`, `Dialog`, `DropdownMenu` and **no hand-written JS** must
land near a hand-written floor, and the same site built with awareness
**off** — the control, `--no-specialize` ([builder.md](builder.md), *The
control*) — must be measurably worse. plan.md has eight thresholds: the bet
is confirmed if all hold, refuted if a marked one fails.

## How it was measured

```sh
pnpm install --frozen-lockfile
node bench/site.mjs     # T1, T2, T3, T5, T6, T8 → bench/results/site.md
node bench/delta.mjs    # T4                     → bench/results/delta.md
node bench/verify.mjs   # T7                     → bench/results/verify.md
node bench/catalog.mjs          # the catalog: T5, T1, T2, T6, T8, T4's question → bench/results/catalog.md
node bench/catalog-verify.mjs   # the catalog in browsers                        → bench/results/catalog-verify.md
node bench/factor.mjs           # the factoring study: packagings of one analysis → bench/results/factor.md
```

Each script builds the binary from `go/` (`go build -trimpath
./cmd/reactogenic`) — or takes `$REACTOGENIC_BINARY`: the numbers here are
of one binary for all six — and its site — the first three the docs site
(`site/`: four pages), the next two the catalog site (`bench/catalog-site`:
ten), the last both — into a temporary directory; `bench/README.md` has the options. The full
tables are in `bench/results/`; what follows is taken from them.

| | |
| --- | --- |
| builds | `reactogenic build` (what ships); `--inline always`; `--no-specialize` (the control: one sheet and one script for the site, the same HTML); and `--inline never` — not one of the plan's three: the default's blobs as files |
| bytes | each file on its own: raw, `gzip -9`, `brotli -q 11` (Node's zlib). "Raw / gzip / brotli" below |
| cold load | `bench/measure.mjs`: every page in headless Chrome 154 from a local server, fresh profile, three runs, the union of the requests |
| session | the four pages in order — `/`, `/guide/`, `/syntax/`, `/reference/cli/` — with a warm HTTP cache: each URL once |
| what a page is | its HTML as rendered, its CSS, its script — cut out of the document and checked against `_rg/report.json`: 64 raw sizes, all equal |
| browsers | Chromium 153.0.8010.12 (the full browser, back/forward cache on) and WebKit 26.6, by Playwright, at 1200 and 400 px |
| machine | darwin-arm64, Go 1.27.1, Node 25.2.1. `site.mjs` and `delta.mjs` write the same bytes on a second run |

## The numbers

What each page ships — the same in the default build, `--inline always`
and `--inline never` — and what the control ships on every page:

| Page | HTML as rendered | CSS | JS | Behaviours |
| --- | ---: | ---: | ---: | --- |
| `/` | 9,678 / 3,451 / 2,806 | 8,961 / 2,405 / 2,091 | 563 / 327 / 250 | `overlays`, `invokers` |
| `/guide/` | 10,538 / 3,680 / 3,054 | 7,144 / 2,076 / 1,797 | 252 / 176 / 126 | `overlays` |
| `/syntax/` | 36,532 / 10,142 / 8,697 | 8,834 / 2,382 / 2,068 | 1,443 / 707 / 588 | `overlays`, `menu-keys` with typeahead — the flag, and the mount's data `{"typeahead":true}` — `invokers` |
| `/reference/cli/` | 23,321 / 7,508 / 6,338 | 7,099 / 2,066 / 1,785 | 252 / 176 / 126 | `overlays` |
| the control, every page | the same HTML | 9,344 / 2,472 / 2,145 | 1,667 / 841 / 710 | all three, and a table |

The control's script is its behaviours, 1,387 / 683 / 563, and its own cost
— the list of modules, the table from a document's path to its mounts, the
loop that reads it — 280 / 230 / 178. `/syntax/` without its mount calls is
1,387 B too: the control's behaviours are that page's, to the byte count.

**Against the control**, smaller by (raw / gzip / brotli):

| Page | CSS | JS | JS, against the control's behaviours alone (raw) | CSS + JS, brotli | … of the page |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 4.1% / 2.7% / 2.5% | 66.2% / 61.1% / 64.8% | 60.0% | 2,855 → 2,341 | 9.1% |
| `/guide/` | 23.5% / 16.0% / 16.2% | 84.9% / 79.1% / 82.3% | 82.1% | 2,855 → 1,923 | 15.8% |
| `/syntax/` | 5.5% / 3.6% / 3.6% | 13.4% / 15.9% / 17.2% | 0.0% | 2,855 → 2,656 | 1.7% |
| `/reference/cli/` | 24.0% / 16.4% / 16.8% | 84.9% / 79.1% / 82.3% | 82.1% | 2,855 → 1,911 | 10.3% |

"Of the page" is HTML + CSS + JS, each compressed apart. 72 of the control
sheet's 94 selectors and at-rules are on all four pages; `/reference/cli/`
has none of its own, `/guide/` one. Four are on no page.

**As delivered.** Under `auto` — a file when two or more pages share a blob
of 4096 B or more ([builder.md](builder.md), *Packaging*) — every blob of
the default build is inlined: no two pages have the same sheet, and the one
script two pages share is 252 B. So the default build is `--inline
always`'s. Of the control's two blobs, which all four pages share, the sheet
(9,344 B) is a file and the script (1,667 B) is inlined in every page.

| Build | Requests per page, cold | Page, cold, mean | Session: requests | Session: total |
| --- | ---: | ---: | ---: | ---: |
| default, and `--inline always` | 2 | 28,934 / 8,867 / 7,585 | 5 | 115,035 / 34,941 / 29,871 |
| control | 3 | 31,346 / 9,672 / 8,242 | 6 | 96,651 / 30,745 / 26,064 |
| `--inline never` | 4 | 29,000 / 9,011 / 7,633 | 12 | 115,047 / 35,340 / 29,939 |

The second request of the default build is the favicon (234 B). Cold, the
default build is 8.0% smaller than the control (brotli, mean) with two
requests to its three. **Over the session the control is 12.7% smaller**
(16.0% raw): cumulative brotli after each page, default 5,269 → 10,232 →
21,620 → 29,871, control 5,816 → 9,580 → 19,016 → 26,064 — the control is
ahead from the visitor's second page. Its sheet is fetched once (its script
is in every page: 710 B brotli of each); the default build's four sheets —
8,961, 7,144, 8,834 and 7,099 B, of which 7,099 B are the same rules:
`/reference/cli/`'s sheet is what every page has — are each inlined in
their page, and as files (`--inline never`) they are four files.

**Against React.** The best React build of the research
(`bench/baselines/2026-10-04.md`, `c1-astro-radix`: Astro + React islands +
Radix) parses 317–321 KB of JS per page (90 KB brotli), in 14–18 requests.
That is **another site of the same shape** — four docs pages with a side
menu, a dialog and a menu, built during the research and not in the
repository — not this site built with React. Page against page, this
site's JS is 564×, 1,257×, 223× and 1,266× smaller raw; its heaviest page
against that build's lightest, 219× raw and 152× brotli.

**Against hand-written code.** Not claimed (plan.md): no hand-written or
Astro build of *this* site exists. The research's numbers, of its own
site — which has a theme switch, copy buttons and search, 76% of its JS —
for scale only:

| | JS to parse, mean, raw | Page, mean, brotli | Requests |
| --- | ---: | ---: | ---: |
| the hand-written floor (`d-floor-page`) | 897 | 7,527 | 3 |
| the floor at behaviour parity | 1,584 | — | 3 |
| Astro + hand-written scripts (`c2-astro-vanilla`) | 829 | 7,668 | 2 |
| this site, default build | 628 | 7,585 | 2 |

## The thresholds

| | Threshold | Measured | |
| --- | --- | --- | --- |
| T1 runtime (refutes) | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, no `<runtime>` row | on all four pages the report's rows add up to the script — 248 `overlays` + 307 `invokers` + 8 entry = 563; 248 + 4 = 252; 248 + 832 `menu-keys` + 307 + 56 = 1,443 — every row is a mounted behaviour or the entry, none is `<runtime>`. Read: each script is function declarations and one constant, then the mount calls — one of them with its data, `{typeahead:!0}` — which are the only statements that run; one `<script>` per document, no handler attribute. (Since the rulings the builder refuses a page that has any other: shell-script.) The three distinct scripts are printed whole in `bench/results/site.md` | **pass** |
| T2 JS per page (refutes above 5 KB brotli) | ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page | `/syntax/`: 1,443 raw, 588 brotli. 57 B under the budget: one more behaviour of `invokers`' size would exceed it | **pass** |
| T3 against React | ≥ 100× below the best React build of an equivalent site | 219× raw at the worst pairing, 152× brotli. Another site (above) | **pass** |
| T4 precision (refutes) | deleting the *Install* dialog from `/` removes its markup, the CSS rules only it matched and `invokers` from that page, nothing else; every other page the same bytes | HTML: one span of 1,065 B cut out — the trigger and the `<dialog>`, whole — every other byte where it was. CSS: −1,752 B, 14 selectors, each naming `.rg-dialog`; none came; none that stays names what left. JS: 563 → 252, `invokers` left, `overlays` the same bytes; the script is now `/guide/`'s. The other three documents: the same bytes, under `--inline always` and as the site ships | **pass** |
| T5 awareness | against the control, in brotli bytes: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one | CSS: 2.5%, 16.2%, 3.6%, 16.8% — no page at 20% (raw: 23.5% and 24.0% on two). JS: 17.2% on `/syntax/` (raw: 13.4%; against the control's behaviours alone, 0.0%). On the catalog it holds: *The catalog* | **fail** here |
| T6 authoring (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 21 source files: `.rtsx`, one `.css`, `.json`, `.md`, `.svg`. No `<script>`, `<style>`, stylesheet link, `style` attribute, handler, `mount(`, import of a behaviour or `dangerouslySetInnerHTML` outside the code samples the pages show; one stylesheet import in the whole site, `layout.rtsx:8`. `site/test/browser.mjs` is JS: a test of the built site, not built into it | **pass** |
| T7 behaviour (refutes) | the browser checks pass on the built site | 702 passed, 8 known, 0 failed: Chromium 153.0.8010.12, 355; WebKit 26.6, 347 and 8 known (below). 688 of them are of the site; 14 of the builder's fixture, which the threshold does not ask for | **pass** |
| T8 requests | ≤ 3 per page, cold | 2: the document and the favicon. (`--inline never`: 4) | **pass** |

**T5's unit.** plan.md gave none. It was first read in raw bytes, as T2 and
T3 are, and its CSS half held raw (two pages) and not compressed. The owner
ruled (2026-10-05; decisions.md, L): **brotli bytes** — what a page
transfers, each blob compressed on its own (`brotli -q 11`, as
`bench/measure.mjs` does), with raw and gzip reported beside it. plan.md
says so now, and `bench/site.mjs` decides it so. In brotli T5 fails on the
docs site on both halves. The JS half has to, in every unit: `/syntax/`
mounts every behaviour the site has with its one flag on, and the control
is the union over the site — no page that uses everything can be smaller
than everything. The CSS half fails because a page's sheet there is 76–96%
the layout's and the site's own (*Where it added little*). Neither is so
where a page uses a few components of many: *The catalog*.

**T4, as first written** — the cheat-sheet dialog of `/syntax/` — is not a
deletion of one component: a menu item commands it. `delta.mjs` runs it
both ways. The dialog alone: the build refuses, `error idref-not-found:
Page /syntax/: commandfor="cheat-sheet" on <button> names no element of
the page`. With its item: the menu becomes a list of links (`role="menu"`
and `menu-keys` leave with `invokers`: 1,443 → 252 B), 15 selectors leave —
one of them, `.rg-menu>:is(a,button)`, because the items are no longer the
menu's children — and no other page changes.

**T7, what ran.** On every page at both widths: `aria-current="page"` on
the page's own link and on nothing else, its group the one open; the links
menu anchored below its trigger, end-aligned, in the viewport, with no menu
role; the side menu a sticky column at 1200 px, and at 400 px a drawer that
opens, closes on its scrim without activating what is behind, on its close
button and on a link into the page. The Install dialog: modal, in the
viewport, named by its title; Esc, the close button, the scrim and the
*Close* action each close it with focus back on *Install*. The action menu:
focus on its first item, ArrowDown and ArrowUp with wrap, Home, End,
typeahead, Tab closes it, its item opens the cheat sheet and closes the
menu, Esc returns focus to the menu's trigger. With the engine's `command`
taken away, the page's script opens both dialogs. Back (Chromium): a page
left through a link in its open drawer, action menu or dialog comes back
from the page cache with nothing open — and the same page without its
script comes back with it open.

- **Computed styles**, the default build against the control: 248
  comparisons of the site (124 per engine), every element and its
  `::before`, `::after`, `::marker`, `::backdrop` — at rest, dark, with
  reduced motion, under the pointer (a link, a button, a menu item, a
  summary, the dialog's close button), with keyboard focus, and with the
  links menu, the drawer, each dialog and the action menu open. All equal.
  (2,317 elements and pseudo-elements on average over the 256 comparisons
  of the run, the fixture's eight among them.) The comparison is checked
  against itself: one matching rule taken out of one page is seen.
- **What the site cannot show.** Its three behaviours write nothing to the
  page, so no state of it has a class taken away or an element gone — and
  that is where the review found the pruner wrong: `.card:not(.collapsed)`
  was dropped for a card that is collapsed as it loads, and matched after
  `classList.toggle`. So the same comparison runs on the builder's own
  fixture (`go/internal/build/testdata/served`), 8 more (4 per engine):
  `/toggle/` as loaded and after a click that toggles a class off, removes
  one, writes an id over and sets a text over an element — four rules match
  then that matched nothing — `/frame/`, whose body gets a class from the
  document in its `<iframe>`, and — since the rulings — `/styled/`, whose
  own `<style>` elements the default build prunes with the page's sheet and
  the control leaves as written. All equal; with the pruner as it was
  before the review, the frame's is not.
- **The probe** of the pruner's tables (`cssprune/selector.go`): each of
  the 119 selectors the pruner takes for known — its pseudo-classes and
  pseudo-elements, the legacy one-colon forms, 16 forms of each `:nth-*()`
  name — is one rule in `SEL, p {}` in both engines. No name leaves the
  table.
- **Another packaging**: `node bench/verify.mjs --inline never` — every
  blob a file — is not in this run's results: `bench/results/verify.md` is
  of the default packaging.
- **Known**, 8, all WebKit: a dialog opened by a click leaves focus on
  `<body>` when it closes (2: at 1200 and at 400 px; components.md, *Known
  limits*); Playwright's WebKit has no page cache, so Back restores
  nothing (6: three pages left with an overlay open, each with and without
  its script).

## The catalog

The ruling on T5 (decisions.md, L). The docs site cannot show what
awareness is worth where a page uses a few components of many: it has
three, in one layout, and a page that mounts everything.

**What it is.** A measurement fixture — not a product, not the docs site,
not the design system (that is `packages/ui`):

| | |
| --- | --- |
| `bench/catalog` | twenty components in `.rtsx` with typed slots, one `.css` each in components.md's convention: the four of `@reactogenic/ui` (`Button`, `Dialog`, `DropdownMenu`, `SideMenu`) and sixteen written for this — `Accordion`, `Avatar`, `Badge`, `Breadcrumbs`, `Callout`, `Card`, `Checkbox`, `CodeBlock`, `Field`, `Pagination`, `Progress`, `Select`, `Table`, `Tabs`, `Toast`, `Tooltip`. Seven behaviours — `overlays`, `invokers`, `menu-keys`, and `tabs`, `field`, `toast`, `copy` — with four flags between them |
| `bench/catalog-site` | ten pages of the kinds a product site has, in one layout (the side menu, a menu of links, a button), built by `reactogenic build`: `reactogenic check` and the three builds print no diagnostic |
| their size | a component's CSS is 0.5–2.0 KB minified (1.2 KB on average; `@reactogenic/ui`'s four: 0.4–2.5 KB; Bootstrap 5.3's, for the fourteen kinds it has: 0.5–6.1 KB, 3.2 KB on average). A behaviour is 360–800 B in a page's script. Sized against those two references, not against the result (`bench/catalog/README.md`) |

**How it was chosen.** Which page uses which components, and why, was
written down before the first build was measured
(`bench/catalog-site/README.md`) and not changed after: three to five
components beyond the layout's on seven pages, six on the dashboard, eight
on the settings page — the one that uses many — and none on the 404 page.
Every component is used by some page, so that the control carries nothing
that only it would. What changed after the first browser pass is five
fixes to the fixture's CSS, listed there.

**The numbers**, in brotli bytes (`bench/results/catalog.md` has raw and
gzip, and every table whole), of the same tree and binary as the docs
site's. The control ships every page the same sheet — 31,883 / 6,446 /
5,740 B — and the same script, 4,277 / 1,703 / 1,507 B:

| Page | Beyond the layout | CSS | smaller than the control's 5,740 | JS | smaller than the control's 1,507 | CSS + JS | the page, smaller by |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | `Card`, `Badge`, `Avatar` | 2,141 | 62.7% | 126 | 91.6% | 2,267 against 7,247 | 56.1% |
| `/pricing/` | `Tabs`, `Card`, `Badge`, `Tooltip`, `Accordion` | 2,581 | 55.0% | 404 | 73.2% | 2,985 against 7,247 | 48.1% |
| `/docs/` | `Breadcrumbs`, `Tabs`, `CodeBlock`, `Callout` | 2,440 | 57.5% | 649 | 56.9% | 3,089 against 7,247 | 45.2% |
| `/docs/api/` | `Breadcrumbs`, `Table`, `Badge`, `CodeBlock` | 2,399 | 58.2% | 307 | 79.6% | 2,706 against 7,247 | 49.6% |
| `/changelog/` | `Badge`, `Callout`, `Pagination` | 2,266 | 60.5% | 126 | 91.6% | 2,392 against 7,247 | 55.6% |
| `/blog/` | `Avatar`, `Badge`, `Callout`, `CodeBlock` | 2,246 | 60.9% | 126 | 91.6% | 2,372 against 7,247 | 54.6% |
| `/dashboard/` | `Card`, `Progress`, `Table`, `Badge`, `Dialog`, an action `DropdownMenu` | 2,756 | 52.0% | 588 | 61.0% | 3,344 against 7,247 | 43.8% |
| `/settings/` | `Tabs`, `Field`, `Select`, `Checkbox`, `Avatar`, `Callout`, `Toast`, `Dialog` | 3,342 | 41.8% | 876 | 41.9% | 4,218 against 7,247 | 32.3% |
| `/contact/` | `Field`, `Select`, `Checkbox` | 2,213 | 61.4% | 126 | 91.6% | 2,339 against 7,247 | 58.1% |
| `/404/` | — | 1,553 | 72.9% | 126 | 91.6% | 1,679 against 7,247 | 70.2% |

A page's HTML is the same in both builds. The control's script is its
behaviours — 3,587 / 1,412 / 1,210 B — and its table, 690 / 364 / 330 B;
against the behaviours alone the page's own are 32.6% to 89.9% smaller.

**The session.** Cold, a page of the default build transfers 4,454 B
(brotli, mean of ten) in 2 requests — 3 on `/`, which has a picture — and
the control's 9,041 B in 4: 50.7% less. With a warm cache, the ten pages in
the order of the table:

| After | default | control | |
| --- | ---: | ---: | --- |
| 1 page | 4,169 | 9,218 | the default build, by 54.8% |
| 2 pages | 8,737 | 10,879 | the default build, by 19.7% |
| 3 pages | 13,748 | 12,870 | the control, by 6.4% |
| 5 pages | 22,169 | 16,336 | the control, by 26.3% |
| 10 pages | 43,305 | 23,953 | the control, by 44.7% |

The control's sheet and script are two files, fetched once: 7,247 B, and
after that a page is its HTML. The default build's ten sheets are ten
different blobs of 1.6–3.4 KB, each inlined in its page, and of its six
scripts the one five pages share is 252 B, under the 4096 B at which a
shared blob becomes a file (builder.md, *Packaging*). As files
(`--inline never`) they are ten sheets and six scripts all the same:
43,602 B, and 28 requests. A reader who sees one page or two is better off
with the default build; from the third, with the control.

**The thresholds, on it** — as each can be read on a fixture that is not
the site they were written for:

| | Read on the catalog as | Measured | |
| --- | --- | --- | --- |
| T1 runtime (refutes) | as written | on all ten pages the report's rows add up to the script — 252 B to 2,448 B — every row a mounted behaviour or the entry, none `<runtime>`; the only statements that run are the mount calls | **pass** |
| T2 JS per page (refutes above 5 KB brotli) | the 1.5 KB budget is for the docs site's three behaviours (decisions.md, F) and is not carried over; its bound is, and the heaviest page is reported | `/settings/`, five behaviours: 2,448 / 1,022 / 876 B. Eight of ten pages are under 1.5 KB raw. The docs site's budget is 500 B a behaviour mounted; here `/settings/` is at 490 B and `/docs/` — three behaviours, 341 B of mount calls — at 583 B: said, and not judged, the reading being made after the fact | **pass** on the bound; the budget is not judged |
| T3 against React | not applicable: no React build of this fixture exists | — | not measured |
| T4 precision (refutes) | the same question of two other components (T4 names the docs site's dialog) | the FAQ `Accordion` deleted from `/pricing/`: 1,115 B of markup in one span, 12 selectors that all name `.bc-accordion`, no behaviour; the `Toast` from `/settings/`: 384 B, 8 selectors that name `.bc-toast` and the token only it read, the `toast` behaviour (2,448 → 1,932 B). No other page changed, either time | **pass** |
| T5 awareness | in brotli bytes; "at least two pages" reads "at least half of the pages" | CSS: 41.8% to 72.9% smaller, ten pages of ten at 20% or more. JS: 41.9% to 91.6%, every page at 30% or more. It holds in gzip and raw too | **pass** |
| T6 authoring (refutes) | of the site's source; the catalog is the design system's side of the rule | 17 files: no `<script>`, `<style>`, handler, `mount(` or behaviour import; one stylesheet import, in `layout.rtsx` | **pass** |
| T7 behaviour (refutes) | a sanity pass and the style comparison, not a suite per component | *In a browser*, below: 1,348 checks pass, none is known, none fails | **pass**, as far as it was asked |
| T8 requests | as written | 2 per page, 3 on `/` (the document, the favicon, a picture). The control: 4 and 5 | **pass** |

**In a browser** (`bench/results/catalog-verify.md`). The site and the
control in Chromium 153.0.8010.12 and WebKit 26.6, at 1200 and 400 px:
**1,348 checks pass, none is known, none fails** — 674 in each engine.

- **Every page loads**, at both widths, in both builds: a title, a
  heading, no sideways scroll, one script — the builder's — no React, and
  no console error, page error, failed request or 4xx — 20 of 20 loads in
  each engine.
- **Pruning changes nothing that is seen**, asked two ways in each state.
  The computed style of every element and of its `::before`, `::after`,
  `::marker` and `::backdrop` is the same in the two builds; and no
  selector that the control's sheet has and the page's own lacks matches
  an element of the page — a question that would name the selector that
  was wrongly dropped. In 244 states per engine: 488 comparisons of
  computed styles, 826 elements and pseudo-elements each on average, all
  equal; and 488 times the second question, 211 dropped selectors asked
  each time on average, none matching. Both
  questions are checked against themselves: with one matching rule taken
  out of one page, each sees it.
- **The states**: at rest, dark, reduced motion, forced colours, keyboard
  focus, the layout's menu open, its drawer at 400 px, hover on links,
  rows, tabs and buttons — and what the catalog's behaviours write, which
  the docs site's never do: a tab selected by a click, by a key and by the
  URL's hash (`aria-selected`, `tabIndex`, `hidden`), a field counted to its
  limit (`data-full`), a password shown (`type`, `aria-pressed`), a toast
  shown, a sample copied, a switch thrown, an accordion item opened, the
  action menu and both dialogs open. With M built these are the states
  that matter most: an `aria-*` rule stays in a page's sheet only because
  the page's script names the attribute.
- **The behaviours do what they are for** — each checked once, as the way
  into a state: arrow keys and Home / End on tabs, the hash, typeahead,
  the counter, the reveal button, the toast hiding itself after its
  timeout, the copy button saying *Copied*.
- No check failed, on this tree or on the one before M was built: no
  selector was wrongly dropped, and no defect of the pruner or of the
  builder showed. Two of the fixture's own CSS did, by its first pass
  (*Found on the way*).

**What is not realistic**, and bears on the numbers:

- **The pages are short**: 2.5–9.4 KB of HTML, against 9.7–36 KB on the
  docs site. So HTML is 29–42% of what a page transfers here (54–77%
  there), and every "of the page" share above flatters the default build.
  T5 compares CSS with CSS and JS with JS and does not move with it; nor do
  the bytes saved a page: 3.0–5.6 KB.
- **Every page has the side menu**, 2.5 KB of CSS that a marketing page
  would not carry — in both builds.
- **The catalog has options no page uses**: 16 of the control's 313
  selectors, ≈ 1.2 KB raw, are on no page. A design system has such; they
  are part of what awareness removes, and 3.8% of the control's sheet.
- **The weight is a choice.** A design system of Bootstrap's weight would
  roughly triple the control's sheet, and what awareness removes of it. It
  was not built.

## What it shows

1. **No React, no runtime, no hand-written JS** (T1, T6): the site's
   source is components and one stylesheet, and a page's script is the
   behaviours its components mounted, 252 B to 1,443 B.
2. **Per-page precision** (T4): a component deleted from a page takes
   exactly its markup, its selectors and its behaviour with it, and touches
   no other page.
3. **The pruned build looks and behaves as the unpruned one** (T7), in two
   engines, in every state that was tried. On the docs site that is a
   weaker statement than it reads: its behaviours write nothing, and the
   pruner was wrong exactly where one does (above; fixed, and tried on the
   builder's fixture). The catalog's behaviours do write — `aria-selected`,
   `hidden`, `tabIndex`, `data-full`, `aria-pressed`, an input's `type`, a
   button's text — and there the two builds are equal too (*The catalog*,
   *In a browser*).
4. **Where a page uses a few components of many, awareness halves the
   page** (T5, on the catalog): 42–73% of the control's CSS and 42–92% of
   its JS do not ship, on every one of ten pages, and a page is 3.0–5.6 KB
   lighter cold — in brotli, of a control that is 7.2 KB.

## What component awareness itself added

The control is the same compiler, the same components and the same HTML,
without the per-page decisions. Against it:

| | The docs site | The catalog |
| --- | --- | --- |
| JS | 224 to 1,415 B raw per page (122 to 584 B brotli). 280 B of it is the control's own table | 1,829 to 4,025 B raw (631 to 1,381 B brotli). 690 B of it is the control's table, which grows with the site |
| CSS | 383 to 2,245 B raw per page (54 to 360 B brotli) | 15,434 to 26,039 B raw (2,398 to 4,187 B brotli) |
| a page | 199 to 944 B brotli: 1.7% to 15.8% of what the page transfers | 3,029 to 5,568 B brotli: 32% to 70% of what a page transfers — of pages that are short (*The catalog*) |
| a flag | the site has one, `RG_MENU_TYPEAHEAD`, and one mount with data, `{ typeahead: true }`: turning `typeahead` off takes 300 of `menu-keys`' 832 B and 15 B of the entry from `/syntax/` — 315 B — and no CSS and no byte of HTML (`delta.mjs`) | four. `tabs` is 799 B on the page whose tabs follow the hash and 626 B on the two whose tabs do not; a `Field` without a counter or a password mounts no script at all (`/contact/`: 252 B, `overlays` alone) |
| tokens | no custom property is dropped: every page reads all eight | a page's `:root` keeps the custom properties the page reads: 3 to 14 declarations are dropped per page, of the fixture's twelve tokens, the design system's eight and the site's two |
| requests | 2 instead of 3 — packaging, which the control could have too (`--no-specialize --inline always` was not measured) | 2 instead of 4: the control's sheet and script are both files there |
| checks | a button that commands a dialog that is not on the page stops the build (above); links are checked per page; ids and `aria-current` are generated per page, and right in the browser | the same checks passed on ten pages; none was provoked |
| soundness | a page's sheet is pruned without anything seen changing: that takes the page's HTML, the page's script and the components' CSS convention together (components.md, *CSS convention*) | the same, with 167 to 260 of the control's 313 selectors dropped per page, and behaviours that write state |

## Where it added little

- **The page that uses everything**: `/syntax/` gains 224 B of JS — less
  than the control's table — and 510 B of CSS.
- **A shared layout**: the side menu, the links menu and the button are on
  every page, and the site's own sheet is used nearly whole everywhere — 35
  to 37 of its 38 rules. A page's sheet is 76–96% of the bundle; the
  research expected 39–76% (research.md).
- **A visit**: per-page sheets are different blobs, so nothing is cached
  across pages. On the docs site, where they differ by a rule or two, the
  control wins from the second page (above); splitting a sheet into what
  every page keeps and the page's own rest — (b) of the factoring study,
  computed from the sheets and not built — puts 89 of the sheet's units in
  one file and leaves two pages some 1.8 KB and two a rule or none: over
  the four pages it transfers 4.1% less than the control (*The factoring
  study*). **On the catalog awareness
  still loses a visit, and by more**: the sheets differ by whole
  components, the control wins from the third page, and over ten it
  transfers 44.7% less (*The catalog*, *The session*). There 47 of the
  control's 313 selectors are on every page — ≈ 4.7 KB raw: the layout's.
  The better awareness prunes, the less two pages' sheets have in common;
  what packaging should share of it is the owner's (decisions.md, K), with
  the study below, and is not designed here.
- **The page that uses many**: `/settings/` of the catalog, eight
  components and five behaviours, is where T5 is nearest its bar — CSS
  41.8% smaller, JS 41.9% (32.6% against the control's behaviours alone,
  bar 30%). A site whose typical page is that page would fail it.
- **The HTML**: 54–77% of what a page transfers, and React's static
  output to the byte. Nothing is done to it.
- **Unused CSS at the site's level**: 4 of 94 selectors match nothing on
  any page.
- **State nobody can reach**: every page's sheet had 301 B (raw) of rules
  for a disabled button, a disabled menu item and a menu's current item,
  though no page has such an element and no behaviour makes one. The owner
  ruled on it (decisions.md, M, option 2), and it is built: an attribute
  that only a script writes — `aria-*`, `disabled`, `checked`, … — is
  decided on the page unless the page's script names it. That took the
  menu's `[aria-current]` rule, 72 B, out of every sheet. **229 B stay**:
  `.rg-button:is(:disabled,[aria-disabled=true])` and the menu's same rule
  — `:disabled` is a pseudo-class, and every pseudo-class is still
  "maybe". About 3% of a sheet. On the catalog, by a cruder count — rules
  for a state attribute that no element of the page has: 0 to 336 B a
  page, up to 4% of a sheet (`bench/results/catalog.md` has it per page).

## What the measurement does not show

- **That awareness is where the win over React comes from.** It is not:
  the control has no React either and is 190× below the React baseline
  (1,667 B against 317 KB). The research found the same of hand-written
  platform code — Astro with hand-written scripts is at 829 B of JS and
  7.7 KB a page. The 219–1,266× is what leaving React is worth; awareness
  is the last 13–85% of 1.7 KB.
- **How near the floor this site is.** No hand-written build of it exists.
  T2 is a budget, not a ratio.
- **What a generic tool gets from the same inputs.** The pruner needs the
  emitted HTML and the names the script writes — not the component model. A
  text-scanning pruner or an import-graph bundler on this site was not run.
  The barrel import (`@reactogenic/ui`) reaches every component's CSS from
  every page, so an import graph alone would ship the control's sheet; that
  is reasoned, not measured. On the catalog the same holds for its barrel —
  and a pruner that reads the built HTML without the component model would
  get much of the CSS half of T5; what it would get was not measured there
  either. The JS half is the record's: which behaviours a page mounted,
  with which flags.
- **A real site of that scale.** The catalog is a fixture: twenty
  components and ten pages written in a day by the hand that measured
  them, with no user. The selection of components per page was fixed
  before measuring and is argued in its README, but nobody else chose it;
  the pages are shorter than real ones; the sixteen components have no
  spec, no contract tests, and a browser pass in place of a suite. What it
  shows is how the builder behaves when pages differ in what they use —
  not what a given product would save.
- **That a visit is ever better with awareness, as built.** On neither
  site is it, from the third page on. What other packagings of the same
  analysis would transfer is computed, for both sites, and none is built
  (*The factoring study*): on the docs site sharing what every page needs
  wins the visit by a few percent; on the catalog nothing that keeps T5
  does.
- **That pruning is sound for any behaviour.** For the three of the design
  system, and for the fixture's one that toggles, removes and rewrites by
  name. A behaviour that computes the name it writes (`"is-" + state`) is
  outside the contract and is not seen (builder.md, *The page's script*).
- **Time.** No CPU, parse or paint measurement: bytes and requests only.
- **A real network.** A local server; each file compressed on its own; no
  header, connection or CDN cost.
- **`--base`**: not measured here; the site's own suite builds under one.

## The ruling that was reversed

The owner first ruled on K (2026-10-06): one sheet for all of the design
system's CSS, chosen for the site by the classes its pages resolved — parts
and variants as classes, through `variants()` — with a page's own CSS
apart, in a layer of its own, a file per page. It was built (branch
`rgp2-070-ds-css`: kept as a reference commit, not merged) and measured
with these scripts:

| | The build of that ruling | This build |
| --- | --- | --- |
| a visit | the control still transferred less: 4.4% on the docs site, 25.0% on the catalog | 12.7%, 44.7% |
| a page cold, on the catalog | its CSS within 6–9% of the control's: T5 from pass to **fail** | 41.8% to 72.9% smaller: pass |
| requests | 4 a page: T8 **fails** | 2 |
| a dialog deleted from one page | its rules stayed in what that page fetched — the site's sheet has them while any page does: T4's question, **failed** | they leave |
| a slot's `className` | cost a part its rule: a part was a class, and a slot's props replace its attachment's | parts are `data-part` |

It bought two thirds of the visit on the docs site and under half on the
catalog, and paid with what the bet is about: the page loaded cold, and
per-page precision. The owner: "a step in a wrong direction" — and seven
rules in its place (decisions.md): analysis is per artifact and exact, and
is never turned into a site-wide union because packaging shares files;
factoring across pages is the packager's problem, for CSS and JS alike; the
thresholds are not weakened; phase 1's `className` semantics stand; the
design system has no `!important`. What stands of the first ruling is that
a variant resolves into a class of its own.

## The factoring study

**Deferred by the owner (2026-10-09)**: no packaging is tuned until the
design system is ready — "for now I just need a solution that works. Later,
when we have all the components in place we'll be able to do a proper
test." What follows is kept for that test; packaging is as built.

`bench/factor.mjs` → `bench/results/factor.md`. **A study, not a build**:
what the owner's next decision — the factoring policy (builder.md,
*Packaging*, OPEN) — has to go on. The analysis is the default build's, as
built: each page's pruned sheet, split into units with a stable identity in
source order (387 on the catalog, 111 on the docs site; every sheet is
written back from its units to the byte), and its script's modules. Ten
packagings of it are written out as files and sized as `measure.mjs` sizes
them; today's packaging and the control come out as the bench measured
them in Chrome, to the byte (29,871 and 26,064 B over the docs site's
session; 43,305 and 23,953 over the catalog's). In brotli, CSS and JS
factored alike:

| Packaging | Docs site: cold page, mean | requests | session | Catalog: cold page, mean | requests | session |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| (a) today: everything inlined | 7,585 | 2 | 29,871 | 4,454 | 2–3 | 43,305 |
| (b′) the rules every sheet starts with, as one file | 7,830 | 4 | 29,446 | 4,606 | 4–5 | 42,349 |
| (b) what every page needs, as one sheet and one script; the rest inlined | 7,803 | 4 | 24,992 | 4,842 | 4–5 | 33,290 |
| (c) what two pages or more need, shared | 7,918 | 4 | 24,256 | 6,588 | 4–5 | 26,348 |
| (c) … three or more | 7,803 | 4 | 24,992 | 5,432 | 4–5 | 28,917 |
| (c) … half of the pages or more | 7,918 | 4 | 24,256 | 4,950 | 4–5 | 31,298 |
| (d) a file per component and per behaviour, whole | 8,471 | 8–11 | 25,149 | 6,820 | 8–20 | 29,394 |
| (d) … each page's own part of it, a file where two pages' are the same | 8,285 | 7–8 | 28,148 | 5,191 | 5–9 | 38,688 |
| (e) one sheet — the union of what the pages need — and one script | 8,210 | 3 | 26,032 | 8,858 | 4–5 | 23,771 |
| (f) the control | 8,242 | 3 | 26,064 | 9,041 | 4–5 | 23,953 |

| | The docs site | The catalog |
| --- | --- | --- |
| T5 as written, on what a page fetches cold | fails under every packaging: it fails on the analysis itself, (a) | holds under (a), (b′), (b) and (c) from three pages up; fails under (c) from two (JS: 24.7% at the least), both (d) (JS: 8.3%, 22.3%), and (e) |
| T8, ≤ 3 requests | (a), (e) and the control; every packaging that shares a sheet *and* a script is at 4 | (a) alone: the control is at 4–5 |
| the visit — no more than the control over the session | (b), (c), (d, whole), (e): by 0.1% to 6.9% | (e) alone, by 0.8% |
| all three | **none** | **none** |
| the cascade | (b′), (d), (e) keep it by construction; (b) changes the order of 45 pairs of rules that could matter, (c) from two pages of 1 | (b): 53 pairs, (c): 34 to 92 |

- **Over a visit that reaches every page, one sheet of the union is the
  floor — and the control's sheet is that union but for the rules no page
  uses**: 4 of 111 units on the docs site, 16 of 387 on the catalog. Any
  exact packaging transfers every needed rule at least once over such a
  session, in more and smaller pieces, each compressed on its own: on the
  catalog the CSS over the session is 5,740 B for the control, 5,557 B for
  the union, 14,125 B for (b) and 23,937 B as inlined today. So awareness
  can win a long visit by its CSS only where the design system is larger
  than what the site uses — neither fixture is that — or where the visit
  is short.
- **On the docs site it is the script that decides**: the control's, under
  4096 B, is inlined in every page — 2,840 B over the session against
  1,090 B for the pages' own. That is why (b) wins the visit there by 4.1%
  though its CSS over the session is more than the control's: 3,051 B
  against 2,145.
- **Cold and the visit pull apart.** Today's packaging is the lightest cold
  page, in one request; every shared file costs the cold page a request,
  and — unless it is exact for that page — bytes: 0.9 KB of CSS a page for
  (c) from two pages on the docs site, 9.1 KB on the catalog.
- **Scripts are too small to share**: every script (b) and (c) would share
  is under 4096 B (258 to 1,557 B). With `auto`'s rule kept, those
  packagings are the study's *CSS alone* rows: one request fewer — 3 on the
  docs site, and 3 on the catalog but for the page with a picture.
- **Order.** (b) and (c) put a rule after rules it preceded. Whether that
  changes a computed style is a question about the page — does one element
  match both — which the pruner could answer and today does not.

Not in it: time, a network, a browser (the cold loads are computed from the
files); scripts that run — a shared script there is the pages' own modules
with an `export`, not a build; any policy: it recommends none.

## Not run

Firefox (it does not start in the sandbox this was run in) — neither the
checks nor the probe; Safari proper (WebKit 26.6 by Playwright stands in);
the versions of the floor (Chrome 135, Firefox 147, Safari 26.2); any touch
device; Windows; the React baseline rebuilt for this site; a hand-written
floor of this site. On the catalog, besides: T3 (no React build of it
exists); the builds under `--base` and `--inline always`; the browser pass
under `--inline never`; a suite per component — what its behaviours do is
checked once each, where a state was needed for the comparison.

## Found on the way

| | |
| --- | --- |
| packaging | per-page pruning leaves nothing to share: the control transfers 12.7% less over four pages (19.8% while its script was a file: under the 4096 B rule the script is in every page). builder.md, *Packaging*, has the OPEN; decisions.md, K |
| T5 | its JS half cannot hold on a page that mounts everything; its unit was not given. decisions.md, L — ruled: brotli, and measured again on a catalog, where it holds on every page |
| packaging, on the catalog | the control transfers 44.7% less over ten pages, and `--inline never` does not help the default build: ten sheets are ten files. decisions.md, K, at a scale where it matters more |
| the catalog's own CSS, by its first browser pass | a tooltip centred on its word left a 400 px screen, and a table with a sticky header was `overflow: visible` at every width: fixed in the fixture (`bench/catalog-site/README.md`). Neither was the builder's |
| builder.md, *The report* | "gzip: 0–5 B above `gzip -9`" was measured on small files. On the docs site Go's `compress/gzip` is from 77 B below to 3 B above zlib's, under 1%: corrected |
| research.md | "CSS 39–76% of the one-bundle file per page": 76–96% here |
| the pruner, by the review of this tree | the page was asked before its script: a class or an id the page has was "yes" though the script takes it away (`:not(.collapsed)` dropped, and matching after a click); `textContent` set over an element (`:not(:has(b))`); a document of the site in an `<iframe>` writing to a pruned page; `classList.remove` leaving its page unpruned. Fixed (plan.md, RGP2-020), with no byte of the docs site changed: its behaviours write nothing |
| the pruner's precision | state rules stayed on pages where nothing can reach the state: 301 B a page. Ruled (decisions.md, M, option 2) and built: 72 B of it went — the rule on an attribute — and 229 B stay, the rules on `:disabled` |
| routes, with the rulings | a file stops being a variant, silently, when anything of the project imports it — a test beside the page, a second variant that borrows from the first: its document is then not built, and a route may have no `index`. builder.md, *Routes*, has the OPEN |
| `bench/site.mjs`, with the rulings | T1's check for a `javascript:` URL looked for the word anywhere in the page, and the `build` reference now says it in its text: the check reads attributes. And `bench/measure.mjs` once counted three requests for `/` in the `--inline never` build, the favicon's missed; repeated, it is four, and the report the same bytes on two runs |
| `bench/verify.mjs`, with the rulings | it waits for a page's animations to end: a paused one in the fixture's new page hung it. The fixture's animation runs |
| `bytes.go` | its comment on the report's gzip still said "a few bytes more" than `gzip -9`: from 76 B fewer to 3 B more, as builder.md has it. Corrected |
| the render engine's timeout, with the owner's rule 7 | `TestMemory` (`go/internal/build/render`) had failed twice in some thirty-five runs, with the *timeout's* message after 0.4 s. Not the memory limit: the engine's own timeout is a deadline on the wall clock, and the machine slept between short wakes while those runs were made — a page that was rendering when the clock stepped was ended on its next poll. Not reproduced by repetition (1,100 runs); reproduced by stepping the wall clock under the test (35 of 61 moments fail, in 0.16–0.30 s), and fixed: the builder times a page on the monotonic clock (0 of 61; 1,000 runs). A build on a laptop that slept would have failed the same way (plan.md, RGP2-071) |
| the report, with rules 1–3 | it had one number for what a page needs and what it fetches — a blob's size. They are two columns now (`fetches`), equal while packaging shares only identical blobs |
| `bench/verify.mjs`, a check that can fail by timing | one run of three on this tree reported 701 passed, 8 known, 1 failed: in WebKit at 1200 px, with the Install dialog open, the *Install* button under the pointer still had its hover background in one build's page and not in the other's. The two sheets have the same rule; the script waits on one of the two pages, and WebKit drops `:hover` from what a modal dialog covers on a timer. The same site, to the byte, passes all 702 in the runs before and after. Not fixed: plan.md, RGP2-071 |
| Safari's Tab | stops at neither links nor buttons by default (Option-Tab does): at 400 px, with the drawer closed, Tab reaches nothing on these pages. The engine's and the system's, as for every site; `verify.mjs` presses Option-Tab in WebKit |
