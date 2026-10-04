// overlays — page-level (specs/phase02/components.md, *Behaviours*). Mounted
// by everything that opens: Dialog, DropdownMenu, SideMenu.
//
// The back/forward cache keeps a page as it was left: open the drawer, follow
// a link in it, press Back — and the page returns with the drawer open (a
// modal dialog returns with the page still scroll-locked). So what is open is
// closed when the page is left.

export default function mountOverlays(): void {
  addEventListener("pagehide", closeOverlays);
}

function closeOverlays(): void {
  for (const popover of document.querySelectorAll<HTMLElement>(":popover-open")) {
    popover.hidePopover();
  }
  for (const dialog of document.querySelectorAll<HTMLDialogElement>("dialog[open]")) {
    dialog.close();
  }
}
