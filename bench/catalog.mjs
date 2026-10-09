#!/usr/bin/env node
// catalog.mjs — the bet on a catalog (specs/phase02/plan.md, RGP2-050;
// decisions.md, *For the owner*, L: "rerun on a catalog of ~20 components").
//
//   node bench/catalog.mjs [--binary <reactogenic>] [--md bench/results/catalog.md] [--json out.json]
//                          [--keep <dir>] [--runs 3] [--static]
//
// Measures a FIXTURE: `bench/catalog-site` — ten pages of the kinds a product
// site has — written on `bench/catalog`, twenty components (the four of
// @reactogenic/ui and sixteen more). Neither is the design system or a
// product; their READMEs say what they are and how the pages were chosen,
// before anything was measured.
//
// The site is type-checked (`reactogenic check`: no diagnostic) and built
// with the `reactogenic` binary — the one given, $REACTOGENIC_BINARY, or
// built here from go/ — three ways, each of which must print no diagnostic:
//
//   default   reactogenic build                   what ships
//   control   reactogenic build --no-specialize   awareness off: one sheet and one script for the site
//   never     reactogenic build --inline never    the default's blobs as files
//
// and each is measured with measure.mjs's method (its header has it): every
// page loaded cold in headless Chrome from a local server, the files it
// fetched sized raw / gzip -9 / brotli -q 11 — each on its own — the
// requests, and a warm session over the ten pages in the order of
// lib.mjs's CATALOG_PAGES. Then, from the builds' own files and
// `_rg/report.json`: what each page is, the default against the control page
// by page (CSS, JS, CSS + JS, and as a share of the page), the session page
// by page, and the thresholds: T5 in brotli bytes with raw and gzip beside,
// T1, T2 as it can be read on a catalog, T6, T8. Last, a delta in T4's
// style: one component deleted from one page, twice.
//
// The output is one Markdown file (and stdout); it names no path and no
// version of this machine, so a second run writes the same bytes. Exit
// status: 1 when a threshold that refutes the bet fails, when a build prints
// a diagnostic, or when a number here disagrees with the builder's report.
//
// No dependency (node >= 22). Without Chrome ($CHROME, or the usual places)
// measure.mjs follows the pages' own <link> and <script> instead.
import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, realpathSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, extname, join, relative, resolve } from "node:path";
import { CATALOG_PAGES as PAGES, binary, build, catalog, catalogSite as site, controlEntry, cssItems, cssKey, minus, mustBuild, n, options, page, percent, repo, scriptChecks, size, smaller, tri } from "./lib.mjs";

const BUILDS = { default: [], control: ["--no-specialize"], never: ["--inline", "never"] };
const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/catalog.md"));
const bin = binary(opts.binary);
const work = realpathSync(opts.keep ? (mkdirSync(resolve(opts.keep), { recursive: true }), resolve(opts.keep)) : mkdtempSync(join(tmpdir(), "reactogenic-bench-catalog-")));

const problems = []; // a number that is not what the report says, or a build that said something
const must = (ok, text) => { if (!ok) problems.push(text); return ok; };
const zero = () => ({ raw: 0, gz: 0, br: 0 });
const plus = (...xs) => xs.reduce((a, b) => ({ raw: a.raw + b.raw, gz: a.gz + b.gz, br: a.br + b.br }), zero());
const three = (a, b) => `${smaller(a.raw, b.raw)} / ${smaller(a.gz, b.gz)} / ${smaller(a.br, b.br)}`;
const tick = (name) => "`" + name + "`";

// ---- the type check ------------------------------------------------------------
const checked = spawnSync(bin, ["check", "-p", join(site, "tsconfig.json")], { encoding: "utf8" });
must(checked.status === 0 && (checked.stdout + checked.stderr).trim() === "", `\`reactogenic check\` on the catalog site: status ${checked.status}, output: ${(checked.stdout + checked.stderr).trim().split("\n")[0]}`);

// ---- the builds ------------------------------------------------------------------
const builds = {};
for (const [name, flags] of Object.entries(BUILDS)) {
  const out = join(work, name);
  const run = build(bin, site, out, flags);
  if (run.status !== 0) { process.stderr.write(run.stdout + run.stderr); throw new Error(`reactogenic build ${flags.join(" ")} failed with status ${run.status}`); }
  // A build that has nothing to say prints one line: "10 pages written to …".
  const said = (run.stdout + run.stderr).trim().split("\n").filter((line) => !/^\d+ pages written to /.test(line));
  must(said.length === 0, `\`reactogenic build ${flags.join(" ")}\` printed a diagnostic: ${said[0]}`);
  const report = JSON.parse(readFileSync(join(out, "_rg/report.json"), "utf8"));
  const pages = {};
  for (const pathname of PAGES) {
    const entry = report.pages.find((p) => p.pathname === pathname);
    if (!entry) throw new Error(`${name}: the build has no page ${pathname}`);
    const parts = page(out, entry);
    must(Buffer.byteLength(parts.html) === entry.html.raw, `${name} ${pathname}: the HTML without packaging is ${Buffer.byteLength(parts.html)} B, the report says ${entry.html.raw}`);
    must(Buffer.byteLength(parts.css) === (entry.css?.raw ?? 0), `${name} ${pathname}: the CSS is ${Buffer.byteLength(parts.css)} B, the report says ${entry.css?.raw}`);
    must(Buffer.byteLength(parts.js) === (entry.js?.raw ?? 0), `${name} ${pathname}: the script is ${Buffer.byteLength(parts.js)} B, the report says ${entry.js?.raw}`);
    must(Buffer.byteLength(parts.document) === entry.document.raw, `${name} ${pathname}: the document is ${Buffer.byteLength(parts.document)} B, the report says ${entry.document.raw}`);
    pages[pathname] = { entry, ...parts, sizes: { document: size(parts.document), html: size(parts.html), css: size(parts.css), js: size(parts.js) } };
  }
  must(report.pages.length === PAGES.length, `${name}: ${report.pages.length} pages, ${PAGES.length} expected`);
  builds[name] = { flags, out, report, pages };
}
for (const pathname of PAGES) {
  const d = builds.default.pages[pathname], o = builds.never.pages[pathname];
  must(o.html === d.html && o.css === d.css && o.js === d.js, `${pathname}: \`--inline never\` changes what the page is, not only how it is delivered`);
  must(builds.control.pages[pathname].html === d.html, `${pathname}: the control's HTML is not the default build's`);
}
const D = builds.default.pages, C = builds.control.pages;
must(new Set(PAGES.map((p) => C[p].css)).size === 1 && new Set(PAGES.map((p) => C[p].js)).size === 1, "the control's sheet or script is not the same on every page");

