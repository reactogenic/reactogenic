// Package lsp is `reactogenic lsp` (specs/phase01/ide.md): the fork's
// TypeScript 7 language server, with the .rtsx transform built in as a
// content mapper.
package lsp

import (
	"context"
	"errors"
	"io"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// Options configure Serve.
type Options struct {
	Log io.Writer // the server's log, and what the front has to say
	Cwd string
	// Version is the binary's: the server info, and part of every cache key.
	Version string
	// ClientProcessID is the editor's process (`--clientProcessId`); 0 takes
	// the processId of the client's initialize. The server ends when that
	// process is gone.
	ClientProcessID int
}

// ErrNoShutdown is Serve's result for an `exit` that no `shutdown` preceded:
// exit status 1, as LSP has it.
var ErrNoShutdown = errors.New("exit without shutdown")

// Serve runs the server on in and out until the client's `exit`, the end of
// in, or the end of the client's process. Its result is nil after shutdown
// and exit, and when in just ends; an error — a non-zero exit status — for
// an exit without shutdown, for input that is not LSP, and for a client that
// is gone.
func Serve(ctx context.Context, in io.Reader, out io.Writer, o Options) error {
	return newFront(o.Log).serve(ctx, in, out, o)
}

func (f *front) serve(ctx context.Context, in io.Reader, out io.Writer, o Options) error {
	mapper.Register(o.Version)
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	serverIn, toServer := io.Pipe()
	fromServer, serverOut := io.Pipe()
	go f.fromClient(in, toServer)
	relayed, written := make(chan error, 1), make(chan error, 1)
	go func() { relayed <- f.fromServer(fromServer) }()
	go func() { written <- f.outgoing.drain(out) }()
	err := server.Run(ctx, server.Options{
		In: serverIn, Out: serverOut, Err: o.Log, Cwd: o.Cwd, Name: "reactogenic", Version: o.Version,
		SetParentProcessID: parentWatchdog(ctx, stop, o.ClientProcessID),
	})
	f.mu.Lock()
	ended, shutdown := f.ended, f.shutdown
	f.mu.Unlock()
	// The server's reader may still wait for input, and fromClient may still
	// be writing to it: closing this end releases both.
	serverIn.Close()
	serverOut.Close()
	<-relayed // everything the server wrote is queued for the client
	f.outgoing.close()
	<-written
	switch {
	case ended == errExit && shutdown:
		return nil
	case ended == errExit:
		return ErrNoShutdown
	case ended != nil:
		return ended
	case ctx.Err() != nil:
		return context.Cause(ctx) // the watchdog's reason, or the caller's
	}
	return err
}
