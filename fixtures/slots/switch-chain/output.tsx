import { NOT_ASSIGNED as _NOT_ASSIGNED } from "@reactogenic/core";
export const a = (
  <Input $Hint={state.kind === "error" ? { tone: "bad", children: "e" } : state.kind === "warn" ? { tone: "warn", children: "w" } : _NOT_ASSIGNED} />
);
