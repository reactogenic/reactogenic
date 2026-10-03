package lsp_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// The fixture: a container with slots, a page that fills them, a segment, a
// .ts helper and a .tsx entry.
var app = lsptest.With(lsptest.Core, map[string]string{
	"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export type Size = "md" | "lg";
export interface ButtonProps {
  size: Size;
  /** The label. */
  $Label?: Slot<{ className?: string; children?: string }>;
  $Icon?: Slot<{ className?: string }, { iconSize: Size }>;
}
export function Button({ size, $Label, $Icon }: ButtonProps) {
  return (
    <button>
      <span slot={$Icon} &iconSize={size} />
      <b slot={$Label} className="label">Button</b>
    </button>
  );
}
`,
	"src/util.ts": `export const twice = (n: number, unit?: string) => n * 2;
export const greeting = "hello";
export interface Options { unit: string }
`,
	"src/intro.rtsx": `export default function Intro() {
  return <p>intro</p>;
}
`,
	"src/page.rtsx": `import { Switch } from "@reactogenic/core";
import { Button, type Size } from "./button";
import { twice, type Options } from "./util";

export function Page({ status }: { status: "loading" | "ready" }) {
  const size: Size = "lg";
  const count = twice(2);
  const options: Options = { unit: "px" };
  return (
    <main title={count + options.unit}>
      <section #intro />
      <Button size>
        <$Icon className="icon" { iconSize }>
          {iconSize.toUpperCase()}
        </$Icon>
        <$Label>
          Save
        </$Label>
      </Button>
      <Switch on={status} exhaustive>
        <$Case is="loading">
          Loading
        </$Case>
        <$Case is="ready">
          Ready
        </$Case>
      </Switch>
    </main>
  );
}
`,
	"src/main.tsx": `import { Page } from "./page";
import { twice } from "./util";
export const app = <Page status="ready" />;
export const four = twice(2);
`,
})

func start(t *testing.T, files map[string]string) *lsptest.Client {
	t.Helper()
	return lsptest.Start(t, lsptest.Project(t, files), serve)
}

// ide.md, the feature table of *reactogenic lsp* (RGP1-105): one scenario per
// row, diagnostics and rename aside.
func TestFeatures(t *testing.T) {
	c := start(t, app)
	const page = "src/page.rtsx"
	c.Open(page)
	doc := map[string]any{"uri": c.URI(page)}
	at := func(needle string, n, offset int) map[string]any {
		return map[string]any{"textDocument": doc, "position": c.At(page, needle, n, offset)}
	}

	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}

	t.Run("hover", func(t *testing.T) {
		if hover := c.Hover(page, c.At(page, "iconSize.to", 1, 2)); !strings.Contains(hover, `iconSize: Size`) {
			t.Errorf("a param in a slot body: %q", hover)
		}
		// The slot tag is the prop: its declared type, and its doc comment.
		if hover := c.Hover(page, c.At(page, "<$Label", 1, 3)); !strings.Contains(hover, "$Label") || !strings.Contains(hover, "The label.") {
			t.Errorf("a slot tag: %q", hover)
		}
	})
	t.Run("signature help", func(t *testing.T) {
		var help struct {
			Signatures []struct {
				Label string `json:"label"`
			} `json:"signatures"`
		}
		c.Request("textDocument/signatureHelp", at("twice(2", 1, 6), &help)
		if len(help.Signatures) != 1 || !strings.Contains(help.Signatures[0].Label, "n: number") {
			t.Errorf("%+v", help)
		}
	})
	t.Run("definition", func(t *testing.T) {
		if def := c.Definition(page, c.At(page, "<$Icon", 1, 3)); len(def) != 1 || def[0] != "src/button.rtsx 7:3" {
			t.Errorf("a slot tag goes to the slot's declaration: %q", def)
		}
		if def := c.Definition(page, c.At(page, "twice(2", 1, 1)); len(def) != 1 || def[0] != "src/util.ts 1:14" {
			t.Errorf("into a .ts file: %q", def)
		}
	})
	t.Run("implementation", func(t *testing.T) {
		var locations []lsptest.Location
		c.Request("textDocument/implementation", at("<Button", 1, 2), &locations)
		if len(locations) != 1 || c.Rel(locations[0].URI) != "src/button.rtsx" || locations[0].Range.String() != "9:17-9:23" {
			t.Errorf("%+v", locations)
		}
	})
	t.Run("type definition", func(t *testing.T) {
		var locations []lsptest.Location
		c.Request("textDocument/typeDefinition", at("options.unit", 1, 1), &locations)
		if len(locations) != 1 || c.Rel(locations[0].URI) != "src/util.ts" || locations[0].Range.Start.Line != 2 {
			t.Errorf("%+v", locations)
		}
	})
	t.Run("references", func(t *testing.T) {
		var locations []lsptest.Location
		params := at("twice(2", 1, 1)
		params["context"] = map[string]any{"includeDeclaration": true}
		c.Request("textDocument/references", params, &locations)
		var files []string
		for _, l := range locations {
			files = append(files, fmt.Sprintf("%s:%d", c.Rel(l.URI), l.Range.Start.Line+1))
		}
		sort.Strings(files)
		if got := strings.Join(files, " "); got != "src/main.tsx:2 src/main.tsx:4 src/page.rtsx:3 src/page.rtsx:7 src/util.ts:1" {
			t.Log("(page.rtsx: the import on line 3, the call on line 7)")
			t.Errorf("%s", got)
		}
	})
	t.Run("highlights", func(t *testing.T) {
		var highlights []struct{ Range lsptest.Range }
		c.Request("textDocument/documentHighlight", at("count = ", 1, 1), &highlights)
		if len(highlights) != 2 {
			t.Errorf("%+v", highlights)
		}
	})
	t.Run("completion", func(t *testing.T) {
		labels := func(params map[string]any) []string {
			var list struct {
				Items []struct {
					Label string `json:"label"`
				} `json:"items"`
			}
			c.Request("textDocument/completion", params, &list)
			var out []string
			for _, item := range list.Items {
				out = append(out, item.Label)
			}
			return out
		}
		has := func(labels []string, want string) bool {
			for _, l := range labels {
				if l == want {
					return true
				}
			}
			return false
		}
		// The props of a slot, inside its tag.
		if got := labels(at(`className="icon"`, 1, 5)); !has(got, "className?") {
			t.Errorf("slot props: %q", got)
		}
		// Members, right after the dot, in a file that no longer parses.
		broken := strings.Replace(c.Text(page), "iconSize.toUpperCase()", "iconSize.", 1)
		c.Change(page, broken)
		if got := labels(at("iconSize.", 1, len("iconSize."))); !has(got, "toUpperCase") || !has(got, "length") {
			t.Errorf("members in a half-typed file: %d items", len(got))
		}
		// A slot name being typed completes to the container's slots.
		c.Change(page, strings.Replace(app[page], "<$Label>\n          Save\n        </$Label>", "<$La", 1))
		if got := labels(at("<$La", 1, 4)); !has(got, "$Label?") && !has(got, "$Label") {
			t.Errorf("slot names: %q", got)
		}
		c.Change(page, app[page])
	})
	t.Run("auto-import", func(t *testing.T) {
		c.Change(page, strings.Replace(app[page], "const count = twice(2);", "const count = twice(2); greetin", 1))
		items := c.Completion(page, c.At(page, "greetin", 1, 7))
		var item *lsptest.CompletionItem
		for i := range items {
			if items[i].Label == "greeting" && item == nil {
				item = &items[i]
			}
		}
		if item == nil {
			t.Fatalf("no auto-import item for an export of util.ts among %d items", len(items))
		}
		// The import is written when the item is accepted: resolved, as the
		// editor does, and applied with the word completed.
		c.ApplyTo(page, c.Resolve(*item).AdditionalTextEdits)
		c.Change(page, strings.Replace(c.Text(page), "; greetin", "; greeting;", 1))
		if text := c.Text(page); !strings.Contains(text, `import { greeting, twice, type Options } from "./util";`) {
			t.Errorf("the import written:\n%s", text[:strings.Index(text, "export")])
		}
		if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
			t.Errorf("after the completion: %q", got)
		}
		c.Change(page, app[page])
	})
	t.Run("document symbols", func(t *testing.T) {
		// A tree, as the editor asks for it: the outline.
		var symbols []struct {
			Name           string        `json:"name"`
			SelectionRange lsptest.Range `json:"selectionRange"`
			Children       []struct {
				Name string `json:"name"`
			} `json:"children"`
		}
		c.Request("textDocument/documentSymbol", map[string]any{"textDocument": doc}, &symbols)
		found := false
		for _, s := range symbols {
			if s.Name == "Page" {
				children := ""
				for _, child := range s.Children {
					children += child.Name + " "
				}
				found = s.SelectionRange.String() == "5:17-5:21" && strings.HasPrefix(children, "size count options ")
			}
		}
		if !found {
			t.Errorf("%+v", symbols)
		}
	})
	t.Run("semantic tokens", func(t *testing.T) {
		var tokens struct {
			Data []int `json:"data"`
		}
		c.Request("textDocument/semanticTokens/full", map[string]any{"textDocument": doc}, &tokens)
		if len(tokens.Data) == 0 || len(tokens.Data)%5 != 0 {
			t.Errorf("%d numbers", len(tokens.Data))
		}
	})
	t.Run("inlay hints", func(t *testing.T) {
		var hints []json.RawMessage
		whole := map[string]any{"textDocument": doc, "range": map[string]any{"start": lsptest.Position{}, "end": lsptest.Position{Line: 40}}}
		c.Request("textDocument/inlayHint", whole, &hints)
	})
	t.Run("folding", func(t *testing.T) {
		var ranges []struct {
			StartLine int `json:"startLine"`
			EndLine   int `json:"endLine"`
		}
		c.Request("textDocument/foldingRange", map[string]any{"textDocument": doc}, &ranges)
		folds := map[string]bool{}
		for _, r := range ranges {
			folds[fmt.Sprintf("%d-%d", r.StartLine+1, r.EndLine+1)] = true
		}
		// A component with slots, a slot element and a Switch: none of them
		// has an element in the virtual text.
		// (Whole lines, as the editor folds: the closing tag's line stays.)
		for name, lines := range map[string]string{"<Button> with slots": "12-18", "<$Icon>": "13-14", "<Switch>": "20-26", "<$Case>": "21-22"} {
			if !folds[lines] {
				t.Errorf("%s (lines %s) does not fold: %v", name, lines, folds)
			}
		}
	})
	t.Run("selection ranges", func(t *testing.T) {
		var ranges []struct {
			Range  lsptest.Range   `json:"range"`
			Parent json.RawMessage `json:"parent"`
		}
		c.Request("textDocument/selectionRange", map[string]any{"textDocument": doc, "positions": []any{c.At(page, "iconSize.to", 1, 2)}}, &ranges)
		depth := 0
		for raw := ranges[0].Parent; len(raw) > 0 && string(raw) != "null"; depth++ {
			var parent struct {
				Parent json.RawMessage `json:"parent"`
			}
			json.Unmarshal(raw, &parent)
			raw = parent.Parent
		}
		// Out of the slot body through `$Icon`, `Button`, `main`, the function.
		if len(ranges) != 1 || depth < 8 {
			t.Errorf("selection expands %d times", depth)
		}
	})
	t.Run("closing tags", func(t *testing.T) {
		for typed, want := range map[string]string{"<div>": `$0</div>`, "<$Label { x }>": `$0</\$Label>`, "<Button size>": `$0</Button>`} {
			text := strings.Replace(app[page], "<section #intro />", "<section #intro />\n      "+typed, 1)
			c.Change(page, text)
			var result *struct {
				Edit struct {
					NewText string `json:"newText"`
				} `json:"_vs_textEdit"`
			}
			c.Request("textDocument/_vs_onAutoInsert", map[string]any{"_vs_textDocument": doc, "_vs_position": c.At(page, typed, 1, len(typed)), "_vs_ch": ">"}, &result)
			if result == nil || result.Edit.NewText != want {
				t.Errorf("after %s: %+v", typed, result)
			}
		}
		c.Change(page, app[page])
	})
	t.Run("linked editing", func(t *testing.T) {
		linked := func(needle string, offset int) string {
			var result *struct {
				Ranges []lsptest.Range `json:"ranges"`
			}
			c.Request("textDocument/linkedEditingRange", at(needle, 1, offset), &result)
			if result == nil {
				return "none"
			}
			return fmt.Sprint(result.Ranges)
		}
		// A plain tag pair. A tag with slots and a slot tag are rebuilt: no
		// pair, rather than a wrong one (ide.md, *Not in the first release*).
		for needle, want := range map[string]string{"<main": "[10:6-10:10 28:7-28:11]", "<Button": "none", "<$Icon": "none"} {
			if got := linked(needle, 2); got != want {
				t.Errorf("%s: %s, want %s", needle, got, want)
			}
		}
	})
	t.Run("call hierarchy", func(t *testing.T) {
		type item struct {
			Name string `json:"name"`
			URI  string `json:"uri"`
		}
		prepare := func(needle string) json.RawMessage {
			var items []json.RawMessage
			c.Request("textDocument/prepareCallHierarchy", at(needle, 1, 1), &items)
			if len(items) != 1 {
				t.Fatalf("%s: %d items", needle, len(items))
			}
			return items[0]
		}
		// Callers: the .tsx module, and Page — at the call's source position.
		var incoming []struct {
			From       item            `json:"from"`
			FromRanges []lsptest.Range `json:"fromRanges"`
		}
		c.Request("callHierarchy/incomingCalls", map[string]any{"item": prepare("twice(2")}, &incoming)
		callers := []string{}
		for _, call := range incoming {
			callers = append(callers, fmt.Sprintf("%s %s %v", c.Rel(call.From.URI), strings.TrimPrefix(call.From.Name, c.Root), call.FromRanges))
		}
		sort.Strings(callers)
		if got := strings.Join(callers, ", "); got != "src/main.tsx /src/main.tsx [4:21-4:26], src/page.rtsx Page [7:17-7:22]" {
			t.Errorf("callers of twice: %s", got)
		}
		// Callees: a call in a slot body is the enclosing function's; no
		// generated call (the slot's render, the Switch) is listed.
		var outgoing []struct {
			To item `json:"to"`
		}
		c.Request("callHierarchy/outgoingCalls", map[string]any{"item": prepare("Page(")}, &outgoing)
		callees := []string{}
		for _, call := range outgoing {
			callees = append(callees, call.To.Name)
		}
		sort.Strings(callees)
		if got := strings.Join(callees, " "); got != "Button toUpperCase twice" {
			t.Errorf("callees of Page: %s", got)
		}
	})
	t.Run("code actions", func(t *testing.T) {
		// A quick fix, as a literal with its edit: the missing import.
		c.Change(page, strings.Replace(app[page], "const count = twice(2);", "const count = twice(2); greeting;", 1))
		diagnostics := c.Diagnostics(page)
		if got := lsptest.Lines(diagnostics); len(got) != 1 || got[0] != "7:27 TS2304" {
			t.Fatalf("diagnostics: %q", got)
		}
		var actions []struct {
			Title string                `json:"title"`
			Kind  string                `json:"kind"`
			Edit  lsptest.WorkspaceEdit `json:"edit"`
		}
		c.Request("textDocument/codeAction", map[string]any{"textDocument": doc, "range": diagnostics[0].Range, "context": map[string]any{"diagnostics": diagnostics, "only": []string{"quickfix"}}}, &actions)
		fixed := false
		for _, action := range actions {
			if action.Title == `Update import from "./util"` && action.Kind == "quickfix" && !fixed {
				c.Apply(action.Edit)
				fixed = true
			}
		}
		if got := lsptest.Lines(c.Diagnostics(page)); !fixed || len(got) != 0 {
			t.Errorf("actions %+v; diagnostics after the fix: %q", actions, got)
		}
		c.Change(page, app[page])
	})
	t.Run("workspace symbols", func(t *testing.T) {
		var symbols []struct {
			Name     string           `json:"name"`
			Location lsptest.Location `json:"location"`
		}
		c.Request("workspace/symbol", map[string]any{"query": "t"}, &symbols)
		var names []string
		for _, s := range symbols {
			if !strings.HasSuffix(s.Location.URI, ".rtsx") {
				t.Errorf("%s is in %s: not ours", s.Name, c.Rel(s.Location.URI))
			}
			names = append(names, s.Name)
		}
		if len(names) == 0 {
			t.Error("no symbols of .rtsx files")
		}
	})
	t.Run("file rename", func(t *testing.T) {
		edited := func(oldRel, newRel string) string {
			var edit lsptest.WorkspaceEdit
			c.Request("workspace/willRenameFiles", map[string]any{"files": []any{map[string]any{"oldUri": c.URI(oldRel), "newUri": c.URI(newRel)}}}, &edit)
			return strings.Join(c.Edits(edit), ", ")
		}
		// A .ts file: its .rtsx importers only; main.tsx is the user's TypeScript's.
		if got := edited("src/util.ts", "src/helpers.ts"); got != `src/page.rtsx 3:38-3:44 "./helpers"` {
			t.Errorf("renaming util.ts edits %q", got)
		}
		// An .rtsx file: every importer — no other server knows the module —
		// and without the extension, as the import was written.
		if got := edited("src/page.rtsx", "src/home.rtsx"); got != `src/main.tsx 1:23-1:29 "./home"` {
			t.Errorf("renaming page.rtsx edits %q", got)
		}
		if got := edited("src/button.rtsx", "src/btn.rtsx"); got != `src/page.rtsx 2:36-2:44 "./btn"` {
			t.Errorf("renaming button.rtsx edits %q", got)
		}
	})
}

// ide.md, *Tolerance*: while a file is being typed the server keeps
// answering. Over typing-like mutants of the page — cut off, a character
// deleted, a sigil typed — hover, completion and diagnostics at the edit
// never fail (a panic in the checker on a recovered tree would).
func TestTypingNeverFails(t *testing.T) {
	c := start(t, app)
	const page = "src/page.rtsx"
	c.Open(page)
	src := app[page]
	typed := []string{"<", ">", "{", "}", "&", "#", "$", ".", "=", "/", "\"", "("}
	requests := 0
	try := func(text string, offset int) {
		c.Change(page, text)
		offset = min(offset, len(text))
		at := map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}, "position": lsptest.PositionAt(text, offset)}
		for _, method := range []string{"textDocument/hover", "textDocument/completion", "textDocument/definition"} {
			requests++
			if err := c.Try(method, at, nil); err != nil {
				t.Fatalf("%s failed at offset %d: %v\n--- text\n%s", method, offset, err, text)
			}
		}
		requests++
		if err := c.Try("textDocument/diagnostic", map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}}, nil); err != nil {
			t.Fatalf("diagnostics failed: %v\n--- text\n%s", err, text)
		}
	}
	stride := 7
	if testing.Short() {
		stride = 41
	}
	for i := 0; i <= len(src); i += stride {
		try(src[:i], i)
		if i < len(src) {
			try(src[:i]+src[i+1:], i)
		}
		try(src[:i]+typed[(i/stride)%len(typed)]+src[i:], i+1)
	}
	t.Logf("%d requests", requests)
}

// ide.md, second OPEN of *Not in the first release*: at the end of a copied
// expression that generated text follows, does completion offer auto-imports?
func TestAutoImportAtCopyBoundary(t *testing.T) {
	c := start(t, app)
	const page = "src/page.rtsx"
	c.Open(page)
	for name, edit := range map[string][2]string{
		"a slot attribute's value": {`className="icon"`, `className={greetin}`},
		"a Switch subject":         {`on={status}`, `on={greetin}`},
		"a slot body":              {`{iconSize.toUpperCase()}`, `{greetin}`},
	} {
		c.Change(page, strings.Replace(app[page], edit[0], edit[1], 1))
		var list struct {
			Items []struct {
				Label string `json:"label"`
			} `json:"items"`
		}
		c.Request("textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}, "position": c.At(page, "greetin", 1, 7)}, &list)
		found := false
		for _, item := range list.Items {
			found = found || item.Label == "greeting"
		}
		if !found {
			t.Errorf("%s: no auto-import among %d items", name, len(list.Items))
		}
	}
}

// The user's settings reach the server (ide.md, *Testing*: the client
// answers workspace/configuration as VS Code does) — at the start, and when
// they change.
func TestSettings(t *testing.T) {
	returnTypes := func(on bool) map[string]any {
		return map[string]any{"typescript": map[string]any{"inlayHints": map[string]any{"functionLikeReturnTypes": map[string]any{"enabled": on}}}}
	}
	c := lsptest.StartWith(t, lsptest.Project(t, app), serve, lsptest.Options{Settings: returnTypes(true)})
	const intro = "src/intro.rtsx"
	c.Open(intro)
	hints := func() string {
		var hints []struct {
			Position lsptest.Position `json:"position"`
			Label    json.RawMessage  `json:"label"`
		}
		whole := map[string]any{"textDocument": map[string]any{"uri": c.URI(intro)}, "range": map[string]any{"start": lsptest.Position{}, "end": lsptest.Position{Line: 3}}}
		c.Request("textDocument/inlayHint", whole, &hints)
		out := ""
		for _, h := range hints {
			var parts []struct {
				Value string `json:"value"`
			}
			json.Unmarshal(h.Label, &parts)
			out += fmt.Sprintf("%d:%d", h.Position.Line+1, h.Position.Character+1)
			for _, part := range parts {
				out += part.Value
			}
			out += "; "
		}
		return out
	}
	if got := hints(); got != "1:32: Element; " {
		t.Errorf("hints with return types on: %s", got)
	}
	c.Configure(returnTypes(false))
	if got := hints(); got != "" {
		t.Errorf("hints with return types off: %s", got)
	}
}
