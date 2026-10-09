import { Button, loaded, Toolbar } from "../button";

export default function Page() {
  return (
    <html lang="en">
      <head />
      <body data-loaded={loaded}>
        <Button size="sm">Small</Button>
        <Button size="md">Medium</Button>
        <Toolbar />
        <Button look="ghost" size="sm">Ghost</Button>
        <p className="literal">A class that is written, not resolved.</p>
      </body>
    </html>
  );
}
