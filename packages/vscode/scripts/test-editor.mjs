// The editor suite (`pnpm test:editor`): the extension in a real VS Code with
// an isolated profile, against a copy of test/fixture. Two windows, one after
// the other:
//
//   trusted     test/editor/trusted.ts     the server and the client's features
//   untrusted   test/editor/untrusted.ts   highlighting only, no server process
//
//   node scripts/test-editor.mjs [--vsix file.vsix] [trusted|untrusted ...]
//
// Without --vsix it tests this checkout: the extension from this folder, and
// $REACTOGENIC_BINARY — unset: built from ../../go. With --vsix it tests a
// package as it ships: the unpacked .vsix and the binary bundled in it.
//
//   $VSCODE_EXECUTABLE    the VS Code to run; unset: downloaded into .vscode-test
//   $VSCODE_VERSION       the version to download ("stable")
//   $EDITOR_TEST_VERBOSE  show VS Code's stderr instead of keeping it in a file
//
// It opens windows: on Linux without a display, run it under `xvfb-run -a`.
import { execFileSync, spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { downloadAndUnzipVSCode } from "@vscode/test-electron";
import { build, root } from "./build.mjs";

const args = process.argv.slice(2);
const vsix = args.includes("--vsix") ? path.resolve(args.splice(args.indexOf("--vsix"), 2)[1]) : undefined;
const suites = args.length ? args : ["trusted", "untrusted"];
// Short: VS Code puts a socket under the user-data directory.
const tmp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "rtsx-")));

await build({ test: true }); // the suites; and the extension, for a run without --vsix
let extension = root;
let binary;
if (vsix) {
  execFileSync("unzip", ["-q", vsix, "-d", path.join(tmp, "vsix")]);
  extension = path.join(tmp, "vsix/extension");
} else {
  // A private copy of the binary: the suites count its processes.
  binary = path.join(tmp, "bin", process.platform === "win32" ? "reactogenic.exe" : "reactogenic");
  fs.mkdirSync(path.dirname(binary));
  if (process.env.REACTOGENIC_BINARY) {
    fs.copyFileSync(process.env.REACTOGENIC_BINARY, binary);
    fs.chmodSync(binary, 0o755);
  } else {
    execFileSync("go", ["build", "-o", binary, "./cmd/reactogenic"], { cwd: path.resolve(root, "../../go"), stdio: "inherit" });
  }
}
const vscode = process.env.VSCODE_EXECUTABLE ?? (await downloadAndUnzipVSCode({ version: process.env.VSCODE_VERSION ?? "stable", cachePath: path.join(root, ".vscode-test") }));

// What a first launch would otherwise ask or show.
const settings = {
  "security.workspace.trust.startupPrompt": "never", // an unknown folder opens in restricted mode, silently
  "security.workspace.trust.banner": "never",
  "workbench.startupEditor": "none",
  "workbench.secondarySideBar.defaultVisibility": "hidden",
  "chat.disableAIFeatures": true,
  "telemetry.telemetryLevel": "off",
  "update.mode": "none",
  "extensions.autoUpdate": false,
  "git.enabled": false,
};
// A terminal inside VS Code passes its own identity on: the child must not take it.
const env = Object.fromEntries(
  Object.entries(process.env).filter(([name]) => name !== "ELECTRON_RUN_AS_NODE" && name !== "REACTOGENIC_BINARY" && !name.startsWith("VSCODE_")),
);
if (binary) {
  env.REACTOGENIC_BINARY = binary;
}

let failed = false;
for (const suite of suites) {
  const dir = path.join(tmp, suite);
  const workspace = path.join(dir, "ws");
  fs.cpSync(path.join(root, "test/fixture"), workspace, { recursive: true });
  fs.mkdirSync(path.join(dir, "user/User"), { recursive: true });
  fs.writeFileSync(path.join(dir, "user/User/settings.json"), JSON.stringify(settings, null, 2));
  const launch = [
    workspace,
    `--user-data-dir=${path.join(dir, "user")}`,
    `--extensions-dir=${path.join(dir, "extensions")}`,
    `--extensionDevelopmentPath=${extension}`,
    `--extensionTestsPath=${path.join(root, "dist/test", `${suite}.js`)}`,
    "--no-sandbox",
    "--disable-gpu-sandbox",
    "--disable-updates",
    "--skip-welcome",
    "--skip-release-notes",
    "--no-cached-data",
    // Trust is what the second suite is about: there the folder stays untrusted.
    ...(suite === "untrusted" ? [] : ["--disable-workspace-trust"]),
  ];
  console.log(`\n== ${suite}: ${vscode}\n   extension: ${extension}`);
  const code = await new Promise((resolve) => {
    const child = spawn(vscode, launch, { env, stdio: ["ignore", "pipe", "pipe"] });
    // stdout has the suite's report; stderr, the workbench's own chatter.
    child.stdout.pipe(process.stdout);
    child.stderr.pipe(process.env.EDITOR_TEST_VERBOSE ? process.stderr : fs.createWriteStream(path.join(dir, "stderr.log")));
    child.on("error", (error) => {
      console.error(String(error));
      resolve(1);
    });
    child.on("exit", (status, signal) => resolve(status ?? signal));
  });
  console.log(`== ${suite}: exit ${code}`);
  if (code !== 0) {
    failed = true;
    console.error(`   VS Code's stderr: ${path.join(dir, "stderr.log")}`);
  }
}
if (failed) {
  process.exit(1); // the profile and the workspace copy stay, to look at
}
fs.rmSync(tmp, { recursive: true, force: true });
