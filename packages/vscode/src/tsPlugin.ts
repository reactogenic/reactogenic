// What the window tells the TS server plugin (src/plugin, ide.md, *The
// plugin*): the `reactogenic.server.path` setting, and whether the workspace
// is trusted — without which the plugin never runs the workspace's own CLI.
// The plugin is loaded by VS Code's TypeScript, often before this extension is
// activated; until it hears from here it runs `$REACTOGENIC_BINARY` or the
// binary in the .vsix.
import * as vscode from "vscode";
import type { Config } from "./plugin/virtual";

/** As in package.json, `typescriptServerPlugins`, and scripts/build.mjs. */
export const PLUGIN = "reactogenic-typescript-plugin";

interface TypeScriptApi {
  configurePlugin(pluginId: string, configuration: Config): void;
}

/** Sends the configuration to tsserver, now and for every tsserver started later. */
export async function configureTsPlugin(output: vscode.LogOutputChannel): Promise<void> {
  // The built-in TypeScript extension; absent when the user disabled it.
  const typescript = vscode.extensions.getExtension<{ getAPI?(version: 0): TypeScriptApi | undefined }>("vscode.typescript-language-features");
  if (!typescript) {
    return;
  }
  const folder = vscode.workspace.workspaceFolders?.find((f) => f.uri.scheme === "file");
  const config: Config = {
    serverPath: vscode.workspace.getConfiguration("reactogenic").get<string | null>("server.path") ?? null,
    workspaceFolder: folder?.uri.fsPath,
    trusted: vscode.workspace.isTrusted,
  };
  try {
    const api = (await typescript.activate()).getAPI?.(0);
    api?.configurePlugin(PLUGIN, config);
    output.info(`TypeScript plugin: ${JSON.stringify(config)}`);
  } catch (error) {
    output.warn(`TypeScript plugin not configured: ${error instanceof Error ? error.message : String(error)}`);
  }
}
