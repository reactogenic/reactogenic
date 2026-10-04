// The engine has no Intl: what would be formatted without it is refused, not
// rendered unformatted.
export default function LocalePage() {
  return (
    <html lang="en">
      <body>{(1234567.891).toLocaleString("en-US")}</body>
    </html>
  );
}
