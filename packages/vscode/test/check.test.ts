// The task *reactogenic: check* and its problem matcher `$reactogenic`,
// against what the binary really prints for test/fixture.
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { describe, expect, test } from "vitest";
import { checkArgs, isUnsupported } from "../src/protocol.ts";

const root = path.resolve(import.meta.dirname, "..");
const fixture = path.join(root, "test/fixture");
const manifest = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const { contributes } = manifest;
const pattern = contributes.problemPatterns[0];
const regexp = new RegExp(pattern.regexp);

function check(...args: string[]): { lines: string[]; status: number | null } {
  const result = spawnSync(process.env.REACTOGENIC_BINARY!, args, { cwd: fixture, encoding: "utf8" });
  expect(result.error).toBeUndefined();
  return { lines: result.stdout.split("\n").filter(Boolean), status: result.status };
}

/** A line as VS Code's problem collector reads it. */
function problem(line: string) {
  const m = regexp.exec(line);
  return m && {
    file: m[pattern.file],
    line: Number(m[pattern.line]),
    column: Number(m[pattern.column]),
    severity: m[pattern.severity],
    code: m[pattern.code],
    message: m[pattern.message],
  };
}

describe("$reactogenic", () => {
  test("reads every line of `reactogenic check --pretty=false`", () => {
    const { lines, status } = check(...checkArgs({}));
    expect(status).toBe(1);
    expect(lines.map(problem)).toEqual([
      { file: "src/broken.rtsx", line: 4, column: 9, severity: "error", code: "TS2322", message: "Type 'string' is not assignable to type 'number'." },
      { file: "src/broken.rtsx", line: 7, column: 16, severity: "warning", code: "segment-children", message: "Contents will be overwritten by the segment `intro`" },
      { file: "src/broken.rtsx", line: 9, column: 10, severity: "error", code: "undeclared-slot", message: "`$Badge` is not declared in `Button`" },
    ]);
  });

  test("the same with -p, from the task's `project`", () => {
    expect(checkArgs({ project: "tsconfig.json" })).toEqual(["check", "--pretty=false", "-p", "tsconfig.json"]);
    expect(check(...checkArgs({ project: "tsconfig.json" })).lines.map(problem)).toHaveLength(3);
  });

  test("reads nothing of the pretty output, which the task does not ask for", () => {
    const { lines } = check("check");
    expect(lines.length).toBeGreaterThan(3);
    expect(lines.map(problem).filter(Boolean)).toEqual([]);
  });

  test.each([
    ["C:\\ws\\src\\page.rtsx(1,2): error TS1005: ';' expected.", { file: "C:\\ws\\src\\page.rtsx", line: 1, column: 2, code: "TS1005" }],
    ["/abs/src/a (copy).rtsx(10,20): warning segment-children: x", { file: "/abs/src/a (copy).rtsx", line: 10, column: 20, severity: "warning" }],
    // The first position is the file's: a message may hold something that looks like one.
    ["src/a.rtsx(1,2): error TS1: see b.ts(3,4): error TS2: nested", { file: "src/a.rtsx", line: 1, code: "TS1", message: "see b.ts(3,4): error TS2: nested" }],
  ])("%s", (line, want) => {
    expect(problem(line)).toMatchObject(want);
  });

  test.each([
    "  src/button.rtsx:8:3 - `$Icon` is declared here", // related information
    "error TS18003: No inputs were found", // no file: nothing to attach a problem to
    "src/broken.rtsx:4:9 - error TS2322: pretty",
    "Found 2 error(s).",
    "",
  ])("not a problem: %j", (line) => {
    expect(problem(line)).toBeNull();
  });

  test("is applied to closed documents only, relative to the task's folder", () => {
    expect(contributes.problemMatchers).toEqual([
      {
        name: "reactogenic",
        label: "reactogenic check",
        owner: "reactogenic", // the server's diagnostic collection: src/server.ts
        source: "reactogenic",
        applyTo: "closedDocuments",
        fileLocation: ["autoDetect", "${cwd}"],
        pattern: "$reactogenic",
      },
    ]);
    expect(pattern.name).toBe("reactogenic");
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
