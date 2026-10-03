// The editor suite, fourth window: *Show Transpiled TSX* (ide.md,
// *Commands*) — `reactogenic/transpiled` of the server under test, shown
// read-only beside the source; and, against test/editor/old-server.mjs, a
// server from before that request.
import * as assert from "node:assert/strict";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";
import { activate, binary, code, file, open, runner, serverState, sleep, until } from "./harness";

const SCHEME = "reactogenic-transpiled";
const shown = () => vscode.window.visibleTextEditors.filter((e) => e.document.uri.scheme === SCHEME);

export const run = runner(() => {
  describe("reactogenic.rtsx, Show Transpiled TSX", () => {
    let api: Api;

    before(async () => {
      await open("src/page.rtsx");
      api = await activate();
      const state = await api.ready();
      assert.equal(state.error, undefined);
      assert.equal(state.binary?.path, binary().path, "the server under test runs");
    });

    it("shows the emitted TSX beside the source", async () => {
      const page = await open("src/page.rtsx");
      assert.equal(await vscode.commands.executeCommand("reactogenic.showTranspiled"), "shown");
      const [tsx] = await until("the transpiled document", () => shown().length > 0 && shown());
      const text = tsx.document.getText();
      // The slots are props, the shorthand is written out, the rest is the source's.
      assert.match(text, /<Button size=\{size\} \$Icon=\{\{ className: "icon", children: \(\{ size \}\) => /);
      assert.match(text, /\$Label=\{\{ children: "Save" \}\} \/>/);
      assert.doesNotMatch(text, /<\$Icon/);
      assert.ok(text.startsWith('import { Button, type Size } from "./button";'), text);
      assert.notEqual(tsx.viewColumn, page.viewColumn);
      assert.equal(vscode.window.activeTextEditor?.document, page.document, "the focus stays in the source");
    });

    it("nothing else reports on the transpiled document: it is rtsx, a superset of TSX", async function () {
      this.timeout(120_000);
      const [tsx] = shown();
      // VS Code's TypeScript is up: it reports on a .ts file of the workspace.
      const util = file("src/util.ts");
      await vscode.window.showTextDocument(await vscode.workspace.openTextDocument(util), { preview: false, viewColumn: vscode.ViewColumn.One });
      await until("TypeScript's diagnostic for util.ts", () => vscode.languages.getDiagnostics(util).some((d) => code(d) === 2322), 90_000);
      await sleep(3000);
      const problems = vscode.languages
        .getDiagnostics()
        .filter(([uri, diagnostics]) => uri.scheme === SCHEME && diagnostics.length > 0)
        .map(([uri, diagnostics]) => `${uri.path.split("/").pop()}: ${diagnostics.map((d) => `${d.source} ${code(d)} ${d.message}`).join(" | ")}`);
      await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      assert.deepEqual(problems, []);
      assert.equal(tsx.document.languageId, "rtsx");
      assert.equal(tsx.document.uri.path.split("/").pop(), "page.transpiled.rtsx");
    });

    it("an edit to the source refreshes it: the unsaved buffer is what is transpiled", async () => {
      const page = await open("src/page.rtsx");
      const [tsx] = shown();
      const label = page.document.getText().indexOf("<$Label>Save");
      await page.edit((builder) => builder.replace(new vscode.Range(page.document.positionAt(label + 8), page.document.positionAt(label + 12)), "Store"));
      try {
        await until("the refresh", () => tsx.document.getText().includes('$Label={{ children: "Store" }}'), 10_000);
      } finally {
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
      await until("the refresh after the revert", () => tsx.document.getText().includes('$Label={{ children: "Save" }}'), 10_000);
    });

    it("the command in the transpiled document itself does nothing", async () => {
      const [tsx] = shown();
      await vscode.window.showTextDocument(tsx.document, { viewColumn: tsx.viewColumn });
      assert.equal(await vscode.commands.executeCommand("reactogenic.showTranspiled"), "no-document");
      assert.equal(shown().length, 1);
    });

    it("a server without the request: the command says that it is too old", async function () {
      const old = process.env.RTSX_OLD_SERVER;
      if (!old) {
        this.skip(); // Windows: the stand-in is started by a shell script
      }
      const settings = vscode.workspace.getConfiguration("reactogenic");
      const before = await api.ready();
      try {
        await settings.update("server.path", old, vscode.ConfigurationTarget.Global);
        const state = await serverState(api, "the old server", (s) => s.starts > before.starts && s.running);
        assert.equal(state.version, "0.0.0-old", "the stand-in runs");
        await open("src/button.rtsx");
        assert.equal(await vscode.commands.executeCommand("reactogenic.showTranspiled"), "unsupported");
        // Nothing is shown for it.
        await sleep(500);
        assert.deepEqual(
          shown().filter((e) => e.document.uri.path.endsWith("button.transpiled.rtsx")),
          [],
        );
      } finally {
        await settings.update("server.path", undefined, vscode.ConfigurationTarget.Global);
      }
      const back = await serverState(api, "the server again", (s) => s.running && !s.error && s.version !== "0.0.0-old");
      assert.equal(back.binary?.path, binary().path);
    });
  });
});
