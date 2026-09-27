import { isAssigned as _isAssigned, renderSlot as _renderSlot, slotArgs as _slotArgs, slotKey as _slotKey, slotProps as _slotProps } from "@reactogenic/core";
export const a = (_isAssigned($Title) ? <Text className="title" {..._slotProps($Title)}>{_renderSlot($Title, {}, "Hello, World!")}</Text> : <Text className="title">Hello, World!</Text>);
export const b = (_isAssigned($Heading) ? <h1 className="title" {..._slotProps($Heading)}>{_renderSlot($Heading, {})}</h1> : null);
export const c = (((_args) => _isAssigned($Badge) ? <span key={_slotKey($Badge, _args)} {..._slotProps($Badge)}>{_renderSlot($Badge, _args, "new")}</span> : <span>new</span>)(_slotArgs($Badge, { size: size })));
