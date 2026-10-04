// The TS server plugin (ide.md, *The plugin*) in a real tsserver, TypeScript
// 5.9 and 6.0, driven over stdio as VS Code's TypeScript extension drives it.
// Every `.rtsx` file is on disk only: tsserver never opens one.
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { RETRY_AFTER, TIMEOUT } from "../src/plugin/virtual.ts";
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
  "src/deep/leaf.rtsx": "export const leaf = <i>leaf</i>;\n",
  // The first file of the project: its imports reach three folders.
  "src/aliased.tsx": `import Intro from "@/intro";
import { Page } from "@/page";
import { part } from "@/parts";
import { leaf } from "@/deep/leaf";

export const all = [Intro, Page, part, leaf];
`,
};

/** A project of two files, for the scenarios that need no more: main.tsx imports page.rtsx. */
const SMALL = {
  "tsconfig.json": TSCONFIG,
  "types/jsx.d.ts": types("jsx.d.ts"),
  "types/core.d.ts": types("core.d.ts"),
  "src/page.rtsx": "export function Page({ title }: { title: string }) {\n  return <h1 title>{title}</h1>;\n}\nexport const helper = 1;\n",
  "src/main.tsx": 'import { Page } from "./page";\nexport const app = <Page title="x" />;\n',
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
    // Five `.rtsx` modules in three folders. The first importer resolves to
    // four of them: one process makes those, and the fifth, button.rtsx,
    // which lies beside two of them and is asked for next.
    expect(spawned.count()).toBe(1);
    expect(server.pluginLog().filter((line) => line.includes("spawn")).map((line) => /(\d+) file\(s\)/.exec(line)?.[1])).toEqual(["5"]);
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
    // Extensionless, explicit, a directory's index.rtsx; and through a `paths`
    // alias, where TypeScript answers with the module's declaration — the
    // whole text — instead of the file: its start, for all of them.
    for (const [importer, specifier, file] of [
      ["src/main.tsx", '"./page"', "src/page.rtsx"],
      ["src/main.tsx", '"./button.rtsx"', "src/button.rtsx"],
      ["src/main.tsx", '"./parts"', "src/parts/index.rtsx"],
      ["src/aliased.tsx", '"@/intro"', "src/intro.rtsx"],
      ["src/aliased.tsx", '"@/page"', "src/page.rtsx"],
      ["src/aliased.tsx", '"@/parts"', "src/parts/index.rtsx"],
    ]) {
      const body = await server.request("definitionAndBoundSpan", server.at(importer, specifier, 1, 2));
      expect(
        body.definitions.map((d: any) => [d.file, d.start, d.end]),
        specifier,
      ).toEqual([[server.file(file), { line: 1, offset: 1 }, { line: 1, offset: 1 }]]);
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

  test("workspace symbols of .rtsx files: listed here while the language server does not run, its own once it does", async () => {
    const inRtsx = async (searchValue: string) => (await server.request("navto", { searchValue, file: server.file("src/main.tsx") })).filter((s: any) => s.file.endsWith(".rtsx"));
    // No word from the extension: no `.rtsx` document was opened, no server runs. Each symbol is its declaration in the source.
    const symbols = await inRtsx("Page");
    expect(symbols.map((s: any) => [s.name, path.basename(s.file), s.start])).toEqual([
      ["Page", "page.rtsx", { line: 9, offset: 1 }],
      ["PageProps", "page.rtsx", { line: 5, offset: 1 }],
    ]);
    expect(sourceText({ ...symbols[0], where: "navto" })).toBe(PAGE.slice(PAGE.indexOf("export function Page"), PAGE.lastIndexOf("}") + 1));
    expect(sourceText({ ...symbols[1], where: "navto" })).toBe("export interface PageProps {\n  title: string;\n}");
    // A name the transform made (the import of the segment root `#intro`) is nobody's symbol.
    expect(await inRtsx("_Section")).toEqual([]);

    // The server runs: it lists them, and listed twice they would show twice.
    await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { languageServer: true } });
    expect(await inRtsx("Page")).toEqual([]);
    const own = await server.request("navto", { searchValue: "describe", file: server.file("src/main.tsx") });
    expect(own.map((s: any) => path.basename(s.file))).toEqual(["types.ts"]);
    // And it has stopped.
    await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { languageServer: false } });
    expect((await inRtsx("Page")).map((s: any) => s.name)).toEqual(["Page", "PageProps"]);
    expect(spawned.count()).toBe(1); // that word is no reason to make a text again

    // types.ts is imported by page.rtsx and button.rtsx: no edit goes into an .rtsx file.
    const moved = await server.request("getEditsForFileRename", { oldFilePath: server.file("src/types.ts"), newFilePath: server.file("src/kinds.ts") });
    expect(moved.filter((edit: any) => edit.fileName.endsWith(".rtsx"))).toEqual([]);
  });

  test("a {@link} to a component in an .rtsx file leads to its declaration, whatever it holds", async () => {
    // `Page` has a segment root, a shorthand and slots; `Button` attaches slots: no span of theirs maps exactly.
    write(dir, "src/doc.ts", 'import { Page } from "./page";\nimport { Button } from "./button.rtsx";\n/** See {@link Page} and {@link Button}. */\nexport const documented = [Page, Button];\n');
    server.open("src/doc.ts");
    expect(await server.codes("src/doc.ts")).toEqual([]);
    await server.request("configure", { preferences: { displayPartsForJSDoc: true } });
    const info = await server.request("quickinfo", server.at("src/doc.ts", "documented", 1));
    const links = info.documentation.filter((part: any) => part.kind === "linkName");
    expect(links.map((part: any) => [part.text, path.basename(part.target.file), part.target.start])).toEqual([
      ["Page", "page.rtsx", { line: 9, offset: 1 }],
      ["Button", "button.rtsx", { line: 3, offset: 1 }],
    ]);
    expect(sourceText({ ...links[0].target, where: "link" })).toBe(PAGE.slice(PAGE.indexOf("export function Page"), PAGE.lastIndexOf("}") + 1));
    expect(sourceText({ ...links[1].target, where: "link" })).toMatch(/^export function Button\(.*\n\}$/s);
  });

  test("no edit is offered into an .rtsx file", async () => {
    // `props.missing`: TypeScript's fix would declare the property in PageProps — in page.rtsx.
    write(dir, "src/fix.tsx", 'import type { PageProps } from "./page";\nexport const read = (props: PageProps) => props.missing;\n');
    server.open("src/fix.tsx");
    await server.until("src/fix.tsx", (codes) => codes.includes(2339), "the missing property");
    const at = server.at("src/fix.tsx", "missing", 1);
    const fixes = await server.request("getCodeFixes", { file: at.file, startLine: at.line, startOffset: at.offset, endLine: at.line, endOffset: at.offset + 7, errorCodes: [2339] });
    expect(fixes.flatMap((fix: any) => fix.changes.map((change: any) => path.basename(change.fileName)))).not.toContain("page.rtsx");

    // Move to file, with page.rtsx as the target: refused (by TypeScript itself: not a file it writes).
    const whole = server.at("src/fix.tsx", "export const read", 1);
    const range = { file: whole.file, startLine: whole.line, startOffset: 1, endLine: whole.line, endOffset: 80 };
    const moved = await server.request("getEditsForRefactor", { ...range, refactor: "Move to file", action: "Move to file", interactiveRefactorArguments: { targetFile: server.file("src/page.rtsx") } });
    expect(moved.edits).toEqual([]);
    expect(moved.notApplicableReason).toBeTruthy();
    // And page.rtsx is not among the files it suggests.
    const suggestions = await server.request("getMoveToRefactoringFileSuggestions", range);
    expect(suggestions.files.length).toBeGreaterThan(0);
    expect(suggestions.files.filter((file: string) => file.endsWith(".rtsx"))).toEqual([]);
    // Into a TypeScript file it still works.
    const allowed = await server.request("getEditsForRefactor", { ...range, refactor: "Move to file", action: "Move to file", interactiveRefactorArguments: { targetFile: server.file("src/types.ts") } });
    expect(allowed.edits.map((edit: any) => path.basename(edit.fileName)).sort()).toEqual(["fix.tsx", "types.ts"]);
  });

  test("the requests that walk the whole program work with .rtsx files in it", async () => {
    // Completion with auto-import: `Page` is offered from page.rtsx; the
    // import is an edit of this file, and names the module as the convention
    // is. `twin.rtsx` has a `twin.ts` beside it, which `./twin` would be:
    // its import keeps the extension.
    await server.request("configure", { preferences: { includeCompletionsForModuleExports: true, includeCompletionsWithInsertText: true, allowIncompleteCompletions: true } });
    write(dir, "src/twin.ts", "export const other = 1;\n");
    write(dir, "src/twin.rtsx", "export const Twinned = <p />;\n");
    write(dir, "src/auto.tsx", 'import "./twin.rtsx";\nexport const page = [Pag, Twinn];\n');
    server.open("src/auto.tsx");
    await server.until("src/auto.tsx", (codes) => !codes.includes(2307), "twin.rtsx");
    for (const [typed, name, specifier] of [
      ["Pag,", "Page", "./page"],
      ["Twinn]", "Twinned", "./twin.rtsx"],
    ]) {
      const at = server.at("src/auto.tsx", typed, 1, typed.length - 1);
      const completions = await server.request("completionInfo", { ...at, prefix: typed.slice(0, -1) });
      const entry = completions.entries.find((e: any) => e.name === name && e.source);
      expect(entry, `${name}, from an .rtsx module`).toBeDefined();
      // The list names the module as the import will.
      expect([entry.source, entry.sourceDisplay?.map((part: any) => part.text).join("")], name).toEqual([specifier, specifier]);
      // The editor sends the entry's `data` back: TypeScript finds the entry by it, not by the source.
      const [details] = await server.request("completionEntryDetails", { ...at, entryNames: [{ name, source: entry.source, data: entry.data }] });
      expect(details.codeActions.map((action: any) => action.description)).toEqual([`Add import from "${specifier}"`]);
      expect(details.sourceDisplay?.map((part: any) => part.text).join("")).toBe(specifier);
      const edits = details.codeActions.flatMap((action: any) => action.changes);
      expect(edits.map((change: any) => path.basename(change.fileName))).toEqual(["auto.tsx"]);
      expect(edits[0].textChanges.map((edit: any) => edit.newText.trim())).toEqual([`import { ${name} } from "${specifier}";`]);
    }

    // The project's errors, file by file; its file list; refactorings; the outline of a file.
    const project = await server.request("projectInfo", { file: server.file("src/main.tsx"), needFileNameList: true });
    expect(project.fileNames.filter((name: string) => name.endsWith(".rtsx")).map((name: string) => path.basename(name)).sort()).toEqual(["button.rtsx", "index.rtsx", "intro.rtsx", "leaf.rtsx", "page.rtsx", "twin.rtsx"]);
    const main = server.at("src/main.tsx", "const props", 1);
    await server.request("getApplicableRefactors", { file: main.file, startLine: main.line, startOffset: 1, endLine: main.line, endOffset: 40 });
    await server.request("organizeImports", { scope: { type: "file", args: { file: server.file("src/main.tsx") } } });
    await server.request("navtree", { file: server.file("src/main.tsx") });
    await server.request("quickinfo", server.at("src/main.tsx", "Page", 1));
    await server.request("compilerOptionsDiagnostics-full", { projectFileName: project.configFileName });
    // An .rtsx file's own diagnostics are not tsserver's to give.
    expect(await server.request("semanticDiagnosticsSync", { file: server.file("src/page.rtsx") })).toEqual([]);
    expect(await server.request("syntacticDiagnosticsSync", { file: server.file("src/page.rtsx") })).toEqual([]);
    expect(await server.request("suggestionDiagnosticsSync", { file: server.file("src/page.rtsx") })).toEqual([]);
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
      expect([first.count(), second.count()]).toEqual([1, 0]);
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: second.path } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect([first.count(), second.count()]).toEqual([1, 1]); // every text again, in one process
      const definition = await server.request("definition", server.at("src/main.tsx", "Page", 1));
      expect(definition.map((d: any) => [path.basename(d.file), d.start])).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);
      // The same configuration again changes nothing.
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: second.path } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(second.count()).toBe(1);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("the setting replaces the binary in every project", async () => {
    const config = JSON.stringify({ compilerOptions: JSON.parse(TSCONFIG).compilerOptions, include: ["src", "../types"] });
    const dir = project({
      "types/jsx.d.ts": types("jsx.d.ts"),
      "a/tsconfig.json": config,
      "a/src/page.rtsx": SMALL["src/page.rtsx"],
      "a/src/main.tsx": SMALL["src/main.tsx"],
      "b/tsconfig.json": config,
      "b/src/page.rtsx": SMALL["src/page.rtsx"],
      "b/src/main.tsx": SMALL["src/main.tsx"],
    });
    const first = counting(path.join(dir, ".bin"), binary, "first");
    const second = counting(path.join(dir, ".bin"), binary, "second");
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: first.path });
    try {
      server.open("a/src/main.tsx");
      server.open("b/src/main.tsx");
      expect([await server.codes("a/src/main.tsx"), await server.codes("b/src/main.tsx")]).toEqual([[], []]);
      expect([first.count(), second.count()]).toEqual([2, 0]); // a batch per project
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: second.path } });
      expect([await server.codes("a/src/main.tsx"), await server.codes("b/src/main.tsx")]).toEqual([[], []]);
      expect([first.count(), second.count()]).toEqual([2, 2]);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a project loaded again (TypeScript: Reload Project, an edit of tsconfig.json) keeps one text per file, at source positions", async () => {
    // tsserver enables the plugin again for a project it loads again: over the host and the service it has wrapped.
    const dir = project(FILES);
    const spawned = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path });
    const definition = async () => (await server.request("definition", server.at("src/main.tsx", "Page", 1))).map((d: any) => [path.basename(d.file), d.start]);
    try {
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(await definition()).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);
      expect(spawned.count()).toBe(1);

      server.notify("reloadProjects", {});
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(await definition()).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);

      const config = JSON.parse(TSCONFIG);
      write(dir, "tsconfig.json", JSON.stringify({ ...config, compilerOptions: { ...config.compilerOptions, noUnusedLocals: true } }));
      write(dir, "src/unused.tsx", 'import { Page } from "./page";\nexport {};\n');
      server.open("src/unused.tsx");
      await server.until("src/unused.tsx", (codes) => codes.includes(6133), "the new tsconfig.json");
      expect(await definition()).toEqual([["page.rtsx", { line: 9, offset: 17 }]]);
      expect(spawned.count()).toBe(1); // no text was made a second time, of a text
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a configuration sent before any project is open is the one its first project has", async () => {
    // As when the extension is active before VS Code starts tsserver: no
    // binary in the environment, the setting's alone.
    const dir = project(FILES);
    const configured = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir);
    try {
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: configured.path, trusted: true } });
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(configured.count()).toBe(1);
      expect(server.pluginLog().join("\n")).not.toContain("reloading");
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

  test("an explicit ./x.rtsx resolves where a relative import must name its file (node16, a module)", async () => {
    const dir = project({
      "package.json": JSON.stringify({ type: "module" }),
      "tsconfig.json": JSON.stringify({ compilerOptions: { strict: true, jsx: "preserve", module: "node16", target: "es2022", lib: ["es2022"], types: [], noEmit: true }, include: ["src"] }),
      "src/value.rtsx": "export const value = 1;\n",
      "src/main.ts": 'import { value } from "./value.rtsx";\nimport { value as bare } from "./value";\nexport const both = [value, bare];\n',
    });
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: binary });
    try {
      server.open("src/main.ts");
      // The extensionless one is an error there for any file, TypeScript's own.
      const diagnostics = await server.diagnostics("src/main.ts");
      expect(diagnostics.map((d) => `${d.start.line} TS${d.code}`)).toEqual(["2 TS2834"]);
      const definition = await server.request("definition", server.at("src/main.ts", "value", 1));
      expect(definition.map((d: any) => [path.basename(d.file), d.start])).toEqual([["value.rtsx", { line: 1, offset: 14 }]]);
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

  test("a relative path names a file of the workspace: not run until the workspace is trusted", async () => {
    // The user's own setting, or environment, holds `tools/reactogenic`; the repository holds a file there.
    for (const from of ["setting", "environment"] as const) {
      const dir = project(SMALL);
      const tool = counting(path.join(dir, "tools"), binary);
      const server = new TsServer(alias, dir, from === "environment" ? { REACTOGENIC_BINARY: "tools/reactogenic" } : {});
      const configuration = { serverPath: from === "setting" ? "tools/reactogenic" : null, workspaceFolder: dir };
      try {
        await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, trusted: false } });
        server.open("src/main.tsx");
        expect(await server.codes("src/main.tsx"), from).toEqual([2307]);
        expect(tool.count(), from).toBe(0);
        expect(server.pluginLog().join("\n"), from).toMatch(/tools\/reactogenic is relative, and the workspace is not trusted: not run/);
        await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, trusted: true } });
        await server.until("src/main.tsx", (codes) => codes.length === 0, `${from}: the file runs once the workspace is trusted`);
        expect(tool.count(), from).toBeGreaterThan(0);
      } finally {
        await server.close();
        fs.rmSync(dir, { recursive: true, force: true });
      }
    }
  });

  test("the workspace's CLI installed later: found when the client says a lockfile changed; replaced in place, its texts are made again", async () => {
    const dir = project(SMALL);
    const server = new TsServer(alias, dir);
    const configuration = { trusted: true, workspaceFolder: dir };
    try {
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, installs: 0 } });
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([2307]);

      // `pnpm install`: the CLI and its platform package appear.
      write(dir, "node_modules/@reactogenic/cli/package.json", JSON.stringify({ name: "@reactogenic/cli", version: "9.0.0" }));
      const cli = counting(path.join(dir, `node_modules/@reactogenic/cli-${process.platform}-${process.arch}/bin`), binary);
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, installs: 1 } });
      await server.until("src/main.tsx", (codes) => codes.length === 0, "the CLI, after the install");
      expect(cli.count()).toBe(1);
      expect(server.pluginLog().join("\n")).toContain("reloading the projects");

      // Another lockfile change that left the CLI alone: nothing is made again.
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, installs: 2 } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(cli.count()).toBe(1);

      // An upgrade in place (npm's flat layout): the same path, another file.
      fs.appendFileSync(cli.path, "# upgraded\n");
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { ...configuration, installs: 3 } });
      expect(await server.codes("src/main.tsx")).toEqual([]);
      expect(cli.count()).toBe(2);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("the workspace's CLI installed later, and no word from the client: found at an importer's next edit, for every importer", async () => {
    const dir = project({ ...SMALL, "src/second.tsx": 'import { Page } from "./page";\nexport const second = <Page title="2" />;\n' });
    const server = new TsServer(alias, dir);
    try {
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { trusted: true, workspaceFolder: dir } });
      server.open("src/main.tsx");
      server.open("src/second.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([2307]);
      expect(await server.codes("src/second.tsx")).toEqual([2307]);
      write(dir, "node_modules/@reactogenic/cli/package.json", JSON.stringify({ name: "@reactogenic/cli", version: "9.0.0" }));
      const cli = counting(path.join(dir, `node_modules/@reactogenic/cli-${process.platform}-${process.arch}/bin`), binary);
      // "No binary" is believed for RETRY_AFTER, not for good.
      await new Promise((resolve) => setTimeout(resolve, RETRY_AFTER + 500));
      server.insert("src/main.tsx", 1, "// edited\n");
      await server.until("src/main.tsx", (codes) => codes.length === 0, "the CLI, at the importer's edit");
      // second.tsx was not edited: its import is resolved by loading the projects again.
      await server.until("src/second.tsx", (codes) => codes.length === 0, "the other importer");
      expect(cli.count()).toBeGreaterThan(0);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a binary that answers something else leaves the imports unresolved, and every request answered", async () => {
    const dir = project(SMALL);
    // The right number of entries, each of another shape: a CLI of another version.
    const answers = {
      notext: '{"id":1,"result":{"files":[{"file":"x"}]}}',
      nulls: '{"id":1,"result":{"files":[{"file":"x","text":null,"spans":null}]}}',
      nothing: '{"id":1,"result":{"files":[null]}}',
      pairs: '{"id":1,"result":{"files":[{"file":"x","text":"export {};","spans":[[0,10]]}]}}',
      strings: '{"id":1,"result":{"files":[{"file":"x","text":"export {};","spans":[["0","10","0","0","1","0"]]}]}}',
    };
    for (const [name, answer] of Object.entries(answers)) {
      write(dir, `.bin/${name}`, `#!/bin/sh\ncat > /dev/null\necho '${answer}'\n`);
      fs.chmodSync(path.join(dir, ".bin", name), 0o755);
    }
    const server = new TsServer(alias, dir);
    try {
      server.open("src/main.tsx");
      for (const name of Object.keys(answers)) {
        await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: path.join(dir, ".bin", name) } });
        expect(await server.codes("src/main.tsx"), name).toEqual([2307]);
        // TypeScript's own answers, not "Cannot read properties of undefined".
        expect((await server.request("definition", server.at("src/main.tsx", "Page", 1))).map((d: any) => path.basename(d.file)), name).toEqual(["main.tsx"]);
        expect((await server.request("references", server.at("src/main.tsx", "Page", 1))).refs.length, name).toBe(2);
        expect(server.pluginLog().join("\n"), name).toContain(`${name} serve failed: its answer for ${server.file("src/page.rtsx")} is not a text and its spans`);
      }
      await server.request("configurePlugin", { pluginName: PLUGIN, configuration: { serverPath: binary } });
      await server.until("src/main.tsx", (codes) => codes.length === 0, "the real binary");
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a binary that never answers holds tsserver up once, for seconds, and is then left alone", async () => {
    const dir = project(SMALL);
    write(dir, ".bin/hangs", '#!/bin/sh\necho run >> "$0.count"\nexec sleep 60\n');
    fs.chmodSync(path.join(dir, ".bin/hangs"), 0o755);
    const runs = () => fs.readFileSync(path.join(dir, ".bin/hangs.count"), "utf8").split("\n").filter(Boolean).length;
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: path.join(dir, ".bin/hangs") });
    try {
      server.open("src/main.tsx");
      let started = Date.now();
      expect(await server.codes("src/main.tsx")).toEqual([2307]);
      const blocked = Date.now() - started;
      expect(blocked).toBeGreaterThanOrEqual(TIMEOUT);
      expect(blocked).toBeLessThan(TIMEOUT + 5_000);
      expect(runs()).toBe(1);
      expect(server.pluginLog().join("\n")).toMatch(/hangs serve failed: no answer within \d+ ms — not run again for 60 s/);

      // A failure is tried again after RETRY_AFTER, at the importer's next edit; a binary that hung is not.
      await new Promise((resolve) => setTimeout(resolve, RETRY_AFTER + 500));
      server.insert("src/main.tsx", 1, "// edited\n");
      started = Date.now();
      expect(await server.codes("src/main.tsx")).toEqual([2307]);
      expect(Date.now() - started).toBeLessThan(2_000);
      expect(runs()).toBe(1);

      // Until the file at its path is another one.
      fs.writeFileSync(path.join(dir, ".bin/hangs"), `#!/bin/sh\nexec "${binary}" "$@"\n`);
      server.insert("src/main.tsx", 1, "// edited again\n");
      await server.until("src/main.tsx", (codes) => codes.length === 0, "the binary, replaced");
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("VS Code's second tsserver, which answers while the first one loads the projects, runs nothing", async () => {
    const dir = project(SMALL);
    const spawned = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path }, ["--serverMode", "partialSemantic"]);
    try {
      server.open("src/main.tsx");
      // What that server is asked: the import is where it ends, as for any module it has not loaded.
      const definition = await server.request("definitionAndBoundSpan", server.at("src/main.tsx", "Page", 2));
      expect(definition.definitions.map((d: any) => path.basename(d.file))).toEqual(["main.tsx"]);
      await server.request("quickinfo", server.at("src/main.tsx", "app", 1));
      expect(spawned.count()).toBe(0);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe.each(VERSIONS)("%s: a project among others", (alias) => {
  test("an unresolved import is looked up a second time once per folder, not once per importer", async () => {
    // A clone before its install: 8 folders of 12 files, each with three packages that are not there and one file that is not.
    const FOLDERS = 8;
    const PER_FOLDER = 12;
    const files: Record<string, string> = { "tsconfig.json": TSCONFIG };
    for (let folder = 0; folder < FOLDERS; folder++) {
      const names = Array.from({ length: PER_FOLDER }, (_, i) => `m${i}`);
      for (const name of names) {
        files[`src/d${folder}/${name}.ts`] = `import * as a from "react";\nimport * as b from "@scope/ui/button";\nimport * as c from "zod";\nimport * as d from "./gone";\nexport const ${name} = [a, b, c, d];\n`;
      }
      files[`src/d${folder}/index.ts`] = `${names.map((name) => `import { ${name} } from "./${name}";`).join("\n")}\nexport const all${folder} = [${names.join(", ")}];\n`;
    }
    files["src/main.ts"] = `${Array.from({ length: FOLDERS }, (_, folder) => `import { all${folder} } from "./d${folder}/index";`).join("\n")}\nexport const all = [${Array.from({ length: FOLDERS }, (_, folder) => `all${folder}`).join(", ")}];\n`;
    const dir = project(files);
    const spawned = counting(path.join(dir, ".bin"), binary);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: spawned.path });
    try {
      server.open("src/main.ts");
      expect(await server.codes("src/main.ts")).toEqual([]);
      server.open("src/d3/m5.ts");
      expect(await server.codes("src/d3/m5.ts")).toEqual([2307, 2307, 2307, 2307]); // TypeScript's own answers
      const lookups = server
        .pluginLog()
        .map((line) => /(\d+) unresolved import\(s\) looked up as \.rtsx/.exec(line)?.[1])
        .reduce((sum, count) => sum + Number(count ?? 0), 0);
      // Four names in each folder — not in each of its twelve files.
      expect(lookups).toBe(FOLDERS * 4);
      expect(spawned.count()).toBe(0);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("a composite project: no TS6307 for an .rtsx module its `include` covers, as in `reactogenic check`", async () => {
    const check = (dir: string) => {
      const result = spawnSync(binary, ["check", "-p", "tsconfig.json", "--pretty=false"], { cwd: dir, encoding: "utf8" });
      return { status: result.status, errors: [...`${result.stdout}${result.stderr}`.matchAll(/^(\S+)\((\d+),(\d+)\): error (TS\d+)/gm)].map((m) => `${m[1]} ${m[2]}:${m[3]} ${m[4]}`) };
    };
    const options = { ...JSON.parse(TSCONFIG).compilerOptions, noEmit: undefined, composite: true, declaration: true, outDir: "dist", rootDir: "src" };
    const sources = {
      "types/jsx.d.ts": types("jsx.d.ts"),
      "src/page.rtsx": SMALL["src/page.rtsx"],
      "src/extra.ts": "export const extra = 1;\n",
      "src/index.ts": 'export { Page } from "./page";\nexport { extra } from "./extra";\n',
    };
    // As monorepos with project references write it: every file under `include`.
    const covered = project({ ...sources, "tsconfig.json": JSON.stringify({ compilerOptions: options, include: ["src", "types"] }) });
    // And one whose list names neither module: the error is TypeScript's for both, and `check`'s.
    const named = project({ ...sources, "tsconfig.json": JSON.stringify({ compilerOptions: options, include: ["src/index.ts", "types"] }) });
    const servers = [covered, named].map((dir) => new TsServer(alias, dir, { REACTOGENIC_BINARY: binary }));
    try {
      const reported = async (server: TsServer) => (await server.diagnostics("src/index.ts")).map((d) => `src/index.ts ${d.start.line}:${d.start.offset} TS${d.code}`);
      servers[0].open("src/index.ts");
      expect(await reported(servers[0])).toEqual([]);
      expect(check(covered)).toEqual({ status: 0, errors: [] });

      servers[1].open("src/index.ts");
      const both = ["src/index.ts 1:22 TS6307", "src/index.ts 2:23 TS6307"];
      expect(await reported(servers[1])).toEqual(both);
      expect(check(named)).toEqual({ status: 1, errors: both });
    } finally {
      await Promise.all(servers.map((server) => server.close()));
      fs.rmSync(covered, { recursive: true, force: true });
      fs.rmSync(named, { recursive: true, force: true });
    }
  });

  test("two projects share a file: a rename that reaches an .rtsx file of either is refused, whichever is asked", async () => {
    const config = JSON.stringify({ compilerOptions: JSON.parse(TSCONFIG).compilerOptions, include: ["src", "../shared", "../types"] });
    const files = {
      "types/jsx.d.ts": types("jsx.d.ts"),
      "shared/util.ts": "export function helper(n: number) { return n; }\nexport function plain(n: number) { return n; }\n",
      // Project a holds no .rtsx file at all.
      "a/tsconfig.json": config,
      "a/src/main.ts": 'import { helper, plain } from "../../shared/util";\nexport const one = [helper(1), plain(1)];\n',
      "b/tsconfig.json": config,
      "b/src/page.rtsx": 'import { helper } from "../../shared/util";\nexport const Page = () => <p>{helper(2)}</p>;\n',
      "b/src/main.ts": 'import { Page } from "./page";\nexport const page = Page;\n',
      "b/src/plain.ts": 'import { helper, plain } from "../../shared/util";\nexport const three = [helper(3), plain(3)];\n',
    };
    const dir = project(files);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: binary });
    try {
      // a first: it is the project tsserver asks about a file the two share.
      server.open("a/src/main.ts");
      expect(await server.codes("a/src/main.ts")).toEqual([]);
      server.open("shared/util.ts");
      server.open("b/src/main.ts");
      server.open("b/src/plain.ts");
      expect(await server.codes("b/src/main.ts")).toEqual([]);
      const rename = (name: string, needle: string, nth: number) => server.request("rename", { ...server.at(name, needle, nth), findInStrings: false, findInComments: false });

      // The declaration, a use in the project without .rtsx, a use in the other.
      for (const [name, nth] of [
        ["shared/util.ts", 1],
        ["a/src/main.ts", 2],
        ["b/src/plain.ts", 2],
      ] as const) {
        const body = await rename(name, "helper", nth);
        expect(body.info.canRename, name).toBe(false);
        expect(body.info.localizedErrorMessage, name).toMatch(/This rename reaches .*src\/page\.rtsx/);
        expect(body.locs, name).toEqual([]);
      }
      // A name no .rtsx file uses is renamed in both projects, as without the plugin.
      const plain = await rename("shared/util.ts", "plain", 1);
      expect(plain.info.canRename).toBe(true);
      expect(plain.locs.map((file: any) => `${path.relative(dir, file.file)} ${file.locs.length}`).sort()).toEqual(["a/src/main.ts 2", "b/src/plain.ts 2", "shared/util.ts 1"]);
      for (const name of Object.keys(files)) {
        expect(server.text(name), name).toBe(files[name as keyof typeof files]);
      }
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe.each(VERSIONS)("%s: the imports tsserver writes", (alias) => {
  const SOURCES = {
    ...SMALL,
    "src/main.tsx": 'import { Page } from "./page";\nexport const app = [<Page title="x" />, "kept/name.rtsx"];\nexport const other = 1;\n',
    "src/sub/target.tsx": "export const target = 1;\n",
    "src/statement.tsx": "import Pag\nexport {};\n",
    "src/paste.tsx": "export const here = 1;\n",
    "src/uses.tsx": "export const uses = [helper, <Page title='y' />];\n",
  };
  let dir: string;
  let server: TsServer;
  /** Every text an edit writes, with its file. */
  const written = (edits: any[]) => edits.flatMap((edit: any) => edit.textChanges.map((change: any) => `${path.relative(dir, edit.fileName)}: ${change.newText.trim()}`)).filter((text: string) => !text.endsWith(": "));

  beforeAll(async () => {
    dir = project(SOURCES);
    server = new TsServer(alias, dir, { REACTOGENIC_BINARY: binary });
    for (const name of ["src/main.tsx", "src/sub/target.tsx", "src/statement.tsx", "src/paste.tsx", "src/uses.tsx"]) {
      server.open(name);
    }
    expect(await server.codes("src/main.tsx")).toEqual([]);
    await server.request("configure", {
      preferences: { includeCompletionsForModuleExports: true, includeCompletionsWithInsertText: true, includeCompletionsForImportStatements: true, includeCompletionsWithSnippetText: true, allowIncompleteCompletions: true },
    });
  });
  afterAll(async () => {
    await server?.close();
    fs.rmSync(dir, { recursive: true, force: true });
  });

  test("the quick fix for a name that is not imported, and the fix for all of them", async () => {
    expect(await server.codes("src/uses.tsx")).toEqual([2304, 2304]);
    const at = server.at("src/uses.tsx", "helper", 1);
    const fixes = await server.request("getCodeFixes", { file: at.file, startLine: at.line, startOffset: at.offset, endLine: at.line, endOffset: at.offset + 6, errorCodes: [2304] });
    const fix = fixes.find((candidate: any) => candidate.fixName === "import");
    expect(fix.description).toBe('Add import from "./page"');
    expect(written(fix.changes)).toEqual(['src/uses.tsx: import { helper } from "./page";']);
    const all = await server.request("getCombinedCodeFix", { scope: { type: "file", args: { file: at.file } }, fixId: "fixMissingImport" });
    expect(written(all.changes)).toEqual(['src/uses.tsx: import { helper, Page } from "./page";']);
  });

  test("an import statement being typed: the entry's text is the statement", async () => {
    const at = server.at("src/statement.tsx", "Pag", 1, 3);
    const completions = await server.request("completionInfo", { ...at, prefix: "Pag" });
    const entries = completions.entries.filter((entry: any) => entry.name === "Page");
    expect(entries.map((entry: any) => [entry.insertText, entry.source])).toEqual([['import { Page$1 } from "./page";', "./page"]]);
  });

  test("a statement moved to another file takes its import along; a string in it stays as written", async () => {
    const start = server.at("src/main.tsx", "export const app", 1);
    const range = { file: start.file, startLine: start.line, startOffset: 1, endLine: start.line, endOffset: SOURCES["src/main.tsx"].split("\n")[start.line - 1].length + 1 };
    const moved = await server.request("getEditsForRefactor", { ...range, refactor: "Move to file", action: "Move to file", interactiveRefactorArguments: { targetFile: server.file("src/sub/target.tsx") } });
    // From src/sub the `paths` alias is the shorter name: it loses the extension as a relative one does.
    expect(written(moved.edits)).toEqual(['src/sub/target.tsx: import { Page } from "@/page";', 'src/sub/target.tsx: export const app = [<Page title="x" />, "kept/name.rtsx"];']);
    const fresh = await server.request("getEditsForRefactor", { ...range, refactor: "Move to a new file", action: "Move to a new file" });
    expect(written(fresh.edits)).toEqual(['src/app.tsx: import { Page } from "./page";\n\nexport const app = [<Page title="x" />, "kept/name.rtsx"];']);
  });

  test("pasted code brings its import", async () => {
    const copied = server.at("src/main.tsx", "export const other", 1);
    const body = await server.request("getPasteEdits", {
      file: server.file("src/paste.tsx"),
      pastedText: ['export const pasted = <Page title="z" />;'],
      pasteLocations: [{ start: { line: 2, offset: 1 }, end: { line: 2, offset: 1 } }],
      copiedFrom: { file: server.file("src/main.tsx"), spans: [{ start: { line: copied.line - 1, offset: 1 }, end: { line: copied.line - 1, offset: 60 } }] },
    });
    expect(written(body.edits)).toEqual(['src/paste.tsx: import { Page } from "./page";', 'src/paste.tsx: export const pasted = <Page title="z" />;']);
  });
});

describe.each(VERSIONS)("%s: a folder with .rtsx files is moved", (alias) => {
  const SOURCES = {
    ...SMALL,
    "src/parts/item.rtsx": "export const item = <i>item</i>;\n",
    "src/parts/util.ts": "export const util = 1;\n",
    // An importer that moves with its module, and one of a module that stays.
    "src/parts/uses.tsx": 'import { item } from "./item";\nimport { Page } from "../page";\nexport const uses = [item, Page];\n',
    "src/other.rtsx": 'import { util } from "./parts/util";\nexport const other = <p>{util}</p>;\n',
    "src/main.tsx": 'import { item } from "./parts/item";\nimport { util } from "./parts/util";\nimport { other } from "./other";\nimport { uses } from "./parts/uses";\nexport const all = [item, util, other, uses];\n',
  };
  /** The specifiers an answer writes, with their files. */
  const specifiers = (dir: string, edits: any[]) => edits.map((edit: any) => `${path.relative(dir, edit.fileName)}: ${edit.textChanges.map((change: any) => `${change.start.line} ${change.newText}`).join(", ")}`);

  // The editor asks tsserver once the folder has moved (VS Code's
  // TypeScript, on `onDidRenameFiles`); the language server was asked before
  // (`willRenameFiles`), and its edits are in the files by then.
  test("no language server: tsserver updates the import of the .rtsx module in a TypeScript file", async () => {
    const dir = project(SOURCES);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: binary });
    try {
      server.open("src/main.tsx");
      expect(await server.codes("src/main.tsx")).toEqual([]);
      fs.renameSync(path.join(dir, "src/parts"), path.join(dir, "src/pieces"));
      await server.until("src/main.tsx", (codes) => codes.includes(2307), "the folder has moved");
      const edits = await server.request("getEditsForFileRename", { oldFilePath: server.file("src/parts"), newFilePath: server.file("src/pieces") });
      // The imports of main.tsx. other.rtsx imports `./parts/util` too: no edit goes into an .rtsx file.
      // uses.tsx moved with `./item`, and away from nothing: its imports stay as written — no `./item.rtsx`.
      expect(specifiers(dir, edits)).toEqual(["src/main.tsx: 1 ./pieces/item, 2 ./pieces/util, 4 ./pieces/uses"]);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  test("with the language server's edit already in the file, nothing is written twice", async () => {
    const dir = project(SOURCES);
    const server = new TsServer(alias, dir, { REACTOGENIC_BINARY: binary });
    try {
      // The buffer as the server's edit left it: the import of the .rtsx module is its.
      server.open("src/main.tsx", SOURCES["src/main.tsx"].replace("./parts/item", "./pieces/item"));
      fs.renameSync(path.join(dir, "src/parts"), path.join(dir, "src/pieces"));
      await server.until("src/main.tsx", (codes) => codes.length === 2 && codes.every((code) => code === 2307), "the folder has moved: `./parts/util` and `./parts/uses` alone are unresolved");
      const edits = await server.request("getEditsForFileRename", { oldFilePath: server.file("src/parts"), newFilePath: server.file("src/pieces") });
      expect(specifiers(dir, edits)).toEqual(["src/main.tsx: 2 ./pieces/util, 4 ./pieces/uses"]);
    } finally {
      await server.close();
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
