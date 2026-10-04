// Reads the clock: shell-nondeterministic, wherever it is rendered.
export function Clock() {
  return <time>{Date.now()}</time>;
}

export default function Later() {
  return <p>later</p>;
}
