# The catalog site, measured

Written by `node bench/catalog.mjs` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). **A fixture**: `bench/catalog-site`, 10 pages, on `bench/catalog`, 20 components — neither is the design system or a product (their READMEs). Three builds, each loaded cold in headless Chrome by `bench/measure.mjs`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.

`reactogenic check` on the site: no diagnostic. The three builds: no diagnostic.

## The catalog

What each component weighs. CSS: its file as written (with comments); as the builder bundles it (the report's `bytesIn`: nesting lowered, no comments, not minified); and minified — the selectors and declarations of the control's sheet that name its root class. "On" is the pages whose sheet keeps a rule of it.

| Component | From | CSS, written | bundled | minified ≈ | Behaviours it mounts | On |
| --- | --- | ---: | ---: | ---: | --- | ---: |
| `Button` | `@reactogenic/ui` | 686 | 609 | 447 | — | 10 |
| `Dialog` | `@reactogenic/ui` | 3,817 | 2,392 | 1,667 | `overlays`, `invokers` | 2 |
| `DropdownMenu` | `@reactogenic/ui` | 3,566 | 1,743 | 1,412 | `overlays`, `menu-keys`, `invokers` | 10 |
| `SideMenu` | `@reactogenic/ui` | 5,322 | 3,523 | 2,460 | `overlays` | 10 |
| `Accordion` | the fixture | 1,631 | 1,647 | 1,178 | — | 1 |
| `Avatar` | the fixture | 974 | 907 | 613 | — | 3 |
| `Badge` | the fixture | 1,342 | 1,243 | 966 | — | 6 |
| `Breadcrumbs` | the fixture | 816 | 748 | 513 | — | 2 |
| `Callout` | the fixture | 1,616 | 1,592 | 1,205 | — | 4 |
| `Card` | the fixture | 1,858 | 1,784 | 1,256 | — | 3 |
| `Checkbox` | the fixture | 2,181 | 2,058 | 1,471 | — | 2 |
| `CodeBlock` | the fixture | 1,914 | 1,814 | 1,361 | `copy` | 3 |
| `Field` | the fixture | 2,788 | 2,604 | 2,005 | `field` | 2 |
| `Pagination` | the fixture | 1,486 | 1,382 | 1,029 | — | 1 |
| `Progress` | the fixture | 2,294 | 2,137 | 1,525 | — | 1 |
| `Select` | the fixture | 1,585 | 1,518 | 1,134 | — | 2 |
| `Table` | the fixture | 2,019 | 1,791 | 1,295 | — | 2 |
| `Tabs` | the fixture | 1,449 | 1,456 | 1,104 | `tabs` | 3 |
| `Toast` | the fixture | 1,706 | 1,509 | 1,046 | `overlays`, `toast` | 1 |
| `Tooltip` | the fixture | 1,822 | 1,514 | 1,122 | — | 1 |
| tokens (2 files) | | 1,763 | 1,070 | | | |
| the site's own `site.css` | | 8,138 | 6,789 | | | |
| **the twenty** | | **40,872** | **33,971** | **24,809** | | |

The sixteen of the fixture are 18,823 B minified, 1,176 B each on average; the four of `@reactogenic/ui`, 5,986 B, 1,497 B each. Components no page uses: none.

| Behaviour | From | Written | In a page's script (the report's row) | Flags | On |
| --- | --- | ---: | ---: | --- | ---: |
| `overlays` | `@reactogenic/ui` | 1,583 | 248 | — | 10 |
| `invokers` | `@reactogenic/ui` | 1,347 | 307 | — | 2 |
| `menu-keys` | `@reactogenic/ui` | 3,258 | 832 | `RG_MENU_TYPEAHEAD` | 1 |
| `tabs` | the fixture | 2,772 | 626 – 799 | `RG_TABS_HASH` | 3 |
| `field` | the fixture | 2,147 | 576 | `RG_FIELD_COUNT`, `RG_FIELD_REVEAL` | 1 |
| `toast` | the fixture | 1,642 | 465 | — | 1 |
| `copy` | the fixture | 1,455 | 360 | — | 2 |

## The pages

In the order of the session. "Components" is the report's record of what was rendered, less the page, the layout and the helpers.

| Page | Components rendered | Behaviours mounted | Rules kept of the sheet's | Custom properties dropped |
| --- | --- | --- | ---: | ---: |
| `/` | Avatar ×2, Badge ×2, Button ×4, Card ×9, DropdownMenu, SideMenu | overlays | 80 of 307 | 9 |
| `/pricing/` | Accordion, Badge ×2, Button ×8, Card ×6, DropdownMenu, SideMenu, Tabs, Tooltip ×2 | overlays, tabs | 100 of 307 | 8 |
| `/docs/` | Breadcrumbs, Button ×2, Callout ×2, CodeBlock ×7, DropdownMenu, SideMenu, Tabs | overlays, tabs `RG_TABS_HASH`, copy | 96 of 307 | 4 |
| `/docs/api/` | Badge ×9, Breadcrumbs, Button ×2, CodeBlock ×2, DropdownMenu, SideMenu, Table ×2 | overlays, copy | 92 of 307 | 3 |
| `/changelog/` | Badge ×14, Button ×2, Callout, DropdownMenu, Pagination, SideMenu | overlays | 80 of 307 | 3 |
| `/blog/` | Avatar, Badge ×2, Button ×2, Callout, CodeBlock, DropdownMenu, SideMenu | overlays | 78 of 307 | 8 |
| `/dashboard/` | Badge ×6, Button ×5, Card ×3, Dialog, DropdownMenu ×2, Progress ×5, SideMenu, Table | overlays, menu-keys `RG_MENU_TYPEAHEAD`, invokers | 105 of 307 | 4 |
| `/settings/` | Avatar, Button ×7, Callout, Checkbox ×8, Dialog, DropdownMenu, Field ×6, Select ×2, SideMenu, Tabs, Toast | overlays, tabs, field `RG_FIELD_COUNT`, toast, field `RG_FIELD_REVEAL`, invokers | 145 of 307 | 4 |
| `/contact/` | Button ×4, Checkbox, DropdownMenu, Field ×4, Select, SideMenu | overlays | 85 of 307 | 10 |
| `/404/` | Button ×4, DropdownMenu, SideMenu | overlays | 52 of 307 | 14 |

