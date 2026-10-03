package lsp_test

import (
	"cmp"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// A global JSX namespace declared by a module: what a project without a
// single .ts, .tsx or .d.ts file has in place of `jsx.d.ts`.
const jsxModule = `declare global {
  namespace JSX {
    interface Element {}
    interface IntrinsicElements { [name: string]: any }
  }
}
export {};
`

// ide.md, *The engine*: the mapper is built in, not configured — whatever
// the project looks like, the page of TestBuiltInMapper is served at source
// positions: hover and definition across files, its one mistake (line 4,
// the shorthand `size`), and go to definition on the specifier of the .rtsx
// module. A tsconfig `contentMappers` entry is ignored — the package it
// names does not exist here — and nothing is registered for a mapped
// extension.
func TestProjectShapes(t *testing.T) {
	option := func(config, more string) string { // one more compiler option
		return strings.Replace(config, `"noEmit": true`, `"noEmit": true, `+more, 1)
	}
	top := func(config, more string) string { // one more top-level property
		return strings.Replace(config, `"include": ["src"]`, `"include": ["src"], `+more, 1)
	}
	imports := func(specifier string) string { // the page, importing the button another way
		return strings.Replace(page, `"./button"`, specifier, 1)
	}
	const entry = `"contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"] }]`
	// No project at the root: the projects are under packages/.
	monorepo := map[string]string{
		"tsconfig.json": "", "src/jsx.d.ts": "", "src/button.rtsx": "", "src/page.rtsx": "",
		"packages/ui/src/jsx.d.ts":    lsptest.JSXTypes,
		"packages/ui/src/button.rtsx": button,
		"packages/app/src/jsx.d.ts":   lsptest.JSXTypes,
	}
	for _, tc := range []struct {
		name      string
		files     map[string]string // over a tsconfig, jsx.d.ts, button.rtsx and page.rtsx under src/
		page      string            // the document ("": src/page.rtsx)
		button    string            // where `<Button` is defined ("": src/button.rtsx 1:17)
		specifier string            // the button's specifier in the page ("": "./button")
		module    string            // where the specifier leads ("": src/button.rtsx 1:1)
		loose     bool              // no JSX types: the mistake is one of several errors
		also      map[string]string // other documents, and their errors
	}{
		{name: "a contentMappers entry for .rtsx", files: map[string]string{"tsconfig.json": top(lsptest.TSConfig, entry)}},
		{name: "a contentMappers entry for another extension", files: map[string]string{
			"tsconfig.json": top(lsptest.TSConfig, `"contentMappers": [{ "package": "some-vue-mapper", "extensions": [".vue"] }]`),
		}},
		{name: "a contentMappers entry and runExternalCode", files: map[string]string{
			"tsconfig.json": top(option(lsptest.TSConfig, `"runExternalCode": true`), entry),
		}},
		{name: ".rtsx files only", files: map[string]string{"src/jsx.d.ts": "", "src/jsx.rtsx": jsxModule}},
		{name: "a paths alias", specifier: `"@/button"`, files: map[string]string{
			"tsconfig.json": option(lsptest.TSConfig, `"paths": { "@/*": ["./src/*"] }`),
			"src/page.rtsx": imports(`"@/button"`),
		}},
		{name: "extends", files: map[string]string{
			"tsconfig.json":    `{ "extends": "./config/base.json", "include": ["src"] }`,
			"config/base.json": lsptest.TSConfig,
		}},
		{name: "extends, the base with a contentMappers entry", files: map[string]string{
			"tsconfig.json":    `{ "extends": "./config/base.json", "include": ["src"] }`,
			"config/base.json": top(lsptest.TSConfig, entry),
		}},
		{
			name: "two projects, one importing the other's module through an alias",
			page: "packages/app/src/page.rtsx", button: "packages/ui/src/button.rtsx 1:17",
			specifier: `"@ui/button"`, module: "packages/ui/src/button.rtsx 1:1",
			files: lsptest.With(monorepo, map[string]string{
				"packages/ui/tsconfig.json":  lsptest.TSConfig,
				"packages/ui/src/bad.rtsx":   "import { Button } from \"./button\";\nexport const b = <Button size=\"x\" />;\n",
				"packages/app/tsconfig.json": option(lsptest.TSConfig, `"paths": { "@ui/*": ["../ui/src/*"] }`),
				"packages/app/src/page.rtsx": imports(`"@ui/button"`),
			}),
			also: map[string]string{"packages/ui/src/bad.rtsx": "2:26 TS2322"},
		},
		{
			name: "a composite project, referenced",
			page: "packages/app/src/page.rtsx", button: "packages/ui/src/button.rtsx 1:17",
			specifier: `"../../ui/src/button"`, module: "packages/ui/src/button.rtsx 1:1",
			files: lsptest.With(monorepo, map[string]string{
				"packages/ui/tsconfig.json":  strings.Replace(lsptest.TSConfig, `"noEmit": true`, `"composite": true, "outDir": "dist", "declaration": true`, 1),
				"packages/app/tsconfig.json": top(lsptest.TSConfig, `"references": [{ "path": "../ui" }]`),
				"packages/app/src/page.rtsx": imports(`"../../ui/src/button"`),
			}),
		},
		{
			// The root's tsconfig includes src/ only.
			name: "a loose file, next to a project", loose: true,
			page: "loose/page.rtsx", button: "loose/button.rtsx 1:17", module: "loose/button.rtsx 1:1",
			files: map[string]string{"loose/button.rtsx": button, "loose/page.rtsx": page},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t, lsptest.With(map[string]string{"src/button.rtsx": button, "src/page.rtsx": page}, tc.files))
			rel := cmp.Or(tc.page, "src/page.rtsx")
			c.Open(rel)
			if hover := c.Hover(rel, c.At(rel, "<Button", 1, 2)); !strings.Contains(hover, "function Button") {
				t.Errorf("hover on the component: %q", hover)
			}
			got := lsptest.Lines(c.Diagnostics(rel))
			found := false
			for _, line := range got {
				found = found || line == "4:24 TS2322"
			}
			if !found || (!tc.loose && len(got) != 1) {
				t.Errorf("diagnostics: %q", got)
			}
			if def := c.Definition(rel, c.At(rel, "<Button", 1, 2)); len(def) != 1 || def[0] != cmp.Or(tc.button, "src/button.rtsx 1:17") {
				t.Errorf("definition of the component: %q", def)
			}
			if def := c.Definition(rel, c.At(rel, cmp.Or(tc.specifier, `"./button"`), 1, 3)); len(def) != 1 || def[0] != cmp.Or(tc.module, "src/button.rtsx 1:1") {
				t.Errorf("definition on the specifier: %q", def)
			}
			for other, want := range tc.also {
				c.Open(other)
				if got := strings.Join(lsptest.Lines(c.Diagnostics(other)), ", "); got != want {
					t.Errorf("%s: %s, want %s", other, got, want)
				}
			}
			for _, r := range c.Registrations() {
				if strings.Contains(r, "content-mapper") {
					t.Errorf("dynamic registration for a mapped extension: %s", r)
				}
			}
		})
	}
}

