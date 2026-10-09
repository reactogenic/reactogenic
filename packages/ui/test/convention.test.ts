// The CSS convention (specs/phase02/components.md, *CSS convention*), checked
// on the components' own CSS in the form the builder prunes — nesting
// lowered — and on the behaviours, whose side of it is what they may write.
// It is what makes per-page pruning exact: a rule goes with its component
// — its root, a variant's own class, a part — and no script makes a selector
// match that the page's HTML does not show.
import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import * as esbuild from "esbuild";
import { beforeAll, describe, expect, test } from "vitest";

const src = resolve(import.meta.dirname, "../src");

// A component's CSS, and its root classes.
const sheets: Record<string, string[]> = {
  "button.css": [".rg-button"],
  "dialog.css": [".rg-dialog"],
  "dropdown-menu.css": [".rg-menu"],
  "side-menu.css": [".rg-sidemenu", ".rg-sidemenu-toggle"],
};

interface Rule {
  sheet: string;
  selector: string; // one selector of the rule's list
  body: string;
  within: string[]; // the at-rules around it, outermost first
}

// The rules of flat CSS, at-rules entered. esbuild's output has no comments
// and these sheets no strings, so a brace is a brace.
function rulesOf(sheet: string, css: string, within: string[] = []): Rule[] {
  const found: Rule[] = [];
  for (let at = 0; ; ) {
    const open = css.indexOf("{", at);
    if (open < 0) {
      return found;
    }
    const prelude = css.slice(at, open).split(";").at(-1)!.trim(); // after any statement (`@layer a, b;`)
    let close = open + 1;
    for (let depth = 1; depth > 0; close++) {
      depth += css[close] === "{" ? 1 : css[close] === "}" ? -1 : 0;
    }
    const body = css.slice(open + 1, close - 1);
    if (/^@(media|supports|layer|starting-style)\b/.test(prelude)) {
      found.push(...rulesOf(sheet, body, [...within, prelude]));
    } else if (!prelude.startsWith("@")) {
      found.push(...selectors(prelude).map((selector) => ({ sheet, selector, body, within })));
    } // else `@position-try`: declarations, no selector
    at = close;
  }
}

// A selector list at its top-level commas.
function selectors(list: string): string[] {
  const found: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i <= list.length; i++) {
    depth += list[i] === "(" ? 1 : list[i] === ")" ? -1 : 0;
    if (i === list.length || (list[i] === "," && depth === 0)) {
      found.push(list.slice(start, i).trim());
      start = i + 1;
    }
  }
  return found;
}

// A selector without what its functions and attribute selectors hold.
function outline(selector: string): string {
  let out = selector;
  for (let last = ""; last !== out; ) {
    last = out;
    out = out.replace(/\([^()]*\)|\[[^\]]*\]/g, "");
  }
  return out;
}

const rules: Rule[] = [];

beforeAll(async () => {
  for (const sheet of Object.keys(sheets)) {
    const { code } = await esbuild.transform(readFileSync(resolve(src, sheet), "utf8"), { loader: "css", supported: { nesting: false } });
    rules.push(...rulesOf(sheet, code));
  }
  await esbuild.stop();
});