## `default` — `reactogenic build`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 3 | 15,024 / 4,682 / 3,894 | 0 / 0 / 0 | 0 / 0 / 0 | 394 / 346 / 308 | 15,418 / 5,028 / 4,202 | 252 / 8,816 / 0 | 252 |
| `/pricing/` | 2 | 19,698 / 5,478 / 4,592 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 19,872 / 5,640 / 4,729 | 911 / 11,339 / 0 | 911 |
| `/docs/` | 2 | 20,622 / 5,892 / 5,031 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 20,796 / 6,054 / 5,168 | 1,748 / 10,628 / 0 | 1,748 |
| `/docs/api/` | 2 | 18,928 / 5,462 / 4,609 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 19,102 / 5,624 / 4,746 | 661 / 10,453 / 0 | 661 |
| `/changelog/` | 2 | 15,480 / 4,601 / 3,860 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 15,654 / 4,763 / 3,997 | 252 / 9,486 / 0 | 252 |
| `/blog/` | 2 | 14,734 / 4,854 / 4,050 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 14,908 / 5,016 / 4,187 | 252 / 9,209 / 0 | 252 |
| `/dashboard/` | 2 | 20,761 / 5,821 / 4,986 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 20,935 / 5,983 / 5,123 | 1,443 / 12,282 / 0 | 1,443 |
| `/settings/` | 2 | 28,363 / 7,442 / 6,350 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 28,537 / 7,604 / 6,487 | 2,448 / 16,479 / 0 | 2,448 |
| `/contact/` | 2 | 14,280 / 4,248 / 3,512 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 14,454 / 4,410 / 3,649 | 252 / 9,461 / 0 | 252 |
| `/404/` | 2 | 8,613 / 2,811 / 2,313 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 8,787 / 2,973 / 2,450 | 252 / 5,852 / 0 | 252 |
| **session (10 pages, warm cache)** | 12 | 176,503 / 51,291 / 43,197 | 0 / 0 / 0 | 0 / 0 / 0 | 394 / 346 / 308 | 176,897 / 51,637 / 43,505 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,910 / 2,096 / 1,627 | 8,816 / 2,475 / 2,150 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/pricing/` | 7,402 / 2,097 / 1,628 | 11,339 / 2,971 / 2,598 | 911 / 506 / 404 | inline | inline |
| `/docs/` | 8,200 / 2,421 / 1,957 | 10,628 / 2,802 / 2,446 | 1,748 / 784 / 649 | inline | inline |
| `/docs/api/` | 7,768 / 2,420 / 1,902 | 10,453 / 2,759 / 2,420 | 661 / 386 / 307 | inline | inline |
| `/changelog/` | 5,696 / 1,885 / 1,481 | 9,486 / 2,616 / 2,280 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/blog/` | 5,227 / 2,166 / 1,694 | 9,209 / 2,586 / 2,251 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/dashboard/` | 6,990 / 2,074 / 1,678 | 12,282 / 3,155 / 2,775 | 1,443 / 707 / 588 | inline | inline |
| `/settings/` | 9,390 / 2,697 / 2,145 | 16,479 / 3,842 / 3,367 | 2,448 / 1,022 / 876 | inline | inline |
| `/contact/` | 4,521 / 1,609 / 1,212 | 9,461 / 2,539 / 2,217 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/404/` | 2,463 / 925 / 689 | 5,852 / 1,795 / 1,557 | 252 / 176 / 126 | inline | inline, the same on 5 pages |

