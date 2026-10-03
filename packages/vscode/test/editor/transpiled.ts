// The editor suite, fourth window: *Show Transpiled TSX* where the server has
// `reactogenic/transpiled`. The real one gains it with RGP1-108; until then
// $REACTOGENIC_BINARY is test/editor/fake-server.mjs, which answers as the
// real one does for a stopped file — the source itself, which is not TSX.
import * as assert from "node:assert/strict";
import * as vscode from "vscode";
import { activate, code, file, open, runner, sleep, until } from "./harness";

const SCHEME = "reactogenic-transpiled";
const shown = () => vscode.window.visibleTextEditors.filter((e) => e.document.uri.scheme === SCHEME);

export const run = runner(() => {
  describe("reactogenic.rtsx, a server with reactogenic/transpiled", () => {
    before(async () => {
      await open("src/page.rtsx");
      const api = await activate();
      const state = await api.ready();
      assert.equal(state.error, undefined);
      assert.equal(state.version, "0.0.0-fake", "the stand-in server runs");
    });

    it("shows the transpiled text beside the source", async () => {
      const page = await open("src/page.rtsx");
      assert.equal(await vscode.commands.executeCommand("reactogenic.showTranspiled"), "shown");
      const [tsx] = await until("the transpiled document", () => shown().length > 0 && shown());
      assert.equal(tsx.document.getText(), `// transpiled 1\n${page.document.getText()}`);
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

    it("an edit to the source refreshes it", async () => {
      const page = await open("src/page.rtsx");
      const [tsx] = shown();
      await page.edit((builder) => builder.insert(new vscode.Position(0, 0), "// edited\n"));
      try {
        await until("the refresh", () => tsx.document.getText().startsWith("// transpiled 2\n// edited\n"), 10_000);
      } finally {
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
    });

    it("the command in the transpiled document itself does nothing", async () => {
      const [tsx] = shown();
      await vscode.window.showTextDocument(tsx.document, { viewColumn: tsx.viewColumn });
      assert.equal(await vscode.commands.executeCommand("reactogenic.showTranspiled"), "no-document");
      assert.equal(shown().length, 1);
    });
  });
});
