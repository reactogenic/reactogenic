// The TS server plugin (ide.md, *The plugin*) in a real tsserver, TypeScript
// 5.9 and 6.0, driven over stdio as VS Code's TypeScript extension drives it.
// Every `.rtsx` file is on disk only: tsserver never opens one.
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { counting, type FileSpan, indexOf, PLUGIN, project, root, rtsxSpans, TsServer, VERSIONS, versionOf, write } from "./tsserver.ts";

const binary = process.env.REACTOGENIC_BINARY!;
const types = (name: string) => fs.readFileSync(path.join(root, "test/fixture/types", name), "utf8");

const TSCONFIG = JSON.stringify({
  compilerOptions: {
    strict: true,
    jsx: "preserve",
    module: "esnext",
    moduleResolution: "bundler",
    target: "es2022",
    lib: ["es2022"],
    types: [],
    noEmit: true,
    paths: { "@reactogenic/core": ["./types/core.d.ts"], "@/*": ["./src/*"] },
  },
  include: ["src", "types"],
});

// Everything `.rtsx` has: a segment root (an import the transform adds), a
// shorthand prop, slots with params — so that no position of page.rtsx after
// its first lines is the same in the virtual text.
const PAGE = `import { Button } from "./button";
import { describe } from "./types";
import type { Size } from "./types";

export interface PageProps {
  title: string;
}

export function Page({ title }: PageProps) {
  const size: Size = "lg";
  const label = describe(size);
  return (
    <main>
      <section #intro />
      <h1 title={label}>{title}</h1>
      <Button size>
        <$Icon className="icon" { size }>
          {describe(size)}
        </$Icon>
        <$Label>Save</$Label>
      </Button>
    </main>
  );
}
`;

const FILES: Record<string, string> = {
  "tsconfig.json": TSCONFIG,
  "types/jsx.d.ts": types("jsx.d.ts"),
  "types/core.d.ts": types("core.d.ts"),
  "src/types.ts": `import type { Slot } from "@reactogenic/core";

export type Size = "md" | "lg";

export interface ButtonProps {
  size: Size;
  $Icon?: Slot<{ className?: string }, { size: Size }>;
  $Label?: Slot<{ className?: string; children?: string }>;
}

export function describe(size: Size): string {
  return size;
}
`,
  "src/button.rtsx": `import type { ButtonProps } from "./types";

export function Button({ size, $Label, $Icon }: ButtonProps) {
  return (
    <button data-size={size}>
      <span slot={$Icon} &size />
      <b slot={$Label} className="label">Button</b>
    </button>
  );
}
`,
  "src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
  "src/page.rtsx": PAGE,
  "src/parts/index.rtsx": "export const part = <i>part</i>;\n",
  "src/main.tsx": `import { Page } from "./page";
import type { PageProps } from "./page";
import { Button } from "./button.rtsx";
import { part } from "./parts";
import Intro from "@/intro";

const props: PageProps = { title: "Home" };
export const app = <Page {...props} />;
export const bare = [<Button size="md" />, part, <Intro />];
`,
  "src/wrong.tsx": `import { Page } from "./page";

export const wrong = <Page title={1} />;
`,
  "src/aliased.tsx": `import Intro from "@/intro";
import { Page } from "@/page";

export const both = [Intro, Page];
`,
};

/** The virtual text of a file, straight from the binary. */
function virtual(file: string, code: string): string {
  const result = spawnSync(binary, ["serve"], { input: `${JSON.stringify({ id: 1, method: "virtual", params: { files: [{ file, code }] } })}\n`, encoding: "utf8" });
  return JSON.parse(result.stdout).result.files[0].text;
}

/** Each span is a place of the file on disk; returns its text. */
function sourceText(span: FileSpan): string {
  const text = fs.readFileSync(span.file, "utf8");
  const start = indexOf(text, span.start);
  const end = indexOf(text, span.end);
  expect(start, `${span.where}: ${span.file} has no ${span.start.line}:${span.start.offset}`).toBeGreaterThanOrEqual(0);
  expect(end, `${span.where}: ${span.file} has no ${span.end.line}:${span.end.offset}`).toBeGreaterThanOrEqual(start);
  return text.slice(start, end);
}

