# The docs site, measured

Written by `node bench/site.mjs` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). Four builds of `site/`, each loaded cold in headless Chrome by `bench/measure.mjs`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.

## `default` — `reactogenic build`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 2 | 19,276 / 6,112 / 5,124 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 19,510 / 6,288 / 5,280 | 563 / 8,969 / 0 | 563 |
| `/guide/` | 2 | 17,998 / 5,862 / 4,969 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 18,232 / 6,038 / 5,125 | 252 / 7,152 / 0 | 252 |
| `/syntax/` | 2 | 46,873 / 13,161 / 11,399 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 47,107 / 13,337 / 11,555 | 1,443 / 8,842 / 0 | 1,443 |
| `/reference/cli/` | 2 | 30,736 / 9,699 / 8,265 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 30,970 / 9,875 / 8,421 | 252 / 7,107 / 0 | 252 |
| **session (4 pages, warm cache)** | 5 | 114,883 / 34,834 / 29,757 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 115,117 / 35,010 / 29,913 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,698 / 3,455 / 2,810 | 8,969 / 2,414 / 2,100 | 563 / 327 / 250 | inline | inline |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,152 / 2,086 / 1,807 | 252 / 176 / 126 | inline | inline |
| `/syntax/` | 36,542 / 10,148 / 8,712 | 8,842 / 2,390 / 2,078 | 1,443 / 707 / 588 | inline | inline |
| `/reference/cli/` | 23,331 / 7,515 / 6,342 | 7,107 / 2,075 / 1,800 | 252 / 176 / 126 | inline | inline |

## `always` — `reactogenic build --inline always`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 2 | 19,276 / 6,112 / 5,124 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 19,510 / 6,288 / 5,280 | 563 / 8,969 / 0 | 563 |
| `/guide/` | 2 | 17,998 / 5,862 / 4,969 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 18,232 / 6,038 / 5,125 | 252 / 7,152 / 0 | 252 |
| `/syntax/` | 2 | 46,873 / 13,161 / 11,399 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 47,107 / 13,337 / 11,555 | 1,443 / 8,842 / 0 | 1,443 |
| `/reference/cli/` | 2 | 30,736 / 9,699 / 8,265 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 30,970 / 9,875 / 8,421 | 252 / 7,107 / 0 | 252 |
| **session (4 pages, warm cache)** | 5 | 114,883 / 34,834 / 29,757 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 115,117 / 35,010 / 29,913 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,698 / 3,455 / 2,810 | 8,969 / 2,414 / 2,100 | 563 / 327 / 250 | inline | inline |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,152 / 2,086 / 1,807 | 252 / 176 / 126 | inline | inline |
| `/syntax/` | 36,542 / 10,148 / 8,712 | 8,842 / 2,390 / 2,078 | 1,443 / 707 / 588 | inline | inline |
| `/reference/cli/` | 23,331 / 7,515 / 6,342 | 7,107 / 2,075 / 1,800 | 252 / 176 / 126 | inline | inline |

