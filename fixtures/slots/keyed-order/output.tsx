import { KEYED as _KEYED, slotEntryName as _slotEntryName } from "@reactogenic/core";
export const a = (
  <Menu $Item={{ [_KEYED]: true, "#10": { children: "Ten" }, "#9": { children: "Nine" }, "#2": {}, "b": {}, "0.5": {}, "01": {}, "##top": {}, "#11": {}, ["__proto__"]: {}, ...(more ? { "#3": {} } : {}) }} />
);
export const b = (
  <Menu $Item={{ [_KEYED]: true, [_slotEntryName(n)]: {}, [_slotEntryName(0.5)]: {}, [_slotEntryName(7)]: {}, "#7": {}, '##x': {}, "name": {}, [_slotEntryName("\x31")]: {}, [_slotEntryName(`8`)]: {}, [_slotEntryName(a ? 1 : "one")]: {}, ...(more ? { [_slotEntryName(m)]: {} } : {}) }} />
);
export const c = <Menu $Item={{ ...given, [_KEYED]: true, "#5": {} }} />;
