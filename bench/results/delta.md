# Deleting one component from one page

Written by `node bench/delta.mjs` (specs/phase02/plan.md, RGP2-050, T4). The docs site is copied, built, edited and built again; both builds are `reactogenic build --inline always`.

## T4: the *Install* dialog of `/`

Deleted from `site/pages/index.rtsx`:

```tsx
        <Dialog>
          <$Trigger>Install</$Trigger>
          <$Title>Install</$Title>
          <p>In a Vite + React project (React 19, Vite 8, Node 22 or later):</p>
          <Code>{INSTALL}</Code>
          <p>
            The packages are published under the <code>alpha</code> tag. <code>@reactogenic/cli</code> brings the{" "}
            <code>reactogenic</code> binary for macOS, Linux and Windows, arm64 and x64.
          </p>
          <$Action key="close" variant="ghost">Close</$Action>
          <$Action key="guide" href="/guide/">Getting started</$Action>
        </Dialog>
```

### The other pages

| Page | Document, before | after | The same bytes | … as the site ships (`--inline auto`) |
| --- | ---: | ---: | --- | --- |
| `/guide/` | 18,070 | 18,070 | yes: HTML, CSS and script | yes |
| `/syntax/` | 46,693 | 46,693 | yes: HTML, CSS and script | yes |
| `/reference/cli/` | 28,980 | 28,980 | yes: HTML, CSS and script | yes |

### `/`

| | Before | After | Left |
| --- | ---: | ---: | ---: |
| HTML as rendered | 9,689 | 8,619 | 1,070 |
| CSS | 9,041 | 7,289 | 1,752 |
| JS | 563 | 252 | 311 |
| document | 19,339 | 16,206 | 3,133 |

**HTML.** The page after is the page before with one span of 1,070 B cut out, at byte 2,847; every other byte is where it was. The span is the dialog's trigger and the `<dialog>`, whole, and nothing else:

```html
<button type="button" class="rg-button" command="show-modal" commandfor="d1">Install</button><dialog id="d1" class="rg-dialog" closedby="any" aria-labelledby="d1-t"><div data-part="panel"><header><h2 id="d1-t">Install</h2><button type="button" data-part="close" command="close" commandfor="d1" aria-label="Close">✕</button></header><div data-part="body"><p>In a Vite + React project (React 19, Vite 8, Node 22 or later):</p><figure class="code"><pre><code>pnpm add @reactogenic/core@alpha
pnpm add -D @reactogenic/vite@alpha @reactogenic/cli@alpha</code></pre></figure><p>The packages are published under the <code>alpha</code> tag. <code>@reactogenic/cli</code> brings the <code>reactogenic</code> binary for macOS, Linux and Windows, arm64 and x64.</p></div><footer><button type="button" class="rg-button" data-variant="ghost" command="close" commandfor="d1">Close</button><a class="rg-button" href="/guide/">Getting started</a></footer></div><button type="button" data-part="scrim" command="close" commandfor="d1" tabindex="-1" aria-hidden="true"></button></dialog>
```

What that markup had and the page no longer has: `command` (attr), `commandfor` (attr), `dialog` (tag), `d1` (id), `rg-dialog` (class), `closedby` (attr), `aria-labelledby` (attr), `data-part=panel` (value), `d1-t` (id), `data-part=body` (value).

**CSS.** 89 selectors and at-rules before, 75 after: 14 left, 0 came. Each selector that left names something only the deleted markup had:

