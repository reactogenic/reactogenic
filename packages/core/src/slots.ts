import type { Key, ReactNode } from "react";

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
 * Holds a slot's key function: `<$Option key={({ value }) => value} />`. A
 * symbol, so it never clashes with the contract's own `key`.
 */
export const SLOT_KEY: unique symbol = Symbol.for("reactogenic.slotKey");

/**
 * A slot's value: its props, with a body that takes `Args` when given — and
 * then a key function of the same args, for an attachment that runs per item.
 */
export type SlotValue<Props, Args = never> = [Args] extends [never]
  ? Props
  : Omit<Props, "children"> & {
      children?: (args: Args) => ReactNode;
      readonly [SLOT_KEY]?: (args: Args) => Key;
    };

/**
 * A slot: the complete prop contract of the element the container attaches
 * it to — `Slot<P>` — or, with `Args`, a slot whose body is a function of the
 * attachment's args (`&name`) — `Slot<P, A>`. A body is allowed only if
 * `children` is in the contract, or `Args` is given.
 */
export type Slot<Props, Args = never> = SlotValue<Props, Args> | NotAssigned;

/**
 * Marks a keyed slot's value. The caller's compiled code puts it in the
 * object literal, so an attachment can tell a keyed slot from a singular one.
 */
export const KEYED: unique symbol = Symbol.for("reactogenic.keyed");

/**
 * A keyed slot: entries by key, each a `Slot<P, A>` value. The caller fills
 * it with `<$X key=…>`; an attachment with `key` renders the entry of that
 * key. Keys are strings or numbers (integer-like keys enumerate first).
 */
export type KeyedSlot<Props, Args = never> = { readonly [KEYED]: true } & {
  readonly [key: string]: SlotValue<Props, Args>;
};

/** A keyed slot's entry type, or the slot itself for a singular one. */
export type SlotEntry<S> = S extends { readonly [KEYED]: true } & { readonly [key: string]: infer Entry }
  ? Entry | undefined
  : S;

/**
 * The value an attachment with `key` renders: a keyed slot's entry for that
 * key, or — for a singular slot attached many times — the slot itself.
 */
export function slotEntry<S>(slot: S, key: string | number): SlotEntry<S> {
  if (typeof slot === "object" && slot !== null && (slot as { [KEYED]?: true })[KEYED] === true) {
    return (slot as unknown as Record<string, unknown>)[key] as SlotEntry<S>;
  }
  return slot as SlotEntry<S>;
}

/**
 * The args of an attachment, typed for its slot as `renderSlot` types them:
 * the compiled attachment builds them once, here, for the key and the body.
 */
export type SlotArgs<S> = S extends readonly unknown[]
  ? never
  : ArgsOf<Exclude<SlotEntry<S>, NotAssigned | undefined | null>>;

export function slotArgs<S>(_slot: S, args: SlotArgs<S>): SlotArgs<S> {
  return args;
}

/**
 * The React key of an attachment with args: the slot's key function applied
 * to them, or else the attachment's own `key`.
 */
export function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined {
  const keyOf = typeof slot === "object" && slot !== null ? (slot as { [SLOT_KEY]?: (args: object) => Key })[SLOT_KEY] : undefined;
  return keyOf ? keyOf(args) : fallback;
}

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
