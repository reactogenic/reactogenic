package lsp_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// One mistake (TS2322 on `n`); the rest is JSX and an rtsx slot element with
// params, which plain TypeScript does not parse.
const untitled = `const n: number = "x";
function Box(p: { $Icon?: unknown }) { return null; }
export const a = <div>{n}</div>;
export const b = <Box><$Icon className="i" { size }>{size}</$Icon></Box>;
`

// ide.md, *VS Code extension* → Client: the server is attached to untitled
// documents of the language rtsx. Such a document has no file name, so no
// extension to find the mapper by: the language id of didOpen decides.
func TestUntitledDocument(t *testing.T) {
	// The second and third are VS Code's names for a new file that has a path.
	for _, uri := range []string{"untitled:Untitled-1", "untitled:/tmp/new%20file", "untitled:/tmp/new.rtsx"} {
		t.Run(uri, func(t *testing.T) {
			client := start(t, app)
			doc := map[string]any{"uri": uri}
			open := func(language, text string) {
				client.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": language, "version": 1, "text": text}})
			}
			pull := func() []string {
				t.Helper()
				var report struct {
					Items []lsptest.Diagnostic `json:"items"`
				}
				client.Request("textDocument/diagnostic", map[string]any{"textDocument": doc}, &report)
				lines := []string{}
				for _, d := range report.Items {
					if d.Severity != 4 {
						lines = append(lines, d.String())
					}
				}
				return lines
			}
			// Mapped: the type error at its place, and no error that says
			// "this is not TypeScript" — a syntax error (TS1xxx: the `>` of
			// `</div>` read as a regular expression), an element name read as
			// an identifier (TS2304), JSX without the option (TS17004).
			// (A document outside every project has no JSX types: TS7026.)
			mapped := func(when string) {
				t.Helper()
				got := pull()
				t.Logf("%s: %q", when, got)
				if !slices.Contains(got, "1:7 TS2322") {
					t.Errorf("%s: no TS2322 at 1:7: %q", when, got)
				}
				for _, line := range got {
					code := line[strings.Index(line, " ")+1:]
					if (strings.HasPrefix(code, "TS1") && len(code) == 6) || code == "TS2304" || strings.HasPrefix(code, "TS17") {
						t.Errorf("%s: read as plain TypeScript: %q", when, got)
						break
					}
				}
			}

			open("rtsx", untitled)
			mapped("opened")

			// The answers name the document as the client does.
			at := lsptest.PositionAt(untitled, strings.Index(untitled, "{n}")+1)
			var hover struct {
				Contents struct {
					Value string `json:"value"`
				} `json:"contents"`
			}
			client.Request("textDocument/hover", map[string]any{"textDocument": doc, "position": at}, &hover)
			if !strings.Contains(hover.Contents.Value, "const n: number") {
				t.Errorf("hover: %q", hover.Contents.Value)
			}
			var definition []struct {
				TargetURI string `json:"targetUri"`
			}
			client.Request("textDocument/definition", map[string]any{"textDocument": doc, "position": at}, &definition)
			if len(definition) != 1 || definition[0].TargetURI != uri {
				t.Errorf("definition in the document itself: %+v, want %q", definition, uri)
			}
			var edit struct {
				Changes         map[string]json.RawMessage `json:"changes"`
				DocumentChanges []struct {
					TextDocument struct {
						URI string `json:"uri"`
					} `json:"textDocument"`
				} `json:"documentChanges"`
			}
			client.Request("textDocument/rename", map[string]any{"textDocument": doc, "position": at, "newName": "m"}, &edit)
			renamed := []string{}
			for name := range edit.Changes {
				renamed = append(renamed, name)
			}
			for _, change := range edit.DocumentChanges {
				renamed = append(renamed, change.TextDocument.URI)
			}
			if len(renamed) != 1 || renamed[0] != uri {
				t.Errorf("rename edits the documents %q, want %q", renamed, uri)
			}

			// The source-tree features, which the front answers itself.
			typed := untitled + "export const c = <$Label>"
			client.Notify("textDocument/didChange", map[string]any{
				"textDocument":   map[string]any{"uri": uri, "version": 2},
				"contentChanges": []any{map[string]any{"text": typed}},
			})
			var closing *struct {
				Edit struct {
					NewText string `json:"newText"`
				} `json:"_vs_textEdit"`
			}
			client.Request("textDocument/_vs_onAutoInsert", map[string]any{
				"_vs_textDocument": doc, "_vs_position": lsptest.PositionAt(typed, len(typed)), "_vs_ch": ">",
			}, &closing)
			if closing == nil || closing.Edit.NewText != `$0</\$Label>` {
				t.Errorf("closing tag: %+v", closing)
			}

			// A text that holds the document's own name stays as typed.
			named := untitled + "export const name = \"" + uri + "\";\n"
			client.Notify("textDocument/didChange", map[string]any{
				"textDocument":   map[string]any{"uri": uri, "version": 3},
				"contentChanges": []any{map[string]any{"text": named}},
			})
			mapped("changed")
			client.Request("textDocument/hover", map[string]any{"textDocument": doc, "position": lsptest.PositionAt(named, strings.Index(named, "name =")+1)}, &hover)
			if !strings.Contains(hover.Contents.Value, `const name: "`+uri+`"`) {
				t.Errorf("hover on a string that is the document's name: %q", hover.Contents.Value)
			}

			// Closed: the name is free for a document of another language.
			client.Notify("textDocument/didClose", map[string]any{"textDocument": doc})
			open("typescript", "const s: string = 1;\n")
			if got := pull(); len(got) != 1 || got[0] != "1:7 TS2322" {
				t.Errorf("the same name as TypeScript: %q", got)
			}
			client.Notify("textDocument/didClose", map[string]any{"textDocument": doc})
		})
	}
}
