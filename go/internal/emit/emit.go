// Package emit builds a pass's output by editing its input text, and keeps a
// map from every byte of output back to the input (RGP1-030b/c).
//
// Output is made of two kinds of text:
//
//   - copied: bytes taken unchanged from the input; mapped exactly;
//   - synthesized: new text written for a construct, its origin; mapped to
//     the origin's span (diagnostics.md, *Mapping*).
//
// Code the author wrote therefore keeps its exact bytes and positions, and
// everything else points at the construct that produced it.
package emit

import (
	"fmt"
	"slices"
	"strings"
)

// Span is a half-open byte range [Pos, End).
type Span struct{ Pos, End int }

func (s Span) Len() int { return s.End - s.Pos }

// Piece is a run of replacement text.
type Piece struct {
	Text   string
	Copied bool
	From   Span // copied: where Text comes from; synthesized: its origin
	// Without: the editor features this copy does not answer (ide.md, *Span
	// map*). Zero: all of them. Synthesized text answers none.
	Without Features
}

// Lacking returns the copy without the features f.
func (p Piece) Lacking(f Features) Piece {
	p.Without |= f
	return p
}

// Copy takes input[s] unchanged.
func Copy(input string, s Span) Piece {
	return Piece{Text: input[s.Pos:s.End], Copied: true, From: s}
}

// Synth writes new text on behalf of the construct at origin.
func Synth(text string, origin Span) Piece {
	return Piece{Text: text, From: origin}
}

// Edit replaces input[Span] with Pieces. An empty Span inserts; no Pieces
// deletes.
type Edit struct {
	Span   Span
	Pieces []Piece
}

// Segment maps a run of output back to the input.
type Segment struct {
	Out    Span
	Copied bool
	In     Span // copied: same length as Out; synthesized: the origin
	// Without: for a copied segment, the features it does not answer.
	Without Features
}

// Map maps output positions to input positions. Its segments cover the
// output without gaps, in order.
type Map struct {
	Segments []Segment
}

// Identity is the map of an unchanged text of length n.
func Identity(n int) *Map {
	if n == 0 {
		return &Map{}
	}
	return &Map{Segments: []Segment{{Out: Span{0, n}, Copied: true, In: Span{0, n}}}}
}

// Apply applies edits to input. Edits must not overlap; they may come in any
// order. Two insertions at one position keep their given order.
func Apply(input string, edits []Edit) (string, *Map, error) {
	edits = slices.Clone(edits)
	slices.SortStableFunc(edits, func(a, b Edit) int {
		if a.Span.Pos != b.Span.Pos {
			return a.Span.Pos - b.Span.Pos
		}
		return a.Span.End - b.Span.End // an insertion before a replacement at the same position
	})
	var (
		out strings.Builder
		m   = &Map{}
		pos = 0
	)
	copyInput := func(s Span) {
		if s.Len() > 0 {
			m.add(Segment{Out: Span{out.Len(), out.Len() + s.Len()}, Copied: true, In: s})
			out.WriteString(input[s.Pos:s.End])
		}
	}
	for _, e := range edits {
		if e.Span.Pos < pos || e.Span.End < e.Span.Pos || e.Span.End > len(input) {
			return "", nil, fmt.Errorf("emit: edit [%d,%d) overlaps or is out of range", e.Span.Pos, e.Span.End)
		}
		copyInput(Span{pos, e.Span.Pos})
		for _, p := range e.Pieces {
			if p.Copied && input[p.From.Pos:p.From.End] != p.Text {
				return "", nil, fmt.Errorf("emit: copied piece does not match input [%d,%d)", p.From.Pos, p.From.End)
			}
			if p.Text == "" {
				continue
			}
			seg := Segment{Out: Span{out.Len(), out.Len() + len(p.Text)}, Copied: p.Copied, In: p.From}
			if p.Copied {
				seg.Without = p.Without
			}
			m.add(seg)
			out.WriteString(p.Text)
		}
		pos = e.Span.End
	}
	copyInput(Span{pos, len(input)})
	return out.String(), m, nil
}

// add appends s, merging it into the previous segment when they continue
// each other.
func (m *Map) add(s Segment) {
	if n := len(m.Segments); n > 0 {
		last := &m.Segments[n-1]
		if last.Out.End == s.Out.Pos && last.Copied == s.Copied && last.Without == s.Without &&
			(s.Copied && last.In.End == s.In.Pos || !s.Copied && last.In == s.In) {
			last.Out.End = s.Out.End
			if s.Copied {
				last.In.End = s.In.End
			}
			return
		}
	}
	m.Segments = append(m.Segments, s)
}

// segmentAt returns the index of the segment holding output position pos;
// pos == output length returns the last segment.
func (m *Map) segmentAt(pos int) int {
	i, _ := slices.BinarySearchFunc(m.Segments, pos, func(s Segment, p int) int {
		switch {
		case s.Out.End <= p:
			return -1
		case s.Out.Pos > p:
			return 1
		}
		return 0
	})
	return min(i, len(m.Segments)-1)
}

// Source maps an output span to the input. A span inside copied text maps
// exactly; an end that falls in synthesized text widens to that origin.
func (m *Map) Source(out Span) Span {
	if len(m.Segments) == 0 {
		return Span{}
	}
	first := m.Segments[m.segmentAt(out.Pos)]
	start, end := first.In.Pos, first.In.End
	if first.Copied {
		start = first.In.Pos + min(out.Pos-first.Out.Pos, first.In.Len())
		end = start
	}
	if out.Len() > 0 {
		lastPos := out.End - 1
		last := m.Segments[m.segmentAt(lastPos)]
		end = last.In.End
		if last.Copied {
			end = last.In.Pos + lastPos - last.Out.Pos + 1
		}
	}
	return Span{min(start, end), max(start, end)}
}

// Output maps an input position to the output, through copied text only:
// the first copy of pos. Text the passes rewrote has no output position.
func (m *Map) Output(pos int) (int, bool) {
	for _, s := range m.Segments {
		if s.Copied && s.In.Pos <= pos && pos < s.In.End {
			return s.Out.Pos + pos - s.In.Pos, true
		}
	}
	return 0, false
}

// Then composes two passes: m maps this pass's output to its input, and
// prev maps that input to the original source. The result maps this pass's
// output straight to the source.
func (m *Map) Then(prev *Map) *Map {
	out := &Map{}
	for _, s := range m.Segments {
		if !s.Copied {
			out.add(Segment{Out: s.Out, In: prev.Source(s.In)})
			continue
		}
		// Copied text crosses prev's segments: split it along them.
		for pos := s.In.Pos; pos < s.In.End; {
			ps := prev.Segments[prev.segmentAt(pos)]
			end := min(s.In.End, ps.Out.End)
			o := Span{s.Out.Pos + pos - s.In.Pos, s.Out.Pos + end - s.In.Pos}
			if ps.Copied {
				off := ps.In.Pos - ps.Out.Pos
				out.add(Segment{Out: o, Copied: true, In: Span{pos + off, end + off}, Without: s.Without | ps.Without})
			} else {
				out.add(Segment{Out: o, In: ps.In})
			}
			pos = end
		}
	}
	return out
}