| Under | Selector | Names | Declarations, B |
| --- | --- | --- | ---: |
| `@layer rg.components` | `.rg-dialog` | `rg-dialog` | 209 |
| `@layer rg.components @media(prefers-reduced-motion:no-preference)` | `.rg-dialog` | `rg-dialog` | 79 |
| `@layer rg.components` | `.rg-dialog[open]` | `rg-dialog` | 66 |
| `@layer rg.components @starting-style` | `.rg-dialog[open]` | `rg-dialog` | 9 |
| `@layer rg.components` | `.rg-dialog::backdrop` | `rg-dialog` | 26 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]` | `rg-dialog`, `data-part=panel` | 263 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header` | `rg-dialog`, `data-part=panel` | 77 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>h2` | `rg-dialog`, `data-part=panel` | 46 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>[data-part=close]` | `rg-dialog`, `data-part=panel` | 121 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>[data-part=close]:hover` | `rg-dialog`, `data-part=panel` | 26 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>[data-part=body]` | `rg-dialog`, `data-part=panel`, `data-part=body` | 53 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>footer` | `rg-dialog`, `data-part=panel` | 131 |
| `@layer rg.components` | `.rg-dialog>[data-part=scrim]` | `rg-dialog` | 98 |
| `@layer rg.components` | `:root:has(.rg-dialog:modal)` | `rg-dialog` | 15 |

Of the 75 that stay, none names a tag, class, id or value that left with the markup.

By source file (`_rg/report.json`), rules kept before → after: `tokens.css` 1 → 1, `button.css` 4 → 4, `dialog.css` 14 → 0, `dropdown-menu.css` 7 → 7, `side-menu.css` 22 → 22, `site.css` 37 → 37.

**JS.** Before: 248 `overlays.ts` + 307 `invokers.ts` + 8 `<entry>` = 563 B. After: 248 `overlays.ts` + 4 `<entry>` = 252 B. Behaviours that left: `invokers`. Every module that stays is the same bytes.
The script is now the one `/guide/` has, to the byte.

Components rendered (the record): `Action` ×2 → 0, `Button` ×5 → 2, `Code` ×5 → 4, `Dialog` ×1 → 0, `Each` ×10 → 9.

## The cheat-sheet dialog of `/syntax/`, alone

T4 as it was first written. A menu item commands the dialog (`commandfor="cheat-sheet"`), so deleting the dialog and nothing else (25 lines of `site/pages/syntax/index.rtsx`) leaves a button that opens nothing. The build refuses it, exit status 1:

```
pages/syntax/index.rtsx: error idref-not-found: Page /syntax/: `commandfor="cheat-sheet"` on `<button>` names no element of the page
```

## The cheat-sheet dialog of `/syntax/`, with the menu item that opens it

Deleted from `site/pages/syntax/index.rtsx`:

```tsx
          <$Item key="cheat-sheet" command="show-modal" commandfor="cheat-sheet">Cheat sheet…</$Item>
…
        <Dialog id="cheat-sheet">
          <$Title>Cheat sheet</$Title>
          <table className="cheat-sheet">
            <thead>
              <tr>
                <th>You write</th>
                <th>It means</th>
              </tr>
            </thead>
            <tbody>
              <Each items={CHEAT_SHEET} { item: row }>
                <tr key={row.write}>
                  <td><code>{row.write}</code></td>
                  <td>
                    <Match on={row.becomes}>
                      <code>{row.becomes}</code>
                      <br />
                    </Match>
                    {row.note}
                  </td>
                </tr>
              </Each>
            </tbody>
          </table>
        </Dialog>
