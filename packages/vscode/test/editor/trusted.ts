// The editor suite (ide.md, *Testing* → extension): a trusted workspace,
// test/fixture, with $REACTOGENIC_BINARY built from this checkout.
import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import * as vscode from "vscode";
import type { Api } from "../../src/extension";
import type { ServerState } from "../../src/server";
import { activate, at, binary, code, file, onlyServers, open, processes, runner, runTask, serverState, sleep, until, where } from "./harness";

/** A problem as the Problems panel words it. */
const problem = (d: vscode.Diagnostic) => `${where(d)} ${vscode.DiagnosticSeverity[d.severity]} ${d.source}(${code(d)}): ${d.message}`;

export const run = runner(() => {
  describe("reactogenic.rtsx, trusted workspace", () => {
    let api: Api;
    let page: vscode.TextEditor;

    before(async () => {
      page = await open("src/page.rtsx");
      api = await activate();
    });

    /** The state once `is` holds: a restart is queued a moment after its cause. */
    const state = (what: string, is: (s: ServerState) => unknown, timeout?: number) => serverState(api, what, is, timeout);

    /** Appends `lines` after the `<Button size>` line of page.rtsx, a cursor at the end of each, and types `text`. */
    const typeAfter = async (lines: string[], text: string) => {
      const editor = await open("src/page.rtsx");
      const document = editor.document;
      const after = document.lineAt(at(document, "<Button size>").line).range.end;
      await editor.edit((builder) => builder.insert(after, lines.map((line) => `\n${line}`).join("")));
      const numbers = lines.map((_, i) => after.line + 1 + i);
      editor.selections = numbers.map((line) => new vscode.Selection(document.lineAt(line).range.end, document.lineAt(line).range.end));
      await sleep(300);
      await vscode.commands.executeCommand("workbench.action.focusActiveEditorGroup");
      await vscode.commands.executeCommand("type", { text });
      return { document, text: () => numbers.map((line) => document.lineAt(line).text) };
    };

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
      // One process, and its command line to the letter: `lsp --stdio`, once.
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

    it("typing > at two cursors closes each tag with its own name", async () => {
      const typed = await typeAfter(["        <div", "        <$Label"], ">");
      try {
        await until("a closing tag", () => typed.text().every((line) => line.includes("</")), 10_000);
        assert.deepEqual(typed.text(), ["        <div></div>", "        <$Label></$Label>"]);
        // Both cursors stay between their tags.
        await vscode.commands.executeCommand("type", { text: "x" });
        await until("the next character", () => typed.text().every((line) => line.includes("x")), 10_000);
        assert.deepEqual(typed.text(), ["        <div>x</div>", "        <$Label>x</$Label>"]);
      } finally {
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
    });

    it("typing > at two cursors in tags of one name closes both", async () => {
      const typed = await typeAfter(["        <div", "        <div"], ">");
      try {
        await until("a closing tag", () => typed.text().every((line) => line.includes("</")), 10_000);
        await vscode.commands.executeCommand("type", { text: "x" });
        await until("the next character", () => typed.text().every((line) => line.includes("x")), 10_000);
        assert.deepEqual(typed.text(), ["        <div>x</div>", "        <div>x</div>"]);
      } finally {
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
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

    it("an untitled rtsx document is served as .rtsx", async () => {
      // (A slot under a component: under `<div>` it would be an orphan, and
      // the file would report that and nothing of TypeScript's.)
      const text =
        "const n: number = 'x';\nexport const a = <div>{n}</div>;\nexport const b = <Box><$Icon className=\"i\" { size }>{size}</$Icon></Box>;\nfunction Box(p: { $Icon?: unknown }) { return null; }\n";
      const document = await vscode.workspace.openTextDocument({ language: "rtsx", content: text });
      await vscode.window.showTextDocument(document, { preview: false });
      try {
        assert.equal(document.uri.scheme, "untitled");
        assert.equal(document.languageId, "rtsx");
        const diagnostics = await until("TS2322 from the server", () => {
          const all = vscode.languages.getDiagnostics(document.uri);
          return all.some((d) => code(d) === 2322) && all;
        });
        assert.equal(document.getText(diagnostics.find((d) => code(d) === 2322)?.range), "n");
        // Mapped, not read as plain TypeScript: no syntax error (TS1xxx: the
        // `>` of `</div>` as a regular expression), no element name as an
        // identifier (TS2304), no "JSX without the option" (TS17004).
        const foreign = diagnostics.filter((d) => {
          const n = Number(code(d));
          return (n >= 1000 && n < 2000) || n === 2304 || (n >= 17000 && n < 18000);
        });
        assert.deepEqual(foreign.map(problem), []);
        const hovers = await vscode.commands.executeCommand<vscode.Hover[]>("vscode.executeHoverProvider", document.uri, new vscode.Position(1, 23));
        assert.equal(hovers.length, 1);
        assert.match(hovers[0].contents.map((c) => (typeof c === "string" ? c : c.value)).join("\n"), /const n: number/);
        // A definition inside the document names it as VS Code does.
        const definitions = await vscode.commands.executeCommand<(vscode.Location | vscode.LocationLink)[]>(
          "vscode.executeDefinitionProvider",
          document.uri,
          new vscode.Position(1, 23),
        );
        assert.deepEqual(
          definitions.map((d) => ("targetUri" in d ? d.targetUri : d.uri).toString()),
          [document.uri.toString()],
        );
      } finally {
        await vscode.commands.executeCommand("workbench.action.revertAndCloseActiveEditor");
      }
      assert.equal((await api.ready()).running, true);
    });

    it("the check task runs the server's binary and reports closed documents: .rtsx through $reactogenic, .ts as TypeScript's", async () => {
      const broken = file("src/broken.rtsx");
      const util = file("src/util.ts");
      const closed = (uri: vscode.Uri) => !vscode.workspace.textDocuments.some((d) => d.uri.toString() === uri.toString());
      assert.ok(closed(broken) && closed(util), "broken.rtsx and util.ts are closed");
      const tasks = await vscode.tasks.fetchTasks({ type: "reactogenic" });
      assert.deepEqual(
        tasks.map((t) => `${t.source}: ${t.name}`),
        ["reactogenic: check"],
      );
      const [task] = tasks;
      const execution = task.execution as vscode.ProcessExecution;
      assert.equal(execution.process, binary().path);
      assert.deepEqual(execution.args, ["check", "--pretty=false"]);

      assert.equal(await runTask(task), 1);
      const problems = await until("the task's problems", () => {
        const diagnostics = vscode.languages.getDiagnostics(broken);
        return diagnostics.length >= 3 && diagnostics;
      });
      assert.deepEqual(problems.map(problem).sort(), [
        "3:8 Error reactogenic(TS2322): Type 'string' is not assignable to type 'number'.",
        "6:15 Warning reactogenic(segment-children): Contents will be overwritten by the segment `intro`",
        "8:9 Error reactogenic(undeclared-slot): `$Badge` is not declared in `Button`",
      ]);
      // A .ts file's line is a problem of TypeScript's, as `$tsc` would make it.
      const other = await until("the task's problem in util.ts", () => vscode.languages.getDiagnostics(util).length > 0 && vscode.languages.getDiagnostics(util), 10_000);
      assert.deepEqual(other.map(problem), ["2:13 Error ts(2322): Type 'string' is not assignable to type 'number'."]);
      assert.equal(typeof code(other[0]), "string", "the task's problem, not TypeScript's own: the file is closed");
      assert.deepEqual(task.problemMatchers, ["$reactogenic", "$reactogenic-ts"]);
    });

    it("Restart Server keeps the task's problems of closed documents", async () => {
      const broken = file("src/broken.rtsx");
      assert.equal(vscode.languages.getDiagnostics(broken).length, 3);
      const before = await api.ready();
      await vscode.commands.executeCommand("reactogenic.restartServer");
      await state("a restart", (s) => s.starts > before.starts && s.running);
      await sleep(1000);
      assert.equal(vscode.languages.getDiagnostics(broken).length, 3);
    });

    it("an opened .ts file has its problem once: VS Code's TypeScript replaces the task's", async function () {
      this.timeout(120_000);
      const util = file("src/util.ts");
      await open("src/util.ts");
      try {
        // TypeScript's own diagnostic has the number as its code; the task's, the text.
        await until("TypeScript's own TS2322", () => vscode.languages.getDiagnostics(util).some((d) => code(d) === 2322), 90_000);
        await sleep(500);
        assert.deepEqual(vscode.languages.getDiagnostics(util).map(problem), ["2:13 Error ts(2322): Type 'string' is not assignable to type 'number'."]);
      } finally {
        await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      }
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
      // Each problem once: the server reports the task's three lines — the
      // transpiler's and the reworded one under the same names — and no
      // line of the task is left beside them. (Suggestions aside: the
      // unused `count` is a hint, which `check` does not print.)
      assert.deepEqual(
        vscode.languages
          .getDiagnostics(editor.document.uri)
          .filter((d) => d.severity !== vscode.DiagnosticSeverity.Hint)
          .map(problem)
          .sort(),
        [
          "3:8 Error ts(2322): Type 'string' is not assignable to type 'number'.",
          "6:15 Warning reactogenic(segment-children): Contents will be overwritten by the segment `intro`",
          "8:9 Error reactogenic(undeclared-slot): `$Badge` is not declared in `Button`",
        ],
      );
      // And they go with the document.
      await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      await until("the file's problems to go", () => vscode.languages.getDiagnostics(file("src/broken.rtsx")).length === 0);
    });

    // The reporting layer in the server's diagnostics path (ide.md,
    // *Diagnostics*): what the editor shows is what `check` prints.
    it("a slot-term diagnostic: undeclared-slot at the slot name", async () => {
      const editor = await open("src/broken.rtsx");
      try {
        const diagnostic = await until("undeclared-slot", () =>
          vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === "undeclared-slot"),
        );
        assert.equal(editor.document.getText(diagnostic.range), "$Badge");
        assert.equal(where(diagnostic), "8:9");
        assert.equal(diagnostic.message, "`$Badge` is not declared in `Button`");
        assert.equal(diagnostic.severity, vscode.DiagnosticSeverity.Error);
        assert.equal(diagnostic.source, "reactogenic");
        // And the transpiler's own, with its severity.
        const warning = vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === "segment-children");
        assert.ok(warning, "segment-children");
        assert.equal(warning.severity, vscode.DiagnosticSeverity.Warning);
        assert.equal(editor.document.getText(warning.range), "#intro");
        // A suggestion of TypeScript's passes through with its tag: the
        // unused name is faded, not underlined.
        const unused = vscode.languages.getDiagnostics(editor.document.uri).find((d) => code(d) === 6133);
        assert.ok(unused, "TS6133");
        assert.equal(unused.severity, vscode.DiagnosticSeverity.Hint);
        assert.deepEqual(unused.tags, [vscode.DiagnosticTag.Unnecessary]);
        assert.equal(editor.document.getText(unused.range), "count");
      } finally {
        await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      }
    });

    // ide.md, *Rename*: the server is attached to .rtsx documents only, and
    // computes its edit of a .ts file from the file on disk.
    it("a rename into a .ts file with unsaved changes is refused; saved, it is edited", async () => {
      const shaped = await open("src/shaped.rtsx");
      await until("the server's answer for shaped.rtsx", async () => {
        const hovers = await vscode.commands.executeCommand<vscode.Hover[]>("vscode.executeHoverProvider", shaped.document.uri, at(shaped.document, "shape.wide", 7));
        return hovers.length > 0;
      });
      const rename = async () =>
        await vscode.commands.executeCommand<vscode.WorkspaceEdit>("vscode.executeDocumentRenameProvider", shaped.document.uri, at(shaped.document, "shape.wide", 7), "broad");
      const shape = await open("src/shape.ts");
      const saved = shape.document.getText();
      // One line above the declaration, not saved: every position below it moves.
      await shape.edit((builder) => builder.insert(new vscode.Position(0, 0), "// unsaved\n"));
      assert.equal(shape.document.isDirty, true);
      try {
        await assert.rejects(rename, /Rename refused: shape\.ts has unsaved changes, and the Reactogenic server reads it from disk/);
        assert.equal(shape.document.getText(), `// unsaved\n${saved}`, "nothing was edited");
      } finally {
        await vscode.window.showTextDocument(shape.document);
        await vscode.commands.executeCommand("workbench.action.files.revert");
      }
      assert.equal(shape.document.isDirty, false);
      // Saved, the same rename edits both files, each at its name.
      const edit = await rename();
      const edits = edit.entries().flatMap(([uri, list]) => list.map((e) => `${uri.path.split("/").pop()} ${e.range.start.line}:${e.range.start.character} ${e.newText}`));
      assert.deepEqual(edits.sort(), ["shape.ts 3:2 broad", "shaped.rtsx 3:26 broad"]);
      for (const [uri, list] of edit.entries()) {
        const document = await vscode.workspace.openTextDocument(uri);
        assert.deepEqual(
          list.map((e) => document.getText(e.range)),
          ["wide"],
        );
      }
      await vscode.window.showTextDocument(shape.document);
      await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
      await vscode.window.showTextDocument(shaped.document);
      await vscode.commands.executeCommand("workbench.action.closeActiveEditor");
    });

    // The command's other answers, and the refresh: test/editor/transpiled.ts.
    it("Show Transpiled TSX: shows the TSX beside the source", async () => {
      await open("src/page.rtsx");
      assert.equal(await vscode.commands.executeCommand<string>("reactogenic.showTranspiled"), "shown");
      const tsx = await until("the transpiled document", () => vscode.window.visibleTextEditors.find((e) => e.document.uri.scheme === "reactogenic-transpiled"));
      // Language rtsx, a superset of TSX: no other extension reports on it.
      assert.equal(tsx.document.languageId, "rtsx");
      assert.match(tsx.document.getText(), /\$Icon=\{\{/);
      assert.notEqual(tsx.viewColumn, vscode.window.activeTextEditor?.viewColumn);
      await sleep(2000);
      assert.deepEqual(vscode.languages.getDiagnostics(tsx.document.uri), []);
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

    it("a binary that never answers: the next restart does not wait for it; left alone, it is an error", async function () {
      if (process.platform === "win32") {
        this.skip(); // the stand-in is a shell script
      }
      this.timeout(90_000);
      const settings = vscode.workspace.getConfiguration("reactogenic");
      const expected = binary();
      // It starts, reads nothing and never exits.
      const hanging = path.join(os.tmpdir(), `reactogenic-hangs-${process.pid}`);
      const sleeper = `sleep ${600_000 + (process.pid % 100_000)}`;
      const sleeping = () => processes(sleeper)?.length ?? 0;
      fs.writeFileSync(hanging, `#!/bin/sh\nexec ${sleeper}\n`, { mode: 0o755 });
      try {
        await settings.update("server.path", hanging, vscode.ConfigurationTarget.Global);
        await until("the hanging process", () => sleeping() === 1, 10_000);
        // The setting is removed while that start is under way.
        const removed = Date.now();
        await settings.update("server.path", undefined, vscode.ConfigurationTarget.Global);
        const back = await state("the server again", (s) => s.running && !s.error, 30_000);
        assert.deepEqual(back.binary, expected);
        assert.ok(Date.now() - removed < 8_000, `the restart waited ${Date.now() - removed} ms behind the hanging start`);
        await until("the hanging process to be killed", () => sleeping() === 0, 10_000);

        // Left alone, the start gives up after its time limit.
        await settings.update("server.path", hanging, vscode.ConfigurationTarget.Global);
        const failed = await state("the error", (s) => s.error, 40_000);
        assert.equal(failed.running, false);
        assert.match(failed.error ?? "", /lsp --stdio did not answer within \d+ s/);
        assert.deepEqual(failed.status, { text: "reactogenic: no server", detail: failed.error });
        await until("the hanging process to be killed", () => sleeping() === 0, 10_000);
        // And Restart Server is not stuck behind it. (Not awaited: the command ends with the start.)
        void vscode.commands.executeCommand("reactogenic.restartServer");
        await until("the hanging process again", () => sleeping() === 1, 10_000);
      } finally {
        await settings.update("server.path", undefined, vscode.ConfigurationTarget.Global);
        fs.rmSync(hanging, { force: true });
      }
      const back = await state("the server again", (s) => s.running && !s.error, 30_000);
      assert.deepEqual(back.binary, expected);
      await until("no hanging process", () => sleeping() === 0, 10_000);
      await onlyServers([back.pid]);
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

    it("five Restart Server at once: each gives up the start before it, and one server is left", async () => {
      const before = await api.ready();
      await Promise.all(Array.from({ length: 5 }, () => vscode.commands.executeCommand("reactogenic.restartServer")));
      const after = await state("the server", (s) => s.running && !s.error);
      assert.equal(after.starts, before.starts + 5);
      await onlyServers([after.pid]);
    });

    it("a server that keeps crashing: the status item says it stopped, and Restart Server brings it back", async function () {
      if (process.platform === "win32") {
        this.skip();
      }
      this.timeout(120_000);
      // The language client restarts a crashed server four times, then gives up.
      const first = await api.ready();
      for (let crash = 1; crash <= 5; crash++) {
        const { pid } = await api.ready();
        assert.ok(pid, `a server to crash (${crash})`);
        process.kill(pid, "SIGKILL");
        if (crash < 5) {
          const again = await state("the client's own restart", (s) => s.running && s.pid !== pid && !s.error);
          assert.equal(again.status.text, first.status.text);
        }
      }
      const stopped = await state("the stop", (s) => !s.running && s.error);
      assert.match(stopped.error ?? "", /^The server stopped/);
      assert.deepEqual(stopped.status, { text: "reactogenic: no server", detail: stopped.error });
      await onlyServers([]);

      await vscode.commands.executeCommand("reactogenic.restartServer");
      const back = await state("the server again", (s) => s.running && !s.error);
      assert.deepEqual(back.status, first.status);
      await onlyServers([back.pid]);
    });
  });
});
