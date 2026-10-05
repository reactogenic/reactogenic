import { createElement } from "react";

// An element made by hand is an element: the shell rule is the same. Where
// a component's element was made by hand is not known: `in Row` has no place.
function Row() {
  return createElement("button", { type: "button", onClick: () => {} }, "by hand");
}

export default function ByHandPage() {
  return (
    <html lang="en">
      <body>{createElement(Row)}</body>
    </html>
  );
}
