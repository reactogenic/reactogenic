# The docs site, measured

Written by `node bench/site.mjs` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). Four builds of `site/`, each loaded cold in headless Chrome by `bench/measure.mjs`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.

## `default` — `reactogenic build`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 2 | 19,339 / 6,119 / 5,128 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 19,573 / 6,295 / 5,284 | 563 / 9,041 / 0 | 563 |
| `/guide/` | 2 | 18,070 / 5,873 / 4,974 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 18,304 / 6,049 / 5,130 | 252 / 7,224 / 0 | 252 |
| `/syntax/` | 2 | 46,693 / 13,084 / 11,320 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 46,927 / 13,260 / 11,476 | 1,446 / 8,914 / 0 | 1,446 |
| `/reference/cli/` | 2 | 28,980 / 9,155 / 7,782 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 29,214 / 9,331 / 7,938 | 252 / 7,179 / 0 | 252 |
| **session (4 pages, warm cache)** | 5 | 113,082 / 34,231 / 29,204 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 113,316 / 34,407 / 29,360 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,689 / 3,453 / 2,808 | 9,041 / 2,425 / 2,099 | 563 / 327 / 250 | inline | inline |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,224 / 2,096 / 1,812 | 252 / 176 / 126 | inline | inline |
| `/syntax/` | 36,287 / 10,067 / 8,638 | 8,914 / 2,401 / 2,084 | 1,446 / 704 / 591 | inline | inline |
| `/reference/cli/` | 21,503 / 6,966 / 5,849 | 7,179 / 2,088 / 1,803 | 252 / 176 / 126 | inline | inline |

## `always` — `reactogenic build --inline always`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 2 | 19,339 / 6,119 / 5,128 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 19,573 / 6,295 / 5,284 | 563 / 9,041 / 0 | 563 |
| `/guide/` | 2 | 18,070 / 5,873 / 4,974 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 18,304 / 6,049 / 5,130 | 252 / 7,224 / 0 | 252 |
| `/syntax/` | 2 | 46,693 / 13,084 / 11,320 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 46,927 / 13,260 / 11,476 | 1,446 / 8,914 / 0 | 1,446 |
| `/reference/cli/` | 2 | 28,980 / 9,155 / 7,782 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 29,214 / 9,331 / 7,938 | 252 / 7,179 / 0 | 252 |
| **session (4 pages, warm cache)** | 5 | 113,082 / 34,231 / 29,204 | 0 / 0 / 0 | 0 / 0 / 0 | 234 / 176 / 156 | 113,316 / 34,407 / 29,360 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,689 / 3,453 / 2,808 | 9,041 / 2,425 / 2,099 | 563 / 327 / 250 | inline | inline |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,224 / 2,096 / 1,812 | 252 / 176 / 126 | inline | inline |
| `/syntax/` | 36,287 / 10,067 / 8,638 | 8,914 / 2,401 / 2,084 | 1,446 / 704 / 591 | inline | inline |
| `/reference/cli/` | 21,503 / 6,966 / 5,849 | 7,179 / 2,088 / 1,803 | 252 / 176 / 126 | inline | inline |

## `control` — `reactogenic build --no-specialize`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 4 | 9,801 / 3,516 / 2,849 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | 234 / 176 / 156 | 21,032 / 6,998 / 5,850 | 0 / 0 / 0 | 1,645 |
| `/guide/` | 4 | 10,660 / 3,746 / 3,108 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | 234 / 176 / 156 | 21,891 / 7,228 / 6,109 | 0 / 0 / 0 | 1,645 |
| `/syntax/` | 4 | 36,399 / 10,132 / 8,697 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | 234 / 176 / 156 | 47,630 / 13,614 / 11,698 | 0 / 0 / 0 | 1,645 |
| `/reference/cli/` | 4 | 21,615 / 7,023 / 5,890 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | 234 / 176 / 156 | 32,846 / 10,505 / 8,891 | 0 / 0 / 0 | 1,645 |
| **session (4 pages, warm cache)** | 7 | 78,475 / 24,417 / 20,544 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | 234 / 176 / 156 | 89,706 / 27,899 / 23,545 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,689 / 3,453 / 2,808 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | file, 4 pages | file, 4 pages |
| `/guide/` | 10,548 / 3,688 / 3,061 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | file, 4 pages | file, 4 pages |
| `/syntax/` | 36,287 / 10,067 / 8,638 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | file, 4 pages | file, 4 pages |
| `/reference/cli/` | 21,503 / 6,966 / 5,849 | 9,352 / 2,482 / 2,155 | 1,645 / 824 / 690 | file, 4 pages | file, 4 pages |

