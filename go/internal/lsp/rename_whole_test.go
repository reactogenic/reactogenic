package lsp_test

import (
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// What TypeScript renames in part — in a .tsx file as well — is made whole
// or refused when the rename is the server's (ide.md, *Rename*): the tests of
// this file, each with the check that the reviewed build failed: the edit
// applied leaves a diagnostic.

// renamed asks for the rename and applies it; the refusal otherwise.
func renamed(t *testing.T, c *lsptest.Client, rel, needle string, offset int, to string) ([]string, error) {
	t.Helper()
	at := c.At(rel, needle, 1, offset)
	_, _, prepareErr := c.PrepareRename(rel, at)
	edit, err := c.Rename(rel, at, to)
	if (prepareErr != nil) != (err != nil || len(c.Edits(edit)) == 0) && prepareErr != nil {
		t.Errorf("`%s`: prepareRename refuses (%v), rename: %v %q", needle, prepareErr, err, c.Edits(edit))
	}
	if err != nil {
		return nil, err
	}
	c.Apply(edit)
	return c.Edits(edit), nil
}

func allProblems(c *lsptest.Client, docs ...string) []string {
	out := []string{}
	for _, rel := range docs {
		for _, d := range c.Diagnostics(rel) {
			out = append(out, rel+" "+d.String()+" "+d.Message)
		}
	}
	return out
}

// A string is not a name. TypeScript finds the strings of its type — in
// other functions, in the library's declarations — and not always the type
// that declares them (`is="loading"` alone, of the fixture: a rename that
// leaves TS2367 and `switch-missing-case`). It is refused, in any file; and
// no rename edits a file of the library.
func TestRenameOfAString(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	const refusal = `is a string, not a name`
	for _, from := range []struct {
		name, rel, needle string
		offset            int
	}{
		{"a case of a Switch", "src/page.rtsx", `is="loading"`, 5},
		{"the next case: the subject is narrowed there", "src/page.rtsx", `is="ready"`, 5},
	} {
		at := c.At(from.rel, from.needle, 1, from.offset)
		if edit, err := c.Rename(from.rel, at, "fresh"); err == nil || !strings.Contains(err.Error(), refusal) || !strings.Contains(err.Error(), "-32803") {
			t.Errorf("%s: rename: %v %q", from.name, err, c.Edits(edit))
		}
		if _, _, err := c.PrepareRename(from.rel, at); err == nil || !strings.Contains(err.Error(), refusal) {
			t.Errorf("%s: prepareRename: %v", from.name, err)
		}
	}
	// In the type that declares it, a string is no name to TypeScript at all.
	at := c.At("src/page.rtsx", `"ready" }`, 1, 2)
	if _, _, err := c.PrepareRename("src/page.rtsx", at); err == nil || !strings.Contains(err.Error(), "You cannot rename this element") {
		t.Errorf("a string in a type: prepareRename: %v", err)
	}
	if edit, err := c.Rename("src/page.rtsx", at, "fresh"); err != nil || len(c.Edits(edit)) != 0 {
		t.Errorf("a string in a type: %v %q", err, c.Edits(edit))
	}
	if got := problems(c); len(got) != 0 {
		t.Errorf("after the refusals: %q", got)
	}

	// With the DOM library, "a" is the name of an element in its
	// declarations: TypeScript's rename edits lib.dom.d.ts three times.
	lib := start(t, map[string]string{
		"tsconfig.json": strings.Replace(lsptest.TSConfig, `"lib": ["es2022"]`, `"lib": ["es2022", "dom"]`, 1),
		"src/plain.tsx": "export function pick(mode: \"a\" | \"b\") {\n  return mode === \"a\" ? 1 : 2;\n}\n",
	})
	lib.Open("src/plain.tsx")
	edit, err := lib.Rename("src/plain.tsx", lib.At("src/plain.tsx", `=== "a"`, 1, 5), "fresh")
	for _, line := range lib.Edits(edit) {
		if !strings.HasPrefix(line, "src/plain.tsx ") {
			t.Errorf("the rename edits %s", line)
		}
	}
	if err == nil || !strings.Contains(err.Error(), refusal) {
		t.Errorf("a string that the library has too: %v %q", err, lib.Edits(edit))
	}
	if _, _, err := lib.PrepareRename("src/plain.tsx", lib.At("src/plain.tsx", `=== "a"`, 1, 5)); err == nil || !strings.Contains(err.Error(), refusal) {
		t.Errorf("a string that the library has too: prepareRename: %v", err)
	}
}

// A slot whose name is no identifier is declared, and destructured, under a
// quoted key: `{ "$sub-item": $sub }`. TypeScript's search does not take the
// key of a binding pattern for a reference; the rename does.
func TestRenameQuotedSlot(t *testing.T) {
	const kit, page, plain = "src/kit.rtsx", "src/page.rtsx", "src/plain.tsx"
	files := lsptest.With(lsptest.Core, map[string]string{
		"src/jsx.d.ts": renameApp["src/jsx.d.ts"],
		kit: `import type { Slot } from "@reactogenic/core";
export interface CardProps {
  "$sub-item"?: Slot<{ children?: unknown }>;
  "aria-label"?: string;
}
export function Card({ "$sub-item": $sub, "aria-label": label }: CardProps) {
  return <article title={label}><b slot={$sub}>none</b></article>;
}
`,
		page: `import { Card } from "./kit";
export const page = (
  <Card aria-label="card">
    <$sub-item>one</$sub-item>
  </Card>
);
`,
		plain: `export function Row({ "sub-item": sub, wide }: { "sub-item"?: string; wide?: boolean }) {
  return <i title={sub}>{wide}</i>;
}
export const row = <Row sub-item="x" wide />;
`,
	})
	docs := []string{kit, page, plain}
	for _, tc := range []struct {
		name, rel, needle string
		offset            int
		to                string
		lines             []string
	}{
		{"from the opening tag", page, "<$sub-item", 3, "$sub-thing", []string{`"$sub-thing"?: Slot<`, `Card({ "$sub-thing": $sub, `, `<$sub-thing>one</$sub-thing>`}},
		{"from the closing tag, to an identifier", page, "</$sub-item", 4, "$Sub", []string{`"$Sub"?: Slot<`, `Card({ "$Sub": $sub, `, `<$Sub>one</$Sub>`}},
		{"from the declaration", kit, `"$sub-item"?`, 3, "$sub-thing", []string{`Card({ "$sub-thing": $sub, `, `<$sub-thing>one</$sub-thing>`}},
		{"an attribute that a prop declares", page, "aria-label", 2, "aria-name", []string{`"aria-name"?: string;`, `"aria-name": label }`, `<Card aria-name="card">`}},
		{"in a .tsx file", plain, `"sub-item"?`, 3, "sub-thing", []string{`Row({ "sub-thing": sub, wide }: { "sub-thing"?: string;`, `<Row sub-thing="x" wide />`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t, files)
			for _, rel := range docs {
				c.Open(rel)
			}
			if got := allProblems(c, docs...); len(got) != 0 {
				t.Fatalf("the fixture has diagnostics: %q", got)
			}
			edits, err := renamed(t, c, tc.rel, tc.needle, tc.offset, tc.to)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			all := c.Text(kit) + c.Text(page) + c.Text(plain)
			for _, line := range tc.lines {
				if !strings.Contains(all, line) {
					t.Errorf("no `%s` after the rename; edits %q", line, edits)
				}
			}
			if got := allProblems(c, docs...); len(got) != 0 {
				t.Errorf("after the rename: %q\n  edits: %q", got, edits)
			}
		})
	}
}

