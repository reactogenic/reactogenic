#!/usr/bin/env node
// delta.mjs — T4, precision (specs/phase02/plan.md, RGP2-050): delete one
// component from one page of the docs site, rebuild, and diff the two
// outputs. What leaves must be that component's — its markup, the CSS rules
// only it matched, the behaviours only it mounted — and nothing else; and no
// other page's bytes may change.
//
//   node bench/delta.mjs [--binary <reactogenic>] [--md bench/results/delta.md] [--keep <dir>]
//
// The site is copied to a temporary directory (its node_modules is a link to
// site/node_modules: run `pnpm install` first) and built before and after
// each edit, under `--inline always` — sharing couples how pages are
// delivered, not what they are (builder.md, *Packaging*) — and once more as
// it ships, to say whether the delivery of another page moved.
//
//   install       T4: the *Install* dialog of `/`. It has its own `$Trigger`:
//                 nothing else refers to it.
//   cheat-sheet   T4 as first written: the cheat-sheet dialog of `/syntax/`.
//                 A menu item commands it, so deleting the dialog alone must
//                 be refused by the build (idref-not-found); deleted together
//                 with its item, the menu is a menu of links, and
//                 `menu-keys` leaves with `invokers`. Reported, not judged.
//   typeahead     one option of one component: `typeahead` off on the action
//                 menu of `/syntax/`. The site's one flag
//                 (`RG_MENU_TYPEAHEAD`): what it is worth. Reported.
//
// Exit status: 1 when T4 fails. No dependency (node >= 22).
import { cpSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { PAGES, binary, build, cssItems as items, cssKey as key, minus, mustBuild, n, options, page, repo, site } from "./lib.mjs";

const opts = options();
const mdOut = resolve(opts.md ?? join(repo, "bench/results/delta.md"));
const bin = binary(opts.binary);
const temp = opts.keep ? resolve(opts.keep) : mkdtempSync(join(tmpdir(), "reactogenic-bench-delta-"));
mkdirSync(temp, { recursive: true });
// The real path: it is what the build prints, and it is taken out of what is reported.
const work = realpathSync(temp);

// A copy of the site: its source, and a link to its installed packages.
function copy(name) {
  const dir = join(work, name);
  rmSync(dir, { recursive: true, force: true });
  cpSync(site, dir, { recursive: true, filter: (from) => !/\/(node_modules|dist|test)$/.test(from) });
  symlinkSync(join(site, "node_modules"), join(dir, "node_modules"), "dir");
  return dir;
}
// Deletes the one place where `re` matches in `file` of the copy.
function remove(dir, file, re, what) {
  const path = join(dir, file);
  const text = readFileSync(path, "utf8");
  const found = [...text.matchAll(new RegExp(re.source, re.flags.includes("g") ? re.flags : re.flags + "g"))];
  if (found.length !== 1) throw new Error(`${file}: ${what} is written ${found.length} times, not once: the site changed, and this script with it`);
  writeFileSync(path, text.replace(re, ""));
  return found[0][0];
}

// ---- the diff of two pages -----------------------------------------------------
// `after` is `before` with one span cut out: where, and what.
function cutOut(before, after, looksRight) {
  const length = before.length - after.length;
  if (length <= 0) return null;
  let prefix = 0;
  while (prefix < after.length && before[prefix] === after[prefix]) prefix++;
  let suffix = 0;
  while (suffix < after.length && before[before.length - 1 - suffix] === after[after.length - 1 - suffix]) suffix++;
  // Every start between the two is the same deletion, read at another
  // offset: the one that cuts whole elements is the one a person would name.
  let any = null;
  for (let s = Math.min(prefix, after.length); s >= Math.max(0, after.length - suffix); s--) {
    if (before.slice(0, s) + before.slice(s + length) !== after) continue;
    const span = before.slice(s, s + length);
    any ??= { at: s, span };
    if (looksRight(span)) return { at: s, span, whole: true };
  }
  return any;
}
// What the markup that left had and the page that stays has not: tag names,
// classes, ids, attribute names, `data-part` values.
function vocabulary(span, rest) {
  const words = (html) => {
    const w = new Set();
    for (const tag of html.matchAll(/<([a-z][a-z0-9-]*)((?:\s+[^\s=>\/]+(?:="[^"]*")?)*)\s*\/?>/gi)) {
      w.add(`tag:${tag[1].toLowerCase()}`);
      for (const a of tag[2].matchAll(/([^\s=>\/]+)(?:="([^"]*)")?/g)) {
        const name = a[1].toLowerCase(), value = a[2] ?? "";
        w.add(`attr:${name}`);
        if (name === "class") for (const c of value.split(/\s+/).filter(Boolean)) w.add(`class:${c}`);
        if (name === "id") w.add(`id:${value}`);
        if (name.startsWith("data-") || name === "role") w.add(`value:${name}=${value}`);
      }
    }
    return w;
  };
  const kept = words(rest);
  return [...words(span)].filter((w) => !kept.has(w));
}
function names(selector, word) {
  const [kind, name] = [word.slice(0, word.indexOf(":")), word.slice(word.indexOf(":") + 1)];
  const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  if (kind === "tag") return new RegExp(`(^|[\\s>+~(,])${esc(name)}(?![\\w-])`).test(selector);
  if (kind === "class") return new RegExp(`\\.${esc(name)}(?![\\w-])`).test(selector);
  if (kind === "id") return new RegExp(`#${esc(name)}(?![\\w-])`).test(selector);
  if (kind === "attr") return new RegExp(`\\[${esc(name)}[\\]=~|^$*\\s]`).test(selector);
  const [attr, value] = name.split("=");
  return new RegExp(`\\[${esc(attr)}=["']?${esc(value)}["']?\\]`).test(selector);
}

const failures = [];
let md = `# Deleting one component from one page\n\n`;
md += `Written by \`node bench/delta.mjs\` (specs/phase02/plan.md, RGP2-050, T4). The docs site is copied, built, edited and built again; both builds are \`reactogenic build --inline always\`.\n`;

function compare(title, edit, expect) {
  const dir = copy(title);
  const outs = {};
  for (const [mode, flags] of [["always", ["--inline", "always"]], ["auto", []]]) outs[mode] = { before: join(work, `${title}-${mode}-before`), flags };
  const before = {}, after = {};
  for (const mode of Object.keys(outs)) before[mode] = mustBuild(bin, dir, outs[mode].before, outs[mode].flags);
  const deleted = edit(dir);
  for (const mode of Object.keys(outs)) {
    outs[mode].after = join(work, `${title}-${mode}-after`);
    after[mode] = mustBuild(bin, dir, outs[mode].after, outs[mode].flags);
  }
  const fail = (text) => { failures.push(`${title}: ${text}`); return `**NO** — ${text}`; };
  const parts = (mode, when, pathname) => {
    const report = (when === "before" ? before : after)[mode];
    const entry = report.pages.find((p) => p.pathname === pathname);
    return { entry, ...page(outs[mode][when], entry) };
  };

  md += `\n## ${expect.heading}\n\nDeleted from \`site/${expect.file}\`:\n\n\`\`\`tsx\n${deleted.map((d) => d.replace(/^\n+|\s+$/g, "")).join("\n…\n")}\n\`\`\`\n`;

  // The other pages.
  md += `\n### The other pages\n\n| Page | Document, before | after | The same bytes | … as the site ships (\`--inline auto\`) |\n| --- | ---: | ---: | --- | --- |\n`;
  for (const pathname of PAGES.filter((p) => p !== expect.page)) {
    const a = parts("always", "before", pathname), b = parts("always", "after", pathname);
    const same = a.document === b.document && a.html === b.html && a.css === b.css && a.js === b.js;
    const x = parts("auto", "before", pathname), y = parts("auto", "after", pathname);
    md += `| \`${pathname}\` | ${n(Buffer.byteLength(a.document))} | ${n(Buffer.byteLength(b.document))} | ${same ? "yes: HTML, CSS and script" : fail(`${pathname} changed`)} | ${x.document === y.document ? "yes" : "no: its delivery moved"} |\n`;
  }

  // The page.
  const a = parts("always", "before", expect.page), b = parts("always", "after", expect.page);
  md += `\n### \`${expect.page}\`\n\n| | Before | After | Left |\n| --- | ---: | ---: | ---: |\n`;
  for (const [what, x, y] of [["HTML as rendered", a.html, b.html], ["CSS", a.css, b.css], ["JS", a.js, b.js], ["document", a.document, b.document]])
    md += `| ${what} | ${n(Buffer.byteLength(x))} | ${n(Buffer.byteLength(y))} | ${n(Buffer.byteLength(x) - Buffer.byteLength(y))} |\n`;

  // HTML.
  md += `\n**HTML.** `;
  const cut = cutOut(a.html, b.html, expect.markup);
  let gone = [];
  if (expect.spans === 1) {
    if (!cut) md += fail("the page after is not the page before with one span cut out");
    else {
      md += `The page after is the page before with one span of ${n(Buffer.byteLength(cut.span))} B cut out, at byte ${n(Buffer.byteLength(a.html.slice(0, cut.at)))}; every other byte is where it was. `;
      md += cut.whole ? `The span is ${expect.markupIs}:\n\n\`\`\`html\n${cut.span}\n\`\`\`\n` : fail(`the span is not ${expect.markupIs}: ${cut.span.slice(0, 200)}…`) + "\n";
      gone = vocabulary(cut.span, b.html);
    }
  } else {
    // Several places changed: say which elements, by their tags.
    const tags = (html) => html.match(/<[^>]+>/g) ?? [];
    const x = tags(a.html), y = tags(b.html);
    let p = 0;
    while (p < y.length && x[p] === y[p]) p++;
    let s = 0;
    while (s < y.length - p && x[x.length - 1 - s] === y[y.length - 1 - s]) s++;
    const was = a.html.slice(a.html.indexOf(x[p], 0) < 0 ? 0 : nth(a.html, x, p), nth(a.html, x, x.length - s));
    const is = b.html.slice(nth(b.html, y, p), nth(b.html, y, y.length - s));
    md += `Between the first and the last tag that differ, ${n(Buffer.byteLength(was))} B became ${n(Buffer.byteLength(is))} B; every byte before and after is where it was.\n\nBefore:\n\n\`\`\`html\n${was}\n\`\`\`\n\nAfter:\n\n\`\`\`html\n${is}\n\`\`\`\n`;
    gone = vocabulary(was, b.html);
  }
  if (gone.length) md += `\nWhat that markup had and the page no longer has: ${gone.map((w) => "`" + w.slice(w.indexOf(":") + 1) + "`" + ` (${w.slice(0, w.indexOf(":"))})`).join(", ")}.\n`;

  // CSS.
  const ia = items(a.css), ib = items(b.css);
  const removed = minus(ia, ib), added = minus(ib, ia);
  md += `\n**CSS.** ${ia.length} selectors and at-rules before, ${ib.length} after: ${removed.length} left, ${added.length} came.`;
  if (added.length) md += ` ${expect.judged ? fail("the sheet gained " + added.map(key).join("; ")) : "Came: " + added.map((it) => "`" + key(it) + "`").join("; ")}`;
  const unexplained = removed.filter((it) => !gone.some((w) => names(it.selector, w)));
  md += removed.length ? ` Each selector that left names something only the deleted markup had${unexplained.length === 0 ? "" : " — but for " + unexplained.length}:\n\n` : "\n";
  if (removed.length) {
    md += `| Under | Selector | Names | Declarations, B |\n| --- | --- | --- | ---: |\n`;
    for (const it of removed) {
      const why = gone.filter((w) => names(it.selector, w)).map((w) => "`" + w.slice(w.indexOf(":") + 1) + "`");
      md += `| ${it.context ? "`" + it.context + "`" : ""} | \`${it.selector.replaceAll("|", "\\|")}\` | ${why.join(", ") || "**nothing that left**"} | ${it.body === null ? "" : n(Buffer.byteLength(it.body))} |\n`;
    }
  }
  if (unexplained.length && expect.judged) fail(`${unexplained.length} selectors left that name nothing of the deleted markup: ${unexplained.map((it) => it.selector).join("; ")}`);
  // The converse: a selector that stays and can only match what left.
  const stale = ib.filter((it) => it.body !== null && !it.selector.startsWith("@") && gone.filter((w) => !w.startsWith("attr:")).some((w) => names(it.selector.replace(/:(not|has|is|where)\((?:[^()]|\([^()]*\))*\)/g, ""), w)));
  md += `\nOf the ${ib.length} that stay, ${stale.length === 0 ? "none names" : stale.length + " name"} a tag, class, id or value that left with the markup${stale.length ? ": " + stale.map((it) => "`" + it.selector + "`").join(", ") + (expect.judged ? " — " + fail("rules that only the deleted markup matched are still in the sheet") : "") : ""}.\n`;
  const sources = (report) => Object.fromEntries(report.pages.find((p) => p.pathname === expect.page).styles.sources.map((s) => [s.file, s]));
  const sa = sources(before.always), sb = sources(after.always);
  md += `\nBy source file (\`_rg/report.json\`), rules kept before → after: ${Object.keys(sa).map((f) => `\`${f.split("/").at(-1)}\` ${sa[f].rules - sa[f].rulesDropped} → ${sb[f].rules - sb[f].rulesDropped}`).join(", ")}.\n`;

  // JS.
  const rows = (entry) => (entry.modules ?? []).map((r) => `${n(r.bytes)} \`${r.path === "<entry>" ? "<entry>" : r.path.split("/").at(-1)}\``).join(" + ") || "no script";
  const mods = (entry) => (entry.mounts ?? []).map((m) => m.module.split("/").at(-1));
  const left = mods(a.entry).filter((m) => !mods(b.entry).includes(m));
  const bytes = (entry) => Object.fromEntries((entry.modules ?? []).filter((r) => r.path !== "<entry>").map((r) => [r.path.split("/").at(-1), r.bytes]));
  const ma = bytes(a.entry), mb = bytes(b.entry);
  const resized = Object.keys(mb).filter((f) => ma[f] !== mb[f]);
  md += `\n**JS.** Before: ${rows(a.entry)} = ${n(Buffer.byteLength(a.js))} B. After: ${rows(b.entry)} = ${n(Buffer.byteLength(b.js))} B. Behaviours that left: ${left.map((m) => "`" + m + "`").join(", ") || "none"}. `;
  md += resized.length === 0 ? `Every module that stays is the same bytes.` : `Of those that stay, ${resized.map((f) => `\`${f}\` went from ${n(ma[f])} to ${n(mb[f])} B`).join(", ")}.`;
  const flagsOf = (entry) => (entry.mounts ?? []).flatMap((m) => Object.entries(m.flags ?? {}).map(([k, v]) => `\`${k}=${v}\``)).join(" ") || "none";
  if (flagsOf(a.entry) !== flagsOf(b.entry)) md += ` Flags: ${flagsOf(a.entry)} → ${flagsOf(b.entry)}.`;
  md += "\n";
  if (expect.judged && !(JSON.stringify(left) === JSON.stringify(expect.behaviours) && resized.length === 0)) md += `\n${fail(`expected ${expect.behaviours.join(", ")} to leave and the rest to stay as it was`)}\n`;
  const twin = PAGES.find((p) => p !== expect.page && parts("always", "after", p).js === b.js);
  if (twin) md += `The script is now the one \`${twin}\` has, to the byte.\n`;

  // Components.
  const comps = (entry) => Object.fromEntries(entry.components.map((c) => [c.name, c.count]));
  const ca = comps(a.entry), cb = comps(b.entry);
  const moved = Object.keys(ca).filter((k) => ca[k] !== (cb[k] ?? 0)).map((k) => `\`${k}\` ×${ca[k]} → ${cb[k] ?? 0}`);
  md += `\nComponents rendered (the record): ${moved.join(", ") || "the same"}.\n`;
}
// The offset of the i-th tag of `tags` in `html`.
function nth(html, tags, i) {
  let at = 0;
  for (let k = 0; k < i; k++) at = html.indexOf(tags[k], at) + tags[k].length;
  return i < tags.length ? html.indexOf(tags[i], at) : html.length;
}

