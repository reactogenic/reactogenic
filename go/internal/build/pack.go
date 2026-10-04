package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// assets is the directory of the build's own files in the output, and
// reportFile the byte report in it — which is also how a directory is known
// to be a previous output (CheckOut).
const (
	assets     = "_rg"
	reportFile = assets + "/report.json"
)

// blob is the CSS or the JS of one page or of several: identical content is
// one blob (builder.md, *Packaging*).
type blob struct {
	kind    string // "css", "js"
	content string
	hash    string // of the content
	pages   []int  // the pages it serves, in their order
	file    bool   // delivered as a file; otherwise inlined in each page
}

// path is the blob's file, from the output's root: `_rg/page-1a2b3c4d.css`.
// The name says what made it — `site` for the control's one bundle, `page`
// otherwise — and never which page: a blob's URL depends on its content
// alone, so a page's bytes do not change with what another page ships.
func (b *blob) path(control bool) string {
	name := "page"
	if control {
		name = "site"
	}
	return assets + "/" + name + "-" + b.hash + "." + b.kind
}

// pack makes the blobs of a site: per kind, one for each distinct content,
// in the order of the first page that has it. The second result is, per
// page, the index of its CSS blob and of its JS blob; -1: the page has none.
func pack(site []built, inline string) (blobs []*blob, of [][2]int, err error) {
	byContent := map[string]int{} // kind and content → index in blobs
	byPath := map[string]string{} // kind and hash → content: two contents under one name
	of = make([][2]int, len(site))
	for i, p := range site {
		for k, content := range [2]string{p.css, p.js} {
			of[i][k] = -1
			if content == "" {
				continue
			}
			kind := [2]string{"css", "js"}[k]
			at, known := byContent[kind+"\x00"+content]
			if !known {
				sum := sha256.Sum256([]byte(content))
				b := &blob{kind: kind, content: content, hash: hex.EncodeToString(sum[:])[:8]}
				if other, taken := byPath[kind+b.hash]; taken && other != content {
					return nil, nil, fmt.Errorf("two %s blobs share the hash %s", kind, b.hash)
				}
				byPath[kind+b.hash] = content
				at = len(blobs)
				blobs, byContent[kind+"\x00"+content] = append(blobs, b), at
			}
			blobs[at].pages = append(blobs[at].pages, i)
			of[i][k] = at
		}
	}
	for _, b := range blobs {
		b.file = asFile(b, inline)
	}
	return blobs, of, nil
}

// asFile is the `--inline` rule (builder.md, *Packaging*). It is about
// caching, not size: a file costs a request's headers, so a blob one page
// uses is cheaper inline, and one many pages share is cheaper as a file from
// the second page on — when it is large enough to pay for the request.
func asFile(b *blob, inline string) bool {
	if !inlinable(b) {
		return true
	}
	switch inline {
	case InlineAlways:
		return false
	case InlineNever:
		return true
	}
	return len(b.pages) > 1 && (len(b.pages)-1)*len(b.content) > 1024
}

// inlinable: the content can stand inside its element. HTML ends a `<style>`
// at `</style` and a `<script>` at `</script`, wherever in the content they
// are, and reads a script that holds `<!--` by rules of its own. esbuild
// escapes the first two in what it prints; a blob that has any of them all
// the same is a file, whatever `--inline` says.
func inlinable(b *blob) bool {
	lower := strings.ToLower(b.content)
	if b.kind == "css" {
		return !strings.Contains(lower, "</style")
	}
	return !strings.Contains(lower, "</script") && !strings.Contains(lower, "<!--")
}

// tag is the element that delivers a blob to a page: a `<style>` or a
// `<script type="module">` with the content, or — for a file — a
// `<link rel="stylesheet">` or a `<script type="module" src>` to it, under
// the base.
func (b *blob) tag(base string, control bool) string {
	url := base + b.path(control)
	switch {
	case b.kind == "css" && b.file:
		return `<link rel="stylesheet" href="` + url + `">`
	case b.kind == "css":
		return "<style>" + b.content + "</style>"
	case b.file:
		return `<script type="module" src="` + url + `"></script>`
	}
	return `<script type="module">` + b.content + "</script>"
}

// write packages the site and writes it to opts.Out, which it empties first
// (builder.md, *Packaging*): the byte report, the blobs that are files, each
// page's `index.html`, and `public/` as it is.
func write(opts Options, site []built, static []string) (*Report, error) {
	blobs, of, err := pack(site, opts.Inline)
	if err != nil {
		return nil, err
	}
	documents := make([]string, len(site))
	bare := make([]string, len(site)) // the page without what packaging adds to it: its HTML alone
	for i, p := range site {
		var head, body string
		if at := of[i][0]; at >= 0 {
			head = blobs[at].tag(opts.Base, opts.NoSpecialize)
		}
		if at := of[i][1]; at >= 0 {
			body = blobs[at].tag(opts.Base, opts.NoSpecialize)
		}
		documents[i] = doctype + document(p.page.HTML, opts.Base, head, body)
		bare[i] = doctype + document(p.page.HTML, opts.Base, "", "")
	}
	bytes := byteReport(opts, site, blobs, of, documents, bare, static)
	encoded, err := bytes.JSON()
	if err != nil {
		return nil, err
	}

	if err := empty(opts.Out); err != nil {
		return nil, err
	}
	// The report first: a build that fails from here on leaves a directory
	// that is still known as the builder's, and the next one can empty it.
	if err := writeFile(opts.Out, reportFile, encoded); err != nil {
		return nil, err
	}
	for _, b := range blobs {
		if b.file {
			if err := writeFile(opts.Out, b.path(opts.NoSpecialize), []byte(b.content)); err != nil {
				return nil, err
			}
		}
	}
	for _, file := range static {
		content, err := os.ReadFile(filepath.Join(opts.public(), filepath.FromSlash(file)))
		if err != nil {
			return nil, err
		}
		if err := writeFile(opts.Out, file, content); err != nil {
			return nil, err
		}
	}
	for i, p := range site {
		if err := writeFile(opts.Out, output(p.page.Pathname), []byte(documents[i])); err != nil {
			return nil, err
		}
	}
	return bytes, nil
}

// output is a page's file, from the output's root: `<pathname>index.html`.
func output(pathname string) string {
	return strings.TrimPrefix(pathname, "/") + "index.html"
}

func writeFile(out, name string, content []byte) error {
	file := filepath.Join(out, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, content, 0o644)
}
