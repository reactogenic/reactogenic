package lsp_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

type semanticToken struct {
	Range lsptest.Range
	Type  string // by the server's legend
	Text  string // what it covers (one line of ASCII)
}

// semanticTokens decodes a document's semantic tokens, in the order sent.
// It fails the test on a token that runs backwards or into the one before
// it: the same text coloured twice.
func semanticTokens(t *testing.T, c *lsptest.Client, rel string) []semanticToken {
	t.Helper()
	var provider struct {
		Legend struct {
			TokenTypes []string `json:"tokenTypes"`
		} `json:"legend"`
	}
	if err := json.Unmarshal(c.Initialized.Capabilities["semanticTokensProvider"], &provider); err != nil {
		t.Fatal(err)
	}
	var tokens struct {
		Data []int `json:"data"`
	}
	c.Request("textDocument/semanticTokens/full", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &tokens)
	if len(tokens.Data) == 0 || len(tokens.Data)%5 != 0 {
		t.Fatalf("%s: %d numbers", rel, len(tokens.Data))
	}
	lines := strings.Split(c.Text(rel), "\n")
	var out []semanticToken
	line, character, end := 0, 0, 0
	for i := 0; i < len(tokens.Data); i += 5 {
		deltaLine, deltaStart, length, kind := tokens.Data[i], tokens.Data[i+1], tokens.Data[i+2], tokens.Data[i+3]
		if deltaLine < 0 || deltaStart < 0 || (i > 0 && deltaLine == 0 && character+deltaStart < end) {
			t.Errorf("%s: token %d (+%d lines, +%d) overlaps the one before it, at %d:%d", rel, i/5, deltaLine, deltaStart, line+1, character+1)
		}
		if deltaLine > 0 {
			line, character = line+deltaLine, deltaStart
		} else {
			character += deltaStart
		}
		end = character + length
		token := semanticToken{Range: lsptest.Range{Start: lsptest.Position{Line: line, Character: character}, End: lsptest.Position{Line: line, Character: end}}}
		if kind >= 0 && kind < len(provider.Legend.TokenTypes) {
			token.Type = provider.Legend.TokenTypes[kind]
		}
		if line < len(lines) && end <= len(lines[line]) {
			token.Text = lines[line][character:end]
		}
		if token.Type == "" || token.Text == "" {
			t.Errorf("%s: token %d at %s: type %d, text %q", rel, i/5, token.Range, kind, token.Text)
		}
		out = append(out, token)
	}
	return out
}

// ide.md, *Span map*: the copies of names — slot tags, the attribute names
// of a slot element, arg names — carry no semantic tokens, so tag and
// attribute names keep the grammar's colours with or without a server. The
// binding of a shorthand is the user's variable: coloured as one, once.
func TestSemanticTokens(t *testing.T) {
	c := start(t, app)
	const page, container = "src/page.rtsx", "src/button.rtsx"
	type place struct {
		needle string
		n      int // the n-th occurrence
		offset int
	}
	for rel, want := range map[string]struct {
		none   []place          // no token here
		tokens map[place]string // a token here: `type text`
	}{
		page: {
			none: []place{
				{"<$Icon", 1, 1}, {"</$Icon", 1, 2}, {"<$Label", 1, 1}, {"</$Label", 1, 2}, // slot tags
				{"<$Case", 1, 1}, {"</$Case", 1, 2}, {"<$Case", 2, 1}, {"</$Case", 2, 2},
				{`className="icon"`, 1, 0}, {`is="loading"`, 1, 0}, // attribute names of slot elements
			},
			tokens: map[place]string{
				{"<Button size", 1, 8}:  "variable size",      // the binding of the shorthand
				{"{ iconSize }", 1, 2}:  "parameter iconSize", // a slot's param
				{"iconSize.to", 1, 0}:   "parameter iconSize", // and its use in the body
				{"on={status}", 1, 4}:   "parameter status",   // a Switch subject: copied per case, coloured once
				{"twice(2)", 1, 0}:      "function twice",
				{"options.unit}", 1, 8}: "property unit",
			},
		},
		container: {
			none: []place{{"&iconSize", 1, 1}}, // an arg name
			tokens: map[place]string{
				{"&iconSize={size}", 1, 11}: "parameter size",  // an arg's value
				{"slot={$Icon}", 1, 6}:      "parameter $Icon", // emitted in both branches of the attachment
			},
		},
	} {
		c.Open(rel)
		tokens := semanticTokens(t, c, rel)
		at := func(p place) string {
			position := c.At(rel, p.needle, p.n, p.offset)
			for _, token := range tokens {
				if token.Range.Start.Line == position.Line && token.Range.Start.Character <= position.Character && position.Character < token.Range.End.Character {
					return token.Type + " " + token.Text
				}
			}
			return ""
		}
		for _, p := range want.none {
			if got := at(p); got != "" {
				t.Errorf("%s: a semantic token on a copied name, at %q (%d): %s", rel, p.needle, p.n, got)
			}
		}
		for p, token := range want.tokens {
			if got := at(p); got != token {
				t.Errorf("%s: at %q the token is %q, want %q", rel, p.needle, got, token)
			}
		}
	}
}

