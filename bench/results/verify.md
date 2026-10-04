# The built site in a browser

Written by `node bench/verify.mjs` (specs/phase02/plan.md, RGP2-050, T7). The site as `reactogenic build` writes it, and the control (`--no-specialize`), served over HTTP; every page at 1200 and 400 px.

| Engine | Passed | Known | Failed |
| --- | ---: | ---: | ---: |
| chromium 153.0.8010.12 | 348 | 0 | 0 |
| webkit 26.6 | 340 | 8 | 0 |

Of those, 248 are comparisons of computed styles between the two builds (248 equal): 2,363 elements and pseudo-elements each on average, custom properties included.

| Check | chromium 153.0.8010.12 | webkit 26.6 |
| --- | --- | --- |
| no sideways scroll | 8 of 8 | 8 of 8 |
| one script, the builder's; no React | 8 of 8 | 8 of 8 |
| aria-current="page" is on this page's link of the side menu, and on nothing else | 8 of 8 | 8 of 8 |
| … its group is the one group that is open, and it is styled | 8 of 8 | 8 of 8 |
| styles, at rest: the two builds are equal | 8 of 8 | 8 of 8 |
| dark: the page follows the OS | 8 of 8 | 8 of 8 |
| styles, dark, at rest: the two builds are equal | 8 of 8 | 8 of 8 |
| styles, reduced motion, at rest: the two builds are equal | 8 of 8 | 8 of 8 |
| the pointer is over the links menu's trigger | 8 of 8 | 8 of 8 |
| styles, hover on the links menu's trigger: the two builds are equal | 8 of 8 | 8 of 8 |
| the pointer is over a link of the text | 8 of 8 | 8 of 8 |
| styles, hover on a link of the text: the two builds are equal | 8 of 8 | 8 of 8 |
| the pointer is over the current link of the side menu | 4 of 4 | 4 of 4 |
| styles, hover on the current link of the side menu: the two builds are equal | 4 of 4 | 4 of 4 |
| the pointer is over a group's summary | 4 of 4 | 4 of 4 |
| styles, hover on a group's summary: the two builds are equal | 4 of 4 | 4 of 4 |
| Tab gives an element keyboard focus | 8 of 8 | 8 of 8 |
| styles, keyboard focus (:focus-visible): the two builds are equal | 8 of 8 | 8 of 8 |
| the links menu opens | 8 of 8 | 8 of 8 |
| … anchored below its trigger, end-aligned, in the viewport | 8 of 8 | 8 of 8 |
| … a list of links: no menu role, so no arrow keys are promised | 8 of 8 | 8 of 8 |
| styles, links menu open: the two builds are equal | 8 of 8 | 8 of 8 |
| the pointer is over an item of the links menu | 8 of 8 | 8 of 8 |
| styles, links menu open, hover on an item: the two builds are equal | 8 of 8 | 8 of 8 |
| Tab moves from the trigger into the open links menu | 8 of 8 | 8 of 8 |
| styles, links menu open, keyboard focus on an item: the two builds are equal | 8 of 8 | 8 of 8 |
| Esc closes the links menu | 8 of 8 | 8 of 8 |
| the side menu is a sticky column, and there is no toggle | 4 of 4 | 4 of 4 |
| … beside the content | 4 of 4 | 4 of 4 |
| the pointer is over the Install button | 2 of 2 | 2 of 2 |
| styles, hover on the Install button: the two builds are equal | 2 of 2 | 2 of 2 |
| Install opens the dialog, modal | 2 of 2 | 2 of 2 |
| … its panel in the viewport | 4 of 4 | 4 of 4 |
| … named by its title | 2 of 2 | 2 of 2 |
| styles, Install dialog open: the two builds are equal | 2 of 2 | 2 of 2 |
| styles, dark, Install dialog open: the two builds are equal | 2 of 2 | 2 of 2 |
| styles, reduced motion, Install dialog open: the two builds are equal | 2 of 2 | 2 of 2 |
| the pointer is over the dialog's close button | 2 of 2 | 2 of 2 |
| styles, Install dialog open, hover on its close button: the two builds are equal | 2 of 2 | 2 of 2 |
| Tab stays in the open dialog | 2 of 2 | 2 of 2 |
| styles, Install dialog open, keyboard focus in it: the two builds are equal | 2 of 2 | 2 of 2 |
| Esc closes it | 2 of 2 | 2 of 2 |
| … and focus is back on Install (opened by a click) | 2 of 2 | 0 of 2, 2 known |
| Enter on Install opens it | 2 of 2 | 2 of 2 |
| Esc closes it, focus back on Install | 2 of 2 | 2 of 2 |
| the close button closes it, focus back on Install | 2 of 2 | 2 of 2 |
| a click on the scrim closes it, focus back on Install | 2 of 2 | 2 of 2 |
| the Close action closes it, focus back on Install | 2 of 2 | 2 of 2 |
| the pointer is over a link of the side menu | 3 of 3 | 3 of 3 |
| styles, hover on a link of the side menu: the two builds are equal | 3 of 3 | 3 of 3 |
| #check lands below the sticky header | 1 of 1 | 1 of 1 |
| the action menu opens, anchored below its trigger, in the viewport | 2 of 2 | 2 of 2 |
| … focus on its first item | 2 of 2 | 2 of 2 |
| styles, action menu open: the two builds are equal | 2 of 2 | 2 of 2 |
| ArrowDown, ArrowDown, ArrowDown (wraps), ArrowUp (wraps), Home, End | 2 of 2 | 2 of 2 |
| styles, action menu open, keyboard focus on its last item: the two builds are equal | 2 of 2 | 2 of 2 |
| typeahead: s, g, c | 2 of 2 | 2 of 2 |
| the pointer is over an item of the action menu | 2 of 2 | 2 of 2 |
| styles, action menu open, hover on an item: the two builds are equal | 2 of 2 | 2 of 2 |
| Tab closes the action menu | 2 of 2 | 2 of 2 |
| Enter on its first item opens the cheat sheet, modal, and closes the menu | 2 of 2 | 2 of 2 |
| styles, cheat sheet open: the two builds are equal | 2 of 2 | 2 of 2 |
| styles, dark, cheat sheet open: the two builds are equal | 2 of 2 | 2 of 2 |
| Esc closes the cheat sheet, focus back on the menu's trigger | 2 of 2 | 2 of 2 |
| by pointer: the item opens the cheat sheet and closes the menu | 2 of 2 | 2 of 2 |
| … and a click on its scrim closes it | 2 of 2 | 2 of 2 |
| the side menu is closed, and there is a toggle | 4 of 4 | 4 of 4 |
| the toggle opens the drawer, in the viewport | 4 of 4 | 4 of 4 |
| styles, drawer open: the two builds are equal | 4 of 4 | 4 of 4 |
| styles, dark, drawer open: the two builds are equal | 4 of 4 | 4 of 4 |
| the pointer is over a group's summary in the drawer | 4 of 4 | 4 of 4 |
| styles, drawer open, hover on a group's summary: the two builds are equal | 4 of 4 | 4 of 4 |
| styles, drawer open, a group toggled: the two builds are equal | 4 of 4 | 4 of 4 |
| a click on the scrim closes the drawer | 4 of 4 | 4 of 4 |
| … and activates nothing behind it | 4 of 4 | 4 of 4 |
| the close button closes the drawer | 4 of 4 | 4 of 4 |
| the pointer is over a link of the drawer | 3 of 3 | 3 of 3 |
| styles, drawer open, hover on a link: the two builds are equal | 3 of 3 | 3 of 3 |
| a link of the drawer into this page closes the drawer | 3 of 3 | 3 of 3 |
| the comparison: a build against itself is equal, and with one rule taken out of one page it is not | 1 of 1 | 1 of 1 |
| without the engine's `command`, the page's script opens the Install dialog | 1 of 1 | 1 of 1 |
| … and closes it | 1 of 1 | 1 of 1 |
| … and the menu's item opens the cheat sheet | 1 of 1 | 1 of 1 |
| (without the script and without `command`, the button is dead: the check can fail) | 1 of 1 | 1 of 1 |
| Back to /guide/, left with its drawer open: restored from the page cache, and nothing is open | 1 of 1 | 0 of 1, 1 known |
| (the same page without its script comes back with the drawer open: the check can fail) | 1 of 1 | 0 of 1, 1 known |
| Back to /syntax/, left with its action menu open: restored from the page cache, and nothing is open | 1 of 1 | 0 of 1, 1 known |
| (the same page without its script comes back with the action menu open: the check can fail) | 1 of 1 | 0 of 1, 1 known |
| Back to /, left with its Install dialog open: restored from the page cache, and nothing is open | 1 of 1 | 0 of 1, 1 known |
| (the same page without its script comes back with the Install dialog open: the check can fail) | 1 of 1 | 0 of 1, 1 known |
| the probe: `SEL, p {}` is one rule for each of the 119 selectors the pruner takes for known | 1 of 1 | 1 of 1 |
| (a selector no engine knows leaves no rule: the probe can fail) | 1 of 1 | 1 of 1 |
| no page error, console error or failed request | 1 of 1 | 1 of 1 |

