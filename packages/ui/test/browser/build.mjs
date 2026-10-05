// A stand-in for `reactogenic build` (specs/phase02/builder.md), just enough
// to put the components in a browser before the builder exists: the pages of
// test/site are transpiled by phase 1, rendered by React's static renderer
// with a stand-in for the build-time protocol, and written with their CSS
// and the behaviours they mounted — one esbuild build per page, the page's
// flags as `define`. What it does not do is what the builder is for: CSS
// pruning, the page checks, packaging by content hash.
import { spawn } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";
import * as esbuild from "esbuild";
import { binary } from "../binary.mjs";

const here = import.meta.dirname;
const packageRoot = resolve(here, "../..");

// `reactogenic serve`: newline-delimited JSON on stdin and stdout (RGP1-053).
function transpiler() {
  const child = spawn(binary(), ["serve"], { stdio: ["pipe", "pipe", "inherit"] });
  const pending = new Map();
  let next = 1;
  createInterface({ input: child.stdout }).on("line", (line) => {
    const response = JSON.parse(line);
    const waiting = pending.get(response.id);
    pending.delete(response.id);
    if (response.error !== undefined) waiting?.reject(new Error(response.error));
    else waiting?.resolve(response.result);
  });
  return {
    transform(file, code) {
      const id = next++;
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
        child.stdin.write(JSON.stringify({ id, method: "transform", params: { file, code } }) + "\n");
      });
    },
    close() {
      child.stdin.end(JSON.stringify({ id: 0, method: "close" }) + "\n");
    },
  };
}

// .rtsx → TSX by the phase 1 transpiler; an error diagnostic fails the build.
function rtsx(server) {
  return {
    name: "rtsx",
    setup(build) {
      build.onLoad({ filter: /\.rtsx$/ }, async ({ path }) => {
        const result = await server.transform(path, readFileSync(path, "utf8"));
        const errors = result.diagnostics.filter((d) => d.severity === "error");
        if (errors.length > 0) {
          return { errors: errors.map((d) => ({ text: `${d.code}: ${d.message}`, location: { file: path, line: d.line, column: d.column - 1 } })) };
        }
        return { contents: result.code, loader: "tsx" };
      });
    },
  };
}

// pages/index.rtsx → "/", pages/a/b/index.rtsx → "/a/b/": the `index` variant
// of each route (builder.md, *Routes*) — the test site has no other.
function routes(pagesDir) {
  const walk = (dir) =>
    readdirSync(dir).flatMap((name) => {
      const path = join(dir, name);
      return statSync(path).isDirectory() ? walk(path) : name === "index.rtsx" ? [path] : [];
    });
  return walk(pagesDir)
    .map((file) => {
      const dir = relative(pagesDir, dirname(file)).split("\\").join("/");
      return { file, pathname: dir === "" ? "/" : `/${dir}/` };
    })
    .sort((a, b) => a.pathname.localeCompare(b.pathname));
}

/**
 * The script of a page from its mounts, as builder.md's *Behaviours* has it:
 * a generated entry, one build, every `RG_…` flag of a mounted module defined
 * — `false` unless a mount set it; a mount's data is the second argument of
 * its call. `flags` overrides the page's own (the sizes script turns each
 * one on and off).
 */
export async function behaviours(mounts, flags) {
  const modules = [...new Set(mounts.map((mount) => mount.module))];
  if (modules.length === 0) {
    return "";
  }
  const define = {};
  for (const module of modules) {
    const source = readFileSync(resolve(packageRoot, "src/behaviors", module.split("/").at(-1) + ".ts"), "utf8");
    for (const [flag] of source.matchAll(/\bRG_[A-Z0-9_]+\b/g)) {
      define[flag] = "false";
    }
  }
  for (const mount of mounts) {
    for (const [flag, on] of Object.entries(mount.flags ?? {})) {
      if (on) define[flag] = "true";
    }
  }
  Object.assign(define, flags);
  const calls = [];
  for (const mount of mounts) {
    const name = `m${modules.indexOf(mount.module)}`;
    const data = mount.data === undefined ? "" : `, ${JSON.stringify(mount.data)}`;
    const call = mount.id === undefined ? `${name}();` : `${name}(document.getElementById(${JSON.stringify(mount.id)})${data});`;
    // A page-level behaviour runs once, however often it is mounted.
    if (!calls.includes(call)) calls.push(call);
  }
  const entry = modules.map((module, i) => `import m${i} from ${JSON.stringify(module)};`).join("\n") + "\n" + calls.join("\n");
  const result = await esbuild.build({
    stdin: { contents: entry, resolveDir: packageRoot, loader: "js" },
    bundle: true,
    minify: true,
    format: "esm",
    write: false,
    define,
    logLevel: "warning",
  });
  return result.outputFiles[0].text.trim();
}

