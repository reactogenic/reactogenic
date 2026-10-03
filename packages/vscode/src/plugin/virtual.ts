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
  /** The workspace is trusted: its own `@reactogenic/cli` may run. */
  trusted?: boolean;
}

/** An `.rtsx` file as TypeScript sees it. */
export interface Virtual {
  /** The source the text was made from: positions map into it. */
  source: string;
  text: string;
  map: SpanMap;
  /** The binary that made it. */
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

/** How long one batch may take: a binary that hangs must not hang tsserver. */
const TIMEOUT = 20_000;
/** After a failure the binary is left alone this long: every file of a program would fail in turn. */
export const RETRY_AFTER = 5_000;
/** What one request may hold beside the files asked for. */
const BATCH_FILES = 128;
const BATCH_BYTES = 4 << 20;

/** The folder that holds `server/reactogenic` in a platform's .vsix: the plugin is `node_modules/<name>/index.js` in it. */
const EXTENSION_PATH = path.resolve(__dirname, "..", "..");

export class Transformer {
  /** By file name, as TypeScript writes it. One entry per file: the last good one. */
  private readonly cache = new Map<string, Virtual>();
  private readonly binaries = new Map<string, string | undefined>();
  private readonly failed = new Map<string, number>();
  private config: Config = {};
  /** Bumped when a text may differ though its source does not: another binary. */
  generation = 0;
  /** Processes started, for the log. */
  spawns = 0;

  constructor(private readonly log: (message: string) => void = () => {}) {}

  /** True when the configuration changed. */
  configure(config: Config | undefined): boolean {
    const next: Config = {
      serverPath: typeof config?.serverPath === "string" && config.serverPath ? config.serverPath : undefined,
      workspaceFolder: typeof config?.workspaceFolder === "string" ? config.workspaceFolder : undefined,
      trusted: config?.trusted === true,
    };
    if (next.serverPath === this.config.serverPath && next.workspaceFolder === this.config.workspaceFolder && next.trusted === (this.config.trusted ?? false)) {
      return false;
    }
    this.config = next;
    this.binaries.clear();
    this.failed.clear();
    this.generation++;
    return true;
  }

  /**
   * The binary for a project (ide.md, *Which binary runs*): the setting,
   * `$REACTOGENIC_BINARY`, the nearest `@reactogenic/cli` at or above the
   * project's directory — in a trusted workspace only — and the one in the
   * .vsix. Decided once per directory and configuration.
   */
  binary(projectDir: string): string | undefined {
    if (!this.binaries.has(projectDir)) {
      const resolution = resolveServer({
        // Rows 1, 2 and 4 are the user's own; only the walk into the workspace needs its trust.
        trusted: true,
        settingPath: this.config.serverPath,
        workspaceFolder: this.config.workspaceFolder ?? projectDir,
        documentDir: this.config.trusted ? projectDir : undefined,
        extensionPath: EXTENSION_PATH,
      });
      const binary = resolution.kind === "binary" ? resolution.binary.path : undefined;
      this.log(`${projectDir}: ${binary ?? ("message" in resolution ? resolution.message : "no binary")}`);
      this.binaries.set(projectDir, binary);
    }
    return this.binaries.get(projectDir);
  }

  /** True when the file has a text, the current one or a last good one. */
  has(file: string): boolean {
    return this.cache.has(file);
  }

  /** The entry made by `binary` from exactly this source, if any. */
  private fresh(binary: string, file: string, code: string): Virtual | undefined {
    const entry = this.cache.get(file);
    return entry && entry.binary === binary && entry.source === code ? entry : undefined;
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
    const binary = this.binary(projectDir);
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
    const binary = this.binary(projectDir);
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
  private run(binary: string, cwd: string, batch: readonly Source[]): void {
    const last = this.failed.get(binary);
    if (last !== undefined && Date.now() - last < RETRY_AFTER) {
      return;
    }
    try {
      this.spawns++;
      const started = Date.now();
      const result = spawnSync(binary, ["serve"], {
        cwd: fs.existsSync(cwd) ? cwd : undefined,
        input: `${JSON.stringify({ id: 1, method: "virtual", params: { files: batch } })}\n`,
        encoding: "utf8",
        maxBuffer: 1 << 30,
        timeout: TIMEOUT,
        windowsHide: true,
      });
      if (result.error) {
        throw result.error;
      }
      const line = result.stdout.slice(0, result.stdout.indexOf("\n") + 1 || undefined);
      const response = line ? JSON.parse(line) : undefined;
      const files: { file: string; text: string; spans: Tuple[] }[] | undefined = response?.result?.files;
      if (!Array.isArray(files) || files.length !== batch.length) {
        // A binary from before the request answers `unknown method "virtual"`.
        throw new Error(response?.error ?? `exit status ${result.status}${result.stderr ? `: ${result.stderr.trim()}` : ""}`);
      }
      files.forEach((made, i) => {
        this.cache.set(batch[i].file, { source: batch[i].code, text: made.text, map: new SpanMap(made.spans), binary });
      });
      this.failed.delete(binary);
      this.log(`spawn ${this.spawns}: ${batch.length} file(s) in ${Date.now() - started} ms`);
    } catch (error) {
      this.failed.set(binary, Date.now());
      // The next attempt looks for the binary again: an install may have replaced it.
      this.binaries.clear();
      this.log(`${binary} serve failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
}

function list(dir: string): string[] {
  try {
    return fs.readdirSync(dir);
  } catch {
    return [];
  }
}
