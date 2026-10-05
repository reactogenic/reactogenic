#!/usr/bin/env node
// measure.mjs — what does each page of a static site cost the browser?
//
// Usage:
//   node measure.mjs <name>=<distDir>[:spa] [<name>=<distDir> ...] [options]
//
//   <distDir>   a directory of static files, served as-is from "/"
//   :spa        unknown paths without an extension get /index.html (single-page apps)
//
// Options:
//   --pages /,/getting-started/,/syntax/,/api/   paths to visit (default: these four)
//   --json out.json      write the raw numbers
//   --md out.md          write Markdown tables (also printed to stdout)
//   --static             do not launch a browser: follow <link>/<script>/modulepreload and
//                        static `import`s only (misses dynamic imports and island loaders)
//   --budget 6000        virtual-time budget per page load, ms (default 6000)
//   --runs 3             cold loads per page; the union of their requests is counted (default 3)
//   CHROME=/path/to/chrome   browser binary (default: Google Chrome on macOS, then PATH)
//
// Example (Reactogenic's own output, later):
//   node measure.mjs reactogenic=../../site/dist floor=d-floor/dist-page --md results/rg.md
//
// Method. Each page is loaded COLD (fresh browser profile) in headless Chrome from a local
// server that logs every request; nothing is clicked. A page is loaded --runs times and the
// union of the requests is taken, because idle-time work is not deterministic in one run. The files requested are then sized on
// disk: raw, gzip -9, brotli -q 11 (each file compressed on its own, as a CDN would).
//   requests  = every URL fetched by the load, the document included
//   HTML      = the document
//   CSS / JS  = external files of that type; "inl" = <style> / inline <script> bytes, which
//               are counted inside HTML for transfer and shown separately for information
//   other     = anything else the load fetched (JSON, RSC payloads, prefetches, fonts, images)
//   JS parse  = raw bytes of external JS + inline executable <script>: what the engine must
//               parse before the page is fully interactive without user input
//   session   = the pages visited in order with a warm HTTP cache: each URL counted once
// No dependencies: node >= 20 (zlib has brotli).
import { createServer } from "node:http";
import { readFileSync, existsSync, statSync, writeFileSync, mkdtempSync, rmSync } from "node:fs";
import { join, extname, resolve, posix } from "node:path";
import { tmpdir } from "node:os";
import { spawn } from "node:child_process";
import { gzipSync, brotliCompressSync, constants as Z } from "node:zlib";

const args = process.argv.slice(2);
const opt = (name, dflt) => {
  const i = args.indexOf("--" + name);
  if (i < 0) return dflt;
  const v = args[i + 1];
  args.splice(i, 2);
  return v;
};
const flag = (name) => {
  const i = args.indexOf("--" + name);
  if (i < 0) return false;
  args.splice(i, 1);
  return true;
};
const pages = opt("pages", "/,/getting-started/,/syntax/,/api/").split(",");
const jsonOut = opt("json");
const mdOut = opt("md");
const budget = Number(opt("budget", 6000));
const runs = Number(opt("runs", 3));
const staticOnly = flag("static");
const targets = args.map((a) => {
  const [name, rest] = a.split("=");
  if (!rest) throw new Error(`expected name=dir, got ${a}`);
  const spa = rest.endsWith(":spa");
  return { name, dir: resolve(spa ? rest.slice(0, -4) : rest), spa };
});
if (!targets.length) {
  console.error("usage: node measure.mjs <name>=<distDir>[:spa] ... [--md out.md] [--json out.json]");
  process.exit(2);
}

const CHROME =
  process.env.CHROME ||
  ["/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/chromium-browser"].find(existsSync);

const TYPES = { ".html": "text/html", ".css": "text/css", ".js": "text/javascript", ".mjs": "text/javascript", ".json": "application/json", ".svg": "image/svg+xml", ".txt": "text/plain", ".woff2": "font/woff2", ".png": "image/png", ".ico": "image/x-icon", ".map": "application/json", ".rsc": "text/x-component" };

function resolveFile(t, urlPath) {
  let p = decodeURIComponent(urlPath.split("?")[0].split("#")[0]);
  let f = join(t.dir, p);
  if (existsSync(f) && statSync(f).isDirectory()) f = join(f, "index.html");
  if (!existsSync(f) && existsSync(f + ".html")) f += ".html";
  if (!existsSync(f) && t.spa && !extname(p)) f = join(t.dir, "index.html");
  return existsSync(f) && statSync(f).isFile() ? f : null;
}

