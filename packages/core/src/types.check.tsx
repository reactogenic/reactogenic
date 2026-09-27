// Type-level tests: `tsc --noEmit` (pnpm typecheck) fails if an expectation
// breaks. syntax.md, *Slots*.
import type { ComponentProps, ReactNode } from "react";
import { Each, isAssigned, NOT_ASSIGNED, renderSlot, type FnSlot, type Slot } from "./index.ts";

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

// FnSlot<P, A>: the body takes the args.
type Icon = FnSlot<ComponentProps<"span">, { size: "md" | "lg" }>;
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
