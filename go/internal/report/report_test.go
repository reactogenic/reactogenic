package report

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
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

// A stand-in for the installed @reactogenic/core: what the emitted code of
// these tests calls.
const core = `type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol };
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
export type Slot<Props> = Props | NotAssigned;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export declare function renderSlot<S extends object>(slot: S, args: object, fallback?: ReactNode): ReactNode;
export declare function Match(props: { on: unknown; children?: unknown }): never;`

// program writes a project and builds its program, with the transform
// registered for the test: tolerant, as the server runs it, or strict, as
// `check` does.
func program(t *testing.T, tolerant bool, files map[string]string) (*rtsx.Program, string) {
	t.Helper()
	if tolerant {
		mapper.Register("tolerant")
	} else {
		mapper.RegisterStrict("strict")
	}
	t.Cleanup(func() { rtsx.RegisterMapper(nil) })
	return build(t, files)
}

func build(t *testing.T, files map[string]string) (*rtsx.Program, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.ToSlash(dir)
	for name, text := range map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"node_modules/@reactogenic/core/package.json": `{ "name": "@reactogenic/core", "types": "index.d.ts" }`,
		"node_modules/@reactogenic/core/index.d.ts":   core,
	} {
		if _, ok := files[name]; !ok {
			files[name] = text
		}
	}
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, config := rtsx.NewProgram(dir+"/tsconfig.json", dir, rtsx.OSFS())
	for _, d := range config {
		t.Fatalf("tsconfig: %s", rtsx.Message(d))
	}
	return p, dir
}

// lines prints reports as "file:line:col severity CODE", relative to dir.
func lines(dir string, reports []Report) []string {
	var out []string
	for _, r := range reports {
		out = append(out, fmt.Sprintf("%s:%d:%d %s %s", strings.TrimPrefix(r.File, dir+"/src/"), r.Line, r.Col, r.Severity, r.Code))
	}
	return out
}

