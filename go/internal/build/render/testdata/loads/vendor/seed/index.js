// A package that reads the dice when it loads: no code of the project is on
// the stack.
const seed = Math.random();

export function uid() {
  return "id" + seed;
}
