// The editor suite, second window: the same fixture, not trusted. Highlighting
// only — the server reads tsconfig and can run a binary from node_modules.
import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as vscode from "vscode";
import { activate, at, binary, onlyServers, open, runner, sleep } from "./harness";

export const run = runner(() => {
  describe("reactogenic.rtsx, untrusted workspace", () => {
    let page: vscode.TextEditor;

    before(async () => {
      page = await open("src/page.rtsx");
    });

    it("the window is in restricted mode, and the binary exists", () => {
      assert.equal(vscode.workspace.isTrusted, false);
      assert.ok(fs.existsSync(binary().path), binary().path);
    });

    it("the document's language id is rtsx: highlighting works", () => {
      assert.equal(page.document.languageId, "rtsx");
    });

    it("starts no server process", async () => {
      const api = await activate();
      const state = await api.ready();
      assert.deepEqual(state, {
        trusted: false,
        running: false,
        pid: undefined,
        binary: undefined,
        version: undefined,
        status: { text: "reactogenic: off", detail: "Untrusted workspace: syntax highlighting only" },
        error: undefined,
        starts: 0,
      });
      // Not a moment later either, and not for a request.
      const hovers = await vscode.commands.executeCommand<vscode.Hover[]>("vscode.executeHoverProvider", page.document.uri, at(page.document, "<$Icon", 3));
      assert.deepEqual(hovers, []);
      await vscode.commands.executeCommand("reactogenic.restartServer");
      await sleep(2000);
      assert.equal((await api.ready()).starts, 0);
      await onlyServers([]);
    });
  });
});
