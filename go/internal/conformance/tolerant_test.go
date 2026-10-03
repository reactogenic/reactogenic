package conformance

import (
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
// nothing and changes nothing.
func TestTolerantEqualsStrict(t *testing.T) {
	for _, c := range corpus(t) {
		in := transpiler.Input{Files: c.Files, Entry: c.Entry, UntilPass: c.UntilPass}
		strict, strictErr := transpiler.Transpile(in)
		if strictErr != nil || strict.Map == nil {
			continue // a syntax error in the source: the modes differ by design
		}
		in.Tolerant = true
		tolerant, err := transpiler.Transpile(in)
		if err != nil || tolerant.Stopped != "" || tolerant.TSX != strict.TSX || len(tolerant.Diagnostics) != len(strict.Diagnostics) {
			t.Errorf("%s: tolerant differs from strict (err %v, stopped %q)", c.ID, err, tolerant.Stopped)
			continue
		}
		for i, d := range strict.Diagnostics {
			if tolerant.Diagnostics[i] != d {
				t.Errorf("%s: diagnostic %d differs: %+v / %+v", c.ID, i, tolerant.Diagnostics[i], d)
			}
		}
	}
}

// ide.md, *Tolerance* (RGP1-104): a file being typed. Over typing-like
// mutants of the corpus — text cut off at a point, one character deleted, a
// sigil or bracket typed — the transform never panics, never hangs, and
// always yields a virtual text with a map the compiler accepts.
func TestTolerantMutants(t *testing.T) {
	typed := []string{"<", ">", "{", "}", "&", "#", "$", ".", "=", "/", "\"", "("}
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
				// The source parsed clean and a pass broke it: a real bug,
				// reported as in strict mode. Mutants reach it rarely.
				if !strings.Contains(err.Error(), "does not parse") {
					t.Errorf("%s: %v\n--- source\n%s", c.ID, err, text)
				}
				return
			}
			if out.Map == nil {
				t.Errorf("%s: no map\n--- source\n%s", c.ID, text)
				return
			}
			if out.Stopped != "" {
				stopped[out.Stopped[:strings.Index(out.Stopped, ":")]]++
			}
			for _, d := range out.Diagnostics {
				if strings.HasPrefix(d.Code, "TS") {
					broken++
					break
				}
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
