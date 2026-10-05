// A behaviour of the fixture's own that takes away what its page has when it
// loads: a class, toggled and removed; an id, written over; what an element
// holds, replaced by text. And it writes state the page has not: an
// `aria-expanded`, a `data-state`. It names each, in full (builder.md,
// *Behaviours*, what a behaviour writes), and adds or moves no element.
export default function mountToggle(root: HTMLElement): void {
  root.addEventListener("click", () => {
    root.classList.toggle("collapsed");
    for (const item of root.querySelectorAll(".item")) {
      item.classList.remove("active");
    }
    const label = root.querySelector("i");
    if (label) {
      label.id = "second";
    }
    // The status, and not by its class: a class the script names is "maybe"
    // on every element, and `.status:not(:has(b))` would stay for that alone.
    const status = root.lastElementChild;
    if (status) {
      status.textContent = "Copied";
    }
    // State the page does not have as it loads, and only a script writes:
    // named by the property that reflects the attribute, and by `dataset`'s
    // key (builder.md, *CSS*, *Runtime state*).
    root.ariaExpanded = "true";
    root.dataset.state = "open";
  });
}
