// `react`, as every module of the render bundle but React's own imports it
// (specs/phase02/builder.md, *Shell code in phase 2*): the project's React,
// with the elements made by hand going through the builder's runtime — the
// shell rule and the record do not depend on who made an element — and with
// state, effects and refs refused: shell-react is the hook, however it was
// reached (re-exported, through a namespace, from a compiled package) — and
// `use` of a promise, which suspends.
import React from "reactogenic:real/react";
import { cloneElement, createElementByHand as createElement, stateful, usable } from "reactogenic:jsx";

export * from "reactogenic:real/react";
export { cloneElement, createElement };

export const useState = stateful("useState");
export const useReducer = stateful("useReducer");
export const useEffect = stateful("useEffect");
export const useLayoutEffect = stateful("useLayoutEffect");
export const useInsertionEffect = stateful("useInsertionEffect");
export const useRef = stateful("useRef");
export const useImperativeHandle = stateful("useImperativeHandle");
export const useSyncExternalStore = stateful("useSyncExternalStore");
export const useTransition = stateful("useTransition");
export const useDeferredValue = stateful("useDeferredValue");
export const useOptimistic = stateful("useOptimistic");
export const useActionState = stateful("useActionState");
export const use = usable(React.use);

export default {
  ...React,
  cloneElement,
  createElement,
  useState,
  useReducer,
  useEffect,
  useLayoutEffect,
  useInsertionEffect,
  useRef,
  useImperativeHandle,
  useSyncExternalStore,
  useTransition,
  useDeferredValue,
  useOptimistic,
  useActionState,
  use,
};
