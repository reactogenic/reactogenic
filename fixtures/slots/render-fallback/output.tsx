import { renderSlot as _renderSlot } from "@reactogenic/core";
export const a = ($Title ? <Text className="title" {...$Title}>{_renderSlot($Title, {}, "Hello, World!")}</Text> : <Text className="title">Hello, World!</Text>);
export const b = ($Heading ? <h1 className="title" {...$Heading}>{_renderSlot($Heading, {})}</h1> : null);
export const c = ($Badge ? <span {...$Badge}>{_renderSlot($Badge, { size: size }, "new")}</span> : <span>new</span>);
