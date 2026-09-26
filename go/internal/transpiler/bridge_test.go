package transpiler

import (
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// The bridge reaches tsgo's parser. Slot params are a syntax error in plain
// TSX; `#name` is not: TypeScript's parser (5.x and tsgo) reads it as an
// attribute named "#about-us". esbuild and Babel reject it. See the Segment
// roots grammar in specs/phase01/syntax.md.
func TestBridgeParsesTSX(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		wantError bool
	}{
		{"plain TSX", `const a = <Button size="lg" {...rest} />;`, false},
		{"slot params", `const a = <$Icon { size } />;`, true},
		{"segment root", `const a = <section #about-us />;`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := rtsx.ParseTSX("/test.tsx", c.text)
			if got := len(file.Diagnostics()) > 0; got != c.wantError {
				t.Fatalf("syntax error = %v, want %v: %v", got, c.wantError, file.Diagnostics())
			}
		})
	}
}
