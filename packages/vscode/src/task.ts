// The task *reactogenic: check* (ide.md, *Diagnostics*): project-wide errors,
// from the binary the server runs, into Problems — for closed documents. An
// open document's problems come from a server, and replace the task's for
// that file: the owner of a line's problem is the owner of that server's.
//
//   $reactogenic      .rtsx files     owner `reactogenic`: our server (server.ts)
//   $reactogenic-ts   every other     owner `typescript`, as `$tsc`: VS Code's TypeScript
import * as vscode from "vscode";
import { checkArgs, type CheckTask } from "./protocol";
import type { Server } from "./server";

export const MATCHERS = ["$reactogenic", "$reactogenic-ts"];

export function registerCheckTask(server: Server): vscode.Disposable {
  const task = (folder: vscode.WorkspaceFolder, definition: CheckTask): vscode.Task | undefined => {
    const command = server.binary();
    if (!command) {
      return undefined; // untrusted, or no binary: the status item says why
    }
    const execution = new vscode.ProcessExecution(command, checkArgs(definition), { cwd: folder.uri.fsPath });
    const check = new vscode.Task(definition, folder, "check", "reactogenic", execution, MATCHERS);
    check.group = vscode.TaskGroup.Build;
    check.detail = `reactogenic ${checkArgs(definition).join(" ")}`;
    check.presentationOptions = { clear: true, reveal: vscode.TaskRevealKind.Silent };
    return check;
  };
  return vscode.tasks.registerTaskProvider("reactogenic", {
    provideTasks: () =>
      (vscode.workspace.workspaceFolders ?? [])
        .filter((folder) => folder.uri.scheme === "file")
        .flatMap((folder) => task(folder, { type: "reactogenic", command: "check" }) ?? []),
    resolveTask: (given) => {
      const definition = given.definition as CheckTask;
      const folder = typeof given.scope === "object" ? given.scope : vscode.workspace.workspaceFolders?.[0];
      return definition.command === "check" && folder ? task(folder, definition) : undefined;
    },
  });
}
