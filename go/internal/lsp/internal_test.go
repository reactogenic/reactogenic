package lsp

import (
	"errors"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// ide.md, *Tolerance*, rule 5, as the editor sees it: a failure of the
// transpiler is one `internal` diagnostic on the first line, naming the
// pass. The file's source stands in as its virtual text, so nothing
// TypeScript says about it is shown; hover still answers, the other
// documents are checked, the server goes on.
//
// The transform is made to fail for one file. (That a panic in a pass ends
// as this result is the mapper's test, TestPanics.)
func TestInternalDiagnostic(t *testing.T) {
	register := registerMapper
	defer func() { registerMapper = register }()
	registerMapper = func(version string) {
		rtsx.RegisterMapper(&rtsx.Mapper{Name: "reactogenic", Version: version, Extension: ".rtsx", Transform: func(req rtsx.MapperRequest) rtsx.MapperResult {
			n := int32(len(req.Content))
			file := &mapper.File{}
			if strings.HasSuffix(req.FileName, "/failing.rtsx") {
				file = &mapper.File{Stopped: true, Err: errors.New("transpiler: pass 3 (slot hoisting): boom")}
			}
			return rtsx.MapperResult{Text: req.Content, Spans: [][6]int32{{0, n, 0, n, 0, int32(emit.AllFeatures)}}, Extra: file}
		}})
	}
	c, _ := startFront(t, map[string]string{
		"src/failing.rtsx": "export const n: number = \"x\";\n",
		"src/other.rtsx":   "import { n } from \"./failing\";\nexport const s: string = n;\n",
	}, lsptest.Options{})
	const failing, other = "src/failing.rtsx", "src/other.rtsx"
	c.Open(failing)
	c.Open(other)
	var got []string
	for _, d := range c.AllDiagnostics(failing) {
		got = append(got, d.Show())
	}
	if want := `1:1-1:2 error reactogenic("internal") transpiler: pass 3 (slot hoisting): boom`; strings.Join(got, "\n") != want {
		t.Errorf("failing.rtsx:\ngot  %s\nwant %s", strings.Join(got, "\n     "), want)
	}
	if hover := c.Hover(failing, c.At(failing, "n:", 1, 0)); !strings.Contains(hover, "const n: number") {
		t.Errorf("hover in the file that failed: %q", hover)
	}
	if got := strings.Join(lsptest.Lines(c.Diagnostics(other)), ", "); got != "2:14 TS2322" {
		t.Errorf("its importer: %s", got)
	}
}

// ide.md, *Commands*: the step of `reactogenic/transpiled` names the
// tolerance step that produced the text — the last pass that held, or the
// source standing in. (`lowered`, and the text itself: TestTranspiled.) The
// transform is made to stop for two files.
func TestTranspiledSteps(t *testing.T) {
	register := registerMapper
	defer func() { registerMapper = register }()
	registerMapper = func(version string) {
		rtsx.RegisterMapper(&rtsx.Mapper{Name: "reactogenic", Version: version, Extension: ".rtsx", Transform: func(req rtsx.MapperRequest) rtsx.MapperResult {
			n := int32(len(req.Content))
			file := &mapper.File{Stopped: true}
			if strings.HasSuffix(req.FileName, "/stopped.rtsx") {
				file.Output.Map, file.Output.Stopped = emit.Identity(len(req.Content)), "pass 3 (slot hoisting): does not settle"
			} else {
				file.Err = errors.New("transpiler: boom")
			}
			return rtsx.MapperResult{Text: req.Content, Spans: [][6]int32{{0, n, 0, n, 0, int32(emit.AllFeatures)}}, Extra: file}
		}})
	}
	c, _ := startFront(t, map[string]string{
		"src/stopped.rtsx": "export const a = 1;\n",
		"src/failing.rtsx": "export const b = 2;\n",
	}, lsptest.Options{})
	for rel, want := range map[string]string{"src/stopped.rtsx": "stopped: pass 3 (slot hoisting)", "src/failing.rtsx": "source"} {
		c.Open(rel)
		var result struct{ Text, Step string }
		c.Request("reactogenic/transpiled", map[string]any{"textDocument": map[string]any{"uri": c.URI(rel)}}, &result)
		if result.Step != want || result.Text != c.Text(rel) {
			t.Errorf("%s: step %q, text %q; want %q and the source", rel, result.Step, result.Text, want)
		}
	}
}
