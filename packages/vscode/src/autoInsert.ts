// Closing-tag insertion (ide.md, *Tags*): when `>` is typed, ask the server
// for the closing tag and insert it as a snippet. The request is the fork's
// own, so the language client has no feature for it — after the TypeScript 7
// extension's onAutoInsert.ts, which asks once and inserts that one answer
// at every cursor; here each cursor is asked for.
import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import { AUTO_INSERT, type AutoInsertCapabilities, type AutoInsertParams, type AutoInsertResult, typedEnds } from "./protocol";
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
    // Typed, not pasted or replaced: the same one character at every cursor.
    const ch = contentChanges[0].text;
    if (!triggers.has(ch) || contentChanges.some((change) => change.rangeLength > 0 || change.text !== ch)) {
      return;
    }
    // The selections are not moved yet when this event fires: where each
    // character ended is worked out from the changes.
    const cursors = typedEnds(contentChanges.map((change) => change.range.start)).map((end) => new vscode.Position(end.line, end.character));
    pending?.cancel();
    pending?.dispose();
    const cancellation = new vscode.CancellationTokenSource();
    pending = cancellation;
    const version = document.version;

    let answers: (AutoInsertResult | null)[];
    try {
      answers = await Promise.all(
        cursors.map((cursor) => {
          // `>>` is not a tag end; the server decides the rest on the source tree.
          if (cursor.character >= 2 && triggers.has(document.getText(new vscode.Range(cursor.translate(0, -2), cursor.translate(0, -1))))) {
            return null;
          }
          return client.sendRequest<AutoInsertResult | null>(
            AUTO_INSERT,
            {
              _vs_textDocument: client.code2ProtocolConverter.asTextDocumentIdentifier(document),
              _vs_position: client.code2ProtocolConverter.asPosition(cursor),
              _vs_ch: ch,
            } satisfies AutoInsertParams,
            cancellation.token,
          );
        }),
      );
    } catch {
      return; // cancelled, or the server stopped
    }
    if (answers.every((answer) => !answer) || cancellation.token.isCancellationRequested || document.version !== version || vscode.window.activeTextEditor !== editor) {
      return;
    }
    // One snippet edit per cursor, each with its own answer; a cursor without
    // one (it is not in a tag) gets the empty snippet, which keeps it a cursor.
    const snippets = answers.map((answer, i) => {
      if (!answer) {
        return new vscode.SnippetTextEdit(new vscode.Range(cursors[i], cursors[i]), new vscode.SnippetString("$0"));
      }
      const { range, newText } = answer._vs_textEdit;
      const snippet = answer._vs_textEditFormat === 2 ? new vscode.SnippetString(newText) : new vscode.SnippetString("$0").appendText(newText);
      return new vscode.SnippetTextEdit(client.protocol2CodeConverter.asRange(range), snippet);
    });
    const ignore = () => {};
    if (snippets.length === 1) {
      editor.insertSnippet(snippets[0].snippet, snippets[0].range).then(ignore, ignore);
      return;
    }
    const edit = new vscode.WorkspaceEdit();
    edit.set(document.uri, snippets);
    vscode.workspace.applyEdit(edit).then(ignore, ignore);
  });

  return new vscode.Disposable(() => {
    subscription.dispose();
    pending?.cancel();
    pending?.dispose();
  });
}
