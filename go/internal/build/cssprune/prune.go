// Package cssprune removes from a page's CSS what the page cannot use
// (specs/phase02/builder.md, *CSS*): a rule stays only if some element of the
// page may match it, a custom property only if something reads it, a
// @keyframes only if an animation names it, a @position-try only if
// something names it.
//
// The input is the flat CSS esbuild's public API prints for a page — nesting
// lowered, minified or not. The output is that CSS with rules, selectors and
// declarations removed and nothing rewritten: what is kept is kept byte for
// byte, in order, so the driver can minify it again.
//
// Soundness is the requirement: a rule that is dropped could not have
// matched. What can change after load is "maybe" and never drops anything —
// every pseudo-class but :root and the logical ones; the attributes the
// browser writes by itself (`open`, `hidden`, `style`; `dir` on a text
// control, `controls` and `loop` on a player); and whatever the page's
// script names. State only a script can
// write — `aria-*`, `disabled`, `inert`, `data-state`, `checked`,
// `selected`, `value` — is the page's unless the script names it (dynamic;
// the owner's ruling on M, decisions.md). The table of builder.md is the
// contract; each row is a test here, and the corpus test checks every
// selector dropped against an independent matcher (cascadia).
//
// # What it assumes
//
//   - The page is served as it was parsed: the document given is the file
//     that is written — with the base in its links, and with the <style> or
//     <link> and the <script> the driver puts in it, which a rule may select
//     as any other element. Parse it with its doctype: the builder writes
//     `<!doctype html>`, which makes classes and ids case-sensitive. Without
//     exactly that doctype the page may be in quirks mode, and they are
//     matched whatever their case.
//   - The elements the builder put into the page are named (Options.Builder),
//     and are not the page's own: the builder's <script> is not "a script of
//     its own" — its text is Options.Script — and its <link> or <style> is
//     the sheet that is pruned, not another, and is not read: a sheet is not
//     pruned against itself.
//   - The page's built script is given with it (PruneWith): what it names
//     — a class, an id, an attribute, a custom property, an animation — is
//     "maybe" too, and a script that may change the tree leaves the page
//     unpruned. The script is asked before the page: a class the page has
//     is "maybe" as well once the script names it, or may write `class`
//     whole — it takes away as it adds, and `:not(.collapsed)` must not be
//     "no" for what `classList.toggle` will make of it. A script that sets
//     an element's text removes what the element held: `:has()` is never
//     "yes" then. A name the script computes (`"is-" + state`) is not seen:
//     a behaviour names what it writes (builder.md, *Behaviours*) — by the
//     attribute's name, or by a property that reflects it (names.attribute).
//     Without the script, nothing may write but the browser: the attributes
//     that are "maybe" whoever writes.
//   - A script of the page's own — a <script> the builder did not put there,
//     an event handler attribute, a `javascript:` URL (package markup) — is
//     not read: the page is not pruned. That is defence in depth: the driver
//     no longer gets here with one — such a page is shell-script, an error
//     of the page checks, and the build stops before any CSS is pruned
//     (builder.md, *Shell code in phase 2*). A caller of its own may.
//   - Nor is a page pruned that has a document of the site in a frame
//     (<iframe>, <object>, <embed>), whose script reaches the page as the
//     page's own would. A document that reaches it another way — one that
//     opened the page, or frames it — is not seen.
//   - Nor is the page pruned where the browser's tree is not the one
//     written — <template>, <noscript>, <selectedcontent>, markup inside a
//     <select>, an element the user edits (`contenteditable`).
//   - A stylesheet the builder did not bundle — a <link> of the page's own
//     whose `rel` has `stylesheet` (markup.Link: a hint or a relation is no
//     stylesheet), an @import — may read any custom property and name any
//     animation or @position-try: they all stay.
//   - A <style> of the page's own is CSS of the page: given with its text
//     (Options.Styles), it is pruned with the sheet — the same rules, the
//     same "maybe" — and what it names and what it defines count together
//     with the sheet's, in document order: a custom property the sheet
//     declares and a <style> reads stays, a layer a <style> in the head
//     orders is ordered for the sheet that follows it. One that is not
//     given is read for the names it uses, and left as it is.
//   - The sheet is valid where validity is positional: an @import or a
//     @namespace that follows a rule is dead, and stays dead — everything
//     before it is kept as it is.
//
// # What was checked in browsers
//
// The tables of selector.go (what every browser of the floor parses, so that
// a list may be trimmed around it) and the cases of the tests were run in
// Chromium 153 and WebKit 26.6: 3 377 selectors, none of those called known
// rejected; 465 pairs of page and sheet, computed styles equal before and
// after pruning but for the custom properties dropped. Not in Firefox, and
// not at the floor's own versions: plan.md, RGP2-050 has both.
//
// What a script takes away was run there too, built by the driver in four
// packagings: a class toggled and removed, an id written over, a text set
// over an element, a document of the site in a frame — the computed style of
// every element equal to the unpruned build's, as loaded and after the
// click that makes `.card:not(.collapsed)` match (plan.md, RGP2-020).
//
// # What it saves
//
// The corpus of the research (research/css.md): twelve design-system files
// in one bundle, four pages. Bytes after esbuild minified the pruned sheet
// again, as the driver does; raw and gzip are printed by TestCorpusBytes,
// brotli (quality 11) was measured apart with Node's zlib on the same
// sheets, because the standard library has none.
//
//	                          raw    gzip   brotli   of the bundle (brotli)
//	one bundle             29 383   5 786    5 134
//	index                   8 903   2 523    2 188    43%
//	changelog               6 988   2 198    1 918    37%
//	guide                  29 383   5 786    5 134   100%   a <template>: not pruned
//	  without it           19 574   4 322    3 838    75%
//	api                    29 383   5 786    5 134   100%   a <template>: not pruned
//	  without it           19 852   4 336    3 843    75%
//
// research/css.md's corrected figures for the same mode are 44%, 76%, 76%
// and 39% (index, guide, api, changelog) — there of a bundle that kept its
// nesting (4 892 B), with nested output, and with <template> as an open
// boundary. Here both sides are flat, and the two docs pages are measured
// with the template taken out, which is what phase 2 emits. The raw sizes
// agree with the prototype's to the byte where the inputs do (index 8 851 +
// the 52 B @keyframes it wrongly dropped; changelog 6 988).
//
// The second corpus (research/components.md: three components, four pages
// that each use all of them) keeps 98–100%: there is nothing to prune.
//
// One page costs about 1 ms (BenchmarkPrune: reading the 29 KB bundle and
// pruning it for the heaviest page), and a selector a number of steps
// proportional to the page (TestMatchLinear): `.a~li~li` on a list of 20 000
// items takes 12 ms.
package cssprune

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/markup"
)

