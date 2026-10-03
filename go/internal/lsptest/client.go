// Package lsptest is an LSP client for tests (specs/phase01/ide.md,
// *Testing*): it runs a server in-process over a pipe — so the race
// detector sees it — and behaves as VS Code does: UTF-16 positions, pull
// diagnostics with refresh, watched-file events it sends itself. A server
// request it does not know fails the test.
package lsptest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
)

// Serve is a server's entry point: LSP on in and out until in ends.
type Serve func(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error

// Client is one editor session over a project directory.
type Client struct {
	t    *testing.T
	Root string // absolute, with forward slashes

	w       io.WriteCloser
	writeMu sync.Mutex

	mu            sync.Mutex
	next          int
	pending       map[int]chan response
	docs          map[string]string // open documents, by path relative to Root
	versions      map[string]int
	Registrations []string // methods the server registered dynamically
	Refreshes     int      // workspace/diagnostic/refresh requests received
	// Initialized is the server's answer to initialize.
	Initialized struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	done chan error
}

type response struct {
	Result json.RawMessage
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
}

// Start runs serve on root and performs the initialize handshake.
func Start(t *testing.T, root string, serve Serve) *Client {
	t.Helper()
	toServer, clientW := io.Pipe()
	clientR, fromServer := io.Pipe()
	c := &Client{t: t, Root: filepath.ToSlash(root), w: clientW, pending: map[int]chan response{}, docs: map[string]string{}, versions: map[string]int{}, done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.done <- serve(ctx, toServer, fromServer, io.Discard, c.Root)
		fromServer.Close()
	}()
	go c.read(bufio.NewReader(clientR))
	t.Cleanup(func() {
		c.Request("shutdown", nil, nil)
		c.Notify("exit", nil)
		clientW.Close()
		select {
		case <-c.done:
		case <-time.After(10 * time.Second):
			t.Error("the server did not stop after exit")
		}
		cancel()
	})

	c.Request("initialize", map[string]any{
		"processId":        nil,
		"rootUri":          c.URI(""),
		"workspaceFolders": []any{map[string]any{"uri": c.URI(""), "name": "test"}},
		"capabilities": map[string]any{
			"general": map[string]any{"positionEncodings": []string{"utf-16"}},
			"window":  map[string]any{"workDoneProgress": true},
			"workspace": map[string]any{
				"configuration":         true,
				"workspaceFolders":      true,
				"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true},
				"diagnostics":           map[string]any{"refreshSupport": true},
			},
			"textDocument": map[string]any{
				"synchronization": map[string]any{"dynamicRegistration": true},
				"diagnostic":      map[string]any{"dynamicRegistration": true},
				"hover":           map[string]any{"dynamicRegistration": true, "contentFormat": []string{"markdown", "plaintext"}},
				"completion":      map[string]any{"dynamicRegistration": true},
				"definition":      map[string]any{"dynamicRegistration": true, "linkSupport": true},
				"rename":          map[string]any{"dynamicRegistration": true, "prepareSupport": true},
				"foldingRange":    map[string]any{"dynamicRegistration": true},
				"semanticTokens": map[string]any{
					"dynamicRegistration": true,
					"requests":            map[string]any{"full": true, "range": true},
					"formats":             []string{"relative"},
					"tokenTypes":          []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator", "decorator"},
					"tokenModifiers":      []string{"declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary"},
				},
			},
		},
	}, &c.Initialized)
	c.Notify("initialized", map[string]any{})
	return c
}

func (c *Client) read(r *bufio.Reader) {
	for {
		length := 0
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				c.mu.Lock()
				for _, ch := range c.pending {
					close(ch)
				}
				c.pending = map[int]chan response{}
				c.mu.Unlock()
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				length, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params json.RawMessage  `json:"params"`
			response
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			c.t.Errorf("lsptest: bad message from the server: %v", err)
			continue
		}
		switch {
		case msg.Method != "" && msg.ID != nil:
			c.serverRequest(*msg.ID, msg.Method, msg.Params)
		case msg.Method != "":
			// A notification: logs, telemetry, progress.
		case msg.ID != nil:
			var id int
			json.Unmarshal(*msg.ID, &id)
			c.mu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- msg.response
			}
		}
	}
}

