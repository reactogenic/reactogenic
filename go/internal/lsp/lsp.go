// Package lsp is `reactogenic lsp` (specs/phase01/ide.md): the fork's
// TypeScript 7 language server, with the .rtsx transform built in as a
// content mapper.
package lsp

import (
	"context"
	"io"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// Serve runs the server on in and out until the client exits or in ends.
// version is the binary's: the server info, and part of every cache key.
func Serve(ctx context.Context, in io.Reader, out, log io.Writer, cwd, version string) error {
	mapper.Register(version)
	return server.Run(ctx, server.Options{In: in, Out: out, Err: log, Cwd: cwd, Name: "reactogenic", Version: version})
}