// The prop `children` is what an element's body is the value of, and a body
// has no name: with an element that has one, the rename of the prop is
// refused, naming the element — in an .rtsx or a .tsx file. Without one it
// is a rename like any other.
func TestRenameOfChildren(t *testing.T) {
	const kit, page, plain = "src/kit.rtsx", "src/page.rtsx", "src/plain.tsx"
	files := lsptest.With(lsptest.Core, map[string]string{
		"src/jsx.d.ts": renameApp["src/jsx.d.ts"],
		kit: `export function Box({ children, wide }: { children?: unknown; wide?: boolean }) {
  return <div hidden={wide}>{children}</div>;
}
export function Line(props: { children?: string }) {
  return <hr title={props.children} />;
}
`,
		page:  "import { Box, Line } from \"./kit\";\nexport const page = (\n  <main>\n    <Box wide>body</Box>\n    <Line children=\"x\" />\n  </main>\n);\n",
		plain: "import { Line } from \"./kit\";\nexport const plain = <Line></Line>;\n",
	})
	docs := []string{kit, page, plain}
	c := start(t, files)
	for _, rel := range docs {
		c.Open(rel)
	}
	if got := allProblems(c, docs...); len(got) != 0 {
		t.Fatalf("the fixture has diagnostics: %q", got)
	}
	const refusal = "page.rtsx:4:6: `<Box>` has a body, which is its `children`"
	at := c.At(kit, "children?: unknown", 1, 2)
	if edit, err := c.Rename(kit, at, "content"); err == nil || !strings.Contains(err.Error(), refusal) {
		t.Errorf("the prop of an element with a body: %v %q", err, c.Edits(edit))
	}
	if _, _, err := c.PrepareRename(kit, at); err == nil || !strings.Contains(err.Error(), refusal) {
		t.Errorf("prepareRename: %v", err)
	}
	// The binding that the prop is destructured into is another name.
	if edits, err := renamed(t, c, kit, "{ children, wide }", 3, "content"); err != nil || strings.Join(edits, ", ") != `src/kit.rtsx 1:23-1:31 "children: content", src/kit.rtsx 2:30-2:38 "content"` {
		t.Errorf("the binding: %v %q", err, edits)
	}
	// A body in a .tsx file refuses as well.
	c.Change(plain, strings.Replace(files[plain], "<Line></Line>", "<Line>text</Line>", 1))
	if edit, err := c.Rename(kit, c.At(kit, "children?: string", 1, 2), "content"); err == nil || !strings.Contains(err.Error(), "plain.tsx:2:23: `<Line>` has a body") {
		t.Errorf("a body in a .tsx file: %v %q", err, c.Edits(edit))
	}
	c.Change(plain, files[plain])
	// No element of `Line` has a body: its `children` is renamed, the
	// attribute with it.
	edits, err := renamed(t, c, kit, "children?: string", 2, "content")
	if err != nil || !strings.Contains(c.Text(page), `<Line content="x" />`) {
		t.Errorf("a prop that no body is the value of: %v %q", err, edits)
	}
	if got := allProblems(c, docs...); len(got) != 0 {
		t.Errorf("after the renames: %q", got)
	}
}

