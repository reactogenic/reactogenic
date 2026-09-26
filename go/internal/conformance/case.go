// Package conformance runs the transpiler against the examples of
// specs/phase01/syntax.md and the hand-written cases in fixtures/.
// See fixtures/README.md for the formats.
package conformance

import (
	"fmt"
	"slices"
	"strings"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// Case is one conformance test.
type Case struct {
	ID    string // stable: "syntax.md#desugaring-1/2", "fixtures/slots/list"
	Files map[string]string
	Entry string
	// UntilPass compares an intermediate stage ("after pass 2"); 0 = final.
	UntilPass int
	// WantTSX is the expected output; empty when only errors are checked.
	WantTSX    string
	WantErrors []Expectation
	// IgnoreDiagnostics checks output only (spec examples, stages).
	IgnoreDiagnostics bool
	// ListSlots stands in for the checker until RGP1-051: the list slots
	// the case declares, as "Container.$Slot".
	ListSlots []string
}

// Expectation matches one diagnostic. Zero fields match anything, except
// Line and Severity, which must always agree.
type Expectation struct {
	Line     int
	Col      int
	Severity transpiler.Severity
	Code     string
	Message  string // substring
}

func (e Expectation) matches(d transpiler.Diagnostic) bool {
	return d.Line == e.Line &&
		d.Severity == e.Severity &&
		(e.Col == 0 || d.Col == e.Col) &&
		(e.Code == "" || d.Code == e.Code) &&
		(e.Message == "" || strings.Contains(d.Message, e.Message))
}

func (e Expectation) String() string {
	s := fmt.Sprintf("%d", e.Line)
	if e.Col != 0 {
		s += fmt.Sprintf(":%d", e.Col)
	}
	s += " " + e.Severity.String()
	if e.Code != "" {
		s += " " + e.Code
	}
	if e.Message != "" {
		s += fmt.Sprintf(" %q", e.Message)
	}
	return s
}

// Run transpiles c and returns why it fails, or "" when it passes.
func Run(c Case, transpile func(transpiler.Input) (transpiler.Output, error)) string {
	listSlot := func(container, slot string) bool { return slices.Contains(c.ListSlots, container+"."+slot) }
	out, err := transpile(transpiler.Input{Files: c.Files, Entry: c.Entry, UntilPass: c.UntilPass, ListSlot: listSlot})
	if err != nil {
		return err.Error()
	}
	var problems []string
	if c.IgnoreDiagnostics {
		out.Diagnostics = nil
	}

	unmatched := append([]transpiler.Diagnostic(nil), out.Diagnostics...)
	for _, want := range c.WantErrors {
		found := -1
		for i, d := range unmatched {
			if want.matches(d) {
				found = i
				break
			}
		}
		if found < 0 {
			problems = append(problems, "missing diagnostic: "+want.String())
			continue
		}
		unmatched = append(unmatched[:found], unmatched[found+1:]...)
	}
	for _, d := range unmatched {
		problems = append(problems, fmt.Sprintf("unexpected diagnostic: %d:%d %s %s %q", d.Line, d.Col, d.Severity, d.Code, d.Message))
	}

	if c.WantTSX != "" {
		// A stage may still hold .rtsx forms; final output must be plain TSX.
		rtsx := c.UntilPass > 0
		want, _ := canonical(c.WantTSX, rtsx)
		got, errs := canonical(out.TSX, rtsx)
		switch {
		case len(errs) > 0:
			problems = append(problems, "output does not parse as TSX: "+strings.Join(errs, "; ")+"\n"+out.TSX)
		case got != want:
			problems = append(problems, "output differs\n--- want\n"+c.WantTSX+"\n--- got\n"+out.TSX)
		}
	}
	return strings.Join(problems, "\n")
}