// Stats is what Prune did, for the report (builder.md, *The report*). The
// bytes are the sheet's; the counts are of the sheet and of the page's own
// `<style>` elements that were pruned with it (Options.Styles), which also
// have a Source of their own (OwnStyle).
type Stats struct {
	BytesIn, BytesOut int
	Rules             int // style rules read
	RulesDropped      int // no selector of theirs may match, or no declaration was left
	Selectors         int // in those rules' lists
	SelectorsDropped  int
	Properties        int // custom property declarations dropped
	Keyframes         int // @keyframes dropped
	PositionTries     int // @position-try dropped
	// Unpruned: the sheet came back as it was. The page holds a <template>,
	// a <noscript>, a <selectedcontent>, a <select> with more than options
	// in it or an element the user edits; or a script of its own, or a
	// document of the site in a frame; or its script may change the tree.
	Unpruned bool
	// Why says which, as the report words it: "the page has a <template>".
	// "" for a page that was pruned.
	Why string
	// Sources is the same per source file, when the sheet names them:
	// esbuild writes `/* path/to/file.css */` before each file of a bundle
	// unless it minifies whitespace. Empty otherwise.
	Sources []Source
}

// Source is the part of a sheet under one source comment: the file's rules,
// without the comment that names it.
type Source struct {
	Name                string // the comment's text: "ui/dialog.css"; "" before the first one
	BytesIn, BytesOut   int
	Rules, RulesDropped int
}

// Prune returns css without what doc, the parsed page, cannot use. An error
// means the CSS could not be read (an unbalanced bracket, an unterminated
// string): nothing was pruned, and out is empty.
//
// It is PruneWith for a page without a script.
func Prune(css string, doc *html.Node) (out string, stats Stats, err error) {
	return PruneWith(css, doc, Options{})
}

// Options is what a page has besides its HTML.
type Options struct {
	// Script is the page's built script (behaviors.Build); "" for a page
	// that mounts nothing. A behaviour changes the page after it has loaded,
	// so what the script names is runtime state (builder.md, CSS): a class,
	// an id or an attribute it names is "maybe" — on an element that has it
	// too — a custom property or an animation it names is read; and when it
	// names an API that changes the tree, the page is not pruned. State
	// only a script can write (`aria-*`, `disabled`, …) is "maybe" for no
	// other reason: without a script, it is what the page has.
	Script string
	// Builder is the elements of doc that packaging put there: the
	// `<script>` that delivers Script, the `<style>` or `<link>` that
	// delivers the sheet being pruned. A rule may select them as any other
	// element, and that is all they are to the pruner: such a `<script>` is
	// not a script of the page's own, such a `<link>` is not a stylesheet
	// the builder did not bundle, and such a `<style>` is not read for the
	// names it uses. Every other `<script>`, `<link>` and `<style>` of doc
	// is the author's. The builder's `<style>` or `<link>` is also where the
	// sheet stands among the page's own (Styles): without one, at the end
	// of the head — where packaging writes it.
	Builder []*html.Node
	// Styles are `<style>` elements of the page's own whose text is CSS
	// (markup.CSS): each is pruned with the sheet, as a sheet of the page in
	// its place in the document (builder.md, *The builder's own elements*),
	// and PruneWith sets its Out. A `<style>` of doc that is neither here
	// nor the builder's is read for the names it uses.
	Styles []*Style
}

