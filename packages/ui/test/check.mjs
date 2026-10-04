// `pnpm typecheck` for this package: `reactogenic check`, because plain tsc
// cannot read .rtsx. The tsconfig holds the components, the behaviours, the
// tests and the example site (test/site), which uses every component.
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { binary } from "./binary.mjs";

const result = spawnSync(binary(), ["check", "-p", resolve(import.meta.dirname, ".."), ...process.argv.slice(2)], { stdio: "inherit" });
process.exit(result.status ?? 1);
