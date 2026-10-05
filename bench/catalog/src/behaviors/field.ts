// field — per field (bench/catalog/README.md: a measurement fixture, in the
// authoring style of specs/phase02/builder.md, *Behaviours*). Mounted by a
// Field that counts its characters or shows its password; a plain field
// mounts nothing.
//
// Two features, each behind a flag of the page and asked for by the mount's
// data. What they write is named in full: the count's text and its
// `data-full`; the button's `aria-pressed` and text, the input's `type`.
declare const RG_FIELD_COUNT: boolean;
declare const RG_FIELD_REVEAL: boolean;

/** What a field hands its behaviour: the mount's data. */
export interface FieldData {
  /** "12 / 160" as the reader types. */
  count?: boolean;
  /** The button that shows the password. */
  reveal?: boolean;
}

export default function mountField(root: HTMLElement, own?: FieldData): void {
  if (RG_FIELD_COUNT && own?.count) {
    root.addEventListener("input", onInput);
    count(root);
  }
  if (RG_FIELD_REVEAL && own?.reveal) {
    root.addEventListener("click", onReveal);
  }
}

function inputOf(root: HTMLElement): HTMLInputElement | HTMLTextAreaElement {
  return root.querySelector<HTMLInputElement | HTMLTextAreaElement>("input, textarea")!;
}

// "12 / 160", and `data-full` once nothing more fits.
function count(root: HTMLElement): void {
  const input = inputOf(root);
  const output = root.querySelector("output")!;
  output.textContent = input.value.length + " / " + input.maxLength;
  output.toggleAttribute("data-full", input.value.length >= input.maxLength);
}

function onInput(event: Event): void {
  count(event.currentTarget as HTMLElement);
}

// The button turns the password into text and back, and says which it is.
function onReveal(event: MouseEvent): void {
  const button = (event.target as Element).closest<HTMLElement>("[data-part=reveal]");
  if (!button) {
    return;
  }
  const input = inputOf(event.currentTarget as HTMLElement) as HTMLInputElement;
  const shown = input.type === "password";
  input.type = shown ? "text" : "password";
  button.setAttribute("aria-pressed", shown ? "true" : "false");
  button.textContent = shown ? "Hide" : "Show";
}
