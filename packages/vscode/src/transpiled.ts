// *Show Transpiled TSX* (ide.md, *Commands*): the emitted TSX of the active
// `.rtsx` document, read-only beside it, refreshed on every edit.
import * as vscode from "vscode";
import { ResponseError } from "vscode-languageclient/node";
import { isUnsupported, TRANSPILED, type TranspiledParams, type TranspiledResult } from "./protocol";
import type { Server } from "./server";

const SCHEME = "reactogenic-transpiled";

/** What the command did: for the caller, and for the editor suite. */
export type Shown = "shown" | "unsupported" | "no-document" | "no-server" | "failed";

export class Transpiled implements vscode.TextDocumentContentProvider, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<vscode.Uri>();
  readonly onDidChange = this.changed.event;
  /** Virtual URI → the last answer for it. */
  private readonly results = new Map<string, TranspiledResult>();
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly step: vscode.StatusBarItem;
  private readonly subscriptions: vscode.Disposable[];

  constructor(private readonly server: Server) {
    this.step = vscode.window.createStatusBarItem("reactogenic.transpiled", vscode.StatusBarAlignment.Right, 100);
    this.step.name = "Reactogenic: transpiled TSX";
    this.subscriptions = [
      this.changed,
      this.step,
      vscode.workspace.registerTextDocumentContentProvider(SCHEME, this),
      vscode.workspace.onDidChangeTextDocument((e) => this.refresh(e.document.uri)),
      vscode.workspace.onDidCloseTextDocument((d) => d.uri.scheme === SCHEME && this.results.delete(d.uri.toString())),
      vscode.window.onDidChangeActiveTextEditor(() => this.showStep()),
    ];
  }

  /** The command. */
  async show(): Promise<Shown> {
    const source = vscode.window.activeTextEditor?.document;
    if (source?.languageId !== "rtsx") {
      return "no-document";
    }
    if (!this.server.client) {
      void vscode.window.showWarningMessage("Reactogenic: the language server is not running.");
      return "no-server";
    }
    const target = virtual(source.uri);
    try {
      this.results.set(target.toString(), await this.request(source.uri));
    } catch (error) {
      if (error instanceof ResponseError && isUnsupported(error.code)) {
        const version = this.server.state().version;
        void vscode.window.showWarningMessage(
          `Reactogenic: the language server${version ? ` (${version})` : ""} is too old for Show Transpiled TSX. Update @reactogenic/cli or the extension.`,
        );
        return "unsupported";
      }
      void vscode.window.showErrorMessage(`Reactogenic: ${error instanceof Error ? error.message : String(error)}`);
      return "failed";
    }
    this.changed.fire(target); // a document already open shows the new text
    const document = await vscode.workspace.openTextDocument(target);
    await vscode.window.showTextDocument(document, { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true, preview: true });
    this.showStep();
    return "shown";
  }

  provideTextDocumentContent(uri: vscode.Uri): string {
    return this.results.get(uri.toString())?.text ?? "";
  }

  private request(source: vscode.Uri): Promise<TranspiledResult> {
    const client = this.server.client;
    if (!client) {
      return Promise.reject(new Error("the language server is not running"));
    }
    return client.sendRequest<TranspiledResult>(TRANSPILED, { textDocument: { uri: client.code2ProtocolConverter.asUri(source) } } satisfies TranspiledParams);
  }

  /** An edit to a source whose TSX is open: ask again, once the typing pauses. */
  private refresh(source: vscode.Uri): void {
    const target = virtual(source);
    const key = target.toString();
    if (!this.results.has(key)) {
      return;
    }
    clearTimeout(this.timers.get(key));
    this.timers.set(
      key,
      setTimeout(async () => {
        this.timers.delete(key);
        try {
          const result = await this.request(source);
          if (this.results.has(key)) {
            this.results.set(key, result);
            this.changed.fire(target);
            this.showStep();
          }
        } catch {
          // The server restarted or the document closed: the last text stays.
        }
      }, 150),
    );
  }

  private showStep(): void {
    const active = vscode.window.visibleTextEditors.find((e) => e.document.uri.scheme === SCHEME);
    const result = active && this.results.get(active.document.uri.toString());
    if (result) {
      this.step.text = `$(file-code) TSX: ${result.step}`;
      this.step.tooltip = "The tolerance step that produced the transpiled TSX";
      this.step.show();
    } else {
      this.step.hide();
    }
  }

  dispose(): void {
    for (const timer of this.timers.values()) {
      clearTimeout(timer);
    }
    vscode.Disposable.from(...this.subscriptions).dispose();
  }
}

/** `page.rtsx` → `reactogenic-transpiled:…/page.rtsx.tsx`: the tab reads as TSX and is highlighted as TSX. */
function virtual(source: vscode.Uri): vscode.Uri {
  return vscode.Uri.from({ scheme: SCHEME, path: `${source.path}.tsx`, query: source.toString() });
}