// The key of a binding pattern written as a string is no name to
// TypeScript where it stands: not offered from there (it is renamed from the
// property's other names: TestRenameQuotedSlot).
func TestRenameFromAQuotedKey(t *testing.T) {
	const rel = "src/x.rtsx"
	c := start(t, map[string]string{rel: "export function Row({ \"sub-item\": sub }: { \"sub-item\"?: string }) {\n  return <i title={sub} />;\n}\n"})
	c.Open(rel)
	at := c.At(rel, `"sub-item": sub`, 1, 3)
	if _, shown, err := c.PrepareRename(rel, at); err == nil || !strings.Contains(err.Error(), "You cannot rename this element") {
		t.Errorf("prepareRename: %q, %v", shown, err)
	}
	if edit, err := c.Rename(rel, at, "fresh"); err != nil || len(c.Edits(edit)) != 0 {
		t.Errorf("rename: %v %q", err, c.Edits(edit))
	}
}

// An attribute that nothing declares — `data-tone` on an element — is no
// name: renamed, it would be another attribute, and with typed elements an
// error. It is not offered.
func TestRenameOfAnUndeclaredAttribute(t *testing.T) {
	const rel = "src/x.rtsx"
	c := start(t, map[string]string{rel: "export function X({ tone }: { tone: string }) {\n  return <section data-tone={tone} aria-label=\"x\" />;\n}\n"})
	c.Open(rel)
	for _, needle := range []string{"data-tone", "aria-label"} {
		for _, offset := range []int{1, len(needle) - 1} {
			at := c.At(rel, needle, 1, offset)
			if _, shown, err := c.PrepareRename(rel, at); err == nil || !strings.Contains(err.Error(), "You cannot rename this element") {
				t.Errorf("`%s` at +%d: prepareRename: %q, %v", needle, offset, shown, err)
			}
			if edit, err := c.Rename(rel, at, "fresh"); err != nil || len(c.Edits(edit)) != 0 {
				t.Errorf("`%s` at +%d: rename: %v %q", needle, offset, err, c.Edits(edit))
			}
		}
	}
}

// A tag is a component or an intrinsic element by its first letter: a
// rename that changes it — `Box` to `box` — would leave tags that are no
// references to it. Refused, wherever the tag is.
func TestRenameAcrossTagKinds(t *testing.T) {
	c := startRename(t, lsptest.Options{})
	for _, tc := range []struct {
		name, rel, needle string
		offset            int
		to, want          string
	}{
		{"a component without slots, from its tag", "src/page.rtsx", "<Box>", 2, "box", "page.rtsx:21:10: `<Box>` would become an intrinsic element"},
		{"from its declaration", "src/ui.tsx", "Box(", 1, "box", "page.rtsx:21:10: `<Box>` would become an intrinsic element"},
		{"a tag in a .tsx file", "src/page.rtsx", "Page(", 1, "page", "main.tsx:2:21: `<Page>` would become an intrinsic element"},
		{"a name with a hyphen", "src/page.rtsx", "<Box>", 2, "my-box", "page.rtsx:21:10: `<Box>` would become an intrinsic element"},
	} {
		at := c.At(tc.rel, tc.needle, 1, tc.offset)
		if edit, err := c.Rename(tc.rel, at, tc.to); err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "-32803") {
			t.Errorf("%s: %v %q, want %q", tc.name, err, c.Edits(edit), tc.want)
		}
		if _, _, err := c.PrepareRename(tc.rel, at); err != nil {
			t.Errorf("%s: prepareRename: %v", tc.name, err) // it depends on the new name
		}
	}
	// The member of `<UI.Box>` is a property, whatever its case.
	edits, err := renamed(t, c, "src/page.rtsx", "<UI.Box", 5, "box")
	if err != nil || !strings.Contains(c.Text("src/page.rtsx"), "<UI.box wide>") {
		t.Errorf("the member of a tag: %v %q", err, edits)
	}
	if got := problems(c); len(got) != 0 {
		t.Errorf("after it: %q", got)
	}
}
