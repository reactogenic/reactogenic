// A stub of @reactogenic/core's types (as go/internal/lsptest's), reached
// through a `paths` alias: the fixture needs no node_modules.
type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol };
type Key = string | number | bigint;
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
export declare const SLOT_KEY: unique symbol;
export type SlotValue<Props, Args = never> = [Args] extends [never]
  ? Props
  : Omit<Props, "children"> & { children?: (args: Args) => ReactNode; readonly [SLOT_KEY]?: (args: Args) => Key };
export type Slot<Props, Args = never> = SlotValue<Props, Args> | NotAssigned;
export declare const KEYED: unique symbol;
export type KeyedSlot<Props, Args = never> = { readonly [KEYED]: true } & {
  readonly [key: string]: SlotValue<Props, Args>;
};
export type SlotEntry<S> = S extends { readonly [KEYED]: true } & { readonly [key: string]: infer Entry }
  ? Entry | undefined
  : S;
export declare function slotEntry<S>(slot: S, key: string | number): SlotEntry<S>;
export declare function slotEntryName(key: string | number): string;
export declare function slotKeys(slot: unknown): string[];
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export type NoArgs = { readonly [arg: string]: never };
export type ArgsOf<S> = S extends { children?: infer Body }
  ? NonNullable<Body> extends (args: infer Args) => ReactNode
    ? Args
    : NoArgs
  : NoArgs;
export type SlotArgs<S> = S extends readonly unknown[]
  ? never
  : ArgsOf<Exclude<SlotEntry<S>, NotAssigned | undefined | null>>;
export declare function slotArgs<S>(slot: S, args: SlotArgs<S>): SlotArgs<S>;
export declare function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined;
export declare function renderSlot<S extends object>(
  slot: S,
  args: S extends readonly unknown[] ? never : ArgsOf<S>,
  fallback?: ReactNode,
): ReactNode;
export declare function Match(props: { on: unknown; children?: unknown }): never;
export declare function Switch(props: { on: unknown; exhaustive?: true; $Case?: unknown; children?: unknown }): null;
export declare function noMatch(value: never): never;
export declare function Each<T>(props: {
  items: readonly T[];
  children: (params: { item: T; index: number }) => ReactNode;
}): ReactNode;
