package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"
)

// front sits between the client and the fork's server, in this process. It
// answers what TypeScript cannot answer from the virtual text — the
// syntactic features, from the .rtsx source tree (ide.md, *Span map*) — and
// narrows what the server says about files that are not ours: the client
// attaches it to .rtsx documents only, but workspace-wide answers ignore
// that (ide.md, *reactogenic lsp*).
type front struct {
	// outgoing queues the messages for the client. Nothing here may block on
	// writing to the client: the client may itself be blocked writing to us,
	// and each side would wait for the other to read.
	outgoing *queue

	// names renames the documents that are rtsx by language id only.
	names aliases

	mu              sync.Mutex
	docs            map[string]string  // open .rtsx documents, by URI
	pending         map[string]request // forwarded requests whose answers we rewrite, by id
	encoding        string             // position encoding, from the initialize result
	lineFoldingOnly bool
}

type request struct {
	Method string
	Params json.RawMessage
}

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *json.RawMessage `json:"error,omitempty"`
}

func newFront() *front {
	return &front{outgoing: newQueue(), docs: map[string]string{}, pending: map[string]request{}, encoding: "utf-16"}
}

// queue is an unbounded FIFO of message bodies.
type queue struct {
	mu     sync.Mutex
	ready  *sync.Cond
	bodies [][]byte
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.ready = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(body []byte) {
	q.mu.Lock()
	q.bodies = append(q.bodies, body)
	q.mu.Unlock()
	q.ready.Signal()
}

func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.ready.Signal()
}

// drain writes the queue to w until it is closed and empty.
func (q *queue) drain(w io.Writer) error {
	for {
		q.mu.Lock()
		for len(q.bodies) == 0 && !q.closed {
			q.ready.Wait()
		}
		if len(q.bodies) == 0 {
			q.mu.Unlock()
			return nil
		}
		body := q.bodies[0]
		q.bodies = q.bodies[1:]
		q.mu.Unlock()
		if err := writeFrame(w, body); err != nil {
			return err
		}
	}
}

func isRTSX(uri string) bool { return strings.HasSuffix(uri, ".rtsx") }

// sourceName names a document for the source parse, which wants an absolute
// name and reads nothing from it: a document that is not a file has none.
func sourceName(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return uri
	}
	return "/untitled.rtsx"
}

