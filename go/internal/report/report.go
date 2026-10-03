// Package report is the reporting layer that `reactogenic check` and the
// language server share (specs/phase01/ide.md, *Diagnostics*): given the
// program and a file, the diagnostics of that file as its author reads them
// — the transpiler's, TS7's mapped back to the source and reworded in slot
// terms (specs/phase01/diagnostics.md), and the rules that join files.
package report

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

type Severity int

const (
	Error Severity = iota
	Warning
	Suggestion
	Message
)

func (s Severity) String() string {
	return [...]string{"error", "warning", "suggestion", "message"}[s]
}

// Report is one diagnostic, positioned in a file the author wrote.
type Report struct {
	File string // absolute; "" for a diagnostic without a file
	// Span is where, in the text of File — for an .rtsx file, its source.
	Span      emit.Span
	Line, Col int // of Span.Pos, 1-based; 0 for a diagnostic without a file
	Severity  Severity
	Code      string // "TS2322", or a name: a transpiler code ("orphan-slot"), a rewrite ("missing-slot")
	Message   string
	Related   []Report
	// TS is the diagnostic the report was made from, for a host that needs
	// more of it (its tags, its number for TS's quick fixes); nil for the
	// transpiler's reports and the cross-file rules'.
	TS *rtsx.Diagnostic

	at        emit.Span // the span in the source that the notes are looked up by (emit.Map.Source)
	supersede emit.Span // slot-key-inline: TS errors in this span follow from it
	copy      int       // 1 + the index of the copied segment of the map that holds the diagnostic; 0: not in one
	secondary bool      // that copy answers no editor feature: the text is checked in another copy too
}

// File returns the reports of one file of the program, in source order: what
// the editor shows for it, suggestions included.
func File(ctx context.Context, p *rtsx.Program, file *rtsx.SourceFile) []Report {
	syntactic, semantic, suggestion := rtsx.FileDiagnostics(ctx, p, file)
	var ts []*rtsx.Diagnostic
	if _, mapped := mapper.Of(file); !mapped {
		// Of a mapped file, the syntax errors are the source parse's — the
		// transpiler's diagnostics — never those of the virtual text
		// (ide.md, *Tolerance*, rule 2).
		ts = syntactic
	}
	reports := fileReports(ctx, p, file, slices.Concat(ts, semantic, suggestion))
	sortReports(reports)
	return reports
}

// Program returns the reports of the whole program, by file and position:
// what `reactogenic check` prints. config are the diagnostics of reading the
// tsconfig; p may be nil when it could not be read.
func Program(p *rtsx.Program, config []*rtsx.Diagnostic) []Report {
	all := slices.Clone(config)
	if p != nil {
		all = append(all, rtsx.AllDiagnostics(p)...)
	}
	var reports []Report
	var files []*rtsx.SourceFile // those with diagnostics, then the rest of the program
	byFile := map[*rtsx.SourceFile][]*rtsx.Diagnostic{}
	for _, d := range all {
		file := d.File()
		if file == nil {
			reports = append(reports, fromTS(d))
			continue
		}
		if _, ok := byFile[file]; !ok {
			files = append(files, file)
		}
		byFile[file] = append(byFile[file], d)
	}
	if p != nil {
		for _, file := range p.GetSourceFiles() {
			if _, ok := byFile[file]; !ok {
				files = append(files, file)
			}
		}
	}
	for _, file := range files {
		reports = append(reports, fileReports(context.Background(), p, file, byFile[file])...)
	}
	sortReports(reports)
	return reports
}

