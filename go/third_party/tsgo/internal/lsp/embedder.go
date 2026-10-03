// Reactogenic: hooks for a host that embeds this server with a built-in
// content mapper. Not part of upstream: added by go/patches/0006-rtsx-lsp.patch.

package lsp

import (
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
)

// Embedder customizes the server for its host. With one set, the server has
// static capabilities only: it registers nothing dynamically for
// content-mapped extensions (a client that attaches by language would
// otherwise answer every request twice), and it offers no formatting.
type Embedder struct {
	// Name and Version replace the server info.
	Name, Version string
	// Capabilities may edit the static capabilities before they are sent.
	Capabilities func(*lsproto.ServerCapabilities)
}

func (e *Embedder) initializeResult(result *lsproto.InitializeResult) {
	if e.Name != "" {
		version := e.Version
		result.ServerInfo = &lsproto.ServerInfo{Name: e.Name, Version: &version}
	}
	c := result.Capabilities
	c.DocumentFormattingProvider = nil
	c.DocumentRangeFormattingProvider = nil
	c.DocumentOnTypeFormattingProvider = nil
	if e.Capabilities != nil {
		e.Capabilities(c)
	}
}
