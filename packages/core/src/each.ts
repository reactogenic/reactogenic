import type { ReactNode } from "react";
import type { SlotFn } from "./slots.ts";

export interface EachProps<T> {
  items: readonly T[];
  children: SlotFn<{ item: T; index: number }>;
}

/**
 * Renders its body once per item. The body returns the keyed element; `Each`
 * adds no wrapper (syntax.md, *Iteration*).
 */
export function Each<T>({ items, children }: EachProps<T>): ReactNode {
  return items.map((item, index) => children({ item, index }));
}
