import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, test } from "vitest";
import { Each, isAssigned, KEYED, Match, NOT_ASSIGNED, noMatch, renderSlot, SLOT_KEY, slotEntry, slotEntryName, slotKey, slotKeys, slotProps, Switch } from "./index.ts";

describe("renderSlot", () => {
  test("calls a function body with the args", () => {
    expect(renderSlot({ children: ({ size }: { size: string }) => `icon ${size}` }, { size: "lg" })).toBe("icon lg");
  });
  test("returns a plain body as it is", () => {
    expect(renderSlot({ children: "plain" }, {})).toBe("plain");
  });
  test("renders the fallback when there is no body", () => {
    expect(renderSlot({ title: "t" }, {}, "fallback")).toBe("fallback");
    expect(renderSlot({}, {})).toBeUndefined();
  });
});

describe("NOT_ASSIGNED", () => {
  test("isAssigned treats it as no slot", () => {
    expect(isAssigned(NOT_ASSIGNED)).toBe(false);
    expect(isAssigned(undefined)).toBe(false);
    expect(isAssigned({})).toBe(true);
  });
  test("slotProps skips the entries that were not assigned", () => {
    expect(slotProps({ variant: "solid", $IconStart: NOT_ASSIGNED, children: "x" })).toEqual({ variant: "solid", children: "x" });
  });
});

describe("keyed slots", () => {
  test("slotEntry picks a keyed slot's entry, and passes a singular slot through", () => {
    const columns = { [KEYED]: true as const, email: { width: 2 } };
    expect(slotEntry(columns, "email")).toEqual({ width: 2 });
    expect(slotEntry(columns, "name")).toBeUndefined();
    const option = { value: "a" };
    expect(slotEntry(option, "x")).toBe(option);
  });

  // syntax.md, *Keyed slots*: an entry's property name is never integer-like,
  // so the object's own order is the written order.
  test("slotEntryName: an integer-like key and a `#` key get one `#`; any other key is itself", () => {
    expect(slotEntryName("email")).toBe("email");
    expect(slotEntryName("10")).toBe("#10");
    expect(slotEntryName(10)).toBe("#10");
    expect(slotEntryName("0")).toBe("#0");
    expect(slotEntryName(0)).toBe("#0");
    expect(slotEntryName("4294967295")).toBe("#4294967295"); // past an array index: still digits
    expect(slotEntryName("#10")).toBe("##10");
    expect(slotEntryName("#")).toBe("##");
    expect(slotEntryName("")).toBe("");
    // Not integer-like to JavaScript: enumerated as written already.
    for (const key of ["01", "1.0", "-1", "+1", " 1", "1e3", "0.5", "a1", "$1"]) {
      expect(slotEntryName(key)).toBe(key);
    }
    expect(slotEntryName(0.5)).toBe("0.5");
    expect(slotEntryName(-1)).toBe("-1");
    expect(slotEntryName(1e21)).toBe("1e+21");
    // Injective: no two keys share a name.
    const keys = ["1", "#1", "##1", "a", "#a", "", "#", "01", "#01"];
    expect(new Set(keys.map(slotEntryName)).size).toBe(keys.length);
  });

  // What the transpiler emits for `<$X key="10" />`, `<$X key={n} />`.
  const entry = <V extends object>(key: string | number, value: V) => ({ [slotEntryName(key)]: value });

  test("slotKeys: the written order survives — 10, 9, 2, b, 0.5", () => {
    const menu = { [KEYED]: true as const, ...entry("10", { n: 1 }), ...entry(9, { n: 2 }), ...entry("2", { n: 3 }), b: { n: 4 }, ...entry(0.5, { n: 5 }) };
    expect(slotKeys(menu)).toEqual(["10", "9", "2", "b", "0.5"]);
    expect(slotKeys(menu).map((key) => slotEntry(menu, key))).toEqual([{ n: 1 }, { n: 2 }, { n: 3 }, { n: 4 }, { n: 5 }]);
    expect(slotEntry(menu, 10)).toEqual({ n: 1 }); // a number and its string are one key
    expect(slotEntry(menu, "9")).toEqual({ n: 2 });
    expect(slotEntry(menu, "3")).toBeUndefined();
  });

  test("slotKeys: keys that start with `#` come back as written", () => {
    const tabs = { [KEYED]: true as const, ...entry("#x", { n: 1 }), ...entry("#1", { n: 2 }), ...entry("1", { n: 3 }), ...entry("x", { n: 4 }) };
    expect(slotKeys(tabs)).toEqual(["#x", "#1", "1", "x"]);
    expect(slotEntry(tabs, "#x")).toEqual({ n: 1 });
    expect(slotEntry(tabs, "#1")).toEqual({ n: 2 });
    expect(slotEntry(tabs, "1")).toEqual({ n: 3 });
    expect(slotEntry(tabs, 1)).toEqual({ n: 3 });
    expect(slotEntry(tabs, "x")).toEqual({ n: 4 });
  });

  test("a repeated key is last-wins, at its first position", () => {
    const menu = { [KEYED]: true as const, ...entry("10", { n: 1 }), ...entry("9", { n: 2 }), ...entry(10, { n: 3 }) };
    expect(slotKeys(menu)).toEqual(["10", "9"]);
    expect(slotEntry(menu, "10")).toEqual({ n: 3 });
  });

  test("a conditional entry is at its place, or absent", () => {
    const menu = (on: boolean) => ({ [KEYED]: true as const, ...entry("10", {}), ...(on ? entry("9", {}) : {}), ...entry("2", {}) });
    expect(slotKeys(menu(true))).toEqual(["10", "9", "2"]);
    expect(slotKeys(menu(false))).toEqual(["10", "2"]);
  });

  test("an explicit attribute, spread first, keeps its entries first", () => {
    const given: Record<string, { n: number }> = { ...entry("7", { n: 1 }), ...entry("3", { n: 2 }) };
    const menu = { [KEYED]: true as const, ...given, ...entry("5", { n: 3 }), ...entry("3", { n: 4 }) };
    expect(slotKeys(menu)).toEqual(["7", "3", "5"]);
    expect(slotEntry(menu, "3")).toEqual({ n: 4 });
  });

  test("an object written by hand with raw integer names is still found; its order is JavaScript's", () => {
    const raw = { [KEYED]: true as const, b: { n: 1 }, 10: { n: 2 }, 9: { n: 3 } };
    expect(slotEntry(raw, 10)).toEqual({ n: 2 });
    expect(slotEntry(raw, "9")).toEqual({ n: 3 });
    expect(slotEntry(raw, "b")).toEqual({ n: 1 });
    expect(slotKeys(raw)).toEqual(["9", "10", "b"]);
    // The encoded name wins where both are there.
    expect(slotEntry({ [KEYED]: true as const, 10: { n: 1 }, "#10": { n: 2 } }, 10)).toEqual({ n: 2 });
  });

  test("slotKeys of a slot that is not keyed, or not there, is empty", () => {
    expect(slotKeys(undefined)).toEqual([]);
    expect(slotKeys(null)).toEqual([]);
    expect(slotKeys(NOT_ASSIGNED)).toEqual([]);
    expect(slotKeys({ value: "a" })).toEqual([]);
    expect(slotKeys({ [KEYED]: true as const })).toEqual([]);
  });

  test("an inherited name is no entry", () => {
    const menu = { [KEYED]: true as const, a: {} };
    expect(slotEntry(menu, "toString")).toBeUndefined();
    expect(slotEntry(menu, "constructor")).toBeUndefined();
  });
});

