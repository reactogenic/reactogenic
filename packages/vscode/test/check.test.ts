// The task *reactogenic: check* and its problem matchers `$reactogenic` and
// `$reactogenic-ts`, against what the binary really prints for test/fixture.
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { describe, expect, test } from "vitest";
import { checkArgs, isUnsupported, typedEnds } from "../src/protocol.ts";

const root = path.resolve(import.meta.dirname, "..");
const fixture = path.join(root, "test/fixture");
const manifest = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const { contributes } = manifest;

interface Pattern {
  name: string;
  regexp: string;
  file: number;
  line: number;
  column: number;
  severity: number;
  code: number;
  message: number;
}
const patterns: Pattern[] = contributes.problemPatterns;

function check(...args: string[]): { lines: string[]; status: number | null } {
  const result = spawnSync(process.env.REACTOGENIC_BINARY!, args, { cwd: fixture, encoding: "utf8" });
  expect(result.error).toBeUndefined();
  return { lines: result.stdout.split("\n").filter(Boolean), status: result.status };
}

/** A line as VS Code's problem collector reads it, with the matcher that took it: one at most. */
function problem(line: string) {
  const read = patterns.flatMap((pattern) => {
    const m = new RegExp(pattern.regexp).exec(line);
    return m
      ? [
          {
            matcher: `$${pattern.name}`,
            file: m[pattern.file],
            line: Number(m[pattern.line]),
            column: Number(m[pattern.column]),
            severity: m[pattern.severity],
            code: m[pattern.code],
            message: m[pattern.message],
          },
        ]
      : [];
  });
  expect(read.length).toBeLessThanOrEqual(1);
  return read[0] ?? null;
}

describe("$reactogenic and $reactogenic-ts", () => {
  test("between them read every line of `reactogenic check --pretty=false`, each by one", () => {
    const { lines, status } = check(...checkArgs({}));
    expect(status).toBe(1);
    const rtsx = { matcher: "$reactogenic", file: "src/broken.rtsx" };
    expect(lines.map(problem)).toEqual([
      { ...rtsx, line: 4, column: 9, severity: "error", code: "TS2322", message: "Type 'string' is not assignable to type 'number'." },
      { ...rtsx, line: 7, column: 16, severity: "warning", code: "segment-children", message: "Contents will be overwritten by the segment `intro`" },
      { ...rtsx, line: 9, column: 10, severity: "error", code: "undeclared-slot", message: "`$Badge` is not declared in `Button`" },
      // A .ts file: the code as `$tsc` reads it, the number alone.
      { matcher: "$reactogenic-ts", file: "src/util.ts", line: 3, column: 14, severity: "error", code: "2322", message: "Type 'string' is not assignable to type 'number'." },
    ]);
  });

  test("the same with -p, from the task's `project`", () => {
    expect(checkArgs({ project: "tsconfig.json" })).toEqual(["check", "--pretty=false", "-p", "tsconfig.json"]);
    expect(check(...checkArgs({ project: "tsconfig.json" })).lines.map(problem)).toHaveLength(4);
  });

  test("read nothing of the pretty output, which the task does not ask for", () => {
    const { lines } = check("check");
    expect(lines.length).toBeGreaterThan(4);
    expect(lines.map(problem).filter(Boolean)).toEqual([]);
  });

  test.each([
    ["C:\\ws\\src\\page.rtsx(1,2): error TS1005: ';' expected.", { matcher: "$reactogenic", file: "C:\\ws\\src\\page.rtsx", line: 1, column: 2, code: "TS1005" }],
    ["/abs/src/a (copy).rtsx(10,20): warning segment-children: x", { matcher: "$reactogenic", file: "/abs/src/a (copy).rtsx", line: 10, column: 20, severity: "warning" }],
    // By the extension: every file that is not .rtsx is TypeScript's.
    ["C:\\ws\\src\\main.tsx(1,2): error TS1005: ';' expected.", { matcher: "$reactogenic-ts", file: "C:\\ws\\src\\main.tsx", line: 1, column: 2, code: "1005" }],
    ["/abs/src/a (copy).d.mts(10,20): warning TS6133: x", { matcher: "$reactogenic-ts", file: "/abs/src/a (copy).d.mts", severity: "warning", code: "6133" }],
    ["src/rtsx.ts(1,1): error TS1: x", { matcher: "$reactogenic-ts", file: "src/rtsx.ts" }],
    ["src/page.rtsx.ts(1,1): error TS1: x", { matcher: "$reactogenic-ts", file: "src/page.rtsx.ts" }],
    // The first position is the file's: a message may hold something that looks like one — of the other kind, too.
    ["src/a.rtsx(1,2): error TS1: see b.ts(3,4): error TS2: nested", { matcher: "$reactogenic", file: "src/a.rtsx", line: 1, code: "TS1", message: "see b.ts(3,4): error TS2: nested" }],
    ["src/b.ts(3,4): error TS2: see a.rtsx(1,2): error TS1: nested", { matcher: "$reactogenic-ts", file: "src/b.ts", line: 3, code: "2", message: "see a.rtsx(1,2): error TS1: nested" }],
  ])("%s", (line, want) => {
    expect(problem(line)).toMatchObject(want);
  });

  test.each([
    "  src/button.rtsx:8:3 - `$Icon` is declared here", // related information
    "error TS18003: No inputs were found", // no file: nothing to attach a problem to
    "src/broken.rtsx:4:9 - error TS2322: pretty",
    "src/util.ts:3:14 - error TS2322: pretty",
    "Found 2 error(s).",
    "",
  ])("not a problem: %j", (line) => {
    expect(problem(line)).toBeNull();
  });

  test("are applied to closed documents only, relative to the task's folder; each line's owner is the owner of the server that reports the file once it is open", () => {
    const shared = { applyTo: "closedDocuments", fileLocation: ["autoDetect", "${cwd}"] };
    expect(contributes.problemMatchers).toEqual([
      {
        name: "reactogenic",
        label: "reactogenic check: .rtsx files",
        owner: "reactogenic", // our server's diagnostic collection: src/server.ts
        source: "reactogenic",
        ...shared,
        pattern: "$reactogenic",
      },
      {
        name: "reactogenic-ts",
        label: "reactogenic check: TypeScript files",
        owner: "typescript", // VS Code's TypeScript, which our server leaves these files to: as `$tsc`
        source: "ts",
        ...shared,
        pattern: "$reactogenic-ts",
      },
    ]);
    expect(patterns.map((pattern) => pattern.name)).toEqual(["reactogenic", "reactogenic-ts"]);
  });

  test("the task uses both", () => {
    const task = fs.readFileSync(path.join(root, "src/task.ts"), "utf8");
    expect(task).toContain('export const MATCHERS = ["$reactogenic", "$reactogenic-ts"];');
  });
});

