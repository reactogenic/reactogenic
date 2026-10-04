// Resolves, but an import of its own does not.
import { missing } from "../lib/missing";

export default function mountBroken(root: HTMLElement): void {
  missing(root);
}
