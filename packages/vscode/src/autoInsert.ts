// Closing-tag insertion (ide.md, *Tags*): when `>` is typed, ask the server
// for the closing tag and insert it as a snippet. The request is the fork's
// own, so the language client has no feature for it — after the TypeScript 7
// extension's onAutoInsert.ts.
import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import { AUTO_INSERT, type AutoInsertCapabilities, type AutoInsertParams, type AutoInsertResult } from "./protocol";
import { SELECTOR } from "./selector";

export function registerAutoInsert(client: LanguageClient): vscode.Disposable {
  const capabilities = client.initializeResult?.capabilities as AutoInsertCapabilities | undefined;
  const triggers = new Set(capabilities?._vs_onAutoInsertProvider?._vs_triggerCharacters ?? []);
  if (triggers.size === 0) {
    return new vscode.Disposable(() => {});
  }
  let pending: vscode.CancellationTokenSource | undefined;

  const subscription = vscode.workspace.onDidChangeTextDocument(async ({ document, contentChanges, reason }) => {
    if (contentChanges.length === 0 || reason === vscode.TextDocumentChangeReason.Undo || reason === vscode.TextDocumentChangeReason.Redo) {
      return;
    }
    const editor = vscode.window.activeTextEditor;
    if (editor?.document !== document || vscode.languages.match(SELECTOR, document) === 0) {
      return;
    }
    if (!vscode.workspace.getConfiguration("reactogenic", document).get<boolean>("autoClosingTags", true)) {
      return;
    }
    // Typed, not pasted or replaced: one character, at the end of the last change.
    const change = contentChanges[contentChanges.length - 1];
    if (change.rangeLength > 0 || change.text.length !== 1 || !triggers.has(change.text)) {
      return;
    }
    // `>>` is not a tag end; the server decides the rest on the source tree.
    const start = change.range.start;
    if (start.character > 0 && triggers.has(document.getText(new vscode.Range(start.translate(0, -1), start)))) {
      return;
    }
    // The selection is not moved yet when this event fires.
    const position = start.translate(0, 1);
    pending?.cancel();
    pending?.dispose();
    const cancellation = new vscode.CancellationTokenSource();
    pending = cancellation;
    const version = document.version;

    let result: AutoInsertResult | null;
    try {
      result = await client.sendRequest<AutoInsertResult | null>(
        AUTO_INSERT,
        {
          _vs_textDocument: client.code2ProtocolConverter.asTextDocumentIdentifier(document),
          _vs_position: client.code2ProtocolConverter.asPosition(position),
          _vs_ch: change.text,
        } satisfies AutoInsertParams,
        cancellation.token,
      );
    } catch {
      return; // cancelled, or the server stopped
    }
    if (!result || cancellation.token.isCancellationRequested || document.version !== version || vscode.window.activeTextEditor !== editor) {
      return;
    }
    const edit = result._vs_textEdit;
    const range = client.protocol2CodeConverter.asRange(edit.range);
    // The same character typed at several cursors: the same tag at each.
    const cursors = editor.selections.map((s) => s.active);
    const targets = cursors.length > 1 && cursors.some((c) => c.isEqual(position)) ? cursors : range;
    const ignore = () => {};
    if (result._vs_textEditFormat === 2) {
      editor.insertSnippet(new vscode.SnippetString(edit.newText), targets).then(ignore, ignore);
    } else {
      editor
        .edit((builder) => {
          for (const target of Array.isArray(targets) ? targets : [targets]) {
            builder.replace(target, edit.newText);
          }
        })
        .then(ignore, ignore);
    }
  });

  return new vscode.Disposable(() => {
    subscription.dispose();
    pending?.cancel();
    pending?.dispose();
  });
}
