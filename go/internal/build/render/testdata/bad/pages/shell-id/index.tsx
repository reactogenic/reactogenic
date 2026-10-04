import { useShellId } from "@reactogenic/core";

// The counter follows the prefix: the first `d1` and the eleventh `d` would
// both be `d11`.
function Rows() {
  const id = useShellId("d1");
  return <ul id={id} />;
}

export default function ShellIdPage() {
  return (
    <html lang="en">
      <body>
        <Rows />
      </body>
    </html>
  );
}
