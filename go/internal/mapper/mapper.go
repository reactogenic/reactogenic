// Package mapper is the .rtsx transform as a content mapper
// (specs/phase01/ide.md, *The engine*): registered with the fork, it makes
// every .rtsx file a module of the program under its own name, checked
// through its emitted TSX, with a span map back to the source.
package mapper

import (
	"path"
	"regexp"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// File is what travels with a mapped file in the program: the transpiler's
// output — its map, notes, generated names — for the reporting layer.
type File struct {
	transpiler.Output
	// Stopped: the transform did not complete, so the virtual text is not
	// the file's lowered TSX; TS's diagnostics for it mean nothing.
	Stopped bool
	// Err is a failure of the transpiler itself.
	Err error
}

// Of returns what the transform attached to a mapped source file.
func Of(file *rtsx.SourceFile) (*File, bool) {
	_, _, extra, ok := rtsx.MappedFile(file)
	if !ok {
		return nil, false
	}
	f, ok := extra.(*File)
	return f, ok
}

// Register installs the transform for the process. version is part of every
// cache key: a new binary never reuses an old result.
func Register(version string) {
	rtsx.RegisterMapper(&rtsx.Mapper{Name: "reactogenic", Version: version, Extension: ".rtsx", Transform: transform, Depends: depends})
}

func transform(req rtsx.MapperRequest) rtsx.MapperResult {
	out, err := transpiler.Transpile(transpiler.Input{
		Files:    map[string]string{req.FileName: req.Content},
		Entry:    req.FileName,
		ReadFile: req.ReadFile,
	})
	if err != nil || out.Map == nil {
		// Nothing usable: the source as virtual text, mapped 1:1.
		var spans [][6]int32
		if len(req.Content) > 0 {
			spans = [][6]int32{{0, int32(len(req.Content)), 0, int32(len(req.Content)), 0, int32(identityFeatures)}}
		}
		return rtsx.MapperResult{Text: req.Content, Spans: spans, Extra: &File{Output: out, Stopped: true, Err: err}}
	}
	spans := make([][6]int32, 0, len(out.Map.Segments))
	for _, s := range out.Map.Spans() {
		spans = append(spans, s)
	}
	return rtsx.MapperResult{Text: out.TSX, Spans: spans, Extra: &File{Output: out}}
}

// identityFeatures: what the source answers when it stands in as its own
// virtual text — everything but formatting.
const identityFeatures = emit.AllFeatures &^ emit.FeatureFormatting

var segmentRoot = regexp.MustCompile(`#([A-Za-z_$][\w$-]*)`)

// depends lists the sibling files whose existence decides a file's
// transform: the candidates of each `#name` (syntax.md, *Segment files*) —
// found by a scan that may list too many, never too few — and the `.tsx`
// that would make the module ambiguous.
func depends(req rtsx.MapperRequest) []string {
	dir := path.Dir(req.FileName)
	paths := []string{req.FileName[:len(req.FileName)-len(path.Ext(req.FileName))] + ".tsx"}
	seen := map[string]bool{}
	for _, m := range segmentRoot.FindAllStringSubmatch(req.Content, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		for _, ext := range transpiler.SegmentExtensions {
			paths = append(paths, path.Join(dir, m[1]+ext))
		}
	}
	return paths
}
