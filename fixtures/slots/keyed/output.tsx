import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotEntry as _slotEntry, slotProps as _slotProps, KEYED as _KEYED } from "@reactogenic/core";
export const a = (
  <Table data $Column={{ [_KEYED]: true, "email": { ...emailColumn }, [nameKey]: { width: 2, children: "Name" }, ...(showAge ? { "age": {} } : {}) }} />
);
export const b = <Table $Column={{ [_KEYED]: true, "x": {}, [undefined]: {} }} />;
export function Table({ columns, $Column }: any) {
  return (
    <table>
      <Each items={columns}>{({ item: col }) => ((_entry) => _isAssigned(_entry) ? <th key={col.name} {..._slotProps(_entry)}>{_renderSlot(_entry, { col: col }, col.label)}</th> : <th key={col.name}>{col.label}</th>)(_slotEntry($Column, col.name))}</Each>
    </table>
  );
}
