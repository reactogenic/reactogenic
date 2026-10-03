package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"
)

// front sits between the client and the fork's server, in this process. It
// answers what TypeScript cannot answer from the virtual text — the
// syntactic features, from the .rtsx source tree (ide.md, *Span map*) — and
// for that keeps the text of each open .rtsx document. It also ends the
// session: `exit` is its own, so that it works whatever state the server is
// in. (What the server says about files that are not ours is narrowed in
// the server itself: lsp.Embedder.Owns.)
type front struct {
	// outgoing queues the messages for the client. Nothing here may block on
	// writing to the client: the client may itself be blocked writing to us,
	// and each side would wait for the other to read.
	outgoing *queue
	log      io.Writer

	// names renames the documents that are rtsx by language id only.
	names aliases

	mu              sync.Mutex
	docs            map[string]string      // open .rtsx documents, by URI
	sources         map[string]*sourceTree // what was read off their texts, as far as asked for
	pending         map[string]request     // forwarded requests whose answers we rewrite, by id
	encoding        string                 // position encoding, from the initialize result
	lineFoldingOnly bool
	symbolTree      bool  // the client takes document symbols as a tree
	definitionLinks bool  // and definitions as links
	shutdown        bool  // the client asked for shutdown
	ended           error // why fromClient returned: nil (its input ended), errExit, or a framing error
}

// errExit is the client's `exit` notification.
var errExit = errors.New("exit")

// newSyntactic parses a document for the features answered here (a variable
// for the tests of the recover in clientMessage).
var newSyntactic = server.NewSyntactic

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *json.RawMessage `json:"error,omitempty"`
}

