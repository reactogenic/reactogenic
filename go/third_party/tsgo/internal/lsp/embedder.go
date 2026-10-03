// Reactogenic: hooks for a host that embeds this server with a built-in
// content mapper. Not part of upstream: added by go/patches/0006-rtsx-lsp.patch.

package lsp

import (
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
