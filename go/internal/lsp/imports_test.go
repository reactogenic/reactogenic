package lsp_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

const knob = `export function Button({ size }: { size: number }) {
  return <button>{size}</button>;
}
`

// autoImport completes at the end of the first occurrence of typed and
// resolves the item labelled label, as the editor does when the item is
// selected: the import it would write, as `line:col-line:col "text"`, or why
// there is none.
func autoImport(t *testing.T, c *lsptest.Client, rel, typed, label string) string {
	t.Helper()
	items := c.Completion(rel, c.At(rel, typed, 1, len(typed)))
	for _, item := range items {
		if item.Label != label {
			continue
		}
		edits := c.Resolve(item).AdditionalTextEdits
		if len(edits) == 0 {
			continue // a local of that name, not an import
		}
		out := []string{}
		for _, e := range edits {
			out = append(out, fmt.Sprintf("%s %q", e.Range, e.NewText))
		}
		return strings.Join(out, ", ")
	}
	return fmt.Sprintf("no auto-import of %s among %d items", label, len(items))
}

// quickFixes are the quick fixes offered on the document's first error, as
// `title: file line:col-line:col "text"`.
func quickFixes(t *testing.T, c *lsptest.Client, rel string) []string {
	t.Helper()
	diagnostics := c.Diagnostics(rel)
	if len(diagnostics) == 0 {
		t.Fatalf("%s has no error to fix", rel)
	}
	var actions []struct {
		Title string                `json:"title"`
		Edit  lsptest.WorkspaceEdit `json:"edit"`
	}
	c.Request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": c.URI(rel)}, "range": diagnostics[0].Range,
		"context": map[string]any{"diagnostics": diagnostics[:1], "only": []string{"quickfix"}},
	}, &actions)
	out := []string{}
	for _, a := range actions {
		out = append(out, a.Title+": "+strings.Join(c.Edits(a.Edit), ", "))
	}
	return out
}

func renameFiles(c *lsptest.Client, pairs ...string) string {
	files := []any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		files = append(files, map[string]any{"oldUri": c.URI(pairs[i]), "newUri": c.URI(pairs[i+1])})
	}
	var edit lsptest.WorkspaceEdit
	c.Request("workspace/willRenameFiles", map[string]any{"files": files}, &edit)
	return strings.Join(c.Edits(edit), ", ")
}

