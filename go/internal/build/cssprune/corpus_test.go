package cssprune

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
)

// The corpus: sheets as a design system writes them (nested, one file per
// component), bundled by esbuild as the driver will, and the pages they are
// pruned for.
//
//	docs         the modelled design system of research/css.md: 12 files, 4 pages
//	components   the three components of research/components.md, 4 pages
//	adversarial  sheets written to break the pruner, each with its page
//	ui           packages/ui's CSS and the fixture site's, 6 pages the builder built
//	served       a sheet that selects on what packaging writes, 3 built pages
type sheet struct {
	name  string
	entry string
	pages []string
}

func corpus(t testing.TB) []sheet {
	t.Helper()
	glob := func(pattern string) []string {
		files, err := filepath.Glob(pattern)
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no files (%v)", pattern, err)
		}
		return files
	}
	out := []sheet{
		{"docs", "testdata/docs/ds/all.css", glob("testdata/docs/pages/*.html")},
		{"components", "testdata/components/css/all.css", glob("testdata/components/pages/*.html")},
	}
	for _, css := range glob("testdata/adversarial/*.css") {
		name := strings.TrimSuffix(filepath.Base(css), ".css")
		out = append(out, sheet{"adversarial/" + name, css, []string{strings.TrimSuffix(css, ".css") + ".html"}})
	}
	// The design system itself, and pages the builder built with it: the
	// fixture site of internal/build, as its golden output has it — a page
	// of each kind (a drawer, nothing that opens, a menu of links, a dialog,
	// an action menu with a dialog, text), the variants of a route among
	// them (`account/guest.html`). Last, so that the sheets above keep
	// their places.
	out = append(out, sheet{"ui", "testdata/ui/all.css", append(glob("../testdata/golden/never/out/index.html"), glob("../testdata/golden/never/out/*/*.html")...)})
	// Pages as they are served — under a base, with the <link> and the
	// <script> packaging put in them, which the sheet selects on — and one
	// the user edits (`contenteditable`), which is not pruned.
	out = append(out, sheet{"served", "../testdata/served/site.css", append(glob("../testdata/golden/served-files/out/index.html"), glob("../testdata/golden/served-files/out/*/index.html")...)})
	return out
}

// bundle is the driver's step before Prune (builder.md, CSS): esbuild's
// public API, nesting lowered. Without minify the output names its sources.
func bundle(t testing.TB, entry string, minify bool) string {
	t.Helper()
	r := api.Build(api.BuildOptions{
		EntryPoints: []string{entry}, Bundle: true, Write: false, LogLevel: api.LogLevelSilent,
		MinifyWhitespace: minify, MinifySyntax: minify,
		Supported: map[string]bool{"nesting": false},
	})
	if len(r.Errors) > 0 || len(r.OutputFiles) != 1 {
		t.Fatalf("esbuild %s: %v", entry, r.Errors)
	}
	return string(r.OutputFiles[0].Contents)
}

// page is one page of the corpus, as written and parsed. A page the builder
// built is given as the driver gives it: with its script, and with the
// elements packaging wrote named as the builder's (Options).
type page struct {
	name string
	src  string
	doc  *html.Node
	opts Options
}

// built is what the driver says of a page it built, read back from the
// output: the elements that deliver a blob of `_rg/`, and the script among
// them. Without it such a page is one with a script of its own — unpruned —
// and with a stylesheet nobody bundled.
func built(t testing.TB, file string, doc *html.Node) (opts Options) {
	t.Helper()
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				// Under a base too: `/docs/_rg/page-….js`.
				at := strings.Index(a.Val, "/_rg/page-")
				if at < 0 || a.Key != "src" && a.Key != "href" {
					continue
				}
				opts.Builder = append(opts.Builder, n)
				if n.Data == "script" {
					out := filepath.Dir(file)
					for !isDir(filepath.Join(out, "_rg")) {
						if out = filepath.Dir(out); out == "." || out == string(filepath.Separator) {
							t.Fatalf("%s: no _rg/ above it", file)
						}
					}
					script, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(a.Val[at+1:])))
					if err != nil {
						t.Fatal(err)
					}
					opts.Script = string(script)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return opts
}

func isDir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

