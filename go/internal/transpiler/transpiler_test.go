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

// A syntax error's span never runs backwards: a zero-length one (an
// unterminated string, a missing expression at a line end) stays where the
// parser put it, not on the next line's first token.
func TestSyntaxErrorSpans(t *testing.T) {
	for _, c := range []struct {
		src       string
		line, col int
	}{
		{"export function A() {\n  const a = \"abc\n  return <div className=\"x\">hi</div>;\n}\n", 2, 17},
		{"const a = <b>{\n", 1, 15},
	} {
		for _, tolerant := range []bool{false, true} {
			out, err := Transpile(Input{Files: map[string]string{"a.rtsx": c.src}, Entry: "a.rtsx", Tolerant: tolerant})
			if err != nil || len(out.Diagnostics) == 0 {
				t.Fatalf("%q: %v %+v", c.src, err, out.Diagnostics)
			}
			d := out.Diagnostics[0]
			if d.Span.End < d.Span.Pos || d.Span.End > len(c.src) || d.Line != c.line || d.Col != c.col {
				t.Errorf("%q (tolerant %v): %s at %d:%d, span [%d,%d); want %d:%d", c.src, tolerant, d.Code, d.Line, d.Col, d.Span.Pos, d.Span.End, c.line, c.col)
			}
		}
	}
}

// An error on whitespace text — content, not formatting: no line break — is
// on the text itself, not an empty span at the token after it.
func TestErrorOnWhitespaceText(t *testing.T) {
	src := "import { Switch } from \"@reactogenic/core\";\nexport const a = (s: string) => <Switch on={s}> <$Case is=\"a\">A</$Case></Switch>;\n"
	out := transpile(t, src)
	if len(out.Diagnostics) != 1 {
		t.Fatalf("want 1 diagnostic, got %+v", out.Diagnostics)
	}
	space := strings.Index(src, "}> <$Case") + 2
	if d := out.Diagnostics[0]; d.Code != "switch-children" || d.Span != (emit.Span{Pos: space, End: space + 1}) || d.Line != 2 || d.Col != 48 {
		t.Errorf("got %+v, want switch-children on the space at 2:48 [%d,%d)", d, space, space+1)
	}
}

// ide.md, *Span map*, *Attribute strings*: a string attribute that reads the
// same as a JS string is copied where its value moves to a JS position — a
// slot prop, a key, `is=`, an arg — so hover and completion answer inside
// it. One that does not is an atom: the JS literal of its value.
func TestAttributeStringsAreCopied(t *testing.T) {
	src := `import { Switch } from "@reactogenic/core";
export const a = (
  <Table>
    <$Column key="email" title="plain" alt="Tom &amp; Jerry" />
  </Table>
);
export const b = (s: string) => <Switch on={s}><$Case is="loading">A</$Case></Switch>;
export const c = ($Icon: any) => <i slot={$Icon} &size="lg" />;
`
	out := transpile(t, src)
	if len(out.Diagnostics) > 0 {
		t.Fatalf("%+v", out.Diagnostics)
	}
	for _, lit := range []string{`"email"`, `"plain"`, `"loading"`, `"lg"`} {
		in := emit.Span{Pos: strings.Index(src, lit), End: strings.Index(src, lit) + len(lit)}
		copied := false
		for _, s := range out.Map.Segments {
			if s.Copied && s.In.Pos <= in.Pos && in.End <= s.In.End {
				at := s.Out.Pos + in.Pos - s.In.Pos
				copied = out.TSX[at:at+len(lit)] == lit && s.Without&(emit.FeatureHover|emit.FeatureCompletion) == 0
				break
			}
		}
		if !copied {
			t.Errorf("%s is not copied with hover and completion", lit)
		}
	}
	entity := strings.Index(src, `"Tom &amp; Jerry"`)
	if _, ok := out.Map.Output(entity + 1); ok || !strings.Contains(out.TSX, `alt: "Tom & Jerry"`) {
		t.Errorf("a string with a character reference is copied:\n%s", out.TSX)
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

// ide.md, *Tolerance* (RGP1-104): a half-typed file keeps a virtual text the
// editor can work with, and reports only the parser's errors.
func TestTolerant(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"a slot name being typed is a prop name", "const a = <Button><$Ic</Button>;\n", "$Ic={{}}"},
		{"member access keeps its tree", "const a = <Button><$Icon>{iconSize.}</$Icon></Button>;\n", "children: iconSize."},
		{"a sigil without a name", "const a = <span slot={$Icon} &></span>;\n", "_renderSlot($Icon, {})"},
		// Recovery re-parents `<$Icon>` out of `Button`: orphan-slot would be
		// reported on an element the author placed correctly.
		{"no transpiler error under a broken element", "const a = <Button size={><$Icon>x</$Icon></Button>;\n", ""},
		{"nor on a half-typed case", "const a = <Switch on={x}><$Case is=</Switch>;\n", ""},
		// After recovery the parser's flag no longer calls the line breaks
		// around `<$Icn>…</$Icon>` formatting: `$Action` would get
		// `children: ""`, and TS an error that is not the author's.
		{"whitespace after a mismatched closing tag is formatting", "const a = (\n  <Dialog>\n    <$Action>\n      <$Icn>x</$Icon>\n    </$Action>\n  </Dialog>\n);\n", `$Action={{ $Icn: { children: "x" } }}`},
	} {
		out, err := Transpile(Input{Files: map[string]string{"a.rtsx": c.src}, Entry: "a.rtsx", Tolerant: true})
		if err != nil || out.Map == nil || out.Stopped != "" {
			t.Errorf("%s: err %v, stopped %q", c.name, err, out.Stopped)
			continue
		}
		if !strings.Contains(out.TSX, c.want) {
			t.Errorf("%s: output %q lacks %q", c.name, out.TSX, c.want)
		}
		if len(out.Diagnostics) == 0 {
			t.Errorf("%s: no syntax error reported", c.name)
		}
		for _, d := range out.Diagnostics {
			if !strings.HasPrefix(d.Code, "TS") {
				t.Errorf("%s: transpiler diagnostic %s on a half-typed construct", c.name, d.Code)
			}
		}
	}
	// Strict mode is unchanged: a syntax error, and no output.
	out, _ := Transpile(Input{Files: map[string]string{"a.rtsx": "const a = <Button><$Ic</Button>;\n"}, Entry: "a.rtsx"})
	if out.TSX != "" || out.Map != nil || len(out.Diagnostics) == 0 {
		t.Errorf("strict: %+v", out)
	}
}