// Style is a `<style>` element of the page's own, and its CSS.
type Style struct {
	Node *html.Node // the element, in the document that is pruned against
	// CSS is the element's text in the form the pruner reads: flat — as
	// esbuild prints it with nesting lowered.
	CSS string
	// Out is what the page can use of CSS, kept byte for byte: set by
	// PruneWith. CSS itself when the page is not pruned, or when Err says
	// that this text does not read — it is then read for the names it uses,
	// as a `<style>` that was not given.
	Out string
	Err error
}

// OwnStyle is the Name of the Source that counts the page's own `<style>`
// elements, together: their rules read and dropped, their bytes.
const OwnStyle = "<style>"

// PruneWith is Prune for a page and what the builder made for it. doc is
// the page as it is served (builder.md, *CSS*): a `<script>` in it that
// Options.Builder does not name is the page's own, and such a page is not
// pruned.
func PruneWith(css string, doc *html.Node, opts Options) (out string, stats Stats, err error) {
	return newPruner(opts).prune(css, doc)
}

func newPruner(opts Options) *pruner {
	p := &pruner{script: opts.Script}
	if len(opts.Styles) > 0 {
		p.styles = make(map[*html.Node]*Style, len(opts.Styles))
		for _, style := range opts.Styles {
			p.styles[style.Node] = style
		}
	}
	if len(opts.Builder) > 0 {
		p.builder = make(map[*html.Node]bool, len(opts.Builder))
		for _, n := range opts.Builder {
			p.builder[n] = true
		}
	}
	return p
}

// Whole is what Prune says of a sheet it gives back as it is, for a caller
// that has a reason of its own not to prune a page (the driver: a sheet
// whose rules select on its own URL): the sheet is read and counted. The
// caller says why, in Why.
func Whole(css string) (Stats, error) {
	stats := Stats{BytesIn: len(css), BytesOut: len(css), Unpruned: true}
	rules, err := parseRules(css, false, 0)
	if err != nil {
		return stats, fmt.Errorf("cssprune: %w", err)
	}
	stats.Rules, stats.Selectors = count(rules)
	return stats, nil
}

type pruner struct {
	m      matcher
	stats  Stats
	anon   int    // anonymous layers seen in this round
	script string // Options.Script

	// builder: Options.Builder — the elements that are the builder's, not
	// the page's own.
	builder map[*html.Node]bool

	// styles: Options.Styles, by element. own: those of them that are in
	// the page, in document order; sheetAt: where the sheet stands among
	// them — the index of the first that follows it; -1 until it is known.
	styles  map[*html.Node]*Style
	own     []*Style
	sheetAt int

	// other: a stylesheet the builder did not bundle is on the page — a
	// <link>, an @import. It may read any custom property and name any
	// animation or @position-try.
	other bool

	// What the page itself names: in attributes (style, SVG's presentation
	// attributes) and in <style> elements.
	named refs

	// For the tests: every selector, custom property, @keyframes and
	// @position-try dropped.
	onSelector  func(text string, scoped bool)
	onProperty  func(name string)
	onKeyframes func(name string)
	onTry       func(name string)
}

