package lsp_test

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// The one mistake of typingApp, in a statement of its own on the first
// line: no edit below it reaches it.
const typingMistake = "1:65 TS2322"

// typingApp is the Vite test app (packages/vite/test/render) — every
// construct of the language in one realistic file — on the stand-in core
// and JSX types, so that it is checked without node_modules; and one type
// error, on its first line.
func typingApp(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range []string{"app.rtsx", "intro.rtsx"} {
		text, err := os.ReadFile("../../../packages/vite/test/render/src/" + name)
		if err != nil {
			t.Fatal(err)
		}
		files["src/"+name] = string(text)
	}
	const react = "import type { ReactNode } from \"react\";\n"
	if !strings.HasPrefix(files["src/app.rtsx"], react) {
		t.Fatal("the Vite test app no longer starts with its import of ReactNode")
	}
	// One line for one line: the positions stay those of the app.
	files["src/app.rtsx"] = "type ReactNode = string | JSX.Element | undefined; export const wrong: number = \"x\";\n" + strings.TrimPrefix(files["src/app.rtsx"], react)
	// As React's types: an element is a node, children are the `children`
	// prop, and a component returns a node.
	files["src/jsx.d.ts"] = `declare namespace JSX {
  interface Element { readonly $$typeof: symbol }
  type ElementType = string | ((props: any) => Element | string | number | boolean | null | undefined);
  interface ElementChildrenAttribute { children: {} }
  interface IntrinsicAttributes { key?: string | number | null }
  interface IntrinsicElements { [name: string]: any }
}`
	return lsptest.With(lsptest.Core, files)
}

// A syntax error, as TypeScript numbers them: the parser's (TS1xxx), JSX's
// (TS17xxx) and the one the parser reports among the 2000s.
func isSyntaxError(code int) bool {
	return 1000 <= code && code < 2000 || 17000 <= code && code < 18000 || code == 2657
}

// The names the transform generates, and the runtime helpers it calls: none
// is the author's, so none may be in a message.
var generatedName = regexp.MustCompile(`\b(isAssigned|renderSlot|slotArgs|slotEntry|slotProps|slotKey|noMatch|NOT_ASSIGNED|SLOT_KEY|_on|_args|_entry|_slot|_[A-Z]\w*_\w+)\b`)

// reworded are the names the reporting layer gives a TypeScript error
// (diagnostics.md, *Rewrites*), and the rule that asks the checker
// (`slot-conditional`). Every other name is the transpiler's own.
var reworded = map[string]bool{
	"undeclared-slot": true, "missing-slot": true, "slot-type": true, "content-not-allowed": true, "params-required": true,
	"no-values": true, "switch-missing-case": true, "segment-not-component": true, "segment-props": true,
	"segment-root-props": true, "slot-args-missing": true, "slot-list": true, "slot-key-no-args": true,
	"slot-key-inline": true, "slot-no-args": true, "slot-conditional": true,
}

