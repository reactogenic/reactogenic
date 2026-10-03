// Package lsptest is an LSP client for tests (specs/phase01/ide.md,
// *Testing*): it runs a server in-process over a pipe — so the race
// detector sees it — and behaves as VS Code does: UTF-16 positions, pull
// diagnostics with refresh, watched-file events it sends itself, the
// capabilities that change a server's answers (hierarchical symbols, line
// folding, resolved completion items, code action literals, document
// changes), the user's settings on workspace/configuration, and `exit` with
// the pipes still open. A server request it does not know fails the test.
// A change on disk is reported only where the server watches: a watcher it
// registered matches the path.
package lsptest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	options       Options
	registrations []string                    // what the server registered dynamically: `method id`
	watches       bool                        // the client takes file watchers (workspace.didChangeWatchedFiles)
	watchers      map[string][]*regexp.Regexp // the globs of each registered file watcher, by registration id
	asked         []string                    // the methods of the server's requests
	logs          []string                    // the server's window/logMessage texts
	Refreshes     int                         // workspace/diagnostic/refresh requests received
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

// Options vary a session from VS Code's.
type Options struct {
	// Settings are the user's settings, by section ("typescript", "editor",
	// …): the answers to workspace/configuration.
	Settings map[string]any
	// Capabilities edits the client capabilities before initialize: another
	// editor.
	Capabilities func(capabilities map[string]any)
	// Silent names the server requests this client never answers.
	Silent func(method string) bool
	// ProcessID is the client's process id in initialize; 0 is null.
	ProcessID int
}

// Start runs serve on root and performs the initialize handshake.
func Start(t *testing.T, root string, serve Serve) *Client {
	t.Helper()
	return StartWith(t, root, serve, Options{})
}

// StartWith is Start for a client that differs from VS Code.
func StartWith(t *testing.T, root string, serve Serve, options Options) *Client {
	t.Helper()
	toServer, clientW := io.Pipe()
	clientR, fromServer := io.Pipe()
	c := &Client{t: t, Root: filepath.ToSlash(root), w: clientW, pending: map[int]chan response{}, docs: map[string]string{}, versions: map[string]int{}, watchers: map[string][]*regexp.Regexp{}, options: options, done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.done <- serve(ctx, toServer, fromServer, io.Discard, c.Root)
		fromServer.Close()
	}()
	go c.read(bufio.NewReader(clientR))
	t.Cleanup(func() {
		// As an editor stops a server: shutdown, exit, and only then — the
		// server gone — the pipes.
		if err := c.Try("shutdown", nil, nil); err != nil {
			t.Errorf("shutdown: %v", err)
		}
		c.Notify("exit", nil)
		select {
		case err := <-c.done:
			if err != nil {
				t.Errorf("the server ended with %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("the server did not stop after exit")
		}
		clientW.Close()
		cancel()
	})

	capabilities := map[string]any{
		"general": map[string]any{"positionEncodings": []string{"utf-16"}},
		"window":  map[string]any{"workDoneProgress": true},
		"workspace": map[string]any{
			"configuration":          true,
			"workspaceFolders":       true,
			"didChangeConfiguration": map[string]any{"dynamicRegistration": true},
			"didChangeWatchedFiles":  map[string]any{"dynamicRegistration": true},
			"diagnostics":            map[string]any{"refreshSupport": true},
			"workspaceEdit":          map[string]any{"documentChanges": true},
		},
		"textDocument": map[string]any{
			"synchronization": map[string]any{"dynamicRegistration": true},
			"diagnostic": map[string]any{"dynamicRegistration": true, "relatedDocumentSupport": false, // as vscode-languageclient 10
				"relatedInformation": true, "tagSupport": map[string]any{"valueSet": []int{1, 2}}},
			"hover": map[string]any{"dynamicRegistration": true, "contentFormat": []string{"markdown", "plaintext"}},
			"completion": map[string]any{"dynamicRegistration": true, "completionItem": map[string]any{
				"resolveSupport": map[string]any{"properties": []string{"documentation", "detail", "additionalTextEdits"}},
			}},
			"definition":     map[string]any{"dynamicRegistration": true, "linkSupport": true},
			"rename":         map[string]any{"dynamicRegistration": true, "prepareSupport": true},
			"documentSymbol": map[string]any{"dynamicRegistration": true, "hierarchicalDocumentSymbolSupport": true},
			"foldingRange":   map[string]any{"dynamicRegistration": true, "lineFoldingOnly": true},
			"codeAction": map[string]any{"dynamicRegistration": true, "codeActionLiteralSupport": map[string]any{"codeActionKind": map[string]any{
				"valueSet": []string{"", "quickfix", "refactor", "refactor.extract", "refactor.inline", "refactor.rewrite", "source", "source.organizeImports"},
			}}},
			"semanticTokens": map[string]any{
				"dynamicRegistration": true,
				"requests":            map[string]any{"full": true, "range": true},
				"formats":             []string{"relative"},
				"tokenTypes":          []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator", "decorator"},
				"tokenModifiers":      []string{"declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary"},
			},
		},
	}
	if options.Capabilities != nil {
		options.Capabilities(capabilities)
	}
	if workspace, ok := capabilities["workspace"].(map[string]any); ok {
		watched, _ := workspace["didChangeWatchedFiles"].(map[string]any)
		c.watches, _ = watched["dynamicRegistration"].(bool)
	}
	var processID any
	if options.ProcessID != 0 {
		processID = options.ProcessID
	}
	c.Request("initialize", map[string]any{
		"processId":        processID,
		"rootUri":          c.URI(""),
		"workspaceFolders": []any{map[string]any{"uri": c.URI(""), "name": "test"}},
		"capabilities":     capabilities,
	}, &c.Initialized)
	c.Notify("initialized", map[string]any{})
	return c
}

// Registrations is what the server has registered dynamically so far, as
// `method id`.
func (c *Client) Registrations() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.registrations...)
}

