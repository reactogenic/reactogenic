package render

import (
	"cmp"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// run is one Render: the program, and the bundle made from it.
type run struct {
	program *rtsx.Program
	bundle  *bundle
}

// Render executes the pages of routes — the variants of the site's routes:
// modules of program, which has been checked — and returns the record of
// each page that rendered, in the order of routes, and the reports: the shell rules (builder.md, *Shell code in
// phase 2*), the pages that are not documents (*Routes*), and — as warnings
// — what shell code printed. A page that threw, or is not a document, has no
// record; when the bundle cannot be made, or a module throws while it loads,
// no page has.
//
// The caller stops on a report that is an error.
func Render(program *rtsx.Program, routes []Route, opts Options) ([]Page, []report.Report) {
	dir := opts.Dir
	if dir == "" {
		dir = program.GetCurrentDirectory()
	}
	within := limits{opts.Timeout, opts.Memory}
	if within.timeout == 0 {
		within.timeout = 30 * time.Second
	}
	if within.memory == 0 {
		within.memory = maxMemory
	}
	// What is built is what was checked: a page outside the program would
	// be read from the disk, by esbuild, unchecked.
	var reports []report.Report
	for _, route := range routes {
		if file := program.GetSourceFile(route.File); file == nil || file.IsDeclarationFile {
			reports = append(reports, report.Report{File: route.File, Line: 1, Col: 1, Code: "render-bundle", Message: "The page is not a module of the project: the tsconfig does not include it"})
		}
	}
	if len(reports) > 0 {
		return nil, sorted(reports)
	}
	b, reports := build(program, routes, filepath.FromSlash(dir), variant{platform: api.PlatformBrowser})
	if b == nil {
		return nil, sorted(reports)
	}
	r := &run{program, b}

	bytecode, err := compile(b.code)
	if err != nil {
		return nil, sorted(append(reports, report.Report{Code: "internal", Message: "render: " + err.Error()}))
	}
	// What the bundle does as it loads is no page's: the first runtime says
	// it, once.
	first, thrown, err := startCompiled(bytecode, within)
	if err != nil {
		return nil, sorted(append(reports, report.Report{Code: "internal", Message: "render: " + err.Error()}))
	}
	reports = append(reports, r.printed(nil, first.console())...)
	if thrown != nil || len(routes) == 0 {
		first.close()
		if thrown != nil {
			reports = append(reports, r.exception(nil, thrown))
		}
		return nil, sorted(reports)
	}

	var pages []Page
	for i := range routes {
		route := &routes[i]
		// A runtime per page (builder.md, *Shell code in phase 2*): the
		// bundle is loaded anew, so a page starts from what the modules are,
		// not from what the pages before it left of them.
		engine := first
		if i > 0 {
			if engine, thrown, err = startCompiled(bytecode, within); err != nil {
				reports = append(reports, report.Report{File: route.File, Line: 1, Col: 1, Code: "internal", Message: "render: " + err.Error()})
				continue
			}
			engine.console() // what loading prints was said, above
			if thrown != nil {
				// It loaded for the first page: only the timeout ends it now.
				reports = append(reports, r.exception(route, thrown))
				engine.close()
				continue
			}
		}
		result, err := engine.render(route.Path())
		reports = append(reports, r.printed(route, engine.console())...)
		engine.close()
		switch {
		case err != nil:
			reports = append(reports, report.Report{File: route.File, Line: 1, Col: 1, Code: "internal", Message: err.Error()})
		case result.Error != nil:
			reports = append(reports, r.exception(route, result.Error))
		case result.Page == nil:
			reports = append(reports, report.Report{File: route.File, Line: 1, Col: 1, Code: "internal", Message: "render: no page and no error"})
		default:
			page := Page{Route: *route, HTML: withoutDoctype(result.Page.HTML), Components: result.Page.Components}
			if !isDocument(page.HTML) {
				reports = append(reports, r.notDocument(route))
				continue
			}
			for _, m := range result.Page.Mounts {
				page.Mounts = append(page.Mounts, Mount{Module: m.Module, ID: m.ID, Flags: m.Flags, Data: m.Data})
			}
			pages = append(pages, page)
		}
	}
	return pages, sorted(reports)
}

// withoutDoctype: the doctype is the packager's (builder.md, *Routes*).
// React's streaming renderers write `<!DOCTYPE html>` before a root
// `<html>`; renderToStaticMarkup of 19.3 does not — nothing here relies on
// either.
func withoutDoctype(html string) string {
	const doctype = "<!doctype html>"
	if len(html) >= len(doctype) && strings.EqualFold(html[:len(doctype)], doctype) {
		return html[len(doctype):]
	}
	return html
}

// isDocument: the page's root element is `<html>`.
func isDocument(html string) bool {
	rest, ok := strings.CutPrefix(html, "<html")
	return ok && (strings.HasPrefix(rest, ">") || strings.HasPrefix(rest, " "))
}

// pageAt is where a page is in its file, as its author wrote it: at its
// `export default`.
func (r *run) pageAt(route *Route) (name string, at emit.Span, line, col int) {
	if file := r.program.GetSourceFile(route.File); file != nil {
		if span, ok := defaultExport(file); ok {
			return position(file, span)
		}
	}
	return route.File, emit.Span{}, 1, 1
}

// notDocument is page-not-document. For a variant other than `index` the
// message says what made the file a page: nothing mounts or imports it — a
// segment or a module that was left behind is found here (builder.md,
// *Routes*).
func (r *run) notDocument(route *Route) report.Report {
	out := report.Report{Code: "page-not-document", Message: "The page's root element is not `<html>`: a page renders the whole document"}
	if route.Name() != Index {
		out.Message = "`" + path.Base(route.File) + "` is a variant of the route " + route.Pathname + " — nothing mounts or imports it — and its root element is not `<html>`: a variant renders the whole document"
	}
	out.File, out.Span, out.Line, out.Col = r.pageAt(route)
	return out
}

// printed reports what shell code printed while route rendered — nil: while
// the bundle loaded — as warnings, where `console` was called: nobody reads
// the console of a build.
func (r *run) printed(route *Route, messages []printed) []report.Report {
	var reports []report.Report
	for _, m := range messages {
		out := report.Report{Severity: report.Warning, Code: "shell-console", Message: "console." + m.Level + ": " + m.Text}
		if at, ok := r.thrownAt(m.Stack); ok {
			out.File, out.Span, out.Line, out.Col = position(at.file, emit.Span{Pos: at.pos, End: at.pos})
		} else if route != nil {
			out.File, out.Line, out.Col = route.File, 1, 1
		}
		reports = append(reports, out)
	}
	return reports
}

// sorted orders reports by file and position, as internal/report does, and
// reports once what several pages share: a layout's error is one error.
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
