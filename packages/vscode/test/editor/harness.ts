// Shared by the editor suites (run inside VS Code by scripts/test-editor.mjs).
import { execFileSync } from "node:child_process";
import * as path from "node:path";
import Mocha from "mocha";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";
import type { ServerState } from "../../src/server";

/**
 * The binary under test, and the row of ide.md's table that finds it:
 * $REACTOGENIC_BINARY (a private copy, so that `ps` finds this window's
 * servers only), or the one bundled in the .vsix under test.
 */
export function binary(): { path: string; source: "env" | "bundled" } {
  if (process.env.REACTOGENIC_BINARY) {
    return { path: process.env.REACTOGENIC_BINARY, source: "env" };
  }
  const extension = vscode.extensions.getExtension("reactogenic.rtsx");
  return { path: path.join(extension?.extensionPath ?? "", "server", process.platform === "win32" ? "reactogenic.exe" : "reactogenic"), source: "bundled" };
}

/** What VS Code calls with `--extensionTestsPath`: runs the tests `define` declares. */
export function runner(define: () => void): () => Promise<void> {
  return () => {
    const mocha = new Mocha({ ui: "bdd", timeout: 60_000, color: true, reporter: "spec" });
    if (process.env.EDITOR_TEST_GREP) {
      mocha.grep(new RegExp(process.env.EDITOR_TEST_GREP));
    }
    mocha.suite.emit("pre-require", globalThis, "", mocha);
    define();
    return new Promise((resolve, reject) => {
      mocha.run((failures) => (failures ? reject(new Error(`${failures} editor test(s) failed`)) : resolve()));
    });
  };
}

export const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** Polls until `read` returns something truthy: the server answers when its project has loaded. */
export async function until<T>(what: string, read: () => T | undefined | null | false | Thenable<T | undefined | null | false>, timeout = 30_000): Promise<T> {
  const end = Date.now() + timeout;
  for (;;) {
    const value = await read();
    if (value) {
      return value;
    }
    if (Date.now() > end) {
      throw new Error(`timed out waiting for ${what}`);
    }
    await sleep(100);
  }
}

/**
 * The server's state once `is` holds: a restart is queued a moment after its
 * cause. A timeout says what the state was.
 */
export function serverState(api: Api, what: string, is: (s: ServerState) => unknown, timeout?: number): Promise<ServerState> {
  let last: ServerState | undefined;
  return until<ServerState>(
    what,
    async () => {
      last = await api.ready();
      return is(last) ? last : undefined;
    },
    timeout,
  ).catch((error: Error) => {
    throw new Error(`${error.message}; the state is ${JSON.stringify(last)}`);
  });
}

export function file(name: string): vscode.Uri {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    throw new Error("no workspace folder");
  }
  return vscode.Uri.joinPath(folder.uri, name);
}

export async function open(name: string): Promise<vscode.TextEditor> {
  return vscode.window.showTextDocument(await vscode.workspace.openTextDocument(file(name)), { preview: false });
}

export async function activate(): Promise<Api> {
  const extension = vscode.extensions.getExtension<Api>("reactogenic.rtsx");
  if (!extension) {
    throw new Error("the extension reactogenic.rtsx is not loaded");
  }
  return extension.activate();
}

/** The position of `needle` in the document, plus `offset` characters. */
export function at(document: vscode.TextDocument, needle: string, offset = 0): vscode.Position {
  const index = document.getText().indexOf(needle);
  if (index < 0) {
    throw new Error(`${path.basename(document.fileName)} has no ${JSON.stringify(needle)}`);
  }
  return document.positionAt(index + offset);
}

/**
 * Waits until the OS shows exactly these processes of the binary, each run as
 * `<binary> lsp --stdio` to the letter; nothing where there is no `ps`.
 */
export async function onlyServers(pids: (number | undefined)[], of: string = binary().path): Promise<void> {
  if (!processes(of)) {
    return;
  }
  const want = pids.map((pid) => `${pid} ${of} lsp --stdio`);
  let seen: string[] = [];
  try {
    await until("the server processes", () => {
      seen = (processes(of) ?? []).map((p) => `${p.pid} ${p.command}`);
      return JSON.stringify(seen) === JSON.stringify(want);
    });
  } catch {
    throw new Error(`the server processes are ${JSON.stringify(seen)}, not ${JSON.stringify(want)}`);
  }
}

/** The processes whose command line starts with `command`, from the OS; undefined where there is no `ps`. */
export function processes(command: string): { pid: number; command: string }[] | undefined {
  if (process.platform === "win32") {
    return undefined;
  }
  return execFileSync("ps", ["-axww", "-o", "pid=,command="], { encoding: "utf8" })
    .split("\n")
    .flatMap((line) => {
      const m = /^\s*(\d+)\s+(.*)$/.exec(line);
      return m && (m[2] === command || m[2].startsWith(`${command} `)) ? [{ pid: Number(m[1]), command: m[2] }] : [];
    });
}

/** The folder of this suite's window: its workspace is in it (scripts/test-editor.mjs). */
export function suiteDir(): string {
  if (!process.env.RTSX_TEST_DIR) {
    throw new Error("$RTSX_TEST_DIR is not set: run the suite through scripts/test-editor.mjs");
  }
  return process.env.RTSX_TEST_DIR;
}

export const code = (d: vscode.Diagnostic) => (typeof d.code === "object" ? d.code.value : d.code);
export const where = (d: vscode.Diagnostic) => `${d.range.start.line}:${d.range.start.character}`;

/** Runs a task and returns its exit code. */
export async function runTask(task: vscode.Task): Promise<number | undefined> {
  const ended = new Promise<number | undefined>((resolve) => {
    const subscription = vscode.tasks.onDidEndTaskProcess((e) => {
      if (e.execution.task.name === task.name) {
        subscription.dispose();
        resolve(e.exitCode);
      }
    });
  });
  await vscode.tasks.executeTask(task);
  return ended;
}
