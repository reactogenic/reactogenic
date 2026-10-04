package lsp_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// A container whose slot is itself a component with slots (recursive slots),
// and a page that fills them directly, under a `Match`, and nested.
var dialogApp = lsptest.With(lsptest.Core, map[string]string{
	"src/jsx.d.ts": renameApp["src/jsx.d.ts"],
	"src/kit.rtsx": `import type { Slot } from "@reactogenic/core";
export interface ButtonProps {
  $Icon?: Slot<{ className?: string; children?: unknown }>;
  $Badge?: Slot<{ children?: unknown }>;
  "$sub-item"?: Slot<{ title?: string; children?: unknown }>;
  children?: unknown;
}
export function Button({ $Icon, $Badge, "$sub-item": $sub, children }: ButtonProps) {
  return <button><i slot={$Icon} /><b slot={$Badge} /><u slot={$sub} />{children}</button>;
}
export interface DialogProps {
  $Action?: Slot<ButtonProps>;
  $Title?: Slot<{ children?: unknown }>;
  $Close?: Slot<{ children?: unknown }>;
}
export function Dialog({ $Action, $Title, $Close }: DialogProps) {
  return <div><h1 slot={$Title} /><Button slot={$Action} /><u slot={$Close} /></div>;
}
export const Kit = { Dialog, Deep: { Dialog } };
`,
	"src/page.rtsx": `import { Match, Switch } from "@reactogenic/core";
import { Dialog, Button, Kit } from "./kit";
export function Page({ flag, mode }: { flag: boolean; mode: "a" | "b" }) {
  return (
    <main>
      <Dialog>
        <$Action>
          <$Icon className="i">x</$Icon>
          <$sub-item title="t">s</$sub-item>
          Close
        </$Action>
        <Match on={flag}>
          <$Title>t</$Title>
        </Match>
      </Dialog>
      <Dialog>
        <Switch on={mode}>
          <$Case is="a"><$Close>1</$Close></$Case>
          <$Case is="b"><$Close>2</$Close></$Case>
        </Switch>
      </Dialog>
      <Button>
        <$sub-item title="t" />
      </Button>
      <Kit.Dialog>
        <$Title>k</$Title>
      </Kit.Dialog>
      <Kit.Deep.Dialog>
        <$Title>d</$Title>
      </Kit.Deep.Dialog>
    </main>
  );
}
`,
})

func startDialog(t *testing.T) *lsptest.Client {
	t.Helper()
	c := start(t, dialogApp)
	for _, rel := range []string{"src/kit.rtsx", "src/page.rtsx"} {
		c.Open(rel)
		if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
			t.Fatalf("the fixture has diagnostics: %s %q", rel, got)
		}
	}
	return c
}

