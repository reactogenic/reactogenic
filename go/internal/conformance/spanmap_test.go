package conformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// emit's feature bits are the fork's: all twenty, by name. The bridge exports
// a few of them as values; the order of them all is read from the fork's
// declaration, so a re-vendor that adds or reorders a feature fails here.
func TestFeatureBits(t *testing.T) {
	ours := []struct {
		name string
		bit  emit.Features
	}{
		{"FeatureHover", emit.FeatureHover},
		{"FeatureSignatureHelp", emit.FeatureSignatureHelp},
		{"FeatureCompletion", emit.FeatureCompletion},
		{"FeatureDefinition", emit.FeatureDefinition},
		{"FeatureTypeDefinition", emit.FeatureTypeDefinition},
		{"FeatureImplementation", emit.FeatureImplementation},
		{"FeatureReferences", emit.FeatureReferences},
		{"FeatureDocumentHighlights", emit.FeatureDocumentHighlights},
		{"FeatureRename", emit.FeatureRename},
		{"FeatureCallHierarchy", emit.FeatureCallHierarchy},
		{"FeatureCodeActions", emit.FeatureCodeActions},
		{"FeatureFormatting", emit.FeatureFormatting},
		{"FeatureInlayHints", emit.FeatureInlayHints},
		{"FeatureSemanticTokens", emit.FeatureSemanticTokens},
		{"FeatureFoldingRanges", emit.FeatureFoldingRanges},
		{"FeatureSelectionRanges", emit.FeatureSelectionRanges},
		{"FeatureLinkedEditing", emit.FeatureLinkedEditing},
		{"FeatureAutoInsert", emit.FeatureAutoInsert},
		{"FeatureDocumentSymbols", emit.FeatureDocumentSymbols},
		{"FeatureCodeLens", emit.FeatureCodeLens},
	}
	fork := forkFeatures(t)
	if len(fork) != len(ours) {
		t.Fatalf("the fork declares %d features, emit %d: %v", len(fork), len(ours), fork)
	}
	all := emit.Features(0)
	for i, f := range ours {
		if fork[i] != f.name || f.bit != 1<<i {
			t.Errorf("bit %d: the fork has %s, emit %s = %b", i, fork[i], f.name, f.bit)
		}
		all |= f.bit
	}
	if all != emit.AllFeatures {
		t.Errorf("AllFeatures is %b, the features together %b", emit.AllFeatures, all)
	}

	for _, c := range []struct {
		name       string
		ours, fork int32
	}{
		{"hover", int32(emit.FeatureHover), rtsx.SpanFeatureHover},
		{"completion", int32(emit.FeatureCompletion), rtsx.SpanFeatureCompletion},
		{"rename", int32(emit.FeatureRename), rtsx.SpanFeatureRename},
		{"formatting", int32(emit.FeatureFormatting), rtsx.SpanFeatureFormatting},
		{"code lens", int32(emit.FeatureCodeLens), rtsx.SpanFeatureCodeLens},
		{"all", int32(emit.AllFeatures), rtsx.SpanFeatureAll},
		{"verbatim", emit.KindVerbatim, rtsx.SpanKindVerbatim},
		{"atom", emit.KindAtom, rtsx.SpanKindAtom},
	} {
		if c.ours != c.fork {
			t.Errorf("%s: emit has %d, the fork %d", c.name, c.ours, c.fork)
		}
	}
}

// forkFeatures reads the names of the fork's span-map features, in bit
// order, from its declaration: `FeatureHover Feature = 1 << iota` and the
// names that follow it without a value.
func forkFeatures(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "../../third_party/tsgo/internal/spanmap/spanmap.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		block, ok := decl.(*ast.GenDecl)
		if !ok || block.Tok != token.CONST || len(block.Specs) == 0 {
			continue
		}
		first := block.Specs[0].(*ast.ValueSpec)
		if typ, ok := first.Type.(*ast.Ident); !ok || typ.Name != "Feature" || len(first.Values) != 1 {
			continue
		}
		if shift, ok := first.Values[0].(*ast.BinaryExpr); !ok || shift.Op != token.SHL {
			t.Fatalf("the fork's first feature is not `1 << iota`")
		}
		names := []string{first.Names[0].Name}
		for _, spec := range block.Specs[1:] {
			spec := spec.(*ast.ValueSpec)
			if len(spec.Values) > 0 {
				break // FeatureNone, FeatureAll
			}
			names = append(names, spec.Names[0].Name)
		}
		return names
	}
	t.Fatal("no `Feature` constants in the fork's spanmap.go")
	return nil
}

// ide.md, *Span map*: what each copy of a shorthand answers.
const (
	shorthandNameCopy  = emit.FeatureHover | emit.FeatureCompletion | emit.FeatureDefinition | emit.FeatureTypeDefinition | emit.FeatureReferences | emit.FeatureDocumentHighlights
	shorthandValueCopy = emit.FeatureHover | emit.FeatureDefinition | emit.FeatureReferences | emit.FeatureDocumentHighlights | emit.FeatureRename | emit.FeatureSemanticTokens | emit.FeatureInlayHints
)

