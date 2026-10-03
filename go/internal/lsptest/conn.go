package lsptest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Conn is a bare connection to a server, for what Client cannot do: a
// session that ends badly, input that is not LSP. It performs no handshake
// and answers no server request (declare no capability that makes the
// server ask); its input stays open until Close, as an editor's pipe does.
type Conn struct {
	t       *testing.T
	w       io.WriteCloser
	writeMu sync.Mutex
	in      chan Message
	done    chan error
}

// Message is one message from the server.
type Message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Connect runs serve with cwd as its directory.
func Connect(t *testing.T, cwd string, serve Serve) *Conn {
	t.Helper()
	toServer, clientW := io.Pipe()
	clientR, fromServer := io.Pipe()
	c := &Conn{t: t, w: clientW, in: make(chan Message, 1024), done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.done <- serve(ctx, toServer, fromServer, io.Discard, cwd)
		fromServer.Close()
	}()
	go func() {
		defer close(c.in)
		r := bufio.NewReader(clientR)
		for {
			length := 0
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if line = strings.TrimSpace(line); line == "" {
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
			var msg Message
			if json.Unmarshal(body, &msg) == nil && msg.Method == "" {
				c.in <- msg // an answer; the server's logs and requests are dropped
			}
		}
	}()
	t.Cleanup(func() {
		clientW.Close()
		cancel()
		select {
		case <-c.done:
		case <-time.After(10 * time.Second):
			t.Error("the server did not stop when its input ended")
		}
	})
	return c
}

// Write sends bytes as they are: a frame, or what is not one.
func (c *Conn) Write(raw string) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	io.WriteString(c.w, raw)
}

// Send frames and sends a message: `id` 0 makes it a notification.
func (c *Conn) Send(id int, method string, params any) {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != 0 {
		msg["id"] = id
	}
	if params != nil {
		msg["params"] = params
	}
	body, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatal(err)
	}
	c.Write(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

// Answer waits for the response to the request with the id.
func (c *Conn) Answer(id int) Message {
	c.t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		select {
		case msg, ok := <-c.in:
			if !ok {
				c.t.Fatalf("the server closed the connection before answering request %d", id)
			}
			if string(msg.ID) == strconv.Itoa(id) {
				return msg
			}
		case <-timeout:
			c.t.Fatalf("no answer to request %d in 30s", id)
		}
	}
}

// Close ends the server's input.
func (c *Conn) Close() { c.w.Close() }

// Ended waits for serve to return and gives its result; the test fails if
// it has not returned in time.
func (c *Conn) Ended(within time.Duration) error {
	c.t.Helper()
	select {
	case err := <-c.done:
		c.done <- err // for the cleanup
		return err
	case <-time.After(within):
		c.t.Fatalf("the server is still running after %s", within)
		return nil
	}
}
