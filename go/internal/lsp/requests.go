package lsp

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

// The server's own methods, answered inside the fork's server from the
// program a document is in (lsp.Embedder.Requests).
const (
	// methodTranspiled is public (ide.md, *Commands*).
	methodTranspiled = "reactogenic/transpiled"
	// methodSiblings is the front's: the file names next to a document, as
	// the server's file system has them — unsaved buffers included (ide.md,
	// *Segments*).
	methodSiblings = "reactogenic/siblings"
)

var requests = map[string]func(ctx context.Context, program *rtsx.Program, file *rtsx.SourceFile, params []byte) (any, error){
	methodTranspiled: transpiled,
	methodSiblings:   siblings,
}

// transpiledResult is the answer to reactogenic/transpiled.
type transpiledResult struct {
	Text string `json:"text"`
	// Step is the tolerance step that produced the text (ide.md,
	// *Tolerance*): "lowered"; "stopped: <pass>" — the text is the last good
	// pass's; "source" — the source stands in for it.
	Step string `json:"step"`
}

// transpiled is the virtual text of a document as the program has it: what
// TypeScript checks.
func transpiled(_ context.Context, _ *rtsx.Program, file *rtsx.SourceFile, _ []byte) (any, error) {
	mapped, ok := mapper.Of(file)
	if !ok {
		return nil, errors.New("not an .rtsx document of a project")
	}
	return transpiledResult{Text: file.Text(), Step: step(mapped)}, nil
}

func step(file *mapper.File) string {
	switch {
	case file.Err != nil || file.Map == nil:
		return "source"
	case file.Output.Stopped != "":
		// "pass 3 (slot hoisting): why" names the pass first.
		pass, _, _ := strings.Cut(file.Output.Stopped, ":")
		return "stopped: " + pass
	}
	return "lowered"
}

// siblingsResult is the answer to reactogenic/siblings.
type siblingsResult struct {
	Files []string `json:"files"`
}

func siblings(_ context.Context, program *rtsx.Program, file *rtsx.SourceFile, _ []byte) (any, error) {
	if file == nil {
		return siblingsResult{Files: []string{}}, nil
	}
	_, _, _, mapped := rtsx.MappedFile(file)
	if !mapped {
		return siblingsResult{Files: []string{}}, nil
	}
	names := append([]string{}, server.FileNames(program, path.Dir(file.OriginalFileName()))...)
	sort.Strings(names)
	return siblingsResult{Files: names}, nil
}