// slotNames are the labels of a completion, without their `?`, sorted.
func slotNames(labels []string) string {
	names := []string{}
	for _, label := range labels {
		names = append(names, strings.TrimSuffix(label, "?"))
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

// ide.md, *Slots*: after `<$` the list is exactly the `$` props of the
// component the slot would be a slot of — also when a slot of it is filled
// under a `Match` or in the cases of a `Switch` (TypeScript omits it: the
// prop is written), when the `<$` itself is typed under one, and next to a
// slot element that has slots of its own.
func TestSlotCompletionOfTheOwner(t *testing.T) {
	c := startDialog(t)
	const page = "src/page.rtsx"
	source := dialogApp[page]
	const dialog, button = "$Action $Close $Title", "$Badge $Icon $sub-item"
	for _, tc := range []struct {
		name, old, new string
		typed          string // the completion is asked at its end
		want           string
	}{
		{"after a slot that is filled under a Match", "        </Match>\n      </Dialog>", "        </Match>\n        <$\n      </Dialog>", "</Match>\n        <$", dialog},
		{"after slots that are filled in the cases of a Switch", "        </Switch>\n", "        </Switch>\n        <$\n", "</Switch>\n        <$", dialog},
		{"under a Match, the tag still open", "          <$Title>t</$Title>\n", "          <$\n", "<Match on={flag}>\n          <$", dialog},
		{"under a Match, a letter typed", "          <$Title>t</$Title>\n", "          <$T\n", "<Match on={flag}>\n          <$T", dialog},
		{"under a Match, the tag closed with `>`", "          <$Title>t</$Title>\n", "          <$>\n", "<Match on={flag}>\n          <$", dialog},
		{"under a Match, the element complete", "          <$Title>t</$Title>\n", "          <$></$>\n", "<Match on={flag}>\n          <$", dialog},
		{"in a case of a Switch, the tag still open", `<$Case is="b"><$Close>2</$Close></$Case>`, `<$Case is="b"><$</$Case>`, `<$Case is="b"><$`, dialog},
		{"before a slot element that has slots of its own", "        <$Action>\n", "        <$\n        <$Action>\n", "<Dialog>\n        <$", dialog},
		{"inside that slot element: the slots of its component", "          Close\n", "          Close\n          <$\n", "Close\n          <$", button},
	} {
		if !strings.Contains(source, tc.old) {
			t.Fatalf("%s: no %q in the fixture", tc.name, tc.old)
		}
		c.Change(page, strings.Replace(source, tc.old, tc.new, 1))
		if got := slotNames(labelsAt(c, page, tc.typed)); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	c.Change(page, source)
	// The name of a closing tag is not a place to choose a slot: no list.
	for _, closing := range []string{"</$Title", "</$Action", "</$sub-item"} {
		if got := c.Completion(page, c.At(page, closing, 1, 4)); len(got) != 0 {
			t.Errorf("in `%s>`: %d items", closing, len(got))
		}
	}
}

// ide.md, *Span map* → slot groups: references list every tag of a group
// and both names of an element in every .rtsx file of the answer, whichever
// document asks — the container's declaration too, and with the page closed.
func TestReferencesListEveryTag(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page, table = "src/page.rtsx", "src/table.rtsx"
	references := func(rel, needle string, offset int) string {
		var locations []lsptest.Location
		params := map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}, "position": c.At(rel, needle, 1, offset), "context": map[string]any{"includeDeclaration": true}}
		c.Request("textDocument/references", params, &locations)
		var found []string
		for _, l := range locations {
			found = append(found, c.Rel(l.URI)+" "+l.Range.String())
		}
		sort.Strings(found)
		return strings.Join(found, ", ")
	}
	// The prop: its tags, its declaration, the pattern that destructures it.
	// From the pattern, which is also the container's binding, that binding's
	// use too (`slot={$Column}`) — TypeScript's two symbols of `{ $Column }`.
	const tags = "src/page.rtsx 16:10-16:17, src/page.rtsx 17:10-17:17, src/page.rtsx 17:46-17:53, "
	const prop = tags + "src/table.rtsx 14:45-14:52, src/table.rtsx 8:3-8:10"
	const both = tags + "src/table.rtsx 14:45-14:52, src/table.rtsx 20:17-20:24, src/table.rtsx 8:3-8:10"
	const tableTags = "src/page.rtsx 14:8-14:13, src/page.rtsx 19:9-19:14, src/page.rtsx 2:10-2:15, src/table.rtsx 14:17-14:22"
	check := func(when string) {
		t.Helper()
		for from, at := range map[string][4]any{
			"a tag in the page":                      {page, "<$Column", 3, prop},
			"the declaration in the container":       {table, "$Column: KeyedSlot", 2, prop},
			"the binding the container destructures": {table, "$Column, $Row }", 2, both},
		} {
			if rel := at[0].(string); rel == page && when != "" {
				continue
			} else if got := references(rel, at[1].(string), at[2].(int)); got != at[3].(string) {
				t.Errorf("%s%s: the slot's references\n  %s\nwant\n  %s", from, when, got, at[3])
			}
		}
		// A component whose closing tag has no copy: all its children are slots.
		if got := references(table, "function Table", 10); got != tableTags {
			t.Errorf("the component's declaration%s: references\n  %s\nwant\n  %s", when, got, tableTags)
		}
	}
	check("")
	if got := references(page, "<Table", 2); got != tableTags {
		t.Errorf("the component's tag: references\n  %s\nwant\n  %s", got, tableTags)
	}
	// The page closed: its text is the file's.
	c.Close(page)
	check(", the page closed")
}

// Asked from a .tsx document — by a client that attaches the server to it —
// the answer names the .rtsx file, and has every tag there too: the closing
// tag of an element that is emitted without one.
func TestReferencesFromATsxFile(t *testing.T) {
	const card, page = "src/card.tsx", "src/x.rtsx"
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		card: "import type { Slot } from \"@reactogenic/core\";\nexport function Card(props: { $Title?: Slot<{ children?: string }>; children?: string }) {\n  return <div />;\n}\n",
		page: "import { Card } from \"./card\";\nexport const a = (\n  <Card>\n    <$Title>T</$Title>\n  </Card>\n);\n",
	}))
	c.Open(card)
	for _, open := range []bool{false, true} {
		if open {
			c.Open(page)
		}
		var locations []lsptest.Location
		c.Request("textDocument/references", map[string]any{"textDocument": map[string]any{"uri": c.URI(card)}, "position": c.At(card, "Card(", 1, 1), "context": map[string]any{"includeDeclaration": true}}, &locations)
		var found []string
		for _, l := range locations {
			found = append(found, c.Rel(l.URI)+" "+l.Range.String())
		}
		sort.Strings(found)
		if got, want := strings.Join(found, ", "), "src/card.tsx 2:17-2:21, src/x.rtsx 1:10-1:14, src/x.rtsx 3:4-3:8, src/x.rtsx 5:5-5:9"; got != want {
			t.Errorf("the page open: %v: references\n  %s\nwant\n  %s", open, got, want)
		}
	}
}