describe.each(VERSIONS)("%s", (alias) => {
  let dir: string;
  let server: TsServer;
  let spawned: ReturnType<typeof counting>;

  beforeAll(() => {
    dir = project(FILES);
    spawned = counting(path.join(dir, ".bin"), binary);
    server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path });
    server.open("src/main.tsx");
    server.open("src/wrong.tsx");
    server.open("src/types.ts");
    server.open("src/aliased.tsx");
  });
  afterAll(async () => {
    await server?.close();
    fs.rmSync(dir, { recursive: true, force: true });
  });

  test("the fixture means something: page.rtsx's virtual text is not its source", () => {
    const text = virtual(path.join(dir, "src/page.rtsx"), PAGE);
    expect(text).toContain("<Button size={size} $Icon=");
    for (const token of ["export interface PageProps", "export function Page", "describe(size);", "<h1 title"]) {
      expect(text.indexOf(token), token).not.toBe(PAGE.indexOf(token));
    }
    expect(versionOf(alias)).toMatch(alias === "typescript-5.9" ? /^5\.9\./ : /^6\.0\./);
  });

  test("an .rtsx import resolves: no TS2307, no diagnostic at all", async () => {
    expect(await server.diagnostics("src/main.tsx")).toEqual([]);
    expect(server.pluginLog().join("\n")).not.toMatch(/failed|older than/);
  });

  test("one process per batch of files, not per file", () => {
    // Five `.rtsx` modules in two folders: the first import that resolves
    // into a folder takes the folder's other `.rtsx` files with it.
    expect(spawned.count()).toBe(2);
    expect(server.pluginLog().filter((line) => line.includes("spawn")).map((line) => /(\d+) file\(s\)/.exec(line)?.[1])).toEqual(["3", "1"]);
  });

  test("go to definition lands in page.rtsx at source line and column", async () => {
    const body = await server.request("definitionAndBoundSpan", server.at("src/main.tsx", "Page", 1));
    expect(body.definitions).toHaveLength(1);
    const [definition] = body.definitions;
    expect(definition.file).toBe(server.file("src/page.rtsx"));
    expect({ start: definition.start, end: definition.end }).toEqual({
      start: { line: 9, offset: 17 },
      end: { line: 9, offset: 21 },
    });
    const spans = rtsxSpans(body);
    expect(spans.map((span) => sourceText(span))).toEqual(["Page", PAGE.slice(PAGE.indexOf("export function Page"), PAGE.lastIndexOf("}") + 1)]);

    // A type; the plain `definition` request; a type definition.
    const props = await server.request("definition", server.at("src/main.tsx", "PageProps", 1));
    expect(props.map((d: any) => [d.file, d.start])).toEqual([[server.file("src/page.rtsx"), { line: 5, offset: 18 }]]);
    const typeDefinition = await server.request("typeDefinition", server.at("src/main.tsx", "props", 1));
    expect(typeDefinition.map((d: any) => [d.file, d.start])).toEqual([[server.file("src/page.rtsx"), { line: 5, offset: 18 }]]);
  });

  test("a module specifier's definition is the .rtsx file", async () => {
    // Extensionless, explicit, a directory's index.rtsx: TypeScript answers a relative specifier with the file.
    for (const [specifier, file] of [
      ['"./page"', "src/page.rtsx"],
      ['"./button.rtsx"', "src/button.rtsx"],
      ['"./parts"', "src/parts/index.rtsx"],
    ]) {
      const body = await server.request("definitionAndBoundSpan", server.at("src/main.tsx", specifier, 1, 2));
      expect(
        body.definitions.map((d: any) => [d.file, d.start, d.end]),
        specifier,
      ).toEqual([[server.file(file), { line: 1, offset: 1 }, { line: 1, offset: 1 }]]);
    }
    // A `paths` alias: with the module's declaration, which is the file from
    // its first token to its end — in the source, not in the virtual text.
    for (const [specifier, file, source] of [
      ['"@/intro"', "src/intro.rtsx", FILES["src/intro.rtsx"]],
      ['"@/page"', "src/page.rtsx", PAGE],
    ]) {
      const body = await server.request("definitionAndBoundSpan", server.at("src/aliased.tsx", specifier, 1, 2));
      expect(body.definitions, specifier).toHaveLength(1);
      expect(body.definitions[0].file).toBe(server.file(file));
      expect(sourceText({ ...body.definitions[0], where: specifier })).toBe(source);
    }
  });

  test("references list the .rtsx uses at source positions", async () => {
    const body = await server.request("references", server.at("src/types.ts", "describe", 1));
    const inPage = body.refs.filter((ref: any) => ref.file === server.file("src/page.rtsx"));
    // The import, the call in the component, the call in the slot's body — which the transform moved.
    expect(inPage.map((ref: any) => [ref.start.line, ref.start.offset, ref.lineText])).toEqual([
      [2, 10, 'import { describe } from "./types";'],
      [11, 17, "  const label = describe(size);"],
      [18, 12, "          {describe(size)}"],
    ]);
    for (const span of rtsxSpans(body)) {
      expect(sourceText(span), span.where).toMatch(/^describe$|^import \{ describe \} from "\.\/types";$/);
    }

    // A prop: its shorthand use in page.rtsx, its destructuring and its arg in button.rtsx.
    const size = await server.request("references", server.at("src/types.ts", "size: Size;", 1));
    const uses = size.refs.filter((ref: any) => ref.file.endsWith(".rtsx")).map((ref: any) => `${path.basename(ref.file)} ${ref.start.line}:${ref.start.offset} ${ref.lineText.trim()}`);
    expect(uses).toEqual(["button.rtsx 3:26 export function Button({ size, $Label, $Icon }: ButtonProps) {", "page.rtsx 16:15 <Button size>"]);
    for (const span of rtsxSpans(size).filter((span) => !span.where.endsWith(".context"))) {
      expect(sourceText(span), span.where).toBe("size");
    }

    // A slot: the `$Icon` tag of page.rtsx, the destructuring of button.rtsx.
    const icon = await server.request("references", server.at("src/types.ts", "$Icon", 1));
    const tags = icon.refs.filter((ref: any) => ref.file.endsWith(".rtsx")).map((ref: any) => `${path.basename(ref.file)} ${ref.start.line}:${ref.start.offset} ${ref.lineText.trim()}`);
    expect(tags).toEqual(["button.rtsx 3:40 export function Button({ size, $Label, $Icon }: ButtonProps) {", 'page.rtsx 17:10 <$Icon className="icon" { size }>']);
  });

  test("an error's related information points at the source", async () => {
    const diagnostics = await server.diagnostics("src/wrong.tsx");
    expect(diagnostics.map((d) => d.code)).toEqual([2322]);
    const [related] = diagnostics[0].relatedInformation!;
    expect(related.span.file).toBe(server.file("src/page.rtsx"));
    expect(sourceText({ ...related.span, where: "related" })).toBe("title");
    expect(related.span.start).toEqual({ line: 6, offset: 3 });
  });

  test("call hierarchy: callers in .rtsx files, and from an item in one", async () => {
    const [item] = [await server.request("prepareCallHierarchy", server.at("src/types.ts", "describe", 1))].flat();
    expect(item.name).toBe("describe");
    const incoming = await server.request("provideCallHierarchyIncomingCalls", server.at("src/types.ts", "describe", 1));
    // The call in the component's own body, and the one in the slot's body:
    // a call in a slot body is the enclosing component's, as in the server.
    expect(incoming.map((call: any) => [path.basename(call.from.file), call.from.name, call.from.selectionSpan.start, call.fromSpans.map((s: any) => s.start)])).toEqual([
      [
        "page.rtsx",
        "Page",
        { line: 9, offset: 17 },
        [
          { line: 11, offset: 17 },
          { line: 18, offset: 12 },
        ],
      ],
    ]);
    // The editor asks on with the item it was given: a source position in page.rtsx.
    const page = incoming[0].from;
    const onward = await server.request("provideCallHierarchyOutgoingCalls", { file: page.file, ...page.selectionSpan.start });
    const calls = onward.map((call: any) => [call.to.name, call.fromSpans.map((s: any) => PAGE.slice(indexOf(PAGE, s.start), indexOf(PAGE, s.end)))]);
    expect(calls.sort()).toEqual([
      ["Button", ["Button"]], // `<Button size>`: the tag, at 16:8
      ["describe", ["describe", "describe"]],
    ]);
    expect(onward.find((call: any) => call.to.name === "Button").fromSpans[0].start).toEqual({ line: 16, offset: 8 });
    // An item in an .rtsx file is at its source position too: `Button` of button.rtsx.
    const button = onward.find((call: any) => call.to.name === "Button").to;
    expect([path.basename(button.file), button.selectionSpan.start]).toEqual(["button.rtsx", { line: 3, offset: 17 }]);
  });

  test("implementations, highlights, file references and raw references are source positions too", async () => {
    const implementation = await server.request("implementation", server.at("src/main.tsx", "<Page", 1, 1));
    expect(implementation.map((found: any) => [path.basename(found.file), found.start])).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);

    const highlights = await server.request("documentHighlights", { ...server.at("src/main.tsx", "Page", 1), filesToSearch: [server.file("src/main.tsx"), server.file("src/page.rtsx")] });
    const inPage = highlights.find((file: any) => file.file.endsWith("page.rtsx"));
    expect(inPage.highlightSpans.map((span: any) => PAGE.slice(indexOf(PAGE, span.start), indexOf(PAGE, span.end)))).toEqual(["Page"]);

    const importers = await server.request("fileReferences", { file: server.file("src/types.ts") });
    expect(
      importers.refs
        .filter((ref: any) => ref.file.endsWith(".rtsx"))
        .map((ref: any) => `${path.basename(ref.file)} ${ref.start.line}:${ref.start.offset} ${sourceText({ ...ref, where: "fileReferences" })}`)
        .sort(),
    ).toEqual(["button.rtsx 1:35 ./types", "page.rtsx 2:27 ./types", "page.rtsx 3:28 ./types"]);

    // The request that answers in offsets, not lines.
    const full = await server.request("references-full", server.at("src/types.ts", "describe", 1));
    const offsets = full.flatMap((symbol: any) => symbol.references).filter((ref: any) => ref.fileName.endsWith("page.rtsx"));
    expect(offsets.map((ref: any) => ref.textSpan.start).sort((a: number, b: number) => a - b)).toEqual([PAGE.indexOf("describe"), PAGE.indexOf("describe(size);"), PAGE.indexOf("describe(size)}")]);
    for (const ref of offsets) {
      expect(PAGE.slice(ref.textSpan.start, ref.textSpan.start + ref.textSpan.length)).toBe("describe");
    }
  });

  test("workspace symbols and file-rename edits of .rtsx are the language server's", async () => {
    const symbols = await server.request("navto", { searchValue: "Page", file: server.file("src/main.tsx") });
    expect(symbols.filter((s: any) => s.file.endsWith(".rtsx"))).toEqual([]);
    const own = await server.request("navto", { searchValue: "describe", file: server.file("src/main.tsx") });
    expect(own.map((s: any) => path.basename(s.file))).toEqual(["types.ts"]);

    // types.ts is imported by page.rtsx and button.rtsx: those edits are the server's.
    const moved = await server.request("getEditsForFileRename", { oldFilePath: server.file("src/types.ts"), newFilePath: server.file("src/kinds.ts") });
    expect(moved.filter((edit: any) => edit.fileName.endsWith(".rtsx"))).toEqual([]);
    // page.rtsx is imported by main.tsx and wrong.tsx: an import of an .rtsx module is the server's too.
    const page = await server.request("getEditsForFileRename", { oldFilePath: server.file("src/page.rtsx"), newFilePath: server.file("src/home.rtsx") });
    expect(page).toEqual([]);
  });

  test("a rename that reaches an .rtsx file is refused, and changes nothing", async () => {
    const before = Object.fromEntries(Object.keys(FILES).map((name) => [name, server.text(name)]));
    for (const [needle, reached] of [
      ["size: Size;", /button\.rtsx|page\.rtsx/], // a prop used as shorthand: `<Button size>`
      ["$Icon", /button\.rtsx|page\.rtsx/], // a `$Slot` member
      ["describe", /page\.rtsx/],
    ] as const) {
      const body = await server.request("rename", { ...server.at("src/types.ts", needle, 1), findInStrings: false, findInComments: false });
      expect(body.info.canRename, needle).toBe(false);
      expect(body.info.localizedErrorMessage, needle).toMatch(reached);
      expect(body.locs, needle).toEqual([]);
    }
    // A name that stays in TypeScript files is renamed as ever.
    const local = await server.request("rename", { ...server.at("src/main.tsx", "props", 1), findInStrings: false, findInComments: false });
    expect(local.info.canRename).toBe(true);
    expect(local.locs.flatMap((file: any) => file.locs)).toHaveLength(2);
    expect(Object.fromEntries(Object.keys(FILES).map((name) => [name, server.text(name)]))).toEqual(before);
  });

  test("a saved .rtsx with a syntax error keeps its exports", async () => {
    // A tag being typed, and a new export after it: the importer sees the new
    // one — so tsserver has the saved text — and still sees `Page`.
    const broken = `${PAGE.replace("<h1 title={label}>{title}</h1>", "<h1 title={label}>{title}</h1><p")}export const added = 1;\n`;
    const strict = spawnSync(binary, ["serve"], { input: `${JSON.stringify({ id: 1, method: "transform", params: { file: server.file("src/page.rtsx"), code: broken } })}\n`, encoding: "utf8" });
    expect(JSON.parse(strict.stdout).result.diagnostics.length, "the strict transform fails on it").toBeGreaterThan(0);
    const before = spawned.count();
    write(dir, "src/page.rtsx", broken);
    write(dir, "src/later.tsx", 'import { Page, added } from "./page";\nexport const use = [Page, added];\n');
    server.open("src/later.tsx");
    await server.until("src/later.tsx", (codes) => codes.length === 0, "the saved file's new export, and Page");
    expect(await server.codes("src/main.tsx")).toEqual([]);
    write(dir, "src/page.rtsx", PAGE);
    await server.until("src/later.tsx", (codes) => codes.includes(2305), "the file saved back: `added` is gone");
    expect(spawned.count() - before).toBe(2); // one process per save, for the one file
  });

  test("an .rtsx file created or deleted is seen without an edit to its importer", async () => {
    write(dir, "src/early.tsx", 'import { late } from "./late";\nexport const use = late;\n');
    server.open("src/early.tsx");
    expect(await server.codes("src/early.tsx")).toEqual([2307]);
    write(dir, "src/late.rtsx", "export const late = <p />;\n");
    await server.until("src/early.tsx", (codes) => codes.length === 0, "late.rtsx was created");
    fs.rmSync(path.join(dir, "src/late.rtsx"));
    await server.until("src/early.tsx", (codes) => codes.includes(2307), "late.rtsx was deleted");
  });

  test("no answer holds a position that page.rtsx does not have", () => {
    // Every span of every response so far, whatever asked for it.
    const spans = server.responses.flatMap(({ command, body }) => rtsxSpans(body).map((span) => ({ ...span, where: `${command} ${span.where}` })));
    expect(spans.length).toBeGreaterThan(20);
    for (const span of spans) {
      sourceText(span);
    }
  });
});

