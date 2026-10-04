package lsptest

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// A server spells a path's `,` (and a space, a `#`, …) as VS Code does:
// percent-encoded. Rel compares paths.
func TestRel(t *testing.T) {
	c := &Client{Root: "/tmp/Test,_a b/001"}
	for uri, want := range map[string]string{
		"file:///tmp/Test%2C_a%20b/001/src/page.rtsx": "src/page.rtsx",
		"file:///tmp/Test,_a b/001/src/page.rtsx":     "src/page.rtsx", // as this client writes it
		"file:///tmp/Test%2C_a%20b/001":               "",
		"file:///tmp/Test%2C_a%20b/0012/x.ts":         "file:///tmp/Test%2C_a%20b/0012/x.ts", // not under the root
		"untitled:Untitled-1":                         "untitled:Untitled-1",
	} {
		if got := c.Rel(uri); got != want {
			t.Errorf("Rel(%q) = %q, want %q", uri, got, want)
		}
	}
}

type buffer struct{ bytes.Buffer }

func (*buffer) Close() error { return nil }

// A change on disk reaches the server only through a watcher it registered:
// none, or none for that path, and nothing is sent.
func TestWatchedFileNeedsAWatcher(t *testing.T) {
	sent := &buffer{}
	c := &Client{t: t, Root: "/p", w: sent, watchers: map[string][]*regexp.Regexp{}}
	c.watchedFile("src/x.ts", 2)
	if sent.Len() != 0 {
		t.Errorf("without a watcher: %s", sent.String())
	}
	c.watchers["files"] = []*regexp.Regexp{globRegexp("/p/src/**/*")}
	c.watchedFile("other/x.ts", 2)
	if sent.Len() != 0 {
		t.Errorf("a file no watcher matches: %s", sent.String())
	}
	c.watchedFile("src/x.ts", 2)
	if !strings.Contains(sent.String(), `"workspace/didChangeWatchedFiles"`) || !strings.Contains(sent.String(), `"file:///p/src/x.ts"`) {
		t.Errorf("a watched file: %q", sent.String())
	}
}

// The globs of the file watchers a server registers: a file, a directory
// with everything under it, a set of extensions — as a string, a URI, or a
// pattern relative to a base.
func TestWatcherGlobs(t *testing.T) {
	for _, tc := range []struct {
		glob    string // the JSON of the watcher's globPattern
		path    string
		matches bool
	}{
		{`"/p/tsconfig.json"`, "/p/tsconfig.json", true},
		{`"/p/tsconfig.json"`, "/p/src/tsconfig.json", false},
		{`"/p/src/**/*"`, "/p/src/util.ts", true},
		{`"/p/src/**/*"`, "/p/src/deep/er/util.ts", true},
		{`"/p/src/**/*"`, "/p/other/util.ts", false},
		{`"/p/**/*.{ts,tsx,rtsx}"`, "/p/src/page.rtsx", true},
		{`"/p/**/*.{ts,tsx,rtsx}"`, "/p/src/page.css", false},
		{`"/p/a.b/*.ts"`, "/p/aXb/x.ts", false}, // a `.` is a `.`
		{`"file:///p/a%2Cb/**/*"`, "/p/a,b/src/x.ts", true},
		{`{"baseUri": "file:///p/src", "pattern": "**/*"}`, "/p/src/x.ts", true},
		{`{"baseUri": {"uri": "file:///p", "name": "p"}, "pattern": "src/*.ts"}`, "/p/src/x.ts", true},
		{`{"baseUri": {"uri": "file:///p", "name": "p"}, "pattern": "src/*.ts"}`, "/p/src/deep/x.ts", false},
	} {
		if got := globRegexp(globPattern(json.RawMessage(tc.glob))).MatchString(tc.path); got != tc.matches {
			t.Errorf("%s on %s: %v, want %v", tc.glob, tc.path, got, tc.matches)
		}
	}
}
