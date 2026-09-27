import { resolve } from "node:path";
import { build, createServer, type Rolldown } from "vite";
import { expect, test } from "vitest";
import reactogenic from "../src/index.ts";

const app = resolve(import.meta.dirname, "app");

test("vite build: .rtsx resolves without an extension and compiles", async () => {
  const result = (await build({
    root: app,
    logLevel: "silent",
    plugins: [reactogenic()],
    resolve: { alias: { "@reactogenic/core": resolve(import.meta.dirname, "../../core/src/index.ts") } },
    build: { write: false, minify: false },
  })) as Rolldown.RolldownOutput;
  const code = result.output.map((o) => ("code" in o ? o.code : "")).join("\n");
  expect(code).toContain("hello-from-rtsx"); // page.rtsx, imported as "./page"
  expect(code).toContain("intro-segment"); // +intro.rtsx, mounted by #intro
  expect(code).toContain("untitled"); // card.rtsx: the attachment's fallback
  expect(code).not.toContain("slot="); // slots lowered
  expect(code).not.toMatch(/greeting: string/); // types stripped
});

test("dev server: the module is JS, mapped back to the .rtsx", async () => {
  const server = await createServer({
    root: app,
    logLevel: "silent",
    plugins: [reactogenic()],
    server: { middlewareMode: true, ws: false },
    resolve: { alias: { "@reactogenic/core": resolve(import.meta.dirname, "../../core/src/index.ts") } },
  });
  try {
    const result = await server.transformRequest("/src/page.rtsx");
    expect(result?.code).toContain("hello-from-rtsx");
    expect(result?.code).not.toMatch(/: string/);
    expect(result?.code).toMatch(/jsx/);
    const sources = (result?.map as { sources?: string[] } | null)?.sources ?? [];
    expect(sources.some((s) => s.endsWith("page.rtsx"))).toBe(true);
  } finally {
    await server.close();
  }
});

test("a transpiler error fails the build at its .rtsx position", async () => {
  await expect(
    build({ root: resolve(import.meta.dirname, "broken"), logLevel: "silent", plugins: [reactogenic()], build: { write: false } }),
  ).rejects.toThrow(/bad\.rtsx:2:7[\s\S]*params-on-html/);
});
