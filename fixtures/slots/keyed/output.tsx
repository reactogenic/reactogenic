import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotArgs as _slotArgs, slotEntry as _slotEntry, slotKey as _slotKey, slotProps as _slotProps, KEYED as _KEYED } from "@reactogenic/core";
export const a = (
  <Table data $Column={{ [_KEYED]: true, "email": { ...emailColumn }, [nameKey]: { width: 2, children: "Name" }, ...(showAge ? { "age": {} } : {}) }} />
);
export const b = <Table $Column={{ [_KEYED]: true, "x": {}, [undefined]: {} }} />;
export function Table({ columns, $Column }: any) {
  return (
    <table>
      <Each items={columns}>{({ item: col }) => ((_entry, _args) => _isAssigned(_entry) ? <th key={_slotKey(_entry, _args, col.name)} {..._slotProps(_entry)}>{_renderSlot(_entry, _args, col.label)}</th> : <th key={col.name}>{col.label}</th>)(_slotEntry($Column, col.name), _slotArgs($Column, { col: col }))}</Each>
    </table>
  );
}
