// Builds the Go binary once for the suite: the Vite plugin transpiles the
// .rtsx components with it, and check.test.ts runs it.
import { binary } from "./binary.mjs";

export default function setup() {
  binary();
}
