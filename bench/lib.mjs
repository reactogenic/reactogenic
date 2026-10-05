// What site.mjs, verify.mjs and delta.mjs share: the binary, a build of the
// docs site, and the pieces of a built page (specs/phase02/plan.md,
// RGP2-050). No dependency: node >= 22.
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { brotliCompressSync, gzipSync, constants as Z } from "node:zlib";

export const repo = resolve(import.meta.dirname, "..");
export const site = join(repo, "site");
// The site's pages, in the order a visitor of the session opens them.
export const PAGES = ["/", "/guide/", "/syntax/", "/reference/cli/"];

// The catalog run (bench/catalog-site/README.md): a fixture of ten pages on
// the twenty components of bench/catalog, in the order of its session.
export const catalog = join(repo, "bench/catalog");
export const catalogSite = join(repo, "bench/catalog-site");
export const CATALOG_PAGES = ["/", "/pricing/", "/docs/", "/docs/api/", "/changelog/", "/blog/", "/dashboard/", "/settings/", "/contact/", "/404/"];

// The builds that are measured (plan.md, RGP2-050). `never` is not one of the
// three the plan names: it is the default build's blobs as files — what a
// page is, apart from how it is delivered — and the worst case of T8.
export const BUILDS = {
  default: [],
  always: ["--inline", "always"],
  control: ["--no-specialize"],
  never: ["--inline", "never"],
};

// `--name value` and `--name` of a script's own arguments.
export function options(argv = process.argv.slice(2)) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) out._.push(a);
    else if (i + 1 < argv.length && !argv[i + 1].startsWith("--")) out[a.slice(2)] = argv[++i];
    else out[a.slice(2)] = true;
  }
  return out;
}

// The `reactogenic` binary: the path given, $REACTOGENIC_BINARY, or else
// built from go/ once per run.
let built;
export function binary(path) {
  if (path) return resolve(path);
  if (process.env.REACTOGENIC_BINARY) return resolve(process.env.REACTOGENIC_BINARY);
  if (!built) {
    built = join(mkdtempSync(join(tmpdir(), "reactogenic-bench-")), "reactogenic");
    console.error("building the binary: go build -trimpath ./cmd/reactogenic");
    execFileSync("go", ["build", "-trimpath", "-o", built, "./cmd/reactogenic"], { cwd: join(repo, "go"), stdio: ["ignore", "inherit", "inherit"] });
  }
  return built;
}

// `reactogenic build` of the project in `dir` into `out`.
export function build(bin, dir, out, flags = []) {
  const run = spawnSync(bin, ["build", "-p", join(dir, "tsconfig.json"), "--out", out, ...flags], { encoding: "utf8" });
  return { status: run.status ?? 1, stdout: run.stdout ?? "", stderr: run.stderr ?? "" };
}
export function mustBuild(bin, dir, out, flags = []) {
  const run = build(bin, dir, out, flags);
  if (run.status !== 0) {
    process.stderr.write(run.stdout + run.stderr);
    throw new Error(`reactogenic build ${flags.join(" ")} failed with status ${run.status}`);
  }
  return JSON.parse(readFileSync(join(out, "_rg/report.json"), "utf8"));
}

// raw, gzip -9, brotli -q 11: each piece compressed on its own, as measure.mjs does.
export function size(data) {
  const buf = Buffer.isBuffer(data) ? data : Buffer.from(data);
  return {
    raw: buf.length,
    gz: gzipSync(buf, { level: 9 }).length,
    br: brotliCompressSync(buf, { params: { [Z.BROTLI_PARAM_QUALITY]: 11, [Z.BROTLI_PARAM_SIZE_HINT]: buf.length } }).length,
  };
}

