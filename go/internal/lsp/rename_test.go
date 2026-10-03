package lsp_test

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// The rename fixture: every site of ide.md's *Rename* table. A container
// with a singular, a keyed and a parameterised slot, `&arg` and `&&both`;
// a page with shorthand props on a component and on an element, a tag with a
// dot, a slot filled twice, a `Switch` under another name, `Match`, `Each`
// and a segment root; a .ts and a .tsx module beside them.
var renameApp = lsptest.With(lsptest.Core, map[string]string{
	// JSX types with children as a prop, and a component that may return
	// any node: what `Each` and a slot body's callback need.
	"src/jsx.d.ts": `declare namespace JSX {
  interface Element { readonly $$typeof: symbol }
  type ElementType = string | ((props: any) => unknown);
  interface ElementChildrenAttribute { children: {} }
  interface IntrinsicElements { [name: string]: any }
}
`,
	"src/table.rtsx": `import type { Slot, KeyedSlot } from "@reactogenic/core";
export interface Sizing {
  size?: number;
}
export interface TableProps extends Sizing {
  rows: string[];
  $Title?: Slot<{ className?: string; children?: string }>;
  $Column: KeyedSlot<{ label?: string }>;
  $Row?: Slot<{ className?: string }, { row: string; selected: boolean }>;
}
export function Sized({ size }: Sizing) {
  return <i>{size}</i>;
}
export function Table({ rows, size, $Title, $Column, $Row }: TableProps) {
  const selected = rows.length > 0;
  const row = rows[0];
  return (
    <table>
      <caption slot={$Title} className="title">Untitled</caption>
      <th slot={$Column} key="name" />
      <tr slot={$Row} &row &&selected />
      <Sized size />
    </table>
  );
}
`,
	"src/ui.tsx": `export function Box(props: { wide?: boolean; children?: unknown }) {
  return <div />;
}
export const UI = { Box };
`,
	"src/util.ts": `export const twice = (n: number) => n * 2;
export const labels = ["a", "b"];
`,
	"src/intro.rtsx": `export default function Intro() {
  return <p>intro</p>;
}
`,
	"src/page.rtsx": `import { Switch as Choose, Match, Each } from "@reactogenic/core";
import { Table } from "./table";
import { UI, Box } from "./ui";
import { twice, labels } from "./util";

export function Page({ status }: { status: "loading" | "ready" }) {
  const size = twice(1);
  const wide = true;
  const hidden = false;
  return (
    <main hidden>
      <section #intro />
      <input disabled />
      <Table rows={labels} size>
        <$Title className="t">Users</$Title>
        <$Column key="name" label="Name" />
        <$Column key="email" label="Email"></$Column>
        <$Row { row, selected }>{selected ? row : null}</$Row>
      </Table>
      <UI.Box wide>
        <Box>x</Box>
      </UI.Box>
      <Match on={wide}>
        <b>wide</b>
      </Match>
      <Each items={labels} { item }>
        <i key={item}>{item}</i>
      </Each>
      <Choose on={status} exhaustive>
        <$Case is="loading">Loading</$Case>
        <$Case is="ready">Ready</$Case>
      </Choose>
    </main>
  );
}
`,
	"src/main.tsx": `import { Page } from "./page";
export const app = <Page status="ready" />;
`,
})

// renameDocs are the fixture's own files, all open in a rename test: an
// edit lands in a buffer, and each file's diagnostics can be pulled.
var renameDocs = []string{"src/intro.rtsx", "src/main.tsx", "src/page.rtsx", "src/table.rtsx", "src/ui.tsx", "src/util.ts"}

func startRename(t *testing.T, options lsptest.Options) *lsptest.Client {
	t.Helper()
	c := lsptest.StartWith(t, lsptest.Project(t, renameApp), serve, options)
	for _, rel := range renameDocs {
		c.Open(rel)
	}
	if got := problems(c); len(got) != 0 {
		t.Fatalf("the fixture has diagnostics: %q", got)
	}
	return c
}

// problems are the errors and warnings of every file of the fixture, as
// `file line:col code`.
func problems(c *lsptest.Client) []string {
	out := []string{}
	for _, rel := range renameDocs {
		for _, d := range c.Diagnostics(rel) {
			out = append(out, rel+" "+d.String()+" "+d.Message)
		}
	}
	return out
}

