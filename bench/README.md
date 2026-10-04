# bench: what a page costs the browser

`measure.mjs` loads each page of a static site cold in headless Chrome from a
local server and sizes what was fetched: HTML, CSS, JS — raw, gzip -9,
brotli -q 11 — requests, JS to parse, and a warm 4-page session. No
dependencies (Node ≥ 20); its header has the method and the options.

```sh
node bench/measure.mjs reactogenic=site/dist --pages /,/guide/,/syntax/,/reference/cli/ --md out.md
```

`baselines/2026-10-04.md`: the same script on a 4-page docs site built 13
ways during the phase 2 research (Vite SPA, Next.js static export, Astro with
React islands — each with Radix and with Base UI — Astro with hand-written
scripts, Starlight, and a hand-written floor in four packagings). The sites
themselves are not in the repository; how they were built and what the
numbers mean is in `specs/phase02/research/baselines.md`.

The bet's thresholds and how Reactogenic's own site is measured:
`specs/phase02/plan.md`, RGP2-050.