describe.each(VERSIONS)("%s: which binary runs", (alias) => {
  test("a project without .rtsx starts no process", async () => {
    const dir = project({
      "tsconfig.json": TSCONFIG,
      "src/util.ts": "export const twice = (n: number) => n * 2;\n",
      "src/main.ts": 'import { twice } from "./util";\nimport { missing } from "./missing";\nexport const four = [twice(2), missing];\n',
    });
    const spawned = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path });
    try {
      server.open("src/main.ts");
      expect(await server.codes("src/main.ts")).toEqual([2307]); // TypeScript's own answer for "./missing"
      await server.request("references", server.at("src/util.ts", "twice", 1));
      await server.request("rename", { ...server.at("src/util.ts", "twice", 1), findInStrings: false, findInComments: false });
      expect(spawned.count()).toBe(0);
      expect(server.pluginLog().filter((line) => /spawn|binary/.test(line))).toEqual([]);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("the workspace's CLI runs once the client says the workspace is trusted; a tsconfig entry never chooses", async () => {
    const evil = "#!/bin/sh\necho run >> \"$0.count\"\n";
    const dir = project({
      ...FILES,
      // The project names the plugin itself, with a path of its own.
      "tsconfig.json": JSON.stringify({ ...JSON.parse(TSCONFIG), compilerOptions: { ...JSON.parse(TSCONFIG).compilerOptions, plugins: [{ name: PLUGIN, serverPath: "./evil", trusted: true }] } }),
      evil,
      "node_modules/@reactogenic/cli/package.json": JSON.stringify({ name: "@reactogenic/cli", version: "9.0.0" }),
    });
    fs.chmodSync(path.join(dir, "evil"), 0o755);
    const cli = counting(path.join(dir, `node_modules/@reactogenic/cli-${process.platform}-${process.arch}/bin`), binary);
    const server = new TsServer(alias, dir);
    try {
      server.open("src/main.tsx");
      // No binary: the imports are unresolved, as without the plugin.
      expect(new Set(await server.codes("src/main.tsx"))).toEqual(new Set([2307]));
      expect(cli.count()).toBe(0);
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { trusted: true } });
      await server.until("src/main.tsx", (codes) => codes.length === 0, "the workspace's CLI, once trusted");
      expect(cli.count()).toBeGreaterThan(0);
      expect(fs.existsSync(path.join(dir, "evil.count"))).toBe(false);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("the setting, sent by the client, replaces the binary: the texts are made again", async () => {
    const dir = project(FILES);
    const first = counting(path.join(dir, ".bin"), binary, "first");
    const second = counting(path.join(dir, ".bin"), binary, "second");
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: first.path });
    try {
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect([first.count(), second.count()]).toEqual([2, 0]);
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: second.path } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect([first.count(), second.count()]).toEqual([2, 2]);
      const definition = await server.request("definition", server.at("src/main.tsx", "Page", 1));
      expect(definition.map((d: any) => [path.basename(d.file), d.start])).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);
      // The same configuration again changes nothing.
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: second.path } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(second.count()).toBe(2);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a file that yields nothing keeps its last good text, and is made again later", async () => {
    const dir = project(FILES);
    const spawned = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path });
    const definition = async () => (await server.request("definition", server.at("src/main.tsx", "Page", 1))).map((d: any) => [path.basename(d.file), d.start]);
    try {
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(await definition()).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);

      // The binary breaks, and page.rtsx is saved with one more line at its top and one more export.
      fs.writeFileSync(`${spawned.path}.fail`, "");
      write(dir, "src/page.rtsx", `// saved\n${PAGE}export const added = 1;\n`);
      write(dir, "src/later.tsx", 'import { Page, added } from "./page";\nexport const use = [Page, added];\n');
      server.open("src/later.tsx");
      await server.until("src/later.tsx", () => server.pluginLog().some((line) => line.includes("serve failed")), "the transform of the saved file failed");
      // The module is the one from before the save: `Page`, and no `added`.
      expect(await server.codes("src/later.tsx")).toEqual([2305]);
      expect(await server.codes("src/main.tsx")).toEqual([]);
      // Its positions are those of a text the file no longer has: not given out.
      expect(await definition()).toEqual([]);

      // The binary works again: the text is made again without another save.
      fs.rmSync(`${spawned.path}.fail`);
      await server.until("src/later.tsx", (codes) => codes.length === 0, "the saved file's text, made on a later attempt");
      expect(await definition()).toEqual([["page.rtsx", { line: 10, offset: 17 }]]);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a binary without the request leaves the imports unresolved", async () => {
    const dir = project(FILES);
    // As a release from before the plugin answers: an unknown method.
    const old = path.join(dir, ".bin/old");
    write(dir, ".bin/old", '#!/bin/sh\ncat > /dev/null\necho \'{"id":1,"error":"unknown method \\"virtual\\""}\'\n');
    fs.chmodSync(old, 0o755);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: old });
    try {
      server.open("src/main.tsx");
      expect(new Set(await server.codes("src/main.tsx"))).toEqual(new Set([2307]));
      expect(server.pluginLog().join("\n")).toContain('unknown method "virtual"');
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
