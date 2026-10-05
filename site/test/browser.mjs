// Browser checks of the built docs site (specs/phase02/plan.md, RGP2-040).
// The site is built twice by `reactogenic build` — as it ships, and with
// `--no-specialize`: the same HTML, and no pruning — and both are served
// over HTTP. Then, in Chromium and WebKit, at 1200 and 400 px, on every page:
//
//   - the page: no sideways scroll, one script — the builder's — and no
//     error; the side menu marks the page and opens its group;
//   - the components on real output (components.md): the links menu opens
//     anchored; the drawer opens, closes on its scrim and on its button, and
//     is a column above the breakpoint; the Install dialog opens and closes
//     by Esc, its close button, its scrim and its action, with focus back on
//     its trigger; the action menu has arrow keys and typeahead and opens
//     the cheat sheet;
//   - pruning changes nothing that is seen (builder.md, *CSS*: "a dropped
//     rule must never have matched"): the computed style of every element
//     and of its ::before, ::after, ::marker and ::backdrop is the same in
//     the two builds — as loaded, and with each overlay open.
//
//   pnpm --filter @reactogenic/site test:browser
//   ENGINES=chromium BASE=/reactogenic/ INLINE=never SHOTS=/tmp/shots pnpm --filter @reactogenic/site test:browser
//
// The binary is $REACTOGENIC_BINARY, or built from ../go (packages/ui's
// helper). Playwright is packages/ui's devDependency, and is taken from
// there. Firefox: ENGINES=firefox, where it starts.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { extname, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { binary } from "../../packages/ui/test/binary.mjs";

const site = resolve(import.meta.dirname, "..");
const base = process.env.BASE ?? "/";
const engineList = process.env.ENGINES ?? "chromium,webkit";
const shots = process.env.SHOTS; // a directory for screenshots; without it none is taken
if (shots) mkdirSync(shots, { recursive: true });

const out = mkdtempSync(join(tmpdir(), "reactogenic-site-"));
const builtDir = join(out, "default"), controlDir = join(out, "control");
const flags = [...(base === "/" ? [] : ["--base", base]), ...(process.env.INLINE ? ["--inline", process.env.INLINE] : [])];
for (const [dir, more] of [[builtDir, []], [controlDir, ["--no-specialize"]]]) {
  const built = spawnSync(binary(), ["build", "-p", join(site, "tsconfig.json"), "--out", dir, ...flags, ...more], { stdio: "inherit" });
  if (built.status !== 0) process.exit(built.status ?? 1);
}

const require = createRequire(resolve(site, "../packages/ui/package.json"));
const pw = await import(pathToFileURL(require.resolve("playwright")).href).then((m) => m.default ?? m);

const types = { ".html": "text/html; charset=utf-8", ".css": "text/css", ".js": "text/javascript", ".svg": "image/svg+xml", ".json": "application/json" };
function serve(dir) {
  const server = createServer((request, response) => {
    const pathname = decodeURIComponent(new URL(request.url, "http://x").pathname);
    if (!pathname.startsWith(base)) return void response.writeHead(404).end("outside the base");
    let path = join(dir, pathname.slice(base.length));
    if (existsSync(path) && statSync(path).isDirectory()) path = join(path, "index.html");
    if (!existsSync(path)) return void response.writeHead(404).end("not found");
    response.writeHead(200, { "content-type": types[extname(path)] ?? "application/octet-stream" }).end(readFileSync(path));
  });
  return new Promise((done) => server.listen(0, "127.0.0.1", () => done({ origin: `http://127.0.0.1:${server.address().port}`, close: () => server.close() })));
}

const PAGES = ["/", "/guide/", "/syntax/", "/reference/cli/"];
const WIDTHS = [1200, 400];
let passed = 0, failed = 0, known = 0;
const failures = [];
const check = (name, ok, detail = "") => {
  if (ok) passed++;
  else { failed++; failures.push(name + " " + detail); }
  console.log(ok ? "ok  " : "FAIL", name, ok ? "" : detail);
};
// A documented limit (components.md, *Known limits*): said, not failed.
const limit = (name, ok, detail = "") => {
  if (ok) return check(name, true);
  known++;
  console.log("known", name, detail);
};

