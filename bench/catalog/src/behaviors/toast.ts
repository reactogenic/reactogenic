// toast — per toast (bench/catalog/README.md: a measurement fixture, in the
// authoring style of specs/phase02/builder.md, *Behaviours*). Mounted by a
// Toast with a timeout: showing and dismissing are the platform's
// (`popover`), and hiding it again after a while is this script's. The
// clock stops while the pointer or the focus is in it (WCAG 2.2.1).
//
// It writes nothing to the page: it calls `hidePopover()`.

/** What a toast hands its behaviour: the mount's data. */
export interface ToastData {
  /** Milliseconds until the toast hides itself. */
  timeout: number;
}

// A shown toast's clock, and how long each toast stays.
const clocks = new WeakMap<HTMLElement, ReturnType<typeof setTimeout>>();
const stays = new WeakMap<HTMLElement, number>();

export default function mountToast(root: HTMLElement, own: ToastData): void {
  stays.set(root, own.timeout);
  root.addEventListener("toggle", onToggle);
  root.addEventListener("pointerenter", hold);
  root.addEventListener("focusin", hold);
  root.addEventListener("pointerleave", start);
  root.addEventListener("focusout", start);
}

function onToggle(event: Event): void {
  if ((event as ToggleEvent).newState === "open") {
    start(event);
  } else {
    hold(event);
  }
}

function hold(event: Event): void {
  clearTimeout(clocks.get(event.currentTarget as HTMLElement));
}

function start(event: Event): void {
  const toast = event.currentTarget as HTMLElement;
  hold(event);
  if (toast.matches(":popover-open")) {
    clocks.set(toast, setTimeout(hide, stays.get(toast), toast));
  }
}

function hide(toast: HTMLElement): void {
  toast.hidePopover();
}
