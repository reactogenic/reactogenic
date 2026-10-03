package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// options is the compilerOptions object of the suite's tsconfig, for
// projects that write their own.
const options = `"strict": true, "jsx": "preserve", "module": "esnext", "moduleResolution": "bundler", "target": "es2022", "lib": ["es2022"], "types": [], "noEmit": true`

// output is what `reactogenic check --pretty=false -p config` prints, with
// the project's directory as `<root>`.
func output(dir, config string) string {
	var b bytes.Buffer
	Print(&b, Run(dir+"/"+config), dir, false, readFile)
	return strings.ReplaceAll(b.String(), dir, "<root>")
}

// goldenText compares text with testdata/golden/<test name>.txt.
func goldenText(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+".txt")
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("no golden output for %s: run `go test ./internal/check -update`", t.Name())
		return
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\n--- got\n%s--- want\n%s", path, got, want)
	}
}

// with adds the core stub and the JSX types to a project's files.
func with(files map[string]string) map[string]string {
	for k, v := range coreStub {
		files[k] = v
	}
	if _, ok := files["src/jsx.d.ts"]; !ok {
		files["src/jsx.d.ts"] = jsxTypes
	}
	return files
}

// An .rtsx file is a module of the program under its own name (ide.md, *The
// engine*): the shapes of project that the overlay did not cover, or covered
// differently.
func TestProjects(t *testing.T) {
	// A project with nothing but .rtsx files has inputs: no TS18003.
	t.Run("rtsx-only", func(t *testing.T) {
		dir := writeProject(t, map[string]string{
			"tsconfig.json": tsconfig,
			"src/a.rtsx":    "import { b } from \"./b\";\nexport const a: string = b;\n",
			"src/b.rtsx":    "export const b = 1;\n",
		})
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Contains(got, "TS18003") || !strings.Contains(got, "src/a.rtsx(2,14): error TS2322") {
			t.Errorf("got:\n%s", got)
		}
	})

	// Vite's template: the root tsconfig has no files, only references. Each
	// referenced project is checked; the root prints what `-p
	// tsconfig.app.json` and `-p tsconfig.node.json` print.
	t.Run("vite-template", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }] }`,
			"tsconfig.app.json":  `{ "compilerOptions": { ` + options + ` }, "include": ["src"] }`,
			"tsconfig.node.json": `{ "compilerOptions": { ` + options + ` }, "include": ["vite.config.ts"] }`,
			"vite.config.ts":     "export default { port: 1 };\n",
			"src/main.tsx":       "import { Page } from \"./page\";\nexport const app = <Page title={1} />;\n",
			"src/page.rtsx":      "export function Page({ title }: { title: string }) {\n  const n: number = title;\n  return <h1>{n}</h1>;\n}\n",
		}))
		root, app, node := output(dir, "tsconfig.json"), output(dir, "tsconfig.app.json"), output(dir, "tsconfig.node.json")
		goldenText(t, root)
		if node != "" {
			t.Errorf("tsconfig.node.json:\n%s", node)
		}
		if root != app || !strings.Contains(app, "src/page.rtsx(2,9): error TS2322") {
			t.Errorf("the root prints:\n%s`-p tsconfig.app.json` prints:\n%s", root, app)
		}
	})

	// A file of two referenced projects is reported once, by the first.
	t.Run("references-shared-file", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.test.json" }] }`,
			"tsconfig.app.json":  `{ "compilerOptions": { ` + options + ` }, "include": ["src"] }`,
			"tsconfig.test.json": `{ "compilerOptions": { ` + options + ` }, "include": ["src", "test"] }`,
			"src/page.rtsx":      "export const page: number = <p><$Title /></p>;\n",
			"test/page.test.tsx": "import { page } from \"../src/page\";\nexport const s: string = page;\n",
		}))
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Count(got, "src/page.rtsx") != 2 || strings.Count(got, "orphan-slot") != 1 || strings.Count(got, "test/page.test.tsx") != 1 {
			t.Errorf("got:\n%s", got)
		}
	})

	// An .rtsx module of a referenced project is read from its source — it
	// has no output a build could have left. The referenced project is
	// checked first, and its files are its own to report, once and with its
	// own options: lib is not strict, so `pick`'s untyped parameters are no
	// error, though app, which is strict, has the file in its program too.
	t.Run("cross-project", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":       `{ "files": [], "references": [{ "path": "./lib" }, { "path": "./app" }] }`,
			"lib/tsconfig.json":   `{ "compilerOptions": { ` + strings.Replace(options, `"strict": true`, `"strict": false`, 1) + `, "composite": true, "noEmit": false, "emitDeclarationOnly": true, "outDir": "out" }, "include": ["src", "../src/jsx.d.ts"] }`,
			"lib/src/button.rtsx": "export function Button({ size }: { size: number }) {\n  const label: string = size;\n  return <button>{label}</button>;\n}\n",
			"lib/src/util.ts":     "export const twice = (n: number) => n * 2;\nexport const pick = (o, k) => o[k];\n",
			"app/tsconfig.json":   `{ "compilerOptions": { ` + options + ` }, "include": ["src", "../src/jsx.d.ts"], "references": [{ "path": "../lib" }] }`,
			"app/src/page.rtsx":   "import { Button } from \"../../lib/src/button\";\nimport { twice } from \"../../lib/src/util\";\nexport const page = <Button size=\"lg\">{twice(1)}</Button>;\n",
			"app/src/main.tsx":    "import { Button } from \"../../lib/src/button.rtsx\";\nexport const app = <Button size={2} />;\n",
		}))
		root, app, lib := output(dir, "tsconfig.json"), output(dir, "app/tsconfig.json"), output(dir, "lib/tsconfig.json")
		goldenText(t, "# -p .\n"+root+"# -p app\n"+app+"# -p lib\n"+lib)
		if want := "lib/src/button.rtsx(2,9): error TS2322: Type 'number' is not assignable to type 'string'.\n"; lib != want {
			t.Errorf("-p lib:\n%s", lib)
		}
		if !strings.HasPrefix(app, "app/src/page.rtsx(3,29): error TS2322") || !strings.HasSuffix(app, lib) || strings.Count(app, "error") != 2 {
			t.Errorf("-p app:\n%s", app)
		}
		if root != app {
			t.Errorf("-p .:\n%s-p app:\n%s", root, app)
		}
	})

	// A `paths` alias finds an .rtsx module, with and without the extension.
	t.Run("paths-alias", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":   `{ "compilerOptions": { ` + options + `, "paths": { "@/*": ["./src/*"], "@button": ["./src/button"] } }, "include": ["src"] }`,
			"src/button.rtsx": "export function Button({ size }: { size: number }) {\n  return <button>{size}</button>;\n}\n",
			"src/page.rtsx":   "import { Button } from \"@/button\";\nimport { Button as Exact } from \"@button\";\nimport { Button as Explicit } from \"@/button.rtsx\";\nexport const page = <main><Button size=\"a\" /><Exact size=\"b\" /><Explicit size=\"c\" /></main>;\n",
			"src/main.tsx":    "import { page } from \"@/page\";\nexport const app: string = page;\n",
		}))
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Contains(got, "TS2307") || strings.Count(got, "src/page.rtsx(4,") != 3 {
			t.Errorf("got:\n%s", got)
		}
	})

	// A tsconfig that lists the stock content mapper (ide.md, *Stock
	// TypeScript 7.1*) is checked as one without the entry.
	t.Run("content-mappers-entry", func(t *testing.T) {
		files := func(entry string) map[string]string {
			return with(map[string]string{
				"tsconfig.json": `{ "compilerOptions": { ` + options + ` }, "include": ["src"]` + entry + ` }`,
				"src/page.rtsx": "import { Card } from \"./card\";\nexport const page = <Card><$Nope>x</$Nope></Card>;\n",
				"src/card.rtsx": "import type { Slot } from \"@reactogenic/core\";\nexport function Card({ $Title }: { $Title?: Slot<{ children?: string }> }) {\n  return <h2 slot={$Title} />;\n}\n",
				"src/main.tsx":  "import { page } from \"./page.rtsx\";\nexport const app: number = page;\n",
			})
		}
		listed := output(writeProject(t, files(`, "contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]`)), "tsconfig.json")
		plain := output(writeProject(t, files("")), "tsconfig.json")
		goldenText(t, listed)
		if listed != plain || !strings.Contains(listed, "undeclared-slot") {
			t.Errorf("with the entry:\n%swithout:\n%s", listed, plain)
		}
	})

	// syntax.md, *Segment files*: the generated import names the .rtsx file,
	// which node16 / nodenext resolution finds as bundler resolution does.
	for _, resolution := range []string{"node16", "nodenext"} {
		t.Run("segment-"+resolution, func(t *testing.T) {
			dir := writeProject(t, with(map[string]string{
				"package.json":   `{ "name": "app", "type": "module" }`,
				"tsconfig.json":  `{ "compilerOptions": { ` + strings.Replace(options, `"module": "esnext", "moduleResolution": "bundler"`, `"module": "`+resolution+`", "moduleResolution": "`+resolution+`"`, 1) + ` }, "include": ["src"] }`,
				"src/page.rtsx":  "export default function Page() {\n  return <main><section #intro /><section #faq /></main>;\n}\n",
				"src/intro.rtsx": "export default function Intro({ title }: { title: string }) {\n  return <p>{title}</p>;\n}\n",
				"src/faq.tsx":    "export default function Faq() {\n  return <dl />;\n}\n",
			}))
			got := output(dir, "tsconfig.json")
			goldenText(t, got)
			if want := "src/page.rtsx(2,25): error segment-props: A segment takes no props: `intro` requires `title`\n"; !strings.HasPrefix(got, want) || strings.Contains(got, "TS2307") {
				t.Errorf("got:\n%s", got)
			}
		})
	}

	// A segment that mounts itself, directly and through another file
	// (syntax.md, *Segment roots*): each mount of the loop is reported.
	t.Run("segment-self", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":  tsconfig,
			"src/page.rtsx":  "export default function Page() {\n  return <main><section #intro /></main>;\n}\n",
			"src/intro.rtsx": "export default function Intro() {\n  return <div><section #outro /></div>;\n}\n",
			"src/outro.rtsx": "export default function Outro() {\n  return <div><section #intro /><section #outro /></div>;\n}\n",
		}))
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		for _, want := range []string{
			"src/intro.rtsx(2,24): error segment-self: `#outro` mounts the segment it is written in\n",
			"src/outro.rtsx(2,24): error segment-self: `#intro` mounts the segment it is written in\n",
			"src/outro.rtsx(2,42): error segment-self: `#outro` mounts the segment it is written in\n",
		} {
			if strings.Count(got, want) != 1 {
				t.Errorf("want once %q in:\n%s", want, got)
			}
		}
		if strings.Contains(got, "page.rtsx") {
			t.Errorf("page.rtsx is in no loop:\n%s", got)
		}
	})

	// A syntax error fails the check (ide.md, *Tolerance*: the build paths
	// stay strict): the file reports its syntax errors and nothing else, and
	// the rest of the program is still checked.
	t.Run("syntax-error", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":   tsconfig,
			"src/broken.rtsx": "export const n: number = \"x\";\nexport function Broken() {\n  return <div><$Title>t</$Title><span></div>;\n}\n",
			"src/main.tsx":    "import { Broken, n } from \"./broken\";\nexport const s: string = n;\nexport const app = <Broken />;\nexport const m: number = \"y\";\n",
		}))
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		want := "src/broken.rtsx(3,34): error TS17008: JSX element 'span' has no corresponding closing tag.\n" + // not its type error, nor its orphaned slot
			"src/main.tsx(2,14): error TS2322: Type 'number' is not assignable to type 'string'.\n" + // `n` is still exported
			"src/main.tsx(4,14): error TS2322: Type 'string' is not assignable to type 'number'.\n"
		if got != want {
			t.Errorf("got:\n%swant:\n%s", got, want)
		}
	})

	// A syntax error in a .ts / .tsx file is TypeScript's: as in tsc, no
	// types are checked while it stands. The transpiler's errors are
	// reported all the same.
	t.Run("tsx-syntax-error", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json": tsconfig,
			"src/page.rtsx": "export const n: number = \"x\";\nexport const page = <div><$Title>t</$Title></div>;\n",
			"src/main.tsx":  "export const app = <div><span></div>;\n",
		}))
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if !strings.Contains(got, "src/main.tsx(1,26): error TS17008") || !strings.Contains(got, "src/page.rtsx(2,26): error orphan-slot") || strings.Contains(got, "TS2322") {
			t.Errorf("got:\n%s", got)
		}
	})

	// A warning is printed and does not fail the check.
	t.Run("warning", func(t *testing.T) {
		dir := writeProject(t, with(map[string]string{
			"tsconfig.json":  tsconfig,
			"src/page.rtsx":  "export const page = <section #intro>old</section>;\n",
			"src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		}))
		reports := Run(dir + "/tsconfig.json")
		golden(t, dir, reports)
		if len(reports) != 1 || reports[0].Code != "segment-children" || Errors(reports) != 0 {
			t.Errorf("got %+v", reports)
		}
	})
}