## `control` — `reactogenic build --no-specialize`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 3 | 11,449 / 4,271 / 3,518 | 9,352 / 2,482 / 2,155 | 0 / 0 / 0 | 234 / 176 / 156 | 21,035 / 6,929 / 5,829 | 1,667 / 0 / 0 | 1,667 |
| `/guide/` | 3 | 12,299 / 4,518 / 3,773 | 9,352 / 2,482 / 2,155 | 0 / 0 / 0 | 234 / 176 / 156 | 21,885 / 7,176 / 6,084 | 1,667 / 0 / 0 | 1,667 |
| `/syntax/` | 3 | 38,293 / 10,993 / 9,445 | 9,352 / 2,482 / 2,155 | 0 / 0 / 0 | 234 / 176 / 156 | 47,879 / 13,651 / 11,756 | 1,667 / 0 / 0 | 1,667 |
| `/reference/cli/` | 3 | 25,082 / 8,348 / 7,045 | 9,352 / 2,482 / 2,155 | 0 / 0 / 0 | 234 / 176 / 156 | 34,668 / 11,006 / 9,356 | 1,667 / 0 / 0 | 1,667 |
| **session (4 pages, warm cache)** | 6 | 87,123 / 28,130 / 23,781 | 9,352 / 2,482 / 2,155 | 0 / 0 / 0 | 234 / 176 / 156 | 96,709 / 30,788 / 26,092 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,698 / 3,455 / 2,810 | 9,352 / 2,482 / 2,155 | 1,667 / 841 / 710 | file, 4 pages | inline |
| `/guide/` | 10,548 / 3,688 / 3,061 | 9,352 / 2,482 / 2,155 | 1,667 / 841 / 710 | file, 4 pages | inline |
| `/syntax/` | 36,542 / 10,148 / 8,712 | 9,352 / 2,482 / 2,155 | 1,667 / 841 / 710 | file, 4 pages | inline |
| `/reference/cli/` | 23,331 / 7,515 / 6,342 | 9,352 / 2,482 / 2,155 | 1,667 / 841 / 710 | file, 4 pages | inline |

The control's script, split: its behaviours — every module any page mounts, every flag on — are 1,387 / 683 / 563; its own cost, the list of modules, the table from pathname to mounts and the loop that reads it, is 280 / 230 / 178 (compressed apart; the script as a whole is 1,667 / 841 / 710):

```js
var u=[i,r,l],T={"/":[[0],[1]],"/guide/":[[0]],"/reference/cli/":[[0]],"/syntax/":[[0],[2,"m2",{typeahead:!0}],[1]]};for(let[e,t,o]of T[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/|(\.html))?$/,(n,a,f)=>f||"/")]||[])t?u[e](document.getElementById(t),o):u[e]();
```

## `never` — `reactogenic build --inline never`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 4 | 9,810 / 3,520 / 2,852 | 8,969 / 2,414 / 2,100 | 563 / 327 / 250 | 234 / 176 / 156 | 19,576 / 6,437 / 5,358 | 0 / 0 / 0 | 563 |
| `/guide/` | 4 | 10,660 / 3,747 / 3,105 | 7,152 / 2,086 / 1,807 | 252 / 176 / 126 | 234 / 176 / 156 | 18,298 / 6,185 / 5,194 | 0 / 0 / 0 | 252 |
| `/syntax/` | 4 | 36,654 / 10,216 / 8,774 | 8,842 / 2,390 / 2,078 | 1,443 / 707 / 588 | 234 / 176 / 156 | 47,173 / 13,489 / 11,596 | 0 / 0 / 0 | 1,443 |
| `/reference/cli/` | 4 | 23,443 / 7,570 / 6,372 | 7,107 / 2,075 / 1,800 | 252 / 176 / 126 | 234 / 176 / 156 | 31,036 / 9,997 / 8,454 | 0 / 0 / 0 | 252 |
| **session (4 pages, warm cache)** | 12 | 80,567 / 25,053 / 21,103 | 32,070 / 8,965 / 7,785 | 2,258 / 1,210 / 964 | 234 / 176 / 156 | 115,129 / 35,404 / 30,008 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,698 / 3,455 / 2,810 | 8,969 / 2,414 / 2,100 | 563 / 327 / 250 | file, 1 page | file, 1 page |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,152 / 2,086 / 1,807 | 252 / 176 / 126 | file, 1 page | file, 2 pages |
| `/syntax/` | 36,542 / 10,148 / 8,712 | 8,842 / 2,390 / 2,078 | 1,443 / 707 / 588 | file, 1 page | file, 1 page |
| `/reference/cli/` | 23,331 / 7,515 / 6,342 | 7,107 / 2,075 / 1,800 | 252 / 176 / 126 | file, 1 page | file, 2 pages |