// A page of a build, taken apart: the document as written, and — what does
// not depend on `--inline` (builder.md, *The report*) — the HTML as rendered,
// the CSS and the script. Packaging writes two elements, at fixed places
// (builder.md, *Packaging*): the sheet at the end of <head>, the script at
// the end of <body>; they are cut out here, and what the cut gives is checked
// against the report's own numbers by the caller.
export function page(out, entry) {
  const document = readFileSync(join(out, entry.output), "utf8");
  let html = document, css = "", js = "";
  const cut = (open, close, after) => {
    const end = html.lastIndexOf(close + after);
    const start = end < 0 ? -1 : html.lastIndexOf(open, end);
    if (start < 0) throw new Error(`${entry.pathname}: no ${open}…${close} before ${after}`);
    const inner = html.slice(start + open.length, end);
    html = html.slice(0, start) + html.slice(end + close.length);
    return inner;
  };
  // A blob's URL carries `--base`; its file is under `_rg/` of the output.
  const file = (url) => {
    const path = join(out, url.slice(url.lastIndexOf("/_rg/") + 1));
    if (!existsSync(path)) throw new Error(`${entry.pathname}: ${url} is not in the output`);
    return readFileSync(path, "utf8");
  };
  if (entry.css?.delivery === "inline") css = cut("<style>", "</style>", "</head>");
  else if (entry.css?.delivery === "file") {
    cut(`<link rel="stylesheet" href="${entry.css.url}">`, "", "</head>");
    css = file(entry.css.url);
  }
  if (entry.js?.delivery === "inline") js = cut('<script type="module">', "</script>", "</body>");
  else if (entry.js?.delivery === "file") {
    cut(`<script type="module" src="${entry.js.url}">`, "</script>", "</body>");
    js = file(entry.js.url);
  }
  return { document, html, css, js };
}

