package transpiler

import (
	"strings"
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

// ide.md, *Span map* (RGP1-102): names are copied, a shorthand's two copies
// are listed, and every tag of a slot's elements is in its group.
func TestEditorExports(t *testing.T) {
	src := `const size = "lg", v = 1;
export const a = (
  <Button size>
    <$Icon>x</$Icon>
    <$Icon className="i" />
  </Button>
);
export const b = <i slot={$Icon} &size &&v />;
`
	out, err := Transpile(Input{Files: map[string]string{"a.rtsx": src}, Entry: "a.rtsx"})
	if err != nil || len(out.Diagnostics) > 0 {
		t.Fatalf("%v %+v", err, out.Diagnostics)
	}
	text := func(s emit.Span) string { return src[s.Pos:s.End] }

	var shorthands []string
	for _, s := range out.Shorthands {
		shorthands = append(shorthands, s.Kind+" "+text(s.Name))
	}
	if got := strings.Join(shorthands, ", "); got != "attr size, arg size, arg-prop v" {
		t.Errorf("shorthands: %s", got)
	}

	if len(out.SlotGroups) != 1 {
		t.Fatalf("slot groups: %+v", out.SlotGroups)
	}
	g := out.SlotGroups[0]
	if g.Name != "$Icon" || len(g.Tags) != 3 || text(g.Owner) != "<Button size>" {
		t.Errorf("group: %+v", g)
	}
	for _, tag := range g.Tags {
		if text(tag) != "$Icon" {
			t.Errorf("group tag %q", text(tag))
		}
	}

	// The prop name `$Icon=` is the first tag's name, copied; the object key
	// `className` is the attribute's name, copied.
	for _, name := range []emit.Span{g.Tags[0], {Pos: strings.Index(src, "className"), End: strings.Index(src, "className") + len("className")}} {
		pos, ok := out.Map.Output(name.Pos)
		if !ok || !strings.HasPrefix(out.TSX[pos:], text(name)) {
			t.Errorf("%q is not copied into the output", text(name))
		}
	}
	// The two copies of the shorthand `size` answer different features.
	var masks []emit.Features
	for _, s := range out.Map.Segments {
		if s.Copied && s.In == out.Shorthands[0].Name {
			masks = append(masks, emit.AllFeatures&^s.Without)
		}
	}
	if len(masks) != 2 || masks[0]&emit.FeatureCompletion == 0 || masks[0]&emit.FeatureRename != 0 || masks[1]&emit.FeatureRename == 0 || masks[1]&emit.FeatureCompletion != 0 {
		t.Errorf("shorthand copies answer %b", masks)
	}
}
