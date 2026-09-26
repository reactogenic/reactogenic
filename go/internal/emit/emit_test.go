package emit

import (
	"encoding/json"
	"strings"
	"testing"
)

func span(text, sub string) Span {
	i := strings.Index(text, sub)
	if i < 0 {
		panic(sub + " not in " + text)
	}
	return Span{i, i + len(sub)}
}

func TestApplyIdentity(t *testing.T) {
	in := "const a = <Input disabled />;\n"
	out, m, err := Apply(in, nil)
	if err != nil || out != in {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(m.Segments) != 1 || m.Segments[0] != (Segment{Out: Span{0, len(in)}, Copied: true, In: Span{0, len(in)}}) {
		t.Errorf("want one identity segment, got %+v", m.Segments)
	}
}

func TestApplyEdits(t *testing.T) {
	in := "<Input value />"
	attr := span(in, "value")
	out, m, err := Apply(in, []Edit{{
		Span:   Span{attr.End, attr.End}, // insert after `value`
		Pieces: []Piece{Synth("={", attr), Copy(in, attr), Synth("}", attr)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<Input value={value} />"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	// The copied `value` inside the braces maps exactly to the attribute.
	inner := Span{strings.Index(out, "{value}") + 1, strings.Index(out, "{value}") + 6}
	if got := m.Source(inner); got != attr {
		t.Errorf("copied value: Source = %v, want %v", got, attr)
	}
	// A synthesized brace maps to its origin.
	brace := Span{strings.Index(out, "}"), strings.Index(out, "}") + 1}
	if got := m.Source(brace); got != attr {
		t.Errorf("brace: Source = %v, want %v", got, attr)
	}
	// Untouched text maps exactly.
	if got := m.Source(span(out, "Input")); got != span(in, "Input") {
		t.Errorf("tag: Source = %v", got)
	}
}

func TestApplyOrderAndErrors(t *testing.T) {
	in := "abc"
	out, _, err := Apply(in, []Edit{
		{Span: Span{3, 3}, Pieces: []Piece{Synth("2", Span{})}},
		{Span: Span{1, 2}}, // delete b
		{Span: Span{3, 3}, Pieces: []Piece{Synth("3", Span{})}}, // second insertion at 3
		{Span: Span{0, 0}, Pieces: []Piece{Synth("1", Span{})}},
	})
	if err != nil || out != "1ac23" {
		t.Errorf("got %q, %v", out, err)
	}
	if _, _, err := Apply(in, []Edit{{Span: Span{0, 2}}, {Span: Span{1, 3}}}); err == nil {
		t.Error("want an error for overlapping edits")
	}
	if _, _, err := Apply(in, []Edit{{Span: Span{0, 0}, Pieces: []Piece{{Text: "x", Copied: true, From: Span{0, 1}}}}}); err == nil {
		t.Error("want an error for a copied piece that does not match")
	}
}

// Two passes: the composed map sends the final output straight to the source.
func TestThen(t *testing.T) {
	src := "<A x />"
	x := span(src, "x")
	mid, m1, err := Apply(src, []Edit{{Span: Span{x.End, x.End}, Pieces: []Piece{Synth("={x}", x)}}})
	if err != nil {
		t.Fatal(err)
	}
	// Pass 2 moves `x={x}` into a wrapper and keeps the tag.
	attr := span(mid, "x={x}")
	out, m2, err := Apply(mid, []Edit{{Span: attr, Pieces: []Piece{Synth("p={{ ", attr), Copy(mid, attr), Synth(" }}", attr)}}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "<A p={{ x={x} }} />" {
		t.Fatalf("got %q", out)
	}
	m := m2.Then(m1)
	cases := []struct {
		out  string
		want Span
	}{
		{"A", span(src, "A")},          // copied twice
		{"x={x}", x},                   // copied once, then partly synthesized: widens to x
		{"p={{ ", x},                   // synthesized in pass 2, origin mapped through pass 1
		{" />", Span{x.End, len(src)}}, // copied twice
	}
	for _, c := range cases {
		if got := m.Source(span(out, c.out)); got != c.want {
			t.Errorf("Source(%q) = %v, want %v", c.out, got, c.want)
		}
	}
	// Segments still cover the whole output.
	if last := m.Segments[len(m.Segments)-1]; last.Out.End != len(out) || m.Segments[0].Out.Pos != 0 {
		t.Errorf("segments do not cover the output: %+v", m.Segments)
	}
}

// decode reads v3 mappings back into [outLine, outCol, srcLine, srcCol] tuples.
func decode(t *testing.T, mappings string) [][4]int {
	t.Helper()
	var (
		res                   [][4]int
		sLine, sCol, srcIndex int
	)
	for line, group := range strings.Split(mappings, ";") {
		col := 0
		if group == "" {
			continue
		}
		for _, seg := range strings.Split(group, ",") {
			var vals []int
			for i := 0; i < len(seg); {
				v, shift := 0, 0
				for {
					d := strings.IndexByte(base64, seg[i])
					i++
					v |= (d & 31) << shift
					shift += 5
					if d&32 == 0 {
						break
					}
				}
				if v&1 == 1 {
					v = -(v >> 1)
				} else {
					v >>= 1
				}
				vals = append(vals, v)
			}
			col += vals[0]
			srcIndex += vals[1]
			sLine += vals[2]
			sCol += vals[3]
			res = append(res, [4]int{line, col, sLine, sCol})
		}
	}
	_ = srcIndex
	return res
}

func TestSourceMapV3(t *testing.T) {
	src := "const é = <A x />;\nlet y = 1;\n"
	x := span(src, "x")
	out, m, err := Apply(src, []Edit{{Span: Span{x.End, x.End}, Pieces: []Piece{Synth("={x}", x)}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := m.SourceMapV3("a.tsx", "a.rtsx", src, out)
	if err != nil {
		t.Fatal(err)
	}
	var sm struct {
		Version  int
		Sources  []string
		Mappings string
	}
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Fatal(err)
	}
	if sm.Version != 3 || sm.Sources[0] != "a.rtsx" {
		t.Fatalf("bad header: %s", data)
	}
	got := decode(t, sm.Mappings)
	// `é` is one UTF-16 unit, so `x` sits at column 13 on both sides; the
	// synthesized `={x}` (column 14) maps to `x` (13); the rest of the line
	// is copied (column 18 out ← 14 in); line 2 is copied from its start.
	want := [][4]int{{0, 0, 0, 0}, {0, 14, 0, 13}, {0, 18, 0, 14}, {1, 0, 1, 0}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mapping %d = %v, want %v", i, got[i], want[i])
		}
	}
}
