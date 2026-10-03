package check

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// The transform is registered once for the process, as the command does.
func TestMain(m *testing.M) {
	mapper.RegisterStrict("test")
	os.Exit(m.Run())
}

// golden compares the project's whole `check --pretty=false` output with
// testdata/golden/<test name>.txt: every line, message included. First
// recorded from the overlay program, before RGP1-106 moved `check` to the
// mapped one: plan.md lists the five lines that changed with it.
func golden(t *testing.T, dir string, reports []Report) {
	t.Helper()
	var b bytes.Buffer
	Print(&b, reports, dir, false, readFile)
	got := strings.ReplaceAll(b.String(), dir, "<root>")
	path := filepath.Join("testdata", "golden", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+".txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("no golden output for %s: run `go test ./internal/check -update`", t.Name())
		return
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\n--- got\n%s--- want\n%s", path, got, want)
	}
}

// Each mistake once (ide.md, *Diagnostics*): the three projects whose output
// the move to the mapped program changed — the overlay printed a line twice.
func TestGoldenKnownDifferences(t *testing.T) {
	for name, want := range map[string][]string{
		// An error inside a mounted segment. It was reported again under
		// `intro.rtsx.tsx`, the alias the generated import resolved to: a
		// second module.
		"segment": {"src/intro.rtsx:2:9 TS2322"},
		// An import that names the .rtsx file: the same.
		"explicit-import": {"src/b.rtsx:1:14 TS2322"},
		// An attachment with a fallback is emitted twice, as the branches of
		// a ternary. `.nope` is an error in both, with a message per branch:
		// reported once, from the first. `$Label` is possibly undefined in
		// the fallback's branch only: that stays.
		"attachment": {"src/button.rtsx:3:34 TS18048", "src/button.rtsx:3:41 TS2339"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := checkProject(t, name); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("got  %q\nwant %q", got, want)
			}
		})
	}
}

// The Vite test app, with the real @reactogenic/core and React's own types.
// Needs the workspace's node_modules: skipped where they are not installed.
func TestGoldenViteApp(t *testing.T) {
	repo, _ := filepath.Abs("../../..")
	repo = filepath.ToSlash(repo)
	app := repo + "/packages/vite/test/render"
	if _, err := os.Stat(repo + "/packages/vite/node_modules/@types/react"); err != nil {
		t.Skip("pnpm install has not run")
	}
	config := app + "/tsconfig.golden.json"
	err := os.WriteFile(config, []byte(`{
  "compilerOptions": {
    "strict": true, "jsx": "react-jsx", "module": "esnext", "moduleResolution": "bundler", "target": "es2022",
    "lib": ["es2022", "dom"], "types": [], "noEmit": true, "allowImportingTsExtensions": true, "skipLibCheck": true,
    "paths": { "@reactogenic/core": ["`+repo+`/packages/core/src/index.ts"] }
  },
  "include": ["src"]
}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(config)
	golden(t, app, Run(config))
}
