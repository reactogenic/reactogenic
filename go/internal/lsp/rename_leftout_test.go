package lsp_test

import (
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// Code that no virtual text holds (ide.md, *Rename*): the children that a
// segment root overwrites, and every element of a repeated slot but the last
// — the last one wins. TypeScript never sees a name there.
var leftOutApp = lsptest.With(lsptest.Core, map[string]string{
	"src/jsx.d.ts": renameApp["src/jsx.d.ts"],
	"src/kit.rtsx": `import type { Slot } from "@reactogenic/core";
export function Chip(props: { label: string }) {
  return <i>{props.label}</i>;
}
export function Card({ $Header }: { $Header?: Slot<{ tone?: string; children?: unknown }> }) {
  return <article><header slot={$Header}>h</header></article>;
}
`,
	"src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
	"src/page.rtsx": `import { Card, Chip } from "./kit";
export function Page({ wide }: { wide: boolean }) {
  return (
    <main hidden={wide}>
      <section #intro><Chip label="a" /></section>
      <Card>
        <$Header tone="one"><Chip label="b"></Chip></$Header>
        <$Header>two</$Header>
      </Card>
    </main>
  );
}
`,
})

var leftOutDocs = []string{"src/intro.rtsx", "src/kit.rtsx", "src/page.rtsx"}

func startLeftOut(t *testing.T, page string) *lsptest.Client {
	t.Helper()
	files := leftOutApp
	if page != "" {
		files = lsptest.With(leftOutApp, map[string]string{"src/page.rtsx": page})
	}
	c := start(t, files)
	for _, rel := range leftOutDocs {
		c.Open(rel)
	}
	return c
}

// errorsOf are the errors of the documents — not the warning that a segment
// root's children are overwritten.
func errorsOf(c *lsptest.Client, docs []string) []string {
	out := []string{}
	for _, rel := range docs {
		for _, d := range c.Diagnostics(rel) {
			if d.Severity == 1 {
				out = append(out, rel+" "+d.String()+" "+d.Message)
			}
		}
	}
	return out
}

// ide.md, *Rename*, the row of a reference in code that no virtual text
// holds: a component's tag there is a reference that the source's scopes
// find — it is renamed with its declaration, its closing tag with it. The
// oracle: the code that hides the left-out code is taken away afterwards, and
// TypeScript — which sees it then for the first time — has nothing to say.
func TestRenameTagInLeftOutCode(t *testing.T) {
	const page, kit = "src/page.rtsx", "src/kit.rtsx"
	for _, from := range []struct {
		name, rel, needle string
		offset            int
	}{
		{"from the declaration, in another file", kit, "Chip(", 1},
		{"from the import", page, "Chip }", 1},
	} {
		t.Run(from.name, func(t *testing.T) {
			c := startLeftOut(t, "")
			if got := errorsOf(c, leftOutDocs); len(got) != 0 {
				t.Fatalf("the fixture has errors: %q", got)
			}
			at := c.At(from.rel, from.needle, 1, from.offset)
			if _, _, err := c.PrepareRename(from.rel, at); err != nil {
				t.Fatalf("prepareRename: %v", err)
			}
			edit, err := c.Rename(from.rel, at, "Tag")
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			c.Apply(edit)
			text := c.Text(page)
			for _, line := range []string{`<section #intro><Tag label="a" /></section>`, `<$Header tone="one"><Tag label="b"></Tag></$Header>`} {
				if !strings.Contains(text, line) {
					t.Errorf("after the rename, no `%s` in\n%s\nedits: %q", line, text, c.Edits(edit))
				}
			}
			// What was left out, in the open.
			c.Change(page, strings.Replace(strings.Replace(text, " #intro", "", 1), "        <$Header>two</$Header>\n", "", 1))
			if got := errorsOf(c, leftOutDocs); len(got) != 0 {
				t.Errorf("with the left-out code in the open: %q", got)
			}
		})
	}
}

// A name in such code that the scopes cannot decide — the name of an
// attribute, a slot tag, an intrinsic tag, a name that nothing in the file
// declares — may or may not be the renamed one: the rename is refused, and
// not offered. It is the same whether the file has an occurrence that
// TypeScript found or none (a prop that is only used in left-out code).
func TestRenameRefusedByLeftOutCode(t *testing.T) {
	const page, kit = "src/page.rtsx", "src/kit.rtsx"
	const left = "here is in code that the transform leaves out"
	for _, tc := range []struct {
		name        string
		page        string // replacements of the fixture's page, in pairs
		rel, needle string
		offset      int
		to, want    string
	}{
		{"an attribute under a segment root: the file has no other occurrence", "", kit, "label: string", 1, "text", "page.rtsx:5:29: `label` " + left},
		{"an attribute in the first element of a repeated slot", `<section #intro><Chip label="a" /></section>|<section #intro />`, kit, "label: string", 1, "text", "page.rtsx:7:35: `label` " + left},
		{"an attribute of a repeated slot's first element", "", kit, "tone?", 1, "mood", "page.rtsx:7:18: `tone` " + left},
		{"a slot tag under a segment root", `<Chip label="a" />|<Card><$Header>x</$Header></Card>`, kit, "$Header?", 2, "$Top", "page.rtsx:5:30: `$Header` " + left},
		{"a bare attribute bound to the renamed binding", `<Chip label="a" />|<input hidden={wide} wide />`, page, "{ wide }", 3, "broad", "page.rtsx:5:44: `wide` " + left},
		{"a name that nothing in the file declares", `<Chip label="a" />|{label}`, kit, "label: string", 1, "text", "page.rtsx:5:24: `label` " + left},
		{"a member", `<Chip label="a" />|{Card.label}`, kit, "label: string", 1, "text", "page.rtsx:5:29: `label` " + left},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := leftOutApp[page]
			if tc.page != "" {
				old, new, _ := strings.Cut(tc.page, "|")
				if !strings.Contains(text, old) {
					t.Fatalf("no `%s` in the fixture", old)
				}
				text = strings.Replace(text, old, new, 1)
			}
			c := startLeftOut(t, text)
			if got := errorsOf(c, leftOutDocs); len(got) != 0 {
				t.Fatalf("the fixture has errors: %q", got)
			}
			at := c.At(tc.rel, tc.needle, 1, tc.offset)
			if edit, err := c.Rename(tc.rel, at, tc.to); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("rename: %v %q, want a refusal with %q", err, c.Edits(edit), tc.want)
			}
			if _, _, err := c.PrepareRename(tc.rel, at); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("prepareRename: %v, want a refusal with %q", err, tc.want)
			}
		})
	}
}

