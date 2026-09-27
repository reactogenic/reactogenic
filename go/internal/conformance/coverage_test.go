package conformance

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// TestSpecCoverage is the spec conformance gate (RGP1-090):
//
//   - every example of syntax.md passes — strictly, not through the ratchet;
//   - every error code the transpiler reports (the *Compile errors* rows that
//     need *syntax* or *files*) is expected by a fixture's errors.txt;
//   - every code that needs *types*, and every rewrite of diagnostics.md, is
//     asserted by a `reactogenic check` test.
func TestSpecCoverage(t *testing.T) {
	for _, c := range specCases(t) {
		if reason := Run(c, transpiler.Transpile); reason != "" {
			t.Errorf("spec example %s fails:\n%s", c.ID, firstLine(reason))
		}
	}

	transpilerCodes, typeCodes := errorCodes(t)
	fixtureCodes := readAll(t, fixturesRoot, "errors.txt")
	checkTests := readAll(t, "../check", "_test.go")
	for code := range transpilerCodes {
		if !regexp.MustCompile(`(?m)^\d+(:\d+)? (error|warning) ` + regexp.QuoteMeta(code) + `\b`).MatchString(fixtureCodes) {
			t.Errorf("no fixture expects %s (syntax.md, Compile errors)", code)
		}
	}
	for code := range typeCodes {
		if !strings.Contains(checkTests, code) {
			t.Errorf("no reactogenic check test asserts %s", code)
		}
	}
}

// errorCodes reads the codes of syntax.md's *Compile errors* tables and of
// diagnostics.md's *Rewrites* table, by what they need.
func errorCodes(t *testing.T) (transpilerCodes, typeCodes map[string]bool) {
	t.Helper()
	transpilerCodes, typeCodes = map[string]bool{}, map[string]bool{}
	syntaxMD, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("^\\| ([a-z][a-z-]*) \\|.*\\| (syntax|files|types|syntax \\(per file\\), route table \\(per page\\)) \\|$")
	for _, line := range strings.Split(string(syntaxMD), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			if m[2] == "types" {
				typeCodes[m[1]] = true
			} else {
				transpilerCodes[m[1]] = true
			}
		}
	}
	diagMD, err := os.ReadFile(filepath.Join(filepath.Dir(specPath), "diagnostics.md"))
	if err != nil {
		t.Fatal(err)
	}
	rewrites := string(diagMD)[strings.Index(string(diagMD), "## Rewrites"):]
	// A rewrite marked "not specific yet" is a known gap, documented there.
	knownGaps := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| ([a-z][a-z-]*) \\|(.*)$").FindAllStringSubmatch(rewrites, -1) {
		switch {
		case m[1] == "code":
		case strings.Contains(m[2], "not specific yet"):
			delete(typeCodes, m[1])
			knownGaps[m[1]] = true
		default:
			typeCodes[m[1]] = true
		}
	}
	for code := range knownGaps {
		delete(typeCodes, code)
	}
	return transpilerCodes, typeCodes
}

// readAll concatenates the files under root whose name ends in suffix.
func readAll(t *testing.T, root, suffix string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, suffix) {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b.Write(data)
			b.WriteByte('\n')
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
