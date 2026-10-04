// TypeScript is told an image is a module; the render bundle has no loader
// for one (builder.md, *Not in phase 2*: no asset pipeline).
declare module "*.svg" {
  const url: string;
  export default url;
}
