// A stand-in for the builder's engine (specs/phase02/plan.md, *The build-time
// protocol*): renders with React's static renderer while
// `globalThis.__reactogenic_build` answers `pathname()`, `useShellId()`,
// `mount()` and `variants()`, and returns what the builder would record.
import type { MountData, ShellBuild } from "@reactogenic/core";
import { createElement, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";

/** One `mount()` call, as builder.md's record holds it. */
export interface Mount {
  module: string;
  id?: string;
  flags?: Record<string, boolean>;
  /** What the use site hands the behaviour: the second argument of its call. */
  data?: MountData;
}

/** A page being rendered: its pathname, its id counters, its mounts, the classes its variants resolved. */
export class Page {
  readonly mounts: Mount[] = [];
  /** What `variants()` reported: each class once, in the order first resolved (builder.md, *What shell code can ask the builder*). */
  readonly classes: string[] = [];
  readonly #counters = new Map<string, number>();

  constructor(readonly pathname: string) {}

  /** Renders `component` as part of this page; the ids continue where the last render stopped. */
  render(component: ComponentType, pathname = this.pathname): string {
    const build: ShellBuild = {
      pathname,
      id: (prefix) => {
        const n = (this.#counters.get(prefix) ?? 0) + 1;
        this.#counters.set(prefix, n);
        return prefix + n;
      },
      mount: (module, id, flags, data) => {
        this.mounts.push({ module, ...(id === undefined ? {} : { id }), ...(flags === undefined ? {} : { flags }), ...(data === undefined ? {} : { data }) });
      },
      classes: (names) => {
        this.classes.push(...names.filter((name) => !this.classes.includes(name)));
      },
    };
    const host = globalThis as { __reactogenic_build?: ShellBuild };
    host.__reactogenic_build = build;
    try {
      return renderToStaticMarkup(createElement(component));
    } finally {
      delete host.__reactogenic_build;
    }
  }
}

/** The distinct mounts of a list, in first-seen order: what a page's script is made from. */
export function distinct(mounts: Mount[]): Mount[] {
  const seen = new Set<string>();
  return mounts.filter((mount) => {
    const key = JSON.stringify([mount.module, mount.id, mount.flags, mount.data]);
    return !seen.has(key) && seen.add(key);
  });
}

/**
 * HTML in the form the spec prints it — as a parser reads React's output
 * (components.md, *Types*): attribute names in lower case (React prints
 * `popoverTarget`, and HTML's names are case-insensitive), an empty value as
 * a bare attribute (`popover=""`), no comments, no whitespace between tags.
 * Only the comparison is normalised: nothing rewrites the page's markup.
 */
export function normalise(html: string): string {
  return html
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/<[a-zA-Z][^>]*>/g, (tag) =>
      tag.replace(/\s+([a-zA-Z-]+)(="[^"]*")?/g, (_, name: string, value?: string) => ` ${name.toLowerCase()}${value === '=""' ? "" : (value ?? "")}`),
    )
    .replace(/>\s+</g, "><")
    .replace(/\s+/g, " ")
    .trim();
}
