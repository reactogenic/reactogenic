// The builder's JSX runtime (specs/phase02/builder.md, *The record*): the
// `jsxImportSource` of the render bundle, and of nothing else. It wraps
// React's own runtime, so the elements are React's; it adds the record —
// which function components rendered, how often — and shell rule S1: a host
// element takes no function.
import { createElement as reactCreateElement } from "react";
import { Fragment, jsx as reactJsx, jsxs as reactJsxs } from "react/jsx-runtime";

export { Fragment };

let counts = {}; // the page's components, by name
let current = null; // the component being called: { name, source, owner }
let thrown = null; // { error, frame }: where the exception being thrown left a component
let last = null; // the component called last

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
// where its element was written (`source`: esbuild's, in the coordinates of
// the module's text) — the owners, as React's own stacks have them: the
// component that wrote `<Dialog>`, not the one that placed it.
//
// An exception that no component threw is React's own, about what a
// component returned — after it returned: `after` then, and the stack is
// that of the component called last, the nearest thing known.
export function owners(error) {
  const within = thrown !== null && thrown.error === error;
  const stack = [];
  for (let frame = within ? thrown.frame : last; frame !== null; frame = frame.owner) {
    stack.push({ name: frame.name, ...frame.source });
  }
  return { owners: stack, after: !within && stack.length > 0 };
}

function check(type, props) {
  if (typeof type !== "string" || props == null) return;
  for (const name in props) {
    if (name !== "children" && typeof props[name] === "function") {
      const error = new Error("The shell cannot handle events: `" + name + "` on `<" + type + ">`");
      error.name = "shell-handler";
      throw error;
    }
  }
}

// A function component is called through a function of its own, per element:
// React sees an ordinary component — hooks and context work — and the call is
// counted when React makes it, so an element that is never rendered is not in
// the record. Classes, `memo` and `forwardRef` are left alone.
function wrap(type, source) {
  if (typeof type !== "function" || (type.prototype && type.prototype.isReactComponent)) return type;
  const frame = { name: type.displayName || type.name || "Anonymous", source, owner: current };
  return function (props) {
    counts[frame.name] = (counts[frame.name] || 0) + 1;
    const outer = current;
    current = last = frame;
    try {
      return type(props);
    } catch (error) {
      if (thrown === null || thrown.error !== error) thrown = { error, frame };
      throw error;
    } finally {
      current = outer;
    }
  };
}

export function jsx(type, props, key) {
  check(type, props);
  return reactJsx(wrap(type), props, key);
}

export function jsxs(type, props, key) {
  check(type, props);
  return reactJsxs(wrap(type), props, key);
}

// What esbuild emits with `jsxDev`: React's production runtime does the work;
// `source` is all that is kept of the development call.
export function jsxDEV(type, props, key, isStatic, source) {
  check(type, props);
  return (isStatic ? reactJsxs : reactJsx)(wrap(type, source), props, key);
}

// `<div {...props} key="k" />` — a key after a spread — is compiled to
// `createElement` of the import source itself.
export function createElement(type, props, ...children) {
  check(type, props);
  return reactCreateElement(wrap(type), props, ...children);
}
