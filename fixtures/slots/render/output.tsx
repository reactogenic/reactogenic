import { Text } from "./text";
import { renderSlot as _renderSlot } from "@reactogenic/core";
export function Button(props: any) {
  const { $Label, $IconEnd, size } = props;
  return (
    <button>
      {$Label ? <Text {...$Label}>{_renderSlot($Label.children, {})}</Text> : null}
      {$IconEnd ? <div {...$IconEnd} key="icon">{_renderSlot($IconEnd.children, { size: size, ...extra })}</div> : null}
      {props.$Hint ? <span {...props.$Hint}>{_renderSlot(props.$Hint.children, {})}</span> : null}
      <div slot="header" />
      <div slot={name} />
      <Tooltip $Content={{ children: $Label ? <em {...$Label}>{_renderSlot($Label.children, {})}</em> : null }} />
    </button>
  );
}
export const lone = () => ($Only ? <Text {...$Only}>{_renderSlot($Only.children, {})}</Text> : null);
