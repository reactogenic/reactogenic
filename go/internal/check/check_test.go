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

// A stand-in for the installed @reactogenic/core, for temporary projects.
var coreStub = map[string]string{
	"node_modules/@reactogenic/core/package.json": `{ "name": "@reactogenic/core", "types": "index.d.ts" }`,
	"node_modules/@reactogenic/core/index.d.ts": `type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol }; // as React: a function is not a node
export type SlotFn<Params> = (params: Params) => ReactNode;
export type OptionalSlotFn<Params> = ReactNode | SlotFn<Params>;
export type Slot<Props, Children = never> = [Children] extends [never] ? Props : Omit<Props, "children"> & { children: Children };
export type NoArgs = { readonly [arg: string]: never };
export declare function renderSlot<Children>(children: Children, args: Children extends (params: infer Params) => ReactNode ? Params : NoArgs): ReactNode;`,
}

// syntax.md, *Rendering a slot*: a function slot rendered without the args
// it declares is a TS error, reported on the element in the .rtsx.
func TestFunctionSlotNeedsArgs(t *testing.T) {
	files := map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/button.rtsx": `import type { Slot, SlotFn } from "@reactogenic/core";
interface ButtonProps {
  size: string;
  $IconEnd?: Slot<{ className?: string }, SlotFn<{ size: string }>>;
}
export function Button({ size, $IconEnd }: ButtonProps) {
  return (
    <button>
      <div slot={$IconEnd} />
      <i slot={$IconEnd} size />
    </button>
  );
}
`,
	}
	for k, v := range coreStub {
		files[k] = v
	}
	dir := writeProject(t, files)
	var got []string
	for _, r := range Run(dir + "/tsconfig.json") {
		got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+" "+r.Code+" "+r.Message)
	}
	// Line 9 lacks `size`; line 10 passes it (shorthand) and is fine.
	if len(got) != 1 || !strings.HasPrefix(got[0], "src/button.rtsx:9 TS2741") || !strings.Contains(got[0], "size") {
		t.Errorf("got %q", got)
	}
}

// A slot whose body is not a function takes no args: an arg is an error on
// that attribute, as calling a function of no parameters with one.
func TestPlainSlotTakesNoArgs(t *testing.T) {
	files := map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/label.rtsx": `import type { Slot } from "@reactogenic/core";
interface LabelProps {
  $Label?: Slot<{ title?: string; children?: string }>;
}
export function Label({ $Label }: LabelProps) {
  const tone = "muted";
  return (
    <p>
      <span slot={$Label} />
      <span slot={$Label} tone />
    </p>
  );
}
`,
	}
	for k, v := range coreStub {
		files[k] = v
	}
	dir := writeProject(t, files)
	var got []string
	for _, r := range Run(dir + "/tsconfig.json") {
		got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
	}
	// Line 10, the `tone` attribute; line 9 passes nothing and is fine.
	if len(got) != 1 || got[0] != "src/label.rtsx:10:27 TS2322" {
		t.Errorf("got %q", got)
	}
}
