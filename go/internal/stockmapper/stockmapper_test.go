package stockmapper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/conformance"
)

// host is the other end of the protocol, as TypeScript speaks it: framed
// JSON-RPC over two pipes, string ids. Whatever the mapper writes must be a
// framed message: anything else fails the read.
type host struct {
	t      *testing.T
	in     *io.PipeWriter
	out    *bufio.Reader
	served chan error
	next   int
}

type response struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func start(t *testing.T, opts Options) *host {
	t.Helper()
	serverIn, in := io.Pipe()
	out, serverOut := io.Pipe()
	h := &host{t: t, in: in, out: bufio.NewReader(out), served: make(chan error, 1)}
	go func() {
		h.served <- Serve(serverIn, serverOut, opts)
		serverOut.Close()
	}()
	t.Cleanup(func() { in.Close() })
	return h
}

func (h *host) send(method string, params any) string {
	h.t.Helper()
	h.next++
	id := fmt.Sprintf("api%d", h.next)
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		h.t.Fatal(err)
	}
	h.write(body)
	return id
}

func (h *host) write(body []byte) {
	h.t.Helper()
	if _, err := fmt.Fprintf(h.in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		h.t.Fatal(err)
	}
}

func (h *host) read() response {
	h.t.Helper()
	body, err := readMessage(h.out)
	if err != nil {
		h.t.Fatalf("reading a response: %v", err)
	}
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		h.t.Fatalf("response %s: %v", body, err)
	}
	if !strings.Contains(string(body), `"jsonrpc":"2.0"`) {
		h.t.Errorf("response without jsonrpc 2.0: %s", body)
	}
	return r
}

// call sends a request and returns its result.
func (h *host) call(method string, params any) json.RawMessage {
	h.t.Helper()
	id := h.send(method, params)
	r := h.read()
	if string(r.ID) != `"`+id+`"` {
		h.t.Fatalf("%s: answered %s, want %q", method, r.ID, id)
	}
	if r.Error != nil {
		h.t.Fatalf("%s: error %d %s", method, r.Error.Code, r.Error.Message)
	}
	return r.Result
}

func (h *host) transform(fileName, content string) transformResult {
	h.t.Helper()
	return decode(h.t, h.call("transform", transformParams{FileName: fileName, Content: content, ProjectHandle: "p:0"}))
}

func decode(t *testing.T, raw json.RawMessage) transformResult {
	t.Helper()
	var result transformResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("transform result %s: %v", raw, err)
	}
	return result
}

// end closes the host's side and returns what Serve returned.
func (h *host) end() error {
	h.t.Helper()
	h.in.Close()
	select {
	case err := <-h.served:
		if rest, _ := io.ReadAll(h.out); len(rest) > 0 {
			h.t.Errorf("bytes after the last response: %q", rest)
		}
		return err
	case <-time.After(10 * time.Second):
		h.t.Fatal("the mapper did not stop at the end of its input")
		return nil
	}
}

// files is an in-memory disk.
type files map[string]string

func (f files) options() Options {
	return Options{
		ReadFile:   func(p string) (string, bool) { text, ok := f[p]; return text, ok },
		FileExists: func(p string) bool { _, ok := f[p]; return ok },
	}
}

// spanMap checks a result as the compiler does before it accepts one — the
// map is valid for the two texts and covers the virtual text without gaps —
// and returns the map.
func spanMap(t *testing.T, result transformResult, content string) *rtsx.SpanMap {
	t.Helper()
	if result.Extension != ".tsx" {
		t.Errorf("virtual extension %q", result.Extension)
	}
	tuples, covered := make([][6]int32, 0, len(result.Mappings)), 0
	for _, s := range result.Mappings {
		if int(s[0]) != covered {
			t.Errorf("gap in the span map at virtual offset %d", covered)
		}
		covered = int(s[0] + s[1])
		tuples = append(tuples, s)
	}
	if covered != len(result.Text) {
		t.Errorf("the span map covers %d of %d bytes", covered, len(result.Text))
	}
	m := rtsx.NewSpanMap(tuples)
	if err := rtsx.ValidateSpanMap(m, result.Text, content); err != nil {
		t.Fatalf("span map: %v", err)
	}
	return m
}

// sourceOf maps the nth occurrence of needle in the virtual text back.
func sourceOf(t *testing.T, m *rtsx.SpanMap, virtual, needle string, nth int) (pos int, exact bool) {
	t.Helper()
	at := -1
	for i := 0; i < nth; i++ {
		next := strings.Index(virtual[at+1:], needle)
		if next < 0 {
			t.Fatalf("no occurrence %d of %q in:\n%s", nth, needle, virtual)
		}
		at += 1 + next
	}
	pos, _, exact = rtsx.SpanSource(m, at, at+len(needle))
	return pos, exact
}

func lines(ds []diagnostic, content string) []string {
	var out []string
	for _, d := range ds {
		line := 1 + strings.Count(content[:d.Start], "\n")
		col := d.Start - strings.LastIndex(content[:d.Start], "\n")
		out = append(out, fmt.Sprintf("%d:%d+%d %d %s", line, col, d.Length, d.Code, d.MessageText))
	}
	return out
}

