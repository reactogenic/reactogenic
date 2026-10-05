#!/usr/bin/env node
// catalog-verify.mjs — the catalog site (bench/catalog-site: a measurement
// fixture) in real browsers. A sanity pass, and the larger soundness test of
// the pruner: twenty components' CSS pruned per page, against the same pages
// with the whole sheet (specs/phase02/bet.md, *The catalog*).
//
//   node bench/catalog-verify.mjs [--binary <reactogenic>] [--engines chromium,webkit] [--inline never|always]
//                                 [--md bench/results/catalog-verify.md] [--shots <dir>] [--keep <dir>]
//
// The site is built as it ships and with `--no-specialize` — the control: the
// same HTML, the whole site's CSS and script on every page — and both are
// served over HTTP. Then, in each engine, at 1200 and 400 px, on every page:
//
//   1. it loads: a title, a heading, no sideways scroll, one script — the
//      builder's — no React, and no console error, page error, failed
//      request or 4xx;
//   2. pruning changes nothing that is seen (builder.md, *CSS*: "a dropped
//      rule must never have matched"), asked two ways, in every state below:
//        - the computed style of every element, and of its ::before,
//          ::after, ::marker and ::backdrop, is the same in the two builds
//          (the method of verify.mjs: compare.mjs);
//        - no selector that the control's sheet has and the page's own sheet
//          has not matches an element of the page. This one names the
//          selector that was wrongly dropped;
//      at rest, in dark, with reduced motion, with keyboard focus, with the
//      layout's menu open and — at 400 px — its drawer; and in the states the
//      catalog's own components and behaviours make: a tab selected by a
//      click, by a key and by the URL's hash, a tooltip shown, an accordion
//      item opened, a sample copied, the action menu and the dialogs open, a
//      field counted to its limit, a password shown, a switch thrown, a
//      toast shown;
//   3. the behaviours do what they are for: the checks that the states above
//      were reached.
//
// Exit status: 1 on any failure. *Known* is a limit of an engine or of this
// harness, said and not failed.
//
// Playwright is packages/ui's devDependency and is taken from there
// (`pnpm --filter @reactogenic/ui exec playwright install chromium webkit`).
// Chromium is the full browser (`channel: "chromium"`). Firefox:
// `--engines firefox`, where it starts.
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { differences, serve, settle, takeRule } from "./compare.mjs";
import { CATALOG_PAGES as PAGES, binary, catalogSite, mustBuild, options, repo } from "./lib.mjs";

const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/catalog-verify.md"));
const engineList = String(opts.engines ?? "chromium,webkit").split(",");
const shots = opts.shots ? resolve(opts.shots) : null;
if (shots) mkdirSync(shots, { recursive: true });
const bin = binary(opts.binary);
const work = opts.keep ? resolve(opts.keep) : mkdtempSync(join(tmpdir(), "reactogenic-bench-catalog-verify-"));
const flags = opts.inline ? ["--inline", String(opts.inline)] : [];
const builtDir = join(work, "default"), controlDir = join(work, "control");
mustBuild(bin, catalogSite, builtDir, flags);
mustBuild(bin, catalogSite, controlDir, [...flags, "--no-specialize"]);

const require = createRequire(join(repo, "packages/ui/package.json"));
const pw = await import(pathToFileURL(require.resolve("playwright")).href).then((m) => m.default ?? m);

const WIDE = 1200, NARROW = 400;
const results = []; // { engine, at, name, state: "ok" | "known" | "FAIL", detail }
let engine = "";
const record = (state, at, name, detail = "") => {
  results.push({ engine, at, name, state, detail });
  console.log(state === "ok" ? "ok   " : state === "known" ? "known" : "FAIL ", at ? `${at}: ${name}` : name, state === "ok" ? "" : detail);
};
const check = (at, name, ok, detail = "") => record(ok ? "ok" : "FAIL", at, name, ok ? "" : detail);
const limit = (at, name, ok, detail = "") => record(ok ? "ok" : "known", at, name, ok ? "" : detail);

