package server

import (
	"context"
	"fmt"
	"math"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/json"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// Syntactic answers, for the text of an .rtsx file, the LSP features that
// need only its tree (specs/phase01/ide.md, *Span map*: syntactic features
// run on the source tree). Results are LSP JSON, positions in encoding
// ("utf-16" or "utf-8").
type Syntactic struct{ s *ls.Syntactic }

// documentName is the file name of every syntactic parse. Nothing reads it,
// and the parser wants a normalized absolute path — which a document's URI
// (`untitled:new.rtsx`, `file:/single/slash.rtsx`) is not.
const documentName = "/document.rtsx"

// NewSyntactic parses text as .rtsx.
func NewSyntactic(text, encoding string) *Syntactic {
	return &Syntactic{s: ls.NewSyntactic(rtsx.ParseRTSX(documentName, text), lsproto.PositionEncodingKind(encoding))}
}

// FoldingRanges is the result of textDocument/foldingRange.
func (s *Syntactic) FoldingRanges(lineFoldingOnly bool) ([]byte, error) {
	ranges := s.s.FoldingRanges(context.Background(), lineFoldingOnly)
	if ranges == nil {
		ranges = []*lsproto.FoldingRange{}
	}
	return json.Marshal(ranges)
}

// SelectionRanges is the result of textDocument/selectionRange for the
// request's `positions` (JSON).
func (s *Syntactic) SelectionRanges(positions []byte) ([]byte, error) {
	var at []lsproto.Position
	if err := json.Unmarshal(positions, &at); err != nil {
		return nil, err
	}
	if err := inRange(at...); err != nil {
		return nil, err
	}
	return json.Marshal(s.s.SelectionRanges(at))
}

// DocumentSymbols is the result of textDocument/documentSymbol: a tree for a
// client that supports one (hierarchical), else flat, located in uri.
func (s *Syntactic) DocumentSymbols(uri string, hierarchical bool) ([]byte, error) {
	if hierarchical {
		return json.Marshal(s.s.DocumentSymbols(context.Background()))
	}
	return json.Marshal(s.s.SymbolInformations(context.Background(), lsproto.DocumentUri(uri)))
}

// ClosingTag is the closing tag to insert after a `>` just typed at the
// request's `position` (JSON), or "".
func (s *Syntactic) ClosingTag(position []byte) (string, error) {
	var at lsproto.Position
	if err := json.Unmarshal(position, &at); err != nil {
		return "", err
	}
	if err := inRange(at); err != nil {
		return "", err
	}
	return s.s.ClosingTag(at), nil
}

// File is the parsed text: the .rtsx source tree.
func (s *Syntactic) File() *ast.SourceFile { return s.s.File() }

// Offset is the byte offset in the text of an LSP `position` (JSON).
func (s *Syntactic) Offset(position []byte) (int, error) {
	var at lsproto.Position
	if err := json.Unmarshal(position, &at); err != nil {
		return 0, err
	}
	if err := inRange(at); err != nil {
		return 0, err
	}
	return s.s.Offset(at), nil
}

// Range is the LSP range of the span pos..end of the text.
func (s *Syntactic) Range(pos, end int) Range { return s.s.Range(pos, end) }

// Range is an LSP range; it marshals as one.
type Range = lsproto.Range

// inRange refuses a position the server's converters cannot take: they hold
// lines and characters as int32, and one beyond that would index the line
// map with a negative number. (A negative one is refused before: it does
// not decode.)
func inRange(positions ...lsproto.Position) error {
	for _, p := range positions {
		if p.Line > math.MaxInt32 || p.Character > math.MaxInt32 {
			return fmt.Errorf("position %d:%d is out of range", p.Line, p.Character)
		}
	}
	return nil
}

// document is a text that is in no program, for the server's converters.
type document string

func (d document) FileName() string         { return documentName }
func (d document) OriginalFileName() string { return documentName }
func (d document) Text() string             { return string(d) }
func (d document) OriginalText() string     { return string(d) }
func (document) SpanMap() *spanmap.SpanMap  { return nil }

// ApplyChange applies one ranged content change of textDocument/didChange —
// `range` as JSON, and the new text — exactly as the server applies it to
// its own copy of the document: the same line map (a lone `\r` breaks a
// line), the same clamping of a position past the end of its line. A host
// that keeps the text of a document stays equal to the server this way. A
// range the server would reject (a negative position) is an error.
func ApplyChange(text, encoding string, changeRange []byte, newText string) (string, error) {
	var r lsproto.Range
	if err := json.Unmarshal(changeRange, &r); err != nil {
		return text, err
	}
	if err := inRange(r.Start, r.End); err != nil {
		return text, err
	}
	lineMap := lsconv.ComputeLSPLineStarts(text)
	converters := lsconv.NewConverters(lsproto.PositionEncodingKind(encoding), func(string) *lsconv.LSPLineMap { return lineMap })
	spans := converters.FromLSPRange(document(text), r, spanmap.FeatureAll)
	if len(spans) != 1 {
		return text, fmt.Errorf("a range with %d spans", len(spans))
	}
	return core.TextChange{TextRange: spans[0].Span, NewText: newText}.ApplyTo(text), nil
}
