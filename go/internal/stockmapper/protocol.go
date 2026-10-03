// Package stockmapper is `reactogenic content-mapper`
// (specs/phase01/ide.md, *Stock TypeScript 7.1*): the .rtsx transform as a
// standard TypeScript 7.1 content mapper, a child process of `tsc
// --runExternalCode` or of the TypeScript 7.1 language server.
//
// The protocol is the host's (the fork's internal/contentmapper/hostimpl.go):
// JSON-RPC 2.0 in LSP `Content-Length` framing on stdin and stdout, four
// requests, all sent by the host — initialize, openProject, transform,
// closeProject. This file is the protocol, on the standard library alone;
// transform.go is the transform.
package stockmapper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DiagnosticSource prefixes the codes of the mapper's diagnostics:
// `error reactogenic101: orphan-slot: …`.
const DiagnosticSource = "reactogenic"

// Options are the mapper's surroundings.
type Options struct {
	// ReadFile and FileExists read the files next to a transformed one:
	// segments, import targets. nil: the disk.
	ReadFile   func(path string) (string, bool)
	FileExists func(path string) bool
	// Log receives what went wrong — a recovered panic, a message that is
	// not JSON. Never stdout: that is the protocol's. nil: nowhere.
	Log io.Writer

	// beforeTransform runs first in every transform; tests make it panic.
	beforeTransform func(fileName string)
}

// eofGrace is how long transforms still running at the end of input get to
// answer. The host has gone by then; a mapper that lingers is an orphan.
const eofGrace = 2 * time.Second

// Serve answers the host's requests on in and out until in ends. Transforms
// run concurrently and are answered in any order, each exactly once: an
// unanswered request hangs the host, and a mapper that dies is not started
// again for the rest of the host's session.
func Serve(in io.Reader, out io.Writer, opts Options) error {
	s := newServer(out, opts)
	reader := bufio.NewReader(in)
	for {
		body, err := readMessage(reader)
		if err != nil {
			s.wait()
			if err == io.EOF {
				return nil
			}
			return err
		}
		var msg message
		if err := json.Unmarshal(body, &msg); err != nil {
			s.logf("not a JSON-RPC message: %v", err)
			continue
		}
		if len(msg.ID) == 0 || msg.Method == "" {
			continue // a notification, or a response: the mapper sends no requests
		}
		s.handle(msg)
	}
}

type server struct {
	opts Options

	writeMu sync.Mutex
	out     *bufio.Writer

	pending sync.WaitGroup

	mu       sync.Mutex
	projects map[string]*project // by projectHandle
}

func newServer(out io.Writer, opts Options) *server {
	if opts.ReadFile == nil {
		opts.ReadFile = func(path string) (string, bool) {
			b, err := os.ReadFile(path)
			return string(b), err == nil
		}
	}
	if opts.FileExists == nil {
		opts.FileExists = func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		}
	}
	return &server{opts: opts, out: bufio.NewWriter(out), projects: map[string]*project{}}
}

