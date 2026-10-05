// A per-root behaviour, in the authoring style of builder.md, *Behaviours*:
// flags are bare `declare const RG_…` identifiers, a feature is a top-level
// function (here in a module of its own), its state is module-level, `mount`
// only wires.
import { step } from "../lib/step";
import { onType } from "../lib/typeahead";

declare const RG_MENU_TYPEAHEAD: boolean;

// `own` is the mount's data: what this use site handed the behaviour
// (builder.md, *Behaviours*). Recorded, for the tests that run the script.
export default function mountMenuKeys(root: HTMLElement, own?: Record<string, unknown>): void {
  root.dataset.menuKeys = "fx:menu-keys";
  if (own) root.dataset.own = JSON.stringify(own);
  root.addEventListener("keydown", onKey);
  if (RG_MENU_TYPEAHEAD) root.addEventListener("keydown", onType);
}

function onKey(e: KeyboardEvent): void {
  const root = e.currentTarget as HTMLElement;
  const by = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
  if (by) root.dataset.at = String(step(Number(root.dataset.count), Number(root.dataset.at), by));
}
