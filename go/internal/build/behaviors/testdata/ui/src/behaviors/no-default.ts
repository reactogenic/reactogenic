// Not a behaviour: no default export.
export function mountNamed(root: HTMLElement): void {
  root.dataset.named = "fx:no-default";
}
