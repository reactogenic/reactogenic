// invokers — page-level (specs/phase02/components.md, *Behaviours*). Mounted
// by Dialog and by every button or menu item with `command`.
//
// `command` / `commandfor` where the browser has none (before Chrome 135,
// Firefox 144, Safari 26.2 — about 15% of usage): without it every dialog
// button is dead. Feature-detected: at the floor it adds no listener.
// Covers the commands of the floor, which is all `Command` has — a dialog's
// `show-modal` and `close`; popovers use `popovertarget`, which is older.
// Not `request-close`: it is Chrome 139, so in Chrome 135–138 — which have
// `command`, and get no fallback — a button with it is dead.

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
  } else if (command === "close") {
    dialog.close();
  }
}
