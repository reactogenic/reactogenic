import { expect, test } from "vitest";

test("the package entry points load", async () => {
  await expect(import("./index.ts")).resolves.toBeDefined();
  await expect(import("./vite.ts")).resolves.toBeDefined();
});
