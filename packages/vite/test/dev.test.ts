// The dev transform next to @vitejs/plugin-react, as a real app runs it:
// plugin-react turns on Oxc's `refresh`, and `.rtsx` must not inherit it —
// no Fast Refresh for `.rtsx` in phase 1 (vite.md), and the registration
// calls it emits fail wherever React Refresh's runtime is not set up (SSR).
import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { createElement, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createServer } from "vite";
import { expect, test } from "vitest";
import reactogenic from "../src/index.ts";

test("with plugin-react, the dev transform of .rtsx runs outside the browser", async () => {
  const server = await createServer({
    root: resolve(import.meta.dirname, "render"),
    configFile: false,
    logLevel: "silent",
    server: { middlewareMode: true },
    appType: "custom",
    plugins: [reactogenic(), react()],
    resolve: { alias: { "@reactogenic/core": resolve(import.meta.dirname, "../../core/src/index.ts") } },
  });
  try {
    const app = (await server.ssrLoadModule("/src/app.rtsx")) as { Columns: ComponentType };
    expect(renderToStaticMarkup(createElement(app.Columns))).toBe('<tr><th class="wide">Mail</th><th>Name</th><th>Age</th></tr>');
  } finally {
    await server.close();
  }
});
