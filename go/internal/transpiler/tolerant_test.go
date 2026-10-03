package transpiler

import (
	"fmt"
	"strings"
	"testing"
)

// tolerantCodes transpiles files[entry] in tolerant mode and returns the
// transpiler's own diagnostics — not the parser's — as "line:col code", and
// the output. The source must have a syntax error.
func tolerantCodes(t *testing.T, files map[string]string, entry string) (string, Output) {
	t.Helper()
	out, err := Transpile(Input{Files: files, Entry: entry, Tolerant: true})
	if err != nil || out.Map == nil {
		t.Fatalf("%q: err %v", files[entry], err)
	}
	var codes []string
	syntaxErrors := 0
	for _, d := range out.Diagnostics {
		if strings.HasPrefix(d.Code, "TS") {
			syntaxErrors++
			continue
		}
		codes = append(codes, fmt.Sprintf("%d:%d %s", d.Line, d.Col, d.Code))
	}
	if syntaxErrors == 0 {
		t.Fatalf("%q: no syntax error — the case tests nothing", files[entry])
	}
	return strings.Join(codes, ", "), out
}

const tolerantHead = "import { Switch, Match } from \"@reactogenic/core\";\nexport function A({ s, ok }: { s: any; ok: boolean }) {\n"

// inReturn is a component that returns jsx, on lines 4….
func inReturn(jsx string) string {
	return tolerantHead + "  return (\n" + jsx + "\n  );\n}\n"
}

// ide.md, *Tolerance*, rule 3, in both directions: a transpiler diagnostic is
// dropped under a broken element — an unclosed tag leaves no flag in the
// tree, only the parser's error range — and kept next to a syntax error that
// is somewhere else.
func TestTolerantRule3(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		// Dropped.
		{"valid cases after an unclosed one (TS17008)", inReturn("    <Switch on={s}>\n      <$Case is=\"x\">\n      <$Case is=\"a\">A</$Case>\n      <$Case is=\"b\">B</$Case>\n    </Switch>"), ""},
		{"a slot after an unclosed Match", inReturn("    <Button>\n      <Match on={ok}>\n        <$Icon className=\"i\" />\n      text\n    </Button>"), ""},
		{"a slot after an unclosed div", inReturn("    <Button>\n      <div>\n      <$Icon className=\"i\" />\n    </Button>"), ""},
		{"`<` typed before the cases", inReturn("    <Switch on={s}>\n      <\n      <$Case is=\"a\">A</$Case>\n      <$Case is=\"b\">B</$Case>\n    </Switch>"), ""},
		{"`<$` typed before the cases", inReturn("    <Switch on={s}>\n      <$\n      <$Case is=\"a\">A</$Case>\n      <$Case is=\"b\">B</$Case>\n    </Switch>"), ""},
		// Kept: the syntax error is in another statement or another function.
		{"a root element, error in the statement before", tolerantHead + "  const y = s.;\n  return (<Switch><$Case is=\"a\">A</$Case></Switch>);\n}\n", "4:11 flow-no-subject"},
		{"the same under a div", tolerantHead + "  const y = s.;\n  return (<div><Switch><$Case is=\"a\">A</$Case></Switch></div>);\n}\n", "4:16 flow-no-subject"},
		{"a root element, error in another function", "export function B() {\n  const y = ;\n}\n" + inReturn("    <Switch><$Case is=\"a\">A</$Case></Switch>"), "7:5 flow-no-subject"},
		{"an orphan at the top level", "export const a = <$Orphan />;\nexport function B() {\n  const y = ;\n}\n", "1:18 orphan-slot"},
	} {
		if got, _ := tolerantCodes(t, map[string]string{"a.rtsx": c.src}, "a.rtsx"); got != c.want {
			t.Errorf("%s: transpiler diagnostics %q, want %q\n%s", c.name, got, c.want, c.src)
		}
	}
	// A file-level diagnostic has no node that could be broken — with or
	// without a comment on the first line.
	for _, src := range []string{"export const a = 1;\nexport const b = ;\n", "// comment first\nexport const a = 1;\nexport const b = ;\n"} {
		if got, _ := tolerantCodes(t, map[string]string{"input.rtsx": src, "input.tsx": ""}, "input.rtsx"); got != "1:1 ambiguous-module" {
			t.Errorf("%q: transpiler diagnostics %q, want ambiguous-module", src, got)
		}
	}
}

