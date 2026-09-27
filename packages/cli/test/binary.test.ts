import { expect, test } from "vitest";
import { binaryPath, platformPackage } from "../binary.js";

test("the platform package follows Node's names", () => {
  expect(platformPackage("darwin", "arm64")).toBe("@reactogenic/cli-darwin-arm64");
  expect(platformPackage("win32", "x64")).toBe("@reactogenic/cli-win32-x64");
});

test("REACTOGENIC_BINARY overrides", () => {
  expect(binaryPath({ env: { REACTOGENIC_BINARY: "/opt/reactogenic" } })).toBe("/opt/reactogenic");
});

test("the binary comes from the platform package", () => {
  const seen: string[] = [];
  const resolve = (id: string) => (seen.push(id), `/node_modules/${id}`);
  expect(binaryPath({ env: {}, platform: "linux", arch: "x64", resolve })).toBe("/node_modules/@reactogenic/cli-linux-x64/bin/reactogenic");
  expect(binaryPath({ env: {}, platform: "win32", arch: "arm64", resolve })).toBe("/node_modules/@reactogenic/cli-win32-arm64/bin/reactogenic.exe");
});

test("a missing platform package says what to do", () => {
  const resolve = () => {
    throw new Error("not found");
  };
  expect(() => binaryPath({ env: {}, platform: "linux", arch: "riscv64", resolve })).toThrow(/cli-linux-riscv64 is not installed[\s\S]*REACTOGENIC_BINARY/);
});
