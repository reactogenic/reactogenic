// The document of the fixture's pages: the title is the last element of the
// head, so what packaging adds there follows it (`title + style`).
import type { ReactNode } from "react";
import "./site.css";

export function Layout({ title, children }: { title: string; children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <title>{title}</title>
      </head>
      <body>{children}</body>
    </html>
  );
}
