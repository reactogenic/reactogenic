// A real tsserver over stdio, with the plugin loaded as VS Code's TypeScript
// extension loads it: by name (--globalPlugins) from the extension's folder
// (--pluginProbeLocations), where scripts/build.mjs puts it.
import { type ChildProcessWithoutNullStreams, spawn } from "node:child_process";
import fs from "node:fs";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";

const require = createRequire(import.meta.url);
export const root = path.resolve(import.meta.dirname, "..");
export const PLUGIN = "reactogenic-typescript-plugin";

/** The TypeScript versions the plugin is tested with: npm aliases in package.json. */
export const VERSIONS = ["typescript-5.9", "typescript-6.0"] as const;

export function versionOf(alias: string): string {
  return require(`${alias}/package.json`).version;
}

/** A project on disk: `files` by relative path, written into a fresh folder. */
export function project(files: Record<string, string>): string {
  // The real path: tsserver names files by it, and macOS's temporary folder is a link.
  const dir = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "rtsx-plugin-")));
  for (const [name, text] of Object.entries(files)) {
    write(dir, name, text);
  }
  return dir;
}

export function write(dir: string, name: string, text: string): void {
  fs.mkdirSync(path.dirname(path.join(dir, name)), { recursive: true });
  fs.writeFileSync(path.join(dir, name), text);
}

/**
 * A stand-in for the binary that counts its processes: it appends a line to
 * `<file>.count` and then is `binary`. `count()` is how many ran. While
 * `<file>.fail` exists it fails instead.
 */
export function counting(dir: string, binary: string, name = "reactogenic"): { path: string; count(): number } {
  const file = path.join(dir, name);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(file, `#!/bin/sh\necho run >> "${file}.count"\nif [ -e "${file}.fail" ]; then echo broken >&2; exit 3; fi\nexec "${binary}" "$@"\n`);
  fs.chmodSync(file, 0o755);
  return {
    path: file,
    count: () => (fs.existsSync(`${file}.count`) ? fs.readFileSync(`${file}.count`, "utf8").split("\n").filter(Boolean).length : 0),
  };
}

export interface Location {
  line: number;
  offset: number;
}

interface Message {
  seq: number;
  type: "response" | "event";
  command?: string;
  event?: string;
  request_seq?: number;
  success?: boolean;
  message?: string;
  body?: any;
}

export class TsServer {
  private readonly child: ChildProcessWithoutNullStreams;
  private readonly waiting = new Map<number, { resolve(message: Message): void; reject(error: Error): void }>();
  private buffer = Buffer.alloc(0);
  private seq = 0;
  private stderr = "";
  /** Every response body received, with its command: the tests assert on all of them at the end. */
  readonly responses: { command: string; body: unknown }[] = [];
  readonly log: string;

  constructor(
    alias: string,
    readonly dir: string,
    env: Record<string, string | undefined> = {},
  ) {
    this.log = path.join(dir, ".tsserver.log");
    const tsserver = path.join(path.dirname(require.resolve(`${alias}/package.json`)), "lib", "tsserver.js");
    this.child = spawn(
      process.execPath,
      [
        tsserver,
        "--globalPlugins",
        PLUGIN,
        "--pluginProbeLocations",
        root,
        "--disableAutomaticTypingAcquisition",
        "--logVerbosity",
        "normal",
        "--logFile",
        this.log,
      ],
      // No binary unless the test names one: neither the developer's nor the test run's.
      { cwd: dir, env: { ...process.env, REACTOGENIC_BINARY: undefined, ...env } as NodeJS.ProcessEnv },
    );
    this.child.stderr.on("data", (chunk) => {
      this.stderr += chunk;
    });
    this.child.stdout.on("data", (chunk: Buffer) => this.read(chunk));
    this.child.on("exit", (code) => {
      for (const { reject } of this.waiting.values()) {
        reject(new Error(`tsserver exited with ${code}: ${this.stderr}\n${this.logTail()}`));
      }
      this.waiting.clear();
    });
  }

  private read(chunk: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    for (;;) {
      const header = this.buffer.indexOf("\r\n\r\n");
      if (header < 0) {
        return;
      }
      const length = Number(/Content-Length: (\d+)/.exec(this.buffer.subarray(0, header).toString())?.[1]);
      const start = header + 4;
      if (this.buffer.length < start + length) {
        return;
      }
      const message: Message = JSON.parse(this.buffer.subarray(start, start + length).toString("utf8"));
      this.buffer = this.buffer.subarray(start + length);
      if (message.type === "response" && message.request_seq !== undefined) {
        this.waiting.get(message.request_seq)?.resolve(message);
        this.waiting.delete(message.request_seq);
      }
    }
  }

  /** A request and its response's body; a failed request throws with tsserver's message. */
  async request<T = any>(command: string, args: unknown): Promise<T> {
    const seq = ++this.seq;
    const response = new Promise<Message>((resolve, reject) => this.waiting.set(seq, { resolve, reject }));
    this.child.stdin.write(`${JSON.stringify({ seq, type: "request", command, arguments: args })}\n`);
    const message = await response;
    if (!message.success) {
      throw new Error(`${command}: ${message.message}`);
    }
    this.responses.push({ command, body: message.body });
    return message.body as T;
  }

