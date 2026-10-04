package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/checktest"
)

// RGP1-073: every rewrite of diagnostics.md, *Rewrites*, on one project.
func TestRewrites(t *testing.T) {
	var got []string
	for _, r := range runFiles(t, checktest.Get("rewrites").Files) {
		if strings.HasSuffix(r.File, "page.rtsx") {
			got = append(got, fmt.Sprintf("%d:%d %s: %s", r.Line, r.Col, r.Code, r.Message))
		}
	}
	want := []string{
		"4:26 undeclared-slot: `$Nope` is not declared in `Card`",
		"5:20 missing-slot: `Must` requires `$Title`",
		"6:35 no-values: `$Title` provides no values",
		"7:41 content-not-allowed: `$Box` takes no body",
		"8:26 slot-type: `$Label` does not match its declaration in `Card`: Type '{}' is not assignable to type 'Slot<{ children: ReactNode; }> | undefined'.",
		"9:32 params-required: `$Icon` requires params: its body is a function of the attachment's args",
		"10:19 switch-missing-case: Missing `\"b\"`, `\"c\"`",
		"11:24 segment-not-component: The segment `num` has no default component",
		"12:24 segment-props: A segment takes no props: `props` requires `x`",
		"13:27 segment-root-props: `Plain` must accept `id` and `children` to be a segment root",
		"14:27 TS2322: Type 'boolean' is not assignable to type 'string'.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// runFiles writes a temporary project and checks it.
func runFiles(t *testing.T, files map[string]string) []Report {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	for name, text := range files {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(text), 0o644)
	}
	reports := Run(filepath.ToSlash(dir) + "/tsconfig.json")
	golden(t, filepath.ToSlash(dir), reports)
	return reports
}
