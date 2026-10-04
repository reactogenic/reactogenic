// State at the top of a module: a cache of the ids given so far. It is the
// page's own — every page is rendered in a realm of its own — so the second
// page of a build starts where the first did.
const seen = new Map<string, number>();

export function slug(text: string): string {
  const n = seen.get(text) ?? 0;
  seen.set(text, n + 1);
  return n === 0 ? text : text + "-" + n;
}

// The same, on the global object and on a prototype.
export function visits(): number {
  const g = globalThis as unknown as { visits?: number };
  return (g.visits = (g.visits ?? 0) + 1);
}

export function patched(): string {
  const p = Array.prototype as unknown as { patched?: string };
  const was = p.patched ?? "no";
  p.patched = "yes";
  return was;
}