// The four requests, as hostimpl.go sends them.
func TestHandshake(t *testing.T) {
	h := start(t, files{}.options())

	var init initializeResult
	if err := json.Unmarshal(h.call("initialize", map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}}), &init); err != nil {
		t.Fatal(err)
	}
	if init.PositionEncoding != "utf-8" || init.DiagnosticSource != "reactogenic" {
		t.Errorf("initialize: %+v", init)
	}
	// A static mapper: no configIdentity, no watchedFiles — the host rejects
	// either from a mapper that does not declare dynamicConfig.
	if open := h.call("openProject", map[string]any{"configFileName": "/proj/tsconfig.json", "projectHandle": "p:0", "compilerOptions": map[string]any{"strict": true}}); string(open) != "{}" {
		t.Errorf("openProject: %s", open)
	}
	if closed := h.call("closeProject", map[string]any{"projectHandle": "p:0"}); string(closed) != "null" {
		t.Errorf("closeProject: %s", closed)
	}

	// An unknown request is answered, with an error; the id comes back as it
	// was sent, whatever its type.
	h.write([]byte(`{"jsonrpc":"2.0","id":7,"method":"shutdown"}`))
	if r := h.read(); string(r.ID) != "7" || r.Error == nil || r.Error.Code != errMethodNotFound {
		t.Errorf("unknown method: %+v", r)
	}
	// A notification, a response and a broken message get no answer: the
	// next response is the next request's.
	h.write([]byte(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"api1"}}`))
	h.write([]byte(`{"jsonrpc":"2.0","id":"x","result":null}`))
	h.write([]byte(`{not json`))
	h.call("initialize", map[string]any{})

	if err := h.end(); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

const buttonSource = `import type { Slot } from "@reactogenic/core";
export function Button({ size, $Icon }: { size: number; $Icon?: Slot<{ className?: string }, { size: number }> }) {
  return <button><span slot={$Icon} &size /></button>;
}
`

const pageSource = `import { Button } from "./button";
export function Page() {
  const size = 2;
  return <main>
    <Button size>
      <$Icon className="i" { size }>{size}</$Icon>
    </Button>
  </main>;
}
`

// A slot and a shorthand: the virtual text is the transpiler's, the map
// carries copied code back exactly and generated code to its construct, and
// the extensionless import of an .rtsx sibling is explicit.
func TestTransform(t *testing.T) {
	disk := files{"/proj/src/button.rtsx": buttonSource, "/proj/src/page.rtsx": pageSource}
	h := start(t, disk.options())
	h.call("initialize", map[string]any{})
	h.call("openProject", map[string]any{"configFileName": "/proj/tsconfig.json", "projectHandle": "p:0", "compilerOptions": map[string]any{}})

	result := h.transform("/proj/src/page.rtsx", pageSource)
	m := spanMap(t, result, pageSource)
	for _, want := range []string{`from "./button.rtsx";`, `size={size}`, `$Icon={{ className: "i", children: ({ size }) => `} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("the virtual text lacks %q:\n%s", want, result.Text)
		}
	}
	if len(result.Diagnostics) != 0 || result.DiagnosticDirectives != nil {
		t.Errorf("diagnostics %+v, directives %+v", result.Diagnostics, result.DiagnosticDirectives)
	}

	// Copied: the binding in the slot body, at its own place.
	body := strings.Index(result.Text, "=> size") + len("=> ")
	if pos, _, exact := rtsx.SpanSource(m, body, body+len("size")); !exact || pos != strings.Index(pageSource, "{size}</$Icon>")+1 {
		t.Errorf("the slot body maps to %d (exact: %v)", pos, exact)
	}
	// Copied: the slot's name, from its tag.
	if pos, exact := sourceOf(t, m, result.Text, "$Icon", 1); !exact || pos != strings.Index(pageSource, "$Icon") {
		t.Errorf("the slot name maps to %d (exact: %v)", pos, exact)
	}
	// Both copies of the shorthand come from its one token.
	shorthand := strings.Index(result.Text, "size={size}")
	for _, copy := range []int{shorthand, shorthand + len("size={")} {
		if pos, _, exact := rtsx.SpanSource(m, copy, copy+len("size")); !exact || pos != strings.Index(pageSource, "<Button size")+len("<Button ") {
			t.Errorf("the shorthand copy at %d maps to %d (exact: %v)", copy, pos, exact)
		}
	}
	// Generated: the extension written into the import lands on the
	// specifier, as a whole; the specifier itself stays exact.
	specifier := strings.Index(pageSource, `"./button"`)
	if pos, exact := sourceOf(t, m, result.Text, ".rtsx", 1); exact || pos != specifier {
		t.Errorf("the inserted extension maps to %d (exact: %v), want the specifier at %d", pos, exact, specifier)
	}
	if pos, exact := sourceOf(t, m, result.Text, "./button", 1); !exact || pos != specifier+1 {
		t.Errorf("the specifier maps to %d (exact: %v)", pos, exact)
	}

	// The same file, transformed again: the same answer.
	if again := h.transform("/proj/src/page.rtsx", pageSource); again.Text != result.Text {
		t.Errorf("a second transform differs")
	}
	if err := h.end(); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// ide.md, *The engine*: `./button` finds button.rtsx after every built-in
// extension — in the virtual text here, since stock TypeScript does not look.
func TestExplicitImports(t *testing.T) {
	disk := files{
		"/proj/src/button.rtsx":   "",
		"/proj/src/card.rtsx":     "",
		"/proj/src/card.ts":       "", // a built-in sibling wins
		"/proj/src/ui/index.rtsx": "",
		"/proj/src/lib/index.ts":  "",
		"/proj/src/lib.rtsx":      "", // a file before a directory
		// A package.json that names its entry is TypeScript's business; one
		// that names none leaves the directory to its index.
		"/proj/src/pkg/package.json":   `{ "name": "pkg", "types": "./entry.d.ts" }`,
		"/proj/src/pkg/index.rtsx":     "",
		"/proj/src/named/package.json": `{ "name": "named" }`,
		"/proj/src/named/index.rtsx":   "",
		"/proj/src/index.rtsx":         "",
		"/proj/src/deep/inner.rtsx":    "",
		"/proj/shared/panel.rtsx":      "",
		"/proj/shared/exact/entry.tsx": "",
		"/proj/src/styles.css":         "",
	}
	h := start(t, disk.options())
	h.call("openProject", map[string]any{"configFileName": "/proj/tsconfig.json", "projectHandle": "p:0", "compilerOptions": map[string]any{
		"configFilePath": "/proj/tsconfig.json",
		"paths": map[string]any{
			"@/*": []string{"./src/*"}, "@shared/*": []string{"./missing/*", "./shared/*"}, "entry": []string{"./shared/exact/entry.tsx"},
			// Aliases the extension cannot be appended to: exact ones, to a
			// file and to a directory, and a pattern with text after its `*`.
			"@button": []string{"./src/button"}, "@ui": []string{"./src/ui"}, "@card": []string{"./src/card"},
			"kit/*/mod": []string{"./src/*"}, "@panel": []string{"./missing/panel", "./shared/panel"},
		},
	}})
	source := `import a from "./button";
import b from './card';
import c from "./ui";
import d from "./lib";
import e from "./pkg";
import f from ".";
import g from "./ui/";
import h from "./button.rtsx";
import i from "./nowhere";
import j from "@/button";
import k from "@shared/panel";
import l from "entry";
import m from "react";
import "./styles.css";
export * from "./deep/inner";
export type T = import("./button").T;
export const lazy = () => import("../src/button");
import n from "./named";
import o from "@button";
import p from '@ui';
import q from "@card";
import r from "kit/button/mod";
import s from "kit/ui/mod";
import u from "kit/deep/inner/mod";
import v from "@panel";
import w from "@/named";
declare module "./button" { interface Extra { x: number } }
declare module "@button" { interface Extra { y: number } }
declare module "react" { interface Extra { z: number } }
declare global { interface Window { page: true } }
`
	result := h.transform("/proj/src/page.rtsx", source)
	want := `import a from "./button.rtsx";
import b from './card';
import c from "./ui/index.rtsx";
import d from "./lib.rtsx";
import e from "./pkg";
import f from "./index.rtsx";
import g from "./ui/index.rtsx";
import h from "./button.rtsx";
import i from "./nowhere";
import j from "@/button.rtsx";
import k from "@shared/panel.rtsx";
import l from "entry";
import m from "react";
import "./styles.css";
export * from "./deep/inner.rtsx";
export type T = import("./button.rtsx").T;
export const lazy = () => import("../src/button.rtsx");
import n from "./named/index.rtsx";
import o from "./button.rtsx";
import p from './ui/index.rtsx';
import q from "@card";
import r from "./button.rtsx";
import s from "./ui/index.rtsx";
import u from "./deep/inner.rtsx";
import v from "../shared/panel.rtsx";
import w from "@/named/index.rtsx";
declare module "./button.rtsx" { interface Extra { x: number } }
declare module "./button.rtsx" { interface Extra { y: number } }
declare module "react" { interface Extra { z: number } }
declare global { interface Window { page: true } }
`
	if result.Text != want {
		t.Errorf("virtual text:\n%s\nwant:\n%s", result.Text, want)
	}
	m := spanMap(t, result, source)
	// A replaced alias is an atom on the specifier the author wrote, quotes
	// and all: an error there lands on `"@button"`.
	alias := strings.Index(source, `"@button"`)
	aliasEnd := alias + len(`"@button"`)
	at := strings.Index(result.Text, `import o from "./button.rtsx"`) + len(`import o from `)
	if pos, end, exact := rtsx.SpanSource(m, at+1, at+1+len(`./button.rtsx`)); exact || pos != alias || end != aliasEnd {
		t.Errorf("the replaced alias maps to [%d,%d) (exact: %v), want the specifier [%d,%d)", pos, end, exact, alias, aliasEnd)
	}
	if pos, end, _ := rtsx.SpanSource(m, at, at+len(`"./button.rtsx"`)); pos != alias || end != aliasEnd {
		t.Errorf("the specifier of the replaced alias maps to [%d,%d), want [%d,%d)", pos, end, alias, aliasEnd)
	}
	// From another directory the replacement climbs.
	if deep := h.transform("/proj/src/deep/page.rtsx", "import o from \"@button\";\nimport v from \"@panel\";\n"); deep.Text != "import o from \"../button.rtsx\";\nimport v from \"../../shared/panel.rtsx\";\n" {
		t.Errorf("from src/deep:\n%s", deep.Text)
	}

	// Without the project — a handle the mapper was not told about — the
	// relative imports are still explicit; the aliases are not.
	loose := decode(t, h.call("transform", transformParams{FileName: "/proj/src/page.rtsx", Content: source, ProjectHandle: "unknown"}))
	if !strings.Contains(loose.Text, `"./button.rtsx"`) || !strings.Contains(loose.Text, `"@/button"`) {
		t.Errorf("without a project:\n%s", loose.Text)
	}
	// A closed project forgets its aliases.
	h.call("closeProject", map[string]any{"projectHandle": "p:0"})
	if closed := h.transform("/proj/src/page.rtsx", source); !strings.Contains(closed.Text, `"@/button"`) {
		t.Errorf("after closeProject:\n%s", closed.Text)
	}
}

// What replaces an alias names the file from the importing file's directory,
// as the host writes paths: `/` everywhere, a drive letter on Windows.
func TestRelativeSpecifier(t *testing.T) {
	for _, c := range [][3]string{
		{"/proj/src", "/proj/src/button.rtsx", "./button.rtsx"},
		{"/proj/src", "/proj/src/ui/index.rtsx", "./ui/index.rtsx"},
		{"/proj/src/deep/er", "/proj/src/button.rtsx", "../../button.rtsx"},
		{"/proj/src", "/shared/panel.rtsx", "../../shared/panel.rtsx"},
		{"/", "/button.rtsx", "./button.rtsx"},
		{"C:/proj/src", "C:/proj/shared/panel.rtsx", "../shared/panel.rtsx"},
		{"C:/proj/src", "D:/shared/panel.rtsx", "D:/shared/panel.rtsx"}, // no common root: the path itself
	} {
		if got := relativeSpecifier(c[0], c[1]); got != c[2] {
			t.Errorf("relativeSpecifier(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// An .rtsx file is always a module: one without imports or exports says so
// in its virtual text.
func TestAlwaysAModule(t *testing.T) {
	h := start(t, files{"/proj/src/button.rtsx": ""}.options())
	source := "const answer = 42;\n"
	result := h.transform("/proj/src/loose.rtsx", source)
	if result.Text != source+"\nexport {};\n" {
		t.Errorf("virtual text: %q", result.Text)
	}
	spanMap(t, result, source)
	// As a module, its `declare module` is an augmentation — of an .rtsx
	// module, by its full name.
	source = "declare module \"./button\" { interface Extra { x: number } }\n"
	result = h.transform("/proj/src/augment.rtsx", source)
	if result.Text != "declare module \"./button.rtsx\" { interface Extra { x: number } }\n\nexport {};\n" {
		t.Errorf("an augmentation alone: %q", result.Text)
	}
	spanMap(t, result, source)
	if empty := h.transform("/proj/src/empty.rtsx", ""); empty.Text != "\nexport {};\n" {
		t.Errorf("an empty file: %q", empty.Text)
	} else {
		spanMap(t, empty, "")
	}
	if module := h.transform("/proj/src/module.rtsx", "export const answer = 42;\n"); strings.Contains(module.Text, "export {}") {
		t.Errorf("a module: %q", module.Text)
	}
}

// Transpiler errors become the mapper's diagnostics: a number from the
// table, the name leading the message, at the source span. Warnings are not
// sent.
func TestTranspilerDiagnostics(t *testing.T) {
	disk := files{"/proj/src/intro.rtsx": "export default function Intro() { return null; }\n"}
	h := start(t, disk.options())
	source := `export function Page() {
  return <main>
    <$Icon />
    <section #intro>old</section>
    <section #gone />
  </main>;
}
`
	result := h.transform("/proj/src/page.rtsx", source)
	spanMap(t, result, source)
	got := strings.Join(lines(result.Diagnostics, source), "\n")
	// In source order; `segment-children` on line 4 is a warning.
	want := "3:5+9 101 orphan-slot: Slot must be immediate child of the component\n" +
		"5:14+5 306 segment-not-found: No segment `gone` next to `page.rtsx`: looked for `gone.rtsx`, `.tsx`, `.jsx`, `.ts`, `.js`"
	if got != want {
		t.Errorf("diagnostics:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(result.Text, `import _Section_intro from "./intro.rtsx";`) {
		t.Errorf("the virtual text lost the segment:\n%s", result.Text)
	}
}

// ide.md, *Tolerance*, rule 4: a construct that stays as written — an arg
// without an attachment, params on an intrinsic element — leaves a virtual
// text that is not TSX. The file is stopped: the transpiler's error is sent,
// and an ignore directive covers what TypeScript would say about the text.
func TestUnlowered(t *testing.T) {
	h := start(t, files{}.options())
	for source, want := range map[string]string{
		"declare const size: number;\nexport const a = <option &size />;\n":      "2:26+5 102 arg-without-slot: `&size` is an arg of a slot attachment; this element has no `slot={$X}`",
		"declare const size: number;\nexport const a = <div { size }>x</div>;\n": "2:23+8 103 params-on-html: Params are only allowed on components and slot elements",
	} {
		result := h.transform("/proj/src/page.rtsx", source)
		spanMap(t, result, source)
		if got := strings.Join(lines(result.Diagnostics, source), "\n"); got != want {
			t.Errorf("diagnostics:\n%s\nwant:\n%s", got, want)
		}
		if ds := result.DiagnosticDirectives; ds == nil || len(ds.Directives) != 1 || ds.Directives[0] != [5]int{0, 0, 0, len(result.Text), policyIgnore} {
			t.Errorf("%q: directives: %+v", source, result.DiagnosticDirectives)
		}
	}
}

// ide.md, *Tolerance*: a file being typed still has a virtual text and a
// valid map. Its syntax errors are TypeScript's to report while the virtual
// text has any, and the mapper's when the passes lowered them away.
func TestBrokenFile(t *testing.T) {
	disk := files{"/proj/src/button.rtsx": buttonSource}
	h := start(t, disk.options())
	source := `import { Button } from "./button";
export function Page({ user }: { user: { name: string } }) {
  const size = 2;
  return <main>
    <Button size><$Icon { size }>{size}</$Icon></Button>
    <p>{user.}</p>
  </main>;
}
`
	result := h.transform("/proj/src/page.rtsx", source)
	spanMap(t, result, source)
	// The file is not blank: completion after `user.` has a program to ask.
	for _, want := range []string{`from "./button.rtsx"`, "size={size}", "$Icon={{", "{user.}"} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("the virtual text lacks %q:\n%s", want, result.Text)
		}
	}
	// TS1003 is in the virtual text: not sent twice.
	if len(result.Diagnostics) != 0 {
		t.Errorf("diagnostics: %q", lines(result.Diagnostics, source))
	}
	virtual := rtsx.ParseTSX("/proj/src/page.rtsx.tsx", result.Text)
	if len(virtual.Diagnostics()) != 1 || virtual.Diagnostics()[0].Code() != 1003 {
		t.Errorf("the virtual text's own syntax errors: %d", len(virtual.Diagnostics()))
	}

	// One mistake, one report — also where TypeScript finds it in generated
	// text, at another place than the source parse does (`size` became
	// `size={size}`), or under another code (TS1003 for the source's
	// TS1145), or after the element a slot was lowered into.
	for name, body := range map[string]string{
		"a file cut off in a tag":  "import { Button } from \"./button\";\nexport function Page() {\n  const size = 2;\n  return <Button size",
		"a value still to come":    "import { Button } from \"./button\";\nexport function Page() {\n  const size = 2;\n  return <Button size= />;\n}\n",
		"a brace left open":        "import { Button } from \"./button\";\nexport const P = ({ a }: { a: string }) => <Button size={1}><$Icon>{a.</$Icon></Button>;\n",
		"a brace typed into a tag": "import { Button } from \"./button\";\nexport function Page() {\n  const size = 2;\n  return <But{ton size />;\n}\n",
		"params half typed":        "import { Button } from \"./button\";\nexport function Page() {\n  const size = 2;\n  return <Button size><$Icon { si>{size}</$Icon></Button>;\n}\n",
	} {
		result := h.transform("/proj/src/typing.rtsx", body)
		spanMap(t, result, body)
		if got := syntaxErrors(result); len(got) != 0 {
			t.Errorf("%s: the mapper sends %q", name, lines(got, body))
		}
		if virtual := rtsx.ParseTSX("/proj/src/typing.rtsx.tsx", result.Text); len(virtual.Diagnostics()) == 0 {
			t.Errorf("%s: nobody reports the mistake:\n%s", name, result.Text)
		}
	}

	// A syntax error in code a pass replaces — the children of a segment
	// root — is in no virtual text: the mapper reports it, under
	// TypeScript's number.
	disk["/proj/src/intro.rtsx"] = "export default function Intro() { return null; }\n"
	source = `export function Page({ user }: { user: { name: string } }) {
  return <section #intro>{user.}</section>;
}
`
	result = h.transform("/proj/src/lowered.rtsx", source)
	spanMap(t, result, source)
	if virtual := rtsx.ParseTSX("/proj/src/lowered.rtsx.tsx", result.Text); len(virtual.Diagnostics()) != 0 {
		t.Fatalf("the virtual text should parse:\n%s", result.Text)
	}
	if got := lines(result.Diagnostics, source); len(got) != 1 || got[0] != "2:32+1 1003 Identifier expected." {
		t.Errorf("diagnostics: %q", got)
	}
}

// A segment root's import of a `.tsx` / `.ts` file is the transpiler's, and
// TS5097 on it is not the author's. The import loses its extension: no
// TS5097, and every other error TypeScript has for that module still shows
// (a file that is not a module: TS2306, on the specifier). Only where a
// sibling would win the extensionless import does the extension stay, under
// an ignore directive — on that specifier, and nowhere else.
func TestSegmentImport(t *testing.T) {
	disk := files{
		"/proj/src/intro.tsx":   "export default function Intro() { return null; }\n",
		"/proj/src/outro.ts":    "export default function Outro() { return null; }\n",
		"/proj/src/outro.js":    "", // `./outro` is outro.ts all the same
		"/proj/src/plain.jsx":   "export default function Plain() { return null; }\n",
		"/proj/src/about.rtsx":  "export default function About() { return null; }\n",
		"/proj/src/helper.tsx":  "export const help = 1;\n",
		"/proj/src/widget.rtsx": "",
	}
	h := start(t, disk.options())
	source := `import { Widget } from "./widget";
import { help } from "./helper.tsx";
export function Page() {
  return <main><section #intro /><section #outro /><section #plain /><section #about />{help}<Widget /></main>;
}
`
	result := h.transform("/proj/src/page.rtsx", source)
	m := spanMap(t, result, source)
	// The author's own `./helper.tsx` keeps its extension, and its TS5097;
	// a .jsx and an .rtsx segment keep theirs, which no error is about.
	for _, want := range []string{
		`import _Section_intro from "./intro";`, `import _Section_outro from "./outro";`, `import _Section_plain from "./plain.jsx";`,
		`import _Section_about from "./about.rtsx";`, `from "./helper.tsx"`, `from "./widget.rtsx"`,
	} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("the virtual text lacks %q:\n%s", want, result.Text)
		}
	}
	if result.DiagnosticDirectives != nil {
		t.Errorf("directives: %+v", result.DiagnosticDirectives)
	}
	// An error on the generated specifier lands on the segment root.
	if pos, exact := sourceOf(t, m, result.Text, `"./intro"`, 1); exact || pos != strings.Index(source, "#intro") {
		t.Errorf("the specifier maps to %d (exact: %v), want `#intro` at %d", pos, exact, strings.Index(source, "#intro"))
	}

	// `intro.ts` next to the segment `intro.tsx`: TypeScript would take the
	// extensionless import to the .ts. The import stays explicit, under the
	// directive. The insertions before it are counted.
	disk["/proj/src/intro.ts"] = "export const other = 1;\n"
	result = h.transform("/proj/src/page.rtsx", source)
	spanMap(t, result, source)
	if result.DiagnosticDirectives == nil || len(result.DiagnosticDirectives.Directives) != 1 {
		t.Fatalf("directives: %+v\n%s", result.DiagnosticDirectives, result.Text)
	}
	if result.DiagnosticDirectives.UnusedExpectDirectiveDiagnostics == nil {
		t.Errorf("unusedExpectDirectiveDiagnostics must be an array")
	}
	d := result.DiagnosticDirectives.Directives[0]
	if got := result.Text[d[2]:d[3]]; got != `"./intro.tsx"` || d[4] != policyIgnore {
		t.Errorf("the directive covers %q, policy %d", got, d[4])
	}
	if !strings.Contains(result.Text, `import _Section_outro from "./outro";`) {
		t.Errorf("virtual text:\n%s", result.Text)
	}
	raw, _ := json.Marshal(result.DiagnosticDirectives)
	if !regexp.MustCompile(`^\{"unusedExpectDirectiveDiagnostics":\[\],"directives":\[\[0,0,\d+,\d+,0\]\]\}$`).Match(raw) {
		t.Errorf("wire form: %s", raw)
	}
}

// A panic inside a transform is that file's `internal` error, not the end
// of the process: the host never starts a mapper twice.
func TestPanic(t *testing.T) {
	opts := files{}.options()
	opts.beforeTransform = func(fileName string) {
		if strings.HasSuffix(fileName, "boom.rtsx") {
			panic("boom")
		}
	}
	var log strings.Builder
	opts.Log = &log
	h := start(t, opts)

	source := "export const a = <$Icon />;\nexport const b = 1;\n"
	result := h.transform("/proj/src/boom.rtsx", source)
	spanMap(t, result, source)
	if result.Text != source {
		t.Errorf("the source should stand in as the virtual text: %q", result.Text)
	}
	if got := lines(result.Diagnostics, source); len(got) != 1 || got[0] != "1:1+27 1 internal: transpiler: boom" {
		t.Errorf("diagnostics: %q", got)
	}
	// Unlowered constructs: nothing TypeScript says about them is shown.
	if ds := result.DiagnosticDirectives; ds == nil || len(ds.Directives) != 1 || ds.Directives[0] != [5]int{0, 0, 0, len(source), policyIgnore} {
		t.Errorf("directives: %+v", result.DiagnosticDirectives)
	}
	if !strings.Contains(log.String(), "panic transforming /proj/src/boom.rtsx: boom") {
		t.Errorf("log: %q", log.String())
	}

	// The mapper is still there.
	if next := h.transform("/proj/src/fine.rtsx", "export const a = 1;\n"); next.Text != "export const a = 1;\n" || len(next.Diagnostics) != 0 {
		t.Errorf("after the panic: %+v", next)
	}
	if err := h.end(); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// Transforms arrive together and are answered in any order, each once.
func TestConcurrentTransforms(t *testing.T) {
	disk := files{"/proj/src/button.rtsx": buttonSource}
	release := make(chan struct{})
	opts := disk.options()
	// The first transform waits for the others to have arrived.
	opts.beforeTransform = func(fileName string) {
		if strings.HasSuffix(fileName, "/page0.rtsx") {
			<-release
		}
	}
	h := start(t, opts)

	const n = 64
	want := map[string]string{}
	for i := 0; i < n; i++ {
		source := strings.Replace(pageSource, "const size = 2", fmt.Sprintf("const size = %d", i), 1)
		id := h.send("transform", transformParams{FileName: fmt.Sprintf("/proj/src/page%d.rtsx", i), Content: source, ProjectHandle: "p:0"})
		want[`"`+id+`"`] = fmt.Sprintf("const size = %d;", i)
	}
	for i := 0; i < n; i++ {
		if i == n-1 {
			close(release) // every other answer is in: page0 was not in their way
		}
		r := h.read()
		marker, ok := want[string(r.ID)]
		if !ok {
			t.Fatalf("an answer to %s: not asked, or answered twice", r.ID)
		}
		delete(want, string(r.ID))
		if r.Error != nil {
			t.Fatalf("%s: %+v", r.ID, r.Error)
		}
		if (string(r.ID) == `"api1"`) != (i == n-1) {
			t.Errorf("answer %d of %d is %s: page0 (api1) is the one held back", i+1, n, r.ID)
		}
		if result := decode(t, r.Result); !strings.Contains(result.Text, marker) || !strings.Contains(result.Text, `"./button.rtsx"`) {
			t.Errorf("%s answered with another file's text", r.ID)
		}
	}
	if err := h.end(); err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// The host closes stdin and kills its child; a mapper behind the Node
// launcher is a grandchild and has only the end of input to go by.
func TestEndOfInput(t *testing.T) {
	// A transform still running at the end answers before Serve returns.
	entered, release := make(chan struct{}), make(chan struct{})
	opts := files{}.options()
	opts.beforeTransform = func(string) { close(entered); <-release }
	h := start(t, opts)
	id := h.send("transform", transformParams{FileName: "/proj/a.rtsx", Content: "export const a = 1;\n"})
	<-entered
	h.in.Close()
	close(release)
	if r := h.read(); string(r.ID) != `"`+id+`"` || r.Error != nil {
		t.Errorf("the last transform: %+v", r)
	}
	if err := h.end(); err != nil {
		t.Errorf("Serve at the end of input: %v", err)
	}

	// Input that ends inside a message is an error, not a hang.
	for _, input := range []string{"Content-Length: 50\r\n\r\n{}", "Content-Length: 2\r\n", "\r\n{}"} {
		if err := Serve(strings.NewReader(input), io.Discard, Options{}); err == nil {
			t.Errorf("Serve(%q) returned no error", input)
		}
	}
	if err := Serve(strings.NewReader(""), io.Discard, Options{}); err != nil {
		t.Errorf("Serve on empty input: %v", err)
	}
}

// The framing the host reads: an exact `Content-Length` header, CRLF, the
// body's length in bytes.
func TestFraming(t *testing.T) {
	var out strings.Builder
	source := "export const s = \"…\";\n" // not ASCII: lengths are bytes
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "api1", "method": "transform", "params": transformParams{FileName: "/a.rtsx", Content: source}})
	input := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(body), body)
	if err := Serve(strings.NewReader(input), &out, Options{}); err != nil {
		t.Fatal(err)
	}
	header, rest, ok := strings.Cut(out.String(), "\r\n\r\n")
	if !ok || header != fmt.Sprintf("Content-Length: %d", len(rest)) {
		t.Fatalf("frame: %q", out.String())
	}
	var r response
	if err := json.Unmarshal([]byte(rest), &r); err != nil {
		t.Fatal(err)
	}
	result := decode(t, r.Result)
	if result.Text != source || len(result.Mappings) != 1 || result.Mappings[0][1] != int32(len(source)) {
		t.Errorf("result: %+v", result)
	}
}

