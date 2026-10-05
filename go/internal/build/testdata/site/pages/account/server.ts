// The route's middleware to be: which of the route's artifacts answers a
// request (builder.md, *Routes*). The builder does not read it — every
// variant is built, and a static host serves `index.html` — but it is a
// module of the project, and is type-checked as one.
export default function select(cookie: string | null): "index" | "guest" {
  return cookie?.includes("session=") ? "index" : "guest";
}
