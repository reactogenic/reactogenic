package mapper

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

const jsxTypes = `declare namespace JSX {
  interface Element {}
  interface IntrinsicElements { [name: string]: any }
}`

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
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
	return dir
}

// ide.md, *The engine* (RGP1-103): with the mapper registered, .rtsx files
// are modules of the program under their own names — found with and without
// the extension and through a `paths` alias, one module per file — and a TS
// error inside one lands on the source, through its span map.
func TestMappedProgram(t *testing.T) {
	Register("test")
	defer rtsx.RegisterMapper(nil)

	dir := writeProject(t, map[string]string{
		"tsconfig.json": `{
  "compilerOptions": { "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true, "paths": { "@/*": ["./src/*"] } },
  "include": ["src"],
  "contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]
}`,
		"src/jsx.d.ts": jsxTypes,
		"src/button.rtsx": `export function Button({ size }: { size: number }) {
  return <button>{size}</button>;
}
`,
		"src/page.rtsx": `import { Button } from "./button";
import { Button as Explicit } from "./button.rtsx";
import { Button as Aliased } from "@/button";
import { twice } from "./util";
export function Page() {
  const size: string = "lg";
  return <main><Button size /><Explicit size={twice(1)} /><Aliased size={2} /></main>;
}
`,
		"src/util.ts": `export const twice = (n: number) => n * 2;
`,
		"src/main.tsx": `import { Page } from "./page";
import { Page as P2 } from "./page.rtsx";
export const app = [<Page />, <P2 />];
`,
	})
	program, configDiagnostics := rtsx.NewProgram(dir+"/tsconfig.json", dir, rtsx.OSFS())
	for _, d := range configDiagnostics {
		t.Errorf("config: %s", rtsx.Message(d))
	}

	// One module per file, each under its own name.
	var names []string
	for _, f := range program.GetSourceFiles() {
		if name := f.FileName(); strings.HasPrefix(name, dir+"/src/") {
			names = append(names, strings.TrimPrefix(name, dir+"/src/"))
		}
	}
	sort.Strings(names)
	if got := strings.Join(names, " "); got != "button.rtsx jsx.d.ts main.tsx page.rtsx util.ts" {
		t.Errorf("source files: %s", got)
	}

	// The one mistake — `size` is a string — is reported in page.rtsx at
	// the shorthand `size`, line 7.
	var got []string
	for _, d := range rtsx.AllDiagnostics(program) {
		file := d.File()
		if file == nil {
			got = append(got, rtsx.Message(d))
			continue
		}
		pos := d.Pos()
		if source, spans, _, ok := rtsx.MappedFile(file); ok {
			pos, _, _ = rtsx.SpanSource(spans, rtsx.SkipTrivia(file.Text(), d.Pos()), d.End())
			line := 1 + strings.Count(source[:pos], "\n")
			got = append(got, fmt.Sprintf("%s:%d:%d TS%d", filepath.Base(file.FileName()), line, pos-strings.LastIndex(source[:pos], "\n"), d.Code()))
			continue
		}
		got = append(got, fmt.Sprintf("%s TS%d %s", filepath.Base(file.FileName()), d.Code(), rtsx.Message(d)))
	}
	if strings.Join(got, "|") != "page.rtsx:7:24 TS2322" {
		t.Errorf("diagnostics: %q", got)
	}

	// The transpiler's output travels with the file.
	for _, f := range program.GetSourceFiles() {
		if strings.HasSuffix(f.FileName(), "page.rtsx") {
			if file, ok := Of(f); !ok || file.Stopped || len(file.Shorthands) != 1 {
				t.Errorf("page.rtsx carries %+v", file)
			}
		}
	}
}

// request is a mapper request over files in memory.
func request(files map[string]string, name string) rtsx.MapperRequest {
	return rtsx.MapperRequest{
		FileName:   name,
		Content:    files[name],
		FileExists: func(p string) bool { _, ok := files[p]; return ok },
		ReadFile:   func(p string) (string, bool) { text, ok := files[p]; return text, ok },
	}
}

func codes(f *File) string {
	var out []string
	for _, d := range f.Diagnostics {
		out = append(out, fmt.Sprintf("%d:%d %s", d.Line, d.Col, d.Code))
	}
	return strings.Join(out, ", ")
}

