package conformance

import (
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

func corpus(t *testing.T) []Case {
	t.Helper()
	fixtures, err := LoadFixtures(fixturesRoot)
	if err != nil {
		t.Fatal(err)
	}
	return append(specCases(t), fixtures...)
}

// ide.md, *Tolerance*: when the source parses clean, tolerant mode hides
// nothing and changes nothing — the text, the diagnostics, the map and
// everything exported beside it.
func TestTolerantEqualsStrict(t *testing.T) {
	for _, c := range corpus(t) {
		in := transpiler.Input{Files: c.Files, Entry: c.Entry, UntilPass: c.UntilPass}
		strict, strictErr := transpiler.Transpile(in)
		if strictErr != nil || strict.Map == nil {
			continue // a syntax error in the source: the modes differ by design
		}
		in.Tolerant = true
		tolerant, err := transpiler.Transpile(in)
		if err != nil || tolerant.Stopped != "" || tolerant.TSX != strict.TSX {
			t.Errorf("%s: tolerant differs from strict (err %v, stopped %q)", c.ID, err, tolerant.Stopped)
			continue
		}
		for name, pair := range map[string][2]any{
			"diagnostics":     {tolerant.Diagnostics, strict.Diagnostics},
			"map":             {tolerant.Map.Segments, strict.Map.Segments},
			"notes":           {tolerant.Notes, strict.Notes},
			"generated names": {tolerant.Generated, strict.Generated},
			"shorthands":      {tolerant.Shorthands, strict.Shorthands},
			"slot groups":     {tolerant.SlotGroups, strict.SlotGroups},
			"dropped":         {tolerant.Dropped, strict.Dropped},
			"unlowered":       {tolerant.Unlowered, strict.Unlowered},
		} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Errorf("%s: %s differ:\n%+v\n%+v", c.ID, name, pair[0], pair[1])
			}
		}
	}
}

// What TS7 checks is TSX: of a source that parses, the output of all the
// passes parses as plain TSX — or it is marked (Output.Unlowered: an error
// left a construct as written), and the reporting layer then drops what TS
// says about the file (ide.md, *Tolerance*, rule 4). An error code that
// leaves its construct in the output without marking it fails here.
func TestOutputIsTSXOrMarked(t *testing.T) {
	marked := 0
	for _, c := range corpus(t) {
		if c.UntilPass != 0 {
			continue
		}
		out, err := transpiler.Transpile(transpiler.Input{Files: c.Files, Entry: c.Entry})
		if err != nil || out.Map == nil {
			continue // a syntax error in the source
		}
		if out.Unlowered {
			marked++
			continue
		}
		if errs := rtsx.ParseTSX("/"+c.Entry+".tsx", out.TSX).Diagnostics(); len(errs) > 0 {
			t.Errorf("%s: the output is not TSX (%s) and not marked:\n%s", c.ID, rtsx.Message(errs[0]), out.TSX)
		}
	}
	if marked < 2 { // fixtures/slots/arg-without-slot, fixtures/parse/params-on-html
		t.Errorf("%d marked outputs: the corpus has no unlowered construct", marked)
	}
}

// ide.md, *Tolerance* (RGP1-104): a file being typed. Over typing-like
// mutants of the corpus — text cut off at a point, one character deleted, a
// sigil or bracket typed — the transform never panics, never hangs, and
// always yields a virtual text with a map the compiler accepts.
func TestTolerantMutants(t *testing.T) {
	// "\n": Enter inside a string attribute leaves valid JSX, not valid JS.
	typed := []string{"<", ">", "{", "}", "&", "#", "$", ".", "=", "/", "\"", "(", "\n"}
	mutants, broken, stopped := 0, 0, map[string]int{}
	for _, c := range corpus(t) {
		src := c.Files[c.Entry]
		stride := max(1, len(src)/100)
		try := func(text string) {
			mutants++
			files := map[string]string{}
			for name, content := range c.Files {
				files[name] = content
			}
			files[c.Entry] = text
			out, err := transpiler.Transpile(transpiler.Input{Files: files, Entry: c.Entry, UntilPass: c.UntilPass, Tolerant: true})
			if err != nil {
				// Only a source that parses clean gets here: a pass failed
				// on it, or wrote text that does not parse. A bug of ours,
				// in strict mode too.
				t.Errorf("%s: %v\n--- source\n%s", c.ID, err, text)
				return
			}
			if out.Map == nil {
				t.Errorf("%s: no map\n--- source\n%s", c.ID, text)
				return
			}
			if out.Stopped != "" {
				stopped[out.Stopped[:strings.Index(out.Stopped, ":")]]++
			}
			syntaxErrors := false
			for _, d := range out.Diagnostics {
				syntaxErrors = syntaxErrors || strings.HasPrefix(d.Code, "TS")
				// A span the reporting layer can turn into a range.
				if d.Span.Pos < 0 || d.Span.End < d.Span.Pos || d.Span.End > len(text) {
					t.Errorf("%s: %s has the span [%d,%d) in %d bytes\n--- source\n%s", c.ID, d.Code, d.Span.Pos, d.Span.End, len(text), text)
					return
				}
			}
			if syntaxErrors {
				broken++
			}
			var tuples [][6]int32
			covered := 0
			for _, s := range out.Map.Spans() {
				if int(s[0]) != covered {
					t.Errorf("%s: gap in the span map at %d\n--- source\n%s", c.ID, covered, text)
					return
				}
				covered = int(s[0] + s[1])
				tuples = append(tuples, s)
			}
			if covered != len(out.TSX) {
				t.Errorf("%s: the span map covers %d of %d bytes\n--- source\n%s", c.ID, covered, len(out.TSX), text)
				return
			}
			if err := rtsx.ValidateSpanMap(rtsx.NewSpanMap(tuples), out.TSX, text); err != nil {
				t.Errorf("%s: %v\n--- source\n%s", c.ID, err, text)
			}
		}
		for i := 0; i <= len(src); i += stride {
			try(src[:i]) // typing, top to bottom
			if i < len(src) {
				try(src[:i] + src[i+1:]) // a deleted character
			}
			try(src[:i] + typed[(i/stride)%len(typed)] + src[i:]) // a typed one
		}
	}
	if mutants < 10000 || broken < mutants/3 {
		t.Errorf("%d mutants, %d with syntax errors: the corpus is not exercised", mutants, broken)
	}
	t.Logf("%d mutants, %d with syntax errors, stopped: %v", mutants, broken, stopped)
}