// ide.md, *Tolerance*: a `$Case` being typed does not take its `Switch` out
// of the virtual text — the other cases, the subject and what is typed so
// far stay, so hover and completion keep working and nothing reads as
// unused. Code that had to be left out marks the file (Output.Dropped).
func TestTolerantSwitchKept(t *testing.T) {
	for _, c := range []struct {
		typed   string
		want    string // in the output, besides the first case
		dropped bool
	}{
		{"<$Case is={Status.}", "s === Status.", false},
		{"<$Case is={Status.", "s === Status.", false},
		{"<$Case", "", false},
		{"<$", "", false},
		{"<", "", false},
		{"<$Case is=", "", false},
		{"<$Case is={", "", false},
		{"<$Case is=>{two}</$Case>", "", true}, // its body is left out
	} {
		src := inReturn("    <Switch on={s}>\n      <$Case is=\"a\">{one}</$Case>\n      " + c.typed + "\n    </Switch>")
		codes, out := tolerantCodes(t, map[string]string{"a.rtsx": src}, "a.rtsx")
		if codes != "" {
			t.Errorf("%q: transpiler diagnostics %q", c.typed, codes)
		}
		if !strings.Contains(out.TSX, `s === "a" ? one`) || !strings.Contains(out.TSX, c.want) {
			t.Errorf("%q: the Switch is not lowered around the half-typed case (want %q):\n%s", c.typed, c.want, out.TSX)
		}
		if out.Dropped != c.dropped || out.Stopped != "" {
			t.Errorf("%q: dropped %v, stopped %q; want dropped %v", c.typed, out.Dropped, out.Stopped, c.dropped)
		}
	}
	// A Match whose tag is still open keeps its subject.
	src := inReturn("    <div>\n      <Match on={Status.}\n    </div>")
	if codes, out := tolerantCodes(t, map[string]string{"a.rtsx": src}, "a.rtsx"); codes != "" || !strings.Contains(out.TSX, "Status.") {
		t.Errorf("Match: diagnostics %q, output\n%s", codes, out.TSX)
	}
}

// A construct replaced by `null` — its error reported or not — marks the
// file when it held code: TS no longer sees what the author wrote there.
func TestDropped(t *testing.T) {
	for _, c := range []struct {
		name, jsx string
		dropped   bool
	}{
		{"a case without a test", "<Switch on={s}><$Case>{x}</$Case></Switch>", true},
		{"a Match without a subject", "<Match>{x}</Match>", true},
		{"an orphaned slot with a body", "<div><$Icon>{x}</$Icon></div>", true},
		{"an orphaned slot without code", "<div><$Icon className=\"i\" /></div>", false},
		{"nothing wrong", "<Switch on={s}><$Case is=\"a\">{x}</$Case></Switch>", false},
	} {
		for _, tolerant := range []bool{false, true} {
			out, err := Transpile(Input{Files: map[string]string{"a.rtsx": inReturn("    " + c.jsx)}, Entry: "a.rtsx", Tolerant: tolerant})
			if err != nil || out.Dropped != c.dropped {
				t.Errorf("%s (tolerant %v): err %v, dropped %v, want %v\n%s", c.name, tolerant, err, out.Dropped, c.dropped, out.TSX)
			}
		}
	}
}

// ide.md, *Tolerance*, rule 5: in tolerant mode a panic inside a pass ends
// the passes there — whether the source parses or not — with the last good
// text, the pass named, and one `internal` diagnostic on the first line.
func TestTolerantPanic(t *testing.T) {
	for _, src := range []string{
		"const size = 1;\nexport const a = <section #intro size />;\n",
		"const size = 1;\nexport const a = <section #intro size />;\nconst b = ;\n",
	} {
		calls := 0
		read := func(p string) (string, bool) {
			if p == "intro.rtsx" {
				if calls++; calls > 2 { // pass 0 finds it and reads it; pass 4 looks again
					panic("boom")
				}
				return "", true
			}
			return "", false
		}
		out, err := Transpile(Input{Files: map[string]string{"input.rtsx": src}, Entry: "input.rtsx", ReadFile: read, Tolerant: true})
		if err != nil || out.Map == nil {
			t.Fatalf("%q: err %v", src, err)
		}
		if out.Stopped != "pass 4 (segment roots): boom" {
			t.Errorf("%q: stopped %q", src, out.Stopped)
		}
		// Pass 1 completed: its text is kept.
		if !strings.Contains(out.TSX, "<section #intro size={size} />") {
			t.Errorf("%q: the last good text is not kept:\n%s", src, out.TSX)
		}
		internal := 0
		for _, d := range out.Diagnostics {
			if d.Code == "internal" {
				internal++
				if d.Line != 1 || d.Col != 1 || d.Message != out.Stopped {
					t.Errorf("%q: internal diagnostic %+v", src, d)
				}
			}
		}
		if internal != 1 {
			t.Errorf("%q: %d internal diagnostics: %+v", src, internal, out.Diagnostics)
		}
	}
}
