# js-behaviours — the JavaScript of a zero-React page

Phase 2 research, 2026-10-04. Everything measured is in
`exp-js-behaviours/` (next to this file; `README.md` there lists the scripts). Sizes are
**minified / gzip -9 / brotli q11** bytes unless one number is given, which is then brotli.

Tools: esbuild **0.28.2** (latest; `npm view esbuild time` → 2026-08-08), Node 25.2.1,
Chrome 154.0.8037.93 headless, terser 5.51.2 (comparison only), Go 1.27.1 for the Go API check.

## Answer in short

| Question | Answer | Evidence |
| --- | --- | --- |
| Does the JS need a new kind of compiler? | **No.** esbuild + `Define` + one authoring rule lands 19% above hand-written code (64–115 B brotli per page) | §2 |
| Where does the win come from? | From what the builder decides **before** esbuild: which features each page needs (−54% vs shipping the library) and which HTML it emits (a further −45%) | §2 |
| Authoring model | `export function mountX(root)` per component, features switched by compile-time constants, each feature a top-level function | §1, §2 |
| HTML contract | platform first (`popover`, `commandfor`, `<details>`); JS only for what the platform lacks (arrow keys, typeahead) | §1 |
| Chunking | inline `<script type="module">` per page, specialised; no esbuild `Splitting` | §3 |
| Lazy / idle loading | worth nothing at this size | §4 |

The four pages of the test site, per page, brotli:

| | home | guide | api | changelog | sum |
| --- | --- | --- | --- | --- | --- |
| ship the whole behaviour library | 1612 | 1612 | 1612 | 1612 | 6448 |
| recommended (platform-first HTML + flags + esbuild) | 276 | 433 | 837 | 63 | 1609 |
| hand-written lower bound, same HTML | 201 | 351 | 737 | 64 | 1353 |

## 0. What was built

A stand-in for the docs site: 4 pages, 3 behavioural components, features per use site
(`pages.mjs`). This is what a component-aware builder knows.

| page | use sites |
| --- | --- |
| home | sidemenu `#nav` (drawer) · menu `#version` (3 links; wrap, arrow-open, outside click) |
| guide | sidemenu (drawer, collapsible sections, sessionStorage persistence, scroll active link into view) · menu `#version` |
| api | sidemenu (all) · menu `#version` · menu `#filter` (typeahead, Home/End, disabled items, checkbox/radio items, JS flip placement) · dialog `#search` (triggers, backdrop click, `#search` hash) |
| changelog | sidemenu (drawer) · dialog `#release` (triggers, backdrop, exit animation, scroll lock) |

- Behaviours are real: dropdown menu per the WAI-ARIA menu-button pattern (roving focus,
  arrows with wrap, Home/End, typeahead with a 500 ms buffer, Escape, Tab, outside click,
  disabled and checkable items), dialog on native `<dialog>`, side menu.
- The same logic is written in six styles (`src/flags`, `flags2`, `split`, `options`,
  `class`, `models/*`) and twice by hand (`src/ideal`, `src/ideal-platform`).
- `html.mjs` renders each page (~11–12 kB HTML, 2.2–2.5 kB brotli) for five HTML contracts.
- Every variant is tested in Chrome through the DevTools protocol (`test/testcode.mjs`):
  **544 assertions pass, 0 fail** for the model comparison, and every row of the
  specialisation table passes too (`results/exp1-tests.md`, `results/exp2-specialise.md`).
- Go API check (`go-api/main.go`): the per-page build through `api.Build` is
  **byte-identical** to the JS API output (`cmp` on home and api) and takes **2.2–2.4 ms**
  per page in-process.

## 1. Authoring models

