// Package server runs the language server with whatever content mapper is
// registered (rtsx.RegisterMapper) built in (specs/phase01/ide.md,
// *reactogenic lsp*). It is its own package so that only a binary that
// serves LSP links the language service.
package server

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/lsp"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

// Options configure a run of the server.
type Options struct {
	In  io.Reader // LSP messages, framed
	Out io.Writer
	Err io.Writer // log output
	Cwd string
	// Name and Version are reported as the server info.
	Name, Version string
	// SetParentProcessID, when set, is called with the processId of the
	// client's initialize: the process whose end ends the server (upstream's
	// watchdog hook; the watching itself is the host's).
	SetParentProcessID func(pid int)
}

// Run serves LSP until the client exits, In ends or ctx is done. Nothing
// external runs: no content mapper process, no automatic type acquisition.
func Run(ctx context.Context, o Options) error {
	s := lsp.NewServer(&lsp.ServerOptions{
		In:                 lsp.ToReader(o.In),
		Out:                lsp.ToWriter(o.Out),
		Err:                o.Err,
		Cwd:                o.Cwd,
		FS:                 bundled.WrapFS(osvfs.FS()),
		DefaultLibraryPath: bundled.LibPath(),
		TypingsLocation:    "", // no automatic type acquisition
		NpmInstall: func(string, []string) ([]byte, error) {
			return nil, errors.New("this server does not install packages")
		},
		ProgressDelay:      250 * time.Millisecond,
		SetParentProcessID: o.SetParentProcessID,
		Embedder:           &lsp.Embedder{Name: o.Name, Version: o.Version, Capabilities: capabilities},
	})
	err := s.Run(ctx)
	if errors.Is(err, context.Canceled) && ctx.Err() == nil {
		// The client's exit: the server's loops stop each other through a
		// context of their own, and one of them reports that.
		return nil
	}
	return err
}

// capabilities takes out what upstream advertises and this server does not
// offer (specs/phase01/ide.md, the table of *reactogenic lsp*, lists what
// stays):
//   - code lens: its ranges are wrong on a declaration that holds mapped
//     constructs, and the lens's command is filled in by upstream's own VS
//     Code extension;
//   - `_vs_references`: Visual Studio's variant of references;
//   - `experimental`: the custom requests of upstream's VS Code extension
//     (source definition, multi-document highlights).
func capabilities(c *lsproto.ServerCapabilities) {
	c.CodeLensProvider = nil
	c.VSReferencesProvider = nil
	c.Experimental = nil
}