// ---- T4: the Install dialog of `/` ----------------------------------------------
compare("install", (dir) => [remove(dir, "pages/index.rtsx", /\n\s*<Dialog>[\s\S]*?<\/Dialog>/, "the Install dialog")], {
  heading: "T4: the *Install* dialog of `/`",
  file: "pages/index.rtsx", page: "/", judged: true, spans: 1,
  markup: (span) => /^<button\b[^>]*\bcommandfor="([^"]+)"[^>]*>Install<\/button><dialog\b[^>]*\bid="\1"[\s\S]*<\/dialog>$/.test(span) && span.split("<dialog").length === 2 && span.split("</dialog>").length === 2,
  markupIs: "the dialog's trigger and the `<dialog>`, whole, and nothing else",
  behaviours: ["invokers"],
});

// ---- T4 as first written: the cheat sheet of `/syntax/` ----------------------------
{
  const dir = copy("cheat-sheet-alone");
  const deleted = remove(dir, "pages/syntax/index.rtsx", /\n\s*<Dialog id="cheat-sheet">[\s\S]*?<\/Dialog>/, "the cheat-sheet dialog");
  const run = build(bin, dir, join(work, "cheat-sheet-alone-out"), ["--inline", "always"]);
  const said = (run.stdout + run.stderr).trim().replaceAll(dir + "/", "").replaceAll(dir, ".");
  md += `\n## The cheat-sheet dialog of \`/syntax/\`, alone\n\nT4 as it was first written. A menu item commands the dialog (\`commandfor="cheat-sheet"\`), so deleting the dialog and nothing else (${deleted.trim().split("\n").length} lines of \`site/pages/syntax/index.rtsx\`) leaves a button that opens nothing. `;
  md += run.status !== 0 && /idref-not-found|command-target/.test(said)
    ? `The build refuses it, exit status ${run.status}:\n\n\`\`\`\n${said}\n\`\`\`\n`
    : `**The build did not refuse it** (exit status ${run.status}):\n\n\`\`\`\n${said}\n\`\`\`\n`;
  if (run.status === 0 || !/idref-not-found|command-target/.test(said)) failures.push("cheat-sheet alone: a button that commands a dialog that is not on the page was built");
}
compare("cheat-sheet", (dir) => [
  remove(dir, "pages/syntax/index.rtsx", /\n\s*<\$Item key="cheat-sheet"[^\n]*<\/\$Item>/, "the menu item that opens the cheat sheet"),
  remove(dir, "pages/syntax/index.rtsx", /\n\s*<Dialog id="cheat-sheet">[\s\S]*?<\/Dialog>/, "the cheat-sheet dialog"),
], {
  heading: "The cheat-sheet dialog of `/syntax/`, with the menu item that opens it",
  file: "pages/syntax/index.rtsx", page: "/syntax/", judged: false, spans: 2,
  behaviours: ["menu-keys", "invokers"],
});

compare("typeahead", (dir) => [remove(dir, "pages/syntax/index.rtsx", /(?<=<DropdownMenu) typeahead(?=>)/, "the menu's `typeahead`")], {
  heading: "One option: `typeahead` off on the action menu of `/syntax/`",
  file: "pages/syntax/index.rtsx", page: "/syntax/", judged: false, spans: 1,
  markup: (span) => span === ' data-typeahead=""',
  markupIs: "the attribute the behaviour reads",
});

md += `\n## Verdict\n\n${failures.length === 0 ? "T4 **passes**: deleting the *Install* dialog from `/` removed its markup, the CSS selectors that name it and the `invokers` behaviour from that page, nothing else of the page, and no byte of any other page." : "T4 **fails**:\n\n" + failures.map((f) => "- " + f).join("\n")}\n`;

console.log(md);
mkdirSync(dirname(mdOut), { recursive: true });
writeFileSync(mdOut, md);
if (!opts.keep) rmSync(work, { recursive: true, force: true });
console.error(`\nwritten: ${relative(process.cwd(), mdOut)}`);
for (const f of failures) console.error(`FAILED: ${f}`);
process.exit(failures.length ? 1 : 0);