Known — a documented limit, said and not failed:

- webkit 26.6, 1200px /: … and focus is back on Install (opened by a click) — focus on body: WebKit does not focus a button on click (components.md, Known limits)
- webkit 26.6, 400px /: … and focus is back on Install (opened by a click) — focus on body: WebKit does not focus a button on click (components.md, Known limits)
- webkit 26.6: Back to /guide/, left with its drawer open: restored from the page cache, and nothing is open — the page was loaded again: Playwright's WebKit has no page cache, so there was nothing to restore
- webkit 26.6: (the same page without its script comes back with the drawer open: the check can fail) — no page cache
- webkit 26.6: Back to /syntax/, left with its action menu open: restored from the page cache, and nothing is open — the page was loaded again: Playwright's WebKit has no page cache, so there was nothing to restore
- webkit 26.6: (the same page without its script comes back with the action menu open: the check can fail) — no page cache
- webkit 26.6: Back to /, left with its Install dialog open: restored from the page cache, and nothing is open — the page was loaded again: Playwright's WebKit has no page cache, so there was nothing to restore
- webkit 26.6: (the same page without its script comes back with the Install dialog open: the check can fail) — no page cache

Not run: firefox; Safari proper; any touch device; the versions of the floor (Chrome 135, Firefox 147, Safari 26.2).