const settle = async (page) => {
  await page.evaluate(async () => {
    await Promise.all(document.getAnimations().map((a) => a.finished.catch(() => {})));
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  });
};
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

async function compare(name, a, b) {
  await settle(a); await settle(b);
  // What a closed <details> holds is laid out lazily, and the first read of
  // a size in it may be the one before layout (seen in Chromium, comparing a
  // build with itself: `block-size: 0px` once, the height on the next read).
  // So every element's size is read once before the styles are.
  const layout = () => { for (const el of document.querySelectorAll("*")) void getComputedStyle(el).blockSize; };
  await a.evaluate(layout); await b.evaluate(layout);
  const [x, y] = [await a.evaluate(snapshot), await b.evaluate(snapshot)];
  if (x.length !== y.length) return check(`styles ${name}`, false, `${x.length} against ${y.length} elements and pseudo-elements`);
  const diffs = [];
  let customOnly = 0;
  for (let i = 0; i < x.length; i++) {
    if (x[i][0] !== y[i][0]) { diffs.push(`element ${i}: ${x[i][0]} against ${y[i][0]}`); continue; }
    if (x[i][1] !== y[i][1]) {
      const p = Object.fromEntries(x[i][1].split(";").map((d) => [d.slice(0, d.indexOf(":")), d.slice(d.indexOf(":") + 1)]));
      const q = Object.fromEntries(y[i][1].split(";").map((d) => [d.slice(0, d.indexOf(":")), d.slice(d.indexOf(":") + 1)]));
      for (const k in p) if (p[k] !== q[k]) diffs.push(`${x[i][0]} { ${k}: ${p[k]} (pruned) / ${q[k]} (unpruned) }`);
    } else if (x[i][2] !== y[i][2]) {
      // A custom property nothing reads may be dropped: said, not failed.
      const p = x[i][2].split(";"), q = y[i][2].split(";");
      if (!customOnly) console.log(`     custom properties of ${x[i][0]}: only unpruned [${q.filter((d) => !p.includes(d)).join("; ")}], only pruned [${p.filter((d) => !q.includes(d)).join("; ")}]`);
      customOnly++;
    }
  }
  check(`styles ${name}: ${x.length} elements and pseudo-elements equal${customOnly ? ` (${customOnly} differ in the custom properties they carry)` : ""}`, diffs.length === 0, `${diffs.length} differences:\n     ${diffs.slice(0, 8).join("\n     ")}`);
}

