// A published React package: compiled JavaScript that calls React's JSX
// runtime and its hooks itself. The shell rules hold for it too.
import { jsx } from "react/jsx-runtime";
import { useState } from "react";

export function LibButton({ label }) {
  return jsx("button", { type: "button", onClick: () => alert(label), children: label });
}

export function LibCounter() {
  const [n] = useState(0);
  return jsx("output", { children: n });
}
