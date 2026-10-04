// What @types/react 19.3 lacks (specs/phase02/components.md, *Types*): the
// invoker-command attributes. It has `popover`, `popoverTarget`,
// `popoverTargetAction` and `closedby` already. React's renderer passes
// unknown attributes through as written, so these are lower-case, as in HTML.
import "react";

declare module "react" {
  interface ButtonHTMLAttributes<T> {
    /** What the button does to the element `commandfor` names. */
    command?: "show-modal" | "close" | "request-close" | "show-popover" | "hide-popover" | "toggle-popover" | `--${string}` | undefined;
    /** The id of the element the command is for. */
    commandfor?: string | undefined;
  }
}