// ide.md, *Segments*: the transform is a function of the file's text and of
// which siblings exist — what its cache key holds. A sibling's contents
// change nothing: `segment-self`, which follows mounts through other files,
// is not the transform's to report.
func TestTransformReadsNoSibling(t *testing.T) {
	page := "export default function Page() {\n  return <main #intro />;\n}\n"
	var results []rtsx.MapperResult
	for _, intro := range []string{
		"export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"export default function Intro() {\n  return <nav #page />;\n}\n", // mounts the page back
	} {
		files := map[string]string{"/src/page.rtsx": page, "/src/intro.rtsx": intro}
		req := request(files, "/src/page.rtsx")
		var bits []bool
		for _, p := range depends(req) {
			bits = append(bits, req.FileExists(p))
		}
		if fmt.Sprint(bits) != "[false true false false false false]" {
			t.Fatalf("depends bits %v", bits)
		}
		results = append(results, transform(req, true))
	}
	a, b := results[0], results[1]
	if a.Text != b.Text || fmt.Sprint(a.Spans) != fmt.Sprint(b.Spans) {
		t.Errorf("virtual text or spans differ:\n%s\n---\n%s", a.Text, b.Text)
	}
	fa, fb := a.Extra.(*File), b.Extra.(*File)
	if codes(fa) != "" || codes(fb) != "" || fa.Stopped || fb.Stopped {
		t.Errorf("same key, different results: diagnostics %q / %q", codes(fa), codes(fb))
	}
	// Existence is still seen: without the sibling, segment-not-found.
	f := transform(request(map[string]string{"/src/page.rtsx": page}, "/src/page.rtsx"), true).Extra.(*File)
	if codes(f) != "2:16 segment-not-found" {
		t.Errorf("no sibling: %q", codes(f))
	}
}

// ide.md, *Tolerance*, rule 5: a panic in a pass leaves the previous pass's
// text and names the pass, whether the source parses or not; a panic outside
// the passes is caught here, with the source as its own virtual text.
func TestPanics(t *testing.T) {
	for _, broken := range []string{"", "const b = ;\n"} {
		files := map[string]string{"/src/page.rtsx": "const size = 1;\nexport const a = <section #intro size />;\n" + broken, "/src/intro.rtsx": ""}
		req := request(files, "/src/page.rtsx")
		calls := 0
		req.FileExists = func(p string) bool {
			if p == "/src/intro.rtsx" {
				if calls++; calls > 2 { // pass 0 looks twice; then pass 4
					panic("boom")
				}
			}
			_, ok := files[p]
			return ok
		}
		result := transform(req, true)
		f := result.Extra.(*File)
		if !f.Stopped || f.Err != nil || f.Output.Stopped != "pass 4 (segment roots): boom" {
			t.Errorf("broken %q: stopped %v, err %v, output stopped %q", broken, f.Stopped, f.Err, f.Output.Stopped)
		}
		if !strings.Contains(codes(f), "1:1 internal") || !strings.Contains(result.Text, "size={size}") {
			t.Errorf("broken %q: diagnostics %q, text\n%s", broken, codes(f), result.Text)
		}
		if err := rtsx.ValidateSpanMap(rtsx.NewSpanMap(result.Spans), result.Text, req.Content); err != nil {
			t.Errorf("broken %q: %v", broken, err)
		}
	}

	// The last resort: the parser refuses a name that is not normalized.
	files := map[string]string{"/src/../page.rtsx": "export const a = <b />;\n"}
	result := transform(request(files, "/src/../page.rtsx"), true)
	f := result.Extra.(*File)
	if !f.Stopped || f.Err == nil || result.Text != files["/src/../page.rtsx"] || len(result.Spans) != 1 {
		t.Errorf("last resort: stopped %v, err %v, text %q, spans %v", f.Stopped, f.Err, result.Text, result.Spans)
	}
}

// Strict, as `reactogenic check` runs it (RegisterStrict): a clean file is
// transformed exactly as the tolerant transform does it; on a file with a
// syntax error the passes do not run — it reports its syntax errors only,
// and its source is its virtual text, stopped. A panic in a pass is caught
// at the boundary.
func TestStrict(t *testing.T) {
	clean := "const size = 1;\nexport const a = <Button size><$Icon>i</$Icon></Button>;\nexport const b = <i><$Orphan /></i>;\n"
	strict, tolerant := transform(request(map[string]string{"/src/a.rtsx": clean}, "/src/a.rtsx"), false), transform(request(map[string]string{"/src/a.rtsx": clean}, "/src/a.rtsx"), true)
	fs, ft := strict.Extra.(*File), tolerant.Extra.(*File)
	if strict.Text != tolerant.Text || fmt.Sprint(strict.Spans) != fmt.Sprint(tolerant.Spans) || codes(fs) != codes(ft) || fs.Stopped || ft.Stopped || codes(fs) != "3:21 orphan-slot" {
		t.Errorf("a clean file: strict %q (%s), tolerant %q (%s)", strict.Text, codes(fs), tolerant.Text, codes(ft))
	}

	broken := clean + "export const c = <Button size><$Icon>i</$Icon><b></Button>;\n"
	result := transform(request(map[string]string{"/src/a.rtsx": broken}, "/src/a.rtsx"), false)
	f := result.Extra.(*File)
	if !f.Stopped || f.Err != nil || f.Map != nil || result.Text != broken || len(result.Spans) != 1 {
		t.Errorf("a syntax error: stopped %v, err %v, text %q, spans %v", f.Stopped, f.Err, result.Text, result.Spans)
	}
	if codes(f) != "4:48 TS17008" { // not the orphan slot of line 3: fix the syntax first
		t.Errorf("a syntax error: diagnostics %q", codes(f))
	}

	files := map[string]string{"/src/page.rtsx": "export const a = <section #intro />;\n", "/src/intro.rtsx": ""}
	req := request(files, "/src/page.rtsx")
	calls := 0
	req.FileExists = func(p string) bool {
		if p == "/src/intro.rtsx" {
			if calls++; calls > 2 { // pass 0 looks twice; then pass 4
				panic("boom")
			}
		}
		_, ok := files[p]
		return ok
	}
	result = transform(req, false)
	f = result.Extra.(*File)
	if !f.Stopped || f.Err == nil || !strings.Contains(f.Err.Error(), "boom") || result.Text != files["/src/page.rtsx"] {
		t.Errorf("a panic: stopped %v, err %v, text %q", f.Stopped, f.Err, result.Text)
	}
}

// A construct the transform had to leave out marks the file as a stopped
// one: TS's diagnostics for it are not about the author's code.
func TestDroppedIsStopped(t *testing.T) {
	src := "import { Switch } from \"@reactogenic/core\";\nexport const a = (s: string, x: string) => <Switch on={s}><$Case>{x}</$Case></Switch>;\n"
	f := transform(request(map[string]string{"/src/a.rtsx": src}, "/src/a.rtsx"), true).Extra.(*File)
	if !f.Stopped || f.Output.Stopped != "" || !f.Dropped || !strings.Contains(codes(f), "case-no-test") {
		t.Errorf("stopped %v (%q), dropped %v, diagnostics %q", f.Stopped, f.Output.Stopped, f.Dropped, codes(f))
	}
}

// A module of a referenced composite project is read from its source, as in
// the language server: an .rtsx file has no output a build could leave for
// the importer, so without that its import can never check.
func TestMappedProgramAcrossReference(t *testing.T) {
	Register("test")
	defer rtsx.RegisterMapper(nil)

	options := `"strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": []`
	dir := writeProject(t, map[string]string{
		"lib/tsconfig.json": `{ "compilerOptions": { ` + options + `, "composite": true, "outDir": "out", "rootDir": "." }, "include": ["."] }`,
		"lib/jsx.d.ts":      jsxTypes,
		"lib/button.rtsx":   "export function Button({ size }: { size: number }) {\n  return <button>{size}</button>;\n}\n",
		"lib/util.ts":       "export const twice = (n: number) => n * 2;\n",
		"app/tsconfig.json": `{ "compilerOptions": { ` + options + `, "noEmit": true }, "include": ["src"], "references": [{ "path": "../lib" }] }`,
		"app/src/jsx.d.ts":  jsxTypes,
		"app/src/page.rtsx": `import { Button } from "../../lib/button";
import { twice } from "../../lib/util";
export function Page() {
  return <main><Button size={twice("1")} /><Button size="lg" /></main>;
}
`,
	})
	program, configDiagnostics := rtsx.NewProgram(dir+"/app/tsconfig.json", dir+"/app", rtsx.OSFS())
	for _, d := range configDiagnostics {
		t.Errorf("config: %s", rtsx.Message(d))
	}
	// The two mistakes of page.rtsx, typed by lib's sources — and nothing
	// about a missing declaration file or an output that was not built.
	var got []string
	for _, d := range rtsx.AllDiagnostics(program) {
		name := "(no file)"
		if d.File() != nil {
			name = filepath.Base(d.File().FileName())
		}
		got = append(got, fmt.Sprintf("%s TS%d", name, d.Code()))
	}
	if strings.Join(got, "|") != "page.rtsx TS2345|page.rtsx TS2322" {
		t.Errorf("diagnostics: %q", got)
	}
}