// An attachment's fallback is emitted twice — as the slot's fallback and as
// the element without the slot — and each copy goes through the later runs.
// What is found there is recorded once.
func TestFallbackRecordedOnce(t *testing.T) {
	out := transpile(t, "export const a = <div><span slot={$Badge}><$Icon /></span></div>;\n")
	if len(out.Diagnostics) != 1 || out.Diagnostics[0].Code != "orphan-slot" {
		t.Errorf("diagnostics: %+v", out.Diagnostics)
	}
	out = transpile(t, "const size = 1;\nexport const a = <span slot={$Badge} &size><Button><$Icon className=\"i\" /></Button></span>;\n")
	if len(out.Diagnostics) != 0 || len(out.SlotGroups) != 1 {
		t.Errorf("diagnostics %+v, slot groups %+v", out.Diagnostics, out.SlotGroups)
	}
	seen := map[Note]bool{}
	for _, n := range out.Notes {
		if seen[n] {
			t.Errorf("note recorded twice: %+v", n)
		}
		seen[n] = true
	}
	if strings.Count(out.TSX, "$Icon={{ className: \"i\" }}") != 2 {
		t.Errorf("the fallback is emitted in both branches:\n%s", out.TSX)
	}
}

// Emitted text that does not parse is reported against the pass that wrote
// it — a repeating pass re-parses its own output.
func TestDoesNotParseNamesThePass(t *testing.T) {
	saved := passes
	defer func() { passes = saved }()
	ran := false
	passes = []pass{
		{0, "checks", func(*passContext) []emit.Edit { return nil }, false},
		{1, "one", func(*passContext) []emit.Edit { return nil }, false},
		{2, "two", func(*passContext) []emit.Edit {
			if ran {
				return nil
			}
			ran = true
			return []emit.Edit{{Span: emit.Span{}, Pieces: []emit.Piece{emit.Synth("(", emit.Span{})}}}
		}, true},
	}
	_, err := Transpile(Input{Files: map[string]string{"a.rtsx": "export const a = 1;\n"}, Entry: "a.rtsx"})
	if err == nil || !strings.Contains(err.Error(), "pass 2 (two) produced code that does not parse") {
		t.Errorf("got %v", err)
	}
}
