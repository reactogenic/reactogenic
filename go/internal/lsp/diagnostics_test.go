package lsp_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/checktest"
	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

var diagnosticsApp = lsptest.With(lsptest.Core, map[string]string{
	"src/button.rtsx": `import type { Slot } from "@reactogenic/core";
export interface ButtonProps {
  size: string;
  /** @deprecated use size */
  dim?: string;
  $Label?: Slot<{ className?: string; children?: string }>;
}
export function Button({ size, $Label }: ButtonProps) {
  return <button><b slot={$Label}>{size}</b></button>;
}
`,
	"src/intro.rtsx": "export default function Intro() {\n  return <p>intro</p>;\n}\n",
	"src/page.rtsx": `import { Button } from "./button";
export function Page({ value }: { value: number }) {
  const unused = 1;
  return (
    <main>
      <Button size="m" dim="x"><$Badge>new</$Badge></Button>
      <Button size />
      <section #intro>old</section>
      <Button size={value} />
    </main>
  );
}
`,
	// Related locations outside .rtsx files: a .ts module, a lib file.
	"src/util.ts":    "export interface Props { size: string }\n",
	"src/props.rtsx": "import type { Props } from \"./util\";\nexport const q: Props = {};\nexport const p = new Promise();\n",
	// Spans of no length: a file-level diagnostic, and syntax errors — at the
	// end of a line, and at the end of the text.
	"src/card.rtsx": "export const a = <p>rtsx</p>;\n",
	"src/card.tsx":  "export const a = <p>tsx</p>;\n",
	"src/open.rtsx": "const o = { a: 1 };\nexport const y = o.\r\nexport const z = (1",
})

// shown is every diagnostic of a document as the server answers a pull,
// suggestions included: `range severity source(code) tags message`, the
// text of the range, and under it each related location with its text.
func shown(c *lsptest.Client, rel string) string {
	var lines []string
	for _, d := range c.AllDiagnostics(rel) {
		lines = append(lines, fmt.Sprintf("%s  %q", d.Show(), c.RangeText(rel, d.Range)))
		for _, r := range d.Related {
			file, text := c.Rel(r.Location.URI), ""
			if !strings.Contains(file, ":") { // a file of the project, not a lib file of the binary
				text = fmt.Sprintf(" %q", c.RangeText(file, r.Location.Range))
			}
			lines = append(lines, fmt.Sprintf("  %s %s%s: %s", file, r.Location.Range, text, r.Message))
		}
	}
	return strings.Join(lines, "\n")
}

// ide.md, *Diagnostics*: what a pull of an .rtsx document answers — the
// reports of the reporting layer as LSP diagnostics.
func TestDiagnostics(t *testing.T) {
	c := start(t, diagnosticsApp)
	for _, doc := range []struct{ rel, want string }{
		{"src/page.rtsx", strings.Join([]string{
			// A suggestion passes through untouched, with its tag: the
			// editor fades the name.
			`3:9-3:15 hint ts(6133) unnecessary 'unused' is declared but its value is never read.  "unused"`,
			`6:24-6:27 hint ts(6385) deprecated 'dim' is deprecated.  "dim"`,
			`  src/button.rtsx 4:7-4:28 "@deprecated use size ": The declaration was marked as deprecated here.`,
			// A TS error reworded in slot terms: its name as the code, at
			// the slot's name.
			"6:33-6:39 error reactogenic(\"undeclared-slot\") `$Badge` is not declared in `Button`  \"$Badge\"",
			// A TS error that is not reworded keeps its number and the
			// source "ts": TypeScript's quick fixes find it by them. Its
			// related locations are mapped — one of ours, without a place
			// of its own, is at the diagnostic's; one of TS's in another
			// .rtsx file, at that file's source.
			`7:15-7:19 error ts(2322) Type 'boolean' is not assignable to type 'string'.  "size"`,
			"  src/page.rtsx 7:15-7:19 \"size\": No `size` in scope: a bare attribute is `true`. Did you mean `size={size}`?",
			`  src/button.rtsx 3:3-3:7 "size": The expected type comes from property 'size' which is declared here on type 'ButtonProps'`,
			// A transpiler warning stays a warning.
			"8:16-8:22 warning reactogenic(\"segment-children\") Contents will be overwritten by the segment `intro`  \"#intro\"",
			`9:15-9:19 error ts(2322) Type 'number' is not assignable to type 'string'.  "size"`,
			`  src/button.rtsx 3:3-3:7 "size": The expected type comes from property 'size' which is declared here on type 'ButtonProps'`,
		}, "\n")},
		{"src/props.rtsx", strings.Join([]string{
			`2:14-2:15 error ts(2741) Property 'size' is missing in type '{}' but required in type 'Props'.  "q"`,
			`  src/util.ts 1:26-1:30 "size": 'size' is declared here.`,
			`3:18-3:31 error ts(2554) Expected 1 arguments, but got 0.  "new Promise()"`,
			`  bundled:///libs/lib.es2015.promise.d.ts 29:13-29:109: An argument for 'executor' was not provided.`,
		}, "\n")},
		// A span of no length is a range of one character, so that every
		// client has something to underline: the first of the file for a
		// file-level diagnostic; the line break after a line that ends too
		// early — both characters of a CRLF; nothing at the end of the text.
		{"src/card.rtsx", "1:1-1:2 error reactogenic(\"ambiguous-module\") `card.tsx` and `card.rtsx` side by side: an import of `./card` is ambiguous  \"e\""},
		{"src/open.rtsx", strings.Join([]string{
			`2:20-3:1 error ts(1003) Identifier expected.  "\r\n"`,
			`3:20-3:20 error ts(1005) ')' expected.  ""`,
		}, "\n")},
	} {
		c.Open(doc.rel)
		if got := shown(c, doc.rel); got != doc.want {
			t.Errorf("%s:\n%s\nwant:\n%s", doc.rel, got, doc.want)
		}
	}
}

