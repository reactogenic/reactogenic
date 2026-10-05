// compare.mjs — the computed-style comparison of two builds of one page, and
// a static server for a build: the method of verify.mjs (T7), lifted for
// catalog-verify.mjs. verify.mjs keeps its own copy, so that the measurement
// of the docs site is the script that was reviewed. No dependency: the pages
// are Playwright's, and the caller's.
import { existsSync, readFileSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join } from "node:path";

const types = { ".html": "text/html; charset=utf-8", ".css": "text/css", ".js": "text/javascript", ".svg": "image/svg+xml", ".json": "application/json" };

// A build's output over HTTP. `requests` has every path asked for.
export function serve(dir) {
  const requests = [];
  const server = createServer((request, response) => {
    const pathname = decodeURIComponent(new URL(request.url, "http://x").pathname);
    requests.push(pathname);
    let path = join(dir, pathname);
    if (existsSync(path) && statSync(path).isDirectory()) path = join(path, "index.html");
    if (!existsSync(path)) return void response.writeHead(404).end("not found");
    response.writeHead(200, { "content-type": types[extname(path)] ?? "application/octet-stream" }).end(readFileSync(path));
  });
  return new Promise((done) => server.listen(0, "127.0.0.1", () => done({ origin: `http://127.0.0.1:${server.address().port}`, requests, close: () => server.close() })));
}

// Every animation that ends has ended, and two frames have passed. One that
// does not end (an indeterminate progress bar) is not waited for.
export const settle = (page) => page.evaluate(async () => {
  const ending = document.getAnimations().filter((a) => Number.isFinite(a.effect?.getComputedTiming().endTime ?? Infinity));
  await Promise.all(ending.map((a) => a.finished.catch(() => {})));
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
});

// Every element's computed style — and its ::before, ::after, ::marker,
// ::backdrop — but for the elements packaging writes, which differ between
// the two builds by design. Custom properties apart: one that nothing reads
// may be dropped, and that is said, not failed. Run in the page.
const snapshot = () => {
  const pseudos = [null, "::before", "::after", "::marker", "::backdrop"];
  const out = [];
  const skip = (el) => el.localName === "style" || el.localName === "script" || (el.localName === "link" && el.rel === "stylesheet");
  // An animation that never ends is at another frame in each page: its
  // running values are not compared, what it is declared as is.
  const moving = new Set(["background-position", "background-position-x", "background-position-y"]);
  for (const el of document.querySelectorAll("*")) {
    if (skip(el)) continue;
    const endless = el.getAnimations().some((a) => !Number.isFinite(a.effect?.getComputedTiming().endTime ?? Infinity));
    for (const ps of pseudos) {
      const cs = getComputedStyle(el, ps);
      let text = "", custom = "";
      for (let i = 0; i < cs.length; i++) {
        const k = cs[i];
        if (k.startsWith("--")) custom += k + ":" + cs.getPropertyValue(k) + ";";
        else if (!(endless && moving.has(k))) text += k + ":" + cs.getPropertyValue(k) + ";";
      }
      // Chromium enumerates custom properties in no fixed order.
      custom = custom.split(";").sort().join(";");
      const label = el.localName + (el.id ? "#" + el.id : "") + (typeof el.className === "string" && el.className ? "." + el.className.replace(/\s+/g, ".") : "") + (el.dataset.part ? `[data-part=${el.dataset.part}]` : "") + (ps || "");
      out.push([label, text, custom]);
    }
  }
  return out;
};

// The differences between the two pages' computed styles: a list of lines.
export async function differences(a, b) {
  await settle(a); await settle(b);
  // What a closed <details> holds is laid out lazily, and the first read of
  // a size in it may be the one before layout: every element's size is read
  // once before the styles (verify.mjs).
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

// Takes one rule that matches an element out of the page's sheets, as a
// wrong pruner would: its selector, or null. `has` is a string its selector
// must hold.
export const takeRule = (page, has) => page.evaluate((has) => {
  const visit = (rules) => {
    for (let i = 0; i < rules.length; i++) {
      const rule = rules[i];
      if (rule.selectorText?.includes(has) && rule.style?.length && !rule.selectorText.includes("::") && document.querySelector(rule.selectorText)) {
        const text = rule.selectorText;
        rule.parentRule ? rule.parentRule.deleteRule(i) : rule.parentStyleSheet.deleteRule(i);
        return text;
      }
      if (rule.cssRules) { const found = visit(rule.cssRules); if (found) return found; }
    }
    return null;
  };
  for (const sheet of document.styleSheets) { const found = visit(sheet.cssRules); if (found) return found; }
  return null;
}, has);
