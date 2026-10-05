package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/checktest"
)

// The projects of the goldens are data, in internal/checktest: the language
// server's tests pull the diagnostics of the same projects.
const (
	tsconfig = checktest.TSConfig
	jsxTypes = checktest.JSXTypes
)

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
	dir := writeProject(t, checktest.Get("check").Files)
	reports := Run(dir + "/tsconfig.json")
	golden(t, dir, reports)
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

// ide.md, *The engine* and *Stock TypeScript 7.1*: a project that also runs
// stock TypeScript 7.1 lists @reactogenic/cli in tsconfig `contentMappers`.
// `check` ignores the entry: the same reports as without it — no TS18068
// for the `--runExternalCode` it does not need — and the package's command
// is not run.
func TestContentMappersEntryIgnored(t *testing.T) {
	check := func(entry string) []string {
		dir := writeProject(t, map[string]string{
			"tsconfig.json": strings.Replace(tsconfig, `"include": ["src"]`, entry+`"include": ["src"]`, 1),
			"src/jsx.d.ts":  jsxTypes,
			"src/b.rtsx":    "import { C } from \"./c\";\nconst label = 1;\nexport const b = <C label />;\nexport const o = <div><$Title>t</$Title></div>;\n",
			"src/c.tsx":     "export function C(p: { label: string }) {\n  return <div>{p.label}</div>;\n}\n",
			// A mapper package whose command, if anything ran it, would
			// leave a file behind.
			"node_modules/@reactogenic/cli/package.json": `{ "name": "@reactogenic/cli", "version": "0.0.0",
  "typescript": { "contentMapper": { "exec": ["sh", "-c", "touch spawned"] } } }`,
		})
		var got []string
		for _, r := range Run(dir + "/tsconfig.json") {
			got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
		}
		if _, err := os.Stat(filepath.Join(dir, "node_modules/@reactogenic/cli/spawned")); err == nil {
			t.Errorf("the configured mapper's command was run")
		}
		return got
	}
	plain := check("")
	if want := []string{"src/b.rtsx:3:21 TS2322", "src/b.rtsx:4:23 orphan-slot"}; strings.Join(plain, "\n") != strings.Join(want, "\n") {
		t.Fatalf("without the entry: %q, want %q", plain, want)
	}
	for name, entry := range map[string]string{
		"the package":        `{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }`,
		"an unknown package": `{ "package": "not-installed", "extensions": [".rtsx"] }`,
	} {
		if got := check(`"contentMappers": [` + entry + "],\n  "); strings.Join(got, "\n") != strings.Join(plain, "\n") {
			t.Errorf("%s: %q, without the entry %q", name, got, plain)
		}
	}
}

// A stand-in for the installed @reactogenic/core, for temporary projects.
var coreStub = checktest.Core

// checkProject writes a project of internal/checktest and returns its
// reports as "file:line:col CODE".
func checkProject(t *testing.T, name string) []string {
	t.Helper()
	dir := writeProject(t, checktest.Get(name).Files)
	reports := Run(dir + "/tsconfig.json")
	golden(t, dir, reports)
	var got []string
	for _, r := range reports {
		got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
	}
	return got
}

