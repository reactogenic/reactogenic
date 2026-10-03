package lsp_test

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/lsp"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

func serve(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
	return lsp.Serve(ctx, in, out, lsp.Options{Log: log, Cwd: cwd, Version: "0.0.0-test"})
}

const button = `export function Button({ size }: { size: number }) {
  return <button>{size}</button>;
}
`

// One mistake: the binding 'size' is a string, the prop a number.
const page = `import { Button } from "./button";
export function Page() {
  const size: string = "lg";
  return <main><Button size /></main>;
}
`

// ide.md, *The engine* (RGP1-103): the mapper is built in — a tsconfig
// project without a contentMappers entry, a project reached through
// 'references' (Vite's layout), and a loose file without a tsconfig all
// serve .rtsx at source positions.
func TestBuiltInMapper(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
	}{
		{"tsconfig", map[string]string{"src/button.rtsx": button, "src/page.rtsx": page}},
		{"references", map[string]string{
			"tsconfig.json":     `{ "files": [], "references": [{ "path": "./tsconfig.app.json" }] }`,
			"tsconfig.app.json": lsptest.TSConfig,
			"src/button.rtsx":   button, "src/page.rtsx": page,
		}},
		{"no tsconfig", map[string]string{"tsconfig.json": "", "src/jsx.d.ts": "", "src/button.rtsx": button, "src/page.rtsx": page}},
	} {
		t.Run(c.name, func(t *testing.T) {
			client := lsptest.Start(t, lsptest.Project(t, c.files), serve)
			client.Open("src/page.rtsx")

			// Hover on a copied expression: the binding, in the shorthand.
			if hover := client.Hover("src/page.rtsx", client.At("src/page.rtsx", "size", 2, 1)); !strings.Contains(hover, "size: string") {
				t.Errorf("hover on the shorthand: %q", hover)
			}
			// Hover across files: the component, imported without extension.
			if hover := client.Hover("src/page.rtsx", client.At("src/page.rtsx", "<Button", 1, 2)); !strings.Contains(hover, "function Button") {
				t.Errorf("hover on the component: %q", hover)
			}
			// The TS error, at the shorthand in the source. (A loose file
			// has no JSX types: more errors, but this one among them.)
			got := lsptest.Lines(client.Diagnostics("src/page.rtsx"))
			found := false
			for _, line := range got {
				found = found || line == "4:24 TS2322"
			}
			if !found || (c.name != "no tsconfig" && len(got) != 1) {
				t.Errorf("diagnostics: %q", got)
			}
			if def := client.Definition("src/page.rtsx", client.At("src/page.rtsx", "<Button", 1, 2)); len(def) != 1 || def[0] != "src/button.rtsx 1:17" {
				t.Errorf("definition: %q", def)
			}

			for _, r := range client.Registrations() {
				if strings.Contains(r, "content-mapper") {
					t.Errorf("dynamic registration for a mapped extension: %s", r)
				}
			}
		})
	}
}

// The server is ours: its info, and exactly the capabilities of ide.md's
// feature table (*reactogenic lsp*) — a capability the fork gains with a
// re-vendor fails here until the table has a row for it, or the bridge
// takes it out.
func TestServerInfo(t *testing.T) {
	client := lsptest.Start(t, lsptest.Project(t, map[string]string{"src/button.rtsx": button}), serve)
	if info := client.Initialized.ServerInfo; info.Name != "reactogenic" || info.Version != "0.0.0-test" {
		t.Errorf("server info: %+v", info)
	}
	rows := map[string]string{ // capability: its row in the table
		"positionEncoding":           "(the protocol)",
		"textDocumentSync":           "(the protocol: whole documents — the front keeps each text)",
		"diagnosticProvider":         "diagnostics",
		"hoverProvider":              "hover, signature help",
		"signatureHelpProvider":      "hover, signature help",
		"definitionProvider":         "go to definition, type definition, implementation, references, highlights",
		"typeDefinitionProvider":     "go to definition, type definition, implementation, references, highlights",
		"implementationProvider":     "go to definition, type definition, implementation, references, highlights",
		"referencesProvider":         "go to definition, type definition, implementation, references, highlights",
		"documentHighlightProvider":  "go to definition, type definition, implementation, references, highlights",
		"callHierarchyProvider":      "call hierarchy",
		"completionProvider":         "completion, auto-import",
		"renameProvider":             "rename",
		"semanticTokensProvider":     "semantic tokens",
		"inlayHintProvider":          "inlay hints",
		"documentSymbolProvider":     "document symbols, folding, selection ranges",
		"foldingRangeProvider":       "document symbols, folding, selection ranges",
		"selectionRangeProvider":     "document symbols, folding, selection ranges",
		"_vs_onAutoInsertProvider":   "closing-tag insertion",
		"linkedEditingRangeProvider": "linked editing",
		"codeActionProvider":         "code actions; organize / sort / remove unused imports",
		"workspaceSymbolProvider":    "workspace symbols",
		"workspace":                  "file rename",
	}
	for capability := range client.Initialized.Capabilities {
		if rows[capability] == "" {
			t.Errorf("the server advertises %s, which no row of ide.md's table lists", capability)
		}
	}
	for capability, row := range rows {
		if _, ok := client.Initialized.Capabilities[capability]; !ok {
			t.Errorf("the server does not advertise %s (row %q)", capability, row)
		}
	}
	var sync struct {
		Change int `json:"change"`
	}
	var workspace map[string]map[string]json.RawMessage
	json.Unmarshal(client.Initialized.Capabilities["textDocumentSync"], &sync)
	json.Unmarshal(client.Initialized.Capabilities["workspace"], &workspace)
	if sync.Change != 1 {
		t.Errorf("text document sync kind %d, want 1: whole documents", sync.Change)
	}
	if len(workspace) != 1 || len(workspace["fileOperations"]) != 1 || !strings.Contains(string(workspace["fileOperations"]["willRename"]), "rtsx") {
		t.Errorf("workspace capabilities: %s", client.Initialized.Capabilities["workspace"])
	}
}

