package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// These tests look inside the front; the scenarios of ide.md's feature
// table are in the external test package.

var frontApp = map[string]string{
	"src/util.ts": "export const twice = (n: number) => n * 2;\n",
	"src/page.rtsx": `import { twice } from "./util";
export function Page() {
  const count = twice(2);
  return <main title={"" + count}><b>{count}</b></main>;
}
`,
	"src/main.tsx": `import { Page } from "./page";
import { twice } from "./util";
export const app = <Page />;
export const four = twice(2);
`,
}

// startFront is a session whose front the test holds.
func startFront(t *testing.T, files map[string]string, options lsptest.Options) (*lsptest.Client, *front) {
	t.Helper()
	f := newFront(io.Discard)
	serve := func(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
		return f.serve(ctx, in, out, Options{Log: log, Cwd: cwd, Version: "0.0.0-test"})
	}
	return lsptest.StartWith(t, lsptest.Project(t, files), serve, options), f
}

func (f *front) awaited() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pending)
}

// A request whose answer the front rewrites is forgotten when the answer
// comes, whatever it is: a result, an error, null. (`initialize` is the one
// request awaited today, and a server answers it with a result — so the
// answers are fed to the front here.)
func TestAwaitedRequestIsForgotten(t *testing.T) {
	for name, answer := range map[string]string{
		"a result": `{"jsonrpc":"2.0","id":7,"result":{"capabilities":{"textDocumentSync":{"change":2}}}}`,
		"an error": `{"jsonrpc":"2.0","id":7,"error":{"code":-32800,"message":"cancelled"}}`,
		"null":     `{"jsonrpc":"2.0","id":7,"result":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFront(io.Discard)
			id := json.RawMessage("7")
			f.await(message{ID: &id, Method: "initialize"})
			if n := f.awaited(); n != 1 {
				t.Fatalf("%d requests awaited after one was forwarded", n)
			}
			f.fromServer(strings.NewReader(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(answer), answer)))
			if n := f.awaited(); n != 0 {
				t.Errorf("%d requests still awaited after the answer", n)
			}
			// The answer went on to the client: rewritten if it is a result.
			f.outgoing.close()
			var sent strings.Builder
			f.outgoing.drain(&sent)
			if got := sent.String(); !strings.Contains(got, `"id":7`) || (name == "a result") != strings.Contains(got, `"change":1`) || (name == "an error") != strings.Contains(got, "-32800") {
				t.Errorf("sent to the client: %q", got)
			}
		})
	}
}

// Over a session — errors, null results, cancelled requests — nothing stays
// awaited.
func TestNoRequestStaysAwaited(t *testing.T) {
	c, f := startFront(t, frontApp, lsptest.Options{})
	c.Open("src/page.rtsx")
	c.Hover("src/page.rtsx", c.At("src/page.rtsx", "count", 1, 1))
	if n := f.awaited(); n != 0 {
		t.Fatalf("%d requests awaited after initialize", n)
	}
	for i := 0; i < 20; i++ {
		// An error: the query is not a string.
		if err := c.Try("workspace/symbol", map[string]any{"query": 42}, nil); err == nil {
			t.Fatal("workspace/symbol with a number as its query: no error")
		}
		// null: a rename with nothing to edit.
		var edit json.RawMessage
		c.Request("workspace/willRenameFiles", map[string]any{"files": []any{map[string]any{"oldUri": c.URI("src/nothing.ts"), "newUri": c.URI("src/nothing2.ts")}}}, &edit)
		if string(edit) != "null" && len(edit) != 0 {
			t.Fatalf("renaming a file nobody imports: %s", edit)
		}
	}
	if n := f.awaited(); n != 0 {
		t.Errorf("%d requests awaited after 20 errors and 20 null results", n)
	}
	// Cancelled, as the symbol picker does on every keystroke: the server
	// answers all the same — an error, or the result it already had, which
	// is narrowed as any other.
	for i := 0; i < 20; i++ {
		answer := c.Start("workspace/symbol", map[string]any{"query": "t"})
		c.Notify("$/cancelRequest", map[string]any{"id": answer.ID})
		var symbols []struct {
			Location lsptest.Location `json:"location"`
		}
		if err := answer.Wait(&symbols); err != nil && !strings.Contains(err.Error(), "-32800") {
			t.Errorf("a cancelled workspace/symbol: %v", err)
		}
		for _, s := range symbols {
			if !strings.HasSuffix(s.Location.URI, ".rtsx") {
				t.Errorf("a cancelled workspace/symbol answered with a symbol of %s", c.Rel(s.Location.URI))
			}
		}
	}
	if n := f.awaited(); n != 0 {
		t.Errorf("%d requests awaited after 20 cancelled ones", n)
	}
}

// The front asks for whole documents. A client that sends a range anyway is
// served: the front's copy changes as the server's does — the server's line
// map (a lone \r ends a line), its clamping, either position encoding.
func TestRangedChange(t *testing.T) {
	for _, encoding := range []string{"utf-16", "utf-8"} {
		t.Run(encoding, func(t *testing.T) {
			c, f := startFront(t, frontApp, lsptest.Options{Capabilities: func(capabilities map[string]any) {
				capabilities["general"] = map[string]any{"positionEncodings": []string{encoding}}
				capabilities["textDocument"].(map[string]any)["documentSymbol"] = map[string]any{}
			}})
			var sync struct {
				Change int `json:"change"`
			}
			json.Unmarshal(c.Initialized.Capabilities["textDocumentSync"], &sync)
			if got := string(c.Initialized.Capabilities["positionEncoding"]); got != `"`+encoding+`"` || sync.Change != 1 {
				t.Fatalf("position encoding %s, sync kind %d; want %q and 1 (whole documents)", got, sync.Change, encoding)
			}
			const page = "src/page.rtsx"
			text := "const s = \"\U0001F600\"; const a = 1;\rexport const b = <div>\r\n  x\r\n</div>;\r\n"
			c.OpenAs(page, text)
			uri := c.URI(page)
			change := func(version, line, from, to int, text string) {
				c.Notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}, "contentChanges": []any{
					map[string]any{"range": map[string]any{"start": map[string]int{"line": line, "character": from}, "end": map[string]int{"line": line, "character": to}}, "text": text},
				}})
			}
			// `c` after `a`, behind an astral character: 4 bytes, 2 units.
			character := strings.Index(text, " = 1")
			if encoding == "utf-16" {
				character -= 2
			}
			change(2, 0, character, character, "c")
			// `b` → `bee` on line 1, which is one only if the lone \r ends line 0.
			change(3, 1, 13, 14, "bee")
			// Far past the end of line 2 (`  x`, then \r\n): clamped by the
			// server to the start of the next line, so by the front.
			change(4, 2, 99, 99, "{s}")
			want := "const s = \"\U0001F600\"; const ac = 1;\rexport const bee = <div>\r\n  x\r\n{s}</div>;\r\n"

			// The server's copy: what TypeScript finds at each edited place —
			// a line or a character off, and it is another token, or none.
			for _, probe := range []struct {
				line, character int
				hover           string
			}{
				{0, character, "const ac: 1"},
				{1, 14, "const bee:"},
				{3, 1, "const s:"}, // after the line break, not before it
			} {
				if hover := c.Hover(page, lsptest.Position{Line: probe.line, Character: probe.character}); !strings.Contains(hover, probe.hover) {
					t.Errorf("the server's copy: hover at %d:%d is %q, want %q", probe.line+1, probe.character+1, hover, probe.hover)
				}
			}
			if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
				t.Errorf("the server's diagnostics after the changes: %q", got)
			}
			// The front's copy: its own answer, the symbols, and the text.
			var symbols []struct {
				Name string `json:"name"`
			}
			c.Request("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}}, &symbols)
			names := ""
			for _, s := range symbols {
				names += s.Name + " "
			}
			if names != "s ac bee " {
				t.Errorf("the front's symbols after the changes: %q", names)
			}
			f.mu.Lock()
			got := f.docs[uri]
			f.mu.Unlock()
			if got != want {
				t.Errorf("the front's copy\n got %q\nwant %q", got, want)
			}
		})
	}
}

// A panic in one of the front's own handlers answers that request with an
// InternalError; the process — the whole server — goes on.
func TestFrontRecovers(t *testing.T) {
	c, _ := startFront(t, frontApp, lsptest.Options{})
	const page = "src/page.rtsx"
	c.Open(page)
	doc := map[string]any{"uri": c.URI(page)}
	parse := newSyntactic
	defer func() { newSyntactic = parse }()
	newSyntactic = func(text, encoding string) *server.Syntactic { panic("a bug in the front") }
	for method, params := range map[string]map[string]any{
		"textDocument/foldingRange":     {"textDocument": doc},
		"textDocument/selectionRange":   {"textDocument": doc, "positions": []any{lsptest.Position{}}},
		"textDocument/_vs_onAutoInsert": {"_vs_textDocument": doc, "_vs_position": lsptest.Position{}, "_vs_ch": ">"},
	} {
		err := c.Try(method, params, nil)
		if err == nil || !strings.Contains(err.Error(), "-32603") || !strings.Contains(err.Error(), "a bug in the front") {
			t.Errorf("%s: %v; want an InternalError (-32603) naming the panic", method, err)
		}
	}
	newSyntactic = parse
	var folds []json.RawMessage
	c.Request("textDocument/foldingRange", map[string]any{"textDocument": doc}, &folds)
	if hover := c.Hover(page, c.At(page, "count", 1, 1)); len(folds) == 0 || !strings.Contains(hover, "count: number") {
		t.Errorf("after the panics: %d folds, hover %q", len(folds), hover)
	}
}

// The server ends when the editor's process is gone, though its input stays
// open (another process holds the pipe): the process named on the command
// line, else the one named in initialize.
func TestParentWatchdog(t *testing.T) {
	if !processAliveSupported {
		t.Skip("no process probing on this platform")
	}
	interval := watchdogInterval
	watchdogInterval = 20 * time.Millisecond
	defer func() { watchdogInterval = interval }()
	root := lsptest.Project(t, frontApp)
	for _, fromFlag := range []bool{true, false} {
		t.Run(fmt.Sprintf("--clientProcessId=%v", fromFlag), func(t *testing.T) {
			editor := exec.Command("sleep", "60")
			if err := editor.Start(); err != nil {
				t.Skip(err)
			}
			defer editor.Process.Kill()
			pid := editor.Process.Pid
			o := Options{Log: io.Discard, Cwd: root, Version: "0.0.0-test"}
			var processID any // initialize's
			if fromFlag {
				o.ClientProcessID = pid
			} else {
				processID = pid
			}
			conn := lsptest.Connect(t, root, func(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
				return Serve(ctx, in, out, o)
			})
			conn.Send(1, "initialize", map[string]any{"processId": processID, "rootUri": "file://" + root, "capabilities": map[string]any{}})
			conn.Answer(1)
			conn.Send(0, "initialized", map[string]any{})
			time.Sleep(5 * watchdogInterval)
			conn.Send(2, "workspace/symbol", map[string]any{"query": "Page"})
			if answer := conn.Answer(2); answer.Error != nil {
				t.Fatalf("with the editor alive: %+v", answer.Error)
			}
			editor.Process.Kill()
			editor.Wait() // reaped: a zombie still answers signal 0
			err := conn.Ended(10 * time.Second)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("client process %d has exited", pid)) {
				t.Errorf("Serve returned %v", err)
			}
		})
	}
}

// The editor is killed with answers it has not read, and whoever holds its
// end of the pipes neither reads nor closes them. The watchdog still ends
// the server: it does not wait for a queue that cannot drain.
func TestParentWatchdogWithUnreadAnswers(t *testing.T) {
	if !processAliveSupported {
		t.Skip("no process probing on this platform")
	}
	interval, drain := watchdogInterval, drainTimeout
	watchdogInterval, drainTimeout = 20*time.Millisecond, 100*time.Millisecond
	defer func() { watchdogInterval, drainTimeout = interval, drain }()
	root := lsptest.Project(t, frontApp)
	editor := exec.Command("sleep", "60")
	if err := editor.Start(); err != nil {
		t.Skip(err)
	}
	defer editor.Process.Kill()
	pid := editor.Process.Pid

	in, client := io.Pipe()
	answers, out := io.Pipe() // never read: a pipe that is full from the first byte
	done := make(chan error, 1)
	go func() {
		done <- Serve(context.Background(), in, out, Options{Log: io.Discard, Cwd: root, Version: "0.0.0-test"})
	}()
	defer func() { // only now does the pipe's holder let go
		client.Close()
		answers.Close()
	}()
	send := func(message string) {
		fmt.Fprintf(client, "Content-Length: %d\r\n\r\n%s", len(message), message)
	}
	uri := "file://" + root + "/src/page.rtsx"
	text, _ := json.Marshal(frontApp["src/page.rtsx"])
	send(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":%d,"rootUri":"file://%s","capabilities":{}}}`, pid, root))
	send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"` + uri + `","languageId":"rtsx","version":1,"text":` + string(text) + `}}}`)
	for id := 2; id < 5; id++ {
		send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"%s"}}}`, id, uri))
	}
	time.Sleep(5 * watchdogInterval) // the watchdog is running, the answers queued
	select {
	case err := <-done:
		t.Fatalf("with the editor alive, Serve returned %v", err)
	default:
	}
	editor.Process.Kill()
	editor.Wait()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("client process %d has exited", pid)) {
			t.Errorf("Serve returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the server is still running 10s after the editor's process was killed")
	}
}

