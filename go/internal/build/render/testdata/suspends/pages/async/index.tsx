async function Feed() {
  const items = await Promise.resolve(["a", "b"]);
  return (
    <ul>
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  );
}

export default function AsyncPage() {
  return (
    <html lang="en">
      <body>
        <Feed />
      </body>
    </html>
  );
}
