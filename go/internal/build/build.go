// Package build is the builder: `reactogenic build`
// (specs/phase02/builder.md). Pages in .rtsx and framework-owned layout
// components become, per page, plain HTML, the CSS that page can use and the
// JS of the behaviours it mounted — no React in the output.
//
// This package is the driver of builder.md's pipeline; each stage is a
// package of its own:
//
//	program, diagnostics   internal/check: what `check` prints, and stops
//	routes                 routes.go
//	execute                render: the HTML and the record of each page
//	check the page         pagecheck, on the parsed HTML
//	JS                     behaviors
//	CSS                    css.go (esbuild: a page's CSS in import order), cssprune
//	package                pack.go, html.go: dedupe by content, inline or file, write
//	the report             bytes.go: <out>/_rg/report.json, and `--report`
//
// Main is the command; Run is the build, for a caller that has the options.
// Two builds of one input are byte-identical: nothing here is ordered by a
// map, and nothing of the machine — a path, a clock — is in the output.
package build

import (
	"cmp"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/microsoft/TypeScript/tsc/rtsx"
	"golang.org/x/net/html"

	"github.com/reactogenic/reactogenic/go/internal/build/behaviors"
	"github.com/reactogenic/reactogenic/go/internal/build/cssprune"
	"github.com/reactogenic/reactogenic/go/internal/build/pagecheck"
	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// The values of `--inline` (builder.md, *Packaging*).
const (
	InlineAuto   = "auto"
	InlineAlways = "always"
	InlineNever  = "never"
)

// Options are the flags of `reactogenic build`, resolved: every path is
// absolute.
type Options struct {
	Config string // the tsconfig.json, its symbolic links resolved: as the program names its files
	// Pages is the root of the routes. It may be a link, or be reached
	// through one: a page is then looked up in the program by the file it
	// is (named).
	Pages string
	// Public is the directory of files copied as they are; "": `public`
	// next to Pages.
	Public string
	Out    string // the output directory
	Base   string // "/", "/docs/": normalised (NormalBase)
	Inline string // InlineAuto, InlineAlways or InlineNever
	// NoSpecialize builds the control (builder.md, *The control*): one
	// unpruned CSS bundle and one script for the site.
	NoSpecialize bool
}

// dir is the project directory: the tsconfig's.
func (o Options) dir() string { return filepath.Dir(o.Config) }

// public is the directory of files copied as they are: `public` next to the
// pages.
func (o Options) public() string {
	if o.Public != "" {
		return o.Public
	}
	return filepath.Join(filepath.Dir(o.Pages), "public")
}

// Run builds the site and writes it to opts.Out. The reports are what the
// build has to say, in the order `check` prints them; with an error among
// them nothing is written, and the output directory is as it was. The byte
// report is nil then. err is a failure that is not the project's: the
// output cannot be written — or is not the builder's to empty (CheckOut),
// which is asked again before anything is removed.
//
// The caller has registered the .rtsx transform (mapper.RegisterStrict).
func Run(opts Options) (reports []report.Report, bytes *Report, err error) {
	dir := opts.dir()

	// Routes first: they cost nothing, and without a page there is nothing
	// to check the project for.
	routes, err := findRoutes(opts.Pages)
	if err != nil || len(routes) == 0 {
		message := fmt.Sprintf("%s holds no page: no `index.rtsx` or `index.tsx`", opts.Pages)
		if err != nil {
			message = opts.Pages + " is not a directory"
		}
		return []report.Report{{Code: "pages-not-found", Message: message}}, nil, nil
	}
	static, err := publicFiles(opts.public())
	if err != nil {
		return nil, nil, err
	}

	// The program and its diagnostics, as `check` — through the projects the
	// tsconfig references too: any error stops the build. The pages are
	// rendered from the project that lists them.
	var files []string
	for _, route := range routes {
		files = append(files, route.File, filepath.ToSlash(real(filepath.FromSlash(route.File))))
	}
	reports, program := check.Program(filepath.ToSlash(opts.Config), files)
	if program == nil || check.Errors(reports) > 0 {
		return reports, nil, nil
	}
	named(program, routes)

	// What `public/` holds is in the output where the build would write.
	reports = append(reports, conflicts(opts, routes, static)...)

	// Execute: the HTML and the record of each page.
	pages, rendered := render.Render(program, routes, render.Options{Dir: dir})
	if reports = append(reports, rendered...); check.Errors(reports) > 0 {
		return sorted(reports), nil, nil
	}

	if len(pages) != len(routes) {
		return nil, nil, fmt.Errorf("render: %d pages of %d rendered, and no error", len(pages), len(routes))
	}

	// The checks, on the page as rendered and as a browser will parse it:
	// before any link gets the base (builder.md, *Checks on the page*).
	served := map[string]bool{}
	for _, file := range static {
		served["/"+file] = true
	}
	for _, page := range pages {
		doc, err := html.Parse(strings.NewReader(doctype + page.HTML))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", page.File, err)
		}
		reports = append(reports, pagecheck.Check(page, doc, routes, served)...)
	}

	site := make([]built, len(pages))
	for i, page := range pages {
		site[i].page = page
	}

	// JS: the behaviours each page mounted; for the control, one script.
	// Before the CSS: a page's sheet is pruned against the page as it is
	// served, its `<script>` included.
	scripts := behaviors.Options{Dir: dir, Base: opts.Base, Cache: &behaviors.Cache{}}
	if opts.NoSpecialize {
		js, wrong := behaviors.BuildControl(pages, scripts)
		reports = append(reports, wrong...)
		for i := range site {
			site[i].js = js
		}
	} else {
		for i := range site {
			p := &site[i]
			var wrong []report.Report
			p.js, p.modules, wrong = behaviors.Build(p.page, scripts)
			reports = append(reports, wrong...)
		}
	}

	// CSS: one esbuild build for the site's stylesheets, then per page.
	sheets, bundled := styles(program, dir, routes, opts.NoSpecialize)
	reports = append(reports, bundled...)
	switch {
	case sheets == nil: // the bundle failed: reported
	case opts.NoSpecialize:
		// The control's CSS knows no page: everything any page imports.
		css := minify(sheets[0])
		for i := range site {
			site[i].css = css
		}
	default:
		wrong, err := prune(opts, site, sheets)
		if err != nil {
			return nil, nil, err
		}
		reports = append(reports, wrong...)
	}

	reports = sorted(reports)
	if check.Errors(reports) > 0 {
		return reports, nil, nil
	}
	bytes, err = write(opts, site, static)
	return reports, bytes, err
}

