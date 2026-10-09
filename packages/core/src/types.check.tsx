// Type-level tests: `tsc --noEmit` (pnpm typecheck) fails if an expectation
// breaks. syntax.md, *Slots*.
import type { ComponentProps, ReactNode } from "react";
import { Each, isAssigned, KEYED, NOT_ASSIGNED, renderSlot, SLOT_KEY, slotEntry, slotEntryName, slotKeys, variants, type KeyedSlot, type Slot } from "./index.ts";

// Slot<P> is the complete contract: a body only if `children` is in it.
type Box = Slot<{ color: string }>;
export const a: Box = { color: "red" };
// @ts-expect-error `children` is not in the contract
export const b: Box = { color: "red", children: "Hi" };
type Title = Slot<{ children?: ReactNode }>;
export const c: Title = {};
export const c2: Title = NOT_ASSIGNED; // a conditional that did not assign
type Label = Slot<{ children: ReactNode }>;
// @ts-expect-error the body is required
export const d: Label = {};
type Span = Slot<ComponentProps<"span">>;
export const e: Span = { className: "x", children: "Save" };
// @ts-expect-error `colour` is not a prop of <span>
export const f: Span = { colour: "red" };

// Slot<P, A>: the body takes the args.
type Icon = Slot<ComponentProps<"span">, { size: "md" | "lg" }>;
export const g: Icon = { className: "i", children: ({ size }) => size };
export const h: Icon = { className: "i" }; // no body: the attachment's fallback
// @ts-expect-error `colour` is not an arg
export const i: Icon = { children: ({ colour }) => colour };

// renderSlot: args follow function-call rules, on a slot that is there.
declare const iconOrNot: Icon;
declare const spanOrNot: Span;
const icon = isAssigned(iconOrNot) ? iconOrNot : { className: "i" };
const span = isAssigned(spanOrNot) ? spanOrNot : {};
export const r1 = renderSlot(icon, { size: "lg" });
// @ts-expect-error a function slot needs its args
export const r2 = renderSlot(icon, {});
export const r3 = renderSlot(span, {});
// @ts-expect-error a slot whose body is not a function takes no args
export const r4 = renderSlot(span, { size: "lg" });
declare const list: Span[];
// @ts-expect-error a slot is singular: an array is rejected at its attachment
export const r5 = renderSlot(list, {});

// Each infers the item type.
export const k = <Each items={[1, 2]}>{({ item }) => <b key={item}>{item.toFixed(1)}</b>}</Each>;
// @ts-expect-error item is a number
export const l = <Each items={[1, 2]}>{({ item }) => item.toUpperCase()}</Each>;

// KeyedSlot<P> / KeyedSlot<P, A>: entries by key, as the caller's compiled
// object literal writes them — entry props still checked.
type Column = KeyedSlot<{ width?: number; children?: ReactNode }>;
export const m1: Column = { [KEYED]: true, email: { width: 2 }, name: { children: "Name" } };
// @ts-expect-error `widht` is not a prop of an entry
export const m2: Column = { [KEYED]: true, email: { widht: 2 } };
type Cell = KeyedSlot<{ className?: string }, { row: number }>;
export const m3: Cell = { [KEYED]: true, total: { children: ({ row }) => row } };
// What `key="10"` and `key={expr}` compile to: an encoded name, the entry's
// props checked all the same.
declare const id: string | number;
export const m4: Column = { [KEYED]: true, "#10": { width: 2 }, [slotEntryName(id)]: { children: "Name" } };
// @ts-expect-error `widht` is not a prop of an entry
export const m5: Column = { [KEYED]: true, [slotEntryName(id)]: { widht: 2 } };
// @ts-expect-error a key is a string or a number
export const m6: string = slotEntryName(() => "a");
// A container iterates the keys as written, whatever the slot is.
declare const maybe: Column | undefined;
export const m7: string[] = slotKeys(maybe);
// An attachment with `key` gets the entry of a keyed slot, the slot itself otherwise.
declare const columns: Column;
export const e1: { width?: number; children?: ReactNode } | undefined = slotEntry(columns, "email");
declare const option: Slot<{ value?: string }>;
export const e2: Slot<{ value?: string }> = slotEntry(option, "a");

// A key function takes the args of a function slot; a slot without args has none.
type Option = Slot<{ value?: string; children?: ReactNode }, { value: string; label: string }>;
export const k1: Option = { [SLOT_KEY]: ({ value }) => value, children: ({ label }) => label };
// @ts-expect-error `id` is not an arg of `$Option`
export const k2: Option = { [SLOT_KEY]: ({ id }) => id };
// @ts-expect-error a slot without args takes no key function
export const k3: Slot<{ value?: string }> = { [SLOT_KEY]: () => "a" };

// variants: a dimension or a value the map does not have is a type error
// (builder.md, *What shell code can ask the builder*).
const buttonVariants = { size: { sm: "rg-button-sm", md: "" }, look: { ghost: "rg-button-ghost" } } as const;
declare const size: "sm" | "md" | undefined;
export const v1: string = variants("rg-button", buttonVariants, { size, look: "ghost" });
export const v2: string = variants("rg-button", buttonVariants);
export const v3: string = variants("rg-button", buttonVariants, {});
// @ts-expect-error `xl` is no value of `size`
variants("rg-button", buttonVariants, { size: "xl" });
// @ts-expect-error `colour` is no dimension of the map
variants("rg-button", buttonVariants, { colour: "red" });
// @ts-expect-error a value is a key of its dimension, not the class
variants("rg-button", buttonVariants, { look: "rg-button-ghost" });
// @ts-expect-error a map's values are classes: strings
variants("rg-button", { size: { sm: 1 } }, { size: "sm" });
