#!/usr/bin/env node
// verify.mjs — T7, behaviour (specs/phase02/plan.md, RGP2-050): the docs
// site as `reactogenic build` writes it, in real browsers.
//
//   node bench/verify.mjs [--binary <reactogenic>] [--engines chromium,webkit] [--inline never|always]
//                         [--md bench/results/verify.md] [--shots <dir>] [--keep <dir>]
//
// The site is built as it ships and with `--no-specialize` — the control: the
// same HTML, the whole site's CSS and script on every page — and both are
// served over HTTP. Then, in Chromium and WebKit, at 1200 and 400 px:
//
//   1. the components on real output (components.md): the side menu marks
//      the page (`aria-current`); it is a column at 1200 px and a drawer at
//      400 px, which opens, closes on its scrim without activating what is
//      behind, on its close button and on a link into the page; the links
//      menu opens anchored; the Install dialog opens modal and closes by
//      Esc, its close button, its scrim and its action, focus back on its
//      trigger; the action menu has arrow keys, Home, End, typeahead, Tab,
//      and its item opens the cheat sheet; the dialogs open where the engine
//      has no `command`; Back does not restore an open overlay;
//   2. pruning changes nothing that is seen (builder.md, *CSS*: "a dropped
//      rule must never have matched"): the computed style of every element
//      of every page, and of its ::before, ::after, ::marker and ::backdrop,
//      is the same in the two builds — at rest, in dark, with reduced
//      motion, under the pointer, with keyboard focus, and with each overlay
//      open;
//   3. the validity probe of the pruner (plan.md, RGP2-050): every
//      pseudo-class and pseudo-element `cssprune` takes for known to every
//      browser of the floor, and the `:nth-*()` forms it accepts, must parse
//      in the engine — a name it rejects makes the whole rule dead there,
//      and must leave the table (go/internal/build/cssprune/selector.go).
//
// Exit status: 1 on any failure. *Known* is a documented limit of an engine
// (components.md, *Known limits*) or of this harness, said and not failed.
//
// Playwright is packages/ui's devDependency and is taken from there, with the
// browsers it installed (`pnpm --filter @reactogenic/ui exec playwright
// install chromium webkit`). Chromium is the full browser, not the headless
// shell — which paints the page beside an open drawer wrongly — and with its
// back/forward cache on. Playwright's WebKit has no page cache: Back is
// Chromium's check. Firefox: `--engines firefox`, where it starts.
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, extname, join, relative, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { PAGES, binary, mustBuild, options, repo, site } from "./lib.mjs";

const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/verify.md"));
const engineList = String(opts.engines ?? "chromium,webkit").split(",");
const shots = opts.shots ? resolve(opts.shots) : null;
if (shots) mkdirSync(shots, { recursive: true });
const bin = binary(opts.binary);
const work = opts.keep ? resolve(opts.keep) : mkdtempSync(join(tmpdir(), "reactogenic-bench-verify-"));
const flags = opts.inline ? ["--inline", String(opts.inline)] : [];
const builtDir = join(work, "default"), controlDir = join(work, "control");
mustBuild(bin, site, builtDir, flags);
mustBuild(bin, site, controlDir, [...flags, "--no-specialize"]);

const require = createRequire(join(repo, "packages/ui/package.json"));
const pw = await import(pathToFileURL(require.resolve("playwright")).href).then((m) => m.default ?? m);

// ---- serving ---------------------------------------------------------------------
const types = { ".html": "text/html; charset=utf-8", ".css": "text/css", ".js": "text/javascript", ".svg": "image/svg+xml", ".json": "application/json" };
// `noScript`: the same pages without the builder's script — what Back does
// to a page that nothing closes (the control of that check).
function serve(dir, { noScript = false } = {}) {
  const reports = [];
  const server = createServer((request, response) => {
    if (request.method === "POST") {
      let body = "";
      request.on("data", (chunk) => (body += chunk));
      request.on("end", () => { reports.push(JSON.parse(body)); response.writeHead(204).end(); });
      return;
    }
    let path = join(dir, decodeURIComponent(new URL(request.url, "http://x").pathname));
    if (existsSync(path) && statSync(path).isDirectory()) path = join(path, "index.html");
    if (!existsSync(path)) return void response.writeHead(404).end("not found");
    let body = readFileSync(path);
    if (noScript && path.endsWith(".html")) body = Buffer.from(body.toString().replace(/<script type="module"[^>]*>[\s\S]*?<\/script>(?=<\/body>)/, ""));
    // No `no-store`: a page that says so is not kept by the back/forward cache.
    response.writeHead(200, { "content-type": types[extname(path)] ?? "application/octet-stream" }).end(body);
  });
  const report = async () => {
    for (let waited = 0; reports.length === 0 && waited < 3000; waited += 100) await new Promise((done) => setTimeout(done, 100));
    return reports.shift();
  };
  return new Promise((done) => server.listen(0, "127.0.0.1", () => done({ origin: `http://127.0.0.1:${server.address().port}`, report, close: () => server.close() })));
}

