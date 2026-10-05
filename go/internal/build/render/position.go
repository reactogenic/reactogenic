package render

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// An exception of shell code is reported where the author wrote the code
// (builder.md, *Shell code in phase 2*): the engine's stack is in the
// bundle's coordinates; the bundle's source map leads to a program file's
// text — for an .rtsx module its emitted TSX — and the file's span map from
// there to the source.

// site is a position in the text of a program file.
type site struct {
	file *rtsx.SourceFile
	pos  int
}

// position returns where a span of file's text — the virtual text of a
// mapped file — is in what the author wrote (as internal/report places TS's
// diagnostics).
func position(file *rtsx.SourceFile, span emit.Span) (name string, at emit.Span, line, col int) {
	text := file.Text()
	if source, spans, _, ok := rtsx.MappedFile(file); ok {
		pos, end, _ := rtsx.SpanSource(spans, span.Pos, span.End)
		pos = min(max(pos, 0), len(source))
		text, span = source, emit.Span{Pos: pos, End: min(max(end, pos), len(source))}
	}
	line, col = emit.LineCol(text, span.Pos)
	return file.FileName(), span, line, col
}

// frameAt matches the position that ends a line of the engine's stack:
// `    at Dialog (<eval>:120:17)`, `    at <eval>:3:1`.
var frameAt = regexp.MustCompile(`:(\d+):(\d+)\)?$`)

// frame is a line of an engine stack that the bundle's source map knows: a
// module of the bundle, and a 0-based line and UTF-16 column in its text.
type frame struct {
	source       string
	line, column int
}

// frames reads an engine stack, innermost first.
func (r *run) frames(stack string) []frame {
	var out []frame
	for line := range strings.SplitSeq(stack, "\n") {
		m := frameAt.FindStringSubmatch(strings.TrimRight(line, " \r"))
		if m == nil {
			continue
		}
		bundleLine, _ := strconv.Atoi(m[1])
		bundleColumn, _ := strconv.Atoi(m[2])
		if source, srcLine, srcColumn, ok := r.bundle.sites.lookup(bundleLine, bundleColumn); ok {
			out = append(out, frame{source, srcLine, srcColumn})
		}
	}
	return out
}

// project returns the program's module of that name when it is the
// project's own code: a library's is not where the author looks.
func (r *run) project(name string) *rtsx.SourceFile {
	file := r.program.GetSourceFile(name)
	if file == nil || file.IsDeclarationFile || r.program.IsSourceFileFromExternalLibrary(file) {
		return nil
	}
	return file
}

// thrownAt is the innermost frame of an engine stack that is in the
// project's code: frames of the builder's own modules, of React and of other
// packages are passed over.
func (r *run) thrownAt(stack string) (site, bool) {
	for _, f := range r.frames(stack) {
		if file := r.project(f.source); file != nil {
			return site{file, offsetAt(file.Text(), f.line, f.column)}, true
		}
	}
	return site{}, false
}