describe("key functions", () => {
  test("slotKey applies the slot's key function, or falls back to the attachment's key", () => {
    const option = { [SLOT_KEY]: ({ value }: { value: string }) => value };
    expect(slotKey(option, { value: "a" }, "i0")).toBe("a");
    expect(slotKey({}, { value: "a" }, "i0")).toBe("i0");
    expect(slotKey(NOT_ASSIGNED, { value: "a" })).toBeUndefined();
    expect(Object.getOwnPropertySymbols(slotProps(option))).toEqual([]);
  });
});

describe("Each", () => {
  test("renders the body per item, with index, and no wrapper", () => {
    const html = renderToStaticMarkup(
      <ul>
        <Each items={["a", "b"]}>{({ item, index }) => <li key={item}>{item}{index}</li>}</Each>
      </ul>,
    );
    expect(html).toBe("<ul><li>a0</li><li>b1</li></ul>");
  });
  test("renders nothing for no items", () => {
    expect(renderToStaticMarkup(<Each items={[]}>{() => <li />}</Each>)).toBe("");
  });
});

describe("compile-time elements", () => {
  test("Switch and Match throw when rendered", () => {
    expect(() => renderToStaticMarkup(<Match on={1}>x</Match>)).toThrow(/compiled away/);
    expect(() => renderToStaticMarkup(<Switch on={1} $Case={[]} />)).toThrow(/compiled away/);
  });
  test("noMatch throws with the value", () => {
    expect(() => noMatch("oops" as never)).toThrow('No $Case matched "oops"');
  });
});
