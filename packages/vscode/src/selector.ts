/** The documents the server is attached to: `.rtsx` only (ide.md, *The `.ts` side*). */
export const SELECTOR: { language: string; scheme: string }[] = [
  { language: "rtsx", scheme: "file" },
  // No file name, so no extension: the server maps it by its language id
  // (go/internal/lsp/alias.go). Not `git:` — the left side of a diff.
  { language: "rtsx", scheme: "untitled" },
];
