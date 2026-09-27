package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

const tsconfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true
  },
  "include": ["src"]
}`

// A global JSX namespace, so the test needs no React types.
const jsxTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
  interface ElementChildrenAttribute { children: {} }
}`

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir) // macOS: /var → /private/var
	for name, text := range files {
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

func messages(diags []*rtsx.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		name := "(global)"
		if d.File() != nil {
			name = filepath.Base(d.File().FileName())
		}
		out = append(out, name+": "+rtsx.Message(d))
	}
	return out
}

// RGP1-050: .tsx and .rtsx import each other; the program type-checks.
func TestMixedProject(t *testing.T) {
	dir := write(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/a.tsx":     "import { B } from \"./b\";\nexport const a = <B label=\"x\" />;\n",
		"src/b.rtsx":    "import { C } from \"./c\";\nexport function B({ label }: { label: string }) {\n  return <C label />;\n}\n",
		"src/c.tsx":     "export function C(p: { label: string }) {\n  return <div>{p.label}</div>;\n}\n",
	})
	p := Open(dir + "/tsconfig.json")
	if diags := p.Diagnostics(); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", messages(diags))
	}
	for _, name := range []string{"a.tsx", "b.tsx", "c.tsx"} {
		if p.Program.GetSourceFile(dir+"/src/"+name) == nil {
			t.Errorf("%s is not in the program", name)
		}
	}
	out, ok := p.Output(dir + "/src/b.rtsx")
	if !ok || !strings.Contains(out.TSX, "<C label={label} />") {
		t.Errorf("b.rtsx output: %q", out.TSX)
	}
}

// A type error in an .rtsx file is reported, on its virtual .tsx.
func TestTypeErrorInRTSX(t *testing.T) {
	dir := write(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/b.rtsx":    "import { C } from \"./c\";\nexport const b = <C label={1} />;\n",
		"src/c.tsx":     "export function C(p: { label: string }) {\n  return <div>{p.label}</div>;\n}\n",
	})
	got := messages(Open(dir + "/tsconfig.json").Diagnostics())
	if len(got) != 1 || !strings.HasPrefix(got[0], "b.tsx: Type 'number' is not assignable to type 'string'") {
		t.Errorf("got %q", got)
	}
}
