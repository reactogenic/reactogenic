// A plain TSX file that imports an .rtsx module: VS Code's own TypeScript
// reads it, through the TS server plugin (test/editor/typescript.ts).
import { Page } from "./page";

export const app = <Page />;