## Summary

Means over 4 cold page loads; brotli -q 11 unless marked raw.

| Approach | Req / page | HTML br | CSS br (ext + inline raw) | JS br (ext) | JS to parse, raw | Other br | Page total br | Session total br | Session JS br |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| default | 2 | 7,439 | 0 + 8,018 | 0 | 628 | 156 | 7,595 | 29,913 | 0 |
| always | 2 | 7,439 | 0 + 8,018 | 0 | 628 | 156 | 7,595 | 29,913 | 0 |
| control | 3 | 5,945 | 2,155 + 0 | 0 | 1,667 | 156 | 8,256 | 26,092 | 0 |
| never | 4 | 5,276 | 1,946 + 0 | 273 | 628 | 156 | 7,651 | 30,008 | 964 |

## The default build against the control

What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control.

| Page | CSS, default | CSS, control | smaller: raw / gzip / brotli |
| --- | ---: | ---: | ---: |
| `/` | 8,969 / 2,414 / 2,100 | 9,352 / 2,482 / 2,155 | 4.1% / 2.7% / 2.6% |
| `/guide/` | 7,152 / 2,086 / 1,807 | 9,352 / 2,482 / 2,155 | 23.5% / 16.0% / 16.1% |
| `/syntax/` | 8,842 / 2,390 / 2,078 | 9,352 / 2,482 / 2,155 | 5.5% / 3.7% / 3.6% |
| `/reference/cli/` | 7,107 / 2,075 / 1,800 | 9,352 / 2,482 / 2,155 | 24.0% / 16.4% / 16.5% |

| Page | JS, default | JS, control | smaller: raw / gzip / brotli | default without its entry | control without its table | smaller, raw |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 563 / 327 / 250 | 1,667 / 841 / 710 | 66.2% / 61.1% / 64.8% | 555 | 1,387 | 60.0% |
| `/guide/` | 252 / 176 / 126 | 1,667 / 841 / 710 | 84.9% / 79.1% / 82.3% | 248 | 1,387 | 82.1% |
| `/syntax/` | 1,443 / 707 / 588 | 1,667 / 841 / 710 | 13.4% / 15.9% / 17.2% | 1,387 | 1,387 | 0.0% |
| `/reference/cli/` | 252 / 176 / 126 | 1,667 / 841 / 710 | 84.9% / 79.1% / 82.3% | 248 | 1,387 | 82.1% |

How much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.

| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's 94 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 88 | 72 | 16 | 1,777 | 6 |
| `/guide/` | 73 | 72 | 1 | 45 | 21 |
| `/syntax/` | 88 | 72 | 16 | 1,838 | 6 |
| `/reference/cli/` | 72 | 72 | 0 | 0 | 22 |

72 of the control's 94 are on all four pages (≈ 6,884 B of selectors and declarations): the layout's — the side menu, the links menu, the button — and the site's own sheet. 4 are on no page: `:is(.rg-menu>:is(a,button),.rg-menu>li>a)[aria-current]`, `.rg-sidemenu>[data-part=header]`, `.rg-sidemenu>[data-part=footer]`, `.rg-sidemenu:not(:popover-open):has(dialog:modal)`.

What a visitor's browser fetches, as each build delivers it (from the tables above):

| Build | Requests per page, cold | Page, cold: mean total | Session of 4 pages, warm cache: requests | … total |
| --- | ---: | ---: | ---: | ---: |
| `default` | 2 | 28,955 / 8,885 / 7,595 | 5 | 115,117 / 35,010 / 29,913 |
| `always` | 2 | 28,955 / 8,885 / 7,595 | 5 | 115,117 / 35,010 / 29,913 |
| `control` | 3 | 31,367 / 9,691 / 8,256 | 6 | 96,709 / 30,788 / 26,092 |
| `never` | 4 | 29,021 / 9,027 / 7,651 | 12 | 115,129 / 35,404 / 30,008 |