// TypeScript's style checks — unused locals, as `noUnusedLocals` reports
// them — are warnings in the editor and errors in `check`, as in VS Code's
// own TypeScript: the user's `reportStyleChecksAsWarnings`, on by default.
// And `validate.enable` turns an .rtsx document's diagnostics off, the
// transpiler's with TypeScript's.
func TestDiagnosticSettings(t *testing.T) {
	files := map[string]string{
		"tsconfig.json": strings.Replace(lsptest.TSConfig, `"noEmit": true`, `"noEmit": true, "noUnusedLocals": true`, 1),
		"src/page.rtsx": "export function Page() {\n  const unused = 1;\n  return <main />;\n}\n",
		"src/page.tsx":  "export const x = 1;\n",
	}
	const page = "src/page.rtsx"
	show := func(c *lsptest.Client) string {
		var got []string
		for _, d := range c.AllDiagnostics(page) {
			got = append(got, d.Show())
		}
		return strings.Join(got, "\n")
	}
	const ambiguous = "1:1-1:2 error reactogenic(\"ambiguous-module\") `page.tsx` and `page.rtsx` side by side: an import of `./page` is ambiguous\n"
	const unused = " ts(6133) unnecessary 'unused' is declared but its value is never read."

	c := start(t, files)
	c.Open(page)
	if got, want := show(c), ambiguous+"2:9-2:15 warning"+unused; got != want {
		t.Errorf("by default:\ngot  %s\nwant %s", got, want)
	}
	c.Configure(map[string]any{"typescript": map[string]any{"reportStyleChecksAsWarnings": false}})
	if got, want := show(c), ambiguous+"2:9-2:15 error"+unused; got != want {
		t.Errorf("reportStyleChecksAsWarnings off:\ngot  %s\nwant %s", got, want)
	}
	c.Configure(map[string]any{"typescript": map[string]any{"validate": map[string]any{"enable": false}}})
	if got := show(c); got != "" {
		t.Errorf("validate.enable off: %s", got)
	}
}

// ide.md, *Tolerance*, rules 2–4 as the editor sees them.
func TestTolerance(t *testing.T) {
	c := start(t, lsptest.With(lsptest.Core, map[string]string{
		// Rule 2: the syntax errors are the source parse's, each once —
		// TypeScript finds the unclosed tag in the virtual text too. The
		// rest of a file being typed is still checked.
		"src/typing.rtsx": "export const n: number = \"x\";\nexport const a = <div><span></div>;\n",
		// Rules 2 and 4: `&size` stays in the virtual text as written, which
		// is then not TSX. TypeScript's syntax errors there are not shown,
		// nor anything else it says of the file — not the true type error
		// of line 3 either, until the construct is fixed.
		"src/args.rtsx": "declare const size: number;\nexport const a = <option &size />;\nexport const n: number = \"x\";\n",
		// Rule 4: an orphaned slot's contents are left out of the virtual
		// text, and every name used only there would read as unused.
		"src/orphan.rtsx": "export const n: number = \"x\";\nconst label = \"t\";\nexport const o = <div><$Title>{label}</$Title></div>;\n",
		// A stopped file's module is still what it exports: its importers
		// are checked.
		"src/main.rtsx": "import { n } from \"./orphan\";\nexport const s: string = n;\n",
	}))
	for rel, want := range map[string]string{
		"src/typing.rtsx": "1:14 TS2322, 2:24 TS17008",
		"src/args.rtsx":   "2:26 arg-without-slot",
		"src/orphan.rtsx": "3:23 orphan-slot",
		"src/main.rtsx":   "2:14 TS2322",
	} {
		c.Open(rel)
		if got := strings.Join(lsptest.Lines(c.AllDiagnostics(rel)), ", "); got != want {
			t.Errorf("%s: %s; want %s", rel, got, want)
		}
	}

	// The two projects of check's goldens with a syntax error, which
	// TestEqualsCheck leaves out: where the hosts differ, by design.
	//   - `check` is strict: of broken.rtsx it prints the syntax error and
	//     does not lower the file (TS17008 only). The editor lowers what
	//     parses, and shows the type error of line 1 too; the orphaned slot
	//     is under the broken element, so its error is dropped (rule 3).
	//   - While a .tsx file has a syntax error `check` prints no type error
	//     at all — tsc's steps — and of page.rtsx only `orphan-slot`. A
	//     document's diagnostics have no steps (ide.md, *Diagnostics*).
	for name, want := range map[string]map[string]string{
		"syntax-error":     {"src/broken.rtsx": "1:14 TS2322, 3:34 TS17008"},
		"tsx-syntax-error": {"src/page.rtsx": "1:14 TS2322, 2:26 orphan-slot"},
	} {
		c := lsptest.Start(t, lsptest.Write(t, checktest.Get(name).Files), serve)
		for rel, want := range want {
			c.Open(rel)
			if got := strings.Join(lsptest.Lines(c.AllDiagnostics(rel)), ", "); got != want {
				t.Errorf("%s, %s: %s; want %s", name, rel, got, want)
			}
		}
	}
}

