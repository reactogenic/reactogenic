// RGP2-012: both sides of the build-time protocol (specs/phase02/plan.md, M1).
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, test, vi } from "vitest";
import { mount, pathname, useShellId, variants, type ShellBuild } from "./index.ts";

const host = globalThis as { __reactogenic_build?: ShellBuild };

// The engine's side, as plan.md describes it: one object per page.
function standIn(path: string) {
  const counters = new Map<string, number>();
  const mounts: unknown[][] = [];
  resolved.length = 0;
  host.__reactogenic_build = {
    pathname: path,
    id(prefix) {
      const n = (counters.get(prefix) ?? 0) + 1;
      counters.set(prefix, n);
      return prefix + n;
    },
    mount(...call) {
      mounts.push(call);
    },
    classes(names) {
      resolved.push([...names]);
    },
  };
  return mounts;
}

// What `variants()` reported to the stand-in, call by call.
const resolved: string[][] = [];

const buttonVariants = { size: { sm: "rg-button-sm", md: "", lg: "rg-button-lg" }, look: { ghost: "rg-button-ghost" } } as const;

function Ids({ prefix }: { prefix?: string }) {
  return <i id={useShellId(prefix)} />;
}

afterEach(() => {
  delete host.__reactogenic_build;
  vi.unstubAllGlobals();
});

describe("at build time", () => {
  test("pathname is the route being rendered", () => {
    standIn("/guide/");
    expect(pathname()).toBe("/guide/");
    standIn("/");
    expect(pathname()).toBe("/");
  });

  test("useShellId counts per prefix, in render order; no prefix is r", () => {
    standIn("/");
    const html = renderToStaticMarkup(
      <>
        <Ids prefix="d" />
        <Ids prefix="m" />
        <Ids prefix="m" />
        <Ids />
        <Ids prefix="d" />
        <Ids />
      </>,
    );
    expect(html).toBe('<i id="d1"></i><i id="m1"></i><i id="m2"></i><i id="r1"></i><i id="d2"></i><i id="r2"></i>');
  });

  test("ids start again on the next page", () => {
    standIn("/");
    expect(renderToStaticMarkup(<Ids prefix="d" />)).toBe('<i id="d1"></i>');
    standIn("/guide/");
    expect(renderToStaticMarkup(<Ids prefix="d" />)).toBe('<i id="d1"></i>');
  });

  test("mount is recorded as called: module, id, flags, data", () => {
    const mounts = standIn("/");
    mount("@reactogenic/ui/behaviors/overlays");
    mount("@reactogenic/ui/behaviors/menu-keys", "m1", { RG_MENU_TYPEAHEAD: true }, { typeahead: true });
    mount("@reactogenic/ui/behaviors/menu-keys", "m2");
    mount("@reactogenic/ui/behaviors/menu-keys", "m3", undefined, { items: [1, "two", null], wrap: undefined });
    expect(mounts).toEqual([
      ["@reactogenic/ui/behaviors/overlays", undefined, undefined, undefined],
      ["@reactogenic/ui/behaviors/menu-keys", "m1", { RG_MENU_TYPEAHEAD: true }, { typeahead: true }],
      ["@reactogenic/ui/behaviors/menu-keys", "m2", undefined, undefined],
      ["@reactogenic/ui/behaviors/menu-keys", "m3", undefined, { items: [1, "two", null], wrap: undefined }],
    ]);
  });

  test("what the builder throws for a mount's data is the mount's", () => {
    standIn("/");
    host.__reactogenic_build!.mount = () => {
      throw new Error("mount-data");
    };
    expect(() => mount("@reactogenic/ui/behaviors/overlays", undefined, undefined, { typeahead: true })).toThrow("mount-data");
  });

  test("variants: the base, then the class of each chosen value, in the map's order", () => {
    standIn("/");
    expect(variants("rg-button", buttonVariants, { look: "ghost", size: "sm" })).toBe("rg-button rg-button-sm rg-button-ghost");
    expect(variants("rg-button", buttonVariants, { size: "lg" })).toBe("rg-button rg-button-lg");
    // No choice, `undefined`, and a value without a class of its own add nothing.
    expect(variants("rg-button", buttonVariants)).toBe("rg-button");
    expect(variants("rg-button", buttonVariants, { size: undefined, look: undefined })).toBe("rg-button");
    expect(variants("rg-button", buttonVariants, { size: "md" })).toBe("rg-button");
    expect(variants("", buttonVariants, { look: "ghost" })).toBe("rg-button-ghost");
    // What the types refuse adds nothing either: no class is made up.
    expect(variants("rg-button", buttonVariants, { size: "toString" } as never)).toBe("rg-button");
    expect(variants("rg-button", buttonVariants, { colour: "red" } as never)).toBe("rg-button");
  });

  test("variants reports every class of its result to the builder, call by call", () => {
    standIn("/");
    variants("rg-button", buttonVariants, { look: "ghost", size: "sm" });
    variants("rg-button", buttonVariants, { size: "md" });
    variants("rg-menu  rg-surface", { align: { end: "rg-menu-end rg-menu-flip" } }, { align: "end" });
    variants("", buttonVariants);
    expect(resolved).toEqual([["rg-button", "rg-button-sm", "rg-button-ghost"], ["rg-button"], ["rg-menu", "rg-surface", "rg-menu-end", "rg-menu-flip"], []]);
  });

  test("variants: a builder without `classes` is asked nothing", () => {
    standIn("/");
    delete host.__reactogenic_build!.classes;
    expect(variants("rg-button", buttonVariants, { look: "ghost" })).toBe("rg-button rg-button-ghost");
  });
});

describe("in React", () => {
  test("variants only returns the string", () => {
    resolved.length = 0;
    expect(variants("rg-button", buttonVariants, { size: "sm", look: "ghost" })).toBe("rg-button rg-button-sm rg-button-ghost");
    expect(resolved).toEqual([]);
  });

  test("pathname is location.pathname", () => {
    vi.stubGlobal("location", { pathname: "/from/location/" });
    expect(pathname()).toBe("/from/location/");
  });

  test("useShellId is React's useId, whatever the prefix", () => {
    const html = renderToStaticMarkup(
      <>
        <Ids prefix="d" />
        <Ids />
      </>,
    );
    const ids = [...html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
    expect(ids).toHaveLength(2);
    expect(ids[0]).not.toBe(ids[1]);
    // React's shape (`_R_0_`), not the builder's.
    expect(ids.every((id) => /^_R_\w*_$/.test(id!))).toBe(true);
  });

  test("mount does nothing", () => {
    expect(mount("@reactogenic/ui/behaviors/overlays", "x", { RG_FLAG: true }, { typeahead: true })).toBeUndefined();
  });
});
