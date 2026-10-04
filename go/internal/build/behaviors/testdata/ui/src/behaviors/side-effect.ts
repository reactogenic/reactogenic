// Breaks the rule "no top-level side effects": the query runs on import,
// whatever the page's flags (`sideEffects: false` in package.json hides it
// from a bundler that trusts the annotation).
import "../lib/registers";

const reduce = matchMedia("(prefers-reduced-motion: reduce)");

export default function mountSideEffect(root: HTMLElement): void {
  if (!reduce.matches) root.dataset.animated = "fx:side-effect";
}
