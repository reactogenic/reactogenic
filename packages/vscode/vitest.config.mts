import { defineConfig } from "vitest/config";

// Unit tests only: the editor suite (test/editor, `pnpm test:editor`) runs
// inside VS Code under mocha.
export default defineConfig({
  test: {
    include: ["test/*.test.{mjs,ts}"],
    globalSetup: ["test/setup.ts"],
    testTimeout: 60_000,
  },
});