/**
 * Builds test/site into `out`: `<out><pathname>index.html` per page, CSS in a
 * `<style>` at the end of `<head>`, the page's behaviours in a
 * `<script type="module">` at the end of `<body>`. Returns the record of each
 * page: `{ pathname, html, css, js, mounts }`.
 */
export async function build(out, { script = true } = {}) {
  const site = resolve(here, "../site");
  const pages = routes(join(site, "pages"));
  rmSync(out, { recursive: true, force: true });
  // Inside the package, so that the bundle's `react` resolves from it.
  const work = join(packageRoot, ".examples", "render");
  mkdirSync(work, { recursive: true });

  const server = transpiler();
  const plugins = [rtsx(server)];
  const records = [];
  try {
    // The render bundle: every page, React left to Node.
    const bundle = join(work, "site.mjs");
    await esbuild.build({
      stdin: {
        contents:
          pages.map((page, i) => `import P${i} from ${JSON.stringify(page.file)};`).join("\n") +
          `\nexport const pages = { ${pages.map((page, i) => `${JSON.stringify(page.pathname)}: P${i}`).join(", ")} };`,
        resolveDir: site,
        loader: "js",
      },
      bundle: true,
      format: "esm",
      platform: "node",
      jsx: "automatic",
      outfile: bundle,
      external: ["react", "react/*", "react-dom", "react-dom/*"],
      loader: { ".css": "empty" },
      define: { "process.env.NODE_ENV": '"production"' },
      plugins,
      logLevel: "warning",
    });
    const rendered = await import(pathToFileURL(bundle).href + `?${Date.now()}`);
    const { createElement } = await import("react");
    const { renderToStaticMarkup } = await import("react-dom/server");

    for (const page of pages) {
      // The build-time protocol (plan.md, M1).
      const counters = new Map();
      const mounts = [];
      globalThis.__reactogenic_build = {
        pathname: page.pathname,
        id(prefix) {
          const n = (counters.get(prefix) ?? 0) + 1;
          counters.set(prefix, n);
          return prefix + n;
        },
        mount(module, id, flags, data) {
          mounts.push({ module, id, flags, data });
        },
      };
      let html;
      try {
        html = renderToStaticMarkup(createElement(rendered.pages[page.pathname]));
      } finally {
        delete globalThis.__reactogenic_build;
      }

      // The page's CSS, in import order, nesting lowered: the form the builder ships.
      const styles = await esbuild.build({
        entryPoints: [page.file],
        bundle: true,
        minify: true,
        supported: { nesting: false },
        write: false,
        outdir: work,
        jsx: "automatic",
        external: ["react", "react/*", "react-dom", "react-dom/*"],
        plugins,
        logLevel: "warning",
      });
      const css = styles.outputFiles.find((file) => file.path.endsWith(".css"))?.text.trim() ?? "";
      const js = script ? await behaviours(mounts) : "";

      // Packaging, `--inline always`: one request per page.
      let document = html;
      if (css !== "") document = document.replace("</head>", `<style>${css}</style></head>`);
      if (js !== "") document = document.replace("</body>", `<script type="module">${js}</script></body>`);
      const file = join(out, page.pathname, "index.html");
      mkdirSync(dirname(file), { recursive: true });
      writeFileSync(file, `<!doctype html>${document}`);
      records.push({ pathname: page.pathname, html, css, js, mounts });
    }
  } finally {
    server.close();
  }
  return records;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const out = resolve(process.argv[2] ?? join(packageRoot, ".examples", "dist"));
  for (const page of await build(out)) {
    console.log(page.pathname.padEnd(16), `html ${page.html.length} B`, `css ${page.css.length} B`, `js ${page.js.length} B`);
  }
  console.log(out);
  await esbuild.stop();
}
