// The content-mapper manifest (specs/phase01/ide.md, *Stock TypeScript 7.1*):
// TypeScript 7.1 reads `typescript.contentMapper` from this package's
// package.json and runs `exec` in the package directory, with the protocol on
// the child's stdin and stdout.
import { spawnSync } from "node:child_process";
import { chmodSync, existsSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "vitest";

const packageDir = join(dirname(fileURLToPath(import.meta.url)), "..");
const manifest = JSON.parse(readFileSync(join(packageDir, "package.json"), "utf8"));

test("the package declares the content mapper", () => {
  const mapper = manifest.typescript.contentMapper;
  expect(mapper.exec).toEqual(["node", "./bin/reactogenic.js", "content-mapper"]);
  // The launcher is published, and is the command's own.
  expect(existsSync(join(packageDir, mapper.exec[1]))).toBe(true);
  expect(manifest.files).toContain("bin");
  expect(manifest.bin.reactogenic).toBe("bin/reactogenic.js");
  // What the transform reads of the tsconfig, so a change invalidates
  // TypeScript's caches; no dynamic configuration.
  expect(mapper.compilerOptions).toEqual(["paths"]);
  expect(mapper.dynamicConfig).toBeUndefined();
});

test.skipIf(process.platform === "win32")("the launcher passes arguments, stdin, stdout and the exit status through", () => {
  // A stand-in for the binary: arguments to stderr, stdin to stdout.
  const binary = join(mkdtempSync(join(tmpdir(), "reactogenic-cli-")), "reactogenic");
  writeFileSync(
    binary,
    `#!/usr/bin/env node
process.stderr.write(JSON.stringify(process.argv.slice(2)));
process.stdin.pipe(process.stdout);
process.stdin.on("end", () => process.exit(7));
`,
  );
  chmodSync(binary, 0o755);
  const frame = 'Content-Length: 2\r\n\r\n{}';
  const result = spawnSync(process.execPath, [join(packageDir, manifest.typescript.contentMapper.exec[1]), ...manifest.typescript.contentMapper.exec.slice(2)], {
    cwd: packageDir,
    env: { ...process.env, REACTOGENIC_BINARY: binary },
    input: frame,
    encoding: "utf8",
  });
  expect(result.stderr).toBe('["content-mapper"]');
  expect(result.stdout).toBe(frame); // nothing of the launcher's own
  expect(result.status).toBe(7);
});
