// Which `reactogenic` binary runs the language server (ide.md, *Which binary
// runs*). No `vscode` import: unit-tested in plain Node (test/resolve.test.ts).
import { createHash } from "node:crypto";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

/** The first `@reactogenic/cli` whose binary has `reactogenic lsp`. */
export const MIN_CLI_VERSION = "0.1.0-alpha.1";

/** A change to one of these restarts the server: the workspace CLI may be another one now. */
export const LOCKFILES = ["package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"];

export type Source = "setting" | "env" | "workspace" | "bundled";

export interface Binary {
  path: string;
  source: Source;
  /** The version of the npm package it came from (workspace only); the server reports its own. */
  version?: string;
}

/** A row of the table that was looked at and not used. */
export interface Skipped {
  source: Source;
  reason: string;
  /** The workspace CLI is older than the minimum: the editor and `reactogenic check` now differ. */
  tooOld?: string;
}

export type Resolution =
  | { kind: "binary"; binary: Binary; skipped: Skipped[] }
  /** An untrusted workspace: highlighting only, no process. */
  | { kind: "untrusted" }
  /** No row matched (the universal .vsix in a project without the CLI): highlighting only. */
  | { kind: "none"; message: string; skipped: Skipped[] }
  /** An explicit path that does not exist. */
  | { kind: "error"; message: string; skipped: Skipped[] };

export interface ResolveInput {
  /** `vscode.workspace.isTrusted`. */
  trusted: boolean;
  /** The setting `reactogenic.server.path`. */
  settingPath?: string | null;
  /** Relative setting paths resolve against it. */
  workspaceFolder?: string;
  /** The folder of the first `.rtsx` document opened: the walk to `node_modules` starts here. */
  documentDir?: string;
  /** The extension's folder: the bundled binary is `server/reactogenic[.exe]` in it. */
  extensionPath: string;
  env?: Record<string, string | undefined>;
  platform?: string;
  arch?: string;
  minVersion?: string;
}

/** The table of ide.md, first match. Reads the file system; runs nothing. */
export function resolveServer(input: ResolveInput): Resolution {
  if (!input.trusted) {
    return { kind: "untrusted" };
  }
  const platform = input.platform ?? process.platform;
  const arch = input.arch ?? process.arch;
  const env = input.env ?? process.env;
  const min = input.minVersion ?? MIN_CLI_VERSION;
  const skipped: Skipped[] = [];

  // 1 and 2 are explicit: a path that does not exist is an error, not a fall-through.
  if (input.settingPath) {
    const file = absolute(input.settingPath, input.workspaceFolder);
    return isFile(file)
      ? { kind: "binary", binary: { path: file, source: "setting" }, skipped }
      : { kind: "error", message: `reactogenic.server.path: ${file} does not exist`, skipped };
  }
  if (env.REACTOGENIC_BINARY) {
    const file = absolute(env.REACTOGENIC_BINARY, input.workspaceFolder);
    return isFile(file)
      ? { kind: "binary", binary: { path: file, source: "env" }, skipped }
      : { kind: "error", message: `$REACTOGENIC_BINARY: ${file} does not exist`, skipped };
  }

  // 3: the workspace's own CLI, so that the editor and `reactogenic check` agree.
  const cli = input.documentDir ? findWorkspaceCli(input.documentDir, platform, arch) : undefined;
  if (cli) {
    if (!parseVersion(cli.version)) {
      skipped.push({ source: "workspace", reason: `${cli.dir}: unreadable version ${JSON.stringify(cli.version)}` });
    } else if (compareVersions(cli.version, min) < 0) {
      skipped.push({
        source: "workspace",
        reason: `@reactogenic/cli ${cli.version} has no language server (needs >= ${min})`,
        tooOld: cli.version,
      });
    } else if (!cli.binary) {
      skipped.push({
        source: "workspace",
        reason: `@reactogenic/cli ${cli.version}: @reactogenic/cli-${platform}-${arch} is not installed (optional dependencies skipped?)`,
      });
    } else {
      return { kind: "binary", binary: { path: cli.binary, source: "workspace", version: cli.version }, skipped };
    }
  } else {
    skipped.push({ source: "workspace", reason: "@reactogenic/cli is not installed" });
  }

  // 4: the binary in the .vsix.
  const bundled = path.join(input.extensionPath, "server", executable(platform));
  if (isFile(bundled)) {
    return { kind: "binary", binary: { path: bundled, source: "bundled" }, skipped };
  }
  skipped.push({ source: "bundled", reason: `this build of the extension has no binary for ${platform}-${arch}` });
  return {
    kind: "none",
    message: `No reactogenic binary: ${skipped.map((s) => s.reason).join("; ")}. Install @reactogenic/cli >= ${min}, or set reactogenic.server.path.`,
    skipped,
  };
}

export interface WorkspaceCli {
  /** The real directory of `@reactogenic/cli`. */
  dir: string;
  version: string;
  /** The platform package's binary, when installed. */
  binary?: string;
}

/**
 * The nearest `node_modules/@reactogenic/cli` at or above `startDir`, and the
 * binary of its platform package, looked up from the CLI's real directory as
 * Node would: beside it in npm's flat layout, beside its target in pnpm's
 * store. Read with `fs` — nothing of the workspace is loaded.
 */
