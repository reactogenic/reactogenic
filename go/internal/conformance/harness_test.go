package conformance

import (
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

func TestCanonicalIgnoresFormatting(t *testing.T) {
	same := []struct{ a, b string }{
		{`<Button size="lg" />;`, "(\n  <Button   size=\"lg\"/>\n);"},
		{"<p>\n  Fizz\n  Buzz\n</p>;", "<p>Fizz Buzz</p>;"},
		{`{a ? <b /> : null}`, `(a ? <b /> : null);`},
		{"x; // comment", "x;"},
	}
	for _, c := range same {
		a, errA := Canonical(c.a)
		b, errB := Canonical(c.b)
		if len(errA)+len(errB) > 0 {
			t.Fatalf("parse errors: %v %v", errA, errB)
		}
		if a != b {
			t.Errorf("want equal:\n%s\n%s\n--- a\n%s--- b\n%s", c.a, c.b, a, b)
		}
	}

	different := []struct{ a, b string }{
		{`<Button size="lg" />;`, `<Button size="md" />;`},
		{`<Input disabled />;`, `<Input disabled={disabled} />;`},
		{`a ? <b /> : null;`, `a && <b />;`},
	}
	for _, c := range different {
		a, _ := Canonical(c.a)
		b, _ := Canonical(c.b)
		if a == b {
			t.Errorf("want different: %s vs %s", c.a, c.b)
		}
	}
}

func TestExtractSpec(t *testing.T) {
	md := "## Sec\n\n" +
		"```tsx\n// .rtsx\n<a #intro />   // Warning: note\n```\n\nprose\n\n" +
		"```tsx\n// after pass 2 — x\nstage;\n```\n\n" +
		"```tsx\n// .tsx\nfinal;\n```\n\n" +
		"```tsx\n// not a pair\nx;\n```\n"
	cases := ExtractSpec("t.md", md)
	if len(cases) != 2 {
		t.Fatalf("want 2 cases, got %d", len(cases))
	}
	if c := cases[0]; c.ID != "t.md#sec/1" || c.UntilPass != 2 || c.WantTSX != specPreludeAfter+"stage;\n" {
		t.Errorf("stage case: %+v", c)
	}
	if c := cases[1]; c.ID != "t.md#sec/2" || c.UntilPass != 0 || c.WantTSX != specPreludeAfter+"final;\n" {
		t.Errorf("final case: %+v", c)
	}
	c := cases[0]
	if got := c.Files["input.rtsx"]; strings.Contains(got, "Warning") || got != specPrelude+"<a #intro />\n" {
		t.Errorf("annotation not stripped: %q", got)
	}
	if _, ok := c.Files["+intro.rtsx"]; !ok {
		t.Errorf("no stub for segment #intro: %v", c.Files)
	}
}

func TestSlugDeduplicates(t *testing.T) {
	seen := map[string]int{}
	for _, want := range []string{"desugaring", "desugaring-1", "desugaring-static-switch", "params-on-a-component-children-is-the-default-slot"} {
		heading := map[string]string{
			"desugaring":               "Desugaring",
			"desugaring-1":             "Desugaring",
			"desugaring-static-switch": "Desugaring: static `Switch`",
			"params-on-a-component-children-is-the-default-slot": "Params on a component: `children` is the default slot",
		}[want]
		if got := slug(heading, seen); got != want {
			t.Errorf("slug(%q) = %q, want %q", heading, got, want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	got, err := parseErrors("# comment\n3:5 error orphan-slot\n7 warning segment-children \"overwritten\"\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Expectation{
		{Line: 3, Col: 5, Code: "orphan-slot"},
		{Line: 7, Severity: transpiler.Warning, Code: "segment-children", Message: "overwritten"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := parseErrors("oops\n"); err == nil {
		t.Error("want an error for a malformed line")
	}
}

func TestRun(t *testing.T) {
	c := Case{
		ID:         "x",
		WantTSX:    "<Input value={value} />;",
		WantErrors: []Expectation{{Line: 1, Code: "orphan-slot"}},
	}
	fake := func(out transpiler.Output) func(transpiler.Input) (transpiler.Output, error) {
		return func(transpiler.Input) (transpiler.Output, error) { return out, nil }
	}
	orphan := transpiler.Diagnostic{Line: 1, Col: 1, Code: "orphan-slot"}

	if r := Run(c, fake(transpiler.Output{TSX: "(<Input  value={value}/>);", Diagnostics: []transpiler.Diagnostic{orphan}})); r != "" {
		t.Errorf("want pass, got: %s", r)
	}
	if r := Run(c, fake(transpiler.Output{TSX: "<Input value />;", Diagnostics: []transpiler.Diagnostic{orphan}})); !strings.Contains(r, "output differs") {
		t.Errorf("want output mismatch, got: %q", r)
	}
	if r := Run(c, fake(transpiler.Output{TSX: "<Input value={value} />;"})); !strings.Contains(r, "missing diagnostic") {
		t.Errorf("want missing diagnostic, got: %q", r)
	}
	extra := transpiler.Diagnostic{Line: 2, Code: "slot-key"}
	if r := Run(c, fake(transpiler.Output{TSX: "<Input value={value} />;", Diagnostics: []transpiler.Diagnostic{orphan, extra}})); !strings.Contains(r, "unexpected diagnostic") {
		t.Errorf("want unexpected diagnostic, got: %q", r)
	}
}
