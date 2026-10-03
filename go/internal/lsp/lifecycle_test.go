package lsp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/lsp"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// A client that declares nothing: the server asks it nothing.
func initialize(c *lsptest.Conn, root string) {
	c.Send(1, "initialize", map[string]any{"processId": nil, "rootUri": "file://" + root, "capabilities": map[string]any{}})
	c.Answer(1)
}

// How a session ends (ide.md, *reactogenic lsp*): `exit` is the front's, so
// it works in any state of the server and with the input still open; its
// result is the exit status — nil only after `shutdown`.
func TestExit(t *testing.T) {
	root := lsptest.Project(t, map[string]string{"src/button.rtsx": button})
	for _, c := range []struct {
		name    string
		session func(c *lsptest.Conn)
		want    error
		reports string // or: the error names this
	}{
		{"shutdown, exit", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Send(0, "initialized", map[string]any{})
			c.Send(2, "shutdown", nil)
			c.Answer(2)
			c.Send(0, "exit", nil)
		}, nil, ""},
		{"exit without shutdown", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Send(0, "initialized", map[string]any{})
			c.Send(0, "exit", nil)
		}, lsp.ErrNoShutdown, ""},
		{"exit before initialized", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Send(0, "exit", nil)
		}, lsp.ErrNoShutdown, ""},
		{"exit before initialize", func(c *lsptest.Conn) {
			c.Send(0, "exit", nil)
		}, lsp.ErrNoShutdown, ""},
		{"the input ends", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Send(0, "initialized", map[string]any{})
			c.Close()
		}, nil, ""},
		{"a header without Content-Length", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Write("Content-Type: application/json\r\n\r\n{}")
		}, nil, "Content-Length"},
		{"a Content-Length that is no number", func(c *lsptest.Conn) {
			c.Write("Content-Length: many\r\n\r\n{}")
		}, nil, `"many"`},
		{"the input ends inside a message", func(c *lsptest.Conn) {
			initialize(c, root)
			c.Write("Content-Length: 100\r\n\r\n{}")
			c.Close()
		}, nil, "inside a message body"},
	} {
		t.Run(c.name, func(t *testing.T) {
			conn := lsptest.Connect(t, root, serve)
			c.session(conn)
			// The input is still open, unless the session closed it.
			err := conn.Ended(10 * time.Second)
			switch {
			case c.reports != "":
				if err == nil || !strings.Contains(err.Error(), c.reports) {
					t.Errorf("Serve returned %v; want an error about %s", err, c.reports)
				}
			case !errors.Is(err, c.want) || (c.want == nil && err != nil):
				t.Errorf("Serve returned %v; want %v", err, c.want)
			}
		})
	}
}