// codes lists every error the transpiler can report, by a number below
// TypeScript's own.
func TestCodeTable(t *testing.T) {
	seen := map[int32]string{}
	for name, n := range codes {
		if n < 1 || n >= codeUnknown {
			t.Errorf("%s: %d is outside 1..%d", name, n, codeUnknown-1)
		}
		if other, dup := seen[n]; dup {
			t.Errorf("%d is both %s and %s", n, other, name)
		}
		seen[n] = name
	}
	for code, want := range map[string]int32{"orphan-slot": 101, "TS1005": 1005, "TS17002": 17002, "new-code": codeUnknown, "TS12": codeUnknown} {
		if got, _ := numericCode(code); got != want {
			t.Errorf("numericCode(%s) = %d, want %d", code, got, want)
		}
	}

	// Every code named where the transpiler reports one.
	reported := regexp.MustCompile(`(?:errorAt|report)\([^"\n]*"([a-z][a-z0-9-]*)"|Code:\s*"([a-z][a-z0-9-]*)"`)
	found := 0
	for _, dir := range []string{"../syntax", "../transpiler"} {
		sources, err := filepath.Glob(dir + "/*.go")
		if err != nil || len(sources) == 0 {
			t.Fatalf("%s: no sources (%v)", dir, err)
		}
		for _, source := range sources {
			if strings.HasSuffix(source, "_test.go") {
				continue
			}
			text, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range reported.FindAllStringSubmatch(string(text), -1) {
				found++
				if _, ok := codes[m[1]+m[2]]; !ok {
					t.Errorf("%s reports %s: not in the code table", source, m[1]+m[2])
				}
			}
		}
	}
	if found < 20 {
		t.Errorf("found %d report sites: the scan no longer matches the sources", found)
	}
}