```

### The other pages

| Page | Document, before | after | The same bytes | … as the site ships (`--inline auto`) |
| --- | ---: | ---: | --- | --- |
| `/` | 19,339 | 19,339 | yes: HTML, CSS and script | yes |
| `/guide/` | 18,070 | 18,070 | yes: HTML, CSS and script | yes |
| `/reference/cli/` | 28,980 | 28,980 | yes: HTML, CSS and script | yes |

### `/syntax/`

| | Before | After | Left |
| --- | ---: | ---: | ---: |
| HTML as rendered | 36,287 | 33,566 | 2,721 |
| CSS | 8,914 | 7,244 | 1,670 |
| JS | 1,446 | 252 | 1,194 |
| document | 46,693 | 41,108 | 5,585 |

**HTML.** Between the first and the last tag that differ, 3,011 B became 290 B; every byte before and after is where it was.

Before:

```html
<button type="button" class="rg-button" id="m2-t" popoverTarget="m2" aria-haspopup="menu">Reference</button><div id="m2" class="rg-menu" data-typeahead="" popover="" role="menu" aria-labelledby="m2-t"><button type="button" role="menuitem" autofocus="" command="show-modal" commandfor="cheat-sheet">Cheat sheet…</button><a href="https://github.com/reactogenic/reactogenic/blob/main/specs/phase01/syntax.md" role="menuitem">Specification on GitHub</a><a href="/guide/" role="menuitem">Getting started</a></div><dialog id="cheat-sheet" class="rg-dialog" closedby="any" aria-labelledby="cheat-sheet-t"><div data-part="panel"><header><h2 id="cheat-sheet-t">Cheat sheet</h2><button type="button" data-part="close" command="close" commandfor="cheat-sheet" aria-label="Close">✕</button></header><div data-part="body"><table class="cheat-sheet"><thead><tr><th>You write</th><th>It means</th></tr></thead><tbody><tr><td><code>&lt;Input value /&gt;</code></td><td><code>value={value}</code><br/>when value is bound in the module&#x27;s scope chain; otherwise true, as in TSX</td></tr><tr><td><code>&lt;$Icon className=&quot;i&quot;&gt;…&lt;/$Icon&gt;</code></td><td><code>$Icon={{ className: &quot;i&quot;, children: … }}</code><br/>a slot element is the prop of its parent component</td></tr><tr><td><code>&lt;$Icon { size }&gt;…&lt;/$Icon&gt;</code></td><td><code>children: ({ size }) =&gt; …</code><br/>params: the values the component hands out</td></tr><tr><td><code>&lt;$Column key=&quot;email&quot; /&gt;</code></td><td><code>$Column={{ [KEYED]: true, &quot;email&quot;: {} }}</code><br/>with a key: one entry of a KeyedSlot</td></tr><tr><td><code>&lt;span slot={$Icon}&gt;★&lt;/span&gt;</code></td><td>in the component: the slot renders as this element; the children are the fallback</td></tr><tr><td><code>&lt;span slot={$Icon} &amp;size &amp;&amp;value={x} /&gt;</code></td><td>on an attachment: an arg for the slot&#x27;s body; &amp;&amp; is an arg and a prop</td></tr><tr><td><code>&lt;Match on={user}&gt;…&lt;/Match&gt;</code></td><td><code>{user ? … : null}</code><br/>the body is inline, so user is narrowed in it</td></tr><tr><td><code>&lt;Switch on={s}&gt;&lt;$Case is=&quot;a&quot;&gt;…&lt;/$Case&gt;&lt;/Switch&gt;</code></td><td><code>{s === &quot;a&quot; ? … : null}</code><br/>first match wins; with exhaustive a missing case is a type error</td></tr><tr><td><code>&lt;Each items { item, index }&gt;…&lt;/Each&gt;</code></td><td><code>&lt;Each items={items}&gt;{({ item, index }) =&gt; …}&lt;/Each&gt;</code><br/>a runtime component; the key goes on the body&#x27;s root element</td></tr><tr><td><code>&lt;section #pricing /&gt;</code></td><td><code>&lt;section id=&quot;pricing&quot;&gt;&lt;_Section_pricing /&gt;&lt;/section&gt;</code><br/>the default export of ./pricing.rtsx, imported under that name</td></tr></tbody></table></div></div><button type="button" data-part="scrim" command="close" commandfor="cheat-sheet" tabindex="-1" aria-hidden="true"></button></dialog>
```

After:

```html
<button type="button" class="rg-button" popoverTarget="m2">Reference</button><ul id="m2" class="rg-menu" popover=""><li><a href="https://github.com/reactogenic/reactogenic/blob/main/specs/phase01/syntax.md">Specification on GitHub</a></li><li><a href="/guide/">Getting started</a></li></ul>
```

What that markup had and the page no longer has: `m2-t` (id), `aria-haspopup` (attr), `data-typeahead` (attr), `data-typeahead=` (value), `role` (attr), `role=menu` (value), `aria-labelledby` (attr), `role=menuitem` (value), `autofocus` (attr), `command` (attr), `commandfor` (attr), `dialog` (tag), `cheat-sheet` (id), `rg-dialog` (class), `closedby` (attr), `data-part=panel` (value), `cheat-sheet-t` (id), `data-part=body` (value), `cheat-sheet` (class), `br` (tag).

**CSS.** 89 selectors and at-rules before, 74 after: 15 left, 0 came. Each selector that left names something only the deleted markup had — but for 1:

| Under | Selector | Names | Declarations, B |
| --- | --- | --- | ---: |
| `@layer rg.components` | `.rg-dialog` | `rg-dialog` | 209 |
| `@layer rg.components @media(prefers-reduced-motion:no-preference)` | `.rg-dialog` | `rg-dialog` | 79 |
| `@layer rg.components` | `.rg-dialog[open]` | `rg-dialog` | 66 |
| `@layer rg.components @starting-style` | `.rg-dialog[open]` | `rg-dialog` | 9 |
| `@layer rg.components` | `.rg-dialog::backdrop` | `rg-dialog` | 26 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]` | `rg-dialog`, `data-part=panel` | 263 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header` | `rg-dialog`, `data-part=panel` | 77 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>h2` | `rg-dialog`, `data-part=panel` | 46 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>[data-part=close]` | `rg-dialog`, `data-part=panel` | 121 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>header>[data-part=close]:hover` | `rg-dialog`, `data-part=panel` | 26 |
| `@layer rg.components` | `.rg-dialog>[data-part=panel]>[data-part=body]` | `rg-dialog`, `data-part=panel`, `data-part=body` | 53 |
| `@layer rg.components` | `.rg-dialog>[data-part=scrim]` | `rg-dialog` | 98 |
| `@layer rg.components` | `:root:has(.rg-dialog:modal)` | `rg-dialog` | 15 |
| `@layer rg.components` | `.rg-menu>:is(a,button)` | **nothing that left** | 187 |
|  | `.cheat-sheet td code` | `cheat-sheet` | 41 |

