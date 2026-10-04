package build

import (
	"cmp"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/pagecheck"
)

// A page's HTML is React's, to the byte (builder.md, *The record*), and
// packaging keeps it so: the page is not parsed and written again — it is
// read token by token for the few places packaging writes at, and everything
// else is copied.

// edit replaces src[pos:end] with text; pos == end inserts.
type edit struct {
	pos, end int
	text     string
}

// document is a page as it is written to its file (builder.md, *Packaging*):
// src, the page as rendered, with the base before every root-relative
// `href`, head — the page's stylesheet — at the end of `<head>`, and body —
// its script — at the end of `<body>`. base is normalised; "/" changes no
// link.
func document(src, base, head, body string) string {
	var edits []edit
	headAt, bodyAt, htmlEnd := -1, -1, -1
	prefix := strings.TrimSuffix(base, "/")
	z := html.NewTokenizer(strings.NewReader(src))
	for pos := 0; ; {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		raw := string(z.Raw())
		start := pos
		pos += len(raw)
		switch kind {
		case html.EndTagToken:
			switch name, _ := z.TagName(); string(name) {
			case "head":
				if headAt < 0 {
					headAt = start
				}
			case "body":
				bodyAt = start
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			if string(name) == "html" && htmlEnd < 0 {
				htmlEnd = pos
			}
			// `<base href>` is not a link, as for the page check.
			if prefix == "" || string(name) == "base" {
				continue
			}
			if at, end, quoted, ok := hrefOf(raw); ok {
				if value, changed := withBase(raw[at:end], prefix, quoted); changed {
					edits = append(edits, edit{start + at, start + end, value})
				}
			}
		}
	}
	// React writes `</head>` and `</body>`; a page without them is still a
	// page, and a browser puts a `<style>` it meets early in the head, a
	// `<script>` it meets late in the body.
	if headAt < 0 {
		headAt = max(htmlEnd, 0)
	}
	if bodyAt < 0 {
		bodyAt = len(src)
	}
	if head != "" {
		edits = append(edits, edit{headAt, headAt, head})
	}
	if body != "" {
		edits = append(edits, edit{bodyAt, bodyAt, body})
	}
	slices.SortStableFunc(edits, func(a, b edit) int { return cmp.Compare(a.pos, b.pos) })
	var out strings.Builder
	at := 0
	for _, e := range edits {
		out.WriteString(src[at:e.pos])
		out.WriteString(e.text)
		at = e.end
	}
	out.WriteString(src[at:])
	return out.String()
}

// hrefOf finds the value of a start tag's `href` — the first, as a browser
// takes it — in the tag's text: tag[at:end], inside its quotes if it has
// them.
func hrefOf(tag string) (at, end int, quoted, ok bool) {
	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r' }
	i := 1
	for i < len(tag) && !space(tag[i]) && tag[i] != '/' && tag[i] != '>' { // the tag's name
		i++
	}
	for {
		for i < len(tag) && (space(tag[i]) || tag[i] == '/') {
			i++
		}
		if i >= len(tag) || tag[i] == '>' {
			return 0, 0, false, false
		}
		name := i
		i++ // a name may start with `=`
		for i < len(tag) && !space(tag[i]) && tag[i] != '/' && tag[i] != '>' && tag[i] != '=' {
			i++
		}
		isHref := strings.EqualFold(tag[name:i], "href")
		for i < len(tag) && space(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			if isHref {
				return 0, 0, false, false // `href` without a value
			}
			continue
		}
		i++
		for i < len(tag) && space(tag[i]) {
			i++
		}
		quoted = i < len(tag) && (tag[i] == '"' || tag[i] == '\'')
		if quoted {
			quote := tag[i]
			i++
			at = i
			for i < len(tag) && tag[i] != quote {
				i++
			}
			end = i
			i = min(i+1, len(tag))
		} else {
			at = i
			for i < len(tag) && !space(tag[i]) && tag[i] != '>' {
				i++
			}
			end = i
		}
		if isHref {
			return at, end, quoted, true
		}
	}
}

// withBase is an `href` under `--base` (builder.md, *Packaging*): a
// root-relative link — the page check's own rule for one — gets the base
// before it, `/guide/` → `/docs/guide/`; `//host`, a URL with a scheme and a
// relative link are left alone. value is the attribute's text as written,
// its character references unresolved; prefix is the base without its last
// slash.
func withBase(value, prefix string, quoted bool) (string, bool) {
	link := html.UnescapeString(value)
	if _, rootRelative := pagecheck.RootRelative(link); !rootRelative {
		return value, false
	}
	// The link starts after what a URL parser strips: spaces and controls.
	lead := 0
	for lead < len(value) && value[lead] <= ' ' {
		lead++
	}
	if lead < len(value) && (value[lead] == '/' || value[lead] == '\\') {
		// As written, to the byte: only the base is new.
		return value[:lead] + prefix + value[lead:], true
	}
	// The slash is written as a character reference: the value is written
	// again.
	written := html.EscapeString(prefix + strings.TrimLeftFunc(link, func(r rune) bool { return r <= ' ' }))
	if !quoted {
		written = `"` + written + `"`
	}
	return written, true
}