// restore puts the fixture's texts back.
func restore(c *lsptest.Client) {
	for _, rel := range renameDocs {
		if c.Text(rel) != renameApp[rel] {
			c.Change(rel, renameApp[rel])
		}
	}
}

// ide.md, *Rename*: one case per row of the table. Each asserts the exact
// edits; then they are applied to the documents, the lines they touched are
// compared, and the project must have no diagnostic afterwards.
func TestRenameTable(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page, table = "src/page.rtsx", "src/table.rtsx"
	for _, tc := range []struct {
		name        string
		rel, needle string
		n, offset   int
		to          string
		edits       []string
		lines       []string // lines of the documents afterwards
	}{
		{
			name: "plain copied text: a param of a slot body, and its use",
			rel:  page, needle: "{ row, selected }", n: 1, offset: 3, to: "line",
			edits: []string{`src/page.rtsx 18:17-18:20 "row: line"`, `src/page.rtsx 18:45-18:48 "line"`},
			lines: []string{`<$Row { row: line, selected }>{selected ? line : null}</$Row>`},
		},
		{
			name: "plain copied text: across .ts, .tsx and .rtsx",
			rel:  page, needle: "labels }", n: 1, offset: 1, to: "names",
			edits: []string{`src/page.rtsx 14:20-14:26 "names"`, `src/page.rtsx 26:20-26:26 "names"`, `src/page.rtsx 4:17-4:23 "labels as names"`},
			lines: []string{`import { twice, labels as names } from "./util";`, `<Table rows={names} size>`, `<Each items={names} { item }>`},
		},
		{
			name: "shorthand: the binding renamed",
			rel:  page, needle: "size = twice", n: 1, offset: 1, to: "dim",
			edits: []string{`src/page.rtsx 14:28-14:32 "size={dim}"`, `src/page.rtsx 7:9-7:13 "dim"`},
			lines: []string{`const dim = twice(1);`, `<Table rows={labels} size={dim}>`},
		},
		{
			name: "shorthand: the prop renamed",
			rel:  table, needle: "size?: number", n: 1, offset: 1, to: "scale",
			edits: []string{
				`src/page.rtsx 14:28-14:32 "scale={size}"`,
				`src/table.rtsx 11:25-11:29 "scale: size"`, `src/table.rtsx 14:31-14:35 "scale: size"`,
				`src/table.rtsx 22:14-22:18 "scale={size}"`, `src/table.rtsx 3:3-3:7 "scale"`,
			},
			lines: []string{`<Table rows={labels} scale={size}>`, `<Sized scale={size} />`, `export function Sized({ scale: size }: Sizing) {`},
		},
		{
			name: "shorthand on an element: the binding renamed",
			rel:  page, needle: "hidden = false", n: 1, offset: 1, to: "gone",
			edits: []string{`src/page.rtsx 11:11-11:17 "hidden={gone}"`, `src/page.rtsx 9:9-9:15 "gone"`},
			lines: []string{`<main hidden={gone}>`},
		},
		{
			name: "shorthand, from the bare name itself: it is the binding",
			rel:  page, needle: "<UI.Box wide", n: 1, offset: 9, to: "broad",
			edits: []string{`src/page.rtsx 20:15-20:19 "wide={broad}"`, `src/page.rtsx 23:18-23:22 "broad"`, `src/page.rtsx 8:9-8:13 "broad"`},
			lines: []string{`const broad = true;`, `<UI.Box wide={broad}>`, `<Match on={broad}>`},
		},
		{
			name: "arg shorthand: the binding renamed",
			rel:  table, needle: "row = rows", n: 1, offset: 1, to: "first",
			edits: []string{`src/table.rtsx 16:9-16:12 "first"`, `src/table.rtsx 21:24-21:27 "row={first}"`},
			lines: []string{`<tr slot={$Row} &row={first} &&selected />`},
		},
		{
			name: "arg shorthand: the arg renamed",
			rel:  table, needle: "row: string", n: 1, offset: 1, to: "line",
			edits: []string{`src/page.rtsx 18:17-18:20 "line: row"`, `src/table.rtsx 21:24-21:27 "line={row}"`, `src/table.rtsx 9:41-9:44 "line"`},
			lines: []string{`<tr slot={$Row} &line={row} &&selected />`, `<$Row { line: row, selected }>{selected ? row : null}</$Row>`},
		},
		{
			name: "&&name: the binding renamed",
			rel:  table, needle: "selected = rows", n: 1, offset: 1, to: "chosen",
			edits: []string{`src/table.rtsx 15:9-15:17 "chosen"`, `src/table.rtsx 21:30-21:38 "selected={chosen}"`},
			lines: []string{`<tr slot={$Row} &row &&selected={chosen} />`},
		},
		{
			name: "a tag name with a closing tag: a component",
			rel:  page, needle: "<Box>", n: 1, offset: 2, to: "Panel",
			edits: []string{`src/page.rtsx 21:10-21:13 "Panel"`, `src/page.rtsx 21:17-21:20 "Panel"`, `src/page.rtsx 3:14-3:17 "Box as Panel"`},
			lines: []string{`<Panel>x</Panel>`},
		},
		{
			name: "a tag name with a closing tag: the same offset inside `<UI.Box>`",
			rel:  page, needle: "<UI.Box", n: 1, offset: 5, to: "Panel",
			edits: []string{`src/page.rtsx 20:11-20:14 "Panel"`, `src/page.rtsx 22:12-22:15 "Panel"`, `src/ui.tsx 4:21-4:24 "Panel: Box"`},
			lines: []string{`<UI.Panel wide>`, `</UI.Panel>`},
		},
		{
			name: "a tag name with a closing tag: the object of `<UI.Box>`",
			rel:  page, needle: "<UI.Box", n: 1, offset: 2, to: "Kit",
			edits: []string{`src/page.rtsx 20:8-20:10 "Kit"`, `src/page.rtsx 22:9-22:11 "Kit"`, `src/page.rtsx 3:10-3:12 "UI as Kit"`},
			lines: []string{`import { UI as Kit, Box } from "./ui";`, `<Kit.Box wide>`, `</Kit.Box>`},
		},
		{
			name: "a tag name whose closing tag is not emitted: all its children are slots",
			rel:  page, needle: "</Table", n: 1, offset: 3, to: "Grid",
			edits: []string{`src/page.rtsx 14:8-14:13 "Grid"`, `src/page.rtsx 19:9-19:14 "Grid"`, `src/page.rtsx 2:10-2:15 "Table as Grid"`},
			lines: []string{`<Grid rows={labels} size>`, `</Grid>`},
		},
		{
			name: "a slot tag: its closing tag, and the slot's declaration",
			rel:  page, needle: "<$Title", n: 1, offset: 3, to: "$Heading",
			edits: []string{
				`src/page.rtsx 15:10-15:16 "$Heading"`, `src/page.rtsx 15:38-15:44 "$Heading"`,
				`src/table.rtsx 14:37-14:43 "$Heading: $Title"`, `src/table.rtsx 7:3-7:9 "$Heading"`,
			},
			lines: []string{`<$Heading className="t">Users</$Heading>`, `export function Table({ rows, size, $Heading: $Title, $Column, $Row }: TableProps) {`},
		},
		{
			name: "a slot tag: every tag of its group, from the declaration",
			rel:  table, needle: "$Column: KeyedSlot", n: 1, offset: 2, to: "$Col",
			edits: []string{
				`src/page.rtsx 16:10-16:17 "$Col"`, `src/page.rtsx 17:10-17:17 "$Col"`, `src/page.rtsx 17:46-17:53 "$Col"`,
				`src/table.rtsx 14:45-14:52 "$Col: $Column"`, `src/table.rtsx 8:3-8:10 "$Col"`,
			},
			lines: []string{`<$Col key="name" label="Name" />`, `<$Col key="email" label="Email"></$Col>`},
		},
		{
			name: "a slot tag: from the second element of its group",
			rel:  page, needle: "<$Column", n: 2, offset: 3, to: "$Col",
			edits: []string{
				`src/page.rtsx 16:10-16:17 "$Col"`, `src/page.rtsx 17:10-17:17 "$Col"`, `src/page.rtsx 17:46-17:53 "$Col"`,
				`src/table.rtsx 14:45-14:52 "$Col: $Column"`, `src/table.rtsx 8:3-8:10 "$Col"`,
			},
		},
		{
			name: "a slot tag: from its closing tag",
			rel:  page, needle: "</$Column", n: 1, offset: 4, to: "$Col",
			edits: []string{
				`src/page.rtsx 16:10-16:17 "$Col"`, `src/page.rtsx 17:10-17:17 "$Col"`, `src/page.rtsx 17:46-17:53 "$Col"`,
				`src/table.rtsx 14:45-14:52 "$Col: $Column"`, `src/table.rtsx 8:3-8:10 "$Col"`,
			},
		},
		{
			name: "the `$X` of slot={$X}",
			rel:  table, needle: "slot={$Title}", n: 1, offset: 7, to: "$Heading",
			edits: []string{`src/table.rtsx 14:37-14:43 "$Title: $Heading"`, `src/table.rtsx 19:22-19:28 "$Heading"`},
			lines: []string{`<caption slot={$Heading} className="title">Untitled</caption>`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer restore(c)
			at := c.At(tc.rel, tc.needle, tc.n, tc.offset)
			if _, _, err := c.PrepareRename(tc.rel, at); err != nil {
				t.Fatalf("prepareRename: %v", err)
			}
			edit, err := c.Rename(tc.rel, at, tc.to)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got := c.Edits(edit); strings.Join(got, "\n") != strings.Join(tc.edits, "\n") {
				t.Errorf("edits:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(tc.edits, "\n  "))
			}
			c.Apply(edit)
			all := ""
			for _, rel := range renameDocs {
				all += c.Text(rel)
			}
			for _, line := range tc.lines {
				if !strings.Contains(all, line) {
					t.Errorf("after the rename, no line `%s`", line)
				}
			}
			if got := problems(c); len(got) != 0 {
				t.Errorf("after the rename: %q", got)
			}
		})
	}
}

// ide.md, *Rename*, "both renamed, the plain token": with TypeScript's
// `useAliasesForRenames` off, renaming a prop renames the binding
// destructured from it too, and a shorthand that is both stays one name.
func TestRenameShorthandBoth(t *testing.T) {
	c := startRename(t, lsptest.Options{Settings: map[string]any{"typescript": map[string]any{"preferences": map[string]any{"useAliasesForRenames": false}}}})
	edit, err := c.Rename("src/table.rtsx", c.At("src/table.rtsx", "size?: number", 1, 1), "scale")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`src/page.rtsx 14:28-14:32 "scale={size}"`, // Page's binding is another one
		`src/table.rtsx 11:25-11:29 "scale"`, `src/table.rtsx 12:14-12:18 "scale"`, `src/table.rtsx 14:31-14:35 "scale"`,
		`src/table.rtsx 22:14-22:18 "scale"`, // `<Sized size />`: the prop and the binding
		`src/table.rtsx 3:3-3:7 "scale"`,
	}
	if got := c.Edits(edit); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edits:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	c.Apply(edit)
	if text := c.Text("src/table.rtsx"); !strings.Contains(text, "<Sized scale />") {
		t.Errorf("table.rtsx:\n%s", text)
	}
	if got := problems(c); len(got) != 0 {
		t.Errorf("after the rename: %q", got)
	}
}