// ---- checks ----------------------------------------------------------------------
const WIDE = 1200, NARROW = 400;
const results = []; // { engine, at, name, state: "ok" | "known" | "FAIL", detail }
let engine = "";
const record = (state, at, name, detail = "") => {
  results.push({ engine, at, name, state, detail });
  console.log(state === "ok" ? "ok   " : state === "known" ? "known" : "FAIL ", at ? `${at}: ${name}` : name, state === "ok" ? "" : detail);
};
const check = (at, name, ok, detail = "") => record(ok ? "ok" : "FAIL", at, name, ok ? "" : detail);
// A documented limit: said, not failed.
const limit = (at, name, ok, detail = "") => record(ok ? "ok" : "known", at, name, ok ? "" : detail);

const settle = (page) => page.evaluate(async () => {
  await Promise.all(document.getAnimations().map((a) => a.finished.catch(() => {})));
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
});
const active = (page) => page.evaluate(() => {
  const el = document.activeElement;
  return el === document.body ? "body" : el.id || el.textContent.trim().slice(0, 40) || el.getAttribute("data-part") || el.localName;
});
const state = (page, id) => page.evaluate((id) => {
  const el = document.getElementById(id);
  const r = el.getBoundingClientRect();
  return {
    open: el.localName === "dialog" ? el.open && el.matches(":modal") : el.matches(":popover-open"),
    display: getComputedStyle(el).display,
    rect: { left: r.left, top: r.top, right: r.right, bottom: r.bottom, width: r.width, height: r.height },
    viewport: { width: innerWidth, height: innerHeight },
  };
}, id);
const rect = (page, selector) => page.evaluate((selector) => {
  const r = document.querySelector(selector).getBoundingClientRect();
  return { rect: { left: r.left, top: r.top, right: r.right, bottom: r.bottom, width: r.width, height: r.height }, viewport: { width: innerWidth, height: innerHeight } };
}, selector);
const inside = (s, slack = 1) => s.rect.left >= -slack && s.rect.top >= -slack && s.rect.right <= s.viewport.width + slack && s.rect.bottom <= s.viewport.height + slack;

// Every element's computed style — and its ::before, ::after, ::marker,
// ::backdrop — but for the elements packaging writes, which differ between
// the two builds by design. Custom properties apart: one that nothing reads
// may be dropped, and that is said, not failed.
const snapshot = () => {
  const pseudos = [null, "::before", "::after", "::marker", "::backdrop"];
  const out = [];
  const skip = (el) => el.localName === "style" || el.localName === "script" || (el.localName === "link" && el.rel === "stylesheet");
  for (const el of document.querySelectorAll("*")) {
    if (skip(el)) continue;
    for (const ps of pseudos) {
      const cs = getComputedStyle(el, ps);
      let text = "", custom = "";
      for (let i = 0; i < cs.length; i++) {
        const k = cs[i];
        if (k.startsWith("--")) custom += k + ":" + cs.getPropertyValue(k) + ";";
        else text += k + ":" + cs.getPropertyValue(k) + ";";
      }
      // Chromium enumerates custom properties in no fixed order.
      custom = custom.split(";").sort().join(";");
      const label = el.localName + (el.id ? "#" + el.id : "") + (typeof el.className === "string" && el.className ? "." + el.className.replace(/\s+/g, ".") : "") + (ps || "");
      out.push([label, text, custom]);
    }
  }
  return out;
};
let comparisons = 0, compared = 0;
// The differences between the two pages' computed styles: a list of lines.
async function differences(a, b) {
  await settle(a); await settle(b);
  // What a closed <details> holds is laid out lazily, and the first read of
  // a size in it may be the one before layout (seen in Chromium, comparing a
  // build with itself): every element's size is read once before the styles.
  const layout = () => { for (const el of document.querySelectorAll("*")) void getComputedStyle(el).blockSize; };
  await a.evaluate(layout); await b.evaluate(layout);
  const [x, y] = [await a.evaluate(snapshot), await b.evaluate(snapshot)];
  if (x.length !== y.length) return { count: x.length, diffs: [`${x.length} against ${y.length} elements and pseudo-elements`], custom: 0 };
  const diffs = [];
  let custom = 0;
  const decls = (text) => Object.fromEntries(text.split(";").map((d) => [d.slice(0, d.indexOf(":")), d.slice(d.indexOf(":") + 1)]));
  for (let i = 0; i < x.length; i++) {
    if (x[i][0] !== y[i][0]) { diffs.push(`element ${i}: ${x[i][0]} against ${y[i][0]}`); continue; }
    if (x[i][1] !== y[i][1]) {
      const p = decls(x[i][1]), q = decls(y[i][1]);
      for (const k in p) if (p[k] !== q[k]) diffs.push(`${x[i][0]} { ${k}: ${p[k]} (pruned) / ${q[k]} (unpruned) }`);
    } else if (x[i][2] !== y[i][2]) custom++;
  }
  return { count: x.length, diffs, custom };
}
async function compare(at, name, a, b) {
  const d = await differences(a, b);
  comparisons++; compared += d.count;
  check(at, `styles, ${name}: the two builds are equal`, d.diffs.length === 0, `${d.count} elements and pseudo-elements, ${d.diffs.length} differences:\n      ${d.diffs.slice(0, 8).join("\n      ")}`);
  if (d.custom) console.log(`      (${d.custom} elements differ in the custom properties they carry: nothing reads them)`);
}

