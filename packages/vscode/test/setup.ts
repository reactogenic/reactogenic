// The Go binary for the tests that read its output: $REACTOGENIC_BINARY when
// set, else built once from this checkout (as packages/vite's tests do).
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

export default function setup() {
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
