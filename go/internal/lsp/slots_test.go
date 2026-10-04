package lsp_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// labels are the labels of the completion at the end of the n-th typed.
func labelsAt(c *lsptest.Client, rel, typed string) []string {
	out := []string{}
	for _, item := range c.Completion(rel, c.At(rel, typed, 1, len(typed))) {
		out = append(out, item.Label)
	}
	return out
}

// ide.md, *Slots*: after `<$` inside a component's children, exactly its
// declared slots — TypeScript's items for those that are not written yet,
// then the written ones. The inserted text is the bare name. (Under a
// `Match`, in a `$Case`, next to nested slots: TestSlotCompletionOfTheOwner.)
func TestSlotCompletion(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	slots := renameApp[page][strings.Index(renameApp[page], "        <$Title"):strings.Index(renameApp[page], "      </Table>")]
	typing := func(written string) { c.Change(page, strings.Replace(renameApp[page], slots, written, 1)) }
	defer restore(c)

	// Table has `rows` and `size` too: props, not slots. (`size` is not
	// written here, so TypeScript offers it.)
	c.Change(page, strings.Replace(strings.Replace(renameApp[page], slots, "<$\n", 1), "<Table rows={labels} size>", "<Table rows={labels}>", 1))
	if got := labelsAt(c, page, "<$"); strings.Join(got, " ") != "$Title? $Column $Row?" {
		t.Errorf("`<$` in <Table>: %q", got)
	}
	typing("<$Ti\n")
	items := c.Completion(page, c.At(page, "<$Ti", 1, 4))
	if len(items) != 3 {
		t.Fatalf("`<$Ti`: %d items", len(items))
	}
	for _, item := range items {
		var raw struct {
			InsertText string          `json:"insertText"`
			TextEdit   json.RawMessage `json:"textEdit"`
		}
		json.Unmarshal(item.Raw, &raw)
		if name := strings.TrimSuffix(item.Label, "?"); raw.InsertText != name || raw.TextEdit != nil {
			t.Errorf("%s inserts %q (edit %s), want the bare name", item.Label, raw.InsertText, raw.TextEdit)
		}
	}

	// After a keyed slot's first entry: TypeScript leaves `$Column` out — the
	// prop is there — and a keyed slot is filled many times.
	typing("<$Column key=\"name\" label=\"Name\" />\n        <$\n")
	if got := labelsAt(c, page, "/>\n        <$"); strings.Join(got, " ") != "$Title? $Row? $Column" {
		t.Errorf("after a keyed slot's first entry: %q", got)
	}
	// The appended name is the front's own item: resolving it changes nothing.
	for _, item := range c.Completion(page, c.At(page, "/>\n        <$", 1, 13)) {
		if item.Label == "$Column" {
			if resolved := c.Resolve(item); resolved.Label != "$Column" {
				t.Errorf("resolved: %s", resolved.Raw)
			}
		}
	}
	// The same name again, complete: the second tag of its group, answered
	// at the first.
	typing("<$Column key=\"name\" label=\"Name\" />\n        <$Column\n")
	if got := labelsAt(c, page, "/>\n        <$Column"); strings.Join(got, " ") != "$Title? $Column $Row?" {
		t.Errorf("at the second tag of a group: %q", got)
	}
	// Everything filled: what is written, once each.
	typing(slots + "        <$\n")
	if got := labelsAt(c, page, "</$Row>\n        <$"); strings.Join(got, " ") != "$Title? $Column $Row?" {
		t.Errorf("with every slot written: %q", got)
	}
	// Under a `Switch` — lowered away, so TypeScript is never asked.
	c.Change(page, strings.Replace(renameApp[page], `<$Case is="ready">Ready</$Case>`, "<$", 1))
	if got := labelsAt(c, page, "</$Case>\n        <$"); strings.Join(got, " ") != "$Case" {
		t.Errorf("under a Switch: %q", got)
	}
	// Inside a slot tag, its props (the attribute names are the object's keys).
	c.Change(page, renameApp[page])
	if got := labelsAt(c, page, `<$Column key="name" la`); strings.Join(got, " ") != "label?" {
		t.Errorf("props of a slot: %q", got)
	}
}

