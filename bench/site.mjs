#!/usr/bin/env node
// site.mjs — the bet, in bytes (specs/phase02/plan.md, RGP2-050).
//
//   node bench/site.mjs [--binary <reactogenic>] [--md bench/results/site.md] [--json out.json]
//                       [--keep <dir>] [--runs 3] [--static]
//
// Builds the docs site (site/) with the `reactogenic` binary — the one given,
// $REACTOGENIC_BINARY, or built here from go/ — in the three modes the plan
// names and one more:
//
//   default   reactogenic build                   what ships
//   always    reactogenic build --inline always   every blob inlined: one request per page
//   control   reactogenic build --no-specialize   awareness off: one sheet and one script for the site
//   never     reactogenic build --inline never    the default's blobs as files: what a page is,
//                                                 apart from how it is delivered
//
// and measures each with measure.mjs's method (its header has it): every page
// loaded cold in headless Chrome from a local server, the files it fetched
// sized raw / gzip -9 / brotli -q 11, the requests, the JS to parse, and a
// warm 4-page session. Then, from the builds' own files and `_rg/report.json`:
// what each page is (HTML as rendered, its CSS, its script), the control's
// script split into its behaviours and its pathname table, the default
// against the control page by page, and the thresholds this script can
// decide: T1, T2, T3, T5 (in brotli bytes), T6, T8. T4 is delta.mjs's, T7
// verify.mjs's.
//
// The output is one Markdown file (and stdout); it names no path and no
// version of this machine, so a second run writes the same bytes. Exit
// status: 1 when a threshold that refutes the bet fails, or when a number
// here disagrees with the builder's report.
//
// No dependency (node >= 22). Without Chrome ($CHROME, or the usual places)
// measure.mjs follows the pages' own <link> and <script> instead: the same
// numbers on this site, which loads nothing lazily.
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, extname, join, relative, resolve } from "node:path";
import { BUILDS, PAGES, binary, controlEntry, cssItems, cssKey, minus, mustBuild, n, options, page, percent, repo, scriptChecks, site, size, smaller, tri } from "./lib.mjs";

const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/site.md"));
const bin = binary(opts.binary);
const work = opts.keep ? resolve(opts.keep) : mkdtempSync(join(tmpdir(), "reactogenic-bench-site-"));
mkdirSync(work, { recursive: true });

const problems = []; // a number that is not what the report says: the measurement is wrong
const must = (ok, text) => { if (!ok) problems.push(text); return ok; };

// ---- the builds --------------------------------------------------------------
const builds = {};
const gzipApart = []; // the report's gzip minus gzip -9, per piece
for (const [name, flags] of Object.entries(BUILDS)) {
  const out = join(work, name);
  const report = mustBuild(bin, site, out, flags);
  const pages = {};
  for (const pathname of PAGES) {
    const entry = report.pages.find((p) => p.pathname === pathname);
    if (!entry) throw new Error(`${name}: the build has no page ${pathname}`);
    const parts = page(out, entry);
    // What was cut out of the document is what the report counted.
    must(Buffer.byteLength(parts.html) === entry.html.raw, `${name} ${pathname}: the HTML without packaging is ${Buffer.byteLength(parts.html)} B, the report says ${entry.html.raw}`);
    must(Buffer.byteLength(parts.css) === (entry.css?.raw ?? 0), `${name} ${pathname}: the CSS is ${Buffer.byteLength(parts.css)} B, the report says ${entry.css?.raw}`);
    must(Buffer.byteLength(parts.js) === (entry.js?.raw ?? 0), `${name} ${pathname}: the script is ${Buffer.byteLength(parts.js)} B, the report says ${entry.js?.raw}`);
    must(Buffer.byteLength(parts.document) === entry.document.raw, `${name} ${pathname}: the document is ${Buffer.byteLength(parts.document)} B, the report says ${entry.document.raw}`);
    const sizes = { document: size(parts.document), html: size(parts.html), css: size(parts.css), js: size(parts.js) };
    // The report's gzip is Go's DEFLATE, the one here zlib's (builder.md, *The report*).
    for (const piece of Object.keys(sizes)) if (entry[piece]) gzipApart.push(entry[piece].gzip - sizes[piece].gz);
    pages[pathname] = { entry, ...parts, sizes };
  }
  must(report.pages.length === PAGES.length, `${name}: ${report.pages.length} pages, ${PAGES.length} expected`);
  builds[name] = { flags, out, report, pages };
}
// A page is the same page in every build: only its delivery differs.
for (const pathname of PAGES) {
  const d = builds.default.pages[pathname];
  for (const other of ["always", "never"]) {
    const o = builds[other].pages[pathname];
    must(o.html === d.html && o.css === d.css && o.js === d.js, `${pathname}: \`${BUILDS[other].join(" ")}\` changes what the page is, not only how it is delivered`);
  }
  must(builds.control.pages[pathname].html === d.html, `${pathname}: the control's HTML is not the default build's`);
}

