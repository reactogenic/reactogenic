package lsp_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/checktest"
	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// ide.md, *Diagnostics*: the editor shows what `reactogenic check` prints —
// same codes, same messages, same positions. For every project of check's
// goldens (internal/checktest), and every .rtsx document of it: the errors
// and warnings the server answers a pull with are check's lines for that
// file — severity, code, message, line and column, and the related
// locations under them.
//
// Both run in this process, on the same directory. The transform is the
// server's, tolerant; check's is strict, and the two are the same transform
// for a source that parses — so the projects with a syntax error are left
// out: there the hosts differ by design (TestTolerance has those).
func TestEqualsCheck(t *testing.T) {
	for _, p := range checktest.Projects {
		t.Run(p.Name, func(t *testing.T) {
			if p.SyntaxError {
				t.Skip("a syntax error: the build is strict, the editor tolerant")
			}
			config := p.Config
			if config == "" {
				config = "tsconfig.json"
			}
			var documents []string
			for name := range p.Files {
				if strings.HasSuffix(name, ".rtsx") && !strings.Contains(name, "node_modules/") {
					documents = append(documents, name)
				}
			}
			equalsCheck(t, lsptest.Write(t, p.Files), config, documents, p.Unlisted)
		})
	}

	// The Vite test app, with the real @reactogenic/core and React's own
	// types — a copy, under a tsconfig.json the server finds. Needs the
	// workspace's node_modules: skipped where they are not installed.
	t.Run("vite-app", func(t *testing.T) {
		repo, _ := filepath.Abs("../../..")
		repo = filepath.ToSlash(repo)
		modules := repo + "/packages/vite/node_modules"
		if _, err := os.Stat(modules + "/@types/react"); err != nil {
			t.Skip("pnpm install has not run")
		}
		files := map[string]string{"tsconfig.json": `{
  "compilerOptions": {
    "strict": true, "jsx": "react-jsx", "module": "esnext", "moduleResolution": "bundler", "target": "es2022",
    "lib": ["es2022", "dom"], "types": [], "noEmit": true, "allowImportingTsExtensions": true, "skipLibCheck": true,
    "paths": { "@reactogenic/core": ["` + repo + `/packages/core/src/index.ts"] }
  },
  "include": ["src"]
}`}
		var documents []string
		app := repo + "/packages/vite/test/render"
		err := filepath.WalkDir(app+"/src", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			text, err := os.ReadFile(path)
			name := strings.TrimPrefix(filepath.ToSlash(path), app+"/")
			files[name] = string(text)
			if strings.HasSuffix(name, ".rtsx") {
				documents = append(documents, name)
			}
			return err
		})
		if err != nil || len(documents) == 0 {
			t.Fatalf("%s: %d .rtsx files, %v", app, len(documents), err)
		}
		dir := lsptest.Write(t, files)
		if err := os.Symlink(modules, dir+"/node_modules"); err != nil {
			t.Fatal(err)
		}
		equalsCheck(t, dir, "tsconfig.json", documents, nil)
	})
}

// equalsCheck runs check on the tsconfig and the server on the directory,
// and compares them document by document.
func equalsCheck(t *testing.T, dir, config string, documents, unlisted []string) {
	t.Helper()
	c := lsptest.Start(t, dir, serve) // first: Serve registers the transform
	want := map[string][]string{}
	for _, r := range check.Run(dir + "/" + config) {
		rel := strings.TrimPrefix(r.File, dir+"/")
		want[rel] = append(want[rel], fmt.Sprintf("%d:%d %s %s: %s", r.Line, r.Col, r.Severity, r.Code, r.Message))
		for _, related := range r.Related {
			// A related line without a place of its own is shown at its
			// diagnostic's: an LSP related location cannot be without one.
			where := fmt.Sprintf("%s:%d:%d", rel, r.Line, r.Col)
			if related.File != "" {
				where = fmt.Sprintf("%s:%d:%d", strings.TrimPrefix(related.File, dir+"/"), related.Line, related.Col)
			}
			want[rel] = append(want[rel], "  "+where+" "+related.Message)
		}
	}
	sort.Strings(documents)
	lines := 0
	for _, rel := range documents {
		c.Open(rel)
		var got []string
		for _, d := range c.Diagnostics(rel) {
			line, col := lineCol(c.Text(rel), d.Range.Start)
			code := strings.Trim(string(d.Code), `"`)
			if d.Source == "ts" {
				code = "TS" + code
			}
			severity := map[int]report.Severity{1: report.Error, 2: report.Warning, 3: report.Message}[d.Severity]
			got = append(got, fmt.Sprintf("%d:%d %s %s: %s", line, col, severity, code, d.Message))
			for _, related := range d.Related {
				file := c.Rel(related.Location.URI)
				line, col := lineCol(c.Text(file), related.Location.Range.Start)
				got = append(got, fmt.Sprintf("  %s:%d:%d %s", file, line, col, related.Message))
			}
		}
		if len(unlistedIn(unlisted, rel)) > 0 {
			// In no program of the project: check does not report the file.
			// The editor checks an open document all the same.
			if len(want[rel]) != 0 || len(got) == 0 {
				t.Errorf("%s is in no program: check prints %q, the editor shows %q", rel, want[rel], got)
			}
			continue
		}
		lines += len(got)
		if strings.Join(got, "\n") != strings.Join(want[rel], "\n") {
			t.Errorf("%s\nthe editor shows:\n%s\ncheck prints:\n%s", rel, strings.Join(got, "\n"), strings.Join(want[rel], "\n"))
		}
	}
	t.Logf("%d documents, %d lines", len(documents), lines)
}

func unlistedIn(unlisted []string, rel string) []string {
	for _, name := range unlisted {
		if name == rel {
			return []string{name}
		}
	}
	return nil
}

// lineCol is an LSP position as check prints one: 1-based, the column in
// characters.
func lineCol(text string, at lsptest.Position) (int, int) {
	return emit.LineCol(text, lsptest.Offset(text, at))
}
