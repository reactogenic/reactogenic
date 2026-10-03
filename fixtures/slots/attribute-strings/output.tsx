import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotArgs as _slotArgs, slotEntry as _slotEntry, slotKey as _slotKey, slotProps as _slotProps, KEYED as _KEYED } from "@reactogenic/core";
export const a = (
  <Button title="Tom &amp; Jerry" $Icon={{ className: "\n      w-4 h-4\n      text-red-500\n    " }} $Label={{ title: "Tom & Jerry", path: "C:\\new\\table", quote: "say \"hi\" & & © &nope; &amp", plain: "as is", children: "x" }} />
);
export const b = (
  status === "a&b" ? "A"
  : status === "C:\\temp" ? "B"
  : status === "plain" ? "C"
  : null
);
export const c = (
  <Table $Column={{ [_KEYED]: true, "a&b": {}, "C:\\new": {} }} />
);
export const d = "a\\b" ? "yes" : null;
export function Box({ $Title, $Row }: any) {
  return (
    <div>
      {((_args) => _isAssigned($Title) ? <h1 key={_slotKey($Title, _args)} title="C:\temp" {..._slotProps($Title)}>{_renderSlot($Title, _args, "fallback")}</h1> : <h1 title="C:\temp">fallback</h1>)(_slotArgs($Title, { title: "C:\\temp", plain: "a&b" }))}
      {((_entry) => _isAssigned(_entry) ? <p key="r&amp;1" {..._slotProps(_entry)}>{_renderSlot(_entry, {})}</p> : null)(_slotEntry($Row, "r&1"))}
    </div>
  );
}