describe("CSS convention", () => {
  test("every rule is there", () => {
    expect(rules.length).toBeGreaterThan(40);
    expect(new Set(rules.map((rule) => rule.sheet))).toEqual(new Set(Object.keys(sheets)));
  });

  test("every selector starts at a root class of its component — or names it in :has(), for a rule on an ancestor", () => {
    const rooted = (selector: string, roots: string[]): boolean => {
      // What lowering makes of `> a, > li > a { &:hover { … } }`: `:is(.rg-menu > a, .rg-menu > li > a):hover`.
      if (selector.startsWith(":is(")) {
        let close = 4;
        for (let depth = 1; depth > 0; close++) {
          depth += selector[close] === "(" ? 1 : selector[close] === ")" ? -1 : 0;
        }
        return selectors(selector.slice(4, close - 1)).every((alternative) => rooted(alternative, roots));
      }
      return roots.some((root) => new RegExp(`^(\\${root}|:root:has\\(\\${root})(?![\\w-])`).test(selector));
    };
    const stray = rules.filter(({ sheet, selector }) => !rooted(selector, sheets[sheet]!));
    expect(stray.map((rule) => `${rule.sheet}: ${rule.selector}`)).toEqual([]);
    expect(rooted(".rg-dialogue > a", [".rg-dialog"]) || rooted("a.rg-dialog", [".rg-dialog"]) || rooted(":is(.rg-menu > a, li > a):hover", [".rg-menu"])).toBe(false);
  });

  test("a class is a component's root or a variant of it; a part is `data-part`; state is the platform's — a pseudo-class or one of these attributes", () => {
    const roots = Object.values(sheets).flat();
    for (const { sheet, selector } of rules) {
      // Every class of a rule is its component's: a root, or a root and a variant's name (`rg-button-ghost`).
      const own = sheets[sheet]!;
      const stray = (selector.match(/\.[a-zA-Z][\w-]*/g) ?? []).filter((name) => !own.some((root) => name === root || name.startsWith(root + "-")));
      expect(stray, `${sheet}: ${selector}`).toEqual([]);
    }
    // The variants, and nothing else: no part is a class. A slot's
    // `className` replaces its attachment's (phase01/syntax.md, *Slots*), so
    // a part that was a class would lose its rule to `<$Footer className="mine">`.
    const classes = new Set(rules.flatMap((rule) => rule.selector.match(/\.[a-zA-Z][\w-]*/g) ?? []));
    expect([...classes].filter((name) => !roots.includes(name)).sort()).toEqual([".rg-button-ghost", ".rg-menu-end"]);
    // No option is an attribute. Parts are written by the component; `open`
    // is the browser's, `aria-*` the component's or a behaviour's.
    const attributes = new Set(rules.flatMap((rule) => [...rule.selector.matchAll(/\[([\w-]+)/g)].map((match) => match[1])));
    expect([...attributes].sort()).toEqual(["aria-current", "aria-disabled", "data-part", "open"]);
  });

  // A class a rule names and no component writes is a rule the builder drops
  // on every page, silently: a typo in either file.
  test("every class a sheet selects is one its component writes — a root in a `className`, a variant in a `variants()` map — and the other way", () => {
    const components: Record<string, string> = { "button.css": "button.rtsx", "dialog.css": "dialog.rtsx", "dropdown-menu.css": "dropdown-menu.rtsx", "side-menu.css": "side-menu.rtsx" };
    for (const [sheet, component] of Object.entries(components)) {
      const source = readFileSync(resolve(src, component), "utf8");
      const written = new Set([...source.matchAll(/"((?:rg-[\w-]+ ?)+)"/g)].flatMap((match) => match[1]!.split(" ")));
      const selected = new Set(rules.filter((rule) => rule.sheet === sheet).flatMap((rule) => rule.selector.match(/\.[a-zA-Z][\w-]*/g) ?? []));
      expect([...selected].filter((name) => !written.has(name.slice(1))), sheet).toEqual([]);
      expect([...written].filter((name) => !selected.has("." + name)), component).toEqual([]);
      // A variant's class is resolved, never written on an element: only a `variants()` map holds it.
      const maps = [...source.matchAll(/const \w+Variants = (\{.*\}) as const;/g)].map((match) => match[1]!).join(" ");
      const variant = [...selected].map((name) => name.slice(1)).filter((name) => !sheets[sheet]!.includes("." + name));
      expect(variant.filter((name) => !maps.includes(`"${name}"`) || source.split(`"${name}"`).length !== 2), component).toEqual([]);
    }
  });

  // The owner's rule 6 (decisions.md, K): the design system's rules are in
  // layers, and a layered `!important` beats an unlayered one — no project's
  // CSS could answer it.
  test("no `!important`", () => {
    const important = /!\s*important/i;
    for (const file of readdirSync(src).filter((name) => name.endsWith(".css"))) {
      const css = readFileSync(resolve(src, file), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
      expect(important.test(css), file).toBe(false);
    }
    expect(rules.filter((rule) => important.test(rule.body)).map((rule) => `${rule.sheet}: ${rule.selector}`)).toEqual([]);
    expect(important.test(".a { color: red !important }") && important.test(".a{color:red! IMPORTANT}")).toBe(true);
  });

  test("a rule reaches the component's own structure: child combinators — a descendant only in a side menu's sections, never in a slot's content", () => {
    const descendant = rules.filter((rule) => /[^\s>+~]\s+[^\s>+~]/.test(outline(rule.selector)));
    expect(descendant.filter((rule) => !rule.selector.startsWith(".rg-sidemenu > :is(section, details)")).map((rule) => `${rule.sheet}: ${rule.selector}`)).toEqual([]);
    expect(descendant.length).toBeGreaterThan(0); // the side menu's items
  });

  test("motion: every transition is inside (prefers-reduced-motion: no-preference)", () => {
    const moving = rules.filter((rule) => /(^|[;\s])(transition|animation)[\w-]*\s*:/.test(rule.body));
    expect(moving.map((rule) => rule.selector).sort()).toEqual([".rg-dialog", ".rg-sidemenu"]);
    expect(moving.filter((rule) => !rule.within.some((at) => at.includes("prefers-reduced-motion: no-preference"))).map((rule) => `${rule.sheet}: ${rule.selector}`)).toEqual([]);
  });
});

// builder.md, *CSS*: state only a script can write is decided on the page
// unless the page's script names it — a behaviour names the state it writes
// (components.md, *CSS convention*), and anything a script wrote under a
// name the builder cannot read would make a pruned rule match. These
// behaviours write nothing: they call the platform.
describe("behaviours", () => {
  const dir = resolve(src, "behaviors");
  // Reading is fine (`button.id === …`, `dialog.open`); a call or an
  // assignment is a write — and `classList`, `dataset` and `style` are there
  // to be written. The properties that reflect state are among them:
  // `disabled`, `ariaExpanded`, `defaultChecked` …
  const writes = /\.(classList|dataset|style)\b|\.(className|id|innerHTML|outerHTML|textContent|innerText|hidden|inert|open|disabled|value|checked|selected|role|aria[A-Z]\w*|default[A-Z]\w*|tabIndex|setAttribute\w*|toggleAttribute|removeAttribute\w*|insertAdjacent\w+|append\w*|prepend|before|after|replace\w+|remove|cloneNode)\s*(\(|=[^=])|\bcreateElement\b/;

  test.each(readdirSync(dir))("%s writes no class, attribute or element", (file) => {
    const code = readFileSync(resolve(dir, file), "utf8").replace(/\/\/.*$/gm, "");
    expect(writes.exec(code)?.[0]).toBeUndefined();
  });

  test("the check sees a write", () => {
    for (const code of [
      'menu.classList.add("open")',
      'root.dataset.placement = "top"',
      'item.setAttribute("tabindex", "0")',
      "item.tabIndex = 0;root.hidden = true",
      'document.createElement("div")',
      'trigger.ariaExpanded = "true"',
      "item.disabled = true",
      "page.inert = true",
      "input.defaultChecked = true",
      'option.value = "x"',
      'item.removeAttribute("aria-disabled")',
      "trigger.ariaControlsElements = [menu]",
    ]) {
      expect(writes.test(code), code).toBe(true);
    }
    expect(writes.test('menu.hidePopover(); trigger.focus(); if (button.id === "x" && !dialog.open) dialog.showModal(); item.textContent!.trim()')).toBe(false);
  });
});
