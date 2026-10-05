// A page-level behaviour with a flag of its own.
declare const RG_INVOKERS_POPOVER: boolean;

export default function mountInvokers(): void {
  addEventListener("click", onClick);
  if (RG_INVOKERS_POPOVER) addEventListener("click", onPopover);
}

function onClick(e: Event): void {
  (e.target as HTMLElement).dataset.invoked = "fx:invokers";
}

function onPopover(e: Event): void {
  (e.target as HTMLElement).dataset.popover = "fx:invokers-popover";
}
