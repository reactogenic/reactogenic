// The plugin's position mapping (src/plugin/spans.ts), on hand-written maps
// and on what the binary returns for real `.rtsx` text.
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { describe, expect, test } from "vitest";
import { ATOM, Feature, lineAndCharacter, lineStarts, SpanMap, type Tuple, VERBATIM } from "../src/plugin/spans.ts";

const ALL = Object.values(Feature).reduce((a, b) => a | b, 0) & ~Feature.Formatting;

// source:  `<Button size />`           virtual: `<Button size={size} />`
//           0       8   12              0       8   12 14  18
// `size` is copied twice — the prop's name and the binding — as a shorthand
// is (ide.md, *Several copies of one token*); `={` and `}` are generated.
const NAME = ALL & ~(Feature.Rename | Feature.SemanticTokens | Feature.InlayHints);
const VALUE = ALL & ~(Feature.Completion | Feature.TypeDefinition);
const shorthand = new SpanMap([
  [0, 8, 0, 8, VERBATIM, ALL], // `<Button `
  [8, 4, 8, 4, VERBATIM, NAME], // `size`, the name
  [12, 2, 8, 4, ATOM, 0], // `={`
  [14, 4, 8, 4, VERBATIM, VALUE], // `size`, the value
  [18, 1, 8, 4, ATOM, 0], // `}`
  [19, 3, 12, 3, VERBATIM, ALL], // ` />`
]);

describe("virtual to source", () => {
  test("a span in copied text maps exactly", () => {
    expect(shorthand.toSource(1, 6, Feature.References)).toEqual({ start: 1, length: 6 }); // `Button`
    expect(shorthand.toSource(8, 4, Feature.References)).toEqual({ start: 8, length: 4 });
    expect(shorthand.toSource(14, 4, Feature.References)).toEqual({ start: 8, length: 4 }); // both copies: one source token
    expect(shorthand.toSource(20, 2, Feature.Definition)).toEqual({ start: 13, length: 2 }); // `/>` after the generated text
  });

  test("a span in generated text, or reaching into it, has no source span", () => {
    expect(shorthand.toSource(12, 2, Feature.References)).toBeUndefined();
    expect(shorthand.toSource(12, 1, Feature.References)).toBeUndefined();
    expect(shorthand.toSource(8, 6, Feature.References)).toBeUndefined(); // `size={`
    expect(shorthand.toSource(14, 5, Feature.References)).toBeUndefined(); // `size}`
    expect(shorthand.toSource(30, 2, Feature.References)).toBeUndefined(); // outside the text
  });

  test("a copy answers only its features", () => {
    expect(shorthand.toSource(8, 4, Feature.Rename)).toBeUndefined(); // the name copy is not renamed
    expect(shorthand.toSource(14, 4, Feature.Rename)).toEqual({ start: 8, length: 4 });
    expect(shorthand.toSource(14, 4, Feature.TypeDefinition)).toBeUndefined();
    expect(shorthand.toSource(8, 4, Feature.References | Feature.Definition)).toEqual({ start: 8, length: 4 });
  });

  test("a span through several copies maps when they are one run of the source", () => {
    expect(shorthand.toSource(4, 8, Feature.References)).toEqual({ start: 4, length: 8 }); // `ton size`: two spans, contiguous on both sides
    // Two copies that are adjacent in the virtual text and not in the source.
    const moved = new SpanMap([
      [0, 4, 10, 4, VERBATIM, ALL],
      [4, 4, 0, 4, VERBATIM, ALL],
    ]);
    expect(moved.toSource(2, 4, Feature.References)).toBeUndefined();
    expect(moved.toSource(4, 4, Feature.References)).toEqual({ start: 0, length: 4 });
  });

  test("an empty span belongs to the copy it starts in, else to the one that ends there", () => {
    expect(shorthand.toSource(8, 0, Feature.References)).toEqual({ start: 8, length: 0 });
    expect(shorthand.toSource(12, 0, Feature.References)).toEqual({ start: 12, length: 0 }); // the end of the name, though `={` starts there
    expect(shorthand.toSource(13, 0, Feature.References)).toBeUndefined();
    expect(shorthand.toSource(22, 0, Feature.References)).toEqual({ start: 15, length: 0 }); // the end of the text
  });

  test("the ends of a declaration map through what was lowered between them", () => {
    expect(shorthand.toSourceEnds(0, 22)).toEqual({ start: 0, length: 15 }); // the whole element
    expect(shorthand.toSourceEnds(1, 11)).toEqual({ start: 1, length: 11 });
    expect(shorthand.toSourceEnds(0, 13)).toBeUndefined(); // ends in `={`
    expect(shorthand.toSourceEnds(13, 9)).toBeUndefined(); // starts in it
    const reordered = new SpanMap([
      [0, 4, 10, 4, VERBATIM, ALL],
      [4, 4, 0, 4, VERBATIM, ALL],
    ]);
    expect(reordered.toSourceEnds(0, 8)).toBeUndefined(); // the source runs the other way
  });

  test("a diagnostic's range widens to the construct that generated its text", () => {
    expect(shorthand.toSourceWide(1, 6)).toEqual({ start: 1, length: 6 });
    expect(shorthand.toSourceWide(12, 2)).toEqual({ start: 8, length: 4 }); // `={`: on `size`
    expect(shorthand.toSourceWide(8, 11)).toEqual({ start: 8, length: 4 }); // `size={size}`
    expect(shorthand.toSourceWide(1, 21)).toEqual({ start: 1, length: 14 });
    expect(shorthand.toSourceWide(22, 0)).toEqual({ start: 15, length: 0 });
    expect(new SpanMap([]).toSourceWide(0, 0)).toBeUndefined();
  });
});