// ide.md, *Rename*: correct or refused. A refusal is an LSP error that
// names the place, and nothing is edited; where the refusal does not depend
// on the new name, prepareRename refuses too.
func TestRenameRefused(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page, table = "src/page.rtsx", "src/table.rtsx"
	for _, tc := range []struct {
		name        string
		rel, needle string
		n, offset   int
		to          string
		want        string // in the error
		prepare     bool   // prepareRename refuses with the same error
	}{
		{"&&name: the arg renamed", table, "selected: boolean", 1, 1, "chosen", "table.rtsx:21:30: `&&selected` names an arg and a prop at once", true},
		{"an occurrence that cannot be written back: a slot's body is its `children`", table, "children?: string", 1, 1, "content", "page.rtsx:15:31: `children` is also written by the transform, for `Users`", true},
		{"a slot tag, to a name without `$`", page, "<$Title", 1, 3, "Heading", "page.rtsx:15:10: `<$Title>` is a slot element; the new name must start with `$` too", false},
		{"a slot, from its declaration, to a name without `$`", table, "$Title?:", 1, 2, "heading", "page.rtsx:15:10: `<$Title>` is a slot element", false},
		{"a component, to a `$` name", page, "<Box>", 1, 2, "$Box", "page.rtsx:21:10: `<Box>` would become a slot element", false},
		{"the `$X` of slot={$X}, to a name without `$`", table, "slot={$Title}", 1, 7, "heading", "table.rtsx:19:22: `slot={$Title}` attaches a slot; the new name must start with `$` too", false},
		{"a binding, to a name that a bare attribute would then mean", page, "hidden = false", 1, 1, "disabled", "page.rtsx:13:14: the new name would capture a bare attribute", false},
		{"a name that does not parse where a shorthand is written out", page, "size = twice", 1, 1, "a-b", "page.rtsx", false},
		{"a segment root", page, "#intro", 1, 2, "outro", "A segment root cannot be renamed: `#intro`", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := c.At(tc.rel, tc.needle, tc.n, tc.offset)
			edit, err := c.Rename(tc.rel, at, tc.to)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "-32803") {
				t.Errorf("rename: %v %q, want a RequestFailed error with %q", err, c.Edits(edit), tc.want)
			}
			if _, _, err := c.PrepareRename(tc.rel, at); tc.prepare != (err != nil) || tc.prepare && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("prepareRename: %v; refuses: %v", err, tc.prepare)
			}
		})
	}
	if got := problems(c); len(got) != 0 {
		t.Errorf("after the refusals: %q", got)
	}
}

