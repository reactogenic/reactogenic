// The example site (test/site), page by page, through the stand-in builder:
// what the spec says beyond its examples — the current page and the open
// disclosures per pathname, ids, options, and which behaviours a page mounts.
import { createElement, type ComponentType } from "react";
import { describe, expect, test } from "vitest";
import Actions from "./site/pages/actions/index.rtsx";
import Fit from "./site/pages/fit/index.rtsx";
import Flow from "./site/pages/guide/flow/index.rtsx";
import Slots from "./site/pages/guide/slots/index.rtsx";
import Index from "./site/pages/index.rtsx";
import Menus from "./site/pages/menus/index.rtsx";
import Nested from "./site/pages/nested/index.rtsx";
import Plain from "./site/pages/plain/index.rtsx";
import { Probe } from "./site/probe.rtsx";
import { distinct, normalise, Page } from "./stand-in.ts";

const OVERLAYS = "@reactogenic/ui/behaviors/overlays";
const INVOKERS = "@reactogenic/ui/behaviors/invokers";
const MENU_KEYS = "@reactogenic/ui/behaviors/menu-keys";

function build(pathname: string, component: ComponentType) {
  const page = new Page(pathname);
  const html = normalise(page.render(component));
  return { html, mounts: distinct(page.mounts) };
}

// The side menu of a page: from its toggle to the end of the <nav>.
function sideMenu(html: string): string {
  return html.slice(html.indexOf('<button type="button" class="rg-sidemenu-toggle"'), html.indexOf("</nav>") + "</nav>".length);
}

describe("SideMenu", () => {
  const menu = (current: string, syntax: string, reference: string) =>
    '<button type="button" class="rg-sidemenu-toggle" popovertarget="nav" aria-label="Documentation">☰</button>' +
    '<nav id="nav" class="rg-sidemenu" popover aria-label="Documentation">' +
    '<button type="button" data-part="close" popovertarget="nav" popovertargetaction="hide" aria-label="Close">✕</button>' +
    '<div data-part="header">Example docs</div>' +
    `<section><ul><li><a href="/"${current === "/" ? ' aria-current="page"' : ""}>Introduction</a></li></ul></section>` +
    `<section><h2>Guide</h2><ul><li><details${syntax}><summary>Syntax</summary><ul>` +
    `<li><a href="/guide/slots/"${current === "/guide/slots/" ? ' aria-current="page"' : ""}>Slots</a></li>` +
    `<li><a href="/guide/flow/"${current === "/guide/flow/" ? ' aria-current="page"' : ""}>Flow control</a></li>` +
    "</ul></details></li></ul></section>" +
    `<details${reference}><summary>Reference</summary><ul>` +
    `<li><a href="/actions/"${current === "/actions/" ? ' aria-current="page"' : ""}>Actions</a></li></ul></details>` +
    '<div data-part="footer">MIT licensed</div>' +
    '<button type="button" data-part="scrim" popovertarget="nav" popovertargetaction="hide" tabindex="-1" aria-hidden="true"></button></nav>';

  test("the current page and the disclosures around it are the builder's", () => {
    // Two pages' menus differ in exactly those attributes.
    expect(sideMenu(build("/", Index).html)).toBe(menu("/", "", ""));
    expect(sideMenu(build("/guide/slots/", Slots).html)).toBe(menu("/guide/slots/", " open", ""));
    expect(sideMenu(build("/guide/flow/", Flow).html)).toBe(menu("/guide/flow/", " open", ""));
    // A collapsed section is open on the page it holds.
    expect(sideMenu(build("/actions/", Actions).html)).toBe(menu("/actions/", "", " open"));
  });

  test("the trailing slash is normalised", () => {
    expect(sideMenu(build("/guide/slots", Slots).html)).toBe(menu("/guide/slots/", " open", ""));
  });

  // The forms a link to a page takes are the builder's (builder.md, *Checks
  // on the page*, a root-relative link). A link with a query or a fragment
  // names a state or a place of a page — not the page.
  test.each([
    ["/guide/", "/guide/", true],
    ["/guide/", "/guide", true],
    ["/guide", "/guide/", true],
    ["/guide/", "/guide/index.html", true],
    ["/", "/index.html", true],
    ["/über/", "/%C3%BCber/", true], // the builder names a page by its directory
    ["/%C3%BCber/", "/über/", true], // … and `location.pathname`, in React, is encoded
    ["/guide/", "/guide/slots/", false],
    ["/guide/", "/", false],
    ["/guide/", "/guide/#keyed", false],
    ["/guide/", "/guide/?tab=2", false],
    ["/guide/", "#keyed", false],
    ["/guide/", "https://example.com/guide/", false],
    ["/a/b/", "/a%2Fb/", false], // an encoded slash is no separator
    ["/100%/", "/100%/", true], // a `%` that encodes nothing stands for itself
    ["/guide/", "/100%/", false],
  ])("on %s, href=%s is the current page: %s", (pathname, href, current) => {
    const { html } = build(pathname, () => createElement(Probe, { href }));
    const link = `<a href="${href}"${current ? ' aria-current="page"' : ""}>Link</a>`;
    // Every disclosure around the current page is open.
    expect(html).toContain(`<details${current ? " open" : ""}><summary>Group</summary><ul><li>${link}</li></ul></details>`);
  });

  // WCAG 2.5.3, Label in Name: a button that shows "Menu" is not named "Docs".
  test("the label names the toggle's fallback, the icon — not the author's own content", () => {
    expect(build("/", Index).html).toContain('<button type="button" class="rg-sidemenu-toggle" popovertarget="nav" aria-label="Documentation">☰</button>');
    expect(build("/nested/", Nested).html).toContain('<button type="button" class="rg-sidemenu-toggle" popovertarget="nav">Menu</button>');
    // The landmark keeps its name.
    expect(build("/nested/", Nested).html).toContain('<nav id="nav" class="rg-sidemenu" popover aria-label="Docs">');
  });
});