// ide.md, *Specifiers the server writes*: extensionless for an .rtsx module,
// unless a built-in sibling would win the import. The sibling is next to the
// module, wherever the importer and the server's directory are.
func TestSpecifierNextToSibling(t *testing.T) {
	const use = "export function Page() {\n  return <main><Button size={1} /></main>;\n}\n"
	for _, tc := range []struct {
		name, dir, sibling, want string
	}{
		{"no sibling", "src/ui/", "", "./button"},
		{"a .ts sibling", "src/ui/", "button.ts", "./button.rtsx"},
		{"a .d.ts sibling", "src/ui/", "button.d.ts", "./button.rtsx"},
		{"a .ts sibling, deeper", "src/ui/deep/", "button.ts", "./button.rtsx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := tc.dir + "page.rtsx"
			files := map[string]string{tc.dir + "button.rtsx": knob, page: use}
			if tc.sibling != "" {
				files[tc.dir+tc.sibling] = "export declare const NotTheButton: number;\n"
			}
			c := start(t, files)
			c.Open(page)
			line := fmt.Sprintf("import { Button } from %q;\n\n", tc.want)
			// The quick fix on the unresolved name.
			if got := quickFixes(t, c, page); len(got) != 1 || got[0] != fmt.Sprintf(`Add import from %q: %s 1:1-1:1 %q`, tc.want, page, line) {
				t.Errorf("quick fixes: %q", got)
			}
			// The completion item, resolved.
			if got := autoImport(t, c, page, "<Button", "Button"); got != fmt.Sprintf("1:1-1:1 %q", line) {
				t.Errorf("auto-import: %s", got)
			}
			// What is written is the import of the .rtsx module.
			c.Change(page, line+use)
			if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
				t.Errorf("with %s written: %q", tc.want, got)
			}
		})
	}
	t.Run("file rename", func(t *testing.T) {
		c := start(t, map[string]string{
			"src/ui/btn.rtsx":    knob,
			"src/ui/button.ts":   "export const NotTheButton = 1;\n",
			"src/ui/toggle.d.ts": "export declare const NotTheButton: number;\n",
			"src/ui/page.rtsx":   "import { Button } from \"./btn\";\n" + use,
			"src/main.tsx":       "import { Button } from \"./ui/btn\";\nexport const b = Button;\n",
		})
		c.Open("src/ui/page.rtsx")
		for to, want := range map[string]string{
			"knob":   `src/main.tsx 1:25-1:33 "./ui/knob", src/ui/page.rtsx 1:25-1:30 "./knob"`,
			"button": `src/main.tsx 1:25-1:33 "./ui/button.rtsx", src/ui/page.rtsx 1:25-1:30 "./button.rtsx"`,
			"toggle": `src/main.tsx 1:25-1:33 "./ui/toggle.rtsx", src/ui/page.rtsx 1:25-1:30 "./toggle.rtsx"`,
		} {
			if got := renameFiles(c, "src/ui/btn.rtsx", "src/ui/"+to+".rtsx"); got != want {
				t.Errorf("btn.rtsx → %s.rtsx edits %s", to, got)
			}
		}
	})
	// A file converted in place — util.ts becomes util.rtsx — is not its own
	// sibling: the file renamed away is still on disk when the server is
	// asked, and is gone when the edits apply. `./util` goes on resolving,
	// so nothing is edited; and back.
	t.Run("a file renamed onto its own name", func(t *testing.T) {
		c := start(t, map[string]string{
			"src/util.ts":   "export const twice = (n: number) => n * 2;\n",
			"src/card.rtsx": "import { twice } from \"./util\";\nexport function Card() {\n  return <div>{twice(1)}</div>;\n}\n",
			"src/main.tsx":  "import { twice } from \"./util\";\nimport { Card } from \"./card\";\nexport const m = [twice, Card];\n",
		})
		c.Open("src/card.rtsx")
		for _, pair := range [][2]string{{"src/util.ts", "src/util.rtsx"}, {"src/card.rtsx", "src/card.tsx"}} {
			if got := renameFiles(c, pair[0], pair[1]); got != "" {
				t.Errorf("%s → %s edits %s", pair[0], pair[1], got)
			}
		}
		// Moved as well as converted: its .rtsx importer follows, without an
		// extension (main.tsx is the user's TypeScript's to edit).
		if got, want := renameFiles(c, "src/util.ts", "src/lib/util.rtsx"), `src/card.rtsx 1:24-1:30 "./lib/util"`; got != want {
			t.Errorf("util.ts → lib/util.rtsx edits %s, want %s", got, want)
		}
	})
	// A folder renamed with a module and its sibling in it: they move
	// together, and the sibling still wins the extensionless import.
	t.Run("a folder with a sibling pair", func(t *testing.T) {
		c := start(t, map[string]string{
			"src/ui/button.rtsx": knob,
			"src/ui/button.ts":   "export const NotTheButton = 1;\n",
			"src/ui/knob.rtsx":   knob,
			"src/page.rtsx":      "import { Button } from \"./ui/button.rtsx\";\nimport { Button as Knob } from \"./ui/knob\";\nexport const page = [Button, Knob];\n",
		})
		c.Open("src/page.rtsx")
		if got, want := renameFiles(c, "src/ui", "src/kit"), `src/page.rtsx 1:25-1:41 "./kit/button.rtsx", src/page.rtsx 2:33-2:42 "./kit/knob"`; got != want {
			t.Errorf("ui → kit edits %s, want %s", got, want)
		}
	})
}