// ---- measure.mjs on each -----------------------------------------------------
const jsonTmp = join(work, "measure.json");
const measured = spawnSync(process.execPath, [
  join(repo, "bench/measure.mjs"),
  ...Object.keys(BUILDS).map((name) => `${name}=${builds[name].out}`),
  "--pages", PAGES.join(","), "--json", jsonTmp, "--runs", String(opts.runs ?? 3), ...(opts.static ? ["--static"] : []),
], { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"], maxBuffer: 1 << 26 });
if (measured.status !== 0) throw new Error(`measure.mjs failed with status ${measured.status}`);
const measure = Object.fromEntries(JSON.parse(readFileSync(jsonTmp, "utf8")).map((r) => [r.name, r]));
const sections = Object.fromEntries(measured.stdout.split(/^### /m).slice(1).map((s) => {
  const name = s.slice(0, s.indexOf("\n")).trim();
  return [name, s.slice(s.indexOf("\n")).replace(/^`[^`]*` — loaded with /m, "Loaded with ").trim()];
}));
const mode = measure.default.mode;
for (const name of Object.keys(BUILDS)) for (const p of measure[name].pages) must(p.missing.length === 0, `${name} ${p.page}: requests that 404ed: ${p.missing.join(", ")}`);

// ---- the control's script: behaviours and table --------------------------------
const split = controlEntry(C["/"].js);
must(split !== null, "the control's script does not end with the pathname table: its shape changed");
const control = { css: C["/"].sizes.css, js: C["/"].sizes.js, behaviours: split ? size(split.behaviours) : null, table: split ? size(split.entry) : null };

// ---- T1: every byte of a page's script ------------------------------------------
const t1 = PAGES.map((pathname) => scriptChecks(pathname, D[pathname]));
// A page's script without its entry — the mount calls, the last statements of
// the script — is its behaviours: what the control's behaviours are compared with.
const own = Object.fromEntries(PAGES.map((pathname, i) => {
  const js = D[pathname].js;
  must(js.endsWith(t1[i].calls), `${pathname}: the script does not end with its mount calls`);
  return [pathname, size(js.slice(0, js.length - t1[i].calls.length))];
}));

// ---- the catalog: what each component weighs --------------------------------------
// CSS: the file as written; as the builder bundles it (`bytesIn` of the
// report: nesting lowered, comments gone, not minified); and minified, as it
// stands in the control's sheet — the selectors and declarations of the rules
// that name its root class. JS: the module as written, and as it stands in a
// page's script (the report's row: minified, with that page's flags).
const COMPONENTS = [
  ["Button", "@reactogenic/ui", "button", [".rg-button"], []],
  ["Dialog", "@reactogenic/ui", "dialog", [".rg-dialog"], ["overlays", "invokers"]],
  ["DropdownMenu", "@reactogenic/ui", "dropdown-menu", [".rg-menu", "--rg-menu"], ["overlays", "menu-keys", "invokers"]],
  ["SideMenu", "@reactogenic/ui", "side-menu", [".rg-sidemenu"], ["overlays"]],
  ["Accordion", "catalog", "accordion", [".bc-accordion"], []],
  ["Avatar", "catalog", "avatar", [".bc-avatar"], []],
  ["Badge", "catalog", "badge", [".bc-badge"], []],
  ["Breadcrumbs", "catalog", "breadcrumbs", [".bc-breadcrumbs"], []],
  ["Callout", "catalog", "callout", [".bc-callout"], []],
  ["Card", "catalog", "card", [".bc-card"], []],
  ["Checkbox", "catalog", "checkbox", [".bc-check"], []],
  ["CodeBlock", "catalog", "code-block", [".bc-code"], ["copy"]],
  ["Field", "catalog", "field", [".bc-field"], ["field"]],
  ["Pagination", "catalog", "pagination", [".bc-pagination"], []],
  ["Progress", "catalog", "progress", [".bc-progress", "bc-progress-travel"], []],
  ["Select", "catalog", "select", [".bc-select"], []],
  ["Table", "catalog", "table", [".bc-table"], []],
  ["Tabs", "catalog", "tabs", [".bc-tabs"], ["tabs"]],
  ["Toast", "catalog", "toast", [".bc-toast"], ["overlays", "toast"]],
  ["Tooltip", "catalog", "tooltip", [".bc-tooltip"], []],
];
const srcDir = (pkg) => (pkg === "catalog" ? join(catalog, "src") : join(repo, "packages/ui/src"));
const sources = D["/"].entry.styles.sources;
const whole = cssItems(C["/"].css);
const weight = (list) => list.reduce((a, it) => a + Buffer.byteLength(it.selector) + (it.body === null ? 1 : Buffer.byteLength(it.body) + 2), 0);
const named = (it, names) => names.some((name) => it.selector.includes(name));
const componentRows = COMPONENTS.map(([name, pkg, file, names, behaviours]) => {
  const source = sources.find((s) => s.file.endsWith(`/${file}.css`));
  must(source !== undefined, `${name}: the report has no stylesheet ${file}.css`);
  const pagesUsing = PAGES.filter((p) => D[p].entry.styles.sources.find((s) => s.file.endsWith(`/${file}.css`))?.bytesOut > 0);
  return { name, pkg, file, behaviours, written: statSync(join(srcDir(pkg), `${file}.css`)).size, bundled: source?.bytesIn ?? 0, minified: weight(whole.filter((it) => named(it, names))), pagesUsing };
});
const BEHAVIOURS = [["overlays", "@reactogenic/ui"], ["invokers", "@reactogenic/ui"], ["menu-keys", "@reactogenic/ui"], ["tabs", "catalog"], ["field", "catalog"], ["toast", "catalog"], ["copy", "catalog"]];
const behaviourRows = BEHAVIOURS.map(([name, pkg]) => {
  const rows = PAGES.flatMap((p) => (D[p].entry.modules ?? []).filter((r) => r.path.endsWith(`/behaviors/${name}.ts`)).map((r) => ({ page: p, bytes: r.bytes })));
  const flags = [...new Set(PAGES.flatMap((p) => (D[p].entry.mounts ?? []).filter((m) => m.module.endsWith(`/behaviors/${name}`)).flatMap((m) => Object.keys(m.flags ?? {}))))];
  const sizes = [...new Set(rows.map((r) => r.bytes))].sort((a, b) => a - b);
  return { name, pkg, written: statSync(join(srcDir(pkg), "behaviors", `${name}.ts`)).size, sizes, flags, pages: [...new Set(rows.map((r) => r.page))] };
});
// The convention both packages are held to (components.md, *CSS convention*),
// asked of their source: no `!important` — a layered one beats the project's
// unlayered CSS, and no project could answer it (decisions.md, K, rule 6) —
// and no option as an attribute: a variant is a class through `variants()`,
// so the only `data-*` a rule selects is a part, or state a behaviour writes
// and names (`data-full`, field.ts).
const sheetsOf = (pkg) => readdirSync(srcDir(pkg)).filter((f) => f.endsWith(".css")).sort().map((f) => ({ name: `${pkg === "catalog" ? "bench/catalog" : "packages/ui"}/src/${f}`, text: readFileSync(join(srcDir(pkg), f), "utf8").replace(/\/\*[\s\S]*?\*\//g, "") }));
const conventionSheets = [...sheetsOf("ui"), ...sheetsOf("catalog")];
const important = conventionSheets.filter((f) => /!\s*important/i.test(f.text)).map((f) => f.name);
must(important.length === 0, `\`!important\` in the design system's CSS: ${important.join(", ")}`);
const optionAttributes = [...new Set(conventionSheets.flatMap((f) => [...f.text.matchAll(/\[(data-[\w-]+)/g)].map((m) => m[1])))].sort();
must(optionAttributes.every((name) => name === "data-part" || name === "data-full"), `an option is an attribute in the catalog's CSS: ${optionAttributes.join(", ")}`);
const siteCss = sources.find((s) => s.file === "site.css");
const tokens = sources.filter((s) => s.file.endsWith("/tokens.css"));

// ---- the site's source (T6) --------------------------------------------------------
function walk(dir, out = []) {
  for (const name of readdirSync(dir).sort()) {
    if (name === "node_modules" || name === "dist") continue;
    const path = join(dir, name);
    if (statSync(path).isDirectory()) walk(path, out); else out.push(path);
  }
  return out;
}
const shipped = walk(site).map((f) => relative(site, f));
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

// ---- a delta in T4's style -------------------------------------------------------
// One component deleted from one page: what leaves that page must be the
// component's, and no other page may change. Under `--inline always`:
// sharing couples how pages are delivered, not what they are (plan.md, T4).
function copy(name) {
  const dir = join(work, name);
  rmSync(dir, { recursive: true, force: true });
  cpSync(site, dir, { recursive: true, filter: (from) => !/\/(node_modules|dist)$/.test(from) });
  symlinkSync(join(site, "node_modules"), join(dir, "node_modules"), "dir");
  return dir;
}
function partsOf(dir, out) {
  const report = mustBuild(bin, dir, out, ["--inline", "always"]);
  return Object.fromEntries(PAGES.map((pathname) => { const entry = report.pages.find((p) => p.pathname === pathname); return [pathname, { entry, ...page(out, entry) }]; }));
}
const before = partsOf(copy("delta-before"), join(work, "delta-before-out"));
const DELTAS = [
  { what: "the FAQ `Accordion` of `/pricing/`", pathname: "/pricing/", file: "pages/pricing/index.rtsx", re: /\n *<Accordion exclusive>[\s\S]*?<\/Accordion>/, names: [".bc-accordion"], behaviours: [] },
  { what: "the `Toast` of `/settings/` — with its trigger, *Save changes*", pathname: "/settings/", file: "pages/settings/index.rtsx", re: /\n *<Toast tone="success">[\s\S]*?<\/Toast>/, names: [".bc-toast"], behaviours: ["toast"] },
];
const deltas = DELTAS.map((delta, index) => {
  const dir = copy(`delta-${index}`);
  const path = join(dir, delta.file);
  const text = readFileSync(path, "utf8");
  const found = [...text.matchAll(new RegExp(delta.re.source, "g"))];
  if (found.length !== 1) throw new Error(`${delta.file}: ${delta.what} is written ${found.length} times, not once: the site changed, and this script with it`);
  writeFileSync(path, text.replace(delta.re, ""));
  const after = partsOf(dir, join(work, `delta-${index}-out`));
  const a = before[delta.pathname], b = after[delta.pathname];
  // The markup that left: what is between the longest common prefix and suffix.
  let head = 0;
  while (head < b.html.length && a.html[head] === b.html[head]) head++;
  let tail = 0;
  while (tail < b.html.length - head && a.html[a.html.length - 1 - tail] === b.html[b.html.length - 1 - tail]) tail++;
  const oneSpan = head + tail === b.html.length;
  const left = minus(cssItems(a.css), cssItems(b.css)), came = minus(cssItems(b.css), cssItems(a.css));
  // A rule that is rewritten — `:root`, less a token only the component read —
  // leaves and comes: the same selector on both sides.
  const rewritten = left.filter((it) => came.some((x) => x.context === it.context && x.selector === it.selector));
  const gone = left.filter((it) => !rewritten.includes(it));
  const foreign = gone.filter((it) => !named(it, delta.names));
  const newcomers = came.filter((it) => !rewritten.some((x) => x.context === it.context && x.selector === it.selector));
  const rowsOf = (p) => (p.entry.modules ?? []).filter((r) => r.path !== "<entry>");
  const modulesLeft = rowsOf(a).filter((r) => !rowsOf(b).some((x) => x.path === r.path)).map((r) => r.path.split("/").at(-1).replace(".ts", ""));
  const modulesChanged = rowsOf(b).filter((r) => rowsOf(a).find((x) => x.path === r.path)?.bytes !== r.bytes).map((r) => r.path.split("/").at(-1));
  const others = PAGES.filter((p) => p !== delta.pathname && (before[p].html !== after[p].html || before[p].css !== after[p].css || before[p].js !== after[p].js));
  const ok = oneSpan && foreign.length === 0 && newcomers.length === 0 && modulesChanged.length === 0 && JSON.stringify(modulesLeft) === JSON.stringify(delta.behaviours) && others.length === 0;
  return { ...delta, html: Buffer.byteLength(a.html) - Buffer.byteLength(b.html), oneSpan, css: Buffer.byteLength(a.css) - Buffer.byteLength(b.css), gone, foreign, rewritten, newcomers, js: [Buffer.byteLength(a.js), Buffer.byteLength(b.js)], modulesLeft, modulesChanged, others, ok };
});

// ---- the tables ------------------------------------------------------------------
const mean = (xs) => xs.reduce((a, b) => a + b, 0) / xs.length;
const delivery = (e) => (e ? (e.delivery === "file" ? `file, ${e.pages} page${e.pages === 1 ? "" : "s"}` : `inline${e.pages > 1 ? `, the same on ${e.pages} pages` : ""}`) : "—");
const heaviest = PAGES.reduce((a, b) => (D[b].sizes.js.raw > D[a].sizes.js.raw ? b : a));

let md = `# The catalog site, measured\n\n`;
md += `Written by \`node bench/catalog.mjs\` (specs/phase02/plan.md, RGP2-050; the conclusion is specs/phase02/bet.md). **A fixture**: \`bench/catalog-site\`, ${PAGES.length} pages, on \`bench/catalog\`, ${COMPONENTS.length} components — neither is the design system or a product (their READMEs). `;
md += `Three builds, each ${mode === "chrome" ? "loaded cold in headless Chrome" : "crawled statically (no browser)"} by \`bench/measure.mjs\`. Bytes are raw / gzip -9 / brotli -q 11, each file compressed on its own.\n\n`;
md += `\`reactogenic check\` on the site: ${checked.status === 0 && (checked.stdout + checked.stderr).trim() === "" ? "no diagnostic" : "**diagnostics**"}. The three builds: ${problems.some((p) => p.includes("printed a diagnostic")) ? "**a diagnostic was printed**" : "no diagnostic"}.\n`;

// The catalog.
md += `\n## The catalog\n\n`;
md += `What each component weighs. CSS: its file as written (with comments); as the builder bundles it (the report's \`bytesIn\`: nesting lowered, no comments, not minified); and minified — the selectors and declarations of the control's sheet that name its root class. "On" is the pages whose sheet keeps a rule of it.\n\n`;
md += `| Component | From | CSS, written | bundled | minified ≈ | Behaviours it mounts | On |\n| --- | --- | ---: | ---: | ---: | --- | ---: |\n`;
for (const c of componentRows) md += `| \`${c.name}\` | ${c.pkg === "catalog" ? "the fixture" : "`@reactogenic/ui`"} | ${n(c.written)} | ${n(c.bundled)} | ${n(c.minified)} | ${c.behaviours.map(tick).join(", ") || "—"} | ${c.pagesUsing.length} |\n`;
md += `| tokens (${tokens.length} files) | | ${n(statSync(join(srcDir("ui"), "tokens.css")).size + statSync(join(srcDir("catalog"), "tokens.css")).size)} | ${n(tokens.reduce((a, s) => a + s.bytesIn, 0))} | | | |\n`;
md += `| the site's own \`site.css\` | | ${n(statSync(join(site, "site.css")).size)} | ${n(siteCss.bytesIn)} | | | |\n`;
md += `| **the twenty** | | **${n(componentRows.reduce((a, c) => a + c.written, 0))}** | **${n(componentRows.reduce((a, c) => a + c.bundled, 0))}** | **${n(componentRows.reduce((a, c) => a + c.minified, 0))}** | | |\n`;
md += `\nThe sixteen of the fixture are ${n(componentRows.filter((c) => c.pkg === "catalog").reduce((a, c) => a + c.minified, 0))} B minified, ${n(Math.round(mean(componentRows.filter((c) => c.pkg === "catalog").map((c) => c.minified))))} B each on average; the four of \`@reactogenic/ui\`, ${n(componentRows.filter((c) => c.pkg !== "catalog").reduce((a, c) => a + c.minified, 0))} B, ${n(Math.round(mean(componentRows.filter((c) => c.pkg !== "catalog").map((c) => c.minified))))} B each. Components no page uses: ${componentRows.filter((c) => c.pagesUsing.length === 0).map((c) => tick(c.name)).join(", ") || "none"}.\n\n`;
md += `| Behaviour | From | Written | In a page's script (the report's row) | Flags | On |\n| --- | --- | ---: | ---: | --- | ---: |\n`;
for (const b of behaviourRows) md += `| \`${b.name}\` | ${b.pkg === "catalog" ? "the fixture" : "`@reactogenic/ui`"} | ${n(b.written)} | ${b.sizes.map(n).join(" – ")} | ${b.flags.map(tick).join(", ") || "—"} | ${b.pages.length} |\n`;

// The pages.
md += `\n## The pages\n\nIn the order of the session. "Components" is the report's record of what was rendered, less the page, the layout and the helpers.\n\n`;
const HELPERS = new Set(["Page", "Layout", "Each", "Item", "Items", "Section", "MenuItem", "Action", "Head", "Crumb", "Step", "Option", "Plans"]);
md += `| Page | Components rendered | Behaviours mounted | Rules kept of the sheet's | Custom properties dropped |\n| --- | --- | --- | ---: | ---: |\n`;
for (const p of PAGES) {
  const e = D[p].entry;
  const components = e.components.filter((c) => !HELPERS.has(c.name)).map((c) => `${c.name}${c.count > 1 ? " ×" + c.count : ""}`).join(", ");
  const behaviours = [...new Set((e.mounts ?? []).map((m) => m.module.split("/").at(-1) + (Object.entries(m.flags ?? {}).filter(([, v]) => v).map(([k]) => ` \`${k}\``).join(""))))].join(", ");
  md += `| \`${p}\` | ${components} | ${behaviours} | ${e.styles.rules - e.styles.rulesDropped} of ${e.styles.rules} | ${e.styles.customPropertiesDropped} |\n`;
}

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
md += `What component awareness changes, page by page: the same HTML in both builds; "smaller" is 1 − default / control, raw / gzip / brotli.\n\n`;
md += `| Page | CSS, default | CSS, control | smaller |\n| --- | ---: | ---: | ---: |\n`;
for (const p of PAGES) md += `| \`${p}\` | ${tri(D[p].sizes.css)} | ${tri(control.css)} | ${three(D[p].sizes.css, control.css)} |\n`;
md += `\n| Page | JS, default | JS, control | smaller | its behaviours (the script without its mount calls) | the control's behaviours (without its table) | smaller |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n`;
for (const p of PAGES) md += `| \`${p}\` | ${tri(D[p].sizes.js)} | ${tri(control.js)} | ${three(D[p].sizes.js, control.js)} | ${tri(own[p])} | ${control.behaviours ? tri(control.behaviours) : "—"} | ${control.behaviours ? three(own[p], control.behaviours) : "—"} |\n`;
md += `\nCSS and JS together, and what that is of the page — its HTML, CSS and JS, each compressed apart:\n\n`;
md += `| Page | CSS + JS, default | CSS + JS, control | smaller | the page, default | the page, control | smaller |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n`;
const both = Object.fromEntries(PAGES.map((p) => [p, { d: plus(D[p].sizes.css, D[p].sizes.js), c: plus(control.css, control.js), pd: plus(D[p].sizes.html, D[p].sizes.css, D[p].sizes.js), pc: plus(D[p].sizes.html, control.css, control.js) }]));
for (const p of PAGES) md += `| \`${p}\` | ${tri(both[p].d)} | ${tri(both[p].c)} | ${three(both[p].d, both[p].c)} | ${tri(both[p].pd)} | ${tri(both[p].pc)} | ${three(both[p].pd, both[p].pc)} |\n`;
const sumD = plus(...PAGES.map((p) => both[p].pd)), sumC = plus(...PAGES.map((p) => both[p].pc));
md += `| **mean** | ${tri({ raw: Math.round(mean(PAGES.map((p) => both[p].d.raw))), gz: Math.round(mean(PAGES.map((p) => both[p].d.gz))), br: Math.round(mean(PAGES.map((p) => both[p].d.br))) })} | ${tri(both[PAGES[0]].c)} | ${three(plus(...PAGES.map((p) => both[p].d)), plus(...PAGES.map((p) => both[p].c)))} | ${tri({ raw: Math.round(sumD.raw / PAGES.length), gz: Math.round(sumD.gz / PAGES.length), br: Math.round(sumD.br / PAGES.length) })} | ${tri({ raw: Math.round(sumC.raw / PAGES.length), gz: Math.round(sumC.gz / PAGES.length), br: Math.round(sumC.br / PAGES.length) })} | ${three(sumD, sumC)} |\n`;

// What the sheets have in common.
const sheet = Object.fromEntries(PAGES.map((p) => [p, cssItems(D[p].css)]));
let common = sheet[PAGES[0]];
for (const p of PAGES.slice(1)) common = common.filter(((left) => (it) => { const i = left.findIndex((x) => cssKey(x) === cssKey(it)); if (i >= 0) left.splice(i, 1); return i >= 0; })([...sheet[p]]));
// On no page — and not a rule that every page has with fewer declarations
// (`:root`, less the tokens the page does not read).
const at = (it) => `${it.context} | ${it.selector}`;
const nowhere = whole.filter((it) => PAGES.every((p) => !sheet[p].some((x) => at(x) === at(it))));
const thinned = whole.filter((it) => !nowhere.includes(it) && PAGES.every((p) => !sheet[p].some((x) => cssKey(x) === cssKey(it))));
md += `\nHow much of a page's sheet is the page's own. A sheet is read as its selectors and at-rules (a rule counts once per selector of its list); "≈ B" is the selectors and declarations alone, without the at-rules around them.\n\n`;
md += `| Page | Selectors and at-rules | … on every page | … the page's own | ≈ B of its own | Dropped from the control's ${whole.length} |\n| --- | ---: | ---: | ---: | ---: | ---: |\n`;
for (const p of PAGES) { const mine = minus(sheet[p], common); md += `| \`${p}\` | ${sheet[p].length} | ${common.length} | ${mine.length} | ${n(weight(mine))} | ${minus(whole, sheet[p]).length} |\n`; }
md += `\n${common.length} of the control's ${whole.length} are on all ${PAGES.length} pages (≈ ${n(weight(common))} B of selectors and declarations): the layout's — the side menu, the links menu, the button — and what of the site's own sheet every page has. `;
md += `${nowhere.length} are on no page (≈ ${n(weight(nowhere))} B) — options and parts of the catalog that this site does not use: ${nowhere.map((it) => tick(it.selector)).join(", ") || "none"}. `;
md += `${thinned.length} ${thinned.length === 1 ? "is" : "are"} on pages only with fewer declarations than the control has: ${thinned.map((it) => tick(it.selector)).join(", ") || "none"} — the tokens a page does not read are dropped (the table of *The pages*).\n`;
// State rules kept where nothing can reach the state (decisions.md, M): the
// rules of a page's sheet whose selector is a runtime-state attribute that no
// element of the page has.
const STATE = /\[(aria-[\w-]+|data-state|disabled|checked|selected|hidden|inert)(=[^\]]*)?\]/g;
const unreached = Object.fromEntries(PAGES.map((p) => [p, sheet[p].filter((it) => { const attrs = [...it.selector.matchAll(STATE)]; return attrs.length > 0 && attrs.some(([, name, value]) => !new RegExp(`\\s${name}=${value ? `"${value.slice(1).replaceAll('"', "")}"` : ""}`).test(D[p].html)); })]));
md += `\nRules for a state attribute — \`aria-*\`, \`disabled\`, \`hidden\`, … — that no element of the page has as it loads (runtime state is "maybe": builder.md, *CSS*; decisions.md, M — a behaviour of the page may write it, or nothing can): ${PAGES.map((p) => `\`${p}\` ${unreached[p].length} (≈ ${n(weight(unreached[p]))} B)`).join(", ")}.\n`;

// The session.
md += `\n## The session\n\n`;
md += `The ten pages in order, with a warm HTTP cache — each URL fetched once — as each build delivers them (\`measure.mjs\`). The running total after each page, in brotli bytes:\n\n`;
const running = (name) => { const seen = new Set(); let total = 0; return measure[name].pages.map((p) => { for (const f of p.files) { const key = f.kind === "html" ? "doc:" + p.page : f.url; if (!seen.has(key)) { seen.add(key); total += f.br; } } return total; }); };
const run = Object.fromEntries(Object.keys(BUILDS).map((name) => [name, running(name)]));
md += `| After | default | control | \`--inline never\` | the lighter of default and control |\n| --- | ---: | ---: | ---: | --- |\n`;
PAGES.forEach((p, i) => { md += `| ${i + 1}: \`${p}\` | ${n(run.default[i])} | ${n(run.control[i])} | ${n(run.never[i])} | ${run.default[i] === run.control[i] ? "equal" : run.default[i] < run.control[i] ? `default, by ${smaller(run.default[i], run.control[i])}` : `control, by ${smaller(run.control[i], run.default[i])}`} |\n`; });
for (const name of Object.keys(BUILDS)) must(run[name].at(-1) === measure[name].session.total.br, `${name}: the running total ends at ${run[name].at(-1)} B, the session is ${measure[name].session.total.br}`);
md += `\n| Build | Requests per page, cold | Page, cold: mean total | Session of ${PAGES.length} pages, warm cache: requests | … total |\n| --- | ---: | ---: | ---: | ---: |\n`;
for (const name of Object.keys(BUILDS)) {
  const m = measure[name];
  const total = { raw: Math.round(mean(m.pages.map((p) => p.total.raw))), gz: Math.round(mean(m.pages.map((p) => p.total.gz))), br: Math.round(mean(m.pages.map((p) => p.total.br))) };
  md += `| \`${name}\` | ${[...new Set(m.pages.map((p) => p.requests))].join(", ")} | ${tri(total)} | ${m.session.requests} | ${tri(m.session.total)} |\n`;
}
const sessD = measure.default.session.total, sessC = measure.control.session.total;
const lighter = sessC.br <= sessD.br ? ["control", sessC, "default build", sessD] : ["default build", sessD, "control", sessC];
const how = (e) => (e.delivery === "file" ? `a file, fetched once` : `inlined in each of the ${e.pages} pages`);
const coldD = mean(measure.default.pages.map((p) => p.total.br)), coldC = mean(measure.control.pages.map((p) => p.total.br));
md += `\nCold, a page of the default build is ${smaller(coldD, coldC)} smaller than the control's (brotli, mean of ${PAGES.length}). `;
md += `Over the session the ${lighter[0]} is **${three(lighter[1], lighter[3])} smaller** than the ${lighter[2]} (raw / gzip / brotli). `;
md += `The control's one sheet (${n(control.css.raw)} B) is ${how(C["/"].entry.css)}, its one script (${n(control.js.raw)} B) ${how(C["/"].entry.js)}; `;
const distinctCss = new Set(PAGES.map((p) => D[p].css)).size, distinctJs = new Set(PAGES.map((p) => D[p].js)).size;
md += `the default build's ${distinctCss} distinct sheets — ${PAGES.map((p) => n(D[p].sizes.css.raw)).join(", ")} B — and ${distinctJs} distinct scripts are ${PAGES.every((p) => D[p].entry.css.delivery === "inline" && D[p].entry.js.delivery === "inline") ? "each inlined in their page" : "delivered as the tables above say"}.\n`;
const firstBehind = run.default.findIndex((x, i) => x > run.control[i]);
md += firstBehind < 0 ? `The default build is never behind over this session.\n` : `The control is ahead from page ${firstBehind + 1} of the visit on.\n`;

// T1.
md += `\n## What each page's script is (T1)\n\nThe default build. The rows are \`_rg/report.json\`'s (\`modules\`: from esbuild's metafile).\n\n`;
md += `| Page | Script, raw | Rows of the report | Sum | Mounts |\n| --- | ---: | --- | ---: | --- |\n`;
for (const t of t1) {
  const rows = t.rows.map((r) => `${n(r.bytes)} \`${r.path === "<entry>" ? "<entry>" : r.path.split("/").at(-1)}\``).join(" + ");
  const mounts = t.mounts.map((m) => `\`${m.module.split("/").at(-1)}\`${m.id ? ` on \`#${m.id}\`` : ""}${m.flags ? " " + Object.entries(m.flags).map(([k, v]) => `\`${k}=${v}\``).join(" ") : ""}${m.data ? ` with \`${JSON.stringify(m.data)}\`` : ""}`).join(", ");
  md += `| \`${t.pathname}\` | ${n(D[t.pathname].sizes.js.raw)} | ${rows} | ${n(t.sum)} | ${mounts} |\n`;
}
md += `\nChecked on every page, by reading the script and the document:\n\n`;
for (const name of Object.keys(t1[0].checks)) md += `- ${name}: ${t1.every((t) => t.checks[name]) ? "yes" : "**NO** — " + t1.filter((t) => !t.checks[name]).map((t) => tick(t.pathname)).join(", ")}\n`;
const distinct = [...new Map(PAGES.map((p) => [D[p].js, p])).entries()];
md += `\nThe ${distinct.length} distinct scripts of the site, whole:\n`;
for (const [js] of distinct) md += `\n${PAGES.filter((p) => D[p].js === js).map(tick).join(", ")} — ${n(Buffer.byteLength(js))} B:\n\n\`\`\`js\n${js}\n\`\`\`\n`;

// T6.
md += `\n## The site's source (T6)\n\n${shipped.length} files under \`bench/catalog-site/\` (without \`node_modules/\`): ${shipped.map(tick).join(", ")}.\n\n`;
for (const [name, hits] of Object.entries(t6)) md += `- ${name}: ${hits.length === 0 ? "yes" : "**NO** — " + hits.join(", ")}\n`;
md += `- the stylesheet imports of the whole site: ${cssImports.map(tick).join(", ") || "none"}\n`;
md += `\nThe catalog itself is the other side of the rule: its components import their CSS and call \`mount()\`, as a design system does (\`bench/catalog/src\`).\n`;
md += `\nIts CSS, and \`@reactogenic/ui\`'s — ${conventionSheets.length} files — by the convention (specs/phase02/components.md, *CSS convention*): no \`!important\`: ${important.length === 0 ? "yes" : "**NO** — " + important.join(", ")}; the \`data-*\` attributes its rules select: ${optionAttributes.map(tick).join(", ")} — a part, and state a behaviour writes; every option is a class of its own, through \`variants()\`.\n`;

// The delta.
md += `\n## One component deleted from one page (T4's question)\n\nUnder \`--inline always\`. T4 itself is the docs site's (\`bench/delta.mjs\`); this asks the same of the catalog, twice.\n\n`;
md += `| Deleted | Markup | CSS | Selectors that left | … that do not name the component | Rewritten | Script | Behaviours that left | Other pages changed | |\n| --- | ---: | ---: | ---: | ---: | --- | --- | --- | ---: | --- |\n`;
for (const d of deltas) {
  md += `| ${d.what} | −${n(d.html)} B, ${d.oneSpan ? "one span" : "**several places**"} | −${n(d.css)} B | ${d.gone.length} | ${d.foreign.length}${d.foreign.length ? ": " + d.foreign.map((it) => tick(it.selector)).join(", ") : ""} | ${d.rewritten.map((it) => tick(it.selector)).join(", ") || "—"}${d.newcomers.length ? `; **came:** ${d.newcomers.map((it) => tick(it.selector)).join(", ")}` : ""} | ${n(d.js[0])} → ${n(d.js[1])} B | ${d.modulesLeft.map(tick).join(", ") || "—"}${d.modulesChanged.length ? `; **changed:** ${d.modulesChanged.join(", ")}` : ""} | ${d.others.length}${d.others.length ? ": " + d.others.map(tick).join(", ") : ""} | **${d.ok ? "pass" : "fail"}** |\n`;
}
md += `\n"Rewritten" is a rule that stays with fewer declarations: \`:root\`, less the tokens only the deleted component read.\n`;

// Thresholds.
// T5 is decided in brotli bytes — what a page transfers (plan.md, RGP2-050;
// the owner's ruling, decisions.md, L); raw and gzip are reported beside it.
// On the catalog its CSS half reads "on at least half of the pages".
const UNITS = { br: "brotli", gz: "gzip", raw: "raw" };
const half = Math.ceil(PAGES.length / 2);
const cssOk = (p, unit) => percent(D[p].sizes.css[unit], control.css[unit]) >= 20;
const jsOk = (p, unit) => D[p].sizes.js.raw === 0 || percent(D[p].sizes.js[unit], control.js[unit]) >= 30;
// The same question of the behaviours alone: the page's script without its
// mount calls, the control's without its table (builder.md, *The control*:
// "so that T5 does not divide by it").
const jsOwnOk = (p, unit) => D[p].sizes.js.raw === 0 || !control.behaviours || percent(own[p][unit], control.behaviours[unit]) >= 30;
const t5 = Object.fromEntries(Object.keys(UNITS).map((unit) => [unit, {
  css: PAGES.filter((p) => cssOk(p, unit)), jsUnder: PAGES.filter((p) => !jsOk(p, unit)), jsOwnUnder: PAGES.filter((p) => !jsOwnOk(p, unit)),
}]));
const t5pass = (unit) => t5[unit].css.length >= half && t5[unit].jsUnder.length === 0;
md += `\n## T5, page by page\n\nplan.md, RGP2-050: against the control, **in brotli bytes** — what a page transfers — per-page CSS ≥ 20% smaller on at least half of the pages (${half} of ${PAGES.length}), JS ≥ 30% smaller on every page that ships a script (all ${PAGES.filter((p) => D[p].sizes.js.raw > 0).length} do: the layout mounts \`overlays\`). Raw and gzip beside.\n\n`;
md += `| Page | CSS smaller, brotli | ≥ 20% | gzip | raw | JS smaller, brotli | ≥ 30% | gzip | raw | JS, behaviours alone, brotli | ≥ 30% |\n| --- | ---: | --- | ---: | ---: | ---: | --- | ---: | ---: | ---: | --- |\n`;
const yes = (ok) => (ok ? "yes" : "**no**");
for (const p of PAGES) {
  const c = D[p].sizes.css, j = D[p].sizes.js;
  md += `| \`${p}\` | ${smaller(c.br, control.css.br)} | ${yes(cssOk(p, "br"))} | ${smaller(c.gz, control.css.gz)} | ${smaller(c.raw, control.css.raw)} | ${smaller(j.br, control.js.br)} | ${yes(jsOk(p, "br"))} | ${smaller(j.gz, control.js.gz)} | ${smaller(j.raw, control.js.raw)} | ${control.behaviours ? smaller(own[p].br, control.behaviours.br) : "—"} | ${yes(jsOwnOk(p, "br"))} |\n`;
}
md += `\n| Unit | CSS: pages at 20% or more | JS: pages under 30% | T5 | JS, behaviours alone: pages under 30% |\n| --- | ---: | --- | --- | --- |\n`;
for (const unit of Object.keys(UNITS)) md += `| ${UNITS[unit]}${unit === "br" ? " — **the unit T5 is decided in**" : ""} | ${t5[unit].css.length} of ${PAGES.length} | ${t5[unit].jsUnder.map(tick).join(", ") || "none"} | ${unit === "br" ? `**${t5pass(unit) ? "pass" : "fail"}**` : t5pass(unit) ? "would pass" : "would fail"} | ${t5[unit].jsOwnUnder.map(tick).join(", ") || "none"} |\n`;

const heavy = D[heaviest].sizes.js;
const mountedOn = (p) => new Set((D[p].entry.mounts ?? []).map((m) => m.module)).size;
const heavyMounted = mountedOn(heaviest);
// The docs site's budget, restated per behaviour mounted (1.5 KB for three):
// a reading made after the fact, reported and not judged.
const perBehaviour = (p) => Math.round(D[p].sizes.js.raw / mountedOn(p));
const dearest = PAGES.reduce((a, b) => (perBehaviour(b) > perBehaviour(a) ? b : a));
const requests = measure.default.pages.map((p) => p.requests);
// What a page of the default build fetches beside its document.
const beside = [...new Set(measure.default.pages.flatMap((p) => p.files.filter((f) => f.kind !== "html").map((f) => f.url)))].sort();
const besideOn = (url) => { const on = measure.default.pages.filter((p) => p.files.some((f) => f.url === url)).map((p) => p.page); return on.length === PAGES.length ? tick(url) : `${tick(url)} on ${on.map(tick).join(", ")}`; };
const thresholds = [
  { id: "T1", refutes: true, threshold: "0 bytes of React or of any generic runtime: every JS byte of a page is in a row of its report, and there is no `<runtime>` row",
    measured: `${t1.filter((t) => t.ok).length} of ${t1.length} pages: the rows add up to the script (${t1.map((t) => n(t.sum)).join(", ")} B), no \`<runtime>\` row, and the only statements that run are the mount calls`, verdict: t1.every((t) => t.ok) ? "pass" : "fail" },
  { id: "T2", refutes: true, threshold: "JS on the heaviest page. The budget of plan.md — ≤ 1.5 KB raw — was set for the docs site's three behaviours, and is not carried over to a page that mounts more: read here as its bound alone, **refuted above 5 KB brotli** (\"a micro-runtime, not compilation\"); the heaviest page is reported",
    measured: `\`${heaviest}\`: ${tri(heavy)} B, ${heavyMounted} behaviours. Under 1.5 KB raw: ${PAGES.filter((p) => D[p].sizes.js.raw <= 1500).length} of ${PAGES.length} pages. Not judged — the docs site's budget per behaviour mounted is 500 B (1.5 KB for three): \`${heaviest}\` is at ${n(perBehaviour(heaviest))} B, and the most is \`${dearest}\`, ${n(perBehaviour(dearest))} B (${mountedOn(dearest)} behaviours, ${n(t1[PAGES.indexOf(dearest)].entryBytes)} B of mount calls)`, verdict: heavy.br <= 5000 ? "pass" : "fail", refuted: heavy.br > 5000 },
  { id: "T3", refutes: false, threshold: "≥ 100× below the best React build of an equivalent site", measured: "no React build of this fixture exists", verdict: "not-measured" },
  { id: "T4", refutes: true, threshold: "deleting one component from one page removes its markup, the CSS rules only it matched and the behaviours only it mounted, and nothing else; every other page the same bytes (T4 itself names the docs site's *Install* dialog: `bench/delta.mjs`)",
    measured: deltas.map((d) => `${d.what}: −${n(d.html)} B of markup, ${d.gone.length} selectors (${d.foreign.length} foreign), ${d.modulesLeft.map(tick).join(", ") || "no behaviour"} left, ${d.others.length} other pages changed`).join("; "), verdict: deltas.every((d) => d.ok) ? "pass" : "fail" },
  { id: "T5", refutes: false, threshold: `against the control, in brotli bytes: per-page CSS ≥ 20% smaller on at least half of the pages (${half} of ${PAGES.length}), JS ≥ 30% smaller on every page that ships a script`,
    measured: `CSS: ${t5.br.css.length} of ${PAGES.length} pages at 20% or more (${smaller(Math.max(...PAGES.map((p) => D[p].sizes.css.br)), control.css.br)} to ${smaller(Math.min(...PAGES.map((p) => D[p].sizes.css.br)), control.css.br)}). JS: ${t5.br.jsUnder.length ? `under 30% on ${t5.br.jsUnder.map(tick).join(", ")}` : `every page at 30% or more (${smaller(Math.max(...PAGES.map((p) => D[p].sizes.js.br)), control.js.br)} to ${smaller(Math.min(...PAGES.map((p) => D[p].sizes.js.br)), control.js.br)})`}; against the control's behaviours alone, ${t5.br.jsOwnUnder.length ? `under 30% on ${t5.br.jsOwnUnder.map(tick).join(", ")}` : "every page at 30% or more too"}. In gzip it ${t5pass("gz") ? "holds" : "fails"}, raw it ${t5pass("raw") ? "holds" : "fails"}`,
    verdict: t5pass("br") ? "pass" : "fail" },
  { id: "T6", refutes: true, threshold: "no `<script>`, no hand-written JS, no per-page list of styles or behaviours in the site's source",
    measured: `${shipped.length} source files, ${Object.keys(t6).length} greps: ${t6ok ? "nothing found" : "found"}; one stylesheet import, in \`layout.rtsx\``, verdict: t6ok ? "pass" : "fail" },
  { id: "T7", refutes: true, threshold: "the browser checks pass on the built site", measured: "`node bench/catalog-verify.mjs`", verdict: "not-measured" },
  { id: "T8", refutes: false, threshold: "≤ 3 requests per page, cold",
    measured: `${[...new Set(requests)].join(", ")} per page${mode === "chrome" ? ` (the document, and ${beside.map(besideOn).join(", ")})` : " (static crawl: the favicon and the pictures are not counted)"}; the control: ${[...new Set(measure.control.pages.map((p) => p.requests))].join(", ")}; \`--inline never\`: ${[...new Set(measure.never.pages.map((p) => p.requests))].join(", ")}`, verdict: Math.max(...requests) <= 3 ? "pass" : "fail" },
];
md += `\n## Thresholds, on the catalog\n\nplan.md, RGP2-050, as each can be read on this fixture. The thresholds were set for the docs site; where one names it, the row says how it is read here.\n\n| | Threshold | Measured | Verdict |\n| --- | --- | --- | --- |\n`;
for (const t of thresholds) md += `| ${t.id}${t.refutes ? " (refutes)" : ""} | ${t.threshold} | ${t.measured} | ${t.verdict === "not-measured" ? "not measured here" : `**${t.verdict}**`} |\n`;
md += `\nCross-checked: the raw size of every document, of its HTML without what packaging wrote, of its CSS and of its script is the one \`_rg/report.json\` has, in the three builds${problems.length ? " — **but for the lines below**" : ""}; \`--inline never\` changes no byte of what a page is; the control's HTML is the default build's, and its sheet and script are one each.\n`;
if (problems.length) md += `\n**Wrong:**\n\n${problems.map((p) => "- " + p).join("\n")}\n`;

console.log(md);
mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
if (opts.json) writeFileSync(resolve(opts.json), JSON.stringify({ mode, thresholds, problems, control, t5, pages: Object.fromEntries(PAGES.map((p) => [p, D[p].sizes])), measure }, null, 1));
if (!opts.keep) rmSync(work, { recursive: true, force: true });
console.error(`\nwritten: ${relative(process.cwd(), mdOut)}`);
const refuted = thresholds.filter((t) => t.refutes && (t.verdict === "fail" || t.refuted));
for (const t of refuted) console.error(`REFUTES: ${t.id} — ${t.measured}`);
for (const p of problems) console.error(`WRONG: ${p}`);
process.exit(refuted.length || problems.length ? 1 : 0);
