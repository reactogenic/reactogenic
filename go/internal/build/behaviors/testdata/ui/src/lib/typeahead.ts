// One feature, one module: with RG_MENU_TYPEAHEAD off nothing of it is left.
let typed = "";

export function onType(e: KeyboardEvent): void {
  if (e.key.length !== 1) return;
  typed += e.key;
  (e.currentTarget as HTMLElement).dataset.typed = "fx:typeahead:" + typed;
}