// ide.md, *Not in the first release*: auto-import of a dependency's .rtsx
// exports. A package under node_modules that ships .rtsx modules imports and
// checks like any other; what is missing is the offer of its exports before
// the first import is written — TypeScript's index of dependencies reads
// their files itself, as plain TypeScript.
func TestAutoImportFromPackage(t *testing.T) {
	const page = "src/page.rtsx"
	const use = "export function Page() {\n  return <main><Button size={fromT} /></main>;\n}\n"
	c := start(t, map[string]string{
		"package.json":                 `{ "name": "app", "dependencies": { "ui": "*" } }`,
		"node_modules/ui/package.json": `{ "name": "ui", "version": "1.0.0", "exports": { ".": "./index.ts" } }`,
		"node_modules/ui/index.ts":     "export * from \"./button\";\nexport const fromTs = 1;\n",
		"node_modules/ui/button.rtsx":  knob,
		page:                           use,
	})
	c.Open(page)
	// The package's .ts export is offered, by the package's name.
	if got := autoImport(t, c, page, "{fromT", "fromTs"); got != `1:1-1:1 "import { fromTs } from \"ui\";\n\n"` {
		t.Errorf("auto-import of fromTs: %s", got)
	}
	// Its .rtsx export is not. (When this fails the limit is gone: take the
	// row out of ide.md.)
	if got := autoImport(t, c, page, "<Button", "Button"); !strings.HasPrefix(got, "no auto-import of Button") {
		t.Errorf("auto-import of Button, which ide.md lists as not in the first release: %s", got)
	}
	// Written by hand, the import is the .rtsx module: it resolves and checks.
	c.Change(page, "import { Button, fromTs } from \"ui\";\n"+strings.Replace(use, "fromT}", "fromTs}", 1))
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Errorf("with the import written: %q", got)
	}
	if got := c.Definition(page, c.At(page, "<Button", 1, 2)); len(got) != 1 || got[0] != "node_modules/ui/button.rtsx 1:17" {
		t.Errorf("definition of Button: %q", got)
	}
}

// ide.md, the feature table: go to definition. On a module specifier it
// opens the module, however the specifier reaches it: relative, with the
// extension, or through a `paths` alias — from an .rtsx file and into one.
func TestDefinitionOnSpecifier(t *testing.T) {
	c := start(t, map[string]string{
		"tsconfig.json":   strings.Replace(lsptest.TSConfig, `"noEmit": true`, `"noEmit": true, "paths": { "@/*": ["./src/*"] }`, 1),
		"src/button.rtsx": knob,
		"src/card.rtsx":   "export function Card({ $Title }: { $Title?: { children?: string } }) {\n  return <div><h1 slot={$Title} /></div>;\n}\n", // starts with generated text
		"src/util.ts":     "export const twice = (n: number) => n * 2;\n",
		"src/pages/x.rtsx": `import { Button } from "@/button";
import { Button as B2 } from "@/button.rtsx";
import { Button as B3 } from "../button";
import { Button as B4 } from "../button.rtsx";
import { Card } from "@/card";
import { twice } from "@/util";
export const x = [Button, B2, B3, B4, Card, twice];
`,
		"src/pages/y.tsx": "import { Button } from \"@/button\";\nexport const y = Button;\n",
	})
	for _, file := range []string{"src/pages/x.rtsx", "src/pages/y.tsx"} {
		c.Open(file)
		if got := lsptest.Lines(c.Diagnostics(file)); len(got) != 0 {
			t.Fatalf("%s has errors: %q", file, got)
		}
	}
	for specifier, want := range map[string]string{
		`"@/button"`: "src/button.rtsx 1:1", `"@/button.rtsx"`: "src/button.rtsx 1:1",
		`"../button"`: "src/button.rtsx 1:1", `"../button.rtsx"`: "src/button.rtsx 1:1",
		`"@/card"`: "src/card.rtsx 1:1", `"@/util"`: "src/util.ts 1:1",
	} {
		if got := c.Definition("src/pages/x.rtsx", c.At("src/pages/x.rtsx", specifier, 1, 3)); len(got) != 1 || got[0] != want {
			t.Errorf("x.rtsx, on %s: %q, want %s", specifier, got, want)
		}
	}
	if got := c.Definition("src/pages/y.tsx", c.At("src/pages/y.tsx", `"@/button"`, 1, 3)); len(got) != 1 || got[0] != "src/button.rtsx 1:1" {
		t.Errorf("y.tsx, on \"@/button\": %q", got)
	}
}

// sourceActions asks for the source actions of one kind, as the editor's
// command (or its organize-on-save) does: `title: edits`, the edits as
// Client.Edits renders them. With apply, the first action's edit is applied.
func sourceActions(c *lsptest.Client, rel, kind string, apply bool) []string {
	var actions []struct {
		Title string                 `json:"title"`
		Kind  string                 `json:"kind"`
		Edit  *lsptest.WorkspaceEdit `json:"edit"`
	}
	whole := lsptest.Range{End: lsptest.PositionAt(c.Text(rel), len(c.Text(rel)))}
	c.Request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": c.URI(rel)}, "range": whole,
		"context": map[string]any{"diagnostics": []any{}, "only": []string{kind}},
	}, &actions)
	out := []string{}
	for i, a := range actions {
		if a.Edit == nil {
			out = append(out, a.Title+": no edit")
			continue
		}
		out = append(out, a.Title+": "+strings.Join(c.Edits(*a.Edit), ", "))
		if apply && i == 0 {
			c.Apply(*a.Edit)
		}
	}
	return out
}

