import { Component, createContext, lazy, use, useContext, type ReactNode } from "react";

// Context is render-time React, by either name; a `lazy` component that is
// made and not rendered is nothing.
const Theme = createContext("light");
export const Unused = lazy(() => import("../../clock.tsx"));

function Label() {
  return (
    <p>
      {use(Theme)} {useContext(Theme)}
    </p>
  );
}

// A class that only renders is a component like any other — its state a
// constant — and an error boundary catches nothing in a static render: what
// its content throws ends the page.
class Frame extends Component<{ children: ReactNode }, { tone: string }> {
  state = { tone: "plain" };
  static getDerivedStateFromError() {
    return { tone: "failed" };
  }
  render() {
    return <section data-tone={this.state.tone}>{this.props.children}</section>;
  }
}

export default function ContextPage() {
  return (
    <html lang="en">
      <body>
        <Theme value="dark">
          <Frame>
            <Label />
          </Frame>
        </Theme>
      </body>
    </html>
  );
}
