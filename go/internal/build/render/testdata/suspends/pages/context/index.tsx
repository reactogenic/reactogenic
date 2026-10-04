import { createContext, lazy, use, useContext } from "react";

// Context is render-time React, by either name; a `lazy` component that is
// made and not rendered is nothing.
const Theme = createContext("light");
export const Unused = lazy(() => import("../../clock.tsx"));

function Label() {
  return (
    <p>
      {use(Theme)} {useContext(Theme)}
    </p>
  );
}

export default function ContextPage() {
  return (
    <html lang="en">
      <body>
        <Theme value="dark">
          <Label />
        </Theme>
      </body>
    </html>
  );
}
