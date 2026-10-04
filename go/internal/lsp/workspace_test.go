package lsp_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// workspaceSymbols are the symbols a query finds, as `name file line:col`,
// sorted.
func workspaceSymbols(c *lsptest.Client, query string) []string {
	var symbols []struct {
		Name     string           `json:"name"`
		Location lsptest.Location `json:"location"`
	}
	c.Request("workspace/symbol", map[string]any{"query": query}, &symbols)
	out := []string{}
	for _, s := range symbols {
		out = append(out, fmt.Sprintf("%s %s %d:%d", s.Name, c.Rel(s.Location.URI), s.Location.Range.Start.Line+1, s.Location.Range.Start.Character+1))
	}
	sort.Strings(out)
	return out
}

// ide.md, the feature table: workspace symbols are those declared in .rtsx
// files. TypeScript answers with the best 256 matches: the .rtsx ones are
// chosen before that cut, not from what is left after it — in a project of
// any size a short query matches more than 256 declarations of .ts files.
func TestWorkspaceSymbols(t *testing.T) {
	var rows strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&rows, "export const row%03d = %d;\n", i, i)
	}
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/rows.ts": rows.String(),
		"src/table.rtsx": `import type { Slot } from "@reactogenic/core";
export const rowzz = 1;
export function RowHeaderCell({ $Row }: { $Row?: Slot<{ className?: string }, { rowSize: number }> }) {
  return <th><b slot={$Row} &rowSize={rowzz} /></th>;
}
export function Rows() {
  return <RowHeaderCell><$Row className="row" { rowSize }>{rowSize}</$Row></RowHeaderCell>;
}
`,
	}))
	c.Open("src/table.rtsx")
	if got := lsptest.Lines(c.Diagnostics("src/table.rtsx")); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	// `rowSize`: the parameter of RowHeaderCell's slot type, the arg name
	// and the param of the slot body are not declarations TypeScript lists;
	// neither is a key of a generated object.
	want := []string{"RowHeaderCell src/table.rtsx 3:17", "Rows src/table.rtsx 6:17", "rowzz src/table.rtsx 2:14"}
	for _, query := range []string{"row", "r"} {
		got := workspaceSymbols(c, query)
		var declared []string
		for _, s := range got {
			if strings.HasPrefix(s, "Row") || strings.HasPrefix(s, "rowzz") {
				declared = append(declared, s)
			}
			if !strings.Contains(s, " src/table.rtsx ") {
				t.Errorf("query %q: %s is not ours", query, s)
			}
		}
		if strings.Join(declared, "; ") != strings.Join(want, "; ") {
			t.Errorf("query %q: %q, want %q", query, got, want)
		}
	}
	if got := workspaceSymbols(c, "row"); len(got) != len(want) {
		t.Errorf(`query "row": %q`, got)
	}
}