// Serve leaves nothing running: over sessions that end by `exit` with the
// input still open — a process's stdin — the goroutines do not add up.
func TestServeLeavesNoGoroutines(t *testing.T) {
	root := lsptest.Project(t, frontApp)
	uri := "file://" + root + "/src/page.rtsx"
	session := func() {
		in, client := io.Pipe()
		answers, out := io.Pipe()
		defer client.Close() // only after Serve returned
		go io.Copy(io.Discard, answers)
		done := make(chan error, 1)
		go func() {
			done <- Serve(context.Background(), in, out, Options{Log: io.Discard, Cwd: root, Version: "0.0.0-test"})
			out.Close()
		}()
		send := func(message string) {
			fmt.Fprintf(client, "Content-Length: %d\r\n\r\n%s", len(message), message)
		}
		send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":"file://` + root + `","capabilities":{}}}`)
		send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
		text, _ := json.Marshal(frontApp["src/page.rtsx"])
		send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"` + uri + `","languageId":"rtsx","version":1,"text":` + string(text) + `}}}`)
		send(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"` + uri + `"},"position":{"line":2,"character":9}}}`)
		send(`{"jsonrpc":"2.0","id":3,"method":"shutdown"}`)
		send(`{"jsonrpc":"2.0","method":"exit"}`)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Serve returned %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Serve did not return after exit")
		}
	}
	session()              // what the first session starts for good (caches, the mapper) is not a leak
	settle := func() int { // the lowest count, once it has stopped falling
		n := runtime.NumGoroutine()
		for stable := 0; stable < 10; {
			time.Sleep(20 * time.Millisecond)
			if m := runtime.NumGoroutine(); m < n {
				n, stable = m, 0
			} else {
				stable++
			}
		}
		return n
	}
	before := settle()
	const sessions = 20
	for i := 0; i < sessions; i++ {
		session()
	}
	after := settle()
	if after > before+sessions/2 { // a leak is at least one goroutine per session
		buf := make([]byte, 1<<20)
		t.Errorf("goroutines: %d before, %d after %d sessions\n%s", before, after, sessions, buf[:runtime.Stack(buf, true)])
	}
	t.Logf("goroutines: %d before, %d after %d sessions", before, after, sessions)
}
