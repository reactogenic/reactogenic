// Reactogenic: syntactic features on a source file that is in no program —
// the .rtsx source tree. Not part of upstream: added by
// go/patches/0006-rtsx-lsp.patch.

package ls

import (
	"context"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

// Syntactic answers the features that need only a tree. A content-mapped
// file's virtual text cannot answer them for constructs that are lowered
// away (a slot element has no element there); its source tree can.
type Syntactic struct {
	l    *LanguageService
	file *ast.SourceFile
}

// NewSyntactic serves file, with positions in encoding.
func NewSyntactic(file *ast.SourceFile, encoding lsproto.PositionEncodingKind) *Syntactic {
	lineMap := lsconv.ComputeLSPLineStarts(file.Text())
	converters := lsconv.NewConverters(encoding, func(string) *lsconv.LSPLineMap { return lineMap })
	return &Syntactic{l: &LanguageService{converters: converters}, file: file}
}

// FoldingRanges are TypeScript's folding ranges of the file.
func (s *Syntactic) FoldingRanges(ctx context.Context, lineFoldingOnly bool) []*lsproto.FoldingRange {
	ranges := s.l.addNodeOutliningSpans(ctx, s.file)
	ranges = append(ranges, s.l.addRegionOutliningSpans(ctx, s.file)...)
	if lineFoldingOnly {
		ranges = s.l.adjustFoldingEnd(ranges, s.file)
	}
	return ranges
}

// SelectionRanges are TypeScript's smart selection ranges at positions.
func (s *Syntactic) SelectionRanges(positions []lsproto.Position) []*lsproto.SelectionRange {
	results := make([]*lsproto.SelectionRange, 0, len(positions))
	for _, position := range positions {
		if r := getSmartSelectionRange(s.l, s.file, s.offset(position)); r != nil {
			results = append(results, r)
		}
	}
	return results
}

// DocumentSymbols are TypeScript's document symbols of the file, as a tree.
// On the source tree a declaration's range is its own, whatever constructs it
// holds, and there is no generated declaration (the keys of the objects a
// mapper builds) to list.
func (s *Syntactic) DocumentSymbols(ctx context.Context) []*lsproto.DocumentSymbol {
	symbols := s.l.getDocumentSymbolsForChildren(ctx, s.file.AsNode(), s.file)
	if symbols == nil {
		symbols = []*lsproto.DocumentSymbol{}
	}
	return symbols
}

// SymbolInformations are DocumentSymbols for a client that takes no tree.
func (s *Syntactic) SymbolInformations(ctx context.Context, uri lsproto.DocumentUri) []lsproto.SymbolInformation {
	infos := flattenDocumentSymbols(s.DocumentSymbols(ctx), uri)
	if infos == nil {
		infos = []lsproto.SymbolInformation{}
	}
	return infos
}

// ClosingTag is the closing tag to insert after a `>` just typed at
// position: `</name>` for an opening tag that has none, as TypeScript
// decides it (ProvideOnAutoInsert).
func (s *Syntactic) ClosingTag(position lsproto.Position) string {
	token := astnav.FindPrecedingToken(s.file, s.offset(position))
	if token == nil {
		return ""
	}
	var element *ast.Node
	if token.Kind == ast.KindGreaterThanToken && ast.IsJsxOpeningElement(token.Parent) {
		element = token.Parent.Parent
	} else if ast.IsJsxText(token) && ast.IsJsxElement(token.Parent) {
		element = token.Parent
	}
	if element != nil && isUnclosedTag(element.AsJsxElement()) {
		return "</" + ast.EntityNameToString(element.AsJsxElement().OpeningElement.TagName(), scanner.GetTextOfNode) + ">"
	}
	var fragment *ast.Node
	if token.Kind == ast.KindGreaterThanToken && ast.IsJsxOpeningFragment(token.Parent) {
		fragment = token.Parent.Parent
	} else if ast.IsJsxText(token) && ast.IsJsxFragment(token.Parent) {
		fragment = token.Parent
	}
	if fragment != nil && isUnclosedFragment(fragment.AsJsxFragment()) {
		return "</>"
	}
	return ""
}

func (s *Syntactic) offset(position lsproto.Position) int {
	positions := s.l.converters.FromLSPPosition(s.file, position, spanmap.FeatureAll)
	if len(positions) == 0 {
		return 0
	}
	return int(positions[0].Position)
}