// The selectors of the page's sheets, each under the at-rules around it, and
// whether one matches an element now. Run in the page.
const selectorsNow = () => {
  const split = (text) => {
    const out = [];
    let depth = 0, start = 0, quote = "";
    for (let i = 0; i < text.length; i++) {
      const c = text[i];
      if (quote) { if (c === "\\") i++; else if (c === quote) quote = ""; continue; }
      if (c === '"' || c === "'") quote = c;
      else if (c === "(" || c === "[") depth++;
      else if (c === ")" || c === "]") depth--;
      else if (c === "," && depth === 0) { out.push(text.slice(start, i).trim()); start = i + 1; }
    }
    out.push(text.slice(start).trim());
    return out;
  };
  const out = [];
  const visit = (rules, context, live) => {
    for (const rule of rules) {
      if (rule.selectorText !== undefined) {
        for (const selector of split(rule.selectorText)) {
          // A pseudo-element is its element's: the rule matches where the element does.
          const element = selector.replace(/::[\w-]+(\([^)]*\))?$/, "") || "*";
          let matches = false;
          try { matches = live && document.querySelector(element) !== null; } catch { matches = false; }
          out.push([`${context} | ${selector}`, matches]);
        }
        // Rules nested in a style rule: the engine keeps `&` forms as they are.
        if (rule.cssRules?.length) visit(rule.cssRules, `${context} ${rule.selectorText} {`, live);
      } else if (rule.cssRules) {
        const name = rule.constructor.name;
        const prelude = rule.conditionText ?? rule.name ?? "";
        const on = name === "CSSMediaRule" ? matchMedia(rule.conditionText).matches : name === "CSSStartingStyleRule" ? false : true;
        visit(rule.cssRules, `${context} ${name}(${prelude})`, live && on);
      }
    }
  };
  for (const sheet of document.styleSheets) visit(sheet.cssRules, "", true);
  return out;
};

let comparisons = 0, compared = 0, droppedAsked = 0;
// The two questions of one state.
async function compare(at, name, page, other) {
  const d = await differences(page, other);
  comparisons++; compared += d.count;
  check(at, `styles, ${name}: the two builds are equal`, d.diffs.length === 0, `${d.count} elements and pseudo-elements, ${d.diffs.length} differences:\n      ${d.diffs.slice(0, 8).join("\n      ")}`);
  const mine = new Set((await page.evaluate(selectorsNow)).map(([key]) => key));
  const whole = await other.evaluate(selectorsNow);
  const dropped = whole.filter(([key]) => !mine.has(key));
  droppedAsked += dropped.length;
  const wrong = dropped.filter(([, matches]) => matches).map(([key]) => key);
  check(at, `dropped, ${name}: no selector the page's sheet lacks matches an element`, wrong.length === 0, `${wrong.length} of ${dropped.length} dropped selectors match:\n      ${wrong.slice(0, 12).join("\n      ")}`);
}

