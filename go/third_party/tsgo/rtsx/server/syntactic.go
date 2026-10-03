package server

import (
	"context"

	"github.com/microsoft/TypeScript/tsc/internal/json"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// Syntactic answers, for the text of an .rtsx file, the LSP features that
// need only its tree (specs/phase01/ide.md, *Span map*: syntactic features
// run on the source tree). Results are LSP JSON, positions in encoding
// ("utf-16" or "utf-8").
type Syntactic struct{ s *ls.Syntactic }

// NewSyntactic parses text as .rtsx.
func NewSyntactic(fileName, text, encoding string) *Syntactic {
	return &Syntactic{s: ls.NewSyntactic(rtsx.ParseRTSX(fileName, text), lsproto.PositionEncodingKind(encoding))}
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
	return json.Marshal(s.s.SelectionRanges(at))
}

// ClosingTag is the closing tag to insert after a `>` just typed at the
// position (line and character, zero-based), or "".
func (s *Syntactic) ClosingTag(line, character int) string {
	return s.s.ClosingTag(lsproto.Position{Line: uint32(line), Character: uint32(character)})
}
