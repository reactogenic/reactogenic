// Which binary runs (ide.md, *Which binary runs*): the order of the table,
// the version gate, trust, the Windows copy. Real directories, no editor.
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterAll, describe, expect, test } from "vitest";
import {
  compareVersions,
  findWorkspaceCli,
  MIN_CLI_VERSION,
  parseVersion,
  resolveServer,
  runnable,
  statusOf,
  type ResolveInput,
} from "../src/resolve.ts";

const tmp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "rtsx-resolve-")));
afterAll(() => fs.rmSync(tmp, { recursive: true, force: true }));

let counter = 0;
/** A fresh directory with the given files; a value starting with `->` is a symlink to that path. */
function tree(files: Record<string, string>): string {
  const dir = path.join(tmp, String(counter++));
  for (const [name, text] of Object.entries(files)) {
    const file = path.join(dir, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    if (text.startsWith("->")) {
      fs.symlinkSync(path.join(dir, text.slice(2)), file);
    } else {
      fs.writeFileSync(file, text);
    }
  }
  return dir;
}

const cli = (version: string) => JSON.stringify({ name: "@reactogenic/cli", version });
/** npm's flat layout: the CLI and its platform package side by side. */
const npm = (version: string, platform = "darwin-arm64", exe = "reactogenic") => ({
  "ws/node_modules/@reactogenic/cli/package.json": cli(version),
  [`ws/node_modules/@reactogenic/cli-${platform}/bin/${exe}`]: "workspace binary",
});
const bundled = { "ext/server/reactogenic": "bundled binary" };

function resolve(dir: string, input: Partial<ResolveInput> = {}) {
  return resolveServer({
    trusted: true,
    extensionPath: path.join(dir, "ext"),
    documentDir: path.join(dir, "ws/src"),
    workspaceFolder: path.join(dir, "ws"),
    env: {},
    platform: "darwin",
    arch: "arm64",
    ...input,
  });
}

describe("the order", () => {
  const dir = tree({ ...npm(MIN_CLI_VERSION), ...bundled, "ws/src/page.rtsx": "", "explicit/reactogenic": "", "env/reactogenic": "" });
  const setting = path.join(dir, "explicit/reactogenic");
  const env = { REACTOGENIC_BINARY: path.join(dir, "env/reactogenic") };

  test("1: the setting, before everything", () => {
    expect(resolve(dir, { settingPath: setting, env })).toMatchObject({ kind: "binary", binary: { path: setting, source: "setting" } });
  });
  test("2: $REACTOGENIC_BINARY", () => {
    expect(resolve(dir, { env })).toMatchObject({ kind: "binary", binary: { path: env.REACTOGENIC_BINARY, source: "env" } });
  });
  test("3: the workspace's @reactogenic/cli", () => {
    expect(resolve(dir)).toEqual({
      kind: "binary",
      binary: { path: path.join(dir, "ws/node_modules/@reactogenic/cli-darwin-arm64/bin/reactogenic"), source: "workspace", version: MIN_CLI_VERSION },
      skipped: [],
    });
  });
  test("4: the bundled binary", () => {
    const resolution = resolve(dir, { documentDir: path.join(dir, "ext") });
    expect(resolution).toMatchObject({ kind: "binary", binary: { path: path.join(dir, "ext/server/reactogenic"), source: "bundled" } });
    expect(resolution).toMatchObject({ skipped: [{ source: "workspace", reason: "@reactogenic/cli is not installed" }] });
  });
  test("a setting relative to the workspace folder, and ~", () => {
    expect(resolve(dir, { settingPath: "../explicit/reactogenic" })).toMatchObject({ kind: "binary", binary: { path: setting } });
    expect(resolve(dir, { settingPath: "~/no-such-reactogenic" })).toMatchObject({
      kind: "error",
      message: `reactogenic.server.path: ${path.join(os.homedir(), "no-such-reactogenic")} does not exist`,
    });
  });
});

describe("an explicit path that does not exist is an error, not a fall-through", () => {
  const dir = tree({ ...npm(MIN_CLI_VERSION), ...bundled, "ws/src/page.rtsx": "" });
  const missing = path.join(dir, "nowhere/reactogenic");

  test("the setting", () => {
    expect(resolve(dir, { settingPath: missing })).toEqual({ kind: "error", message: `reactogenic.server.path: ${missing} does not exist`, skipped: [] });
  });
  test("$REACTOGENIC_BINARY", () => {
    expect(resolve(dir, { env: { REACTOGENIC_BINARY: missing } })).toEqual({ kind: "error", message: `$REACTOGENIC_BINARY: ${missing} does not exist`, skipped: [] });
  });
  test("a directory is not a binary", () => {
    expect(resolve(dir, { settingPath: path.join(dir, "ext") })).toMatchObject({ kind: "error" });
  });
  test("its status is an error", () => {
    expect(statusOf(resolve(dir, { settingPath: missing }))).toMatchObject({ severity: "error", detail: expect.stringContaining(missing) });
  });
});

describe("the workspace CLI", () => {
  test("is found by walking up from the document's folder, through pnpm's links", () => {
    const store = "ws/node_modules/.pnpm/@reactogenic+cli@0.2.0/node_modules/@reactogenic";
    const dir = tree({
      [`${store}/cli/package.json`]: cli("0.2.0"),
      "ws/node_modules/.pnpm/@reactogenic+cli-linux-x64@0.2.0/node_modules/@reactogenic/cli-linux-x64/bin/reactogenic": "",
      [`${store}/cli-linux-x64`]: "->ws/node_modules/.pnpm/@reactogenic+cli-linux-x64@0.2.0/node_modules/@reactogenic/cli-linux-x64",
      "ws/packages/app/node_modules/@reactogenic/cli": `->${store}/cli`,
      "ws/packages/app/src/pages/page.rtsx": "",
      ...bundled,
    });
    const binary = path.join(dir, "ws/node_modules/.pnpm/@reactogenic+cli-linux-x64@0.2.0/node_modules/@reactogenic/cli-linux-x64/bin/reactogenic");
    const from = (documentDir: string) => resolve(dir, { documentDir: path.join(dir, documentDir), platform: "linux", arch: "x64" });

    expect(from("ws/packages/app/src/pages")).toMatchObject({ kind: "binary", binary: { path: binary, source: "workspace", version: "0.2.0" } });
    // Only packages/app depends on the CLI: the monorepo's root, and another package, have none.
    expect(from("ws")).toMatchObject({ kind: "binary", binary: { source: "bundled" } });
    expect(from("ws/packages/other/src")).toMatchObject({ kind: "binary", binary: { source: "bundled" } });
  });

  test("the nearest one wins", () => {
    const dir = tree({
      ...npm("0.3.0"),
      "ws/packages/app/node_modules/@reactogenic/cli/package.json": cli("0.2.0"),
      "ws/packages/app/node_modules/@reactogenic/cli-darwin-arm64/bin/reactogenic": "",
    });
    expect(findWorkspaceCli(path.join(dir, "ws/packages/app/src"), "darwin", "arm64")?.version).toBe("0.2.0");
    expect(findWorkspaceCli(path.join(dir, "ws/packages"), "darwin", "arm64")?.version).toBe("0.3.0");
  });

  test("too old: skipped for the bundled binary, with a warning", () => {
    const dir = tree({ ...npm("0.1.0-alpha.0"), ...bundled });
    const resolution = resolve(dir);
    expect(resolution).toEqual({
      kind: "binary",
      binary: { path: path.join(dir, "ext/server/reactogenic"), source: "bundled" },
      skipped: [{ source: "workspace", reason: `@reactogenic/cli 0.1.0-alpha.0 has no language server (needs >= ${MIN_CLI_VERSION})`, tooOld: "0.1.0-alpha.0" }],
    });
    const status = statusOf(resolution, "0.1.3");
    expect(status.severity).toBe("warning");
    expect(status.text).toBe("reactogenic 0.1.3");
    expect(status.detail).toContain("bundled: ");
    expect(status.detail).toContain("@reactogenic/cli 0.1.0-alpha.0 has no language server");
    expect(status.detail).toContain("newer transpiler than this project's reactogenic check");
  });

  test("the minimum is compared numerically: alpha.10 is not older than alpha.9", () => {
    const dir = tree({ ...npm("0.1.0-alpha.10"), ...bundled });
    expect(resolve(dir, { minVersion: "0.1.0-alpha.9" })).toMatchObject({ binary: { source: "workspace", version: "0.1.0-alpha.10" } });
    expect(resolve(dir, { minVersion: "0.1.0-alpha.11" })).toMatchObject({ binary: { source: "bundled" }, skipped: [{ tooOld: "0.1.0-alpha.10" }] });
  });

  test("without its platform package: skipped, no warning", () => {
    const dir = tree({ ...npm(MIN_CLI_VERSION, "linux-x64"), ...bundled });
    const resolution = resolve(dir);
    expect(resolution).toMatchObject({ binary: { source: "bundled" }, skipped: [{ reason: expect.stringContaining("@reactogenic/cli-darwin-arm64 is not installed") }] });
    expect(statusOf(resolution).severity).toBe("info");
  });

  test("an unreadable manifest or version", () => {
    const dir = tree({ "ws/node_modules/@reactogenic/cli/package.json": "{", "ws/x/node_modules/@reactogenic/cli/package.json": "{}", ...bundled });
    expect(resolve(dir)).toMatchObject({ binary: { source: "bundled" }, skipped: [{ reason: "@reactogenic/cli is not installed" }] });
    expect(resolve(dir, { documentDir: path.join(dir, "ws/x") })).toMatchObject({ binary: { source: "bundled" }, skipped: [{ reason: expect.stringContaining("unreadable version") }] });
  });

  test("this repository's own install", () => {
    const repo = path.resolve(import.meta.dirname, "../../..");
    const found = findWorkspaceCli(path.join(repo, "packages/vite/src"));
    expect(found?.dir).toBe(path.join(repo, "packages/cli"));
    expect(found?.version).toBe(JSON.parse(fs.readFileSync(path.join(repo, "packages/cli/package.json"), "utf8")).version);
    const platformPackage = path.join(repo, `packages/cli/node_modules/@reactogenic/cli-${process.platform}-${process.arch}`);
    if (fs.existsSync(platformPackage)) {
      expect(found?.binary).toBe(fs.realpathSync(path.join(platformPackage, "bin", process.platform === "win32" ? "reactogenic.exe" : "reactogenic")));
    }
  });
});

describe("nothing to run", () => {
  test("the universal .vsix in a project without the CLI", () => {
    const dir = tree({ "ws/src/page.rtsx": "", "ext/package.json": "{}" });
    const resolution = resolve(dir, { platform: "linux", arch: "riscv64" });
    // Not an error: this build is for highlighting, until a CLI is installed.
    expect(resolution).toMatchObject({ kind: "none", skipped: [{ source: "workspace" }, { source: "bundled" }] });
    expect(resolution.kind === "none" && resolution.message).toContain("no binary for linux-riscv64");
    expect(statusOf(resolution)).toMatchObject({ severity: "warning", text: "reactogenic: no server" });
  });
  test("no document folder: the workspace row is skipped", () => {
    const dir = tree({ ...npm(MIN_CLI_VERSION), ...bundled });
    expect(resolve(dir, { documentDir: undefined })).toMatchObject({ binary: { source: "bundled" } });
  });
});

describe("trust", () => {
  test("an untrusted workspace resolves nothing, whatever is there", () => {
    const dir = tree({ ...npm(MIN_CLI_VERSION), ...bundled, "explicit/reactogenic": "" });
    const explicit = path.join(dir, "explicit/reactogenic");
    for (const input of [{}, { settingPath: explicit }, { env: { REACTOGENIC_BINARY: explicit } }, { settingPath: path.join(dir, "missing") }]) {
      expect(resolve(dir, { ...input, trusted: false })).toEqual({ kind: "untrusted" });
    }
    expect(statusOf({ kind: "untrusted" })).toEqual({ severity: "info", text: "reactogenic: off", detail: "Untrusted workspace: syntax highlighting only" });
  });
});

describe("the status item", () => {
  test("names the binary, its version and where it was found", () => {
    const dir = tree({ ...npm("0.2.0"), ...bundled });
    const binary = path.join(dir, "ws/node_modules/@reactogenic/cli-darwin-arm64/bin/reactogenic");
    // The server's own version wins over the package's.
    expect(statusOf(resolve(dir), "0.2.0+build")).toEqual({ severity: "info", text: "reactogenic 0.2.0+build", detail: `workspace: ${binary}` });
    expect(statusOf(resolve(dir))).toMatchObject({ text: "reactogenic 0.2.0" });
    expect(statusOf(resolve(dir, { env: { REACTOGENIC_BINARY: binary } }), "1.0.0").detail).toBe(`$REACTOGENIC_BINARY: ${binary}`);
    expect(statusOf(resolve(dir, { settingPath: binary })).detail).toBe(`reactogenic.server.path: ${binary}`);
  });
});

describe("Windows", () => {
  const dir = tree({ ...npm("0.2.0", "win32-x64", "reactogenic.exe"), "ext/server/reactogenic.exe": "bundled" });
  const copies = path.join(dir, "storage/server");
  const win = (input: Partial<ResolveInput> = {}) => resolve(dir, { platform: "win32", arch: "x64", ...input });

  test("the binary is reactogenic.exe", () => {
    expect(win()).toMatchObject({ binary: { path: path.join(dir, "ws/node_modules/@reactogenic/cli-win32-x64/bin/reactogenic.exe"), source: "workspace" } });
    expect(win({ documentDir: undefined })).toMatchObject({ binary: { path: path.join(dir, "ext/server/reactogenic.exe"), source: "bundled" } });
  });

  test("a workspace binary runs from a copy, so that an install can replace it", () => {
    const resolution = win();
    if (resolution.kind !== "binary") {
      throw new Error(resolution.kind);
    }
    const { binary } = resolution;
    const copy = runnable(binary, copies, "win32");
    expect(copy).not.toBe(binary.path);
    expect(copy.startsWith(copies + path.sep)).toBe(true);
    expect(path.basename(copy)).toBe("reactogenic.exe");
    expect(fs.readFileSync(copy, "utf8")).toBe("workspace binary");
    // The same binary: the same copy.
    expect(runnable(binary, copies, "win32")).toBe(copy);
    // `pnpm install` replaced it: a new copy, the old one removed.
    fs.writeFileSync(binary.path, "workspace binary, upgraded");
    const next = runnable(binary, copies, "win32");
    expect(next).not.toBe(copy);
    expect(fs.readFileSync(next, "utf8")).toBe("workspace binary, upgraded");
    expect(fs.existsSync(copy)).toBe(false);
    expect(fs.readdirSync(copies)).toHaveLength(1);
  });

  test("other binaries, and other platforms, run in place", () => {
    const bundledBinary = { path: path.join(dir, "ext/server/reactogenic.exe"), source: "bundled" as const };
    expect(runnable(bundledBinary, copies, "win32")).toBe(bundledBinary.path);
    expect(runnable({ ...bundledBinary, source: "setting" }, copies, "win32")).toBe(bundledBinary.path);
    expect(runnable({ ...bundledBinary, source: "workspace" }, copies, "darwin")).toBe(bundledBinary.path);
  });
});

describe("versions", () => {
  test.each([
    ["0.1.0-alpha.10", "0.1.0-alpha.9", 1], // numerically, not as strings
    ["0.1.0-alpha.9", "0.1.0-alpha.10", -1],
    ["0.1.0-alpha.1", "0.1.0-alpha.1", 0],
    ["0.1.0-alpha.1", "0.1.0", -1], // a pre-release is before its release
    ["0.1.0", "0.1.0-rc.1", 1],
    ["0.1.0", "0.1.1", -1],
    ["0.10.0", "0.9.0", 1],
    ["1.0.0", "0.99.99", 1],
    ["0.2.0-alpha.0", "0.1.0", 1],
    ["0.1.0-alpha.1", "0.1.0-beta.0", -1], // words compare as text
    ["0.1.0-alpha", "0.1.0-alpha.1", -1], // a shorter list is first
    ["0.1.0-alpha.1", "0.1.0-alpha.beta", -1], // a number is before a word
    ["0.1.0-2", "0.1.0-10", -1],
    ["v0.1.0", "0.1.0", 0],
    ["0.1.0+build.5", "0.1.0", 0], // build metadata does not count
    ["0.1.0-alpha.1+build", "0.1.0-alpha.1", 0],
    ["latest", "0.0.1", -1], // unreadable sorts first
    ["0.1", "0.0.1", -1],
    ["", "", 0],
  ])("%s vs %s → %i", (a, b, want) => {
    expect(Math.sign(compareVersions(a, b))).toBe(want);
    expect(Math.sign(compareVersions(b, a))).toBe(-want || 0);
  });

  test("the minimum is the first CLI with `lsp`: after the published 0.1.0-alpha.0", () => {
    expect(parseVersion(MIN_CLI_VERSION)).toBeDefined();
    expect(compareVersions("0.1.0-alpha.0", MIN_CLI_VERSION)).toBeLessThan(0);
    expect(compareVersions("0.1.0", MIN_CLI_VERSION)).toBeGreaterThan(0);
  });
});
