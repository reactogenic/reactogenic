// Reactogenic: hooks for a host that embeds this server with a built-in
// content mapper. Not part of upstream: added by go/patches/0006-rtsx-lsp.patch.

package lsp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
)

// Embedder customizes the server for its host. With one set, no feature is
// registered dynamically: nothing for content-mapped extensions (a client
// that attaches by language would otherwise answer every request twice).
// Only watching is — files, and the configuration — and only with a client
// that declared support for it. The server offers no formatting.
type Embedder struct {
	// Name and Version replace the server info.
	Name, Version string
	// Capabilities may edit the static capabilities before they are sent.
	Capabilities func(*lsproto.ServerCapabilities)
	// Owns reports whether a file is the host's: a content-mapped one. The
	// client attaches this server to those documents only, and another
	// server — which does not know them — to the rest of the project. With
	// Owns set, the workspace-wide answers are narrowed to what no other
	// server gives:
	//   - workspace symbols: those declared in the host's files, chosen
	//     before the best matches are cut to 256;
	//   - file rename: the edits in the host's files, and the edits of
	//     imports that resolve to the host's files — decided per import, so
	//     a request that renames several files, or a folder, is answered
	//     right for each.
	Owns func(fileName string) bool
	// Diagnostics returns the diagnostics of a content-mapped document: all
	// of them, the compiler's too, which the host takes from the program
	// while they are structured — to drop, place and reword them — and
	// returns in its own words (ls.ProvideHostDiagnostics). Without it such
	// a document gets the compiler's diagnostics, mapped back by position.
	Diagnostics ls.HostDiagnostics
}

// hostDiagnostics is Diagnostics; nil without an embedder.
func (e *Embedder) hostDiagnostics() ls.HostDiagnostics {
	if e == nil {
		return nil
	}
	return e.Diagnostics
}

// symbolFiles is the files workspace symbols are collected from; nil: all.
func (e *Embedder) symbolFiles() func(*ast.SourceFile) bool {
	if e == nil || e.Owns == nil {
		return nil
	}
	return func(file *ast.SourceFile) bool { return e.Owns(file.OriginalFileName()) }
}

// renameEdits is the edits of a file rename to keep (ls.FileRenameEdits);
// nil: all. Everything is kept when the rename is this server's own answer
// to textDocument/rename on a module specifier (complete), for a client
// without willRenameFiles: no other server is asked then.
func (e *Embedder) renameEdits(complete bool) ls.FileRenameEdits {
	if e == nil || e.Owns == nil || complete {
		return nil
	}
	return func(importer, imported string) bool { return e.Owns(importer) || e.Owns(imported) }
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

// watchConfiguration registers for configuration changes, as upstream does
// in handleInitialized, with two differences: only with a client that
// declared workspace.didChangeConfiguration.dynamicRegistration (upstream
// asks every client), and without waiting for the answer — a client that
// never answers would otherwise get no reply to any request.
func (e *Embedder) watchConfiguration(s *Server) {
	if !s.clientCapabilities.Workspace.DidChangeConfiguration.DynamicRegistration {
		return
	}
	err := s.sendClientRequestFireAndForget(lsproto.ClientRegisterCapabilityInfo, &lsproto.RegistrationParams{
		Registrations: []*lsproto.Registration{
			{
				Id: "typescript-config-watch-id",
				RegisterOptions: &lsproto.RegisterOptions{
					WorkspaceDidChangeConfiguration: &lsproto.DidChangeConfigurationRegistrationOptions{
						Section: &lsproto.StringOrStrings{
							Strings: &[]string{"js/ts", "typescript", "javascript", "editor"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		s.logger.Errorf("failed to register configuration change watcher: %v", err)
	}
}
