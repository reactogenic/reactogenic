// The builder's JSX runtime (specs/phase02/builder.md, *The record*): what
// `react/jsx-runtime` and `react/jsx-dev-runtime` are in the render bundle,
// for every module of it, and the `jsxImportSource` of the files esbuild
// compiles. The elements are React's own, untouched — `element.type` is the
// component. It adds the record — which function components React called,
// how often — and shell rules S1: a host element takes no function, and
// nothing calls React's state, effects or refs (react.js).
import { cloneElement as reactCloneElement, createElement as reactCreateElement } from "reactogenic:real/react";
import { Fragment, jsx as reactJsx, jsxs as reactJsxs } from "reactogenic:real/react/jsx-runtime";

export { Fragment };

let counts = {}; // the page's components, by name
let current = null; // the component being called: a frame
let thrown = null; // { error, frame }: where the exception being thrown left a component
let last = null; // the component called last

// Where each element of a component was made, by its props — the object
// React hands to the component when it calls it: { source, stack, page,
// owner }. The element itself is not touched.
const made = new WeakMap();

// The page's record starts.
export function start() {
  counts = {};
  current = null;
  thrown = null;
  last = null;
}

export function components() {
  return counts;
}

// The components an exception passed through, innermost first, each with
// where its element was written — `source`: esbuild's, in the coordinates of
// the module's text; without one, `stack`: the engine's, where the element
// was made — the owners, as React's own stacks have them: the component that
// wrote `<Dialog>`, not the one that placed it.
//
// An exception that no component threw is React's own, about what a
// component returned — after it returned: `after` then, and the stack is
// that of the component called last, the nearest thing known.
export function owners(error) {
  const within = thrown !== null && thrown.error === error;
  const stack = [];
  for (let frame = within ? thrown.frame : last; frame != null; frame = frame.owner) {
    stack.push({ name: frame.name, page: frame.page, stack: frame.stack, ...frame.source });
  }
  return { owners: stack, after: !within && stack.length > 0 };
}

function coded(name, message) {
  const error = new Error(message);
  error.name = name;
  return error;
}

// shell-handler, when the element is made.
function check(type, props) {
  if (typeof type !== "string" || props == null) return;
  for (const name in props) {
    if (name !== "children" && typeof props[name] === "function") {
      throw coded("shell-handler", "The shell cannot handle events: `" + name + "` on `<" + type + ">`");
    }
  }
}

function note(element, source, stack) {
  const type = element.type;
  if (typeof type === "function" || (typeof type === "object" && type !== null)) {
    made.set(element.props, { source, stack, owner: current });
  }
  return element;
}

// React's static renderer calls every function component through this (the
// one change the builder makes to it: bundle.go, hooked) — also the function
// of a `memo` and of a `forwardRef`, and a component whose element some
// package made. A component is counted when React calls it, so an element
// that is never rendered is not in the record. A class is not called: not
// counted.
export function call(type, props, secondArg) {
  const element = made.get(props);
  const frame = { name: type.displayName || type.name || "Anonymous", ...element };
  counts[frame.name] = (counts[frame.name] || 0) + 1;
  const outer = current;
  current = last = frame;
  try {
    return type(props, secondArg);
  } catch (error) {
    if (thrown === null || thrown.error !== error) thrown = { error, frame };
    throw error;
  } finally {
    current = outer;
  }
}

// The page's own element: the root of every component stack.
export function root(page) {
  const element = reactJsx(page, {});
  made.set(element.props, { page: true, owner: null });
  return element;
}

export function jsx(type, props, key) {
  check(type, props);
  return note(reactJsx(type, props, key));
}

export function jsxs(type, props, key) {
  check(type, props);
  return note(reactJsxs(type, props, key));
}

// What esbuild emits with `jsxDev`: React's production runtime does the work;
// `source` is all that is kept of the development call.
export function jsxDEV(type, props, key, isStatic, source) {
  check(type, props);
  return note((isStatic ? reactJsxs : reactJsx)(type, props, key), source);
}

// What esbuild makes of `<Row {...props} key="k" />` — a key after a spread:
// `createElement` of the import source, and no `source` with it. The engine's
// stack says where the element was written.
export function createElement(type, props, ...children) {
  check(type, props);
  return note(reactCreateElement(type, props, ...children), undefined, typeof type === "string" ? undefined : new Error().stack);
}

// `React.createElement`, by hand or in a package compiled the old way
// (react.js). Where it was written is not known: a stack costs its depth,
// and such a package makes every element this way.
export function createElementByHand(type, props, ...children) {
  check(type, props);
  return note(reactCreateElement(type, props, ...children));
}

// A clone was written where its original was.
export function cloneElement(element, props, ...children) {
  const clone = reactCloneElement(element, props, ...children);
  check(clone.type, clone.props);
  const original = made.get(element.props);
  if (original !== undefined) made.set(clone.props, original);
  return clone;
}

// shell-react: React's state, effects and refs throw when shell code calls
// them, as the clock does — however the hook was reached (react.js).
export function stateful(name) {
  return function () {
    throw coded("shell-react", "The shell cannot use React state or effects: `" + name + "`");
  };
}
