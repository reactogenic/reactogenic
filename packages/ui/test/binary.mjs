// The `reactogenic` binary of this commit, for the checks and tests of this
// package: $REACTOGENIC_BINARY, or else built from ../../go once per run.
import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

export function binary() {
  if (process.env.REACTOGENIC_BINARY) {
    return process.env.REACTOGENIC_BINARY;
  }
  const file = join(mkdtempSync(join(tmpdir(), "reactogenic-")), "reactogenic");
  execFileSync("go", ["build", "-trimpath", "-o", file, "./cmd/reactogenic"], {
    cwd: resolve(import.meta.dirname, "../../../go"),
    stdio: "inherit",
  });
  process.env.REACTOGENIC_BINARY = file;
  return file;
}
