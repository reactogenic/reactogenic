// invokers — page-level (specs/phase02/components.md, *Behaviours*). Mounted
// by Dialog and by every button or menu item with `command`.
//
// `command` / `commandfor` where the browser has none (before Chrome 135,
// Firefox 144, Safari 26.2 — about 15% of usage): without it every dialog
// button is dead. Feature-detected: at the floor it adds no listener.
// Covers what the components emit — a dialog's `show-modal`, `close`,
// `request-close`; popovers use `popovertarget`, which is older.

export default function mountInvokers(): void {
  if (!("command" in HTMLButtonElement.prototype)) {
    addEventListener("click", invoke);
  }
}

function invoke(event: MouseEvent): void {
  const button = (event.target as Element).closest("button[commandfor]");
  const dialog = button && (document.getElementById(button.getAttribute("commandfor")!) as HTMLDialogElement | null);
  if (!dialog) {
    return;
  }
  const command = button.getAttribute("command");
  if (command === "show-modal") {
    // As the native command: nothing when it is already open.
    if (!dialog.open) {
      dialog.showModal();
    }
  } else if (command === "close" || command === "request-close") {
    dialog.close();
  }
}
