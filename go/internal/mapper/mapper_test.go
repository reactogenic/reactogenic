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
