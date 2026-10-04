// `pnpm --filter @reactogenic/ui sizes`: what each behaviour and each
// component's CSS costs — minified, gzip -9 and brotli 11 bytes. A behaviour
// is built as a page's script is (build.mjs: a generated entry, the flags
// defined); CSS is minified with nesting lowered, the form the builder ships.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { brotliCompressSync, constants, gzipSync } from "node:zlib";
import * as esbuild from "esbuild";
import { behaviours } from "./build.mjs";

const src = resolve(import.meta.dirname, "../../src");

function row(name, text) {
  const bytes = Buffer.from(text);
  const brotli = brotliCompressSync(bytes, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }).length;
  return { name, min: bytes.length, gzip: gzipSync(bytes, { level: 9 }).length, brotli };
}

async function css(name) {
  const result = await esbuild.transform(readFileSync(resolve(src, name), "utf8"), { loader: "css", minify: true, supported: { nesting: false } });
  return row(name, result.code.trim());
}

const at = (name) => `@reactogenic/ui/behaviors/${name}`;
const menu = { module: at("menu-keys"), id: "m1" };
const rows = [
  row("overlays", await behaviours([{ module: at("overlays") }])),
  row("invokers", await behaviours([{ module: at("invokers") }])),
  row("menu-keys", await behaviours([menu])),
  row("menu-keys + RG_MENU_TYPEAHEAD", await behaviours([{ ...menu, flags: { RG_MENU_TYPEAHEAD: true } }])),
  row("overlays + invokers (a page with a dialog)", await behaviours([{ module: at("overlays") }, { module: at("invokers") }])),
  row("all three (an action menu and a dialog)", await behaviours([{ module: at("overlays") }, { module: at("invokers") }, menu])),
  row("all three + RG_MENU_TYPEAHEAD", await behaviours([{ module: at("overlays") }, { module: at("invokers") }, { ...menu, flags: { RG_MENU_TYPEAHEAD: true } }])),
  ...(await Promise.all(["tokens.css", "button.css", "dialog.css", "dropdown-menu.css", "side-menu.css"].map(css))),
];
await esbuild.stop();

const width = Math.max(...rows.map((entry) => entry.name.length));
console.log(`${"".padEnd(width)}    min   gzip  brotli`);
for (const entry of rows) {
  console.log(`${entry.name.padEnd(width)} ${String(entry.min).padStart(6)} ${String(entry.gzip).padStart(6)} ${String(entry.brotli).padStart(7)}`);
}
