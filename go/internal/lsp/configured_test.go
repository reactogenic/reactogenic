package lsp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// ide.md, *The engine*: built in, not configured. A project that also runs
// stock TypeScript 7.1 lists @reactogenic/cli in tsconfig `contentMappers`
// (*Stock TypeScript 7.1*). For our server that entry changes nothing: the
// same hover, diagnostics and definition as without it, nothing reported on
// the tsconfig, and the package's command is not run.
func TestContentMappersEntryIgnored(t *testing.T) {
	const entry = `"contentMappers": [{ "package": "@reactogenic/cli", "extensions": [".rtsx"], "options": { "anything": true } }]`
	withEntry := strings.Replace(lsptest.TSConfig, `"include"`, entry+",\n  \"include\"", 1)
	if withEntry == lsptest.TSConfig {
		t.Fatal("the entry was not added to the tsconfig")
	}
	answers := func(t *testing.T, tsconfig string) []string {
		root := lsptest.Project(t, map[string]string{
			"tsconfig.json":   tsconfig,
			"src/button.rtsx": button, "src/page.rtsx": page,
			// A mapper package whose command, if anything ran it, would
			// leave a file behind.
			"node_modules/@reactogenic/cli/package.json": `{ "name": "@reactogenic/cli", "version": "0.0.0",
  "typescript": { "contentMapper": { "exec": ["sh", "-c", "touch spawned"] } } }`,
		})
		client := lsptest.Start(t, root, serve)
		client.Open("src/page.rtsx")
		got := []string{
			"hover: " + client.Hover("src/page.rtsx", client.At("src/page.rtsx", "size", 2, 1)),
			"hover: " + client.Hover("src/page.rtsx", client.At("src/page.rtsx", "<Button", 1, 2)),
			"diagnostics: " + strings.Join(lsptest.Lines(client.Diagnostics("src/page.rtsx")), ", "),
			"definition: " + strings.Join(client.Definition("src/page.rtsx", client.At("src/page.rtsx", "<Button", 1, 2)), ", "),
		}
		client.Open("tsconfig.json")
		got = append(got, "tsconfig: "+strings.Join(lsptest.Lines(client.Diagnostics("tsconfig.json")), ", "))
		for _, r := range client.Registrations() {
			if strings.Contains(r, "content-mapper") {
				t.Errorf("dynamic registration for a mapped extension: %s", r)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "node_modules/@reactogenic/cli/spawned")); err == nil {
			t.Errorf("the configured mapper's command was run")
		}
		return got
	}
	plain := answers(t, lsptest.TSConfig)
	configured := answers(t, withEntry)
	if strings.Join(configured, "\n") != strings.Join(plain, "\n") {
		t.Errorf("with the entry:\n%s\nwithout:\n%s", strings.Join(configured, "\n"), strings.Join(plain, "\n"))
	}
	if !strings.Contains(plain[0], "size: string") || plain[2] != "diagnostics: 4:24 TS2322" || plain[3] != "definition: src/button.rtsx 1:17" || plain[4] != "tsconfig: " {
		t.Errorf("without the entry:\n%s", strings.Join(plain, "\n"))
	}
}
