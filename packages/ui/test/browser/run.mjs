// Behaviour in real browsers (RGP2-025): `pnpm --filter @reactogenic/ui test:browser`.
// Not part of `pnpm test`: CI has no browsers. First: `npx playwright install chromium webkit`.
//
// Builds test/site with the stand-in builder (build.mjs) — once as it ships
// and once without any script, the control — serves both, and checks them in
// Chromium and WebKit ($ENGINES=chromium,webkit,chrome,firefox; `chrome` is
// the installed stable; $PLAYWRIGHT for the builds of another Playwright). A
// check that fails for a reason that is the engine's, not ours, is listed as
// known, with the reason; anything else fails the run.
//
// `firefox` is here to be run and has never been: where this suite was
// written Playwright's Firefox does not start (a sandboxed shell: "Could not
// find profile folder"). Expect its first run to need `known` entries.
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import * as esbuild from "esbuild";
import { build } from "./build.mjs";

// $PLAYWRIGHT names another Playwright (a path to its package): its browser
// builds are other versions of the engines.
const { chromium, webkit, firefox } = await import(process.env.PLAYWRIGHT ?? "playwright").then((module) => module.default ?? module);

const WIDE = { width: 1200, height: 800 };
const NARROW = { width: 400, height: 700 }; // below the 50rem breakpoint

function serve(dir) {
  const reports = [];
  const server = createServer((request, response) => {
    // What a page restored from the back/forward cache says about itself.
    if (request.method === "POST") {
      let body = "";
      request.on("data", (chunk) => (body += chunk)).on("end", () => (reports.push(JSON.parse(body)), response.end()));
      return;
    }
    let path = join(dir, decodeURIComponent(new URL(request.url, "http://x").pathname));
    if (path.endsWith("/")) path += "index.html";
    if (!existsSync(path)) {
      response.writeHead(404).end();
      return;
    }
    // No `no-store`: a page that says so is not kept by the back/forward cache.
    response.writeHead(200, { "content-type": "text/html; charset=utf-8" }).end(readFileSync(path));
  });
  // The next report, or undefined when none comes.
  const report = async () => {
    for (let waited = 0; reports.length === 0 && waited < 3000; waited += 100) {
      await new Promise((done) => setTimeout(done, 100));
    }
    return reports.shift();
  };
  return new Promise((done) => server.listen(0, "127.0.0.1", () => done({ origin: `http://127.0.0.1:${server.address().port}`, dir, report, close: () => server.close() })));
}

const engines = {
  // Playwright turns the back/forward cache off in Chromium; the Back checks
  // need it — and the full browser: the headless shell has no such cache.
  chromium: () => chromium.launch({ channel: "chromium", ignoreDefaultArgs: ["--disable-back-forward-cache"] }),
  chrome: () => chromium.launch({ channel: "chrome", ignoreDefaultArgs: ["--disable-back-forward-cache"] }),
  webkit: () => webkit.launch(),
  firefox: () => firefox.launch(),
};

// Takes `command` / `commandfor` away from an engine that has them: the
// feature test of `invokers` fails, and the native command is cancelled — so
// a dialog that opens was opened by the behaviour.
const noInvokers = () => {
  delete HTMLButtonElement.prototype.command;
  delete HTMLButtonElement.prototype.commandForElement;
  addEventListener("command", (event) => event.preventDefault(), true);
};

// What the page says about itself, for the assertions.
const probe = {
  active: () => {
    const element = document.activeElement;
    return element === document.body ? "body" : element.id || element.getAttribute("data-part") || element.textContent.trim();
  },
  dialog: (id) => {
    const dialog = document.getElementById(id);
    return dialog.open && dialog.matches(":modal");
  },
  popover: (id) => document.getElementById(id).matches(":popover-open"),
  rect: (selector) => {
    const box = document.querySelector(selector).getBoundingClientRect();
    return { top: Math.round(box.top), bottom: Math.round(box.bottom), left: Math.round(box.left), right: Math.round(box.right), width: Math.round(box.width) };
  },
  // Does the element have something to scroll: [x, y]?
  scrolls: (selector) => {
    const element = document.querySelector(selector);
    return [element.scrollWidth > element.clientWidth, element.scrollHeight > element.clientHeight];
  },
  // Computed values of `properties`, of every element `selector` matches.
  look: ([selector, properties]) =>
    [...document.querySelectorAll(selector)].map((element) => {
      const style = getComputedStyle(element);
      return properties.map((property) => style.getPropertyValue(property));
    }),
};