for (const engineName of engineList) {
  const browser = await pw[engineName].launch(engineName === "chromium" ? { channel: "chromium" } : {});
  engine = `${engineName} ${browser.version()}`;
  console.log(`\n=== ${engine}`);
  const TAB = engineName === "webkit" ? "Alt+Tab" : "Tab";
  const built = await serve(builtDir), control = await serve(controlDir);

  for (const width of [WIDE, NARROW]) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    if (engineName === "chromium") await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    const page = await context.newPage(), other = await context.newPage();
    let errors = [];
    const watch = (p, what) => {
      p.on("pageerror", (e) => errors.push(`${what}: ${e}`));
      p.on("console", (m) => m.type() === "error" && errors.push(`${what}: console ${m.text()}`));
      p.on("requestfailed", (r) => errors.push(`${what}: request failed ${r.url()}`));
      p.on("response", (r) => r.status() >= 400 && errors.push(`${what}: ${r.status()} ${r.url()}`));
    };
    watch(page, "default"); watch(other, "control");
    const both = async (f) => { await f(page); await f(other); };
    const media = (options) => both((p) => p.emulateMedia(options));
    const away = () => both((p) => p.mouse.move(width - 2, 798));
    const hover = (selector) => both(async (p) => { const el = p.locator(selector).filter({ visible: true }).first(); await el.scrollIntoViewIfNeeded(); await el.hover(); });
    const click = (selector) => both((p) => p.locator(selector).filter({ visible: true }).first().click());
    const press = (key) => both((p) => p.keyboard.press(key));
    const ask = (f, arg) => page.evaluate(f, arg);
    const agree = async (f, arg) => JSON.stringify(await page.evaluate(f, arg)) === JSON.stringify(await other.evaluate(f, arg));

    for (const pathname of PAGES) {
      const slug = pathname === "/" ? "home" : pathname.replaceAll("/", " ").trim().replaceAll(" ", "-");
      const at = `${width}px ${pathname}`;
      const shot = async (name = slug, fullPage = true) => { if (shots) await page.screenshot({ path: join(shots, `${engineName}-${width}-${name}.png`), fullPage }); };
      const state = async (name) => { await settle(page); await settle(other); await compare(at, name, page, other); };
      errors = [];
      await page.goto(built.origin + pathname);
      await other.goto(control.origin + pathname);
      await both((p) => p.waitForLoadState("load"));
      await settle(page);
      await shot();

      // ---- the page ----
      const doc = await ask(() => ({
        title: document.title, h1: document.querySelectorAll("h1").length,
        scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth,
        scripts: document.scripts.length, react: document.documentElement.outerHTML.includes("__reactFiber") || "React" in window || "__REACT_DEVTOOLS_GLOBAL_HOOK__" in window,
        font: getComputedStyle(document.body).fontFamily, sheet: [...document.styleSheets].reduce((a, s) => a + s.cssRules.length, 0),
      }));
      check(at, "the page loads: a title, one <h1>, a sheet", doc.title.includes("Exemplar") && doc.h1 === 1 && doc.sheet > 0, JSON.stringify(doc));
      check(at, "no sideways scroll", doc.scroll <= doc.client, `scrollWidth ${doc.scroll} > ${doc.client}`);
      check(at, "one script, the builder's; no React", doc.scripts === 1 && !doc.react, JSON.stringify(doc));

      // ---- at rest ----
      await state("at rest");
      await media({ colorScheme: "dark" });
      await state("dark, at rest");
      await media({ colorScheme: "light", reducedMotion: "reduce" });
      await state("reduced motion, at rest");
      await media({ reducedMotion: "no-preference" });
      await media({ forcedColors: "active" });
      await state("forced colours, at rest");
      await media({ forcedColors: "none" });

      // ---- keyboard focus: the first stops of the page ----
      await press(TAB); await press(TAB);
      check(at, "Tab gives an element keyboard focus", await ask(() => document.activeElement.matches(":focus-visible")));
      await state("keyboard focus");
      await both((p) => p.evaluate(() => document.activeElement.blur()));

      // ---- the layout: the links menu, the drawer ----
      await hover('[popovertarget="m1"]');
      await state("hover on the menu's trigger");
      await click('[popovertarget="m1"]');
      check(at, "the layout's menu opens", await ask(() => document.getElementById("m1").matches(":popover-open")));
      await state("the layout's menu open");
      await hover("#m1 a");
      await state("the layout's menu open, hover on an item");
      await press("Escape");
      await away();
      if (width === WIDE) {
        await hover("#s1 a:not([aria-current])");
        await state("hover on a link of the side menu");
        await away();
      } else {
        await click(".rg-sidemenu-toggle");
        await settle(page);
        check(at, "the drawer opens", await ask(() => document.getElementById("s1").matches(":popover-open")));
        if (pathname === "/pricing/") await shot("pricing-drawer", false);
        await state("drawer open");
        await press("Escape");
        await settle(page);
      }
      const marked = await ask(() => [...document.querySelectorAll("#s1 [aria-current]")].map((a) => a.getAttribute("href")));
      check(at, "the side menu marks this page, and no other", pathname === "/404/" ? marked.length === 0 : marked.length === 1 && marked[0] === pathname, JSON.stringify(marked));

      // ---- the page's own components ----
      if (pathname === "/") {
        await hover("a.bc-card");
        await state("hover on a card that is a link");
        await away();
      }

      if (pathname === "/pricing/") {
        const tabs = () => [...document.querySelectorAll("#t1 [role=tab]")].map((t) => [t.getAttribute("aria-selected"), t.tabIndex, document.getElementById(t.getAttribute("aria-controls")).hidden]);
        await hover('#t1 [role=tab][aria-selected="false"]');
        await state("hover on a tab");
        await click("#t1-yearly");
        check(at, "tabs: a click selects the tab and shows its panel", JSON.stringify(await ask(tabs)) === JSON.stringify([["false", -1, true], ["true", 0, false]]) && await agree(tabs), JSON.stringify(await ask(tabs)));
        await shot("pricing-yearly");
        await state("the second tab selected");
        await press("ArrowLeft");
        check(at, "tabs: ArrowLeft selects the tab before, with focus", JSON.stringify(await ask(tabs)) === JSON.stringify([["true", 0, false], ["false", -1, true]]) && await ask(() => document.activeElement.id === "t1-monthly"), JSON.stringify(await ask(tabs)));
        await state("a tab selected by a key, focus on it");
        await away();
        await hover('.bc-tooltip > [data-part="trigger"]');
        check(at, "tooltip: shown under the pointer", await ask(() => getComputedStyle(document.querySelector(".bc-tooltip > [role=tooltip]")).visibility === "visible"));
        await state("a tooltip shown");
        await away();
        await both((p) => p.locator('.bc-tooltip > [data-part="trigger"]').first().focus());
        check(at, "tooltip: shown with focus on what it describes", await ask(() => getComputedStyle(document.querySelector(".bc-tooltip > [role=tooltip]")).visibility === "visible"));
        await state("a tooltip shown by focus");
        await both((p) => p.evaluate(() => document.activeElement.blur()));
        await hover(".bc-accordion > details:nth-child(2) > summary");
        await state("hover on an accordion's summary");
        await click(".bc-accordion > details:nth-child(2) > summary");
        check(at, "accordion: opening an item closes the other (one `name`)", JSON.stringify(await ask(() => [...document.querySelectorAll(".bc-accordion > details")].map((d) => d.open))) === JSON.stringify([false, true, false, false, false]));
        await state("another accordion item open");
        await away();
      }

      if (pathname === "/docs/") {
        const selected = () => [...document.querySelectorAll("#pm [role=tab]")].filter((t) => t.getAttribute("aria-selected") === "true" && !document.getElementById(t.getAttribute("aria-controls")).hidden).map((t) => t.id);
        await click("#pm-npm");
        check(at, "tabs: a click selects npm", (await ask(selected)).join() === "pm-npm" && await agree(selected));
        await state("the second tab selected");
        await press("End");
        check(at, "tabs: End selects the last", (await ask(selected)).join() === "pm-brew" && await agree(selected));
        await both((p) => p.evaluate(() => { location.hash = "#pm-yarn"; }));
        await settle(page);
        check(at, "tabs: the URL's hash selects the tab it names (`RG_TABS_HASH`)", (await ask(selected)).join() === "pm-yarn" && await agree(selected), (await ask(selected)).join());
        await state("a tab selected by the hash");
        await both((p) => p.evaluate(() => scrollTo(0, 0)));
        await hover('.bc-code > figcaption > [data-part="copy"]');
        await state("hover on a copy button");
        await click('.bc-code > figcaption > [data-part="copy"]');
        await both((p) => p.waitForFunction(() => [...document.querySelectorAll("[data-part=copy]")].some((b) => b.textContent === "Copied"), null, { timeout: 1500 }).catch(() => {}));
        const copied = await ask(() => [...document.querySelectorAll("[data-part=copy]")].filter((b) => b.textContent === "Copied").length);
        (engineName === "chromium" ? check : limit)(at, "copy: the button copies the sample and says so", copied === 1, "the engine refused the clipboard to a page without a user's permission: the button stays as it was, by design");
        await state("a sample copied");
        await away();
      }

      if (pathname === "/docs/api/") {
        await hover(".bc-table > table > tbody > tr");
        await state("hover on a table's row");
        await away();
        await both((p) => p.locator(".bc-table").first().focus());
        await state("focus on a table's scrolling box");
        await both((p) => p.evaluate(() => document.activeElement.blur()));
      }

      if (pathname === "/changelog/") {
        await hover(".bc-pagination a[href]:not([aria-current])");
        await state("hover on a page's number");
        await away();
      }

      if (pathname === "/dashboard/") {
        await both(async (p) => { await p.locator("#m2-t").focus(); await p.keyboard.press("Enter"); });
        await settle(page);
        check(at, "the action menu opens, focus on its first item", await ask(() => document.getElementById("m2").matches(":popover-open") && document.activeElement.textContent === "Redeploy main…"));
        if (width === WIDE) await shot("dashboard-menu", false);
        await state("action menu open");
        await press("ArrowDown");
        await press("v");
        check(at, "menu-keys: ArrowDown, then typeahead `v` (`RG_MENU_TYPEAHEAD`)", await ask(() => document.activeElement.textContent === "View logs"), await ask(() => document.activeElement.textContent));
        await state("action menu open, keyboard focus on an item");
        await press("Home");
        await press("Enter");
        await settle(page);
        check(at, "its item opens the dialog, modal, and closes the menu", await ask(() => document.getElementById("redeploy").matches(":modal") && !document.getElementById("m2").matches(":popover-open")));
        await state("dialog open");
        await media({ colorScheme: "dark" });
        await state("dark, dialog open");
        await media({ colorScheme: "light" });
        await press("Escape");
        await settle(page);
      }

      if (pathname === "/settings/") {
        // The bio counts its characters, and says when nothing more fits.
        const bio = "textarea[name=bio]";
        await both((p) => p.locator(bio).fill("One line."));
        check(at, "field: the counter follows what is typed (`RG_FIELD_COUNT`)", await ask(() => document.querySelector(".bc-field > output").textContent === "9 / 160"), await ask(() => document.querySelector(".bc-field > output").textContent));
        await state("a field typed in, focus in it");
        await both((p) => p.locator(bio).fill("x".repeat(160)));
        check(at, "field: at the limit the count has `data-full`", await ask(() => document.querySelector(".bc-field > output").hasAttribute("data-full")));
        await state("a field at its limit");
        // "Saved": the toast.
        await click('[popovertarget="toast1"][popovertargetaction="show"]');
        await settle(page);
        check(at, "toast: its trigger shows it", await ask(() => document.getElementById("toast1").matches(":popover-open")));
        if (width === WIDE) await shot("settings-toast", false);
        await state("toast shown");
        await hover("#toast1 > [data-part=close]");
        await state("toast shown, hover on its close button");
        await away();
        if (width === WIDE) {
          await page.waitForFunction(() => !document.getElementById("toast1").matches(":popover-open"), null, { timeout: 7000 }).catch(() => {});
          check(at, "toast: it hides itself after its timeout", await ask(() => !document.getElementById("toast1").matches(":popover-open")));
        }
        await both((p) => p.evaluate(() => document.getElementById("toast1").hidePopover()));
        // The second tab: switches.
        await click("#t1-notifications");
        await state("the notifications tab");
        await click('.bc-check[data-variant="switch"] > input:not(:checked):not(:disabled)');
        await state("a switch thrown, focus on it");
        await click('.bc-check:not([data-variant]) > input');
        await state("a checkbox checked");
        // The third: a password shown, the danger zone.
        await click("#t1-security");
        await shot("settings-security");
        await state("the security tab");
        await click('.bc-field > [data-part="control"] > [data-part="reveal"]');
        const shown = () => { const f = document.querySelector('.bc-field:has([data-part="reveal"])'); return [f.querySelector("input").type, f.querySelector("button").getAttribute("aria-pressed"), f.querySelector("button").textContent]; };
        check(at, "field: the button shows the password and says so (`RG_FIELD_REVEAL`)", JSON.stringify(await ask(shown)) === JSON.stringify(["text", "true", "Hide"]) && await agree(shown), JSON.stringify(await ask(shown)));
        await state("a password shown");
        await click('button[commandfor="d1"][command="show-modal"]');
        await settle(page);
        check(at, "the danger zone's button opens its dialog, modal", await ask(() => document.getElementById("d1").matches(":modal")));
        await state("dialog open");
        await press("Escape");
        await settle(page);
      }

      if (pathname === "/contact/") {
        await both((p) => p.locator("input[name=name]").focus());
        await state("focus in a field");
        await hover(".bc-select > [data-part=control] > select");
        await state("hover on a select");
        await click(".bc-check > input");
        await state("a checkbox checked, focus on it");
        await away();
      }

      check(at, "no page error, console error, failed request or 4xx — in either build", errors.length === 0, errors.slice(0, 5).join(" | "));
    }
    await context.close();
  }

  // ---- the two questions can fail ----
  {
    const context = await browser.newContext({ viewport: { width: WIDE, height: 800 } });
    const page = await context.newPage(), other = await context.newPage();
    await page.goto(built.origin + "/settings/"); await other.goto(built.origin + "/settings/");
    const same = await differences(page, other);
    // One rule that matches taken out of one page's sheet, as a wrong pruner would.
    const taken = await takeRule(page, ".bc-field");
    const broken = await differences(page, other);
    const mine = new Set((await page.evaluate(selectorsNow)).map(([key]) => key));
    const lost = (await other.evaluate(selectorsNow)).filter(([key, matches]) => !mine.has(key) && matches);
    check("", "(a build against itself is equal; with one matching rule taken out of one page, both questions see it: the comparison can fail)", same.diffs.length === 0 && taken !== null && broken.diffs.length > 0 && lost.length > 0, JSON.stringify({ same: same.diffs.length, taken, broken: broken.diffs.length, lost: lost.length }));
    await context.close();
  }
  built.close(); control.close();
  await browser.close();
}

