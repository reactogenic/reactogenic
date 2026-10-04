import { uid } from "../ids";

export default function LoadsPage() {
  return (
    <html lang="en">
      <body>{uid()}</body>
    </html>
  );
}