// ide.md, *reactogenic lsp*: no feature is registered dynamically — a
// client that attaches by language would answer twice. Watching is: the
// configuration and files, each only with a client that declared it.
func TestRegistrations(t *testing.T) {
	files := map[string]string{"src/button.rtsx": button, "src/page.rtsx": page}
	// The methods registered once hover is answered — initialization is
	// through by then — and, the file watchers being registered in the
	// background, once the watching is there (where it is expected).
	methods := func(c *lsptest.Client, watching bool) string {
		t.Helper()
		c.Open("src/page.rtsx")
		if hover := c.Hover("src/page.rtsx", c.At("src/page.rtsx", "<Button", 1, 2)); !strings.Contains(hover, "function Button") {
			t.Errorf("hover: %q", hover)
		}
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			seen := map[string]bool{}
			for _, r := range c.Registrations() {
				method, _, _ := strings.Cut(r, " ")
				seen[method] = true
			}
			if watching && !seen["workspace/didChangeWatchedFiles"] && time.Now().Before(deadline) {
				continue
			}
			var out []string
			for method := range seen {
				out = append(out, method)
			}
			sort.Strings(out)
			return strings.Join(out, " ")
		}
	}
	without := func(path ...string) func(map[string]any) {
		return func(capabilities map[string]any) {
			for _, name := range path[:len(path)-1] {
				capabilities = capabilities[name].(map[string]any)
			}
			delete(capabilities, path[len(path)-1])
		}
	}
	t.Run("VS Code", func(t *testing.T) {
		c := lsptest.Start(t, lsptest.Project(t, files), serve)
		if got := methods(c, true); got != "workspace/didChangeConfiguration workspace/didChangeWatchedFiles" {
			t.Errorf("registered: %s", got)
		}
	})
	t.Run("no configuration registration", func(t *testing.T) {
		c := lsptest.StartWith(t, lsptest.Project(t, files), serve, lsptest.Options{Capabilities: without("workspace", "didChangeConfiguration")})
		if got := methods(c, true); got != "workspace/didChangeWatchedFiles" {
			t.Errorf("registered: %s", got)
		}
	})
	t.Run("no capabilities", func(t *testing.T) {
		c := lsptest.StartWith(t, lsptest.Project(t, files), serve, lsptest.Options{Capabilities: func(capabilities map[string]any) { clear(capabilities) }})
		if got := methods(c, false); got != "" || len(c.Asked()) != 0 {
			t.Errorf("registered: %q; asked of the client: %q", got, c.Asked())
		}
	})
	// Initialization does not wait for the client's answer to a
	// registration: one that never answers is served all the same.
	t.Run("a client that never answers", func(t *testing.T) {
		c := lsptest.StartWith(t, lsptest.Project(t, files), serve, lsptest.Options{
			Capabilities: without("workspace", "didChangeWatchedFiles"),
			Silent:       func(method string) bool { return method == "client/registerCapability" },
		})
		done := make(chan string, 1)
		go func() { done <- methods(c, false) }()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			t.Fatal("no answer to hover: initialization waits for the registration's answer")
		}
	})
}