// named gives each route's file the name the program has for it. They
// differ when the pages are reached through a symbolic link: the program
// names a file its tsconfig lists as the tsconfig reaches it (`pages/…`,
// link or not) and one it imports by where it is. A page the program does
// not hold keeps its name: render says so (render-bundle).
func named(program *rtsx.Program, routes []render.Route) {
	var byFile map[string]string // the program's modules that may be pages, by the file they are
	for i, route := range routes {
		if render.Source(program, route.File) != nil {
			continue
		}
		if byFile == nil {
			byFile = map[string]string{}
			for _, file := range program.GetSourceFiles() {
				name := file.FileName()
				if !slices.Contains(index, path.Base(name)) || file.IsDeclarationFile {
					continue
				}
				// One file under two names: the first, as the program orders them.
				if is := real(filepath.FromSlash(name)); byFile[is] == "" {
					byFile[is] = name
				}
			}
		}
		if name := byFile[real(filepath.FromSlash(route.File))]; name != "" {
			routes[i].File = name
		}
	}
}

// rounds is how often a page's CSS is pruned again before the page is given
// its whole sheet (prune). A variable for the tests.
var rounds = 8

// prune makes each page's CSS of its sheet: what the page as it is served
// can use (builder.md, *CSS*), minified.
//
// The page that is served is not the page that was rendered: its links
// carry the base, and packaging puts a `<style>` or a `<link>` into it, and
// a `<script>` — elements a rule may select (`a[href^="/docs/"]`,
// `script { display: block }`). Which of them, and under which URL, depends
// on the blob, that is on the pruned sheet itself. So a page is pruned
// against a guess — its sheet inlined — then the site is packaged, and a
// page whose document is another than the one it was pruned against is
// pruned again, until every page was pruned against what is written. The
// stylesheet's own element holds no text there: a sheet is not pruned
// against itself.
//
// One more round is all it takes, unless a rule selects on the sheet's own
// URL — which carries the hash of the sheet. Such a page is given its whole
// sheet after a few rounds: that depends on nothing.
func prune(opts Options, site []built, sheets []string) (reports []report.Report, err error) {
	against := make([]string, len(site)) // the document each page was pruned against
	whole := make([]bool, len(site))     // the page's sheet is final, whatever its document
	minified := map[string]string{}
	small := func(css string) string {
		if _, done := minified[css]; !done {
			minified[css] = minify(css)
		}
		return minified[css]
	}
	for round := 0; ; round++ {
		blobs, of, err := pack(site, opts.Inline)
		if err != nil {
			return nil, err
		}
		settled := true
		for i := range site {
			p := &site[i]
			if sheets[i] == "" || whole[i] {
				continue
			}
			head := "<style></style>"
			if round > 0 {
				head = element(blobs, of[i][0], opts)
			}
			served := doctype + document(p.page.HTML, opts.Base, head, element(blobs, of[i][1], opts))
			if served == against[i] {
				continue
			}
			settled, against[i] = false, served
			if round >= rounds {
				stats, err := cssprune.Whole(sheets[i])
				if err != nil {
					return nil, err // read before, by Prune
				}
				stats.Because = "its rules select on the URL of the stylesheet they are in"
				p.css, p.styles, whole[i] = small(sheets[i]), &stats, true
				continue
			}
			doc, err := html.Parse(strings.NewReader(served))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p.page.File, err)
			}
			pruned, stats, err := cssprune.Prune(sheets[i], doc)
			if err != nil {
				reports = append(reports, report.Page(p.page.File, p.page.Pathname, "css-bundle", err.Error()))
				p.css, p.styles, whole[i] = "", nil, true
				continue
			}
			p.css, p.styles = small(pruned), &stats
		}
		if settled {
			return reports, nil
		}
	}
}

