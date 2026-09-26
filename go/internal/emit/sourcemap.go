package emit

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// SourceMapV3 encodes m as a source map (version 3) for Vite and browsers.
// Columns count UTF-16 code units, as the format requires. Copied text is
// mapped at every line start and segment start; synthesized text maps to
// its origin's start.
func (m *Map) SourceMapV3(file, source, sourceText, output string) ([]byte, error) {
	srcLines := newLineIndex(sourceText)
	var (
		b                 strings.Builder
		line, col         = 0, 0 // output position, UTF-16 columns
		prevCol           = 0
		prevSrc, prevSCol = 0, 0
		prevSLine         = 0
		first             = true
	)
	emit := func(srcPos int) {
		sLine, sCol := srcLines.lineCol(srcPos)
		if !first {
			b.WriteByte(',')
		}
		first = false
		writeVLQ(&b, col-prevCol)
		writeVLQ(&b, 0-prevSrc) // one source
		writeVLQ(&b, sLine-prevSLine)
		writeVLQ(&b, sCol-prevSCol)
		prevCol, prevSrc, prevSLine, prevSCol = col, 0, sLine, sCol
	}
	for _, s := range m.Segments {
		atStart := true
		for i := s.Out.Pos; i < s.Out.End; {
			if atStart {
				if s.Copied {
					emit(s.In.Pos + i - s.Out.Pos)
				} else {
					emit(s.In.Pos)
				}
				atStart = false
			}
			r, size := utf8.DecodeRuneInString(output[i:])
			i += size
			if r == '\n' {
				b.WriteByte(';')
				line, col, prevCol, first = line+1, 0, 0, true
				atStart = true
				continue
			}
			col += utf16Len(r)
		}
	}
	return json.Marshal(struct {
		Version        int      `json:"version"`
		File           string   `json:"file"`
		Sources        []string `json:"sources"`
		SourcesContent []string `json:"sourcesContent"`
		Names          []string `json:"names"`
		Mappings       string   `json:"mappings"`
	}{3, file, []string{source}, []string{sourceText}, []string{}, b.String()})
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

const base64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func writeVLQ(b *strings.Builder, v int) {
	u := v << 1
	if v < 0 {
		u = -v<<1 | 1
	}
	for {
		digit := u & 31
		u >>= 5
		if u > 0 {
			digit |= 32
		}
		b.WriteByte(base64[digit])
		if u == 0 {
			return
		}
	}
}

// lineIndex turns byte offsets into 0-based lines and UTF-16 columns.
type lineIndex struct {
	text   string
	starts []int
}

func newLineIndex(text string) lineIndex {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return lineIndex{text, starts}
}

func (x lineIndex) lineCol(pos int) (int, int) {
	line := 0
	for line+1 < len(x.starts) && x.starts[line+1] <= pos {
		line++
	}
	col := 0
	for _, r := range x.text[x.starts[line]:min(pos, len(x.text))] {
		col += utf16Len(r)
	}
	return line, col
}

// LineCol returns the 1-based line and column of byte offset pos, counting
// columns in characters.
func LineCol(text string, pos int) (int, int) {
	line, col := 1, 1
	for i, r := range text {
		if i >= pos {
			break
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}
