import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotProps as _slotProps } from "@reactogenic/core";
export const a = (_isAssigned($Title) ? <Text className="title" {..._slotProps($Title)}>{_renderSlot($Title, {}, "Hello, World!")}</Text> : <Text className="title">Hello, World!</Text>);
export const b = (_isAssigned($Heading) ? <h1 className="title" {..._slotProps($Heading)}>{_renderSlot($Heading, {})}</h1> : null);
export const c = (_isAssigned($Badge) ? <span {..._slotProps($Badge)}>{_renderSlot($Badge, { size: size }, "new")}</span> : <span>new</span>);
