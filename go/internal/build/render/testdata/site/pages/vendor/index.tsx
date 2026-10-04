/** @jsxImportSource react */
// JavaScript behind declarations, reached through a `paths` alias: the
// program resolves the import — to the declarations — and the bundle takes
// the JavaScript beside them (builder.md, *One resolver*). And a file that
// names its own JSX import source: React's runtime is the builder's here.
import { legacy } from "~/legacy.js";
import { VendorBadge } from "~/vendor/badge/index.js";

function Note() {
  return <p>{legacy}</p>;
}

export default function VendorPage() {
  return (
    <html lang="en">
      <body>
        <Note />
        <VendorBadge label="compiled" />
      </body>
    </html>
  );
}