func (p *pruner) prune(css string, doc *html.Node) (string, Stats, error) {
	p.stats = Stats{BytesIn: len(css)}
	if doc == nil {
		return "", p.stats, fmt.Errorf("cssprune: no page")
	}
	rules, err := parseRules(css, false, 0)
	if err != nil {
		return "", p.stats, fmt.Errorf("cssprune: %w", err)
	}
	if why := p.page(doc); why != "" {
		p.stats.Unpruned, p.stats.Why, p.stats.BytesOut = true, why, len(css)
		p.stats.Rules, p.stats.Selectors = count(rules)
		for _, style := range p.styles {
			style.Out = style.CSS
		}
		return css, p.stats, nil
	}
	// The sheets of the page, in the order a browser reads them: the page's
	// own `<style>` elements, and the sheet where its element stands.
	type sheet struct {
		rules []*rule
		style *Style // nil: the sheet
		// conditional: a `<style media>` — what it declares is declared
		// only when the medium applies, as inside `@media`.
		conditional bool
	}
	var sheets []*sheet
	for i, style := range p.own {
		if i == p.sheetAt {
			sheets = append(sheets, &sheet{rules: rules})
		}
		own, err := parseRules(style.CSS, false, 0)
		if err != nil {
			// Not CSS the pruner reads: left as it is, and what it names
			// stays — as for a `<style>` that was not given.
			style.Out, style.Err = style.CSS, fmt.Errorf("cssprune: %w", err)
			p.read(style.CSS)
			continue
		}
		media, _ := attribute(style.Node, "media")
		media = strings.ToLower(strings.TrimSpace(media))
		sheets = append(sheets, &sheet{rules: own, style: style, conditional: media != "" && media != "all"})
	}
	if p.sheetAt < 0 || p.sheetAt >= len(p.own) {
		sheets = append(sheets, &sheet{rules: rules})
	}
	for _, s := range sheets {
		freeze(s.rules)
		p.selectors(s.rules, false)
		p.other = p.other || imports(s.rules)
	}
	// Custom properties and @keyframes, to a fixed point: dropping one may
	// empty a rule, which may empty an at-rule, whose prelude was the last
	// to name another. So for @position-try. With a sheet that is not here,
	// nothing says what is read: none is dropped. The sheets are one
	// cascade: what one names, another may define, and a layer is ordered
	// where its name first occurs in any of them.
	for {
		p.anon = 0
		declared := &layers{}
		for _, s := range sheets {
			scope := declared
			if s.conditional {
				scope = &layers{up: declared}
			}
			p.resolve(s.rules, "", scope)
		}
		if p.other {
			break
		}
		r := refs{dashed: maps.Clone(p.named.dashed), words: maps.Clone(p.named.words)}
		for _, s := range sheets {
			p.collect(s.rules, &r)
		}
		changed := false
		for _, s := range sheets {
			changed = p.sweep(s.rules, &r) || changed
		}
		if !changed {
			break
		}
	}
	var b strings.Builder
	p.print(&b, rules, true)
	p.stats.BytesOut = b.Len()
	var own *Source // the page's own `<style>` elements, together
	for _, s := range sheets {
		if s.style == nil {
			continue
		}
		var out strings.Builder
		p.print(&out, s.rules, false)
		s.style.Out = out.String()
		if own == nil {
			p.stats.Sources = append(p.stats.Sources, Source{Name: OwnStyle})
			own = &p.stats.Sources[len(p.stats.Sources)-1]
		}
		n, dropped := tally(&rule{kind: kGroup, live: true, rules: s.rules})
		own.Rules, own.RulesDropped = own.Rules+n, own.RulesDropped+dropped
		own.BytesIn, own.BytesOut = own.BytesIn+len(s.style.CSS), own.BytesOut+out.Len()
		p.stats.RulesDropped += dropped
	}
	return b.String(), p.stats, nil
}

// read notes what a text of CSS that is not pruned names: it is kept as it
// is, so whatever it uses stays — and with an `@import` in it, a sheet that
// is not here may read anything.
func (p *pruner) read(css string) {
	p.named.all(css)
	p.other = p.other || strings.Contains(strings.ToLower(css), "@import")
}

