import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, test } from "vitest";
import { Each, isAssigned, Match, NOT_ASSIGNED, noMatch, renderSlot, slotProps, Switch } from "./index.ts";

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
