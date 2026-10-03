// Package mapper is the .rtsx transform as a content mapper
// (specs/phase01/ide.md, *The engine*): registered with the fork, it makes
// every .rtsx file a module of the program under its own name, checked
// through its emitted TSX, with a span map back to the source.
package mapper

import (
	"fmt"
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
	// Stopped: the virtual text is not the file's lowered TSX — the passes
	// ended early (Output.Stopped names the pass), or code was left out of
	// it (Output.Dropped) — so TS's diagnostics for it mean nothing.
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

// Register installs the transform for the process, tolerant: the editor's
// (ide.md, *Tolerance*). version is part of every cache key: a new binary
// never reuses an old result.
func Register(version string) {
	register(version, true)
}

// RegisterStrict installs the transform as the builds run it — `reactogenic
// check`: a file with a syntax error reports its syntax errors and is not
// lowered. Its source stands in as its virtual text, so that the modules
// importing it still find what it exports, and the file is Stopped.
func RegisterStrict(version string) {
	register(version, false)
}

func register(version string, tolerant bool) {
	mapped := func(req rtsx.MapperRequest) rtsx.MapperResult {
		result := transform(req, tolerant)
		result.StatementStarts = statementStarts(result.Text, result.Spans)
		return result
	}
	rtsx.RegisterMapper(&rtsx.Mapper{Name: "reactogenic", Version: version, Extension: ".rtsx", Transform: mapped, Depends: depends})
}

// statementStarts are the line starts of the generated text that precedes
// the whole source — the imports the transform adds to a file that has none
// (a segment's, the core's), each on a line of its own. An import that
// TypeScript inserts there, before or after them, goes to the start of the
// source (ide.md, *In a file whose only import is generated*); any other
// position there is inside a generated import, and no edit goes to it.
func statementStarts(text string, spans [][6]int32) []int32 {
	for _, s := range spans {
		if s[4] != emit.KindVerbatim {
			continue
		}
		var starts []int32
		for at := int32(0); s[2] == 0 && at < s[0]; at++ { // generated text, then the source from its first byte
			if at == 0 || text[at-1] == '\n' {
				starts = append(starts, at)
			}
		}
		return starts
	}
	return nil
}

// transform never fails (ide.md, *Tolerance*): a file being typed gets the
// passes on its recovered tree; one that stops a pass — a panic in it
// included — keeps the last good text; a failure of the transpiler itself
// leaves the source as its own virtual text, mapped 1:1. The recover here is
// the last resort, for a panic outside the passes.
//
// Strict (tolerant false), the passes do not run on a source with a syntax
// error: that file too is its own virtual text.
func transform(req rtsx.MapperRequest, tolerant bool) (result rtsx.MapperResult) {
	identity := func(out transpiler.Output, err error) rtsx.MapperResult {
		var spans [][6]int32
		if len(req.Content) > 0 {
			spans = [][6]int32{{0, int32(len(req.Content)), 0, int32(len(req.Content)), 0, int32(identityFeatures)}}
		}
		return rtsx.MapperResult{Text: req.Content, Spans: spans, Extra: &File{Output: out, Stopped: true, Err: err}}
	}
	defer func() {
		if r := recover(); r != nil {
			result = identity(transpiler.Output{}, fmt.Errorf("transpiler: %v", r))
		}
	}()
	out, err := transpiler.Transpile(transpiler.Input{
		Files: map[string]string{req.FileName: req.Content},
		Entry: req.FileName,
		// Of its siblings the transform learns only which exist (ide.md,
		// *Segments*) — what `depends` puts in the cache key. Their contents
		// are not in the key, so they must not reach the result.
		ReadFile: func(p string) (string, bool) { return "", req.FileExists(p) },
		Tolerant: tolerant,
	})
	if err != nil || out.Map == nil {
		return identity(out, err)
	}
	spans := make([][6]int32, 0, len(out.Map.Segments))
	for _, s := range out.Map.Spans() {
		spans = append(spans, s)
	}
	return rtsx.MapperResult{Text: out.TSX, Spans: spans, Extra: &File{Output: out, Stopped: out.Stopped != "" || out.Dropped}}
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