// page indexes the document. It says why the page cannot be pruned
// (builder.md, CSS) — "": it can — which is when the tree a browser matches
// against is not the one written: the content of a <template> is cloned somewhere at run time,
// so what it matches — and what matches because of it, `.list:has(.row)` —
// is not known; the content of a <noscript> is elements or text depending on
// the browser; a <selectedcontent> is filled at load with a copy of the
// selected option's content, which `selectedcontent .x`, `button .x` and
// `.x:not(option .x)` then match; and what a <select> holds besides its
// options is kept by a parser that knows the customizable select (this one,
// Chrome 135, WebKit 26.6) and dropped, with its end tags, by one that does
// not — where `<select><div><option>` makes `select > option` match. Whether
// the floor's Firefox and Safari are of the second kind was not checked.
//
// And an element the user edits — `contenteditable`, unless it is "false" —
// gets elements no script wrote: Bold puts a <b> around the selection, Enter
// a <div> after the line, a paste whatever was copied. `.editor b` matches
// then, and nothing of the page as written said so.
func (p *pruner) page(doc *html.Node) (why string) {
	p.named = refs{dashed: map[string]bool{}, words: map[string]bool{}}
	p.own, p.sheetAt = nil, -1
	p.m.quirks = !standards(doc)
	not := func(because string) { // the first reason is the one that is said
		if why == "" {
			why = because
		}
	}
	const inSelect = "the page has markup in a <select>"
	// in: 1 inside a <select>, 2 inside one of its options, where the older
	// parser keeps text only.
	var walk func(n *html.Node, in int)
	walk = func(n *html.Node, in int) {
		if n.Type == html.ElementNode {
			if p.m.root == nil {
				p.m.root = n
			}
			p.m.els = append(p.m.els, n)
			plain := true // an element like any other: not one a <select> may hold
			switch tag := strings.ToLower(n.Data); tag {
			case "template", "noscript", "selectedcontent":
				not("the page has a <" + tag + ">")
			case "script":
				// A script of the page's own: what it writes is not known.
				// The builder's is the one that was read (Options.Script).
				if !p.builder[n] && markup.Runs(n) {
					not(ownScript)
				}
				plain = false
			case "iframe", "frame", "object", "embed":
				// A document of the site's own in a frame is of the page's
				// origin: its script writes to the page as the page's own
				// would, and is read as little.
				if framed(n, tag) {
					not("the page has a document of the site in a frame (`<" + tag + ">`)")
				}
			case "option":
				if plain = false; in == 1 {
					in = 2
				} else if in == 2 {
					not(inSelect)
				}
			case "optgroup", "hr":
				plain = false
			case "select":
				if in != 0 {
					not(inSelect)
				}
				plain, in = false, 1
			case "body":
				// The sheet's element is not named: it is the last of the
				// head's, where packaging writes it.
				if n.Namespace == "" && p.sheetAt < 0 {
					p.sheetAt = len(p.own)
				}
			case "style":
				switch style := p.styles[n]; {
				case p.builder[n]:
					// The builder's own holds the sheet that is pruned: what
					// a sheet names is no reason to keep it. It is where
					// the sheet stands among the page's own.
					if p.sheetAt < 0 {
						p.sheetAt = len(p.own)
					}
				case style != nil:
					// One of the page's own, pruned with the sheet.
					p.own = append(p.own, style)
				default:
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.TextNode {
							p.read(c.Data)
						}
					}
				}
			case "link":
				// By its `rel` (markup.Link): the builder's own is the sheet
				// that is pruned, not another; a hint or a relation is no
				// stylesheet at all.
				switch {
				case p.builder[n]:
					if p.sheetAt < 0 {
						p.sheetAt = len(p.own)
					}
				case markup.Link(n).Effect().Unbundled:
					p.other = true
				}
			}
			if in != 0 && (plain || in == 2 && n.Data != "option") {
				not(inSelect)
			}
			for _, a := range n.Attr {
				if a.Key == "style" || strings.Contains(a.Val, "--") {
					p.named.all(a.Val)
				}
				if markup.Handler(a.Key) || markup.JavaScriptURL(a.Key, a.Val) {
					not(ownScript)
				}
				// Any value but "false" is taken for editable: an unknown
				// one inherits, and what it inherits is not looked up.
				if a.Namespace == "" && a.Key == "contenteditable" && !strings.EqualFold(strings.TrimSpace(a.Val), "false") {
					not("the page has an element the user edits (`contenteditable`)")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, in)
		}
	}
	walk(doc, 0)
	p.m.memo = map[memoKey]tri{}
	if why == "" && p.script != "" {
		p.m.script = scriptNames(p.script)
		if writer := p.m.script.tree; writer != "" {
			// By the name found: `append` may be a URLSearchParams's, and
			// the report then says what to look for.
			not("the page's script may change the tree: it names `" + writer + "`")
		}
		for name := range p.m.script.words {
			p.named.words[name] = true
			if strings.HasPrefix(name, "--") {
				p.named.dashed[name] = true
			}
		}
	}
	return why
}

// ownScript is the reason for a page that has what runs and the builder did
// not make (package markup). The driver stops on such a page before it is
// pruned — shell-script — so this is said to a caller of its own only.
const ownScript = "the page has a script of its own"