func corpus(t *testing.T) []conformance.Case {
	t.Helper()
	spec, err := os.ReadFile("../../../specs/phase01/syntax.md")
	if err != nil {
		t.Fatal(err)
	}
	cases := conformance.ExtractSpec("syntax.md", string(spec))
	fixtures, err := conformance.LoadFixtures("../../../fixtures")
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, fixtures...)
	if len(cases) < 60 {
		t.Fatalf("%d cases", len(cases))
	}
	return cases
}

// caseServer is a mapper over the files of a corpus case, and its log: a
// transform that panics still answers (the source stands in, a valid
// result), so only the log tells.
func caseServer(c conformance.Case) (*server, *strings.Builder) {
	disk := files{}
	for name, text := range c.Files {
		disk["/case/"+name] = text
	}
	opts, log := disk.options(), &strings.Builder{}
	opts.Log = log
	return newServer(io.Discard, opts), log
}

// syntaxErrors are the source parse's errors among a result's diagnostics:
// those under TypeScript's own numbers.
func syntaxErrors(result transformResult) []diagnostic {
	var out []diagnostic
	for _, d := range result.Diagnostics {
		if d.Code >= 1000 {
			out = append(out, d)
		}
	}
	return out
}

// Over the conformance corpus the stock mapper answers every file with a
// map the compiler accepts, with no code outside the table, and without a
// panic behind the answer.
func TestCorpus(t *testing.T) {
	withDiagnostics := 0
	for _, c := range corpus(t) {
		s, log := caseServer(c)
		source := c.Files[c.Entry]
		result := s.transform(transformParams{FileName: "/case/" + c.Entry, Content: source}, nil)
		t.Run(c.ID, func(t *testing.T) {
			spanMap(t, result, source)
			if log.Len() > 0 {
				t.Errorf("the mapper logged:\n%s", log)
			}
			if len(result.Diagnostics) > 0 {
				withDiagnostics++
			}
			for _, d := range result.Diagnostics {
				if d.Code == codeUnknown || d.Start < 0 || d.Start+d.Length > len(source) {
					t.Errorf("diagnostic %+v", d)
				}
			}
		})
	}
	if withDiagnostics == 0 {
		t.Errorf("no case of the corpus has a transpiler error")
	}
}