function serve(t) {
  const log = [];
  const server = createServer((req, res) => {
    const f = resolveFile(t, req.url);
    if (req.url !== "/favicon.ico") log.push({ url: req.url, file: f });
    if (!f) return res.writeHead(404).end("not found");
    res.writeHead(200, { "content-type": TYPES[extname(f)] || "application/octet-stream", "cache-control": "no-store" });
    res.end(readFileSync(f));
  });
  return new Promise((ok) => server.listen(0, "127.0.0.1", () => ok({ server, log, port: server.address().port })));
}

function chrome(url) {
  const profile = mkdtempSync(join(tmpdir(), "measure-"));
  return new Promise((ok, fail) => {
    const p = spawn(CHROME, ["--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-background-networking", "--disable-component-update", "--disable-sync", "--window-size=1280,900", `--user-data-dir=${profile}`, `--virtual-time-budget=${budget}`, "--dump-dom", url], { stdio: ["ignore", "pipe", "ignore"] });
    let dom = "", quiet;
    // Chrome's new headless mode prints the DOM but does not always exit: stop it once the
    // dump is complete (it is written after the virtual-time budget has run out).
    p.stdout.on("data", (d) => {
      dom += d;
      clearTimeout(quiet);
      quiet = setTimeout(() => p.kill("SIGKILL"), 400);
    });
    const timer = setTimeout(() => p.kill("SIGKILL"), 45000);
    p.on("error", fail);
    p.on("close", () => {
      clearTimeout(timer);
      try { rmSync(profile, { recursive: true, force: true }); } catch {}
      ok(dom);
    });
  });
}