// framed reports whether an `<iframe>`, `<frame>`, `<object>` or `<embed>`
// holds a document whose script can reach the page (`parent.document`): one
// of the page's own origin.
//
//   - `srcdoc` is: it has the origin of the page that wrote it.
//   - A URL is when it is the site's — written without a scheme and without
//     a host, as the page's links are (`/frame.html`, `demo/`). The builder
//     does not know where the site is served: a URL in full is taken for
//     another site's, as the link check takes it (builder.md, *Checks on
//     the page*). `data:` and `about:blank` are nobody's; `javascript:` is
//     a script of the page's own (page).
//   - `sandbox` on an `<iframe>` takes the document's scripts away, or its
//     origin, unless it allows both.
func framed(el *html.Node, tag string) bool {
	if sandbox, ok := attribute(el, "sandbox"); ok && tag == "iframe" {
		scripts, origin := false, false
		for _, token := range strings.Fields(sandbox) {
			scripts = scripts || strings.EqualFold(token, "allow-scripts")
			origin = origin || strings.EqualFold(token, "allow-same-origin")
		}
		if !scripts || !origin {
			return false
		}
	}
	if _, ok := attribute(el, "srcdoc"); ok && tag == "iframe" {
		return true
	}
	name := "src"
	if tag == "object" {
		name = "data"
	}
	url, _ := attribute(el, name)
	// As a URL parser reads it: no spaces or controls around it, no tabs or
	// line breaks in it, `\` a `/`.
	url = strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return -1
		case '\\':
			return '/'
		}
		return r
	}, strings.TrimFunc(url, func(r rune) bool { return r <= ' ' }))
	if url == "" || strings.HasPrefix(url, "//") {
		return false // no document; another host
	}
	// A scheme: a letter, then letters, digits, `+`, `-`, `.`, up to a `:`.
	for i := 0; i < len(url); i++ {
		c := url[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return !(c == ':' && i > 0)
		}
	}
	return true
}

// imports reports whether the sheet still imports another: one the builder
// did not bundle.
func imports(rules []*rule) bool {
	for _, r := range rules {
		if r.kind == kAt && r.name == "import" || (r.kind == kGroup || r.kind == kLayer) && imports(r.rules) {
			return true
		}
	}
	return false
}

// standards reports whether the page is surely not in quirks mode: it starts
// with `<!doctype html>` and nothing more, as the builder writes it. Any
// other doctype may be one of those that mean quirks, and none does: classes
// and ids then match whatever their case (builder.md, CSS, *case*).
func standards(doc *html.Node) bool {
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.DoctypeNode:
			return strings.EqualFold(c.Data, "html") && len(c.Attr) == 0
		case html.ElementNode:
			return false
		}
	}
	return false
}

func count(rules []*rule) (n, sels int) {
	for _, r := range rules {
		switch r.kind {
		case kStyle:
			n++
			if list, err := splitList(r.head); err == nil {
				sels += len(list)
			}
		case kGroup, kLayer:
			a, b := count(r.rules)
			n, sels = n+a, sels+b
		}
	}
	return n, sels
}

// freeze keeps the head of a sheet as it is when an @import or a @namespace
// stands out of place in it (builder.md, CSS, *Out of place*). A browser
// ignores one that follows a rule, so dropping that rule would bring it to
// life; and it does not ignore one that follows `@layer a;`, so the statement
// an emptied block leaves would too. Everything before the last such rule
// becomes an at-rule the pruner does not know.
func freeze(rules []*rule) {
	last, misplaced := -1, false
	for i, r := range rules {
		switch {
		case r.kind == kComment:
		case r.kind == kAt && (r.name == "import" || r.name == "namespace"):
			if misplaced {
				last = i
			}
		case r.kind == kAt && !r.block && (r.name == "charset" || r.name == "layer"):
		default:
			misplaced = true // whatever follows is
		}
	}
	for _, r := range rules[:max(last, 0)] {
		if r.kind != kComment {
			r.kind = kAt
		}
	}
}

// selectors decides every style rule: which selectors of its list stay.
// scoped: inside @scope, where a selector may be relative.
func (p *pruner) selectors(rules []*rule, scoped bool) {
	for _, r := range rules {
		switch r.kind {
		case kGroup, kLayer:
			p.selectors(r.rules, scoped || r.name == "scope")
		case kStyle:
			p.stats.Rules++
			list, err := splitList(r.head)
			if err != nil || r.opaque {
				// Kept whole. A nested rule may match without its parent
				// (`:is(&.x, .q)`), so the parent's list decides nothing.
				p.stats.Selectors += len(list)
				r.opaque = true
				continue
			}
			p.stats.Selectors += len(list)
			var gone []string
			// whole: the list cannot be trimmed. A selector that would go
			// may be one a browser rejects, and the rule with it; one that
			// does not parse here may not be a selector at all, and the
			// text around it must stay as it is.
			whole := false
			for _, text := range list {
				sel, err := parseSelector(text, scoped)
				switch {
				case err != nil:
					r.sels, whole = append(r.sels, text), true
				case p.m.may(sel):
					r.sels = append(r.sels, text)
				default:
					gone = append(gone, text)
					whole = whole || !sel.known
				}
			}
			if len(r.sels) > 0 && whole {
				r.sels, gone = []string{r.head}, nil
			}
			r.dead = len(r.sels) == 0
			p.stats.SelectorsDropped += len(gone)
			if p.onSelector != nil {
				for _, text := range gone {
					p.onSelector(text, scoped)
				}
			}
		}
	}
}

