import { expect, test } from "vitest";

test("the package entry point loads", async () => {
  await expect(import("./index.ts")).resolves.toBeDefined();
});
