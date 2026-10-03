package lsptest

import (
	"os"
	"path/filepath"
	"testing"
)

// TSConfig is the tsconfig of test projects.
const TSConfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true
  },
  "include": ["src"]
}`

// JSXTypes is a minimal JSX namespace, as `src/jsx.d.ts`.
const JSXTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
}`

// Project writes files into a fresh directory and returns it. A
// `tsconfig.json` and `src/jsx.d.ts` are added unless given; "" as a file's
// text leaves it out.
func Project(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]string{"tsconfig.json": TSConfig, "src/jsx.d.ts": JSXTypes}
	for name, text := range files {
		all[name] = text
	}
	for name, text := range all {
		if text == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir)
}
