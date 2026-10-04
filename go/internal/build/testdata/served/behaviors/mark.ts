// A behaviour of the fixture's own: it marks its element when it is mounted.
export default function mountMark(root: HTMLElement): void {
  root.setAttribute("aria-busy", "false");
}
