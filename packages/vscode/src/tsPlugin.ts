// What the window tells the TS server plugin (src/plugin, ide.md, *The
// plugin*): the `reactogenic.server.path` setting; whether the workspace is
// trusted — without which the plugin never runs anything of the workspace's;
// that a lockfile changed, so that it looks for the workspace's CLI again;
// and whether the language server lists workspace symbols — while it does
// not, the plugin lists those of `.rtsx` files. The plugin is loaded by VS Code's
// TypeScript, often before this extension is activated; until it hears from
// here it runs `$REACTOGENIC_BINARY` or the binary in the .vsix.
import * as vscode from "vscode";
import type { Config } from "./plugin/virtual";

/** As in package.json, `typescriptServerPlugins`, and scripts/build.mjs. */
export const PLUGIN = "reactogenic-typescript-plugin";

interface TypeScriptApi {
  configurePlugin(pluginId: string, configuration: Config): void;
}

/** What of the window's language server the plugin is told. */
export interface LanguageServer {
  /** How often a lockfile of the workspace has changed. */
  installs: number;
  /** It runs and has a project loaded: the workspace symbols of `.rtsx` files are its to list. */
  listsSymbols(): boolean;
}

/** The configuration as the window is now. */
export function pluginConfig(server: LanguageServer): Config {
  const folder = vscode.workspace.workspaceFolders?.find((f) => f.uri.scheme === "file");
  return {
    serverPath: vscode.workspace.getConfiguration("reactogenic").get<string | null>("server.path") ?? null,
    workspaceFolder: folder?.uri.fsPath,
    trusted: vscode.workspace.isTrusted,
    installs: server.installs,
    languageServer: server.listsSymbols(),
  };
}

export class TsPlugin {
  private sent: string | undefined;
  /** What tsserver was told last. */
  config: Config | undefined;

  constructor(
    private readonly output: vscode.LogOutputChannel,
    private readonly server: LanguageServer,
  ) {}

  /**
   * Sends the configuration to tsserver, now and for every tsserver started
   * later (VS Code's TypeScript keeps the last one). Called at every event
   * that may change it; sent when it has.
   */
  async configure(): Promise<void> {
    // The built-in TypeScript extension; absent when the user disabled it.
    const typescript = vscode.extensions.getExtension<{ getAPI?(version: 0): TypeScriptApi | undefined }>("vscode.typescript-language-features");
    if (!typescript) {
      return;
    }
    try {
      const api = (await typescript.activate()).getAPI?.(0);
      const config = pluginConfig(this.server); // as it is now: the server may have started meanwhile
      const text = JSON.stringify(config);
      if (!api || text === this.sent) {
        return;
      }
      this.sent = text;
      this.config = config;
      api.configurePlugin(PLUGIN, config);
      this.output.info(`TypeScript plugin: ${text}`);
    } catch (error) {
      this.output.warn(`TypeScript plugin not configured: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
}
