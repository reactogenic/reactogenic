package server

import (
	"os"
	"runtime"
	"sync"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// VirtualParams: a batch of .rtsx files, each as in `transform`.
type VirtualParams struct {
	Files []TransformParams `json:"files"`
}

// VirtualResult has one entry per file of the request, in its order.
type VirtualResult struct {
	Files []VirtualFile `json:"files"`
}

// VirtualFile is an .rtsx file as TypeScript sees it: the text of the
// language server's content mapper (ide.md, *The engine*) — the tolerant
// transform, which never fails — and its span map.
//
// Spans are the tuples of emit.Map.Spans — [virtualStart, virtualLength,
// originalStart, originalLength, kind, features] — with every offset in
// UTF-16 code units, as a JavaScript string is indexed: the caller maps
// positions both ways without the bytes.
type VirtualFile struct {
	File  string           `json:"file"`
	Text  string           `json:"text"`
	Spans []emit.SpanTuple `json:"spans"`
	// Stopped: Text is not the file's lowered TSX (ide.md, *Tolerance*, 4),
	// so what TypeScript reports inside it means nothing. Its declarations
	// are still there.
	Stopped bool `json:"stopped,omitempty"`
}

// moduleMarker makes a file a module, at its end: an .rtsx file is always
// one. In our own hosts the mapper says so; for another host only the text
// can, as in the stock content mapper.
const moduleMarker = "\nexport {};\n"

// virtualAll transforms the batch on every processor: the files are
// independent, and the caller waits for the whole answer.
func virtualAll(p VirtualParams) *VirtualResult {
	result := &VirtualResult{Files: make([]VirtualFile, len(p.Files))}
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(len(p.Files), runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				result.Files[i] = virtual(p.Files[i])
			}
		}()
	}
	for i := range p.Files {
		next <- i
	}
	close(next)
	wg.Wait()
	return result
}

// virtual never fails: the mapper's transform ends, at worst, with the
// source as its own virtual text, and so does a panic here.
func virtual(p TransformParams) (result VirtualFile) {
	defer func() {
		if r := recover(); r != nil {
			result = VirtualFile{File: p.File, Text: p.Code, Spans: utf16Spans(emit.Identity(len(p.Code)).Spans(), p.Code, p.Code), Stopped: true}
		}
	}()
	text, file := mapper.Transform(p.File, p.Code, func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	})
	m := file.Map
	if m == nil {
		m = emit.Identity(len(text))
	}
	spans := m.Spans()
	if rtsx.ParseTSX(p.File+".tsx", text).ExternalModuleIndicator == nil {
		spans = append(spans, emit.SpanTuple{int32(len(text)), int32(len(moduleMarker)), int32(len(p.Code)), 0, emit.KindAtom, 0})
		text += moduleMarker
	}
	return VirtualFile{File: p.File, Text: text, Spans: utf16Spans(spans, text, p.Code), Stopped: file.Stopped}
}

// utf16Spans rewrites the byte offsets of spans — into the virtual text and
// into the source — as UTF-16 offsets.
func utf16Spans(spans []emit.SpanTuple, virtual, source string) []emit.SpanTuple {
	v, s := utf16Offsets(virtual), utf16Offsets(source)
	if v == nil && s == nil {
		return spans
	}
	for i, t := range spans {
		vStart, sStart := v.at(t[0]), s.at(t[2])
		spans[i] = emit.SpanTuple{vStart, v.at(t[0]+t[1]) - vStart, sStart, s.at(t[2]+t[3]) - sStart, t[4], t[5]}
	}
	return spans
}

// utf16Index[i] is the UTF-16 offset of byte offset i; nil for ASCII text,
// where the two are equal. An offset inside a character is its start.
type utf16Index []int32

func utf16Offsets(text string) utf16Index {
	ascii := true
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return nil
	}
	index := make(utf16Index, len(text)+1)
	units := int32(0)
	for i, r := range text { // i: the first byte of each character
		index[i] = units
		for j := i + 1; j < len(text) && text[j]&0xC0 == 0x80; j++ {
			index[j] = units
		}
		units++
		if r > 0xFFFF {
			units++ // a surrogate pair
		}
	}
	index[len(text)] = units
	return index
}

func (x utf16Index) at(offset int32) int32 {
	if x == nil {
		return offset
	}
	return x[max(0, min(int(offset), len(x)-1))]
}
