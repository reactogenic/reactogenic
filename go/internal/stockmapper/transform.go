package stockmapper

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// transformResult is the host's TransformResult: the virtual TSX, its span
// map, the transpiler's errors, and the virtual ranges whose TypeScript
// diagnostics are dropped.
type transformResult struct {
	Text      string           `json:"text"`
	Extension string           `json:"extension"`
	Mappings  []emit.SpanTuple `json:"mappings"`
	// Diagnostics become syntax errors of the file: while one stands, `tsc`
	// prints no type error of the program.
	Diagnostics          []diagnostic `json:"diagnostics,omitempty"`
	DiagnosticDirectives *directives  `json:"diagnosticDirectives,omitempty"`
}

// diagnostic is an error at source offsets (bytes).
type diagnostic struct {
	MessageText string `json:"messageText"`
	Start       int    `json:"start"`
	Length      int    `json:"length"`
	Code        int32  `json:"code"`
}

type directives struct {
	UnusedExpectDirectiveDiagnostics []struct{} `json:"unusedExpectDirectiveDiagnostics"`
	// [originalStart, originalLength, virtualStart, virtualEnd, policy]
	Directives [][5]int `json:"directives"`
}

const policyIgnore = 0

// transform is the built-in mapper's transform (go/internal/mapper) plus
// what the stock host cannot do for a mapped file:
//
//   - imports get their `.rtsx` (resolve.go): after the passes, an edit to
//     the emitted text whose map is composed with theirs, as the passes
//     compose one another. The inserted text is synthesized on the specifier:
//     an atom without features, so the specifier itself still answers from
//     its copied text;
//   - a file without imports or exports gets `export {}`: an .rtsx file is
//     always a module (in our hosts the mapper says so; here only the text
//     can);
//   - TS5097 on the import a segment root generates for a `.tsx` / `.ts`
//     file is dropped (syntax.md, *Segment files*), by an ignore directive
//     over that specifier and nothing else;
//   - a stopped file (ide.md, *Tolerance*) drops every TypeScript
//     diagnostic of its virtual text, by a directive over all of it.
//
// It never fails: a panic leaves the source as its own virtual text, with
// an `internal` error — a mapper that dies is not started again.
func (s *server) transform(p transformParams, proj *project) (result transformResult) {
	defer func() {
		if r := recover(); r != nil {
			s.logf("panic transforming %s: %v\n%s", p.FileName, r, debug.Stack())
			result = sourceAsVirtual(p.Content, fmt.Sprintf("transpiler: %v", r))
		}
	}()
	if s.opts.beforeTransform != nil {
		s.opts.beforeTransform(p.FileName)
	}
	text, file := mapper.Transform(p.FileName, p.Content, s.opts.ReadFile)
	toSource := file.Map
	if toSource == nil {
		toSource = emit.Identity(len(text))
	}

	// What TypeScript will parse, but for the edits below.
	virtual, emitted := rtsx.ParseTSX(p.FileName+".tsx", text), toSource
	var (
		edits    []emit.Edit
		inserted []insertion
		ignored  [][2]int // virtual ranges, before the edits
	)
	for _, specifier := range virtual.Imports() {
		span := emit.Span{Pos: rtsx.TokenStart(virtual, specifier), End: specifier.End()}
		name, ok := quoted(text[span.Pos:span.End])
		if !ok {
			continue // unterminated, or written with escapes
		}
		if suffix := s.explicitSuffix(p.FileName, name, proj); suffix != "" {
			at := span.End - 1 // before the closing quote
			edits = append(edits, emit.Edit{Span: emit.Span{Pos: at, End: at}, Pieces: []emit.Piece{emit.Synth(suffix, span)}})
			inserted = append(inserted, insertion{at, len(suffix)})
			continue
		}
		if (strings.HasSuffix(name, ".tsx") || strings.HasSuffix(name, ".ts")) && isSegmentImport(file, toSource.Source(span)) {
			ignored = append(ignored, [2]int{span.Pos, span.End})
		}
	}
	if virtual.ExternalModuleIndicator == nil {
		end := emit.Span{Pos: len(text), End: len(text)}
		edits = append(edits, emit.Edit{Span: end, Pieces: []emit.Piece{emit.Synth("\nexport {};\n", end)}})
	}
	if len(edits) > 0 {
		next, m, err := emit.Apply(text, edits)
		if err != nil {
			panic(err) // edits are insertions at distinct positions
		}
		text, toSource = next, m.Then(toSource)
	}

	result = transformResult{Text: text, Extension: ".tsx", Mappings: toSource.Spans()}
	switch {
	case file.Stopped && len(text) > 0:
		// The virtual text still holds unlowered constructs: what
		// TypeScript says about it means nothing.
		result.DiagnosticDirectives = &directives{Directives: [][5]int{{0, 0, 0, len(text), policyIgnore}}}
	case len(ignored) > 0:
		result.DiagnosticDirectives = &directives{}
		for _, r := range ignored {
			shift := shiftAt(inserted, r[0])
			result.DiagnosticDirectives.Directives = append(result.DiagnosticDirectives.Directives, [5]int{0, 0, r[0] + shift, r[1] + shift, policyIgnore})
		}
	}
	if result.DiagnosticDirectives != nil {
		result.DiagnosticDirectives.UnusedExpectDirectiveDiagnostics = []struct{}{}
	}

	if file.Err != nil {
		result.Diagnostics = append(result.Diagnostics, internalError(p.Content, file.Err.Error()))
	}
	for _, d := range file.Diagnostics {
		if d.Severity != transpiler.Error {
			continue // the contract has no warnings
		}
		code, named := numericCode(d.Code)
		if !named && reportedByTypeScript(virtual, emitted, code, d.Span.Pos) {
			continue // TypeScript finds the same syntax error in the virtual text
		}
		message := d.Message
		if named {
			message = d.Code + ": " + d.Message
		}
		start := min(max(d.Span.Pos, 0), len(p.Content))
		end := min(max(d.Span.End, start), len(p.Content))
		result.Diagnostics = append(result.Diagnostics, diagnostic{MessageText: message, Start: start, Length: end - start, Code: code})
	}
	// The passes report in their own order; the reader wants the file's.
	sort.SliceStable(result.Diagnostics, func(i, j int) bool { return result.Diagnostics[i].Start < result.Diagnostics[j].Start })
	return result
}