// loadPages reads a sheet's pages. A page with a <template> is not pruned,
// so it is also given with the template taken out — what phase 2 emits.
func loadPages(t testing.TB, s sheet) []page {
	t.Helper()
	var out []page
	for _, file := range s.pages {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(file), ".html")
		if name == "index" { // a built page: named by its directory, `out` for "/"
			name = filepath.Base(filepath.Dir(file))
		}
		doc := parsePage(t, string(src))
		out = append(out, page{name, string(src), doc, built(t, file, doc)})
		if strings.Contains(string(src), "<template") {
			doc := parsePage(t, string(src))
			var strip func(n *html.Node)
			strip = func(n *html.Node) {
				for c := n.FirstChild; c != nil; {
					next := c.NextSibling
					if c.Type == html.ElementNode && c.Data == "template" {
						n.RemoveChild(c)
					} else {
						strip(c)
					}
					c = next
				}
			}
			strip(doc)
			var b strings.Builder
			if err := html.Render(&b, doc); err != nil {
				t.Fatal(err)
			}
			out = append(out, page{name + " (no template)", b.String(), doc, Options{}})
		}
	}
	return out
}

// taken is a page after its script took away what it names, two ways: every
// class and id that is a word of the script — all of them, where the script
// names the attribute (`className`, `id`) — and, where it names
// `textContent`, whatever the elements of the body held. Nothing for a page
// without a script.
//
// A selector the pruner dropped must match nothing there either:
// `.card:not(.collapsed)` once `classList.toggle("collapsed")` ran,
// `.status:not(:has(b))` once the status is text. The page as written cannot
// show that, and the design system's behaviours write nothing; the `served`
// fixture's `/toggle/` does. The words are found by `word`, not by the
// pruner's scriptNames.
func taken(t testing.TB, pg page) (after []*html.Node) {
	t.Helper()
	script := pg.opts.Script
	if script == "" {
		return nil
	}
	walk := func(doc *html.Node, each func(el *html.Node)) *html.Node {
		var visit func(n *html.Node)
		visit = func(n *html.Node) {
			if n.Type == html.ElementNode {
				each(n)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
		}
		visit(doc)
		return doc
	}
	classes, ids := word(script, "className"), word(script, "id")
	after = append(after, walk(parsePage(t, pg.src), func(el *html.Node) {
		var attrs []html.Attribute
		for _, a := range el.Attr {
			switch a.Key {
			case "class":
				var kept []string
				for _, class := range strings.Fields(a.Val) {
					if !classes && !word(script, class) {
						kept = append(kept, class)
					}
				}
				a.Val = strings.Join(kept, " ")
			case "id":
				if ids || word(script, a.Val) {
					continue
				}
			}
			attrs = append(attrs, a)
		}
		el.Attr = attrs
	}))
	// The text set on every element of one depth below the body, depth by
	// depth: the document's own elements are nobody's text.
	for depth, emptied := 1, true; emptied && word(script, "textContent"); depth++ {
		emptied = false
		after = append(after, walk(parsePage(t, pg.src), func(el *html.Node) {
			at, p := 0, el
			for ; p != nil && p.Data != "body"; p = p.Parent {
				at++
			}
			for c := el.FirstChild; c != nil && p != nil && at == depth; {
				next := c.NextSibling
				if c.Type == html.ElementNode {
					el.RemoveChild(c)
					emptied = true
				}
				c = next
			}
		}))
	}
	return after
}

// word reports whether name occurs in s as a whole word.
func word(s, name string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if (j == 0 || !isNameChar(s[j-1]) || strings.HasPrefix(name, "--")) && (end == len(s) || !isNameChar(s[end])) {
			return true
		}
		i = j + 1
	}
}

// Where an animation may be named: an animation property, or a custom
// property that one may read.
var animationValue = regexp.MustCompile(`(?i)(?:animation(?:-name)?|--[\w-]+)\s*:([^;}]*)`)