The control's script, split: its behaviours — every module any page mounts, every flag on — are 1,405 / 687 / 569; its own cost, the list of modules, the table from pathname to mounts and the loop that reads it, is 240 / 202 / 152 (compressed apart; the script as a whole is 1,645 / 824 / 690):

```js
var u=[a,r,l],p={"/":[[0],[1]],"/guide/":[[0]],"/reference/cli/":[[0]],"/syntax/":[[0],[2,"m2"],[1]]};for(let[e,t]of p[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/)?$/,"/")]||[])t?u[e](document.getElementById(t)):u[e]();
```

## `never` — `reactogenic build --inline never`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 4 | 9,801 / 3,518 / 2,849 | 9,041 / 2,425 / 2,099 | 563 / 327 / 250 | 234 / 176 / 156 | 19,639 / 6,446 / 5,354 | 0 / 0 / 0 | 563 |
| `/guide/` | 4 | 10,660 / 3,747 / 3,088 | 7,224 / 2,096 / 1,812 | 252 / 176 / 126 | 234 / 176 / 156 | 18,370 / 6,195 / 5,182 | 0 / 0 / 0 | 252 |
| `/syntax/` | 4 | 36,399 / 10,137 / 8,688 | 8,914 / 2,401 / 2,084 | 1,446 / 704 / 591 | 234 / 176 / 156 | 46,993 / 13,418 / 11,519 | 0 / 0 / 0 | 1,446 |
| `/reference/cli/` | 4 | 21,615 / 7,023 / 5,887 | 7,179 / 2,088 / 1,803 | 252 / 176 / 126 | 234 / 176 / 156 | 29,280 / 9,463 / 7,972 | 0 / 0 / 0 | 252 |
| **session (4 pages, warm cache)** | 12 | 78,475 / 24,425 / 20,512 | 32,358 / 9,010 / 7,798 | 2,261 / 1,207 / 967 | 234 / 176 / 156 | 113,328 / 34,818 / 29,433 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 9,689 / 3,453 / 2,808 | 9,041 / 2,425 / 2,099 | 563 / 327 / 250 | file, 1 page | file, 1 page |
| `/guide/` | 10,548 / 3,688 / 3,061 | 7,224 / 2,096 / 1,812 | 252 / 176 / 126 | file, 1 page | file, 2 pages |
| `/syntax/` | 36,287 / 10,067 / 8,638 | 8,914 / 2,401 / 2,084 | 1,446 / 704 / 591 | file, 1 page | file, 1 page |
| `/reference/cli/` | 21,503 / 6,966 / 5,849 | 7,179 / 2,088 / 1,803 | 252 / 176 / 126 | file, 1 page | file, 2 pages |

## Summary

Means over 4 cold page loads; brotli -q 11 unless marked raw.

| Approach | Req / page | HTML br | CSS br (ext + inline raw) | JS br (ext) | JS to parse, raw | Other br | Page total br | Session total br | Session JS br |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| default | 2 | 7,301 | 0 + 8,090 | 0 | 628 | 156 | 7,457 | 29,360 | 0 |
| always | 2 | 7,301 | 0 + 8,090 | 0 | 628 | 156 | 7,457 | 29,360 | 0 |
| control | 4 | 5,136 | 2,155 + 0 | 690 | 1,645 | 156 | 8,137 | 23,545 | 690 |
| never | 4 | 5,128 | 1,950 + 0 | 273 | 628 | 156 | 7,507 | 29,433 | 967 |

