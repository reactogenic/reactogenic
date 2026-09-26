export const a = <List>{({ item }) => null}</List>;
export const b = <List>{({ item }) => "text"}</List>;
export const c = <List>{({ item }) => <><Row item={item} /><Row item={item} /></>}</List>;
export const d = <List>{({ item }) => item.name}</List>;
