# The factoring study: packagings of one analysis

`node bench/factor.mjs`. **A study, not a build**: for the owner's decision on the factoring policy (specs/phase02/builder.md, *Packaging*, OPEN; decisions.md, K). Nothing here ships, and no policy is implemented.

Analysis is the same in every row but the control's: each page's sheet is the rules that can match on that page, its script the behaviours it mounted — the default build's, as built. A row is a **packaging** of that: which of it is inlined in the page, which is a file of the page's own, which is factored into files that pages share. The bytes are of the files each packaging would write, each compressed on its own (raw / gzip -9 / brotli -q 11, as `measure.mjs`). A page loaded **cold** fetches its document, the files it links and the static files it shows (the favicon; a picture on the catalog's `/`); the **session** is the pages in the bench's order with a warm cache, each URL once.

| | |
| --- | --- |
| (a) today | every page's sheet and script inlined |
| (b′) the common head | CSS: the rules every page's sheet starts with, as one file — a prefix of each, in source order; the rest inlined. JS: as (b) |
| (b) needed by every page | the rules and the modules every page needs, as one sheet and one script; the rest inlined |
| (c) needed by ≥ 2 | the rules and the modules two pages or more need, shared; the rest inlined |
| (c) needed by ≥ 3 | … three or more |
| (c) needed by ≥ half | … half of the pages or more |
| (d) a file per component, whole | one file per component and per behaviour module — everything of it that any page needs — linked by the pages that need any of it |
| (d) a file per component, pruned | per component and per module, each page's own part: a file where two pages have the same bytes, inlined where not |
| (e) one sheet, one script | the union of what the pages need, as one sheet; the script is the control's |
| (f) the control | `--no-specialize`, as built: nothing decided per page |

The thresholds are plan.md's (RGP2-050), **as written**: **T5** — against the control, in brotli bytes, each blob on its own: per-page CSS ≥ 20% smaller on at least two pages (of a catalog: on at least half of its pages), JS ≥ 30% smaller on every page that ships one — read on what a page *fetches* cold under the packaging: "what a page transfers". **T8** — ≤ 3 requests per page, cold. **The visit** — the packaging transfers no more than the control over the session, in brotli — is **no threshold of plan.md today**: the owner named it as a target (decisions.md, K, rule 4), and it is given here so that a policy can be judged by it.

T5 read on what a page *needs* is row (a)'s in every row: no packaging changes the analysis. "Fetched beyond the page's own" is the raw bytes of CSS, or of JS, that a cold page fetches over its own sheet, or script, as analysis left it: for a packaging that hands a page no rule it does not need — (b′), (b), (d, pruned) — that is what the split itself costs (a block reopened, a selector list cut in two, an `import`); for the others, mostly rules and modules of other pages.

## The docs site

`site/`: four pages, three components in one layout. 4 artifacts; the session: `/`, `/guide/`, `/syntax/`, `/reference/cli/`.

**What the artifacts need.** The control's sheet is 111 units (9,344 / 2,472 / 2,145 B); 107 of them are needed by some page. By how many pages need a unit:

| Needed by | Units | ≈ B, raw | |
| --- | ---: | ---: | --- |
| every page (4) | 89 | 7,013 | `tokens`, `rg-button`, `rg-menu`, `rg-sidemenu`, `site` |
| 2 pages | 15 | 1,609 | `rg-dialog`, `site` |
| 1 page | 3 | 442 | `rg-dialog`, `rg-menu`, `site` |
| no page | 4 | 297 | `rg-menu`, `rg-sidemenu` |

Every page's sheet is written back from its units to the byte (4 of 4), and the control's. The components, by the top-level block of the source a rule stands in: `tokens`, `rg-button`, `rg-dialog`, `rg-menu`, `rg-sidemenu`, `site`.

The scripts' modules — a module built under other flags is another:

| Module | B in a script | Pages |
| --- | ---: | --- |
| `overlays.ts` | 248 | every page (4) |
| `invokers.ts` | 307 | `/`, `/syntax/` |
| `menu-keys.ts` | 832 | `/syntax/` |

The control's script is 1,667 / 841 / 710 B, inlined in every page (under 4096 B); its sheet is a file.

**CSS and JS together.**

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 5,119 / 7,585 / 11,544 | 28,934 / 8,867 / 7,585 | 2 | 115,035 / 34,941 / 29,871 | 5 | page 2 |
| (b′) the common head | 2 | 5,350 / 7,830 / 11,802 | 29,034 / 9,219 / 7,830 | 4 | 112,225 / 34,534 / 29,446 | 7 | page 2 |
| (b) needed by every page | 2 | 5,195 / 7,803 / 11,924 | 29,083 / 9,239 / 7,803 | 4 | 93,559 / 29,689 / 24,992 | 7 | never |
| (c) needed by ≥ 2 | 2 | 5,447 / 7,918 / 11,799 | 30,088 / 9,343 / 7,918 | 4 | 91,571 / 28,736 / 24,256 | 7 | never |
| (c) needed by ≥ 3 | 2 | 5,195 / 7,803 / 11,924 | 29,083 / 9,239 / 7,803 | 4 | 93,559 / 29,689 / 24,992 | 7 | never |
| (c) needed by ≥ half | 2 | 5,447 / 7,918 / 11,799 | 30,088 / 9,343 / 7,918 | 4 | 91,571 / 28,736 / 24,256 | 7 | never |
| (d) a file per component, whole | 9 | 5,789 / 8,471 / 12,674 | 29,474 / 10,187 / 8,471 | 8–11 | 92,421 / 30,035 / 25,149 | 14 | page 1 |
| (d) a file per component, pruned | 6 | 5,753 / 8,285 / 12,230 | 29,228 / 9,895 / 8,285 | 7–8 | 103,293 / 33,396 / 28,148 | 11 | page 1 |
| (e) one sheet, one script | 1 | 5,784 / 8,210 / 11,714 | 31,049 / 9,627 / 8,210 | 3 | 96,354 / 30,707 / 26,032 | 6 | never |
| (f) the control | 1 | 5,816 / 8,242 / 11,737 | 31,346 / 9,672 / 8,242 | 3 | 96,651 / 30,745 / 26,064 | 6 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least two pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 2.5%–16.8%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 2 | fails: the control transfers 12.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | -4.8%–8.7%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | **fail**: 4, 4 of 4 pages over | fails: the control transfers 11.5% less | 0 / 0 | 47 / 47 |
| (b) needed by every page | -12.0%–16.8%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | **fail**: 4, 4 of 4 pages over | **holds**: 4.1% less | 53 / 210 | 47 / 47 |
| (c) needed by ≥ 2 | -3.5%–3.9%; 0 of 4 pages | **fail** | 1.5%–57.9% | **fail** | **fail**: 4, 4 of 4 pages over | **holds**: 6.9% less | 905 / 1,694 | 204 / 356 |
| (c) needed by ≥ 3 | -12.0%–16.8%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | **fail**: 4, 4 of 4 pages over | **holds**: 4.1% less | 53 / 210 | 47 / 47 |
| (c) needed by ≥ half | -3.5%–3.9%; 0 of 4 pages | **fail** | 1.5%–57.9% | **fail** | **fail**: 4, 4 of 4 pages over | **holds**: 6.9% less | 905 / 1,694 | 204 / 356 |
| (d) a file per component, whole | -36.9%–-10.5%; 0 of 4 pages | **fail** | -16.3%–74.4% | **fail** | **fail**: 8–11, 4 of 4 pages over | **holds**: 3.5% less | 162 / 213 | 102 / 174 |
| (d) a file per component, pruned | -36.2%–-8.6%; 0 of 4 pages | **fail** | -8.7%–74.4% | **fail** | **fail**: 7–8, 4 of 4 pages over | fails: the control transfers 7.4% less | 0 / 0 | 87 / 116 |
| (e) one sheet, one script | 1.5%; 0 of 4 pages | **fail** | 0.0% | **fail** | pass: 3 | **holds**: 0.1% less | 1,038 / 1,948 | 1,040 / 1,415 |
| (f) the control | — | | — | | pass: 3 | — | 1,335 / 2,245 | 1,040 / 1,415 |

**CSS alone** — the scripts as today: inlined in each page.

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 5,119 / 7,585 / 11,544 | 28,934 / 8,867 / 7,585 | 2 | 115,035 / 34,941 / 29,871 | 5 | page 2 |
| (b′) the common head | 1 | 5,281 / 7,752 / 11,682 | 28,987 / 9,115 / 7,752 | 3 | 112,811 / 34,658 / 29,531 | 6 | page 2 |
| (b) needed by every page | 1 | 5,127 / 7,734 / 11,842 | 29,036 / 9,137 / 7,734 | 3 | 94,145 / 29,821 / 25,113 | 6 | never |
| (c) needed by ≥ 2 | 1 | 5,362 / 7,771 / 11,672 | 29,885 / 9,156 / 7,771 | 3 | 92,458 / 28,981 / 24,430 | 6 | never |
| (c) needed by ≥ 3 | 1 | 5,127 / 7,734 / 11,842 | 29,036 / 9,137 / 7,734 | 3 | 94,145 / 29,821 / 25,113 | 6 | never |
| (c) needed by ≥ half | 1 | 5,362 / 7,771 / 11,672 | 29,885 / 9,156 / 7,771 | 3 | 92,458 / 28,981 / 24,430 | 6 | never |
| (d) a file per component, whole | 6 | 5,726 / 8,355 / 12,476 | 29,372 / 10,002 / 8,355 | 7–8 | 93,150 / 30,095 / 25,275 | 11 | page 1 |
| (d) a file per component, pruned | 4 | 5,689 / 8,176 / 12,060 | 29,141 / 9,729 / 8,176 | 5–6 | 104,080 / 33,532 / 28,305 | 9 | page 1 |
| (e) one sheet, one script | 1 | 5,316 / 7,772 / 11,612 | 30,010 / 9,131 / 7,772 | 3 | 92,196 / 28,723 / 24,282 | 6 | never |
| (f) the control | 1 | 5,816 / 8,242 / 11,737 | 31,346 / 9,672 / 8,242 | 3 | 96,651 / 30,745 / 26,064 | 6 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least two pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 2.5%–16.8%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 2 | fails: the control transfers 12.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | -4.8%–8.7%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | fails: the control transfers 11.7% less | 0 / 0 | 0 / 0 |
| (b) needed by every page | -12.0%–16.8%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | **holds**: 3.6% less | 53 / 210 | 0 / 0 |
| (c) needed by ≥ 2 | -3.5%–3.9%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | **holds**: 6.3% less | 905 / 1,694 | 0 / 0 |
| (c) needed by ≥ 3 | -12.0%–16.8%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | **holds**: 3.6% less | 53 / 210 | 0 / 0 |
| (c) needed by ≥ half | -3.5%–3.9%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | **holds**: 6.3% less | 905 / 1,694 | 0 / 0 |
| (d) a file per component, whole | -36.9%–-10.5%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | **fail**: 7–8, 4 of 4 pages over | **holds**: 3.0% less | 162 / 213 | 0 / 0 |
| (d) a file per component, pruned | -36.2%–-8.6%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | **fail**: 5–6, 4 of 4 pages over | fails: the control transfers 7.9% less | 0 / 0 | 0 / 0 |
| (e) one sheet, one script | 1.5%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 3 | **holds**: 6.8% less | 1,038 / 1,948 | 0 / 0 |
| (f) the control | — | | — | | pass: 3 | — | 1,335 / 2,245 | 1,040 / 1,415 |

**JS alone** — the sheets as today: inlined in each page.

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 5,119 / 7,585 / 11,544 | 28,934 / 8,867 / 7,585 | 2 | 115,035 / 34,941 / 29,871 | 5 | page 2 |
| (b′) the common head | 1 | 5,175 / 7,658 / 11,629 | 28,981 / 8,975 / 7,658 | 3 | 114,449 / 34,831 / 29,768 | 6 | page 2 |
| (b) needed by every page | 1 | 5,175 / 7,658 / 11,629 | 28,981 / 8,975 / 7,658 | 3 | 114,449 / 34,831 / 29,768 | 6 | page 2 |
| (c) needed by ≥ 2 | 1 | 5,306 / 7,728 / 11,657 | 29,138 / 9,059 / 7,728 | 3 | 114,148 / 34,716 / 29,683 | 6 | page 2 |
| (c) needed by ≥ 3 | 1 | 5,175 / 7,658 / 11,629 | 28,981 / 8,975 / 7,658 | 3 | 114,449 / 34,831 / 29,768 | 6 | page 2 |
| (c) needed by ≥ half | 1 | 5,306 / 7,728 / 11,657 | 29,138 / 9,059 / 7,728 | 3 | 114,148 / 34,716 / 29,683 | 6 | page 2 |
| (d) a file per component, whole | 3 | 5,181 / 7,713 / 11,771 | 29,036 / 9,060 / 7,713 | 3–5 | 114,306 / 34,911 / 29,793 | 8 | page 2 |
| (d) a file per component, pruned | 2 | 5,181 / 7,696 / 11,703 | 29,021 / 9,038 / 7,696 | 3–4 | 114,248 / 34,824 / 29,725 | 7 | page 2 |
| (e) one sheet, one script | 0 | 5,707 / 8,020 / 11,662 | 29,974 / 9,354 / 8,020 | 2 | 119,193 / 36,886 / 31,611 | 5 | page 2 |
| (f) the control | 1 | 5,816 / 8,242 / 11,737 | 31,346 / 9,672 / 8,242 | 3 | 96,651 / 30,745 / 26,064 | 6 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least two pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 2.5%–16.8%; 0 of 4 pages | **fail** | 17.2%–82.3% | **fail** | pass: 2 | fails: the control transfers 12.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | 2.5%–16.8%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | pass: 3 | fails: the control transfers 12.4% less | 0 / 0 | 47 / 47 |
| (b) needed by every page | 2.5%–16.8%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | pass: 3 | fails: the control transfers 12.4% less | 0 / 0 | 47 / 47 |
| (c) needed by ≥ 2 | 2.5%–16.8%; 0 of 4 pages | **fail** | 1.5%–57.9% | **fail** | pass: 3 | fails: the control transfers 12.2% less | 0 / 0 | 204 / 356 |
| (c) needed by ≥ 3 | 2.5%–16.8%; 0 of 4 pages | **fail** | 5.4%–75.1% | **fail** | pass: 3 | fails: the control transfers 12.4% less | 0 / 0 | 47 / 47 |
| (c) needed by ≥ half | 2.5%–16.8%; 0 of 4 pages | **fail** | 1.5%–57.9% | **fail** | pass: 3 | fails: the control transfers 12.2% less | 0 / 0 | 204 / 356 |
| (d) a file per component, whole | 2.5%–16.8%; 0 of 4 pages | **fail** | -16.3%–74.4% | **fail** | **fail**: 3–5, 2 of 4 pages over | fails: the control transfers 12.5% less | 0 / 0 | 102 / 174 |
| (d) a file per component, pruned | 2.5%–16.8%; 0 of 4 pages | **fail** | -8.7%–74.4% | **fail** | **fail**: 3–4, 2 of 4 pages over | fails: the control transfers 12.3% less | 0 / 0 | 87 / 116 |
| (e) one sheet, one script | 2.5%–16.8%; 0 of 4 pages | **fail** | 0.0% | **fail** | pass: 2 | fails: the control transfers 17.5% less | 0 / 0 | 1,040 / 1,415 |
| (f) the control | — | | — | | pass: 3 | — | 1,335 / 2,245 | 1,040 / 1,415 |

**The session by kind** — brotli, each blob on its own: a file once, what is inlined on every page that has it. (CSS and JS together; the documents' HTML is the same in every row.)

| Packaging | CSS over the session | JS over the session |
| --- | ---: | ---: |
| (a) today | 7,741 | 1,090 |
| (b′) the common head | 7,393 | 957 |
| (b) needed by every page | 3,051 | 957 |
| (c) needed by ≥ 2 | 2,342 | 840 |
| (c) needed by ≥ 3 | 3,051 | 957 |
| (c) needed by ≥ half | 2,342 | 840 |
| (d) a file per component, whole | 2,937 | 999 |
| (d) a file per component, pruned | 6,396 | 945 |
| (e) one sheet, one script | 2,113 | 2,840 |
| (f) the control | 2,145 | 2,840 |

The least CSS any candidate transfers over the session is (e) one sheet, one script's, 2,113 B; the control's sheet is 2,145 B. The pages' own scripts are 1,090 B over the session and the control's 2,840 B — under 4096 B, so inlined in each of the 4 pages.

The scripts (b) and (c) would share are 258–567 B as written: under the 4096 B from which the builder's `auto` makes a shared blob a file. With that rule kept they stay inlined, and "CSS alone" is what (b) and (c) are.

**The cascade.** A rule that moves into a shared file changes its place among the rules left behind. What would guarantee the same computed styles:

| Packaging | The same computed styles | Pairs of rules whose order changes and could matter |
| --- | --- | --- |
| (a) today | nothing moves | none |
| (b′) the common head | by construction: the file is a prefix of every page's sheet, the rest follows it — the same rules in the same order | none |
| (b) needed by every page | **needs a proof**: the shared file is linked first, so a rule left inline now follows every shared rule it preceded | 45 over the 4 pages, 23 on the worst — `.rg-dialog` was before `.rg-menu` (/); `.rg-dialog` was before `.rg-sidemenu` (/); `.rg-dialog[open]` was before `.rg-sidemenu>[data-part=close]` (/) |
| (c) needed by ≥ 2 | **needs a proof**, as (b); and a page gets rules it does not need — those match nothing on it, and a custom property it does not need is read by nothing on it: that is the pruner's soundness | 1 over the 4 pages, 1 on the worst — `.rg-menu>:is(a,button)` was before `.rg-sidemenu::backdrop` (/syntax/) |
| (c) needed by ≥ 3 | **needs a proof**, as (b), with the pruner's soundness for what a page does not need | 45 over the 4 pages, 23 on the worst — `.rg-dialog` was before `.rg-menu` (/); `.rg-dialog` was before `.rg-sidemenu` (/); `.rg-dialog[open]` was before `.rg-sidemenu>[data-part=close]` (/) |
| (c) needed by ≥ half | **needs a proof**, as (b), with the pruner's soundness for what a page does not need | 1 over the 4 pages, 1 on the worst — `.rg-menu>:is(a,button)` was before `.rg-sidemenu::backdrop` (/syntax/) |
| (d) a file per component, whole | by construction, with the pruner's soundness: the files are linked in source order, so a page has its own rules in their order, and between them rules that match nothing on it | none |
| (d) a file per component, pruned | by construction: files and inlined parts stand in the head in source order — the page's own sheet, cut at file boundaries | none |
| (e) one sheet, one script | by construction, with the pruner's soundness: source order, and the rules a page does not need match nothing on it | none |
| (f) the control | nothing is pruned | none |

A pair is counted when the packaging puts a rule after one it preceded, both are rules the page needs, in the same layer, of equal specificity, and declare a property in common: then order decides between them **if** an element matches both. Whether one does is what the pruner would have to say — it matches every selector against the page, and reports only kept or dropped today. Zero pairs is safe as counted; any other number is that many questions to answer per build, or a rule left where it was.

**T5, T8 and the visit together.** With CSS and JS factored alike: **no candidate**. With the CSS factored and the scripts inlined as today: **no candidate**.
The visit alone — no more than the control over the session: together, (b) needed by every page; (c) needed by ≥ 2; (c) needed by ≥ 3; (c) needed by ≥ half; (d) a file per component, whole; (e) one sheet, one script; CSS alone, (b) needed by every page; (c) needed by ≥ 2; (c) needed by ≥ 3; (c) needed by ≥ half; (d) a file per component, whole; (e) one sheet, one script. Of those, with no rule or module a page does not need: together, (b) needed by every page; CSS alone, (b) needed by every page.
The least any candidate transfers over the session is 24,256 B ((c) needed by ≥ 2); the control: 26,064 B.

## The catalog

`bench/catalog-site`: ten pages on twenty components — a fixture. 10 artifacts; the session: `/`, `/pricing/`, `/docs/`, `/docs/api/`, `/changelog/`, `/blog/`, `/dashboard/`, `/settings/`, `/contact/`, `/404/`.

**What the artifacts need.** The control's sheet is 387 units (31,883 / 6,446 / 5,740 B); 371 of them are needed by some page. By how many pages need a unit:

| Needed by | Units | ≈ B, raw | |
| --- | ---: | ---: | --- |
| every page (10) | 64 | 5,220 | `tokens`, `rg-button`, `rg-menu`, `rg-sidemenu`, `site` |
| 9 pages | 4 | 318 | `tokens`, `rg-sidemenu`, `site` |
| 8 pages | 1 | 65 | `site` |
| 7 pages | 4 | 207 | `tokens`, `site` |
| 6 pages | 17 | 616 | `tokens`, `bc-badge`, `site` |
| 5 pages | 3 | 260 | `tokens`, `site` |
| 4 pages | 22 | 1,286 | `tokens`, `bc-badge`, `bc-callout`, `site` |
| 3 pages | 34 | 3,033 | `rg-sidemenu`, `tokens`, `bc-avatar`, `bc-badge`, `bc-card`, `bc-code`, `bc-tabs`, `site` |
| 2 pages | 70 | 6,934 | `rg-dialog`, `tokens`, `bc-badge`, `bc-breadcrumbs`, `bc-callout`, `bc-check`, `bc-code`, `bc-field`, `bc-select`, `bc-table`, `site` |
| 1 page | 152 | 12,509 | `rg-dialog`, `rg-menu`, `bc-accordion`, `bc-avatar`, `bc-badge`, `bc-callout`, `bc-card`, `bc-check`, `bc-code`, `bc-field`, `bc-pagination`, `bc-progress`, `bc-select`, `bc-table`, `bc-tabs`, `bc-toast`, `bc-tooltip`, `site` |
| no page | 16 | 1,215 | `rg-menu`, `rg-sidemenu`, `bc-accordion`, `bc-avatar`, `bc-card`, `bc-pagination`, `bc-progress`, `bc-select`, `bc-toast`, `site` |

Every page's sheet is written back from its units to the byte (10 of 10), and the control's. The components, by the top-level block of the source a rule stands in: `tokens`, `rg-button`, `rg-dialog`, `rg-menu`, `rg-sidemenu`, `bc-accordion`, `bc-avatar`, `bc-badge`, `bc-breadcrumbs`, `bc-callout`, `bc-card`, `bc-check`, `bc-code`, `bc-field`, `bc-pagination`, `bc-progress`, `bc-select`, `bc-table`, `bc-tabs`, `bc-toast`, `bc-tooltip`, `site`.

The scripts' modules — a module built under other flags is another:

| Module | B in a script | Pages |
| --- | ---: | --- |
| `overlays.ts` | 248 | every page (10) |
| `tabs.ts` | 626 | `/pricing/`, `/settings/` |
| `tabs.ts` | 799 | `/docs/` |
| `copy.ts` | 360 | `/docs/`, `/docs/api/` |
| `menu-keys.ts` | 832 | `/dashboard/` |
| `invokers.ts` | 307 | `/dashboard/`, `/settings/` |
| `field.ts` | 576 | `/settings/` |
| `toast.ts` | 465 | `/settings/` |

The control's script is 4,277 / 1,703 / 1,507 B, a file; its sheet is a file.

**CSS and JS together.**

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 2,436 / 4,454 / 6,460 | 17,808 / 5,279 / 4,454 | 2–3 | 176,517 / 51,333 / 43,305 | 12 | page 3 |
| (b′) the common head | 2 | 2,562 / 4,606 / 6,651 | 17,930 / 5,495 / 4,606 | 4–5 | 172,985 / 50,265 / 42,349 | 14 | page 3 |
| (b) needed by every page | 2 | 2,616 / 4,842 / 7,007 | 17,970 / 5,808 / 4,842 | 4–5 | 127,198 / 40,296 / 33,290 | 14 | page 4 |
| (c) needed by ≥ 2 | 2 | 5,231 / 6,588 / 7,880 | 28,026 / 7,795 / 6,588 | 4–5 | 101,326 / 32,491 / 26,348 | 14 | page 7 |
| (c) needed by ≥ 3 | 2 | 3,660 / 5,432 / 7,538 | 21,270 / 6,472 / 5,432 | 4–5 | 109,785 / 35,432 / 28,917 | 14 | page 7 |
| (c) needed by ≥ half | 2 | 2,891 / 4,950 / 7,159 | 18,386 / 5,949 / 4,950 | 4–5 | 119,467 / 38,137 / 31,298 | 14 | page 5 |
| (d) a file per component, whole | 29 | 4,047 / 6,820 / 10,111 | 22,672 / 8,407 / 6,820 | 8–20 | 104,651 / 36,954 / 29,394 | 41 | page 3 |
| (d) a file per component, pruned | 10 | 2,854 / 5,191 / 7,595 | 18,093 / 6,349 / 5,191 | 5–9 | 145,390 / 46,774 / 38,688 | 22 | page 3 |
| (e) one sheet, one script | 2 | 7,931 / 8,858 / 9,383 | 41,589 / 10,210 / 8,858 | 4–5 | 99,814 / 29,174 / 23,771 | 14 | never |
| (f) the control | 2 | 8,121 / 9,041 / 9,565 | 42,804 / 10,416 / 9,041 | 4–5 | 101,029 / 29,363 / 23,953 | 14 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least half of the pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 41.8%–72.9%; 10 of 10 pages | pass | 41.9%–91.6% | pass | pass: 2–3 | fails: the control transfers 44.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | 40.6%–72.1%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 4–5, 10 of 10 pages over | fails: the control transfers 43.4% less | 22 / 22 | 47 / 47 |
| (b) needed by every page | 34.1%–71.6%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 4–5, 10 of 10 pages over | fails: the control transfers 28.0% less | 61 / 254 | 47 / 47 |
| (c) needed by ≥ 2 | 20.3%–36.7%; 10 of 10 pages | pass | 24.7%–55.7% | **fail** | **fail**: 4–5, 10 of 10 pages over | fails: the control transfers 9.1% less | 9,077 / 12,510 | 1,089 / 1,346 |
| (c) needed by ≥ 3 | 25.1%–53.3%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 4–5, 10 of 10 pages over | fails: the control transfers 17.2% less | 3,362 / 5,363 | 47 / 47 |
| (c) needed by ≥ half | 31.8%–66.5%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 4–5, 10 of 10 pages over | fails: the control transfers 23.5% less | 478 / 1,147 | 47 / 47 |
| (d) a file per component, whole | -11.0%–47.7%; 7 of 10 pages | pass | 8.3%–87.9% | **fail** | **fail**: 8–20, 10 of 10 pages over | fails: the control transfers 18.5% less | 4,256 / 5,222 | 151 / 463 |
| (d) a file per component, pruned | 17.3%–66.8%; 9 of 10 pages | pass | 22.3%–87.9% | **fail** | **fail**: 5–9, 10 of 10 pages over | fails: the control transfers 38.1% less | 0 / 0 | 93 / 174 |
| (e) one sheet, one script | 3.2%; 0 of 10 pages | **fail** | 0.0% | **fail** | **fail**: 4–5, 10 of 10 pages over | **holds**: 0.8% less | 20,284 / 24,824 | 3,430 / 4,025 |
| (f) the control | — | | — | | **fail**: 4–5, 10 of 10 pages over | — | 21,499 / 26,039 | 3,430 / 4,025 |

**CSS alone** — the scripts as today: inlined in each page.

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 2,436 / 4,454 / 6,460 | 17,808 / 5,279 / 4,454 | 2–3 | 176,517 / 51,333 / 43,305 | 12 | page 3 |
| (b′) the common head | 1 | 2,492 / 4,534 / 6,536 | 17,883 / 5,388 / 4,534 | 3–4 | 174,837 / 50,818 / 42,802 | 13 | page 3 |
| (b) needed by every page | 1 | 2,554 / 4,773 / 6,906 | 17,923 / 5,706 / 4,773 | 3–4 | 129,050 / 40,891 / 33,774 | 13 | page 4 |
| (c) needed by ≥ 2 | 1 | 4,676 / 6,129 / 7,638 | 26,937 / 7,230 / 6,129 | 3–4 | 104,453 / 33,648 / 27,372 | 13 | page 6 |
| (c) needed by ≥ 3 | 1 | 3,600 / 5,361 / 7,436 | 21,223 / 6,370 / 5,361 | 3–4 | 111,637 / 36,035 / 29,383 | 13 | page 7 |
| (c) needed by ≥ half | 1 | 2,834 / 4,881 / 7,046 | 18,339 / 5,848 / 4,881 | 3–4 | 121,319 / 38,739 / 31,780 | 13 | page 5 |
| (d) a file per component, whole | 22 | 3,982 / 6,667 / 9,621 | 22,521 / 8,171 / 6,667 | 7–15 | 107,915 / 37,788 / 30,310 | 34 | page 3 |
| (d) a file per component, pruned | 6 | 2,784 / 5,068 / 7,320 | 18,000 / 6,161 / 5,068 | 4–7 | 148,239 / 47,500 / 39,411 | 18 | page 3 |
| (e) one sheet, one script | 1 | 6,505 / 7,660 / 8,717 | 38,131 / 8,870 / 7,660 | 3–4 | 103,728 / 31,101 / 25,355 | 13 | page 7 |
| (f) the control | 2 | 8,121 / 9,041 / 9,565 | 42,804 / 10,416 / 9,041 | 4–5 | 101,029 / 29,363 / 23,953 | 14 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least half of the pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 41.8%–72.9%; 10 of 10 pages | pass | 41.9%–91.6% | pass | pass: 2–3 | fails: the control transfers 44.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | 40.6%–72.1%; 10 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 44.0% less | 22 / 22 | 0 / 0 |
| (b) needed by every page | 34.1%–71.6%; 10 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 29.1% less | 61 / 254 | 0 / 0 |
| (c) needed by ≥ 2 | 20.3%–36.7%; 10 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 12.5% less | 9,077 / 12,510 | 0 / 0 |
| (c) needed by ≥ 3 | 25.1%–53.3%; 10 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 18.5% less | 3,362 / 5,363 | 0 / 0 |
| (c) needed by ≥ half | 31.8%–66.5%; 10 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 24.6% less | 478 / 1,147 | 0 / 0 |
| (d) a file per component, whole | -11.0%–47.7%; 7 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 7–15, 10 of 10 pages over | fails: the control transfers 21.0% less | 4,256 / 5,222 | 0 / 0 |
| (d) a file per component, pruned | 17.3%–66.8%; 9 of 10 pages | pass | 41.9%–91.6% | pass | **fail**: 4–7, 10 of 10 pages over | fails: the control transfers 39.2% less | 0 / 0 | 0 / 0 |
| (e) one sheet, one script | 3.2%; 0 of 10 pages | **fail** | 41.9%–91.6% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 5.5% less | 20,284 / 24,824 | 0 / 0 |
| (f) the control | — | | — | | **fail**: 4–5, 10 of 10 pages over | — | 21,499 / 26,039 | 3,430 / 4,025 |

**JS alone** — the sheets as today: inlined in each page.

| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| (a) today | 0 | 2,436 / 4,454 / 6,460 | 17,808 / 5,279 / 4,454 | 2–3 | 176,517 / 51,333 / 43,305 | 12 | page 3 |
| (b′) the common head | 1 | 2,509 / 4,532 / 6,559 | 17,855 / 5,391 / 4,532 | 3–4 | 174,665 / 50,831 / 42,915 | 13 | page 3 |
| (b) needed by every page | 1 | 2,509 / 4,532 / 6,559 | 17,855 / 5,391 / 4,532 | 3–4 | 174,665 / 50,831 / 42,915 | 13 | page 3 |
| (c) needed by ≥ 2 | 1 | 3,002 / 4,926 / 6,740 | 18,897 / 5,855 / 4,926 | 3–4 | 173,390 / 50,288 / 42,419 | 13 | page 3 |
| (c) needed by ≥ 3 | 1 | 2,509 / 4,532 / 6,559 | 17,855 / 5,391 / 4,532 | 3–4 | 174,665 / 50,831 / 42,915 | 13 | page 3 |
| (c) needed by ≥ half | 1 | 2,509 / 4,532 / 6,559 | 17,855 / 5,391 / 4,532 | 3–4 | 174,665 / 50,831 / 42,915 | 13 | page 3 |
| (d) a file per component, whole | 7 | 2,514 / 4,628 / 6,978 | 17,959 / 5,535 / 4,628 | 3–7 | 173,253 / 50,699 / 42,601 | 19 | page 3 |
| (d) a file per component, pruned | 4 | 2,514 / 4,586 / 6,760 | 17,901 / 5,477 / 4,586 | 3–5 | 173,668 / 50,703 / 42,666 | 16 | page 3 |
| (e) one sheet, one script | 1 | 3,876 / 5,670 / 7,158 | 21,266 / 6,641 / 5,670 | 3–4 | 172,603 / 49,629 / 41,901 | 13 | page 3 |
| (f) the control | 2 | 8,121 / 9,041 / 9,565 | 42,804 / 10,416 / 9,041 | 4–5 | 101,029 / 29,363 / 23,953 | 14 | — |

| Packaging | T5, CSS: smaller than the control's, brotli (at least half of the pages at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |
| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |
| (a) today | 41.8%–72.9%; 10 of 10 pages | pass | 41.9%–91.6% | pass | pass: 2–3 | fails: the control transfers 44.7% less | 0 / 0 | 0 / 0 |
| (b′) the common head | 41.8%–72.9%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 44.2% less | 0 / 0 | 47 / 47 |
| (b) needed by every page | 41.8%–72.9%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 44.2% less | 0 / 0 | 47 / 47 |
| (c) needed by ≥ 2 | 41.8%–72.9%; 10 of 10 pages | pass | 24.7%–55.7% | **fail** | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 43.5% less | 0 / 0 | 1,089 / 1,346 |
| (c) needed by ≥ 3 | 41.8%–72.9%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 44.2% less | 0 / 0 | 47 / 47 |
| (c) needed by ≥ half | 41.8%–72.9%; 10 of 10 pages | pass | 36.4%–88.4% | pass | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 44.2% less | 0 / 0 | 47 / 47 |
| (d) a file per component, whole | 41.8%–72.9%; 10 of 10 pages | pass | 8.3%–87.9% | **fail** | **fail**: 3–7, 6 of 10 pages over | fails: the control transfers 43.8% less | 0 / 0 | 151 / 463 |
| (d) a file per component, pruned | 41.8%–72.9%; 10 of 10 pages | pass | 22.3%–87.9% | **fail** | **fail**: 3–5, 6 of 10 pages over | fails: the control transfers 43.9% less | 0 / 0 | 93 / 174 |
| (e) one sheet, one script | 41.8%–72.9%; 10 of 10 pages | pass | 0.0% | **fail** | **fail**: 3–4, 1 of 10 pages over | fails: the control transfers 42.8% less | 0 / 0 | 3,430 / 4,025 |
| (f) the control | — | | — | | **fail**: 4–5, 10 of 10 pages over | — | 21,499 / 26,039 | 3,430 / 4,025 |

**The session by kind** — brotli, each blob on its own: a file once, what is inlined on every page that has it. (CSS and JS together; the documents' HTML is the same in every row.)

| Packaging | CSS over the session | JS over the session |
| --- | ---: | ---: |
| (a) today | 23,937 | 3,454 |
| (b′) the common head | 23,294 | 2,941 |
| (b) needed by every page | 14,125 | 2,941 |
| (c) needed by ≥ 2 | 7,818 | 2,398 |
| (c) needed by ≥ 3 | 9,861 | 2,941 |
| (c) needed by ≥ half | 12,220 | 2,941 |
| (d) a file per component, whole | 9,804 | 2,601 |
| (d) a file per component, pruned | 21,215 | 2,714 |
| (e) one sheet, one script | 5,557 | 1,507 |
| (f) the control | 5,740 | 1,507 |

The least CSS any candidate transfers over the session is (e) one sheet, one script's, 5,557 B; the control's sheet is 5,740 B. The pages' own scripts are 3,454 B over the session and the control's 1,507 B — a file, fetched once.

The scripts (b) and (c) would share are 258–1,557 B as written: under the 4096 B from which the builder's `auto` makes a shared blob a file. With that rule kept they stay inlined, and "CSS alone" is what (b) and (c) are.

**The cascade.** A rule that moves into a shared file changes its place among the rules left behind. What would guarantee the same computed styles:

| Packaging | The same computed styles | Pairs of rules whose order changes and could matter |
| --- | --- | --- |
| (a) today | nothing moves | none |
| (b′) the common head | by construction: the file is a prefix of every page's sheet, the rest follows it — the same rules in the same order | none |
| (b) needed by every page | **needs a proof**: the shared file is linked first, so a rule left inline now follows every shared rule it preceded | 53 over the 10 pages, 25 on the worst — `.rg-sidemenu>[data-part=header]` was before `.rg-sidemenu>[data-part=close]` (/docs/); `.rg-sidemenu>[data-part=header]` was before `.rg-sidemenu>[data-part=scrim]` (/docs/); `.rg-dialog` was before `.rg-menu` (/dashboard/) |
| (c) needed by ≥ 2 | **needs a proof**, as (b); and a page gets rules it does not need — those match nothing on it, and a custom property it does not need is read by nothing on it: that is the pruner's soundness | 34 over the 10 pages, 20 on the worst — `.bc-accordion` was before `.bc-badge` (/pricing/); `.bc-accordion` was before `.bc-card` (/pricing/); `.bc-accordion` was before `.bc-tabs` (/pricing/) |
| (c) needed by ≥ 3 | **needs a proof**, as (b), with the pruner's soundness for what a page does not need | 92 over the 10 pages, 51 on the worst — `.bc-accordion` was before `.bc-badge` (/pricing/); `.bc-accordion` was before `.bc-card` (/pricing/); `.bc-accordion` was before `.bc-tabs` (/pricing/) |
| (c) needed by ≥ half | **needs a proof**, as (b), with the pruner's soundness for what a page does not need | 79 over the 10 pages, 25 on the worst — `.bc-avatar` was before `.bc-badge` (/); `.rg-sidemenu>[data-part=header]` was before `.rg-sidemenu>[data-part=close]` (/docs/); `.rg-sidemenu>[data-part=header]` was before `.rg-sidemenu>[data-part=scrim]` (/docs/) |
| (d) a file per component, whole | by construction, with the pruner's soundness: the files are linked in source order, so a page has its own rules in their order, and between them rules that match nothing on it | none |
| (d) a file per component, pruned | by construction: files and inlined parts stand in the head in source order — the page's own sheet, cut at file boundaries | none |
| (e) one sheet, one script | by construction, with the pruner's soundness: source order, and the rules a page does not need match nothing on it | none |
| (f) the control | nothing is pruned | none |

A pair is counted when the packaging puts a rule after one it preceded, both are rules the page needs, in the same layer, of equal specificity, and declare a property in common: then order decides between them **if** an element matches both. Whether one does is what the pruner would have to say — it matches every selector against the page, and reports only kept or dropped today. Zero pairs is safe as counted; any other number is that many questions to answer per build, or a rule left where it was.

**T5, T8 and the visit together.** With CSS and JS factored alike: **no candidate**. With the CSS factored and the scripts inlined as today: **no candidate**.
The visit alone — no more than the control over the session: together, (e) one sheet, one script; CSS alone, none. Of those, with no rule or module a page does not need: together, none; CSS alone, none.
The least any candidate transfers over the session is 23,771 B ((e) one sheet, one script); the control: 23,953 B.

## What it says

- **Over a visit that reaches every page, one sheet of the union is the floor, and the control's sheet is that floor but for what no page uses.** Every packaging of exact per-page analysis transfers each needed rule at least once over such a session — the union — and in more, smaller pieces, each compressed on its own; (e) is the union as one file, and the control's sheet is (e) and the rules no page needs (*The session by kind*). So where the site uses nearly all of the design system it imports — both sites here — awareness cannot win a long visit by its CSS: only where the design system is larger than what the site uses, or where the visit is short. The scripts go either way, by how the control's is delivered: inlined in every page, the pages' own win the session; as a file, the control's does.
- **Cold and the visit pull apart.** What makes a cold page light — its own rules only, inlined, one request — leaves nothing in the cache for the next page; what makes a visit cheap — a shared file — costs the cold page a request and, where the file holds more than the page needs, bytes. Of the packagings that hand a page nothing it does not need, (b) shares the most; (c) and (d, whole) buy the visit with rules of other pages; (e) is the control without its unused rules.
- **A request is counted, a round trip is not.** T8 counts requests; the bytes here count no header and no round trip of the first view (builder.md, *Packaging*: the 4096 B rule is for that). *JS alone* shows the other side: a script of 250 B as a file is a request for 250 B.
- **Order.** (b′), (d) and (e) keep the cascade by construction. (b) and (c) — the ones that share the most while staying near exact — change the order of rules, and need the pruner to say, per page, that no element matches both rules of a pair: it does not today.

Not measured: time; a real network; a browser (the cold loads are computed from the files); scripts that run — a shared script here is the pages' own modules and an `export`, not a build.