// ide.md, the feature table: file rename. Decided per import, not per
// request: an edit is ours when it is in an .rtsx document, or rewrites an
// import of an .rtsx module — no other server knows either. The rest is the
// user's TypeScript's, which makes the same edit.
func TestFileRename(t *testing.T) {
	files := map[string]string{
		"src/util.ts":         "export const twice = (n: number) => n * 2;\n",
		"src/page.rtsx":       "import { twice } from \"./util\";\nexport function Page() {\n  return <p>{twice(1)}</p>;\n}\n",
		"src/entry.tsx":       "import { Page } from \"./page\";\nimport { twice } from \"./util\";\nimport { Card } from \"./parts/card\";\nimport { half } from \"./parts/math\";\nimport { Shell } from \"./shell\";\nexport const e = [Page, twice, Card, half, Shell];\n",
		"src/shell.tsx":       "import { twice } from \"./util\";\nexport const Shell = () => <p>{twice(1)}</p>;\n",
		"src/list.rtsx":       "import { Card } from \"./parts/card\";\nimport { half } from \"./parts/math\";\nimport { Shell } from \"./shell\";\nexport const l = [Card, half, Shell];\n",
		"src/parts/card.rtsx": "import { twice } from \"../util\";\nexport function Card() {\n  return <p>{twice(1)}</p>;\n}\n",
		"src/parts/math.ts":   "import { twice } from \"../util\";\nexport const half = (n: number) => twice(n) / 4;\n",
	}
	c := start(t, files)
	c.Open("src/page.rtsx")
	for _, tc := range []struct {
		name  string
		pairs []string
		want  string
	}{
		{"a .ts file: its .rtsx importers", []string{"src/util.ts", "src/helpers.ts"},
			`src/page.rtsx 1:24-1:30 "./helpers", src/parts/card.rtsx 1:24-1:31 "../helpers"`},
		{"a .tsx file: its .rtsx importers", []string{"src/shell.tsx", "src/frame.tsx"},
			`src/list.rtsx 3:24-3:31 "./frame"`},
		{"an .rtsx file: every importer", []string{"src/page.rtsx", "src/home.rtsx"},
			`src/entry.tsx 1:23-1:29 "./home"`},
		{"an .rtsx file that moves: its own imports too", []string{"src/parts/card.rtsx", "src/card.rtsx"},
			`src/entry.tsx 3:23-3:35 "./card", src/list.rtsx 1:23-1:35 "./card", src/parts/card.rtsx 1:24-1:31 "./util"`},
		{"a .ts and an .rtsx file in one request", []string{"src/util.ts", "src/helpers.ts", "src/page.rtsx", "src/home.rtsx"},
			`src/entry.tsx 1:23-1:29 "./home", src/page.rtsx 1:24-1:30 "./helpers", src/parts/card.rtsx 1:24-1:31 "../helpers"`},
		{"the same, the other way round", []string{"src/page.rtsx", "src/home.rtsx", "src/util.ts", "src/helpers.ts"},
			`src/entry.tsx 1:23-1:29 "./home", src/page.rtsx 1:24-1:30 "./helpers", src/parts/card.rtsx 1:24-1:31 "../helpers"`},
		// A folder: the imports of its .rtsx modules, in every importer; the
		// imports of its .ts modules, in .rtsx importers.
		{"a folder", []string{"src/parts", "src/pieces"},
			`src/entry.tsx 3:23-3:35 "./pieces/card", src/list.rtsx 1:23-1:35 "./pieces/card", src/list.rtsx 2:23-2:35 "./pieces/math"`},
	} {
		if got := renameFiles(c, tc.pairs...); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}

	// A tsconfig entry: of an .rtsx file, ours; of a .ts file, not.
	listed := start(t, map[string]string{
		"tsconfig.json": strings.Replace(lsptest.TSConfig, `"include": ["src"]`, `"files": ["src/jsx.d.ts", "src/a.rtsx", "src/b.ts"]`, 1),
		"src/a.rtsx":    "export const a = <p />;\n",
		"src/b.ts":      "export const b = 1;\n",
	})
	listed.Open("src/a.rtsx")
	if got := renameFiles(listed, "src/a.rtsx", "src/a2.rtsx"); got != `tsconfig.json 6:30-6:40 "src/a2.rtsx"` {
		t.Errorf("an .rtsx file in tsconfig's files: %s", got)
	}
	if got := renameFiles(listed, "src/b.ts", "src/b2.ts"); got != "" {
		t.Errorf("a .ts file in tsconfig's files: %s", got)
	}

	// The filters: files of the extensions TypeScript handles, .rtsx — and
	// folders, which hold them.
	var workspace struct {
		FileOperations struct {
			WillRename struct {
				Filters []struct {
					Scheme  string `json:"scheme"`
					Pattern struct {
						Glob    string `json:"glob"`
						Matches string `json:"matches"`
					} `json:"pattern"`
				} `json:"filters"`
			} `json:"willRename"`
		} `json:"fileOperations"`
	}
	if err := json.Unmarshal(c.Initialized.Capabilities["workspace"], &workspace); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, f := range workspace.FileOperations.WillRename.Filters {
		got = append(got, strings.TrimSpace(f.Scheme+" "+f.Pattern.Glob+" "+f.Pattern.Matches))
	}
	if strings.Join(got, "; ") != "file **/*.{ts,tsx,js,jsx,cts,cjs,mts,mjs,json,rtsx}; file ** folder" {
		t.Errorf("willRename filters: %q", got)
	}
}

// A client without willRenameFiles support is given the file's rename and
// the edits of its importers in one answer to textDocument/rename on a
// module specifier: no other server is asked, so nothing is left out.
func TestRenameOfSpecifierWithoutWillRename(t *testing.T) {
	c := lsptest.StartWith(t, lsptest.Project(t, map[string]string{
		"src/util.ts":   "export const twice = (n: number) => n * 2;\n",
		"src/page.rtsx": "import { twice } from \"./util\";\nexport function Page() {\n  return <p>{twice(1)}</p>;\n}\n",
		"src/main.tsx":  "import { twice } from \"./util\";\nexport const four = twice(2);\n",
	}), serve, lsptest.Options{Capabilities: func(capabilities map[string]any) {
		// It can rename a file itself, when an edit says so.
		capabilities["workspace"].(map[string]any)["workspaceEdit"] = map[string]any{"documentChanges": true, "resourceOperations": []string{"create", "rename", "delete"}}
	}})
	c.Open("src/page.rtsx")
	var edit lsptest.WorkspaceEdit
	c.Request("textDocument/rename", map[string]any{"textDocument": map[string]any{"uri": c.URI("src/page.rtsx")}, "position": c.At("src/page.rtsx", `"./util"`, 1, 4), "newName": "helpers"}, &edit)
	if got := strings.Join(c.Edits(edit), ", "); got != `src/main.tsx 1:24-1:30 "./helpers", src/page.rtsx 1:24-1:30 "./helpers"` {
		t.Errorf("edits: %s", got)
	}
}
