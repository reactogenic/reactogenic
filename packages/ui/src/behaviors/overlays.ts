// overlays — page-level (specs/phase02/components.md, *Behaviours*). Mounted
// by everything that opens: Dialog, DropdownMenu, SideMenu.
//
// What is open is closed when the reader goes somewhere else:
//
// - to another page (`pagehide`). The back/forward cache keeps a page as it
//   was left: open the drawer, follow a link in it, press Back — and the page
//   returns with the drawer open (a modal dialog returns with the page still
//   scroll-locked).
// - to another place of this page (`navigate`, of the Navigation API, which
//   every browser of the floor has). A `#fragment` link in an open drawer,
//   menu or dialog scrolls the page under it and leaves it open over what
//   the reader asked for: the page is not left, so nothing else closes it.
//   Not `hashchange`: a link to the fragment the page is already at — the
//   second click on the same link — changes no hash.
//
// `navigate` comes for a navigation that leaves the page too, earlier than
// `pagehide`; that one stays for what `navigate` does not see (an address
// typed into the browser) and for a browser without the API.

export default function mountOverlays(): void {
  addEventListener("pagehide", closeOverlays);
  (globalThis as { navigation?: EventTarget }).navigation?.addEventListener("navigate", closeOverlays);
}

function closeOverlays(): void {
  for (const popover of document.querySelectorAll<HTMLElement>(":popover-open")) {
    popover.hidePopover();
  }
  for (const dialog of document.querySelectorAll<HTMLDialogElement>("dialog[open]")) {
    dialog.close();
  }
}