// ide.md, *Span map*, over the corpus (RGP1-102): the span map of every
// output is one the compiler accepts; it answers positions as emit.Map does;
// no source text has two copies answering the same feature; and a shorthand
// has exactly its two copies — the name and the value — each with the
// features of the spec's table.
func TestSpanMaps(t *testing.T) {
	checked, nodes, backwards, shorthands := 0, 0, 0, 0
	for _, c := range corpus(t) {
		out, err := transpiler.Transpile(transpiler.Input{Files: c.Files, Entry: c.Entry, UntilPass: c.UntilPass})
		if err != nil || out.Map == nil || out.TSX == "" {
			continue
		}
		checked++
		src := c.Files[c.Entry]
		var tuples [][6]int32
		covered := 0
		for _, s := range out.Map.Spans() {
			if int(s[0]) != covered {
				t.Errorf("%s: gap in the span map at virtual offset %d", c.ID, covered)
			}
			covered = int(s[0] + s[1])
			tuples = append(tuples, s)
		}
		if covered != len(out.TSX) {
			t.Errorf("%s: the span map covers %d of %d bytes", c.ID, covered, len(out.TSX))
		}
		m := rtsx.NewSpanMap(tuples)
		if err := rtsx.ValidateSpanMap(m, out.TSX, src); err != nil {
			t.Errorf("%s: %v", c.ID, err)
			continue
		}

		// The prop of a slot first filled by a lowered `Match` / `Switch`
		// (`$Hint={c ? { … } : …}`) starts with a name copied from after
		// its origin, the flow element's opening tag: a span over the whole
		// attribute runs backwards in the source, and the two maps collapse
		// it differently. TS reports on the name or the value, never on that
		// span. Nothing else may collapse.
		var conditionalSlot func(n *rtsx.Node) bool
		conditionalSlot = func(n *rtsx.Node) bool {
			if n.Kind == rtsx.KindJsxAttributes { // the same span, when it is the only attribute
				return len(n.Properties()) == 1 && conditionalSlot(n.Properties()[0])
			}
			if n.Kind != rtsx.KindJsxAttribute || !strings.HasPrefix(rtsx.NodeText(n.Name()), "$") {
				return false
			}
			init := n.Initializer()
			return init != nil && init.Kind == rtsx.KindJsxExpression && init.Expression() != nil && init.Expression().Kind == rtsx.KindConditionalExpression
		}
		// Every node of the virtual tree maps to the same source span.
		file := rtsx.ParseTSX("/virtual.tsx", out.TSX)
		var visit func(n *rtsx.Node) bool
		visit = func(n *rtsx.Node) bool {
			if pos := rtsx.TokenStart(file, n); n.End() > pos {
				nodes++
				want := out.Map.Source(emit.Span{Pos: pos, End: n.End()})
				gotPos, gotEnd, _ := rtsx.SpanSource(m, pos, n.End())
				if gotPos == gotEnd && want.Pos != want.End && conditionalSlot(n) {
					backwards++
				} else if gotPos != want.Pos || gotEnd != want.End {
					t.Errorf("%s: virtual [%d,%d) %q: span map says [%d,%d), emit.Map [%d,%d)", c.ID, pos, n.End(), out.TSX[pos:n.End()], gotPos, gotEnd, want.Pos, want.End)
				}
			}
			return n.ForEachChild(visit)
		}
		file.AsNode().ForEachChild(visit)

		// A shorthand: two answering copies, by the table — or none, when
		// its element is not emitted at all (a repeated slot's first value).
		segs := out.Map.Segments
		shorthand := func(s emit.Span) bool {
			for _, sh := range out.Shorthands {
				if sh.Name.Pos <= s.Pos && s.End <= sh.Name.End {
					return true
				}
			}
			return false
		}
		for _, sh := range out.Shorthands {
			var answer []emit.Features
			copies := 0
			for _, s := range segs {
				if !s.Copied || s.In.End <= sh.Name.Pos || sh.Name.End <= s.In.Pos {
					continue
				}
				copies++
				if has := emit.AllFeatures &^ s.Without &^ emit.FeatureFormatting; has != 0 {
					answer = append(answer, has)
					if s.In != sh.Name {
						t.Errorf("%s: shorthand %q: a copy covers source [%d,%d)", c.ID, src[sh.Name.Pos:sh.Name.End], s.In.Pos, s.In.End)
					}
				}
			}
			if copies == 0 {
				continue
			}
			shorthands++
			slices.Sort(answer)
			want := []emit.Features{shorthandNameCopy, shorthandValueCopy}
			slices.Sort(want)
			if !slices.Equal(answer, want) {
				t.Errorf("%s: shorthand %q (%s): its copies answer %b, want the name copy %b and the value copy %b", c.ID, src[sh.Name.Pos:sh.Name.End], sh.Kind, answer, shorthandNameCopy, shorthandValueCopy)
			}
		}

		// One answering copy per feature everywhere else.
		for i, a := range segs {
			if !a.Copied {
				continue
			}
			for _, b := range segs[i+1:] {
				if !b.Copied || a.In.End <= b.In.Pos || b.In.End <= a.In.Pos {
					continue
				}
				overlap := emit.Span{Pos: max(a.In.Pos, b.In.Pos), End: min(a.In.End, b.In.End)}
				both := (emit.AllFeatures &^ a.Without) & (emit.AllFeatures &^ b.Without) &^ emit.FeatureFormatting
				if both != 0 && !shorthand(overlap) {
					t.Errorf("%s: source %q is copied twice and both copies answer features %b", c.ID, src[overlap.Pos:overlap.End], both)
				}
			}
		}
	}
	if checked < 40 || nodes < 2000 || shorthands < 10 {
		t.Errorf("only %d outputs, %d nodes and %d shorthands checked", checked, nodes, shorthands)
	}
	t.Logf("%d outputs, %d virtual nodes, %d shorthands, %d backwards (conditional slots)", checked, nodes, shorthands, backwards)
}
