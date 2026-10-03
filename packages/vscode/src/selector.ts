/** The documents the server is attached to: `.rtsx` only (ide.md, *The `.ts` side*). */
export const SELECTOR: { language: string; scheme: string }[] = [
  { language: "rtsx", scheme: "file" },
  { language: "rtsx", scheme: "untitled" }, // not `git:` — the left side of a diff
];