// ide.md, *The engine*, Siblings: `twin.ts` next to `twin.rtsx` wins an
// extensionless import, silently — as in Vite, where `.rtsx` is the last of
// `resolve.extensions`. The .rtsx module is reached by its extension.
func TestBuiltInSiblingWins(t *testing.T) {
	const rel = "src/page.rtsx"
	const text = "import { twinOther } from \"./twin\";\nimport { twinThing } from \"./twin.rtsx\";\nexport const sum = twinOther + twinThing;\n"
	c := start(t, map[string]string{
		"src/twin.rtsx": "export const twinThing = 1;\n",
		"src/twin.ts":   "export const twinOther = 2;\n",
		rel:             text,
	})
	c.Open(rel)
	if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
		t.Errorf("diagnostics: %q", got)
	}
	for needle, want := range map[string]string{
		`"./twin"`: "src/twin.ts 1:1", `"./twin.rtsx"`: "src/twin.rtsx 1:1",
		"twinOther +": "src/twin.ts 1:14", "twinThing;": "src/twin.rtsx 1:14",
	} {
		if def := c.Definition(rel, c.At(rel, needle, 1, 3)); len(def) != 1 || def[0] != want {
			t.Errorf("definition at %s: %q, want %s", needle, def, want)
		}
	}
	// Without the extension the .rtsx module's export is not there: the
	// import is the .ts file's.
	c.Change(rel, strings.Replace(text, `"./twin.rtsx"`, `"./twin"`, 1))
	if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 1 || got[0] != "2:10 TS2305" {
		t.Errorf("importing the .rtsx export from \"./twin\": %q", got)
	}
}

// ide.md, *reactogenic lsp*: .ts files are in the server's program and are
// read from disk — the client attaches the server to .rtsx documents only.
// A .ts file saved (changed on disk) reaches an open .rtsx document without
// an edit to it.
func TestTSFilesAreReadFromDisk(t *testing.T) {
	c := start(t, app)
	const page = "src/page.rtsx"
	c.Open(page)
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	c.WriteFile("src/util.ts", strings.Replace(app["src/util.ts"], "(n: number,", "(n: string,", 1))
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 1 || got[0] != "7:23 TS2345" {
		t.Errorf("after util.ts changed on disk: %q", got)
	}
	c.WriteFile("src/util.ts", app["src/util.ts"])
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Errorf("after util.ts changed back: %q", got)
	}
}
