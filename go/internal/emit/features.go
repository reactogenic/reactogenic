package emit

// Features is a set of editor features a copied piece may answer. The bits
// are those of the fork's span map (spanmap.Feature); a test in the bridge
// keeps them equal.
type Features int32

const (
	FeatureHover Features = 1 << iota
	FeatureSignatureHelp
	FeatureCompletion
	FeatureDefinition
	FeatureTypeDefinition
	FeatureImplementation
	FeatureReferences
	FeatureDocumentHighlights
	FeatureRename
	FeatureCallHierarchy
	FeatureCodeActions
	FeatureFormatting
	FeatureInlayHints
	FeatureSemanticTokens
	FeatureFoldingRanges
	FeatureSelectionRanges
	FeatureLinkedEditing
	FeatureAutoInsert
	FeatureDocumentSymbols
	FeatureCodeLens

	AllFeatures Features = FeatureCodeLens<<1 - 1
)

// Span-map segment kinds (spanmap.Kind).
const (
	KindVerbatim = 0
	KindAtom     = 1
)

// SpanTuple is one span-map segment as the content-mapper contract writes
// it: [virtualStart, virtualLength, originalStart, originalLength, kind,
// features].
type SpanTuple [6]int32

// Spans converts the map to span-map segments (ide.md, *Span map*): a copied
// segment is verbatim and answers every feature but formatting and those it
// lacks; a synthesized one is an atom over its origin and answers none, so
// only diagnostics land there. Segments cover the output: no gaps.
func (m *Map) Spans() []SpanTuple {
	spans := make([]SpanTuple, 0, len(m.Segments))
	for _, s := range m.Segments {
		kind, features := int32(KindAtom), Features(0)
		if s.Copied {
			kind, features = KindVerbatim, AllFeatures&^FeatureFormatting&^s.Without
		}
		spans = append(spans, SpanTuple{int32(s.Out.Pos), int32(s.Out.Len()), int32(s.In.Pos), int32(s.In.Len()), kind, int32(features)})
	}
	return spans
}