// ide.md, *Rename*, the row of `&&name`, on a component: the one token
// names the binding, the arg of the slot and the prop of the element. The
// binding renamed writes the value out; the arg or the prop renamed is
// refused — the prop has no token at all: its name is generated.
func TestRenameArgProp(t *testing.T) {
	const row = "src/row.rtsx"
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/jsx.d.ts": strings.Replace(renameApp["src/jsx.d.ts"], "interface IntrinsicElements", "interface IntrinsicAttributes { key?: unknown }\n  interface IntrinsicElements", 1),
		row: `import type { Slot } from "@reactogenic/core";
export function Cell(props: { wide?: boolean; className?: string; children?: unknown }) {
  return <td />;
}
export function Row({ $Cell }: { $Cell?: Slot<{ className?: string }, { wide: boolean }> }) {
  const wide = true;
  return (
    <tr>
      <Cell slot={$Cell} &&wide />
    </tr>
  );
}
`,
	}))
	c.Open(row)
	if got := lsptest.Lines(c.Diagnostics(row)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	const refusal = "row.rtsx:9:28: `&&wide` names an arg and a prop at once; only the binding behind it can be renamed"
	for name, needle := range map[string]string{"the prop": "wide?: boolean", "the arg": "wide: boolean"} {
		at := c.At(row, needle, 1, 1)
		if edit, err := c.Rename(row, at, "broad"); err == nil || !strings.Contains(err.Error(), refusal) {
			t.Errorf("%s renamed: %v %q", name, err, c.Edits(edit))
		}
		if _, _, err := c.PrepareRename(row, at); err == nil || !strings.Contains(err.Error(), refusal) {
			t.Errorf("%s: prepareRename: %v", name, err)
		}
	}
	// The binding, from its declaration and from the `&&wide` itself.
	for _, at := range []lsptest.Position{c.At(row, "wide = true", 1, 1), c.At(row, "&&wide", 1, 3)} {
		edit, err := c.Rename(row, at, "broad")
		if got := strings.Join(c.Edits(edit), ", "); err != nil || got != `src/row.rtsx 6:9-6:13 "broad", src/row.rtsx 9:28-9:32 "wide={broad}"` {
			t.Fatalf("the binding renamed: %v %s", err, got)
		}
	}
	edit, _ := c.Rename(row, c.At(row, "wide = true", 1, 1), "broad")
	c.Apply(edit)
	if text := c.Text(row); !strings.Contains(text, "<Cell slot={$Cell} &&wide={broad} />") {
		t.Errorf("after the rename:\n%s", text)
	}
	if got := lsptest.Lines(c.Diagnostics(row)); len(got) != 0 {
		t.Errorf("after the rename: %q", got)
	}
}