// ---- measure.mjs on each -----------------------------------------------------
const jsonTmp = join(work, "measure.json");
const measured = spawnSync(process.execPath, [
  join(repo, "bench/measure.mjs"),
  ...Object.keys(BUILDS).map((name) => `${name}=${builds[name].out}`),
  "--pages", PAGES.join(","), "--json", jsonTmp, "--runs", String(opts.runs ?? 3), ...(opts.static ? ["--static"] : []),
], { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"], maxBuffer: 1 << 26 });
if (measured.status !== 0) throw new Error(`measure.mjs failed with status ${measured.status}`);
const measure = Object.fromEntries(JSON.parse(readFileSync(jsonTmp, "utf8")).map((r) => [r.name, r]));
// measure.mjs's own tables, by build; its `dir` line is this machine's.
const sections = Object.fromEntries(measured.stdout.split(/^### /m).slice(1).map((s) => {
  const name = s.slice(0, s.indexOf("\n")).trim();
  return [name, s.slice(s.indexOf("\n")).replace(/^`[^`]*` — loaded with /m, "Loaded with ").trim()];
}));
const mode = measure.default.mode;
for (const name of Object.keys(BUILDS)) for (const p of measure[name].pages) must(p.missing.length === 0, `${name} ${p.page}: requests that 404ed: ${p.missing.join(", ")}`);

// ---- the control's script: behaviours and table --------------------------------
const controlJs = builds.control.pages["/"].js;
const split = controlEntry(controlJs);
must(split !== null, "the control's script does not end with the pathname table: its shape changed");
const control = {
  css: builds.control.pages["/"].sizes.css,
  js: builds.control.pages["/"].sizes.js,
  behaviours: split ? size(split.behaviours) : null,
  table: split ? size(split.entry) : null,
};

// ---- T1: every byte of a page's script ------------------------------------------
// lib.mjs reads the script: its statements, the report's rows, the document.
const t1 = PAGES.map((pathname) => scriptChecks(pathname, builds.default.pages[pathname]));

// ---- T6: the site's source --------------------------------------------------------
function walk(dir, out = []) {
  for (const name of readdirSync(dir).sort()) {
    if (name === "node_modules" || name === "dist") continue;
    const path = join(dir, name);
    if (statSync(path).isDirectory()) walk(path, out); else out.push(path);
  }
  return out;
}
const sourceFiles = walk(site).map((f) => relative(site, f));
// test/ checks the built site; it is not built into it.
const shipped = sourceFiles.filter((f) => !f.startsWith("test/"));
// The site shows code: a sample is a template-literal constant of the module
// that shows it (site/README.md), and a tag in running text is a string in
// braces (`{"<dialog>"}`). Both are text of the page, not code of the site,
// and a grep must not read them: their content is blanked, the lines kept.
const blank = (text) => text.replace(/[^\n]/g, " ");
const read = (f) => {
  const text = readFileSync(join(site, f), "utf8");
  if (!f.endsWith(".rtsx")) return text;
  return text.replace(/`(?:\\[\s\S]|[^`\\])*`/g, (s) => "`" + blank(s.slice(1, -1)) + "`").replace(/\{(["'])(?:\\.|(?!\1)[^\\\n])*\1\}/g, (s) => "{" + blank(s.slice(1, -1)) + "}");
};
const grep = (re, files = shipped.filter((f) => /\.(rtsx|tsx|ts|css|svg|html)$/.test(f))) =>
  files.flatMap((f) => read(f).split("\n").flatMap((line, i) => (re.test(line) ? [`${f}:${i + 1}`] : [])));
const SOURCE = new Set([".rtsx", ".css", ".json", ".md", ".svg"]);
const t6 = {
  "no file of JS or TS: every source file is .rtsx, .css, .json, .md or .svg": shipped.filter((f) => !SOURCE.has(extname(f))),
  "no <script>": grep(/<script\b/i),
  "no <style>, no <link rel=stylesheet>, no style attribute": grep(/<style\b|rel=["']?stylesheet|\sstyle=/i),
  "no event handler": grep(/\son[A-Z][A-Za-z]*=|\son[a-z]+=["'{]/, shipped.filter((f) => /\.(rtsx|svg)$/.test(f))),
  "no `mount(`, no import of a behaviour": grep(/\bmount\s*\(|behaviors\//, shipped.filter((f) => f.endsWith(".rtsx"))),
  "no HTML written as a string": grep(/dangerouslySetInnerHTML/),
  "one stylesheet imported, once, by the layout: no list per page": grep(/import\s+["'][^"']+\.css["']/).filter((at) => !at.startsWith("layout.rtsx:")),
};
const cssImports = grep(/import\s+["'][^"']+\.css["']/);
const t6ok = Object.values(t6).every((hits) => hits.length === 0) && cssImports.length === 1;

// ---- T3: the React baseline of the research ----------------------------------------
// bench/baselines/2026-10-04.md, `c1-astro-radix`: Astro + React islands +
// Radix. ANOTHER site of the same shape (four docs pages: a side menu, a
// dialog, a menu), built during the research and not in the repository.
const baselineText = readFileSync(join(repo, "bench/baselines/2026-10-04.md"), "utf8");
const baselineRows = (baselineText.split(/^### /m).find((s) => s.startsWith("c1-astro-radix")) ?? "")
  .split("\n").filter((l) => /^\| `\//.test(l)).map((l) => {
    const cells = l.split("|").map((c) => c.trim());
    const three = (c) => c.split("/").map((x) => Number(x.replaceAll(",", "").trim()));
    return { page: cells[1].replaceAll("`", ""), requests: Number(cells[2]), js: three(cells[5]), parse: Number(cells[9].replaceAll(",", "")) };
  });
must(baselineRows.length === 4, "bench/baselines/2026-10-04.md has no four rows of c1-astro-radix");

// ---- the tables ------------------------------------------------------------------
const D = builds.default.pages;
const heaviest = PAGES.reduce((a, b) => (D[b].sizes.js.raw > D[a].sizes.js.raw ? b : a));
const mean = (xs) => xs.reduce((a, b) => a + b, 0) / xs.length;
const delivery = (e) => (e ? (e.delivery === "file" ? `file, ${e.pages} page${e.pages === 1 ? "" : "s"}` : "inline") : "—");

let md = `# The docs site, measured\n\n`;
md += `Written by \`node bench/site.mjs\` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). `;
md += `Four builds of \`site/\`, each ${mode === "chrome" ? "loaded cold in headless Chrome" : "crawled statically (no browser)"} by \`bench/measure.mjs\`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.\n`;

for (const name of Object.keys(BUILDS)) {
  const b = builds[name];
  md += `\n## \`${name}\` — \`reactogenic build${b.flags.length ? " " + b.flags.join(" ") : ""}\`\n\n`;
  md += `As delivered. ${sections[name]}\n`;
  md += `\nWhat each page is, whatever the delivery — the HTML as rendered, without what packaging writes into it:\n\n`;
  md += `| Page | HTML as rendered | CSS | JS | CSS delivered | JS delivered |\n| --- | ---: | ---: | ---: | --- | --- |\n`;
  for (const pathname of PAGES) {
    const p = b.pages[pathname];
    md += `| \`${pathname}\` | ${tri(p.sizes.html)} | ${tri(p.sizes.css)} | ${tri(p.sizes.js)} | ${delivery(p.entry.css)} | ${delivery(p.entry.js)} |\n`;
  }
  if (name === "control" && split) {
    md += `\nThe control's script, split: its behaviours — every module any page mounts, every flag on — are ${tri(control.behaviours)}; `;
    md += `its own cost, the list of modules, the table from pathname to mounts and the loop that reads it, is ${tri(control.table)} `;
    md += `(compressed apart; the script as a whole is ${tri(control.js)}):\n\n\`\`\`js\n${split.entry}\n\`\`\`\n`;
  }
}

md += `\n## Summary\n\n${sections.Summary}\n`;

// The default against the control.
md += `\n## The default build against the control\n\n`;
md += `What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control.\n\n`;
md += `| Page | CSS, default | CSS, control | smaller: raw / gzip / brotli |\n| --- | ---: | ---: | ---: |\n`;
const cssRows = PAGES.map((pathname) => ({ pathname, d: D[pathname].sizes.css, c: control.css }));
for (const r of cssRows) md += `| \`${r.pathname}\` | ${tri(r.d)} | ${tri(r.c)} | ${smaller(r.d.raw, r.c.raw)} / ${smaller(r.d.gz, r.c.gz)} / ${smaller(r.d.br, r.c.br)} |\n`;
md += `\n| Page | JS, default | JS, control | smaller: raw / gzip / brotli | default without its entry | control without its table | smaller, raw |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n`;
const jsRows = PAGES.map((pathname, i) => ({ pathname, d: D[pathname].sizes.js, c: control.js, own: D[pathname].sizes.js.raw - t1[i].entryBytes }));
for (const r of jsRows)
  md += `| \`${r.pathname}\` | ${tri(r.d)} | ${tri(r.c)} | ${smaller(r.d.raw, r.c.raw)} / ${smaller(r.d.gz, r.c.gz)} / ${smaller(r.d.br, r.c.br)} | ${n(r.own)} | ${control.behaviours ? n(control.behaviours.raw) : "—"} | ${control.behaviours ? smaller(r.own, control.behaviours.raw) : "—"} |\n`;

// What the four sheets have in common: a sheet as its selectors and at-rules
// (lib.mjs), the control's as the whole.
const sheet = Object.fromEntries(PAGES.map((p) => [p, cssItems(D[p].css)]));
const whole = cssItems(builds.control.pages["/"].css);
let common = sheet[PAGES[0]];
for (const p of PAGES.slice(1)) common = common.filter(((left) => (it) => { const i = left.findIndex((x) => cssKey(x) === cssKey(it)); if (i >= 0) left.splice(i, 1); return i >= 0; })([...sheet[p]]));
const weight = (list) => list.reduce((a, it) => a + Buffer.byteLength(it.selector) + (it.body === null ? 1 : Buffer.byteLength(it.body) + 2), 0);
const nowhere = whole.filter((it) => PAGES.every((p) => !sheet[p].some((x) => cssKey(x) === cssKey(it))));
md += `\nHow much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.\n\n`;
md += `| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's ${whole.length} |\n| --- | ---: | ---: | ---: | ---: | ---: |\n`;
for (const p of PAGES) {
  const own = minus(sheet[p], common);
  md += `| \`${p}\` | ${sheet[p].length} | ${common.length} | ${own.length} | ${n(weight(own))} | ${minus(whole, sheet[p]).length} |\n`;
}
md += `\n${common.length} of the control's ${whole.length} are on all four pages (≈ ${n(weight(common))} B of selectors and declarations): the layout's — the side menu, the links menu, the button — and the site's own sheet. `;
md += `${nowhere.length} are on no page: ${nowhere.map((it) => "`" + it.selector + "`").join(", ") || "none"}.\n`;

md += `\nWhat a visitor's browser fetches, as each build delivers it (from the tables above):\n\n`;
md += `| Build | Requests per page, cold | Page, cold: mean total | Session of ${PAGES.length} pages, warm cache: requests | … total |\n| --- | ---: | ---: | ---: | ---: |\n`;
for (const name of Object.keys(BUILDS)) {
  const m = measure[name];
  const total = { raw: Math.round(mean(m.pages.map((p) => p.total.raw))), gz: Math.round(mean(m.pages.map((p) => p.total.gz))), br: Math.round(mean(m.pages.map((p) => p.total.br))) };
  md += `| \`${name}\` | ${[...new Set(m.pages.map((p) => p.requests))].join(", ")} | ${tri(total)} | ${m.session.requests} | ${tri(m.session.total)} |\n`;
}
const sessD = measure.default.session.total, sessC = measure.control.session.total;
// Which of the two transfers less over the session, and how each delivers
// its blobs (builder.md, *Packaging*: `--inline auto`).
const lighter = sessC.br <= sessD.br ? ["control", sessC, "default build", sessD] : ["default build", sessD, "control", sessC];
const how = (e) => (e.delivery === "file" ? `a file, fetched once` : `inlined in each of the ${e.pages} pages`);
const controlPage = builds.control.pages["/"].entry;
md += `\nOver the session the ${lighter[0]} is **${smaller(lighter[1].raw, lighter[3].raw)} / ${smaller(lighter[1].gz, lighter[3].gz)} / ${smaller(lighter[1].br, lighter[3].br)} smaller** than the ${lighter[2]} (raw / gzip / brotli). `;
md += `The control's one sheet (${n(control.css.raw)} B) is ${how(controlPage.css)}, its one script (${n(control.js.raw)} B) ${how(controlPage.js)}; `;
md += `the default build's four sheets — ${PAGES.map((p) => n(D[p].sizes.css.raw)).join(", ")} B, no two the same — are each inlined in their page.\n`;

// T1.
md += `\n## What each page's script is (T1)\n\nThe default build. The rows are \`_rg/report.json\`'s (\`modules\`: from esbuild's metafile).\n\n`;
md += `| Page | Script, raw | Rows of the report | Sum | Mounts |\n| --- | ---: | --- | ---: | --- |\n`;
for (const t of t1) {
  const rows = t.rows.map((r) => `${n(r.bytes)} \`${r.path === "<entry>" ? "<entry>" : r.path.split("/").at(-1)}\``).join(" + ");
  const mounts = t.mounts.map((m) => `\`${m.module.split("/").at(-1)}\`${m.id ? ` on \`#${m.id}\`` : ""}${m.flags ? " " + Object.entries(m.flags).map(([k, v]) => `\`${k}=${v}\``).join(" ") : ""}${m.data ? ` with \`${JSON.stringify(m.data)}\`` : ""}`).join(", ");
  md += `| \`${t.pathname}\` | ${n(D[t.pathname].sizes.js.raw)} | ${rows} | ${n(t.sum)} | ${mounts} |\n`;
}
md += `\nChecked on every page, by reading the script and the document:\n\n`;
for (const name of Object.keys(t1[0].checks)) md += `- ${name}: ${t1.every((t) => t.checks[name]) ? "yes" : "**NO** — " + t1.filter((t) => !t.checks[name]).map((t) => "`" + t.pathname + "`").join(", ")}\n`;
const distinct = [...new Map(PAGES.map((p) => [D[p].js, p])).entries()];
md += `\nThe ${distinct.length} distinct scripts of the site, whole:\n`;
for (const [js] of distinct) md += `\n${PAGES.filter((p) => D[p].js === js).map((p) => "`" + p + "`").join(", ")} — ${n(Buffer.byteLength(js))} B:\n\n\`\`\`js\n${js}\n\`\`\`\n`;

// T6.
md += `\n## The site's source (T6)\n\n${shipped.length} files under \`site/\` (without \`test/\`, which checks the built site and is not built into it; without \`node_modules/\` and \`dist/\`): ${shipped.map((f) => "`" + f + "`").join(", ")}.\n\n`;
for (const [name, hits] of Object.entries(t6)) md += `- ${name}: ${hits.length === 0 ? "yes" : "**NO** — " + hits.join(", ")}\n`;
md += `- the stylesheet imports of the whole site: ${cssImports.map((at) => "`" + at + "`").join(", ") || "none"}\n`;

// Thresholds.
// T5 is decided in brotli bytes — what a page transfers (plan.md, RGP2-050;
// the owner's ruling, decisions.md, L); raw and gzip are reported beside it.
const UNITS = { br: "brotli", gz: "gzip", raw: "raw" };
const cssAt = (unit) => cssRows.filter((r) => percent(r.d[unit], r.c[unit]) >= 20);
const jsUnder = (unit) => jsRows.filter((r) => r.d.raw > 0 && percent(r.d[unit], r.c[unit]) < 30);
const t5css = (unit) => `${cssRows.map((r) => smaller(r.d[unit], r.c[unit])).join(", ")} ${UNITS[unit]} — ${cssAt(unit).length} page${cssAt(unit).length === 1 ? "" : "s"} at 20% or more`;
const t5js = (unit) => `${jsRows.map((r) => smaller(r.d[unit], r.c[unit])).join(", ")} ${UNITS[unit]}${jsUnder(unit).length ? ` — under 30% on ${jsUnder(unit).map((r) => "`" + r.pathname + "`").join(", ")}` : ""}`;
const heavy = D[heaviest].sizes.js;
const ratios = PAGES.map((p, i) => baselineRows[i].parse / D[p].sizes.js.raw);
const worst = Math.min(...baselineRows.map((r) => r.parse)) / heavy.raw;
const worstBr = Math.min(...baselineRows.map((r) => r.js[2])) / heavy.br;
const requests = measure.default.pages.map((p) => p.requests);
const thresholds = [
  { id: "T1", refutes: true, threshold: "0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, and there is no `<runtime>` row",
    measured: `${t1.filter((t) => t.ok).length} of ${t1.length} pages: the rows add up to the script (${t1.map((t) => n(t.sum)).join(", ")} B), no \`<runtime>\` row, and the only statements that run are the mount calls`, verdict: t1.every((t) => t.ok) ? "pass" : "fail" },
  { id: "T2", refutes: true, threshold: "JS ≤ 1.5 KB raw (≈ 0.7 KB brotli) on the heaviest page; refutes above 5 KB brotli",
    measured: `\`${heaviest}\`: ${tri(heavy)} B`, verdict: heavy.raw <= 1500 ? "pass" : "fail", refuted: heavy.br > 5000 },
  { id: "T3", refutes: false, threshold: "≥ 100× below the best React build of an equivalent site (Astro + React islands + Radix: 317 KB raw)",
    measured: `JS to parse, raw, page against page: ${ratios.map((r) => Math.round(r) + "×").join(", ")}; this site's heaviest page against that build's lightest: ${Math.round(worst)}× raw, ${Math.round(worstBr)}× brotli (external JS). Another site of the same shape, built during the research`, verdict: worst >= 100 ? "pass" : "fail" },
  { id: "T4", refutes: true, threshold: "deleting the Install dialog from `/` removes its markup, its CSS rules and `invokers` from that page, and nothing else", measured: "`node bench/delta.mjs`", verdict: "not-measured" },
  { id: "T5", refutes: false, threshold: "against the control, in brotli bytes — what a page transfers; raw and gzip beside: per-page CSS ≥ 20% smaller on at least two pages, JS ≥ 30% smaller on every page that ships one",
    measured: `CSS: ${t5css("br")} (${t5css("gz")}; ${t5css("raw")}). JS: ${t5js("br")} (${t5js("gz")}; ${t5js("raw")})`,
    verdict: cssAt("br").length >= 2 && jsUnder("br").length === 0 ? "pass" : "fail" },
  { id: "T6", refutes: true, threshold: "no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source",
    measured: `${shipped.length} source files, ${Object.keys(t6).length} greps: ${t6ok ? "nothing found" : "found"}; one stylesheet import, in \`layout.rtsx\``, verdict: t6ok ? "pass" : "fail" },
  { id: "T7", refutes: true, threshold: "the browser checks pass on the built site", measured: "`node bench/verify.mjs`", verdict: "not-measured" },
  { id: "T8", refutes: false, threshold: "≤ 3 requests per page, cold",
    measured: `${[...new Set(requests)].join(", ")} per page${mode === "chrome" ? " (the document and the favicon)" : " (static crawl: the favicon is not counted)"}; \`--inline never\`: ${[...new Set(measure.never.pages.map((p) => p.requests))].join(", ")}`, verdict: Math.max(...requests) <= 3 ? "pass" : "fail" },
];
md += `\n## Thresholds\n\nplan.md, RGP2-050. T4 and T7 are measured by their own scripts.\n\n| | Threshold | Measured | Verdict |\n| --- | --- | --- | --- |\n`;
for (const t of thresholds) md += `| ${t.id}${t.refutes ? " (refutes)" : ""} | ${t.threshold} | ${t.measured} | ${t.verdict === "not-measured" ? "not measured here" : `**${t.verdict}**`} |\n`;
md += `\nCross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one \`_rg/report.json\` has, in the four builds${problems.length ? " — **but for the lines below**" : ""}; \`--inline always\` and \`--inline never\` change no byte of what a page is; the control's HTML is the default build's. `;
md += `The report's gzip (Go's \`compress/gzip\`) is between ${Math.min(...gzipApart)} and +${Math.max(...gzipApart)} B of \`gzip -9\` (zlib) on these ${gzipApart.length} pieces.\n`;
if (problems.length) md += `\n**The measurement disagrees with the builder's report:**\n\n${problems.map((p) => "- " + p).join("\n")}\n`;

console.log(md);
mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
if (opts.json) writeFileSync(resolve(opts.json), JSON.stringify({ mode, thresholds, problems, control, pages: Object.fromEntries(PAGES.map((p) => [p, D[p].sizes])), measure }, null, 1));
if (!opts.keep) rmSync(work, { recursive: true, force: true });
console.error(`\nwritten: ${relative(process.cwd(), mdOut)}`);
const refuted = thresholds.filter((t) => t.refutes && (t.verdict === "fail" || t.refuted));
for (const t of refuted) console.error(`REFUTES: ${t.id} — ${t.measured}`);
for (const p of problems) console.error(`WRONG: ${p}`);
process.exit(refuted.length || problems.length ? 1 : 0);