// ide.md, *Span map* → slot groups, and *Slots*: a request on any tag of a
// slot group — the second `<$Column key=…>`, a closing `</$X>` — is answered
// at the copied tag, with the answer's own range set back to the tag asked
// about. References and highlights list every tag of the group.
func TestSlotGroupRequests(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	doc := map[string]any{"uri": c.URI(page)}
	every := "16:10-16:17 17:10-17:17 17:46-17:53" // `$Column`: both elements, and the closing tag
	for _, tag := range []struct {
		name, needle string
		n, offset    int
		token        string
	}{
		{"the first tag: the copied one", "<$Column", 1, 3, "16:10-16:17"},
		{"the second element of the group", "<$Column", 2, 3, "17:10-17:17"},
		{"its closing tag", "</$Column", 1, 4, "17:46-17:53"},
	} {
		at := c.At(page, tag.needle, tag.n, tag.offset)
		params := map[string]any{"textDocument": doc, "position": at}

		var hover struct {
			Contents struct{ Value string }
			Range    lsptest.Range
		}
		c.Request("textDocument/hover", params, &hover)
		if !strings.Contains(hover.Contents.Value, "(property) TableProps.$Column: KeyedSlot<") || hover.Range.String() != tag.token {
			t.Errorf("%s: hover %q at %s, want the slot's declared type at %s", tag.name, hover.Contents.Value, hover.Range, tag.token)
		}
		// The `$Column` member of the container's props.
		if links := definitions(c, "textDocument/definition", params); strings.Join(links, ", ") != tag.token+" → src/table.rtsx 8:3-8:10" {
			t.Errorf("%s: definition: %q", tag.name, links)
		}
		// Type definition: the type of a prop written in JSX is its value's,
		// and a slot's value is the object the transform builds — generated
		// code, no place to go to. The same from every tag.
		if links := definitions(c, "textDocument/typeDefinition", params); len(links) != 0 {
			t.Errorf("%s: type definition: %q", tag.name, links)
		}
		var references []lsptest.Location
		params["context"] = map[string]any{"includeDeclaration": true}
		c.Request("textDocument/references", params, &references)
		var found []string
		for _, l := range references {
			found = append(found, c.Rel(l.URI)+" "+l.Range.String())
		}
		sort.Strings(found)
		if got, want := strings.Join(found, ", "), "src/page.rtsx 16:10-16:17, src/page.rtsx 17:10-17:17, src/page.rtsx 17:46-17:53, src/table.rtsx 14:45-14:52, src/table.rtsx 8:3-8:10"; got != want {
			t.Errorf("%s: references %s", tag.name, got)
		}
		var highlights []struct{ Range lsptest.Range }
		c.Request("textDocument/documentHighlight", map[string]any{"textDocument": doc, "position": at}, &highlights)
		var ranges []string
		for _, h := range highlights {
			ranges = append(ranges, h.Range.String())
		}
		sort.Strings(ranges)
		if got := strings.Join(ranges, " "); got != every {
			t.Errorf("%s: highlights %s, want %s", tag.name, got, every)
		}
		if shown, name, err := c.PrepareRename(page, at); err != nil || name != "$Column" || shown.String() != tag.token {
			t.Errorf("%s: prepareRename %s %q %v", tag.name, shown, name, err)
		}
		// (The rename from each tag: TestRenameTable.)
	}
	// A slot filled once: its closing tag.
	var hover struct {
		Contents struct{ Value string }
		Range    lsptest.Range
	}
	c.Request("textDocument/hover", map[string]any{"textDocument": doc, "position": c.At(page, "</$Title", 1, 4)}, &hover)
	if !strings.Contains(hover.Contents.Value, "TableProps.$Title?: Slot<") || hover.Range.String() != "15:38-15:44" {
		t.Errorf("</$Title>: hover %q at %s", hover.Contents.Value, hover.Range)
	}
}

// definitions are the links of a definition request, as `origin → file
// target`, sorted.
func definitions(c *lsptest.Client, method string, params map[string]any) []string {
	var links []struct {
		Origin lsptest.Range `json:"originSelectionRange"`
		URI    string        `json:"targetUri"`
		Target lsptest.Range `json:"targetSelectionRange"`
	}
	c.Request(method, params, &links)
	out := []string{}
	for _, l := range links {
		out = append(out, l.Origin.String()+" → "+c.Rel(l.URI)+" "+l.Target.String())
	}
	sort.Strings(out)
	return out
}