// ide.md, *Span map*: names are copied, not synthesized, so TypeScript's own
// features reach them — and where one source token has several copies, the
// answer is one answer.
func TestCopiedNames(t *testing.T) {
	c := start(t, app)
	const page, container = "src/page.rtsx", "src/button.rtsx"
	c.Open(page)
	c.Open(container)
	definition := func(rel string, at lsptest.Position) string {
		got := c.Definition(rel, at)
		sort.Strings(got)
		return strings.Join(got, ", ")
	}
	// A shorthand is two symbols — the prop and the binding — as `{ size }`
	// is to TypeScript: both answer.
	t.Run("a shorthand", func(t *testing.T) {
		at := c.At(page, "<Button size", 1, 9)
		if hover := c.Hover(page, at); !strings.Contains(hover, "(property) ButtonProps.size: Size") || !strings.Contains(hover, "const size:") {
			t.Errorf("hover shows not both the prop and the binding: %q", hover)
		}
		if got := definition(page, at); got != "src/button.rtsx 4:3, src/page.rtsx 6:9" {
			t.Errorf("definition: %s", got)
		}
	})
	// The key of the generated props object.
	t.Run("an attribute name of a slot element", func(t *testing.T) {
		at := c.At(page, `className="icon"`, 1, 2)
		if hover := c.Hover(page, at); !strings.Contains(hover, "className") {
			t.Errorf("hover: %q", hover)
		}
		if got := definition(page, at); got != "src/button.rtsx 7:18" {
			t.Errorf("definition: %s", got)
		}
	})
	// The key of the generated args object — emitted in both branches of the
	// attachment, answered once.
	t.Run("an arg name", func(t *testing.T) {
		at := c.At(container, "&iconSize", 1, 2)
		if hover := c.Hover(container, at); !strings.Contains(hover, "iconSize") {
			t.Errorf("hover: %q", hover)
		}
		if got := definition(container, at); got != "src/button.rtsx 7:42" {
			t.Errorf("definition: %s", got)
		}
	})
	// A Switch subject is repeated per case: the first copy answers.
	t.Run("a Switch subject", func(t *testing.T) {
		at := c.At(page, "on={status}", 1, 5)
		if got := definition(page, at); got != "src/page.rtsx 5:24" {
			t.Errorf("definition: %s", got)
		}
		var locations []lsptest.Location
		c.Request("textDocument/references", map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}, "position": at, "context": map[string]any{"includeDeclaration": true}}, &locations)
		seen := map[string]int{}
		for _, l := range locations {
			seen[fmt.Sprintf("%s %s", c.Rel(l.URI), l.Range)]++
		}
		// The binding and its use, each once — however many cases repeat it.
		for _, want := range []string{"src/page.rtsx 5:24-5:30", "src/page.rtsx 20:19-20:25"} {
			if seen[want] != 1 {
				t.Errorf("references: %s is listed %d times", want, seen[want])
			}
		}
		for location, n := range seen {
			if n != 1 || !strings.HasPrefix(location, "src/page.rtsx ") {
				t.Errorf("references: %s, %d times", location, n)
			}
		}
	})
}

