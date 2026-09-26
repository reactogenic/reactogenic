import type { Key, ReactNode } from "react";
import type { OptionalSlotFn } from "./slots.ts";

// `Switch` and `Match` exist only as elements: the transpiler lowers them to
// conditional expressions and drops their import (syntax.md, *Flow
// control*). These declarations give them their slots and types; the
// functions throw if anything renders them anyway.

export interface MatchProps<T> {
  on: T;
  children?: OptionalSlotFn<{ value: NonNullable<T> }>;
}

export interface SwitchCase {
  is?: unknown;
  default?: true;
  key?: Key;
  children?: ReactNode;
}

export interface SwitchProps<T> {
  on: T;
  exhaustive?: true;
  $Case: SwitchCase[];
  children?: OptionalSlotFn<{ value: T }>;
}

function compileTimeOnly(name: string): never {
  throw new Error(
    `<${name}> exists only in .rtsx and is compiled away; it was rendered at runtime. ` +
      `Is the Reactogenic Vite plugin running on this file?`,
  );
}

export function Match<T>(_props: MatchProps<T>): ReactNode {
  return compileTimeOnly("Match");
}

export function Switch<T>(_props: SwitchProps<T>): ReactNode {
  return compileTimeOnly("Switch");
}

/**
 * The end of an exhaustive `Switch`: TS proves it unreachable; at runtime it
 * throws, because data can lie.
 */
export function noMatch(value: never): never {
  throw new Error(`No $Case matched ${JSON.stringify(value) ?? String(value)}`);
}
