package render

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// sourceMap is the render bundle's source map (version 3), read: the first
// hop of an exception's way back to the author's text (plan.md, RGP2-011).
// internal/emit writes the format; nothing in the repository read it.
type sourceMap struct {
	sources []string
	lines   [][]mapping // by 0-based line of the bundle, in column order
}

// mapping is one segment: from column of its bundle line on, the text comes
// from (line, column) of sources[source]. Lines and columns are 0-based;
// columns count UTF-16 code units, as the format has them.
type mapping struct {
	column               int
	source, line, origin int
}

func parseSourceMap(data []byte) (*sourceMap, error) {
	var raw struct {
		Version  int      `json:"version"`
		Sources  []string `json:"sources"`
		Mappings string   `json:"mappings"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Version != 3 {
		return nil, fmt.Errorf("source map version %d", raw.Version)
	}
	m := &sourceMap{sources: raw.Sources}
	var source, line, origin int // relative to the previous segment, across lines
	for text := range strings.SplitSeq(raw.Mappings, ";") {
		var segments []mapping
		column := 0 // relative within the line
		for segment := range strings.SplitSeq(text, ",") {
			if segment == "" {
				continue
			}
			fields, err := decodeVLQ(segment)
			if err != nil {
				return nil, err
			}
			column += fields[0]
			if len(fields) < 4 {
				continue // generated text with no source
			}
			source, line, origin = source+fields[1], line+fields[2], origin+fields[3]
			if source < 0 || source >= len(raw.Sources) {
				return nil, fmt.Errorf("source map: source %d of %d", source, len(raw.Sources))
			}
			segments = append(segments, mapping{column, source, line, origin})
		}
		m.lines = append(m.lines, segments)
	}
	return m, nil
}

const base64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// decodeVLQ decodes the base64 variable-length quantities of one segment.
func decodeVLQ(segment string) ([]int, error) {
	var fields []int
	value, shift := 0, 0
	for i := 0; i < len(segment); i++ {
		digit := strings.IndexByte(base64, segment[i])
		if digit < 0 {
			return nil, fmt.Errorf("source map: %q is not a VLQ", segment)
		}
		value |= (digit & 31) << shift
		if digit&32 != 0 {
			shift += 5
			continue
		}
		if value&1 != 0 {
			value = -(value >> 1)
		} else {
			value >>= 1
		}
		fields = append(fields, value)
		value, shift = 0, 0
	}
	if shift != 0 || len(fields) == 0 {
		return nil, fmt.Errorf("source map: %q is not a VLQ", segment)
	}
	return fields, nil
}

// lookup returns where the text at a position of the bundle — 1-based line
// and column, as an engine's stack has them — comes from: the source, and a
// 0-based line and UTF-16 column in it. The segment is the last one that
// starts at or before the column.
func (m *sourceMap) lookup(line, column int) (source string, srcLine, srcColumn int, ok bool) {
	if line < 1 || line > len(m.lines) {
		return "", 0, 0, false
	}
	segments := m.lines[line-1]
	i, found := slices.BinarySearchFunc(segments, column-1, func(s mapping, c int) int { return s.column - c })
	if !found {
		i--
	}
	if i < 0 {
		return "", 0, 0, false
	}
	s := segments[i]
	return m.sources[s.source], s.line, s.origin, true
}

// offsetAt is the byte offset in text of a 0-based line and a column in
// UTF-16 code units, clamped to the line.
func offsetAt(text string, line, column int) int {
	pos := 0
	for ; line > 0; line-- {
		next := strings.IndexByte(text[pos:], '\n')
		if next < 0 {
			return len(text)
		}
		pos += next + 1
	}
	for column > 0 && pos < len(text) && text[pos] != '\n' {
		r, size := utf8.DecodeRuneInString(text[pos:])
		pos += size
		column--
		if r >= 0x10000 {
			column--
		}
	}
	return pos
}
