import type { ReactNode } from "react";

/** A slot body that needs the container's values: `(params) => body`. */
export type SlotFn<Params> = (params: Params) => ReactNode;

/** A slot body that may take the container's values, or not. */
export type OptionalSlotFn<Params> = ReactNode | SlotFn<Params>;

/**
 * A slot: the props of the element the container renders for it (its
 * options), and its body. Without `Children`, the body is the props' own
 * `children` — `Slot<{ children?: ReactNode }>` has an optional body. With
 * it, `Children` replaces them and the body is required:
 * `Slot<ComponentProps<"div">, SlotFn<{ size: Size }>>`.
 * Rendered with `<El slot={$X} … />` in .rtsx.
 */
export type Slot<Props, Children = never> = [Children] extends [never]
  ? Props
  : Omit<Props, "children"> & { children: Children };

/** No args: what a body that is not a function takes. */
export type NoArgs = { readonly [arg: string]: never };

/**
 * Renders a slot body: calls it with `args` when it is a function, returns
 * it as it is otherwise. The args are the function's parameter; a body that
 * cannot be a function takes none — as calling a function of no parameters
 * with one is an error.
 */
export function renderSlot<Children>(
  children: Children,
  args: Children extends (params: infer Params) => ReactNode ? Params : NoArgs,
): ReactNode {
  return typeof children === "function" ? children(args) : (children as ReactNode);
}