function equal(actual, expected, what) {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`${what}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
  }
}
function near(actual, expected, what, tolerance = 1) {
  if (Math.abs(actual - expected) > tolerance) {
    throw new Error(`${what}: got ${actual}, want ${expected} ± ${tolerance}`);
  }
}

async function suite(name, launch, site, control) {
  const browser = await launch();
  const results = [];
  const version = browser.version();
  const settle = (page) => page.waitForTimeout(350); // past the 0.2s transitions
  // WebKit's Tab skips links and buttons (a macOS setting); with Option it stops at them too.
  const tab = name === "webkit" ? "Alt+Tab" : "Tab";
  const open = async (origin, path, viewport, options = {}) => {
    const context = await browser.newContext({ viewport, ...options.context });
    const page = await context.newPage();
    if (options.init) await page.addInitScript(options.init);
    await page.goto(origin + path);
    return page;
  };
  // A check; `known` maps an engine to the reason a failure there is the engine's.
  const check = async (title, body, known = {}) => {
    try {
      await body();
      results.push({ title, status: "pass" });
    } catch (error) {
      const message = String(error.message ?? error).split("\n")[0];
      const reason = known[name] ?? (name === "chrome" ? known.chromium : undefined);
      results.push(reason === undefined ? { title, status: "FAIL", message } : { title, status: "known", message: `${reason} (${message})` });
    }
  };
  const focusByKeyboard = async (page, selector) => {
    await page.focus(selector);
    await page.keyboard.press("Enter");
    await settle(page);
  };

  // ---- Dialog ----
  {
    const page = await open(site.origin, "/", WIDE);
    const trigger = "Install"; // the $Trigger of d1, by its label

    await check("dialog: opens from its $Trigger; focus goes to the close button, never to the scrim", async () => {
      await page.click('button[commandfor="d1"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), true, "d1 open and modal");
      equal(await page.evaluate(probe.active), "close", "focus");
      equal(await page.evaluate(() => document.activeElement.closest("dialog").id), "d1", "focus is in");
    });
    await check("dialog: Tab never reaches the scrim", async () => {
      const seen = [];
      for (let i = 0; i < 4; i++) {
        await page.keyboard.press(tab);
        seen.push(await page.evaluate(() => document.activeElement.closest("[data-part=scrim]") !== null));
      }
      equal(seen.includes(true), false, "focus on the scrim");
      await page.keyboard.press("Escape");
      await settle(page);
    });
    await check("dialog: Esc closes it and focus returns to the invoker", async () => {
      await focusByKeyboard(page, 'button[commandfor="d1"]');
      equal(await page.evaluate(probe.dialog, "d1"), true, "open");
      await page.keyboard.press("Escape");
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), false, "open after Esc");
      equal(await page.evaluate(probe.active), trigger, "focus");
    });
    await check("dialog: the close button closes it and focus returns to the invoker", async () => {
      await focusByKeyboard(page, 'button[commandfor="d1"]');
      await page.click('#d1 [data-part="close"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), false, "open after close");
      equal(await page.evaluate(probe.active), trigger, "focus");
    });
    await check("dialog: a click on the panel keeps it; the scrim closes it and focus returns to the invoker", async () => {
      await focusByKeyboard(page, 'button[commandfor="d1"]');
      await page.click('#d1 [data-part="body"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), true, "open after a click inside");
      await page.mouse.click(5, 5);
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), false, "open after a click outside");
      equal(await page.evaluate(probe.active), trigger, "focus");
    });
    await check(
      "dialog: opened with the mouse, focus returns to the invoker too",
      async () => {
        await page.click('button[commandfor="d1"]');
        await settle(page);
        await page.keyboard.press("Escape");
        await settle(page);
        equal(await page.evaluate(probe.active), trigger, "focus");
      },
      { webkit: "WebKit does not focus a button that is clicked, so there is no invoker to return to" },
    );
    await check("dialog: the page does not scroll behind it", async () => {
      await page.evaluate(() => scrollTo(0, 0));
      equal(await page.evaluate(() => document.documentElement.scrollHeight > innerHeight), true, "the page is taller than the viewport");
      await page.click('button[commandfor="d1"]');
      await settle(page);
      equal(await page.evaluate(() => getComputedStyle(document.documentElement).overflow), "hidden", "overflow of <html>");
      await page.mouse.move(600, 700);
      await page.mouse.wheel(0, 400);
      await page.keyboard.press("PageDown");
      await page.keyboard.press("End");
      await settle(page);
      equal(await page.evaluate(() => Math.round(scrollY)), 0, "scrollY");
      await page.keyboard.press("Escape");
      await settle(page);
      await page.mouse.wheel(0, 400);
      await settle(page);
      equal(await page.evaluate(() => scrollY > 0), true, "scrolls again when it is closed");
      await page.evaluate(() => scrollTo(0, 0));
    });
    await check("dialog: opens from a button elsewhere; an $Action closes it", async () => {
      await page.click('header button[commandfor="shortcuts"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "shortcuts"), true, "open");
      await page.click("#shortcuts footer button");
      await settle(page);
      equal(await page.evaluate(probe.dialog, "shortcuts"), false, "open after the action");
    });
    await check('dialog: closedby="closerequest" has no light dismiss; Esc closes; an $Action with href is a link', async () => {
      await page.click('button[commandfor="d2"]');
      await settle(page);
      await page.mouse.click(5, 5);
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d2"), true, "open after a click outside");
      equal(await page.evaluate(() => document.querySelector("#d2 footer a").getAttribute("href")), "/guide/slots/", "the link action");
      await page.keyboard.press("Escape");
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d2"), false, "open after Esc");
    });
    await page.context().close();
  }

  // ---- Dialog: content larger than the viewport; inside the author's form ----
  {
    // The panel and the header's close button are inside the viewport, and
    // the dialog itself is never what scrolls.
    const fits = async (page, id, viewport) => {
      const panel = await page.evaluate(probe.rect, `#${id} > [data-part="panel"]`);
      equal([panel.left >= 0, panel.right <= viewport.width, panel.top >= 0, panel.bottom <= viewport.height], [true, true, true, true], `the panel ${JSON.stringify(panel)} inside the viewport`);
      const close = await page.evaluate(probe.rect, `#${id} [data-part="close"]`);
      equal([close.left >= panel.left, close.right <= panel.right, close.top >= panel.top], [true, true, true], `the close button ${JSON.stringify(close)} inside the panel`);
      equal(await page.evaluate(probe.scrolls, `#${id}`), [false, false], "the dialog scrolls [x, y]");
    };
    const narrow = await open(site.origin, "/fit/", NARROW);
    await check("dialog: a line wider than the panel scrolls in the body; the panel stays inside the viewport", async () => {
      await narrow.click('button[commandfor="wide"]');
      await settle(narrow);
      await fits(narrow, "wide", NARROW);
      equal(await narrow.evaluate(probe.scrolls, '#wide [data-part="body"]'), [true, false], "the body scrolls [x, y]");
      await narrow.keyboard.press("Escape");
      await settle(narrow);
    });
    await check("dialog: an unbreakable word in the title and in the body wraps; the panel stays inside the viewport", async () => {
      await narrow.click('button[commandfor="url"]');
      await settle(narrow);
      await fits(narrow, "url", NARROW);
      equal(await narrow.evaluate(probe.scrolls, '#url [data-part="body"]'), [false, false], "the body scrolls [x, y]");
      await narrow.keyboard.press("Escape");
      await settle(narrow);
    });
    await narrow.context().close();

    const page = await open(site.origin, "/fit/", WIDE);
    await check("dialog: a body taller than the viewport scrolls; header and footer stay; a click beside the panel still closes", async () => {
      await page.click('button[commandfor="tall"]');
      await settle(page);
      await fits(page, "tall", WIDE);
      equal(await page.evaluate(probe.scrolls, '#tall [data-part="body"]'), [false, true], "the body scrolls [x, y]");
      await page.mouse.move(600, 400);
      await page.mouse.wheel(0, 2000);
      await settle(page);
      equal(await page.evaluate(() => document.querySelector('#tall [data-part="body"]').scrollTop > 1000), true, "the wheel scrolled the body");
      await fits(page, "tall", WIDE);
      const footer = await page.evaluate(probe.rect, "#tall footer button");
      equal(footer.top >= 0 && footer.bottom <= WIDE.height, true, `the action ${JSON.stringify(footer)} inside the viewport`);
      equal(await page.evaluate(() => document.elementFromPoint(5, 400).getAttribute("data-part")), "scrim", "beside the panel");
      await page.mouse.click(5, 400);
      await settle(page);
      equal(await page.evaluate(probe.dialog, "tall"), false, "open after a click beside the panel");
    });
    await check("dialog inside the author's <form>: the scrim is the dialog's, covers the viewport, closes it, and submits nothing", async () => {
      equal(
        await page.evaluate(() => {
          const scrim = document.querySelector("#confirm [data-part=scrim]");
          return [document.forms.length, scrim?.parentElement.id, [...document.forms[0].elements].filter((element) => element.type === "submit").length];
        }),
        [1, "confirm", 0],
        "[forms of the page, the scrim's parent, submit buttons of the author's form]",
      );
      await page.click('button[commandfor="confirm"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "confirm"), true, "open");
      const scrim = await page.evaluate(probe.rect, "#confirm [data-part=scrim]");
      equal([scrim.left, scrim.top, scrim.right, scrim.bottom], [0, 0, WIDE.width, WIDE.height], "the scrim");
      await page.mouse.click(5, 5);
      await settle(page);
      equal(await page.evaluate(probe.dialog, "confirm"), false, "open after a click outside");
      equal(new URL(page.url()).pathname + new URL(page.url()).search, "/fit/", "the page");
    });
    await page.context().close();
  }

  // ---- Components in a side menu's slots ----
  {
    // What a rule of the side menu would change: the same component outside
    // the side menu is the measure.
    const same = async (page, inside, outside, properties) => {
      const a = await page.evaluate(probe.look, [inside, properties]);
      const b = await page.evaluate(probe.look, [outside, properties]);
      equal(a.length > 0, true, `${inside} matches`);
      equal(a, b, `${properties.join(", ")} of ${inside} — want those of ${outside}`);
    };
    const box = ["display", "position", "padding-left", "padding-top", "margin-top", "color", "font-size", "font-weight", "background-color", "border-radius"];
    for (const [width, viewport] of [["wide", WIDE], ["narrow", NARROW]]) {
      const page = await open(site.origin, "/nested/", viewport);
      const drawer = async () => {
        if (width === "narrow" && !(await page.evaluate(probe.popover, "nav"))) {
          await page.click(".rg-sidemenu-toggle");
          await settle(page);
        }
      };
      await check(`in a side menu's $Header, ${width}: a dialog keeps its close button, its scrim and its look`, async () => {
        await drawer();
        await page.click('button[commandfor="search"]');
        await settle(page);
        equal(await page.evaluate(probe.dialog, "search"), true, "open and modal");
        equal(await page.evaluate(probe.active), "close", "focus");
        const scrim = await page.evaluate(probe.rect, "#search [data-part=scrim]");
        equal([scrim.left, scrim.top, scrim.right, scrim.bottom], [0, 0, viewport.width, viewport.height], "the scrim");
        for (const part of ['> [data-part="panel"]', "h2", '[data-part="close"]', '[data-part="body"]', '[data-part="scrim"]']) {
          await same(page, `#search ${part}`, `#plain ${part}`, box);
        }
        await page.mouse.click(viewport.width - 5, 5);
        await settle(page);
        equal(await page.evaluate(probe.dialog, "search"), false, "open after a click outside");
      });
      await check(`in a side menu's $Header, ${width}: a menu keeps its look`, async () => {
        await drawer();
        await page.click('[popovertarget="ver"]');
        await settle(page);
        equal(await page.evaluate(probe.popover, "ver"), true, "open");
        for (const part of ["", " li", " a"]) {
          await same(page, `#ver${part}`, `#plain-ver${part}`, box.filter((property) => property !== "display" || part !== ""));
        }
        await page.keyboard.press("Escape");
        await settle(page);
      });
      await check(`in a side menu's $Footer, ${width}: an action menu has its keys, and its item opens the dialog in $Header`, async () => {
        await drawer();
        await page.click('[popovertarget="tools"]');
        await settle(page);
        const walk = [await page.evaluate(probe.active)];
        for (const key of ["ArrowDown", "ArrowDown"]) {
          await page.keyboard.press(key);
          walk.push(await page.evaluate(probe.active));
        }
        equal(walk, ["Search…", "Home", "Search…"], "focus after each key");
        await page.keyboard.press("Enter");
        await settle(page);
        equal([await page.evaluate(probe.popover, "tools"), await page.evaluate(probe.dialog, "search")], [false, true], "[menu open, dialog open]");
        const close = await page.evaluate(probe.rect, '#search [data-part="close"]');
        equal(close.width > 0 && close.left >= 0 && close.right <= viewport.width, true, `the dialog's close button ${JSON.stringify(close)} is on screen`);
        await page.keyboard.press("Escape");
        await settle(page);
        equal(await page.evaluate(probe.dialog, "search"), false, "open after Esc");
      });
      await page.context().close();
    }
  }

  // ---- DropdownMenu ----
  {
    const page = await open(site.origin, "/guide/flow/", WIDE);

    await check("menu: opens anchored to its trigger", async () => {
      await page.click("#actions-t");
      await settle(page);
      equal(await page.evaluate(probe.popover, "actions"), true, "open");
      const trigger = await page.evaluate(probe.rect, "#actions-t");
      const menu = await page.evaluate(probe.rect, "#actions");
      near(menu.top, trigger.bottom + 4, "the menu's top: below the trigger, 0.25rem apart");
      near(menu.left, trigger.left, "the menu's left edge");
      await page.keyboard.press("Escape");
      await settle(page);
    });
    await check('menu: align="end" lines up the right edges', async () => {
      await page.click('[popovertarget="versions"]');
      await settle(page);
      const trigger = await page.evaluate(probe.rect, '[popovertarget="versions"]');
      const menu = await page.evaluate(probe.rect, "#versions");
      near(menu.right, trigger.right, "the menu's right edge");
      near(menu.top, trigger.bottom + 4, "the menu's top");
      await page.keyboard.press("Escape");
      await settle(page);
    });
    await check("menu: flips above its trigger near the bottom edge", async () => {
      await page.evaluate(() => document.querySelector('[popovertarget="more"]').scrollIntoView({ block: "end" }));
      await settle(page);
      const trigger = await page.evaluate(probe.rect, '[popovertarget="more"]');
      equal(trigger.bottom > WIDE.height - 40, true, "the trigger is at the bottom of the viewport");
      await page.click('[popovertarget="more"]');
      await settle(page);
      const menu = await page.evaluate(probe.rect, "#more");
      near(menu.bottom, trigger.top - 4, "the menu's bottom: above the trigger");
      equal(menu.top >= 0 && menu.bottom <= WIDE.height, true, "the menu is inside the viewport");
      await page.keyboard.press("Escape");
      await page.evaluate(() => scrollTo(0, 0));
      await settle(page);
    });
    await check("menu of links: a list without the menu role; an action menu: role, name, aria-haspopup", async () => {
      equal(await page.evaluate(() => [document.getElementById("versions").localName, document.getElementById("versions").getAttribute("role")]), ["ul", null], "links");
      equal(
        await page.evaluate(() => {
          const menu = document.getElementById("actions");
          const trigger = document.getElementById(menu.getAttribute("aria-labelledby"));
          return [menu.getAttribute("role"), trigger.textContent, trigger.getAttribute("aria-haspopup"), trigger.popoverTargetElement === menu];
        }),
        ["menu", "Actions", "menu", true],
        "actions",
      );
    });
    await check("action menu: focus enters at the first item; arrows wrap and skip a disabled item; Home, End", async () => {
      await page.click("#actions-t");
      await settle(page);
      const walk = [await page.evaluate(probe.active)];
      for (const key of ["ArrowDown", "ArrowDown", "ArrowDown", "ArrowUp", "Home", "End", "ArrowUp", "ArrowUp", "ArrowUp"]) {
        await page.keyboard.press(key);
        walk.push(await page.evaluate(probe.active));
      }
      equal(
        walk,
        ["Keyboard shortcuts…", "Source", "Spec", "Keyboard shortcuts…", "Spec", "Keyboard shortcuts…", "Spec", "Source", "Keyboard shortcuts…", "Spec"],
        "focus after each key",
      );
      equal(await page.evaluate(() => Math.round(scrollY)), 0, "the arrows did not scroll the page");
    });
    await check("action menu: Tab closes it and moves on from the trigger", async () => {
      await page.keyboard.press(tab);
      await settle(page);
      equal(await page.evaluate(probe.popover, "actions"), false, "open after Tab");
      equal(await page.evaluate(probe.active), "after", "focus: the link after the trigger");
    });
    await check("action menu: Tab closes it with focus back on the trigger first", async () => {
      // The same, seen from inside the keydown: after the behaviour, before the browser's own Tab.
      if (!(await page.evaluate(probe.popover, "actions"))) {
        await page.click("#actions-t");
        await settle(page);
      }
      await page.evaluate(() => {
        addEventListener("keydown", (event) => {
          if (event.key === "Tab") window.seen = [document.getElementById("actions").matches(":popover-open"), document.activeElement.id];
        });
      });
      await page.keyboard.press(tab);
      await settle(page);
      equal(await page.evaluate(() => window.seen), [false, "actions-t"], "[open, focus] at the end of the keydown");
    });
    await check("action menu: Esc closes it and focus returns to the trigger", async () => {
      await focusByKeyboard(page, "#actions-t");
      equal(await page.evaluate(probe.active), "Keyboard shortcuts…", "focus in the menu");
      await page.keyboard.press("Escape");
      await settle(page);
      equal(await page.evaluate(probe.popover, "actions"), false, "open after Esc");
      equal(await page.evaluate(probe.active), "actions-t", "focus");
    });
    await check("action menu: an item opens a dialog and the menu closes; closing the dialog returns focus to the trigger", async () => {
      await page.click("#actions-t");
      await settle(page);
      await page.keyboard.press("Enter"); // the first item: command="show-modal"
      await settle(page);
      equal([await page.evaluate(probe.popover, "actions"), await page.evaluate(probe.dialog, "shortcuts")], [false, true], "[menu open, dialog open]");
      await page.keyboard.press("Escape");
      await settle(page);
      equal(await page.evaluate(probe.dialog, "shortcuts"), false, "dialog open after Esc");
      equal(await page.evaluate(probe.active), "actions-t", "focus");
    });
    await check("action menu: a click on an item closes the menu too", async () => {
      await page.click("#actions-t");
      await settle(page);
      await page.click('#actions [commandfor="shortcuts"]');
      await settle(page);
      equal([await page.evaluate(probe.popover, "actions"), await page.evaluate(probe.dialog, "shortcuts")], [false, true], "[menu open, dialog open]");
      await page.keyboard.press("Escape");
      await settle(page);
    });
    await check("typeahead (RG_MENU_TYPEAHEAD): a printable key moves to the next item that starts with it", async () => {
      await page.click("#actions-t");
      await settle(page);
      const walk = [];
      for (const key of ["s", "s", "s", "k", "p", "x"]) {
        await page.keyboard.press(key);
        walk.push(await page.evaluate(probe.active));
      }
      // "p": Print is disabled, so nothing starts with it; "x": nothing either.
      equal(walk, ["Source", "Spec", "Source", "Keyboard shortcuts…", "Keyboard shortcuts…", "Keyboard shortcuts…"], "focus after each key");
      await page.keyboard.press("Escape");
      await settle(page);
    });
    await page.context().close();

    const plain = await open(site.origin, "/actions/", WIDE);
    await check("without the flag: no typeahead, and its code is not in the page", async () => {
      await plain.click("#m2-t");
      await settle(plain);
      equal(await plain.evaluate(probe.active), "Keyboard shortcuts…", "focus in the menu");
      await plain.keyboard.press("s");
      equal(await plain.evaluate(probe.active), "Keyboard shortcuts…", "focus after a key");
      await plain.keyboard.press("ArrowDown");
      equal(await plain.evaluate(probe.active), "Source", "the arrows still work");
      const script = (path) => /<script type="module">([\s\S]*?)<\/script>/.exec(readFileSync(join(site.dir, path, "index.html"), "utf8"))?.[1] ?? "";
      equal([/startsWith|toLowerCase/.test(script("/guide/flow/")), /startsWith|toLowerCase/.test(script("/actions/"))], [true, false], "[with the flag, without]");
    });
    await plain.context().close();

    const menus = await open(site.origin, "/menus/", WIDE);
    await check("typeahead is the menu's: on a page that has one with it, a menu without it has none", async () => {
      const after = async (id, key) => {
        await focusByKeyboard(menus, `[popovertarget="${id}"]`);
        const walk = [await menus.evaluate(probe.active)];
        await menus.keyboard.press(key);
        walk.push(await menus.evaluate(probe.active));
        await menus.keyboard.press("Escape");
        await settle(menus);
        return walk;
      };
      equal(await after("one", "b"), ["Alpha", "Beta"], "the menu with typeahead: focus before and after `b`");
      equal(await after("two", "b"), ["Alpha", "Alpha"], "the menu without: focus before and after `b`");
    });
    await check("action menu: the author's id on $Trigger is the one that names the menu, and focus returns to it", async () => {
      equal(
        await menus.evaluate(() => [...document.querySelectorAll("[aria-labelledby]")].filter((element) => document.getElementById(element.getAttribute("aria-labelledby")) === null).map((element) => element.id)),
        [],
        "elements whose aria-labelledby names nothing",
      );
      equal(await menus.evaluate(() => document.getElementById("three").getAttribute("aria-labelledby")), "mine", "the menu's name");
      await menus.click("#mine");
      await settle(menus);
      await menus.click('#three [commandfor="shortcuts"]');
      await settle(menus);
      equal([await menus.evaluate(probe.popover, "three"), await menus.evaluate(probe.dialog, "shortcuts")], [false, true], "[menu open, dialog open]");
      await menus.keyboard.press("Escape");
      await settle(menus);
      equal(await menus.evaluate(probe.active), "mine", "focus");
    });
    await check("a disabled link item is no link: no href, aria-disabled; focus and the arrows skip it", async () => {
      // [element, has href, aria-disabled, is a link, takes focus]
      const disabled = (selector) =>
        menus.evaluate(
          (s) =>
            [...document.querySelectorAll(s)].map((a) => {
              a.focus();
              return [a.localName, a.hasAttribute("href"), a.getAttribute("aria-disabled"), a.matches(":any-link"), document.activeElement === a];
            }),
          selector,
        );
      equal(await disabled("#three > a:first-child"), [["a", false, "true", false, false]], "in an action menu");
      equal(await disabled("#links > li:first-child > a"), [["a", false, "true", false, false]], "in a menu of links");
      equal(await disabled("a#off"), [["a", false, "true", false, false]], "a Button with href and disabled");
      await menus.click("#mine");
      await settle(menus);
      const walk = [await menus.evaluate(probe.active)];
      for (const key of ["ArrowDown", "ArrowDown", "Home", "End", "ArrowUp", "ArrowUp"]) {
        await menus.keyboard.press(key);
        walk.push(await menus.evaluate(probe.active));
      }
      equal(walk, ["Open", "Home", "Open", "Open", "Home", "Open", "Home"], "focus after each key");
      await menus.keyboard.press("Escape");
      await settle(menus);
    });
    await check("menu at the right edge: flips to the trigger's other edge, keeps its width, stays inside the viewport", async () => {
      await menus.click('[popovertarget="edge"]');
      await settle(menus);
      const trigger = await menus.evaluate(probe.rect, '[popovertarget="edge"]');
      const menu = await menus.evaluate(probe.rect, "#edge");
      equal(WIDE.width - trigger.left < menu.width, true, "the menu is wider than what is left of the viewport");
      equal([menu.left >= 0, menu.right <= WIDE.width], [true, true], `the menu ${JSON.stringify(menu)} inside the viewport`);
      near(menu.right, trigger.right, "the menu's right edge");
      equal(menu.bottom - menu.top < 60, true, `the label is one line (the menu is ${menu.bottom - menu.top}px tall)`);
      await menus.keyboard.press("Escape");
      await settle(menus);
    });
    await menus.context().close();

    const phone = await open(site.origin, "/menus/", NARROW);
    await check("menu on a narrow screen: never wider than the viewport", async () => {
      await phone.click('[popovertarget="edge"]');
      await settle(phone);
      const menu = await phone.evaluate(probe.rect, "#edge");
      equal([menu.left >= 0, menu.right <= NARROW.width], [true, true], `the menu ${JSON.stringify(menu)} inside the viewport`);
      await phone.keyboard.press("Escape");
      await settle(phone);
    });
    await check("menu wider than the room on either side of its trigger: against the viewport's edge, still under the trigger", async () => {
      await phone.click('[popovertarget="middle"]');
      await settle(phone);
      const trigger = await phone.evaluate(probe.rect, '[popovertarget="middle"]');
      const menu = await phone.evaluate(probe.rect, "#middle");
      equal([NARROW.width - trigger.left < menu.width, trigger.right < menu.width], [true, true], "the menu fits on neither side");
      equal([menu.left >= 0, menu.right <= NARROW.width], [true, true], `the menu ${JSON.stringify(menu)} inside the viewport`);
      near(menu.top, trigger.bottom + 4, "the menu's top");
    });
    await phone.context().close();
  }

  // ---- SideMenu ----
  {
    const wide = await open(site.origin, "/guide/slots/", WIDE);
    await check("side menu, wide: a column of the page; toggle and close button hidden; the current page marked", async () => {
      const state = await wide.evaluate(() => {
        const nav = document.getElementById("nav");
        const current = nav.querySelectorAll("[aria-current]");
        return {
          display: getComputedStyle(nav).display,
          position: getComputedStyle(nav).position,
          open: nav.matches(":popover-open"),
          toggle: getComputedStyle(document.querySelector(".rg-sidemenu-toggle")).display,
          close: getComputedStyle(nav.querySelector("[data-part=close]")).display,
          current: [...current].map((link) => link.getAttribute("href")),
          group: current[0].closest("details").open,
        };
      });
      equal(state, { display: "block", position: "sticky", open: false, toggle: "none", close: "none", current: ["/guide/slots/"], group: true }, "state");
      const nav = await wide.evaluate(probe.rect, "#nav");
      equal([nav.left, nav.width], [0, 256], "[left, width]: the first column of the site's grid");
    });
    await wide.context().close();

    const page = await open(site.origin, "/", NARROW);
    await check("side menu, narrow: hidden; the toggle opens it as a popover over the page", async () => {
      equal(await page.evaluate(() => getComputedStyle(document.getElementById("nav")).display), "none", "display when closed");
      await page.click(".rg-sidemenu-toggle");
      await settle(page);
      equal(await page.evaluate(probe.popover, "nav"), true, "open");
      const nav = await page.evaluate(probe.rect, "#nav");
      equal([nav.left, nav.top, nav.width, nav.bottom], [0, 0, 288, NARROW.height], "[left, top, width, bottom]");
      equal(await page.evaluate(() => getComputedStyle(document.querySelector("#nav [data-part=close]")).display), "block", "the close button");
    });
    await check("drawer: its close button closes it", async () => {
      await page.click('#nav [data-part="close"]');
      await settle(page);
      equal(await page.evaluate(probe.popover, "nav"), false, "open after close");
    });
    await check("drawer: Esc closes it and focus returns to the toggle", async () => {
      await focusByKeyboard(page, ".rg-sidemenu-toggle");
      equal(await page.evaluate(probe.popover, "nav"), true, "open");
      await page.keyboard.press("Escape");
      await settle(page);
      equal(await page.evaluate(probe.popover, "nav"), false, "open after Esc");
      equal(await page.evaluate(() => document.activeElement.className), "rg-sidemenu-toggle", "focus");
    });
    // A point on `selector` that is beside the drawer: under its scrim
    // exactly while the drawer is open.
    const behind = async (target, selector) => {
      const box = await target.evaluate(probe.rect, selector);
      const point = { x: box.right - 6, y: (box.top + box.bottom) / 2 };
      equal(point.x > 288 + 6, true, `${selector} reaches beside the drawer`);
      const onScrim = await target.evaluate(({ x, y }) => document.elementFromPoint(x, y).getAttribute("data-part") === "scrim", point);
      equal(onScrim, await target.evaluate(probe.popover, "nav"), "the scrim is over the point");
      return point;
    };
    await check("drawer: a click on the backdrop closes it and does not activate the link behind it", async () => {
      await page.click(".rg-sidemenu-toggle");
      await settle(page);
      const point = await behind(page, "#behind");
      await page.mouse.click(point.x, point.y);
      await settle(page);
      equal(await page.evaluate(probe.popover, "nav"), false, "open after the click");
      equal(new URL(page.url()).pathname, "/", "the page");
    });
    await check("drawer: the same for a button behind it — the dialog it commands does not open", async () => {
      await page.click(".rg-sidemenu-toggle");
      await settle(page);
      const point = await behind(page, 'header [commandfor="shortcuts"]');
      await page.mouse.click(point.x, point.y);
      await settle(page);
      equal([await page.evaluate(probe.popover, "nav"), await page.evaluate(probe.dialog, "shortcuts")], [false, false], "[drawer open, dialog open]");
    });
    await check("drawer: closed, the same click does activate the link", async () => {
      const point = await behind(page, "#behind");
      await page.mouse.click(point.x, point.y);
      await page.waitForURL("**/guide/flow/", { timeout: 5000 });
    });
    await page.context().close();

    const touch = await open(site.origin, "/", NARROW, { context: { hasTouch: true } });
    await check("drawer: a tap on the backdrop closes it and does not activate the link behind it", async () => {
      await touch.tap(".rg-sidemenu-toggle");
      await settle(touch);
      equal(await touch.evaluate(probe.popover, "nav"), true, "open");
      const point = await behind(touch, "#behind");
      await touch.touchscreen.tap(point.x, point.y);
      await settle(touch);
      equal(await touch.evaluate(probe.popover, "nav"), false, "open after the tap");
      equal(new URL(touch.url()).pathname, "/", "the page");
      await touch.touchscreen.tap(point.x, point.y);
      await touch.waitForURL("**/guide/flow/", { timeout: 5000 });
    });
    await touch.context().close();
  }

  // ---- overlays: Back to a page left with an open overlay ----
  {
    // Leaves `path` through a link inside an open overlay, comes Back, and
    // says whether the page was restored from the cache and what is open.
    const leaveAndReturn = async (server, path, viewport, leave) => {
      const page = await open(server.origin, path, viewport);
      // Only a page that is restored keeps this listener: a page that is
      // loaded again is a new document, and reports nothing.
      await page.evaluate(() =>
        addEventListener("pageshow", (event) =>
          fetch("/", {
            method: "POST",
            body: JSON.stringify({
              restored: event.persisted,
              open: [...document.querySelectorAll(":popover-open, dialog[open]")].map((element) => element.id),
              overflow: getComputedStyle(document.documentElement).overflow,
            }),
          }),
        ),
      );
      await leave(page);
      await page.waitForURL((url) => url.pathname !== path);
      // Not page.goBack(): it waits for a load, and a restored page fires none.
      await page.evaluate(() => history.back()).catch(() => {});
      const state = (await server.report()) ?? { restored: false };
      await page.context().close();
      return state;
    };
    const cases = {
      drawer: ["/guide/slots/", NARROW, async (page) => (await page.click(".rg-sidemenu-toggle"), await settle(page), await page.click('#nav a[href="/guide/flow/"]'))],
      menu: ["/guide/flow/", WIDE, async (page) => (await page.click('[popovertarget="versions"]'), await settle(page), await page.click('#versions a[href="/"]'))],
      dialog: ["/", WIDE, async (page) => (await page.click('button[commandfor="d2"]'), await settle(page), await page.click("#d2 footer a"))],
    };
    // The page cache is the engine's to use or not: without it there is nothing to restore.
    const noCache = { webkit: "Playwright's WebKit has no page cache: the page is loaded again" };
    for (const [what, [path, viewport, leave]] of Object.entries(cases)) {
      await check(
        `Back to a page left with an open ${what}: restored from the cache, and it is closed`,
        async () => {
          equal(await leaveAndReturn(site, path, viewport, leave), { restored: true, open: [], overflow: "visible" }, "state after Back");
        },
        noCache,
      );
      await check(
        `control, no script: the same page comes back with the ${what} open`,
        async () => {
          const state = await leaveAndReturn(control, path, viewport, leave);
          equal([state.restored, state.open?.length], [true, 1], "[restored, open overlays]");
        },
        noCache,
      );
    }
  }

  // ---- invokers: an engine without `command` ----
  {
    await check("control, no script, no native command: the dialog's button is dead", async () => {
      const page = await open(control.origin, "/", WIDE, { init: noInvokers });
      equal(await page.evaluate(() => "command" in HTMLButtonElement.prototype), false, "the feature test");
      await page.click('button[commandfor="d1"]');
      await settle(page);
      equal(await page.evaluate(() => document.getElementById("d1").open), false, "open");
      await page.context().close();
    });
    await check("invokers: without native command the dialog still opens and closes", async () => {
      const page = await open(site.origin, "/", WIDE, { init: noInvokers });
      await page.click('button[commandfor="d1"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), true, "open from its $Trigger");
      await page.click('#d1 [data-part="close"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), false, "open after the close button");
      // The scrim is a command button too.
      await page.click('button[commandfor="d1"]');
      await settle(page);
      await page.mouse.click(5, 5);
      await settle(page);
      equal(await page.evaluate(probe.dialog, "d1"), false, "open after a click on the scrim");
      await page.click('header button[commandfor="shortcuts"]');
      await settle(page);
      equal(await page.evaluate(probe.dialog, "shortcuts"), true, "open from a button elsewhere");
      await page.click("#shortcuts footer button");
      await settle(page);
      equal(await page.evaluate(probe.dialog, "shortcuts"), false, "open after the action");
      await page.context().close();
    });
    await check("invokers: at the floor it adds no listener", async () => {
      const page = await open(site.origin, "/", WIDE, {
        init: () => {
          const add = EventTarget.prototype.addEventListener;
          window.listened = [];
          EventTarget.prototype.addEventListener = function (type, ...rest) {
            window.listened.push(type);
            return add.call(this, type, ...rest);
          };
        },
      });
      equal(await page.evaluate(() => "command" in HTMLButtonElement.prototype), true, "the engine has command");
      equal(await page.evaluate(() => window.listened), ["pagehide"], "listeners the page's script added");
      await page.context().close();
    });
    await check("a page with nothing that opens ships no script", async () => {
      const page = await open(site.origin, "/plain/", WIDE);
      equal(await page.evaluate(() => document.scripts.length), 0, "scripts");
      await page.context().close();
    });
  }

  await browser.close();
  return { name, version, results };
}

const out = mkdtempSync(join(tmpdir(), "reactogenic-ui-"));
await build(join(out, "site"));
await build(join(out, "control"), { script: false });
await esbuild.stop();
const site = await serve(join(out, "site"));
const control = await serve(join(out, "control"));

let failed = 0;
for (const name of (process.env.ENGINES ?? "chromium,webkit").split(",")) {
  const { version, results } = await suite(name, engines[name], site, control);
  console.log(`\n${name} ${version}`);
  for (const result of results) {
    console.log(`  ${result.status.padEnd(5)} ${result.title}${result.message ? `\n          ${result.message}` : ""}`);
  }
  const count = (status) => results.filter((result) => result.status === status).length;
  console.log(`  ${count("pass")} passed, ${count("known")} known, ${count("FAIL")} failed`);
  failed += count("FAIL");
}

site.close();
control.close();
rmSync(out, { recursive: true, force: true });
process.exit(failed === 0 ? 0 : 1);
