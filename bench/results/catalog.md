# The catalog site, measured

Written by `node bench/catalog.mjs` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). **A fixture**: `bench/catalog-site`, 10 pages, on `bench/catalog`, 20 components — neither is the design system or a product (their READMEs). Three builds, each loaded cold in headless Chrome by `bench/measure.mjs`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.

`reactogenic check` on the site: no diagnostic. The three builds: no diagnostic.

## The catalog

What each component weighs. CSS: its file as written (with comments); as the builder bundles it (the report's `bytesIn`: nesting lowered, no comments, not minified); and minified — the selectors and declarations of the control's sheet that name its root class. "On" is the pages whose sheet keeps a rule of it.

| Component | From | CSS, written | bundled | minified ≈ | Behaviours it mounts | On |
| --- | --- | ---: | ---: | ---: | --- | ---: |
| `Button` | `@reactogenic/ui` | 680 | 605 | 443 | — | 10 |
| `Dialog` | `@reactogenic/ui` | 3,817 | 2,392 | 1,667 | `overlays`, `invokers` | 2 |
| `DropdownMenu` | `@reactogenic/ui` | 3,560 | 1,739 | 1,408 | `overlays`, `menu-keys`, `invokers` | 10 |
| `SideMenu` | `@reactogenic/ui` | 5,322 | 3,523 | 2,460 | `overlays` | 10 |
| `Accordion` | the fixture | 1,628 | 1,645 | 1,176 | — | 1 |
| `Avatar` | the fixture | 964 | 903 | 609 | — | 3 |
| `Badge` | the fixture | 1,315 | 1,228 | 951 | — | 6 |
| `Breadcrumbs` | the fixture | 816 | 748 | 513 | — | 2 |
| `Callout` | the fixture | 1,610 | 1,592 | 1,205 | — | 4 |
| `Card` | the fixture | 1,842 | 1,766 | 1,238 | — | 3 |
| `Checkbox` | the fixture | 2,174 | 2,033 | 1,446 | — | 2 |
| `CodeBlock` | the fixture | 1,918 | 1,820 | 1,367 | `copy` | 3 |
| `Field` | the fixture | 2,791 | 2,610 | 2,011 | `field` | 2 |
| `Pagination` | the fixture | 1,484 | 1,382 | 1,029 | — | 1 |
| `Progress` | the fixture | 2,290 | 2,141 | 1,529 | — | 1 |
| `Select` | the fixture | 1,582 | 1,517 | 1,133 | — | 2 |
| `Table` | the fixture | 2,013 | 1,792 | 1,296 | — | 2 |
| `Tabs` | the fixture | 1,441 | 1,438 | 1,086 | `tabs` | 3 |
| `Toast` | the fixture | 1,698 | 1,505 | 1,042 | `overlays`, `toast` | 1 |
| `Tooltip` | the fixture | 1,818 | 1,514 | 1,122 | — | 1 |
| tokens (2 files) | | 1,763 | 1,070 | | | |
| the site's own `site.css` | | 8,138 | 6,789 | | | |
| **the twenty** | | **40,763** | **33,893** | **24,731** | | |

The sixteen of the fixture are 18,753 B minified, 1,172 B each on average; the four of `@reactogenic/ui`, 5,978 B, 1,495 B each. Components no page uses: none.

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
| `/` | 3 | 14,962 / 4,650 / 3,861 | 0 / 0 / 0 | 0 / 0 / 0 | 394 / 346 / 308 | 15,356 / 4,996 / 4,169 | 252 / 8,798 / 0 | 252 |
| `/pricing/` | 2 | 19,600 / 5,439 / 4,568 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 19,774 / 5,601 / 4,705 | 911 / 11,299 / 0 | 911 |
| `/docs/` | 2 | 20,606 / 5,865 / 5,011 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 20,780 / 6,027 / 5,148 | 1,748 / 10,624 / 0 | 1,748 |
| `/docs/api/` | 2 | 18,935 / 5,423 / 4,581 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 19,109 / 5,585 / 4,718 | 661 / 10,435 / 0 | 661 |
| `/changelog/` | 2 | 15,417 / 4,571 / 3,840 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 15,591 / 4,733 / 3,977 | 252 / 9,467 / 0 | 252 |
| `/blog/` | 2 | 14,698 / 4,829 / 4,048 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 14,872 / 4,991 / 4,185 | 252 / 9,197 / 0 | 252 |
| `/dashboard/` | 2 | 20,780 / 5,787 / 4,957 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 20,954 / 5,949 / 5,094 | 1,443 / 12,271 / 0 | 1,443 |
| `/settings/` | 2 | 28,278 / 7,395 / 6,323 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 28,452 / 7,557 / 6,460 | 2,448 / 16,449 / 0 | 2,448 |
| `/contact/` | 2 | 14,257 / 4,233 / 3,509 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 14,431 / 4,395 / 3,646 | 252 / 9,453 / 0 | 252 |
| `/404/` | 2 | 8,590 / 2,795 / 2,299 | 0 / 0 / 0 | 0 / 0 / 0 | 174 / 162 / 137 | 8,764 / 2,957 / 2,436 | 252 / 5,844 / 0 | 252 |
| **session (10 pages, warm cache)** | 12 | 176,123 / 50,987 / 42,997 | 0 / 0 / 0 | 0 / 0 / 0 | 394 / 346 / 308 | 176,517 / 51,333 / 43,305 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,866 / 2,083 / 1,624 | 8,798 / 2,458 / 2,141 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/pricing/` | 7,344 / 2,086 / 1,620 | 11,299 / 2,949 / 2,581 | 911 / 506 / 404 | inline | inline |
| `/docs/` | 8,188 / 2,409 / 1,951 | 10,624 / 2,789 / 2,440 | 1,748 / 784 / 649 | inline | inline |
| `/docs/api/` | 7,793 / 2,407 / 1,902 | 10,435 / 2,737 / 2,399 | 661 / 386 / 307 | inline | inline |
| `/changelog/` | 5,652 / 1,874 / 1,478 | 9,467 / 2,600 / 2,266 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/blog/` | 5,203 / 2,156 / 1,688 | 9,197 / 2,572 / 2,246 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/dashboard/` | 7,020 / 2,064 / 1,674 | 12,271 / 3,133 / 2,756 | 1,443 / 707 / 588 | inline | inline |
| `/settings/` | 9,335 / 2,676 / 2,139 | 16,449 / 3,822 / 3,342 | 2,448 / 1,022 / 876 | inline | inline |
| `/contact/` | 4,506 / 1,601 / 1,201 | 9,453 / 2,529 / 2,213 | 252 / 176 / 126 | inline | inline, the same on 5 pages |
| `/404/` | 2,448 / 918 / 688 | 5,844 / 1,785 / 1,553 | 252 / 176 / 126 | inline | inline, the same on 5 pages |

## `control` — `reactogenic build --no-specialize`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 5 | 5,978 / 2,145 / 1,663 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 394 / 346 / 308 | 42,532 / 10,640 / 9,218 | 0 / 0 / 0 | 4,277 |
| `/pricing/` | 4 | 7,456 / 2,146 / 1,661 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 43,790 / 10,457 / 9,045 | 0 / 0 / 0 | 4,277 |
| `/docs/` | 4 | 8,300 / 2,471 / 1,991 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 44,634 / 10,782 / 9,375 | 0 / 0 / 0 | 4,277 |
| `/docs/api/` | 4 | 7,905 / 2,463 / 1,945 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 44,239 / 10,774 / 9,329 | 0 / 0 / 0 | 4,277 |
| `/changelog/` | 4 | 5,764 / 1,931 / 1,521 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 42,098 / 10,242 / 8,905 | 0 / 0 / 0 | 4,277 |
| `/blog/` | 4 | 5,315 / 2,217 / 1,736 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 41,649 / 10,528 / 9,120 | 0 / 0 / 0 | 4,277 |
| `/dashboard/` | 4 | 7,132 / 2,120 / 1,715 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 43,466 / 10,431 / 9,099 | 0 / 0 / 0 | 4,277 |
| `/settings/` | 4 | 9,447 / 2,738 / 2,181 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 45,781 / 11,049 / 9,565 | 0 / 0 / 0 | 4,277 |
| `/contact/` | 4 | 4,618 / 1,661 / 1,248 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 40,952 / 9,972 / 8,632 | 0 / 0 / 0 | 4,277 |
| `/404/` | 4 | 2,560 / 976 / 737 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 174 / 162 / 137 | 38,894 / 9,287 / 8,121 | 0 / 0 / 0 | 4,277 |
| **session (10 pages, warm cache)** | 14 | 64,475 / 20,868 / 16,398 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | 394 / 346 / 308 | 101,029 / 29,363 / 23,953 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,866 / 2,083 / 1,624 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/pricing/` | 7,344 / 2,086 / 1,620 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/docs/` | 8,188 / 2,409 / 1,951 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/docs/api/` | 7,793 / 2,407 / 1,902 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/changelog/` | 5,652 / 1,874 / 1,478 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/blog/` | 5,203 / 2,156 / 1,688 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/dashboard/` | 7,020 / 2,064 / 1,674 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/settings/` | 9,335 / 2,676 / 2,139 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/contact/` | 4,506 / 1,601 / 1,201 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |
| `/404/` | 2,448 / 918 / 688 | 31,883 / 6,446 / 5,740 | 4,277 / 1,703 / 1,507 | file, 10 pages | file, 10 pages |

The control's script, split: its behaviours — every module any page mounts, every flag on — are 3,587 / 1,412 / 1,210; its own cost, the list of modules, the table from pathname to mounts and the loop that reads it, is 690 / 364 / 330 (compressed apart; the script as a whole is 4,277 / 1,703 / 1,507):

```js
var k=[s,l,c,u,E,p,L],N={"/":[[0]],"/404/":[[0]],"/blog/":[[0]],"/changelog/":[[0]],"/contact/":[[0]],"/dashboard/":[[0],[1,"m2",{typeahead:!0}],[2]],"/docs/":[[0],[3,"pm",{hash:!0}],[4,"c1",{done:"Copied"}],[4,"c2",{done:"Copied"}],[4,"c3",{done:"Copied"}],[4,"c4",{done:"Copied"}],[4,"c5",{done:"Copied"}],[4,"c7",{done:"Copied"}]],"/docs/api/":[[0],[4,"c1",{done:"Copied"}]],"/pricing/":[[0],[3,"t1"]],"/settings/":[[0],[3,"t1"],[5,"f4",{count:!0}],[6,"toast1",{timeout:4e3}],[5,"f5",{reveal:!0}],[5,"f6",{reveal:!0}],[2]]};for(let[e,t,n]of N[decodeURIComponent(location.pathname).replace(/(\/index\.html|\/|(\.html))?$/,(o,a,i)=>i||"/")]||[])t?k[e](document.getElementById(t),n):k[e]();
```

## `never` — `reactogenic build --inline never`

As delivered. Loaded with headless Chrome, cold cache. Bytes are raw / gzip -9 / brotli -q 11.

| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `/` | 5 | 5,978 / 2,145 / 1,667 | 8,798 / 2,458 / 2,141 | 252 / 176 / 126 | 394 / 346 / 308 | 15,422 / 5,125 / 4,242 | 0 / 0 / 0 | 252 |
| `/pricing/` | 4 | 7,456 / 2,148 / 1,664 | 11,299 / 2,949 / 2,581 | 911 / 506 / 404 | 174 / 162 / 137 | 19,840 / 5,765 / 4,786 | 0 / 0 / 0 | 911 |
| `/docs/` | 4 | 8,300 / 2,471 / 1,992 | 10,624 / 2,789 / 2,440 | 1,748 / 784 / 649 | 174 / 162 / 137 | 20,846 / 6,206 / 5,218 | 0 / 0 / 0 | 1,748 |
| `/docs/api/` | 4 | 7,905 / 2,466 / 1,945 | 10,435 / 2,737 / 2,399 | 661 / 386 / 307 | 174 / 162 / 137 | 19,175 / 5,751 / 4,788 | 0 / 0 / 0 | 661 |
| `/changelog/` | 4 | 5,764 / 1,932 / 1,522 | 9,467 / 2,600 / 2,266 | 252 / 176 / 126 | 174 / 162 / 137 | 15,657 / 4,870 / 4,051 | 0 / 0 / 0 | 252 |
| `/blog/` | 4 | 5,315 / 2,217 / 1,734 | 9,197 / 2,572 / 2,246 | 252 / 176 / 126 | 174 / 162 / 137 | 14,938 / 5,127 / 4,243 | 0 / 0 / 0 | 252 |
| `/dashboard/` | 4 | 7,132 / 2,121 / 1,713 | 12,271 / 3,133 / 2,756 | 1,443 / 707 / 588 | 174 / 162 / 137 | 21,020 / 6,123 / 5,194 | 0 / 0 / 0 | 1,443 |
| `/settings/` | 4 | 9,447 / 2,738 / 2,180 | 16,449 / 3,822 / 3,342 | 2,448 / 1,022 / 876 | 174 / 162 / 137 | 28,518 / 7,744 / 6,535 | 0 / 0 / 0 | 2,448 |
| `/contact/` | 4 | 4,618 / 1,662 / 1,249 | 9,453 / 2,529 / 2,213 | 252 / 176 / 126 | 174 / 162 / 137 | 14,497 / 4,529 / 3,725 | 0 / 0 / 0 | 252 |
| `/404/` | 4 | 2,560 / 977 / 741 | 5,844 / 1,785 / 1,553 | 252 / 176 / 126 | 174 / 162 / 137 | 8,830 / 3,100 / 2,557 | 0 / 0 / 0 | 252 |
| **session (10 pages, warm cache)** | 28 | 64,475 / 20,877 / 16,407 | 103,837 / 27,374 / 23,937 | 7,463 / 3,581 / 2,950 | 394 / 346 / 308 | 176,169 / 52,178 / 43,602 | | |

What each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:

| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |
| --- | ---: | ---: | ---: | --- | --- |
| `/` | 5,866 / 2,083 / 1,624 | 8,798 / 2,458 / 2,141 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/pricing/` | 7,344 / 2,086 / 1,620 | 11,299 / 2,949 / 2,581 | 911 / 506 / 404 | file, 1 page | file, 1 page |
| `/docs/` | 8,188 / 2,409 / 1,951 | 10,624 / 2,789 / 2,440 | 1,748 / 784 / 649 | file, 1 page | file, 1 page |
| `/docs/api/` | 7,793 / 2,407 / 1,902 | 10,435 / 2,737 / 2,399 | 661 / 386 / 307 | file, 1 page | file, 1 page |
| `/changelog/` | 5,652 / 1,874 / 1,478 | 9,467 / 2,600 / 2,266 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/blog/` | 5,203 / 2,156 / 1,688 | 9,197 / 2,572 / 2,246 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/dashboard/` | 7,020 / 2,064 / 1,674 | 12,271 / 3,133 / 2,756 | 1,443 / 707 / 588 | file, 1 page | file, 1 page |
| `/settings/` | 9,335 / 2,676 / 2,139 | 16,449 / 3,822 / 3,342 | 2,448 / 1,022 / 876 | file, 1 page | file, 1 page |
| `/contact/` | 4,506 / 1,601 / 1,201 | 9,453 / 2,529 / 2,213 | 252 / 176 / 126 | file, 1 page | file, 5 pages |
| `/404/` | 2,448 / 918 / 688 | 5,844 / 1,785 / 1,553 | 252 / 176 / 126 | file, 1 page | file, 5 pages |

## Summary

Means over 10 cold page loads; brotli -q 11 unless marked raw.

| Approach | Req / page | HTML br | CSS br (ext + inline raw) | JS br (ext) | JS to parse, raw | Other br | Page total br | Session total br | Session JS br |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| default | 2 | 4,300 | 0 + 10,384 | 0 | 847 | 154 | 4,454 | 43,305 | 0 |
| control | 4 | 1,640 | 5,740 + 0 | 1,507 | 4,277 | 154 | 9,041 | 23,953 | 1,507 |
| never | 4 | 1,641 | 2,394 + 0 | 345 | 847 | 154 | 4,534 | 43,602 | 2,950 |

## The default build against the control

What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control, raw / gzip / brotli.

| Page | CSS, default | CSS, control | smaller |
| --- | ---: | ---: | ---: |
| `/` | 8,798 / 2,458 / 2,141 | 31,883 / 6,446 / 5,740 | 72.4% / 61.9% / 62.7% |
| `/pricing/` | 11,299 / 2,949 / 2,581 | 31,883 / 6,446 / 5,740 | 64.6% / 54.3% / 55.0% |
| `/docs/` | 10,624 / 2,789 / 2,440 | 31,883 / 6,446 / 5,740 | 66.7% / 56.7% / 57.5% |
| `/docs/api/` | 10,435 / 2,737 / 2,399 | 31,883 / 6,446 / 5,740 | 67.3% / 57.5% / 58.2% |
| `/changelog/` | 9,467 / 2,600 / 2,266 | 31,883 / 6,446 / 5,740 | 70.3% / 59.7% / 60.5% |
| `/blog/` | 9,197 / 2,572 / 2,246 | 31,883 / 6,446 / 5,740 | 71.2% / 60.1% / 60.9% |
| `/dashboard/` | 12,271 / 3,133 / 2,756 | 31,883 / 6,446 / 5,740 | 61.5% / 51.4% / 52.0% |
| `/settings/` | 16,449 / 3,822 / 3,342 | 31,883 / 6,446 / 5,740 | 48.4% / 40.7% / 41.8% |
| `/contact/` | 9,453 / 2,529 / 2,213 | 31,883 / 6,446 / 5,740 | 70.4% / 60.8% / 61.4% |
| `/404/` | 5,844 / 1,785 / 1,553 | 31,883 / 6,446 / 5,740 | 81.7% / 72.3% / 72.9% |

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
| `/` | 9,050 / 2,634 / 2,267 | 36,160 / 8,149 / 7,247 | 75.0% / 67.7% / 68.7% | 14,916 / 4,717 / 3,891 | 42,026 / 10,232 / 8,871 | 64.5% / 53.9% / 56.1% |
| `/pricing/` | 12,210 / 3,455 / 2,985 | 36,160 / 8,149 / 7,247 | 66.2% / 57.6% / 58.8% | 19,554 / 5,541 / 4,605 | 43,504 / 10,235 / 8,867 | 55.1% / 45.9% / 48.1% |
| `/docs/` | 12,372 / 3,573 / 3,089 | 36,160 / 8,149 / 7,247 | 65.8% / 56.2% / 57.4% | 20,560 / 5,982 / 5,040 | 44,348 / 10,558 / 9,198 | 53.6% / 43.3% / 45.2% |
| `/docs/api/` | 11,096 / 3,123 / 2,706 | 36,160 / 8,149 / 7,247 | 69.3% / 61.7% / 62.7% | 18,889 / 5,530 / 4,608 | 43,953 / 10,556 / 9,149 | 57.0% / 47.6% / 49.6% |
| `/changelog/` | 9,719 / 2,776 / 2,392 | 36,160 / 8,149 / 7,247 | 73.1% / 65.9% / 67.0% | 15,371 / 4,650 / 3,870 | 41,812 / 10,023 / 8,725 | 63.2% / 53.6% / 55.6% |
| `/blog/` | 9,449 / 2,748 / 2,372 | 36,160 / 8,149 / 7,247 | 73.9% / 66.3% / 67.3% | 14,652 / 4,904 / 4,060 | 41,363 / 10,305 / 8,935 | 64.6% / 52.4% / 54.6% |
| `/dashboard/` | 13,714 / 3,840 / 3,344 | 36,160 / 8,149 / 7,247 | 62.1% / 52.9% / 53.9% | 20,734 / 5,904 / 5,018 | 43,180 / 10,213 / 8,921 | 52.0% / 42.2% / 43.8% |
| `/settings/` | 18,897 / 4,844 / 4,218 | 36,160 / 8,149 / 7,247 | 47.7% / 40.6% / 41.8% | 28,232 / 7,520 / 6,357 | 45,495 / 10,825 / 9,386 | 37.9% / 30.5% / 32.3% |
| `/contact/` | 9,705 / 2,705 / 2,339 | 36,160 / 8,149 / 7,247 | 73.2% / 66.8% / 67.7% | 14,211 / 4,306 / 3,540 | 40,666 / 9,750 / 8,448 | 65.1% / 55.8% / 58.1% |
| `/404/` | 6,096 / 1,961 / 1,679 | 36,160 / 8,149 / 7,247 | 83.1% / 75.9% / 76.8% | 8,544 / 2,879 / 2,367 | 38,608 / 9,067 / 7,935 | 77.9% / 68.2% / 70.2% |
| **mean** | 11,231 / 3,166 / 2,739 | 36,160 / 8,149 / 7,247 | 68.9% / 61.1% / 62.2% | 17,566 / 5,193 / 4,336 | 42,496 / 10,176 / 8,844 | 58.7% / 49.0% / 51.0% |

How much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.

| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's 313 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/` | 83 | 47 | 36 | 3,630 | 232 |
| `/pricing/` | 104 | 47 | 57 | 6,095 | 211 |
| `/docs/` | 99 | 47 | 52 | 5,480 | 216 |
| `/docs/api/` | 95 | 47 | 48 | 5,299 | 219 |
| `/changelog/` | 83 | 47 | 36 | 4,345 | 232 |
| `/blog/` | 81 | 47 | 34 | 4,023 | 234 |
| `/dashboard/` | 110 | 47 | 63 | 7,159 | 205 |
| `/settings/` | 148 | 47 | 101 | 10,960 | 167 |
| `/contact/` | 88 | 47 | 41 | 4,306 | 227 |
| `/404/` | 55 | 47 | 8 | 834 | 260 |

47 of the control's 313 are on all 10 pages (≈ 4,704 B of selectors and declarations): the layout's — the side menu, the links menu, the button — and what of the site's own sheet every page has. 16 are on no page (≈ 1,215 B) — options and parts of the catalog that this site does not use: `:is(.rg-menu>:is(a,button),.rg-menu>li>a)[aria-current]`, `.rg-sidemenu>:is(section,details) ul ul`, `.rg-sidemenu>:is(section,details) li>details>summary`, `.rg-sidemenu>:is(section,details) summary`, `.rg-sidemenu:not(:popover-open):has(dialog:modal)`, `.bc-accordion.bc-accordion-flush`, `.bc-accordion.bc-accordion-flush>details`, `.bc-avatar.bc-avatar-square`, `.bc-card>[data-part=media]`, `.bc-card>[data-part=media]>:is(img,svg)`, `.bc-pagination.bc-pagination-compact>ul`, `.bc-progress.bc-progress-success`, `.bc-select.bc-select-sm>[data-part=control]>select`, `.bc-toast.bc-toast-danger`, `:is(main,main>section,main>article)>h3`, `main kbd`. 1 is on pages only with fewer declarations than the control has: `:root` — the tokens a page does not read are dropped (the table of *The pages*).

Rules for a state attribute — `aria-*`, `disabled`, `hidden`, … — that no element of the page has as it loads (runtime state is "maybe": builder.md, *CSS*; decisions.md, M — a behaviour of the page may write it, or nothing can): `/` 2 (≈ 229 B), `/pricing/` 2 (≈ 229 B), `/docs/` 2 (≈ 229 B), `/docs/api/` 2 (≈ 229 B), `/changelog/` 0 (≈ 0 B), `/blog/` 2 (≈ 229 B), `/dashboard/` 2 (≈ 229 B), `/settings/` 3 (≈ 336 B), `/contact/` 2 (≈ 229 B), `/404/` 2 (≈ 229 B).

## The session

The ten pages in order, with a warm HTTP cache — each URL fetched once — as each build delivers them (`measure.mjs`). The running total after each page, in brotli bytes:

| After | default | control | `--inline never` | the lighter of default and control |
| --- | ---: | ---: | ---: | --- |
| 1: `/` | 4,169 | 9,218 | 4,242 | default, by 54.8% |
| 2: `/pricing/` | 8,737 | 10,879 | 8,891 | default, by 19.7% |
| 3: `/docs/` | 13,748 | 12,870 | 13,972 | control, by 6.4% |
| 4: `/docs/api/` | 18,329 | 14,815 | 18,623 | control, by 19.2% |
| 5: `/changelog/` | 22,169 | 16,336 | 22,411 | control, by 26.3% |
| 6: `/blog/` | 26,217 | 18,072 | 26,391 | control, by 31.1% |
| 7: `/dashboard/` | 31,174 | 19,787 | 31,448 | control, by 36.5% |
| 8: `/settings/` | 37,497 | 21,968 | 37,846 | control, by 41.4% |
| 9: `/contact/` | 41,006 | 23,216 | 41,308 | control, by 43.4% |
| 10: `/404/` | 43,305 | 23,953 | 43,602 | control, by 44.7% |

| Build | Requests per page, cold | Page, cold: mean total | Session of 10 pages, warm cache: requests | … total |
| --- | ---: | ---: | ---: | ---: |
| `default` | 3, 2 | 17,808 / 5,279 / 4,454 | 12 | 176,517 / 51,333 / 43,305 |
| `control` | 5, 4 | 42,804 / 10,416 / 9,041 | 14 | 101,029 / 29,363 / 23,953 |
| `never` | 5, 4 | 17,874 / 5,434 / 4,534 | 28 | 176,169 / 52,178 / 43,602 |

Cold, a page of the default build is 50.7% smaller than the control's (brotli, mean of 10). Over the session the control is **42.8% / 42.8% / 44.7% smaller** than the default build (raw / gzip / brotli). The control's one sheet (31,883 B) is a file, fetched once, its one script (4,277 B) a file, fetched once; the default build's 10 distinct sheets — 8,798, 11,299, 10,624, 10,435, 9,467, 9,197, 12,271, 16,449, 9,453, 5,844 B — and 6 distinct scripts are each inlined in their page.
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

Its CSS, and `@reactogenic/ui`'s — 22 files — by the convention (specs/phase02/components.md, *CSS convention*): no `!important`: yes; the `data-*` attributes its rules select: `data-full`, `data-part` — a part, and state a behaviour writes; every option is a class of its own, through `variants()`.

## One component deleted from one page (T4's question)

Under `--inline always`. T4 itself is the docs site's (`bench/delta.mjs`); this asks the same of the catalog, twice.

| Deleted | Markup | CSS | Selectors that left | … that do not name the component | Rewritten | Script | Behaviours that left | Other pages changed | |
| --- | ---: | ---: | ---: | ---: | --- | --- | --- | ---: | --- |
| the FAQ `Accordion` of `/pricing/` | −1,115 B, one span | −1,127 B | 12 | 0 | — | 911 → 911 B | — | 0 | **pass** |
| the `Toast` of `/settings/` — with its trigger, *Save changes* | −384 B, one span | −1,159 B | 8 | 0 | `:root` | 2,448 → 1,932 B | `toast` | 0 | **pass** |

"Rewritten" is a rule that stays with fewer declarations: `:root`, less the tokens only the deleted component read.

## T5, page by page

plan.md, RGP2-050: against the control, **in brotli bytes** — what a page transfers — per-page CSS ≥ 20% smaller on at least half of the pages (5 of 10), JS ≥ 30% smaller on every page that ships a script (all 10 do: the layout mounts `overlays`). Raw and gzip beside.

| Page | CSS smaller, brotli | ≥ 20% | gzip | raw | JS smaller, brotli | ≥ 30% | gzip | raw | JS, behaviours alone, brotli | ≥ 30% |
| --- | ---: | --- | ---: | ---: | ---: | --- | ---: | ---: | ---: | --- |
| `/` | 62.7% | yes | 61.9% | 72.4% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/pricing/` | 55.0% | yes | 54.3% | 64.6% | 73.2% | yes | 70.3% | 78.7% | 67.4% | yes |
| `/docs/` | 57.5% | yes | 56.7% | 66.7% | 56.9% | yes | 54.0% | 59.1% | 50.4% | yes |
| `/docs/api/` | 58.2% | yes | 57.5% | 67.3% | 79.6% | yes | 77.3% | 84.5% | 75.8% | yes |
| `/changelog/` | 60.5% | yes | 59.7% | 70.3% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/blog/` | 60.9% | yes | 60.1% | 71.2% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/dashboard/` | 52.0% | yes | 51.4% | 61.5% | 61.0% | yes | 58.5% | 66.3% | 53.6% | yes |
| `/settings/` | 41.8% | yes | 40.7% | 48.4% | 41.9% | yes | 40.0% | 42.8% | 32.6% | yes |
| `/contact/` | 61.4% | yes | 60.8% | 70.4% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |
| `/404/` | 72.9% | yes | 72.3% | 81.7% | 91.6% | yes | 89.7% | 94.1% | 89.9% | yes |

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
| T4 (refutes) | deleting one component from one page removes its markup, the CSS rules only it matched and the behaviours only it mounted, and nothing else; every other page the same bytes (T4 itself names the docs site's *Install* dialog: `bench/delta.mjs`) | the FAQ `Accordion` of `/pricing/`: −1,115 B of markup, 12 selectors (0 foreign), no behaviour left, 0 other pages changed; the `Toast` of `/settings/` — with its trigger, *Save changes*: −384 B of markup, 8 selectors (0 foreign), `toast` left, 0 other pages changed | **pass** |
| T5 | against the control, in brotli bytes: per-page CSS ≥ 20% smaller on at least half of the pages (5 of 10), JS ≥ 30% smaller on every page that ships a script | CSS: 10 of 10 pages at 20% or more (41.8% to 72.9%). JS: every page at 30% or more (41.9% to 91.6%); against the control's behaviours alone, every page at 30% or more too. In gzip it holds, raw it holds | **pass** |
| T6 (refutes) | no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source | 17 source files, 7 greps: nothing found; one stylesheet import, in `layout.rtsx` | **pass** |
| T7 (refutes) | the browser checks pass on the built site | `node bench/catalog-verify.mjs` | not measured here |
| T8 | ≤ 3 requests per page, cold | 3, 2 per page (the document, and `/avatars/mira.svg` on `/`, `/favicon.svg`); the control: 5, 4; `--inline never`: 5, 4 | **pass** |

Cross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one `_rg/report.json` has, in the three builds; `--inline never` changes no byte of what a page is; the control's HTML is the default build's, and its sheet and script are one each.