// One mistake, one report (ide.md, *Stock TypeScript 7.1*): over typing-like
// mutants of the corpus — the text cut off, a character deleted, one typed —
// a result never carries a syntax error of the source parse while its
// virtual text has syntax errors of its own, which TypeScript reports; and a
// source that does not parse is never passed off as clean. No transform
// panics on the way, and every result is one the compiler accepts.
func TestSyntaxErrorsOnce(t *testing.T) {
	typed := []string{"<", ">", "{", "}", "&", "#", "$", ".", "=", "/", "\"", "("}
	mutants, broken, typeScripts, mappers, twice, silent, shown := 0, 0, 0, 0, 0, 0, 0
	for _, c := range corpus(t) {
		s, log := caseServer(c)
		src := c.Files[c.Entry]
		stride := max(1, len(src)/100)
		try := func(text string) {
			mutants++
			fileName := "/case/" + c.Entry
			result := s.transform(transformParams{FileName: fileName, Content: text}, nil)
			fail := func(format string, args ...any) {
				if shown++; shown <= 10 {
					t.Errorf("%s: %s\n--- source\n%s\n--- virtual\n%s", c.ID, fmt.Sprintf(format, args...), text, result.Text)
				}
			}
			if log.Len() > 0 {
				fail("the mapper logged:\n%s", log)
				log.Reset()
			}
			spanMap(t, result, text)
			if len(rtsx.ParseRTSX(fileName, text).Diagnostics()) == 0 {
				return
			}
			broken++
			virtual, fromMapper := rtsx.ParseTSX(fileName+".tsx", result.Text).Diagnostics(), syntaxErrors(result)
			switch {
			case len(virtual) > 0 && len(fromMapper) > 0:
				twice++
				fail("TypeScript reports %d syntax errors in the virtual text (the first: TS%d %s) and the mapper sends %q", len(virtual), virtual[0].Code(), rtsx.Message(virtual[0]), lines(fromMapper, text))
			case len(virtual) > 0:
				typeScripts++
			case len(fromMapper) > 0:
				mappers++
			case len(result.Diagnostics) == 0:
				silent++
				fail("the source has syntax errors and nobody reports one")
			}
		}
		for i := 0; i <= len(src); i += stride {
			try(src[:i]) // typing, top to bottom
			if i < len(src) {
				try(src[:i] + src[i+1:]) // a deleted character
			}
			try(src[:i] + typed[(i/stride)%len(typed)] + src[i:]) // a typed one
		}
	}
	if twice+silent > 0 {
		t.Errorf("%d mutants reported twice, %d not at all", twice, silent)
	}
	if mutants < 10000 || broken < mutants/3 || mappers == 0 {
		t.Errorf("%d mutants, %d with syntax errors, %d of them the mapper's to report: the corpus is not exercised", mutants, broken, mappers)
	}
	t.Logf("%d mutants, %d with syntax errors: %d reported by TypeScript, %d by the mapper", mutants, broken, typeScripts, mappers)
}

// A Content-Length no message has is a protocol error — not an allocation,
// not a panic.
func TestContentLengthBound(t *testing.T) {
	for _, length := range []string{"9000000000000000000", "1000000000000", "1073741825", "99999999999999999999"} {
		err := Serve(strings.NewReader("Content-Length: "+length+"\r\n\r\n{}"), io.Discard, Options{})
		if err == nil || !strings.Contains(err.Error(), "content mapper protocol: Content-Length") {
			t.Errorf("Content-Length %s: %v", length, err)
		}
	}
	// Below the bound, a length the input does not have is the end of the
	// input, found without the claimed allocation.
	if err := Serve(strings.NewReader("Content-Length: 1073741824\r\n\r\n{}"), io.Discard, Options{}); err != io.ErrUnexpectedEOF {
		t.Errorf("a gigabyte announced, two bytes sent: %v", err)
	}
}