// A client that takes plain locations for a definition gets them: there is
// no range of the answer's own to set back.
func TestSlotGroupRequestsWithoutLinks(t *testing.T) {
	c := lsptest.StartWith(t, lsptest.Project(t, renameApp), serve, lsptest.Options{Capabilities: func(capabilities map[string]any) {
		capabilities["textDocument"].(map[string]any)["definition"] = map[string]any{}
	}})
	const page = "src/page.rtsx"
	c.Open(page)
	if got := c.Definition(page, c.At(page, "</$Column", 1, 4)); len(got) != 1 || got[0] != "src/table.rtsx 8:3" {
		t.Errorf("definition from a closing slot tag: %q", got)
	}
	if got := c.Definition(page, c.At(page, "#intro", 1, 2)); len(got) != 1 || got[0] != "src/intro.rtsx 1:1" {
		t.Errorf("definition of a segment root: %q", got)
	}
}

// ide.md, *Segments*: go to definition on `#name` opens the mounted file —
// the first of the lookup order next to the document (syntax.md, *Segment
// files*) — and after `#` the sibling modules not yet mounted in the file
// are listed. The names are the server's: a buffer never saved is a sibling.
func TestSegments(t *testing.T) {
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/intro.rtsx":    "export default function Intro() {\n  return <p>intro</p>;\n}\n",
		"src/intro.tsx.bak": "not a module",
		"src/about-us.tsx":  "export default function About() {\n  return <p>about</p>;\n}\n",
		"src/about-us.ts":   "export default function About() {\n  return null;\n}\n", // loses to the .tsx
		"src/util.ts":       "export default function Util() {\n  return null;\n}\n",
		"src/404.tsx":       "export default function Missing() {\n  return <p>404</p>;\n}\n", // not a segment name
		"src/types.d.ts":    "export {};\n",
		"src/nested/x.rtsx": "export default function X() {\n  return <p>x</p>;\n}\n",
		"src/page.rtsx": `export function Page() {
  return (
    <main>
      <section #intro />
      <section #about-us />
      <section #util />
      <section #nothing />
    </main>
  );
}
`,
	}))
	const page = "src/page.rtsx"
	c.Open(page)
	for name, want := range map[string]string{"#intro": "src/intro.rtsx 1:1", "#about-us": "src/about-us.tsx 1:1", "#util": "src/util.ts 1:1", "#nothing": ""} {
		for _, offset := range []int{0, 1, len(name)} { // on the `#`, in the name, at its end
			var links []struct {
				Origin lsptest.Range `json:"originSelectionRange"`
				URI    string        `json:"targetUri"`
				Target lsptest.Range `json:"targetSelectionRange"`
			}
			c.Request("textDocument/definition", map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}, "position": c.At(page, name+" ", 1, offset)}, &links)
			got := ""
			if len(links) == 1 {
				got = fmt.Sprintf("%s %d:%d", c.Rel(links[0].URI), links[0].Target.Start.Line+1, links[0].Target.Start.Character+1)
				if origin := c.RangeText(page, links[0].Origin); origin != name {
					t.Errorf("%s: the link is from %q", name, origin)
				}
			}
			if got != want || len(links) > 1 {
				t.Errorf("definition of %s at +%d: %q, want %q", name, offset, got, want)
			}
		}
	}
	// A segment root answers nothing else by position: no hover.
	if hover := c.Hover(page, c.At(page, "#intro", 1, 2)); hover != "" {
		t.Errorf("hover on #intro: %q", hover)
	}

	// After `#`: the siblings, each once, without the file itself, the
	// mounted ones, a declaration file, a name that is not a segment's, a
	// folder.
	complete := func(typed string) []string {
		c.Change(page, strings.Replace(c.Text(page), "      <section #nothing />\n", "      <div "+typed+" />\n", 1))
		defer c.Change(page, strings.Replace(c.Text(page), "      <div "+typed+" />\n", "      <section #nothing />\n", 1))
		var out []string
		for _, item := range c.Completion(page, c.At(page, "<div "+typed, 1, len("<div "+typed))) {
			var raw struct {
				Detail   string `json:"detail"`
				TextEdit struct {
					Range   lsptest.Range `json:"range"`
					NewText string        `json:"newText"`
				} `json:"textEdit"`
			}
			json.Unmarshal(item.Raw, &raw)
			if replaced := c.RangeText(page, raw.TextEdit.Range); replaced != typed || raw.TextEdit.NewText != item.Label {
				t.Errorf("%s replaces %q with %q", item.Label, replaced, raw.TextEdit.NewText)
			}
			out = append(out, item.Label+" "+raw.Detail)
		}
		return out
	}
	// `#util` is mounted; `#nothing` is the root being replaced by the typed one.
	if got := complete("#"); strings.Join(got, ", ") != "" {
		t.Errorf("after `#`, every sibling mounted: %q", got)
	}
	c.Change(page, strings.Replace(c.Text(page), "      <section #about-us />\n      <section #util />\n", "", 1))
	for _, typed := range []string{"#", "#ab"} {
		if got := complete(typed); strings.Join(got, ", ") != "#about-us about-us.tsx, #util util.ts" {
			t.Errorf("after `%s`: %q", typed, got)
		}
	}
	// A new file that is only a buffer yet, and one created on disk.
	c.OpenAs("src/draft.rtsx", "export default function Draft() {\n  return <p>draft</p>;\n}\n")
	c.WriteFile("src/saved.tsx", "export default function Saved() {\n  return <p>saved</p>;\n}\n")
	if got := complete("#"); strings.Join(got, ", ") != "#draft draft.rtsx, #about-us about-us.tsx, #saved saved.tsx, #util util.ts" {
		t.Errorf("with a new buffer and a new file: %q", got)
	}
	if got := c.Definition(page, c.At(page, "#intro", 1, 2)); len(got) != 1 || got[0] != "src/intro.rtsx 1:1" {
		t.Errorf("definition after the edits: %q", got)
	}
}