// --static fallback: resources named in the HTML, plus static imports of modules
function crawl(t, page) {
  const seen = new Map();
  const add = (u, from) => {
    if (/^(https?:)?\/\//.test(u) || u.startsWith("data:")) return;
    const abs = u.startsWith("/") ? u : posix.join(posix.dirname(from), u);
    if (seen.has(abs)) return;
    const f = resolveFile(t, abs);
    seen.set(abs, f);
    if (f && /\.m?js$/.test(f)) for (const m of readFileSync(f, "utf8").matchAll(/(?:import|from)\s*["']([^"']+)["']/g)) add(m[1], abs);
  };
  const f = resolveFile(t, page);
  seen.set(page, f);
  const html = readFileSync(f, "utf8");
  for (const m of html.matchAll(/<(?:script|link)\b[^>]*?\b(?:src|href)=["']?([^"'\s>]+)/g))
    if (/<script/.test(m[0]) || /rel=["']?(stylesheet|modulepreload)/.test(m[0])) add(m[1], page);
  return [...seen].map(([url, file]) => ({ url, file }));
}

const sizes = new Map();
function size(buf) {
  return { raw: buf.length, gz: gzipSync(buf, { level: 9 }).length, br: brotliCompressSync(buf, { params: { [Z.BROTLI_PARAM_QUALITY]: 11, [Z.BROTLI_PARAM_SIZE_HINT]: buf.length } }).length };
}
function sizeFile(f) {
  if (!sizes.has(f)) sizes.set(f, size(readFileSync(f)));
  return sizes.get(f);
}
const zero = () => ({ raw: 0, gz: 0, br: 0 });
const add = (a, b) => ((a.raw += b.raw), (a.gz += b.gz), (a.br += b.br), a);
const kind = (f) => ({ ".html": "html", ".css": "css", ".js": "js", ".mjs": "js" })[extname(f)] || "other";

function inline(html) {
  let js = 0, css = 0, data = 0;
  for (const m of html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/g)) {
    if (/\bsrc=/.test(m[1])) continue;
    const type = /\btype=["']?([^"'\s>]+)/.exec(m[1])?.[1];
    if (!type || type === "module" || /javascript/.test(type)) js += Buffer.byteLength(m[2]);
    else data += Buffer.byteLength(m[2]);
  }
  for (const m of html.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)) css += Buffer.byteLength(m[1]);
  return { js, css, data };
}

const results = [];
for (const t of targets) {
  const { server, log, port } = await serve(t);
  const perPage = [];
  const sessionSeen = new Set();
  const session = { requests: 0, html: zero(), css: zero(), js: zero(), other: zero(), total: zero() };
  for (const page of pages) {
    let reqs;
    if (staticOnly || !CHROME) reqs = crawl(t, page);
    else {
      // Idle-time loads (requestIdleCallback, link prefetch) may or may not fit one run's
      // budget: load the page `runs` times, each cold, and take the union of what was fetched.
      const uniq = new Map();
      for (let i = 0; i < runs; i++) {
        log.length = 0;
        await chrome(`http://127.0.0.1:${port}${page}`);
        for (const r of log) if (!uniq.has(r.url)) uniq.set(r.url, r);
      }
      reqs = [...uniq.values()];
    }
    const row = { page, requests: reqs.length, missing: [], files: [], html: zero(), css: zero(), js: zero(), other: zero(), total: zero(), inline: { js: 0, css: 0, data: 0 }, jsParse: 0 };
    let first = true;
    for (const r of reqs) {
      if (!r.file) { row.missing.push(r.url); continue; }
      const k = first ? "html" : kind(r.file) === "html" ? "other" : kind(r.file);
      const s = sizeFile(r.file);
      add(row[k], s); add(row.total, s);
      row.files.push({ url: r.url, kind: k, ...s });
      if (k === "html") row.inline = inline(readFileSync(r.file, "utf8"));
      if (k === "js") row.jsParse += s.raw;
      const key = k === "html" ? "doc:" + page : r.file;
      if (!sessionSeen.has(key)) { sessionSeen.add(key); session.requests++; add(session[k], s); add(session.total, s); }
      first = false;
    }
    row.jsParse += row.inline.js;
    perPage.push(row);
  }
  server.close();
  results.push({ name: t.name, dir: t.dir, mode: staticOnly || !CHROME ? "static" : "chrome", pages: perPage, session });
}

// ---- report ---------------------------------------------------------------------
const n = (x) => x.toLocaleString("en-US");
const tri = (s) => `${n(s.raw)} / ${n(s.gz)} / ${n(s.br)}`;
let md = "";
for (const r of results) {
  md += `\n### ${r.name}\n\n\`${r.dir}\` — loaded with ${r.mode === "chrome" ? "headless Chrome, cold cache" : "static crawl"}. Bytes are raw / gzip -9 / brotli -q 11.\n\n`;
  md += `| Page | Req | HTML | CSS | JS | Other | Total | Inline JS / CSS / data (raw, inside HTML) | JS to parse (raw) |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n`;
  for (const p of r.pages)
    md += `| \`${p.page}\` | ${p.requests} | ${tri(p.html)} | ${tri(p.css)} | ${tri(p.js)} | ${tri(p.other)} | ${tri(p.total)} | ${n(p.inline.js)} / ${n(p.inline.css)} / ${n(p.inline.data)} | ${n(p.jsParse)} |\n`;
  md += `| **session (${r.pages.length} pages, warm cache)** | ${r.session.requests} | ${tri(r.session.html)} | ${tri(r.session.css)} | ${tri(r.session.js)} | ${tri(r.session.other)} | ${tri(r.session.total)} | | |\n`;
  const miss = r.pages.flatMap((p) => p.missing.map((m) => `${p.page} -> ${m}`));
  if (miss.length) md += `\nRequests that 404ed (not counted): ${miss.map((m) => `\`${m}\``).join(", ")}\n`;
}
const mean = (xs) => Math.round(xs.reduce((a, b) => a + b, 0) / xs.length);
md += `\n### Summary\n\nMeans over ${pages.length} cold page loads; brotli -q 11 unless marked raw.\n\n`;
md += `| Approach | Req / page | HTML br | CSS br (ext + inline raw) | JS br (ext) | JS to parse, raw | Other br | Page total br | Session total br | Session JS br |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n`;
for (const r of results) {
  const P = r.pages;
  md += `| ${r.name} | ${mean(P.map((p) => p.requests))} | ${n(mean(P.map((p) => p.html.br)))} | ${n(mean(P.map((p) => p.css.br)))} + ${n(mean(P.map((p) => p.inline.css)))} | ${n(mean(P.map((p) => p.js.br)))} | ${n(mean(P.map((p) => p.jsParse)))} | ${n(mean(P.map((p) => p.other.br)))} | ${n(mean(P.map((p) => p.total.br)))} | ${n(r.session.total.br)} | ${n(r.session.js.br)} |\n`;
}
console.log(md);
if (mdOut) writeFileSync(mdOut, md);
if (jsonOut) writeFileSync(jsonOut, JSON.stringify(results, null, 1));
