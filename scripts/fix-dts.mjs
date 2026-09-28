// Declaration files keep the `.ts` extensions of relative imports
// (rewriteRelativeImportExtensions only rewrites .js); a consumer's tsc
// rejects them without allowImportingTsExtensions. Rewrite them to `.js`.
//
//   node scripts/fix-dts.mjs <dir>
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const dir = process.argv[2];
for (const entry of readdirSync(dir, { recursive: true })) {
  const file = join(dir, entry);
  if (!file.endsWith(".d.ts")) continue;
  const text = readFileSync(file, "utf8");
  const fixed = text.replace(/(from\s+|import\()(["'])(\.{1,2}\/[^"']*?)\.tsx?\2/g, "$1$2$3.js$2");
  if (fixed !== text) writeFileSync(file, fixed);
}
