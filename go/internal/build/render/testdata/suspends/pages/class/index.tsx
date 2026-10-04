import { Component } from "react";

// A class with an effect: its initial state is rendered, and
// componentDidMount never runs — the hook's mistake, in the older form.
class Ticker extends Component<{ label: string }, { ticks: number }> {
  state = { ticks: 0 };
  componentDidMount() {
    this.setState({ ticks: 1 });
  }
  render() {
    return (
      <p>
        {this.props.label} {this.state.ticks}
      </p>
    );
  }
}

export default function ClassPage() {
  return (
    <html lang="en">
      <body>
        <Ticker label="ticks" />
      </body>
    </html>
  );
}
