import { Suspense } from "react";
import { Clock } from "../../clock.tsx";

// A boundary renders its fallback for whatever its content throws — the
// clock's error among it — and says nothing.
export default function SuspensePage() {
  return (
    <html lang="en">
      <body>
        <Suspense fallback={<p>Loading</p>}>
          <h1>Changelog</h1>
          <Clock />
        </Suspense>
      </body>
    </html>
  );
}