// serverRequest answers what a server may ask of a client.
func (c *Client) serverRequest(id json.RawMessage, method string, params json.RawMessage) {
	var result any
	switch method {
	case "workspace/configuration":
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		json.Unmarshal(params, &p)
		result = make([]any, len(p.Items))
	case "client/registerCapability":
		var p struct {
			Registrations []struct {
				ID     string `json:"id"`
				Method string `json:"method"`
			} `json:"registrations"`
		}
		json.Unmarshal(params, &p)
		c.mu.Lock()
		for _, r := range p.Registrations {
			c.Registrations = append(c.Registrations, r.Method+" "+r.ID)
		}
		c.mu.Unlock()
	case "client/unregisterCapability", "window/workDoneProgress/create",
		"workspace/inlayHint/refresh", "workspace/semanticTokens/refresh", "workspace/codeLens/refresh":
	case "workspace/diagnostic/refresh":
		c.mu.Lock()
		c.Refreshes++
		c.mu.Unlock()
	default:
		c.t.Errorf("lsptest: the server sent a request this client does not answer: %s", method)
		go c.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "not supported by the test client"}})
		return
	}
	// Not from the read loop: an editor keeps reading while it writes.
	go c.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (c *Client) send(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatal(err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) {
	c.send(message(nil, method, params))
}

func message(id *int, method string, params any) map[string]any {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		msg["id"] = *id
	}
	if params != nil {
		msg["params"] = params
	}
	return msg
}

// Request sends a request and decodes its result into result (may be nil).
// A server error fails the test; use Try to inspect one.
func (c *Client) Request(method string, params, result any) {
	c.t.Helper()
	if err := c.Try(method, params, result); err != nil {
		c.t.Errorf("%s: %v", method, err) // not Fatal: subtests share the client
	}
}

// Try is Request, returning the server's error.
func (c *Client) Try(method string, params, result any) error {
	c.t.Helper()
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	c.send(message(&id, method, params))
	select {
	case r, ok := <-ch:
		if !ok {
			return fmt.Errorf("the server closed the connection")
		}
		if r.Error != nil {
			return fmt.Errorf("%s (%d)", r.Error.Message, r.Error.Code)
		}
		if result != nil && len(r.Result) > 0 {
			return json.Unmarshal(r.Result, result)
		}
		return nil
	case <-time.After(60 * time.Second):
		return fmt.Errorf("no response in 60s")
	}
}

// URI is the file URI of a path relative to the root.
func (c *Client) URI(rel string) string {
	if rel == "" {
		return "file://" + c.Root
	}
	return "file://" + c.Root + "/" + rel
}

// Rel is the path relative to the root of a file URI.
func (c *Client) Rel(uri string) string {
	return strings.TrimPrefix(strings.TrimPrefix(uri, "file://"+c.Root), "/")
}

func languageID(rel string) string {
	switch filepath.Ext(rel) {
	case ".rtsx":
		return "rtsx"
	case ".tsx":
		return "typescriptreact"
	case ".ts":
		return "typescript"
	}
	return "plaintext"
}

// Text is the text of a document: its open buffer, or the file.
func (c *Client) Text(rel string) string {
	c.mu.Lock()
	text, open := c.docs[rel]
	c.mu.Unlock()
	if open {
		return text
	}
	data, err := os.ReadFile(filepath.Join(c.Root, rel))
	if err != nil {
		c.t.Fatal(err)
	}
	return string(data)
}

// Open opens the file as the editor does.
func (c *Client) Open(rel string) {
	c.OpenAs(rel, c.Text(rel))
}

// OpenAs opens a document with text that may not be on disk.
func (c *Client) OpenAs(rel, text string) {
	c.mu.Lock()
	c.docs[rel] = text
	c.versions[rel] = 1
	c.mu.Unlock()
	c.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel), "languageId": languageID(rel), "version": 1, "text": text}})
}

// Change replaces an open document's text.
func (c *Client) Change(rel, text string) {
	c.mu.Lock()
	c.docs[rel] = text
	c.versions[rel]++
	version := c.versions[rel]
	c.mu.Unlock()
	c.Notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": c.URI(rel), "version": version},
		"contentChanges": []any{map[string]any{"text": text}},
	})
}