func same(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n  %s\nwant:\n  %s", what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// file returns the reports of one file of the program, and what the
// transform attached to it.
func file(t *testing.T, p *rtsx.Program, name string) ([]Report, *mapper.File) {
	t.Helper()
	f := p.GetSourceFile(name)
	if f == nil {
		t.Fatalf("%s is not in the program", name)
	}
	attached, _ := mapper.Of(f)
	return File(context.Background(), p, f), attached
}

// ide.md, *Tolerance*, rule 4: for a stopped file every TS diagnostic of the
// file is dropped; the transpiler's diagnostics remain, and the rest of the
// program is reported as ever.
func TestStoppedFile(t *testing.T) {
	// Code was left out: an orphaned slot element that holds an expression
	// becomes `null`, so `title` — used only there — would read as unused,
	// and the line below it has a type error TS would report.
	const page = `export function Page({ title }: { title: string }) {
  const n: number = "x";
  return <div><$Title>{title}</$Title>{n}</div>;
}
`
	const main = "import { Page } from \"./page\";\nexport const app = <Page title={1} />;\n"
	for _, tolerant := range []bool{true, false} {
		p, dir := program(t, tolerant, map[string]string{
			"tsconfig.json": strings.Replace(tsconfig, `"noEmit": true`, `"noEmit": true, "noUnusedParameters": true`, 1),
			"src/page.rtsx": page,
			"src/main.tsx":  main,
		})
		reports, f := file(t, p, dir+"/src/page.rtsx")
		if !f.Stopped || !f.Dropped {
			t.Fatalf("tolerant %v: the file is not stopped: %+v", tolerant, f)
		}
		same(t, fmt.Sprintf("tolerant %v, page.rtsx", tolerant), lines(dir, reports), []string{"page.rtsx:3:15 error orphan-slot"})
		// The whole program: the stopped file's module still has its
		// exports, and its importer is checked against them.
		same(t, fmt.Sprintf("tolerant %v, the program", tolerant), lines(dir, Program(p, nil)), []string{
			"main.tsx:2:26 error TS2322",
			"page.rtsx:3:15 error orphan-slot",
		})
	}
}

// The same file with nothing left out is not stopped: TS's diagnostics are
// reported next to the transpiler's. (An orphaned slot of text alone holds
// no code.)
func TestNotStopped(t *testing.T) {
	p, dir := program(t, true, map[string]string{
		"src/page.rtsx": "export function Page({ title }: { title: string }) {\n  const n: number = \"x\";\n  return <div><$Title>t</$Title>{n}{title}</div>;\n}\n",
	})
	reports, f := file(t, p, dir+"/src/page.rtsx")
	if f.Stopped {
		t.Fatalf("stopped: %+v", f)
	}
	same(t, "page.rtsx", lines(dir, reports), []string{"page.rtsx:2:9 error TS2322", "page.rtsx:3:15 error orphan-slot"})
}

// A file with a syntax error. Tolerant (the editor): the passes run on the
// recovered tree; the syntax errors are the source parse's, TS's own for the
// virtual text are never shown (rule 2), and — nothing being left out — TS's
// type errors are. Strict (`check`): the passes do not run, the source stands
// in as the virtual text, and the file reports its syntax errors only. Either
// way the module keeps its exports for its importers.
func TestSyntaxError(t *testing.T) {
	files := func() map[string]string {
		return map[string]string{
			"src/page.rtsx": "export const n: number = \"x\";\nexport function Page() {\n  return <div><span></div>;\n}\n",
			"src/main.tsx":  "import { Page, n } from \"./page\";\nexport const s: string = n;\nexport const app = <Page />;\n",
		}
	}
	p, dir := program(t, true, files())
	reports, f := file(t, p, dir+"/src/page.rtsx")
	if f.Stopped {
		t.Fatalf("tolerant: stopped: %+v", f)
	}
	same(t, "tolerant, page.rtsx", lines(dir, reports), []string{"page.rtsx:1:14 error TS2322", "page.rtsx:3:16 error TS17008"})
	for _, r := range reports {
		if r.Code == "TS17008" && r.TS != nil {
			t.Errorf("the syntax error is TS's, of the virtual text: %+v", r)
		}
	}
	same(t, "tolerant, the program", lines(dir, Program(p, nil)), []string{
		"main.tsx:2:14 error TS2322", "page.rtsx:1:14 error TS2322", "page.rtsx:3:16 error TS17008",
	})

	p, dir = program(t, false, files())
	reports, f = file(t, p, dir+"/src/page.rtsx")
	if !f.Stopped || f.Map != nil {
		t.Fatalf("strict: the source is not its own virtual text: %+v", f)
	}
	same(t, "strict, page.rtsx", lines(dir, reports), []string{"page.rtsx:3:16 error TS17008"})
	// The virtual text's syntax errors stop nothing: main.tsx is checked.
	same(t, "strict, the program", lines(dir, Program(p, nil)), []string{"main.tsx:2:14 error TS2322", "page.rtsx:3:16 error TS17008"})
}

// A failure of the transpiler itself is one `internal` report on the first
// line, and the file is stopped (ide.md, *Tolerance*, rule 5).
func TestInternalError(t *testing.T) {
	rtsx.RegisterMapper(&rtsx.Mapper{Name: "reactogenic", Version: "failing", Extension: ".rtsx", Transform: func(req rtsx.MapperRequest) rtsx.MapperResult {
		n := int32(len(req.Content))
		return rtsx.MapperResult{Text: req.Content, Spans: [][6]int32{{0, n, 0, n, 0, int32(emit.AllFeatures)}},
			Extra: &mapper.File{Stopped: true, Err: errors.New("transpiler: pass 3 (slot hoisting): boom")}}
	}})
	t.Cleanup(func() { rtsx.RegisterMapper(nil) })
	p, dir := build(t, map[string]string{"src/page.rtsx": "export const n: number = \"x\";\n"})
	reports, _ := file(t, p, dir+"/src/page.rtsx")
	same(t, "page.rtsx", lines(dir, reports), []string{"page.rtsx:1:1 error internal"})
	if len(reports) == 1 && reports[0].Message != "transpiler: pass 3 (slot hoisting): boom" {
		t.Errorf("message: %q", reports[0].Message)
	}
}

// ide.md, *Diagnostics*: one layer for both hosts. For every file, what the
// per-file form returns — the editor's — is what the whole-program form —
// `check`'s — has for that file, suggestions aside: transpiler diagnostics
// and warnings, rewrites, the cross-file rules, the TS5097 drop, the merged
// duplicate. In a project with `noEmit` and in a composite one that emits
// declarations: the per-file form asks for declaration diagnostics there.
func TestFileEqualsProgram(t *testing.T) {
	for name, config := range map[string]string{
		"noEmit":    tsconfig,
		"composite": strings.Replace(tsconfig, `"noEmit": true`, `"composite": true, "emitDeclarationOnly": true, "outDir": "out"`, 1),
	} {
		t.Run(name, func(t *testing.T) { fileEqualsProgram(t, config) })
	}
}

func fileEqualsProgram(t *testing.T, config string) {
	p, dir := program(t, true, map[string]string{
		"tsconfig.json": config,
		"src/card.rtsx": `import type { Slot } from "@reactogenic/core";
export function Card({ $Title, $Label }: { $Title: Slot<{ children?: string }>; $Label?: Slot<{ title?: string; children?: string }> }) {
  return <article><h2 slot={$Title} /><b slot={$Label} title={$Label.nope}>Label</b></article>;
}
`,
		"src/page.rtsx": `import { Match } from "@reactogenic/core";
import { Card } from "./card";
declare const c: boolean;
export default function Page() {
  const unused = 1;
  return (
    <main>
      <Card><Match on={c}><$Title>Hi</$Title></Match></Card>
      <Card><$Nope>x</$Nope><$Title>t</$Title></Card>
      <section #intro>old</section>
      <section #faq />
      <i><$Orphan /></i>
    </main>
  );
}
`,
		"src/intro.rtsx": "export default function Intro() {\n  return <div><section #page /></div>;\n}\n",
		"src/faq.tsx":    "export default function Faq() {\n  return <dl />;\n}\n",
		"src/main.tsx":   "import Page from \"./page\";\nexport const app: number = <Page />;\n",
	})
	whole := Program(p, nil)
	same(t, "the program", lines(dir, whole), []string{
		"card.rtsx:3:63 error TS18048",       // `$Label` in the fallback branch
		"card.rtsx:3:70 error TS2339",        // `.nope`: in both branches, once
		"intro.rtsx:2:24 error segment-self", // through page.rtsx
		"main.tsx:2:14 error TS2322",         // a file that is not mapped, as it is
		"page.rtsx:8:13 error slot-conditional",
		"page.rtsx:9:14 error undeclared-slot",
		"page.rtsx:10:16 warning segment-children",
		"page.rtsx:10:16 error segment-self", // through intro.rtsx
		"page.rtsx:12:10 error orphan-slot",
		// `#faq` mounts faq.tsx: no TS5097 on the generated `./faq.tsx`.
	})
	for _, f := range p.GetSourceFiles() {
		if !strings.HasPrefix(f.FileName(), dir+"/src/") {
			continue
		}
		var want, got []string
		for _, r := range whole {
			if r.File == f.FileName() {
				want = append(want, fmt.Sprintf("%d:%d %s %s: %s", r.Line, r.Col, r.Severity, r.Code, r.Message))
			}
		}
		suggestions := 0
		for _, r := range File(context.Background(), p, f) {
			if r.Severity == Suggestion {
				// Suggestions pass through untouched: TS's code, its diagnostic.
				if suggestions++; r.TS == nil || !strings.HasPrefix(r.Code, "TS") {
					t.Errorf("suggestion: %+v", r)
				}
				continue
			}
			got = append(got, fmt.Sprintf("%d:%d %s %s: %s", r.Line, r.Col, r.Severity, r.Code, r.Message))
		}
		same(t, f.FileName(), got, want)
		if strings.HasSuffix(f.FileName(), "/page.rtsx") && suggestions == 0 {
			t.Errorf("page.rtsx: no suggestion for the unused `unused`")
		}
	}
}

// A report's Span is in the source: the text it covers is what the author
// wrote, for a copied expression and for a construct alike.
func TestSpans(t *testing.T) {
	const page = "import { Card } from \"./card\";\nconst size: string = \"lg\";\nexport const a = <Card size />;\nexport const b = <Card size={1}><$Nope>x</$Nope></Card>;\n"
	p, dir := program(t, true, map[string]string{
		"src/card.rtsx": "export function Card({ size }: { size: number }) {\n  return <b>{size}</b>;\n}\n",
		"src/page.rtsx": page,
	})
	reports, _ := file(t, p, dir+"/src/page.rtsx")
	var got []string
	for _, r := range reports {
		got = append(got, fmt.Sprintf("%s %q", r.Code, page[r.Span.Pos:r.Span.End]))
		for _, rel := range r.Related {
			if rel.File != dir+"/src/card.rtsx" || rel.Line != 1 {
				t.Errorf("related information of %s: %+v", r.Code, rel)
			}
		}
	}
	same(t, "spans", got, []string{`TS2322 "size"`, `undeclared-slot "$Nope"`})
}

// ide.md, *Each mistake once*.
func TestOnce(t *testing.T) {
	at := emit.Span{Pos: 10, End: 14}
	report := func(code, message string, copy int, secondary bool) Report {
		return Report{Span: at, Code: code, Message: message, copy: copy, secondary: secondary}
	}
	for name, c := range map[string]struct {
		in   []Report
		want string
	}{
		"same range, code and message: one": {
			[]Report{report("TS2322", "a", 1, false), report("TS2322", "a", 2, true), report("TS2322", "a", 0, false)}, "TS2322 a"},
		"a secondary copy with the primary's code: dropped, whatever its message": {
			[]Report{report("TS2339", "narrowed", 1, false), report("TS2339", "not narrowed", 2, true)}, "TS2339 narrowed"},
		"the primary copy is kept wherever it comes": {
			[]Report{report("TS2339", "not narrowed", 1, true), report("TS2339", "narrowed", 2, false)}, "TS2339 narrowed"},
		// `&&name={expr}`: the element's prop is emitted before the arg, which
		// is the copy that answers — and TS says the same of both.
		"a secondary copy before the primary one, the same message: the primary": {
			[]Report{report("TS2339", "same", 1, true), report("TS2339", "same", 2, false)}, "TS2339 same"},
		// … with a fallback: the prop once more, in the other branch.
		"secondary, primary, secondary, one message: one": {
			[]Report{report("TS2339", "same", 1, true), report("TS2339", "same", 2, false), report("TS2339", "same", 3, true)}, "TS2339 same"},
		"secondary, secondary, primary, one message: one": {
			[]Report{report("TS2339", "same", 1, true), report("TS2339", "same", 3, true), report("TS2339", "same", 2, false)}, "TS2339 same"},
		"secondary copies only, one message: one": {
			[]Report{report("TS2339", "same", 2, true), report("TS2339", "same", 3, true)}, "TS2339 same"},
		"secondary copies only: the first": {
			[]Report{report("TS2339", "x", 2, true), report("TS2339", "y", 3, true)}, "TS2339 x"},
		"a secondary copy with a code of its own: kept": {
			[]Report{report("TS2339", "x", 1, false), report("TS18048", "y", 2, true)}, "TS2339 x|TS18048 y"},
		"two copies that both answer (a shorthand's name and value): both": {
			[]Report{report("TS2322", "the prop", 1, false), report("TS2322", "the binding", 2, false)}, "TS2322 the prop|TS2322 the binding"},
		"synthesized text, one construct, two mistakes: both": {
			[]Report{report("TS2322", "x", 0, false), report("TS2322", "y", 0, false)}, "TS2322 x|TS2322 y"},
		"a secondary copy and synthesized text: unrelated": {
			[]Report{report("TS2322", "x", 0, false), report("TS2322", "y", 2, true)}, "TS2322 x|TS2322 y"},
		"another range": {
			[]Report{report("TS2339", "x", 1, false), {Span: emit.Span{Pos: 20, End: 24}, Code: "TS2339", Message: "x", copy: 2, secondary: true}}, "TS2339 x|TS2339 x"},
	} {
		var got []string
		for _, r := range once(c.in) {
			got = append(got, r.Code+" "+r.Message)
		}
		if strings.Join(got, "|") != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}

// *Each mistake once*, through the whole layer: `&&name` is an arg and a
// prop — one source text, emitted as the element's prop (in each branch,
// with a fallback) and as the arg. A type error in it is reported once — not
// twice, and not never — by both forms of the layer and in both modes.
func TestArgAndProp(t *testing.T) {
	// What an attachment with args calls, besides the stand-in's.
	const coreWithArgs = `type ReactNode = string | number | boolean | null | undefined | { readonly $$typeof: symbol };
export declare const NOT_ASSIGNED: unique symbol;
export type NotAssigned = typeof NOT_ASSIGNED;
type Key = string | number | bigint;
export type Slot<Props, Args = never> = (Omit<Props, "children"> & { children?: (args: Args) => ReactNode }) | NotAssigned;
export declare function slotArgs<S>(slot: S, args: object): object;
export declare function slotKey(slot: unknown, args: object, fallback?: Key | null): Key | null | undefined;
export declare function isAssigned<S>(slot: S): slot is Exclude<S, NotAssigned | undefined | null>;
export declare function slotProps<S extends object>(slot: S): S;
export declare function renderSlot<S extends object>(slot: S, args: object, fallback?: ReactNode): ReactNode;`
	const rows = `import type { Slot } from "@reactogenic/core";
type P = { $Row: Slot<{ className?: string }, { className: string }>; row: { id: string } };
export function A({ $Row, row }: P) {
  return <tr slot={$Row} &&className={row.nope} />;
}
export function B({ $Row }: P) {
  return <tr slot={$Row} &&className />;
}
export function C({ $Row, row }: P) {
  return <tr slot={$Row} &&className={row.nope}>fallback</tr>;
}
export function D({ $Row }: P) {
  return <tr slot={$Row} &&className>fallback</tr>;
}
`
	want := []string{
		"rows.rtsx:4:43 error TS2339", // `row.nope`
		"rows.rtsx:7:28 error TS2304", // `className`: no such name
		"rows.rtsx:10:43 error TS2339",
		"rows.rtsx:13:28 error TS2304",
	}
	for _, tolerant := range []bool{true, false} {
		p, dir := program(t, tolerant, map[string]string{"src/rows.rtsx": rows, "node_modules/@reactogenic/core/index.d.ts": coreWithArgs})
		reports, _ := file(t, p, dir+"/src/rows.rtsx")
		var errors []Report
		for _, r := range reports {
			if r.Severity == Error {
				errors = append(errors, r)
			}
		}
		same(t, fmt.Sprintf("tolerant %v, File", tolerant), lines(dir, errors), want)
		same(t, fmt.Sprintf("tolerant %v, Program", tolerant), lines(dir, Program(p, nil)), want)
	}
}

// ide.md, *Tolerance*, rule 4: a construct that is an error where it stands
// and that no pass lowers — an arg on an element without `slot={$X}`, params
// on an intrinsic element — is in the virtual text as written, which is then
// not TSX. TS reads `<option &size />` as `<option` `& size / >`: what it
// says about that file is dropped, like its syntax errors. The file is
// stopped; its importers are checked.
func TestUnlowered(t *testing.T) {
	for name, c := range map[string]struct {
		source string
		want   []string
	}{
		"arg-without-slot": {
			"declare const size: number;\ndeclare const v: number;\nexport const a = <option &size />;\nexport const b = <option slot=\"header\" &&value={v} />;\nexport const n: number = \"x\";\n",
			[]string{"page.rtsx:3:26 error arg-without-slot", "page.rtsx:4:40 error arg-without-slot"},
		},
		"params-on-html": {
			"declare const size: number;\nexport const a = <div { size }>body</div>;\nexport const n: number = \"x\";\n",
			[]string{"page.rtsx:2:23 error params-on-html"},
		},
	} {
		for _, tolerant := range []bool{true, false} {
			what := fmt.Sprintf("%s, tolerant %v", name, tolerant)
			p, dir := program(t, tolerant, map[string]string{
				"src/page.rtsx": c.source,
				"src/main.tsx":  "import { n } from \"./page\";\nexport const s: string = n;\n",
			})
			reports, f := file(t, p, dir+"/src/page.rtsx")
			if !f.Stopped || !f.Unlowered {
				t.Errorf("%s: stopped %v, unlowered %v", what, f.Stopped, f.Unlowered)
			}
			// The premise: TS does not parse that text.
			if len(p.GetSyntacticDiagnostics(context.Background(), p.GetSourceFile(dir+"/src/page.rtsx"))) == 0 {
				t.Errorf("%s: the virtual text is TSX:\n%s", what, f.TSX)
			}
			same(t, what+", File", lines(dir, reports), c.want)
			same(t, what+", Program", lines(dir, Program(p, nil)), append([]string{"main.tsx:2:14 error TS2322"}, c.want...))
		}
	}
}

// The errors that only an emit of declarations finds (TS4094: a private
// member of an exported anonymous class) are in both forms of the layer, in
// a project that emits as in one with `noEmit`: `check` never emits, so
// nothing else would report them.
func TestDeclarationDiagnostics(t *testing.T) {
	for name, emit := range map[string]string{
		"a composite project that emits": `"composite": true, "emitDeclarationOnly": true, "outDir": "out"`,
		"declarations under noEmit":      `"noEmit": true, "declaration": true`,
	} {
		p, dir := program(t, false, map[string]string{
			"tsconfig.json": strings.Replace(tsconfig, `"noEmit": true`, emit, 1),
			"src/a.rtsx":    "export const C = class { private x = 1 };\nexport const el = <div />;\n",
			"src/b.ts":      "export const D = class { private y = 1 };\n",
		})
		want := []string{"a.rtsx:1:14 error TS4094", "b.ts:1:14 error TS4094"}
		same(t, name+", Program", lines(dir, Program(p, nil)), want)
		var got []string
		for _, name := range []string{"a.rtsx", "b.ts"} {
			reports, _ := file(t, p, dir+"/src/"+name)
			for _, r := range reports {
				if r.Severity != Suggestion {
					got = append(got, lines(dir, []Report{r})...)
				}
			}
		}
		same(t, name+", File", got, want)
	}

	// Where the two forms still differ: the program's follows `tsc`'s steps —
	// declaration errors are a later step than type errors, reported while
	// there are none — and the per-file form has no steps, as TS's own
	// language server has none.
	p, dir := program(t, false, map[string]string{
		"tsconfig.json": strings.Replace(tsconfig, `"noEmit": true`, `"noEmit": true, "declaration": true`, 1),
		"src/a.rtsx":    "export const C = class { private x = 1 };\nexport const n: number = \"x\";\n",
	})
	same(t, "next to a type error, Program", lines(dir, Program(p, nil)), []string{"a.rtsx:2:14 error TS2322"})
	reports, _ := file(t, p, dir+"/src/a.rtsx")
	reports = slices.DeleteFunc(reports, func(r Report) bool { return r.Severity == Suggestion })
	same(t, "next to a type error, File", lines(dir, reports), []string{"a.rtsx:1:14 error TS4094", "a.rtsx:2:14 error TS2322"})
}
