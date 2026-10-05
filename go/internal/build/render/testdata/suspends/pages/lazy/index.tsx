import { lazy } from "react";

const Later = lazy(() => import("../../clock.tsx"));

export default function LazyPage() {
  return (
    <html lang="en">
      <body>
        <Later />
      </body>
    </html>
  );
}
