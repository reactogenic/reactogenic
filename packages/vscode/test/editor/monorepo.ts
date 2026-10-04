// The editor suite, third window: the workspace's own CLI (ide.md, *Which
// binary runs*, row 3). scripts/test-editor.mjs lays out
//
//   repo/node_modules/@reactogenic/cli …     the CLI of the opened folder, above it
//   repo/pnpm-lock.yaml                      its lockfile, above it too
//   repo/packages/app/                       the opened folder
//   repo/packages/app/nested/node_modules/…  a package inside it with a CLI of its own
//   elsewhere/node_modules/…                 a project outside the workspace, with its CLI
//
// and sets no $REACTOGENIC_BINARY.
import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";
import type { ServerState } from "../../src/server";
import { activate, file, onlyServers, open, runner, serverState, sleep, suiteDir, until } from "./harness";

/** The binary of the CLI installed in `root/node_modules`. */
function cli(root: string): string {
  const exe = process.platform === "win32" ? "reactogenic.exe" : "reactogenic";
  return fs.realpathSync(path.join(root, "node_modules/@reactogenic", `cli-${process.platform}-${process.arch}`, "bin", exe));
}

export const run = runner(() => {
  describe("reactogenic.rtsx, a package of a repository", () => {
    const repo = path.join(suiteDir(), "repo");
    const elsewhere = path.join(suiteDir(), "elsewhere");
    let api: Api;

    const state = (what: string, is: (s: ServerState) => unknown, timeout?: number) => serverState(api, what, is, timeout);

    before(async () => {
      // The first .rtsx document of the window is not the workspace's.
      const outside = await vscode.workspace.openTextDocument(vscode.Uri.file(path.join(elsewhere, "src/page.rtsx")));
      await vscode.window.showTextDocument(outside, { preview: false });
      api = await activate();
    });

    it("a document outside the workspace does not choose the binary: the opened folder's CLI runs", async () => {
      const active = vscode.window.activeTextEditor?.document.uri;
      assert.equal(active?.fsPath, path.join(elsewhere, "src/page.rtsx"));
      assert.equal(active && vscode.workspace.getWorkspaceFolder(active), undefined);
      const s = await api.ready();
      assert.equal(s.error, undefined);
      assert.deepEqual(s.binary, { path: cli(repo), source: "workspace", version: "9.9.9" });
      assert.equal(s.running, true);
      assert.equal(s.status.detail, `workspace: ${cli(repo)}`);
      await onlyServers([s.pid], cli(repo));
      await onlyServers([], cli(elsewhere));
    });

    it("the workspace's own document, opened after it, changes nothing", async () => {
      const before = await api.ready();
      await open("src/page.rtsx");
      await sleep(1000);
      const after = await api.ready();
      assert.equal(after.starts, before.starts);
      assert.equal(after.pid, before.pid);
    });

    it("the check task runs that binary", async () => {
      const [task] = await vscode.tasks.fetchTasks({ type: "reactogenic" });
      assert.equal((task.execution as vscode.ProcessExecution).process, cli(repo));
    });

    it("a change of the lockfile above the opened folder restarts the server", async () => {
      const before = await api.ready();
      const lockfile = path.join(repo, "pnpm-lock.yaml");
      assert.equal(vscode.workspace.getWorkspaceFolder(vscode.Uri.file(lockfile)), undefined);
      fs.writeFileSync(lockfile, "lockfileVersion: '9.0'\n# changed\n");
      const after = await state("a restart", (s) => s.starts > before.starts && s.running, 15_000);
      assert.notEqual(after.pid, before.pid);
      assert.deepEqual(after.binary, before.binary);
      // The TS server plugin is told too: it looks for the workspace's CLI again.
      await until("the plugin's word of the lockfile", () => (api.tsPlugin()?.installs ?? 0) > 0 && api.tsPlugin()?.languageServer === true);
      assert.equal(api.tsPlugin()?.trusted, true);
      // And again: the watch is renewed with each start.
      fs.rmSync(lockfile);
      const again = await state("a second restart", (s) => s.starts > after.starts && s.running, 15_000);
      await onlyServers([again.pid], cli(repo));
    });

    it("Restart Server chooses again, by the active document", async () => {
      const nested = file("nested");
      await open("nested/src/page.rtsx");
      await sleep(1000);
      const before = await api.ready();
      assert.equal(before.binary?.path, cli(repo), "opening a document does not move a server that a document chose");

      await vscode.commands.executeCommand("reactogenic.restartServer");
      const after = await state("the nested package's CLI", (s) => s.starts > before.starts && s.running);
      assert.equal(after.binary?.path, cli(nested.fsPath));
      await onlyServers([after.pid], cli(nested.fsPath));
      await onlyServers([], cli(repo));

      // Back: the opened folder's own document is active.
      await open("src/page.rtsx");
      await vscode.commands.executeCommand("reactogenic.restartServer");
      const back = await state("the opened folder's CLI", (s) => s.starts > after.starts && s.running);
      assert.equal(back.binary?.path, cli(repo));
    });

    it("with only an outside document open, Restart Server keeps to the workspace", async () => {
      await vscode.commands.executeCommand("workbench.action.closeAllEditors");
      const outside = await vscode.workspace.openTextDocument(vscode.Uri.file(path.join(elsewhere, "src/page.rtsx")));
      await vscode.window.showTextDocument(outside, { preview: false });
      const before = await api.ready();
      await vscode.commands.executeCommand("reactogenic.restartServer");
      const after = await state("a restart", (s) => s.starts > before.starts && s.running);
      assert.notEqual(after.binary?.path, cli(elsewhere));
      await onlyServers([], cli(elsewhere));
    });
  });
});