// What has no name to TypeScript is not offered for renaming: prepareRename
// refuses, with TypeScript's words, and a rename edits nothing.
func TestRenameNotOffered(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	for name, at := range map[string]lsptest.Position{
		"a `Switch` under another name: its tag is lowered away, its import dropped": c.At(page, "Choose,", 1, 1),
		"the tag of the lowered `Switch`":                                            c.At(page, "<Choose", 1, 2),
		"an intrinsic tag: an index signature declares it":                           c.At(page, "<main", 1, 2),
		"an attribute of a lowered element":                                          c.At(page, "exhaustive", 1, 2),
		"a declaration in node_modules, through a generic component's prop":          c.At(page, "items=", 1, 1),
	} {
		if _, shown, err := c.PrepareRename(page, at); err == nil || !strings.Contains(err.Error(), "You cannot rename") {
			t.Errorf("%s: prepareRename: %q, %v", name, shown, err)
		}
		// A client that does not prepare is given no edit either.
		if edit, _ := c.Rename(page, at, "fresh"); len(c.Edits(edit)) != 0 {
			t.Errorf("%s: rename edits %q", name, c.Edits(edit))
		}
	}
}

// A name that TypeScript cannot see — it is in the children that a segment
// root overwrites, which no virtual text holds — is renamed with its
// declaration: the source's own scopes find it.
func TestRenameReachesOverwrittenChildren(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	c.Change(page, strings.Replace(renameApp[page], "<section #intro />", "<section #intro>{labels.length}</section>", 1))
	warning := []string{"src/page.rtsx 12:16 segment-children Contents will be overwritten by the segment `intro`"}
	if got := problems(c); strings.Join(got, "\n") != strings.Join(warning, "\n") {
		t.Fatalf("before: %q", got)
	}
	edit, err := c.Rename("src/util.ts", c.At("src/util.ts", "labels =", 1, 1), "names")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`src/page.rtsx 12:24-12:30 "names"`, // in the overwritten children
		`src/page.rtsx 14:20-14:26 "names"`, `src/page.rtsx 26:20-26:26 "names"`, `src/page.rtsx 4:17-4:23 "names"`,
		`src/util.ts 2:14-2:20 "names"`,
	}
	if got := c.Edits(edit); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edits:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	c.Apply(edit)
	if got := problems(c); strings.Join(got, "\n") != strings.Join(warning, "\n") {
		t.Errorf("after: %q", got)
	}
}

