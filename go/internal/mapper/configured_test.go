package mapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// ide.md, *The engine*: built in, not configured. A tsconfig that lists
// @reactogenic/cli in `contentMappers` — as a project that also runs stock
// TypeScript 7.1 does (*Stock TypeScript 7.1*) — changes nothing in a
// program of ours: no TS18068 for the missing `--runExternalCode`, the same
// diagnostics, and the package's command is not run.
func TestContentMappersEntryIgnored(t *testing.T) {
	Register("test")
	defer rtsx.RegisterMapper(nil)

	const options = `"compilerOptions": { "strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true },
  "include": ["src"]`
	check := func(tsconfig string) []string {
		dir := writeProject(t, map[string]string{
			"tsconfig.json": tsconfig,
			"src/jsx.d.ts":  jsxTypes,
			"src/button.rtsx": `export function Button({ size }: { size: number }) {
  return <button>{size}</button>;
}
`,
			"src/page.rtsx": `import { Button } from "./button";
export function Page() {
  const size: string = "lg";
  return <main><Button size /></main>;
}
`,
			// A mapper package whose command, if anything ran it, would
			// leave a file behind.
			"node_modules/@reactogenic/cli/package.json": `{ "name": "@reactogenic/cli", "version": "0.0.0",
  "typescript": { "contentMapper": { "exec": ["sh", "-c", "touch spawned"] } } }`,
		})
		program, configDiagnostics := rtsx.NewProgram(dir+"/tsconfig.json", dir, rtsx.OSFS())
		var got []string
		for _, d := range configDiagnostics {
			got = append(got, fmt.Sprintf("config TS%d %s", d.Code(), rtsx.Message(d)))
		}
		for _, d := range rtsx.AllDiagnostics(program) {
			name := "-"
			if d.File() != nil {
				name = filepath.Base(d.File().FileName())
			}
			got = append(got, fmt.Sprintf("%s TS%d %s", name, d.Code(), rtsx.Message(d)))
		}
		if _, err := os.Stat(filepath.Join(dir, "node_modules/@reactogenic/cli/spawned")); err == nil {
			t.Errorf("the configured mapper's command was run")
		}
		return got
	}

	plain := check("{\n  " + options + "\n}")
	if len(plain) != 1 || !strings.HasPrefix(plain[0], "page.rtsx TS2322 ") {
		t.Errorf("diagnostics without the entry: %q", plain)
	}
	for name, entry := range map[string]string{
		"the package":        `{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }`,
		"with options":       `{ "package": "@reactogenic/cli", "extensions": [".rtsx"], "options": { "anything": true } }`,
		"an unknown package": `{ "package": "not-installed", "extensions": [".rtsx"] }`,
	} {
		got := check("{\n  " + options + ",\n  \"contentMappers\": [" + entry + "]\n}")
		if strings.Join(got, "\n") != strings.Join(plain, "\n") {
			t.Errorf("%s: diagnostics\n%s\nwithout the entry\n%s", name, strings.Join(got, "\n"), strings.Join(plain, "\n"))
		}
	}
}
