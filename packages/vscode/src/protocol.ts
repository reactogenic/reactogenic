// What the extension says to the binary beyond standard LSP. No `vscode`
// import: unit-tested in plain Node.

/** The emitted TSX of a document (ide.md, *Commands*). */
export const TRANSPILED = "reactogenic/transpiled";
export interface TranspiledParams {
  textDocument: { uri: string };
}
export interface TranspiledResult {
  text: string;
  /** The tolerance step that produced the text. */
  step: string;
}

/** Closing-tag insertion (ide.md, *Tags*): the fork's request, which the language client does not know. */
export const AUTO_INSERT = "textDocument/_vs_onAutoInsert";
export interface AutoInsertParams {
  _vs_textDocument: { uri: string };
  _vs_position: { line: number; character: number };
  _vs_ch: string;
}
export interface AutoInsertResult {
  /** 1: plain text, 2: a snippet. */
  _vs_textEditFormat: number;
  _vs_textEdit: { range: { start: { line: number; character: number }; end: { line: number; character: number } }; newText: string };
}
export interface AutoInsertCapabilities {
  _vs_onAutoInsertProvider?: { _vs_triggerCharacters?: string[] };
}

interface Position {
  line: number;
  character: number;
}

/**
 * Where each typed character ends once the change is applied. `starts` are
 * where one character was typed at each cursor, in the document before the
 * change — as a change event gives them, in any order: a character typed
 * earlier on the same line moves the later ones.
 */
export function typedEnds(starts: readonly Position[]): Position[] {
  return starts.map((start) => ({
    line: start.line,
    character: start.character + 1 + starts.filter((other) => other.line === start.line && other.character < start.character).length,
  }));
}

/**
 * Does this JSON-RPC error code mean "the server does not have the request"?
 * MethodNotFound (-32601) by the protocol; the fork's dispatcher answers an
 * unknown method with InvalidRequest (-32600).
 */
export function isUnsupported(code: unknown): boolean {
  return code === -32601 || code === -32600;
}

/** The definition of the task *reactogenic: check*. */
export interface CheckTask {
  type: "reactogenic";
  command: "check";
  /** A tsconfig.json, or a directory holding one. */
  project?: string;
}

/** `check --pretty=false`: the lines the `$reactogenic` problem matcher reads. */
export function checkArgs(task: Pick<CheckTask, "project">): string[] {
  return ["check", "--pretty=false", ...(task.project ? ["-p", task.project] : [])];
}
