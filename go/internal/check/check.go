// Package check is `reactogenic check` (specs/phase01/diagnostics.md): the
// transpiler's errors and TS7's, all reported on the files the author wrote.
package check

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/project"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// Report is one diagnostic, positioned in a file the author wrote.
type Report struct {
	File      string // absolute
	Line, Col int    // 1-based; 0 for a diagnostic without a file
	Error     bool   // an error, not a warning
	Code      string // "TS2322", or a transpiler code such as "orphan-slot"
	Message   string
	Related   []Report

	span      emit.Span // in File, for TS diagnostics of an .rtsx file
	supersede emit.Span // slot-key-inline: TS errors in this span follow from it
}

// Run type-checks the project of the tsconfig at configPath (absolute).
func Run(configPath string) []Report {
	p := project.Open(configPath)
	var reports []Report
	// Transpiler errors, once per file (RGP1-075).
	for file, out := range p.Outputs() {
		for _, d := range out.Diagnostics {
			reports = append(reports, Report{File: file, Line: d.Line, Col: d.Col, Error: d.Severity == transpiler.Error, Code: d.Code, Message: d.Message})
		}
	}
	var ts []Report
	for _, d := range p.Diagnostics() {
		ts = append(ts, fromTS(p, d))
	}
	reports = append(reports, superseded(ts)...)
	reports = append(reports, slotConditionals(p)...)
	for _, file := range p.Ambiguous() {
		tsx := strings.TrimSuffix(file, ".rtsx") + ".tsx"
		reports = append(reports, Report{File: file, Line: 1, Col: 1, Error: true, Code: "ambiguous-module",
			Message: fmt.Sprintf("`%s` and `%s` side by side: an import of `./%s` is ambiguous, and the `.rtsx` is not checked",
				filepath.Base(tsx), filepath.Base(file), strings.TrimSuffix(filepath.Base(tsx), ".tsx"))})
	}
	sort.SliceStable(reports, func(i, j int) bool {
		a, b := reports[i], reports[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return reports
}

// fromTS maps a TS diagnostic back: from a virtual Foo.tsx to Foo.rtsx
// through the transpiler's map (diagnostics.md, *Mapping*), or as it is.
func fromTS(p *project.Project, d *rtsx.Diagnostic) Report {
	r := Report{Error: rtsx.IsError(d), Code: fmt.Sprintf("TS%d", d.Code()), Message: flatten(d)}
	if d.File() == nil {
		return r
	}
	span := emit.Span{Pos: rtsx.SkipTrivia(d.File().Text(), d.Pos()), End: d.End()}
	r.File, r.Line, r.Col = position(p, d.File(), span)
	if src, ok := p.SourceOf(d.File().FileName()); ok {
		out, _ := p.Output(src)
		text, _ := p.Source(src)
		r.span = out.Map.Source(span)
		if code, message, related, ok := rewrite(d, out, text, r.span); ok {
			r.Code, r.Message, r.Related = code, message, related
			if code == "slot-key-inline" {
				r.supersede = innermostNote(out.Notes, r.span).Tag
			}
		}
		r.Message = renameGenerated(r.Message, out.Generated)
	}
	for _, rel := range d.RelatedInformation() {
		rr := Report{Message: flatten(rel)}
		if rel.File() != nil {
			rr.File, rr.Line, rr.Col = position(p, rel.File(), emit.Span{Pos: rtsx.SkipTrivia(rel.File().Text(), rel.Pos()), End: rel.End()})
		}
		r.Related = append(r.Related, rr)
	}
	return r
}

// superseded drops the TS errors that follow from another one: a key
// function passed by reference is read as an entry key, and everything TS
// then says about that slot element restates the misreading.
func superseded(reports []Report) []Report {
	var kept []Report
	for _, r := range reports {
		drop := false
		for _, s := range reports {
			if s.supersede.Len() > 0 && s.File == r.File && s.Code != r.Code &&
				s.supersede.Pos <= r.span.Pos && r.span.End <= s.supersede.End {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, r)
		}
	}
	return kept
}

// position returns where span of file is in the author's source.
func position(p *project.Project, file *rtsx.SourceFile, span emit.Span) (string, int, int) {
	name := file.FileName()
	if src, ok := p.SourceOf(name); ok {
		out, _ := p.Output(src)
		text, _ := p.Source(src)
		if out.Map != nil {
			span = out.Map.Source(span)
		}
		line, col := emit.LineCol(text, span.Pos)
		return src, line, col
	}
	line, col := emit.LineCol(file.Text(), span.Pos)
	return name, line, col
}

// flatten renders a message and its chain, indented as tsc does.
func flatten(d *rtsx.Diagnostic) string {
	var b strings.Builder
	var walk func(d *rtsx.Diagnostic, depth int)
	walk = func(d *rtsx.Diagnostic, depth int) {
		if depth > 0 {
			b.WriteString("\n" + strings.Repeat("  ", depth))
		}
		b.WriteString(rtsx.Message(d))
		for _, next := range d.MessageChain() {
			walk(next, depth+1)
		}
	}
	walk(d, 0)
	return b.String()
}

// Errors counts the reports that are errors.
func Errors(reports []Report) int {
	n := 0
	for _, r := range reports {
		if r.Error {
			n++
		}
	}
	return n
}

// Print writes reports in tsc's format, paths relative to cwd. pretty adds a
// code frame of the source line: `file:line:col - error CODE: message`;
// otherwise `file(line,col): error CODE: message`.
func Print(w io.Writer, reports []Report, cwd string, pretty bool, readFile func(string) (string, bool)) {
	for _, r := range reports {
		kind := "warning"
		if r.Error {
			kind = "error"
		}
		name := rel(cwd, r.File)
		switch {
		case r.File == "":
			fmt.Fprintf(w, "%s %s: %s\n", kind, r.Code, r.Message)
		case pretty:
			fmt.Fprintf(w, "%s:%d:%d - %s %s: %s\n", name, r.Line, r.Col, kind, r.Code, r.Message)
			frame(w, readFile, r.File, r.Line, r.Col)
		default:
			fmt.Fprintf(w, "%s(%d,%d): %s %s: %s\n", name, r.Line, r.Col, kind, r.Code, r.Message)
		}
		for _, rr := range r.Related {
			if rr.File != "" {
				fmt.Fprintf(w, "  %s:%d:%d - %s\n", rel(cwd, rr.File), rr.Line, rr.Col, rr.Message)
			} else {
				fmt.Fprintf(w, "  %s\n", rr.Message)
			}
		}
	}
	if pretty && len(reports) > 0 {
		fmt.Fprintf(w, "\nFound %d error(s).\n", Errors(reports))
	}
}

func rel(cwd, file string) string {
	if file == "" {
		return ""
	}
	if r, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return file
}

// frame prints the source line of a report with a caret under its column.
func frame(w io.Writer, readFile func(string) (string, bool), file string, line, col int) {
	text, ok := readFile(file)
	if !ok {
		return
	}
	lines := strings.Split(text, "\n")
	if line < 1 || line > len(lines) {
		return
	}
	src := strings.TrimRight(lines[line-1], "\r")
	gutter := fmt.Sprintf("%d", line)
	fmt.Fprintf(w, "\n%s %s\n%s %s^\n\n", gutter, src, strings.Repeat(" ", len(gutter)), strings.Repeat(" ", max(col-1, 0)))
}
