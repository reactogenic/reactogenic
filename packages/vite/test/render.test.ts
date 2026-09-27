// RGP1-064: the compiled output runs. The app is built for Node with the
// plugin, imported, and rendered by React; the HTML shows what slots,
// fallbacks, args, flow control and segments do at runtime.
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { createElement, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { build } from "vite";
import { beforeAll, expect, test } from "vitest";
import reactogenic from "../src/index.ts";

let app: { WithSlots: ComponentType<{ mode: "loading" | "ready" }>; WithoutSlots: ComponentType };

beforeAll(async () => {
  const outDir = mkdtempSync(join(tmpdir(), "reactogenic-render-"));
  await build({
    root: resolve(import.meta.dirname, "render"),
    logLevel: "silent",
    plugins: [reactogenic()],
    resolve: { alias: { "@reactogenic/core": resolve(import.meta.dirname, "../../core/src/index.ts") } },
    build: { ssr: "src/app.rtsx", outDir, minify: false, rolldownOptions: { external: [/^react/] } },
  });
  app = await import(pathToFileURL(join(outDir, "app.js")).href);
});

test("slots, args, flow control and segments render", () => {
  const html = renderToStaticMarkup(createElement(app.WithSlots, { mode: "ready" }));
  // $Title replaces the attachment's props and fallback.
  expect(html).toContain('<h1 class="custom">Picked</h1>');
  // One $Option, attached per option: && props reach the element, & args the body.
  expect(html).toContain('<li class="a">Alpha</li>');
  expect(html).toContain('<li class="b">Beta!</li>');
  // Switch, and the segment root.
  expect(html).toContain("Ready");
  expect(html).not.toContain("Loading");
  expect(html).toContain('<section id="intro"><p>intro</p></section>');
});

test("without slots, the attachments render their fallbacks", () => {
  const html = renderToStaticMarkup(createElement(app.WithoutSlots));
  expect(html).toContain('<h1 class="default">Untitled</h1>');
  expect(html).toContain('<li class="a">Alpha</li><li class="b">Beta</li>');
});