// ---- the probe's tables, read off the pruner's source -----------------------------
const selectorGo = readFileSync(join(repo, "go/internal/build/cssprune/selector.go"), "utf8");
const table = (name) => {
  const m = new RegExp(`var ${name} = set\\(([\\s\\S]*?)\\)`).exec(selectorGo);
  if (!m) throw new Error(`cssprune/selector.go has no ${name}: the probe has nothing to ask`);
  return [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
};
const PROBE = [
  ...table("knownPseudoClass").map((name) => `:${name}`),
  ...table("knownPseudoElement").map((name) => `::${name}`),
  ...table("legacyPseudoElement").map((name) => `:${name}`),
  // What validNth accepts: An+B in its forms, `even`, `odd` — without `of S`.
  ...table("knownNth").flatMap((name) => ["even", "odd", "3", "+3", "-3", "n", "+n", "-n", "2n", "2n+1", "2n-1", "-n+3", "2n + 1", "2n - 1", "EVEN", "2N+1"].map((arg) => `:${name}(${arg})`)),
];

for (const engineName of engineList) {
  // The full Chromium, with its back/forward cache: Playwright turns it off.
  const browser = await pw[engineName].launch(engineName === "chromium" ? { channel: "chromium", ignoreDefaultArgs: ["--disable-back-forward-cache"] } : {});
  engine = `${engineName} ${browser.version()}`;
  console.log(`\n=== ${engine}`);
  // Safari's Tab stops at text fields and at little else, as macOS has it by
  // default; Option-Tab stops at every link and button. Playwright's WebKit
  // is the same: at 400 px, where the drawer is closed, Tab finds nothing.
  const TAB = engineName === "webkit" ? "Alt+Tab" : "Tab";
  const built = await serve(builtDir), control = await serve(controlDir), bare = await serve(builtDir, { noScript: true });
  const errors = [];
  const watch = (page, what) => {
    page.on("pageerror", (e) => errors.push(`${what}: ${e}`));
    page.on("console", (m) => m.type() === "error" && errors.push(`${what}: console ${m.text()}`));
    page.on("requestfailed", (r) => errors.push(`${what}: request failed ${r.url()}`));
    page.on("response", (r) => r.status() >= 400 && errors.push(`${what}: ${r.status()} ${r.url()}`));
  };

  for (const width of [WIDE, NARROW]) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage(), other = await context.newPage();
    watch(page, "default"); watch(other, "control");
    const both = async (f) => { await f(page); await f(other); };
    const shot = async (name) => { if (shots) await page.screenshot({ path: join(shots, `${engineName}-${width}-${name}.png`) }); };
    // The pointer over the first element both pages show for `selector`: is it
    // under it? null when the page shows none (a link in a closed group).
    const hover = async (selector) => {
      const shown = (p) => p.locator(selector).filter({ visible: true }).first();
      if ((await shown(page).count()) === 0) return null;
      await both(async (p) => { await shown(p).scrollIntoViewIfNeeded(); await shown(p).hover(); });
      return (await shown(page).evaluate((el) => el.matches(":hover"))) && (await shown(other).evaluate((el) => el.matches(":hover")));
    };
    // … and what the two builds make of it. Nothing to compare when no such element is shown.
    const under = async (at, what, selector, name = `hover on ${what}`) => {
      const over = await hover(selector);
      if (over === null) return;
      check(at, `the pointer is over ${what}`, over);
      await compare(at, name, page, other);
    };
    const away = () => both((p) => p.mouse.move(width - 2, 798));
    const media = (options) => both((p) => p.emulateMedia(options));

    for (const pathname of PAGES) {
      const slug = pathname === "/" ? "home" : pathname.replaceAll("/", " ").trim().replaceAll(" ", "-");
      const at = `${width}px ${pathname}`;
      await page.goto(built.origin + pathname);
      await other.goto(control.origin + pathname);
      await both((p) => p.waitForLoadState("load"));
      await settle(page);
      await shot(slug);

      // ---- the page ----
      const doc = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth,
        scripts: document.scripts.length, react: document.documentElement.outerHTML.includes("__reactFiber") || "React" in window || "__REACT_DEVTOOLS_GLOBAL_HOOK__" in window,
      }));
      check(at, "no sideways scroll", doc.scroll <= doc.client, `scrollWidth ${doc.scroll} > ${doc.client}`);
      check(at, "one script, the builder's; no React", doc.scripts === 1 && !doc.react, JSON.stringify(doc));

      // ---- aria-current ----
      const current = await page.evaluate(() => {
        const marked = [...document.querySelectorAll("[aria-current]")];
        const details = [...document.querySelectorAll("#s1 details")];
        return {
          marked: marked.map((a) => [a.getAttribute("href"), a.getAttribute("aria-current"), !!a.closest("#s1")]),
          around: marked.length === 1 && (() => { for (let d = marked[0].closest("details"); d; d = d.parentElement.closest("details")) if (!d.open) return false; return true; })(),
          open: details.filter((d) => d.open).length, groups: details.length,
          weight: marked.length === 1 ? getComputedStyle(marked[0]).fontWeight : "",
          plain: getComputedStyle(document.querySelector("#s1 a:not([aria-current])")).fontWeight,
        };
      });
      check(at, 'aria-current="page" is on this page\'s link of the side menu, and on nothing else',
        current.marked.length === 1 && current.marked[0][0] === pathname && current.marked[0][1] === "page" && current.marked[0][2], JSON.stringify(current.marked));
      check(at, "… its group is the one group that is open, and it is styled", current.around && current.open === (pathname === "/" ? 0 : 1) && current.groups === 3 && current.weight !== current.plain, JSON.stringify(current));

      // ---- at rest ----
      await compare(at, "at rest", page, other);
      await media({ colorScheme: "dark" });
      const dark = await page.evaluate(() => [matchMedia("(prefers-color-scheme: dark)").matches, getComputedStyle(document.body).backgroundColor]);
      check(at, "dark: the page follows the OS", dark[0] && dark[1] !== "rgb(255, 255, 255)", JSON.stringify(dark));
      await compare(at, "dark, at rest", page, other);
      if (pathname === "/") await shot("home-dark");
      await media({ colorScheme: "light", reducedMotion: "reduce" });
      await compare(at, "reduced motion, at rest", page, other);
      await media({ reducedMotion: "no-preference" });

      // ---- under the pointer ----
      for (const [what, selector] of [["the links menu's trigger", '[popovertarget="m1"]'], ["a link of the text", "main a[href]"], ...(width === WIDE ? [["a link of the side menu", "#s1 a:not([aria-current])"], ["the current link of the side menu", "#s1 a[aria-current]"], ["a group's summary", "#s1 summary"]] : [])]) {
        await under(at, what, selector);
      }
      await away();

      // ---- keyboard focus: the first stops of the page ----
      await both((p) => p.keyboard.press(TAB));
      await both((p) => p.keyboard.press(TAB));
      const focused = await page.evaluate(() => document.activeElement.matches(":focus-visible"));
      check(at, "Tab gives an element keyboard focus", focused, await active(page));
      await compare(at, "keyboard focus (:focus-visible)", page, other);
      await both((p) => p.evaluate(() => document.activeElement.blur()));

      // ---- the links menu ----
      await both((p) => p.getByRole("button", { name: "Links", exact: true }).click());
      await settle(page);
      const menu = await state(page, "m1");
      const trigger = (await rect(page, '[popovertarget="m1"]')).rect;
      check(at, "the links menu opens", menu.open && menu.display !== "none", JSON.stringify(menu));
      check(at, "… anchored below its trigger, end-aligned, in the viewport",
        Math.abs(menu.rect.top - trigger.bottom) <= 8 && Math.abs(menu.rect.right - trigger.right) <= 2 && inside(menu), JSON.stringify({ menu: menu.rect, trigger }));
      const role = await page.evaluate(() => [document.getElementById("m1").getAttribute("role"), document.querySelectorAll("#m1 [role]").length, document.querySelectorAll("#m1 a[href]").length]);
      check(at, "… a list of links: no menu role, so no arrow keys are promised", role[0] === null && role[1] === 0 && role[2] === 3, JSON.stringify(role));
      if (pathname === "/") await shot("links-menu");
      await compare(at, "links menu open", page, other);
      await under(at, "an item of the links menu", "#m1 a", "links menu open, hover on an item");
      await both((p) => p.keyboard.press(TAB));
      check(at, "Tab moves from the trigger into the open links menu", await page.evaluate(() => !!document.activeElement.closest("#m1") && document.activeElement.matches(":focus-visible")), await active(page));
      await compare(at, "links menu open, keyboard focus on an item", page, other);
      await both((p) => p.keyboard.press("Escape"));
      check(at, "Esc closes the links menu", !(await state(page, "m1")).open);
      await away();

      // ---- the side menu: a column, or a drawer ----
      const nav = await page.evaluate(() => {
        const nav = document.getElementById("s1"), toggle = document.querySelector(".rg-sidemenu-toggle");
        const r = nav.getBoundingClientRect();
        return { display: getComputedStyle(nav).display, position: getComputedStyle(nav).position, toggle: getComputedStyle(toggle).display, left: r.left, top: r.top, width: r.width, height: r.height, open: nav.matches(":popover-open") };
      });
      if (width === WIDE) {
        check(at, "the side menu is a sticky column, and there is no toggle", nav.display !== "none" && !nav.open && nav.toggle === "none" && nav.position === "sticky" && nav.left === 0 && nav.width > 150 && nav.width < 400 && nav.height > 300, JSON.stringify(nav));
        const content = (await rect(page, ".site-content")).rect.left;
        check(at, "… beside the content", content >= nav.width - 1, `content starts at ${content}, the column is ${nav.width} wide`);
      } else {
        check(at, "the side menu is closed, and there is a toggle", nav.display === "none" && nav.toggle !== "none", JSON.stringify(nav));
        await both((p) => p.locator(".rg-sidemenu-toggle").click());
        await settle(page);
        const drawer = await state(page, "s1");
        check(at, "the toggle opens the drawer, in the viewport", drawer.open && drawer.rect.left === 0 && drawer.rect.width > 150 && drawer.rect.width < width && drawer.rect.height >= 700, JSON.stringify(drawer));
        if (pathname === "/" || pathname === "/syntax/") await shot(`${slug}-drawer`);
        await compare(at, "drawer open", page, other);
        await media({ colorScheme: "dark" });
        await compare(at, "dark, drawer open", page, other);
        await media({ colorScheme: "light" });
        await under(at, "a link of the drawer", "#s1 a:not([aria-current])", "drawer open, hover on a link");
        await under(at, "a group's summary in the drawer", "#s1 summary", "drawer open, hover on a group's summary");
        // A group of another page opens in it.
        await both((p) => p.locator("#s1 summary").last().click());
        await compare(at, "drawer open, a group toggled", page, other);
        await both((p) => p.locator("#s1 summary").last().click());
        // The scrim: the rest of the viewport.
        await both((p) => p.mouse.click(width - 10, 400));
        await settle(page);
        check(at, "a click on the scrim closes the drawer", !(await state(page, "s1")).open);
        const stayed = await page.evaluate(() => location.pathname);
        check(at, "… and activates nothing behind it", stayed === pathname, stayed);
        await both((p) => p.locator(".rg-sidemenu-toggle").click());
        await both((p) => p.locator('#s1 > [data-part="close"]').click());
        check(at, "the close button closes the drawer", !(await state(page, "s1")).open);
        // A link into this page: the drawer must not stay over what was asked for.
        if (pathname !== "/") {
          const into = `#s1 a[href^="${pathname}#"]`;
          await both((p) => p.locator(".rg-sidemenu-toggle").click());
          await settle(page);
          await both((p) => p.locator(into).first().click());
          await settle(page);
          const after = { open: (await state(page, "s1")).open, hash: await page.evaluate(() => location.hash), api: await page.evaluate(() => "navigation" in window) };
          (after.api ? check : limit)(at, "a link of the drawer into this page closes the drawer", !after.open && after.hash !== "", after.api ? JSON.stringify(after) : "the engine has no Navigation API (components.md, Known limits)");
          await both((p) => p.keyboard.press("Escape"));
          await both((p) => p.evaluate(() => scrollTo(0, 0)));
          // WebKit keeps `:hover` on what was under the pointer until it moves.
          await away();
        }
      }

      // ---- the Install dialog ----
      if (pathname === "/") {
        const openByKey = async () => { await both(async (p) => { await p.getByRole("button", { name: "Install", exact: true }).focus(); await p.keyboard.press("Enter"); }); await settle(page); };
        await under(at, "the Install button", 'button[commandfor="d1"]');
        await both((p) => p.getByRole("button", { name: "Install", exact: true }).click());
        await settle(page);
        const d = await state(page, "d1");
        check(at, "Install opens the dialog, modal", d.open, JSON.stringify(d));
        check(at, "… its panel in the viewport", inside(await rect(page, '#d1 > [data-part="panel"]')));
        const named = await page.evaluate(() => document.getElementById(document.getElementById("d1").getAttribute("aria-labelledby"))?.textContent);
        check(at, "… named by its title", named === "Install", String(named));
        await shot("install-dialog");
        await compare(at, "Install dialog open", page, other);
        await media({ colorScheme: "dark" });
        await compare(at, "dark, Install dialog open", page, other);
        await media({ colorScheme: "light", reducedMotion: "reduce" });
        await compare(at, "reduced motion, Install dialog open", page, other);
        await media({ reducedMotion: "no-preference" });
        await under(at, "the dialog's close button", '#d1 header [data-part="close"]', "Install dialog open, hover on its close button");
        await both((p) => p.keyboard.press(TAB));
        check(at, "Tab stays in the open dialog", await page.evaluate(() => !!document.activeElement.closest("#d1") && document.activeElement.matches(":focus-visible")), await active(page));
        await compare(at, "Install dialog open, keyboard focus in it", page, other);
        await away();
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(at, "Esc closes it", !(await state(page, "d1")).open);
        limit(at, "… and focus is back on Install (opened by a click)", (await active(page)) === "Install", `focus on ${await active(page)}: WebKit does not focus a button on click (components.md, Known limits)`);

        await openByKey();
        check(at, "Enter on Install opens it", (await state(page, "d1")).open);
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(at, "Esc closes it, focus back on Install", !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.locator('#d1 header [data-part="close"]').click());
        await settle(page);
        check(at, "the close button closes it, focus back on Install", !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.mouse.click(8, 8));
        await settle(page);
        check(at, "a click on the scrim closes it, focus back on Install", !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.locator("#d1 footer").getByRole("button", { name: "Close" }).click());
        await settle(page);
        check(at, "the Close action closes it, focus back on Install", !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));
      }

      // ---- the action menu and the cheat sheet ----
      if (pathname === "/syntax/") {
        const openMenu = async () => { await both(async (p) => { await p.locator("#m2-t").focus(); await p.keyboard.press("Enter"); }); await settle(page); };
        await openMenu();
        const m = await state(page, "m2");
        const t = (await rect(page, "#m2-t")).rect;
        check(at, "the action menu opens, anchored below its trigger, in the viewport", m.open && Math.abs(m.rect.top - t.bottom) <= 8 && Math.abs(m.rect.left - t.left) <= 2 && inside(m), JSON.stringify({ m, t }));
        check(at, "… focus on its first item", (await active(page)) === "Cheat sheet…", await active(page));
        await shot("action-menu");
        await compare(at, "action menu open", page, other);
        const press = async (key) => { await both((p) => p.keyboard.press(key)); return active(page); };
        const seen = [await press("ArrowDown"), await press("ArrowDown"), await press("ArrowDown"), await press("ArrowUp"), await press("Home"), await press("End")];
        check(at, "ArrowDown, ArrowDown, ArrowDown (wraps), ArrowUp (wraps), Home, End",
          JSON.stringify(seen) === JSON.stringify(["Specification on GitHub", "Getting started", "Cheat sheet…", "Getting started", "Cheat sheet…", "Getting started"]), JSON.stringify(seen));
        await compare(at, "action menu open, keyboard focus on its last item", page, other);
        const typed = [await press("s"), await press("g"), await press("c")];
        check(at, "typeahead: s, g, c", JSON.stringify(typed) === JSON.stringify(["Specification on GitHub", "Getting started", "Cheat sheet…"]), JSON.stringify(typed));
        await under(at, "an item of the action menu", "#m2 [role=menuitem]:last-child", "action menu open, hover on an item");
        await away();
        await both((p) => p.locator("#m2 [role=menuitem]").first().focus());
        await both((p) => p.keyboard.press("Tab"));
        await settle(page);
        check(at, "Tab closes the action menu", !(await state(page, "m2")).open);

        await openMenu();
        await both((p) => p.keyboard.press("Enter"));
        await settle(page);
        const sheet = await state(page, "cheat-sheet");
        check(at, "Enter on its first item opens the cheat sheet, modal, and closes the menu", sheet.open && !(await state(page, "m2")).open, JSON.stringify(sheet));
        check(at, "… its panel in the viewport", inside(await rect(page, '#cheat-sheet > [data-part="panel"]')));
        await shot("cheat-sheet");
        await compare(at, "cheat sheet open", page, other);
        await media({ colorScheme: "dark" });
        await compare(at, "dark, cheat sheet open", page, other);
        await media({ colorScheme: "light" });
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(at, "Esc closes the cheat sheet, focus back on the menu's trigger", !(await state(page, "cheat-sheet")).open && (await active(page)) === "m2-t", await active(page));

        // By pointer too.
        await both((p) => p.locator("#m2-t").click());
        await both((p) => p.getByRole("menuitem", { name: "Cheat sheet…" }).click());
        await settle(page);
        check(at, "by pointer: the item opens the cheat sheet and closes the menu", (await state(page, "cheat-sheet")).open && !(await state(page, "m2")).open);
        await both((p) => p.mouse.click(8, 8));
        await settle(page);
        check(at, "… and a click on its scrim closes it", !(await state(page, "cheat-sheet")).open);
      }

      // A link of the side menu into a page: the section is reached, below the header.
      if (pathname === "/guide/" && width === WIDE) {
        await page.locator('#s1 a[href$="/guide/#check"]').click();
        await settle(page);
        const top = await page.evaluate(() => document.getElementById("check").getBoundingClientRect().top);
        check(at, "#check lands below the sticky header", top >= 40 && top < 200, `top ${top}`);
      }
    }
    await context.close();
  }

  // ---- the comparison sees a difference ----
  {
    const context = await browser.newContext({ viewport: { width: WIDE, height: 800 } });
    const page = await context.newPage(), other = await context.newPage();
    await page.goto(built.origin + "/guide/"); await other.goto(built.origin + "/guide/");
    const same = await differences(page, other);
    // One rule that matches taken out of one page's sheet, as a wrong pruner would.
    const taken = await page.evaluate(() => {
      const visit = (rules) => {
        for (let i = 0; i < rules.length; i++) {
          if (rules[i].selectorText?.includes("[aria-current]") && document.querySelector(rules[i].selectorText)) { const text = rules[i].selectorText; rules[i].parentRule ? rules[i].parentRule.deleteRule(i) : rules[i].parentStyleSheet.deleteRule(i); return text; }
          if (rules[i].cssRules) { const found = visit(rules[i].cssRules); if (found) return found; }
        }
        return null;
      };
      for (const sheet of document.styleSheets) { const found = visit(sheet.cssRules); if (found) return found; }
      return null;
    });
    const broken = await differences(page, other);
    check("", "the comparison: a build against itself is equal, and with one rule taken out of one page it is not", same.diffs.length === 0 && taken !== null && broken.diffs.length > 0, JSON.stringify({ same: same.diffs.length, taken, broken: broken.diffs.length }));
    await context.close();
  }

  // ---- the dialogs where the engine has no `command`: the invokers behaviour ----
  {
    const context = await browser.newContext({ viewport: { width: WIDE, height: 800 } });
    await context.addInitScript(() => {
      delete HTMLButtonElement.prototype.command;
      delete HTMLButtonElement.prototype.commandForElement;
      addEventListener("command", (event) => event.preventDefault(), true);
    });
    const page = await context.newPage();
    watch(page, "no command");
    await page.goto(built.origin + "/");
    await page.getByRole("button", { name: "Install", exact: true }).click();
    check("", "without the engine's `command`, the page's script opens the Install dialog", (await state(page, "d1")).open);
    await page.locator('#d1 header [data-part="close"]').click();
    check("", "… and closes it", !(await state(page, "d1")).open);
    await page.goto(built.origin + "/syntax/");
    await page.locator("#m2-t").click();
    await page.getByRole("menuitem", { name: "Cheat sheet…" }).click();
    check("", "… and the menu's item opens the cheat sheet", (await state(page, "cheat-sheet")).open);
    // The same page with no script at all: the button is dead. That is what the behaviour is for.
    const dead = await context.newPage();
    await dead.goto(bare.origin + "/");
    await dead.getByRole("button", { name: "Install", exact: true }).click();
    check("", "(without the script and without `command`, the button is dead: the check can fail)", !(await dead.evaluate(() => document.getElementById("d1").open)));
    await context.close();
  }

  // ---- Back to a page left with an open overlay ----
  {
    // Leaves `path` through a link inside an open overlay, comes Back, and
    // says whether the page was restored from the cache and what is open.
    const leaveAndReturn = async (server, path, width, leave) => {
      const context = await browser.newContext({ viewport: { width, height: 800 } });
      const page = await context.newPage();
      await page.goto(server.origin + path);
      // Only a page that is restored keeps this listener: a page that is
      // loaded again is a new document, and reports nothing.
      await page.evaluate(() => addEventListener("pageshow", (event) => fetch("/", { method: "POST", body: JSON.stringify({ restored: event.persisted, open: [...document.querySelectorAll(":popover-open, dialog[open]")].map((el) => el.id) }) })));
      await leave(page);
      await page.waitForURL((url) => url.pathname !== path);
      // Not page.goBack(): it waits for a load, and a restored page fires none.
      await page.evaluate(() => history.back()).catch(() => {});
      const state = (await server.report()) ?? { restored: false, open: [] };
      await context.close();
      return state;
    };
    const cases = {
      "drawer": ["/guide/", NARROW, async (page) => { await page.click(".rg-sidemenu-toggle"); await settle(page); await page.click('#s1 a[href="/"]'); }],
      "action menu": ["/syntax/", WIDE, async (page) => { await page.click("#m2-t"); await settle(page); await page.click('#m2 a[href="/guide/"]'); }],
      "Install dialog": ["/", WIDE, async (page) => { await page.click('button[commandfor="d1"]'); await settle(page); await page.click("#d1 footer a"); }],
    };
    for (const [what, [path, width, leave]] of Object.entries(cases)) {
      const back = await leaveAndReturn(built, path, width, leave);
      const cached = back.restored === true;
      (cached || engineName === "chromium" ? check : limit)("", `Back to ${path}, left with its ${what} open: restored from the page cache, and nothing is open`, cached && back.open.length === 0,
        cached || engineName === "chromium" ? JSON.stringify(back) : "the page was loaded again: Playwright's WebKit has no page cache, so there was nothing to restore");
      const naked = await leaveAndReturn(bare, path, width, leave);
      (naked.restored === true || engineName === "chromium" ? check : limit)("", `(the same page without its script comes back with the ${what} open: the check can fail)`, naked.restored === true && naked.open.length === 1,
        naked.restored === true || engineName === "chromium" ? JSON.stringify(naked) : "no page cache");
    }
  }

  // ---- the pruner's tables of what every browser parses ----
  {
    const page = await browser.newPage();
    await page.goto(built.origin + "/guide/");
    // A selector the engine cannot parse makes it drop the whole rule.
    const rejected = await page.evaluate((selectors) => selectors.filter((selector) => {
      const sheet = new CSSStyleSheet();
      sheet.replaceSync(`${selector}, p {}`);
      return sheet.cssRules.length !== 1;
    }), PROBE);
    const sure = await page.evaluate(() => { const sheet = new CSSStyleSheet(); sheet.replaceSync(":no-such-thing, p {} ::no-such-thing, p {} :nth-child(2n of), p {}"); return sheet.cssRules.length; });
    check("", `the probe: \`SEL, p {}\` is one rule for each of the ${PROBE.length} selectors the pruner takes for known`, rejected.length === 0, `the engine rejects ${rejected.join(" ")}: they must leave cssprune's tables`);
    check("", "(a selector no engine knows leaves no rule: the probe can fail)", sure === 0, `${sure} rules`);
    await page.close();
  }

  check("", "no page error, console error or failed request", errors.length === 0, errors.slice(0, 5).join(" | "));
  built.close(); control.close(); bare.close();
  await browser.close();
}

