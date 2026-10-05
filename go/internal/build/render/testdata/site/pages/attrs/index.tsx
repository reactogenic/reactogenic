// The 36 cases of specs/phase02/research/evaluation.md, *Variant cases* —
// ordinary design-system markup that a hand-written serializer got wrong on
// 19 of them — and a `<select defaultValue>`. React's own renderer in the
// engine must give what React gives in Node (differential_test.go). A page
// in plain .tsx, too.
import type { CSSProperties } from "react";

const open = true;
const closed = false;
const n = 0.1 + 0.2;
const big = 1e21;
const emoji = "a😀b";
const label = "Tom & \"Jerry\" <b>";

export default function AttrsPage() {
  return (
    <html lang="en">
      <head>
        <title>Attributes</title>
      </head>
      <body>
        <button aria-expanded={closed} aria-controls="m">menu</button>
        <div aria-hidden={open}>x</div>
        <dialog aria-modal={open} open>x</dialog>
        <div data-open={open} data-closed={closed}>x</div>
        <div hidden inert>x</div>
        <button disabled={closed} tabIndex={0}>x</button>
        <label htmlFor="q">Search</label>
        <input id="q" type="text" defaultValue="hi" readOnly maxLength={10} autoComplete="off" spellCheck={false} />
        <input type="checkbox" defaultChecked />
        <textarea defaultValue="text" rows={3} />
        <div style={{ lineHeight: 1.5, zIndex: 10, gridRow: 2, WebkitLineClamp: 3, marginTop: 8, "--rg-gap": 4, width: "50%" } as CSSProperties}>x</div>
        <div style={{ fontSize: 14.5, opacity: 0, margin: 0, aspectRatio: 2, columnCount: 3, flexBasis: 10 }}>x</div>
        <a href="/f.zip" download>dl</a>
        <img src="/a.png" alt="" srcSet="/a2.png 2x" crossOrigin="anonymous" />
        <svg viewBox="0 0 10 10" strokeWidth={2} fillRule="evenodd"><path d="M0 0" strokeLinecap="round" /></svg>
        <table><tbody><tr><td colSpan={2} rowSpan={1}>x</td></tr></tbody></table>
        <p>{n} {big} {-0} {1 / 3} {100}</p>
        <p>{emoji.length} {emoji.slice(1, 3) === "😀" ? "utf16" : "runes"}</p>
        <p title={label}>{label}</p>
        <p>&copy; 2026 &mdash; Reactogenic&nbsp;docs &rarr; &hellip;</p>
        <p>{"a"}{"b"}{1}{null}{false}{true}{undefined}c</p>
        <div dangerouslySetInnerHTML={{ __html: "<b>raw</b>" }} />
        <div contentEditable draggable suppressHydrationWarning>x</div>
        <meta charSet="utf-8" />
        <ul>{[3, 1, 2].map((x) => <li key={x}>{x}</li>)}</ul>
        <p className={["a", closed && "b", "c"].filter(Boolean).join(" ")}>x</p>
        <p>{["b", "a", "C"].sort().join("")} {[10, 9, 1].sort().join(",")}</p>
        <p>{"Flow Control & More".toLowerCase().replace(/[^a-z0-9]+/g, "-")}</p>
        <p>{Object.keys({ b: 1, 2: 1, a: 1, 1: 1 }).join(",")}</p>
        <p>{(1234.5678).toFixed(2)} {String(42)} {Number("7") + 1} {parseInt("12px", 10)}</p>
        <p>{JSON.stringify({ a: 1, b: [true, null] })}</p>
        <button popoverTarget="m" popoverTargetAction="toggle">x</button>
        <details open name="acc"><summary>s</summary>d</details>
        <video autoPlay muted loop playsInline controls />
        <option value="a" selected>a</option>
        <select defaultValue="b"><option value="a">a</option><option value="b">b</option></select>
        <form noValidate action="/s"><input name="q" required /></form>
      </body>
    </html>
  );
}
