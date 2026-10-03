package lsp_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// A page with every probed position behind the text `§` stands for, on its
// own line: a slot tag, a slot's params and body, a shorthand, a call.
const textPage = `import { Button, type Size } from "./button";
import { twice } from "./util";

export function Page() {
  const size: Size = "lg";
  const s = "§"; const count = twice(2);
  return (
    <main title={s + count}>
      <b title="§" /><Button size><$Icon className="i" { iconSize }>{iconSize.toUpperCase()}</$Icon><$Label>Save</$Label></Button>
    </main>
  );
}
`

// A UTF-8 byte order mark, as some editors write it at the start of a file.
const byteOrderMark = "\xEF\xBB\xBF"

// positionIn is the LSP position of a byte offset in text, in a session's
// position encoding.
func positionIn(text string, offset int, encoding string) lsptest.Position {
	if encoding != "utf-8" {
		return lsptest.PositionAt(text, offset)
	}
	return lsptest.Position{Line: strings.Count(text[:offset], "\n"), Character: offset - (strings.LastIndex(text[:offset], "\n") + 1)}
}

// Positions are the client's: lines as it ends them, characters in the
// encoding it chose. Whatever the text is made of, what TypeScript answers
// (through the span map) and what the front answers (from the source tree)
// land on the same characters.
func TestTextShapes(t *testing.T) {
	const rel = "src/page.rtsx"
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	with := func(s string) string { return strings.ReplaceAll(textPage, "§", s) }
	for _, tc := range []struct {
		name     string
		text     string // the document
		crlf     bool   // the container too
		bom      bool   // on disk; an editor's buffer has none
		encoding string // "": utf-16, as VS Code
	}{
		{name: "CRLF", text: crlf(with("ab")), crlf: true},
		{name: "a BOM on disk", text: with("ab"), bom: true},
		{name: "non-ASCII before a position", text: with("é日本")},
		{name: "astral characters before a position", text: with("\U0001F600\U0001D4B3")},
		{name: "a utf-8 client", text: with("é\U0001F600"), encoding: "utf-8"},
		{name: "CRLF, astral characters, a utf-8 client", text: crlf(with("\U0001F600")), crlf: true, encoding: "utf-8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := lsptest.With(app, map[string]string{rel: tc.text, "src/main.tsx": ""})
			if tc.crlf {
				files["src/button.rtsx"] = crlf(files["src/button.rtsx"])
			}
			if tc.bom {
				files[rel], files["src/button.rtsx"] = byteOrderMark+files[rel], byteOrderMark+files["src/button.rtsx"]
			}
			options := lsptest.Options{}
			if tc.encoding != "" {
				options.Capabilities = func(capabilities map[string]any) {
					capabilities["general"] = map[string]any{"positionEncodings": []string{tc.encoding}}
				}
			}
			c := lsptest.StartWith(t, lsptest.Project(t, files), serve, options)
			if got := string(c.Initialized.Capabilities["positionEncoding"]); tc.encoding != "" && got != `"`+tc.encoding+`"` {
				t.Fatalf("position encoding %s, want %q", got, tc.encoding)
			}
			text := tc.text
			c.OpenAs(rel, text)
			doc := map[string]any{"uri": c.URI(rel)}
			at := func(needle string, offset int) lsptest.Position {
				t.Helper()
				i := strings.Index(text, needle)
				if i < 0 {
					t.Fatalf("no %q in the text", needle)
				}
				return positionIn(text, i+offset, tc.encoding)
			}
			span := func(needle string) lsptest.Range {
				t.Helper()
				return lsptest.Range{Start: at(needle, 0), End: at(needle, len(needle))}
			}
			if got := lsptest.Lines(c.Diagnostics(rel)); len(got) != 0 {
				t.Errorf("the fixture has errors: %q", got)
			}

			// TypeScript's answers, mapped back.
			if hover := c.Hover(rel, at("twice(2", 1)); !strings.Contains(hover, "twice") {
				t.Errorf("hover on a call: %q", hover)
			}
			if hover := c.Hover(rel, at("iconSize.to", 2)); !strings.Contains(hover, "iconSize: Size") {
				t.Errorf("hover on a param in a slot body: %q", hover)
			}
			if hover := c.Hover(rel, at("<$Label", 3)); !strings.Contains(hover, "The label.") {
				t.Errorf("hover on a slot tag: %q", hover)
			}
			if def := c.Definition(rel, at("<$Icon", 3)); len(def) != 1 || def[0] != "src/button.rtsx 7:3" {
				t.Errorf("definition of a slot tag: %q", def)
			}

			// The front's answers, from the source tree.
			var selection []struct {
				Range lsptest.Range `json:"range"`
			}
			c.Request("textDocument/selectionRange", map[string]any{"textDocument": doc, "positions": []any{at("iconSize.to", 2)}}, &selection)
			if want := (lsptest.Range{Start: at("iconSize.to", 0), End: at("iconSize.to", len("iconSize"))}); len(selection) != 1 || selection[0].Range != want {
				t.Errorf("selection range: %+v, want %s", selection, want)
			}
			var folds []struct {
				StartLine int `json:"startLine"`
				EndLine   int `json:"endLine"`
			}
			c.Request("textDocument/foldingRange", map[string]any{"textDocument": doc}, &folds)
			folded := false
			for _, fold := range folds {
				folded = folded || fold.StartLine+1 == 8 && fold.EndLine+1 == 9
			}
			if !folded { // whole lines, as the editor folds: the closing tag's line stays
				t.Errorf("<main> (lines 8-9) does not fold: %+v", folds)
			}

			// An error in copied text, and one in a slot body — moved text:
			// the whole range, start and end.
			for _, edit := range []struct{ old, new, at, code string }{
				{"twice(2)", `twice("a")`, `"a"`, "TS2345"},
				{"iconSize.toUpperCase()", "iconSize.nope()", "nope", "TS2339"},
			} {
				text = strings.Replace(tc.text, edit.old, edit.new, 1)
				c.Change(rel, text)
				want := span(edit.at)
				got := c.Diagnostics(rel)
				if len(got) != 1 || got[0].Range != want || got[0].String() != fmt.Sprintf("%d:%d %s", want.Start.Line+1, want.Start.Character+1, edit.code) {
					t.Errorf("%s: %q (%+v), want %s at %s", edit.new, lsptest.Lines(got), got, edit.code, want)
				}
			}

			// A closing tag, typed mid-line.
			text = strings.Replace(tc.text, " /><Button size>", " /><div><Button size>", 1)
			c.Change(rel, text)
			var closing *struct {
				Edit struct {
					NewText string `json:"newText"`
				} `json:"_vs_textEdit"`
			}
			c.Request("textDocument/_vs_onAutoInsert", map[string]any{"_vs_textDocument": doc, "_vs_position": at("<div>", len("<div>")), "_vs_ch": ">"}, &closing)
			if closing == nil || closing.Edit.NewText != "$0</div>" {
				t.Errorf("closing tag: %+v", closing)
			}
		})
	}
}
