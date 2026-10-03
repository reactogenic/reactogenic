package lsp_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsp"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

func serve(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
	return lsp.Serve(ctx, in, out, log, cwd, "0.0.0-test")
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

			if len(client.Registrations) > 0 {
				for _, r := range client.Registrations {
					if strings.Contains(r, "content-mapper") {
						t.Errorf("dynamic registration for a mapped extension: %s", r)
					}
				}
			}
		})
	}
}

// The server is ours: its info, static capabilities, no formatting.
func TestServerInfo(t *testing.T) {
	client := lsptest.Start(t, lsptest.Project(t, map[string]string{"src/button.rtsx": button}), serve)
	if info := client.Initialized.ServerInfo; info.Name != "reactogenic" || info.Version != "0.0.0-test" {
		t.Errorf("server info: %+v", info)
	}
	for _, capability := range []string{"documentFormattingProvider", "documentRangeFormattingProvider", "documentOnTypeFormattingProvider"} {
		if _, ok := client.Initialized.Capabilities[capability]; ok {
			t.Errorf("the server advertises %s", capability)
		}
	}
	for _, capability := range []string{"hoverProvider", "completionProvider", "definitionProvider", "diagnosticProvider", "renameProvider"} {
		if _, ok := client.Initialized.Capabilities[capability]; !ok {
			t.Errorf("the server does not advertise %s", capability)
		}
	}
}
