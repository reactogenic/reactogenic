// Components whose classes are resolved by `variants()` of @reactogenic/core
// — the real one: the record of the page has them (builder.md, *What shell
// code can ask the builder*).
import type { ReactNode } from "react";
import { variants } from "@reactogenic/core";

const buttonVariants = { size: { sm: "button-sm", md: "", lg: "button-lg" }, look: { ghost: "button-ghost" } } as const;

// While the module loads no page renders: nothing is asked of the builder.
export const loaded = variants("loaded", buttonVariants, { size: "lg" });

export function Button({ size, look, children }: { size?: "sm" | "md" | "lg"; look?: "ghost"; children?: ReactNode }) {
  return <button className={variants("button", buttonVariants, { size, look })}>{children}</button>;
}

// A second component that resolves the same classes: both are in the record.
export function Toolbar() {
  return <div className={variants("toolbar button", buttonVariants, { look: "ghost" })} />;
}
