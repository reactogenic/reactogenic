#!/usr/bin/env node
// factor.mjs — a STUDY of packagings, for the owner's next decision
// (specs/phase02/builder.md, *Packaging*, OPEN: the factoring policy;
// decisions.md, K, rules 1–3). It builds nothing that ships.
//
//   node bench/factor.mjs [--binary <reactogenic>] [--md bench/results/factor.md] [--keep <dir>]
//
// Analysis is per artifact and exact: a page's sheet holds the rules that can
// match on that page, its script the behaviours it mounted. Packaging decides
// how that reaches the browser. Today it inlines both in every page and
// shares a blob only when it is the same bytes; over a visit the control —
// one sheet for the site, fetched once — transfers less (bet.md). This
// script asks what other packagings of THE SAME analysis would transfer.
//
// For the docs site (`site/`) and the catalog site (`bench/catalog-site`) it
// builds the default build and the control with the `reactogenic` binary —
// the one given, $REACTOGENIC_BINARY, or built here from go/ — and takes
// from the default build, per page:
//
//   CSS   the page's pruned sheet, split into UNITS with a stable identity:
//         its at-rules entered (`@layer`, `@media`, …), a style rule is one
//         unit per selector of its list — the pruner trims lists — and, for
//         a rule that declares a custom property, per declaration — the
//         pruner drops those one by one. A unit is named by the at-rules
//         around it and its text; its place is its place in the control's
//         sheet, which is every page's sheet before pruning: source order.
//         Every sheet is written back from its units to the byte, or the
//         script fails.
//   JS    the page's script, cut into its modules by the rows of
//         `_rg/report.json` (`modules`: esbuild's metafile), and its entry —
//         the mount calls. A module built under other flags is another
//         module. (The minifier names things per script: a module is the
//         same bytes on two pages but for its one-letter names.)
//
// Then, for each candidate packaging, it writes the files that packaging
// would — shared files, and each page's document with what stays inlined —
// and sizes them as measure.mjs does: each file on its own, raw, gzip -9,
// brotli -q 11. A page loaded cold fetches its document, the files it
// links, and the site's static files it shows (the favicon, a picture); a
// session is the pages in lib.mjs's order with a warm cache, each URL once.
//
// What it does not do: run anything in a browser (the cold loads are
// computed from the files, which on these sites is what Chrome fetches:
// `site.mjs`, `catalog.mjs`); rebuild a script — a shared script file is the
// modules' text as the pages' own builds have it plus an `export`, a page's
// inline script an `import` and its entry, so two modules' one-letter names
// may collide: the bytes are a packaging's, the files are not runnable; or
// prove a cascade: for each candidate it says what would guarantee the same
// computed styles, and counts the pairs of rules whose order the candidate
// changes.
//
// The output is one Markdown file (and stdout); it names no path and no
// version of this machine: a second run writes the same bytes. No dependency
// (node >= 22).
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { CATALOG_PAGES, PAGES, binary, catalogSite, mustBuild, n, options, page, repo, site, size } from "./lib.mjs";

const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/factor.md"));
const bin = binary(opts.binary);
const work = opts.keep ? resolve(opts.keep) : mkdtempSync(join(tmpdir(), "reactogenic-bench-factor-"));
mkdirSync(work, { recursive: true });

const problems = []; // something here is not what the builds say: the study is wrong
const must = (ok, text) => { if (!ok) problems.push(text); return ok; };

// ---- a sheet as units ------------------------------------------------------------
const GROUPS = /^@(layer|media|supports|container|scope|starting-style)\b/;
// `text` at the top-level occurrences of `sep`: outside strings, brackets and braces.
function splitTop(text, sep) {
  const out = [];
  let depth = 0, start = 0, quote = "";
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quote) { if (c === "\\") i++; else if (c === quote) quote = ""; continue; }
    if (c === '"' || c === "'") quote = c;
    else if (c === "(" || c === "[" || c === "{") depth++;
    else if (c === ")" || c === "]" || c === "}") depth--;
    else if (c === sep && depth === 0) { out.push(text.slice(start, i)); start = i + 1; }
  }
  out.push(text.slice(start));
  return out;
}
// The units of a sheet, in order. `path` is the at-rules around a unit, each
// with the number of that block in the sheet — two `@layer rg.components{}`
// blocks are two files' — and `rule` the number of its rule.
function parse(css) {
  const units = [];
  let blocks = 0, rules = 0;
  (function walk(text, path) {
    for (let i = 0; i < text.length; ) {
      let depth = 0, quote = "", j = i;
      for (; j < text.length; j++) {
        const c = text[j];
        if (quote) { if (c === "\\") j++; else if (c === quote) quote = ""; continue; }
        if (c === '"' || c === "'") quote = c;
        else if (c === "(" || c === "[") depth++;
        else if (c === ")" || c === "]") depth--;
        else if ((c === "{" || c === ";") && depth === 0) break;
      }
      const prelude = text.slice(i, j).trim();
      if (j >= text.length || text[j] === ";") {
        if (prelude) units.push({ path, rule: rules++, kind: "statement", selector: prelude, body: null });
        i = j + 1;
        continue;
      }
      let k = j + 1;
      for (let d = 1; k < text.length && d > 0; k++) {
        const c = text[k];
        if (quote) { if (c === "\\") k++; else if (c === quote) quote = ""; continue; }
        if (c === '"' || c === "'") quote = c;
        else if (c === "{") d++;
        else if (c === "}") d--;
      }
      const body = text.slice(j + 1, k - 1);
      if (GROUPS.test(prelude)) walk(body, [...path, { prelude, inst: blocks++ }]);
      else if (prelude.startsWith("@")) units.push({ path, rule: rules++, kind: "at", selector: prelude, body });
      else {
        const rule = rules++;
        const declarations = splitTop(body, ";");
        const parts = declarations.some((d) => d.trimStart().startsWith("--")) ? declarations : [body];
        for (const selector of splitTop(prelude, ",")) for (const part of parts) units.push({ path, rule, kind: "style", selector: selector.trim(), body: part });
      }
      i = k;
    }
  })(css, []);
  const seen = new Map();
  for (const u of units) {
    const text = `${u.path.map((p) => p.prelude).join(" ")} | ${u.selector} {${u.body ?? ";"}}`;
    const nth = seen.get(text) ?? 0;
    seen.set(text, nth + 1);
    u.key = `${text} #${nth}`;
  }
  return units;
}
// Units, in source order, as the sheet they are: blocks reopened where the
// source had them, a rule's selectors joined again where they declare the same.
function serialize(units) {
  let out = "";
  const open = [];
  for (let i = 0; i < units.length; ) {
    const u = units[i];
    let common = 0;
    while (common < open.length && common < u.path.length && open[common].inst === u.path[common].inst) common++;
    while (open.length > common) { out += "}"; open.pop(); }
    for (let k = common; k < u.path.length; k++) { out += u.path[k].prelude + "{"; open.push(u.path[k]); }
    if (u.kind === "statement") { out += u.selector + ";"; i++; continue; }
    if (u.kind === "at") { out += `${u.selector}{${u.body}}`; i++; continue; }
    const groups = [];
    let j = i;
    for (; j < units.length && units[j].kind === "style" && units[j].rule === u.rule && units[j].path.at(-1)?.inst === u.path.at(-1)?.inst; j++) {
      const last = groups.at(-1);
      if (last && last.selector === units[j].selector) last.declarations.push(units[j].body);
      else groups.push({ selector: units[j].selector, declarations: [units[j].body] });
    }
    for (let g = 0; g < groups.length; ) {
      const body = groups[g].declarations.join(";");
      let h = g + 1;
      while (h < groups.length && groups[h].declarations.join(";") === body) h++;
      out += `${groups.slice(g, h).map((x) => x.selector).join(",")}{${body}}`;
      g = h;
    }
    i = j;
  }
  return out + "}".repeat(open.length);
}

