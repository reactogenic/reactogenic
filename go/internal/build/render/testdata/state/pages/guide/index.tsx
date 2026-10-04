import { patched, slug, visits } from "../../slug.ts";

export default function GuidePage() {
  return (
    <html lang="en">
      <body>
        <h2 id={slug("install")} data-visits={visits()} data-patched={patched()}>
          Install
        </h2>
        <a href="#install">Install</a>
      </body>
    </html>
  );
}