// WriteFile writes a file on disk and tells the server, as a file watcher
// does. kind: 1 created, 2 changed.
func (c *Client) WriteFile(rel, text string) {
	p := filepath.Join(c.Root, rel)
	kind := 2
	if _, err := os.Stat(p); err != nil {
		kind = 1
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.Notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": c.URI(rel), "type": kind}}})
}

// RemoveFile deletes a file on disk and tells the server.
func (c *Client) RemoveFile(rel string) {
	if err := os.Remove(filepath.Join(c.Root, rel)); err != nil {
		c.t.Fatal(err)
	}
	c.Notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": c.URI(rel), "type": 3}}})
}

// Position is an LSP position: zero-based line, UTF-16 character.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is an LSP range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// At is the position of offset bytes into the n-th (1-based) occurrence of
// needle in the document.
func (c *Client) At(rel, needle string, n, offset int) Position {
	c.t.Helper()
	text := c.Text(rel)
	i := -1
	for k := 0; k < n; k++ {
		j := strings.Index(text[i+1:], needle)
		if j < 0 {
			c.t.Fatalf("%s: %q occurs fewer than %d times", rel, needle, n)
		}
		i += 1 + j
	}
	return position(text, i+offset)
}

// PositionAt is the LSP position of a byte offset in text.
func PositionAt(text string, offset int) Position { return position(text, offset) }

func position(text string, offset int) Position {
	line := strings.Count(text[:offset], "\n")
	start := strings.LastIndex(text[:offset], "\n") + 1
	return Position{Line: line, Character: len(utf16.Encode([]rune(text[start:offset])))}
}

// Show renders a range as `line:col-line:col`, one-based, as editors do.
func (r Range) String() string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line+1, r.Start.Character+1, r.End.Line+1, r.End.Character+1)
}

func (c *Client) doc(rel string, at Position) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}, "position": at}
}

// Hover returns the hover text at a position, or "".
func (c *Client) Hover(rel string, at Position) string {
	c.t.Helper()
	var result *struct {
		Contents json.RawMessage `json:"contents"`
	}
	c.Request("textDocument/hover", c.doc(rel, at), &result)
	if result == nil {
		return ""
	}
	var markup struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(result.Contents, &markup) == nil && markup.Value != "" {
		return markup.Value
	}
	return string(result.Contents)
}

// Diagnostic is one pulled diagnostic.
type Diagnostic struct {
	Range    Range           `json:"range"`
	Severity int             `json:"severity"` // 1 error, 2 warning, 3 information, 4 hint
	Code     json.RawMessage `json:"code"`
	Source   string          `json:"source"`
	Message  string          `json:"message"`
}

// String is `line:col CODE`: how check prints a position and a code.
func (d Diagnostic) String() string {
	code := strings.Trim(string(d.Code), `"`)
	if _, err := strconv.Atoi(code); err == nil {
		code = "TS" + code
	}
	return fmt.Sprintf("%d:%d %s", d.Range.Start.Line+1, d.Range.Start.Character+1, code)
}

// Diagnostics pulls a document's diagnostics, as the editor does after a
// change; suggestions (hints) are left out.
func (c *Client) Diagnostics(rel string) []Diagnostic {
	c.t.Helper()
	var result struct {
		Items []Diagnostic `json:"items"`
	}
	c.Request("textDocument/diagnostic", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &result)
	var out []Diagnostic
	for _, d := range result.Items {
		if d.Severity != 4 {
			out = append(out, d)
		}
	}
	return out
}

// Lines renders diagnostics as their String forms.
func Lines(diagnostics []Diagnostic) []string {
	out := []string{}
	for _, d := range diagnostics {
		out = append(out, d.String())
	}
	return out
}

// Location is where a definition or reference is.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Definition returns the definitions of the symbol at a position, as
// `file line:col`.
func (c *Client) Definition(rel string, at Position) []string {
	c.t.Helper()
	var raw json.RawMessage
	c.Request("textDocument/definition", c.doc(rel, at), &raw)
	var links []struct {
		TargetURI            string `json:"targetUri"`
		TargetSelectionRange Range  `json:"targetSelectionRange"`
		Location
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, &links); err != nil {
		links = links[:1]
		json.Unmarshal(raw, &links[0])
	}
	var out []string
	for _, l := range links {
		uri, r := l.URI, l.Range
		if l.TargetURI != "" {
			uri, r = l.TargetURI, l.TargetSelectionRange
		}
		out = append(out, fmt.Sprintf("%s %d:%d", c.Rel(uri), r.Start.Line+1, r.Start.Character+1))
	}
	return out
}