// ---- specificity, for the pairs whose order a packaging changes --------------------
function specificity(selector) {
  let a = 0, b = 0, c = 0;
  const add = (s) => { a += s[0]; b += s[1]; c += s[2]; };
  const max = (list) => splitTop(list, ",").map((s) => specificity(s.trim())).reduce((m, s) => (s[0] - m[0] || s[1] - m[1] || s[2] - m[2]) > 0 ? s : m, [0, 0, 0]);
  const ident = /^-?[_a-zA-Z -￿][-\w -￿]*/;
  let start = true; // at the start of a compound: an identifier here is a type
  for (let i = 0; i < selector.length; ) {
    const c0 = selector[i], rest = selector.slice(i);
    if (/[\s>+~]/.test(c0)) { start = true; i++; continue; }
    if (c0 === "*" || c0 === "&") { start = false; i++; continue; }
    if (c0 === "#") { a++; i += 1 + (ident.exec(rest.slice(1))?.[0].length ?? 0); start = false; continue; }
    if (c0 === ".") { b++; i += 1 + (ident.exec(rest.slice(1))?.[0].length ?? 0); start = false; continue; }
    if (c0 === "[") { b++; let d = 0, q = ""; for (; i < selector.length; i++) { const ch = selector[i]; if (q) { if (ch === "\\") i++; else if (ch === q) q = ""; } else if (ch === '"' || ch === "'") q = ch; else if (ch === "[") d++; else if (ch === "]" && --d === 0) { i++; break; } } start = false; continue; }
    if (c0 === ":") {
      const element = selector[i + 1] === ":";
      const name = ident.exec(rest.slice(element ? 2 : 1))?.[0] ?? "";
      i += (element ? 2 : 1) + name.length;
      let args = null;
      if (selector[i] === "(") { let d = 0; const from = i + 1; for (; i < selector.length; i++) { if (selector[i] === "(") d++; else if (selector[i] === ")" && --d === 0) break; } args = selector.slice(from, i); i++; }
      if (element || ["before", "after", "first-line", "first-letter"].includes(name)) c++;
      else if (name === "where") { /* nothing */ }
      else if (["is", "not", "has", "matches"].includes(name) && args !== null) add(max(args));
      else if (/^nth-(last-)?child$/.test(name) && args !== null && / of /.test(args)) { b++; add(max(args.slice(args.indexOf(" of ") + 4))); }
      else b++;
      start = false;
      continue;
    }
    const word = start ? ident.exec(rest)?.[0] : null;
    if (word) { c++; i += word.length; start = false; continue; }
    i++;
  }
  return [a, b, c];
}
const properties = (u) => (u.kind === "style" ? splitTop(u.body, ";").map((d) => d.slice(0, d.indexOf(":")).trim()).filter(Boolean) : []);
const layerOf = (u) => u.path.filter((p) => p.prelude.startsWith("@layer")).map((p) => p.prelude).join(" ");

// ---- a site, read ------------------------------------------------------------------
const cache = new Map();
const sized = (text) => { let s = cache.get(text); if (!s) cache.set(text, (s = size(text))); return s; };
const zero = () => ({ raw: 0, gz: 0, br: 0 });
const plus = (a, b) => ({ raw: a.raw + b.raw, gz: a.gz + b.gz, br: a.br + b.br });
const sum = (list) => list.reduce(plus, zero());
const hash = (text) => createHash("sha256").update(text).digest("hex").slice(0, 8);
const tri = (s) => `${n(s.raw)} / ${n(s.gz)} / ${n(s.br)}`;
const mean = (list) => Math.round(list.reduce((a, b) => a + b, 0) / list.length);
const pct = (x) => `${x.toFixed(1)}%`;
const tick = (s) => "`" + s + "`";

