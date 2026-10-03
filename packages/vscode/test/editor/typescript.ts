// The `.ts` side (ide.md, *The plugin*) in the editor: VS Code's own
// TypeScript, with the plugin this extension contributes, on src/main.tsx —
// which imports `./page`, an `.rtsx` module. The window opens a `.tsx` file
// first, as a user does: the extension itself is not activated yet.
import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as vscode from "vscode";
import { activate, at, code, file, open, runner, sleep, until } from "./harness";

export const run = runner(() => {
  describe("reactogenic.rtsx, the .ts side", () => {
    let main: vscode.TextEditor;

    const target = (found: vscode.Location | vscode.LocationLink) => ("targetUri" in found ? { uri: found.targetUri, range: found.targetSelectionRange ?? found.targetRange } : found);
    /**
     * TypeScript's definitions at `Page` in main.tsx's import. While the
     * project loads, VS Code's syntax server answers with the import itself:
     * the answer that counts is the one that leaves the file, or none at all
     * once there are errors to show.
     */
    const definitions = () =>
      until(
        "TypeScript's definition of Page",
        async () => {
          const result = await vscode.commands.executeCommand<(vscode.Location | vscode.LocationLink)[]>("vscode.executeDefinitionProvider", main.document.uri, at(main.document, "Page", 1));
          const found = result.map(target);
          const loaded = found.some((d) => d.uri.fsPath !== main.document.uri.fsPath) || vscode.languages.getDiagnostics(main.document.uri).length > 0;
          return loaded && found;
        },
        90_000,
      );
    /** Where page.rtsx declares `Page`, from the file on disk. */
    const declared = () => {
      const lines = fs.readFileSync(file("src/page.rtsx").fsPath, "utf8").split("\n");
      const line = lines.findIndex((text) => text.startsWith("export function Page"));
      return new vscode.Position(line, "export function ".length);
    };

    before(async () => {
      main = await open("src/main.tsx");
    });

    it("an .rtsx import resolves before the extension is activated: a definition at the source position, no TS2307", async () => {
      const found = await definitions();
      assert.deepEqual(
        found.map((d) => [d.uri.fsPath, d.range.start.line, d.range.start.character]),
        [[file("src/page.rtsx").fsPath, declared().line, declared().character]],
      );
      // The project is loaded: what TypeScript has to say about main.tsx follows at once.
      await sleep(2_000);
      const diagnostics = vscode.languages.getDiagnostics(main.document.uri);
      assert.deepEqual(diagnostics.map(code), [], diagnostics.map((d) => d.message).join("\n"));
      assert.equal(vscode.extensions.getExtension("reactogenic.rtsx")?.isActive, false, "nothing has asked for the extension yet");
    });

    it("and after it: the extension's configuration changes nothing that shows", async () => {
      const api = await activate();
      assert.equal((await api.ready()).running, true);
      await sleep(1_000); // configurePlugin reaches tsserver
      const found = await definitions();
      assert.deepEqual(
        found.map((d) => [d.uri.fsPath, d.range.start.line, d.range.start.character]),
        [[file("src/page.rtsx").fsPath, declared().line, declared().character]],
      );
      assert.deepEqual(vscode.languages.getDiagnostics(main.document.uri).map(code), []);
    });
  });
});