Of the 74 that stay, none names a tag, class, id or value that left with the markup.

By source file (`_rg/report.json`), rules kept before → after: `tokens.css` 1 → 1, `button.css` 4 → 4, `dialog.css` 13 → 0, `dropdown-menu.css` 7 → 7, `side-menu.css` 22 → 22, `site.css` 37 → 36.

**JS.** Before: 248 `overlays.ts` + 850 `menu-keys.ts` + 307 `invokers.ts` + 41 `<entry>` = 1,446 B. After: 248 `overlays.ts` + 4 `<entry>` = 252 B. Behaviours that left: `menu-keys`, `invokers`. Every module that stays is the same bytes. Flags: `RG_MENU_TYPEAHEAD=true` → none.
The script is now the one `/guide/` has, to the byte.

Components rendered (the record): `Dialog` ×1 → 0, `Each` ×18 → 17, `MenuItem` ×6 → 5.

## One option: `typeahead` off on the action menu of `/syntax/`

Deleted from `site/pages/syntax/index.rtsx`:

```tsx
 typeahead
```

### The other pages

| Page | Document, before | after | The same bytes | … as the site ships (`--inline auto`) |
| --- | ---: | ---: | --- | --- |
| `/` | 19,339 | 19,339 | yes: HTML, CSS and script | yes |
| `/guide/` | 18,070 | 18,070 | yes: HTML, CSS and script | yes |
| `/reference/cli/` | 28,980 | 28,980 | yes: HTML, CSS and script | yes |

### `/syntax/`

| | Before | After | Left |
| --- | ---: | ---: | ---: |
| HTML as rendered | 36,287 | 36,269 | 18 |
| CSS | 8,914 | 8,914 | 0 |
| JS | 1,446 | 1,126 | 320 |
| document | 46,693 | 46,355 | 338 |

**HTML.** The page after is the page before with one span of 18 B cut out, at byte 2,959; every other byte is where it was. The span is the attribute the behaviour reads:

```html
 data-typeahead=""
```

**CSS.** 89 selectors and at-rules before, 89 after: 0 left, 0 came.

Of the 89 that stay, none names a tag, class, id or value that left with the markup.

By source file (`_rg/report.json`), rules kept before → after: `tokens.css` 1 → 1, `button.css` 4 → 4, `dialog.css` 13 → 13, `dropdown-menu.css` 7 → 7, `side-menu.css` 22 → 22, `site.css` 37 → 37.

**JS.** Before: 248 `overlays.ts` + 850 `menu-keys.ts` + 307 `invokers.ts` + 41 `<entry>` = 1,446 B. After: 248 `overlays.ts` + 530 `menu-keys.ts` + 307 `invokers.ts` + 41 `<entry>` = 1,126 B. Behaviours that left: none. Of those that stay, `menu-keys.ts` went from 850 to 530 B. Flags: `RG_MENU_TYPEAHEAD=true` → `RG_MENU_TYPEAHEAD=false`.

Components rendered (the record): the same.

## Verdict

T4 **passes**: deleting the *Install* dialog from `/` removed its markup, the CSS selectors that name it and the `invokers` behaviour from that page, nothing else of the page, and no byte of any other page.