## `control` — `reactogenic build --no-specialize`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 5 | 6,022 / 2,154 / 1,674 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 394 / 346 / 308 | 42,654 / 10,689 / 9,266 | 0 / 0 / 0 | 4,277 |
| `/pricing/` | 4 | 7,514 / 2,156 / 1,668 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 43,926 / 10,507 / 9,089 | 0 / 0 / 0 | 4,277 |
| `/docs/` | 4 | 8,312 / 2,481 / 1,996 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 44,724 / 10,832 / 9,417 | 0 / 0 / 0 | 4,277 |
| `/docs/api/` | 4 | 7,880 / 2,476 / 1,948 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 44,292 / 10,827 / 9,369 | 0 / 0 / 0 | 4,277 |
| `/changelog/` | 4 | 5,808 / 1,941 / 1,525 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 42,220 / 10,292 / 8,946 | 0 / 0 / 0 | 4,277 |
| `/blog/` | 4 | 5,339 / 2,224 / 1,737 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 41,751 / 10,575 / 9,158 | 0 / 0 / 0 | 4,277 |
| `/dashboard/` | 4 | 7,102 / 2,132 / 1,719 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 43,514 / 10,483 / 9,140 | 0 / 0 / 0 | 4,277 |
| `/settings/` | 4 | 9,502 / 2,758 / 2,190 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 45,914 / 11,109 / 9,611 | 0 / 0 / 0 | 4,277 |
| `/contact/` | 4 | 4,633 / 1,666 / 1,252 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 41,045 / 10,017 / 8,673 | 0 / 0 / 0 | 4,277 |
| `/404/` | 4 | 2,575 / 982 / 733 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 38,987 / 9,333 / 8,154 | 0 / 0 / 0 | 4,277 |
| **session (10 pages, warm cache)** | 14 | 64,687 / 20,970 / 16,442 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | 394 / 346 / 308 | 101,319 / 29,505 / 24,034 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,910 / 2,096 / 1,627 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/pricing/` | 7,402 / 2,097 / 1,628 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/docs/` | 8,200 / 2,421 / 1,957 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/docs/api/` | 7,768 / 2,420 / 1,902 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/changelog/` | 5,696 / 1,885 / 1,481 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/blog/` | 5,227 / 2,166 / 1,694 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/dashboard/` | 6,990 / 2,074 / 1,678 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/settings/` | 9,390 / 2,697 / 2,145 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/contact/` | 4,521 / 1,609 / 1,212 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/404/` | 2,463 / 925 / 689 | 31,961 / 6,486 / 5,777 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |

The control's script, split: its behaviours — every module any page mounts, every flag on — are 3,587 / 1,412 / 1,210; its own cost, the list of modules, the table from pathname to mounts and the loop that reads it, is 690 / 364 / 330 (compressed apart; the script as a whole is 4,277 / 1,703 / 1,507):

```js
var k=[s,l,c,u,E,p,L],N={"/":[[0]],"/404/":[[0]],"/blog/":[[0]],"/changelog/":[[0]],"/contact/":[[0]],"/dashboard/":[[0],[1,"m2",{typeahead:!0}],[2]],"/docs/":[[0],[3,"pm",{hash:!0}],[4,"c1",{done:"Copied"}],[4,"c2",{done:"Copied"}],[4,"c3",{done:"Copied"}],[4,"c4",{done:"Copied"}],[4,"c5",{done:"Copied"}],[4,"c7",{done:"Copied"}]],"/docs/api/":[[0],[4,"c1",{done:"Copied"}]],"/pricing/":[[0],[3,"t1"]],"/settings/":[[0],[3,"t1"],[5,"f4",{count:!0}],[6,"toast1",{timeout:4e3}],[5,"f5",{reveal:!0}],[5,"f6",{reveal:!0}],[2]]};for(let[e,t,n]of N[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/|(\.html))?$/,(o,a,i)=>i||"/")]||[])t?k[e](document.getElementById(t),n):k[e]();
```

## `never` — `reactogenic build --inline never`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 5 | 6,022 / 2,156 / 1,674 | 8,816 / 2,475 / 2,150 | 252 / 176 / 126 | 394 / 346 / 308 | 15,484 / 5,153 / 4,258 | 0 / 0 / 0 | 252 |
| `/pricing/` | 4 | 7,514 / 2,156 / 1,673 | 11,339 / 2,971 / 2,598 | 911 / 506 / 404 | 174 / 162 / 137 | 19,938 / 5,795 / 4,812 | 0 / 0 / 0 | 911 |
| `/docs/` | 4 | 8,312 / 2,484 / 2,000 | 10,628 / 2,802 / 2,446 | 1,748 / 784 / 649 | 174 / 162 / 137 | 20,862 / 6,232 / 5,232 | 0 / 0 / 0 | 1,748 |
| `/docs/api/` | 4 | 7,880 / 2,481 / 1,951 | 10,453 / 2,759 / 2,420 | 661 / 386 / 307 | 174 / 162 / 137 | 19,168 / 5,788 / 4,815 | 0 / 0 / 0 | 661 |
| `/changelog/` | 4 | 5,808 / 1,942 / 1,528 | 9,486 / 2,616 / 2,280 | 252 / 176 / 126 | 174 / 162 / 137 | 15,720 / 4,896 / 4,071 | 0 / 0 / 0 | 252 |
| `/blog/` | 4 | 5,339 / 2,226 / 1,741 | 9,209 / 2,586 / 2,251 | 252 / 176 / 126 | 174 / 162 / 137 | 14,974 / 5,150 / 4,255 | 0 / 0 / 0 | 252 |
| `/dashboard/` | 4 | 7,102 / 2,133 / 1,718 | 12,282 / 3,155 / 2,775 | 1,443 / 707 / 588 | 174 / 162 / 137 | 21,001 / 6,157 / 5,218 | 0 / 0 / 0 | 1,443 |
| `/settings/` | 4 | 9,502 / 2,760 / 2,191 | 16,479 / 3,842 / 3,367 | 2,448 / 1,022 / 876 | 174 / 162 / 137 | 28,603 / 7,786 / 6,571 | 0 / 0 / 0 | 2,448 |
| `/contact/` | 4 | 4,633 / 1,667 / 1,253 | 9,461 / 2,539 / 2,217 | 252 / 176 / 126 | 174 / 162 / 137 | 14,520 / 4,544 / 3,733 | 0 / 0 / 0 | 252 |
| `/404/` | 4 | 2,575 / 984 / 734 | 5,852 / 1,795 / 1,557 | 252 / 176 / 126 | 174 / 162 / 137 | 8,853 / 3,117 / 2,554 | 0 / 0 / 0 | 252 |
| **session (10 pages, warm cache)** | 28 | 64,687 / 20,989 / 16,463 | 104,005 / 27,540 / 24,061 | 7,463 / 3,581 / 2,950 | 394 / 346 / 308 | 176,549 / 52,456 / 43,782 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,910 / 2,096 / 1,627 | 8,816 / 2,475 / 2,150 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/pricing/` | 7,402 / 2,097 / 1,628 | 11,339 / 2,971 / 2,598 | 911 / 506 / 404 | file, 1 page | file, 1 page |
| `/docs/` | 8,200 / 2,421 / 1,957 | 10,628 / 2,802 / 2,446 | 1,748 / 784 / 649 | file, 1 page | file, 1 page |
| `/docs/api/` | 7,768 / 2,420 / 1,902 | 10,453 / 2,759 / 2,420 | 661 / 386 / 307 | file, 1 page | file, 1 page |
| `/changelog/` | 5,696 / 1,885 / 1,481 | 9,486 / 2,616 / 2,280 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/blog/` | 5,227 / 2,166 / 1,694 | 9,209 / 2,586 / 2,251 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/dashboard/` | 6,990 / 2,074 / 1,678 | 12,282 / 3,155 / 2,775 | 1,443 / 707 / 588 | file, 1 page | file, 1 page |
| `/settings/` | 9,390 / 2,697 / 2,145 | 16,479 / 3,842 / 3,367 | 2,448 / 1,022 / 876 | file, 1 page | file, 1 page |
| `/contact/` | 4,521 / 1,609 / 1,212 | 9,461 / 2,539 / 2,217 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/404/` | 2,463 / 925 / 689 | 5,852 / 1,795 / 1,557 | 252 / 176 / 126 | file, 1 page | file, 5 pages |

## Summary

Means over 10 cold page loads; brotli -q 11 unless marked raw.

| Approach | Req / page | HTML br | CSS br (ext + inline raw) | JS br (ext) | JS to parse, raw | Other br | Page total br | Session total br | Session JS br |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| default | 2 | 4,320 | 0 + 10,401 | 0 | 847 | 154 | 4,474 | 43,505 | 0 |
| control | 4 | 1,644 | 5,777 + 0 | 1,507 | 4,277 | 154 | 9,082 | 24,034 | 1,507 |
| never | 4 | 1,646 | 2,406 + 0 | 345 | 847 | 154 | 4,552 | 43,782 | 2,950 |

## The default build against the control

What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control, raw / gzip / brotli.

| Page | CSS, default | CSS, control | smaller |
| --- | ---: | ---: | ---: |
| `/` | 8,816 / 2,475 / 2,150 | 31,961 / 6,486 / 5,777 | 72.4% / 61.8% / 62.8% |
| `/pricing/` | 11,339 / 2,971 / 2,598 | 31,961 / 6,486 / 5,777 | 64.5% / 54.2% / 55.0% |
| `/docs/` | 10,628 / 2,802 / 2,446 | 31,961 / 6,486 / 5,777 | 66.7% / 56.8% / 57.7% |
| `/docs/api/` | 10,453 / 2,759 / 2,420 | 31,961 / 6,486 / 5,777 | 67.3% / 57.5% / 58.1% |
| `/changelog/` | 9,486 / 2,616 / 2,280 | 31,961 / 6,486 / 5,777 | 70.3% / 59.7% / 60.5% |
| `/blog/` | 9,209 / 2,586 / 2,251 | 31,961 / 6,486 / 5,777 | 71.2% / 60.1% / 61.0% |
| `/dashboard/` | 12,282 / 3,155 / 2,775 | 31,961 / 6,486 / 5,777 | 61.6% / 51.4% / 52.0% |
| `/settings/` | 16,479 / 3,842 / 3,367 | 31,961 / 6,486 / 5,777 | 48.4% / 40.8% / 41.7% |
| `/contact/` | 9,461 / 2,539 / 2,217 | 31,961 / 6,486 / 5,777 | 70.4% / 60.9% / 61.6% |
| `/404/` | 5,852 / 1,795 / 1,557 | 31,961 / 6,486 / 5,777 | 81.7% / 72.3% / 73.0% |

| Page | JS, default | JS, control | smaller | its behaviours (the script without its mount calls) | the control's behaviours (without its table) | smaller |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 252 / 176 / 126 | 4,277 / 1,703 / 1,507 | 94.1% / 89.7% / 91.6% | 248 / 173 / 122 | 3,587 / 1,412 / 1,210 | 93.1% / 87.7% / 89.9% |
| `/pricing/` | 911 / 506 / 404 | 4,277 / 1,703 / 1,507 | 78.7% / 70.3% / 73.2% | 874 / 493 / 395 | 3,587 / 1,412 / 1,210 | 75.6% / 65.1% / 67.4% |
| `/docs/` | 1,748 / 784 / 649 | 4,277 / 1,703 / 1,507 | 59.1% / 54.0% / 56.9% | 1,407 / 729 / 600 | 3,587 / 1,412 / 1,210 | 60.8% / 48.4% / 50.4% |
| `/docs/api/` | 661 / 386 / 307 | 4,277 / 1,703 / 1,507 | 84.5% / 77.3% / 79.6% | 608 / 353 / 293 | 3,587 / 1,412 / 1,210 | 83.0% / 75.0% / 75.8% |
| `/changelog/` | 252 / 176 / 126 | 4,277 / 1,703 / 1,507 | 94.1% / 89.7% / 91.6% | 248 / 173 / 122 | 3,587 / 1,412 / 1,210 | 93.1% / 87.7% / 89.9% |
| `/blog/` | 252 / 176 / 126 | 4,277 / 1,703 / 1,507 | 94.1% / 89.7% / 91.6% | 248 / 173 / 122 | 3,587 / 1,412 / 1,210 | 93.1% / 87.7% / 89.9% |
| `/dashboard/` | 1,443 / 707 / 588 | 4,277 / 1,703 / 1,507 | 66.3% / 58.5% / 61.0% | 1,387 / 682 / 562 | 3,587 / 1,412 / 1,210 | 61.3% / 51.7% / 53.6% |
| `/settings/` | 2,448 / 1,022 / 876 | 4,277 / 1,703 / 1,507 | 42.8% / 40.0% / 41.9% | 2,222 / 962 / 815 | 3,587 / 1,412 / 1,210 | 38.1% / 31.9% / 32.6% |
| `/contact/` | 252 / 176 / 126 | 4,277 / 1,703 / 1,507 | 94.1% / 89.7% / 91.6% | 248 / 173 / 122 | 3,587 / 1,412 / 1,210 | 93.1% / 87.7% / 89.9% |
| `/404/` | 252 / 176 / 126 | 4,277 / 1,703 / 1,507 | 94.1% / 89.7% / 91.6% | 248 / 173 / 122 | 3,587 / 1,412 / 1,210 | 93.1% / 87.7% / 89.9% |

CSS and JS together, and what that is of the page — its HTML, CSS and JS, each compressed apart:

| Page | CSS + JS, default | CSS + JS, control | smaller | the page, default | the page, control | smaller |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 9,068 / 2,651 / 2,276 | 36,238 / 8,189 / 7,284 | 75.0% / 67.6% / 68.8% | 14,978 / 4,747 / 3,903 | 42,148 / 10,285 / 8,911 | 64.5% / 53.8% / 56.2% |
| `/pricing/` | 12,250 / 3,477 / 3,002 | 36,238 / 8,189 / 7,284 | 66.2% / 57.5% / 58.8% | 19,652 / 5,574 / 4,630 | 43,640 / 10,286 / 8,912 | 55.0% / 45.8% / 48.0% |
| `/docs/` | 12,376 / 3,586 / 3,095 | 36,238 / 8,189 / 7,284 | 65.8% / 56.2% / 57.5% | 20,576 / 6,007 / 5,052 | 44,438 / 10,610 / 9,241 | 53.7% / 43.4% / 45.3% |
| `/docs/api/` | 11,114 / 3,145 / 2,727 | 36,238 / 8,189 / 7,284 | 69.3% / 61.6% / 62.6% | 18,882 / 5,565 / 4,629 | 44,006 / 10,609 / 9,186 | 57.1% / 47.5% / 49.6% |
| `/changelog/` | 9,738 / 2,792 / 2,406 | 36,238 / 8,189 / 7,284 | 73.1% / 65.9% / 67.0% | 15,434 / 4,677 / 3,887 | 41,934 / 10,074 / 8,765 | 63.2% / 53.6% / 55.7% |
| `/blog/` | 9,461 / 2,762 / 2,377 | 36,238 / 8,189 / 7,284 | 73.9% / 66.3% / 67.4% | 14,688 / 4,928 / 4,071 | 41,465 / 10,355 / 8,978 | 64.6% / 52.4% / 54.7% |
| `/dashboard/` | 13,725 / 3,862 / 3,363 | 36,238 / 8,189 / 7,284 | 62.1% / 52.8% / 53.8% | 20,715 / 5,936 / 5,041 | 43,228 / 10,263 / 8,962 | 52.1% / 42.2% / 43.8% |
| `/settings/` | 18,927 / 4,864 / 4,243 | 36,238 / 8,189 / 7,284 | 47.8% / 40.6% / 41.7% | 28,317 / 7,561 / 6,388 | 45,628 / 10,886 / 9,429 | 37.9% / 30.5% / 32.3% |
| `/contact/` | 9,713 / 2,715 / 2,343 | 36,238 / 8,189 / 7,284 | 73.2% / 66.8% / 67.8% | 14,234 / 4,324 / 3,555 | 40,759 / 9,798 / 8,496 | 65.1% / 55.9% / 58.2% |
| `/404/` | 6,104 / 1,971 / 1,683 | 36,238 / 8,189 / 7,284 | 83.2% / 75.9% / 76.9% | 8,567 / 2,896 / 2,372 | 38,701 / 9,114 / 7,973 | 77.9% / 68.2% / 70.2% |
| **mean** | 11,248 / 3,183 / 2,752 | 36,238 / 8,189 / 7,284 | 69.0% / 61.1% / 62.2% | 17,604 / 5,222 / 4,353 | 42,595 / 10,228 / 8,885 | 58.7% / 48.9% / 51.0% |

How much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.

| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's 313 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 83 | 47 | 36 | 3,640 | 232 |
| `/pricing/` | 104 | 47 | 57 | 6,127 | 211 |
| `/docs/` | 99 | 47 | 52 | 5,476 | 216 |
| `/docs/api/` | 95 | 47 | 48 | 5,309 | 219 |
| `/changelog/` | 83 | 47 | 36 | 4,356 | 232 |
| `/blog/` | 81 | 47 | 34 | 4,027 | 234 |
| `/dashboard/` | 110 | 47 | 63 | 7,162 | 205 |
| `/settings/` | 148 | 47 | 101 | 10,982 | 167 |
| `/contact/` | 88 | 47 | 41 | 4,306 | 227 |
| `/404/` | 55 | 47 | 8 | 834 | 260 |

47 of the control's 313 are on all 10 pages (≈ 4,712 B of selectors and declarations): the layout's — the side menu, the links menu, the button — and what of the site's own sheet every page has. 16 are on no page (≈ 1,221 B) — options and parts of the catalog that this site does not use: `:is(.rg-menu>:is(a,button),.rg-menu>li>a)[aria-current]`, `.rg-sidemenu>:is(section,details) ul ul`, `.rg-sidemenu>:is(section,details) li>details>summary`, `.rg-sidemenu>:is(section,details) summary`, `.rg-sidemenu:not(:popover-open):has(dialog:modal)`, `.bc-accordion[data-variant=flush]`, `.bc-accordion[data-variant=flush]>details`, `.bc-avatar[data-shape=square]`, `.bc-card>[data-part=media]`, `.bc-card>[data-part=media]>:is(img,svg)`, `.bc-pagination[data-variant=compact]>ul`, `.bc-progress[data-tone=success]`, `.bc-select[data-size=sm]>[data-part=control]>select`, `.bc-toast[data-tone=danger]`, `:is(main,main>section,main>article)>h3`, `main kbd`. 1 is on pages only with fewer declarations than the control has: `:root` — the tokens a page does not read are dropped (the table of *The pages*).

Rules for a state attribute — `aria-*`, `disabled`, `hidden`, … — that no element of the page has as it loads (runtime state is "maybe": builder.md, *CSS*; decisions.md, M — a behaviour of the page may write it, or nothing can): `/` 2 (≈ 229 B), `/pricing/` 2 (≈ 229 B), `/docs/` 2 (≈ 229 B), `/docs/api/` 2 (≈ 229 B), `/changelog/` 0 (≈ 0 B), `/blog/` 2 (≈ 229 B), `/dashboard/` 2 (≈ 229 B), `/settings/` 3 (≈ 336 B), `/contact/` 2 (≈ 229 B), `/404/` 2 (≈ 229 B).

## The session

The ten pages in order, with a warm HTTP cache — each URL fetched once — as each build delivers them (`measure.mjs`). The running total after each page, in brotli bytes:

| After | default | control | `--inline never` | the lighter of default and control |
| --- | ---: | ---: | ---: | --- |
| 1: `/` | 4,202 | 9,266 | 4,258 | default, by 54.7% |
| 2: `/pricing/` | 8,794 | 10,934 | 8,933 | default, by 19.6% |
| 3: `/docs/` | 13,825 | 12,930 | 14,028 | control, by 6.5% |
| 4: `/docs/api/` | 18,434 | 14,878 | 18,706 | control, by 19.3% |
| 5: `/changelog/` | 22,294 | 16,403 | 22,514 | control, by 26.4% |
| 6: `/blog/` | 26,344 | 18,140 | 26,506 | control, by 31.1% |
| 7: `/dashboard/` | 31,330 | 19,859 | 31,587 | control, by 36.6% |
| 8: `/settings/` | 37,680 | 22,049 | 38,021 | control, by 41.5% |
| 9: `/contact/` | 41,192 | 23,301 | 41,491 | control, by 43.4% |
| 10: `/404/` | 43,505 | 24,034 | 43,782 | control, by 44.8% |

| Build | Requests per page, cold | Page, cold: mean total | Session of 10 pages, warm cache: requests | … total |
| --- | ---: | ---: | ---: | ---: |
| `default` | 3, 2 | 17,846 / 5,310 / 4,474 | 12 | 176,897 / 51,637 / 43,505 |
| `control` | 5, 4 | 42,903 / 10,466 / 9,082 | 14 | 101,319 / 29,505 / 24,034 |
| `never` | 5, 4 | 17,912 / 5,462 / 4,552 | 28 | 176,549 / 52,456 / 43,782 |

Cold, a page of the default build is 50.7% smaller than the control's (brotli, mean of 10). Over the session the control is **42.7% / 42.9% / 44.8% smaller** than the default build (raw / gzip / brotli). The control's one sheet (31,961 B) is a file, fetched once, its one script (4,277 B) a file, fetched once; the default build's 10 distinct sheets — 8,816, 11,339, 10,628, 10,453, 9,486, 9,209, 12,282, 16,479, 9,461, 5,852 B — and 6 distinct scripts are each inlined in their page.
The control is ahead from page 3 of the visit on.

## What each page's script is (T1)

The default build. The rows are `_rg/report.json`'s (`modules`: from esbuild's metafile).

| Page | Script, raw | Rows of the report | Sum | Mounts |
| --- | ---: | --- | ---: | --- |
| `/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/pricing/` | 911 | 248 `overlays.ts` + 626 `tabs.ts` + 37 `<entry>` | 911 | `overlays`, `tabs` on `#t1` `RG_TABS_HASH=false` |
| `/docs/` | 1,748 | 248 `overlays.ts` + 799 `tabs.ts` + 360 `copy.ts` + 341 `<entry>` | 1,748 | `overlays`, `tabs` on `#pm` `RG_TABS_HASH=true` with `{"hash":true}`, `copy` on `#c1` with `{"done":"Copied"}`, `copy` on `#c2` with `{"done":"Copied"}`, `copy` on `#c3` with `{"done":"Copied"}`, `copy` on `#c4` with `{"done":"Copied"}`, `copy` on `#c5` with `{"done":"Copied"}`, `copy` on `#c7` with `{"done":"Copied"}` |
| `/docs/api/` | 661 | 248 `overlays.ts` + 360 `copy.ts` + 53 `<entry>` | 661 | `overlays`, `copy` on `#c1` with `{"done":"Copied"}` |
| `/changelog/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/blog/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/dashboard/` | 1,443 | 248 `overlays.ts` + 832 `menu-keys.ts` + 307 `invokers.ts` + 56 `<entry>` | 1,443 | `overlays`, `menu-keys` on `#m2` `RG_MENU_TYPEAHEAD=true` with `{"typeahead":true}`, `invokers` |
| `/settings/` | 2,448 | 248 `overlays.ts` + 626 `tabs.ts` + 576 `field.ts` + 465 `toast.ts` + 307 `invokers.ts` + 226 `<entry>` | 2,448 | `overlays`, `tabs` on `#t1` `RG_TABS_HASH=false`, `field` on `#f4` `RG_FIELD_COUNT=true` `RG_FIELD_REVEAL=false` with `{"count":true}`, `toast` on `#toast1` with `{"timeout":4000}`, `field` on `#f5` `RG_FIELD_COUNT=false` `RG_FIELD_REVEAL=true` with `{"reveal":true}`, `field` on `#f6` `RG_FIELD_COUNT=false` `RG_FIELD_REVEAL=true` with `{"reveal":true}`, `invokers` |
| `/contact/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |
| `/404/` | 252 | 248 `overlays.ts` + 4 `<entry>` | 252 | `overlays` |

Checked on every page, by reading the script and the document:

- the rows add up to the script: yes
- no <runtime> row: yes
- every row is a mounted behaviour or the entry: yes
- the script's only statements that run are the entry's mount calls: yes
- one <script> in the document, the builder's: yes
- no handler attribute, no javascript: URL: yes
- nothing of React or of a runtime in the script: yes

The 6 distinct scripts of the site, whole:

`/`, `/changelog/`, `/blog/`, `/contact/`, `/404/` — 252 B:

```js
function o(){addEventListener("pagehide",n),globalThis.navigation?.addEventListener("navigate",n)}function n(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}o();
```

`/pricing/` — 911 B:

```js
function a(){addEventListener("pagehide",l),globalThis.navigation?.addEventListener("navigate",l)}function l(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}function s(e,n){let t=e.firstElementChild;t.addEventListener("click",f),t.addEventListener("keydown",m)}function d(e){return[...e.children]}function c(e,n){for(let t of d(e)){let o=t===n;t.setAttribute("aria-selected",o?"true":"false"),t.tabIndex=o?0:-1,document.getElementById(t.getAttribute("aria-controls")).hidden=!o}}function f(e){let n=e.currentTarget,t=e.target.closest("[role=tab]");t&&t.parentElement===n&&c(n,t)}function m(e){let n=e.currentTarget,t=d(n),o=t.indexOf(e.target),i={ArrowRight:o+1,ArrowLeft:o-1,Home:0,End:-1}[e.key];if(o<0||i===void 0)return;e.preventDefault();let r=t.at(i%t.length);r.focus(),c(n,r)}a();s(document.getElementById("t1"));
```

`/docs/` — 1,748 B:

```js
function i(){addEventListener("pagehide",u),globalThis.navigation?.addEventListener("navigate",u)}function u(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}var d=[];function r(e,n){let t=e.firstElementChild;t.addEventListener("click",g),t.addEventListener("keydown",p),n?.hash&&(d.length||addEventListener("hashchange",T),d.push(t),f(t))}function c(e){return[...e.children]}function s(e,n){for(let t of c(e)){let o=t===n;t.setAttribute("aria-selected",o?"true":"false"),t.tabIndex=o?0:-1,document.getElementById(t.getAttribute("aria-controls")).hidden=!o}}function g(e){let n=e.currentTarget,t=e.target.closest("[role=tab]");t&&t.parentElement===n&&s(n,t)}function p(e){let n=e.currentTarget,t=c(n),o=t.indexOf(e.target),l={ArrowRight:o+1,ArrowLeft:o-1,Home:0,End:-1}[e.key];if(o<0||l===void 0)return;e.preventDefault();let m=t.at(l%t.length);m.focus(),s(n,m)}function T(){d.forEach(f)}function f(e){let n=c(e).find(t=>"#"+t.id===location.hash);n&&s(e,n)}var E=new WeakMap;function a(e,n){E.set(e,n.done),e.addEventListener("click",h)}function h(e){let n=e.currentTarget,t=e.target.closest("[data-part=copy]");if(!t)return;let o=t.textContent;navigator.clipboard.writeText(n.querySelector("code").textContent).then(()=>{t.textContent=E.get(n),setTimeout(v,2e3,t,o)},H)}function v(e,n){e.textContent=n}function H(){}i();r(document.getElementById("pm"),{hash:!0});a(document.getElementById("c1"),{done:"Copied"});a(document.getElementById("c2"),{done:"Copied"});a(document.getElementById("c3"),{done:"Copied"});a(document.getElementById("c4"),{done:"Copied"});a(document.getElementById("c5"),{done:"Copied"});a(document.getElementById("c7"),{done:"Copied"});
```

`/docs/api/` — 661 B:

```js
function n(){addEventListener("pagehide",r),globalThis.navigation?.addEventListener("navigate",r)}function r(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}var a=new WeakMap;function i(e,t){a.set(e,t.done),e.addEventListener("click",c)}function c(e){let t=e.currentTarget,o=e.target.closest("[data-part=copy]");if(!o)return;let l=o.textContent;navigator.clipboard.writeText(t.querySelector("code").textContent).then(()=>{o.textContent=a.get(t),setTimeout(d,2e3,o,l)},s)}function d(e,t){e.textContent=t}function s(){}n();i(document.getElementById("c1"),{done:"Copied"});
```

`/dashboard/` — 1,443 B:

```js
function i(){addEventListener("pagehide",d),globalThis.navigation?.addEventListener("navigate",d)}function d(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}function r(e,t){e.addEventListener("keydown",u),e.addEventListener("click",f),t?.typeahead&&e.addEventListener("keydown",E)}var s="[role=menuitem]:not(:disabled, [aria-disabled=true])";function c(e){return[...e.querySelectorAll(s)]}function m(e){e.hidePopover(),document.getElementById(e.getAttribute("aria-labelledby"))?.focus()}function u(e){let t=e.currentTarget,o=c(t),n=o.indexOf(e.target),a={ArrowDown:n+1,ArrowUp:n-1,Home:0,End:-1}[e.key];e.key==="Tab"?m(t):a!==void 0&&(e.preventDefault(),o.at(a%o.length)?.focus())}function f(e){e.target.closest(s)&&m(e.currentTarget)}function E(e){let t=e.key.toLowerCase();if(t.length!==1||t===" "||e.ctrlKey||e.metaKey||e.altKey)return;let o=c(e.currentTarget),n=o.indexOf(e.target);[...o.slice(n+1),...o.slice(0,n+1)].find(a=>a.textContent.trim().toLowerCase().startsWith(t))?.focus()}function l(){"command"in HTMLButtonElement.prototype||addEventListener("click",y)}function y(e){let t=e.target.closest("button[commandfor]"),o=t&&document.getElementById(t.getAttribute("commandfor"));if(!o)return;let n=t.getAttribute("command");n==="show-modal"?o.open||o.showModal():n==="close"&&o.close()}i();r(document.getElementById("m2"),{typeahead:!0});l();
```

`/settings/` — 2,448 B:

```js
function r(){addEventListener("pagehide",f),globalThis.navigation?.addEventListener("navigate",f)}function f(){for(let e of document.querySelectorAll(":popover-open"))e.hidePopover();for(let e of document.querySelectorAll("dialog[open]"))e.close()}function l(e,t){let n=e.firstElementChild;n.addEventListener("click",H),n.addEventListener("keydown",h)}function E(e){return[...e.children]}function T(e,t){for(let n of E(e)){let o=n===t;n.setAttribute("aria-selected",o?"true":"false"),n.tabIndex=o?0:-1,document.getElementById(n.getAttribute("aria-controls")).hidden=!o}}function H(e){let t=e.currentTarget,n=e.target.closest("[role=tab]");n&&n.parentElement===t&&T(t,n)}function h(e){let t=e.currentTarget,n=E(t),o=n.indexOf(e.target),u={ArrowRight:o+1,ArrowLeft:o-1,Home:0,End:-1}[e.key];if(o<0||u===void 0)return;e.preventDefault();let m=n.at(u%n.length);m.focus(),T(t,m)}function a(e,t){t?.count&&(e.addEventListener("input",M),L(e)),t?.reveal&&e.addEventListener("click",b)}function v(e){return e.querySelector("input, textarea")}function L(e){let t=v(e),n=e.querySelector("output");n.textContent=t.value.length+" / "+t.maxLength,n.toggleAttribute("data-full",t.value.length>=t.maxLength)}function M(e){L(e.currentTarget)}function b(e){let t=e.target.closest("[data-part=reveal]");if(!t)return;let n=v(e.currentTarget),o=n.type==="password";n.type=o?"text":"password",t.setAttribute("aria-pressed",o?"true":"false"),t.textContent=o?"Hide":"Show"}var p=new WeakMap,g=new WeakMap;function c(e,t){g.set(e,t.timeout),e.addEventListener("toggle",y),e.addEventListener("pointerenter",i),e.addEventListener("focusin",i),e.addEventListener("pointerleave",s),e.addEventListener("focusout",s)}function y(e){e.newState==="open"?s(e):i(e)}function i(e){clearTimeout(p.get(e.currentTarget))}function s(e){let t=e.currentTarget;i(e),t.matches(":popover-open")&&p.set(t,setTimeout(x,g.get(t),t))}function x(e){e.hidePopover()}function d(){"command"in HTMLButtonElement.prototype||addEventListener("click",A)}function A(e){let t=e.target.closest("button[commandfor]"),n=t&&document.getElementById(t.getAttribute("commandfor"));if(!n)return;let o=t.getAttribute("command");o==="show-modal"?n.open||n.showModal():o==="close"&&n.close()}r();l(document.getElementById("t1"));a(document.getElementById("f4"),{count:!0});c(document.getElementById("toast1"),{timeout:4e3});a(document.getElementById("f5"),{reveal:!0});a(document.getElementById("f6"),{reveal:!0});d();
```

## The site's source (T6)

17 files under `bench/catalog-site/` (without `node_modules/`): `README.md`, `layout.rtsx`, `package.json`, `pages/404/index.rtsx`, `pages/blog/index.rtsx`, `pages/changelog/index.rtsx`, `pages/contact/index.rtsx`, `pages/dashboard/index.rtsx`, `pages/docs/api/index.rtsx`, `pages/docs/index.rtsx`, `pages/index.rtsx`, `pages/pricing/index.rtsx`, `pages/settings/index.rtsx`, `public/avatars/mira.svg`, `public/favicon.svg`, `site.css`, `tsconfig.json`.

- no file of JS or TS: every source file is .rtsx, .css, .json, .md or .svg: yes
- no <script>: yes
- no <style>, no <link rel=stylesheet>, no style attribute: yes
- no event handler: yes
- no `mount(`, no import of a behaviour: yes
- no HTML written as a string: yes
- one stylesheet imported, once, by the layout: no list per page: yes
- the stylesheet imports of the whole site: `layout.rtsx:7`

The catalog itself is the other side of the rule: its components import their CSS and call `mount()`, as a design system does (`bench/catalog/src`).

## One component deleted from one page (T4's question)

Under `--inline always`. T4 itself is the docs site's (`bench/delta.mjs`); this asks the same of the catalog, twice.

| Deleted | Markup | CSS | Selectors that left | … that do not name the component | Rewritten | Script | Behaviours that left | Other pages changed | |
| --- | ---: | ---: | ---: | ---: | --- | --- | --- | ---: | --- |
| the FAQ `Accordion` of `/pricing/` | −1,115 B, one span | −1,127 B | 12 | 0 | — | 911 → 911 B | — | 0 | **pass** |
| the `Toast` of `/settings/` — with its trigger, *Save changes* | −387 B, one span | −1,161 B | 8 | 0 | `:root` | 2,448 → 1,932 B | `toast` | 0 | **pass** |

"Rewritten" is a rule that stays with fewer declarations: `:root`, less the tokens only the deleted component read.

## T5, page by page

plan.md, RGP2-050: against the control, **in brotli bytes** — what a page transfers — per-page CSS ≥ 20% smaller on at least half of the pages (5 of 10), JS ≥ 30% smaller on every page that ships a script (all 10 do: the layout mounts `overlays`). Raw and gzip beside.

| Page | CSS smaller, brotli | ≥ 20% | gzip | raw | JS smaller, brotli | ≥ 30% | gzip | raw | JS, behaviours alone, brotli | ≥ 30% |
| --- | ---: | --- | ---: | ---: | ---: | --- | ---: | ---: | ---: | --- |
| `/` | 62.8% | yes | 61.8% | 72.4% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/pricing/` | 55.0% | yes | 54.2% | 64.5% | 73.2% | yes | 70.3% | 78.7% | 67.4% | yes |
| `/docs/` | 57.7% | yes | 56.8% | 66.7% | 56.9% | yes | 54.0% | 59.1% | 50.4% | yes |
| `/docs/api/` | 58.1% | yes | 57.5% | 67.3% | 79.6% | yes | 77.3% | 84.5% | 75.8% | yes |
| `/changelog/` | 60.5% | yes | 59.7% | 70.3% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/blog/` | 61.0% | yes | 60.1% | 71.2% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/dashboard/` | 52.0% | yes | 51.4% | 61.6% | 61.0% | yes | 58.5% | 66.3% | 53.6% | yes |
| `/settings/` | 41.7% | yes | 40.8% | 48.4% | 41.9% | yes | 40.0% | 42.8% | 32.6% | yes |
| `/contact/` | 61.6% | yes | 60.9% | 70.4% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/404/` | 73.0% | yes | 72.3% | 81.7% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |

| Unit | CSS: pages at 20% or more | JS: pages under 30% | T5 | JS, behaviours alone: pages under 30% |
| --- | ---: | --- | --- | --- |
| brotli — **the unit T5 is decided in** | 10 of 10 | none | **pass** | none |
| gzip | 10 of 10 | none | would pass | none |
| raw | 10 of 10 | none | would pass | none |

## Thresholds, on the catalog

plan.md, RGP2-050, as each can be read on this fixture. The thresholds were set for the docs site; where one names it, the row says how it is read here.

| | Threshold | Measured | Verdict |
| --- | --- | --- | --- |
| T1 (refutes) | 0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, and there is no `<runtime>` row | 10 of 10 pages: the rows add up to the script (252, 911, 1,748, 661, 252, 252, 1,443, 2,448, 252, 252 B), no `<runtime>` row, and the only statements that run are the mount calls | **pass** |
| T2 (refutes) | JS on the heaviest page. The budget of plan.md — ≤ 1.5 KB raw — was set for the docs site's three behaviours, and is not carried over to a page that mounts more: read here as its bound alone, **refuted above 5 KB brotli** ("a micro-runtime, not compilation"); the heaviest page is reported | `/settings/`: 2,448 / 1,022 / 876 B, 5 behaviours. Under 1.5 KB raw: 8 of 10 pages. Not judged — the docs site's budget per behaviour mounted is 500 B (1.5 KB for three): `/settings/` is at 490 B, and the most is `/docs/`, 583 B (3 behaviours, 341 B of mount calls) | **pass** |
| T3 | ≥ 100× below the best React build of an equivalent site | no React build of this fixture exists | not measured here |
| T4 (refutes) | deleting one component from one page removes its markup, the CSS rules only it matched and the behaviours only it mounted, and nothing else; every other page the same bytes (T4 itself names the docs site's *Install* dialog: `bench/delta.mjs`) | the FAQ `Accordion` of `/pricing/`: −1,115 B of markup, 12 selectors (0 foreign), no behaviour left, 0 other pages changed; the `Toast` of `/settings/` — with its trigger, *Save changes*: −387 B of markup, 8 selectors (0 foreign), `toast` left, 0 other pages changed | **pass** |
| T5 | against the control, in brotli bytes: per-page CSS ≥ 20% smaller on at least half of the pages (5 of 10), JS ≥ 30% smaller on every page that ships a script | CSS: 10 of 10 pages at 20% or more (41.7% to 73.0%). JS: every page at 30% or more (41.9% to 91.6%); against the control's behaviours alone, every page at 30% or more too. In gzip it holds, raw it holds | **pass** |
| T6 (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 17 source files, 7 greps: nothing found; one stylesheet import, in `layout.rtsx` | **pass** |
| T7 (refutes) | the browser checks pass on the built site | `node bench/catalog-verify.mjs` | not measured here |
| T8 | ≤ 3 requests per page, cold | 3, 2 per page (the document, and `/avatars/mira.svg` on `/`, `/favicon.svg`); the control: 5, 4; `--inline never`: 5, 4 | **pass** |

Cross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one `_rg/report.json` has, in the three builds; `--inline never` changes no byte of what a page is; the control's HTML is the default build's, and its sheet and script are one each.
