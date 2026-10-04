import type { ReactElement } from "react";
import { Clock } from "../../clock.tsx";

// A boundary that did not go through the builder's runtime: an element
// written out as an object, as a package with a JSX runtime of its own makes
// it. What it swallows is reported all the same.
const boundary = {
  $$typeof: Symbol.for("react.transitional.element"),
  type: Symbol.for("react.suspense"),
  key: null,
  ref: null,
  props: { fallback: "Loading", children: <Clock /> },
} as unknown as ReactElement;

export default function ByHandPage() {
  return (
    <html lang="en">
      <body>{boundary}</body>
    </html>
  );
}
