# The bet, measured

RGP2-050 ([plan.md](plan.md)). The numbers are of 2026-10-05, on the tree
that has the builder, the components and the docs site together — measured
on `rgp2-integrate` (afc30d7), and **again on `rgp2-final`**, after the
review of that tree changed the pruner (plan.md, RGP2-020: the page's script
is asked before the page; a frame; `classList.remove`). `site.mjs` and
`delta.mjs` wrote the same bytes both times: no byte of the docs site moved,
and no verdict below did. `verify.mjs` has ten checks more — the pruner's
fixture, for what the site cannot show.

**Verdict: undecided.** Every threshold that could refute the bet holds.
One that could not, T5 — the one that isolates what component awareness
adds over the same build without it — fails: on the page that uses
everything the site has, awareness saves nothing; over a visit of several
pages, the build without it transfers less. The site ships no React and
almost no JS; that much is shown. That *awareness* is what makes the build
"ridiculously" small is not: on a site of three components in a shared
layout it is worth 0.2–0.9 KB a page, compressed.

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
```

Each script builds the binary from `go/` (`go build -trimpath
./cmd/reactogenic`) and the site (`site/`: four pages) into a temporary
directory; `bench/README.md` has the options. The full tables are in
`bench/results/`; what follows is taken from them.

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
| `/` | 9,689 / 3,453 / 2,808 | 9,041 / 2,425 / 2,099 | 563 / 327 / 250 | `overlays`, `invokers` |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,224 / 2,096 / 1,812 | 252 / 176 / 126 | `overlays` |
| `/syntax/` | 36,287 / 10,067 / 8,638 | 8,914 / 2,401 / 2,084 | 1,446 / 704 / 591 | `overlays`, `menu-keys` with typeahead, `invokers` |
| `/reference/cli/` | 21,503 / 6,966 / 5,849 | 7,179 / 2,088 / 1,803 | 252 / 176 / 126 | `overlays` |
| the control, every page | the same HTML | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | all three, and a table |

The control's script is its behaviours, 1,405 / 687 / 569, and its own cost
— the list of modules, the table from pathname to mounts, the loop that
reads it — 240 / 202 / 152. `/syntax/` without its mount calls is 1,405 B
too: the control's behaviours are that page's, to the byte count.

**Against the control**, smaller by (raw / gzip / brotli):

| Page | CSS | JS | JS, against the control's behaviours alone (raw) | CSS + JS, brotli | … of the page |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 3.3% / 2.3% / 2.6% | 65.8% / 60.3% / 63.8% | 60.5% | 2,845 → 2,349 | 8.8% |
| `/guide/` | 22.8% / 15.6% / 15.9% | 84.7% / 78.6% / 81.7% | 82.3% | 2,845 → 1,938 | 15.4% |
| `/syntax/` | 4.7% / 3.3% / 3.3% | 12.1% / 14.6% / 14.3% | 0.0% | 2,845 → 2,675 | 1.5% |
| `/reference/cli/` | 23.2% / 15.9% / 16.3% | 84.7% / 78.6% / 81.7% | 82.3% | 2,845 → 1,929 | 10.5% |

"Of the page" is HTML + CSS + JS, each compressed apart. 73 of the control
sheet's 94 selectors and at-rules are on all four pages; `/reference/cli/`
has none of its own, `/guide/` one. Three are on no page.

**As delivered.** Under `auto` every blob of this site is inlined — no two
pages have the same sheet, and the one script two pages share is 179 B
gzipped — so the default build is `--inline always`'s. The control's two
blobs are files: four pages share them.

| Build | Requests per page, cold | Page, cold, mean | Session: requests | Session: total |
| --- | ---: | ---: | ---: | ---: |
| default, and `--inline always` | 2 | 28,505 / 8,734 / 7,457 | 5 | 113,316 / 34,407 / 29,360 |
| control | 4 | 30,850 / 9,586 / 8,137 | 7 | 89,706 / 27,899 / 23,545 |
| `--inline never` | 4 | 28,571 / 8,881 / 7,507 | 12 | 113,328 / 34,818 / 29,433 |

The second request of the default build is the favicon (234 B). Cold, the
default build is 8.4% smaller than the control (brotli, mean) with half the
requests. **Over the session the control is 19.8% smaller** (20.8% raw):
cumulative brotli after each page, default 5,284 → 10,258 → 21,578 →
29,360, control 5,850 → 8,958 → 17,655 → 23,545 — the control is ahead
from the visitor's second page. Its sheet and script are fetched once; the
default build's four sheets — 9,041, 7,224, 8,914 and 7,179 B, of which
7,179 B are the same rules: `/reference/cli/`'s sheet is what every page
has — are each inlined in their page, and as files (`--inline never`) they
are four files.

**Against React.** The best React build of the research
(`bench/baselines/2026-10-04.md`, `c1-astro-radix`: Astro + React islands +
Radix) parses 317–321 KB of JS per page (90 KB brotli), in 14–18 requests.
That is **another site of the same shape** — four docs pages with a side
menu, a dialog and a menu, built during the research and not in the
repository — not this site built with React. Page against page, this
site's JS is 564×, 1,257×, 222× and 1,266× smaller raw; its heaviest page
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
| this site, default build | 628 | 7,457 | 2 |

## The thresholds

| | Threshold | Measured | |
| --- | --- | --- | --- |
| T1 runtime (refutes) | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, no `<runtime>` row | on all four pages the report's rows add up to the script — 248 `overlays` + 307 `invokers` + 8 entry = 563; 248 + 4 = 252; 248 + 850 `menu-keys` + 307 + 41 = 1,446 — every row is a mounted behaviour or the entry, none is `<runtime>`. Read: each script is function declarations and one constant, then the mount calls, which are the only statements that run; one `<script>` per document, no handler attribute. The three distinct scripts are printed whole in `bench/results/site.md` | **pass** |
| T2 JS per page (refutes above 5 KB brotli) | ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page | `/syntax/`: 1,446 raw, 591 brotli. 54 B under the budget: one more behaviour of `invokers`' size would exceed it | **pass** |
| T3 against React | ≥ 100× below the best React build of an equivalent site | 219× raw at the worst pairing, 152× brotli. Another site (above) | **pass** |
| T4 precision (refutes) | deleting the *Install* dialog from `/` removes its markup, the CSS rules only it matched and `invokers` from that page, nothing else; every other page the same bytes | HTML: one span of 1,070 B cut out — the trigger and the `<dialog>`, whole — every other byte where it was. CSS: −1,752 B, 14 selectors, each naming `.rg-dialog`; none came; none that stays names what left. JS: 563 → 252, `invokers` left, `overlays` the same bytes; the script is now `/guide/`'s. The other three documents: the same bytes, under `--inline always` and as the site ships | **pass** |
| T5 awareness | against the control: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one | CSS: 22.8% and 23.2% raw on two pages — 15.9% and 16.3% in brotli. JS: 12.1% on `/syntax/`, 0.0% against the control's behaviours alone | **fail** |
| T6 authoring (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 21 source files: `.rtsx`, one `.css`, `.json`, `.md`, `.svg`. No `<script>`, `<style>`, stylesheet link, `style` attribute, handler, `mount(`, import of a behaviour or `dangerouslySetInnerHTML` outside the code samples the pages show; one stylesheet import in the whole site, `layout.rtsx:8`. `site/test/browser.mjs` is JS: a test of the built site, not built into it | **pass** |
| T7 behaviour (refutes) | the browser checks pass on the built site | 698 passed, 8 known, 0 failed: Chromium 353; WebKit 345 and 8 known (below). 688 of them are of the site; 10 of the pruner's fixture, which the threshold does not ask for | **pass** |
| T8 requests | ≤ 3 per page, cold | 2: the document and the favicon. (`--inline never`: 4) | **pass** |

**T5, as read.** plan.md gave no unit; it is read in raw bytes, as T2 and
T3 are, and plan.md says so now. The CSS half holds raw and not compressed.
The JS half fails on `/syntax/` in every unit, and has to: that page mounts
every behaviour the site has with its one flag on, and the control is the
union over the site. No page that uses everything can be smaller than
everything.

**T4, as first written** — the cheat-sheet dialog of `/syntax/` — is not a
deletion of one component: a menu item commands it. `delta.mjs` runs it
both ways. The dialog alone: the build refuses, `error idref-not-found:
Page /syntax/: commandfor="cheat-sheet" on <button> names no element of
the page`. With its item: the menu becomes a list of links (`role="menu"`
and `menu-keys` leave with `invokers`: 1,446 → 252 B), 15 selectors leave —
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
  `::before`, `::after`, `::marker`, `::backdrop`, 2,363 on average — at
  rest, dark, with reduced motion, under the pointer (a link, a button, a
  menu item, a summary, the dialog's close button), with keyboard focus,
  and with the links menu, the drawer, each dialog and the action menu
  open. All equal. The comparison is checked against itself: one matching
  rule taken out of one page is seen.
- **What the site cannot show.** Its three behaviours write nothing to the
  page, so no state of it has a class taken away or an element gone — and
  that is where the review found the pruner wrong: `.card:not(.collapsed)`
  was dropped for a card that is collapsed as it loads, and matched after
  `classList.toggle`. So the same comparison runs on the builder's own
  fixture (`go/internal/build/testdata/served`), 6 more (3 per engine):
  `/toggle/` as loaded and after a click that toggles a class off, removes
  one, writes an id over and sets a text over an element — four rules match
  then that matched nothing — and `/frame/`, whose body gets a class from
  the document in its `<iframe>`. All equal; with the pruner as it was, the
  frame's is not.
- **The probe** of the pruner's tables (`cssprune/selector.go`): each of
  119 selectors — 40 pseudo-classes, 11 pseudo-elements, the four legacy
  one-colon forms, 16 forms of each of the four `:nth-*()` names — is one
  rule in `SEL, p {}` in both engines. No name leaves the table.
- **Another packaging**: `node bench/verify.mjs --inline never` — every
  blob a file — gives the same 698, 8 and 0.
- **Known**, 8, all WebKit: a dialog opened by a click leaves focus on
  `<body>` when it closes (2; components.md, *Known limits*); Playwright's
  WebKit has no page cache, so Back restores nothing (6).

## What it shows

1. **No React, no runtime, no hand-written JS** (T1, T6): the site's
   source is components and one stylesheet, and a page's script is the
   behaviours its components mounted, 252 B to 1,446 B.
2. **Per-page precision** (T4): a component deleted from a page takes
   exactly its markup, its selectors and its behaviour with it, and touches
   no other page.
3. **The pruned build looks and behaves as the unpruned one** (T7), in two
   engines, in every state that was tried. On this site that is a weaker
   statement than it reads: its behaviours write nothing, and the pruner
   was wrong exactly where one does (above; fixed, and tried on the
   fixture).

## What component awareness itself added

The control is the same compiler, the same components and the same HTML,
without the per-page decisions. Against it:

| | |
| --- | --- |
| JS | 199 to 1,393 B raw per page (99 to 564 B brotli). 240 B of it is the control's own table |
| CSS | 311 to 2,173 B raw per page (56 to 352 B brotli) |
| a page | 170 to 916 B brotli: 1.5% to 15.4% of what the page transfers |
| a flag | the site has one, `RG_MENU_TYPEAHEAD`: turning `typeahead` off takes 320 of `menu-keys`' 850 B from `/syntax/` and no CSS (`delta.mjs`) |
| requests | 2 instead of 4 — packaging, which the control could have too (`--no-specialize --inline always` was not measured) |
| checks | a button that commands a dialog that is not on the page stops the build (above); links are checked per page; ids and `aria-current` are generated per page, and right in the browser |
| soundness | a page's sheet is pruned without anything seen changing: that takes the page's HTML, the page's script and the components' CSS convention together (components.md, *CSS convention*) |

## Where it added little

- **The page that uses everything**: `/syntax/` gains 199 B of JS — less
  than the control's table — and 438 B of CSS.
- **A shared layout**: the side menu, the links menu and the button are on
  every page, and the site's own sheet is used nearly whole everywhere — 35
  to 37 of its 38 rules. A page's sheet is 77–97% of the bundle; the
  research expected 39–76% (research.md).
- **A visit**: per-page sheets that differ by a rule or two are different
  blobs, so nothing is cached across pages, and the control wins from the
  second page (above). Splitting a sheet into what every page keeps and the
  page's own rest would put ≈ 7.2 KB in one file and leave ≈ 1.8 KB on two
  pages and under 0.1 KB on the others — an estimate from the selectors,
  not a build. The owner's (decisions.md, K).
- **The HTML**: 54–76% of what a page transfers, and React's static
  output to the byte. Nothing is done to it.
- **Unused CSS at the site's level**: 3 of 94 selectors match nothing on
  any page.
- **State nobody can reach**: every page's sheet has 301 B (raw) of rules
  for a disabled button, a disabled menu item and a menu's current item —
  `.rg-button:is(:disabled,[aria-disabled=true])` and two more — though no
  page has such an element and no behaviour makes one. Runtime state is
  "maybe" by rule, whoever could write it (builder.md, *CSS*; decisions.md,
  M). 3–4% of a sheet.

## What the measurement does not show

- **That awareness is where the win over React comes from.** It is not:
  the control has no React either and is 193× below the React baseline
  (1,645 B against 317 KB). The research found the same of hand-written
  platform code — Astro with hand-written scripts is at 829 B of JS and
  7.7 KB a page. The 219–1,266× is what leaving React is worth; awareness
  is the last 12–85% of 1.6 KB.
- **How near the floor this site is.** No hand-written build of it exists.
  T2 is a budget, not a ratio.
- **What a generic tool gets from the same inputs.** The pruner needs the
  emitted HTML and the names the script writes — not the component model. A
  text-scanning pruner or an import-graph bundler on this site was not run.
  The barrel import (`@reactogenic/ui`) reaches every component's CSS from
  every page, so an import graph alone would ship the control's sheet; that
  is reasoned, not measured.
- **Scale.** Three components, four pages, one flag. A catalog of 20
  components where a page uses 3–5 — where per-page precision should
  matter, and where the control's sheet and table grow with the site — was
  not built.
- **That pruning is sound for any behaviour.** For the three of the design
  system, and for the fixture's one that toggles, removes and rewrites by
  name. A behaviour that computes the name it writes (`"is-" + state`) is
  outside the contract and is not seen (builder.md, *The page's script*).
- **Time.** No CPU, parse or paint measurement: bytes and requests only.
- **A real network.** A local server; each file compressed on its own; no
  header, connection or CDN cost.
- **`--base`**: not measured here; the site's own suite builds under one.

## Not run

Firefox (it does not start in the sandbox this was run in) — neither the
checks nor the probe; Safari proper (WebKit 26.6 by Playwright stands in);
the versions of the floor (Chrome 135, Firefox 147, Safari 26.2); any touch
device; Windows; a 20-component catalog; the React baseline rebuilt for
this site; a hand-written floor of this site.

## Found on the way

| | |
| --- | --- |
| packaging | per-page pruning leaves nothing to share: the control transfers 19.8% less over four pages. builder.md, *Packaging*, has the OPEN; decisions.md, K |
| T5 | its JS half cannot hold on a page that mounts everything; its unit was not given. decisions.md, L |
| builder.md, *The report* | "gzip: 0–5 B above `gzip -9`" was measured on small files. On the docs site Go's `compress/gzip` is from 76 B below to 3 B above zlib's, under 1%: corrected |
| research.md | "CSS 39–76% of the one-bundle file per page": 77–97% here |
| the pruner, by the review of this tree | the page was asked before its script: a class or an id the page has was "yes" though the script takes it away (`:not(.collapsed)` dropped, and matching after a click); `textContent` set over an element (`:not(:has(b))`); a document of the site in an `<iframe>` writing to a pruned page; `classList.remove` leaving its page unpruned. Fixed (plan.md, RGP2-020), with no byte of the docs site changed: its behaviours write nothing |
| the pruner's precision | state rules stay on pages where nothing can reach the state: 301 B a page. As specified; decisions.md, M |
| `bytes.go` | its comment on the report's gzip still said "a few bytes more" than `gzip -9`: from 76 B fewer to 3 B more, as builder.md has it. Corrected |
| Safari's Tab | stops at neither links nor buttons by default (Option-Tab does): at 400 px, with the drawer closed, Tab reaches nothing on these pages. The engine's and the system's, as for every site; `verify.mjs` presses Option-Tab in WebKit |
