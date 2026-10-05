// What a published React package looks like: compiled JavaScript that calls
// React's JSX runtime itself, declarations beside it. Its elements are the
// record's and the shell rules' all the same (builder.md, *The record*).
import { jsx, jsxs } from "react/jsx-runtime";
import { createElement, useId } from "react";

function Dot() {
  return createElement("i", { className: "dot" });
}

export function VendorBadge({ label }) {
  const id = useId();
  return jsxs("span", { className: "badge", "data-id": typeof id, children: [jsx(Dot, {}), label] });
}
