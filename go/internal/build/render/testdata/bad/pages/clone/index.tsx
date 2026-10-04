import { cloneElement } from "react";

const button = <button type="button">cloned</button>;

export default function ClonePage() {
  return (
    <html lang="en">
      <body>{cloneElement(button, { onClick: () => {} })}</body>
    </html>
  );
}