// awaitRefresh waits for the server to ask for diagnostics to be pulled
// again, after `before` such requests.
func awaitRefresh(t *testing.T, c *lsptest.Client, before int, what string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); c.RefreshCount() <= before; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Errorf("%s: the server did not ask the client to pull diagnostics again", what)
			return
		}
	}
}

// ide.md, *Diagnostics*: after a change to any file the server asks the
// client to pull again (workspace/diagnostic/refresh) — a client pulls the
// document that changed, not the others — and an open document's
// diagnostics follow an edit elsewhere, with no edit to it.
func TestRefresh(t *testing.T) {
	files := map[string]string{
		"src/util.ts":     "export const n: number = 1;\n",
		"src/button.rtsx": "export function Button({ size }: { size: number }) {\n  return <button>{size}</button>;\n}\n",
		"src/page.rtsx":   "import { Button } from \"./button\";\nimport { n } from \"./util\";\nexport const page = <Button size={n} />;\n",
	}
	c := start(t, files)
	const page, button = "src/page.rtsx", "src/button.rtsx"
	step := func(what, want string, do func()) {
		t.Helper()
		before := c.RefreshCount()
		do()
		awaitRefresh(t, c, before, what)
		if got := strings.Join(lsptest.Lines(c.Diagnostics(page)), ", "); got != want {
			t.Errorf("%s: page.rtsx has %q, want %q", what, got, want)
		}
	}
	c.Open(page)
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Fatalf("the fixture has errors: %q", got)
	}

	// Opening a document whose text is the file's changes nothing for the
	// others: no refresh.
	before := c.RefreshCount()
	c.Open(button)
	c.Diagnostics(button)
	time.Sleep(300 * time.Millisecond)
	if n := c.RefreshCount() - before; n != 0 {
		t.Errorf("opening a saved document: %d refresh requests", n)
	}

	text := strings.Replace(files[button], "size: number", "size: string", 1)
	step("an edit of another open document", "3:29 TS2322", func() { c.Change(button, text) })
	// Closed without saving: the file on disk is the module again.
	step("that document closed, its edit discarded", "", func() { c.Close(button) })
	// Opened with a text that is not the file's: a buffer restored by the
	// editor.
	step("a document opened with unsaved text", "3:29 TS2322", func() { c.OpenAs(button, text) })
	step("its edit undone", "", func() { c.Change(button, files[button]) })
	// A .ts file is read from disk; its watcher reports the change.
	step("a .ts file changed on disk", "3:29 TS2322", func() { c.WriteFile("src/util.ts", "export const n: string = \"\";\n") })
	step("an .rtsx file changed on disk", "", func() {
		c.Close(button)
		c.WriteFile(button, text)
	})
	// The project's options are read again when its tsconfig changes.
	step("the tsconfig changed on disk", "", func() {
		c.WriteFile("tsconfig.json", strings.Replace(lsptest.TSConfig, `"strict": true`, `"strict": false`, 1))
	})
	c.Change(page, files[page]+"export const pick = (o) => o;\n")
	if got := lsptest.Lines(c.Diagnostics(page)); len(got) != 0 {
		t.Errorf("an untyped parameter, not strict: %q", got)
	}
	step("the tsconfig strict again", "4:22 TS7006", func() { c.WriteFile("tsconfig.json", lsptest.TSConfig) })
}

