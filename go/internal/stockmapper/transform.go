package stockmapper

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

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
//   - imports and module augmentations get their `.rtsx` (resolve.go):
//     after the passes, an edit to the emitted text whose map is composed
//     with theirs, as the passes compose one another. The inserted text is
//     synthesized on the specifier: an atom without features, so the
//     specifier itself still answers from its copied text. An alias that
//     cannot carry the extension is replaced by a relative path, the whole
//     of it an atom on the specifier;
//   - a file without imports or exports gets `export {}`: an .rtsx file is
//     always a module (in our hosts the mapper says so; here only the text
//     can);
//   - the import a segment root generates for a `.tsx` / `.ts` file
//     (syntax.md, *Segment files*) loses its extension, and TS5097 with it.
//     Where a sibling would win the extensionless import — `intro.ts` next
//     to the segment `intro.tsx` — the extension stays and an ignore
//     directive over that specifier drops TS5097, with whatever else
//     TypeScript reports on that specifier: a directive has no codes;
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
	text, file := mapper.Transform(p.FileName, p.Content, s.opts.FileExists)
	toSource := file.Map
	if toSource == nil {
		toSource = emit.Identity(len(text))
	}

	// What TypeScript will parse, but for the edits below.
	virtualName := p.FileName + ".tsx"
	virtual := rtsx.ParseTSX(virtualName, text)
	var (
		edits   []emit.Edit
		moved   []shift
		ignored [][2]int // virtual ranges, before the edits
	)
	replace := func(span emit.Span, with string, origin emit.Span) {
		edits = append(edits, emit.Edit{Span: span, Pieces: []emit.Piece{emit.Synth(with, origin)}})
		moved = append(moved, shift{span.End, len(with) - span.Len()})
	}
	specifiers, isModule := virtual.Imports(), virtual.ExternalModuleIndicator != nil
	switch {
	case isModule:
		specifiers = append(specifiers[:len(specifiers):len(specifiers)], virtual.ModuleAugmentations...)
	case len(virtual.AmbientModuleNames) > 0:
		// `declare module "./button"` augments once the file is a module:
		// parsed as TypeScript will, with the `export {}` of below. It is
		// appended, so the positions are this text's.
		specifiers = append(specifiers[:len(specifiers):len(specifiers)], rtsx.ParseTSX(virtualName, text+moduleMarker).ModuleAugmentations...)
	}
	for _, specifier := range specifiers {
		span := emit.Span{Pos: rtsx.TokenStart(virtual, specifier), End: specifier.End()}
		if span.Pos < 0 || span.End > len(text) || span.Len() < 2 {
			continue
		}
		name, ok := quoted(text[span.Pos:span.End])
		if !ok {
			continue // `global`; unterminated, or written with escapes
		}
		inner := emit.Span{Pos: span.Pos + 1, End: span.End - 1} // between the quotes
		suffix, whole := s.explicitImport(p.FileName, name, proj)
		switch {
		case suffix != "":
			replace(emit.Span{Pos: inner.End, End: inner.End}, suffix, span)
		case whole != "":
			replace(inner, whole, span)
		case (strings.HasSuffix(name, ".tsx") || strings.HasSuffix(name, ".ts")) && isSegmentImport(file, toSource.Source(span)):
			if bare, same := s.extensionless(p.FileName, name); same {
				replace(inner, bare, span)
			} else {
				ignored = append(ignored, [2]int{span.Pos, span.End})
			}
		}
	}
	if !isModule {
		end := emit.Span{Pos: len(text), End: len(text)}
		edits = append(edits, emit.Edit{Span: end, Pieces: []emit.Piece{emit.Synth(moduleMarker, end)}})
	}
	if len(edits) > 0 {
		next, m, err := emit.Apply(text, edits)
		if err != nil {
			panic(err) // edits are at distinct specifiers and at the end
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
			by := shiftAt(moved, r[0])
			result.DiagnosticDirectives.Directives = append(result.DiagnosticDirectives.Directives, [5]int{0, 0, r[0] + by, r[1] + by, policyIgnore})
		}
	}
	if result.DiagnosticDirectives != nil {
		result.DiagnosticDirectives.UnusedExpectDirectiveDiagnostics = []struct{}{}
	}

	if file.Err != nil {
		result.Diagnostics = append(result.Diagnostics, internalError(p.Content, file.Err.Error()))
	}
	// Whether TypeScript finds syntax errors in the virtual text — the text
	// it will parse, edits included: `export {}` after a file cut off at
	// `export` makes one statement of the two, which parses.
	typeScriptReports := sync.OnceValue(func() bool {
		final := virtual
		if len(edits) > 0 {
			final = rtsx.ParseTSX(virtualName, text)
		}
		return len(final.Diagnostics()) > 0
	})
	for _, d := range file.Diagnostics {
		if d.Severity != transpiler.Error {
			continue // the contract has no warnings
		}
		code, named := numericCode(d.Code)
		if !named {
			// A syntax error of the source parse. While the virtual text has
			// syntax errors of its own, TypeScript reports the mistake
			// there — at another place and often under another code, so no
			// comparison tells "the same" — and the source parse's are not
			// sent: one mistake, one report. They are the mapper's to send
			// when the virtual text parses: the passes lowered the broken
			// code away (the children of a segment root).
			if typeScriptReports() {
				continue
			}
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

// moduleMarker makes a file a module, at its end.
const moduleMarker = "\nexport {};\n"

// shift: an edit that ends at from made the text after it longer by by.
type shift struct{ from, by int }

// shiftAt: how far the edits before pos moved it.
func shiftAt(moved []shift, pos int) int {
	total := 0
	for _, m := range moved {
		if m.from <= pos {
			total += m.by
		}
	}
	return total
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
