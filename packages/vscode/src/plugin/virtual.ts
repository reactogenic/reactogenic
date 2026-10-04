// The virtual text of `.rtsx` files, for a caller that cannot wait
// asynchronously (tsserver's host API is synchronous): `reactogenic serve`
// run with `spawnSync`, one process per batch of files (ide.md, *The plugin*).
import { spawnSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import { resolveServer } from "../resolve";
import { lineStarts, SpanMap, type Tuple } from "./spans";

/** What the extension sends with `configurePlugin`; nothing else configures the plugin. */
export interface Config {
  /** The setting `reactogenic.server.path`. */
  serverPath?: string | null;
  /** A relative `serverPath` resolves against it. */
  workspaceFolder?: string;
  /** The workspace is trusted: its own `@reactogenic/cli` may run, and a relative path is resolved in it. */
  trusted?: boolean;
  /** Counts the changes of the workspace's lockfiles: after one, the workspace's CLI may be another. */
  installs?: number;
  /** `reactogenic lsp` runs in the window and has a project — an `.rtsx` document is open: the workspace symbols of `.rtsx` files are its to list. */
  languageServer?: boolean;
}

/** An `.rtsx` file as TypeScript sees it. */
export interface Virtual {
  /** The source the text was made from: positions map into it. */
  source: string;
  text: string;
  map: SpanMap;
  /** The binary that made it: its path and the file's stamp (`Chosen.id`). */
  binary: string;
  starts?: number[];
}

export function sourceLineStarts(virtual: Virtual): number[] {
  virtual.starts ??= lineStarts(virtual.source);
  return virtual.starts;
}

export interface Source {
  file: string;
  code: string;
}

/**
 * How long one batch may take — tsserver answers nothing meanwhile: this
 * much, and so much more per file. Two hundred files take 200 ms; the first
 * run of a new binary, half a second.
 */
export const TIMEOUT = 5_000;
const TIMEOUT_PER_FILE = 25;
/**
 * After a failure the binary is left alone this long: every file of a program
 * would fail in turn. A project without a binary looks for one again as often.
 */
export const RETRY_AFTER = 5_000;
/**
 * A binary that did not answer in time is left alone much longer, and twice
 * as long after each time: every attempt holds tsserver up for the whole
 * timeout. Until the configuration changes, or the file at its path does.
 */
const HUNG_AFTER = 60_000;
const HUNG_MAX = 16 * 60_000;
/** What one request may hold beside the files asked for. */
const BATCH_FILES = 128;
const BATCH_BYTES = 4 << 20;

/** The folder that holds `server/reactogenic` in a platform's .vsix: the plugin is `node_modules/<name>/index.js` in it. */
const EXTENSION_PATH = path.resolve(__dirname, "..", "..");

/** A binary, and what tells it from another file at the same path. */
interface Chosen {
  path: string;
  id: string;
}

export class Transformer {
  /** By file name, as TypeScript writes it. One entry per file: the last good one. */
  private readonly cache = new Map<string, Virtual>();
  /** By project directory. An entry without a binary is looked up again after RETRY_AFTER. */
  private readonly binaries = new Map<string, { chosen: Chosen | undefined; at: number }>();
  /** By `Chosen.id`: when the binary failed, how long it is left alone, and whether it had not answered. */
  private readonly failed = new Map<string, { at: number; wait: number; hung: boolean }>();
  private readonly ids = new Map<string, number>();
  private config: Config = {};
  /** Why the project asked about last has no binary, for the log. */
  private none = "no binary";
  /** `reactogenic lsp` lists the workspace symbols of `.rtsx` files: the extension says so. */
  languageServer = false;
  /** Bumped when a text may differ though its source does not: another binary. */
  generation = 0;
  /** Processes started, for the log. */
  spawns = 0;

  constructor(private readonly log: (message: string) => void = () => {}) {}

  /**
   * True when a text may differ now: another setting, folder or trust — or,
   * after an install, another binary for a project that has asked for one.
   */
  configure(config: Config | undefined): boolean {
    this.languageServer = config?.languageServer === true;
    const next: Config = {
      serverPath: typeof config?.serverPath === "string" && config.serverPath ? config.serverPath : undefined,
      workspaceFolder: typeof config?.workspaceFolder === "string" ? config.workspaceFolder : undefined,
      trusted: config?.trusted === true,
      installs: typeof config?.installs === "number" ? config.installs : undefined,
    };
    const same = next.serverPath === this.config.serverPath && next.workspaceFolder === this.config.workspaceFolder && next.trusted === (this.config.trusted ?? false);
    if (same && next.installs === this.config.installs) {
      return false;
    }
    this.config = next;
    const before = [...this.binaries];
    this.binaries.clear();
    this.failed.clear();
    // An install that left every project's binary as it was changes nothing.
    if (same && before.every(([projectDir, was]) => this.chosen(projectDir)?.id === was.chosen?.id)) {
      return false;
    }
    this.generation++;
    return true;
  }

  /**
   * The binary for a project (ide.md, *Which binary runs*): the setting,
   * `$REACTOGENIC_BINARY`, the nearest `@reactogenic/cli` at or above the
   * project's directory, and the one in the .vsix. Decided once per directory
   * and configuration — and while there is none, again after RETRY_AFTER: an
   * install may have brought one.
   */
  private chosen(projectDir: string): Chosen | undefined {
    const known = this.binaries.get(projectDir);
    if (known?.chosen) {
      // One that failed is looked at again: the file at its path may be another by now.
      if (!this.failed.has(known.chosen.id) || known.chosen.id === identity(known.chosen.path)) {
        return known.chosen;
      }
    } else if (known && Date.now() - known.at < RETRY_AFTER) {
      return undefined;
    }
    const chosen = this.resolve(projectDir);
    if (!known || known.chosen?.id !== chosen?.id) {
      this.log(`${projectDir}: ${chosen?.path ?? this.none}`);
    }
    this.binaries.set(projectDir, { chosen, at: Date.now() });
    return chosen;
  }

  /**
   * Rows 1, 2 and 4 are the user's own. The workspace's trust is needed for
   * the walk of row 3 — and for a relative path of rows 1 and 2, which names
   * a file of the workspace.
   */
  private resolve(projectDir: string): Chosen | undefined {
    const trusted = this.config.trusted === true;
    const explicit = this.config.serverPath ?? process.env.REACTOGENIC_BINARY;
    if (!trusted && explicit && !isAbsolute(explicit)) {
      // As an explicit path that does not exist: no binary, not a fall-through.
      this.none = `${this.config.serverPath ? "reactogenic.server.path" : "$REACTOGENIC_BINARY"}: ${explicit} is relative, and the workspace is not trusted: not run`;
      return undefined;
    }
    const resolution = resolveServer({
      trusted: true,
      settingPath: this.config.serverPath,
      workspaceFolder: this.config.workspaceFolder ?? projectDir,
      documentDir: trusted ? projectDir : undefined,
      extensionPath: EXTENSION_PATH,
    });
    if (resolution.kind !== "binary") {
      this.none = "message" in resolution ? resolution.message : "no binary";
      return undefined;
    }
    return { path: resolution.binary.path, id: identity(resolution.binary.path) };
  }

  /** The path of the project's binary; undefined when it has none. */
  binary(projectDir: string): string | undefined {
    return this.chosen(projectDir)?.path;
  }

  /**
   * A number for the project's binary, part of a text's version: two
   * projects that hold one file and run different binaries (a monorepo with
   * two versions of the CLI) do not share its parsed text.
   */
  binaryId(projectDir: string): number {
    const binary = this.chosen(projectDir);
    if (binary === undefined) {
      return 0;
    }
    let id = this.ids.get(binary.id);
    if (id === undefined) {
      id = this.ids.size + 1;
      this.ids.set(binary.id, id);
    }
    return id;
  }

  /** True when the file has a text, the current one or a last good one; without a file, when any has. */
  has(file?: string): boolean {
    return file === undefined ? this.cache.size > 0 : this.cache.has(file);
  }

  /** The entry made by `binary` from exactly this source, if any. */
  private fresh(binary: Chosen, file: string, code: string): Virtual | undefined {
    const entry = this.cache.get(file);
    return entry && entry.binary === binary.id && entry.source === code ? entry : undefined;
  }

  /**
   * The virtual text of `file` for the source `code`. With a miss, one
   * process transforms it together with `others` and with the `.rtsx` files
   * beside them that are not cached yet: a program asks for them next.
   *
   * Undefined when there is no binary, or it failed and the file has no
   * earlier text; an earlier text is kept (its `source` then differs from
   * `code`).
   */
  get(projectDir: string, file: string, code: string, others: readonly string[], read: (file: string) => string | undefined): Virtual | undefined {
    const binary = this.chosen(projectDir);
    if (!binary) {
      return this.cache.get(file);
    }
    const hit = this.fresh(binary, file, code);
    if (hit) {
      return hit;
    }
    const batch: Source[] = [{ file, code }];
    const seen = new Set([file]);
    let bytes = code.length;
    const add = (other: string) => {
      if (seen.has(other) || batch.length >= BATCH_FILES || bytes >= BATCH_BYTES) {
        return;
      }
      seen.add(other);
      const text = read(other);
      if (text !== undefined && !this.fresh(binary, other, text)) {
        batch.push({ file: other, code: text });
        bytes += text.length;
      }
    };
    others.forEach(add);
    for (const dir of new Set([file, ...others].map((f) => path.posix.dirname(f)))) {
      for (const name of list(dir)) {
        if (name.endsWith(".rtsx")) {
          add(`${dir}/${name}`);
        }
      }
    }
    this.run(binary, projectDir, batch);
    return this.fresh(binary, file, code) ?? this.cache.get(file);
  }

  /**
   * The files an importer resolves to, as they are on disk: those without a
   * current text are transformed together. True when there is a binary.
   */
  ensure(projectDir: string, files: readonly string[], read: (file: string) => string | undefined): boolean {
    const binary = this.chosen(projectDir);
    if (!binary) {
      return false;
    }
    for (const file of files) {
      const code = read(file);
      if (code !== undefined && !this.fresh(binary, file, code)) {
        this.get(projectDir, file, code, files, read); // the rest of them with it
        break;
      }
    }
    return true;
  }

  /** One process, one request. A failure leaves the cache as it was. */
  private run(binary: Chosen, cwd: string, batch: readonly Source[]): void {
    const last = this.failed.get(binary.id);
    if (last !== undefined && Date.now() - last.at < last.wait) {
      return;
    }
    const timeout = TIMEOUT + TIMEOUT_PER_FILE * batch.length;
    try {
      this.spawns++;
      const started = Date.now();
      const result = spawnSync(binary.path, ["serve"], {
        cwd: fs.existsSync(cwd) ? cwd : undefined,
        input: `${JSON.stringify({ id: 1, method: "virtual", params: { files: batch } })}\n`,
        encoding: "utf8",
        maxBuffer: 1 << 30,
        timeout,
        killSignal: "SIGKILL",
        windowsHide: true,
      });
      if (result.error) {
        throw result.error;
      }
      const line = result.stdout.slice(0, result.stdout.indexOf("\n") + 1 || undefined);
      const response = line ? JSON.parse(line) : undefined;
      const files: unknown = response?.result?.files;
      if (!Array.isArray(files) || files.length !== batch.length) {
        // A binary from before the request answers `unknown method "virtual"`.
        throw new Error(typeof response?.error === "string" ? response.error : `exit status ${result.status}${result.stderr ? `: ${result.stderr.trim()}` : ""}`);
      }
      // All of the answer or none of it: an entry without a text, or with
      // spans that are none, would fail inside the program being built.
      const made = files.map((entry: unknown, i) => {
        if (!isVirtualFile(entry)) {
          throw new Error(`its answer for ${batch[i].file} is not a text and its spans: another version of the CLI?`);
        }
        return entry;
      });
      made.forEach((entry, i) => {
        this.cache.set(batch[i].file, { source: batch[i].code, text: entry.text, map: new SpanMap(entry.spans), binary: binary.id });
      });
      this.failed.delete(binary.id);
      this.log(`spawn ${this.spawns}: ${batch.length} file(s) in ${Date.now() - started} ms`);
    } catch (error) {
      const hung = (error as NodeJS.ErrnoException | undefined)?.code === "ETIMEDOUT";
      const wait = !hung ? RETRY_AFTER : last?.hung ? Math.min(last.wait * 2, HUNG_MAX) : HUNG_AFTER;
      // The next attempt looks for the binary again (`chosen`): an install may have replaced it.
      this.failed.set(binary.id, { at: Date.now(), wait, hung });
      const why = hung ? `no answer within ${timeout} ms — not run again for ${wait / 1000} s, unless the file or the configuration changes` : error instanceof Error ? error.message : String(error);
      this.log(`${binary.path} serve failed: ${why}`);
    }
  }
}

/** The path, and what changes when the file there is replaced: an upgrade in place (npm's flat layout). */
function identity(file: string): string {
  try {
    const stat = fs.statSync(file);
    return `${file}\0${stat.mtimeMs}:${stat.size}`;
  } catch {
    return `${file}\0`;
  }
}

/** `~` is the user's home, as in src/resolve.ts: not a path into the workspace. */
function isAbsolute(file: string): boolean {
  return path.isAbsolute(file) || file === "~" || file.startsWith("~/");
}

/** An entry of the answer as `serve` writes it: a text, and tuples of six offsets and flags. */
function isVirtualFile(entry: unknown): entry is { text: string; spans: Tuple[] } {
  if (typeof entry !== "object" || entry === null) {
    return false;
  }
  const { text, spans } = entry as { text?: unknown; spans?: unknown };
  return typeof text === "string" && Array.isArray(spans) && spans.every((span) => Array.isArray(span) && span.length === 6 && span.every((n) => Number.isInteger(n) && n >= 0));
}

function list(dir: string): string[] {
  try {
    return fs.readdirSync(dir);
  } catch {
    return [];
  }
}