// message is a request: the id is echoed byte for byte (the host's ids are
// strings such as "api3"; a number in their place is never matched).
type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC's error codes.
const (
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

type initializeResult struct {
	PositionEncoding string `json:"positionEncoding"`
	DiagnosticSource string `json:"diagnosticSource"`
}

type openProjectParams struct {
	ConfigFileName  string          `json:"configFileName"`
	ProjectHandle   string          `json:"projectHandle"`
	CompilerOptions json.RawMessage `json:"compilerOptions"`
}

type closeProjectParams struct {
	ProjectHandle string `json:"projectHandle"`
}

type transformParams struct {
	FileName      string `json:"fileName"`
	Content       string `json:"content"`
	ProjectHandle string `json:"projectHandle"`
}

func (s *server) handle(msg message) {
	switch msg.Method {
	case "initialize":
		// Offsets are bytes: what the transpiler's map counts, and what the
		// host converts to anyway. It offers utf-8 always. (Neither encoding
		// serves a source that is not valid UTF-8: the host's JSON carries
		// U+FFFD for the bad byte, so the copied text no longer equals the
		// original it is checked against — ide.md, the limits.)
		s.reply(msg.ID, initializeResult{PositionEncoding: "utf-8", DiagnosticSource: DiagnosticSource}, nil)
	case "openProject":
		var p openProjectParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			s.reply(msg.ID, nil, &rpcError{Code: errInvalidParams, Message: err.Error()})
			return
		}
		s.mu.Lock()
		s.projects[p.ProjectHandle] = newProject(p)
		s.mu.Unlock()
		s.reply(msg.ID, struct{}{}, nil) // static configuration: no identity, no watched files
	case "closeProject":
		var p closeProjectParams
		if err := json.Unmarshal(msg.Params, &p); err == nil {
			s.mu.Lock()
			delete(s.projects, p.ProjectHandle)
			s.mu.Unlock()
		}
		s.reply(msg.ID, nil, nil)
	case "transform":
		s.pending.Go(func() {
			// The last line of defence: the transform recovers by itself
			// and still answers with a file; this answers at all.
			defer func() {
				if r := recover(); r != nil {
					s.logf("panic answering %s: %v\n%s", msg.ID, r, debug.Stack())
					s.reply(msg.ID, nil, &rpcError{Code: errInternal, Message: fmt.Sprint(r)})
				}
			}()
			var p transformParams
			if err := json.Unmarshal(msg.Params, &p); err != nil {
				s.reply(msg.ID, nil, &rpcError{Code: errInvalidParams, Message: err.Error()})
				return
			}
			s.mu.Lock()
			project := s.projects[p.ProjectHandle]
			s.mu.Unlock()
			s.reply(msg.ID, s.transform(p, project), nil)
		})
	default:
		s.reply(msg.ID, nil, &rpcError{Code: errMethodNotFound, Message: "method not found: " + msg.Method})
	}
}

// reply writes one response. A nil result with no error is `null`.
func (s *server) reply(id json.RawMessage, result any, rpcErr *rpcError) {
	response := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *rpcError       `json:"error,omitempty"`
	}{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		raw, err := json.Marshal(result)
		if err != nil {
			response.Error = &rpcError{Code: errInternal, Message: err.Error()}
		} else {
			response.Result = raw
		}
	}
	body, err := json.Marshal(response)
	if err != nil {
		s.logf("response %s: %v", id, err)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(body))
	s.out.Write(body)
	if err := s.out.Flush(); err != nil {
		s.logf("write: %v", err)
	}
}

// wait lets running transforms answer, for a while.
func (s *server) wait() {
	done := make(chan struct{})
	go func() {
		s.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(eofGrace):
	}
}

func (s *server) logf(format string, args ...any) {
	if s.opts.Log != nil {
		fmt.Fprintf(s.opts.Log, "reactogenic content-mapper: "+format+"\n", args...)
	}
}

// maxMessage bounds a message: a source file and its path. A length above it
// is not a message — and one that fits an int but no slice would be a panic.
const maxMessage = 1 << 30

// readMessage reads one framed message: headers, a blank line, then
// Content-Length bytes.
func readMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for headers := 0; ; headers++ {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && (line != "" || headers > 0) {
				return nil, io.ErrUnexpectedEOF // the input ends inside a message
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if length < 0 {
				return nil, fmt.Errorf("content mapper protocol: a message without Content-Length")
			}
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue // Content-Type, or a header of a later protocol
		}
		length, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil || length < 0 || length > maxMessage {
			return nil, fmt.Errorf("content mapper protocol: Content-Length %q", strings.TrimSpace(value))
		}
	}
	// The header is a claim, not an allocation: beyond a megabyte the body
	// grows as its bytes arrive.
	var body bytes.Buffer
	body.Grow(min(length, 1<<20))
	if _, err := io.CopyN(&body, r, int64(length)); err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return body.Bytes(), nil
}
