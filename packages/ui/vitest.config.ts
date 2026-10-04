import reactogenic from "@reactogenic/vite";
import { defineConfig } from "vitest/config";

// The components are .rtsx: the phase 1 plugin transpiles them for the tests.
// The browser suite (test/browser, `pnpm test:browser`) is not part of this
// run: CI has no browsers.
export default defineConfig({
  plugins: [reactogenic()],
  test: { globalSetup: ["test/setup.ts"], include: ["test/*.test.ts"], testTimeout: 60_000 },
});
