package transpiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
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
		// Dropped: the owner's opening tag is half-typed, and recovery leaves
		// its children with no element around them — the statement answers.
		{"an attribute being typed on the owner", inReturn("    <Button variant=>\n      <$Icon className=\"i\" />\n      <$Label>{s}</$Label>\n      <$Hint>h</$Hint>\n    </Button>"), ""},
		{"the same on a keyed owner", inReturn("    <Table rows={s} data=>\n      <$Column key=\"email\" />\n      <$Column key=\"name\" width={2}>Name</$Column>\n    </Table>"), ""},
		{"the subject of a Switch being retyped", inReturn("    <Switch on=>\n      <$Case is=\"a\">A</$Case>\n      <$Case is=\"b\">B</$Case>\n      <$Case is=\"c\">C</$Case>\n    </Switch>"), ""},
		{"the owner's `>` deleted", inReturn("    <Button variant=\"x\"\n      <$Icon className=\"i\" />\n      <$Label>{s}</$Label>\n    </Button>"), ""},
		{"an orphan and the error in one statement", "export const a = <$Orphan />, b = ;\n", ""},
		// Kept: the syntax error is in another statement or another function.
		{"a root element, error in the statement before", tolerantHead + "  const y = s.;\n  return (<Switch><$Case is=\"a\">A</$Case></Switch>);\n}\n", "4:11 flow-no-subject"},
		{"a root element, an unterminated string on the line before", tolerantHead + "  const y = \"abc\n  return (<Switch><$Case is=\"a\">A</$Case></Switch>);\n}\n", "4:11 flow-no-subject"},
		{"a root element in a block, error after the block", tolerantHead + "  if (ok) {\n    return (<Switch><$Case is=\"a\">A</$Case></Switch>);\n  }\n  const y = ;\n}\n", "4:13 flow-no-subject"},
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

// A construct replaced by `null` or left out — its error reported or not —
// marks the file when it held code: TS no longer sees what the author wrote
// there.
func TestDropped(t *testing.T) {
	for _, c := range []struct {
		name, jsx string
		dropped   bool
	}{
		{"a case without a test", "<Switch on={s}><$Case>{x}</$Case></Switch>", true},
		{"a Match without a subject", "<Match>{x}</Match>", true},
		{"an orphaned slot with a body", "<div><$Icon>{x}</$Icon></div>", true},
		{"an orphaned slot without code", "<div><$Icon className=\"i\" /></div>", false},
		// A conditional child that is neither a slot's value nor a child.
		{"a mixed conditional slot", "<Button>{ok ? <$Icon className=\"i\" /> : <b>{x}</b>}</Button>", true},
		{"a conditional of two slots", "<Button>{ok ? <$Icon className=\"i\" /> : <$Label>t</$Label>}</Button>", true},
		{"a conditional slot", "<Button>{ok ? <$Icon className=\"i\" /> : null}</Button>", false},
		// The body wins over a `children` attribute.
		{"children given twice", "<Button><$Icon children={x}>body</$Icon></Button>", true},
		{"children given twice, a string", "<Button><$Icon children=\"x\">body</$Icon></Button>", false},
		{"nothing wrong", "<Switch on={s}><$Case is=\"a\">{x}</$Case></Switch>", false},
	} {
		for _, tolerant := range []bool{false, true} {
			out, err := Transpile(Input{Files: map[string]string{"a.rtsx": inReturn("    " + c.jsx)}, Entry: "a.rtsx", Tolerant: tolerant})
			if err != nil || out.Dropped != c.dropped {
				t.Errorf("%s (tolerant %v): err %v, dropped %v, want %v\n%s", c.name, tolerant, err, out.Dropped, c.dropped, out.TSX)
			}
		}
	}
	// The same where the error is dropped: a `Match` not closed yet, above a
	// slot element and a child, lowers to a mixed conditional. `x` is in the
	// source and no longer in the output.
	src := inReturn("    <Button>\n      <Match on={ok}>\n      <$Icon className=\"i\" />\n      {x}\n    </Button>")
	codes, out := tolerantCodes(t, map[string]string{"a.rtsx": src}, "a.rtsx")
	if codes != "" || !out.Dropped || !strings.Contains(out.TSX, "<Button />") {
		t.Errorf("an unclosed Match above a slot: diagnostics %q, dropped %v\n%s", codes, out.Dropped, out.TSX)
	}
}

// A construct that is an error where it stands and that no pass lowers — an
// arg without an attachment, params on an intrinsic element — stays in the
// output as written and marks the file (Output.Unlowered): that text is not
// TSX. Everything else is lowered, or replaced: the output is TSX.
func TestUnlowered(t *testing.T) {
	for _, c := range []struct {
		name, jsx string
		unlowered bool
	}{
		{"an arg without an attachment", "<option &ok />", true},
		{"an arg and prop without an attachment", "<option slot=\"header\" &&value={s} />", true},
		{"an arg on a component", "<Button &ok />", true},
		{"params on an intrinsic element", "<div { x }>{x}</div>", true},
		{"params on a component", "<Button { x }>{x}</Button>", false},
		{"params on a slot element", "<Button><$Icon { x }>{x}</$Icon></Button>", false},
		{"params given twice", "<Button { x } { y }>{x}</Button>", false},
		{"an orphaned slot", "<div><$Icon { x }>{x}</$Icon></div>", false},
		{"nothing wrong", "<option value={s} />", false},
	} {
		for _, tolerant := range []bool{false, true} {
			out, err := Transpile(Input{Files: map[string]string{"a.rtsx": inReturn("    " + c.jsx)}, Entry: "a.rtsx", Tolerant: tolerant})
			if err != nil || out.Unlowered != c.unlowered {
				t.Errorf("%s (tolerant %v): err %v, unlowered %v, want %v\n%s", c.name, tolerant, err, out.Unlowered, c.unlowered, out.TSX)
				continue
			}
			if tsx := len(rtsx.ParseTSX("/a.tsx", out.TSX).Diagnostics()) == 0; tsx == c.unlowered {
				t.Errorf("%s (tolerant %v): unlowered %v, and the output parses as TSX: %v\n%s", c.name, tolerant, c.unlowered, tsx, out.TSX)
			}
		}
	}
	// The same where the error is dropped, the tag being half-typed: the arg
	// is in the output all the same.
	src := inReturn("    <div>\n      <option &ok\n    </div>")
	codes, out := tolerantCodes(t, map[string]string{"a.rtsx": src}, "a.rtsx")
	if codes != "" || !out.Unlowered || !strings.Contains(out.TSX, "&ok") {
		t.Errorf("an arg in an unclosed tag: diagnostics %q, unlowered %v\n%s", codes, out.Unlowered, out.TSX)
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