// ide.md, the feature table: organize imports. In a file whose imports are
// all the author's, they are organized as in a .tsx file. An import the
// transform generates (a segment's, the core import of a container) stands
// in the import block with no source text of its own: there the edit cannot
// be written back, and the action is not offered — never one with no edit.
func TestOrganizeImports(t *testing.T) {
	const imports = "import { twice, greeting } from \"./util\";\nimport { Options } from \"./util\";\nimport { Button } from \"./button\";\n"
	base := lsptest.With(lsptest.Core, map[string]string{
		"src/button.rtsx": "import type { Slot } from \"@reactogenic/core\";\nexport function Button({ size, $Label }: { size: number; $Label?: Slot<{ children?: string }> }) {\n  return <button><b slot={$Label} />{size}</button>;\n}\n",
		"src/intro.rtsx":  "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/util.ts":     "export const twice = (n: number) => n * 2;\nexport const greeting = \"hello\";\nexport interface Options { unit: string }\n",
	})
	kinds := map[string]string{ // the kind, and the import block it leaves
		"source.sortImports":         "import { Button } from \"./button\";\nimport { greeting, Options, twice } from \"./util\";\n",
		"source.removeUnusedImports": "import { twice } from \"./util\";\nimport { Button } from \"./button\";\n",
		"source.organizeImports":     "import { Button } from \"./button\";\nimport { twice } from \"./util\";\n",
	}
	for _, tc := range []struct {
		name, body string
		generated  bool
	}{
		{"a file that fills slots", "export function Page() {\n  return <main><Button size={twice(1)}><$Label>Save</$Label></Button></main>;\n}\n", false},
		{"a segment mounter", "export function Page() {\n  return <main><section #intro /><Button size={twice(1)} /></main>;\n}\n", true},
		{"a container", "export function Card({ $Title }: { $Title?: { children?: string } }) {\n  return <div><h1 slot={$Title} /><Button size={twice(1)} /></div>;\n}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const page = "src/page.rtsx"
			c := start(t, lsptest.With(base, map[string]string{page: imports + tc.body}))
			c.Open(page)
			if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
				t.Fatalf("the fixture has errors: %q", got)
			}
			for kind, want := range kinds {
				c.Change(page, imports+tc.body)
				got := sourceActions(c, page, kind, true)
				if tc.generated {
					if len(got) != 0 || c.Text(page) != imports+tc.body {
						t.Errorf("%s is offered: %q", kind, got)
					}
					continue
				}
				if len(got) != 1 || c.Text(page) != want+tc.body {
					t.Errorf("%s: %q gives\n%s", kind, got, c.Text(page))
				}
				if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
					t.Errorf("after %s: %q", kind, got)
				}
				// Once more: the same text.
				if sourceActions(c, page, kind, true); c.Text(page) != want+tc.body {
					t.Errorf("%s, again:\n%s", kind, c.Text(page))
				}
			}
		})
	}
}

