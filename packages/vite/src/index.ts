// @reactogenic/vite — runs .rtsx in Vite (specs/phase01/vite.md).
import { transformWithOxc, type Plugin, type ResolvedConfig } from "vite";
import { Server } from "./server.ts";

export interface ReactogenicOptions {
  /** The `reactogenic` binary. Default: $REACTOGENIC_BINARY, else `reactogenic` on PATH. */
  binary?: string;
}

// Vite's default resolve.extensions; `.rtsx` joins them.
const defaultExtensions = [".mjs", ".js", ".mts", ".ts", ".jsx", ".tsx", ".json"];

/**
 * `.rtsx` → TSX (the Go transpiler) → JS (Vite's Oxc), with one source map
 * back to the `.rtsx`. The plugin compiles `.rtsx` itself: Vite picks files
 * for its own transform by extension (vite.md, *Findings*). No Fast Refresh
 * for `.rtsx` in phase 1: an edit reloads the page.
 */
export default function reactogenic(options: ReactogenicOptions = {}): Plugin {
  let server: Server | undefined;
  let config: ResolvedConfig;
  const close = () => {
    server?.close();
    server = undefined;
  };
  return {
    name: "reactogenic",
    enforce: "pre",
    config(user) {
      const extensions = user.resolve?.extensions ?? defaultExtensions;
      return extensions.includes(".rtsx") ? undefined : { resolve: { extensions: [...extensions, ".rtsx"] } };
    },
    configResolved(resolved) {
      config = resolved;
    },
    configureServer(dev) {
      dev.httpServer?.once("close", close);
    },
    async transform(code, id) {
      const file = id.split("?")[0]!;
      if (!file.endsWith(".rtsx")) {
        return;
      }
      server ??= new Server(options.binary ?? process.env.REACTOGENIC_BINARY ?? "reactogenic");
      const tsx = await server.transform(file, code);
      for (const d of tsx.diagnostics) {
        const report = { message: `${d.code}: ${d.message}`, id: file, loc: { file, line: d.line, column: d.column - 1 } };
        if (d.severity === "error") {
          this.error(report);
        }
        this.warn(report);
      }
      const oxc = typeof config.oxc === "object" ? config.oxc : {};
      const js = await transformWithOxc(
        tsx.code,
        file,
        { lang: "tsx", jsx: oxc.jsx ?? { runtime: "automatic", development: !config.isProduction }, sourcemap: true },
        JSON.parse(tsx.map) as object,
      );
      return { code: js.code, map: js.map, moduleType: "js" };
    },
    closeBundle: close,
  };
}
