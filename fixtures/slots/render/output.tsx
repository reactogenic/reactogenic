import { renderSlot as _renderSlot } from "@reactogenic/core";
export function Select({ options, $Option, selected }: any) {
  return (
    <select>
      <Each items={options}>{({ item: option }) => $Option ? <option key={option.value} className="opt" value={option.value} selected={selected === option.value} {...$Option}>{_renderSlot($Option, { value: option.value, label: option.label, selected: selected === option.value }, option.label)}</option> : <option key={option.value} className="opt" value={option.value} selected={selected === option.value}>
          {option.label}
        </option>}</Each>
    </select>
  );
}
export const d = (props.$Icon ? <i size={size} {...props.$Icon}>{_renderSlot(props.$Icon, { size: size, count: n + 1 })}</i> : null);
export const e = <div slot="header" />;
