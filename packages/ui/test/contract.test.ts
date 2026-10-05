// The HTML contract (RGP2-025): every `.rtsx` example of
// specs/phase02/components.md, taken from the spec at test time — never
// copied — transpiled by phase 1 and rendered by React's static renderer
// with a stand-in for the builder, gives the HTML the spec shows after it.
// And what each example mounts is what the spec's *Behaviours* table says.
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { createElement, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeAll, describe, expect, test } from "vitest";
import { distinct, normalise, Page, type Mount } from "./stand-in.ts";

const OVERLAYS = "@reactogenic/ui/behaviors/overlays";
const INVOKERS = "@reactogenic/ui/behaviors/invokers";
const MENU_KEYS = "@reactogenic/ui/behaviors/menu-keys";

interface Example {
  name: string; // "Dialog/1": the `##` heading and the example's number under it
  rtsx: string;
  html: string;
  pathname: string; // from `<!-- emitted for the page /x/ -->`, else "/"
}

// A ```tsx block whose first line is `// .rtsx …` or `// name.rtsx …`, and
// the ```html block that follows it.
function examples(spec: string): Example[] {
  const found: Example[] = [];
  const counts = new Map<string, number>();
  let heading = "";
  let rtsx: string | undefined;
  for (const match of spec.matchAll(/^## +(.+)$|^```(\w+)\n([\s\S]*?)^```$/gm)) {
    const [, title, language, body] = match;
    if (title !== undefined) {
      heading = title.replace(/`/g, "");
    } else if (language === "tsx" && /^\/\/ (\w+)?\.rtsx\b/.test(body!)) {
      rtsx = body!.slice(body!.indexOf("\n") + 1); // without the `// .rtsx` line: inside JSX it would be text
    } else if (language === "html" && rtsx !== undefined) {
      const n = (counts.get(heading) ?? 0) + 1;
      counts.set(heading, n);
      found.push({ name: `${heading}/${n}`, rtsx, html: body!, pathname: /emitted for the page (\S+)/.exec(body!)?.[1] ?? "/" });
      rtsx = undefined;
    } else {
      rtsx = undefined;
    }
  }
  return found;
}

const spec = readFileSync(resolve(import.meta.dirname, "../../../specs/phase02/components.md"), "utf8");
const cases = examples(spec);
// Rendered once, in the spec's order, as one page: its ids count on from
// example to example (`m1`, then `m2`), as the spec's HTML does.
const rendered = new Map<string, { html: string; mounts: Mount[] }>();

beforeAll(async () => {
  const dir = resolve(import.meta.dirname, "../.examples");
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir);
  const page = new Page("/");
  for (const [index, example] of cases.entries()) {
    const file = join(dir, `example${index + 1}.rtsx`);
    writeFileSync(
      file,
      `import { Button, Dialog, DropdownMenu, SideMenu } from "@reactogenic/ui";\n` +
        `void [Button, Dialog, DropdownMenu, SideMenu];\n` +
        `export default function Example() {\n  return (\n    <>\n${example.rtsx}    </>\n  );\n}\n`,
    );
    const module = (await import(/* @vite-ignore */ pathToFileURL(file).href)) as { default: ComponentType };
    const before = page.mounts.length;
    const html = page.render(module.default, example.pathname);
    rendered.set(example.name, { html, mounts: page.mounts.slice(before) });
  }
});

test("the spec has the examples this suite knows", () => {
  expect(cases.map((example) => example.name)).toEqual(["Button/1", "Dialog/1", "Dialog/2", "DropdownMenu/1", "DropdownMenu/2", "DropdownMenu/3", "SideMenu/1", "SideMenu/2"]);
});

describe("emitted HTML equals the spec's", () => {
  test.each(cases)("$name", ({ name, html }) => {
    expect(normalise(rendered.get(name)!.html)).toBe(normalise(html));
  });
});

// components.md, *Types*: the spec prints HTML as a parser reads it. As a
// string, React's output is spelled otherwise — and nothing rewrites it
// (builder.md: the HTML is React's, to the byte).
describe("React's spellings", () => {
  test("camelCase names and empty values, as written by the components", () => {
    const menu = rendered.get("DropdownMenu/2")!.html;
    expect(menu).toContain('popoverTarget="m2"');
    expect(menu).toContain('popover=""');
    expect(menu).toContain('autofocus=""');
    const side = rendered.get("SideMenu/1")!.html;
    expect(side).toContain('popoverTargetAction="hide"');
    expect(side).toContain('tabindex="-1"');
    expect(side).toContain('<details open="">');
  });

  test("a bare `popover` or a lower-case `autofocus` is dropped: a component writes `popover=\"\"` and `autoFocus`", () => {
    const error = console.error;
    console.error = () => {}; // React says so, in development
    try {
      expect(renderToStaticMarkup(createElement("ul", { popover: true as unknown as "" }))).toBe("<ul></ul>");
      expect(renderToStaticMarkup(createElement("button", { autofocus: true } as object))).toBe("<button></button>");
    } finally {
      console.error = error;
    }
  });
});

describe("what an example mounts", () => {
  const mounts = (name: string) => distinct(rendered.get(name)!.mounts);

  test("a button that is a link: nothing", () => {
    expect(mounts("Button/1")).toEqual([]);
  });

  test("a dialog: overlays and invokers, both page-level", () => {
    expect(mounts("Dialog/1")).toEqual([{ module: OVERLAYS }, { module: INVOKERS }]);
    expect(mounts("Dialog/2")).toEqual([{ module: OVERLAYS }, { module: INVOKERS }]);
  });

  test("a menu of links: overlays only", () => {
    expect(mounts("DropdownMenu/1")).toEqual([{ module: OVERLAYS }]);
  });

  test("an action menu adds menu-keys on that menu, with its flag — and invokers for the item with a command", () => {
    expect(mounts("DropdownMenu/2")).toEqual([
      { module: OVERLAYS },
      { module: MENU_KEYS, id: "m2", flags: { RG_MENU_TYPEAHEAD: false } },
      { module: INVOKERS },
    ]);
  });

  test("typeahead turns the flag of that mount on, and is that mount's data — not an attribute", () => {
    expect(mounts("DropdownMenu/3")).toEqual([
      { module: OVERLAYS },
      { module: MENU_KEYS, id: "m3", flags: { RG_MENU_TYPEAHEAD: true }, data: { typeahead: true } },
      { module: INVOKERS },
    ]);
  });

  test("a side menu: overlays only", () => {
    expect(mounts("SideMenu/1")).toEqual([{ module: OVERLAYS }]);
    expect(mounts("SideMenu/2")).toEqual([{ module: OVERLAYS }]);
  });
});
