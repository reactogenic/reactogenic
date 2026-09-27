import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotArgs as _slotArgs, slotEntry as _slotEntry, slotKey as _slotKey, slotProps as _slotProps } from "@reactogenic/core";
export function Select({ options, $Option, selected }: any) {
  return (
    <select>
      <Each items={options}>{({ item: option }) => ((_entry, _args) => _isAssigned(_entry) ? <option key={_slotKey(_entry, _args, option.value)} className="opt" value={option.value} selected={selected === option.value} {..._slotProps(_entry)}>{_renderSlot(_entry, _args, option.label)}</option> : <option key={option.value} className="opt" value={option.value} selected={selected === option.value}>
          {option.label}
        </option>)(_slotEntry($Option, option.value), _slotArgs($Option, { value: option.value, label: option.label, selected: selected === option.value }))}</Each>
    </select>
  );
}
export const d = (((_args) => _isAssigned(props.$Icon) ? <i key={_slotKey(props.$Icon, _args)} size={size} {..._slotProps(props.$Icon)}>{_renderSlot(props.$Icon, _args)}</i> : null)(_slotArgs(props.$Icon, { size: size, count: n + 1 })));
export const e = <div slot="header" />;
