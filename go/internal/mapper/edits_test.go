package mapper

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// ide.md, *In a file whose only import is generated*: the imports the
// transform adds to a file that has none stand before the whole source. A
// position at the start of one of their lines, or right after them — where
// an editor's edit inserts a whole import — is the start of the source,
// exactly. Any other position there is inside a generated import (where a
// name would be added to it): that has no place in the source, so an edit
// there is dropped — not written at the source's start. A range — a
// diagnostic's — maps to the construct the import was generated for.
func TestStatementStartsBeforeTheSource(t *testing.T) {
	Register("test")
	defer rtsx.RegisterMapper(nil)

	const container = "export function Card({ $Title }: { $Title?: { children?: string } }) {\n  return <div><h1 slot={$Title} /></div>;\n}\n"
	leading := map[string]int{} // generated imports before the source, by file
	files := map[string]string{
		"tsconfig.json":  `{ "compilerOptions": { "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true }, "include": ["src"] }`,
		"src/jsx.d.ts":   jsxTypes,
		"src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/util.ts":    "export const twice = (n: number) => n * 2;\n",
	}
	for _, f := range []struct {
		name, text string
		imports    int
	}{
		{"src/mounter.rtsx", "export const page = <section #intro />;\n", 1},
		{"src/commented.rtsx", "// The page.\nexport const page = <section #intro />;\n", 1},
		{"src/container.rtsx", container, 1},
		{"src/both.rtsx", strings.Replace(container, "<h1 slot", "<section #intro /><h1 slot", 1), 2},
		// With an import of its own, the generated one follows it: no generated
		// text stands before the source.
		{"src/own.rtsx", "import { twice } from \"./util\";\nexport const page = <section title={String(twice(1))} #intro />;\n", 0},
	} {
		files[f.name], leading[f.name] = f.text, f.imports
	}
	dir := writeProject(t, files)
	program, _ := rtsx.NewProgram(dir+"/tsconfig.json", dir, rtsx.OSFS())
	seen := 0
	for _, f := range program.GetSourceFiles() {
		name := strings.TrimPrefix(f.FileName(), dir+"/")
		want, ok := leading[name]
		if !ok {
			continue
		}
		seen++
		source, spans, _, mapped := rtsx.MappedFile(f)
		if !mapped {
			t.Fatalf("%s is not a mapped file", name)
		}
		virtual := f.Text()
		lead := strings.Index(virtual, source[:10]) // where the source starts
		if lead < 0 || strings.Count(virtual[:lead], "import ") != want {
			t.Fatalf("%s: the source starts at %d of the virtual text; want it after %d generated imports\n%s", name, lead, want, virtual)
		}
		for at, wrong := 0, 0; at <= lead && wrong < 3; at++ {
			start := at == 0 || virtual[at-1] == '\n'
			pos, _, exact := rtsx.SpanSource(spans, at, at)
			if exact != start || (exact && pos != 0) {
				wrong++
				t.Errorf("%s: the position before %q maps to %d, exact: %v; want exact: %v, at the source's start", name, virtual[at:min(at+12, len(virtual))], pos, exact, start)
			}
		}
		if lead > 0 {
			if pos, end, exact := rtsx.SpanSource(spans, 0, len("import")); exact || pos == 0 || end <= pos {
				t.Errorf("%s: the range of the generated `import` maps to [%d, %d), exact: %v; want the construct it is for", name, pos, end, exact)
			}
		}
	}
	if seen != len(leading) {
		t.Errorf("%d of %d files are in the program", seen, len(leading))
	}
}

// ide.md, *Tolerance*, step 5: a panic inside the transform is caught at the
// mapper boundary. Whatever the file — clean, or being typed — nothing
// leaves the transform but a result: virtual text with a valid map, marked
// stopped, so that TypeScript's diagnostics for it are dropped and the
// server keeps running.
func TestPanicIsCaughtAtTheBoundary(t *testing.T) {
	boom := func(string) (string, bool) { panic("boom") }
	for name, source := range map[string]string{
		"a clean file":       "const size = 1;\nexport const a = <Button size><section #intro /></Button>;\n",
		"a file being typed": "const size = 1;\nexport const a = <Button size><section #intro /></Button>;\nexport const b = <;\n",
	} {
		t.Run(name, func(t *testing.T) {
			result := transform(rtsx.MapperRequest{
				FileName: "/p/src/page.rtsx", Content: source, ReadFile: boom,
				FileExists: func(p string) bool { _, ok := boom(p); return ok },
			})
			file, ok := result.Extra.(*File)
			if !ok || !file.Stopped {
				t.Fatalf("not marked stopped: %+v", result.Extra)
			}
			if result.Text == "" {
				t.Errorf("no virtual text")
			}
			if err := rtsx.ValidateSpanMap(rtsx.NewSpanMap(result.Spans), result.Text, source); err != nil {
				t.Errorf("the map: %v", err)
			}
			// What failed is recorded for the `internal` diagnostic.
			if why := file.Output.Stopped; !strings.Contains(why, "boom") && (file.Err == nil || !strings.Contains(file.Err.Error(), "boom")) {
				t.Errorf("the panic is not recorded: stopped %q, err %v", why, file.Err)
			}
		})
	}
}