export function findWorkspaceCli(startDir: string, platform: string = process.platform, arch: string = process.arch): WorkspaceCli | undefined {
  for (const dir of ancestors(startDir)) {
    const manifest = path.join(dir, "node_modules", "@reactogenic", "cli", "package.json");
    let version: unknown;
    try {
      version = JSON.parse(fs.readFileSync(manifest, "utf8")).version;
    } catch {
      continue;
    }
    const cli: WorkspaceCli = { dir: fs.realpathSync(path.dirname(manifest)), version: typeof version === "string" ? version : "" };
    for (const from of ancestors(cli.dir)) {
      if (path.basename(from) === "node_modules") {
        continue;
      }
      const binary = path.join(from, "node_modules", "@reactogenic", `cli-${platform}-${arch}`, "bin", executable(platform));
      if (isFile(binary)) {
        cli.binary = fs.realpathSync(binary);
        break;
      }
    }
    return cli;
  }
  return undefined;
}

/**
 * On Windows a running binary is locked: a workspace binary runs from a copy,
 * so that `pnpm install` can replace it. Copies of other binaries are removed
 * (one still running in another window stays: its removal fails).
 */
export function runnable(binary: Binary, copyRoot: string, platform: string = process.platform): string {
  if (platform !== "win32" || binary.source !== "workspace") {
    return binary.path;
  }
  const stat = fs.statSync(binary.path);
  const key = createHash("sha256").update(`${binary.path}\0${stat.size}\0${stat.mtimeMs}`).digest("hex").slice(0, 16);
  const copy = path.join(copyRoot, key, path.basename(binary.path));
  if (!isFile(copy)) {
    fs.mkdirSync(path.dirname(copy), { recursive: true });
    fs.copyFileSync(binary.path, copy);
  }
  for (const other of fs.readdirSync(copyRoot)) {
    if (other !== key) {
      try {
        fs.rmSync(path.join(copyRoot, other), { recursive: true, force: true });
      } catch {}
    }
  }
  return copy;
}

export interface Status {
  severity: "info" | "warning" | "error";
  /** The binary and its version. */
  text: string;
  /** Where it was found; the warning. */
  detail: string;
}

/** What the status item says: the binary, its version, where it was found. */
export function statusOf(resolution: Resolution, serverVersion?: string): Status {
  switch (resolution.kind) {
    case "untrusted":
      return { severity: "info", text: "reactogenic: off", detail: "Untrusted workspace: syntax highlighting only" };
    case "none":
      return { severity: "warning", text: "reactogenic: no server", detail: resolution.message };
    case "error":
      return { severity: "error", text: "reactogenic: no server", detail: resolution.message };
    case "binary": {
      const { binary, skipped } = resolution;
      const version = serverVersion ?? binary.version;
      const text = version ? `reactogenic ${version}` : "reactogenic";
      const where = `${WHERE[binary.source]}: ${binary.path}`;
      const old = skipped.find((s) => s.tooOld);
      return old
        ? {
            severity: "warning",
            text,
            detail: `${where}. The workspace's ${old.reason}: the editor runs a newer transpiler than this project's reactogenic check.`,
          }
        : { severity: "info", text, detail: where };
    }
  }
}

const WHERE: Record<Source, string> = {
  setting: "reactogenic.server.path",
  env: "$REACTOGENIC_BINARY",
  workspace: "workspace",
  bundled: "bundled",
};

interface Version {
  core: number[];
  pre: string[];
}

/** `1.2.3`, `1.2.3-alpha.10`, with an optional `v` and build metadata; else undefined. */
export function parseVersion(version: string): Version | undefined {
  const m = /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z.-]+)?$/.exec(version.trim());
  return m ? { core: [Number(m[1]), Number(m[2]), Number(m[3])], pre: m[4] ? m[4].split(".") : [] } : undefined;
}

/**
 * Semver precedence: a pre-release sorts before its release, and pre-release
 * identifiers compare one by one — numerically when both are numbers
 * (`alpha.10` > `alpha.9`), a number before a word, a shorter list first.
 * A version that does not parse sorts first.
 */
export function compareVersions(a: string, b: string): number {
  const x = parseVersion(a);
  const y = parseVersion(b);
  if (!x || !y) {
    return Number(Boolean(x)) - Number(Boolean(y));
  }
  for (let i = 0; i < 3; i++) {
    if (x.core[i] !== y.core[i]) {
      return x.core[i] < y.core[i] ? -1 : 1;
    }
  }
  if (!x.pre.length || !y.pre.length) {
    return x.pre.length === y.pre.length ? 0 : x.pre.length ? -1 : 1;
  }
  for (let i = 0; i < Math.max(x.pre.length, y.pre.length); i++) {
    const p = x.pre[i];
    const q = y.pre[i];
    if (p === undefined || q === undefined) {
      return p === undefined ? -1 : 1;
    }
    if (p === q) {
      continue;
    }
    const pn = /^\d+$/.test(p);
    const qn = /^\d+$/.test(q);
    if (pn && qn) {
      return Number(p) < Number(q) ? -1 : 1;
    }
    if (pn !== qn) {
      return pn ? -1 : 1;
    }
    return p < q ? -1 : 1;
  }
  return 0;
}

function executable(platform: string): string {
  return platform === "win32" ? "reactogenic.exe" : "reactogenic";
}

function absolute(file: string, base: string | undefined): string {
  if (file === "~" || file.startsWith("~/")) {
    return path.join(os.homedir(), file.slice(1));
  }
  return path.isAbsolute(file) || !base ? file : path.resolve(base, file);
}

function isFile(file: string): boolean {
  try {
    return fs.statSync(file).isFile();
  } catch {
    return false;
  }
}

/** `dir`, its parent, … up to the root. */
function* ancestors(dir: string): Generator<string> {
  for (let d = path.resolve(dir); ; d = path.dirname(d)) {
    yield d;
    if (d === path.dirname(d)) {
      return;
    }
  }
}
