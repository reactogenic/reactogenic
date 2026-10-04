// The Go binary for the tests that read its output: $REACTOGENIC_BINARY when
// set, else built once from this checkout (as packages/vite's tests do). And
// the TS server plugin, built where tsserver finds it by name
// (node_modules/reactogenic-typescript-plugin): test/plugin.test.ts loads it.
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

export default function setup() {
  execFileSync(process.execPath, ["scripts/build.mjs"], { cwd: resolve(import.meta.dirname, ".."), stdio: ["ignore", "ignore", "inherit"] });
  if (process.env.REACTOGENIC_BINARY && existsSync(process.env.REACTOGENIC_BINARY)) {
    return;
  }
  const binary = join(mkdtempSync(join(tmpdir(), "reactogenic-")), process.platform === "win32" ? "reactogenic.exe" : "reactogenic");
  execFileSync("go", ["build", "-o", binary, "./cmd/reactogenic"], {
    cwd: resolve(import.meta.dirname, "../../../go"),
    stdio: "inherit",
  });
  process.env.REACTOGENIC_BINARY = binary;
}