// ide.md, *Span map*, the names table: a closing tag name answers where a
// closing tag is emitted. A component whose children are all slots is
// emitted self-closing — there is no closing name to copy — and its
// `</Card>` is answered at its opening tag, the answer's range set back to
// the closing name (RGP1-108: the front pairs the tags on the source tree).
func TestClosingTagOfComponent(t *testing.T) {
	const rel = "src/x.rtsx"
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		"src/card.tsx": "import type { Slot } from \"@reactogenic/core\";\nexport function Card(props: { $Title?: Slot<{ children?: string }>; children?: string }) {\n  return <div />;\n}\n",
		rel: `import { Card } from "./card";
export const a = (
  <Card>
    <$Title>T</$Title>
    body
  </Card>
);
export const b = (
  <Card>
    <$Title>T</$Title>
  </Card>
);
export const c = (
  <Card>
    body
  </Card>
);
`,
	}))
	c.Open(rel)
	if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	for n, name := range []string{
		"slots and children: the closing tag is rebuilt",
		"slots only: emitted self-closing",
		"no slots: copied as written",
	} {
		opening, closing := c.At(rel, "<Card", n+1, 2), c.At(rel, "</Card", n+1, 3)
		if hover, def := c.Hover(rel, opening), c.Definition(rel, opening); !strings.Contains(hover, "function Card") || len(def) != 1 || def[0] != "src/card.tsx 2:17" {
			t.Errorf("%s: the opening tag: hover %q, definition %q", name, hover, def)
		}
		var hover struct {
			Contents struct{ Value string }
			Range    lsptest.Range
		}
		c.Request("textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}, "position": closing}, &hover)
		if def := c.Definition(rel, closing); !strings.Contains(hover.Contents.Value, "function Card") || c.RangeText(rel, hover.Range) != "Card" || hover.Range.Start.Line != closing.Line || len(def) != 1 || def[0] != "src/card.tsx 2:17" {
			t.Errorf("%s: the closing tag: hover %q at %s, definition %q", name, hover.Contents.Value, hover.Range, def)
		}
		// Both names of the element, from either: highlights.
		for _, at := range []lsptest.Position{opening, closing} {
			var highlights []struct{ Range lsptest.Range }
			c.Request("textDocument/documentHighlight", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}, "position": at}, &highlights)
			lines := map[int]bool{}
			for _, h := range highlights {
				lines[h.Range.Start.Line] = true
			}
			if !lines[opening.Line] || !lines[closing.Line] {
				t.Errorf("%s: highlights from line %d: %+v", name, at.Line+1, highlights)
			}
		}
	}
}

// ide.md, *Not in the first release*: renaming a segment. The mounter's
// import of a segment file is generated — `#intro` has no specifier to
// rewrite — so renaming the file edits nothing (and leaves
// `segment-not-found` on the mounter).
func TestFileRenameOfSegmentFile(t *testing.T) {
	c := start(t, lsptest.With(app, map[string]string{
		"src/outro.tsx": "export default function Outro() {\n  return <p>outro</p>;\n}\n",
		"src/end.rtsx":  "export const end = <footer #outro />;\n",
	}))
	c.Open("src/page.rtsx")
	c.Open("src/end.rtsx")
	if got := lsptest.Lines(c.Diagnostics("src/page.rtsx")); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	for _, pair := range [][2]string{{"src/intro.rtsx", "src/about.rtsx"}, {"src/outro.tsx", "src/finale.tsx"}} {
		if got := renameFiles(c, pair[0], pair[1]); got != "" {
			t.Errorf("renaming the segment file %s edits %s", pair[0], got)
		}
	}
}

// A file that mounts a segment, moved: the imports it wrote itself follow
// it, as in any other file. Its generated import cannot — a segment is a
// sibling — and does not keep the others from being rewritten.
func TestFileRenameOfMounter(t *testing.T) {
	c := start(t, map[string]string{
		"src/util.ts":      "export const twice = (n: number) => n * 2;\n",
		"src/card.rtsx":    "export function Card() {\n  return <div />;\n}\n",
		"src/mount.rtsx":   "export default function Mount() {\n  return <p>m</p>;\n}\n",
		"src/mounter.rtsx": "import { Card } from \"./card\";\nimport { twice } from \"./util\";\nexport function List() {\n  return <ul title={String(twice(1))}><Card /><section #mount /></ul>;\n}\n",
		"src/main.tsx":     "import { List } from \"./mounter\";\nexport const m = List;\n",
	})
	c.Open("src/mounter.rtsx")
	if got := lsptest.Lines(c.Diagnostics("src/mounter.rtsx")); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}
	want := `src/main.tsx 1:23-1:32 "./deep/mounter", src/mounter.rtsx 1:23-1:29 "../card", src/mounter.rtsx 2:24-2:30 "../util"`
	if got := renameFiles(c, "src/mounter.rtsx", "src/deep/mounter.rtsx"); got != want {
		t.Errorf("mounter.rtsx → deep/mounter.rtsx edits\n got %s\nwant %s", got, want)
	}
	// Renamed in place, nothing of its own changes.
	if got, want := renameFiles(c, "src/mounter.rtsx", "src/list.rtsx"), `src/main.tsx 1:23-1:32 "./list"`; got != want {
		t.Errorf("mounter.rtsx → list.rtsx edits %s, want %s", got, want)
	}
}