// layers is the set of cascade layers ordered so far, by full name ("a.b").
// What a conditional at-rule declares is declared only when it applies, so
// each has its own set, on top of what was declared before it.
type layers struct {
	set map[string]bool
	up  *layers
}

func (l *layers) has(name string) bool {
	for ; l != nil; l = l.up {
		if l.set[name] {
			return true
		}
	}
	return false
}

// add declares a layer and the layers it is in: a.b.c orders a, a.b, a.b.c.
func (l *layers) add(name string) {
	if l.set == nil {
		l.set = map[string]bool{}
	}
	for i := 0; i <= len(name); i++ {
		if i == len(name) || name[i] == '.' {
			l.set[name[:i]] = true
		}
	}
}

// resolve decides what is printed, from what is dead. output: something of
// the list is printed; solid: something more than the order of layers.
//
// An emptied @layer block is the case that needs care: a layer's place in
// the cascade is where its name first occurs, so the block stays as
// `@layer name;` unless something before it already ordered that layer
// (builder.md, CSS: "dropping it would reorder the cascade").
func (p *pruner) resolve(rules []*rule, path string, declared *layers) (output, solid bool) {
	for _, r := range rules {
		r.statement = false
		var o, s bool
		switch r.kind {
		case kComment:
			r.live = true
			continue
		case kStyle:
			o = !r.dead && r.opaque
			for _, d := range r.decls {
				o = o || !r.dead && !d.dead
			}
			s = o
		case kKeyframes, kTry:
			o, s = !r.dead, !r.dead
		case kGroup:
			o, s = p.resolve(r.rules, path, &layers{up: declared})
		case kLayer:
			name := r.prelude
			if name == "" { // anonymous: a layer of its own, which nothing can name again
				p.anon++
				_, s = p.resolve(r.rules, path+"\x00"+strconv.Itoa(p.anon)+".", declared)
				o = s
				break
			}
			was := declared.has(path + name)
			declared.add(path + name)
			if o, s = p.resolve(r.rules, path+name+".", declared); !o && !was {
				o, r.statement = true, true
			}
		default: // kAt, kRaw
			o, s = true, true
			if r.kind == kAt && r.name == "layer" {
				// `@layer a, b;` orders its layers — if a browser reads it:
				// one name that is not a layer's makes it ignore the whole
				// statement. A block here is one kept as it is: frozen, or
				// with a prelude that is no name, which orders nothing.
				names, err := splitList(r.prelude)
				read := err == nil && (!r.block || len(names) == 1)
				for _, name := range names {
					read = read && layerName(name)
				}
				for _, name := range names {
					if read {
						declared.add(path + name)
					}
				}
				s = r.block
			}
		}
		r.live = o
		output, solid = output || o, solid || s
	}
	return output, solid
}

// refs is what the kept CSS and the page name.
type refs struct {
	dashed map[string]bool // --x, anywhere: values, at-rule preludes, attributes
	words  map[string]bool // identifiers and strings where an animation may be named
}

func (r *refs) all(s string) {
	words(s, func(w string) {
		r.words[w] = true
		if strings.HasPrefix(w, "--") {
			r.dashed[w] = true
		}
	})
}

func (r *refs) onlyDashed(s string) {
	words(s, func(w string) {
		if strings.HasPrefix(w, "--") {
			r.dashed[w] = true
		}
	})
}

// animation reports whether a property names animations: animation,
// animation-name, and their prefixed forms.
func animation(prop string) bool {
	if strings.HasPrefix(prop, "-") {
		if i := strings.IndexByte(prop[1:], '-'); i >= 0 {
			prop = prop[i+2:]
		}
	}
	return prop == "animation" || prop == "animation-name"
}

// collect gathers what the live rules name. Anything kept as it is — an
// unknown at-rule, a rule that nests — counts whole.
func (p *pruner) collect(rules []*rule, into *refs) {
	for _, r := range rules {
		if !r.live {
			continue
		}
		switch r.kind {
		case kStyle:
			if r.opaque {
				into.all(r.body)
				continue
			}
			for _, d := range r.decls {
				switch {
				case d.dead:
				case d.name == "":
					into.all(d.text)
				case d.custom:
					// A custom property may carry an animation's name
					// (`animation: var(--a)`), and names others — but
					// reading itself does not make it read.
					words(d.value, func(w string) {
						into.words[w] = true
						if strings.HasPrefix(w, "--") && w != d.name {
							into.dashed[w] = true
						}
					})
				case animation(d.name):
					into.all(d.value)
				default:
					into.onlyDashed(d.value)
				}
			}
		case kKeyframes, kTry:
			// Its own name, in its prelude, does not make a rule named.
			into.onlyDashed(r.body)
		case kGroup:
			into.onlyDashed(r.prelude)
			p.collect(r.rules, into)
		case kLayer:
			if !r.statement {
				p.collect(r.rules, into)
			}
		case kAt, kRaw:
			into.all(r.raw)
		}
	}
}

