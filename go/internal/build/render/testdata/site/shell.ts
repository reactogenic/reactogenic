// A stand-in for `pathname`, `useShellId` and `mount` of @reactogenic/core
// (specs/phase02/plan.md, *The build-time protocol*, RGP2-012): the three on
// top of `globalThis.__reactogenic_build`, so that this package's tests do
// not depend on that package.
import { useId } from "react";

interface Build {
  pathname: string;
  id(prefix: string): string;
  mount(module: string, id: string | undefined, flags: Record<string, boolean> | undefined): void;
}

function build(): Build | undefined {
  return (globalThis as { __reactogenic_build?: Build }).__reactogenic_build;
}

export function pathname(): string {
  return build()?.pathname ?? location.pathname;
}

export function useShellId(prefix?: string): string {
  const b = build();
  return b ? b.id(prefix ?? "") : useId();
}

export function mount(module: string, id?: string, flags?: Record<string, boolean>): void {
  build()?.mount(module, id, flags);
}
