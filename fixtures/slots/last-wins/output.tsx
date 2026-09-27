export const a = <Select $Option={{ value: "2" }} />;
export const b = <Select $Option={{ value: "1" }} />;
export const c = (
  <Select $Option={b ? { value: "b" } : ({ value: "a" })} />
);