// What the passes read in code that is lowered — the `on` of a `Match`, the
// `slot` of an attachment, a slot's tag after its first — has no copy in the
// virtual text either, and is no left-out code: a rename of another `on` or
// `slot` is not refused by it.
func TestRenameIsNotRefusedByLoweredNames(t *testing.T) {
	const rel = "src/x.rtsx"
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/jsx.d.ts": renameApp["src/jsx.d.ts"],
		rel: `import { Match, Switch } from "@reactogenic/core";
import type { Slot, KeyedSlot } from "@reactogenic/core";
export function List({ on, slot, is, $Row }: { on: boolean; slot: string; is: "a" | "b"; $Row: KeyedSlot<{ title?: string }> }) {
  return (
    <ul title={slot}>
      <Match on={on}><li slot={$Row} key="a" /></Match>
      <Switch on={is}>
        <$Case is="a">a</$Case>
        <$Case default>b</$Case>
      </Switch>
    </ul>
  );
}
export const list = (
  <List on slot="s" is="a">
    <$Row key="a" title="A" />
    <$Row key="b" title="B"></$Row>
  </List>
);
`,
	}))
	c.Open(rel)
	if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	for needle, to := range map[string]string{"on: boolean": "open", "slot: string": "place", "is: ": "which", "title?": "label"} {
		at := c.At(rel, needle, 1, 1)
		if _, _, err := c.PrepareRename(rel, at); err != nil {
			t.Errorf("`%s`: prepareRename: %v", needle, err)
		}
		edit, err := c.Rename(rel, at, to)
		if err != nil {
			t.Errorf("`%s`: %v", needle, err)
			continue
		}
		before := c.Text(rel)
		c.Apply(edit)
		if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
			t.Errorf("`%s` → `%s`: %q\n  edits: %q", needle, to, got, c.Edits(edit))
		}
		c.Change(rel, before)
	}
}
