package conformance

import (
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

var update = flag.Bool("update", false, "rewrite testdata/passing.txt from the current results")

const (
	specPath     = "../../../specs/phase01/syntax.md"
	fixturesRoot = "../../../fixtures"
	passingPath  = "testdata/passing.txt"
)

func specCases(t *testing.T) []Case {
	t.Helper()
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	cases := ExtractSpec("syntax.md", string(data))
	if len(cases) == 0 {
		t.Fatal("no example pairs found in syntax.md")
	}
	return cases
}

// Every expected .tsx in the spec must itself be valid TSX; otherwise the
// example cannot be compared with anything.
func TestSpecOutputsParse(t *testing.T) {
	for _, c := range specCases(t) {
		if _, errs := Canonical(c.WantTSX); len(errs) > 0 {
			t.Errorf("%s: expected .tsx does not parse: %s\n%s", c.ID, strings.Join(errs, "; "), c.WantTSX)
		}
	}
}

// TestConformance is a ratchet. testdata/passing.txt lists the cases that
// pass; a listed case that fails is a regression, and a case that passes but
// is not listed must be added (go test -run TestConformance -update). Cases
// that are neither are not implemented yet and are only counted.
func TestConformance(t *testing.T) {
	cases := specCases(t)
	fixtures, err := LoadFixtures(fixturesRoot)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, fixtures...)

	passing := readPassing(t)
	var nowPassing []string
	for _, c := range cases {
		reason := Run(c, transpiler.Transpile)
		listed := slices.Contains(passing, c.ID)
		switch {
		case reason == "":
			nowPassing = append(nowPassing, c.ID)
			if !listed && !*update {
				t.Errorf("%s passes but is not in %s; run with -update", c.ID, passingPath)
			}
		case listed && !*update:
			t.Errorf("%s regressed:\n%s", c.ID, reason)
		default:
			t.Logf("not yet: %s: %s", c.ID, firstLine(reason))
		}
	}
	t.Logf("%d of %d conformance cases pass", len(nowPassing), len(cases))

	if *update {
		slices.Sort(nowPassing)
		data := "# Conformance cases that pass. Maintained by `go test -run TestConformance -update`.\n"
		for _, id := range nowPassing {
			data += id + "\n"
		}
		if err := os.WriteFile(passingPath, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readPassing(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(passingPath)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			ids = append(ids, line)
		}
	}
	return ids
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
