// Package build is the builder: `reactogenic build`
// (specs/phase02/builder.md). Pages in .rtsx and framework-owned layout
// components become, per page, plain HTML, the CSS that page can use and the
// JS of the behaviours it mounted — no React in the output.
//
// This package is the driver of builder.md's pipeline; each stage is a
// package of its own:
//
//	program, diagnostics   internal/report: what `check` prints, and stops
//	routes                 routes.go
//	execute                render: the HTML and the record of each page
//	check the page         pagecheck, on the parsed HTML
//	CSS                    css.go (esbuild: a page's CSS in import order), cssprune
//	JS                     behaviors
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
	"path/filepath"
	"slices"
	"strings"

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
// absolute, with its symbolic links resolved — the names the program gives
// its files.
type Options struct {
	Config string // the tsconfig.json
	Pages  string // the root of the routes
	Out    string // the output directory
	Base   string // "/", "/docs/": normalised (base)
	Inline string // InlineAuto, InlineAlways or InlineNever
	// NoSpecialize builds the control (builder.md, *The control*): one
	// unpruned CSS bundle and one script for the site.
	NoSpecialize bool
}

// dir is the project directory: the tsconfig's.
func (o Options) dir() string { return filepath.Dir(o.Config) }

// public is the directory of files copied as they are: `public` next to the
// pages.
func (o Options) public() string { return filepath.Join(filepath.Dir(o.Pages), "public") }

// Run builds the site and writes it to opts.Out. The reports are what the
// build has to say, in the order `check` prints them; with an error among
// them nothing is written, and the output directory is as it was. The byte
// report is nil then. err is a failure that is not the project's: the
// output cannot be written.
//
// The caller has registered the .rtsx transform (mapper.RegisterStrict) and
// checked opts.Out (CheckOut).
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

	// The program and its diagnostics, as `check`: any error stops the build.
	program, diagnostics := rtsx.NewProgram(filepath.ToSlash(opts.Config), filepath.ToSlash(dir), rtsx.OSFS())
	reports = report.Program(program, diagnostics)
	if program == nil || check.Errors(reports) > 0 {
		return reports, nil, nil
	}

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

	// Each page as the browser will parse it: the document the checks and
	// the pruner see is the one that is served (cssprune, *What it assumes*).
	docs := make([]*html.Node, len(pages))
	for i, page := range pages {
		doc, err := html.Parse(strings.NewReader(doctype + page.HTML))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", page.File, err)
		}
		docs[i] = doc
	}

	// The checks, on the page as rendered: before any link gets the base.
	files := map[string]bool{}
	for _, file := range static {
		files["/"+file] = true
	}
	for i, page := range pages {
		reports = append(reports, pagecheck.Check(page, docs[i], routes, files)...)
	}

	site := make([]built, len(pages))
	for i, page := range pages {
		site[i].page = page
	}

	// CSS: one esbuild build for the site's stylesheets, then per page.
	sheets, bundled := styles(program, dir, routes, opts.NoSpecialize)
	reports = append(reports, bundled...)
	for i := range site {
		p := &site[i]
		switch {
		case sheets == nil: // the bundle failed: reported
		case opts.NoSpecialize:
			// The control's CSS knows no page: everything any page imports.
			p.css = sheets[0]
		case sheets[i] != "":
			pruned, stats, err := cssprune.Prune(sheets[i], docs[i])
			if err != nil {
				reports = append(reports, report.Page(p.page.File, p.page.Pathname, "css-bundle", err.Error()))
				continue
			}
			p.css, p.styles = pruned, &stats
		}
	}
	// Minified once per distinct sheet: the control's is one for every page.
	minified := map[string]string{}
	for i := range site {
		p := &site[i]
		if _, done := minified[p.css]; !done {
			minified[p.css] = minify(p.css)
		}
		p.css = minified[p.css]
	}

	// JS: the behaviours each page mounted; for the control, one script.
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

	reports = sorted(reports)
	if check.Errors(reports) > 0 {
		return reports, nil, nil
	}
	bytes, err = write(opts, site, static)
	return reports, bytes, err
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
// or a character that would have to be escaped where the base is written —
// in an attribute, in the control's script.
func NormalBase(base string) (normal string, ok bool) {
	trimmed := strings.Trim(base, "/")
	if strings.HasPrefix(base, "//") || strings.ContainsAny(trimmed, "?#\\\"'<>&: \t\r\n") || strings.Contains(trimmed, "//") {
		return "", false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	if trimmed == "" {
		return "/", true
	}
	return "/" + trimmed + "/", true
}
