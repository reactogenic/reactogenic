// Package cssprune removes from a page's CSS what the page cannot use
// (specs/phase02/builder.md, *CSS*): a rule stays only if some element of the
// page may match it, a custom property only if something reads it, a
// @keyframes only if an animation names it.
//
// The input is the flat CSS esbuild's public API prints for a page — nesting
// lowered, minified or not. The output is that CSS with rules, selectors and
// declarations removed and nothing rewritten: what is kept is kept byte for
// byte, in order, so the driver can minify it again.
//
// Soundness is the requirement: a rule that is dropped could not have
// matched. What can change after load is "maybe" and never drops anything —
// every pseudo-class but :root and the logical ones, and the attributes a
// behaviour or the browser writes. The table of builder.md is the contract;
// each row is a test here, and the corpus test checks every selector dropped
// against an independent matcher (cascadia).
//
// # What it assumes
//
//   - The page is served as it was parsed. Parse it with its doctype: the
//     builder writes `<!doctype html>`, which makes classes and ids
//     case-sensitive. Without exactly that doctype the page may be in quirks
//     mode, and they are matched whatever their case.
//   - No script adds a class, a data-* attribute or an element
//     (components.md, *CSS convention*): state is written only where it is
//     "maybe". Where the browser's tree is not the one written — <template>,
//     <noscript>, <selectedcontent>, markup inside a <select> — the page is
//     not pruned.
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
)

// Stats is what Prune did, for the report (builder.md, *The report*).
type Stats struct {
	BytesIn, BytesOut int
	Rules             int // style rules read
	RulesDropped      int // no selector of theirs may match, or no declaration was left
	Selectors         int // in those rules' lists
	SelectorsDropped  int
	Properties        int // custom property declarations dropped
	Keyframes         int // @keyframes dropped
	// Unpruned: the page holds a <template>, a <noscript>, a
	// <selectedcontent> or a <select> with more than options in it, and the
	// sheet came back as it was.
	Unpruned bool
	// Sources is the same per source file, when the sheet names them:
	// esbuild writes `/* path/to/file.css */` before each file of a bundle
	// unless it minifies whitespace. Empty otherwise.
	Sources []Source
}

// Source is the part of a sheet under one source comment.
type Source struct {
	Name                string // the comment's text: "ui/dialog.css"; "" before the first one
	BytesIn, BytesOut   int
	Rules, RulesDropped int
}

// Prune returns css without what doc, the parsed page, cannot use. An error
// means the CSS could not be read (an unbalanced bracket, an unterminated
// string): nothing was pruned, and out is empty.
func Prune(css string, doc *html.Node) (out string, stats Stats, err error) {
	return new(pruner).prune(css, doc)
}

type pruner struct {
	m     matcher
	stats Stats
	anon  int // anonymous layers seen in this round

	// What the page itself names: in attributes (style, SVG's presentation
	// attributes) and in <style> elements.
	named refs

	// For the tests: every selector, custom property and @keyframes dropped.
	onSelector  func(text string, scoped bool)
	onProperty  func(name string)
	onKeyframes func(name string)
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
	if !p.page(doc) {
		p.stats.Unpruned, p.stats.BytesOut = true, len(css)
		p.stats.Rules, p.stats.Selectors = count(rules)
		return css, p.stats, nil
	}
	freeze(rules)
	p.selectors(rules, false)
	// Custom properties and @keyframes, to a fixed point: dropping one may
	// empty a rule, which may empty an at-rule, whose prelude was the last
	// to name another.
	for {
		p.anon = 0
		p.resolve(rules, "", &layers{})
		r := refs{dashed: maps.Clone(p.named.dashed), words: maps.Clone(p.named.words)}
		p.collect(rules, &r)
		if !p.sweep(rules, &r) {
			break
		}
	}
	var b strings.Builder
	p.print(&b, rules, true)
	p.stats.BytesOut = b.Len()
	return b.String(), p.stats, nil
}

// page indexes the document. It reports false if the page cannot be pruned
// (builder.md, CSS), because the tree a browser matches against is not the
// one written: the content of a <template> is cloned somewhere at run time,
// so what it matches — and what matches because of it, `.list:has(.row)` —
// is not known; the content of a <noscript> is elements or text depending on
// the browser; a <selectedcontent> is filled at load with a copy of the
// selected option's content, which `selectedcontent .x`, `button .x` and
// `.x:not(option .x)` then match; and what a <select> holds besides its
// options is kept by a parser that knows the customizable select (this one,
// Chrome 135, WebKit 26.6) and dropped, with its end tags, by one that does
// not — where `<select><div><option>` makes `select > option` match. Whether
// the floor's Firefox and Safari are of the second kind was not checked.
func (p *pruner) page(doc *html.Node) bool {
	p.named = refs{dashed: map[string]bool{}, words: map[string]bool{}}
	p.m.quirks = !standards(doc)
	ok := true
	// in: 1 inside a <select>, 2 inside one of its options, where the older
	// parser keeps text only.
	var walk func(n *html.Node, in int)
	walk = func(n *html.Node, in int) {
		if n.Type == html.ElementNode {
			if p.m.root == nil {
				p.m.root = n
			}
			p.m.els = append(p.m.els, n)
			switch tag := strings.ToLower(n.Data); tag {
			case "template", "noscript", "selectedcontent":
				ok = false
			case "option", "optgroup", "hr", "script":
				if ok = ok && in < 2; in == 1 && tag == "option" {
					in = 2
				}
			case "select":
				ok, in = ok && in == 0, 1
			case "style":
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						p.named.all(c.Data)
					}
				}
				fallthrough
			default:
				ok = ok && in == 0
			}
			for _, a := range n.Attr {
				if a.Key == "style" || strings.Contains(a.Val, "--") {
					p.named.all(a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, in)
		}
	}
	walk(doc, 0)
	p.m.memo = map[memoKey]tri{}
	return ok
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
		case kKeyframes:
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
		case kKeyframes:
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

// sweep drops the custom properties nothing reads and the @keyframes no
// animation names (builder.md, CSS). It reports whether it dropped any.
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
		default: // a comment, a @keyframes, an at-rule or a declaration kept as it is
			b.WriteString(r.raw)
		}
		if !named {
			continue
		}
		if r.kind == kComment && !strings.HasPrefix(r.raw, "/*!") {
			source(trim(r.raw[2 : len(r.raw)-2]))
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
