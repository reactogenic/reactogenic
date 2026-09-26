package conformance

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// LoadFixtures reads every fixture directory under root: a directory that
// holds an input.rtsx. See fixtures/README.md.
func LoadFixtures(root string) ([]Case, error) {
	fsys := os.DirFS(root)
	var cases []Case
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if _, err := fs.Stat(fsys, path.Join(p, "input.rtsx")); err != nil {
			return nil
		}
		c, err := loadFixture(fsys, p)
		if err != nil {
			return fmt.Errorf("fixtures/%s: %w", p, err)
		}
		cases = append(cases, c)
		return fs.SkipDir
	})
	return cases, err
}

func loadFixture(fsys fs.FS, dir string) (Case, error) {
	c := Case{ID: "fixtures/" + dir, Entry: "input.rtsx", Files: map[string]string{}}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return c, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return c, err
		}
		switch e.Name() {
		case "output.tsx":
			c.WantTSX = string(data)
		case "errors.txt":
			if c.WantErrors, err = parseErrors(string(data)); err != nil {
				return c, err
			}
		case "check.txt", "README.md":
			// check.txt: `reactogenic check` output, asserted from RGP1-070.
		default:
			c.Files[e.Name()] = string(data)
		}
	}
	return c, nil
}

// LINE[:COL] error|warning [CODE] ["message substring"]
var errorLineRe = regexp.MustCompile(`^(\d+)(?::(\d+))?\s+(error|warning)(?:\s+([a-z][a-z0-9-]*))?(?:\s+"(.*)")?\s*$`)

func parseErrors(text string) ([]Expectation, error) {
	var out []Expectation
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := errorLineRe.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("errors.txt:%d: cannot parse %q", i+1, line)
		}
		e := Expectation{Code: m[4], Message: m[5]}
		e.Line, _ = strconv.Atoi(m[1])
		if m[2] != "" {
			e.Col, _ = strconv.Atoi(m[2])
		}
		if m[3] == "warning" {
			e.Severity = transpiler.Warning
		}
		out = append(out, e)
	}
	return out, nil
}