// syntax.md, *Slots → Attachment*: args follow function-call rules, checked
// by TS7 through renderSlot and reported on the .rtsx.
func TestSlotArgs(t *testing.T) {
	got := checkProject(t, "slot-args")
	want := []string{
		"src/button.rtsx:13:7 slot-args-missing", // a function slot without its args: `$Icon` needs `&size`
		"src/button.rtsx:15:28 slot-no-args",     // an arg to a slot whose body is not a function
		"src/button.rtsx:16:7 slot-list",         // a slot typed as an array
		"src/button.rtsx:17:24 TS2322",           // an arg of the wrong type: TS's own message
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Each mistake once (ide.md, *Diagnostics*) — and not never. `&&name` is an
// arg and a prop: one source text, emitted as the element's prop (twice,
// with a fallback) and as the arg, which is the copy that answers. A type
// error in it — in `&&name={expr}`, or a bare `&&name` that is no binding —
// is one line.
func TestArgAndProp(t *testing.T) {
	got := checkProject(t, "arg-and-prop")
	want := []string{
		"src/rows.rtsx:4:43 TS2339", // `row.nope`
		"src/rows.rtsx:7:28 TS2304", // `className`: no such name
		"src/rows.rtsx:10:43 TS2339",
		"src/rows.rtsx:13:28 TS2304",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// The attachment's children are the fallback; recursive slots and last-wins
// type-check clean.
func TestSlotsTypeCheck(t *testing.T) {
	got := checkProject(t, "slots-type-check")
	if len(got) != 0 {
		t.Errorf("got %q", got)
	}
}

// Foo.tsx next to Foo.rtsx: an import of `./Foo` finds the .tsx, and the
// transpiler reports the pair on the .rtsx (vite.md, *Module resolution*).
func TestAmbiguousModule(t *testing.T) {
	got := checkProject(t, "ambiguous-module")
	if len(got) != 1 || got[0] != "src/card.rtsx:1:1 ambiguous-module" {
		t.Errorf("got %q", got)
	}
}

// Both files of such a pair are modules of the program, each checked: the
// .rtsx is not hidden by the .tsx, as it was when it was served under the
// .tsx's name.
func TestAmbiguousModuleIsChecked(t *testing.T) {
	got := checkProject(t, "ambiguous-module-is-checked")
	want := []string{
		"src/card.rtsx:1:1 ambiguous-module",
		"src/card.rtsx:1:14 TS2322",
		"src/main.tsx:3:29 TS2322", // `a` is card.tsx's: an element
		"src/main.tsx:3:32 TS2322", // `b` is card.rtsx's: a number
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// RGP1-076: an edit of an .rtsx file is picked up and re-checked; so is a
// file created or deleted next to one — a segment's file, whose existence
// decides what its mounter emits.
func TestWatch(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/a.rtsx":    "export const a: number = 1;\n",
		"src/page.rtsx": "export const page = <main><section #intro /></main>;\n",
	})
	stop := make(chan struct{})
	runs := make(chan []Report, 8)
	go Watch(dir+"/tsconfig.json", 20*time.Millisecond, stop, func(r []Report) { runs <- r })
	defer close(stop)
	next := func(what string, want ...string) {
		t.Helper()
		select {
		case reports := <-runs:
			var got []string
			for _, r := range reports {
				got = append(got, strings.TrimPrefix(r.File, dir+"/")+":"+strconv.Itoa(r.Line)+":"+strconv.Itoa(r.Col)+" "+r.Code)
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("%s: got %q, want %q", what, got, want)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("%s: not picked up", what)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(dir+"/"+name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	next("the first run", "src/page.rtsx:1:36 segment-not-found")
	write("src/a.rtsx", "export const a: number = \"x\";\n")
	next("an edit", "src/a.rtsx:1:14 TS2322", "src/page.rtsx:1:36 segment-not-found")
	write("src/intro.rtsx", "export default function Intro({ title }: { title: string }) {\n  return <p>{title}</p>;\n}\n")
	next("a segment file created", "src/a.rtsx:1:14 TS2322", "src/page.rtsx:1:36 segment-props")
	write("src/intro.rtsx", "export default function Intro() {\n  return <p>intro</p>;\n}\n")
	next("the segment file edited", "src/a.rtsx:1:14 TS2322")
	if err := os.Remove(dir + "/src/intro.rtsx"); err != nil {
		t.Fatal(err)
	}
	next("the segment file deleted", "src/a.rtsx:1:14 TS2322", "src/page.rtsx:1:36 segment-not-found")
}

// --watch on a tsconfig that only references other projects watches their
// directories, wherever they are.
func TestWatchReferences(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"solution/tsconfig.json": `{ "files": [], "references": [{ "path": "../app" }] }`,
		"app/tsconfig.json":      tsconfig,
		"app/src/jsx.d.ts":       jsxTypes,
		"app/src/a.rtsx":         "export const a: number = 1;\n",
	})
	stop := make(chan struct{})
	runs := make(chan []Report, 4)
	go Watch(dir+"/solution/tsconfig.json", 20*time.Millisecond, stop, func(r []Report) { runs <- r })
	defer close(stop)
	if first := <-runs; len(first) != 0 {
		t.Fatalf("first run: %+v", first)
	}
	if err := os.WriteFile(dir+"/app/src/a.rtsx", []byte("export const a: number = \"x\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case second := <-runs:
		if len(second) != 1 || second[0].Code != "TS2322" || second[0].File != dir+"/app/src/a.rtsx" {
			t.Errorf("second run: %+v", second)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the edit was not picked up")
	}
}

// A referenced project that is not there is the referencing project's error
// (TS6053), once — and its directory is watched all the same: the project is
// checked when it appears.
func TestWatchMissingReference(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"solution/tsconfig.json": `{ "files": [], "references": [{ "path": "../app" }] }`,
	})
	stop := make(chan struct{})
	runs := make(chan []Report, 8)
	go Watch(dir+"/solution/tsconfig.json", 20*time.Millisecond, stop, func(r []Report) { runs <- r })
	defer close(stop)
	if first := <-runs; len(first) != 1 || first[0].Code != "TS6053" {
		t.Fatalf("first run: %+v", first)
	}
	for name, text := range map[string]string{"app/src/jsx.d.ts": jsxTypes, "app/src/a.rtsx": "export const a: number = \"x\";\n", "app/tsconfig.json": tsconfig} {
		os.MkdirAll(filepath.Dir(dir+"/"+name), 0o755)
		if err := os.WriteFile(dir+"/"+name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A run may fall between the writes: the last one sees them all.
	timeout := time.After(20 * time.Second)
	for {
		select {
		case reports := <-runs:
			if len(reports) == 1 && reports[0].Code == "TS2322" && reports[0].File == dir+"/app/src/a.rtsx" {
				return
			}
			t.Logf("a run: %+v", reports)
		case <-timeout:
			t.Fatal("the project that appeared was not checked")
		}
	}
}

// Keyed slots type-check: entries as slot values, attached by key.
func TestKeyedSlots(t *testing.T) {
	got := checkProject(t, "keyed-slots")
	// Only the typo: an excess property of an entry, on its line.
	if len(got) != 1 || !strings.HasPrefix(got[0], "src/table.rtsx:17:") {
		t.Errorf("got %q", got)
	}
}

// syntax.md, *Keyed slots*: entries are named by their keys encoded. What is
// checked does not change — an entry's props, and the key itself.
func TestKeyedKeys(t *testing.T) {
	got := checkProject(t, "keyed-keys")
	want := []string{
		"src/menu.rtsx:23:43 TS2353",          // an excess prop of an entry under a literal name (`"#10"`)
		"src/menu.rtsx:24:45 slot-type",       // … and under a computed one: TS says it of the whole slot, at the prop
		"src/menu.rtsx:25:40 slot-key-inline", // `string | undefined` is no key: on the value
		"src/menu.rtsx:26:47 TS2345",          // inside the key's expression: TypeScript's own
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// syntax.md, *Conditional slots*: a required slot that the component attaches
// without a fallback cannot be filled conditionally — its false branch is
// NOT_ASSIGNED, and nothing would render.
func TestSlotConditional(t *testing.T) {
	got := checkProject(t, "slot-conditional")
	want := []string{"src/page.rtsx:6:5 slot-conditional"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// syntax.md, *Key functions*: `key={(args) => …}` keys each execution of an
// attachment by the slot's args; any other `key` is an entry key.
func TestKeyFunctions(t *testing.T) {
	got := checkProject(t, "key-functions")
	want := []string{
		"src/select.rtsx:23:12 slot-key-no-args",
		"src/select.rtsx:28:19 slot-key-inline", // on the value: the argument of slotEntryName
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// syntax.md, *Segment files*: `#name` mounts the first of name.rtsx, .tsx,
// .jsx, .ts, .js, and the import names that file — so TS checks the file the
// lookup chose, not the one its own order would (`.ts` before `.tsx`).
func TestSegmentLookup(t *testing.T) {
	got := checkProject(t, "segment-lookup")
	want := []string{"src/page.rtsx:2:59 segment-not-component"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
