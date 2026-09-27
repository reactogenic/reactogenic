// The Node side of `reactogenic serve` (RGP1-053): one long-lived Go process,
// newline-delimited JSON requests on its stdin, one response per line on its
// stdout.
import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";

export interface Diagnostic {
  line: number;
  column: number; // 1-based
  severity: "error" | "warning";
  code: string;
  message: string;
}

export interface TransformResult {
  code: string; // TSX
  map: string; // source map v3, back to the .rtsx
  diagnostics: Diagnostic[];
}

interface Pending {
  resolve: (result: unknown) => void;
  reject: (error: Error) => void;
}

export class Server {
  #process: ChildProcess;
  #pending = new Map<number, Pending>();
  #nextId = 1;
  #failure: Error | undefined;

  constructor(binary: string) {
    this.#process = spawn(binary, ["serve"], { stdio: ["pipe", "pipe", "inherit"] });
    this.#process.on("error", (error) => this.#fail(new Error(`reactogenic: cannot start \`${binary} serve\`: ${error.message}`)));
    this.#process.on("exit", (code) => this.#fail(new Error(`reactogenic: \`${binary} serve\` exited (${code})`)));
    createInterface({ input: this.#process.stdout! }).on("line", (line) => {
      const response = JSON.parse(line) as { id: number; result?: unknown; error?: string };
      const pending = this.#pending.get(response.id);
      this.#pending.delete(response.id);
      if (response.error !== undefined) {
        pending?.reject(new Error(`reactogenic: ${response.error}`));
      } else {
        pending?.resolve(response.result);
      }
    });
  }

  transform(file: string, code: string): Promise<TransformResult> {
    return this.#request("transform", { file, code }) as Promise<TransformResult>;
  }

  close(): void {
    if (this.#failure === undefined) {
      this.#process.stdin!.end(JSON.stringify({ id: 0, method: "close" }) + "\n");
    }
  }

  #request(method: string, params: unknown): Promise<unknown> {
    if (this.#failure !== undefined) {
      return Promise.reject(this.#failure);
    }
    const id = this.#nextId++;
    return new Promise((resolve, reject) => {
      this.#pending.set(id, { resolve, reject });
      this.#process.stdin!.write(JSON.stringify({ id, method, params }) + "\n");
    });
  }

  #fail(error: Error): void {
    this.#failure ??= error;
    for (const pending of this.#pending.values()) {
      pending.reject(error);
    }
    this.#pending.clear();
  }
}