// The control's script, split (builder.md, *The control*): its behaviours,
// and its own cost — the list of modules, the table from pathname to mounts,
// and the loop that looks the page up. The split is by the shape of the
// generated entry, the last statements of the script.
export function controlEntry(js) {
  const at = [...js.matchAll(/var \w+=\[[\w$,]*\],\w+=\{/g)].at(-1)?.index ?? -1;
  if (at < 0 || !js.slice(at).includes("decodeURIComponent(location.pathname)")) return null;
  return { behaviours: js.slice(0, at), entry: js.slice(at) };
}

// ---- T1: every byte of a page's script (plan.md, RGP2-050) ----------------------
// A script is the behaviours' functions and constants, then the entry: the
// mount calls. Nothing else is a statement of its top level.
function topLevel(js) {
  const out = [];
  let depth = 0, start = 0, quote = "";
  for (let i = 0; i < js.length; i++) {
    const c = js[i];
    if (quote) { if (c === "\\") i++; else if (c === quote) quote = ""; continue; }
    if (c === '"' || c === "'" || c === "`") quote = c;
    else if (c === "{" || c === "(" || c === "[") depth++;
    else if (c === "}" || c === ")" || c === "]") {
      depth--;
      // A function declaration ends at its brace: no semicolon follows it.
      if (depth === 0 && c === "}" && js.startsWith("function ", start)) { out.push(js.slice(start, i + 1)); start = i + 1; }
    } else if (c === ";" && depth === 0) { out.push(js.slice(start, i + 1)); start = i + 1; }
  }
  if (start < js.length) out.push(js.slice(start));
  return out;
}
// A mount call: the module's function, its element and — the mount's data
// (builder.md, *Behaviours*) — an object literal; or nothing, for a
// behaviour of the page.
const MOUNT_CALL = /^[\w$]+\((document\.getElementById\("[^"]*"\)(,\{[\s\S]*\})?)?\);$/;
const RUNTIME = /react|hydrat|createElement|createRoot|customElements|innerHTML|import\(|eval\(|new Function/i;
// `p` is a page of a build: its report entry, its parts (`page()`) and their sizes.
export function scriptChecks(pathname, p) {
  const rows = p.entry.modules ?? [];
  const sum = rows.reduce((a, r) => a + r.bytes, 0);
  const entryBytes = rows.find((r) => r.path === "<entry>")?.bytes ?? 0;
  const statements = topLevel(p.js);
  const calls = statements.filter((s) => !s.startsWith("function ") && !s.startsWith("var "));
  const scripts = [...p.document.matchAll(/<script\b/g)].length;
  const handlers = [...p.html.matchAll(/<[a-z][^>]*\s(on[a-z]+)=/gi)].map((m) => m[1]);
  const mounted = new Set((p.entry.mounts ?? []).map((m) => m.module));
  const checks = {
    "the rows add up to the script": sum === p.sizes.js.raw,
    "no <runtime> row": !rows.some((r) => r.path === "<runtime>"),
    "every row is a mounted behaviour or the entry": rows.every((r) => r.path === "<entry>" || mounted.has(r.module)),
    "the script's only statements that run are the entry's mount calls": calls.length > 0 && calls.every((s) => MOUNT_CALL.test(s)) && Buffer.byteLength(calls.join("")) === entryBytes,
    "one <script> in the document, the builder's": scripts === 1,
    // In an attribute's value: the page may say the word in its text (the
    // site documents `shell-script`).
    "no handler attribute, no javascript: URL": handlers.length === 0 && !/<[a-z][^>]*=\s*["']?\s*javascript:/i.test(p.html),
    "nothing of React or of a runtime in the script": !RUNTIME.test(p.js),
  };
  return { pathname, rows, sum, entryBytes, calls: calls.join(""), mounts: p.entry.mounts ?? [], checks, ok: Object.values(checks).every(Boolean) };
}

// ---- a sheet as a list of items, and the difference of two ----------------------
// The sheet is esbuild's flat, minified output. It is read into items: a
// style rule is one item per selector of its list, under the at-rules around
// it; an at-rule that holds no style rules (@keyframes, @position-try, a
// statement) is one item.
const GROUPS = /^@(layer|media|supports|container|scope|starting-style)\b/;
function split(text, sep) {
  const out = [];
  let depth = 0, start = 0, quote = "";
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quote) { if (c === "\\") i++; else if (c === quote) quote = ""; continue; }
    if (c === '"' || c === "'") quote = c;
    else if (c === "(" || c === "[") depth++;
    else if (c === ")" || c === "]") depth--;
    else if (c === sep && depth === 0) { out.push(text.slice(start, i)); start = i + 1; }
  }
  out.push(text.slice(start));
  return out;
}
export function cssItems(css, context = "", out = []) {
  let i = 0;
  while (i < css.length) {
    let depth = 0, quote = "", j = i;
    for (; j < css.length; j++) {
      const c = css[j];
      if (quote) { if (c === "\\") j++; else if (c === quote) quote = ""; continue; }
      if (c === '"' || c === "'") quote = c;
      else if (c === "(" || c === "[") depth++;
      else if (c === ")" || c === "]") depth--;
      else if ((c === "{" || c === ";") && depth === 0) break;
    }
    const prelude = css.slice(i, j).trim();
    if (j >= css.length || css[j] === ";") { if (prelude) out.push({ context, selector: prelude, body: null }); i = j + 1; continue; }
    let k = j + 1;
    for (let d = 1; k < css.length && d > 0; k++) {
      const c = css[k];
      if (quote) { if (c === "\\") k++; else if (c === quote) quote = ""; continue; }
      if (c === '"' || c === "'") quote = c;
      else if (c === "{") d++;
      else if (c === "}") d--;
    }
    const body = css.slice(j + 1, k - 1);
    if (GROUPS.test(prelude)) cssItems(body, context ? `${context} ${prelude}` : prelude, out);
    else if (prelude.startsWith("@")) out.push({ context, selector: prelude, body });
    else for (const selector of split(prelude, ",")) out.push({ context, selector: selector.trim(), body });
    i = k;
  }
  return out;
}
export const cssKey = (it) => `${it.context} | ${it.selector} {${it.body ?? ";"}}`;
// The items of `a` that `b` has not, as multisets.
export function minus(a, b) {
  const key = cssKey;
  const count = new Map();
  for (const it of b) count.set(key(it), (count.get(key(it)) ?? 0) + 1);
  return a.filter((it) => { const c = count.get(key(it)) ?? 0; if (c > 0) count.set(key(it), c - 1); return c === 0; });
}

export const n = (x) => x.toLocaleString("en-US");
export const tri = (s) => `${n(s.raw)} / ${n(s.gz)} / ${n(s.br)}`;
// How much smaller `a` is than `b`, as a percentage, to one decimal.
export const smaller = (a, b) => (b === 0 ? "—" : `${((1 - a / b) * 100).toFixed(1)}%`);
export const percent = (a, b) => (1 - a / b) * 100;
