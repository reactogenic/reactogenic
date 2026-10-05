# The catalog site in a browser

Written by `node bench/catalog-verify.mjs` (specs/phase02/bet.md, *The catalog*). `bench/catalog-site` — a measurement fixture — as `reactogenic build` writes it, and the control (`--no-specialize`), served over HTTP; its 10 pages at 1200 and 400 px.

| Engine | Passed | Known | Failed |
| --- | ---: | ---: | ---: |
| chromium 153.0.8010.12 | 674 | 0 | 0 |
| webkit 26.6 | 674 | 0 | 0 |

Of those, 488 are comparisons of computed styles between the two builds (488 equal) — 826 elements and pseudo-elements each on average — and 488 ask whether a selector the page's sheet lacks matches an element (488 find none; 211 dropped selectors asked each time, on average).

| Check | chromium 153.0.8010.12 | webkit 26.6 |
| --- | --- | --- |
| the page loads: a title, one <h1>, a sheet | 20 of 20 | 20 of 20 |
| no sideways scroll | 20 of 20 | 20 of 20 |
| one script, the builder's; no React | 20 of 20 | 20 of 20 |
| styles, at rest | 20 of 20 | 20 of 20 |
| dropped selectors, at rest | 20 of 20 | 20 of 20 |
| styles, dark, at rest | 20 of 20 | 20 of 20 |
| dropped selectors, dark, at rest | 20 of 20 | 20 of 20 |
| styles, reduced motion, at rest | 20 of 20 | 20 of 20 |
| dropped selectors, reduced motion, at rest | 20 of 20 | 20 of 20 |
| styles, forced colours, at rest | 20 of 20 | 20 of 20 |
| dropped selectors, forced colours, at rest | 20 of 20 | 20 of 20 |
| Tab gives an element keyboard focus | 20 of 20 | 20 of 20 |
| styles, keyboard focus | 20 of 20 | 20 of 20 |
| dropped selectors, keyboard focus | 20 of 20 | 20 of 20 |
| styles, hover on the menu's trigger | 20 of 20 | 20 of 20 |
| dropped selectors, hover on the menu's trigger | 20 of 20 | 20 of 20 |
| the layout's menu opens | 20 of 20 | 20 of 20 |
| styles, the layout's menu open | 20 of 20 | 20 of 20 |
| dropped selectors, the layout's menu open | 20 of 20 | 20 of 20 |
| styles, the layout's menu open, hover on an item | 20 of 20 | 20 of 20 |
| dropped selectors, the layout's menu open, hover on an item | 20 of 20 | 20 of 20 |
| styles, hover on a link of the side menu | 10 of 10 | 10 of 10 |
| dropped selectors, hover on a link of the side menu | 10 of 10 | 10 of 10 |
| the side menu marks this page, and no other | 20 of 20 | 20 of 20 |
| styles, hover on a card that is a link | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a card that is a link | 2 of 2 | 2 of 2 |
| no page error, console error, failed request or 4xx — in either build | 20 of 20 | 20 of 20 |
| styles, hover on a tab | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a tab | 2 of 2 | 2 of 2 |
| tabs: a click selects the tab and shows its panel | 2 of 2 | 2 of 2 |
| styles, the second tab selected | 4 of 4 | 4 of 4 |
| dropped selectors, the second tab selected | 4 of 4 | 4 of 4 |
| tabs: ArrowLeft selects the tab before, with focus | 2 of 2 | 2 of 2 |
| styles, a tab selected by a key, focus on it | 2 of 2 | 2 of 2 |
| dropped selectors, a tab selected by a key, focus on it | 2 of 2 | 2 of 2 |
| tooltip: shown under the pointer | 2 of 2 | 2 of 2 |
| styles, a tooltip shown | 2 of 2 | 2 of 2 |
| dropped selectors, a tooltip shown | 2 of 2 | 2 of 2 |
| tooltip: shown with focus on what it describes | 2 of 2 | 2 of 2 |
| styles, a tooltip shown by focus | 2 of 2 | 2 of 2 |
| dropped selectors, a tooltip shown by focus | 2 of 2 | 2 of 2 |
| styles, hover on an accordion's summary | 2 of 2 | 2 of 2 |
| dropped selectors, hover on an accordion's summary | 2 of 2 | 2 of 2 |
| accordion: opening an item closes the other (one `name`) | 2 of 2 | 2 of 2 |
| styles, another accordion item open | 2 of 2 | 2 of 2 |
| dropped selectors, another accordion item open | 2 of 2 | 2 of 2 |
| tabs: a click selects npm | 2 of 2 | 2 of 2 |
| tabs: End selects the last | 2 of 2 | 2 of 2 |
| tabs: the URL's hash selects the tab it names (`RG_TABS_HASH`) | 2 of 2 | 2 of 2 |
| styles, a tab selected by the hash | 2 of 2 | 2 of 2 |
| dropped selectors, a tab selected by the hash | 2 of 2 | 2 of 2 |
| styles, hover on a copy button | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a copy button | 2 of 2 | 2 of 2 |
| copy: the button copies the sample and says so | 2 of 2 | 2 of 2 |
| styles, a sample copied | 2 of 2 | 2 of 2 |
| dropped selectors, a sample copied | 2 of 2 | 2 of 2 |
| styles, hover on a table's row | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a table's row | 2 of 2 | 2 of 2 |
| styles, focus on a table's scrolling box | 2 of 2 | 2 of 2 |
| dropped selectors, focus on a table's scrolling box | 2 of 2 | 2 of 2 |
| styles, hover on a page's number | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a page's number | 2 of 2 | 2 of 2 |
| the action menu opens, focus on its first item | 2 of 2 | 2 of 2 |
| styles, action menu open | 2 of 2 | 2 of 2 |
| dropped selectors, action menu open | 2 of 2 | 2 of 2 |
| menu-keys: ArrowDown, then typeahead `v` (`RG_MENU_TYPEAHEAD`) | 2 of 2 | 2 of 2 |
| styles, action menu open, keyboard focus on an item | 2 of 2 | 2 of 2 |
| dropped selectors, action menu open, keyboard focus on an item | 2 of 2 | 2 of 2 |
| its item opens the dialog, modal, and closes the menu | 2 of 2 | 2 of 2 |
| styles, dialog open | 4 of 4 | 4 of 4 |
| dropped selectors, dialog open | 4 of 4 | 4 of 4 |
| styles, dark, dialog open | 2 of 2 | 2 of 2 |
| dropped selectors, dark, dialog open | 2 of 2 | 2 of 2 |
| field: the counter follows what is typed (`RG_FIELD_COUNT`) | 2 of 2 | 2 of 2 |
| styles, a field typed in, focus in it | 2 of 2 | 2 of 2 |
| dropped selectors, a field typed in, focus in it | 2 of 2 | 2 of 2 |
| field: at the limit the count has `data-full` | 2 of 2 | 2 of 2 |
| styles, a field at its limit | 2 of 2 | 2 of 2 |
| dropped selectors, a field at its limit | 2 of 2 | 2 of 2 |
| toast: its trigger shows it | 2 of 2 | 2 of 2 |
| styles, toast shown | 2 of 2 | 2 of 2 |
| dropped selectors, toast shown | 2 of 2 | 2 of 2 |
| styles, toast shown, hover on its close button | 2 of 2 | 2 of 2 |
| dropped selectors, toast shown, hover on its close button | 2 of 2 | 2 of 2 |
| toast: it hides itself after its timeout | 1 of 1 | 1 of 1 |
| styles, the notifications tab | 2 of 2 | 2 of 2 |
| dropped selectors, the notifications tab | 2 of 2 | 2 of 2 |
| styles, a switch thrown, focus on it | 2 of 2 | 2 of 2 |
| dropped selectors, a switch thrown, focus on it | 2 of 2 | 2 of 2 |
| styles, a checkbox checked | 2 of 2 | 2 of 2 |
| dropped selectors, a checkbox checked | 2 of 2 | 2 of 2 |
| styles, the security tab | 2 of 2 | 2 of 2 |
| dropped selectors, the security tab | 2 of 2 | 2 of 2 |
| field: the button shows the password and says so (`RG_FIELD_REVEAL`) | 2 of 2 | 2 of 2 |
| styles, a password shown | 2 of 2 | 2 of 2 |
| dropped selectors, a password shown | 2 of 2 | 2 of 2 |
| the danger zone's button opens its dialog, modal | 2 of 2 | 2 of 2 |
| styles, focus in a field | 2 of 2 | 2 of 2 |
| dropped selectors, focus in a field | 2 of 2 | 2 of 2 |
| styles, hover on a select | 2 of 2 | 2 of 2 |
| dropped selectors, hover on a select | 2 of 2 | 2 of 2 |
| styles, a checkbox checked, focus on it | 2 of 2 | 2 of 2 |
| dropped selectors, a checkbox checked, focus on it | 2 of 2 | 2 of 2 |
| the drawer opens | 10 of 10 | 10 of 10 |
| styles, drawer open | 10 of 10 | 10 of 10 |
| dropped selectors, drawer open | 10 of 10 | 10 of 10 |
| (a build against itself is equal; with one matching rule taken out of one page, both questions see it: the comparison can fail) | 1 of 1 | 1 of 1 |

Not run: firefox; Safari proper; any touch device; the versions of the floor (Chrome 135, Firefox 147, Safari 26.2).
