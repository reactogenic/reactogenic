// A page-level behaviour: mounted without an id, run once per page.
export default function mountOverlays(): void {
  addEventListener("pagehide", closeAll);
}

function closeAll(): void {
  for (const el of document.querySelectorAll("[popover]")) el.setAttribute("data-closed", "fx:overlays");
}
