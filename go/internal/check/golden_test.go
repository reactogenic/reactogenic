package check

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// golden compares the project's whole `check --pretty=false` output with
// testdata/golden/<test name>.txt: every line, message included. Recorded
// from the overlay program before RGP1-106, so the move to the mapped
// program shows each difference (plan.md, RGP1-106).
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

// Projects whose output the move to the mapped program is expected to
// change, recorded as they are today.
func TestGoldenKnownDifferences(t *testing.T) {
	for name, files := range map[string]map[string]string{
		// An error inside a mounted segment: reported twice today, once
		// under the alias `intro.rtsx.tsx`.
		"segment": {
			"src/page.rtsx":  "export const page = <main><section #intro /></main>;\n",
			"src/intro.rtsx": "export default function Intro() {\n  const n: number = \"x\";\n  return <p>{n}</p>;\n}\n",
		},
		// An import that names the .rtsx file: the same.
		"explicit-import": {
			"src/main.tsx": "import { b } from \"./b.rtsx\";\nexport const a = b;\n",
			"src/b.rtsx":   "export const b: number = \"x\";\n",
		},
		// An attachment with a fallback is emitted twice: its error too.
		"attachment": {
			"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export function Button({ $Label }: { $Label?: Slot<{ title?: string; children?: string }> }) {
  return <b slot={$Label} title={$Label.nope}>Button</b>;
}
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			checkProject(t, files)
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