// element is what packaging writes into a page for a blob, as the pruner is
// to see it: the element, without the text of one that is inlined. at < 0:
// the page has no such blob.
func element(blobs []*blob, at int, opts Options) string {
	if at < 0 {
		return ""
	}
	b := *blobs[at]
	if !b.file {
		b.content = ""
	}
	return b.tag(opts.Base, opts.NoSpecialize)
}

// built is a page with what was made for it.
type built struct {
	page    render.Page
	css, js string                  // minified; "": none
	styles  *cssprune.Stats         // what pruning did; nil for the control
	modules []behaviors.ModuleBytes // the script's bytes by input; nil for the control
}

// doctype is written before every page (builder.md, *Routes*); with it a
// page is in no-quirks mode, which the pruner relies on.
const doctype = "<!doctype html>"

// sorted orders reports by file and position, as `check` does, and says once
// what several pages share: a behaviour module that runs code when it is
// imported is one mistake, however many pages mount it.
func sorted(reports []report.Report) []report.Report {
	type key struct {
		file          string
		line, col     int
		code, message string
	}
	seen := map[key]bool{}
	var once []report.Report
	for _, r := range reports {
		if k := (key{r.File, r.Line, r.Col, r.Code, r.Message}); !seen[k] {
			seen[k] = true
			once = append(once, r)
		}
	}
	slices.SortStableFunc(once, func(a, b report.Report) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	return once
}

// NormalBase is `--base` as the build uses it: "", "docs", "/docs" and
// "/docs/" are "/" and "/docs/". ok is false for what is not a path a site
// can be served under: a URL with a scheme or a host, a query, a fragment,
// a character that would have to be escaped where the base is written — in
// an attribute, in the control's script — or a `%` that encodes nothing:
// the control's script decodes the page's pathname, which carries the base,
// and `decodeURIComponent` throws on one (builder.md, *The control*).
func NormalBase(base string) (normal string, ok bool) {
	trimmed := strings.Trim(base, "/")
	if strings.HasPrefix(base, "//") || strings.ContainsAny(trimmed, "?#\\\"'<>&: \t\r\n") || strings.Contains(trimmed, "//") {
		return "", false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		// As a browser reads it: `%2e%2e` is `..` too.
		decoded, err := url.PathUnescape(segment)
		if err != nil || !utf8.ValidString(decoded) || decoded == "." || decoded == ".." {
			return "", false
		}
	}
	if trimmed == "" {
		return "/", true
	}
	return "/" + trimmed + "/", true
}
