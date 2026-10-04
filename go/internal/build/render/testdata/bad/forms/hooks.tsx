// Not a page of the fixture: TestShellReact renders it at a pathname per
// form — every way to reach a hook is the hook.
import React from "react";
import { R, useFx, useState } from "../hooks.ts";

const { useRef } = React;
const form = () => (globalThis as unknown as { __reactogenic_build: { pathname: string } }).__reactogenic_build.pathname;

function Form() {
  switch (form()) {
    case "/reexported/":
      return <b>{useState(1)[0]}</b>;
    case "/renamed/":
      useFx(() => {}, []);
      return null;
    case "/destructured/":
      return <b>{String(useRef(null).current)}</b>;
    case "/namespace/":
      return <b>{R.useReducer((n: number) => n, 2)[0]}</b>;
    case "/computed/":
      return <b>{React["useState"](3)[0]}</b>;
    case "/layout-effect/":
      React.useLayoutEffect(() => {});
      return null;
  }
  return <b>{React.useId().length > 0 ? "useId is not state" : ""}</b>;
}

export default function HooksPage() {
  return (
    <html lang="en">
      <body>
        <Form />
      </body>
    </html>
  );
}
