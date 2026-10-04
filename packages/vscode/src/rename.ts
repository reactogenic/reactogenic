// A rename's edits of files the server does not hold open (ide.md, *Rename*).
import * as path from "node:path";
import * as vscode from "vscode";
import { SELECTOR } from "./selector";

/**
 * The targets of `edit` that the editor has unsaved changes in and the server
 * is not attached to. The server holds `.rtsx` documents only and reads every
 * other file from disk (ide.md, *The `.ts` side*): its edit of a `.ts` file
 * is made for the saved text, and would land somewhere else in the buffer.
 */
export function unsavedTargets(edit: vscode.WorkspaceEdit, documents: readonly vscode.TextDocument[]): vscode.TextDocument[] {
  const targets = new Set(edit.entries().map(([uri]) => uri.toString()));
  return documents.filter((document) => document.isDirty && targets.has(document.uri.toString()) && vscode.languages.match(SELECTOR, document) === 0);
}

/** Refuses a rename with such a target: the message is what the editor shows. */
export function refuseUnsaved(edit: vscode.WorkspaceEdit, documents: readonly vscode.TextDocument[]): void {
  const unsaved = unsavedTargets(edit, documents).map((document) => path.basename(document.uri.fsPath));
  if (unsaved.length > 0) {
    const [has, it] = unsaved.length === 1 ? ["has", "it"] : ["have", "them"];
    throw new Error(`Rename refused: ${unsaved.join(", ")} ${has} unsaved changes, and the Reactogenic server reads ${it} from disk — its edits would land in the wrong place. Save ${it} and rename again.`);
  }
}