function read(name, dir, pages) {
  const outs = { default: join(work, name, "default"), control: join(work, name, "control") };
  const reports = { default: mustBuild(bin, dir, outs.default, []), control: mustBuild(bin, dir, outs.control, ["--no-specialize"]) };
  const entry = (kind, pathname) => reports[kind].pages.find((p) => p.pathname === pathname && p.variant === "index") ?? (() => { throw new Error(`${name}, ${kind}: no page ${pathname}`); })();
  const control = pages.map((pathname) => ({ pathname, entry: entry("control", pathname), ...page(outs.control, entry("control", pathname)) }));
  must(new Set(control.map((p) => p.css)).size === 1 && new Set(control.map((p) => p.js)).size === 1, `${name}: the control's sheet or script is not the same on every page`);
  const source = parse(control[0].css); // every page's sheet before pruning: the order of the source
  must(serialize(source) === control[0].css, `${name}: the control's sheet is not written back from its units to the byte`);
  const index = new Map(source.map((u, i) => [u.key, i]));
  source.forEach((u, i) => { u.at = i; });
  const extra = []; // units of a page that the control's sheet has not: placed after the unit before them
  const artifacts = pages.map((pathname) => {
    const e = entry("default", pathname);
    const parts = page(outs.default, e);
    must(parts.html === control.find((c) => c.pathname === pathname).html, `${name} ${pathname}: the control's HTML is not the default build's`);
    let before = -1, nth = 0;
    const units = parse(parts.css).map((u) => {
      const at = index.get(u.key);
      if (at !== undefined) { before = at; nth = 0; return source[at]; }
      const own = { ...u, at: before + ++nth / 1000, path: u.path.map((p) => ({ ...p, inst: `${pathname}:${p.inst}` })), rule: `${pathname}:${u.rule}`, key: `${u.key} @${pathname}` };
      extra.push(own);
      return own;
    });
    must(units.every((u, i) => i === 0 || units[i - 1].at < u.at), `${name} ${pathname}: the sheet's units are not in the order of the control's sheet`);
    must(serialize(units) === parts.css, `${name} ${pathname}: the sheet is not written back from its units to the byte`);
    // The script, by the report's rows: the modules in the order they are in it, then the entry.
    const rows = e.modules ?? [];
    must(rows.reduce((a, r) => a + r.bytes, 0) === Buffer.byteLength(parts.js), `${name} ${pathname}: the report's rows do not add up to the script`);
    const modules = [];
    let offset = 0, mounts = "";
    const bytes = Buffer.from(parts.js);
    for (const row of rows) {
      const text = bytes.subarray(offset, offset + row.bytes).toString();
      offset += row.bytes;
      if (row.path === "<entry>") { mounts = text; continue; }
      must(/^(function |var |let |const |class )/.test(text), `${name} ${pathname}: the row of ${row.path} does not start a statement of the script`);
      const fn = /function ([\w$]+)\(/.exec(text)?.[1] ?? "m";
      // The same module is the same text on two pages but for the names the minifier chose.
      const shape = text.replace(/(?<![\w$.])[A-Za-z_$](?![\w$])/g, "_");
      modules.push({ path: row.path, text, fn, key: `${row.path} ${hash(shape)}`, bytes: row.bytes });
    }
    must(mounts !== "" || rows.length === 0, `${name} ${pathname}: the script has no entry row`);
    must(!/\bfunction\b/.test(mounts), `${name} ${pathname}: the entry is not the mount calls alone`);
    // What the page shows of the site's static files: fetched with it, cached after.
    const statics = [...parts.html.matchAll(/<link\b[^>]*\brel="(?:shortcut )?icon"[^>]*\bhref="([^"]+)"|<img\b[^>]*\bsrc="([^"]+)"/g)].map((m) => m[1] ?? m[2]).filter((url) => url.startsWith("/") && existsSync(join(outs.default, url)));
    return { pathname, entry: e, ...parts, units, modules, mounts, statics: [...new Set(statics)] };
  });
  const statics = Object.fromEntries([...new Set(artifacts.flatMap((a) => a.statics))].map((url) => [url, size(readFileSync(join(outs.default, url)))]));
  return { name, pages, outs, artifacts, control, source, extra, statics };
}

// ---- packagings ---------------------------------------------------------------------
// A packaging of the CSS is, per page, what its <head> gets, in order: a
// file, or units inlined. Of the JS: the files a page's script imports, and
// the modules that stay in it. `null` is the control: its own files.
const needs = (s, pick) => { const count = new Map(); for (const a of s.artifacts) for (const k of new Set(pick(a))) count.set(k, (count.get(k) ?? 0) + 1); return count; };
const byPlace = (units) => [...units].sort((x, y) => x.at - y.at);

// The component a unit is of: the source's top-level block it stands in — a
// file of the design system is one `@layer` block — named by the root class
// its rules start from; what is in no layer is the project's own.
function groupsOf(s) {
  const names = new Map(); // top-level block → name
  for (const u of [...s.source, ...s.extra]) {
    const top = u.path[0];
    if (!top || !top.prelude.startsWith("@layer")) continue;
    const root = /(?:\.|--)((?:rg|bc)-[a-z]+)/.exec(u.selector)?.[1];
    if (root && !names.has(top.inst)) names.set(top.inst, root);
  }
  return (u) => {
    const top = u.path[0];
    if (top && top.prelude.startsWith("@layer")) return names.get(top.inst) ?? "tokens";
    return u.kind === "statement" && u.selector.startsWith("@layer") ? "tokens" : "site";
  };
}

function cssPlans(s) {
  const count = needs(s, (a) => a.units);
  const all = byPlace([...count.keys()]);
  const pages = s.artifacts.length;
  const shared = (at) => all.filter((u) => count.get(u) >= at);
  const split = (file) => (a) => {
    const set = new Set(file);
    const rest = a.units.filter((u) => !set.has(u));
    return [...(a.units.some((u) => set.has(u)) ? [{ file }] : []), ...(rest.length ? [{ inline: rest }] : [])];
  };
  let prefix = 0;
  while (s.artifacts.every((a) => a.units[prefix] !== undefined && a.units[prefix] === s.artifacts[0].units[prefix])) prefix++;
  const group = groupsOf(s);
  const groups = [...new Set(all.map(group))];
  const union = Object.fromEntries(groups.map((g) => [g, all.filter((u) => group(u) === g)]));
  // Per component, each page's own part of it; the same bytes on two pages are one file.
  const own = (a, g) => a.units.filter((u) => group(u) === g);
  const sameOn = new Map();
  for (const a of s.artifacts) for (const g of groups) { const text = serialize(own(a, g)); if (text) sameOn.set(`${g}\n${text}`, (sameOn.get(`${g}\n${text}`) ?? 0) + 1); }
  const files = new Map(); // one array per file, so that it is one file
  const fileOf = (text, units) => { if (!files.has(text)) files.set(text, units); return files.get(text); };
  return {
    groups, group, count, all,
    plans: {
      a: (a) => [{ inline: a.units }],
      b0: split(s.artifacts[0].units.slice(0, prefix)),
      b: split(shared(pages)),
      c2: split(shared(Math.min(2, pages))),
      c3: split(shared(Math.min(3, pages))),
      ch: split(shared(Math.ceil(pages / 2))),
      d1: (a) => groups.filter((g) => own(a, g).length).map((g) => ({ file: union[g] })),
      d2: (a) => {
        const out = [];
        for (const g of groups) {
          const units = own(a, g);
          if (!units.length) continue;
          const text = serialize(units);
          if (sameOn.get(`${g}\n${text}`) > 1) out.push({ file: fileOf(`${g}\n${text}`, units) });
          else if (out.at(-1)?.inline) out.at(-1).inline.push(...units);
          else out.push({ inline: [...units] });
        }
        return out;
      },
      e: () => [{ file: all }],
    },
  };
}

function jsPlans(s) {
  const count = needs(s, (a) => a.modules.map((m) => m.key));
  const first = new Map(); // a module as the first page that has it built it
  for (const a of s.artifacts) for (const m of a.modules) if (!first.has(m.key)) first.set(m.key, m);
  const pages = s.artifacts.length;
  const shared = (at) => [...first.values()].filter((m) => count.get(m.key) >= at);
  const split = (file) => (a) => {
    const keys = new Set(file.map((m) => m.key));
    const mine = a.modules.filter((m) => keys.has(m.key));
    return { imports: mine.length ? [{ file, names: mine.map((m) => m.fn) }] : [], inline: a.modules.filter((m) => !keys.has(m.key)) };
  };
  // Per behaviour module, with every flag any page turns on: its largest build.
  const largest = new Map();
  for (const m of first.values()) if (!largest.has(m.path) || largest.get(m.path)[0].bytes < m.bytes) largest.set(m.path, [m]);
  const single = new Map([...first.values()].map((m) => [m.key, [m]]));
  return {
    count, first,
    plans: {
      a: (a) => ({ imports: [], inline: a.modules }),
      b0: split(shared(pages)),
      b: split(shared(pages)),
      c2: split(shared(Math.min(2, pages))),
      c3: split(shared(Math.min(3, pages))),
      ch: split(shared(Math.ceil(pages / 2))),
      d1: (a) => ({ imports: a.modules.map((m) => ({ file: largest.get(m.path), names: [m.fn], whole: true })), inline: [] }),
      d2: (a) => ({ imports: a.modules.filter((m) => count.get(m.key) > 1).map((m) => ({ file: single.get(m.key), names: [m.fn], whole: true })), inline: a.modules.filter((m) => count.get(m.key) === 1) }),
    },
  };
}

// The files of a site under a packaging of its CSS and one of its JS, and
// what each page fetches. "f" is the control as built; "e", for the JS, is
// the control's script — one script for the site is what the control has.
function pack(s, cssPlan, jsPlan, css, js) {
  const names = new Map(); // a file's content → its URL
  const url = (text, ext) => { if (!names.has(text)) names.set(text, `/_rg/page-${hash(text)}.${ext}`); return names.get(text); };
  const sizes = {}; // URL → size
  const file = (text, ext) => { const u = url(text, ext); sizes[u] = sized(text); return u; };
  return s.artifacts.map((a, i) => {
    const c = s.control[i];
    const fetched = [], cssBlobs = [], jsBlobs = []; // a blob: its text, and whether it is a file
    let head = "", body = "";
    if (cssPlan === "f") {
      const asFile = c.entry.css?.delivery === "file";
      if (asFile) { head = `<link rel="stylesheet" href="${c.entry.css.url}">`; sizes[c.entry.css.url] = sized(c.css); fetched.push(c.entry.css.url); } else if (c.css) head = `<style>${c.css}</style>`;
      if (c.css) cssBlobs.push([c.css, asFile]);
    } else {
      for (const part of css.plans[cssPlan](a)) {
        const text = serialize(part.file ?? part.inline);
        cssBlobs.push([text, part.file !== undefined]);
        if (part.file) { const u = file(text, "css"); head += `<link rel="stylesheet" href="${u}">`; fetched.push(u); } else head += `<style>${text}</style>`;
      }
    }
    if (jsPlan === "f" || jsPlan === "e") {
      // As the builder's `auto` delivers a script every page has: a file from 4096 B.
      const asFile = Buffer.byteLength(c.js) >= 4096;
      if (asFile) { const u = jsPlan === "f" && c.entry.js?.delivery === "file" ? c.entry.js.url : url(c.js, "js"); sizes[u] = sized(c.js); body = `<script type="module" src="${u}"></script>`; fetched.push(u); } else if (c.js) body = `<script type="module">${c.js}</script>`;
      if (c.js) jsBlobs.push([c.js, asFile]);
    } else if (a.js) {
      const plan = js.plans[jsPlan](a);
      let inline = "";
      for (const imported of plan.imports) {
        const text = imported.file.map((m) => m.text).join("") + (imported.whole ? `export{${imported.file[0].fn} as default};` : `export{${imported.file.map((m) => m.fn).join(",")}};`);
        const u = file(text, "js");
        inline += imported.whole ? `import ${imported.names[0]} from"${u}";` : `import{${imported.names.join(",")}}from"${u}";`;
        fetched.push(u);
        jsBlobs.push([text, true]);
      }
      inline += plan.inline.map((m) => m.text).join("") + a.mounts;
      jsBlobs.push([inline, false]);
      body = `<script type="module">${inline}</script>`;
    }
    const at = { head: a.html.lastIndexOf("</head>"), body: a.html.lastIndexOf("</body>") };
    const document = a.html.slice(0, at.head) + head + a.html.slice(at.head, at.body) + body + a.html.slice(at.body);
    return { pathname: a.pathname, document, sizes, files: [...new Set(fetched)], statics: a.statics, css: sum(cssBlobs.map(([text]) => sized(text))), js: sum(jsBlobs.map(([text]) => sized(text))), cssBlobs, jsBlobs };
  });
}

// What a packaging costs: every page cold, and the session.
function measure(s, packed) {
  const cold = packed.map((p) => sum([sized(p.document), ...p.files.map((u) => p.sizes[u]), ...p.statics.map((u) => s.statics[u])]));
  const requests = packed.map((p) => 1 + p.files.length + p.statics.length);
  const seen = new Set();
  let total = zero(), sessionRequests = 0;
  const running = [];
  for (const p of packed) {
    total = plus(total, sized(p.document));
    sessionRequests++;
    for (const u of [...p.files, ...p.statics]) if (!seen.has(u)) { seen.add(u); total = plus(total, p.sizes[u] ?? s.statics[u]); sessionRequests++; }
    running.push(total.br);
  }
  // The session by kind, each blob on its own: a file once, what is inlined on every page that has it.
  const kind = (blobs) => { const files = new Set(); let t = zero(); for (const p of packed) for (const [text, asFile] of p[blobs]) if (!asFile) t = plus(t, sized(text)); else if (!files.has(text)) { files.add(text); t = plus(t, sized(text)); } return t; };
  return { cold, requests, session: total, sessionRequests, running, files: new Set(packed.flatMap((p) => p.files)).size, sessionCss: kind("cssBlobs"), sessionJs: kind("jsBlobs") };
}

// The pairs of rules whose order a packaging of the CSS changes on a page,
// and which could change a computed style: both in one layer, of equal
// specificity, declaring a property in common. Whether an element matches
// both is what would have to be proved.
function flips(s, css, plan) {
  let pairs = 0, worst = 0;
  const example = [];
  const meta = new Map();
  const of = (u) => { if (!meta.has(u)) meta.set(u, { layer: layerOf(u), spec: specificity(u.selector).join(","), props: new Set(properties(u)) }); return meta.get(u); };
  for (const a of s.artifacts) {
    const order = css.plans[plan](a).flatMap((part) => (part.file ?? part.inline).filter((u) => u.kind === "style"));
    const mine = new Set(a.units);
    let here = 0;
    for (let i = 0; i < order.length; i++) {
      if (!mine.has(order[i])) continue; // a rule the page does not need matches nothing on it
      for (let j = i + 1; j < order.length; j++) {
        const x = order[i], y = order[j];
        if (!mine.has(y) || x.at < y.at || x.rule === y.rule) continue;
        const mx = of(x), my = of(y);
        if (mx.layer !== my.layer || mx.spec !== my.spec || ![...mx.props].some((p) => my.props.has(p))) continue;
        here++;
        if (example.length < 3 && !example.some((e) => e.includes(x.selector))) example.push(`${tick(y.selector)} was before ${tick(x.selector)} (${a.pathname})`);
      }
    }
    pairs += here;
    worst = Math.max(worst, here);
  }
  return { pairs, worst, example };
}

// ---- the candidates -----------------------------------------------------------------
const CANDIDATES = [
  ["a", "(a) today", "every page's sheet and script inlined"],
  ["b0", "(b′) the common head", "CSS: the rules every page's sheet starts with, as one file — a prefix of each, in source order; the rest inlined. JS: as (b)"],
  ["b", "(b) needed by every page", "the rules and the modules every page needs, as one sheet and one script; the rest inlined"],
  ["c2", "(c) needed by ≥ 2", "the rules and the modules two pages or more need, shared; the rest inlined"],
  ["c3", "(c) needed by ≥ 3", "… three or more"],
  ["ch", "(c) needed by ≥ half", "… half of the pages or more"],
  ["d1", "(d) a file per component, whole", "one file per component and per behaviour module — everything of it that any page needs — linked by the pages that need any of it"],
  ["d2", "(d) a file per component, pruned", "per component and per module, each page's own part: a file where two pages have the same bytes, inlined where not"],
  ["e", "(e) one sheet, one script", "the union of what the pages need, as one sheet; the script is the control's"],
  ["f", "(f) the control", "`--no-specialize`, as built: nothing decided per page"],
];
const GUARANTEE = {
  a: "nothing moves",
  b0: "by construction: the file is a prefix of every page's sheet, the rest follows it — the same rules in the same order",
  b: "**needs a proof**: the shared file is linked first, so a rule left inline now follows every shared rule it preceded",
  c2: "**needs a proof**, as (b); and a page gets rules it does not need — those match nothing on it, and a custom property it does not need is read by nothing on it: that is the pruner's soundness",
  c3: "**needs a proof**, as (b), with the pruner's soundness for what a page does not need",
  ch: "**needs a proof**, as (b), with the pruner's soundness for what a page does not need",
  d1: "by construction, with the pruner's soundness: the files are linked in source order, so a page has its own rules in their order, and between them rules that match nothing on it",
  d2: "by construction: files and inlined parts stand in the head in source order — the page's own sheet, cut at file boundaries",
  e: "by construction, with the pruner's soundness: source order, and the rules a page does not need match nothing on it",
  f: "nothing is pruned",
};

function study(s, t5pages) {
  const css = cssPlans(s), js = jsPlans(s);
  const controlCss = sized(s.control[0].css), controlJs = sized(s.control[0].js);
  const rows = (cssOf, jsOf) => CANDIDATES.map(([id, label]) => {
    const packed = pack(s, cssOf(id), jsOf(id), css, js);
    const m = measure(s, packed);
    // T5 as written (plan.md): against the control, in brotli bytes, each blob on its own.
    const cssSmaller = packed.map((p) => (1 - p.css.br / controlCss.br) * 100);
    const jsSmaller = packed.map((p, i) => (s.artifacts[i].js ? (1 - p.js.br / controlJs.br) * 100 : null)).filter((x) => x !== null);
    const beyond = (kind) => packed.map((p, i) => p[kind].raw - Buffer.byteLength(s.artifacts[i][kind]));
    return { id, label, packed, ...m, cssSmaller, jsSmaller, t5css: cssSmaller.filter((x) => x >= 20).length >= t5pages, t5js: jsSmaller.every((x) => x >= 30), t8: Math.max(...m.requests) <= 3, beyondCss: beyond("css"), beyondJs: beyond("js") };
  });
  const both = rows((id) => id, (id) => id);
  const cssOnly = rows((id) => id, (id) => (id === "f" ? "f" : "a"));
  const jsOnly = rows((id) => (id === "f" ? "f" : "a"), (id) => id);
  // (a) is the default build as built: its documents are the build's own, to the byte.
  both[0].packed.forEach((p, i) => must(p.document === s.artifacts[i].document, `${s.name} ${p.pathname}: (a) is not the document the default build wrote`));
  both.at(-1).packed.forEach((p, i) => must(p.document === s.control[i].document, `${s.name} ${p.pathname}: (f) is not the document the control wrote`));
  // The scripts that (b) and (c) would share, as written: under 4096 B the builder's `auto` inlines a blob.
  const sharedScripts = ["b", "c2", "c3", "ch"].flatMap((id) => both.find((row) => row.id === id).packed.flatMap((p) => p.jsBlobs.filter(([, asFile]) => asFile).map(([text]) => Buffer.byteLength(text))));
  return { css, js, both, cssOnly, jsOnly, sharedScripts, flips: Object.fromEntries(["b", "c2", "c3", "ch"].map((id) => [id, flips(s, css, id)])) };
}

// ---- the report -----------------------------------------------------------------------
const sites = [
  { s: read("site", site, PAGES), title: "The docs site", what: "`site/`: four pages, three components in one layout", t5pages: 2, t5text: "at least two pages" },
  { s: read("catalog", catalogSite, CATALOG_PAGES), title: "The catalog", what: "`bench/catalog-site`: ten pages on twenty components — a fixture", t5pages: Math.ceil(CATALOG_PAGES.length / 2), t5text: "at least half of the pages" },
];
for (const it of sites) it.r = study(it.s, it.t5pages);

const range = (list, f = n) => (Math.min(...list) === Math.max(...list) ? f(Math.min(...list)) : `${f(Math.min(...list))}–${f(Math.max(...list))}`);
const ahead = (row, control) => { const at = row.running.findIndex((x, i) => control.running[i] < x); return at < 0 ? "never" : `page ${at + 1}`; };
const visit = (row, control) => (row.session.br <= control.session.br ? `**holds**: ${pct((1 - row.session.br / control.session.br) * 100)} less` : `fails: the control transfers ${pct((1 - control.session.br / row.session.br) * 100)} less`);
const yes = (ok) => (ok ? "pass" : "**fail**");
const t8 = (row) => `${yes(row.t8)}: ${range(row.requests)}${row.t8 ? "" : `, ${row.requests.filter((x) => x > 3).length} of ${row.requests.length} pages over`}`;

let md = `# The factoring study: packagings of one analysis\n\n`;
md += `\`node bench/factor.mjs\`. **A study, not a build**: for the owner's decision on the factoring policy (specs/phase02/builder.md, *Packaging*, OPEN; decisions.md, K). Nothing here ships, and no policy is implemented.\n\n`;
md += `Analysis is the same in every row but the control's: each page's sheet is the rules that can match on that page, its script the behaviours it mounted — the default build's, as built. A row is a **packaging** of that: which of it is inlined in the page, which is a file of the page's own, which is factored into files that pages share. The bytes are of the files each packaging would write, each compressed on its own (raw / gzip -9 / brotli -q 11, as \`measure.mjs\`). A page loaded **cold** fetches its document, the files it links and the static files it shows (the favicon; a picture on the catalog's \`/\`); the **session** is the pages in the bench's order with a warm cache, each URL once.\n\n`;
md += `| | |\n| --- | --- |\n`;
for (const [, label, what] of CANDIDATES) md += `| ${label} | ${what} |\n`;
md += `\nThe thresholds are plan.md's (RGP2-050), **as written**: **T5** — against the control, in brotli bytes, each blob on its own: per-page CSS ≥ 20% smaller on at least two pages (of a catalog: on at least half of its pages), JS ≥ 30% smaller on every page that ships one — read on what a page *fetches* cold under the packaging: "what a page transfers". **T8** — ≤ 3 requests per page, cold. **The visit** — the packaging transfers no more than the control over the session, in brotli — is **no threshold of plan.md today**: the owner named it as a target (decisions.md, K, rule 4), and it is given here so that a policy can be judged by it.\n\nT5 read on what a page *needs* is row (a)'s in every row: no packaging changes the analysis. "Fetched beyond the page's own" is the raw bytes of CSS, or of JS, that a cold page fetches over its own sheet, or script, as analysis left it: for a packaging that hands a page no rule it does not need — (b′), (b), (d, pruned) — that is what the split itself costs (a block reopened, a selector list cut in two, an \`import\`); for the others, mostly rules and modules of other pages.\n`;

for (const { s, title, what, r, t5text } of sites) {
  const control = r.both.at(-1);
  md += `\n## ${title}\n\n${what}. ${s.artifacts.length} artifacts; the session: ${s.pages.map(tick).join(", ")}.\n\n`;
  // What the analysis says, before any packaging.
  const count = r.css.count, pages = s.artifacts.length;
  const weight = (u) => Buffer.byteLength(u.selector) + (u.body === null ? 1 : Buffer.byteLength(u.body) + 2);
  md += `**What the artifacts need.** The control's sheet is ${n(s.source.length)} units (${tri(sized(s.control[0].css))} B); ${n(r.css.all.length)} of them are needed by some page${s.extra.length ? ` (and ${s.extra.length} units of a page's sheet are not in the control's as written)` : ""}. By how many pages need a unit:\n\n`;
  md += `| Needed by | Units | ≈ B, raw | |\n| --- | ---: | ---: | --- |\n`;
  for (let k = pages; k >= 1; k--) {
    const units = r.css.all.filter((u) => count.get(u) === k);
    if (units.length) md += `| ${k === pages ? `every page (${k})` : `${k} page${k === 1 ? "" : "s"}`} | ${units.length} | ${n(units.reduce((a, u) => a + weight(u), 0))} | ${[...new Set(units.map(r.css.group))].map(tick).join(", ")} |\n`;
  }
  const never = s.source.filter((u) => !count.has(u));
  md += `| no page | ${never.length} | ${n(never.reduce((a, u) => a + weight(u), 0))} | ${[...new Set(never.map(r.css.group))].map(tick).join(", ")} |\n`;
  md += `\nEvery page's sheet is written back from its units to the byte (${pages} of ${pages}), and the control's. The components, by the top-level block of the source a rule stands in: ${r.css.groups.map(tick).join(", ")}.\n\n`;
  md += `The scripts' modules — a module built under other flags is another:\n\n| Module | B in a script | Pages |\n| --- | ---: | --- |\n`;
  for (const m of r.js.first.values()) md += `| ${tick(m.path.split("/").at(-1))} | ${n(m.bytes)} | ${r.js.count.get(m.key) === pages ? `every page (${pages})` : s.artifacts.filter((a) => a.modules.some((x) => x.key === m.key)).map((a) => tick(a.pathname)).join(", ")} |\n`;
  md += `\nThe control's script is ${tri(sized(s.control[0].js))} B${Buffer.byteLength(s.control[0].js) >= 4096 ? ", a file" : ", inlined in every page (under 4096 B)"}; its sheet is a file.\n`;

  const table = (rows, heading, note) => {
    md += `\n**${heading}**${note ? ` ${note}` : ""}\n\n`;
    md += `| Packaging | Files | Cold page, brotli: min / mean / max | … mean, raw / gzip / brotli | Requests, cold | Session, raw / gzip / brotli | Session requests | The control is ahead from |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n`;
    for (const row of rows) md += `| ${row.label} | ${row.files} | ${n(Math.min(...row.cold.map((c) => c.br)))} / ${n(mean(row.cold.map((c) => c.br)))} / ${n(Math.max(...row.cold.map((c) => c.br)))} | ${n(mean(row.cold.map((c) => c.raw)))} / ${n(mean(row.cold.map((c) => c.gz)))} / ${n(mean(row.cold.map((c) => c.br)))} | ${range(row.requests)} | ${tri(row.session)} | ${row.sessionRequests} | ${row.id === "f" ? "—" : ahead(row, control)} |\n`;
    md += `\n| Packaging | T5, CSS: smaller than the control's, brotli (${t5text} at 20%) | | T5, JS (every page at 30%) | | T8 (≤ 3) | The visit | Fetched beyond the page's own sheet, raw: mean / max | … beyond its own script |\n| --- | --- | --- | --- | --- | --- | --- | ---: | ---: |\n`;
    for (const row of rows) {
      if (row.id === "f") { md += `| ${row.label} | — | | — | | ${t8(row)} | — | ${n(mean(row.beyondCss))} / ${n(Math.max(...row.beyondCss))} | ${n(mean(row.beyondJs))} / ${n(Math.max(...row.beyondJs))} |\n`; continue; }
      md += `| ${row.label} | ${range(row.cssSmaller, pct)}; ${row.cssSmaller.filter((x) => x >= 20).length} of ${row.cssSmaller.length} pages | ${yes(row.t5css)} | ${range(row.jsSmaller, pct)} | ${yes(row.t5js)} | ${t8(row)} | ${visit(row, control)} | ${n(mean(row.beyondCss))} / ${n(Math.max(...row.beyondCss))} | ${n(mean(row.beyondJs))} / ${n(Math.max(...row.beyondJs))} |\n`;
    }
  };
  table(r.both, "CSS and JS together.");
  table(r.cssOnly, "CSS alone", "— the scripts as today: inlined in each page.");
  table(r.jsOnly, "JS alone", "— the sheets as today: inlined in each page.");

  md += `\n**The session by kind** — brotli, each blob on its own: a file once, what is inlined on every page that has it. (CSS and JS together; the documents' HTML is the same in every row.)\n\n| Packaging | CSS over the session | JS over the session |\n| --- | ---: | ---: |\n`;
  for (const row of r.both) md += `| ${row.label} | ${n(row.sessionCss.br)} | ${n(row.sessionJs.br)} |\n`;
  const least = r.both.filter((row) => row.id !== "f").reduce((m, row) => (row.sessionCss.br < m.sessionCss.br ? row : m));
  const today = r.both[0];
  md += `\nThe least CSS any candidate transfers over the session is ${least.label}'s, ${n(least.sessionCss.br)} B; the control's sheet is ${n(control.sessionCss.br)} B. The pages' own scripts are ${n(today.sessionJs.br)} B over the session and the control's ${n(control.sessionJs.br)} B — ${Buffer.byteLength(s.control[0].js) >= 4096 ? "a file, fetched once" : `under 4096 B, so inlined in each of the ${pages} pages`}.\n`;
  md += `\nThe scripts (b) and (c) would share are ${r.sharedScripts.length ? `${range(r.sharedScripts)} B as written${Math.max(...r.sharedScripts) < 4096 ? ": under the 4096 B from which the builder's `auto` makes a shared blob a file. With that rule kept they stay inlined, and \"CSS alone\" is what (b) and (c) are" : ""}` : "none"}.\n`;
  md += `\n**The cascade.** A rule that moves into a shared file changes its place among the rules left behind. What would guarantee the same computed styles:\n\n| Packaging | The same computed styles | Pairs of rules whose order changes and could matter |\n| --- | --- | --- |\n`;
  for (const [id, label] of CANDIDATES) {
    const f = r.flips[id];
    md += `| ${label} | ${GUARANTEE[id]} | ${f ? `${n(f.pairs)} over the ${pages} pages, ${n(f.worst)} on the worst${f.example.length ? ` — ${f.example.join("; ")}` : ""}` : "none"} |\n`;
  }
  md += `\nA pair is counted when the packaging puts a rule after one it preceded, both are rules the page needs, in the same layer, of equal specificity, and declare a property in common: then order decides between them **if** an element matches both. Whether one does is what the pruner would have to say — it matches every selector against the page, and reports only kept or dropped today. Zero pairs is safe as counted; any other number is that many questions to answer per build, or a rule left where it was.\n`;

  // Which packagings meet all three.
  const good = r.both.filter((row) => row.id !== "f" && row.t5css && row.t5js && row.t8 && row.session.br <= control.session.br);
  const goodCss = r.cssOnly.filter((row) => row.id !== "f" && row.t5css && row.t5js && row.t8 && row.session.br <= control.session.br);
  md += `\n**T5, T8 and the visit together.** With CSS and JS factored alike: ${good.length ? good.map((row) => row.label).join("; ") : "**no candidate**"}. With the CSS factored and the scripts inlined as today: ${goodCss.length ? goodCss.map((row) => row.label).join("; ") : "**no candidate**"}.\n`;
  const holds = (rows) => rows.filter((row) => row.id !== "f" && row.session.br <= control.session.br).map((row) => row.label).join("; ") || "none";
  md += `The visit alone — no more than the control over the session: together, ${holds(r.both)}; CSS alone, ${holds(r.cssOnly)}. Of those, with no rule or module a page does not need: together, ${holds(r.both.filter((row) => ["a", "b0", "b", "d2"].includes(row.id)))}; CSS alone, ${holds(r.cssOnly.filter((row) => ["a", "b0", "b", "d2"].includes(row.id)))}.\n`;
  const best = [...r.both, ...r.cssOnly].filter((row) => row.id !== "f").reduce((m, row) => (row.session.br < m.session.br ? row : m));
  md += `The least any candidate transfers over the session is ${n(best.session.br)} B (${best.label}${r.cssOnly.includes(best) ? ", CSS alone" : ""}); the control: ${n(control.session.br)} B.\n`;
}

md += `\n## What it says\n\n`;
md += `- **Over a visit that reaches every page, one sheet of the union is the floor, and the control's sheet is that floor but for what no page uses.** Every packaging of exact per-page analysis transfers each needed rule at least once over such a session — the union — and in more, smaller pieces, each compressed on its own; (e) is the union as one file, and the control's sheet is (e) and the rules no page needs (*The session by kind*). So where the site uses nearly all of the design system it imports — both sites here — awareness cannot win a long visit by its CSS: only where the design system is larger than what the site uses, or where the visit is short. The scripts go either way, by how the control's is delivered: inlined in every page, the pages' own win the session; as a file, the control's does.\n`;
md += `- **Cold and the visit pull apart.** What makes a cold page light — its own rules only, inlined, one request — leaves nothing in the cache for the next page; what makes a visit cheap — a shared file — costs the cold page a request and, where the file holds more than the page needs, bytes. Of the packagings that hand a page nothing it does not need, (b) shares the most; (c) and (d, whole) buy the visit with rules of other pages; (e) is the control without its unused rules.\n`;
md += `- **A request is counted, a round trip is not.** T8 counts requests; the bytes here count no header and no round trip of the first view (builder.md, *Packaging*: the 4096 B rule is for that). *JS alone* shows the other side: a script of 250 B as a file is a request for 250 B.\n`;
md += `- **Order.** (b′), (d) and (e) keep the cascade by construction. (b) and (c) — the ones that share the most while staying near exact — change the order of rules, and need the pruner to say, per page, that no element matches both rules of a pair: it does not today.\n`;
md += `\nNot measured: time; a real network; a browser (the cold loads are computed from the files); scripts that run — a shared script here is the pages' own modules and an \`export\`, not a build.\n`;
if (problems.length) md += `\n## Problems\n\n${problems.map((p) => `- ${p}`).join("\n")}\n`;

mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
process.stdout.write(md);
if (problems.length) { console.error(`\n${problems.length} problem(s): the study is not to be trusted`); process.exit(1); }
