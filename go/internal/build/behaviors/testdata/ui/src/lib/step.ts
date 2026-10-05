// A module the behaviour imports: its flags are the behaviour's too.
declare const RG_MENU_WRAP: boolean;

export function step(count: number, at: number, by: number): number {
  if (RG_MENU_WRAP) return wrap(count, at + by);
  return Math.min(Math.max(at + by, 0), count - 1);
}

function wrap(count: number, to: number): number {
  document.title = "fx:wrap";
  return (to + count) % count;
}
