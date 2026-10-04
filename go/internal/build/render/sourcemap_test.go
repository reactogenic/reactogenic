package render

import (
	"slices"
	"testing"
)

func TestDecodeVLQ(t *testing.T) {
	for _, c := range []struct {
		segment string
		want    []int
	}{
		{"AAAA", []int{0, 0, 0, 0}},
		{"C", []int{1}},
		{"D", []int{-1}},
		{"gB", []int{16}}, // a continuation digit
		{"hB", []int{-16}},
		{"+/////D", []int{2147483647}},
		{"IAAM", []int{4, 0, 0, 6}},
		{"SAAU,", nil}, // not base64
		{"g", nil},     // a continuation with nothing after it
		{"", nil},
	} {
		got, err := decodeVLQ(c.segment)
		if (err != nil) != (c.want == nil) || !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, %v; want %v", c.segment, got, err, c.want)
		}
	}
}

// Two sources; line 1 of the bundle has no mapping; line 3 has a segment
// without a source between two with one.
func TestSourceMapLookup(t *testing.T) {
	m, err := parseSourceMap([]byte(`{"version":3,"sources":["a.ts","b.rtsx"],"mappings":";AAAA,IAAM;EACJ,E,GCEE"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		line, column int // of the bundle, 1-based
		source       string
		srcLine      int // 0-based
		srcColumn    int
		ok           bool
	}{
		{1, 1, "", 0, 0, false}, // a line of the bundle's own
		{2, 1, "a.ts", 0, 0, true},
		{2, 4, "a.ts", 0, 0, true},  // inside the first segment
		{2, 5, "a.ts", 0, 6, true},  // the second starts here
		{2, 90, "a.ts", 0, 6, true}, // …and runs to the end of the line
		{3, 2, "", 0, 0, false},     // before the line's first segment
		{3, 3, "a.ts", 1, 2, true},
		{3, 6, "a.ts", 1, 2, true},   // the sourceless segment is not a place: the one before it
		{3, 8, "b.rtsx", 3, 4, true}, // source, line and column are relative to the last segment that had them
		{4, 1, "", 0, 0, false},      // past the end
		{0, 1, "", 0, 0, false},
	} {
		source, srcLine, srcColumn, ok := m.lookup(c.line, c.column)
		if source != c.source || srcLine != c.srcLine || srcColumn != c.srcColumn || ok != c.ok {
			t.Errorf("%d:%d: %s %d:%d %v; want %s %d:%d %v", c.line, c.column, source, srcLine, srcColumn, ok, c.source, c.srcLine, c.srcColumn, c.ok)
		}
	}
	for _, bad := range []string{`{"version":2,"sources":[],"mappings":""}`, `{"version":3,"sources":["a"],"mappings":"ACAA"}`, `{"version":3,"sources":[],"mappings":"A!"}`, `[`} {
		if _, err := parseSourceMap([]byte(bad)); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

// A source map's column counts UTF-16 code units; the program's texts are
// UTF-8.
func TestOffsetAt(t *testing.T) {
	text := "ab\n<p title=\"😀é\">x</p>\nlast"
	for _, c := range []struct {
		line, column int
		want         string // the text from the offset on, to the end of its line
	}{
		{0, 0, "ab"},
		{0, 2, ""},
		{0, 9, ""}, // clamped to the line
		{1, 10, "😀é\">x</p>"},
		{1, 12, "é\">x</p>"}, // 😀 is two units
		{1, 13, "\">x</p>"},  // é is one, and two bytes
		{2, 1, "ast"},
		{9, 0, ""},
	} {
		rest := text[offsetAt(text, c.line, c.column):]
		if i := indexNewline(rest); i >= 0 {
			rest = rest[:i]
		}
		if rest != c.want {
			t.Errorf("%d:%d: %q; want %q", c.line, c.column, rest, c.want)
		}
	}
}

func indexNewline(s string) int {
	for i := range len(s) {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}
