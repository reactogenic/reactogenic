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
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
export type Slot<Props> = Props | NotAssigned;
export type FnSlot<Props, Args> = (Omit<Props, "children"> & { children?: (args: Args) => ReactNode }) | NotAssigned;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export type NoArgs = { readonly [arg: string]: never };
export type ArgsOf<S> = S extends { children?: infer Body } ? NonNullable<Body> extends (args: infer Args) => ReactNode ? Args : NoArgs : NoArgs;
export declare function renderSlot<S extends object>(slot: S, args: S extends readonly unknown[] ? never : ArgsOf<S>, fallback?: ReactNode): ReactNode;`,
}

// checkProject writes a project with the core stub and returns its reports
// as "file:line:col CODE".
func checkProject(t *testing.T, files map[string]string) []string {
	t.Helper()
	files["tsconfig.json"] = tsconfig
	files["src/jsx.d.ts"] = jsxTypes
	for k, v := range coreStub {
		files[k] = v
	}
	dir := writeProject(t, files)
	var got []string
	for _, r := range Run(dir + "/tsconfig.json") {
		got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
	}
	return got
}

// syntax.md, *Slots → Attachment*: args follow function-call rules, checked
// by TS7 through renderSlot and reported on the .rtsx.
func TestSlotArgs(t *testing.T) {
	got := checkProject(t, map[string]string{"src/button.rtsx": `import type { FnSlot, Slot } from "@reactogenic/core";
interface ButtonProps {
  size: string;
  $Icon?: FnSlot<{ className?: string }, { size: string }>;
  $Label?: Slot<{ title?: string; children?: string }>;
  $List?: Slot<{ children?: string }>[];
}
export function Button({ size, $Icon, $Label, $List }: ButtonProps) {
  const tone = "muted";
  return (
    <button>
      <i slot={$Icon} &size />
      <i slot={$Icon} />
      <span slot={$Label} />
      <span slot={$Label} &tone />
      <span slot={$List} />
    </button>
  );
}
`})
	want := []string{
		"src/button.rtsx:13:7 slot-args-missing", // a function slot without its args: `$Icon` needs `&size`
		"src/button.rtsx:15:27 slot-no-args",     // an arg to a slot whose body is not a function
		"src/button.rtsx:16:7 slot-list",         // a slot typed as an array
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// The attachment's children are the fallback; recursive slots and last-wins
// type-check clean.
func TestSlotsTypeCheck(t *testing.T) {
	got := checkProject(t, map[string]string{
		"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export interface ButtonProps {
  variant?: string;
  $IconStart?: Slot<{ children?: JSX.Element }>;
  children?: string;
}
export function Button({ $IconStart, children }: ButtonProps) {
  return <button><span slot={$IconStart}>+</span>{children}</button>;
}
`,
		"src/dialog.rtsx": `import type { Slot } from "@reactogenic/core";
import { Button, type ButtonProps } from "./button";
export function Dialog({ $Action }: { $Action?: Slot<ButtonProps> }) {
  return <dialog><Button slot={$Action} /></dialog>;
}
export const page = (
  <Dialog>
    <$Action variant="ghost">Cancel</$Action>
    <$Action variant="solid">
      <$IconStart><b /></$IconStart>
      Close
    </$Action>
  </Dialog>
);
`,
	})
	if len(got) != 0 {
		t.Errorf("got %q", got)
	}
}
