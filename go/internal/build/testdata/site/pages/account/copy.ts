// A module of the route's variants: not `.rtsx`, so never a variant.
export function greeting(name?: string): string {
  return name ? `Welcome back, ${name}` : "Welcome";
}
