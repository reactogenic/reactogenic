// The editor suite (`pnpm test:editor`): the extension in a real VS Code with
// an isolated profile, against a copy of test/fixture. Four windows, one after
// the other:
//
//   trusted     test/editor/trusted.ts     the server and the client's features
//   untrusted   test/editor/untrusted.ts   highlighting only, no server process
//   monorepo    test/editor/monorepo.ts    the workspace's own CLI: which one, and its lockfile
//   transpiled  test/editor/transpiled.ts  Show Transpiled TSX, against test/editor/fake-server.mjs
//
//   node scripts/test-editor.mjs [--vsix file.vsix] [trusted|untrusted|monorepo|transpiled ...]
//
// Without --vsix it tests this checkout: the extension from this folder, and
// $REACTOGENIC_BINARY — unset: built from ../../go. With --vsix it tests a
// package as it ships: the unpacked .vsix and the binary bundled in it.
//
//   $VSCODE_EXECUTABLE    the VS Code to run; unset: downloaded into .vscode-test
//   $VSCODE_VERSION       the version to download ("stable")
//   $EDITOR_TEST_VERBOSE  show VS Code's stderr instead of keeping it in a file
//   $EDITOR_TEST_GREP     run only the tests whose title matches this regular expression
//
// It opens windows: on Linux without a display, run it under `xvfb-run -a`.
import { execFileSync, spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { downloadAndUnzipVSCode } from "@vscode/test-electron";
import { build, root } from "./build.mjs";

const SUITES = ["trusted", "untrusted", "monorepo", "transpiled"];
const args = process.argv.slice(2);
const vsix = args.includes("--vsix") ? path.resolve(args.splice(args.indexOf("--vsix"), 2)[1]) : undefined;
const suites = args.length ? args : SUITES;
for (const suite of suites) {
  if (!SUITES.includes(suite)) {
    throw new Error(`unknown suite ${suite}: one of ${SUITES.join(", ")}`);
  }
}
// Short: VS Code puts a socket under the user-data directory.
const tmp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "rtsx-")));

await build({ test: true }); // the suites; and the extension, for a run without --vsix
const exe = process.platform === "win32" ? "reactogenic.exe" : "reactogenic";
let extension = root;
let binary;
if (vsix) {
  execFileSync("unzip", ["-q", vsix, "-d", path.join(tmp, "vsix")]);
  extension = path.join(tmp, "vsix/extension");
} else {
  // A private copy of the binary: the suites count its processes.
  binary = path.join(tmp, "bin", exe);
  fs.mkdirSync(path.dirname(binary));
  if (process.env.REACTOGENIC_BINARY) {
    fs.copyFileSync(process.env.REACTOGENIC_BINARY, binary);
    fs.chmodSync(binary, 0o755);
  } else {
    execFileSync("go", ["build", "-o", binary, "./cmd/reactogenic"], { cwd: path.resolve(root, "../../go"), stdio: "inherit" });
  }
}
// The real binary, for the suite that installs it as a workspace's CLI.
const real = binary ?? path.join(extension, "server", exe);
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

/** `@reactogenic/cli` and its platform package in root/node_modules, as npm lays them out. */
function installCli(root) {
  const scope = path.join(root, "node_modules/@reactogenic");
  fs.mkdirSync(path.join(scope, "cli"), { recursive: true });
  fs.writeFileSync(path.join(scope, "cli/package.json"), JSON.stringify({ name: "@reactogenic/cli", version: "9.9.9" }));
  const bin = path.join(scope, `cli-${process.platform}-${process.arch}`, "bin");
  fs.mkdirSync(bin, { recursive: true });
  try {
    fs.linkSync(real, path.join(bin, exe)); // the binary is 40 MB: one file under several names
  } catch {
    fs.copyFileSync(real, path.join(bin, exe));
  }
}

/** The folder a suite's window opens, and the environment of that window. */
function prepare(suite, dir) {
  const fixture = path.join(root, "test/fixture");
  const suiteEnv = { ...env, RTSX_TEST_DIR: dir };
  if (suite === "monorepo") {
    // The opened folder is a package of a repository: the CLI and the
    // lockfile are above it. A nested package has a CLI of its own, and so
    // has a folder outside the workspace. Row 3 of ide.md's table decides
    // here: no $REACTOGENIC_BINARY.
    if (!fs.existsSync(real)) {
      return { skipped: `${real} does not exist (a universal .vsix has no binary)` };
    }
    const workspace = path.join(dir, "repo/packages/app");
    fs.cpSync(fixture, workspace, { recursive: true });
    fs.cpSync(fixture, path.join(workspace, "nested"), { recursive: true });
    fs.cpSync(fixture, path.join(dir, "elsewhere"), { recursive: true });
    installCli(path.join(dir, "repo"));
    installCli(path.join(workspace, "nested"));
    installCli(path.join(dir, "elsewhere"));
    fs.writeFileSync(path.join(dir, "repo/pnpm-lock.yaml"), "lockfileVersion: '9.0'\n");
    delete suiteEnv.REACTOGENIC_BINARY;
    return { workspace, suiteEnv };
  }
  const workspace = path.join(dir, "ws");
  fs.cpSync(fixture, workspace, { recursive: true });
  if (suite === "transpiled") {
    if (process.platform === "win32") {
      return { skipped: "the stand-in server is started by a shell script" };
    }
    const fake = path.join(dir, "reactogenic");
    fs.writeFileSync(fake, `#!/bin/sh\nexec "${process.execPath}" "${path.join(root, "test/editor/fake-server.mjs")}" "$@"\n`, { mode: 0o755 });
    suiteEnv.REACTOGENIC_BINARY = fake;
  }
  return { workspace, suiteEnv };
}

let failed = false;
for (const suite of suites) {
  const dir = path.join(tmp, suite);
  const { workspace, suiteEnv, skipped } = prepare(suite, dir);
  if (skipped) {
    console.log(`\n== ${suite}: skipped — ${skipped}`);
    continue;
  }
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
    const child = spawn(vscode, launch, { env: suiteEnv, stdio: ["ignore", "pipe", "pipe"] });
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
