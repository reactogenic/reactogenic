package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// request runs one `virtual` request through Serve, as the plugin does: the
// request, then the end of input.
func request(t *testing.T, files ...TransformParams) []VirtualFile {
	t.Helper()
	params, err := json.Marshal(VirtualParams{Files: files})
	if err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(Request{ID: 7, Method: "virtual", Params: params})
	var out strings.Builder
	if err := Serve(strings.NewReader(string(line)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		ID     int
		Error  string
		Result VirtualResult
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if resp.ID != 7 || resp.Error != "" {
		t.Fatalf("response: %s", out.String())
	}
	return resp.Result.Files
}

// units is text as JavaScript indexes it.
func units(text string) []uint16 { return utf16.Encode([]rune(text)) }

// checkSpans: the tuples cover the virtual text without gaps, in UTF-16
// offsets, and every verbatim one holds the same text on both sides.
func checkSpans(t *testing.T, f VirtualFile, source string) {
	t.Helper()
	virtual, original := units(f.Text), units(source)
	at := int32(0)
	for _, s := range f.Spans {
		if s[0] != at || s[1] < 0 {
			t.Fatalf("%s: span %v does not start at %d", f.File, s, at)
		}
		at = s[0] + s[1]
		if s[2] < 0 || s[3] < 0 || int(s[2]+s[3]) > len(original) || int(at) > len(virtual) {
			t.Fatalf("%s: span %v is outside the text", f.File, s)
		}
		if s[4] == emit.KindVerbatim {
			v, o := string(utf16.Decode(virtual[s[0]:at])), string(utf16.Decode(original[s[2]:s[2]+s[3]]))
			if v != o {
				t.Errorf("%s: verbatim span %v: virtual %q, source %q", f.File, s, v, o)
			}
		} else if s[5] != 0 {
			t.Errorf("%s: atom %v has features", f.File, s)
		}
	}
	if int(at) != len(virtual) {
		t.Errorf("%s: spans end at %d, the text at %d", f.File, at, len(virtual))
	}
}

func TestVirtual(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "intro.rtsx"), []byte("export default function Intro() { return <p />; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := func(name string) string { return filepath.ToSlash(filepath.Join(dir, name)) }

	page := "import { Button } from \"./button\";\nexport function Page({ size }: { size: number }) {\n  return <Button size><$Icon className=\"i\" { size }>{size}</$Icon></Button>;\n}\n"
	// Text outside ASCII before the code: two UTF-16 units for the emoji
	// (four bytes), one for the letter (two bytes).
	wide := "// \U0001F600 é\nconst value = 1;\nexport const a = <Input value />;\n"
	// A file being typed: the tag is not closed.
	broken := "export function Page() {\n  return <div><p>;\n}\nexport const kept = 1;\n"
	script := "const alone = <p />;\n"
	mounter := "export function Home() {\n  return <section #intro />;\n}\n"

	files := request(t,
		TransformParams{File: path("page.rtsx"), Code: page},
		TransformParams{File: path("wide.rtsx"), Code: wide},
		TransformParams{File: path("broken.rtsx"), Code: broken},
		TransformParams{File: path("script.rtsx"), Code: script},
		TransformParams{File: path("home.rtsx"), Code: mounter},
		TransformParams{File: path("empty.rtsx"), Code: ""},
	)
	if len(files) != 6 {
		t.Fatalf("want 6 files, got %d", len(files))
	}
	for i, source := range []string{page, wide, broken, script, mounter, ""} {
		checkSpans(t, files[i], source)
	}

	// The text is the language server's mapper's: one transform.
	want, _ := mapper.Transform(path("page.rtsx"), page, func(string) bool { return false })
	if files[0].File != path("page.rtsx") || files[0].Text != want || files[0].Stopped {
		t.Errorf("page: %q\nwant %q", files[0].Text, want)
	}
	if !strings.Contains(files[0].Text, `<Button size={size} $Icon={{ className: "i", children: ({ size }) => size }} />`) {
		t.Errorf("page: %s", files[0].Text)
	}

	// UTF-16: `value` copied twice, both from offset 49 of the source as
	// JavaScript counts — the bytes say 52.
	if got := strings.Index(wide, "value />"); got != 52 {
		t.Fatalf("the source's byte offset: %d", got)
	}
	copies := 0
	for _, s := range files[1].Spans {
		if s[4] == emit.KindVerbatim && s[2] == 49 && s[3] == 5 {
			copies++
		}
	}
	if copies != 2 {
		t.Errorf("wide: %d copies of `value` at UTF-16 offset 49: %v", copies, files[1].Spans)
	}

	// Tolerant: a syntax error does not empty the module.
	if !strings.Contains(files[2].Text, "export function Page()") || !strings.Contains(files[2].Text, "export const kept = 1;") {
		t.Errorf("broken: %q", files[2].Text)
	}

	// An .rtsx file is always a module; the marker is an atom at the end.
	if !strings.HasSuffix(files[3].Text, "\nexport {};\n") || strings.Contains(files[0].Text, "export {}") {
		t.Errorf("script: %q", files[3].Text)
	}
	if last := files[3].Spans[len(files[3].Spans)-1]; last[4] != emit.KindAtom || int(last[2]) != len(script) || last[3] != 0 {
		t.Errorf("script: the marker's span %v", last)
	}
	if files[5].Text != "\nexport {};\n" {
		t.Errorf("empty: %q", files[5].Text)
	}

	// The names beside the file are read from disk: the segment is found.
	if !strings.Contains(files[4].Text, `from "./intro.rtsx"`) {
		t.Errorf("mounter: %q", files[4].Text)
	}
}

// One request, many files: what the plugin sends for a project's first
// program.
func TestVirtualBatch(t *testing.T) {
	var batch []TransformParams
	for i := 0; i < 200; i++ {
		batch = append(batch, TransformParams{File: "/app/c" + string(rune('a'+i%26)) + ".rtsx", Code: "const value = 1;\nexport const a = <Input value />;\n"})
	}
	files := request(t, batch...)
	if len(files) != len(batch) {
		t.Fatalf("want %d files, got %d", len(batch), len(files))
	}
	for i, f := range files {
		if f.File != batch[i].File || !strings.Contains(f.Text, "<Input value={value} />") {
			t.Fatalf("file %d: %+v", i, f)
		}
	}
}

func TestUTF16Offsets(t *testing.T) {
	if utf16Offsets("plain ascii") != nil {
		t.Error("ASCII text needs no index")
	}
	text := "a\U0001F600é中z" // 1 + 4 + 2 + 3 + 1 bytes; 1 + 2 + 1 + 1 + 1 units
	index := utf16Offsets(text)
	for _, tc := range [][2]int32{{0, 0}, {1, 1}, {2, 1}, {4, 1}, {5, 3}, {6, 3}, {7, 4}, {9, 4}, {10, 5}, {11, 6}, {99, 6}, {-1, 0}} {
		if got := index.at(tc[0]); got != tc[1] {
			t.Errorf("byte %d: UTF-16 offset %d, want %d", tc[0], got, tc[1])
		}
	}
}