// ide.md, *Commands*: `reactogenic/transpiled` is the emitted TSX of a
// document as the program has it — what TypeScript checks — and the
// tolerance step that produced it.
func TestTranspiled(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const page = "src/page.rtsx"
	transpiled := func(rel string) (text, step string, err error) {
		var result struct{ Text, Step string }
		err = c.Try("reactogenic/transpiled", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &result)
		return result.Text, result.Step, err
	}
	text, step, err := transpiled(page)
	if err != nil || step != "lowered" {
		t.Fatalf("step %q, %v", step, err)
	}
	for _, line := range []string{
		`import _Section_intro from "./intro.rtsx";`,
		`<section id="intro" ><_Section_intro /></section>`,
		`<Table rows={labels} size={size} $Title={{ className: "t", children: "Users" }}`,
		`{status === "loading" ? "Loading"`,
	} {
		if !strings.Contains(text, line) {
			t.Errorf("no `%s` in:\n%s", line, text)
		}
	}
	// The unsaved buffer, not the file: an edit is in the next answer.
	c.Change(page, strings.Replace(renameApp[page], "<b>wide</b>", "<b>broad</b>", 1))
	if text, _, _ := transpiled(page); !strings.Contains(text, `{wide ? <b>broad</b> : null}`) {
		t.Errorf("after an edit:\n%s", text)
	}
	// A file being typed is lowered around what does not parse.
	c.Change(page, strings.Replace(renameApp[page], "<b>wide</b>", "<b>wide</b", 1))
	if text, step, err := transpiled(page); err != nil || step != "lowered" || !strings.Contains(text, "$Title={{") {
		t.Errorf("with a syntax error: step %q, %v\n%s", step, err, text)
	}
	c.Change(page, renameApp[page])
	// Not ours: a .ts module has no transpiled text.
	if _, _, err := transpiled("src/util.ts"); err == nil || !strings.Contains(err.Error(), "not an .rtsx document") {
		t.Errorf("for a .ts file: %v", err)
	}
	if err := c.Try("reactogenic/transpiled", map[string]any{}, nil); err == nil {
		t.Error("without a document: no error")
	}
}
