// One project, several sites: each directory beside this file is a `--pages`
// root with one thing wrong (build_test.go). They share this document.
import type { ReactNode } from "react";

export function Page({ title, children }: { title: string; children?: ReactNode }) {
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
