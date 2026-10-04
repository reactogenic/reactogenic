// The VS Code extension `reactogenic.rtsx` (ide.md, *VS Code extension*): the
// declarative half is package.json; this is the client that runs
// `reactogenic lsp` for the window.
import * as vscode from "vscode";
import type { Config } from "./plugin/virtual";
import { LOCKFILES } from "./resolve";
import { Server, type ServerState } from "./server";
import { registerCheckTask } from "./task";
import { Transpiled } from "./transpiled";
import { TsPlugin } from "./tsPlugin";

/** What `activate` returns: the editor suite reads it. */
export interface Api {
  /** The server's state, once no start or stop is under way. */
  ready(): Promise<ServerState>;
  /** What the TS server plugin was told last (src/tsPlugin.ts). */
  tsPlugin(): Config | undefined;
}

let server: Server | undefined;

export function activate(context: vscode.ExtensionContext): Api {
  const output = vscode.window.createOutputChannel("Reactogenic", { log: true });
  const running = new Server(context, output);
  server = running;
  const transpiled = new Transpiled(running);
  // The `.ts` side: VS Code's TypeScript runs the plugin, which learns from
  // here the setting, the trust, that a lockfile changed, and whether the server runs.
  const plugin = new TsPlugin(output, running);

  // The lockfiles inside the workspace folders; those above them are the server's to watch.
  const lockfile = (uri: vscode.Uri) => running.lockfile(uri);
  const lockfiles = vscode.workspace.createFileSystemWatcher(`**/{${LOCKFILES.join(",")}}`);

  context.subscriptions.push(
    output,
    running,
    transpiled,
    lockfiles,
    lockfiles.onDidChange(lockfile),
    lockfiles.onDidCreate(lockfile),
    lockfiles.onDidDelete(lockfile),
    vscode.commands.registerCommand("reactogenic.restartServer", () => running.restart("Restart Server")),
    vscode.commands.registerCommand("reactogenic.showTranspiled", () => transpiled.show()),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("reactogenic.server")) {
        void running.restart("reactogenic.server.* changed");
        void plugin.configure();
      }
    }),
    // Untrusted: highlighting only. The server starts when trust is granted.
    vscode.workspace.onDidGrantWorkspaceTrust(() => {
      void running.restart("workspace trusted");
      void plugin.configure();
    }),
    vscode.workspace.onDidChangeWorkspaceFolders(() => void plugin.configure()),
    running.onDidChange(() => void plugin.configure()),
    vscode.workspace.onDidOpenTextDocument((document) => {
      running.opened(document);
      void plugin.configure(); // the first `.rtsx` document: the server has a project now
    }),
    vscode.workspace.onDidCloseTextDocument(() => void plugin.configure()),
    registerCheckTask(running),
  );

  void running.restart("activated");
  void plugin.configure();
  return { ready: () => running.ready().then(() => running.state()), tsPlugin: () => plugin.config };
}

export function deactivate(): Promise<void> | undefined {
  return server?.shutdown();
}