// ide.md, *Tolerance*, rules 3 and 4: while a top-level statement does not
// parse, it shows its syntax errors and nothing else. The virtual text of a
// file being typed is broken in its own way — what the passes lowered
// around the half-typed construct is not what the author will have written
// — and TypeScript's errors about it would land on lines the author is not
// touching: reworded in slot terms that are not true (`$Title` "is a
// list"), naming helpers the author never wrote (`isAssigned`), or raw
// errors about the lowering. The transpiler's own, on a recovered tree,
// are no better where they depend on what is around: `$Title` "is a slot
// tag" in the scope an unclosed brace moved it to.
//
// Typed here, with the rest of the file as it is: an attribute, character
// by character, at the end of every opening tag; and a child on a new line
// under every opening tag that ends its line. On the lines that are not
// being typed, every state shows
//
//   - syntax errors (TypeScript reports those of a .tsx file as far away);
//   - the file's one mistake, in another statement: nothing is hidden there;
//   - one diagnostic that is true: a second root of the segment the app
//     mounts makes the first a duplicate;
//
// and nothing else.
func TestTypingShowsNothingFalse(t *testing.T) {
	files := typingApp(t)
	c := start(t, files)
	const app = "src/app.rtsx"
	c.Open(app)
	if got := strings.Join(lsptest.Lines(c.Diagnostics(app)), ", "); got != typingMistake {
		t.Fatalf("the fixture has %q, want its one mistake: %s", got, typingMistake)
	}
	lines := strings.Split(files[app], "\n")
	attributes := []string{`x={y}`, `x="a b"`, `&a={b}`, `&&a`, `{ a }`, `#about`, `slot={$X}`}
	children := []string{`<$Title className="c">t</$Title>`, `<$Case is="x">t</$Case>`, `{cond && <b>t</b>}`, `<Match on={x} { value }>t</Match>`, `<section #intro />`}
	// Every prefix is a state: 2516 of them. A short run, and one under the
	// race detector — CI's, where the whole run took two minutes more —
	// takes every third.
	stride := 1
	if testing.Short() || raceDetector {
		stride = 3
	}

	states, stopped, failures := 0, 0, 0
	kinds := map[string]int{}
	fail := func(format string, args ...any) {
		if failures++; failures <= 12 {
			t.Errorf(format, args...)
		}
	}
	try := func(what, typed, text string, edited int) {
		states++
		c.Change(app, text)
		mistake := false
		for _, d := range c.Diagnostics(app) {
			if d.Range.Start.Line == edited {
				continue
			}
			if d.String() == typingMistake {
				mistake = true
				continue
			}
			code := strings.Trim(string(d.Code), `"`)
			number, err := strconv.Atoi(code)
			kind, allowed := "", false
			switch {
			case generatedName.MatchString(d.Message):
				kind = "a generated name"
			case err == nil && isSyntaxError(number):
				kinds["syntax errors"]++
				continue
			case err == nil:
				kind = "TS" + code
			case reworded[code]:
				kind = "TypeScript's, reworded as " + code
			default:
				kind = "the transpiler's " + code
				allowed = code == "segment-duplicate" && strings.Contains(typed, "#intro")
			}
			if kinds[kind]++; !allowed {
				fail("%s, typed %q: %s on line %d, which is not being typed — %s %s", what, typed, kind, d.Range.Start.Line+1, d, d.Message)
			}
		}
		// Rule 4's other half is untouched: a stopped file — code left out,
		// a construct as written — shows nothing of TypeScript's.
		_, f := mapper.Transform(c.Root+"/"+app, text, func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		})
		if f.Stopped {
			stopped++
		}
		if mistake == f.Stopped {
			fail("%s, typed %q: the mistake of line 1, in another statement, is shown: %v; the file is stopped: %v", what, typed, mistake, f.Stopped)
		}
	}
	opening := regexp.MustCompile(`<[$A-Za-z][^<>]*?(\s*/)?>`)
	for i, line := range lines {
		m := opening.FindStringSubmatchIndex(line)
		trimmed := strings.TrimSpace(line)
		if m == nil || strings.HasPrefix(trimmed, "</") || strings.Contains(line, "=>") {
			continue
		}
		end := m[1] - 1 // before `>`, or before ` />`
		if m[2] >= 0 {
			end = m[2]
		}
		what := fmt.Sprintf("an attribute in line %d (%.40s)", i+1, trimmed)
		for _, attribute := range attributes {
			for k := 1; k < len(attribute); k += stride { // the finished attribute is not a half-typed state
				typed := " " + attribute[:k]
				text := strings.Join(lines[:i], "\n") + "\n" + line[:end] + typed + line[end:] + "\n" + strings.Join(lines[i+1:], "\n")
				try(what, typed, text, i)
			}
		}
		if strings.HasSuffix(trimmed, ">") && !strings.HasSuffix(trimmed, "/>") && !strings.Contains(line, "</") {
			indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " "))+2)
			what := fmt.Sprintf("a child under line %d (%.40s)", i+1, trimmed)
			for _, child := range children {
				for k := 1; k < len(child); k += stride {
					text := strings.Join(lines[:i+1], "\n") + "\n" + indent + child[:k] + "\n" + strings.Join(lines[i+1:], "\n")
					try(what, child[:k], text, i+1)
				}
			}
		}
	}
	if states < 500 {
		t.Errorf("only %d states: the app's tags are not found", states)
	}
	var summary []string
	for kind, n := range kinds {
		summary = append(summary, fmt.Sprintf("%d %s", n, kind))
	}
	sort.Strings(summary)
	t.Logf("%d states, %d of them stopped, %d failures; off the typed line: %s", states, stopped, failures, strings.Join(summary, "; "))

	// The file as it was: nothing was left behind.
	c.Change(app, files[app])
	if got := strings.Join(lsptest.Lines(c.Diagnostics(app)), ", "); got != typingMistake {
		t.Errorf("the app again: %q", got)
	}
}