describe("Dialog", () => {
  const { html } = build("/", Index);

  test("light dismiss is a scrim after the panel; the close button is the dialog's first focusable element", () => {
    const dialog = html.slice(html.indexOf('<dialog id="d1"'), html.indexOf("</dialog>"));
    expect(dialog).toContain('<dialog id="d1" class="rg-dialog" closedby="any" aria-labelledby="d1-t">');
    const scrim = '<button type="button" data-part="scrim" command="close" commandfor="d1" tabindex="-1" aria-hidden="true">';
    expect(dialog.endsWith(`</div>${scrim}</button>`)).toBe(true);
    const focusable = [...dialog.matchAll(/<(?:button|a|input)\b[^>]*>/g)].map((match) => match[0]);
    expect(focusable[0]).toBe('<button type="button" data-part="close" command="close" commandfor="d1" aria-label="Close">');
    expect(focusable.at(-1)).toBe(scrim);
  });

  test("the scrim is a button of the dialog, never a form: inside the author's <form> a nested form is dropped by the parser", () => {
    const fit = build("/fit/", Fit).html;
    const form = fit.slice(fit.indexOf("<form "), fit.indexOf("</form>"));
    expect(form).toContain('<dialog id="confirm"');
    expect(form.slice(1)).not.toContain("<form");
    // Every button the dialog puts in the author's form is `type="button"`: none submits it.
    expect([...form.matchAll(/<button\b[^>]*>/g)].filter((match) => !match[0].includes(' type="button"'))).toEqual([]);
  });

  test("the author's id on $Title is the one the dialog is named by", () => {
    const fit = build("/fit/", Fit).html;
    expect(fit).toContain('<dialog id="confirm" class="rg-dialog" closedby="any" aria-labelledby="sure">');
    expect(fit).toContain('<h2 id="sure">Sure?</h2>');
  });

  test("closedby other than any has no scrim; without $Action there is no footer", () => {
    const second = html.slice(html.indexOf('<dialog id="d2"'), html.indexOf("</dialog>", html.indexOf('<dialog id="d2"')));
    expect(second).toContain('closedby="closerequest"');
    expect(second).not.toContain("scrim");
    expect(second).toContain("<footer>");
    expect(html.slice(html.indexOf('<dialog id="d1"'), html.indexOf('<dialog id="d2"'))).not.toContain("<footer>");
  });

  test("an author's id names it for a button elsewhere, and no trigger is rendered", () => {
    expect(html).toContain('<button type="button" class="rg-button" data-variant="ghost" command="show-modal" commandfor="shortcuts">Shortcuts</button>');
    expect(html).toContain('</main><dialog id="shortcuts" class="rg-dialog" closedby="any" aria-labelledby="shortcuts-t">');
  });
});