// TestCorpusSound is the soundness test (builder.md, CSS: "a dropped rule
// must never have matched"). For every page and sheet of the corpus:
//
//   - every selector dropped, with its runtime conditions taken out, matches
//     no element under cascadia — a matcher that shares no code with this
//     package;
//   - a custom property dropped is named nowhere in what is left, nor in the
//     page; a @keyframes dropped is named in no animation left;
//   - pruning again changes nothing, the declarations kept are byte for byte
//     those written, and esbuild reads the result.
func TestCorpusSound(t *testing.T) {
	type total struct{ dropped, checked, approx, unparsed, kept, keptStatic int }
	var all total
	notRun := map[string]bool{} // selectors cascadia could not parse
	for _, s := range corpus(t) {
		for _, minify := range []bool{true, false} {
			css := bundle(t, s.entry, minify)
			for _, pg := range loadPages(t, s) {
				name := fmt.Sprintf("%s/%s/minify=%v", s.name, pg.name, minify)
				var n total
				after := taken(t, pg)
				matches := func(sel string, scoped bool) (matched, ok bool) {
					parsed, err := parseSelector(sel, scoped)
					if err != nil {
						t.Errorf("%s: %q does not parse: %v", name, sel, err)
						return false, false
					}
					alts, approx := static(parsed)
					if approx {
						n.approx++
					}
					ok = true
					for _, alt := range alts {
						group, err := cascadia.ParseGroup(alt)
						if err != nil {
							notRun[fmt.Sprintf("%s (as %s): %v", sel, alt, err)] = true
							ok = false
							continue
						}
						if el := cascadia.Query(pg.doc, group); el != nil {
							return true, true
						}
						for _, doc := range after {
							if cascadia.Query(doc, group) != nil {
								t.Errorf("%s: dropped %q, which matches an element once the page's script took away what it names", name, sel)
							}
						}
					}
					return false, ok
				}

				p := newPruner(pg.opts)
				p.onSelector = func(sel string, scoped bool) {
					n.dropped++
					matched, ok := matches(sel, scoped)
					if matched {
						t.Errorf("%s: dropped %q, which matches an element", name, sel)
					}
					if ok {
						n.checked++
					} else {
						n.unparsed++
					}
				}
				var props, frames []string
				p.onProperty = func(name string) { props = append(props, name) }
				p.onKeyframes = func(name string) { frames = append(frames, name) }
				var tries []string
				p.onTry = func(name string) { tries = append(tries, name) }
				out, stats, err := p.prune(css, pg.doc)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				// React writes the attribute as `contentEditable="true"`. A
				// script of the author's is a plain `<script>`; the builder's
				// is a module, and named (built). A frame of the corpus holds
				// a document of the site.
				if (strings.Contains(pg.src, "<template") || strings.Contains(strings.ToLower(pg.src), "contenteditable") || strings.Contains(pg.src, "<script>") || strings.Contains(pg.src, "<iframe")) != stats.Unpruned {
					t.Errorf("%s: Unpruned = %v (%s)", name, stats.Unpruned, stats.Why)
				}
				if stats.Unpruned {
					if out != css {
						t.Errorf("%s: a page with a <template>, one the user edits, one with a script of its own or one with a frame was pruned", name)
					}
					continue
				}
				checkPrunedWith(t, css, out, pg.doc, pg.opts)

				// A @position-try dropped is named nowhere in what is left,
				// nor in the page.
				for _, try := range tries {
					if word(out, try) || word(pg.src, try) || word(pg.opts.Script, try) {
						t.Errorf("%s: dropped @position-try %s, which is still named", name, try)
					}
				}

				for _, prop := range props {
					if word(out, prop) || word(pg.src, prop) || word(pg.opts.Script, prop) {
						t.Errorf("%s: dropped %s, which is still named", name, prop)
					}
				}
				for _, frame := range frames {
					for _, m := range animationValue.FindAllStringSubmatch(out, -1) {
						if word(m[1], frame) {
							t.Errorf("%s: dropped @keyframes %s, which %q names", name, frame, m[0])
						}
					}
					if word(pg.src, frame) || word(pg.opts.Script, frame) {
						t.Errorf("%s: dropped @keyframes %s, which the page names", name, frame)
					}
				}

				// The other direction, for the record: a selector kept whose
				// static part matches nothing is one the pruner could not
				// decide (or kept with its list).
				kept, err := parseRules(out, false, 0)
				if err != nil {
					t.Fatal(err)
				}
				var undecided []string
				for _, r := range styleRules(kept, nil) {
					if r.opaque {
						continue
					}
					list, _ := splitList(r.head)
					for _, sel := range list {
						n.kept++
						parsed, err := parseSelector(sel, true)
						if err != nil {
							undecided = append(undecided, sel)
							continue
						}
						matched := false
						alts, _ := static(parsed)
						for _, alt := range alts {
							if group, err := cascadia.ParseGroup(alt); err != nil || cascadia.Query(pg.doc, group) != nil {
								matched = true
							}
						}
						if matched {
							n.keptStatic++
						} else {
							undecided = append(undecided, sel)
						}
					}
				}
				t.Logf("%-46s selectors: %3d dropped, %3d checked by cascadia (%d widened), %d not run; %3d kept, %d of them match nothing statically %q",
					name, n.dropped, n.checked, n.approx, n.unparsed, n.kept, n.kept-n.keptStatic, undecided)
				all.dropped, all.checked, all.approx, all.unparsed = all.dropped+n.dropped, all.checked+n.checked, all.approx+n.approx, all.unparsed+n.unparsed
				all.kept, all.keptStatic = all.kept+n.kept, all.keptStatic+n.keptStatic
			}
		}
	}
	t.Logf("corpus: %d selectors dropped, %d checked by cascadia (%d of them widened beyond runtime state), %d not run; %d kept",
		all.dropped, all.checked, all.approx, all.unparsed, all.kept)
	var list []string
	for s := range notRun {
		list = append(list, s)
	}
	sort.Strings(list)
	for _, s := range list {
		t.Logf("cascadia cannot parse: %s", s)
	}
	if all.dropped == 0 || all.checked < all.dropped*9/10 {
		t.Errorf("the corpus checks too little: %d of %d dropped selectors", all.checked, all.dropped)
	}
}

