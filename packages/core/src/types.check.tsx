// Type-level tests: `tsc --noEmit` (pnpm typecheck) fails if an expectation
// breaks. The *Declaration matrix* of syntax.md, written as plain TSX.
import type { ReactNode } from "react";
import { Each, renderSlot, type OptionalSlotFn, type SlotFn } from "./index.ts";

interface ButtonProps {
  children: ReactNode;
  $IconStart: { spacing: "tight" | "loose"; children: OptionalSlotFn<{ size: "md" | "lg" }> };
  $IconEnd?: { children?: ReactNode };
  $Row?: { children: SlotFn<{ id: number }> };
}
function Button({ children, $IconStart }: ButtonProps) {
  return <button>{renderSlot($IconStart.children, { size: "lg" })}{children}</button>;
}

// OptionalSlotFn: body only, or params + body.
export const a = <Button $IconStart={{ spacing: "tight", children: "+" }}>Add</Button>;
export const b = <Button $IconStart={{ spacing: "tight", children: ({ size }) => size }}>Add</Button>;
// Required slot missing.
// @ts-expect-error $IconStart is required
export const c = <Button>Add</Button>;
// Unknown option.
// @ts-expect-error colour is not an option
export const d = <Button $IconStart={{ spacing: "tight", colour: "red", children: "+" }}>Add</Button>;
// Param that the container does not hand out.
// @ts-expect-error colour is not a param
export const e = <Button $IconStart={{ spacing: "tight", children: ({ colour }) => colour }}>Add</Button>;
// ReactNode body: no params allowed.
// @ts-expect-error $IconEnd provides no values
export const f = <Button $IconStart={{ spacing: "tight", children: "+" }} $IconEnd={{ children: () => "x" }}>Add</Button>;
// SlotFn: params required.
// @ts-expect-error $Row requires params
export const g = <Button $IconStart={{ spacing: "tight", children: "+" }} $Row={{ children: "x" }}>Add</Button>;
// Optional body.
export const h = <Button $IconStart={{ spacing: "tight", children: "+" }} $IconEnd={{}}>Add</Button>;

// Each infers the item type.
export const i = <Each items={[1, 2]}>{({ item }) => <b key={item}>{item.toFixed(1)}</b>}</Each>;
// @ts-expect-error item is a number
export const j = <Each items={[1, 2]}>{({ item }) => item.toUpperCase()}</Each>;
