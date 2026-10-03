package rtsx

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

// Mapper is a content mapper that runs in this process (specs/phase01/ide.md,
// *The engine*): once registered, files with Extension are modules of every
// program — the CLI's and the language server's, with or without a tsconfig —
// under their own names, checked through their virtual text.
type Mapper struct {
	Name, Version string
	Extension     string
	// Transform turns a file into virtual TSX and its span map. It must not
	// fail on user errors.
	Transform func(MapperRequest) MapperResult
	// Depends lists the paths whose existence the transform of a file
	// depends on; the answer is part of the file's cache key, and the probes
	// are what rebuild a program when one of them is created or deleted.
	Depends func(MapperRequest) []string
}

// MapperRequest is a file to transform, and the file system around it: the
// server's, with unsaved buffers.
type MapperRequest struct {
	FileName   string
	Content    string
	FileExists func(path string) bool
	ReadFile   func(path string) (string, bool)
}

// MapperResult is the virtual TSX of a file, its span-map tuples
// ([virtualStart, virtualLength, originalStart, originalLength, kind,
// features], byte offsets, covering Text), and anything that should travel
// with the parsed file (MappedFile).
type MapperResult struct {
	Text  string
	Spans [][6]int32
	Extra any
}

// RegisterMapper installs m for the process; call it before any program is
// built. nil removes it.
func RegisterMapper(m *Mapper) {
	if m == nil {
		contentmapper.RegisterBuiltIn(nil)
		return
	}
	request := func(r contentmapper.BuiltInRequest) MapperRequest {
		fs := r.FS
		if fs == nil {
			fs = osvfs.FS()
		}
		return MapperRequest{FileName: r.FileName, Content: r.Content, FileExists: fs.FileExists, ReadFile: fs.ReadFile}
	}
	contentmapper.RegisterBuiltIn(&contentmapper.BuiltIn{
		Name:       m.Name,
		Version:    m.Version,
		Extensions: []string{m.Extension},
		Transform: func(r contentmapper.BuiltInRequest) contentmapper.Result {
			result := m.Transform(request(r))
			return contentmapper.Result{
				Text:             result.Text,
				VirtualExtension: ".tsx",
				Mappings:         NewSpanMap(result.Spans),
				Extra:            result.Extra,
				Module:           true,
			}
		},
		FileIdentity: func(r contentmapper.BuiltInRequest) string {
			if m.Depends == nil {
				return ""
			}
			req := request(r)
			var bits strings.Builder
			for _, path := range m.Depends(req) {
				if req.FileExists(path) {
					bits.WriteByte('1')
				} else {
					bits.WriteByte('0')
				}
			}
			return bits.String()
		},
	})
}

// builtInProject is the mapper project a CLI compiler host needs, or nil
// when no mapper is registered.
func builtInProject(fs vfs.FS) contentmapper.Project {
	if mappers, _ := contentmapper.BuiltInMappers(); mappers == nil {
		return nil
	}
	return contentmapper.NewBuiltInProject(fs)
}

// MappedFile returns, for a content-mapped source file, its source text, its
// span map and what its transform attached; ok is false for any other file.
func MappedFile(file *SourceFile) (source string, spans *SpanMap, extra any, ok bool) {
	if file == nil || file.SpanMap() == nil {
		return "", nil, nil, false
	}
	return file.OriginalText(), file.SpanMap(), file.ContentMapperExtra(), true
}
