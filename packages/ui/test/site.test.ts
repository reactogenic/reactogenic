// The example site (test/site), page by page, through the stand-in builder:
// what the spec says beyond its examples — the current page and the open
// disclosures per pathname, ids, options, and which behaviours a page mounts.
import type { ComponentType } from "react";
import { describe, expect, test } from "vitest";
import Actions from "./site/pages/actions/index.rtsx";
import Flow from "./site/pages/guide/flow/index.rtsx";
import Slots from "./site/pages/guide/slots/index.rtsx";
import Index from "./site/pages/index.rtsx";
import Plain from "./site/pages/plain/index.rtsx";
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
});

describe("Dialog", () => {
  const { html } = build("/", Index);

  test("light dismiss is a scrim after the panel; the close button is the dialog's first focusable element", () => {
    const dialog = html.slice(html.indexOf('<dialog id="d1"'), html.indexOf("</dialog>"));
    expect(dialog).toContain('<dialog id="d1" class="rg-dialog" closedby="any" aria-labelledby="d1-t">');
    expect(dialog.endsWith('</div><form data-part="scrim" method="dialog"><button tabindex="-1" aria-hidden="true"></button></form>')).toBe(true);
    const focusable = [...dialog.matchAll(/<(?:button|a|input)\b[^>]*>/g)].map((match) => match[0]);
    expect(focusable[0]).toBe('<button type="button" data-part="close" command="close" commandfor="d1" aria-label="Close">');
    expect(focusable.at(-1)).toBe('<button tabindex="-1" aria-hidden="true">');
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
        '<div id="actions" class="rg-menu" popover role="menu" aria-labelledby="actions-t">' +
        '<button type="button" role="menuitem" autofocus command="show-modal" commandfor="shortcuts">Keyboard shortcuts…</button>',
    );
    expect(html).toContain('<button type="button" role="menuitem" disabled>Print</button>');
  });

  test("generated ids count per page, whatever ids the author gave", () => {
    // The layout's menu is `versions` (the author's); the page's own is the builder's.
    expect(build("/actions/", Actions).html).toContain('<div id="m2" class="rg-menu" popover role="menu" aria-labelledby="m2-t">');
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
