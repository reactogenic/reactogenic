import type { ReactNode } from "react";

/** A slot body that needs the container's values: `(params) => body`. */
export type SlotFn<Params> = (params: Params) => ReactNode;

/** A slot body that may take the container's values, or not. */
export type OptionalSlotFn<Params> = ReactNode | SlotFn<Params>;

/**
 * A slot: the props of the element the container renders for it (its
 * options), and its body. Rendered with `<El slot={$X} … />` in .rtsx.
 */
export type Slot<Props, Children = ReactNode> = Omit<Props, "children"> & { children: Children };

/**
 * Renders a slot body: calls it with `args` when it is a function, returns
 * it as it is otherwise. Containers call it for every `OptionalSlotFn`.
 */
export function renderSlot<Params>(children: OptionalSlotFn<Params>, args: Params): ReactNode {
  return typeof children === "function" ? children(args) : children;
}
