// menu-keys — per menu (specs/phase02/components.md, *Behaviours*). Mounted by
// a DropdownMenu with an action item: `role="menu"` promises the keys of the
// APG menu pattern, and the platform has none of them (`focusgroup` is
// Chrome 150+ only). Opening, Esc, light dismiss and the focus on the first
// item are the browser's.
//
// Authored for the linker (specs/phase02/builder.md, *Behaviours*): the flag
// is a bare identifier the builder defines per page, each feature is a
// top-level function, state is module-level.
declare const RG_MENU_TYPEAHEAD: boolean;

export default function mountMenuKeys(root: HTMLElement): void {
  root.addEventListener("keydown", onKey);
  root.addEventListener("click", onPick);
  if (RG_MENU_TYPEAHEAD) {
    root.addEventListener("keydown", onType);
  }
}

// The items focus can move to.
function itemsOf(menu: HTMLElement): HTMLElement[] {
  return [...menu.querySelectorAll<HTMLElement>("[role=menuitem]:not(:disabled)")];
}

// Closes the menu with focus back on its trigger — the element that labels
// it. Not left to the browser: WebKit does not focus a button on click, so a
// menu opened with the mouse has no invoker to return to.
function close(menu: HTMLElement): void {
  menu.hidePopover();
  document.getElementById(menu.getAttribute("aria-labelledby")!)?.focus();
}

// Arrows with wrap, Home, End; Tab closes and moves on from the trigger.
function onKey(event: KeyboardEvent): void {
  const menu = event.currentTarget as HTMLElement;
  const items = itemsOf(menu);
  const at = items.indexOf(event.target as HTMLElement);
  const to = ({ ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: -1 } as Record<string, number | undefined>)[event.key];
  if (event.key === "Tab") {
    close(menu);
  } else if (to !== undefined) {
    event.preventDefault();
    items.at(to % items.length)?.focus();
  }
}

// Activating an item closes the menu, before the item acts: a dialog it
// opens then returns focus to the trigger, not to an item that is gone.
function onPick(event: MouseEvent): void {
  if ((event.target as Element).closest("[role=menuitem]")) {
    close(event.currentTarget as HTMLElement);
  }
}

// A printable key moves to the next item whose label starts with it.
function onType(event: KeyboardEvent): void {
  const key = event.key.toLowerCase();
  if (key.length !== 1 || key === " " || event.ctrlKey || event.metaKey || event.altKey) {
    return;
  }
  const items = itemsOf(event.currentTarget as HTMLElement);
  const at = items.indexOf(event.target as HTMLElement);
  [...items.slice(at + 1), ...items.slice(0, at + 1)].find((item) => item.textContent!.trim().toLowerCase().startsWith(key))?.focus();
}
