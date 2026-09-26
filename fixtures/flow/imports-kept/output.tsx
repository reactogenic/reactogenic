import { Each, type SlotFn } from "@reactogenic/core";

export const a = (k === 1 ? <Each items={[]}>{() => null}</Each> : null);
export const b = (k ? "x" : null);
export type F = SlotFn<{}>;