// ide.md, the feature table: auto-import. A file whose only import is a
// generated one — a segment mounter, a container — has no import of its own
// to insert next to: the new import goes to the top of the source, whether
// its module sorts before or after the generated one.
func TestAutoImportBesideGeneratedImport(t *testing.T) {
	base := map[string]string{
		"src/button.rtsx": knob, // "./button" sorts before "./intro.rtsx"
		"src/intro.rtsx":  "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/util.ts":     "export const twice = (n: number) => n * 2;\n", // sorts after
		"src/aaa.ts":      "export const aardvark = 1;\n",                 // sorts before
	}
	const mounter = "export function Page() {\n  return <main><section #intro /><Button size={1} /></main>;\n}\n"
	const container = "export function Card({ $Title }: { $Title?: { children?: string } }) {\n  return <div><h1 slot={$Title} /><Button size={1} /></div>;\n}\n"
	for _, tc := range []struct {
		name, text string
		line       int    // where the import goes
		blank      string // what separates it from the code
	}{
		{"a segment mounter", mounter, 1, ""},
		{"a container", container, 1, ""},
		{"a mounter, after a comment", "// The page.\n" + mounter, 1, ""},
		{"a mounter, after a directive", "\"use strict\";\n" + mounter, 1, ""},
		{"a mounter with an import of its own", "import { twice as two } from \"./util\";\n" + strings.Replace(mounter, "<main>", "<main title={String(two(1))}>", 1), 2, ""},
		{"no generated import", strings.Replace(mounter, " #intro", "", 1), 1, "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const page = "src/page.rtsx"
			c := start(t, lsptest.With(lsptest.Core, base, map[string]string{page: tc.text}))
			c.Open(page)
			at := fmt.Sprintf("%d:1-%d:1", tc.line, tc.line)
			line := "import { Button } from \"./button\";\n" + tc.blank
			if got := quickFixes(t, c, page); len(got) != 1 || got[0] != fmt.Sprintf(`Add import from "./button": %s %s %q`, page, at, line) {
				t.Errorf("quick fixes: %q", got)
			}
			if got := autoImport(t, c, page, "<Button", "Button"); got != fmt.Sprintf("%s %q", at, line) {
				t.Errorf("auto-import of Button: %s", got)
			}
			// An identifier being typed, from a module on either side.
			for _, name := range []string{"aardvark", "twice"} {
				typed := name[:len(name)-1]
				c.Change(page, strings.Replace(tc.text, "size={1}", "size={"+typed+"}", 1))
				from := map[string]string{"aardvark": "./aaa", "twice": "./util"}[name]
				want := fmt.Sprintf("%s %q", at, fmt.Sprintf("import { %s } from %q;\n%s", name, from, tc.blank))
				if name == "twice" && tc.line == 2 {
					want = `1:10-1:10 "twice, "` // into the import of ./util that is there
				}
				got := autoImport(t, c, page, "{"+typed, name)
				if got != want {
					t.Errorf("auto-import of %s: %s, want %s", name, got, want)
					continue
				}
				// Accepted: the name resolves.
				for _, item := range c.Completion(page, c.At(page, "{"+typed, 1, len(typed)+1)) {
					if edits := c.Resolve(item).AdditionalTextEdits; item.Label == name && len(edits) > 0 {
						c.ApplyTo(page, edits)
						break
					}
				}
				c.Change(page, strings.Replace(c.Text(page), "size={"+typed+"}", "size={"+name+"}", 1))
				if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 1 || !strings.HasSuffix(got[0], "TS2304") { // Button: still not imported
					t.Errorf("with %s imported: %q\n%s", name, got, c.Text(page))
				}
			}
		})
	}
}

// A file that is only comments — a new file under its header, or one whose
// only import the transform takes out (`Switch`, `Match`): completion at its
// end answers, auto-imports included. (For a content-mapped file upstream
// computes each item's import edit while it completes, and it indexed past a
// text with no statement: every completion there was an error.)
func TestCompletionInAFileOfComments(t *testing.T) {
	const page = "src/page.rtsx"
	for name, text := range map[string]string{
		"a line comment":                    "// A page.\n",
		"a block comment":                   "/* A page. */\n",
		"an import that is dropped, a note": "import { Switch, Match } from \"@reactogenic/core\";\n// A page.\n",
	} {
		t.Run(name, func(t *testing.T) {
			c := start(t, lsptest.With(lsptest.Core, map[string]string{
				"src/util.ts": "export const twice = (n: number) => n * 2;\n",
				page:          text,
			}))
			c.Open(page)
			var items []lsptest.CompletionItem
			for _, item := range c.Completion(page, lsptest.PositionAt(text, len(text))) {
				if item.Label == "twice" {
					items = append(items, item)
				}
			}
			if len(items) != 1 {
				t.Fatalf("%d items for `twice` at the end of the file", len(items))
			}
			// Accepted: the import, and the name where the cursor was.
			c.ApplyTo(page, c.Resolve(items[0]).AdditionalTextEdits)
			c.Change(page, c.Text(page)+"\nexport const two = twice(1);\n")
			if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 || !strings.Contains(c.Text(page), `import { twice } from "./util";`) {
				t.Errorf("with twice accepted: %q\n%s", got, c.Text(page))
			}
		})
	}
}

