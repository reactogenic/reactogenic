package lsp_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

type documentSymbol struct {
	Name           string           `json:"name"`
	Range          lsptest.Range    `json:"range"`
	SelectionRange lsptest.Range    `json:"selectionRange"`
	Children       []documentSymbol `json:"children"`
}

// outline renders a document's symbols, as the editor asks for them (a
// tree), one per line: `name range (selection range)`, children indented.
func outline(c *lsptest.Client, rel string) string {
	var symbols []documentSymbol
	c.Request("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &symbols)
	var b strings.Builder
	var walk func(symbols []documentSymbol, indent string)
	walk = func(symbols []documentSymbol, indent string) {
		for _, s := range symbols {
			fmt.Fprintf(&b, "%s%s %s (%s)\n", indent, s.Name, s.Range, s.SelectionRange)
			walk(s.Children, indent+"  ")
		}
	}
	walk(symbols, "")
	return b.String()
}

// ide.md, the feature table: document symbols. A declaration's range is its
// own — whatever rtsx constructs it holds — so children lie inside their
// parents (breadcrumbs, sticky scroll, the outline following the cursor);
// the keys of generated objects (slot attributes, args) are not symbols.
func TestDocumentSymbols(t *testing.T) {
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/card.rtsx": `import type { Slot } from "@reactogenic/core";
export function Card({ $Title, $Row }: { $Title?: Slot<{ className?: string }>; $Row?: Slot<{ className?: string }, { rowSize: number }> }) {
  return <div><h1 slot={$Title} /><p slot={$Row} &rowSize={1} /></div>;
}
`,
		"src/page.rtsx": `import { Card } from "./card";
export function Plain() {
  const a = 1;
  return a;
}
export function WithShorthand() {
  const size = 1;
  return <input size />;
}
export function WithSlots() {
  const label = "x";
  function inner() {
    const deep = () => <Card><$Title className="t" /></Card>;
    return deep;
  }
  return (
    <Card>
      <$Title className="title" />
      <$Row className="row" { rowSize }>{rowSize + label + inner.name}</$Row>
    </Card>
  );
}
`,
	}))
	c.Open("src/page.rtsx")
	if got := lsptest.Lines(c.Diagnostics("src/page.rtsx")); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	const want = `Card 1:10-1:14 (1:10-1:14)
Plain 2:1-5:2 (2:17-2:22)
  a 3:9-3:14 (3:9-3:10)
WithShorthand 6:1-9:2 (6:17-6:30)
  size 7:9-7:17 (7:9-7:13)
WithSlots 10:1-22:2 (10:17-10:26)
  label 11:9-11:20 (11:9-11:14)
  inner 12:3-15:4 (12:12-12:17)
    deep 13:11-13:61 (13:11-13:15)
`
	if got := outline(c, "src/page.rtsx"); got != want {
		t.Errorf("page.rtsx:\n%s--- want\n%s", got, want)
	}
	c.Open("src/card.rtsx")
	const wantCard = `Slot 1:15-1:19 (1:15-1:19)
Card 2:1-4:2 (2:17-2:21)
`
	if got := outline(c, "src/card.rtsx"); got != wantCard {
		t.Errorf("card.rtsx:\n%s--- want\n%s", got, wantCard)
	}
	// While a declaration is being typed, the outline stays.
	c.Change("src/page.rtsx", strings.Replace(c.Text("src/page.rtsx"), `<$Title className="title" />`, `<$Title className=`, 1))
	if got := outline(c, "src/page.rtsx"); !strings.HasPrefix(got, want[:strings.Index(want, "WithSlots")]+"WithSlots 10:1-") {
		t.Errorf("page.rtsx, half-typed:\n%s", got)
	}

	// A client that takes no tree gets the same declarations, flat.
	flat := lsptest.StartWith(t, c.Root, serve, lsptest.Options{Capabilities: func(capabilities map[string]any) {
		delete(capabilities["textDocument"].(map[string]any), "documentSymbol")
	}})
	flat.Open("src/card.rtsx")
	var infos []struct {
		Name     string           `json:"name"`
		Location lsptest.Location `json:"location"`
	}
	flat.Request("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": flat.URI("src/card.rtsx")}}, &infos)
	got := ""
	for _, info := range infos {
		got += fmt.Sprintf("%s %s %s; ", info.Name, flat.Rel(info.Location.URI), info.Location.Range)
	}
	if got != "Slot src/card.rtsx 1:15-1:19; Card src/card.rtsx 2:1-4:2; " {
		t.Errorf("flat: %s", got)
	}
}
