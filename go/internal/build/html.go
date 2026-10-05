package build

import (
	"cmp"
	"slices"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/cssprune"
	"github.com/reactogenic/reactogenic/go/internal/build/markup"
	"github.com/reactogenic/reactogenic/go/internal/build/pagecheck"
	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// A page's HTML is React's, to the byte (builder.md, *The record*), and
// packaging keeps it so: the page is not parsed and written again — it is
// read token by token for the few places packaging writes at, and everything
// else is copied. Those places: the base before a root-relative `href`, the
// page's stylesheet and script, and the text of a `<style>` element of the
// page's own, which is CSS of the page and pruned as its sheet is
// (*Packaging*).

// edit replaces src[pos:end] with text; pos == end inserts.
type edit struct {
	pos, end int
	text     string
}

// document is a page as it is written to its file (builder.md, *Packaging*):
// src, the page as rendered, with the base before every root-relative
// `href`, head — the page's stylesheet — at the end of `<head>`, and body —
// its script — at the end of `<body>`. base is normalised; "/" changes no
// link. rewrites are texts of src written anew: the page's own `<style>`
// elements, pruned (ownStyle.rewrite).
func document(src, base, head, body string, rewrites ...edit) string {
	edits := slices.Clone(rewrites)
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

// ownStyle is a `<style>` element of the page's own (builder.md, *The
// builder's own elements*): CSS of the page, pruned with its sheet and
// written back in place.
type ownStyle struct {
	pos, end int    // its text, in the page as rendered
	text     string // as a parser reads it: what the element's node holds
	css      bool   // its text is CSS: no `type`, or `text/css` (markup.CSS)
	// flat is the text in the form the pruner reads — nesting lowered, as
	// the page's bundled sheet has it; "": the element is not CSS, or
	// esbuild cannot read it. It is then left as it is, and read for the
	// names it uses.
	flat string
	// out is what is written in place of the text: what the page can use
	// of it, minified. nil: the text as it is.
	out *string
}

// ownStyles finds the `<style>` elements of a page as rendered, in document
// order, each with the place of its text — an element without text among
// them, so that they can be told from the parser's by their order
// (stylesOf) — and, for one that is CSS, the text as the pruner reads it.
// The reports are what esbuild says of a text (css-warning), at the page.
func ownStyles(page render.Page) (styles []ownStyle, reports []report.Report) {
	src := page.HTML
	warn := func(text string) {
		reports = append(reports, report.Report{Severity: report.Warning, File: page.File, Code: "css-warning", Message: "Page " + page.Path() + ": a `<style>` of the page: " + text})
	}
	z := html.NewTokenizer(strings.NewReader(src))
	open := false // the token before was a `<style>` start tag
	for pos := 0; ; {
		kind := z.Next()
		if kind == html.ErrorToken {
			flatten(styles, warn)
			return styles, reports
		}
		start := pos
		pos += len(z.Raw())
		token := z.Token()
		switch {
		case kind == html.StartTagToken && token.Data == "style":
			css := markup.CSS(&html.Node{Type: html.ElementNode, Data: token.Data, Attr: token.Attr})
			styles, open = append(styles, ownStyle{pos: pos, end: pos, css: css}), true
			continue
		case kind == html.TextToken && open:
			style := &styles[len(styles)-1]
			style.pos, style.end, style.text = start, pos, token.Data
		}
		open = false
	}
}

// flatten gives each own style that is CSS the form the pruner reads: what
// esbuild prints of it with nesting lowered. A text esbuild cannot read is
// left as it is, and said.
func flatten(styles []ownStyle, warn func(text string)) {
	for i := range styles {
		style := &styles[i]
		if !style.css || strings.TrimSpace(style.text) == "" {
			continue
		}
		result := api.Transform(style.text, api.TransformOptions{Loader: api.LoaderCSS, Supported: map[string]bool{"nesting": false}, LogLevel: api.LogLevelSilent})
		for _, message := range result.Warnings {
			warn(message.Text)
		}
		if len(result.Errors) > 0 {
			for _, message := range result.Errors {
				warn("it is not pruned: " + message.Text)
			}
			continue
		}
		style.flat = string(result.Code)
	}
}

// rewrites are the texts of the page's own `<style>` elements that are
// written anew.
func rewrites(styles []ownStyle) (edits []edit) {
	for _, style := range styles {
		if style.out != nil {
			edits = append(edits, edit{style.pos, style.end, *style.out})
		}
	}
	return edits
}

// stylesOf pairs the page's own `<style>` elements, as the driver found
// them in the page's text (ownStyles), with the elements of doc — the page
// as it is served, parsed — and returns those to prune: the ones that are
// CSS. own are the builder's elements in doc. The two are paired by their
// order and told apart by their text: when a parser sees other `<style>`
// elements than the text has — markup in an SVG `<style>` — none is
// returned, and each is left as it is: the pruner reads it for the names it
// uses, which is sound.
func stylesOf(doc *html.Node, own []*html.Node, styles []ownStyle) (pruned []*cssprune.Style, at []int) {
	var nodes []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" && !slices.Contains(own, n) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(nodes) != len(styles) {
		return nil, nil
	}
	for i, n := range nodes {
		var text strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.TextNode {
				return nil, nil
			}
			text.WriteString(c.Data)
		}
		if text.String() != styles[i].text {
			return nil, nil
		}
		if styles[i].flat != "" {
			pruned, at = append(pruned, &cssprune.Style{Node: n, CSS: styles[i].flat}), append(at, i)
		}
	}
	return pruned, at
}
