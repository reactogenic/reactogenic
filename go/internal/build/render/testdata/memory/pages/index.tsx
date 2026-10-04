// A loop that ends, some day — and keeps what it makes.
function rows(): number {
  const kept: number[][] = [];
  for (let i = 0; i < 1e9; i++) kept.push(new Array<number>(1000).fill(i));
  return kept.length;
}

export default function HogPage() {
  return (
    <html lang="en">
      <body>{rows()}</body>
    </html>
  );
}
