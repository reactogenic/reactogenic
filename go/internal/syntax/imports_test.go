package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// tags returns the framework export each JSX opening or self-closing tag
// refers to, in order ("" for none).
func tags(t *testing.T, src string) []string {
	t.Helper()
	file := rtsx.ParseRTSX("/test.rtsx", src)
	noErrors(t, file)
	rtsx.Bind(file)
	var out []string
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxOpeningElement || n.Kind == rtsx.KindJsxSelfClosingElement {
			out = append(out, FrameworkExport(n.TagName()))
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return out
}

func TestFrameworkExport(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{`import { Switch, Match, Each } from "@reactogenic/core";
const a = <><Switch on={1}></Switch><Match on={1} /><Each items={[]} /></>;`, []string{"Switch", "Match", "Each"}},
		// Aliases: recognised by origin, not by name.
		{`import { Switch as Choose } from "@reactogenic/core";
const a = <Choose on={1} />;`, []string{"Switch"}},
		// Not the framework's: another package, a local, a shadowing parameter,
		// a type-only import.
		{`import { Switch } from "solid-js";
const a = <Switch />;`, []string{""}},
		{`function Match() { return null; }
const a = <Match />;`, []string{""}},
		{`import { Switch } from "@reactogenic/core";
function f(Switch: any) { return <Switch />; }`, []string{""}},
		{`import type { Match } from "@reactogenic/core";
const a = <Match />;`, []string{""}},
	}
	for i, c := range cases {
		if got := tags(t, c.src); strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("case %d: got %q, want %q", i, got, c.want)
		}
	}
}

func TestCheckFlowAsValue(t *testing.T) {
	const imp = "import { Switch, Match, Each } from \"@reactogenic/core\";\nimport * as R from \"@reactogenic/core\";\n"
	cases := []struct {
		src  string
		want []string // reported spans
	}{
		// Fine: elements, Each as a value, look-alike names that are not references.
		{`const a = <Switch on={1}><$Case is={1}>x</$Case></Switch>;`, nil},
		{`const e = Each; const o = { Switch: 1 }; o.Switch; const p = <X Match="a" />;`, nil},
		// flow-as-value
		{`const S = Switch;`, []string{"Switch"}},
		{`const a = <Box as={Match} />;`, []string{"Match"}},
		{`createElement(Switch, {});`, []string{"Switch"}},
		{`export { Match };`, []string{"Match"}},
		{`export { Switch as Choose };`, []string{"Switch as Choose"}},
		{`export { Match } from "@reactogenic/core";`, []string{"Match"}},
		{`export * from "@reactogenic/core";`, []string{`export * from "@reactogenic/core";`}},
		{`const a = <R.Switch on={1} />;`, []string{"R.Switch"}},
		{`const f = R.Match;`, []string{"R.Match"}},
	}
	for _, c := range cases {
		src := imp + c.src
		file := rtsx.ParseRTSX("/test.rtsx", src)
		noErrors(t, file)
		rtsx.Bind(file)
		var got []string
		for _, e := range CheckFlowAsValue(file) {
			if e.Code != "flow-as-value" {
				t.Errorf("code = %q", e.Code)
			}
			got = append(got, src[e.Pos:e.End])
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s\n got %q, want %q", c.src, got, c.want)
		}
	}
}
