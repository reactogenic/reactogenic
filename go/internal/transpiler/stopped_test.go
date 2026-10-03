package transpiler

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// ide.md, *Tolerance*, step 1: a pass that fails keeps the previous pass's
// text and map, and the result is marked stopped. No file being typed stops
// a pass by itself, so one is made to: the segment pass looks for its
// sibling — the last time the file system is asked — and it panics.
func TestStoppedKeepsPreviousPass(t *testing.T) {
	const src = "const size = 1;\nexport const a = <Button size><section #intro /></Button>;\nexport const b = <;\n"
	calls, failAt := 0, 0
	read := func(p string) (string, bool) {
		if calls++; calls == failAt {
			panic("boom")
		}
		return "", false
	}
	in := Input{Files: map[string]string{"/p/a.rtsx": src}, Entry: "/p/a.rtsx", ReadFile: read, Tolerant: true}
	if out, err := Transpile(in); err != nil || out.Stopped != "" || calls == 0 {
		t.Fatalf("without the panic: %v, stopped %q, %d reads", err, out.Stopped, calls)
	}
	calls, failAt = 0, calls
	out, err := Transpile(in)
	if err != nil || out.Map == nil {
		t.Fatalf("Transpile: %v, map %v", err, out.Map)
	}
	if !strings.HasPrefix(out.Stopped, "pass 4 ") || !strings.Contains(out.Stopped, "boom") {
		t.Errorf("stopped: %q; want pass 4, and its reason", out.Stopped)
	}
	// Pass 1 lowered the shorthand; the segment root is as written.
	if !strings.Contains(out.TSX, "<Button size={size}>") || !strings.Contains(out.TSX, "<section #intro />") {
		t.Errorf("the text is not the one before the pass that failed:\n%s", out.TSX)
	}
	var tuples [][6]int32
	for _, s := range out.Map.Spans() {
		tuples = append(tuples, s)
	}
	if err := rtsx.ValidateSpanMap(rtsx.NewSpanMap(tuples), out.TSX, src); err != nil {
		t.Errorf("the map of the text that was kept: %v", err)
	}
	// The source's own syntax error is still reported.
	found := false
	for _, d := range out.Diagnostics {
		found = found || d.Line == 3
	}
	if !found {
		t.Errorf("the syntax error on line 3 is gone: %+v", out.Diagnostics)
	}
}