All five wire the same stateless core (`src/models/core.ts`) and get the same per-page
`Define`s, so the table isolates the cost of the wiring. `page +br` is what the page grows
by, in brotli, with the JS inlined (JS + the model's extra markup, one stream).

| model | home: JS | home: page +br | api: JS | api: page +br | HTML it adds (api, raw / br) |
| --- | --- | --- | --- | --- | --- |
| a. `mountX(root)` per use site | 1675 / 704 / 585 | 575 | 3968 / 1533 / 1334 | 1315 | 0 / 0 |
| b. delegated listeners on `document` | 1793 / 763 / 658 | 677 | 4075 / 1599 / 1414 | 1479 | 258 / 95 |
| c. custom elements (light DOM) | 1769 / 761 / 634 | 635 | 4052 / 1593 / 1380 | 1392 | 66 / 20 |
| d. inline handler attributes | 1648 / 705 / 600 | 638 | 3768 / 1526 / 1328 | 1385 | 327 / 80 |
| e. declarative + enhancement | 593 / 351 / 276 | 251 | 2131 / 983 / 837 | 822 | −160 / 7 |

Source: `results/exp1-models.md` (all four pages there). Relative to (a): b +12–18%,
c +6–10%, d +5–11%, e −37–56%.

| | a. mount | b. delegated | c. custom elements | d. inline handlers | e. declarative |
| --- | --- | --- | --- | --- | --- |
| who finds the use sites | the builder (ids in the entry) | selectors at event time | the browser | the builder (attributes) | the builder |
| per-instance state | closure | DOM only | element instance | DOM only | browser + closure |
| works before the script runs | no | yes, if a classic script in `<head>` | no | no (`R is not defined`) | **open / close / dismiss: yes** |
| events that do not bubble (`cancel`, `close`, `toggle`) | direct | capture listener | direct | attribute per element | direct |
| CSP without `unsafe-*` | hash of the inline script | hash | hash | **no**: needs `'unsafe-hashes'` + one hash per handler, or `'unsafe-inline'` | hash, or no script at all |
| specialised by `Define` | yes | yes (entry identical on every page) | yes | yes, but handler names are fixed by the HTML | yes |
| fits `layout.md` *Delivery* (`mountDialog(root, slots)` per clone) | **is that design** | no per-instance callbacks (`onClose`) without a registry | auto-mounts on insert; wrapper elements for `<dialog>` / `<nav>` | no | orthogonal: it is the HTML contract |

CSP was run, not assumed (`exp6-csp.mjs`, Chrome 154, `<meta http-equiv>`):

| case | policy | result |
| --- | --- | --- |
| inline `<script type="module">` | `script-src 'self' 'sha256-…'` | runs |
| same, no hash | `script-src 'self'` | blocked |
| inline module importing `/chunk.js` | `'self' 'sha256-…'` | runs |
| `onclick="…"` | `'self' 'sha256-…'` | blocked |
| `onclick="…"` | `'self' 'unsafe-hashes' 'sha256-…'` | runs |
| `popovertarget` / `commandfor`, no inline script | `script-src 'self'` | runs |

`'unsafe-hashes'`: Chrome 69+, Safari 15.4+
([content-security-policy.com/unsafe-hashes](https://content-security-policy.com/unsafe-hashes/), read 2026-10-04).

### Model (e) is a different HTML, not a different wiring

| component | the platform does | JS that is left |
| --- | --- | --- |
| menu: `<button popovertarget aria-haspopup="menu">` + `<div role="menu" popover>`, first item `autofocus`, CSS anchor positioning | toggle, outside click, Escape, focus return, top layer, `aria-expanded`, placement and flipping | arrow keys, Home/End, typeahead, checkable / disabled items, Tab closes |
| dialog: `commandfor` + `command="show-modal"` / `"close"`, `closedby="any"` | open, close, Escape, focus trap, backdrop dismiss | `#hash` opens; backdrop-click fallback (63 B) |
| side menu: popover drawer + `<details>` | drawer, sections | persistence, scroll active link into view |

Platform status, checked 2026-10-04:

| feature | status | source |
| --- | --- | --- |
| `command` / `commandfor` | Baseline since 2025-12: Chrome/Edge 135, Firefox 144, Safari 26.2; **84.72%** global | [caniuse](https://caniuse.com/wf-invoker-commands) |
| `<dialog closedby>` | Chrome 134, Firefox 141, **Safari: not supported** ("blocked since July 2025 by Safari") | [web-features explorer](https://web-platform-dx.github.io/web-features-explorer/features/dialog-closedby/) |
| CSS anchor positioning | Baseline since 2026-01 (Firefox 147); Safari 26.0–26.6 partial (from a search summary of caniuse; not opened) | [caniuse](https://caniuse.com/css-anchor-positioning) |
| popover invoker ↔ popover | "implicit `aria-details` and `aria-expanded` relationship"; Esc returns focus to the invoker | [MDN, Using the Popover API](https://developer.mozilla.org/en-US/docs/Web/API/Popover_API/Using) |
| `focusgroup` (declarative arrow-key navigation) | shipped in Chrome 150 (2026-06-30) only | [Chrome 150 release notes](https://developer.chrome.com/release-notes/150) |

- 15% of users have no `commandfor`. A feature-detected fallback costs
  **268 / 196 / 140** (`src/models/command-fallback.ts`), so the dialog numbers of (e) grow
  by ~140 B on pages with a dialog if that audience matters.
- `focusgroup` would remove the arrow-key code as well. It is one engine today; not usable.

## 2. Specialisation

`results/exp2-specialise.md`; every cell also passed its behaviour tests.

| variant | home | guide | api | changelog | sum (br) |
| --- | --- | --- | --- | --- | --- |
| i. everything, options read from `data-*` at runtime | 4580 / 1832 / 1612 | same | same | same | 6448 |
| ii. only the components the page uses, runtime options | 3641 / 1507 / 1320 | 3641 / 1507 / 1320 | 4580 / 1832 / 1612 | 2104 / 949 / 797 | 5049 |
| iii-b. options **object literal** per use site | 3476 / 1468 / 1279 | 3515 / 1480 / 1293 | 4532 / 1841 / 1620 | 1936 / 904 / 764 | 4956 |
| iii-c. **class** + options literal | 4462 / 1686 / 1492 | 4501 / 1698 / 1505 | 5757 / 2089 / 1853 | 2611 / 1068 / 908 | 5758 |
| iii-a. flags + `Define`, features as nested functions | 1638 / 754 / 630 | 2123 / 950 / 800 | 3420 / 1433 / 1253 | 1114 / 545 / 449 | 3132 |
| **iii-a2. flags + `Define`, features as top-level functions** | 1443 / 642 / 527 | 1928 / 844 / 708 | 3491 / 1442 / 1252 | 1114 / 545 / 449 | **2936** |
| iii-d. one module variant per use site (plugin) | 1638 / 753 / 630 | 2123 / 950 / 801 | 4784 / 1557 / 1365 | 1114 / 545 / 449 | 3245 |
| iv-a. hand-written, same HTML | 885 / 480 / 386 | 1281 / 670 / 560 | 2584 / 1288 / 1136 | 869 / 492 / 396 | 2478 |
| (e) platform-first HTML + flags + `Define` | 593 / 351 / 276 | 959 / 531 / 433 | 2131 / 983 / 837 | 109 / 119 / 63 | **1609** |
| iv-b. hand-written, platform-first HTML | 383 / 254 / 201 | 709 / 438 / 351 | 1688 / 868 / 737 | 79 / 96 / 64 | 1353 |

Reading it:

- **Options objects and classes are not specialised at all.** A literal
  `mountMenu(el, { typeahead: false })` leaves every branch in: iii-b (4956) ≈ ii (5049).
- **Flags + `Define` halve it**: 6448 → 2936 (−54%); against "used components only" −42%.
- **The HTML contract halves it again**: 2936 → 1609 (−45%).
- **esbuild is 19% above hand-written** on the same HTML: 2936 vs 2478, 1609 vs 1353.
  On the feature-rich api page the gap is 10–14% (1252 vs 1136, 837 vs 737).
- **Per-use-site variants are worse than the union**: api 1365 vs 1252. Two specialised
  copies of the menu cost more than one copy with the union of features.

### What esbuild removes and what it leaves (`probes/`, `results/probes.md`)

| form | removed? | probe |
| --- | --- | --- |
| `if (FLAG)` with `Define` boolean | yes | P01 |
| `switch (FLAG)` / `if`-chain on a `Define` string, any case | yes | P02–P04 |
| call to a function that became empty | yes | P15 |
| top-level function only reachable through a false flag | yes | P09, P22 |
| `const FLAG = false` in the same file | yes | P10 |
| `f(el, { focus: false })` → `if (o.focus)` | **no** | P05 |
| `{ focus = false } = {}` default, no argument | **no** | P06 |
| class method nobody calls (also `#private`) | **no** | P07, P21 |
| function **nested** in `mount()`, call removed | **no** | P08 |
| closure variable only written by removed code | **no** | P16 |
| top-level function guarded by an **imported** `const` | **no** (folded too late for tree shaking) | P19 |
| same-file `const` **string** longer than 3 characters | **no** (`"css"` is inlined, `"flip"` is not) | P20 |
| entries of a lookup table `handlers[e.key]` | **no** | P13 |
| class property names | not shortened (`#private` names are) | P17, P21 |
| function with one call site | not inlined | P14 |
| `root.querySelector(...)` when the builder knows the structure | no: the DOM is opaque | P18 |

- A terser post-pass (compress, 3 passes, `unsafe`) removes the nested-function residue
  (iii-a 3177 → 2920) but **does not specialise options literals either** (4997 → 4789) nor
  classes (5768 → 5757). With top-level functions esbuild already equals terser
  (2965 vs 2967). `results/exp2b-terser.md`. So the options-object gap is not a weak
  minifier; it needs partial evaluation.
- Needed esbuild options: `MinifySyntax` (removes the dead call text) and tree shaking
  (`treeShaking=false` keeps the typeahead body: 527 → 737). ESM beats IIFE by 5–6 B.
  `results/exp2c-knobs.md`.
- Gotcha hit during the work: an entry file with no `import`/`export` inside a package
  whose `package.json` says `"type": "commonjs"` (what `npm init -y` writes today) is
  wrapped in a CommonJS shim, +129 B (P02: 165 → 36 after switching to `"module"`).
  Entries passed through `Stdin` with an `import` are not affected.

### What a component-aware compiler could still remove

The difference between iii-a2 and iv-a, by reading both outputs
(`out/exp2/iii-a2/home.js` vs `out/exp2/iv-a/home.js`):

| esbuild keeps | hand-written does | needs |
| --- | --- | --- |
| `root.querySelector("[aria-haspopup]")`, `'[role="menu"]'` | `root.firstElementChild` / `lastElementChild` | knowing the component's markup |
| `Array.from(menu.querySelectorAll(ITEM))` on every key | `menu.children`, and `% 3` | knowing the items are static and how many |
| a generic `mount(root)` + a call | straight-line code for a single instance | knowing the instance count |
| one `pointerdown` listener per component | one merged listener | cross-component view |
| union of features when two use sites differ | per-instance parameter | per-use-site knowledge |
| `setAttribute("aria-expanded", String(x))` | `el.ariaExpanded = "" + x` | nothing: golfing, could be done in the source |

Total: 458 B brotli over 4 pages on the ARIA contract, 256 B on the platform contract.
The hand-written code mixes page knowledge with plain golfing; the split between the two
was not measured.

### Authoring rules that make specialisation free

1. Flags are bare identifiers, `declare const MENU_TYPEAHEAD: boolean`, set by `Define`.
   Never an imported constant, never an options object.
2. One feature = one top-level function (ideally one module). `mount` only wires.
   Nested functions cost 103 B on home (630 vs 527).
3. Per-feature state is module-level (`let typed = ""`), not closure state.
4. No classes.
5. Flags are booleans. A string flag is safe through `Define` only.
6. A difference between two use sites on one page is a `mount` parameter or a DOM
   attribute, not a second copy of the module.

`Define` is build-wide, so the unit of specialisation is **one esbuild build per page**.
That costs 2.2 ms per page in-process (Go API) and rules out sharing chunks between pages
that differ in flags (§3).

## 3. Chunking

`results/exp3-chunking.md`.

### What esbuild `Splitting` produces

Four entries, behaviours as one module per component, site-wide `Define`s:

| file | raw | brotli | contains | imports |
| --- | --- | --- | --- | --- |
| `home.js`, `guide.js` | 145 | 97 | 2 mount calls | menu chunk, sidemenu chunk |
| `api.js` | 215 | 124 | 4 mount calls | all three chunks |
| `changelog.js` | 145 | 100 | 2 mount calls | dialog chunk, sidemenu chunk |
| `chunk-XO3M37GH.js` | 2138 | 833 | `menu.ts` | — |
| `chunk-VIGFDUZS.js` | 762 | 347 | `dialog.ts` | — |
| `chunk-YEWBTKUH.js` | 891 | 401 | `sidemenu.ts` | — |

- Pages that share one behaviour and differ in another get one chunk per **module**, plus
  an entry of ~100 B. Each page makes 3–4 requests, two levels deep.
- Chunks follow **files, not functions**. With all behaviours in one module
  (`declarative.ts`) every page gets one 816 B chunk, and changelog downloads the menu code
  it never calls (891 B instead of 127 B). esbuild's changelog records that splitting one
  file across chunks was removed because of top-level await
  ([CHANGELOG.md](https://github.com/evanw/esbuild/blob/HEAD/CHANGELOG.md), paraphrased
  from a search summary; the behaviour itself is the measurement above).
- The docs still say: "Code splitting is still a work in progress. It currently only works
  with the `esm` output format", with a "known ordering issue"
  ([esbuild API](https://esbuild.github.io/api/), read 2026-10-04).
- Splitting cannot be combined with per-page `Define`s: a shared chunk has to be the same
  bytes on every page.

### Cost of a request

HPACK-encoded real response headers from three static hosts, a Chrome-like request, one
encoder per connection (`exp3-headers.py`):

| host | response headers, 2nd response on the connection | request headers, 2nd request | round trip incl. frame headers |
| --- | --- | --- | --- |
| GitHub Pages | 218 B | 85 B | 330 B |
| Netlify | 157 B | 79 B | 263 B |
| docs.astro.build | 67 B | 84 B | 178 B |

So an external file costs **~180–330 B** before its first byte, plus one round trip until
the behaviour is live. The tables below use 250 B.

Compression floor: the 109 B changelog script gzips to **119 B** (bigger) and brotlis to
63 B. Inlined, a script shares the HTML's compression window and is 2–5% smaller than
standalone (model a: 474 vs 497, 1315 vs 1334).

### Strategies, platform-first contract

| strategy | first view, per page (JS br, requests) | 4-page session: JS br | requests | bytes + 250 B/request | single-page landing, min–max |
| --- | --- | --- | --- | --- | --- |
| 1. **inline, per-page specialised** | 267 / 405 / 815 / 65, 0 | 1552 | 0 | 1552 | 65–815 |
| 6. inline, used components, not specialised | 614 / 754 / 815 / 123, 0 | 2306 | 0 | 2306 | 123–815 |
| 2. external file per page, specialised | 285 / 433 / 837 / 70, 1 | 1625 | 4 | 2625 | 320–1087 |
| 3. one site bundle, cached | 844, 1 | 844 | 1 | 1094 | 1094 |
| 4. esbuild `Splitting` | ~900, 2 | 1194 | 5 | 2444 | 1391–1421 |
| 5. one file per component | 127–952, 1–3 | 952 | 3 | 1702 | 377–1702 |

ARIA contract, same columns: inline specialised 2887 / 0 req; site bundle 1374 + 1 req =
1624; `Splitting` 1999 + 7 req = 3749; external per page 2936 + 4 req = 3936.

Reading it:

- An **external file per page is dominated**: specialised code differs per page, so nothing
  is shared, and it pays the request every time.
- **`Splitting` is the worst of both**: unspecialised code and the most requests.
- The contest is between **inline specialised** and **one site bundle**. On average the
  bundle wins on bytes from the third page of a session (platform: the average page is
  388 B, so 2 pages ≈ 776 and 3 pages ≈ 1164 against 1094; ARIA: 1444 and 2166 against
  1624). Inline wins on a single-page landing (65–815 vs 1094) and has no request at all.
- The whole spread is under 1.5 kB per session. One page's HTML is 2.2–2.5 kB brotli.
  **At this scale chunking is a wash in bytes**; simplicity decides.

### Decision rule

1. **Page-specific JS is inlined**, as `<script type="module">`, whenever the page's script
   is **≤ 4096 B minified** (≈ 1.5 kB brotli). All pages here are 109–2131 B (platform) or
   1114–3491 B (ARIA).
   - An external copy costs 180–330 B and a round trip, and cannot be reused across pages.
   - HTML + JS stays far inside the first flight: 2.5 kB + 0.8 kB against the 14 600 B of
     a 10-segment initial window ([RFC 6928](https://www.rfc-editor.org/rfc/rfc6928.html)).
   - 4096 B is the threshold Vite uses ("smaller than this threshold will be inlined … to
     avoid extra http requests", [build.assetsInlineLimit](https://vite.dev/config/build-options))
     and Astro applies to scripts
     ([Astro, client-side scripts](https://docs.astro.build/en/guides/client-side-scripts/)).
2. **Above it**, move code that is byte-identical on two or more pages into **one** shared
   file per site, with immutable caching. Break-even in bytes: a shared piece of `S` bytes
   used on `k` pages of a session pays off when `(k − 1) · S > 250`.
3. **Never** emit per-entry-set chunks.

The builder emits the CSP hash of each inline script (`sha256`, per page) next to the HTML.

## 4. Loading

- `<script type="module">`, inline or external, is deferred: it runs after parsing, in
  order, before `DOMContentLoaded`. A classic inline script cannot be deferred. Model (d)
  needs its globals before the first click, so it would have to be a classic script.
- **When must the behaviour be there?** Before the first interaction. With the platform
  contract, open / close / dismiss work from first paint with no script (the 16 `NATIVE`
  lines in `results/exp1-tests.md`; the CSP row above runs them with no inline script).
  Only arrow-key navigation waits for the script. With the ARIA contract nothing works
  until it has run.
- **How long does it run?** (`results/exp4-loading.md`, `exp4b-attribute.md`)

  | what | median | at 6× CPU throttle |
  | --- | --- | --- |
  | wiring: home page, model a (1675 B) | 0.3 ms | 1.7 ms |
  | wiring: home page, model e (593 B) | 0.2 ms | 1.5 ms |
  | `sessionStorage.getItem` | 0.5 ms | 1.9 ms |
  | `scrollIntoView` on the active link (forces layout) | 0.4 ms | 50 ms |

  The only cost worth a thought is the forced layout of `scrollIntoView` at load, which is
  a behaviour, not a byte count.
- **Idle or interaction-triggered loading** is not worth it. Qwik's loader alone is "about
  1 kb minified" ([Qwikloader docs](https://qwik.dev/docs/advanced/qwikloader/)); the whole
  behaviour of a page here is 63–837 B brotli, and deferring it would put a network round
  trip in front of the first click to save 0.3 ms.
- `modulepreload` matters only for an external entry that imports chunks. With an inline
  entry there is no chain.

## 5. "Why is this byte here"

`exp5-why.mjs` → `results/exp5-why/<page>.md`. Two layers:

1. **esbuild's Metafile** gives bytes per input module
   (`outputs[].inputs[].bytesInOutput`) and the import chain. Its grain is the file, so
   with one feature per module it reports features directly, including the ones that
   contribute 0 bytes.
2. **The builder adds what esbuild cannot know**: bytes per feature and per use site, by
   ablation (flip one `Define` or drop one mount call, rebuild, diff). 49 builds for the
   whole site took **191 ms**.

Page `/docs/api/`, 3495 B minified, 1258 B brotli:

| module | bytes | share | imported by |
| --- | --- | --- | --- |
| `sidemenu.ts` | 875 | 25.0% | entry |
| `menu.ts` | 867 | 24.8% | entry |
| `menu/core.ts` | 443 | 12.7% | `menu.ts`, `menu/typeahead.ts` |
| `dialog.ts` | 400 | 11.4% | entry |
| `menu/check.ts` | 342 | 9.8% | `menu.ts` |
| `menu/typeahead.ts` | 239 | 6.8% | `menu.ts` |
| `menu/place.ts` | 235 | 6.7% | `menu/core.ts` |
| generated entry | 94 | 2.7% | — |
| bundler glue | 1 | 0.0% | — |

| feature | minified | brotli | needed by |
| --- | --- | --- | --- |
| `menu.typeahead` | 337 | 153 | `#filter` |
| `sidemenu.sections` | 411 | 134 | `#nav` |
| `menu.placement=flip` | 242 | 118 | `#filter` |
| `menu.checkable` | 348 | 82 | `#filter` |
| `sidemenu.drawer` | 375 | 81 | `#nav` |
| `sidemenu.persist` | 140 | 65 | `#nav` |
| `dialog.hash` | 86 | 37 | `#search` |

| use site | minified | brotli |
| --- | --- | --- |
| `<menu id="filter">` | 1152 | 410 |
| `<sidemenu id="nav">` | 887 | 262 |
| `<dialog id="search">` | 415 | 140 |
| `<menu id="version">` | 16 | 7 |

The last row is the union at work: the second menu costs one mount call. On home the report
lists `place.ts`, `typeahead.ts` and `check.ts` as "in the graph, 0 bytes".

## Limits of this research

- Browser tests ran in **Chrome 154 only**. Firefox and Safari are UNVERIFIED (the local
  Firefox is 127, too old for `commandfor`).
- Native Escape and light dismiss need a real user gesture and were not exercised
  (reported as `NATIVE`).
- Brotli is q11, i.e. precompressed files. A CDN compressing on the fly at a lower quality
  gives larger numbers; the ratios between variants should hold (UNVERIFIED).
- The per-request overhead is HPACK computed from real headers, not captured on the wire.
  HTTP/3 (QPACK) is assumed similar (UNVERIFIED).
- Whether a shared site file pays off depends on session depth, which was not measured for
  any real docs site.
- The 3-character threshold for inlining same-file string constants is observed behaviour
  of 0.28.2, not documented.
- Exit animation and scroll lock of the dialog in pure CSS, cross-browser: UNVERIFIED.
- On the api page the union turns the `#version` menu from CSS placement to JS placement.
- No React baseline here; that is the `baselines` researcher's topic.

## Recommendation

1. **No new compiler for the JS.** Use esbuild's Go API in-process: one `api.Build` per
   page, `Stdin` entry generated by the builder, `Bundle`, full minify, `Format: ESM`,
   `Define` = the page's feature flags, `Metafile: true`.
2. **Authoring model: (a)** `export function mountX(root: HTMLElement): void` per
   component, called once per use site by the generated entry. It is the function
   `layout.md` already describes for clones opened from islands, so phase 2 code carries
   over.
3. **HTML contract: (e), platform first.** `popover` + anchor positioning for the menu and
   the drawer, `commandfor` + `closedby` for the dialog, `<details>` for sections. Keep the
   63 B backdrop fallback for Safari; decide on the 140 B `commandfor` fallback.
4. **Specialisation mechanism:** the builder computes, per page and per component, the
   union of features over the use sites and passes it as `Define`s. Behaviours follow the
   six authoring rules of §2. A lint for the two that are easy to break (no options object,
   no nested feature function) is cheap.
5. **Chunking rule:** inline, per page, as one `<script type="module">` while the script is
   ≤ 4096 B minified; emit its `sha256` for CSP. No `Splitting`. Revisit with one shared
   site file when a page exceeds the limit or when islands arrive.
6. **No lazy or idle loading.**
7. **Ship the "why is this byte here" report** as a build output: Metafile per module plus
   ablation per feature and per use site. It is the proof of the bet, and costs ~4 ms per
   row.

Expected result for the docs site: **63–837 B brotli of JS per page, 0 requests**, against
1612 B for the unspecialised library.

## Open questions

1. Is 85% `commandfor` support acceptable for the docs site, or does every dialog page
   carry the 140 B fallback? Same question for `closedby` in Safari (63 B, recommended to
   keep).
2. Per-instance differences that cannot be merged (CSS vs JS placement): a `mount`
   parameter, or forbid mixing on one page?
3. Should the builder go beyond `Define` and generate behaviour code from the component's
   markup (ids instead of `querySelector`, static item lists)? It is worth at most 16% of
   the JS (64–115 B per page) and means a code generator per component. Recommended: not
   in phase 2.
4. Where does the CSP hash go: a `<meta>` in each page, or a headers file for the host?
5. `scrollIntoView` on load forces layout (50 ms on a throttled CPU). Keep it, defer it
   after first paint, or drop it?
6. Sessions on a real docs site: if most are three pages or more, one shared site file
   beats inlining by a few hundred bytes. Is that worth a second code path?
7. When islands arrive, the shell's `mountDialog` must be importable from island bundles.
   Does the inline copy stay, or does the runtime become an external module then?

## Verification (independent)

Skeptic's pass, 2026-10-04. Everything was re-run from a copy in `exp-js-behaviours-verify/`
(same esbuild 0.28.2, Node 25.2.1, Chrome 154.0.8037.93). New scripts there: `v1-variants.mjs`,
`v2-rollup.mjs`, `v3-browsers.mjs`, `v3b-anchor.mjs`, `v3c-flip.mjs`, `v5-paint.mjs`,
`v4-headers.py`, `split-probe/`, `go-api/`; outputs in `results/v*.md`.

**Reproduction:** `probes.md`, `exp1-models.md`, `exp1-tests.md`, `exp2-specialise.md`,
`exp2b-terser.md`, `exp2c-knobs.md`, `exp3-chunking.md`, `exp6-csp.md` and the four
`exp5-why/<page>.md` came out **byte-identical** to the author's files (`diff`). Timings
differ within noise. No number in the report is wrong. The corrections below are about what
the numbers are compared with and what they leave out.

### Verdict per claim

| # | claim | verdict | what I found |
| --- | --- | --- | --- |
| 1 | esbuild + flags is 19% above hand-written, 64–115 B per page; a generator wins at most that | **partly** | Sums reproduce (2936 / 2478, 1609 / 1353) and the ratio holds at gzip -6 and brotli q4 / q5 (1.19–1.21). "64–115 B per page" are the two **averages**. Per page the gap is −1 to +148 B, **10–37%** (home: +37% on both contracts). "At most" is an estimate, not a bound: see note A |
| 2 | flags cut 54% vs the whole library, 42% vs used components | **partly** | The baselines (i, ii) are written in another style: options object plus a runtime `data-*` parser. In the same style (flags2, all flags on) it is 5384 → 2936 = **−45%** and 4233 → 2936 = **−31%**. The author's own row `ii-f` (4220) is in `results/exp2-specialise.md` but not in the report's table |
| 3 | platform-first HTML: 2936 → 1609 (−45%) | **partly** | JS bytes reproduce. The two sides are not the same feature set: note B |
| 4 | options objects and classes are not specialised; terser does not fix it | **confirmed** | Reproduced. Added Rollup 4.64.0 as a third optimiser: it removes the branch in a one-call-site probe (`f(el,{focus:false})`), but on the real pages options 4997 → 4696 (−6%), classes 5768 → 5768 (`results/v2-rollup.md`) |
| 5 | what the minifier removes and leaves; 3-character strings | **confirmed** | 22 probes identical. The threshold is in the source, not only observed: `internal/js_ast/js_ast.go:1674–1676`, "Deliberately only inline small strings … `len(v.Value) <= 3`" (esbuild v0.28.2 module) |
| 6 | top-level functions save 103 B on home; esbuild then equals terser | **confirmed** | 630 vs 527; 2965 vs 2967. Rollup agrees: nested 3177 → 2957, top-level 2965 → 2961 |
| 7 | a variant per use site is larger than the union; one build per page | **confirmed**, experiment corrected | Row iii-d was built from `src/flags` (nested functions) and with `const MENU_PLACEMENT = "flip"`, which esbuild does not fold (P20). Redone on `flags2` with a real `Define` per variant: api **1333** vs 1252. Disjoint feature sets: union 766 vs variants 826. Still larger. A separate build per page is sufficient, not necessary: a per-module `Define` in an `onLoad` plugin works in one build |
| 8 | Go API byte-identical, 2.2–2.4 ms | **confirmed** | Rebuilt with `go build -trimpath`: `cmp` identical for home and api; 1.26–2.07 ms per build over two runs. The binary is 11.1 MB |
| 9 | mount is the smallest wiring: b +12–18%, c +6–10%, d +5–11% | **partly** | Numbers reproduce; over all four pages c is +6–12%. The author's two implementations of model (a) differ by 9% (`models/mount.ts` 3192 vs `flags2` 2936, same HTML, same flags), so c and d are inside implementation noise. Only b is outside it, barely. Choose (a) because it is `layout.md`'s design, not for bytes |
| 10 | CSP: handlers need `'unsafe-hashes'`; hashed inline module runs | **confirmed** (Chrome 154) | Reproduced. Not run in other engines |
| 11 | `Splitting`: one chunk per input file, files not split, no per-page `Define` | **partly** | A chunk is made per **set of entry points**, not per file: `a.ts` and `b.ts` used by the same two entries land in one chunk (`split-probe/`). The same probe confirms that a file is not split: a function used by one entry ships to both. Docs quote confirmed verbatim |
| 12 | a request costs 180–330 B, so an external per-page file is strictly worse | **partly** | HPACK figures reproduce; a fresh capture of GitHub Pages gives 228 B of response headers. "Strictly worse" is true only because the four test pages all differ: note C. Inlining gain measured 1.4–7%, not 2–5% |
| 13 | inline vs site bundle: under 1.5 kB per session, bundle wins from page 3 | **confirmed** as arithmetic | Holds for four distinct pages. With repeated page kinds the bundle wins earlier and by more (note C) |
| 14 | wiring 0.2–0.3 ms; `scrollIntoView` costs ~50 ms throttled; no lazy loading | **partly** | Timings reproduce (0.2–0.3 ms; 1.5 ms and 52 ms at 6×). The 50 ms is **not an added cost**: first contentful paint at 6× throttle is 92 ms with the script and 92 ms without (`results/v5-paint.md`). It is the first layout, done earlier. Qwikloader "about 1 kb minified" confirmed verbatim. The conclusion stands |
| 15 | byte report from Metafile plus ablation | **confirmed** | Reproduced; 49 builds in 266 ms here |
| 16 | `commandfor` 84.72%, Baseline 2025-12; `closedby` not in Safari; `focusgroup` Chrome 150 | **confirmed** | caniuse 84.72%; web-features "Newly available since 2025-12-12"; `closedby` "blocked since July 2025 by Safari"; Chrome 150 notes (2026-06-30) list `focusgroup`, and it moves focus with arrow keys in the local Chrome 154 (`toolbar` and `menu`) |

### Note A — the hand-written bound

- The hand-written files are one attempt by one author. The same author's two versions of
  model (a) differ by 256 B over four pages, which is more than half of the 458 B gap the
  claim attributes to missing page knowledge.
- The split between page knowledge and plain golfing was not measured (the report says so).
  So "a code generator is worth at most 16%" is unverified in both directions.
- This does not change the decision. In absolute terms the gap is at most 148 B per page.

### Note B — the platform contract was measured without its CSS

The test CSS in `html.mjs` is one line and has no anchor positioning, no scroll lock and no
exit animation. The −45% therefore compares an ARIA build that does placement, flipping,
scroll lock and exit animation in JS with a platform build that does not do them at all.

Real input through Playwright (`results/v3-browsers.md`, `v3b-anchor.md`, `v3c-flip.md`),
page api, platform contract:

| check | Chrome 154 | WebKit (Playwright build "26.0") |
| --- | --- | --- |
| author's synthetic tests, models a and e, 4 pages | all pass | all pass |
| menu opens under its button | **no**: 285 px below, 318 px right (centre of the viewport) | **no**: 286 px, 322 px |
| Escape closes menu, drawer, dialog; outside click closes; backdrop click closes | yes | yes |
| focus back on the button after Escape, opened by keyboard | yes | yes |
| focus back on the button after Escape, opened by mouse | yes | **no** (the click does not focus the button) |
| page scroll locked behind the modal dialog | **no** (scrolls 400 px) | **no** |
| with 151–307 B of CSS added (101–145 B brotli): placement and scroll lock | yes | yes |
| same CSS: menu flips above a button at the viewport bottom | only with `position: fixed` | only with `top: anchor(bottom)` |

- The behaviours behind the `NATIVE` lines (Escape, light dismiss, backdrop) were exercised
  on the api page only. They pass, with the one WebKit exception above. The `NATIVE` line
  "scroll lock (CSS :has)" fails until the CSS exists. Link activation was not exercised.
- Placement and scroll lock move to roughly 100–150 B brotli of shared CSS. That is still a
  clear win, but it is not in the report's totals.
- Flipping did not work in both engines with any of the four CSS variants I tried. I did not
  find out why. Treat "the platform does placement and flipping" as unproven for flipping.
- Firefox could not be tested (the Playwright build did not launch; the local one is 127).
  Safari 27.0.1 is installed but was not driven.
- The table row "CSS anchor positioning: Baseline since 2026-01" is wrong as written.
  web-features lists the feature as `baseline: false` (only Safari 27 supports all of it);
  the core properties are in Chrome 125, Firefox 147 and Safari 26. caniuse: 85.91% partial,
  0.01% full (raw data fetched 2026-10-04).
- So about 14% of users get an unanchored menu and about 15% get a dialog button that does
  nothing, unless fallbacks ship. The 140 B `commandfor` fallback is not optional for a
  public docs site. A placement fallback was not written or measured.

### Note C — the chunking rule is fitted to four pages that all differ

A real docs site has a few page kinds and many pages per kind. Pages of one kind get a
byte-identical script (same ids, same flags). Using the report's own numbers and its
250 B per request:

| session | inline | external file per distinct script | one site bundle |
| --- | --- | --- | --- |
| home, guide, api, changelog (the report's) | 1552 | 2625 | 1094 |
| home + 3 guide pages | 1482 | 1218 | 1094 |
| landing on a guide page + 2 more guides | 1215 | 683 | 1094 |
| 10 guide pages | 4050 | 683 | 1094 |

- The report's break-even formula `(k − 1) · S > 250` already says this: the 433 B guide
  script pays off as a file from the second guide page. The "inline while ≤ 4096 B" rule
  contradicts it.
- Hosting facts that the session tables assume away, checked with `curl` on 2026-10-04:
  GitHub Pages answers `content-encoding: gzip` (no brotli) and `cache-control: max-age=600`
  (no immutable caching, no custom headers, so CSP only through `<meta>`). Netlify and
  Vercel answered brotli.
- Ratios between variants do hold at on-the-fly compression levels (gzip -6, brotli q4 / q5:
  `results/v1-variants.md` §D). This closes one of the report's UNVERIFIED items.

### What the report missed

1. **A forgotten flag is silent.** A `declare const` flag with no `Define` builds with 0
   errors and 0 warnings and leaves the bare identifier in the output: a `ReferenceError`
   at run time (`v1-variants.md` §C). The builder must define every declared flag, and the
   modules cannot run unbundled (tests, the phase 1 Vite path) without the same list.
2. **The union changes behaviour, not only bytes.** It is harmless for additive features
   (typeahead). It is a bug for restrictive ones: two dialogs on a page, one of which must
   not close on a backdrop click, both get the dismiss. Such features have to be mount
   parameters, which esbuild does not specialise (claim 4). Which features are which needs
   a rule in the component contract.
3. **Paint can precede the script.** When the document arrives in two pieces, the page
   painted before the inline module ran in 5 of 5 loads (`v5-paint.md` §2). Open sections
   restored from `sessionStorage` then appear after first paint. The test site hides this
   because its side menu is a closed popover. A sidebar that is visible on desktop would
   show the jump; that state belongs in a small blocking script next to the nav, or in the
   URL.
4. **`focusgroup` works in Chrome 154** for the menu's arrow keys. Not usable alone, but it
   means the arrow-key code is a fallback with a known end date.

### Recommendation after verification

The recommendation holds: no new compiler for the JS, a component-aware builder in front of
esbuild's Go API, `mountX(root)`, flags through `Define`, the byte report. Three amendments:

1. **Expected result.** "63–837 B, 0 requests" is the JS of a build without its fallbacks
   and without the CSS that replaces the removed JS. With the `commandfor` fallback and the
   placement and scroll-lock CSS it is roughly 200–1000 B of JS per page plus 100–150 B of
   shared CSS. Quote the bet as −45% from flags alone (same authoring style) and about
   −70% with the platform contract, not −54% and −75%.
2. **Chunking rule.** Deduplicate scripts by content hash across pages first. Inline a
   script that occurs on one page; emit a script that occurs on several pages as one file
   when `(k − 1) · S > 250`. For the three or four distinct pages of phase 2 this gives the
   report's answer (inline everything).
3. **Platform contract.** Adopt it only together with the CSS researcher's placement rules
   and a decision on the two fallbacks. Flipping is unproven.
