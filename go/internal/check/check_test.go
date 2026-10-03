package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
type Key = string | number | bigint;
export declare const SLOT_KEY: unique symbol;
export type SlotValue<Props, Args = never> = [Args] extends [never] ? Props : Omit<Props, "children"> & { children?: (args: Args) => ReactNode; readonly [SLOT_KEY]?: (args: Args) => Key };
export type SlotArgs<S> = S extends readonly unknown[] ? never : ArgsOf<Exclude<SlotEntry<S>, NotAssigned | undefined | null>>;
export declare function slotArgs<S>(slot: S, args: SlotArgs<S>): SlotArgs<S>;
export declare function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined;
export type Slot<Props, Args = never> = SlotValue<Props, Args> | NotAssigned;
export declare const KEYED: unique symbol;
export type KeyedSlot<Props, Args = never> = { readonly [KEYED]: true } & { readonly [key: string]: SlotValue<Props, Args> };
export type SlotEntry<S> = S extends { readonly [KEYED]: true } & { readonly [key: string]: infer Entry } ? Entry | undefined : S;
export declare function slotEntry<S>(slot: S, key: string | number): SlotEntry<S>;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export type NoArgs = { readonly [arg: string]: never };
export type ArgsOf<S> = S extends { children?: infer Body } ? NonNullable<Body> extends (args: infer Args) => ReactNode ? Args : NoArgs : NoArgs;
export declare function renderSlot<S extends object>(slot: S, args: S extends readonly unknown[] ? never : ArgsOf<S>, fallback?: ReactNode): ReactNode;
export declare function Match(props: { on: unknown; children?: unknown }): never;`,
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
	got := checkProject(t, map[string]string{"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
interface ButtonProps {
  size: string;
  $Icon?: Slot<{ className?: string }, { size: string }>;
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
		"src/button.rtsx:15:28 slot-no-args",     // an arg to a slot whose body is not a function
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

// Foo.tsx next to Foo.rtsx: the program sees the .tsx, so the .rtsx is
// never transpiled — reported, not silently skipped.
func TestAmbiguousModule(t *testing.T) {
	got := checkProject(t, map[string]string{
		"src/card.rtsx": "export const a = <p>rtsx</p>;\n",
		"src/card.tsx":  "export const a = <p>tsx</p>;\n",
	})
	if len(got) != 1 || got[0] != "src/card.rtsx:1:1 ambiguous-module" {
		t.Errorf("got %q", got)
	}
}

// RGP1-076: an edit is picked up and re-checked.
func TestWatch(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/a.rtsx":    "export const a: number = 1;\n",
	})
	stop := make(chan struct{})
	runs := make(chan []Report, 4)
	go Watch(dir+"/tsconfig.json", 20*time.Millisecond, stop, func(r []Report) { runs <- r })
	defer close(stop)

	if first := <-runs; len(first) != 0 {
		t.Fatalf("first run: %+v", first)
	}
	if err := os.WriteFile(dir+"/src/a.rtsx", []byte("export const a: number = \"x\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case second := <-runs:
		if len(second) != 1 || second[0].Code != "TS2322" {
			t.Errorf("second run: %+v", second)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the edit was not picked up")
	}
}

// Keyed slots type-check: entries as slot values, attached by key.
func TestKeyedSlots(t *testing.T) {
	got := checkProject(t, map[string]string{"src/table.rtsx": `import type { KeyedSlot } from "@reactogenic/core";
interface TableProps {
  columns: { name: string; label: string }[];
  $Column?: KeyedSlot<{ width?: number; children?: string }>;
}
export function Table({ columns, $Column }: TableProps) {
  return <tr>{columns.map((col) => <th key={col.name} slot={$Column}>{col.label}</th>)}</tr>;
}
export const ok = (
  <Table columns={[]}>
    <$Column key="email" width={2}>Email</$Column>
    <$Column key="name" />
  </Table>
);
export const typo = (
  <Table columns={[]}>
    <$Column key="email" widht={2} />
  </Table>
);
`})
	// Only the typo: an excess property of an entry, on its line.
	if len(got) != 1 || !strings.HasPrefix(got[0], "src/table.rtsx:17:") {
		t.Errorf("got %q", got)
	}
}

// syntax.md, *Conditional slots*: a required slot that the component attaches
// without a fallback cannot be filled conditionally — its false branch is
// NOT_ASSIGNED, and nothing would render.
func TestSlotConditional(t *testing.T) {
	got := checkProject(t, map[string]string{
		"src/card.rtsx": `import type { Slot } from "@reactogenic/core";