// In such code a name that is not a reference — a member, here — cannot be
// told from the renamed one: the rename is refused, and so is its offer.
func TestRenameRefusedByOverwrittenChildren(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	c.Change(page, strings.Replace(renameApp[page], "<section #intro />", "<section #intro>{status.labels}</section>", 1))
	at := c.At("src/util.ts", "labels =", 1, 1)
	const want = "page.rtsx:12:31: `labels` here is in code that the transform leaves out"
	if edit, err := c.Rename("src/util.ts", at, "names"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("rename: %v %q", err, c.Edits(edit))
	}
	if _, _, err := c.PrepareRename("src/util.ts", at); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("prepareRename: %v", err)
	}
}

// Two renames that are TypeScript's own, the same in a .tsx file: a string
// literal is renamed where TypeScript matches it — not in the type that
// declares it — and the prop `children` without the element bodies that are
// its value. Both leave type errors, there as here. (Neither is refused:
// the occurrences TypeScript has are written back, all of them.)
func TestRenameIsTypeScripts(t *testing.T) {
	source := `export function Card(props: { kind: "a" | "b"; children?: unknown }) {
  return props.kind === "a" ? <b>a</b> : <i>b</i>;
}
export const card = <Card kind="a">x</Card>;
`
	want := []string{`<nil> file 2:26-2:27 "fresh"; file 4:33-4:34 "fresh"`, `<nil> file 1:48-1:56 "fresh"`}
	for _, rel := range []string{"src/plain.tsx", "src/mapped.rtsx"} {
		c := start(t, map[string]string{"src/jsx.d.ts": renameApp["src/jsx.d.ts"], rel: source})
		c.Open(rel)
		var got []string
		for _, needle := range []string{`=== "a"`, "children?"} {
			edit, err := c.Rename(rel, c.At(rel, needle, 1, strings.IndexAny(needle, "ac")), "fresh")
			got = append(got, fmt.Sprint(err, " ", strings.ReplaceAll(strings.Join(c.Edits(edit), "; "), rel, "file")))
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s:\n  %s\nwant:\n  %s", rel, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	}
}

// A rename that reaches a file whose virtual text is not its lowered source
// is refused: code that was left out may hold the name.
func TestRenameRefusedByBrokenFile(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	// An orphaned slot element: `null` stands in for it, and `labels` in it
	// is in no virtual text.
	c.Change(page, strings.Replace(renameApp[page], "<input disabled />", "<div><$Orphan>{labels}</$Orphan></div>", 1))
	for _, from := range []string{"src/util.ts", page} {
		needle := map[string]string{"src/util.ts": "labels =", page: "labels }"}[from]
		at := c.At(from, needle, 1, 1)
		edit, err := c.Rename(from, at, "names")
		if err == nil || !strings.Contains(err.Error(), "page.rtsx has an error that keeps part of it out of the transpiled code") {
			t.Errorf("from %s: %v %q", from, err, c.Edits(edit))
		}
		if _, _, err := c.PrepareRename(from, at); err == nil {
			t.Errorf("from %s: prepareRename does not refuse", from)
		}
	}
	// A name the broken file does not hold is renamed.
	if edit, err := c.Rename("src/table.rtsx", c.At("src/table.rtsx", "Sized(", 1, 1), "Fixed"); err != nil || len(c.Edits(edit)) != 2 {
		t.Errorf("a rename that does not reach the file: %v %q", err, c.Edits(edit))
	}
}

// ide.md, *Rename*: "no rename in the fixture project leaves it with a new
// diagnostic". Every identifier of a fixture — each word of its .ts, .tsx
// and .rtsx files outside a string, declarations and uses, keywords too
// (on `function` TypeScript renames the function) — is renamed to a fresh
// name, the edit applied, the diagnostics of every file pulled, the texts
// put back. A rename is correct or refused, and prepareRename refuses
// where the rename does: the fresh name keeps the `$` and the case of the
// old one, so no refusal here depends on it.
//
// One name is left out, and is TypeScript's own in a .tsx file as well
// (TestRenameIsTypeScripts): the prop `children` of a component — an
// element's body has no token that names it.
//
// Two projects: the rename fixture, and the one of the feature scenarios
// (TestFeatures).
func TestRenameEveryName(t *testing.T) {
	// Why a name is not renamed: it is not a name (TypeScript's answer for
	// a keyword of a statement, a JSX text, an attribute of an element, a
	// name the transform lowers away); it is declared in a library; or one
	// of the table's refusals.
	known := []string{
		"You cannot rename this element.",
		"You cannot rename elements that are defined in the standard TypeScript library.",
		"You cannot rename elements that are defined in a 'node_modules' folder.",
		"File rename is not supported by the editor", // `from`: the module, for a client that renames files
		"names an arg and a prop at once",
		"is also written by the transform",
		"A segment root cannot be renamed",
	}
	for _, fixture := range []struct {
		name    string
		files   map[string]string
		renames int      // at least
		refused []string // the table's refusals that the fixture has
	}{
		// `&&selected` from its arg, `children` of a slot from its body, `#intro`.
		{"the rename fixture", renameApp, 130, known[4:]},
		{"the feature fixture", app, 80, []string{"is also written by the transform", "A segment root cannot be renamed"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var docs []string
			for rel := range fixture.files {
				if strings.HasPrefix(rel, "src/") && !strings.HasSuffix(rel, ".d.ts") {
					docs = append(docs, rel)
				}
			}
			sort.Strings(docs)
			renamed, refused := renameEveryName(t, fixture.files, docs)
			for why := range refused {
				expected := false
				for _, k := range known {
					expected = expected || strings.Contains(why, k)
				}
				if !expected {
					t.Errorf("an unexpected refusal: %s — %q", why, refused[why])
				}
			}
			if raceDetector {
				return // a part of the names
			}
			for _, k := range fixture.refused {
				found := false
				for why := range refused {
					found = found || strings.Contains(why, k)
				}
				if !found {
					t.Errorf("no refusal with %q: the fixture has such a name", k)
				}
			}
			if renamed < fixture.renames {
				t.Errorf("%d renames: the fixture has more names than that", renamed)
			}
		})
	}
}

// renameEveryName renames each name of docs, the files of a project that
// has no diagnostic; it returns how many it renamed, and the names that
// prepareRename refused, by reason.
func renameEveryName(t *testing.T, files map[string]string, docs []string) (renamed int, refused map[string][]string) {
	c := start(t, files)
	problems := func() []string {
		out := []string{}
		for _, rel := range docs {
			for _, d := range c.Diagnostics(rel) {
				out = append(out, rel+" "+d.String()+" "+d.Message)
			}
		}
		return out
	}
	for _, rel := range docs {
		c.Open(rel)
	}
	if got := problems(); len(got) != 0 {
		t.Fatalf("the fixture has diagnostics: %q", got)
	}
	word := regexp.MustCompile(`[$A-Za-z_][$\w]*`)
	fresh := func(old string, n int) string {
		switch {
		case strings.HasPrefix(old, "$"):
			return fmt.Sprintf("$Fresh%d", n)
		case unicode.IsUpper(rune(old[0])):
			return fmt.Sprintf("Fresh%d", n)
		}
		return fmt.Sprintf("fresh%d", n)
	}
	var tried, edits int
	refused = map[string][]string{}
	step := 1
	if raceDetector {
		step = 4 // every fourth name: the detector makes each rename ten times slower
	}
	for _, rel := range docs {
		text := files[rel]
		for i, at := range word.FindAllStringIndex(text, -1) {
			old := text[at[0]:at[1]]
			line := text[strings.LastIndex(text[:at[0]], "\n")+1 : at[0]]
			if i%step != 0 || strings.Count(line, `"`)%2 == 1 || old == "children" && !strings.HasSuffix(rel, ".rtsx") {
				continue
			}
			// Inside the name: its end is also the end of what follows it.
			position := lsptest.PositionAt(text, at[0]+len(old)/2)
			where := fmt.Sprintf("%s:%d:%d `%s`", rel, position.Line+1, position.Character+1, old)
			tried++
			// The name the editor would show: on a keyword it is the name of
			// the declaration the keyword starts.
			_, name, prepareErr := c.PrepareRename(rel, position)
			if prepareErr == nil && name != "" {
				old = name[strings.LastIndex(name, ".")+1:]
			}
			edit, err := c.Rename(rel, position, fresh(old, tried))
			if outside := outsideFixture(c, files, edit); len(outside) != 0 {
				t.Errorf("%s: the rename edits %q", where, outside)
				continue
			}
			switch {
			case prepareErr != nil && err == nil && len(c.Edits(edit)) > 0:
				t.Errorf("%s: prepareRename refuses (%v) and rename edits %q", where, prepareErr, c.Edits(edit))
				continue
			case prepareErr != nil:
				refused[reason(prepareErr)] = append(refused[reason(prepareErr)], where)
				continue
			case err != nil:
				t.Errorf("%s: prepareRename accepts, rename refuses: %v", where, err)
				continue
			case len(c.Edits(edit)) == 0:
				t.Errorf("%s: prepareRename accepts, rename edits nothing", where)
				continue
			}
			renamed++
			edits += len(c.Edits(edit))
			c.Apply(edit)
			if got := problems(); len(got) != 0 {
				t.Errorf("%s → %s leaves %q\n  edits: %q", where, fresh(old, tried), got, c.Edits(edit))
			}
			for _, rel := range docs {
				if c.Text(rel) != files[rel] {
					c.Change(rel, files[rel])
				}
			}
		}
	}
	var reasons []string
	for why, where := range refused {
		reasons = append(reasons, fmt.Sprintf("%d × %s (first: %s)", len(where), why, where[0]))
	}
	sort.Strings(reasons)
	t.Logf("%d names tried: %d renamed (%d edits); refused:\n  %s", tried, renamed, edits, strings.Join(reasons, "\n  "))
	if got := problems(); len(got) != 0 {
		t.Errorf("at the end: %q", got)
	}
	return renamed, refused
}

// outsideFixture are the edits of files that are not the fixture's own.
func outsideFixture(c *lsptest.Client, files map[string]string, edit lsptest.WorkspaceEdit) []string {
	var out []string
	for _, line := range c.Edits(edit) {
		rel, _, _ := strings.Cut(line, " ")
		if _, ours := files[rel]; !ours || strings.HasPrefix(rel, "node_modules/") {
			out = append(out, line)
		}
	}
	return out
}

// reason is a refusal without its place.
func reason(err error) string {
	return regexp.MustCompile(`\w+\.rtsx:\d+:\d+`).ReplaceAllString(err.Error(), "…")
}