// ---- the report --------------------------------------------------------------------
const count = (s, e) => results.filter((r) => r.state === s && (!e || r.engine === e)).length;
const engines = [...new Set(results.map((r) => r.engine))];
let md = `# The built site in a browser\n\nWritten by \`node bench/verify.mjs${opts.inline ? " --inline " + opts.inline : ""}\` (specs/phase02/plan.md, RGP2-050, T7). The site as \`reactogenic build${flags.length ? " " + flags.join(" ") : ""}\` writes it, and the control (\`--no-specialize\`), served over HTTP; every page at ${WIDE} and ${NARROW} px.\n\n`;
md += `| Engine | Passed | Known | Failed |\n| --- | ---: | ---: | ---: |\n`;
for (const e of engines) md += `| ${e} | ${count("ok", e)} | ${count("known", e)} | ${count("FAIL", e)} |\n`;
const styles = results.filter((r) => r.name.startsWith("styles, "));
md += `\nOf those, ${styles.length} are comparisons of computed styles between the two builds (${styles.filter((r) => r.state === "ok").length} equal): ${Math.round(compared / Math.max(comparisons, 1)).toLocaleString("en-US")} elements and pseudo-elements each on average, custom properties included.\n`;
const names = [...new Set(results.map((r) => r.name))];
md += `\n| Check | ${engines.join(" | ")} |\n| --- | ${engines.map(() => "---").join(" | ")} |\n`;
for (const name of names) {
  md += `| ${name.replaceAll("|", "\\|")} | ${engines.map((e) => {
    const rs = results.filter((r) => r.name === name && r.engine === e);
    const ok = rs.filter((r) => r.state === "ok").length, known = rs.filter((r) => r.state === "known").length, failed = rs.filter((r) => r.state === "FAIL").length;
    return rs.length === 0 ? "—" : `${ok} of ${rs.length}${known ? `, ${known} known` : ""}${failed ? `, **${failed} failed**` : ""}`;
  }).join(" | ")} |\n`;
}
const said = (s) => results.filter((r) => r.state === s).map((r) => `- ${r.engine}${r.at ? ", " + r.at : ""}: ${r.name} — ${r.detail.split("\n")[0]}`);
if (count("known")) md += `\nKnown — a documented limit, said and not failed:\n\n${said("known").join("\n")}\n`;
if (count("FAIL")) md += `\n**Failed:**\n\n${said("FAIL").join("\n")}\n`;
md += `\nNot run: ${["chromium", "webkit", "firefox"].filter((e) => !engineList.includes(e)).join(", ") || "—"}; Safari proper; any touch device; the versions of the floor (Chrome 135, Firefox 147, Safari 26.2).\n`;
mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
if (!opts.keep) rmSync(work, { recursive: true, force: true });
console.log(`\n${count("ok")} passed, ${count("known")} known, ${count("FAIL")} failed — written: ${relative(process.cwd(), mdOut)}`);
for (const r of results.filter((r) => r.state === "FAIL")) console.log("FAILED:", r.engine, r.at, r.name);
process.exit(count("FAIL") ? 1 : 0);
