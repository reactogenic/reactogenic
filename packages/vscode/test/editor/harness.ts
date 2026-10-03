// Shared by the editor suites (run inside VS Code by scripts/test-editor.mjs).
import { execFileSync } from "node:child_process";
import * as path from "node:path";
import Mocha from "mocha";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";

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

/** Waits until the OS shows exactly these server processes; nothing where there is no `ps`. */
export async function onlyServers(pids: (number | undefined)[]): Promise<void> {
  if (servers()) {
    await until(`the server processes to be ${JSON.stringify(pids)}`, () => JSON.stringify(servers()) === JSON.stringify(pids));
  }
}

/** The pids of the `lsp --stdio` processes of the binary, from the OS; undefined where there is no `ps`. */
export function servers(): number[] | undefined {
  if (process.platform === "win32") {
    return undefined;
  }
  return execFileSync("ps", ["-axww", "-o", "pid=,command="], { encoding: "utf8" })
    .split("\n")
    .filter((line) => line.includes(`${binary().path} lsp --stdio`))
    .map((line) => Number(line.trim().split(/\s+/)[0]));
}