// ide.md, *A generated import is not one to add to*: a name that a generated
// import's own module exports — `Each` or the type `Slot` in a container, a
// named export of a mounted module. TypeScript would add the name to the
// import that is there, which has no source text to add to: the name gets an
// import declaration of its own — never a piece of one (`, Each`) written
// into the source. Applied, the file checks.
func TestAutoImportFromGeneratedImportsModule(t *testing.T) {
	base := lsptest.With(lsptest.Core, map[string]string{
		"src/intro.rtsx":                "export default function Intro() {\n  return <p>intro</p>;\n}\nexport const introTitle = \"t\";\n",
		"src/util.ts":                   "export const twice = (n: number) => n * 2;\n",
		"node_modules/aaa/package.json": `{ "name": "aaa", "types": "index.d.ts" }`,
		"node_modules/aaa/index.d.ts":   "export declare const aaa: string;\n",
	})
	const container = "export function Card({ $Title }: { $Title?: { children?: string } }) {\n  return <div><h1 slot={$Title} />{Eac}</div>;\n}\n"
	const core = "@reactogenic/core"
	for _, tc := range []struct {
		name, text   string
		typed, label string
		from         string
		line         int // where the import goes
	}{
		{"a container, a component of the core", container, "{Eac", "Each", core, 1},
		{"a container, the type of its own slot", "export function Card({ $Title }: { $Title?: Slo<{ children?: string }> }) {\n  return <div><h1 slot={$Title} /></div>;\n}\n", "Slo", "Slot", core, 1},
		{"a container that mounts a segment: two generated imports", strings.Replace(container, "{Eac}", "<section #intro />{Eac}", 1), "{Eac", "Each", core, 1},
		{"a container that mounts a segment, the mounted module", strings.Replace(container, "{Eac}", "<section #intro />{introTitl}", 1), "{introTitl", "introTitle", "./intro.rtsx", 1},
		{"a mounter, a named export of the mounted module", "export function Page() {\n  return <main><section #intro />{introTitl}</main>;\n}\n", "{introTitl", "introTitle", "./intro.rtsx", 1},
		{"a mounter after a comment, the core", "// The page.\nexport function Page() {\n  return <main><section #intro />{Eac}</main>;\n}\n", "{Eac", "Each", core, 1},
		// With imports of its own, the generated import follows them: the new
		// one goes where it sorts among the author's.
		{"a container with a relative import of its own", "import { twice } from \"./util\";\n" + strings.Replace(container, "{Eac}", "{twice(1)}{Eac}", 1), "{Eac", "Each", core, 1},
		{"a container with a package import of its own", "import { aaa } from \"aaa\";\n" + strings.Replace(container, "{Eac}", "{aaa}{Eac}", 1), "{Eac", "Each", core, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const page = "src/page.rtsx"
			c := start(t, lsptest.With(base, map[string]string{page: tc.text}))
			c.Open(page)
			at := fmt.Sprintf("%d:1-%d:1", tc.line, tc.line)
			line := fmt.Sprintf("import { %s } from %q;\n", tc.label, tc.from)
			if got := autoImport(t, c, page, tc.typed, tc.label); got != fmt.Sprintf("%s %q", at, line) {
				t.Errorf("auto-import of %s: %s", tc.label, got)
			}
			// The name written out, and not imported: the quick fix.
			end := strings.Index(tc.text, tc.typed) + len(tc.typed)
			typed, next := strings.TrimPrefix(tc.typed, "{"), tc.text[end:end+1]
			written := func(text string) string { return strings.Replace(text, typed+next, tc.label+next, 1) }
			c.Change(page, written(tc.text))
			if got := quickFixes(t, c, page); len(got) != 1 || got[0] != fmt.Sprintf("Add import from %q: %s %s %q", tc.from, page, at, line) {
				t.Errorf("quick fixes: %q", got)
			}
			// The completion item accepted: the file checks.
			c.Change(page, tc.text)
			for _, item := range c.Completion(page, c.At(page, tc.typed, 1, len(tc.typed))) {
				if edits := c.Resolve(item).AdditionalTextEdits; item.Label == tc.label && len(edits) > 0 {
					c.ApplyTo(page, edits)
					break
				}
			}
			c.Change(page, written(c.Text(page)))
			if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 || !strings.Contains(c.Text(page), line) {
				t.Errorf("with %s accepted: %q\n%s", tc.label, got, c.Text(page))
			}
		})
	}
}