export interface CardProps {
  $Title: Slot<{ children?: string }>;
  $Footer: Slot<{ children?: string }>;
  $Badge?: Slot<{ children?: string }>;
}
export function Card({ $Title, $Footer, $Badge }: CardProps) {
  return (
    <article>
      <h2 slot={$Title} />
      <footer slot={$Footer}>Default</footer>
      <i slot={$Badge} />
    </article>
  );
}
`,
		"src/page.rtsx": `import { Match } from "@reactogenic/core";
import { Card } from "./card";
declare const c: boolean;
export const bad = (
  <Card>
    <Match on={c}><$Title>Hi</$Title></Match>
    <$Footer />
  </Card>
);
export const fine = (
  <Card>
    <$Title>Always</$Title>
    <Match on={c}><$Title>Sometimes</$Title></Match>
    <Match on={c}><$Footer>Sometimes</$Footer></Match>
    <Match on={c}><$Badge>New</$Badge></Match>
  </Card>
);
export const later = (
  <Card>
    <Match on={c}><$Title>Sometimes</$Title></Match>
    <$Title>Always</$Title>
    <$Footer />
  </Card>
);
`,
	})
	want := []string{"src/page.rtsx:6:5 slot-conditional"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// syntax.md, *Key functions*: `key={(args) => …}` keys each execution of an
// attachment by the slot's args; any other `key` is an entry key.
func TestKeyFunctions(t *testing.T) {
	got := checkProject(t, map[string]string{"src/select.rtsx": `import type { Slot } from "@reactogenic/core";
interface SelectProps {
  options: { value: string; label: string }[];
  $Option?: Slot<{ value?: string; children?: string }, { value: string; label: string }>;
  $Hint?: Slot<{ children?: string }>;
}
export function Select({ options, $Option, $Hint }: SelectProps) {
  return (
    <select>
      {options.map((option, i) => <option key={i} slot={$Option} &&value={option.value} &label={option.label} />)}
      <small slot={$Hint} />
    </select>
  );
}
declare function getKey(args: { value: string }): string;
export const ok = (
  <Select options={[]}>
    <$Option key={({ value }) => value} { label }>{label}</$Option>
  </Select>
);
export const noArgs = (
  <Select options={[]}>
    <$Hint key={() => "h"}>Hint</$Hint>
  </Select>
);
export const reference = (
  <Select options={[]}>
    <$Option key={getKey} { label }>{label}</$Option>
  </Select>
);
`})
	want := []string{
		"src/select.rtsx:23:12 slot-key-no-args",
		"src/select.rtsx:28:14 slot-key-inline",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// syntax.md, *Segment files*: `#name` mounts the first of name.rtsx, .tsx,
// .jsx, .ts, .js, and the import names that file — so TS checks the file the
// lookup chose, not the one its own order would (`.ts` before `.tsx`).
func TestSegmentLookup(t *testing.T) {
	got := checkProject(t, map[string]string{
		"src/page.rtsx":   "export default function Page() {\n  return <main><section #intro /><section #faq /><section #broken /></main>;\n}\n",
		"src/intro.rtsx":  "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/intro.ts":    "const notAComponent = 1;\nexport default notAComponent;\n",
		"src/faq.tsx":     "export default function Faq() {\n  return <dl />;\n}\n",
		"src/faq.ts":      "const notAComponent = 1;\nexport default notAComponent;\n",
		"src/broken.rtsx": "const notAComponent = 1;\nexport default notAComponent;\n",
	})
	want := []string{"src/page.rtsx:2:59 segment-not-component"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
