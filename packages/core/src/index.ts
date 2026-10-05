// @reactogenic/core — the phase 1 runtime (specs/phase01/vite.md, *Runtime*),
// and what shell code asks the builder (specs/phase02/builder.md).
export { isAssigned, KEYED, NOT_ASSIGNED, renderSlot, SLOT_KEY, slotArgs, slotEntry, slotEntryName, slotKey, slotKeys, slotProps } from "./slots.ts";
export type { ArgsOf, KeyedSlot, SlotArgs, NoArgs, NotAssigned, Slot, SlotEntry, SlotFn, SlotValue } from "./slots.ts";
export { Each } from "./each.ts";
export type { EachProps } from "./each.ts";
export { Match, noMatch, Switch } from "./flow.ts";
export type { MatchProps, SwitchCase, SwitchProps } from "./flow.ts";
export { mount, pathname, useShellId } from "./shell.ts";
export type { MountData, MountValue, ShellBuild } from "./shell.ts";
