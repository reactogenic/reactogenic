// Builds the Go binary the plugin drives, once for the suite.
import { execFileSync } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

export default function setup() {
  const binary = join(mkdtempSync(join(tmpdir(), "reactogenic-")), "reactogenic");
  execFileSync("go", ["build", "-o", binary, "./cmd/reactogenic"], {
    cwd: resolve(import.meta.dirname, "../../../go"),
    stdio: "inherit",
  });
  process.env.REACTOGENIC_BINARY = binary;
}