  /** A request without a response: `open`, `change`, `close`. */
  notify(command: string, args: unknown): void {
    this.child.stdin.write(`${JSON.stringify({ seq: ++this.seq, type: "request", command, arguments: args })}\n`);
  }

  file(name: string): string {
    return path.join(this.dir, name).split(path.sep).join("/");
  }

  text(name: string): string {
    return fs.readFileSync(path.join(this.dir, name), "utf8");
  }

  open(name: string, fileContent?: string): void {
    this.notify("open", { file: this.file(name), fileContent, projectRootPath: this.dir });
  }

  /** Line and offset, 1-based, of the `nth` occurrence of `needle` in a file on disk, `inside` characters into it. */
  at(name: string, needle: string, nth = 1, inside = 0): { file: string } & Location {
    const text = this.text(name);
    let index = -1;
    for (let i = 0; i < nth; i++) {
      index = text.indexOf(needle, index + 1);
      if (index < 0) {
        throw new Error(`${name}: no occurrence ${nth} of ${JSON.stringify(needle)}`);
      }
    }
    return { file: this.file(name), ...locate(text, index + inside) };
  }

  /** The semantic, syntactic and suggestion diagnostics of an open file. */
  async diagnostics(name: string): Promise<{ code: number; text: string; start: Location; end: Location; relatedInformation?: any[] }[]> {
    const args = { file: this.file(name) };
    return [...(await this.request("syntacticDiagnosticsSync", args)), ...(await this.request("semanticDiagnosticsSync", args))];
  }

  async codes(name: string): Promise<number[]> {
    return (await this.diagnostics(name)).map((d) => d.code);
  }

  /** Polls until `done` holds for the file's diagnostic codes: tsserver learns of files on disk through its watchers. */
  async until(name: string, done: (codes: number[]) => boolean, what: string, timeout = 20_000): Promise<number[]> {
    const deadline = Date.now() + timeout;
    for (;;) {
      const codes = await this.codes(name);
      if (done(codes)) {
        return codes;
      }
      if (Date.now() > deadline) {
        throw new Error(`${name}: ${what} — still ${JSON.stringify(codes)} after ${timeout} ms\n${this.logTail()}`);
      }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }

  /** The plugin's lines in tsserver's log. */
  pluginLog(): string[] {
    return fs.existsSync(this.log)
      ? fs
          .readFileSync(this.log, "utf8")
          .split("\n")
          .filter((line) => line.includes("reactogenic"))
      : [];
  }

  private logTail(): string {
    return this.pluginLog().slice(-20).join("\n");
  }

  async close(): Promise<void> {
    const exited = new Promise((resolve) => this.child.once("exit", resolve));
    this.child.stdin.end();
    this.child.kill();
    await exited;
  }
}

/** 1-based line and offset of an index into `text`. */
export function locate(text: string, index: number): Location {
  const before = text.slice(0, index);
  const line = before.split("\n").length;
  return { line, offset: index - (before.lastIndexOf("\n") + 1) + 1 };
}

/** The index into `text` of a 1-based line and offset; -1 when the text has no such place. */
export function indexOf(text: string, location: Location): number {
  const lines = text.split("\n");
  if (location.line < 1 || location.line > lines.length || location.offset < 1 || location.offset > lines[location.line - 1].length + 1) {
    return -1;
  }
  return lines.slice(0, location.line - 1).reduce((sum, line) => sum + line.length + 1, 0) + location.offset - 1;
}

export interface FileSpan {
  file: string;
  start: Location;
  end: Location;
  /** The key the span was found under, for a message. */
  where: string;
}

/**
 * Every span of a response that lies in an `.rtsx` file, whatever shape holds
 * it: `{ file, start, end }`, with `contextStart` / `contextEnd` beside them,
 * `{ file, span }`, `{ file, selectionSpan }`, and spans in a list under a file
 * (`fromSpans`, `highlightSpans`, `textChanges`, `locs`).
 */
export function rtsxSpans(body: unknown): FileSpan[] {
  const found: FileSpan[] = [];
  const isLocation = (value: any): value is Location => value && typeof value.line === "number" && typeof value.offset === "number";
  const visit = (value: any, file: string | undefined, where: string): void => {
    if (Array.isArray(value)) {
      value.forEach((item, i) => visit(item, file, `${where}[${i}]`));
      return;
    }
    if (!value || typeof value !== "object") {
      return;
    }
    const own: string | undefined = typeof value.file === "string" ? value.file : typeof value.fileName === "string" ? value.fileName : file;
    if (own?.endsWith(".rtsx")) {
      if (isLocation(value.start) && isLocation(value.end)) {
        found.push({ file: own, start: value.start, end: value.end, where });
      }
      if (isLocation(value.contextStart) && isLocation(value.contextEnd)) {
        found.push({ file: own, start: value.contextStart, end: value.contextEnd, where: `${where}.context` });
      }
    }
    for (const [key, child] of Object.entries(value)) {
      // The spans of an incoming call are in the caller's file; everything else inherits the file above it.
      visit(child, key === "fromSpans" && typeof value.from?.file === "string" ? value.from.file : own, `${where}.${key}`);
    }
  };
  visit(body, undefined, "body");
  return found;
}