// readFrame reads one LSP message body.
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			if length, err = strconv.Atoi(strings.TrimSpace(v)); err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("lsp: message without Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeFrame(w io.Writer, body []byte) error {
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func (f *front) toClient(body []byte) error {
	f.outgoing.push(body)
	return nil
}

func (f *front) reply(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	r := json.RawMessage(raw)
	body, err := json.Marshal(message{JSONRPC: "2.0", ID: &id, Result: &r})
	if err != nil {
		return err
	}
	return f.toClient(body)
}

// fromClient reads the client's messages; those it does not answer itself
// go on to the server.
func (f *front) fromClient(in io.Reader, toServer io.WriteCloser) error {
	defer toServer.Close()
	r := bufio.NewReader(in)
	for {
		body, err := readFrame(r)
		if err != nil {
			return err
		}
		body = f.names.fromClient(body)
		var msg message
		if json.Unmarshal(body, &msg) == nil && msg.Method != "" {
			handled, err := f.clientMessage(msg)
			if err != nil {
				return err
			}
			if handled {
				continue
			}
		}
		if err := writeFrame(toServer, body); err != nil {
			return err
		}
	}
}

type textDocument struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

// clientMessage tracks the open .rtsx documents and answers the syntactic
// requests for them.
func (f *front) clientMessage(msg message) (handled bool, err error) {
	var doc textDocument
	json.Unmarshal(msg.Params, &doc)
	uri := doc.TextDocument.URI
	f.mu.Lock()
	text, open := f.docs[uri]
	encoding, lineFoldingOnly := f.encoding, f.lineFoldingOnly
	f.mu.Unlock()

	switch msg.Method {
	case "initialize":
		var p struct {
			Capabilities struct {
				TextDocument struct {
					FoldingRange struct {
						LineFoldingOnly bool `json:"lineFoldingOnly"`
					} `json:"foldingRange"`
				} `json:"textDocument"`
			} `json:"capabilities"`
		}
		json.Unmarshal(msg.Params, &p)
		f.mu.Lock()
		f.lineFoldingOnly = p.Capabilities.TextDocument.FoldingRange.LineFoldingOnly
		f.mu.Unlock()
		f.await(msg)
	case "workspace/symbol", "workspace/willRenameFiles":
		f.await(msg)
	case "textDocument/didOpen":
		if isRTSX(uri) {
			f.mu.Lock()
			f.docs[uri] = doc.TextDocument.Text
			f.mu.Unlock()
		}
	case "textDocument/didChange":
		if open {
			var p struct {
				ContentChanges []struct {
					Range *struct{ Start, End struct{ Line, Character int } } `json:"range"`
					Text  string                                              `json:"text"`
				} `json:"contentChanges"`
			}
			json.Unmarshal(msg.Params, &p)
			for _, change := range p.ContentChanges {
				if change.Range == nil {
					text = change.Text // the sync kind we advertise: the whole document
					continue
				}
				start := offsetAt(text, change.Range.Start.Line, change.Range.Start.Character, encoding)
				end := offsetAt(text, change.Range.End.Line, change.Range.End.Character, encoding)
				text = text[:start] + change.Text + text[end:]
			}
			f.mu.Lock()
			f.docs[uri] = text
			f.mu.Unlock()
		}
	case "textDocument/didClose":
		f.mu.Lock()
		delete(f.docs, uri)
		f.mu.Unlock()
	case "textDocument/foldingRange":
		if open && msg.ID != nil {
			result, err := server.NewSyntactic(sourceName(uri), text, encoding).FoldingRanges(lineFoldingOnly)
			if err != nil {
				return false, nil
			}
			return true, f.reply(*msg.ID, json.RawMessage(result))
		}
	case "textDocument/selectionRange":
		if open && msg.ID != nil {
			var p struct {
				Positions json.RawMessage `json:"positions"`
			}
			json.Unmarshal(msg.Params, &p)
			result, err := server.NewSyntactic(sourceName(uri), text, encoding).SelectionRanges(p.Positions)
			if err != nil {
				return false, nil
			}
			return true, f.reply(*msg.ID, json.RawMessage(result))
		}
	case "textDocument/_vs_onAutoInsert":
		var p struct {
			Doc struct {
				URI string `json:"uri"`
			} `json:"_vs_textDocument"`
			Position struct{ Line, Character int } `json:"_vs_position"`
			Ch       string                        `json:"_vs_ch"`
		}
		json.Unmarshal(msg.Params, &p)
		f.mu.Lock()
		text, open = f.docs[p.Doc.URI]
		f.mu.Unlock()
		if open && msg.ID != nil {
			var result any
			if closing := server.NewSyntactic(sourceName(p.Doc.URI), text, encoding).ClosingTag(p.Position.Line, p.Position.Character); p.Ch == ">" && closing != "" {
				at := map[string]int{"line": p.Position.Line, "character": p.Position.Character}
				result = map[string]any{
					"_vs_textEditFormat": 2, // a snippet: `$0` keeps the cursor before the tag
					"_vs_textEdit":       map[string]any{"range": map[string]any{"start": at, "end": at}, "newText": "$0" + snippetText(closing)},
				}
			}
			return true, f.reply(*msg.ID, result)
		}
	}
	return false, nil
}

// await remembers a forwarded request whose answer is rewritten.
func (f *front) await(msg message) {
	if msg.ID == nil {
		return
	}
	f.mu.Lock()
	f.pending[string(*msg.ID)] = request{Method: msg.Method, Params: msg.Params}
	f.mu.Unlock()
}

// fromServer passes the server's messages to the client, rewriting the
// answers that need it.
func (f *front) fromServer(out io.Reader) error {
	r := bufio.NewReader(out)
	for {
		body, err := readFrame(r)
		if err != nil {
			return err
		}
		body = f.names.fromServer(body)
		var msg message
		if json.Unmarshal(body, &msg) == nil && msg.Method == "" && msg.ID != nil && msg.Result != nil {
			f.mu.Lock()
			req, ok := f.pending[string(*msg.ID)]
			delete(f.pending, string(*msg.ID))
			f.mu.Unlock()
			if ok {
				if result, changed := f.answer(req, *msg.Result); changed {
					msg.Result = &result
					if rewritten, err := json.Marshal(msg); err == nil {
						body = rewritten
					}
				}
			}
		}
		if err := f.toClient(body); err != nil {
			return err
		}
	}
}

// answer rewrites the server's result for req.
func (f *front) answer(req request, result json.RawMessage) (json.RawMessage, bool) {
	switch req.Method {
	case "initialize":
		var r map[string]json.RawMessage
		var caps map[string]any
		if json.Unmarshal(result, &r) != nil || json.Unmarshal(r["capabilities"], &caps) != nil {
			return nil, false
		}
		if encoding, ok := caps["positionEncoding"].(string); ok {
			f.mu.Lock()
			f.encoding = encoding
			f.mu.Unlock()
		}
		// Whole documents on change: this front keeps each .rtsx text.
		if sync, ok := caps["textDocumentSync"].(map[string]any); ok {
			sync["change"] = 1
		}
		// File rename: .rtsx too — no other server knows these modules.
		caps["workspace"] = map[string]any{"fileOperations": map[string]any{"willRename": map[string]any{"filters": []any{
			map[string]any{"scheme": "file", "pattern": map[string]any{"glob": "**/*.{ts,tsx,js,jsx,cts,cjs,mts,mjs,json,rtsx}"}},
		}}}}
		r["capabilities"], _ = json.Marshal(caps)
		out, err := json.Marshal(r)
		return out, err == nil
	case "workspace/symbol":
		// Symbols declared in .rtsx files; the rest is the user's TypeScript's.
		var symbols []json.RawMessage
		if json.Unmarshal(result, &symbols) != nil {
			return nil, false
		}
		kept := []json.RawMessage{}
		for _, s := range symbols {
			var symbol struct {
				Location struct {
					URI string `json:"uri"`
				} `json:"location"`
			}
			if json.Unmarshal(s, &symbol) == nil && isRTSX(symbol.Location.URI) {
				kept = append(kept, s)
			}
		}
		out, err := json.Marshal(kept)
		return out, err == nil
	case "workspace/willRenameFiles":
		// A renamed .rtsx: every importer. A renamed .ts/.tsx: the .rtsx
		// importers only — the user's TypeScript updates the rest.
		var p struct {
			Files []struct {
				OldURI string `json:"oldUri"`
			} `json:"files"`
		}
		json.Unmarshal(req.Params, &p)
		for _, file := range p.Files {
			if isRTSX(file.OldURI) {
				return nil, false
			}
		}
		var edit struct {
			Changes         map[string]json.RawMessage `json:"changes,omitempty"`
			DocumentChanges []json.RawMessage          `json:"documentChanges,omitempty"`
		}
		if json.Unmarshal(result, &edit) != nil {
			return nil, false
		}
		for uri := range edit.Changes {
			if !isRTSX(uri) {
				delete(edit.Changes, uri)
			}
		}
		var kept []json.RawMessage
		for _, change := range edit.DocumentChanges {
			var doc textDocument
			if json.Unmarshal(change, &doc) == nil && isRTSX(doc.TextDocument.URI) {
				kept = append(kept, change)
			}
		}
		edit.DocumentChanges = kept
		out, err := json.Marshal(edit)
		return out, err == nil
	}
	return nil, false
}

// offsetAt is the byte offset of an LSP position in text.
func offsetAt(text string, line, character int, encoding string) int {
	offset := 0
	for ; line > 0; line-- {
		i := strings.IndexByte(text[offset:], '\n')
		if i < 0 {
			return len(text)
		}
		offset += i + 1
	}
	if encoding == "utf-8" {
		return min(offset+character, len(text))
	}
	units := 0
	for i, r := range text[offset:] {
		if units >= character || r == '\n' {
			return offset + i
		}
		units++
		if r > 0xFFFF {
			units++
		}
	}
	return len(text)
}

// snippetText escapes text for an LSP snippet: tag names may hold `$`.
func snippetText(s string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`).Replace(s)
}