type insertion struct{ at, length int }

// shiftAt: how far the insertions before pos moved it.
func shiftAt(inserted []insertion, pos int) int {
	shift := 0
	for _, i := range inserted {
		if i.at <= pos {
			shift += i.length
		}
	}
	return shift
}

// quoted returns the text of a string literal written without escapes.
func quoted(literal string) (string, bool) {
	if len(literal) < 2 || literal[0] != literal[len(literal)-1] || !strings.ContainsRune("\"'`", rune(literal[0])) {
		return "", false
	}
	text := literal[1 : len(literal)-1]
	if strings.ContainsAny(text, "\\\n"+literal[:1]) {
		return "", false
	}
	return text, true
}

// isSegmentImport: the source span is a segment root's — its import is the
// transpiler's, not the author's.
func isSegmentImport(file *mapper.File, source emit.Span) bool {
	for _, note := range file.Notes {
		if note.Kind == "segment" && note.Span == source {
			return true
		}
	}
	return false
}

// reportedByTypeScript: the virtual text has a syntax error with this code
// that maps to this source offset. The host shows it, at the same place; the
// transpiler's copy would be a duplicate. A syntax error the passes lowered
// away — the virtual text parses there — stays the mapper's to report.
func reportedByTypeScript(virtual *rtsx.SourceFile, toSource *emit.Map, code int32, sourcePos int) bool {
	text := virtual.Text()
	for _, d := range virtual.Diagnostics() {
		if d.Code() != code {
			continue
		}
		pos := min(rtsx.SkipTrivia(text, d.Pos()), len(text))
		if toSource.Source(emit.Span{Pos: pos, End: max(d.End(), pos)}).Pos == sourcePos {
			return true
		}
	}
	return false
}

// sourceAsVirtual is the last resort: the source as its own virtual text,
// mapped 1:1, every TypeScript diagnostic of it dropped, one `internal`
// error on the first line.
func sourceAsVirtual(content, why string) transformResult {
	result := transformResult{Text: content, Extension: ".tsx", Mappings: emit.Identity(len(content)).Spans(), Diagnostics: []diagnostic{internalError(content, why)}}
	if len(content) > 0 {
		result.DiagnosticDirectives = &directives{UnusedExpectDirectiveDiagnostics: []struct{}{}, Directives: [][5]int{{0, 0, 0, len(content), policyIgnore}}}
	}
	return result
}

func internalError(content, why string) diagnostic {
	firstLine := len(content)
	if i := strings.IndexAny(content, "\r\n"); i >= 0 {
		firstLine = i
	}
	code, _ := numericCode("internal")
	return diagnostic{MessageText: "internal: " + why, Start: 0, Length: firstLine, Code: code}
}
