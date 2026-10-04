// A per-root behaviour, in the authoring style of builder.md, *Behaviours*:
// flags are bare `declare const RG_…` identifiers, a feature is a top-level
// function (here in a module of its own), its state is module-level, `mount`
// only wires.
import { step } from "../lib/step";
import { onType } from "../lib/typeahead";

declare const RG_MENU_TYPEAHEAD: boolean;

export default function mountMenuKeys(root: HTMLElement): void {
  root.dataset.menuKeys = "fx:menu-keys";
  root.addEventListener("keydown", onKey);
  if (RG_MENU_TYPEAHEAD) root.addEventListener("keydown", onType);
}

function onKey(e: KeyboardEvent): void {
  const root = e.currentTarget as HTMLElement;
  const by = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
  if (by) root.dataset.at = String(step(Number(root.dataset.count), Number(root.dataset.at), by));
}