// sortReports orders by file and position; reports at one position keep
// their order: the transpiler's, TS's, the cross-file rules'.
func sortReports(reports []Report) {
	slices.SortStableFunc(reports, func(a, b Report) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
}

// fileReports turns ts — TS's diagnostics of file — into the reports of
// file. A file that is not mapped reports them as they are.
func fileReports(ctx context.Context, p *rtsx.Program, file *rtsx.SourceFile, ts []*rtsx.Diagnostic) []Report {
	f, mapped := mapper.Of(file)
	if !mapped {
		reports := make([]Report, 0, len(ts))
		for _, d := range ts {
			reports = append(reports, fromTS(d))
		}
		return reports
	}
	name := file.FileName()
	source, _, _, _ := rtsx.MappedFile(file)

	// The transpiler's own, with their names as codes. They travel beside
	// TS's diagnostics, not through the mapper's result: there, any one of
	// them would hide every type error of the program.
	var reports []Report
	internal := false
	for _, d := range f.Diagnostics {
		severity := Error
		if d.Severity == transpiler.Warning {
			severity = Warning
		}
		internal = internal || d.Code == "internal"
		reports = append(reports, Report{File: name, Span: d.Span, Line: d.Line, Col: d.Col, Severity: severity, Code: d.Code, Message: d.Message})
	}
	if f.Err != nil && !internal {
		reports = append(reports, Report{File: name, Line: 1, Col: 1, Code: "internal", Message: f.Err.Error()})
	}

	// A stopped file's virtual text is not its lowered TSX — the source
	// itself, or text with code left out: what TS says about it would be
	// false (ide.md, *Tolerance*, rule 4). The syntax errors and the
	// transpiler's diagnostics above remain.
	if !f.Stopped {
		var own []Report
		for _, d := range ts {
			if d.File() == file && !segmentImport(f, d) {
				own = append(own, fromTS(d))
			}
		}
		reports = append(reports, once(superseded(own))...)
		if p != nil {
			reports = append(reports, slotConditionals(ctx, p, file, f, source)...)
		}
	}
	if p != nil {
		reports = append(reports, segmentSelf(p, file, f, source)...)
	}
	return reports
}

// fromTS maps a TS diagnostic back to what the author wrote: in a mapped
// file, from the virtual text to the source through the file's span map,
// reworded in slot terms where a rule matches (diagnostics.md, *Mapping* and
// *Rewrites*); in any other file, as it is. Related information that points
// into a mapped file is mapped too.
func fromTS(d *rtsx.Diagnostic) Report {
	r := Report{Severity: severity(d), Code: fmt.Sprintf("TS%d", d.Code()), Message: flatten(d), TS: d}
	file := d.File()
	if file == nil {
		return r
	}
	virtual := virtualSpan(d)
	r.File, r.Span, r.Line, r.Col = position(file, virtual)
	if f, ok := mapper.Of(file); ok && f.Map != nil {
		source, _, _, _ := rtsx.MappedFile(file)
		// The notes are looked up by the transpiler's own map: on a
		// zero-length span in synthesized text it gives the whole origin,
		// where the span map gives an empty range at its start.
		r.at = f.Map.Source(virtual)
		r.copy, r.secondary = copyOf(f.Map, virtual)
		if code, message, related, ok := rewrite(d, f.Output, source, r.at); ok {
			r.Code, r.Message, r.Related = code, message, related
			if code == "slot-key-inline" {
				r.supersede = innermostNote(f.Notes, r.at).Tag
			}
		}
		r.Message = renameGenerated(r.Message, f.Generated)
	}
	for _, rel := range d.RelatedInformation() {
		rr := Report{Severity: Message, Message: flatten(rel)}
		if rel.File() != nil {
			rr.File, rr.Span, rr.Line, rr.Col = position(rel.File(), virtualSpan(rel))
		}
		r.Related = append(r.Related, rr)
	}
	return r
}

func severity(d *rtsx.Diagnostic) Severity {
	switch d.Category().Name() {
	case "error":
		return Error
	case "warning":
		return Warning
	case "suggestion":
		return Suggestion
	}
	return Message
}

// virtualSpan is where TS reported d, in the text TS checked: from the first
// token, as tsc prints it.
func virtualSpan(d *rtsx.Diagnostic) emit.Span {
	pos := rtsx.SkipTrivia(d.File().Text(), d.Pos())
	return emit.Span{Pos: pos, End: max(pos, d.End())}
}

// position returns where a span of file's text — the virtual text of a
// mapped file — is in what the author wrote.
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

// segmentImport: TS5097 (an import path ending in `.tsx` or `.ts`) on the
// import a segment root emits. The import names the file found, extension
// included (syntax.md, *Segment files*); it is not the author's, and needs
// no allowImportingTsExtensions.
func segmentImport(f *mapper.File, d *rtsx.Diagnostic) bool {
	if d.Code() != 5097 || f.Map == nil {
		return false
	}
	note := innermostNote(f.Notes, f.Map.Source(virtualSpan(d)))
	return note != nil && note.Kind == "segment"
}

// superseded drops the TS errors that follow from another one: a key
// function passed by reference is read as an entry key, and everything TS
// then says about that slot element restates the misreading.
func superseded(reports []Report) []Report {
	var kept []Report
	for _, r := range reports {
		drop := false
		for _, s := range reports {
			if s.supersede.Len() > 0 && s.Code != r.Code && s.supersede.Pos <= r.at.Pos && r.at.End <= s.supersede.End {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, r)
		}
	}
	return kept
}

// copyOf says which copied segment of m holds the virtual span — 1 + its
// index; 0 when the span is not inside one — and whether that copy is a
// secondary one: it answers no editor feature, as every copy of a text after
// the first (ide.md, *Span map*, *Several copies of one token*).
func copyOf(m *emit.Map, virtual emit.Span) (int, bool) {
	for i, s := range m.Segments {
		if s.Out.Pos <= virtual.Pos && virtual.End <= s.Out.End {
			if !s.Copied {
				return 0, false
			}
			return i + 1, s.Without == emit.AllFeatures
		}
		if s.Out.Pos > virtual.Pos {
			break
		}
	}
	return 0, false
}

// once reports each mistake once (ide.md, *Diagnostics*). Code copied to
// several places of the virtual text is checked in each:
//
//   - reports with the same range, code and message are one;
//   - a report in a secondary copy is dropped when another copy of the same
//     source text has one with the same code at the same range — the copies
//     are branches that narrow differently, so the messages may differ.
//
// Nothing else is merged: two reports on synthesized text share a range —
// their construct's — without being the same mistake.
func once(reports []Report) []Report {
	var kept []Report
next:
	for i, r := range reports {
		for j, q := range reports {
			if i == j || q.Span != r.Span || q.Code != r.Code {
				continue
			}
			if j < i && q.Message == r.Message {
				continue next
			}
			// Of several secondary copies with nothing in the primary one,
			// the first stays.
			if r.secondary && q.copy != 0 && q.copy != r.copy && (!q.secondary || j < i) {
				continue next
			}
		}
		kept = append(kept, r)
	}
	return kept
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