// ide.md, *Span map*, "Closing tags likewise": on the closing tag of a
// member-expression tag that is emitted self-closing — `</Kit.Dialog>` — the
// request is answered at the opening tag, and the answer's own range is the
// part of the closing tag asked about: `Kit`, or `Dialog`.
func TestClosingTagWithMembers(t *testing.T) {
	c := startDialog(t)
	const page = "src/page.rtsx"
	doc := map[string]any{"uri": c.URI(page)}
	for _, tc := range []struct {
		needle string
		offset int
		token  string
		hover  string
	}{
		{"</Kit.Dialog", 3, "Kit", "const Kit"},
		{"</Kit.Dialog", 8, "Dialog", "Dialog"},
		{"</Kit.Deep.Dialog", 7, "Deep", "Deep"},
		{"</Kit.Deep.Dialog", 13, "Dialog", "Dialog"},
	} {
		at := c.At(page, tc.needle, 1, tc.offset)
		params := map[string]any{"textDocument": doc, "position": at}
		var hover struct {
			Contents struct{ Value string }
			Range    lsptest.Range
		}
		c.Request("textDocument/hover", params, &hover)
		if !strings.Contains(hover.Contents.Value, tc.hover) || hover.Range.Start.Line != at.Line || c.RangeText(page, hover.Range) != tc.token {
			t.Errorf("%s +%d: hover %q at %s (`%s`), want `%s` on line %d", tc.needle, tc.offset, hover.Contents.Value, hover.Range, c.RangeText(page, hover.Range), tc.token, at.Line+1)
		}
		shown, name, err := c.PrepareRename(page, at)
		if err != nil || shown.Start.Line != at.Line || c.RangeText(page, shown) != tc.token || !strings.HasSuffix(name, tc.token) {
			t.Errorf("%s +%d: prepareRename %s (`%s`) %q %v, want `%s` on line %d", tc.needle, tc.offset, shown, c.RangeText(page, shown), name, err, tc.token, at.Line+1)
		}
		var links []struct {
			Origin lsptest.Range `json:"originSelectionRange"`
		}
		c.Request("textDocument/definition", params, &links)
		if len(links) == 0 {
			t.Errorf("%s +%d: no definition", tc.needle, tc.offset)
		}
		for _, link := range links { // `Dialog` of `{ Dialog }`: the property, and the function
			if link.Origin.Start.Line != at.Line || c.RangeText(page, link.Origin) != tc.token {
				t.Errorf("%s +%d: definition from %s, want `%s` on line %d", tc.needle, tc.offset, link.Origin, tc.token, at.Line+1)
			}
		}
	}
}

// A slot whose name is no identifier is a quoted key in the virtual text
// when its owner is a slot element (`"$sub-item": { … }`), and a quoted name
// where it is declared. The name that prepareRename shows is the name as it
// is written at the cursor — without the quotes, which are not in its range.
func TestPrepareRenameOfAQuotedName(t *testing.T) {
	c := startDialog(t)
	const page, kit = "src/page.rtsx", "src/kit.rtsx"
	for _, tc := range []struct {
		name, rel, needle string
		n, offset         int
	}{
		{"a tag under a slot element: a quoted key", page, "<$sub-item", 1, 3},
		{"its closing tag", page, "</$sub-item", 1, 4},
		{"a tag under a component: an attribute", page, "<$sub-item", 2, 3},
		{"the declaration", kit, `"$sub-item"?`, 1, 3},
	} {
		shown, name, err := c.PrepareRename(tc.rel, c.At(tc.rel, tc.needle, tc.n, tc.offset))
		if err != nil || name != "$sub-item" || c.RangeText(tc.rel, shown) != "$sub-item" {
			t.Errorf("%s: prepareRename %s (`%s`) %q %v", tc.name, shown, c.RangeText(tc.rel, shown), name, err)
		}
	}
	// And the rename from the nested tag is whole.
	edit, err := c.Rename(page, c.At(page, "<$sub-item", 1, 3), "$sub-thing")
	if err != nil {
		t.Fatal(err)
	}
	c.Apply(edit)
	for _, rel := range []string{kit, page} {
		if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
			t.Errorf("after the rename: %s %q\n  edits: %q", rel, got, c.Edits(edit))
		}
	}
	if text := c.Text(page); strings.Count(text, "$sub-thing") != 3 || strings.Contains(text, "$sub-item") {
		t.Errorf("after the rename:\n%s", text)
	}
}