// sweep drops the custom properties nothing reads, the @keyframes no
// animation names and the @position-try nothing names (builder.md, CSS). It
// reports whether it dropped any.
func (p *pruner) sweep(rules []*rule, r *refs) (changed bool) {
	for _, rule := range rules {
		if !rule.live {
			continue
		}
		switch rule.kind {
		case kGroup, kLayer:
			changed = p.sweep(rule.rules, r) || changed
		case kKeyframes:
			if name := keyframesName(rule.prelude); !r.words[name] {
				rule.dead, changed = true, true
				p.stats.Keyframes++
				if p.onKeyframes != nil {
					p.onKeyframes(name)
				}
			}
		case kTry:
			// A fallback is named by its dashed identifier, in
			// `position-try-fallbacks` or `position-try` — or in a custom
			// property those read: wherever it stands, it is in r.dashed.
			if name := tryName(rule.prelude); !r.dashed[name] {
				rule.dead, changed = true, true
				p.stats.PositionTries++
				if p.onTry != nil {
					p.onTry(name)
				}
			}
		case kStyle:
			for _, d := range rule.decls {
				if d.custom && !d.dead && !r.dashed[d.name] {
					d.dead, changed = true, true
					p.stats.Properties++
					if p.onProperty != nil {
						p.onProperty(d.name)
					}
				}
			}
		}
	}
	return changed
}

// keyframesName decodes `spin`, `"spin"`, `sp\69n`.
func keyframesName(prelude string) string {
	if prelude != "" && (prelude[0] == '"' || prelude[0] == '\'') {
		if s, _, err := readString(prelude, 0); err == nil {
			return s
		}
	}
	name, _ := readName(prelude, 0)
	return name
}

// print writes what is live. top: the sheet itself, where the statistics per
// source file are taken.
func (p *pruner) print(b *strings.Builder, rules []*rule, top bool) {
	var src *Source
	source := func(name string) {
		for i := range p.stats.Sources {
			if p.stats.Sources[i].Name == name {
				src = &p.stats.Sources[i]
				return
			}
		}
		p.stats.Sources = append(p.stats.Sources, Source{Name: name})
		src = &p.stats.Sources[len(p.stats.Sources)-1]
	}
	named := false
	for _, r := range rules {
		named = named || top && r.kind == kComment && !strings.HasPrefix(r.raw, "/*!")
	}
	for _, r := range rules {
		start := b.Len()
		switch {
		case !r.live:
		case r.kind == kStyle && r.opaque:
			b.WriteString(r.head + "{" + r.body + "}")
		case r.kind == kStyle:
			b.WriteString(strings.Join(r.sels, ","))
			b.WriteByte('{')
			first := true
			for _, d := range r.decls {
				if d.dead {
					continue
				}
				if !first {
					b.WriteByte(';')
				}
				b.WriteString(d.text)
				first = false
			}
			b.WriteByte('}')
		case r.kind == kGroup, r.kind == kLayer && !r.statement:
			b.WriteString(r.head)
			b.WriteByte('{')
			p.print(b, r.rules, false)
			b.WriteByte('}')
		case r.kind == kLayer:
			b.WriteString("@layer " + r.prelude + ";")
		default: // a comment, a @keyframes, a @position-try, an at-rule or a declaration kept as it is
			b.WriteString(r.raw)
		}
		if !named {
			continue
		}
		if r.kind == kComment && !strings.HasPrefix(r.raw, "/*!") {
			// The comment is esbuild's, not the file's: its bytes are not
			// counted. They would be the machine's — the file's path from
			// the project, as long as the way to where the install put it.
			source(trim(r.raw[2 : len(r.raw)-2]))
			continue
		} else if src == nil {
			source("")
		}
		src.BytesIn += len(r.raw)
		src.BytesOut += b.Len() - start
		n, dropped := tally(r)
		src.Rules, src.RulesDropped = src.Rules+n, src.RulesDropped+dropped
	}
	if top {
		_, p.stats.RulesDropped = tally(&rule{kind: kGroup, live: true, rules: rules})
	}
}

// tally counts the style rules of r, and those of them that are not printed.
func tally(r *rule) (n, dropped int) {
	switch r.kind {
	case kStyle:
		if !r.live {
			return 1, 1
		}
		return 1, 0
	case kGroup, kLayer:
		for _, c := range r.rules {
			a, b := tally(c)
			if !r.live || r.statement {
				b = a
			}
			n, dropped = n+a, dropped+b
		}
	}
	return n, dropped
}
