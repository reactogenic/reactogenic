// The builder's JSX runtime (specs/phase02/builder.md, *The record*): what
// `react/jsx-runtime` and `react/jsx-dev-runtime` are in the render bundle,
// for every module of it, and the `jsxImportSource` of the files esbuild
// compiles. The elements are React's own, untouched — `element.type` is the
// component. It adds the record — which function components React called,
// how often — and shell rule S1: a host element takes no function, nothing
// suspends, and nothing calls React's state, effects or refs (react.js).
import { cloneElement as reactCloneElement, createElement as reactCreateElement } from "reactogenic:real/react";
import { Fragment, jsx as reactJsx, jsxs as reactJsxs } from "reactogenic:real/react/jsx-runtime";

export { Fragment };

let counts = {}; // the page's components, by name
let current = null; // the component being called: a frame
let thrown = null; // { error, frame }: where the exception being thrown left a component
let last = null; // the component called last
let first = null; // { error, frame }: the first exception that left a component

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
  first = null;
}

export function components() {
  return counts;
}

// What a component threw when the render ended all the same: something
// swallowed it. React's static renderer does that for a Suspense boundary —
// it renders the fallback and tells nobody — and a boundary is refused when
// its element is made (check); this is for the one that was made past the
// builder's runtime: an element written out as an object, by a package with
// a JSX runtime of its own. The first such exception is the page's error.
export function swallowed() {
  if (first === null) return null;
  thrown = first;
  return first.error;
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

const SUSPENSE = Symbol.for("react.suspense");
const LAZY = Symbol.for("react.lazy");

function suspends(what) {
  return coded("shell-react", "The shell cannot suspend: " + what);
}

// A class component is a component like any other while it only renders:
// its state is a constant, and an error boundary catches nothing in a
// static render. One with an effect is `useEffect` in the older form — the
// static renderer never calls it, and the page is its initial state.
const EFFECTS = ["componentDidMount", "componentDidUpdate", "componentWillUnmount", "getSnapshotBeforeUpdate"];

function effects(type) {
  for (const name of EFFECTS) {
    if (typeof type.prototype[name] === "function") {
      throw coded("shell-react", "The shell cannot use React state or effects: `" + name + "` of `" + (type.displayName || type.name || "Anonymous") + "`");
    }
  }
}

// shell-handler and shell-react's boundary, when the element is made. A
// Suspense boundary renders its fallback for anything its content throws —
// a shell rule among it — so there is none in the shell; a `lazy` component
// has nothing to wait for, and is an error where it is rendered, not where
// it is made: an island's, exported beside the shell's, is nobody's mistake.
function check(type, props) {
  if (type === SUSPENSE) throw suspends("`<Suspense>`");
  if (typeof type === "object" && type !== null && type.$$typeof === LAZY) throw suspends("a `lazy` component");
  if (typeof type === "function" && type.prototype != null && type.prototype.isReactComponent) effects(type);
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
    const result = type(props, secondArg);
    // An `async` component: React would wait for it, and the render is one
    // synchronous pass (builder.md, *Not in phase 2*).
    if (result !== null && typeof result === "object" && typeof result.then === "function") {
      throw suspends("`" + frame.name + "` is an async component");
    }
    return result;
  } catch (error) {
    if (thrown === null || thrown.error !== error) thrown = { error, frame };
    if (first === null) first = thrown;
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

// `use` of a context is render-time React; of a promise, it suspends.
export function usable(use) {
  return function (value) {
    if (value !== null && typeof value === "object" && typeof value.then === "function") {
      throw suspends("`use` of a promise");
    }
    return use(value);
  };
}