// Asked is the methods of the requests the server has sent so far.
func (c *Client) Asked() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.asked...)
}

// Logs is what the server has logged so far (window/logMessage): the output
// panel of the editor.
func (c *Client) Logs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.logs...)
}

// Configure changes the user's settings, as the editor does: the server is
// told, and asks again.
func (c *Client) Configure(settings map[string]any) {
	c.mu.Lock()
	c.options.Settings = settings
	c.mu.Unlock()
	c.Notify("workspace/didChangeConfiguration", map[string]any{"settings": settings})
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
		case msg.Method == "window/logMessage":
			var p struct {
				Message string `json:"message"`
			}
			json.Unmarshal(msg.Params, &p)
			c.mu.Lock()
			c.logs = append(c.logs, p.Message)
			c.mu.Unlock()
		case msg.Method != "":
			// A notification: telemetry, progress.
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
	c.mu.Lock()
	c.asked = append(c.asked, method)
	c.mu.Unlock()
	if c.options.Silent != nil && c.options.Silent(method) {
		return
	}
	var result any
	switch method {
	case "workspace/configuration":
		var p struct {
			Items []struct {
				Section string `json:"section"`
			} `json:"items"`
		}
		json.Unmarshal(params, &p)
		sections := make([]any, len(p.Items))
		c.mu.Lock()
		for i, item := range p.Items {
			sections[i] = c.options.Settings[item.Section] // nil: the section is not set
		}
		c.mu.Unlock()
		result = sections
	case "client/registerCapability":
		var p struct {
			Registrations []struct {
				ID      string `json:"id"`
				Method  string `json:"method"`
				Options struct {
					Watchers []struct {
						GlobPattern json.RawMessage `json:"globPattern"`
					} `json:"watchers"`
				} `json:"registerOptions"`
			} `json:"registrations"`
		}
		json.Unmarshal(params, &p)
		c.mu.Lock()
		for _, r := range p.Registrations {
			c.registrations = append(c.registrations, r.Method+" "+r.ID)
			if r.Method != "workspace/didChangeWatchedFiles" {
				continue
			}
			globs := []*regexp.Regexp{}
			for _, w := range r.Options.Watchers {
				globs = append(globs, globRegexp(globPattern(w.GlobPattern)))
			}
			c.watchers[r.ID] = globs
		}
		c.mu.Unlock()
	case "client/unregisterCapability":
		var p struct {
			Unregistrations []struct {
				ID string `json:"id"`
			} `json:"unregisterations"` // as the protocol spells it
		}
		json.Unmarshal(params, &p)
		c.mu.Lock()
		for _, r := range p.Unregistrations {
			delete(c.watchers, r.ID)
		}
		c.mu.Unlock()
	case "window/workDoneProgress/create",
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
	return c.Start(method, params).Wait(result)
}

// Pending is a request that is sent and not yet answered.
type Pending struct {
	ID     int
	answer chan response
}

// Start sends a request without waiting for its answer: what follows may
// cancel it, or overtake it.
func (c *Client) Start(method string, params any) *Pending {
	c.mu.Lock()
	c.next++
	p := &Pending{ID: c.next, answer: make(chan response, 1)}
	c.pending[p.ID] = p.answer
	c.mu.Unlock()
	c.send(message(&p.ID, method, params))
	return p
}

// Wait decodes the answer's result into result (may be nil), or returns the
// server's error.
func (p *Pending) Wait(result any) error {
	select {
	case r, ok := <-p.answer:
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

// Rel is the path relative to the root of a file URI — of its path, not of
// its spelling: a server writes `,` as `%2C`, as VS Code does. A URI that is
// not under the root is returned as it is.
func (c *Client) Rel(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		if rel, ok := strings.CutPrefix(u.Path, c.Root); ok && (rel == "" || rel[0] == '/') {
			return strings.TrimPrefix(rel, "/")
		}
	}
	return uri
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

// globPattern is the glob of a file watcher as one string: a pattern, or a
// pattern relative to a base URI (or to a workspace folder's).
func globPattern(raw json.RawMessage) string {
	var pattern string
	if json.Unmarshal(raw, &pattern) == nil {
		return pattern
	}
	var relative struct {
		BaseURI json.RawMessage `json:"baseUri"`
		Pattern string          `json:"pattern"`
	}
	json.Unmarshal(raw, &relative)
	var base string
	if json.Unmarshal(relative.BaseURI, &base) != nil {
		var folder struct {
			URI string `json:"uri"`
		}
		json.Unmarshal(relative.BaseURI, &folder)
		base = folder.URI
	}
	return strings.TrimSuffix(base, "/") + "/" + relative.Pattern
}

// globRegexp compiles an LSP glob — `*`, `?`, `**`, `{a,b}`, `[a-z]` — over
// a path; a glob that is a file URI is one over its path.
func globRegexp(glob string) *regexp.Regexp {
	if u, err := url.Parse(glob); err == nil && u.Scheme == "file" {
		glob = u.Path
	}
	var re strings.Builder
	re.WriteString("^")
	braces := 0
	for i := 0; i < len(glob); i++ {
		switch ch := glob[i]; {
		case strings.HasPrefix(glob[i:], "**/"):
			re.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			re.WriteString(".*")
			i++
		case ch == '*':
			re.WriteString("[^/]*")
		case ch == '?':
			re.WriteString("[^/]")
		case ch == '{':
			re.WriteString("(?:")
			braces++
		case ch == '}' && braces > 0:
			re.WriteString(")")
			braces--
		case ch == ',' && braces > 0:
			re.WriteString("|")
		case ch == '[' && strings.IndexByte(glob[i:], ']') > 1:
			end := i + strings.IndexByte(glob[i:], ']')
			re.WriteString("[" + strings.Replace(glob[i+1:end], "!", "^", 1) + "]")
			i = end
		default:
			re.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	re.WriteString("$")
	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return regexp.MustCompile(`a^`) // matches nothing
	}
	return compiled
}

// watchedFile tells the server of a change on disk, as the editor's file
// watcher does: when a watcher the server registered matches the path, and
// not otherwise (kind: 1 created, 2 changed, 3 deleted). The server registers
// its watchers in the background, so a client that takes watchers waits for
// one that matches.
func (c *Client) watchedFile(rel string, kind int) {
	path := c.Root + "/" + rel
	watched := func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, globs := range c.watchers {
			for _, glob := range globs {
				if glob.MatchString(path) {
					return true
				}
			}
		}
		return false
	}
	for deadline := time.Now().Add(10 * time.Second); !watched(); time.Sleep(5 * time.Millisecond) {
		if !c.watches || time.Now().After(deadline) {
			return // nobody watches this file: the server is not told
		}
	}
	c.Notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": c.URI(rel), "type": kind}}})
}

// WriteFile writes a file on disk; the server is told as a file watcher
// tells it (watchedFile).
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
	c.watchedFile(rel, kind)
}

// RemoveFile deletes a file on disk; the server is told as WriteFile tells it.
func (c *Client) RemoveFile(rel string) {
	if err := os.Remove(filepath.Join(c.Root, rel)); err != nil {
		c.t.Fatal(err)
	}
	c.watchedFile(rel, 3)
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
	Tags     []int           `json:"tags,omitempty"` // 1 unnecessary, 2 deprecated
	Related  []struct {
		Location Location `json:"location"`
		Message  string   `json:"message"`
	} `json:"relatedInformation,omitempty"`
}

// Show is the whole diagnostic on one line: `1:5-1:9 error ts(2322) [tags]
// message`, the code a number or a name as the server sent it.
func (d Diagnostic) Show() string {
	severity := [...]string{"?", "error", "warning", "information", "hint"}[d.Severity]
	tags := ""
	for _, tag := range d.Tags {
		tags += " " + [...]string{"?", "unnecessary", "deprecated"}[tag]
	}
	return fmt.Sprintf("%s %s %s(%s)%s %s", d.Range, severity, d.Source, d.Code, tags, d.Message)
}

// RangeText is the text of a range of a document.
func (c *Client) RangeText(rel string, r Range) string {
	text := c.Text(rel)
	return text[offset(text, r.Start):offset(text, r.End)]
}

// Offset is the byte offset of an LSP position in text.
func Offset(text string, at Position) int { return offset(text, at) }

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
	var out []Diagnostic
	for _, d := range c.AllDiagnostics(rel) {
		if d.Severity != 4 {
			out = append(out, d)
		}
	}
	return out
}

// AllDiagnostics pulls a document's diagnostics, suggestions included.
func (c *Client) AllDiagnostics(rel string) []Diagnostic {
	c.t.Helper()
	var result struct {
		Items []Diagnostic `json:"items"`
	}
	c.Request("textDocument/diagnostic", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &result)
	return result.Items
}

// RefreshCount is how often the server has asked the client to pull
// diagnostics again (workspace/diagnostic/refresh).
func (c *Client) RefreshCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Refreshes
}

// Close closes an open document.
func (c *Client) Close(rel string) {
	c.mu.Lock()
	delete(c.docs, rel)
	delete(c.versions, rel)
	c.mu.Unlock()
	c.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}})
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
