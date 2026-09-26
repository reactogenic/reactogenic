import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, test } from "vitest";
import { Each, Match, noMatch, renderSlot, Switch } from "./index.ts";

describe("renderSlot", () => {
  test("calls a function body with the args", () => {
    expect(renderSlot(({ size }: { size: string }) => `icon ${size}`, { size: "lg" })).toBe("icon lg");
  });
  test("returns a plain body as it is", () => {
    expect(renderSlot<{ size: string }>("plain", { size: "lg" })).toBe("plain");
    expect(renderSlot<{ size: string }>(undefined, { size: "lg" })).toBeUndefined();
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