func gz(s string) int {
	var b bytes.Buffer
	w, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
	w.Write([]byte(s))
	w.Close()
	return b.Len()
}

// TestCorpusBytes prints what pruning saves on the corpus: one bundle for
// the site against the page's own sheet, both minified by esbuild, as the
// driver does after Prune. The package comment quotes these numbers.
// CSSPRUNE_OUT=dir writes the sheets, to measure brotli with another tool.
func TestCorpusBytes(t *testing.T) {
	dir := os.Getenv("CSSPRUNE_OUT")
	for _, s := range corpus(t) {
		css := bundle(t, s.entry, true)
		t.Logf("%-12s %-22s %7s %6s %6s", s.name, "", "raw", "gzip", "share")
		t.Logf("%-12s %-22s %7d %6d", "", "one bundle", len(css), gz(css))
		if dir != "" {
			write(t, filepath.Join(dir, s.name, "bundle.css"), css)
		}
		for _, pg := range loadPages(t, s) {
			out, stats, err := PruneWith(css, pg.doc, pg.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !stats.Unpruned {
				out = lower(t, out, true)
			}
			note := ""
			if stats.Unpruned {
				note = "  unpruned: " + stats.Why
			}
			t.Logf("%-12s %-22s %7d %6d %5d%%%s  (rules %d-%d, selectors %d-%d, custom properties -%d, @keyframes -%d, @position-try -%d)",
				"", pg.name, len(out), gz(out), 100*gz(out)/gz(css), note,
				stats.Rules, stats.RulesDropped, stats.Selectors, stats.SelectorsDropped, stats.Properties, stats.Keyframes, stats.PositionTries)
			if len(out) > len(css) {
				t.Errorf("%s/%s: pruning grew the sheet: %d > %d", s.name, pg.name, len(out), len(css))
			}
			if dir != "" {
				write(t, filepath.Join(dir, s.name, strings.NewReplacer(" ", "-", "(", "", ")", "").Replace(pg.name)+".css"), out)
			}
		}
	}
}

// BenchmarkPrune: the docs bundle (29 KB, 245 rules) against its heaviest
// page.
func BenchmarkPrune(b *testing.B) {
	s := corpus(b)[0]
	css := bundle(b, s.entry, true)
	pages := loadPages(b, s)
	doc := pages[1].doc // api, without its <template>
	b.SetBytes(int64(len(css)))
	for b.Loop() {
		if _, stats, err := Prune(css, doc); err != nil || stats.Unpruned {
			b.Fatal(err, stats.Unpruned)
		}
	}
}

func write(t testing.TB, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCorpusSources: printed (not minified) by esbuild, a bundle names its
// files, and Stats reports per file (builder.md, The report).
func TestCorpusSources(t *testing.T) {
	s := corpus(t)[0]
	css := bundle(t, s.entry, false)
	_, stats := mustPrune(t, css, loadPages(t, s)[len(loadPages(t, s))-1].doc) // index: no Dialog, no SideMenu
	rules, dropped := 0, 0
	byName := map[string]Source{}
	for _, src := range stats.Sources {
		t.Logf("%-32s rules %3d, dropped %3d, bytes %5d -> %5d", src.Name, src.Rules, src.RulesDropped, src.BytesIn, src.BytesOut)
		rules, dropped = rules+src.Rules, dropped+src.RulesDropped
		byName[filepath.Base(src.Name)] = src
	}
	// The 12 files, and the entry, which holds nothing but its imports.
	if len(stats.Sources) != 13 || rules != stats.Rules || dropped != stats.RulesDropped {
		t.Errorf("%d sources, %d rules (%d), %d dropped (%d)", len(stats.Sources), rules, stats.Rules, dropped, stats.RulesDropped)
	}
	// Nothing of it is left: the comment that names the file is not the file's.
	if d := byName["Dialog.css"]; d.Rules == 0 || d.Rules != d.RulesDropped || d.BytesIn == 0 || d.BytesOut != 0 {
		t.Errorf("Dialog.css on a page without a dialog: %+v", d)
	}
	if b := byName["Button.css"]; b.RulesDropped == 0 || b.RulesDropped == b.Rules {
		t.Errorf("Button.css on a page with two of its variants: %+v", b)
	}
}