describe("source to virtual", () => {
  test("the first copy that answers the feature", () => {
    expect(shorthand.toVirtual(1, Feature.Definition)).toBe(1);
    expect(shorthand.toVirtual(9, Feature.References)).toBe(9); // in the name
    expect(shorthand.toVirtual(9, Feature.Rename)).toBe(15); // only the value is renamed
    expect(shorthand.toVirtual(9, Feature.TypeDefinition)).toBe(9);
    expect(shorthand.toVirtual(13, Feature.Hover)).toBe(20);
  });

  test("the end of a copy, when no copy starts there", () => {
    expect(shorthand.toVirtual(15, Feature.Hover)).toBe(22);
    expect(shorthand.toVirtual(12, Feature.Hover)).toBe(19); // ` />` starts at 12: inside wins over the end of `size`
    expect(shorthand.toVirtual(99, Feature.Hover)).toBeUndefined();
  });

  test("a round trip through every copied offset", () => {
    for (const [virtualStart, length, , , kind, features] of shorthand.spans) {
      if (kind !== VERBATIM || !(features & Feature.References)) {
        continue;
      }
      for (let offset = virtualStart; offset < virtualStart + length; offset++) {
        const source = shorthand.toSource(offset, 1, Feature.References);
        expect(source, `virtual ${offset}`).toBeDefined();
        // Back to a copy of the same source character: the first one.
        const back = shorthand.toVirtual(source!.start, Feature.References);
        expect(shorthand.toSource(back!, 1, Feature.References)).toEqual(source);
      }
    }
  });
});

test("identity: a text that is its own virtual text", () => {
  const map = SpanMap.identity(10);
  expect(map.toSource(3, 4, Feature.References)).toEqual({ start: 3, length: 4 });
  expect(map.toVirtual(7, Feature.Rename)).toBe(7);
  expect(map.toSource(3, 4, Feature.Formatting)).toBeUndefined();
  expect(SpanMap.identity(0).spans).toEqual([]);
});

test("lines, as TypeScript counts them", () => {
  const starts = lineStarts("a\nbc\r\nd\re f");
  expect(starts).toEqual([0, 2, 6, 8, 10]);
  expect(lineAndCharacter(starts, 0)).toEqual({ line: 0, character: 0 });
  expect(lineAndCharacter(starts, 3)).toEqual({ line: 1, character: 1 });
  expect(lineAndCharacter(starts, 6)).toEqual({ line: 2, character: 0 });
  expect(lineAndCharacter(starts, 11)).toEqual({ line: 4, character: 1 });
});

test("the feature bits are those of go/internal/emit/features.go", () => {
  const go = fs.readFileSync(path.resolve(import.meta.dirname, "../../../go/internal/emit/features.go"), "utf8");
  const names = [...go.slice(go.indexOf("const ("), go.indexOf("AllFeatures")).matchAll(/^\tFeature(\w+)/gm)].map((m) => m[1]);
  expect(names.length).toBeGreaterThan(10);
  expect(Object.keys(Feature)).toEqual(names);
  expect(Object.values(Feature)).toEqual(names.map((_, i) => 1 << i));
});

describe("what the binary returns", () => {
  const binary = process.env.REACTOGENIC_BINARY!;
  // Text outside the basic plane before the code: offsets are UTF-16, as a JavaScript string is indexed.
  const source = `// \u{1F600} é
import { Button } from "./button";
export function Page({ size }: { size: number }) {
  return <Button size><$Icon className="i" { size }>{size}</$Icon></Button>;
}
`;
  const result = spawnSync(binary, ["serve"], { input: `${JSON.stringify({ id: 1, method: "virtual", params: { files: [{ file: "/app/page.rtsx", code: source }] } })}\n`, encoding: "utf8" });
  const file: { text: string; spans: Tuple[] } = JSON.parse(result.stdout).result.files[0];
  const map = new SpanMap(file.spans);

  test("covers the virtual text, and every copy is the source's text", () => {
    expect(file.text).toContain('<Button size={size} $Icon={{ className: "i", children: ({ size }) => size }} />');
    let at = 0;
    for (const [start, length, sourceStart, sourceLength, kind] of file.spans) {
      expect(start).toBe(at);
      at += length;
      if (kind === VERBATIM) {
        expect(file.text.slice(start, start + length)).toBe(source.slice(sourceStart, sourceStart + sourceLength));
      }
    }
    expect(at).toBe(file.text.length);
  });

  test("every identifier of the virtual text that maps, maps to itself", () => {
    let mapped = 0;
    for (const match of file.text.matchAll(/[$\w]+/g)) {
      const range = map.toSource(match.index, match[0].length, Feature.Hover);
      if (range) {
        mapped++;
        expect(source.slice(range.start, range.start + range.length)).toBe(match[0]);
        expect(file.text.slice(map.toVirtual(range.start, Feature.Hover)!).startsWith(match[0])).toBe(true);
      }
    }
    expect(mapped).toBeGreaterThan(15);
    // Generated names do not: `children` is the transform's.
    const children = file.text.indexOf("children");
    expect(map.toSource(children, "children".length, Feature.Hover)).toBeUndefined();
    // The slot's tag name is copied into the prop's name.
    const icon = file.text.indexOf("$Icon=");
    expect(map.toSource(icon, 5, Feature.References)).toEqual({ start: source.indexOf("$Icon"), length: 5 });
  });
});
