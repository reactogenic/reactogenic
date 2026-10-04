package render

import (
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
	for line := range strings.SplitSeq(stack, "\n") {
		m := frameAt.FindStringSubmatch(strings.TrimRight(line, " \r"))
		if m == nil {
			continue
		}
		bundleLine, _ := strconv.Atoi(m[1])
		bundleColumn, _ := strconv.Atoi(m[2])
		source, srcLine, srcColumn, ok := r.bundle.sites.lookup(bundleLine, bundleColumn)
		if !ok {
			continue
		}
		if file := r.project(source); file != nil {
			return site{file, offsetAt(file.Text(), srcLine, srcColumn)}, true
		}
	}
	return site{}, false
}

// writtenAt is where the element of a component of an exception's stack was
// written.
func (r *run) writtenAt(o owner) (site, bool) {
	if o.FileName == "" || o.LineNumber < 1 {
		return site{}, false
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
// it (shell-handler, shell-nondeterministic, page-no-default); anything else
// is shell-error, with the exception's message.
func (r *run) exception(route *Route, t *thrown) report.Report {
	out := report.Report{Code: "shell-error", Message: t.Message}
	switch {
	case strings.HasPrefix(t.Name, "shell-"), strings.HasPrefix(t.Name, "page-"):
		out.Code = t.Name
	case t.Name != "" && t.Name != "Error":
		out.Message = t.Name + ": " + t.Message
	}
	at, found := r.thrownAt(t.Stack)
	if found {
		if m := handlerProp.FindStringSubmatch(t.Message); m != nil && out.Code == "shell-handler" {
			at = attribute(at, m[1])
		}
	}
	// The component stack, as related lines: each component where its
	// element was written; the page itself at its `export default`.
	for i, o := range t.Owners {
		related := report.Report{Severity: report.Message, Message: "in " + o.Name}
		if i == 0 && t.After {
			related.Message = "after " + o.Name
		}
		if written, ok := r.writtenAt(o); ok {
			related.File, related.Span, related.Line, related.Col = position(written.file, emit.Span{Pos: written.pos, End: written.pos})
		} else if route != nil {
			related.File, related.Span, related.Line, related.Col = r.pageAt(route)
		}
		if !found && out.File == "" { // no frame of the project's — React's own exception, or a package's: the nearest element
			out.File, out.Span, out.Line, out.Col = related.File, related.Span, related.Line, related.Col
		}
		out.Related = append(out.Related, related)
	}
	switch {
	case found:
		out.File, out.Span, out.Line, out.Col = position(at.file, emit.Span{Pos: at.pos, End: at.pos})
	case out.File == "" && route != nil:
		out.File, out.Line, out.Col = route.File, 1, 1
	}
	return out
}
