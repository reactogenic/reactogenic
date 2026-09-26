package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const tsconfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true
  },
  "include": ["src"]
}`

const jsxTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
}`

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	for name, text := range files {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir)
}

func readFile(p string) (string, bool) {
	b, err := os.ReadFile(p)
	return string(b), err == nil
}

func TestCheck(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		// Shorthand, then a type error on a copied attribute: mapped exactly.
		"src/b.rtsx": "import { C } from \"./c\";\nconst label = 1;\nexport const b = <C label />;\nexport const o = <div><$Title>t</$Title></div>;\n",
		"src/c.tsx":  "export function C(p: { label: string }) {\n  return <div>{p.label}</div>;\n}\nexport const n: number = \"x\";\n",
	})
	reports := Run(dir + "/tsconfig.json")
	var got []string
	for _, r := range reports {
		got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
	}
	want := []string{
		"src/b.rtsx:3:21 TS2322",      // `label` in <C label />
		"src/b.rtsx:4:23 orphan-slot", // the transpiler's own error
		"src/c.tsx:4:14 TS2322",       // plain .tsx, as is
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if Errors(reports) != 3 {
		t.Errorf("errors = %d", Errors(reports))
	}

	var pretty, plain bytes.Buffer
	Print(&pretty, reports, dir, true, readFile)
	Print(&plain, reports, dir, false, readFile)
	if !strings.Contains(pretty.String(), "src/b.rtsx:3:21 - error TS2322: Type 'number' is not assignable to type 'string'.") ||
		!strings.Contains(pretty.String(), "3 export const b = <C label />;\n                      ^") ||
		!strings.Contains(pretty.String(), "Found 3 error(s).") {
		t.Errorf("pretty output:\n%s", pretty.String())
	}
	if !strings.Contains(plain.String(), "src/b.rtsx(4,23): error orphan-slot: Slot must be immediate child of the component") {
		t.Errorf("plain output:\n%s", plain.String())
	}
}

func TestClean(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/a.rtsx":    "export const a = <div>ok</div>;\n",
	})
	if reports := Run(dir + "/tsconfig.json"); len(reports) != 0 {
		t.Errorf("got %+v", reports)
	}
}