for (const engineName of engineList.split(",")) {
  // The full Chromium: Playwright's default headless shell paints the page
  // beside an open drawer wrongly (plan.md, RGP2-040).
  const browser = await pw[engineName].launch(engineName === "chromium" ? { channel: "chromium" } : {});
  const tag = `${engineName} ${browser.version()}`;
  console.log(`\n=== ${tag}`);
  const built = await serve(builtDir), control = await serve(controlDir);
  const errors = [];
  const watch = (page, what) => {
    page.on("pageerror", (e) => errors.push(`${what}: ${e}`));
    page.on("console", (m) => m.type() === "error" && errors.push(`${what}: console ${m.text()}`));
    page.on("requestfailed", (r) => errors.push(`${what}: request failed ${r.url()}`));
    page.on("response", (r) => r.status() >= 400 && errors.push(`${what}: ${r.status()} ${r.url()}`));
  };

  for (const width of WIDTHS) {
    const context = await browser.newContext({ viewport: { width, height: 800 } });
    const page = await context.newPage(), other = await context.newPage();
    watch(page, "default"); watch(other, "control");
    const both = async (f) => { await f(page); await f(other); };
    const shot = async (name, options = {}) => { if (shots) await page.screenshot({ path: join(shots, `${engineName}-${width}-${name}.png`), ...options }); };

    for (const pathname of PAGES) {
      const slug = pathname === "/" ? "home" : pathname.replaceAll("/", " ").trim().replaceAll(" ", "-");
      const at = `${tag} ${width}px ${pathname}`;
      await page.goto(built.origin + base + pathname.slice(1));
      await other.goto(control.origin + base + pathname.slice(1));
      await both((p) => p.waitForLoadState("load"));
      await settle(page);
      await shot(slug);
      if (pathname !== "/syntax/" && pathname !== "/reference/cli/") await shot(`${slug}-full`, { fullPage: true });

      // The page.
      const doc = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth,
        scripts: document.scripts.length, react: document.documentElement.outerHTML.includes("__reactFiber") || "React" in window,
        title: document.title,
      }));
      check(`${at}: no sideways scroll`, doc.scroll <= doc.client, `scrollWidth ${doc.scroll} > ${doc.client}`);
      check(`${at}: one script, the builder's; no React`, doc.scripts === 1 && !doc.react, JSON.stringify(doc));

      // The current page in the side menu.
      const current = await page.evaluate(() => {
        const marked = [...document.querySelectorAll("#s1 [aria-current]")];
        const details = [...document.querySelectorAll("#s1 details")];
        return {
          marked: marked.map((a) => [a.getAttribute("href"), a.getAttribute("aria-current")]),
          around: marked.length === 1 && (() => { for (let d = marked[0].closest("details"); d; d = d.parentElement.closest("details")) if (!d.open) return false; return true; })(),
          open: details.filter((d) => d.open).length, groups: details.length,
          weight: marked.length === 1 ? getComputedStyle(marked[0]).fontWeight : "",
          plain: getComputedStyle(document.querySelector("#s1 a:not([aria-current])")).fontWeight,
        };
      });
      const href = base + pathname.slice(1);
      check(`${at}: the side menu marks this page, and opens only its group`,
        current.marked.length === 1 && current.marked[0][0] === href && current.marked[0][1] === "page" && current.around && current.open === (pathname === "/" ? 0 : 1) && current.groups === 3,
        JSON.stringify(current));
      check(`${at}: the current item is styled`, current.weight !== current.plain, JSON.stringify(current));

      await compare(`${at} as loaded`, page, other);

      // The links menu: a popover anchored to its trigger.
      await both((p) => p.getByRole("button", { name: "Links", exact: true }).click());
      await settle(page);
      const menu = await state(page, "m1");
      const trigger = await page.evaluate(() => { const r = document.querySelector('[popovertarget="m1"]').getBoundingClientRect(); return { left: r.left, right: r.right, bottom: r.bottom, top: r.top }; });
      check(`${at}: the links menu opens`, menu.open && menu.display !== "none", JSON.stringify(menu));
      check(`${at}: … anchored below its trigger, end-aligned, in the viewport`,
        Math.abs(menu.rect.top - trigger.bottom) <= 8 && Math.abs(menu.rect.right - trigger.right) <= 2 && inside(menu), JSON.stringify({ menu: menu.rect, trigger }));
      if (pathname === "/") await shot("links-menu");
      await compare(`${at} links menu open`, page, other);
      await both((p) => p.keyboard.press("Escape"));
      check(`${at}: Esc closes the links menu`, !(await state(page, "m1")).open);

      // The drawer.
      const nav = await page.evaluate(() => {
        const nav = document.getElementById("s1"), toggle = document.querySelector(".rg-sidemenu-toggle");
        const r = nav.getBoundingClientRect();
        return { display: getComputedStyle(nav).display, position: getComputedStyle(nav).position, toggle: getComputedStyle(toggle).display, left: r.left, top: r.top, width: r.width, height: r.height, open: nav.matches(":popover-open") };
      });
      if (width >= 800) {
        check(`${at}: the side menu is a column, and there is no toggle`, nav.display !== "none" && !nav.open && nav.toggle === "none" && nav.position === "sticky" && nav.left === 0 && nav.width > 150 && nav.width < 400 && nav.height > 300, JSON.stringify(nav));
        const content = await page.evaluate(() => document.querySelector(".site-content").getBoundingClientRect().left);
        check(`${at}: … beside the content`, content >= nav.width - 1, `content starts at ${content}, the column is ${nav.width} wide`);
      } else {
        check(`${at}: the side menu is closed, and there is a toggle`, nav.display === "none" && nav.toggle !== "none", JSON.stringify(nav));
        await both((p) => p.locator(".rg-sidemenu-toggle").click());
        await settle(page);
        const drawer = await state(page, "s1");
        check(`${at}: the toggle opens the drawer, in the viewport`, drawer.open && drawer.rect.left === 0 && drawer.rect.width > 150 && drawer.rect.width < width && drawer.rect.height >= 700, JSON.stringify(drawer));
        if (pathname === "/" || pathname === "/syntax/") await shot(`${slug}-drawer`);
        await compare(`${at} drawer open`, page, other);
        // A group of another page opens in it.
        await both((p) => p.locator("#s1 summary").last().click());
        await compare(`${at} drawer open, a group toggled`, page, other);
        await both((p) => p.locator("#s1 summary").last().click());
        // The scrim: the rest of the viewport.
        await both((p) => p.mouse.click(width - 10, 400));
        await settle(page);
        check(`${at}: a click on the scrim closes the drawer`, !(await state(page, "s1")).open);
        const stayed = await page.evaluate(() => location.pathname);
        check(`${at}: … and activates nothing behind it`, stayed === href, stayed);
        await both((p) => p.locator(".rg-sidemenu-toggle").click());
        await both((p) => p.locator('#s1 > [data-part="close"]').click());
        check(`${at}: the close button closes the drawer`, !(await state(page, "s1")).open);
      }

      // Keyboard focus: the first stops of the page, as styled.
      await both((p) => p.keyboard.press("Tab"));
      await both((p) => p.keyboard.press("Tab"));
      await compare(`${at} with keyboard focus on ${(await active(page)).split("\n")[0]}`, page, other);

      // The Install dialog.
      if (pathname === "/") {
        const openByKey = async () => { await both(async (p) => { await p.getByRole("button", { name: "Install", exact: true }).focus(); await p.keyboard.press("Enter"); }); await settle(page); };
        await both((p) => p.getByRole("button", { name: "Install", exact: true }).click());
        await settle(page);
        const d = await state(page, "d1");
        check(`${at}: Install opens the dialog, modal`, d.open, JSON.stringify(d));
        const panel = await page.evaluate(() => { const r = document.querySelector('#d1 > [data-part="panel"]').getBoundingClientRect(); return { rect: { left: r.left, top: r.top, right: r.right, bottom: r.bottom }, viewport: { width: innerWidth, height: innerHeight } }; });
        check(`${at}: … its panel in the viewport`, inside(panel), JSON.stringify(panel));
        await shot("install-dialog");
        await compare(`${at} dialog open`, page, other);
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(`${at}: Esc closes it`, !(await state(page, "d1")).open);
        limit(`${at}: … and focus is back on Install (opened by a click)`, (await active(page)) === "Install", `focus on ${await active(page)}: WebKit does not focus a button on click (components.md, Known limits)`);

        await openByKey();
        check(`${at}: Enter on Install opens it`, (await state(page, "d1")).open);
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(`${at}: Esc closes it, focus back on Install`, !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.locator('#d1 header [data-part="close"]').click());
        await settle(page);
        check(`${at}: the close button closes it, focus back on Install`, !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.mouse.click(8, 8));
        await settle(page);
        check(`${at}: a click on the scrim closes it, focus back on Install`, !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));

        await openByKey();
        await both((p) => p.locator("#d1 footer").getByRole("button", { name: "Close" }).click());
        await settle(page);
        check(`${at}: the Close action closes it, focus back on Install`, !(await state(page, "d1")).open && (await active(page)) === "Install", await active(page));
      }

      // The action menu and the cheat sheet.
      if (pathname === "/syntax/") {
        await both(async (p) => { await p.locator("#m2-t").focus(); await p.keyboard.press("Enter"); });
        await settle(page);
        const m = await state(page, "m2");
        const t = await page.evaluate(() => { const r = document.getElementById("m2-t").getBoundingClientRect(); return { left: r.left, bottom: r.bottom }; });
        check(`${at}: the action menu opens, anchored below its trigger, in the viewport`, m.open && Math.abs(m.rect.top - t.bottom) <= 8 && Math.abs(m.rect.left - t.left) <= 2 && inside(m), JSON.stringify({ m, t }));
        check(`${at}: … focus on its first item`, (await active(page)) === "Cheat sheet…", await active(page));
        await shot("action-menu");
        await compare(`${at} action menu open`, page, other);
        const press = async (key) => { await both((p) => p.keyboard.press(key)); return active(page); };
        const seen = [await press("ArrowDown"), await press("ArrowDown"), await press("ArrowDown"), await press("ArrowUp"), await press("Home"), await press("End")];
        check(`${at}: ArrowDown, ArrowDown, ArrowDown (wraps), ArrowUp (wraps), Home, End`,
          JSON.stringify(seen) === JSON.stringify(["Specification on GitHub", "Getting started", "Cheat sheet…", "Getting started", "Cheat sheet…", "Getting started"]), JSON.stringify(seen));
        await compare(`${at} action menu, focus on the last item`, page, other);
        const typed = [await press("s"), await press("g"), await press("c")];
        check(`${at}: typeahead: s, g, c`, JSON.stringify(typed) === JSON.stringify(["Specification on GitHub", "Getting started", "Cheat sheet…"]), JSON.stringify(typed));
        await both((p) => p.keyboard.press("Enter"));
        await settle(page);
        const sheet = await state(page, "cheat-sheet");
        check(`${at}: the item opens the cheat sheet, modal, and closes the menu`, sheet.open && !(await state(page, "m2")).open, JSON.stringify(sheet));
        const panel = await page.evaluate(() => { const r = document.querySelector('#cheat-sheet > [data-part="panel"]').getBoundingClientRect(); return { rect: { left: r.left, top: r.top, right: r.right, bottom: r.bottom }, viewport: { width: innerWidth, height: innerHeight } }; });
        check(`${at}: … its panel in the viewport`, inside(panel), JSON.stringify(panel));
        await shot("cheat-sheet");
        await compare(`${at} cheat sheet open`, page, other);
        await both((p) => p.keyboard.press("Escape"));
        await settle(page);
        check(`${at}: Esc closes the cheat sheet, focus back on the menu's trigger`, !(await state(page, "cheat-sheet")).open && (await active(page)) === "m2-t", await active(page));

        // By pointer too.
        await both((p) => p.locator("#m2-t").click());
        await both((p) => p.getByRole("menuitem", { name: "Cheat sheet…" }).click());
        await settle(page);
        check(`${at}: by pointer: the item opens the cheat sheet`, (await state(page, "cheat-sheet")).open && !(await state(page, "m2")).open);
        await both((p) => p.mouse.click(8, 8));
        await settle(page);
        check(`${at}: … and a click on its scrim closes it`, !(await state(page, "cheat-sheet")).open);
      }

      // A link of the side menu into a page: the section is reached, below the header.
      if (pathname === "/guide/" && width >= 800) {
        await page.locator('#s1 a[href$="/guide/#check"]').click();
        await settle(page);
        const top = await page.evaluate(() => document.getElementById("check").getBoundingClientRect().top);
        check(`${at}: #check lands below the sticky header`, top >= 40 && top < 200, `top ${top}`);
      }
    }
    await context.close();
  }

  // The dialog where the engine has no `command`: the invokers behaviour.
  {
    const context = await browser.newContext({ viewport: { width: 1200, height: 800 } });
    await context.addInitScript(() => {
      delete HTMLButtonElement.prototype.command;
      delete HTMLButtonElement.prototype.commandForElement;
      addEventListener("command", (event) => event.preventDefault(), true);
    });
    const page = await context.newPage();
    watch(page, "no command");
    await page.goto(built.origin + base);
    await page.getByRole("button", { name: "Install", exact: true }).click();
    check(`${tag}: without the engine's \`command\`, the page's script opens the dialog`, (await state(page, "d1")).open);
    await page.locator('#d1 header [data-part="close"]').click();
    check(`${tag}: … and closes it`, !(await state(page, "d1")).open);
    await context.close();
  }

  check(`${tag}: no page error, console error or failed request`, errors.length === 0, errors.slice(0, 5).join(" | "));
  built.close(); control.close();
  await browser.close();
}
console.log(`\n${passed} passed, ${known} known, ${failed} failed`);
for (const f of failures) console.log("FAILED:", f.split("\n")[0]);
process.exit(failed ? 1 : 0);
