// What shell code can ask the builder (specs/phase02/builder.md, *What shell
// code can ask the builder*): the page being rendered, an id that is readable
// in view-source, the behaviours a component needs, and the classes its
// variants resolve to.
//
// This module is the first of the package to import React at run time
// (`useId`); everything else here takes only types from it. `react` is a
// peer dependency, so nothing changes for an app — but a bundle of
// @reactogenic/core alone now references `react`.
import { useId } from "react";

/**
 * The build-time protocol (specs/phase02/plan.md, M1): while `reactogenic
 * build` renders a page, its engine provides this object as
 * `globalThis.__reactogenic_build`. Its absence means React — a dynamic segment, or
 * Vite.
 */
export interface ShellBuild {
  /** The route being rendered: "/guide/" — the same in every variant of it. */
  readonly pathname: string;
  /** The next id of `prefix` on this page: "d1", "d2". */
  id(prefix: string): string;
  /**
   * Records a behaviour for the page: its module, the root's id, the flags
   * this use site turns on, and the data it hands the behaviour. Throws
   * (`mount-data`) when the data is not a plain object of JSON values, or is
   * given without an id.
   */
  mount(module: string, id: string | undefined, flags: Record<string, boolean> | undefined, data: MountData | undefined): void;
  /**
   * Records the classes a `variants()` call resolved, for the page's report:
   * which variant classes the document has, and which component resolved
   * each. Optional, and nothing is decided by it: a page's CSS is pruned
   * against the page as served, whose elements carry the classes (builder.md,
   * *CSS*).
   */
  classes?(names: readonly string[]): void;
}

/** A value a mount's data may hold: JSON, and nothing else. */
export type MountValue = string | number | boolean | null | undefined | readonly MountValue[] | { readonly [key: string]: MountValue };

/**
 * What a use site hands its behaviour (builder.md, *Behaviours*): a plain
 * object of JSON values — the builder writes it into the page's script as a
 * literal. A key whose value is `undefined` is left out.
 */
export type MountData = { readonly [key: string]: MountValue };

// Read on every call, never cached: the engine sets it per page.
function build(): ShellBuild | undefined {
  return (globalThis as { __reactogenic_build?: ShellBuild }).__reactogenic_build;
}

/** The pathname of the page: the route being built, or `location.pathname` in React. */
export function pathname(): string {
  const shell = build();
  return shell ? shell.pathname : location.pathname;
}

/**
 * An id for an element of the page. At build time it is `prefix` plus a
 * counter per prefix and page, in render order — `d1`, `m1`, `m2`; without a
 * prefix, `r1`, `r2`. In React it is `useId()`, so it follows the rules of
 * hooks in both: call it unconditionally, at the top of a component.
 */
export function useShellId(prefix = "r"): string {
  // Called in both worlds, so the hook order is the same in both; at build
  // time React's static renderer answers and the answer is not used.
  const reactId = useId();
  const shell = build();
  return shell ? shell.id(prefix) : reactId;
}

/**
 * Declares that the page needs a behaviour: `module`'s default export, run
 * on the element with `id` — or once per page when there is no id — with
 * `flags` turned on (builder.md, *Behaviours*). At build time the call is
 * recorded; in React it does nothing in phase 2.
 *
 * `flags` are the page's: a flag any mount turns on is on for every mount of
 * the page — it says whether the code is in the page's script. `data` is
 * this use site's own: what only the behaviour reads, handed to it as the
 * second argument of this mount's call — `module(root, data)`. It is JSON,
 * and a behaviour of the page (no `id`) takes none. What a CSS rule selects
 * stays an attribute.
 */
export function mount(module: string, id?: string, flags?: Record<string, boolean>, data?: MountData): void {
  build()?.mount(module, id, flags, data);
}

/**
 * The variants of a component (components.md, *CSS convention*): per
 * dimension, the class each value resolves to — one class, unique to it.
 * `""` is a value without a class of its own: the default.
 */
export type VariantMap = { readonly [dimension: string]: { readonly [value: string]: string } };

/** A choice among a map's variants: a value of a dimension, or none of it. */
export type VariantChoice<M extends VariantMap> = { readonly [D in keyof M]?: keyof M[D] | undefined };

/**
 * The classes of an element with variants: `base`, then the class of each
 * chosen value, in the order the map names its dimensions. A dimension
 * without a choice (`undefined`) adds nothing, and so does a value whose
 * class is `""`. A dimension or a value the map does not have is a type
 * error.
 *
 * ```ts
 * const buttonVariants = { size: { sm: "rg-button-sm", md: "" }, look: { ghost: "rg-button-ghost" } } as const;
 * variants("rg-button", buttonVariants, { size: "sm", look });   // "rg-button rg-button-sm rg-button-ghost"
 * ```
 *
 * A variant is a class of its own, so a rule for it is kept on exactly the
 * pages with an element that has it (builder.md, *CSS*). At build time every
 * class of the result is also reported to the builder, for the page's
 * report; in React it only returns the string.
 */
export function variants<const M extends VariantMap>(base: string, map: M, choice: VariantChoice<M> = {}): string {
  const names = [base];
  // The map's order, not the choice's: the same element has the same
  // `class` however a use site wrote its props.
  for (const dimension of Object.keys(map)) {
    const value = choice[dimension] as string | undefined;
    if (value !== undefined && Object.hasOwn(map[dimension]!, value)) {
      names.push(map[dimension]![value]!);
    }
  }
  // Class by class: a base of two classes is two classes to the builder.
  const classes = names.flatMap((name) => name.split(/\s+/)).filter((name) => name !== "");
  build()?.classes?.(classes);
  return classes.join(" ");
}