// ---- the report --------------------------------------------------------------------
const count = (s, e) => results.filter((r) => r.state === s && (!e || r.engine === e)).length;
const engines = [...new Set(results.map((r) => r.engine))];
let md = `# The catalog site in a browser\n\nWritten by \`node bench/catalog-verify.mjs${opts.inline ? " --inline " + opts.inline : ""}\` (specs/phase02/bet.md, *The catalog*). \`bench/catalog-site\` — a measurement fixture — as \`reactogenic build${flags.length ? " " + flags.join(" ") : ""}\` writes it, and the control (\`--no-specialize\`), served over HTTP; its ${PAGES.length} pages at ${WIDE} and ${NARROW} px.\n\n`;
md += `| Engine | Passed | Known | Failed |\n| --- | ---: | ---: | ---: |\n`;
for (const e of engines) md += `| ${e} | ${count("ok", e)} | ${count("known", e)} | ${count("FAIL", e)} |\n`;
const styles = results.filter((r) => r.name.startsWith("styles, ")), dropped = results.filter((r) => r.name.startsWith("dropped, "));
md += `\nOf those, ${styles.length} are comparisons of computed styles between the two builds (${styles.filter((r) => r.state === "ok").length} equal) — ${Math.round(compared / Math.max(comparisons, 1)).toLocaleString("en-US")} elements and pseudo-elements each on average — `;
md += `and ${dropped.length} ask whether a selector the page's sheet lacks matches an element (${dropped.filter((r) => r.state === "ok").length} find none; ${Math.round(droppedAsked / Math.max(comparisons, 1)).toLocaleString("en-US")} dropped selectors asked each time, on average).\n`;
const kind = (name) => name.replace(/^(styles|dropped), (.*?): .*$/, (all, what, state) => `${what === "styles" ? "styles" : "dropped selectors"}, ${state}`);
const names = [...new Set(results.map((r) => kind(r.name)))];
md += `\n| Check | ${engines.join(" | ")} |\n| --- | ${engines.map(() => "---").join(" | ")} |\n`;
for (const name of names) {
  md += `| ${name.replaceAll("|", "\\|")} | ${engines.map((e) => {
    const rs = results.filter((r) => kind(r.name) === name && r.engine === e);
    const ok = rs.filter((r) => r.state === "ok").length, known = rs.filter((r) => r.state === "known").length, failed = rs.filter((r) => r.state === "FAIL").length;
    return rs.length === 0 ? "—" : `${ok} of ${rs.length}${known ? `, ${known} known` : ""}${failed ? `, **${failed} failed**` : ""}`;
  }).join(" | ")} |\n`;
}
const said = (s) => results.filter((r) => r.state === s).map((r) => `- ${r.engine}${r.at ? ", " + r.at : ""}: ${r.name} — ${r.detail.replaceAll("\n      ", "; ")}`);
if (count("known")) md += `\nKnown — a limit of an engine or of this harness, said and not failed:\n\n${said("known").join("\n")}\n`;
if (count("FAIL")) md += `\n**Failed:**\n\n${said("FAIL").join("\n")}\n`;
md += `\nNot run: ${["chromium", "webkit", "firefox"].filter((e) => !engineList.includes(e)).join(", ") || "—"}; Safari proper; any touch device; the versions of the floor (Chrome 135, Firefox 147, Safari 26.2).\n`;
mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
if (!opts.keep) rmSync(work, { recursive: true, force: true });
console.log(`\n${count("ok")} passed, ${count("known")} known, ${count("FAIL")} failed — written: ${relative(process.cwd(), mdOut)}`);
for (const r of results.filter((r) => r.state === "FAIL")) console.log("FAILED:", r.engine, r.at, r.name);
process.exit(count("FAIL") ? 1 : 0);
