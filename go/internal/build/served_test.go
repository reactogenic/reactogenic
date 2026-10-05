package build

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/behaviors"
	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/check"
)

// orders are the rules of the `served` fixture's sheet that a page ships:
// each rule there has an `order` of its own.
func orders(css string) []int {
	var out []int
	for _, m := range regexp.MustCompile(`order:(\d+)`).FindAllStringSubmatch(css, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// TestServed: a page's CSS is pruned against the page as it is served
// (builder.md, *CSS*) — with the base in its links, and with the `<style>`
// or `<link>` and the `<script>` packaging puts in it. A rule that matches
// the file that is written is in its sheet; one that only matched the page
// as it was rendered is not.
func TestServed(t *testing.T) {
	everything := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}
	tests := []struct {
		name   string
		args   []string
		home   []int // "/": `<a href="/guide/">`, a `<p>`, a script
		guide  []int // "/guide/": `<a href="/">`, no script
		toggle []int // "/toggle/": no link; a script, after a `<div>`
	}{
		// 2: `a[href="/guide/"]`; 5: `[href$="/guide/"]`; 6: `style`; 9:
		// `title + style`; 10: `script`; 12: `p + script`.
		{"no base, inlined", []string{"--inline", "always"}, []int{1, 2, 5, 6, 9, 10, 12}, []int{1, 6, 9}, []int{6, 9, 10}},
		// 7: `link[rel=stylesheet]`; 9: `title + link`; 11: `script[src]`.
		{"no base, files", []string{"--inline", "never"}, []int{1, 2, 5, 7, 9, 10, 11, 12}, []int{1, 7, 9}, []int{7, 9, 10, 11}},
		// 3: `a[href="/docs/guide/"]`; 4: `a[href^="/docs/"]` — and not 2.
		{"a base, inlined", []string{"--base", "/docs/", "--inline", "always"}, []int{1, 3, 4, 5, 6, 9, 10, 12}, []int{1, 4, 6, 9}, []int{6, 9, 10}},
		// 8: `link[href^="/docs/_rg/"]`: the sheet's own URL.
		{"a base, files", []string{"--base", "/docs/", "--inline", "never"}, []int{1, 3, 4, 5, 7, 8, 9, 10, 11, 12}, []int{1, 4, 7, 8, 9}, []int{7, 8, 9, 10, 11}},
		// One page each: nothing is shared, so nothing is a file.
		{"a base, auto", []string{"--base", "/docs/"}, []int{1, 3, 4, 5, 6, 9, 10, 12}, []int{1, 4, 6, 9}, []int{6, 9, 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := load(t, append([]string{"-p", "served"}, tt.args...)...)
			// What "/toggle/"'s script takes away is "maybe" (builder.md,
			// *The page's script*) — 16: `.card:not(.collapsed) > .body`,
			// the class toggled; 17: `.list:not(:has(.active))`, the class
			// removed; 18: `.label:not(#first)`, the id written over; 19:
			// `.status:not(:has(b))`, the text set. None matches the page as
			// it is written, and each does after a click. Not 20,
			// `.card:not(.list)`: nothing names `list`.
			//
			// State only a script writes is the page's unless the script
			// names it (*Runtime state*) — 21: `.card[aria-expanded=true] >
			// .item`, which the script writes as `ariaExpanded`; 22:
			// `.card[data-state=open] > .item`, as `dataset.state`. Not 23,
			// `.card[aria-pressed=true]`: no script names it, so nobody
			// can make it match — on no pruned page.
			toggle := append(slices.Clone(tt.toggle), 16, 17, 18, 19, 21, 22)
			for pathname, want := range map[string][]int{"/": tt.home, "/guide/": tt.guide, "/toggle/": toggle, "/edit/": everything, "/frame/": everything} {
				if got := orders(s.css(t, pathname)); !slices.Equal(got, want) {
					t.Errorf("%s ships the rules %v, want %v\n%s", pathname, got, want, s.files[s.page(t, pathname).Output])
				}
			}
			// "/toggle/" is pruned: `classList.remove` removes no element,
			// and a text that is set leaves the others where they stand.
			if styles := s.page(t, "/toggle/").Styles; styles == nil || styles.Unpruned || styles.Why != "" || styles.RulesDropped != len(everything)-len(toggle) {
				t.Errorf("/toggle/: %+v", styles)
			}
			// A document of the site in a frame writes to the page as a
			// script of its own would.
			if styles := s.page(t, "/frame/").Styles; styles == nil || !styles.Unpruned || styles.Why != "the page has a document of the site in a frame (`<iframe>`)" || styles.RulesDropped != 0 {
				t.Errorf("/frame/: %+v", styles)
			}
			// The page the user edits has its whole sheet, and the report
			// says why.
			if styles := s.page(t, "/edit/").Styles; styles == nil || !styles.Unpruned || styles.Why != "the page has an element the user edits (`contenteditable`)" || styles.RulesDropped != 0 {
				t.Errorf("/edit/: %+v", styles)
			}
			// The builder's own `<script>` — "/" mounts a behaviour — is
			// not a script of the page's own: the page is pruned, however
			// the script is delivered. (An author's is shell-script, and
			// no page: TestErrors.)
			if styles := s.page(t, "/").Styles; styles == nil || styles.Unpruned || styles.Why != "" || styles.Rules != len(everything) || styles.RulesDropped != len(everything)-len(tt.home) {
				t.Errorf("/: %+v", styles)
			}
			if js := s.js(t, "/"); js == "" {
				t.Errorf("/ mounts a behaviour, and has no script")
			}
			// The control ships every rule to every page, whatever is in it.
			control := load(t, append([]string{"-p", "served", "--no-specialize"}, tt.args...)...)
			for _, p := range control.report.Pages {
				if got := orders(control.css(t, p.Pathname)); !slices.Equal(got, everything) {
					t.Errorf("the control: %s ships the rules %v", p.Pathname, got)
				}
			}
		})
	}
}

// TestServedGivesUp: a page whose document is still another one after
// `rounds` rounds — a rule selects on the URL of the sheet it is in, which
// carries the sheet's hash — is given its whole sheet: that depends on
// nothing. Forced here: no round is allowed after the guess, and with files
// every page's document is another than the guess.
func TestServedGivesUp(t *testing.T) {
	defer func(n int) { rounds = n }(rounds)
	rounds = 1
	s := load(t, "-p", "served", "--inline", "never")
	for _, p := range s.report.Pages {
		if got := orders(s.css(t, p.Pathname)); len(got) != 23 {
			t.Errorf("%s ships the rules %v", p.Pathname, got)
		}
		if p.Styles == nil || !p.Styles.Unpruned || p.Styles.RulesDropped != 0 || p.Styles.Rules != 23 {
			t.Errorf("%s: %+v", p.Pathname, p.Styles)
		}
	}
	if because := s.page(t, "/").Styles.Why; because != "its rules select on the URL of the stylesheet they are in" {
		t.Errorf("/: because %q", because)
	}
	// The six sheets are one now: a file for the site. And the two scripts:
	// of "/", which mounts `mark`, and of "/toggle/".
	pages := 0
	for _, blob := range s.report.Blobs {
		pages += len(blob.Pages)
	}
	if len(s.report.Pages) != 6 || len(s.report.Blobs) != 3 || pages != 6+1+1 {
		t.Errorf("blobs: %+v", s.report.Blobs)
	}
	// A page that is given its whole sheet keeps its own `<style>` elements
	// as they were written: nothing of the page was pruned.
	if html := s.files["styled/index.html"]; !strings.Contains(html, ".never { color: red; }") || !strings.Contains(html, "--unread: 0;") {
		t.Errorf("/styled/ has its whole sheet, and pruned <style> elements:\n%s", html)
	}
}

// TestStylesOf: the page's own `<style>` elements are found in the page's
// text and paired with the parser's by their order and their text; when the
// two disagree — markup in an SVG `<style>`, which is elements to a parser
// and text to the tokenizer — none is given to the pruner, and each is left
// as it is.
func TestStylesOf(t *testing.T) {
	pair := func(src string) (styles []ownStyle, pruned int) {
		styles, _ = ownStyles(render.Page{HTML: src}) // what esbuild says of a text is not asked here
		doc, err := html.Parse(strings.NewReader(doctype + document(src, "/", "<style></style>", "")))
		if err != nil {
			t.Fatal(err)
		}
		given, at := stylesOf(doc, builders(doc, "<style></style>"), styles)
		for k, style := range given {
			if style.CSS != styles[at[k]].flat || style.Node.Data != "style" {
				t.Errorf("%s: style %d is paired with %+v", src, k, styles[at[k]])
			}
		}
		return styles, len(given)
	}
	// Three of the page's own — one that is not CSS among them — and the
	// place of each text.
	src := `<html><head><style>a{b:c}</style></head><body><style type="text/less">.x{.y()}</style><p>x</p><svg><style>circle{fill:red}</style></svg></body></html>`
	styles, pruned := pair(src)
	if len(styles) != 3 || pruned != 2 || src[styles[0].pos:styles[0].end] != "a{b:c}" || src[styles[2].pos:styles[2].end] != "circle{fill:red}" || styles[1].css || styles[1].flat != "" {
		t.Errorf("styles: %+v, %d to prune", styles, pruned)
	}
	// Markup in an SVG <style>: the parser makes an element of it.
	if styles, pruned := pair(`<html><head><style>a{b:c}</style></head><body><svg><style>a{b:c}<b>x</b></style></svg></body></html>`); len(styles) != 2 || pruned != 0 {
		t.Errorf("markup in an SVG <style>: %+v, %d to prune", styles, pruned)
	}
}

// TestOwnStyles: a `<style>` element of the page's own is always allowed, and
// is CSS of the page (builder.md, *The builder's own elements*): pruned with
// the page's sheet, by the same rules, and written back in place, minified —
// the one thing of React's HTML, beside the base, that packaging writes anew.
func TestOwnStyles(t *testing.T) {
	const (
		// In the head, before the sheet: the rule of no element and the
		// property nobody reads are gone, nesting is lowered, and the layer
		// whose block was emptied is still ordered here.
		head = `<style>.note{color:var(--note)}.note b{font-weight:600}:root{--note: teal}@layer late;</style>`
		// Under a medium: pruned as any other.
		print = `<style media="print">.note{display:none}</style>`
		// In the body, after the sheet: the animation a dropped rule named
		// is gone with it.
		body = `<style>@keyframes blink{to{opacity:0}}.note b{animation:blink 1s}@layer late{.note{margin:0}}</style>`
		// Not CSS: as it was written.
		less = `<style type="text/x-less">.never { .mixin(); }</style>`
	)
	for _, args := range [][]string{{"--inline", "always"}, {"--inline", "never"}, {"--base", "/docs/"}} {
		s := load(t, append([]string{"-p", "served"}, args...)...)
		html := s.files["styled/index.html"]
		at := []int{strings.Index(html, head), strings.Index(html, `</head>`), strings.Index(html, print), strings.Index(html, body), strings.Index(html, less)}
		if !slices.IsSorted(at) || at[0] < 0 || strings.Count(html, "never") != 1 {
			t.Errorf("%v: the page's own <style> elements, at %v:\n%s", args, at, html)
		}
		// The sheet is pruned against the same page: `style` and `title +
		// style` match the page's own elements too, `body > style` only
		// them; with the sheet as a file, its `<link>`.
		want := []int{6, 9, 13}
		if slices.Contains(args, "never") {
			want = []int{6, 7, 9, 13}
		}
		if got := orders(s.css(t, "/styled/")); !slices.Equal(got, want) {
			t.Errorf("%v: /styled/ ships the rules %v, want %v", args, got, want)
		}
		// The report counts them, in a row of their own; the page's HTML is
		// the page as it is written, without the builder's two elements.
		p := s.page(t, "/styled/")
		if p.Styles == nil || p.Styles.Unpruned || len(p.Styles.Sources) != 2 || p.Styles.Sources[1].File != "<style>" || p.Styles.Sources[1].Rules != 10 || p.Styles.Sources[1].RulesDropped != 4 {
			t.Errorf("%v: %+v", args, p.Styles)
		}
		if link := `<link rel="stylesheet" href="` + p.CSS.URL + `">`; p.CSS.Delivery == "file" && len(strings.Replace(html, link, "", 1)) != p.HTML.Raw {
			t.Errorf("%v: the HTML is %d B in the report, and the page without the builder's <link> is not", args, p.HTML.Raw)
		}
	}
	// The control prunes nothing: the page's own `<style>` elements are as
	// they were written.
	control := load(t, "-p", "served", "--no-specialize")
	if html := control.files["styled/index.html"]; !strings.Contains(html, ".never { color: red; }") || !strings.Contains(html, "& b { font-weight: 600; }") {
		t.Errorf("the control:\n%s", html)
	}

	// Pages of a project of the test's own: what is not pruned, and why.
	work := scratch(t, nil, map[string]string{
		"tsconfig.json": tsconfig,
		// No sheet of the builder's at all: the page's own is pruned all
		// the same, and the page gets no other element.
		"pages/index.rtsx": pageOf("<style>{`p { margin: 0 } .gone { margin: 0 }`}</style><p>x</p>"),
		// A <template>: the page is not pruned, nor its own <style>.
		"pages/template/index.rtsx": pageOf("<style>{`.gone { margin: 0 }`}</style><template><p>x</p></template>"),
		// … and a <noscript>, here around the <style> itself: text to a
		// browser that runs scripts.
		"pages/noscript/index.rtsx": pageOf("<noscript><style>{`.gone { margin: 0 }`}</style></noscript><p>x</p>"),
		// In SVG it is CSS of the page too.
		"pages/svg/index.rtsx": pageOf("<svg><style>{`circle { fill: red } rect { fill: red }`}</style><circle r=\"1\" /></svg>"),
		// A text that does not read — an unclosed string — is kept byte for
		// byte, and said.
		"pages/broken/index.rtsx": pageOf("<style>{`p { content: \"x } .gone { margin: 0 }`}</style><p>x</p>"),
		// An `@import` in it is a sheet that is not here: kept, and every
		// custom property with it.
		"pages/import/index.rtsx": pageOf("<style>{`@import \"/theme.css\"; :root { --unread: 1 } .gone { margin: 0 }`}</style><p>x</p>"),
	})
	stdout, stderr, status := runIn(t, work, "--report")
	if status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	got := tree(t, filepath.Join(work, "dist"))
	for file, want := range map[string]string{
		"index.html":          "<body><style>p{margin:0}</style><p>x</p></body>",
		"template/index.html": "<body><style>.gone { margin: 0 }</style><template><p>x</p></template></body>",
		"svg/index.html":      "<body><svg><style>circle{fill:red}</style><circle r=\"1\"></circle></svg></body>",
		"noscript/index.html": "<body><noscript><style>.gone { margin: 0 }</style></noscript><p>x</p></body>",
		"broken/index.html":   "<body><style>p { content: \"x } .gone { margin: 0 }</style><p>x</p></body>",
		"import/index.html":   "<body><style>@import\"/theme.css\";:root{--unread: 1}</style><p>x</p></body>",
	} {
		if !strings.Contains(got[file], want) || strings.Contains(got[file], "</style></head>") {
			t.Errorf("%s: no %s in\n%s", file, want, got[file])
		}
	}
	// What esbuild says of the text, and that the pruner does not read it.
	if !strings.Contains(stdout, "pages/broken/index.rtsx: warning css-warning: Page /broken/: a `<style>` of the page: Unterminated string token\n") ||
		!strings.Contains(stdout, "pages/broken/index.rtsx: warning css-warning: Page /broken/: a `<style>` of the page is not pruned: css: unterminated string\n") || strings.Count(stdout, "css-warning") != 3 {
		t.Errorf("what the build says:\n%s", stdout)
	}
	if !strings.Contains(stdout, "CSS rules  not pruned: the page has a <template>") || !strings.Contains(stdout, "1 kept,   1 dropped  <style>  the page's own, in its HTML") {
		t.Errorf("the report:\n%s", stdout)
	}
}

// TestReferences: `-p` is the project as for `check` — a tsconfig that only
// references others is checked, and built, through them (builder.md, the
// flags; *The pipeline*).
func TestReferences(t *testing.T) {
	// The fixture through its solution-style tsconfig: the same site.
	out, _ := buildSite(t, "-p", "site/tsconfig.solution.json", "--report")
	golden(t, "auto/out", tree(t, out))

	// Vite's template: `tsconfig.json` references `tsconfig.app.json`, which
	// lists the pages, and `tsconfig.node.json`, which does not.
	app := strings.Replace(tsconfig, `"include": ["."]`, `"include": ["pages"]`, 1)
	node := strings.Replace(tsconfig, `"include": ["."]`, `"include": ["tool.ts"]`, 1)
	work := scratch(t, nil, map[string]string{
		"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.node.json" }, { "path": "./tsconfig.app.json" }] }`,
		"tsconfig.app.json":  app,
		"tsconfig.node.json": node,
		"tool.ts":            "export const port: number = 5173;\n",
		"pages/index.rtsx":   pageOf(`<p>Home</p>`),
	})
	if reports := check.Run(filepath.ToSlash(filepath.Join(work, "tsconfig.json"))); len(reports) != 0 {
		t.Fatalf("check: %+v", reports)
	}
	for _, p := range []string{".", "tsconfig.json", "tsconfig.app.json"} {
		stdout, stderr, status := runIn(t, work, "-p", p)
		if status != 0 || stderr != "" || stdout != "1 page written to dist\n" {
			t.Errorf("-p %s: status %d\n%s%s", p, status, stdout, stderr)
		}
		if html := tree(t, filepath.Join(work, "dist"))["index.html"]; !strings.Contains(html, "<p>Home</p>") {
			t.Errorf("-p %s: the page: %s", p, html)
		}
	}
	// An error of a referenced project that holds no page stops the build,
	// and is printed as `check` prints it.
	if err := os.WriteFile(filepath.Join(work, "tool.ts"), []byte("export const port: number = \"5173\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, status := runIn(t, work)
	if want := "tool.ts(1,14): error TS2322: Type 'string' is not assignable to type 'number'.\n"; status != 1 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, want)
	}
	// A page no project lists is not built unchecked.
	if err := os.WriteFile(filepath.Join(work, "tsconfig.app.json"), []byte(strings.Replace(tsconfig, `"include": ["."]`, `"include": ["tool.ts"]`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "tool.ts"), []byte("export const port: number = 5173;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, status = runIn(t, work)
	if want := "pages/index.rtsx(1,1): error render-bundle: The page is not a module of the project: the tsconfig does not include it\n"; status != 1 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, want)
	}
}

// TestLinked: `pages` and `public` may be symbolic links (builder.md,
// *Routes*): the pages are found through the link, and are the modules the
// program has under whichever name it gave them.
func TestLinked(t *testing.T) {
	files := map[string]string{
		"tsconfig.json":               tsconfig,
		"real-pages/index.rtsx":       pageOf(`<a href="/x.txt">x</a><a href="/guide/">guide</a>`),
		"real-pages/guide/index.rtsx": pageOf(`<a href="/">home</a><section #part />`),
		// A segment: the page mounts it, so it is no variant — under whichever
		// of its two names the program holds it (variants).
		"real-pages/guide/part.rtsx": "export default function Part() {\n  return <p>part</p>;\n}\n",
		"real-public/x.txt":          "x\n",
	}
	built := func(t *testing.T, work string, args ...string) {
		t.Helper()
		stdout, stderr, status := runIn(t, work, args...)
		if status != 0 || stderr != "" || stdout != "2 pages written to dist\n" {
			t.Fatalf("%v: status %d\n%s%s", args, status, stdout, stderr)
		}
		got := tree(t, filepath.Join(work, "dist"))
		if got["x.txt"] != "x\n" || !strings.Contains(got["index.html"], `href="/guide/"`) || !strings.Contains(got["guide/index.html"], `href="/"`) {
			t.Errorf("%v: the output: %v", args, slices.Sorted(maps.Keys(got)))
		}
	}
	link := func(t *testing.T, work, target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(work, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("both are links", func(t *testing.T) {
		work := scratch(t, nil, files)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		built(t, work)
		built(t, work, "--pages", "pages")
		built(t, work, "--pages", filepath.Join(work, "pages"))
		built(t, work, "--pages", "real-pages")
	})
	t.Run("the pages lead elsewhere: public is beside the link", func(t *testing.T) {
		work := scratch(t, nil, map[string]string{
			"site/tsconfig.json":            tsconfig,
			"site/public/x.txt":             "x\n",
			"shared/pages/index.rtsx":       files["real-pages/index.rtsx"],
			"shared/pages/guide/index.rtsx": files["real-pages/guide/index.rtsx"],
			"shared/pages/guide/part.rtsx":  files["real-pages/guide/part.rtsx"],
			"shared/public/y.txt":           "not the site's\n",
		})
		link(t, filepath.Join(work, "site"), filepath.Join("..", "shared", "pages"), "pages")
		built(t, filepath.Join(work, "site"))
		if got := tree(t, filepath.Join(work, "site", "dist")); got["y.txt"] != "" {
			t.Errorf("the output: %v", slices.Sorted(maps.Keys(got)))
		}
	})
	t.Run("public alone", func(t *testing.T) {
		work := scratch(t, nil, files)
		if err := os.Rename(filepath.Join(work, "real-pages"), filepath.Join(work, "pages")); err != nil {
			t.Fatal(err)
		}
		link(t, work, "real-public", "public")
		built(t, work)
	})
	t.Run("the project's directory is reached through a link", func(t *testing.T) {
		work := scratch(t, nil, files)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		through := filepath.Join(t.TempDir(), "through")
		if err := os.Symlink(work, through); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, status := runIn(t, through)
		if status != 0 || stderr != "" || stdout != "2 pages written to dist\n" {
			t.Fatalf("status %d\n%s%s", status, stdout, stderr)
		}
	})
	t.Run("the tsconfig lists the pages where they are, not through the link", func(t *testing.T) {
		listed := maps.Clone(files)
		listed["tsconfig.json"] = strings.Replace(tsconfig, `"include": ["."]`, `"include": ["real-pages"]`, 1)
		work := scratch(t, nil, listed)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		built(t, work)
	})
}

// TestConflicts: a file of `public/` cannot be where the build writes
// (builder.md, *Routes*: public-conflict) — also not as a file where the
// build needs a directory, or under what the build writes as a file. Found
// before anything is written: the write would stop half-way.
func TestConflicts(t *testing.T) {
	routes := []render.Route{
		{Pathname: "/"}, {Pathname: "/guide/"}, {Pathname: "/guide/more/"}, {Pathname: "/a/b/c/"},
		// A route with two variants, and one whose only variant is not `index`.
		{Pathname: "/account/", Variant: "index"}, {Pathname: "/account/", Variant: "guest"}, {Pathname: "/gate/", Variant: "closed"},
	}
	opts := Options{Pages: filepath.FromSlash("/site/pages")}
	tests := []struct {
		file string
		says string // "": no conflict
	}{
		{"favicon.svg", ""},
		{"guide/logo.svg", ""},
		{"guide/more/index.htm", ""},
		{"a/b/index.html", ""}, // no page is written there
		{"guides", ""},
		{"_rgb/x.css", ""},
		{"index.html", "The page / is written to `index.html`: a file of `public/` cannot be there"},
		{"guide/more/index.html", "The page /guide/more/ is written to `guide/more/index.html`: a file of `public/` cannot be there"},
		{"_rg/mine.css", "`_rg/` is the builder's: a file of `public/` cannot be there"},
		{"_rg", "`_rg/` is the builder's: a file of `public/` cannot be there"},
		{"guide", "The page /guide/ is written to `guide/index.html`: `guide` is a directory of the output, and cannot be a file of `public/`"},
		{"guide/more", "The page /guide/more/ is written to `guide/more/index.html`: `guide/more` is a directory of the output, and cannot be a file of `public/`"},
		{"a", "The page /a/b/c/ is written to `a/b/c/index.html`: `a` is a directory of the output, and cannot be a file of `public/`"},
		{"a/b", "The page /a/b/c/ is written to `a/b/c/index.html`: `a/b` is a directory of the output, and cannot be a file of `public/`"},
		{"index.html/x.txt", "The page / is written to `index.html`: a file of `public/` cannot be under it"},
		{"guide/index.html/deep/x.txt", "The page /guide/ is written to `guide/index.html`: a file of `public/` cannot be under it"},
		// Every variant's document, under its own name.
		{"account/guest.htm", ""},
		{"gate/index.html", ""}, // the route has no `index`: nothing is written there
		{"account/index.html", "The page /account/ is written to `account/index.html`: a file of `public/` cannot be there"},
		{"account/guest.html", "The page /account/guest.html is written to `account/guest.html`: a file of `public/` cannot be there"},
		{"gate/closed.html", "The page /gate/closed.html is written to `gate/closed.html`: a file of `public/` cannot be there"},
		{"gate", "The page /gate/closed.html is written to `gate/closed.html`: `gate` is a directory of the output, and cannot be a file of `public/`"},
		{"account/guest.html/x.txt", "The page /account/guest.html is written to `account/guest.html`: a file of `public/` cannot be under it"},
	}
	for _, tt := range tests {
		reports := conflicts(opts, routes, []string{tt.file})
		switch {
		case tt.says == "" && len(reports) != 0:
			t.Errorf("%s: %+v", tt.file, reports)
		case tt.says != "" && (len(reports) != 1 || reports[0].Code != "public-conflict" || reports[0].Message != tt.says || reports[0].File != "/site/public/"+tt.file):
			t.Errorf("%s: %+v\nwant: %s", tt.file, reports, tt.says)
		}
	}
}

// TestFailedWrite: the new output is written beside the old one, and takes
// its place when all of it is there (builder.md, *The output directory*) —
// a build that cannot write leaves the last output as it was.
func TestFailedWrite(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file that cannot be read")
	}
	work := scratch(t, []string{"site"}, nil)
	site := filepath.Join(work, "site")
	out := filepath.Join(site, "dist")
	if stdout, stderr, status := runIn(t, site); status != 0 {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	before := tree(t, out)

	// A file of `public/` that cannot be read: found when it is copied.
	secret := filepath.Join(site, "public", "robots.txt")
	if err := os.Chmod(secret, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(secret, 0o644)
	stdout, stderr, status := runIn(t, site)
	if status != 1 || stdout != "" || !strings.HasPrefix(stderr, "reactogenic build: ") || !strings.Contains(stderr, "robots.txt") {
		t.Errorf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
	if after := tree(t, out); !maps.Equal(before, after) {
		t.Errorf("a build that could not write changed the output: %v, was %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	if entries, _ := os.ReadDir(out); len(entries) != len(topLevel(before)) {
		t.Errorf("something is left beside the output: %v", entries)
	}

	// Into a directory that is not there: nothing is left that the next
	// build would not take for its own.
	fresh := filepath.Join(work, "fresh")
	if _, _, status := runIn(t, site, "--out", fresh); status != 1 {
		t.Fatalf("status %d", status)
	}
	if entries, _ := os.ReadDir(fresh); len(entries) != 0 {
		t.Errorf("a build that could not write left %v", entries)
	}
	os.Chmod(secret, 0o644)
	if stdout, stderr, status := runIn(t, site, "--out", fresh); status != 0 {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	if after := tree(t, fresh); !maps.Equal(before, after) {
		t.Errorf("the output: %v, want %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
}

// topLevel are the entries of a tree's root.
func topLevel(files map[string]string) map[string]bool {
	top := map[string]bool{}
	for file := range files {
		name, _, _ := strings.Cut(file, "/")
		top[name] = true
	}
	return top
}

// TestNaming: a file of the report is named by what it is on every machine
// (builder.md, *The report*: files) — the project's own from the project
// directory, a package's by its package, wherever the install put it.
func TestNaming(t *testing.T) {
	root := real(t.TempDir())
	for file, content := range map[string]string{
		"repo/site/site.css":                                                       "",
		"repo/site/node_modules/plain/package.json":                                `{"name": "plain"}`,
		"repo/site/node_modules/plain/dist/esm/package.json":                       `{"type": "module"}`,
		"repo/site/node_modules/plain/dist/esm/a.css":                              "",
		"repo/site/node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/package.json": `{"name": "@s/ui", "version": "1.0.0"}`,
		"repo/site/node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/src/b.ts":     "",
		"repo/packages/ui/package.json":                                            `{"name": "@reactogenic/ui"}`,
		"repo/packages/ui/src/behaviors/overlays.ts":                               "",
		"repo/shared/tokens.css":                                                   "",
		"elsewhere/lib/package.json":                                               `not JSON`,
		"elsewhere/lib/c.css":                                                      "",
	} {
		if err := writeFile(root, file, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	names := naming{dir: filepath.Join(root, "repo", "site"), packages: map[string]string{}}
	for name, want := range map[string]string{
		"site.css":                          "site.css",
		"pages/guide/page.css":              "pages/guide/page.css", // need not be there
		"node_modules/plain/dist/esm/a.css": "plain/dist/esm/a.css",
		"node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/src/b.ts": "@s/ui/src/b.ts",
		"../packages/ui/src/behaviors/overlays.ts":                   "@reactogenic/ui/src/behaviors/overlays.ts",
		// Of no package: where it is, from the project.
		"../shared/tokens.css":      "../shared/tokens.css",
		"../../elsewhere/lib/c.css": "../../elsewhere/lib/c.css",
		behaviors.Entry:             behaviors.Entry,
		behaviors.Runtime:           behaviors.Runtime,
	} {
		if got := names.of(name); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}

// TestReportNamesNoMachine: a project whose packages are outside it —
// reached through a link, as with a workspace, `npm link` or a store — has a
// report that names no path of the machine (builder.md, *The report*).
func TestReportNamesNoMachine(t *testing.T) {
	work := scratch(t, []string{"site"}, nil)
	stdout, stderr, status := runIn(t, filepath.Join(work, "site"), "--report")
	if status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	report := tree(t, filepath.Join(work, "site", "dist"))[reportFile]
	repo, _ := filepath.Abs("../../..")
	for _, machine := range []string{"../", filepath.ToSlash(work), filepath.ToSlash(repo), "node_modules"} {
		if strings.Contains(report, machine) || strings.Contains(stdout, machine) {
			t.Errorf("the report holds %q", machine)
		}
	}
	for _, name := range []string{`"@reactogenic/ui/src/behaviors/overlays.ts"`, `"@reactogenic/ui/src/dialog.css"`, `"site.css"`} {
		if !strings.Contains(report, name) {
			t.Errorf("the report does not name %s", name)
		}
	}
	// The same bytes as the build of the fixture in the repository.
	golden(t, "auto/out", tree(t, filepath.Join(work, "site", "dist")))
}