## The default build against the control

What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control.

| Page | CSS, default | CSS, control | smaller: raw / gzip / brotli |
| --- | ---: | ---: | ---: |
| `/` | 9,041 / 2,425 / 2,099 | 9,352 / 2,482 / 2,155 | 3.3% / 2.3% / 2.6% |
| `/guide/` | 7,224 / 2,096 / 1,812 | 9,352 / 2,482 / 2,155 | 22.8% / 15.6% / 15.9% |
| `/syntax/` | 8,914 / 2,401 / 2,084 | 9,352 / 2,482 / 2,155 | 4.7% / 3.3% / 3.3% |
| `/reference/cli/` | 7,179 / 2,088 / 1,803 | 9,352 / 2,482 / 2,155 | 23.2% / 15.9% / 16.3% |

| Page | JS, default | JS, control | smaller: raw / gzip / brotli | default without its entry | control without its table | smaller, raw |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 563 / 327 / 250 | 1,645 / 824 / 690 | 65.8% / 60.3% / 63.8% | 555 | 1,405 | 60.5% |
| `/guide/` | 252 / 176 / 126 | 1,645 / 824 / 690 | 84.7% / 78.6% / 81.7% | 248 | 1,405 | 82.3% |
| `/syntax/` | 1,446 / 704 / 591 | 1,645 / 824 / 690 | 12.1% / 14.6% / 14.3% | 1,405 | 1,405 | 0.0% |
| `/reference/cli/` | 252 / 176 / 126 | 1,645 / 824 / 690 | 84.7% / 78.6% / 81.7% | 248 | 1,405 | 82.3% |

How much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.

| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's 94 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 89 | 73 | 16 | 1,777 | 5 |
| `/guide/` | 74 | 73 | 1 | 45 | 20 |
| `/syntax/` | 89 | 73 | 16 | 1,838 | 5 |
| `/reference/cli/` | 73 | 73 | 0 | 0 | 21 |

73 of the control's 94 are on all four pages (≈ 6,956 B of selectors and declarations): the layout's — the side menu, the links menu, the button — and the site's own sheet. 3 are on no page: `.rg-sidemenu>[data-part=header]`, `.rg-sidemenu>[data-part=footer]`, `.rg-sidemenu:not(:popover-open):has(dialog:modal)`.

What a visitor's browser fetches, as each build delivers it (from the tables above):

| Build | Requests per page, cold | Page, cold: mean total | Session of 4 pages, warm cache: requests | … total |
| --- | ---: | ---: | ---: | ---: |
| `default` | 2 | 28,505 / 8,734 / 7,457 | 5 | 113,316 / 34,407 / 29,360 |
| `always` | 2 | 28,505 / 8,734 / 7,457 | 5 | 113,316 / 34,407 / 29,360 |
| `control` | 4 | 30,850 / 9,586 / 8,137 | 7 | 89,706 / 27,899 / 23,545 |
| `never` | 4 | 28,571 / 8,881 / 7,507 | 12 | 113,328 / 34,818 / 29,433 |

Over the session the control is **20.8% / 18.9% / 19.8% smaller** than the default build (raw / gzip / brotli): its one sheet and one script are fetched once, and the default build's four sheets — 9,041, 7,224, 8,914, 7,179 B, no two the same — are each inlined in their page.

## What each page's script is (T1)

The default build. The rows are `_rg/report.json`'s (`modules`: from esbuild's metafile).

