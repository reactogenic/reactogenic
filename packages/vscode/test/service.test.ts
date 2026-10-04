// The wrapped language service (src/plugin/service.ts) on stand-in services:
// what tsserver's own `rename` cannot be made to ask (test/plugin.test.ts
// drives the real one).
import type * as ts from "typescript";
import { describe, expect, test } from "vitest";
import { decorate, type Program } from "../src/plugin/service.ts";

type Locations = Record<string, ts.RenameLocation[]>;

/** A language service that knows rename only: `locations` by the file asked in. */
function service(locations: Locations, asked: string[] = []): ts.LanguageService {
  return {
    getRenameInfo: () => ({ canRename: true, displayName: "x", fullDisplayName: "x", kind: "function", kindModifiers: "", triggerSpan: { start: 0, length: 1 } }),
    findRenameLocations: (fileName: string) => {
      asked.push(fileName);
      return locations[fileName];
    },
  } as unknown as ts.LanguageService;
}

const at = (fileName: string, start = 0): ts.RenameLocation => ({ fileName, textSpan: { start, length: 1 } });

function program(peers: Record<string, ts.LanguageService[]> = {}): Program {
  return {
    request: () => {},
    active: () => false, // this project holds no .rtsx file
    anywhere: () => true, // another one does
    virtual: () => undefined,
    imported: () => undefined,
    listed: () => false,
    specifier: (_, specifier) => specifier,
    display: (fileName) => fileName.slice(1),
    peers: (fileName) => peers[fileName] ?? [],
    languageServer: () => false,
  };
}

describe("rename, with several projects", () => {
  test("a project's locations that reach an .rtsx file fail the request: never some of them", () => {
    const wrapped = decorate(service({ "/a.ts": [at("/a.ts"), at("/b/page.rtsx", 7)] }), program());
    expect(() => wrapped.findRenameLocations("/a.ts", 0, false, false, {})).toThrow("This rename reaches b/page.rtsx");
    expect(() => wrapped.findRenameLocations("/b/page.rtsx", 7, false, false, {})).toThrow("This rename reaches b/page.rtsx");
    // And those that do not are TypeScript's, untouched.
    const plain = decorate(service({ "/a.ts": [at("/a.ts"), at("/c.ts", 3)] }), program());
    expect(plain.findRenameLocations("/a.ts", 0, false, false, {})).toEqual([at("/a.ts"), at("/c.ts", 3)]);
  });

  test("getRenameInfo follows a shared file into the other projects that hold it, each once", () => {
    // This project: a.ts and shared.ts. A second one holds shared.ts and c.ts; a third, c.ts and page.rtsx.
    const asked: string[] = [];
    const third = service({ "/c.ts": [at("/c.ts"), at("/page.rtsx", 4)] }, asked);
    const second = service({ "/shared.ts": [at("/shared.ts"), at("/c.ts", 2)] }, asked);
    const own = service({ "/a.ts": [at("/a.ts"), at("/shared.ts", 9)] }, asked);
    const peers = { "/shared.ts": [second], "/c.ts": [second, third] };
    expect(decorate(own, program(peers)).getRenameInfo("/a.ts", 0, {})).toEqual({
      canRename: false,
      localizedErrorMessage: "This rename reaches page.rtsx: start it from that file, where Reactogenic's language server writes it back.",
    });
    expect(asked).toEqual(["/a.ts", "/shared.ts", "/c.ts"]);

    // No project reaches one: allowed, and the second project was asked once though two files lead into it.
    asked.length = 0;
    const harmless = service({ "/c.ts": [at("/c.ts")] }, asked);
    expect(decorate(own, program({ "/shared.ts": [second], "/c.ts": [second, harmless] })).getRenameInfo("/a.ts", 0, {}).canRename).toBe(true);
    expect(asked).toEqual(["/a.ts", "/shared.ts", "/c.ts"]);
  });
});