Over the session the control is **16.0% / 12.1% / 12.8% smaller** than the default build (raw / gzip / brotli). The control's one sheet (9,352 B) is a file, fetched once, its one script (1,667 B) inlined in each of the 4 pages; the default build's four sheets — 8,969, 7,152, 8,842, 7,107 B, no two the same — are each inlined in their page.

## What each page's script is (T1)

The default build. The rows are `_rg/report.json`'s (`modules`: from esbuild's metafile).

| Page | Script, raw | Rows of the report | Sum | Mounts |
| --- | ---: | --- | ---: | --- |
| `/` | 563 | 248 `overlays.ts` + 307 `invokers.ts` + 8 `<entry>` | 563 | `overlays`, `invokers` |
| `/guide/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/syntax/` | 1,443 | 248 `overlays.ts` + 832 `menu-keys.ts` + 307 `invokers.ts` + 56 `<entry>` | 1,443 | `overlays`, `menu-keys` on `#m2` `RG_MENU_TYPEAHEAD=true` with `{"typeahead":true}`, `invokers` |
| `/reference/cli/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |

Checked on every page, by reading the script and the document:

- the rows add up to the script: yes
- no <runtime> row: yes
- every row is a mounted behaviour or the entry: yes
- the script's only statements that run are the entry's mount calls: yes
- one <script> in the document, the builder's: yes
- no handler attribute, no javascript: URL: yes
- nothing of React or of a runtime in the script: yes

The 3 distinct scripts of the site, whole:

`/` — 563 B:

```js
function n(){addEventListener("pagehide",a),globalThis.navigation?.addEventListener("navigate",a)}function a(){for(let o of document.querySelectorAll(":popover-open"))o.hidePopover();for(let o of document.querySelectorAll("dialog[open]"))o.close()}function i(){"command"in HTMLButtonElement.prototype||addEventListener("click",r)}function r(o){let t=o.target.closest("button[commandfor]"),e=t&&document.getElementById(t.getAttribute("commandfor"));if(!e)return;let l=t.getAttribute("command");l==="show-modal"?e.open||e.showModal():l==="close"&&e.close()}n();i();
```

`/guide/`, `/reference/cli/` — 252 B:

```js
function o(){addEventListener("pagehide",n),globalThis.navigation?.addEventListener("navigate",n)}function n(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}o();
```

`/syntax/` — 1,443 B:

```js
function i(){addEventListener("pagehide",d),globalThis.navigation?.addEventListener("navigate",d)}function d(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}function r(e,t){e.addEventListener("keydown",u),e.addEventListener("click",f),t?.typeahead&&e.addEventListener("keydown",E)}var s="[role=menuitem]:not(:disabled, [aria-disabled=true])";function c(e){return[...e.querySelectorAll(s)]}function m(e){e.hidePopover(),document.getElementById(e.getAttribute("aria-labelledby"))?.focus()}function u(e){let t=e.currentTarget,o=c(t),n=o.indexOf(e.target),a={ArrowDown:n+1,ArrowUp:n-1,Home:0,End:-1}[e.key];e.key==="Tab"?m(t):a!==void 0&&(e.preventDefault(),o.at(a%o.length)?.focus())}function f(e){e.target.closest(s)&&m(e.currentTarget)}function E(e){let t=e.key.toLowerCase();if(t.length!==1||t===" "||e.ctrlKey||e.metaKey||e.altKey)return;let o=c(e.currentTarget),n=o.indexOf(e.target);[...o.slice(n+1),...o.slice(0,n+1)].find(a=>a.textContent.trim().toLowerCase().startsWith(t))?.focus()}function l(){"command"in HTMLButtonElement.prototype||addEventListener("click",y)}function y(e){let t=e.target.closest("button[commandfor]"),o=t&&document.getElementById(t.getAttribute("commandfor"));if(!o)return;let n=t.getAttribute("command");n==="show-modal"?o.open||o.showModal():n==="close"&&o.close()}i();r(document.getElementById("m2"),{typeahead:!0});l();
```

## The site's source (T6)

21 files under `site/` (without `test/`, which checks the built site and is not built into it; without `node_modules/` and `dist/`): `README.md`, `code.rtsx`, `layout.rtsx`, `package.json`, `pages/guide/editor.rtsx`, `pages/guide/index.rtsx`, `pages/index.rtsx`, `pages/reference/cli/build.rtsx`, `pages/reference/cli/check.rtsx`, `pages/reference/cli/content-mapper.rtsx`, `pages/reference/cli/index.rtsx`, `pages/reference/cli/lsp.rtsx`, `pages/syntax/each.rtsx`, `pages/syntax/flow-control.rtsx`, `pages/syntax/index.rtsx`, `pages/syntax/segment-roots.rtsx`, `pages/syntax/shorthand-props.rtsx`, `pages/syntax/slots.rtsx`, `public/favicon.svg`, `site.css`, `tsconfig.json`.

- no file of JS or TS: every source file is .rtsx, .css, .json, .md or .svg: yes
- no <script>: yes
- no <style>, no <link rel=stylesheet>, no style attribute: yes
- no event handler: yes
- no `mount(`, no import of a behaviour: yes
- no HTML written as a string: yes
- one stylesheet imported, once, by the layout: no list per page: yes
- the stylesheet imports of the whole site: `layout.rtsx:8`

## Thresholds

plan.md, RGP2-050. T4 and T7 are measured by their own scripts.

| | Threshold | Measured | Verdict |
| --- | --- | --- | --- |
| T1 (refutes) | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, and there is no `<runtime>` row | 4 of 4 pages: the rows add up to the script (563, 252, 1,443, 252 B), no `<runtime>` row, and the only statements that run are the mount calls | **pass** |
| T2 (refutes) | JS ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page; refutes above 5 KB brotli | `/syntax/`: 1,443 / 707 / 588 B | **pass** |
| T3 | ≥ 100× below the best React build of an equivalent site (Astro + React islands + Radix: 317 KB raw) | JS to parse, raw, page against page: 564×, 1257×, 223×, 1266×; this site's heaviest page against that build's lightest: 219× raw, 152× brotli (external JS). Another site of the same shape, built during the research | **pass** |
| T4 (refutes) | deleting the Install dialog from `/` removes its markup, its CSS rules and `invokers` from that page, and nothing else | `node bench/delta.mjs` | not measured here |
| T5 | against the control, in brotli bytes — what a page transfers; raw and gzip beside: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one | CSS: 2.6%, 16.1%, 3.6%, 16.5% brotli — 0 pages at 20% or more (2.7%, 16.0%, 3.7%, 16.4% gzip — 0 pages at 20% or more; 4.1%, 23.5%, 5.5%, 24.0% raw — 2 pages at 20% or more). JS: 64.8%, 82.3%, 17.2%, 82.3% brotli — under 30% on `/syntax/` (61.1%, 79.1%, 15.9%, 79.1% gzip — under 30% on `/syntax/`; 66.2%, 84.9%, 13.4%, 84.9% raw — under 30% on `/syntax/`) | **fail** |
| T6 (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 21 source files, 7 greps: nothing found; one stylesheet import, in `layout.rtsx` | **pass** |
| T7 (refutes) | the browser checks pass on the built site | `node bench/verify.mjs` | not measured here |
| T8 | ≤ 3 requests per page, cold | 2 per page (the document and the favicon); `--inline never`: 4 | **pass** |

Cross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one `_rg/report.json` has, in the four builds; `--inline always` and `--inline never` change no byte of what a page is; the control's HTML is the default build's. The report's gzip (Go's `compress/gzip`) is between -77 and +3 B of `gzip -9` (zlib) on these 64 pieces.
