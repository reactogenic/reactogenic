// The editor suite (ide.md, *Testing* → extension): a trusted workspace,
// test/fixture, with $REACTOGENIC_BINARY built from this checkout.
import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";
import type { ServerState } from "../../src/server";
import { activate, at, binary, file, onlyServers, open, runner, sleep, until } from "./harness";

const code = (d: vscode.Diagnostic) => (typeof d.code === "object" ? d.code.value : d.code);
const where = (d: vscode.Diagnostic) => `${d.range.start.line}:${d.range.start.character}`;

export const run = runner(() => {
  describe("reactogenic.rtsx, trusted workspace", () => {
    let api: Api;
    let page: vscode.TextEditor;

    before(async () => {
      page = await open("src/page.rtsx");
      api = await activate();
    });

    /** The state once `is` holds: a restart is queued a moment after its cause. */
    const state = (what: string, is: (s: ServerState) => unknown) =>
      until<ServerState>(what, async () => {
        const s = await api.ready();
        return is(s) ? s : undefined;
      });

    it("the document's language id is rtsx", () => {
      assert.equal(page.document.languageId, "rtsx");
      assert.equal(vscode.workspace.isTrusted, true);
    });

    it("runs one server — $REACTOGENIC_BINARY, or the bundled binary — and the status item names it", async () => {
      const state = await api.ready();
      const expected = binary();
      assert.equal(state.error, undefined);
      assert.equal(state.running, true);
      assert.deepEqual(state.binary, expected);
      assert.ok(state.version, "serverInfo.version");
      const found = expected.source === "env" ? "$REACTOGENIC_BINARY" : "bundled";
      assert.deepEqual(state.status, { text: `reactogenic ${state.version}`, detail: `${found}: ${expected.path}` });
      assert.equal(state.starts, 1);
      assert.ok(state.pid);
      await onlyServers([state.pid]);
    });

    it("exactly one hover result at a slot tag", async () => {
      const position = at(page.document, "<$Icon", 3);
      const hovers = await until("a hover", async () => {
        const result = await vscode.commands.executeCommand<vscode.Hover[]>("vscode.executeHoverProvider", page.document.uri, position);
        return result.length > 0 && result;
      });
      assert.equal(hovers.length, 1);
      const text = hovers[0].contents.map((c) => (typeof c === "string" ? c : c.value)).join("\n");
      assert.match(text, /\(property\) ButtonProps\.\$Icon\?: Slot</);
      // The slot tag's own range, in .rtsx coordinates.
      assert.equal(page.document.getText(hovers[0].range), "$Icon");
    });

    it("exactly one definition result at a slot tag", async () => {
      const position = at(page.document, "<$Icon", 3);
      const definitions = await until("a definition", async () => {
        const result = await vscode.commands.executeCommand<(vscode.Location | vscode.LocationLink)[]>(
          "vscode.executeDefinitionProvider",
          page.document.uri,
          position,
        );
        return result.length > 0 && result;
      });
      assert.equal(definitions.length, 1);
      const [definition] = definitions;
      const uri = "targetUri" in definition ? definition.targetUri : definition.uri;
      const range = "targetUri" in definition ? (definition.targetSelectionRange ?? definition.targetRange) : definition.range;
      assert.equal(uri.toString(), file("src/button.rtsx").toString());
      const button = await vscode.workspace.openTextDocument(uri);
      assert.equal(button.getText(range), "$Icon");
      assert.equal(range.start.line, at(button, "$Icon?: Slot").line);
    });

    it("typing > after <$Icon { size } inserts the closing tag", async () => {
      const editor = await open("src/page.rtsx");
      const document = editor.document;
      const after = document.lineAt(at(document, "<Button size>").line).range.end;
      await editor.edit((builder) => builder.insert(after, "\n        <$Icon { size }"));
      const line = after.line + 1;
      const end = document.lineAt(line).range.end;
      editor.selection = new vscode.Selection(end, end);
      await sleep(300);

      await vscode.commands.executeCommand("workbench.action.focusActiveEditorGroup");
      await vscode.commands.executeCommand("type", { text: ">" });
      await until("the closing tag", () => document.lineAt(line).text.endsWith("</$Icon>"));
      assert.equal(document.lineAt(line).text, "        <$Icon { size }></$Icon>");
      // A snippet: the cursor stays between the tags. (What is typed next
      // shows it; `editor.selection` lags behind after a snippet at the cursor.)
      await vscode.commands.executeCommand("type", { text: "x" });
      await until("the next character", () => document.lineAt(line).text.includes("x"));
      assert.equal(document.lineAt(line).text, "        <$Icon { size }>x</$Icon>");
      await vscode.commands.executeCommand("workbench.action.files.revert");
    });

    it("typing > inserts nothing when reactogenic.autoClosingTags is off", async () => {
      const editor = await open("src/page.rtsx");
      const document = editor.document;
      const settings = vscode.workspace.getConfiguration("reactogenic");
      await settings.update("autoClosingTags", false, vscode.ConfigurationTarget.Global);
      try {
        const after = document.lineAt(at(document, "<Button size>").line).range.end;
        await editor.edit((builder) => builder.insert(after, "\n        <div"));
        const end = document.lineAt(after.line + 1).range.end;
        editor.selection = new vscode.Selection(end, end);
        await sleep(300);
        await vscode.commands.executeCommand("workbench.action.focusActiveEditorGroup");
        await vscode.commands.executeCommand("type", { text: ">" });
        await until("the >", () => document.lineAt(after.line + 1).text.endsWith(">"));
        await sleep(1000);
        assert.equal(document.lineAt(after.line + 1).text, "        <div>");
      } finally {
        await settings.update("autoClosingTags", undefined, vscode.ConfigurationTarget.Global);
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
    });

    it("Toggle Line Comment inside a JSX child writes {/* */}", async () => {
      const editor = await open("src/page.rtsx");
      const document = editor.document;
      const position = at(document, "some text", 3);
      editor.selection = new vscode.Selection(position, position);
      await vscode.commands.executeCommand("workbench.action.focusActiveEditorGroup");
      await vscode.commands.executeCommand("editor.action.commentLine");
      await until("the comment", () => document.lineAt(position.line).text !== "      some text", 5_000);
      assert.equal(document.lineAt(position.line).text, "      {/* some text */}");

      // And a line of code, outside JSX: `//`.
      const outside = at(document, "const size", 0);
      editor.selection = new vscode.Selection(outside, outside);
      await vscode.commands.executeCommand("editor.action.commentLine");
      await until("the comment", () => document.lineAt(outside.line).text.includes("//"), 5_000);
      assert.equal(document.lineAt(outside.line).text, '  // const size: Size = "lg";');
      await vscode.commands.executeCommand("workbench.action.files.revert");
    });

    it("the check task runs the server's binary and reports a closed document through $reactogenic", async () => {
      const broken = file("src/broken.rtsx");
      assert.ok(!vscode.workspace.textDocuments.some((d) => d.uri.toString() === broken.toString()), "broken.rtsx is closed");
      const tasks = await vscode.tasks.fetchTasks({ type: "reactogenic" });
      assert.deepEqual(
        tasks.map((t) => `${t.source}: ${t.name}`),
        ["reactogenic: check"],
      );
      const [task] = tasks;
      const execution = task.execution as vscode.ProcessExecution;
      assert.equal(execution.process, binary().path);
      assert.deepEqual(execution.args, ["check", "--pretty=false"]);
      assert.deepEqual(task.problemMatchers, ["$reactogenic"]);

      const ended = new Promise<number | undefined>((resolve) => {
        const subscription = vscode.tasks.onDidEndTaskProcess((e) => {
          if (e.execution.task.name === "check") {
            subscription.dispose();
            resolve(e.exitCode);
          }
        });
      });
      await vscode.tasks.executeTask(task);
      assert.equal(await ended, 1);
      const problems = await until("the task's problems", () => {
        const diagnostics = vscode.languages.getDiagnostics(broken);
        return diagnostics.length >= 3 && diagnostics;
      });
      assert.deepEqual(
        problems.map((d) => `${where(d)} ${vscode.DiagnosticSeverity[d.severity]} ${code(d)}: ${d.message}`).sort(),
        [
          "3:8 Error TS2322: Type 'string' is not assignable to type 'number'.",
          "6:15 Warning segment-children: Contents will be overwritten by the segment `intro`",
          "8:9 Error undeclared-slot: `$Badge` is not declared in `Button`",
        ],
      );
    });

    it("an open document's problems come from the server, at .rtsx positions", async () => {
      const editor = await open("src/broken.rtsx");
      const diagnostic = await until("TS2322 from the server", () =>
        vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === 2322),
      );
      assert.equal(where(diagnostic), "3:8");
      assert.equal(editor.document.getText(diagnostic.range), "count");
      assert.equal(diagnostic.message, "Type 'string' is not assignable to type 'number'.");
      // One owner: the server's diagnostics replace what the task left for
      // this file, rather than showing beside it.
      // (The task's code is the text `TS2322`; the server's is the number.)
      await until("the task's problems to go", () => !vscode.languages.getDiagnostics(editor.document.uri).some((d) => code(d) === "TS2322"));
      assert.ok(vscode.languages.getDiagnostics(editor.document.uri).some((d) => code(d) === 2322));
      // And they go with the document.
      await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      await until("the file's problems to go", () => vscode.languages.getDiagnostics(file("src/broken.rtsx")).length === 0);
    });

    // TODO(RGP1-107): the server reports raw TypeScript errors today. With the
    // reporting layer in its diagnostics path, this is what the editor shows.
    it.skip("a slot-term diagnostic: undeclared-slot at the slot name", async () => {
      const editor = await open("src/broken.rtsx");
      const diagnostic = await until("undeclared-slot", () =>
        vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === "undeclared-slot"),
      );
      assert.equal(editor.document.getText(diagnostic.range), "$Badge");
      assert.equal(where(diagnostic), "8:9");
      assert.equal(diagnostic.message, "`$Badge` is not declared in `Button`");
      assert.equal(diagnostic.severity, vscode.DiagnosticSeverity.Error);
      // And the transpiler's own, with its severity.
      const warning = vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === "segment-children");
      assert.equal(warning?.severity, vscode.DiagnosticSeverity.Warning);
    });

    it("Show Transpiled TSX: says the server is too old, or shows the TSX beside the source", async () => {
      await open("src/page.rtsx");
      const result = await vscode.commands.executeCommand<string>("reactogenic.showTranspiled");
      if (result === "unsupported") {
        return; // TODO(RGP1-108): the server gains `reactogenic/transpiled`; then the branch below runs
      }
      assert.equal(result, "shown");
      const tsx = await until("the transpiled document", () => vscode.window.visibleTextEditors.find((e) => e.document.uri.scheme === "reactogenic-transpiled"));
      assert.equal(tsx.document.languageId, "typescriptreact");
      assert.match(tsx.document.getText(), /\$Icon=\{\{/);
      assert.notEqual(tsx.viewColumn, vscode.window.activeTextEditor?.viewColumn);
    });

    it("a change of reactogenic.server.path restarts the server; a path that does not exist is an error", async () => {
      const settings = vscode.workspace.getConfiguration("reactogenic");
      const expected = binary();
      const before = await api.ready();
      const missing = path.join(os.tmpdir(), "no-such-reactogenic");
      try {
        await settings.update("server.path", expected.path, vscode.ConfigurationTarget.Global);
        const set = await state("a restart", (s) => s.starts > before.starts && s.running);
        assert.deepEqual(set.binary, { path: expected.path, source: "setting" });
        assert.equal(set.status.detail, `reactogenic.server.path: ${expected.path}`);

        await settings.update("server.path", missing, vscode.ConfigurationTarget.Global);
        const failed = await state("the error", (s) => s.error);
        assert.equal(failed.running, false);
        assert.equal(failed.binary, undefined);
        assert.equal(failed.error, `reactogenic.server.path: ${missing} does not exist`);
        assert.deepEqual(failed.status, { text: "reactogenic: no server", detail: failed.error });
        await onlyServers([]);

        // A binary without `lsp`, as @reactogenic/cli 0.1.0-alpha.0's: an error too, not a retry loop.
        if (process.platform !== "win32") {
          const old = path.join(os.tmpdir(), `reactogenic-without-lsp-${process.pid}`);
          fs.writeFileSync(old, "#!/bin/sh\necho 'reactogenic: unknown command \"lsp\"' >&2\nexit 2\n", { mode: 0o755 });
          try {
            await settings.update("server.path", old, vscode.ConfigurationTarget.Global);
            const refused = await state("the error", (s) => s.error?.includes(old));
            assert.equal(refused.running, false);
            assert.match(refused.error ?? "", /lsp --stdio did not start .* has no language server\.$/);
          } finally {
            fs.rmSync(old, { force: true });
          }
        }
      } finally {
        await settings.update("server.path", undefined, vscode.ConfigurationTarget.Global);
      }
      const back = await state("the server again", (s) => s.running && !s.error);
      assert.deepEqual(back.binary, expected);
    });

    it("a lockfile change restarts the server", async () => {
      const before = await api.ready();
      await vscode.workspace.fs.writeFile(file("pnpm-lock.yaml"), Buffer.from("lockfileVersion: '9.0'\n"));
      const after = await state("a restart", (s) => s.starts > before.starts && s.running);
      assert.notEqual(after.pid, before.pid);
      await onlyServers([after.pid]);
    });

    it("Restart Server: a new process, and still one", async () => {
      const before = await api.ready();
      await vscode.commands.executeCommand("reactogenic.restartServer");
      const after = await api.ready();
      assert.equal(after.running, true);
      assert.equal(after.starts, before.starts + 1);
      assert.notEqual(after.pid, before.pid);
      await onlyServers([after.pid]);
      // The new server answers.
      const editor = await open("src/page.rtsx");
      const position = at(editor.document, "<$Label", 3);
      const hovers = await until("a hover", async () => {
        const result = await vscode.commands.executeCommand<vscode.Hover[]>("vscode.executeHoverProvider", editor.document.uri, position);
        return result.length > 0 && result;
      });
      assert.equal(hovers.length, 1);
    });
  });
});