describe("the manifest's client half", () => {
  test("the task type", () => {
    expect(contributes.taskDefinitions).toMatchObject([{ type: "reactogenic", required: ["command"] }]);
    expect(contributes.taskDefinitions[0].properties.command.enum).toEqual(["check"]);
  });

  test("the commands", () => {
    expect(contributes.commands.map((c: { category: string; title: string }) => `${c.category}: ${c.title}`)).toEqual([
      "Reactogenic: Restart Server",
      "Reactogenic: Show Transpiled TSX",
    ]);
  });

  test("the settings", () => {
    const properties = contributes.configuration.properties;
    expect(Object.keys(properties)).toEqual(["reactogenic.server.path", "reactogenic.autoClosingTags", "reactogenic.trace.server"]);
    expect(properties["reactogenic.autoClosingTags"].default).toBe(true);
    expect(properties["reactogenic.server.path"].default).toBeNull();
  });

  test("an untrusted workspace: limited, and the binary's path cannot come from it", () => {
    expect(manifest.capabilities.untrustedWorkspaces).toMatchObject({ supported: "limited", restrictedConfigurations: ["reactogenic.server.path"] });
  });

  test("one CommonJS bundle; no runtime dependencies to install", () => {
    expect(manifest.main).toBe("./dist/extension.js");
    expect(manifest.dependencies).toBeUndefined();
    expect(manifest.type).toBeUndefined();
  });
});

describe("requests the server may not have", () => {
  test("method not found, as the fork answers it", () => {
    expect(isUnsupported(-32601)).toBe(true); // MethodNotFound
    expect(isUnsupported(-32600)).toBe(true); // InvalidRequest: what `reactogenic lsp` answers today
    expect(isUnsupported(-32603)).toBe(false); // InternalError: a real failure
    expect(isUnsupported(undefined)).toBe(false);
  });
});

describe("closing tags at several cursors", () => {
  test("where each typed character ends: one earlier on the line moves the later ones", () => {
    expect(typedEnds([{ line: 4, character: 12 }])).toEqual([{ line: 4, character: 13 }]);
    // Two lines, a cursor at the end of each — in the order VS Code reports them, last first.
    expect(
      typedEnds([
        { line: 5, character: 13 },
        { line: 4, character: 12 },
      ]),
    ).toEqual([
      { line: 5, character: 14 },
      { line: 4, character: 13 },
    ]);
    // Three on one line: `<a` `<b` `<c`, typed `>` after each.
    expect(
      typedEnds([
        { line: 0, character: 8 },
        { line: 0, character: 5 },
        { line: 0, character: 2 },
        { line: 1, character: 2 },
      ]),
    ).toEqual([
      { line: 0, character: 11 },
      { line: 0, character: 7 },
      { line: 0, character: 3 },
      { line: 1, character: 3 },
    ]);
  });
});
