// The VS Code extension `reactogenic.rtsx` (ide.md, *VS Code extension*): the
// declarative half is package.json; this is the client that runs
// `reactogenic lsp` for the window.
import * as path from "node:path";
import * as vscode from "vscode";
import { LOCKFILES } from "./resolve";
import { Server, type ServerState } from "./server";
import { registerCheckTask } from "./task";
import { Transpiled } from "./transpiled";

/** What `activate` returns: the editor suite reads it. */
export interface Api {
  /** The server's state, once no start or stop is under way. */
  ready(): Promise<ServerState>;
}

let server: Server | undefined;

export function activate(context: vscode.ExtensionContext): Api {
  const output = vscode.window.createOutputChannel("Reactogenic", { log: true });
  const running = new Server(context, output);
  server = running;
  const transpiled = new Transpiled(running);

  // A lockfile changes more than once during an install: restart when it settles.
  let settle: ReturnType<typeof setTimeout> | undefined;
  const lockfile = (uri: vscode.Uri) => {
    if (uri.path.includes("/node_modules/")) {
      return;
    }
    clearTimeout(settle);
    settle = setTimeout(() => void running.restart(`${path.basename(uri.fsPath)} changed`), 1500);
  };
  const lockfiles = vscode.workspace.createFileSystemWatcher(`**/{${LOCKFILES.join(",")}}`);

  context.subscriptions.push(
    output,
    running,
    transpiled,
    lockfiles,
    lockfiles.onDidChange(lockfile),
    lockfiles.onDidCreate(lockfile),
    lockfiles.onDidDelete(lockfile),
    new vscode.Disposable(() => clearTimeout(settle)),
    vscode.commands.registerCommand("reactogenic.restartServer", () => running.restart("Restart Server")),
    vscode.commands.registerCommand("reactogenic.showTranspiled", () => transpiled.show()),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("reactogenic.server")) {
        void running.restart("reactogenic.server.* changed");
      }
    }),
    // Untrusted: highlighting only. The server starts when trust is granted.
    vscode.workspace.onDidGrantWorkspaceTrust(() => void running.restart("workspace trusted")),
    vscode.workspace.onDidOpenTextDocument((document) => running.opened(document)),
    registerCheckTask(running),
  );

  void running.restart("activated");
  return { ready: () => running.ready().then(() => running.state()) };
}

export function deactivate(): Promise<void> | undefined {
  return server?.shutdown();
}
