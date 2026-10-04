# Baselines to measure the bet against

The researcher's prose report was not written to a file; this is its structured answer, followed by the independent verification. The measured tables are `results/final.md` of the experiment (`bench/` holds the measuring scripts).

## Summary

NOT DONE: the prose report at .../phase2-research/baselines.md was not written. The Write tool refused it ("subagents should return findings as text, not write report files") and I did not work around that. The findings are in this answer; report_path is the measured output of measure.mjs (one table per approach plus the summary table). Everything else was built and measured in .../phase2-research/exp-baselines/ (EXP below); the repository is untouched.

What was built: one 4-page docs site (home, getting started, syntax, API) from the repo's own docs, with page-by-page differences in side-menu variant (flat / groups / collapsible), dropdowns (theme everywhere, version on two pages) and dialog (install on home, search on syntax, none on the other two). It was built 13 ways: Vite SPA, Next.js static export, Astro with React islands (each with Radix and with Base UI), Astro with hand-written scripts, Starlight out of the box, and a hand-written floor in four packagings. The 12 same-design builds pass the same behaviour checks in Chrome 154 (verify.mjs); Starlight is a different design and was only measured.

Measured, means over the 4 cold page loads, brotli -q 11:
- Hand-written floor: 358 B of JS (897 B raw to parse), 1,645 B CSS, 5,524 B HTML, 7,527 B total, 3 requests, 2.4-3.2 ms of script at 4x CPU throttle.
- Astro + React islands with Radix (the best React build): 90,312 B JS (318,593 raw), 99,663 B total, 15 requests, 60-67 ms script. That is 252x the floor's JS and 13.2x its page weight.
- Vite SPA with Radix: 103,361 B JS, 105,457 B total. Next export with Radix, link prefetch off: 146,692 B JS, 158,644 B total; with Next's default prefetch each page load pulled the whole site (30 requests, 219,635 B).
- Base UI is 15-19% heavier than Radix in every build.
- Astro with hand-written scripts: 7,668 B total, 2 requests, i.e. the floor.
- Starlight: 43,977 B total, of which CSS 13,529 B.

What it means for the bet: no-React output is 250-470x lighter in JS, but that is already available to anyone who hand-writes platform code (Astro does it today). Per-page precision alone is worth 0.5 KB per page here (one-bundle control: +6.8%), and going below the floor is marginal (rule-level CSS elimination -3.7%, class mangling -1.7%). So the bet is confirmed only as a conjunction: pages authored with framework components and slots, no hand-written JS, and output within about 1.25x of the floor.