// Documents the client should not have sent (ide.md: the client attaches
// to `file` documents of language rtsx) end nothing. One whose URI ends in
// .rtsx is the front's as any other — its URI is never a file name; one
// that does not is not ours: TypeScript's answers, whatever they are.
func TestOddDocuments(t *testing.T) {
	c := start(t, app)
	text := app["src/page.rtsx"]
	answered := func(uri, method string, params map[string]any) string {
		t.Helper()
		var raw json.RawMessage
		if err := c.Try(method, params, &raw); err != nil {
			t.Errorf("%s for %s: %v", method, uri, err)
		}
		return string(raw)
	}
	for _, d := range []struct {
		uri    string
		fronts bool // the front answers the source-tree features
	}{
		{"untitled:Untitled-1.rtsx", true},
		{"untitled:" + c.Root + "/src/new.rtsx", true},
		{"file:" + c.Root + "/src/page.rtsx", true}, // one slash, as Java clients write it
		{"file://" + c.Root + "/src/./page.rtsx", true},
		{"git:" + c.Root + "/src/page.rtsx", true},
		{"untitled:Untitled-1", false}, // a new buffer, language mode rtsx
	} {
		doc := map[string]any{"uri": d.uri}
		at := c.At("src/page.rtsx", "<$Icon", 1, 3)
		c.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": d.uri, "languageId": "rtsx", "version": 1, "text": text}})
		var folds []struct{ StartLine, EndLine int }
		json.Unmarshal([]byte(answered(d.uri, "textDocument/foldingRange", map[string]any{"textDocument": doc})), &folds)
		// The slot element folds only on the source tree (lines 13–14).
		fronted := false
		for _, fold := range folds {
			fronted = fronted || fold.StartLine == 12 && fold.EndLine == 13
		}
		if fronted != d.fronts {
			t.Errorf("%s: folded on the source tree: %v, want %v\n%+v", d.uri, fronted, d.fronts, folds)
		}
		answered(d.uri, "textDocument/selectionRange", map[string]any{"textDocument": doc, "positions": []any{at}})
		typed := strings.Replace(text, "<section #intro />", "<section #intro />\n      <$Label>", 1)
		c.Notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": d.uri, "version": 2}, "contentChanges": []any{map[string]any{"text": typed}}})
		var closing struct {
			Edit struct{ NewText string } `json:"_vs_textEdit"`
		}
		json.Unmarshal([]byte(answered(d.uri, "textDocument/_vs_onAutoInsert", map[string]any{"_vs_textDocument": doc, "_vs_position": lsptest.PositionAt(typed, strings.Index(typed, "<$Label>")+len("<$Label>")), "_vs_ch": ">"})), &closing)
		if fronted := closing.Edit.NewText == `$0</\$Label>`; fronted != d.fronts {
			t.Errorf("%s: closing tag from the source tree: %v, want %v (%q)", d.uri, fronted, d.fronts, closing.Edit.NewText)
		}
		for _, method := range []string{"textDocument/hover", "textDocument/definition", "textDocument/completion"} {
			answered(d.uri, method, map[string]any{"textDocument": doc, "position": at})
		}
		answered(d.uri, "textDocument/diagnostic", map[string]any{"textDocument": doc})
		answered(d.uri, "textDocument/documentSymbol", map[string]any{"textDocument": doc})
		c.Notify("textDocument/didClose", map[string]any{"textDocument": doc})
	}
	// The server is still there, for the documents that are its own.
	c.Open("src/page.rtsx")
	if got := lsptest.Lines(c.Diagnostics("src/page.rtsx")); len(got) != 0 {
		t.Errorf("diagnostics of the page: %q", got)
	}
}

// Positions no document has — a client's mistake — are the server's to
// refuse: the front neither answers them nor dies of them.
func TestMalformedPositions(t *testing.T) {
	c := start(t, app)
	const page = "src/page.rtsx"
	c.Open(page)
	doc := map[string]any{"uri": c.URI(page)}
	for _, position := range []map[string]any{
		{"line": -1, "character": 0},
		{"line": 0, "character": -1},
		{"line": 4294967295, "character": 0}, // an index of -1 once it is an int32
		{"line": 1.5, "character": 0},
		{"line": "1", "character": 0},
	} {
		name := fmt.Sprint(position)
		if err := c.Try("textDocument/_vs_onAutoInsert", map[string]any{"_vs_textDocument": doc, "_vs_position": position, "_vs_ch": ">"}, nil); err == nil {
			t.Errorf("auto-insert at %s: no error", name)
		}
		if err := c.Try("textDocument/selectionRange", map[string]any{"textDocument": doc, "positions": []any{position}}, nil); err == nil {
			t.Errorf("selection range at %s: no error", name)
		}
		c.Notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": c.URI(page), "version": 2}, "contentChanges": []any{
			map[string]any{"range": map[string]any{"start": position, "end": position}, "text": "x"},
		}})
	}
	// Nothing was applied: the front's tree and the server's text are the page's.
	var ranges []struct {
		StartLine int `json:"startLine"`
		EndLine   int `json:"endLine"`
	}
	c.Request("textDocument/foldingRange", map[string]any{"textDocument": doc}, &ranges)
	found := false
	for _, r := range ranges {
		found = found || r.StartLine == 12 && r.EndLine == 13
	}
	if !found {
		t.Errorf("folds after the malformed changes: %+v", ranges)
	}
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Errorf("diagnostics after the malformed changes: %q", got)
	}
}
