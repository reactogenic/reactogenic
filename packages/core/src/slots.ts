import type { ReactNode } from "react";

/** A function of params that renders: `Each`'s body, a function slot's body. */
export type SlotFn<Params> = (params: Params) => ReactNode;

/**
 * A slot: the complete prop contract of the element the container attaches
 * it to. A body is allowed only if `children` is in the contract, and
 * required only if `children` is.
 */
export type Slot<Props> = Props;

/** A function slot: its body receives the attachment's args (`&name`). */
export type FnSlot<Props, Args> = Omit<Props, "children"> & { children?: (args: Args) => ReactNode };

/** No args: what a slot whose body is not a function takes. */
export type NoArgs = { readonly [arg: string]: never };

/** The args a slot's body takes: a function slot's parameter, or none. */
export type ArgsOf<S> = S extends { children?: infer Body }
  ? NonNullable<Body> extends (args: infer Args) => ReactNode
    ? Args
    : NoArgs
  : NoArgs;

/**
 * Renders a slot's body at its attachment: calls it with `args` when it is a
 * function, returns it as it is otherwise, and returns `fallback` — the
 * attachment's children — when there is no body. Args follow function-call
 * rules; a slot typed as an array takes none at all, so it is rejected here.
 */
export function renderSlot<S extends object>(
  slot: S,
  args: S extends readonly unknown[] ? never : ArgsOf<S>,
  fallback?: ReactNode,
): ReactNode {
  const body = (slot as { children?: unknown }).children;
  if (body === undefined) {
    return fallback;
  }
  return typeof body === "function" ? body(args) : (body as ReactNode);
}