// ide.md, *Segments*: a segment file created or deleted updates its
// mounter's diagnostics with no edit to the mounter — on disk, and as a
// buffer that was never saved. So does the `.tsx` sibling that makes the
// module ambiguous. And `segment-self`, which follows mounts through other
// files, updates when a sibling's content changes: it is the reporting
// layer's, read from the program.
func TestSiblings(t *testing.T) {
	c := start(t, map[string]string{
		"src/page.rtsx": "export default function Page() {\n  return <main><section #intro /></main>;\n}\n",
	})
	const page, intro = "src/page.rtsx", "src/intro.rtsx"
	const plain = "export default function Intro() {\n  return <p>intro</p>;\n}\n"
	const loop = "export default function Intro() {\n  return <div><section #page /></div>;\n}\n"
	const missing = "2:25 segment-not-found, 2:25 TS2307"
	c.Open(page)
	pulled := func() string { return strings.Join(lsptest.Lines(c.Diagnostics(page)), ", ") }
	if got := pulled(); got != missing {
		t.Fatalf("without intro.rtsx: %s", got)
	}
	step := func(what, want string, do func()) {
		t.Helper()
		before := c.RefreshCount()
		do()
		awaitRefresh(t, c, before, what)
		if got := pulled(); got != want {
			t.Errorf("%s: page.rtsx has %q, want %q", what, got, want)
		}
	}
	step("intro.rtsx created on disk", "", func() { c.WriteFile(intro, plain) })
	step("intro.rtsx mounts #page, on disk", "2:25 segment-self", func() { c.WriteFile(intro, loop) })
	step("intro.rtsx mounts nothing, on disk", "", func() { c.WriteFile(intro, plain) })
	step("intro.rtsx deleted on disk", missing, func() { c.RemoveFile(intro) })

	step("intro.rtsx opened, never saved", "", func() { c.OpenAs(intro, plain) })
	step("the buffer mounts #page", "2:25 segment-self", func() { c.Change(intro, loop) })
	if got := strings.Join(lsptest.Lines(c.Diagnostics(intro)), ", "); got != "2:24 segment-self" {
		t.Errorf("the buffer itself: %s", got)
	}
	step("the buffer mounts nothing", "", func() { c.Change(intro, plain) })
	step("the buffer closed", missing, func() { c.Close(intro) })

	step("page.tsx created", "1:1 ambiguous-module, "+missing, func() { c.WriteFile("src/page.tsx", "export const x = 1;\n") })
	step("page.tsx deleted", missing, func() { c.RemoveFile("src/page.tsx") })
}

// diagnostics.md, *References*: a file is its lister's — the project whose
// tsconfig names it, not one that only imports it. In the server too: the
// document's project is the app, strict, though the tests' project —
// referenced first, and not strict — holds the file as well. Whichever
// document was opened first; and hover is that project's too.
func TestListerIsTheDocumentsProject(t *testing.T) {
	files := lsptest.With(checktest.Get("references-order").Files, map[string]string{
		"test/helper.rtsx": "import { Page } from \"../src/page\";\nimport { found } from \"../src/found\";\nexport const helper = (o) => <Page items={o} />;\nexport const mine = found;\n",
		"src/found.rtsx":   "export const found = [1].find((n) => n > 0);\n",
	})
	const page, helper, found = "src/page.rtsx", "test/helper.rtsx", "src/found.rtsx"
	const strict = "1:24 TS7031, 2:26 TS7006"
	for _, order := range [][]string{{page, helper}, {helper, page}} {
		c := lsptest.Start(t, lsptest.Write(t, files), serve)
		for _, rel := range order {
			c.Open(rel)
			c.Diagnostics(rel)
		}
		if got := strings.Join(lsptest.Lines(c.Diagnostics(page)), ", "); got != strict {
			t.Errorf("opened %v: page.rtsx has %q, want the app's %q", order, got, strict)
		}
		// The tests' own file is the tests': an untyped parameter is fine.
		if got := strings.Join(lsptest.Lines(c.Diagnostics(helper)), ", "); got != "" {
			t.Errorf("opened %v: helper.rtsx has %q", order, got)
		}
		// One name, read in each document under its own project's options:
		// with strict null checks in the app, without in the tests.
		c.Open(found)
		if hover := c.Hover(found, c.At(found, "found", 1, 1)); !strings.Contains(hover, "const found: number | undefined") {
			t.Errorf("opened %v: hover in found.rtsx, the app's: %q", order, hover)
		}
		if hover := c.Hover(helper, c.At(helper, "mine", 1, 1)); !strings.Contains(hover, "const mine: number") || strings.Contains(hover, "undefined") {
			t.Errorf("opened %v: hover in helper.rtsx, the tests': %q", order, hover)
		}
	}
}
