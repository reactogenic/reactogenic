package conformance

import (
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// emit's feature bits are the fork's.
func TestFeatureBits(t *testing.T) {
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

// ide.md, *Span map*, over the corpus (RGP1-102): the span map of every
// output is one the compiler accepts; it answers positions as emit.Map does;
// and no source text has two copies answering the same feature, except the
// two copies of a shorthand.
func TestSpanMaps(t *testing.T) {
	cases := specCases(t)
	fixtures, err := LoadFixtures(fixturesRoot)
	if err != nil {
		t.Fatal(err)
	}
	checked, nodes, backwards := 0, 0, 0
	for _, c := range append(cases, fixtures...) {
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

		// Every node of the virtual tree maps to the same source span.
		file := rtsx.ParseTSX("/virtual.tsx", out.TSX)
		var visit func(n *rtsx.Node) bool
		visit = func(n *rtsx.Node) bool {
			if pos := rtsx.TokenStart(file, n); n.End() > pos {
				nodes++
				want := out.Map.Source(emit.Span{Pos: pos, End: n.End()})
				gotPos, gotEnd, _ := rtsx.SpanSource(m, pos, n.End())
				// A conditional slot's prop starts with a name copied from
				// after its origin (`{c ? <$Hint` → `$Hint={c ? …}`): a
				// span over the whole attribute runs backwards in the
				// source, and the two maps collapse it differently. TS
				// reports on the name or the value, never on that span.
				if gotPos == gotEnd && want.Pos != want.End {
					backwards++
				} else if gotPos != want.Pos || gotEnd != want.End {
					t.Errorf("%s: virtual [%d,%d) %q: span map says [%d,%d), emit.Map [%d,%d)", c.ID, pos, n.End(), out.TSX[pos:n.End()], gotPos, gotEnd, want.Pos, want.End)
				}
			}
			return n.ForEachChild(visit)
		}
		file.AsNode().ForEachChild(visit)

		// One answering copy per feature.
		shorthand := func(s emit.Span) bool {
			for _, sh := range out.Shorthands {
				if sh.Name.Pos <= s.Pos && s.End <= sh.Name.End {
					return true
				}
			}
			return false
		}
		segs := out.Map.Segments
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
	if checked < 40 || nodes < 2000 {
		t.Errorf("only %d outputs and %d nodes checked", checked, nodes)
	}
	if backwards > 6 {
		t.Errorf("%d spans run backwards in the source; expected only conditional slots", backwards)
	}
	t.Logf("%d outputs, %d virtual nodes, %d backwards", checked, nodes, backwards)
}