func newFront(log io.Writer) *front {
	return &front{outgoing: newQueue(), log: log, docs: map[string]string{}, sources: map[string]*sourceTree{}, pending: map[string]request{}, encoding: "utf-16"}
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

// readFrame reads one LSP message body. Its error is io.EOF only when the
// input ends between two messages.
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for first := true; ; first = false {
		line, err := r.ReadString('\n')
		if err == io.EOF && (!first || line != "") {
			return nil, fmt.Errorf("the input ends inside a message header")
		}
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if name, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "Content-Length") {
			if length, err = strconv.Atoi(strings.TrimSpace(v)); err != nil || length < 0 {
				return nil, fmt.Errorf("bad Content-Length %q", strings.TrimSpace(v))
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("a message header without Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("the input ends inside a message body (%d bytes announced)", length)
	}
	return body, nil
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

// replyError answers a request with an error.
func (f *front) replyError(id json.RawMessage, code int, text string) {
	raw, _ := json.Marshal(map[string]any{"code": code, "message": text})
	r := json.RawMessage(raw)
	if body, err := json.Marshal(message{JSONRPC: "2.0", ID: &id, Error: &r}); err == nil {
		f.toClient(body)
	}
}

// fromClient reads the client's messages; those it does not answer itself
// go on to the server. It returns at `exit` (errExit), when the input ends
// (nil) or cannot be read as LSP messages; what it returns is recorded as
// f.ended before the server's input is closed, so that whoever sees the
// server stop for that reason finds it there.
func (f *front) fromClient(in io.Reader, toServer io.WriteCloser) {
	err := f.readClient(in, toServer)
	f.mu.Lock()
	f.ended = err
	f.mu.Unlock()
	toServer.Close()
}

func (f *front) readClient(in io.Reader, toServer io.Writer) error {
	r := bufio.NewReader(in)
	for {
		body, err := readFrame(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		body = f.names.fromClient(body)
		var msg message
		if json.Unmarshal(body, &msg) == nil && msg.Method != "" {
			if msg.Method == "exit" {
				// Ours: the server takes `exit` only once it is initialized,
				// and ends when its input does.
				return errExit
			}
			handled, forward := f.clientMessage(msg)
			if handled {
				continue
			}
			if forward != nil {
				body = forward
			}
		}
		if err := writeFrame(toServer, body); err != nil {
			return nil // the server is gone: Serve is returning
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
// requests for them; handled means the message does not go on to the server,
// and forward, when not nil, is what goes on in its place.
// A panic here must not end the process, which is the whole server: the
// request is answered with an InternalError, a notification goes on.
func (f *front) clientMessage(msg message) (handled bool, forward []byte) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(f.log, "reactogenic lsp: panic handling %s: %v\n%s\n", msg.Method, r, debug.Stack())
			forward = nil
			if handled = msg.ID != nil; handled {
				f.replyError(*msg.ID, -32603, fmt.Sprintf("InternalError: panic handling request %s: %v", msg.Method, r))
			}
		}
	}()
	var doc textDocument
	json.Unmarshal(msg.Params, &doc)
	uri := doc.TextDocument.URI
	f.mu.Lock()
	text, open := f.docs[uri]
	encoding, lineFoldingOnly, symbolTree := f.encoding, f.lineFoldingOnly, f.symbolTree
	f.mu.Unlock()

	switch msg.Method {
	case "initialize":
		var p struct {
			Capabilities struct {
				TextDocument struct {
					FoldingRange struct {
						LineFoldingOnly bool `json:"lineFoldingOnly"`
					} `json:"foldingRange"`
					DocumentSymbol struct {
						Hierarchical bool `json:"hierarchicalDocumentSymbolSupport"`
					} `json:"documentSymbol"`
					Definition struct {
						LinkSupport bool `json:"linkSupport"`
					} `json:"definition"`
				} `json:"textDocument"`
			} `json:"capabilities"`
		}
		json.Unmarshal(msg.Params, &p)
		f.mu.Lock()
		f.lineFoldingOnly = p.Capabilities.TextDocument.FoldingRange.LineFoldingOnly
		f.symbolTree = p.Capabilities.TextDocument.DocumentSymbol.Hierarchical
		f.definitionLinks = p.Capabilities.TextDocument.Definition.LinkSupport
		f.mu.Unlock()
		f.await(msg)
	case "shutdown":
		f.mu.Lock()
		f.shutdown = true
		f.mu.Unlock()
	case "textDocument/didOpen":
		if isRTSX(uri) {
			f.mu.Lock()
			f.docs[uri] = doc.TextDocument.Text
			f.mu.Unlock()
		}
	case "textDocument/didChange":
		var p struct {
			ContentChanges []struct {
				Range json.RawMessage `json:"range"`
				Text  string          `json:"text"`
			} `json:"contentChanges"`
		}
		json.Unmarshal(msg.Params, &p)
		for _, change := range p.ContentChanges {
			if len(change.Range) == 0 || string(change.Range) == "null" {
				text = change.Text // the sync kind we advertise: the whole document
				continue
			}
			// Not what we advertise, but the server takes a ranged change,
			// so this copy must: with the server's own line map.
			changed, err := server.ApplyChange(text, encoding, change.Range, change.Text)
			if err != nil {
				// A range no document has (a negative position, one beyond
				// int32). The server would refuse the notification, or die
				// of it: it goes no further, for any document, and both
				// copies stay as they were.
				fmt.Fprintf(f.log, "reactogenic lsp: textDocument/didChange for %s dropped: %v\n", uri, err)
				return true, nil
			}
			text = changed
		}
		if open {
			f.mu.Lock()
			f.docs[uri] = text
			f.mu.Unlock()
		}
	case "textDocument/didClose":
		f.mu.Lock()
		delete(f.docs, uri)
		delete(f.sources, uri)
		f.mu.Unlock()
	case "textDocument/foldingRange":
		if open && msg.ID != nil {
			result, err := newSyntactic(text, encoding).FoldingRanges(lineFoldingOnly)
			if err != nil {
				return false, nil
			}
			return f.reply(*msg.ID, json.RawMessage(result)) == nil, nil
		}
	case "textDocument/documentSymbol":
		if open && msg.ID != nil {
			result, err := newSyntactic(text, encoding).DocumentSymbols(uri, symbolTree)
			if err != nil {
				return false, nil
			}
			return f.reply(*msg.ID, json.RawMessage(result)) == nil, nil
		}
	case "textDocument/selectionRange":
		if open && msg.ID != nil {
			var p struct {
				Positions json.RawMessage `json:"positions"`
			}
			json.Unmarshal(msg.Params, &p)
			result, err := newSyntactic(text, encoding).SelectionRanges(p.Positions)
			if err != nil {
				return false, nil // the server says what is wrong with the request
			}
			return f.reply(*msg.ID, json.RawMessage(result)) == nil, nil
		}
	case "textDocument/_vs_onAutoInsert":
		var p struct {
			Doc struct {
				URI string `json:"uri"`
			} `json:"_vs_textDocument"`
			Position json.RawMessage `json:"_vs_position"`
			Ch       string          `json:"_vs_ch"`
		}
		json.Unmarshal(msg.Params, &p)
		f.mu.Lock()
		text, open = f.docs[p.Doc.URI]
		f.mu.Unlock()
		if open && msg.ID != nil {
			closing, err := newSyntactic(text, encoding).ClosingTag(p.Position)
			if err != nil {
				return false, nil
			}
			var result any
			if p.Ch == ">" && closing != "" {
				result = map[string]any{
					"_vs_textEditFormat": 2, // a snippet: `$0` keeps the cursor before the tag
					"_vs_textEdit":       map[string]any{"range": map[string]any{"start": p.Position, "end": p.Position}, "newText": "$0" + snippetText(closing)},
				}
			}
			return f.reply(*msg.ID, result) == nil, nil
		}
	case "completionItem/resolve":
		// An item the front made has nothing more to it.
		if msg.ID != nil && isOurs(msg.Params) {
			return f.reply(*msg.ID, msg.Params) == nil, nil
		}
	default:
		if positional[msg.Method] && open && msg.ID != nil {
			return f.position(msg, uri, text, encoding)
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
	f.pending[string(*msg.ID)] = request{Method: msg.Method}
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
		if json.Unmarshal(body, &msg) == nil && msg.Method == "" && msg.ID != nil {
			// Whatever the answer — a result, null, an error — the request is
			// no longer awaited. The server answers every request, a cancelled
			// one too, so nothing stays here.
			f.mu.Lock()
			req, ok := f.pending[string(*msg.ID)]
			delete(f.pending, string(*msg.ID))
			f.mu.Unlock()
			if ok && msg.Error == nil {
				result := json.RawMessage("null")
				if msg.Result != nil {
					result = *msg.Result
				}
				if result, changed := f.answer(req, result); changed {
					msg.Result = &result
					if rewritten, err := marshal(msg); err == nil {
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
		r["capabilities"], _ = json.Marshal(caps)
		out, err := json.Marshal(r)
		return out, err == nil
	}
	if req.src != nil {
		return f.sourceAnswer(req, result)
	}
	return nil, false
}

// snippetText escapes text for an LSP snippet: tag names may hold `$`.
func snippetText(s string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`).Replace(s)
}
