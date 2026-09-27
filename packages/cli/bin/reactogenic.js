#!/usr/bin/env node
// The `reactogenic` command: runs the platform binary with the same
// arguments and exit status.
import { spawnSync } from "node:child_process";
import { binaryPath } from "../binary.js";

let binary;
try {
  binary = binaryPath();
} catch (error) {
  console.error(/** @type {Error} */ (error).message);
  process.exit(2);
}
const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`reactogenic: ${result.error.message}`);
  process.exit(2);
}
process.exit(result.status ?? 1);
