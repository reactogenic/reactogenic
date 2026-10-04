// A helper in plain TypeScript: the exception is reported here, where the
// clock is read, not in the component that called it.
export function stamp(label: string): string {
  const at = Date.now();
  return label + " " + at;
}