Other artefacts in EXP: measure.mjs (re-runnable on Reactogenic's output, usage in its header), verify.mjs, cpu.mjs, live.mjs, d-floor/build.mjs, results/cpu.md, results/purge.md, results/live.md, results/shots/. EXP is 1.3 GB of node_modules and can be deleted afterwards.

## Load-bearing claims

- **The hand-written floor for the reference site is 358 B of JS per page (brotli; 897 B raw to parse; 291-477 B per page), 1,645 B CSS, 5,524 B HTML, 7,527 B total, 3 requests. It uses only platform primitives (dialog, popover, details, commandfor, CSS anchor positioning) and passes all behaviour checks in Chrome 154.**
  - evidence: EXP/results/final.md row d-floor-page (node measure.mjs ... --runs 5); node verify.mjs d-page=d-floor/dist-page printed 'all checks passed'; source EXP/d-floor/build.mjs
  - confidence: high
- **The best React-authored build (Astro 7.3.5 + React 19.3 islands + radix-ui 1.6.7) ships 90,312 B of JS per page (brotli; 316,736-321,332 B raw to parse) and 99,663 B total: 252x the floor's JS (355x by bytes to parse) and 13.2x its page weight. Vite SPA is 289x / 14.0x, Next.js 16.3.8 static export 410x / 21.1x (prefetch off).**
  - evidence: EXP/results/final.md summary rows c1-astro-radix, a-spa-radix, b-next-radix; ratios computed with node over EXP/results/final.json (floor js 357.75, parse 897, total 7526.75)
  - confidence: high
- **React + ReactDOM alone is about 60 KB brotli (Astro client.js 209,158 raw / 56,149 br plus react-dom chunk 12,201 / 3,823), the Radix code for menu + dialog + collapsible is 87,366 raw / 27,930 br, and the site's own component code is 7,683 B raw: 97.6% of the JS on the page is not the site's code.**
  - evidence: per-file list from EXP/results/final.json for c1-astro-radix /syntax/ (node -e over .pages[2].files)
  - confidence: high
- **Astro with hand-written component scripts and platform primitives (no UI framework) already reaches the floor: 7,668 B per page (1.02x), 2 requests, 829 B of JS to parse, 2.2-3.0 ms script. Bytes alone therefore do not confirm the bet; a no-React pipeline exists today.**
  - evidence: EXP/results/final.md row c2-astro-vanilla; EXP/results/cpu.md; source EXP/c2-astro-vanilla/src (237 lines of .astro)
  - confidence: high
- **Per-page precision is worth little on a 3-component design system: shipping all CSS and all JS on every page costs 8,039 B vs 7,527 B per page (+6.8%; CSS 1,951 vs mean 1,645 B, JS 562 vs mean 358 B), and over a 4-page warm-cache session the single bundle is smaller (24,615 B vs 25,516-29,816 B).**
  - evidence: EXP/results/final.md rows d-floor-bundle vs d-floor-page / d-floor-shared
  - confidence: high
- **There is almost nothing below the hand-written floor: rule-level CSS elimination against each page's complete static HTML saves 3.7% of CSS brotli (1,776 to 1,711 B mean), and mangling all 35 class names and 21 custom properties saves 1.7% of page total (7,478 to 7,352 B).**
  - evidence: node d-floor/purge.mjs (EXP/results/purge.md); node d-floor/mangle.mjs then measure.mjs (EXP/results/d2.md)
  - confidence: medium
- **Main-thread script time on a cold load at 4x CPU throttle is 2.4-3.2 ms for the floor and 56-94 ms for the React builds (20-35x); JS heap 1.1 MB vs 2.4-3.3 MB.**
  - evidence: node cpu.mjs ... (CDP Performance.getMetrics, median of 5, Apple M2 Pro, Chrome 154.0.8037.93); EXP/results/cpu.md
  - confidence: medium
- **Next.js static export duplicates the page content in an inline RSC payload (10-91 KB raw of inline JS; /syntax/ HTML is 142,142 B raw vs 55,879 B for the floor), and with next/link's default prefetch every page load fetched the rest of the site (29-31 requests, 219,635 B brotli).**
  - evidence: EXP/results/final.md tables b-next-radix (prefetch={false}) and b-next-radix-prefetch (default); the cause of whole-document prefetch was not investigated
  - confidence: medium
- **Both candidate 'mainstream' headless libraries were measured: Radix has the installed base (@radix-ui/react-dialog 91.1M downloads/week) while Base UI (19.1M/week) became shadcn/ui's default in July 2026. Radix is the lighter one in every build (Base UI adds 14-21 KB brotli of JS per page), so Radix is the fair comparator.**
  - evidence: https://api.npmjs.org/downloads/point/last-week/<pkg> for week 2026-09-27..10-03; https://ui.shadcn.com/docs/changelog/2026-07-base-ui-default read 2026-10-04; EXP/results/final.md radix vs baseui rows
  - confidence: high
- **My minimal React baselines are generous compared with production docs sites: docusaurus.io/docs references 1,884,240 B of JS (384,576 br), react.dev/learn 1,330,838 (361,105 br), nextjs.org/docs 1,837,189 (486,912 br); Starlight out of the box with the same Markdown ships 85 KB raw / 13.5 KB brotli of CSS on every page.**
  - evidence: node live.mjs <urls> on 2026-10-04 (EXP/results/live.md; statically referenced resources only, brotli recomputed locally); EXP/results/final.md row e-starlight
  - confidence: medium
- **The measurement is reproducible: two complete runs gave identical request counts and bytes for 12 of 13 builds. The exception is Starlight, whose idle-loaded search UI (about 94 KB raw) was caught on 3 of 4 pages per run.**
  - evidence: diff EXP/results/run1.txt EXP/results/run2.txt (only the e-starlight line differs)
  - confidence: high
- **The floor depends on 2026 browser features and was tested in Chrome only: CSS anchor positioning (Baseline newly available since Firefox 147, January 2026) and invoker commands (Baseline since December 2025). Behaviour in Safari and Firefox, and accessibility equivalence with Radix (no typeahead in the floor's menus, no screen-reader review), are UNVERIFIED.**
  - evidence: MDN Invoker Commands API page, baseline banner, read 2026-10-04; web search summary on anchor positioning Baseline, 2026-10-04 (secondary sources); verify.mjs runs on Chrome 154 only
  - confidence: medium

## Recommendation

Adopt the per-page-files floor (EXP/d-floor/dist-page) as the phase-2 target, and state the bet as a conjunction: pages written in .rtsx with SideMenu, Dropdown and Dialog and no hand-written JS must produce output within about 1.25x of the hand-written floor. Bytes alone are already reachable with Astro and hand-written scripts, so the new thing is who writes the platform code.

Proposed definition of "ridiculous", measured with measure.mjs, cpu.mjs and verify.mjs on Reactogenic's build of this same site:
- T1 Runtime: 0 bytes of React or generic runtime; every JS byte belongs to a component instance on that page.
- T2 JS to parse per page: at most 2x the floor (at most 1.8 / 1.5 / 2.4 / 1.5 KB raw for home / getting started / syntax / API). Above 2x there is a runtime hiding.
- T3 JS vs React: at least 100x below Astro + React islands with Radix (317-321 KB raw).
- T4 CSS per page: at most 1.15x the floor (1,645 B brotli mean) and at least 10% below the one-bundle control (the floor achieves 16%).
- T5 HTML per page: at most 1.10x the floor (no hydration markers, serialised props or wrappers; islands pay 1.46x, Next 1.82x).
- T6 Page total: at most 1.25x the floor (mean at most 9.4 KB brotli), which is at least 10x below the best React build.
- T7 Requests: at most 4 per page.
- T8 4-page warm-cache session: at most 1.25x the best floor packaging (at most 31.9 KB brotli).
- T9 Script time at 4x CPU throttle: at most 5 ms on every page (floor 2.4-3.2 ms, React 56-94 ms).
- T10 Behaviour: verify.mjs passes on every page.
- T11 Precision: zero bytes of an unused component or variant, checked by a delta test (deleting the Dialog from /syntax/ removes exactly its markup, CSS and behaviour).
- T12 Authoring: the site source has no script tags, no hand-written JS and no per-page list of styles or behaviours.
Confirmed if all hold. Refuted if any page needs more than 5 KB brotli of JS (a micro-runtime, not compilation), page total exceeds 2x the floor, or T11 or T12 fail.

Consequences for the plan:
1. Build the three layout components on platform primitives (dialog, popover, details, commandfor, anchor positioning). The whole site's behaviour is 0.7-1.2 KB of JS per page; a design that needs a client runtime cannot meet T2.
2. Treat per-page chunking as a packaging choice, not the source of the win. Start with one CSS and one JS file per page (or inline) and do not build a chunk-graph optimiser in phase 2.
3. Do not spend phase 2 on beating the floor; CSS rule elimination and name mangling are measured and marginal.
4. Before claiming "ridiculous" for precision, add a scale test: the same four pages against a catalog of about 20 components where each page uses 3-5. Starlight's 13.5 KB of CSS on every page is the realistic comparison.
5. Quote the React baseline as Astro + React islands with Radix (the smallest) and note that production React docs sites ship 4-7x more JS.
6. Someone should write the prose report file from this answer if one is needed on disk; I could not.

## Open questions

- Authoring parity is unmeasured: T12 is a rule, not a number. Can the 57-link side menu be written in shell code at all without a loop over a constant (the OPEN in specs/later/layout.md on Each / .map() over compile-time constants)?
- Which packaging is the official floor: per-page files (3 requests, 7.5 KB), fully inline (1 request, no CSS caching between pages) or shared chunks (4-8 requests, best 4-page session at 25.5 KB)? The thresholds assume per-page files.
- Does the floor work in Safari 26 and Firefox 147+? Only Chrome 154 was tested; any fallback for anchor positioning, commandfor or closedby adds to the T2 budget.
- What is the accessibility bar: native dialog / popover behaviour plus arrow keys, or parity with Radix (typeahead, collision handling)? Each extra feature is JS on the floor too and moves T2. No screen-reader review was done.
- The theme switch needs a 98-byte inline script in the head to avoid a flash, which is a CSP question (layout.md argues against inline scripts): hash it, or accept the flash?
- Should T2 be measured on the docs-site build only (no islands in phase 2), with the more general shell-component API for islands (dialog.open(template, holes)) costed separately?
- Is real docs search (an index such as Pagefind, about 94 KB raw loaded on idle in Starlight) in or out of the phase-2 site? The reference dialog only filters the page's own 40 headings.
- Should the reference site include syntax highlighting to make HTML sizes realistic? Starlight's /syntax/ HTML is 3.5x the floor's because of it; it would inflate every approach equally.
- Not built: React Router or TanStack Start prerender, Docusaurus, React Aria Components; not measured: LCP / INP / TBT under a network model. Are any of these needed for the decision?
- Why does next/link's default prefetch fetch entire HTML documents of linked routes in a static export (observed, 30 requests per page, cause not investigated)?

## Verification (independent)

Holds in substance, with amendments. The measurements reproduce exactly and the direction is right: build on platform primitives, treat chunking as packaging, do not chase bytes below the floor. Before adopting T1-T12: (1) use the parity floor (mean 1,584 B raw JS) or widen T2; (2) state T4-T6 and the refutation line on non-content bytes, packaging-neutral, and do not name per-page files as the target packaging (inline is at least as good here); (3) decide where theme, copy and search behaviour come from under T12 (76% of the floor's JS is not owned by the three layout components) and whether the shell may loop over constants; (4) add a criterion only Reactogenic can meet, since Astro-vanilla passes the current list, and make the 20-component scale test an exit criterion. Corrections to quoted numbers: Next prefetch total is about 189.5 KB brotli, not 219.6 KB; production docs sites ship 4-6x, not 4-7x. Verification section appended to /private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-baselines/results/final.md; my scripts and outputs are in /private/tmp/claude-501/-Users-msnitkina-code-reactogenic-reactogenic/60cca3ad-28bb-46e3-8248-4b0e102a5a17/scratchpad/phase2-research/exp-baselines-verify/.

- **confirmed** — Hand-written floor: 358 B JS per page (brotli; 897 B raw; 291-477 B), 1,645 B CSS, 5,524 B HTML, 7,527 B total, 3 requests; platform primitives only; passes all behaviour checks in Chrome 154.
  - Re-ran measure.mjs (--runs 3) and my own static sizing script: identical numbers. verify.mjs passes in Chrome 154. One omission: the floor also uses <dialog closedby="any">, which is not in stable Safari. I accidentally re-ran d-floor/build.mjs in the author's directory; the output is byte-identical to the measured files (checked by diff and against final.json).
- **confirmed** — Best React-authored build (Astro 7.3.5 + React 19.3 islands + radix-ui 1.6.7) ships 90,312 B JS per page and 99,663 B total: 252x the floor's JS (355x to parse), 13.2x page weight. Vite SPA 289x / 14.0x, Next 16.3.8 export 410x / 21.1x (prefetch off).
  - Re-measured c1-astro-radix, a-spa-radix and b-next-radix: identical bytes and request counts. Installed versions match. Ratios recomputed and correct. The Astro baseline is built fairly (islands only for interactive parts).
- **confirmed** — React + ReactDOM is about 60 KB brotli, Radix code is 87,366 raw / 27,930 br, the site's own component code is 7,683 B raw: 97.6% of the JS is not the site's code.
  - Per-file list for /syntax/ adds up as stated. Slightly understated: SideCollapsible.WxWdXjsH.js (2,651 B), counted as the site's code, is mostly Radix Collapsible, so the site's own share is nearer 5 KB.
- **confirmed** — Astro with hand-written component scripts and platform primitives already reaches the floor: 7,668 B per page (1.02x), 2 requests, 829 B JS to parse, 2.2-3.0 ms script. Bytes alone do not confirm the bet.
  - Re-measured: identical bytes. Script time 2.1-4.2 ms in my run. It also passes verify.mjs in Playwright WebKit 26.6. Source is 237 lines of .astro as stated.
- **confirmed** — Per-page precision is worth little on a 3-component design system: all-on-every-page costs 8,039 vs 7,527 B (+6.8%), and over a 4-page warm session the single bundle is smaller (24,615 vs 25,516-29,816 B).
  - Numbers reproduce exactly. Valid only for this site size (3 components, 4 pages); it says nothing about a 20-component catalog.
- **partly** — Almost nothing below the floor: rule-level CSS elimination saves 3.7% of CSS brotli; mangling all 35 class names and 21 custom properties saves 1.7% of page total.
  - purge.mjs re-run gives the identical table; mangling re-run on a copy gives 7,478 to 7,352 B as stated. But mangle.mjs treats 5 pieces of documentation text (--watch, --stdio, --pretty, --no, --checker-and-implementation-language) as custom properties and rewrites them, so the mangled pages have corrupted content. Real count: 14 custom properties plus 2 anchor names. The conclusion (marginal) stands.
- **confirmed** — Main-thread script time at 4x CPU throttle is 2.4-3.2 ms for the floor and 56-94 ms for React builds (20-35x); JS heap 1.1 MB vs 2.4-3.3 MB.
  - Re-ran cpu.mjs --runs 5: floor 2.4-2.8 ms, Astro+Radix 59-68, SPA 83-94, Next 56-64; heap matches. Context the claim omits: total main-thread task time differs only 1.5-2x (floor 70-136 ms vs 137-273 ms), because content parse and layout dominate.
- **partly** — Next.js static export duplicates page content in an inline RSC payload (10-91 KB raw inline JS), and with default prefetch every page load fetched the rest of the site (29-31 requests, 219,635 B brotli).
  - Duplication and request counts hold. The byte total is overstated: the three 'whole document' fetches per page are HEAD requests (I logged methods), which transfer no body, but measure.mjs sizes them as full downloads. Corrected totals: 182,571-200,627 B brotli per page, mean 189,513. Prefetch-off rows, used for all headline ratios, are unaffected.
- **confirmed** — Radix has the installed base (@radix-ui/react-dialog 91.1M downloads/week), Base UI (19.1M/week) became shadcn/ui's default in July 2026; Radix is lighter in every build (Base UI adds 14-21 KB brotli per page).
  - npm API for 2026-09-27..10-03 returns 91,089,113 and 19,147,723. The shadcn changelog page says 'Starting today, Base UI is the default component library in shadcn/ui'. Base UI adds 14.7 / 17.2 / 20.9 KB brotli in the SPA / Astro / Next builds.
- **confirmed** — The React baselines are generous versus production docs sites: docusaurus.io/docs 384,576 br of JS, react.dev/learn 361,105, nextjs.org/docs 486,912; Starlight ships 85 KB raw / 13.5 KB br of CSS per page.
  - live.mjs re-run on 2026-10-04: same numbers (docusaurus +54 B). That is 4.0-5.4x the Astro+Radix build in brotli and 4.2-5.9x raw, so the recommendation's '4-7x' should read '4-6x'. Those sites also carry search, analytics and preloads for other routes. Starlight home page CSS is 10.9 KB, the other three 14.4 KB.
- **confirmed** — The measurement is reproducible: two runs gave identical request counts and bytes for 12 of 13 builds; Starlight's idle-loaded search UI is the exception.
  - diff of run1.txt and run2.txt shows only the Starlight line. My own third run of 8 builds matched to the byte.
- **partly** — The floor depends on 2026 browser features and was tested in Chrome only: anchor positioning (Baseline since Firefox 147, January 2026) and invoker commands (Baseline since December 2025). Safari, Firefox and accessibility equivalence with Radix are UNVERIFIED.
  - Baseline dates confirmed on MDN (fetched 2026-10-04). Missing from the claim: closedby on dialog is 'Limited availability' (Safari: preview only per browser-compat-data). I ran verify.mjs and a probe in Playwright WebKit 26.6: all checks pass, same menu position; that build is ahead of stable Safari. Stable Safari and Firefox were not run (Firefox would not start in my sandbox). Gaps found by probe: no typeahead, Tab leaves the menu open, no aria-haspopup, no focus return to the trigger in WebKit, drawer is a non-modal popover. No screen-reader review.

### Missed

- Most of the floor's JS is application logic, not layout-component behaviour: theme (316 B), copy (196 B) and search (492 B) are 1,004 of 1,315 B raw (76%); only menukeys (311 B) belongs to a component, and Dialog, drawer and collapsible need no JS. Under T12 (no hand-written JS), shell rule S1 (no handlers), no islands in phase 2, and page-author raw JS deferred, the reference site cannot be written unless the catalog grows (theme switch, copy button, filter) or those features are dropped. This needs a decision before the target is fixed.
- The reference site loops over data (side links, menu items, search results from site.json). Shell rules S2/S5 forbid loops and 'loops over constants in the shell' is on the deferred list (open note at specs/later/layout.md line 131). A side menu is not practical without it.
- The floor understates JS by about 1.8x once behaviour gaps are closed. I added typeahead, Tab-closes, focus return, aria-haspopup and a backdrop-click fallback: mean JS to parse goes 897 to 1,584 B raw (358 to 571 B brotli), passing verify.mjs and the probe in Chrome and WebKit. That uses 81-93% of T2's 2x budget. Still about 200x below Astro+Radix.
- T4, T5 and T6 are packaging- and content-dependent. The author's own inline packaging fails T5 (HTML 1.35x) and passes T4 trivially; Astro-vanilla fails T5 (1.14x) for the same reason only. Content is 63% of the floor's page bytes, so 'page total at most 1.25x' lets the compiler-controlled part grow 1.67x and the 'refuted above 2x' line lets it grow 3.7x.
- T1-T12 do not separate Reactogenic from Astro: Astro-vanilla meets every byte, request, CPU and behaviour threshold once packaging is neutralised, and its pages contain no script. No threshold tests what is specific to Reactogenic (one typed-slot TSX component source compiled for the shell and rendered by React in an island).
- No compiled-framework baseline (Marko, Qwik, Svelte) and no small-runtime baseline (Preact islands); T3 ('100x below React') is a weak bar without them.
- The scale test (about 20 components, 3-5 per page) is the real test of the bet but is listed as a follow-up; with three components precision is worth 6.8%. It should be a phase-2 exit criterion.
- Per-page files are not the best packaging of the floor: no cross-page caching (session CSS is the sum of four different files); the inline variant is smaller cold (7,478 vs 7,527 B), equal over the session and needs 1 request instead of 3.
- measure.mjs counts every logged request as a full download and does not record the method; it mis-sized Next's HEAD probes. Fix before using it on Reactogenic's output.
- The report file on disk held only measurement tables; the findings, recommendation and thresholds existed only in the author's structured answer. I appended the verification section to it.
- T9 (script at most 5 ms) has thin margin against noise: 4.2 ms measured on Astro-vanilla /syntax/ where the author had 3.0.
