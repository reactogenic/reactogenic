import type { ReactNode } from "react";

/** A function of params that renders: `Each`'s body, a function slot's body. */
export type SlotFn<Params> = (params: Params) => ReactNode;

/**
 * The value of a slot whose assignment did not happen: the false branch of
 * a conditional `<$X>`. Unlike `undefined`, it never replaces a value — an
 * attachment treats it as no slot, and a spread of slot props skips it.
 */
export const NOT_ASSIGNED: unique symbol = Symbol.for("reactogenic.notAssigned");
export type NotAssigned = typeof NOT_ASSIGNED;

/**
 * A slot: the complete prop contract of the element the container attaches
 * it to. A body is allowed only if `children` is in the contract, and
 * required only if `children` is.
 */
export type Slot<Props> = Props | NotAssigned;

/** A function slot: its body receives the attachment's args (`&name`). */
export type FnSlot<Props, Args> =
  | (Omit<Props, "children"> & { children?: (args: Args) => ReactNode })
  | NotAssigned;

/** Is the slot there: neither missing nor NOT_ASSIGNED? */
export function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null> {
  return slot !== undefined && slot !== null && slot !== NOT_ASSIGNED;
}

/**
 * A slot's props for its attachment's element, without the entries that
 * were not assigned — so a nested slot that was not assigned keeps the
 * attachment's default instead of replacing it.
 */
export function slotProps<S extends object>(slot: S): S {
  const props: Record<string, unknown> = {};
  for (const [name, value] of Object.entries(slot)) {
    if (value !== NOT_ASSIGNED) {
      props[name] = value;
    }
  }
  return props as S;
}

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
