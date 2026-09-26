export const a = (
  <Input $Hint={state.kind === "error" ? { tone: "bad", children: "e" } : state.kind === "warn" ? { tone: "warn", children: "w" } : undefined} />
);
