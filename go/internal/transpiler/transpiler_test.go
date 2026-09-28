package transpiler

import (
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/emit"
)

func transpile(t *testing.T, src string) Output {
	t.Helper()
	out, err := Transpile(Input{Files: map[string]string{"input.rtsx": src}, Entry: "input.rtsx"})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// RGP1-030: a file that uses no .rtsx extension passes through byte for
// byte, with an identity map.
func TestPlainTSXPassesThrough(t *testing.T) {
	for _, src := range []string{
		"",
		"export const a = 1;\n",
		"import { useState } from \"react\";\n\n// A comment.\nexport function App<T,>({ items }: { items: T[] }) {\n  const [n, setN] = useState(0); /* é — ✓ */\n  return <ul className=\"list\" {...rest}>{items.map((i) => <li key={String(i)}>{n}</li>)}</ul>;\n}\n",
		"const a = <Input disabled />;\r\nconst b = <></>;\r\n",
	} {
		out := transpile(t, src)
		if len(out.Diagnostics) > 0 {
			t.Errorf("unexpected diagnostics: %+v", out.Diagnostics)
		}
		if out.TSX != src {
			t.Errorf("output differs:\n--- want\n%q\n--- got\n%q", src, out.TSX)
		}
		want := emit.Identity(len(src))
		if len(out.Map.Segments) != len(want.Segments) || len(want.Segments) == 1 && out.Map.Segments[0] != want.Segments[0] {
			t.Errorf("map is not the identity: %+v", out.Map.Segments)
		}
	}
}

func TestSyntaxErrorsAreDiagnostics(t *testing.T) {
	out := transpile(t, "const a = 1;\nconst b = <div>;\n")
	if len(out.Diagnostics) == 0 {
		t.Fatal("want a diagnostic")
	}
	d := out.Diagnostics[0]
	if d.Line != 2 || d.Severity != Error || len(d.Code) < 3 || d.Code[:2] != "TS" {
		t.Errorf("got %+v", d)
	}
	if out.TSX != "" {
		t.Error("want no output for a file that does not parse")
	}
}

// Pass 0 errors come out with source positions, and output is still produced.
func TestCheckErrorsAreDiagnostics(t *testing.T) {
	src := "export const x = (\n  <div { size }>body</div>\n);\n"
	out := transpile(t, src)
	if len(out.Diagnostics) != 1 {
		t.Fatalf("want 1 diagnostic, got %+v", out.Diagnostics)
	}
	if d := out.Diagnostics[0]; d.Line != 2 || d.Col != 8 || d.Code != "params-on-html" {
		t.Errorf("got %+v", d)
	}
}

func TestMissingEntry(t *testing.T) {
	if _, err := Transpile(Input{Files: map[string]string{}, Entry: "input.rtsx"}); err == nil {
		t.Error("want an error")
	}
}

// segment-self: a segment that mounts itself through another segment.
func TestSegmentSelf(t *testing.T) {
	files := map[string]string{
		"page.rtsx":   "export default function Page() {\n  return <main #header />;\n}\n",
		"header.rtsx": "export default function Header() {\n  return <nav #page />;\n}\n",
	}
	out, err := Transpile(Input{Files: files, Entry: "page.rtsx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Diagnostics) != 1 || out.Diagnostics[0].Code != "segment-self" || out.Diagnostics[0].Line != 2 || out.Diagnostics[0].Col != 16 {
		t.Errorf("got %+v", out.Diagnostics)
	}
	// Direct: a segment that mounts itself.
	out, _ = Transpile(Input{Files: map[string]string{"a.rtsx": "export default () => <div #a />;\n"}, Entry: "a.rtsx"})
	if len(out.Diagnostics) != 1 || out.Diagnostics[0].Code != "segment-self" {
		t.Errorf("direct: got %+v", out.Diagnostics)
	}
}

// Segments found through Input.ReadFile, not only Files.
func TestReadFile(t *testing.T) {
	read := func(p string) (string, bool) {
		if p == "intro.rtsx" {
			return "export default () => null;\n", true
		}
		return "", false
	}
	out, err := Transpile(Input{Files: map[string]string{"input.rtsx": "export const a = <section #intro />;\n"}, Entry: "input.rtsx", ReadFile: read})
	if err != nil || len(out.Diagnostics) != 0 {
		t.Errorf("got %v %+v", err, out.Diagnostics)
	}
}
