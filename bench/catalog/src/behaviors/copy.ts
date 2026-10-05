// copy — per code block (bench/catalog/README.md: a measurement fixture, in
// the authoring style of specs/phase02/builder.md, *Behaviours*). Mounted by
// a CodeBlock with `copy`: its button copies the sample, and says so for two
// seconds.
//
// It writes one thing to the page: the button's text, which a behaviour may
// set (builder.md, what a behaviour writes). No class, no attribute.

/** What a code block hands its behaviour: the mount's data. */
export interface CopyData {
  /** What the button says once it has copied: "Copied". */
  done: string;
}

// The text each root's button says after a copy.
const said = new WeakMap<HTMLElement, string>();

export default function mountCopy(root: HTMLElement, own: CopyData): void {
  said.set(root, own.done);
  root.addEventListener("click", onClick);
}

function onClick(event: MouseEvent): void {
  const root = event.currentTarget as HTMLElement;
  const button = (event.target as Element).closest<HTMLElement>("[data-part=copy]");
  if (!button) {
    return;
  }
  const label = button.textContent;
  navigator.clipboard.writeText(root.querySelector("code")!.textContent).then(() => {
    button.textContent = said.get(root)!;
    setTimeout(restore, 2000, button, label);
  }, ignore);
}

function restore(button: HTMLElement, label: string): void {
  button.textContent = label;
}

// A page that may not write to the clipboard: the button stays as it was.
function ignore(): void {}
