package check

import (
	"bytes"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/report"
)

// A report about a page as a whole (report.Page: the builder's page checks)
// has a file and no position: it prints in the format of the others, without
// `(0,0)` and without a code frame of a line it is not about.
func TestPrintPageReport(t *testing.T) {
	reports := []Report{
		report.Page("/site/pages/guide/index.rtsx", "/guide/", "idref-not-found", "`commandfor=\"install\"` on `<button>` names no element of the page"),
		{File: "/site/pages/index.rtsx", Line: 3, Col: 7, Code: "TS2322", Message: "m", Related: []Report{{Message: "related"}}},
	}
	readFile := func(string) (string, bool) { return "one\ntwo\nthree four\n", true }
	for _, tt := range []struct {
		pretty bool
		want   string
	}{
		{false, "pages/guide/index.rtsx: error idref-not-found: Page /guide/: `commandfor=\"install\"` on `<button>` names no element of the page\n" +
			"pages/index.rtsx(3,7): error TS2322: m\n  related\n"},
		{true, "pages/guide/index.rtsx - error idref-not-found: Page /guide/: `commandfor=\"install\"` on `<button>` names no element of the page\n" +
			"pages/index.rtsx:3:7 - error TS2322: m\n\n3 three four\n        ^\n\n  related\n\nFound 2 error(s).\n"},
	} {
		var out bytes.Buffer
		Print(&out, reports, "/site", tt.pretty, readFile)
		if out.String() != tt.want {
			t.Errorf("pretty=%v:\n%s\nwant:\n%s", tt.pretty, out.String(), tt.want)
		}
	}
}
