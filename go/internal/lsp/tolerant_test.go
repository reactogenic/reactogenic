package lsp_test

import (
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// A page whose names are used only inside a Switch: when the Switch leaves
// the virtual text, they all read as unused.
var switchApp = lsptest.With(lsptest.Core, map[string]string{
	"tsconfig.json": strings.Replace(lsptest.TSConfig, `"noEmit": true`, `"noEmit": true, "noUnusedLocals": true, "noUnusedParameters": true`, 1),
	"src/spinner.rtsx": `export function Spinner() {
  return <i>...</i>;
}
`,
	"src/page.rtsx": `import { Switch, Match } from "@reactogenic/core";
import { Spinner } from "./spinner";

export enum Status { loading, done }

export function Page({ status }: { status: Status }) {
  const label = "Done";
  return (
    <main>
      <Switch on={status}>
        <$Case is={Status.loading}><Spinner /></$Case>
        <$Case is={Status.done}>{label}</$Case>
      </Switch>
      <Match on={status}>x</Match>
    </main>
  );
}
`,
})

// ide.md, *Tolerance*: a `$Case` being typed does not take the `Switch` out
// of the virtual text. Completion works in the test being typed, and TS
// reports nothing on the untouched lines — with `noUnusedLocals`, as Vite's
// template sets it, a vanished Switch made every name used in it an error.
func TestTypingACase(t *testing.T) {
	c := start(t, switchApp)
	const page = "src/page.rtsx"
	src := switchApp[page]
	c.Open(page)
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}

	// Each state of typing one more case, on a new line before `</Switch>`.
	for _, typed := range []string{"<", "<$", "<$Case", "<$Case is=", "<$Case is={", "<$Case is={Status.}", "<$Case is={Status.done}>"} {
		text := strings.Replace(src, "      </Switch>", "        "+typed+"\n      </Switch>", 1)
		c.Change(page, text)
		for _, d := range c.Diagnostics(page) {
			// TS6133 / 6196 / 6198: declared but never read.
			if code := strings.Trim(string(d.Code), `"`); code == "6133" || code == "6196" || code == "6198" {
				t.Errorf("typing %q: %s %s", typed, d, d.Message)
			}
		}
	}

	// Completion right after `Status.`, in every place a subject or a test
	// is typed — the tag still open, or closed.
	for name, edit := range map[string][2]string{
		"a new case, tag open":      {"      </Switch>", "        <$Case is={Status.}\n      </Switch>"},
		"a new case, brace open":    {"      </Switch>", "        <$Case is={Status.\n      </Switch>"},
		"a new case, tag closed":    {"      </Switch>", "        <$Case is={Status.}>x</$Case>\n      </Switch>"},
		"an existing case":          {"<$Case is={Status.done}>", "<$Case is={Status.}>"},
		"the Switch subject":        {"<Switch on={status}>", "<Switch on={Status.}>"},
		"a Match subject, tag open": {"<Match on={status}>x</Match>", "<Match on={Status.}"},
	} {
		text := strings.Replace(src, edit[0], edit[1], 1)
		if text == src {
			t.Fatalf("%s: no edit", name)
		}
		c.Change(page, text)
		var list struct {
			Items []struct {
				Label string `json:"label"`
			} `json:"items"`
		}
		at := strings.Index(text, "Status.}")
		if at < 0 {
			at = strings.Index(text, "Status.\n")
		}
		c.Request("textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": c.URI(page)}, "position": lsptest.PositionAt(text, at+len("Status."))}, &list)
		var members []string
		for _, item := range list.Items {
			if item.Label == "loading" || item.Label == "done" {
				members = append(members, item.Label)
			}
		}
		if len(members) != 2 {
			t.Errorf("%s: completion after `Status.` offers %q among %d items", name, members, len(list.Items))
		}
	}
}