| Page | Script, raw | Rows of the report | Sum | Mounts |
| --- | ---: | --- | ---: | --- |
| `/` | 563 | 248 `overlays.ts` + 307 `invokers.ts` + 8 `<entry>` | 563 | `overlays`, `invokers` |
| `/guide/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/syntax/` | 1,446 | 248 `overlays.ts` + 850 `menu-keys.ts` + 307 `invokers.ts` + 41 `<entry>` | 1,446 | `overlays`, `menu-keys` on `#m2` `RG_MENU_TYPEAHEAD=true`, `invokers` |
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

`/syntax/` — 1,446 B:

```js
function a(){addEventListener("pagehide",d),globalThis.navigation?.addEventListener("navigate",d)}function d(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}function r(e){e.addEventListener("keydown",u),e.addEventListener("click",f),e.hasAttribute("data-typeahead")&&e.addEventListener("keydown",E)}var s="[role=menuitem]:not(:disabled, [aria-disabled=true])";function c(e){return[...e.querySelectorAll(s)]}function m(e){e.hidePopover(),document.getElementById(e.getAttribute("aria-labelledby"))?.focus()}function u(e){let o=e.currentTarget,t=c(o),n=t.indexOf(e.target),i={ArrowDown:n+1,ArrowUp:n-1,Home:0,End:-1}[e.key];e.key==="Tab"?m(o):i!==void 0&&(e.preventDefault(),t.at(i%t.length)?.focus())}function f(e){e.target.closest(s)&&m(e.currentTarget)}function E(e){let o=e.key.toLowerCase();if(o.length!==1||o===" "||e.ctrlKey||e.metaKey||e.altKey)return;let t=c(e.currentTarget),n=t.indexOf(e.target);[...t.slice(n+1),...t.slice(0,n+1)].find(i=>i.textContent.trim().toLowerCase().startsWith(o))?.focus()}function l(){"command"in HTMLButtonElement.prototype||addEventListener("click",g)}function g(e){let o=e.target.closest("button[commandfor]"),t=o&&document.getElementById(o.getAttribute("commandfor"));if(!t)return;let n=o.getAttribute("command");n==="show-modal"?t.open||t.showModal():n==="close"&&t.close()}a();r(document.getElementById("m2"));l();
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
| T1 (refutes) | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, and there is no `<runtime>` row | 4 of 4 pages: the rows add up to the script (563, 252, 1,446, 252 B), no `<runtime>` row, and the only statements that run are the mount calls | **pass** |
| T2 (refutes) | JS ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page; refutes above 5 KB brotli | `/syntax/`: 1,446 / 704 / 591 B | **pass** |
| T3 | ≥ 100× below the best React build of an equivalent site (Astro + React islands + Radix: 317 KB raw) | JS to parse, raw, page against page: 564×, 1257×, 222×, 1266×; this site's heaviest page against that build's lightest: 219× raw, 152× brotli (external JS). Another site of the same shape, built during the research | **pass** |
| T4 (refutes) | deleting the Install dialog from `/` removes its markup, its CSS rules and `invokers` from that page, and nothing else | `node bench/delta.mjs` | not measured here |
| T5 | against the control: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one | CSS: 3.3%, 22.8%, 4.7%, 23.2% raw — 2 pages at 20% or more (brotli: 2.6%, 15.9%, 3.3%, 16.3% — 0). JS: 65.8%, 84.7%, 12.1%, 84.7% raw — under 30% on `/syntax/` | **fail** |
| T6 (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 21 source files, 7 greps: nothing found; one stylesheet import, in `layout.rtsx` | **pass** |
| T7 (refutes) | the browser checks pass on the built site | `node bench/verify.mjs` | not measured here |
| T8 | ≤ 3 requests per page, cold | 2 per page (the document and the favicon); `--inline never`: 4 | **pass** |

Cross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one `_rg/report.json` has, in the four builds; `--inline always` and `--inline never` change no byte of what a page is; the control's HTML is the default build's. The report's gzip (Go's `compress/gzip`) is between -76 and +3 B of `gzip -9` (zlib) on these 64 pieces.
