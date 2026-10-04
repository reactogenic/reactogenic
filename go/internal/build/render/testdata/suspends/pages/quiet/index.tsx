import * as React from "react";

// A boundary around what does not suspend is a boundary all the same —
// however it was reached.
export default function QuietPage() {
  return (
    <html lang="en">
      <body>
        {React.createElement(React.Suspense, null, <h1>Changelog</h1>)}
      </body>
    </html>
  );
}