describe("DropdownMenu", () => {
  test("typeahead turns the flag on; a disabled item is skipped for the first focus", () => {
    const { html, mounts } = build("/guide/flow/", Flow);
    expect(mounts).toContainEqual({ module: MENU_KEYS, id: "actions", flags: { RG_MENU_TYPEAHEAD: true } });
    expect(html).toContain(
      '<button type="button" class="rg-button" id="actions-t" popovertarget="actions" aria-haspopup="menu">Actions</button>' +
        '<div id="actions" class="rg-menu" data-typeahead popover role="menu" aria-labelledby="actions-t">' +
        '<button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>',
    );
    expect(html).toContain('<button type="button" role="menuitem" disabled>Print</button>');
  });

  test("typeahead is the menu's: the flag is the page's, the attribute says which menu asked", () => {
    const { html, mounts } = build("/menus/", Menus);
    expect(mounts).toContainEqual({ module: MENU_KEYS, id: "one", flags: { RG_MENU_TYPEAHEAD: true } });
    expect(mounts).toContainEqual({ module: MENU_KEYS, id: "two", flags: { RG_MENU_TYPEAHEAD: false } });
    expect(html).toContain('<div id="one" class="rg-menu" data-typeahead popover role="menu" aria-labelledby="one-t">');
    expect(html).toContain('<div id="two" class="rg-menu" popover role="menu" aria-labelledby="two-t">');
  });

  test("the author's id on $Trigger is the one an action menu is named by", () => {
    const { html } = build("/menus/", Menus);
    expect(html).toContain('<button type="button" class="rg-button" id="mine" popovertarget="three" aria-haspopup="menu">Three</button>');
    expect(html).toContain('<div id="three" class="rg-menu" popover role="menu" aria-labelledby="mine">');
    expect(html).not.toContain("three-t");
  });

  test("a disabled link is a link without its href, in a menu and as a Button; the first focus skips every disabled item", () => {
    const { html } = build("/menus/", Menus);
    expect(html).toContain(
      '<a role="menuitem" aria-disabled="true">Disabled link</a>' +
        '<button type="button" role="menuitem" disabled>Print</button>' +
        '<button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Open</button>' +
        '<a href="/" role="menuitem">Home</a>',
    );
    // A menu of links stays one: `href` decides, not `disabled`.
    expect(html).toContain('<ul id="links" class="rg-menu" popover><li><a aria-disabled="true">Not yet</a></li><li><a href="/">Home</a></li></ul>');
    expect(html).toContain('<a class="rg-button" id="off" aria-disabled="true">A disabled link button</a>');
  });

  // Known limit (components.md): JavaScript enumerates integer-like keys
  // first, ascending. When the KEYED marker carries the written order, this
  // test and the limit go.
  test("known limit: integer-like keys are rendered first, not in the order written", () => {
    const { html } = build("/menus/", Menus);
    const order = html.slice(html.indexOf('<ul id="order"'), html.indexOf("</ul>", html.indexOf('<ul id="order"')));
    expect([...order.matchAll(/<a href="\/">(\w+)<\/a>/g)].map((match) => match[1])).toEqual(["9", "10", "b", "a"]); // written: b, 10, 9, a
  });

  test("generated ids count per page, whatever ids the author gave", () => {
    // The layout's menu is `versions` (the author's); the page's own is the builder's.
    expect(build("/actions/", Actions).html).toContain('<div id="m2" class="rg-menu" popover role="menu" aria-labelledby="m2-t">');
  });
});

// The builder's own checks (builder.md, *Checks on the page*), stood in for:
// the components must never be what makes a page fail them.
describe("references by id", () => {
  const pages: [string, ComponentType][] = [
    ["/", Index],
    ["/actions/", Actions],
    ["/fit/", Fit],
    ["/guide/flow/", Flow],
    ["/guide/slots/", Slots],
    ["/menus/", Menus],
    ["/nested/", Nested],
    ["/plain/", Plain],
  ];

  test.each(pages)("%s: every reference names an element of the page, and no id is there twice", (pathname, component) => {
    const { html } = build(pathname, component);
    const ids = [...html.matchAll(/<[a-z][^>]*? id="([^"]*)"/g)].map((match) => match[1]!);
    expect(ids.filter((id, index) => ids.indexOf(id) !== index)).toEqual([]);
    const references = [...html.matchAll(/ (commandfor|popovertarget|aria-labelledby|aria-describedby|aria-controls|for)="([^"]*)"/g)];
    expect(references.filter(([, , id]) => !ids.includes(id!)).map(([, attribute, id]) => `${attribute}="${id}"`)).toEqual([]);
  });
});

describe("what a page mounts", () => {
  test("the layout alone: overlays, and invokers for its dialog", () => {
    expect(build("/guide/slots/", Slots).mounts).toEqual([{ module: OVERLAYS }, { module: INVOKERS }]);
    expect(build("/", Index).mounts).toEqual([{ module: OVERLAYS }, { module: INVOKERS }]);
  });

  test("an action menu adds menu-keys, per menu, with its flag", () => {
    expect(build("/guide/flow/", Flow).mounts).toEqual([
      { module: OVERLAYS },
      { module: INVOKERS },
      { module: MENU_KEYS, id: "actions", flags: { RG_MENU_TYPEAHEAD: true } },
    ]);
    expect(build("/actions/", Actions).mounts).toEqual([
      { module: OVERLAYS },
      { module: INVOKERS },
      { module: MENU_KEYS, id: "m2", flags: { RG_MENU_TYPEAHEAD: false } },
    ]);
  });

  test("a page with nothing that opens mounts nothing", () => {
    const { html, mounts } = build("/plain/", Plain);
    expect(mounts).toEqual([]);
    expect(html).toContain('<a class="rg-button" href="/">Home</a>');
  });
});
