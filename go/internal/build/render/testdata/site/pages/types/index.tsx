// Code that reads an element's `type` — a `Tabs` that picks its `Tab`
// children, a static of the component, its name: the elements are React's,
// and their `type` is the component itself (builder.md, *The record*). The
// record counts what React called, however the element was made: `memo`,
// `forwardRef`, `createElement`, `cloneElement`, a key after a spread.
import { Children, cloneElement, createElement, forwardRef, isValidElement, memo, type ReactNode } from "react";

function Tab({ label }: { label: string }) {
  return <li>{label}</li>;
}
Tab.role = "tab";

function Other() {
  return <li>other</li>;
}

function Tabs({ children }: { children: ReactNode }) {
  const all = Children.toArray(children);
  const tabs = all.filter((child) => isValidElement(child) && child.type === Tab);
  const first = all.find(isValidElement)?.type as typeof Tab;
  return (
    <ul data-tabs={tabs.length} data-role={String(first.role)} data-name={String(first.name)}>
      {tabs}
    </ul>
  );
}

const Badge = memo(function Badge({ text }: { text: string }) {
  return <b>{text}</b>;
});

const Field = forwardRef<HTMLInputElement, { name: string }>(function Field({ name }, _ref) {
  return <input name={name} />;
});

const a = <Tab label="x" />;
const b = <Tab label="y" />;
const spread = { label: "spread" };

export default function TypesPage() {
  return (
    <html lang="en">
      <body>
        <Tabs>
          <Tab label="one" />
          <Other />
          <Tab label="two" />
          {createElement(Tab, { label: "by hand" })}
          {cloneElement(a, { label: "cloned" })}
          <Tab {...spread} key="k" />
        </Tabs>
        <p>{String(a.type === b.type)} {String(a.type === Tab)}</p>
        <Badge text="memo" />
        <Field name="q" />
      </body>
    </html>
  );
}
