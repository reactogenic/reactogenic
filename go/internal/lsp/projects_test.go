package lsp_test

import (
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// The built-in mapper is one value for the life of the process. A project's
// command line is compared with the snapshot's mappers by identity, so a
// fresh one per snapshot made the inferred project dirty — a new program —
// whenever any document was opened or closed.
func TestInferredProjectIsNotRebuilt(t *testing.T) {
	c := start(t, map[string]string{
		"tsconfig.json": "", "src/jsx.d.ts": "", // no project at the root: loose.rtsx is in the inferred one
		"loose.rtsx":         "export const loose = 1;\n",
		"app/tsconfig.json":  lsptest.TSConfig,
		"app/src/jsx.d.ts":   lsptest.JSXTypes,
		"app/src/page.rtsx":  "export const page = <p />;\n",
		"app/src/other.rtsx": "export const other = <p />;\n",
	})
	c.Open("loose.rtsx")
	updates := func() int {
		// A request after the changes: the snapshot is the latest by then.
		if hover := c.Hover("loose.rtsx", c.At("loose.rtsx", "loose", 1, 1)); !strings.Contains(hover, "const loose: 1") {
			t.Fatalf("hover in the loose file: %q", hover)
		}
		return strings.Count(strings.Join(c.Logs(), "\n"), "Updating inferred project config")
	}
	before := updates()
	c.Open("app/src/page.rtsx")
	if hover := c.Hover("app/src/page.rtsx", c.At("app/src/page.rtsx", "page", 1, 1)); !strings.Contains(hover, "const page") {
		t.Fatalf("hover in the project's file: %q", hover)
	}
	c.Open("app/src/other.rtsx")
	c.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": c.URI("app/src/other.rtsx")}})
	if after := updates(); after != before {
		t.Errorf("the inferred project's configuration was replaced %d times while documents of another project were opened and closed", after-before)
	}
	// The log is where it shows: it does say so when the roots change.
	c.OpenAs("loose2.rtsx", "export const loose2 = 2;\n")
	if after := updates(); after != before+1 {
		t.Errorf("a second loose file: %d updates, %d before\n%s", after, before, strings.Join(c.Logs(), "\n"))
	}
}
