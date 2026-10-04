// The language server of this window: one `reactogenic lsp --stdio` process,
// started for a trusted workspace only, and the status item that names it.
import { type ChildProcess, spawn } from "node:child_process";
import * as path from "node:path";
import * as vscode from "vscode";
import { CloseAction, type ErrorHandler, LanguageClient, type LanguageClientOptions, RevealOutputChannelOn, State } from "vscode-languageclient/node";
import { registerAutoInsert } from "./autoInsert";
import { refuseUnsaved } from "./rename";
import { type Binary, decidingDir, LOCKFILES, lockfileDirs, MIN_CLI_VERSION, type Resolution, resolveServer, runnable, statusOf } from "./resolve";
import { SELECTOR } from "./selector";

/** How long a binary has to answer `initialize`: one that has not by then is not a language server. */
const START_TIMEOUT = 10_000;
/** How long a server has to exit after `shutdown` and `exit`, as the language client gives one it started. */
const EXIT_TIMEOUT = 2_000;

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
  /**
   * The client's server process. We start it, not the client, so that it
   * can be stopped at any moment — also one that never answers `initialize`,
   * which the client cannot stop.
   */
  private process: ChildProcess | undefined;
  private resolution: Resolution = { kind: "untrusted" };
  /** The binary as it runs: the resolved one, or its copy on Windows. */
  private command: string | undefined;
  private version: string | undefined;
  private error: string | undefined;
  private starts = 0;
  /** The folder of the `.rtsx` document that chose the workspace CLI (`decidingDir`); chosen again at each start. */
  private documentDir: string | undefined;
  private features: vscode.Disposable | undefined;
  private queue: Promise<void> = Promise.resolve();
  /** The client whose first start is under way, and how to give that start up. */
  private starting: { client: LanguageClient; abandon: () => void } | undefined;
  /** Watches of the lockfiles above the workspace folders; renewed at each start. */
  private above: vscode.Disposable[] = [];
  private settle: ReturnType<typeof setTimeout> | undefined;
  private readonly status: vscode.LanguageStatusItem;
  /**
   * The problems of open documents, for the life of the window. The
   * `$reactogenic` problem matcher has the same owner, so that an opened
   * document's diagnostics replace what the check task left for it — and a
   * collection, when disposed, clears every problem of its owner: the task's
   * for closed documents too. So it is never disposed (`release`).
   */
  private readonly problems = vscode.languages.createDiagnosticCollection("reactogenic");

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output: vscode.LogOutputChannel,
  ) {
    this.status = vscode.languages.createLanguageStatusItem("reactogenic.server", SELECTOR);
    this.status.name = "Reactogenic";
    this.status.command = { command: "reactogenic.restartServer", title: "Restart" };
  }

  /**
   * Stops the server, resolves the binary again and starts it. Calls are
   * serialized; a start that has not answered yet is given up, not waited for.
   */
  restart(reason: string): Promise<void> {
    this.starting?.abandon();
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
      pid: this.process && alive(this.process) ? this.process.pid : undefined,
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

  /**
   * The first `.rtsx` document that may choose the workspace CLI does, until
   * the next start: the server does not move when another one is opened.
   */
  opened(document: vscode.TextDocument): void {
    if (this.documentDir !== undefined || !isSource(document)) {
      return;
    }
    const dir = decidingDir(this.folders(), [document.uri.fsPath]);
    if (dir === undefined) {
      return; // outside every workspace folder
    }
    this.documentDir = dir;
    const next = this.resolve();
    const now = this.resolution;
    if (next.kind !== now.kind || (next.kind === "binary" && now.kind === "binary" && next.binary.path !== now.binary.path)) {
      void this.restart("first .rtsx document opened");
    }
  }

  /** A lockfile changed — more than once during an install: restart when it settles. */
  lockfile(uri: vscode.Uri): void {
    if (uri.path.includes("/node_modules/")) {
      return;
    }
    clearTimeout(this.settle);
    this.settle = setTimeout(() => void this.restart(`${path.basename(uri.fsPath)} changed`), 1500);
  }

  private folders(): string[] {
    return (vscode.workspace.workspaceFolders ?? []).filter((f) => f.uri.scheme === "file").map((f) => f.uri.fsPath);
  }

  /** The `.rtsx` files open in the window: the active one, the visible ones, the rest. */
  private documents(): string[] {
    const shown = [vscode.window.activeTextEditor, ...vscode.window.visibleTextEditors].flatMap((editor) => (editor ? [editor.document] : []));
    return [...shown, ...vscode.workspace.textDocuments].filter(isSource).map((document) => document.uri.fsPath);
  }

  /** Where the walk to the workspace's CLI starts. */
  private walkStart(): string | undefined {
    return this.documentDir ?? this.folders()[0];
  }

  private resolve(): Resolution {
    return resolveServer({
      trusted: vscode.workspace.isTrusted,
      settingPath: vscode.workspace.getConfiguration("reactogenic").get<string | null>("server.path"),
      workspaceFolder: this.folders()[0],
      documentDir: this.walkStart(),
      extensionPath: this.context.extensionPath,
    });
  }

  private copies(): string {
    return path.join(this.context.globalStorageUri.fsPath, "server");
  }

  /**
   * The lockfiles inside the workspace folders are watched by the window
   * (extension.ts). The CLI and its lockfile may be above the opened folder —
   * a package of a monorepo: those directories are watched here.
   */
  private watch(): void {
    vscode.Disposable.from(...this.above).dispose();
    this.above = [];
    const start = this.walkStart();
    if (!vscode.workspace.isTrusted || start === undefined) {
      return;
    }
    for (const dir of lockfileDirs(start)) {
      if (vscode.workspace.getWorkspaceFolder(vscode.Uri.file(dir))) {
        continue;
      }
      const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(vscode.Uri.file(dir), `{${LOCKFILES.join(",")}}`));
      const changed = (uri: vscode.Uri) => this.lockfile(uri);
      this.above.push(watcher, watcher.onDidChange(changed), watcher.onDidCreate(changed), watcher.onDidDelete(changed));
      this.output.info(`watching the lockfiles of ${dir}`);
    }
  }

  private async start(reason: string): Promise<void> {
    this.error = undefined;
    this.version = undefined;
    this.command = undefined;
    this.documentDir = decidingDir(this.folders(), this.documents());
    const resolution = this.resolve();
    this.resolution = resolution;
    this.watch();
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
    const cwd = this.folders()[0] ?? this.documentDir;
    this.output.info(`${reason}: starting ${command} lsp --stdio (${resolution.binary.source})`);

    // Called by the client at each of its starts — ours, and its own after a crash.
    const launch = (): Promise<ChildProcess> => {
      const child = spawn(command, ["lsp", "--stdio"], { cwd, windowsHide: true });
      if (child.pid === undefined) {
        return new Promise((_, reject) => child.once("error", reject)); // not there, or not executable
      }
      if (this.client !== client) {
        child.kill("SIGKILL");
        return Promise.reject(new Error("this client was given up"));
      }
      this.process = child;
      return Promise.resolve(child);
    };
    // What the client does when the server's process ends. A crash of a
    // running server: its own default — a few restarts, then it gives up.
    // A process that ends while we start it, or after we gave the client up,
    // is not restarted: `start` reports the first, nobody waits for the second.
    let handler: ErrorHandler | undefined;
    const fallback = () => {
      handler ??= client.createDefaultErrorHandler();
      return handler;
    };
    const options: LanguageClientOptions = {
      documentSelector: SELECTOR,
      outputChannel: this.output,
      revealOutputChannelOn: RevealOutputChannelOn.Never,
      errorHandler: {
        error: (error, message, count) => fallback().error(error, message, count),
        closed: () => (this.client !== client || this.starting?.client === client ? { action: CloseAction.DoNotRestart, handled: true } : fallback().closed()),
      },
      middleware: {
        // The server's edit of a file it reads from disk is for the saved text.
        provideRenameEdits: async (document, position, newName, token, next) => {
          const edit = await next(document, position, newName, token);
          if (edit) {
            refuseUnsaved(edit, vscode.workspace.textDocuments);
          }
          return edit;
        },
      },
      diagnosticCollectionProvider: {
        create: (_name, source) => (source === "pull" ? this.problems : vscode.languages.createDiagnosticCollection("reactogenic.project")),
        dispose: (collection) => (collection === this.problems ? this.release() : collection.dispose()),
      },
    };
    const client = new LanguageClient("reactogenic", "Reactogenic", launch, options);
    client.onDidChangeState(({ newState }) => this.changed(client, newState));
    this.client = client;
    this.starts++;
    this.status.busy = true;

    let interrupt: (how: "abandoned" | "timeout") => void = () => {};
    const interrupted = new Promise<"abandoned" | "timeout">((resolve) => {
      interrupt = resolve;
    });
    const timer = setTimeout(() => interrupt("timeout"), START_TIMEOUT);
    this.starting = { client, abandon: () => interrupt("abandoned") };
    // The client's `start` also settles without an error when the process
    // ends before it answers and the client is told not to restart it.
    const started = client.start().then(() => {
      if (client.state !== State.Running) {
        throw new Error("its process ended before it answered");
      }
      return "running" as const;
    });
    let outcome: "running" | "abandoned" | "timeout";
    try {
      outcome = await Promise.race([started, interrupted]);
    } catch (error) {
      // A binary that exits at once: most likely one from before `lsp`.
      const reason = error instanceof Error ? error.message : String(error);
      throw new Error(`${command} lsp --stdio did not start (${reason}). A reactogenic older than ${MIN_CLI_VERSION} has no language server.`);
    } finally {
      clearTimeout(timer);
      this.starting = undefined;
      this.status.busy = false;
    }
    if (outcome === "abandoned") {
      this.discard();
      this.output.info(`${command} lsp --stdio had not answered yet: given up for the next start`);
      return;
    }
    if (outcome === "timeout") {
      throw new Error(`${command} lsp --stdio did not answer within ${START_TIMEOUT / 1000} s: it is not a language server. Its process was stopped.`);
    }
    this.version = client.initializeResult?.serverInfo?.version;
    this.features = registerAutoInsert(client);
    this.show();
    this.output.info(`running: ${this.status.text} (${this.status.detail})`);
  }

  private async stop(): Promise<void> {
    const client = this.client;
    const child = this.process;
    this.client = undefined;
    this.process = undefined;
    this.features?.dispose();
    this.features = undefined;
    if (client) {
      // Both refuse on a client that is not running; then there is nothing to ask.
      await client.stop().catch(() => {});
      await client.dispose().catch(() => {});
    }
    if (child && alive(child)) {
      // It was asked to exit, and does within a moment — or it is made to.
      const timer = setTimeout(() => child.kill("SIGKILL"), EXIT_TIMEOUT);
      child.once("exit", () => clearTimeout(timer));
    }
  }

  /** Gives up the client outside `stop`: it failed to start, or its process does not answer. */
  private discard(): void {
    const client = this.client;
    const child = this.process;
    this.client = undefined;
    this.process = undefined;
    // Refuses on a client that is not running, and marks it as not to be started again.
    void client?.dispose().catch(() => {});
    if (child && alive(child)) {
      child.kill("SIGKILL"); // no `shutdown` to ask for
    }
  }

  /**
   * The client's state changed by itself: the server crashed. The client
   * restarts it — the status item is busy meanwhile — until it has crashed
   * too often; then nothing runs, and the status item must not name a server.
   */
  private changed(client: LanguageClient, state: State): void {
    if (this.client !== client || this.starting?.client === client) {
      return; // a start or a stop of ours: `start`, `stop` and `failed` report it
    }
    this.status.busy = state === State.Starting;
    if (state === State.Running) {
      this.version = client.initializeResult?.serverInfo?.version;
    }
    this.error =
      state === State.Running || state === State.Starting
        ? undefined
        : "The server stopped and was not started again (see the Reactogenic output). Run Reactogenic: Restart Server.";
    this.show();
  }

  /** What the client gets in place of disposing `problems`: its own entries go, the check task's stay. */
  private release(): void {
    const uris: vscode.Uri[] = [];
    this.problems.forEach((uri) => {
      uris.push(uri);
    });
    for (const uri of uris) {
      this.problems.delete(uri);
    }
  }

  private failed(message: string): void {
    this.discard();
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

  /** For `deactivate`: the process is gone, or on its way out, when this settles. */
  shutdown(): Promise<void> {
    this.starting?.abandon();
    this.queue = this.queue.then(() => this.stop());
    return this.queue;
  }

  dispose(): void {
    clearTimeout(this.settle);
    vscode.Disposable.from(...this.above).dispose();
    this.status.dispose();
    void this.shutdown();
  }
}

/** A document that can choose the workspace CLI: an `.rtsx` file on disk. */
function isSource(document: vscode.TextDocument): boolean {
  return document.languageId === "rtsx" && document.uri.scheme === "file";
}

function alive(child: ChildProcess): boolean {
  return child.exitCode === null && child.signalCode === null;
}

const SEVERITY = {
  info: vscode.LanguageStatusSeverity.Information,
  warning: vscode.LanguageStatusSeverity.Warning,
  error: vscode.LanguageStatusSeverity.Error,
};
