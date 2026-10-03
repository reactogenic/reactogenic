// The language server of this window: one `reactogenic lsp --stdio` process,
// started for a trusted workspace only, and the status item that names it.
import * as path from "node:path";
import * as vscode from "vscode";
import { LanguageClient, type LanguageClientOptions, RevealOutputChannelOn, type ServerOptions, State, TransportKind } from "vscode-languageclient/node";
import { registerAutoInsert } from "./autoInsert";
import { type Binary, MIN_CLI_VERSION, type Resolution, resolveServer, runnable, statusOf } from "./resolve";
import { SELECTOR } from "./selector";

/** What a test, or a person reading the log, can know about the server. */
export interface ServerState {
  trusted: boolean;
  running: boolean;
  pid?: number;
  /** The row of the table that matched. */
  binary?: Binary;
  /** `serverInfo.version`. */
  version?: string;
  /** The status item. */
  status: { text: string; detail: string };
  /** Why a server that should run does not. */
  error?: string;
  /** How many servers this window has started. */
  starts: number;
}

export class Server implements vscode.Disposable {
  client: LanguageClient | undefined;
  private resolution: Resolution = { kind: "untrusted" };
  /** The binary as it runs: the resolved one, or its copy on Windows. */
  private command: string | undefined;
  private version: string | undefined;
  private error: string | undefined;
  private starts = 0;
  /** The folder of the first `.rtsx` document opened: where the walk to the workspace CLI starts. */
  private documentDir: string | undefined;
  private features: vscode.Disposable | undefined;
  private queue: Promise<void> = Promise.resolve();
  private readonly status: vscode.LanguageStatusItem;

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output: vscode.LogOutputChannel,
  ) {
    this.status = vscode.languages.createLanguageStatusItem("reactogenic.server", SELECTOR);
    this.status.name = "Reactogenic";
    this.status.command = { command: "reactogenic.restartServer", title: "Restart" };
  }

  /** Stops the server, resolves the binary again and starts it. Calls are serialized. */
  restart(reason: string): Promise<void> {
    this.queue = this.queue.then(async () => {
      await this.stop();
      await this.start(reason).catch((error) => this.failed(error instanceof Error ? error.message : String(error)));
    });
    return this.queue;
  }

  /** Settled when no start or stop is under way. */
  ready(): Promise<void> {
    return this.queue;
  }

  state(): ServerState {
    return {
      trusted: vscode.workspace.isTrusted,
      running: this.client?.state === State.Running,
      pid: this.client?.serverProcess?.pid,
      binary: this.resolution.kind === "binary" ? this.resolution.binary : undefined,
      version: this.version,
      status: { text: this.status.text, detail: this.status.detail ?? "" },
      error: this.error,
      starts: this.starts,
    };
  }

  /** The binary of the `check` task: the one the server runs. Undefined when there is none. */
  binary(): string | undefined {
    if (this.command) {
      return this.command;
    }
    const resolution = this.resolve();
    return resolution.kind === "binary" ? runnable(resolution.binary, this.copies()) : undefined;
  }

  /** The first `.rtsx` document decides which workspace CLI runs, for the life of the window. */
  opened(document: vscode.TextDocument): void {
    if (this.documentDir !== undefined || document.languageId !== "rtsx" || document.uri.scheme !== "file") {
      return;
    }
    this.documentDir = path.dirname(document.uri.fsPath);
    const next = this.resolve();
    const now = this.resolution;
    if (next.kind !== now.kind || (next.kind === "binary" && now.kind === "binary" && next.binary.path !== now.binary.path)) {
      void this.restart("first .rtsx document opened");
    }
  }

  private resolve(): Resolution {
    const folder = vscode.workspace.workspaceFolders?.find((f) => f.uri.scheme === "file")?.uri.fsPath;
    return resolveServer({
      trusted: vscode.workspace.isTrusted,
      settingPath: vscode.workspace.getConfiguration("reactogenic").get<string | null>("server.path"),
      workspaceFolder: folder,
      documentDir: this.documentDir ?? folder,
      extensionPath: this.context.extensionPath,
    });
  }

  private copies(): string {
    return path.join(this.context.globalStorageUri.fsPath, "server");
  }

  private async start(reason: string): Promise<void> {
    this.error = undefined;
    this.version = undefined;
    this.command = undefined;
    for (const document of vscode.workspace.textDocuments) {
      if (this.documentDir === undefined && document.languageId === "rtsx" && document.uri.scheme === "file") {
        this.documentDir = path.dirname(document.uri.fsPath);
      }
    }
    const resolution = this.resolve();
    this.resolution = resolution;
    this.show();
    if (resolution.kind === "untrusted") {
      this.output.info(`${reason}: untrusted workspace — syntax highlighting only, no server process`);
      return;
    }
    for (const skipped of resolution.skipped) {
      this.output.info(`not ${skipped.source}: ${skipped.reason}`);
    }
    if (resolution.kind === "none") {
      this.output.warn(resolution.message); // the status item says it; no notification
      return;
    }
    if (resolution.kind === "error") {
      this.failed(resolution.message);
      return;
    }
    const command = runnable(resolution.binary, this.copies());
    this.command = command;
    const cwd = vscode.workspace.workspaceFolders?.find((f) => f.uri.scheme === "file")?.uri.fsPath ?? this.documentDir;
    this.output.info(`${reason}: starting ${command} lsp --stdio (${resolution.binary.source})`);

    const options: LanguageClientOptions = {
      documentSelector: SELECTOR,
      outputChannel: this.output,
      revealOutputChannelOn: RevealOutputChannelOn.Never,
      // The `$reactogenic` problem matcher has the same owner: an opened
      // document's diagnostics replace what the check task left for it.
      diagnosticCollectionProvider: {
        create: (_name, source) => vscode.languages.createDiagnosticCollection(source === "pull" ? "reactogenic" : "reactogenic.project"),
        dispose: (collection) => collection.dispose(),
      },
    };
    const server: ServerOptions = { command, args: ["lsp", "--stdio"], transport: TransportKind.stdio, options: { cwd } };
    const client = new LanguageClient("reactogenic", "Reactogenic", server, options);
    this.client = client;
    this.starts++;
    this.status.busy = true;
    try {
      await client.start();
    } catch (error) {
      // A binary that exits at once: most likely one from before `lsp`.
      const reason = error instanceof Error ? error.message : String(error);
      throw new Error(`${command} lsp --stdio did not start (${reason}). A reactogenic older than ${MIN_CLI_VERSION} has no language server.`);
    } finally {
      this.status.busy = false;
    }
    this.version = client.initializeResult?.serverInfo?.version;
    this.features = registerAutoInsert(client);
    this.show();
    this.output.info(`running: ${this.status.text} (${this.status.detail})`);
  }

  private async stop(): Promise<void> {
    const client = this.client;
    this.client = undefined;
    this.features?.dispose();
    this.features = undefined;
    if (client) {
      // Both refuse on a client that never started; then there is nothing to stop.
      await client.stop().catch(() => {});
      await client.dispose().catch(() => {});
    }
  }

  private failed(message: string): void {
    const client = this.client;
    this.client = undefined;
    void client?.dispose().catch(() => {});
    this.error = message;
    this.output.error(message);
    this.show();
    void vscode.window.showErrorMessage(`Reactogenic: ${message}`);
  }

  private show(): void {
    const status = this.error
      ? { severity: "error" as const, text: "reactogenic: no server", detail: this.error }
      : statusOf(this.resolution, this.version);
    this.status.text = status.text;
    this.status.detail = status.detail;
    this.status.severity = SEVERITY[status.severity];
  }

  /** For `deactivate`: the process is gone when this settles. */
  shutdown(): Promise<void> {
    this.queue = this.queue.then(() => this.stop());
    return this.queue;
  }

  dispose(): void {
    this.status.dispose();
    void this.shutdown();
  }
}

const SEVERITY = {
  info: vscode.LanguageStatusSeverity.Information,
  warning: vscode.LanguageStatusSeverity.Warning,
  error: vscode.LanguageStatusSeverity.Error,
};