// writtenAt is where the element of a component of an exception's stack was
// written: esbuild's position of a JSX element; for an element made by
// `createElement`, the call.
func (r *run) writtenAt(o owner) (site, bool) {
	if o.FileName == "" || o.LineNumber < 1 {
		return r.thrownAt(o.Stack)
	}
	file := r.project(strings.ReplaceAll(o.FileName, `\`, "/"))
	if file == nil {
		return site{}, false
	}
	return site{file, offsetAt(file.Text(), o.LineNumber-1, o.ColumnNumber-1)}, true
}

// handlerProp reads shell-handler's message (js/jsx.js): the prop.
var handlerProp = regexp.MustCompile("^The shell cannot handle events: `([^`]+)` on ")

// attribute narrows the site of an element to its attribute name, when the
// element writes it out: `onClick` of `<button onClick={…}>`. A function
// that arrives through a spread has no attribute to point at.
func attribute(at site, name string) site {
	for node := rtsx.TokenAt(at.file, at.pos); node != nil; node = node.Parent {
		if node.Kind != rtsx.KindJsxOpeningElement && node.Kind != rtsx.KindJsxSelfClosingElement {
			continue
		}
		for _, attr := range node.Attributes().Properties() {
			if attr.Kind == rtsx.KindJsxAttribute && rtsx.NodeText(attr.Name()) == name {
				return site{at.file, rtsx.TokenStart(at.file, attr.Name())}
			}
		}
		break
	}
	return at
}

// exception reports what a page threw — route nil: what a module threw while
// the bundle loaded. The exception's name is the code when the builder threw
// it (shell-handler, shell-react, shell-nondeterministic, shell-error for
// what the engine lacks, page-no-default, mount-data); anything else is
// shell-error, with the exception's message.
func (r *run) exception(route *Route, t *thrown) report.Report {
	out := report.Report{Code: "shell-error", Message: t.Message}
	switch {
	case strings.HasPrefix(t.Name, "shell-"), strings.HasPrefix(t.Name, "page-"), strings.HasPrefix(t.Name, "mount-"):
		out.Code = t.Name
	case t.Name != "" && t.Name != "Error":
		out.Message = t.Name + ": " + t.Message
	}
	// A variant other than `index`: the message says what made the file a
	// page — a module left behind in a route directory is found here.
	if out.Code == "page-no-default" && route != nil && route.Name() != Index {
		out.Message = "`" + path.Base(route.File) + "` is a variant of the route " + route.Pathname + " — nothing mounts or imports it — and has no default export that is a component"
	}
	at, found := r.thrownAt(t.Stack)
	if found {
		if m := handlerProp.FindStringSubmatch(t.Message); m != nil && out.Code == "shell-handler" {
			at = attribute(at, m[1])
		}
	}
	// The component stack, as related lines: each component where its
	// element was written; the page itself at its `export default`; a
	// component whose element a package made, without a position.
	for i, o := range t.Owners {
		related := report.Report{Severity: report.Message, Message: "in " + o.Name}
		if i == 0 && t.After {
			related.Message = "after " + o.Name
		}
		if written, ok := r.writtenAt(o); ok {
			related.File, related.Span, related.Line, related.Col = position(written.file, emit.Span{Pos: written.pos, End: written.pos})
		} else if o.Page && route != nil {
			related.File, related.Span, related.Line, related.Col = r.pageAt(route)
		}
		if !found && out.File == "" { // no frame of the project's — React's own exception, or a package's: the nearest element that has a place
			out.File, out.Span, out.Line, out.Col = related.File, related.Span, related.Line, related.Col
		}
		out.Related = append(out.Related, related)
	}
	switch {
	case found:
		out.File, out.Span, out.Line, out.Col = position(at.file, emit.Span{Pos: at.pos, End: at.pos})
	case out.File != "":
	case route != nil:
		out.File, out.Line, out.Col = route.File, 1, 1
	default:
		r.loading(&out, t.Stack)
	}
	return out
}

// loading places what a module that is not the project's threw while the
// bundle loaded — a package that reads the clock at its top level: no frame
// is the project's and there is no component. The report is at the
// project's import that leads to the module; where it threw is a related
// line.
func (r *run) loading(out *report.Report, stack string) {
	for _, f := range r.frames(stack) {
		if strings.HasPrefix(f.source, namespace+":") {
			continue
		}
		thrown := report.Report{Severity: report.Message, Message: "thrown here", File: f.source, Line: f.line + 1, Col: f.column + 1}
		if text, err := os.ReadFile(filepath.FromSlash(f.source)); err == nil { // columns count characters
			thrown.Line, thrown.Col = emit.LineCol(string(text), offsetAt(string(text), f.line, f.column))
		}
		out.Related = append(out.Related, thrown)
		if at, ok := r.importOf(f.source); ok {
			out.File, out.Span, out.Line, out.Col = position(at.file, emit.Span{Pos: at.pos, End: at.pos})
		} else {
			out.File, out.Line, out.Col = thrown.File, thrown.Line, thrown.Col
		}
		return
	}
}

// importOf is the import of the project's that leads to a module of the
// bundle, by the shortest way: the specifier, in the project file that wrote
// it.
func (r *run) importOf(name string) (site, bool) {
	seen := map[string]bool{name: true}
	for queue := []string{name}; len(queue) > 0; queue = queue[1:] {
		for _, e := range r.bundle.importers[queue[0]] {
			if file := r.project(e.importer); file != nil {
				for _, specifier := range file.Imports() {
					if specifier.Text() == e.specifier {
						return site{file, rtsx.TokenStart(file, specifier)}, true
					}
				}
			}
			if !seen[e.importer] {
				seen[e.importer] = true
				queue = append(queue, e.importer)
			}
		}
	}
	return site{}, false
}

// defaultExport finds a module's default export in its syntax — not in its
// text, where a comment or a template may hold the words: `export default`
// of a statement, or `X as default` of an export list.
func defaultExport(file *rtsx.SourceFile) (emit.Span, bool) {
	text := file.Text()
	for _, statement := range file.Statements.Nodes {
		start := rtsx.TokenStart(file, statement)
		if statement.Kind == rtsx.KindExportDeclaration {
			if clause := statement.AsExportDeclaration().ExportClause; clause != nil && clause.Kind == rtsx.KindNamedExports {
				for _, specifier := range clause.Elements() {
					if rtsx.NodeText(specifier.Name()) == "default" {
						return emit.Span{Pos: rtsx.TokenStart(file, specifier), End: specifier.End()}, true
					}
				}
			}
			continue
		}
		if !strings.HasPrefix(text[start:], "export") {
			continue
		}
		next := rtsx.SkipTrivia(text, start+len("export"))
		if rest, ok := strings.CutPrefix(text[next:], "default"); ok && !identifier.MatchString(rest) {
			return emit.Span{Pos: start, End: next + len("default")}, true
		}
	}
	return emit.Span{}, false
}

// identifier: the text goes on as a name (`export defaults`).
var identifier = regexp.MustCompile(`^[\p{L}\p{N}_$]`)
