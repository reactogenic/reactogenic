package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RGP1-073: every rewrite of diagnostics.md, *Rewrites*, on one project.
func TestRewrites(t *testing.T) {
	files := map[string]string{
		"tsconfig.json": tsconfig,
		"src/jsx.d.ts":  jsxTypes,
		"src/lib.tsx": `import type { Slot } from "@reactogenic/core";
type ReactNode = string | JSX.Element | undefined;
export function Card(p: { $Title?: Slot<{ tone?: string; children?: ReactNode }>; $Box?: Slot<{ color: string }>; $Label?: Slot<{ children: ReactNode }>; $Icon?: Slot<{ id?: string }, { size: string }> }) { return <div />; }
export function Must(p: { $Title: Slot<{ children?: ReactNode }> }) { return <div />; }
export function Plain(p: { className?: string }) { return <div />; }
export function Input(p: { value: string }) { return <input />; }
`,
		"src/+num.rtsx":   "export default 42;\n",
		"src/+props.rtsx": "export default function P(p: { x: string }) { return <div />; }\n",
		"src/+ok.rtsx":    "export default function Ok() { return <div />; }\n",
		"src/page.rtsx": `import { Card, Must, Plain, Input } from "./lib";
import { Switch } from "@reactogenic/core";
declare function getStatus(): "a" | "b" | "c";
export const p1 = <Card><$Nope>x</$Nope></Card>;
export const p2 = <Must />;
export const p3 = <Card><$Title { x }>t</$Title></Card>;
export const p4 = <Card><$Box color="r">Hi</$Box></Card>;
export const p5 = <Card><$Label /></Card>;
export const p6 = <Card><$Icon>text</$Icon></Card>;
export const p7 = <Switch on={getStatus()} exhaustive><$Case is="a">A</$Case></Switch>;
export const p8 = <div #num />;
export const p9 = <div #props />;
export const p10 = <Plain #ok />;
export const p11 = <Input value />;
`,
	}
	for k, v := range coreStub {
		files[k] = v
	}
	files["node_modules/@reactogenic/core/index.d.ts"] += "\nexport declare function Switch(p: { on: unknown; exhaustive?: true; $Case?: unknown }): null;\nexport declare function noMatch(value: never): never;\n"
	var got []string
	for _, r := range runFiles(t, files) {
		if strings.HasSuffix(r.File, "page.rtsx") {
			got = append(got, fmt.Sprintf("%d:%d %s: %s", r.Line, r.Col, r.Code, r.Message))
		}
	}
	want := []string{
		"4:25 undeclared-slot: `$Nope` is not declared in `Card`",
		"5:20 missing-slot: `Must` requires `$Title`",
		"6:35 no-values: `$Title` provides no values",
		"7:41 content-not-allowed: `$Box` takes no body",
		"8:25 slot-type: `$Label` does not match its declaration in `Card`: Type '{}' is not assignable to type 'Slot<{ children: ReactNode; }> | undefined'.",
		"9:32 params-required: `$Icon` requires params: its body is a function of the attachment's args",
		"10:19 switch-missing-case: Missing `\"b\"`, `\"c\"`",
		"11:24 segment-not-component: `+num` has no default component",
		"12:24 segment-props: A segment takes no props: `+props` requires `x`",
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
	return Run(filepath.ToSlash(dir) + "/tsconfig.json")
}
