import { use } from "react";

const items = Promise.resolve(["a", "b"]);

function Feed() {
  return <p>{use(items).length}</p>;
}

export default function UsePage() {
  return (
    <html lang="en">
      <body>
        <Feed />
      </body>
    </html>
  );
}
