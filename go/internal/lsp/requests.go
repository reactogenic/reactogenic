package lsp

import (
	"context"
	"encoding/json"
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
	// methodSlots is the front's: the `$` props of what the name at a source
	// offset of a document names — a component, a slot (ide.md, *Slots*).
	methodSlots = "reactogenic/slots"
)

var requests = map[string]func(ctx context.Context, program *rtsx.Program, file *rtsx.SourceFile, params []byte) (any, error){
	methodTranspiled: transpiled,
	methodSiblings:   siblings,
	methodSlots:      slots,
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

// slotsResult is the answer to reactogenic/slots.
type slotsResult struct {
	Slots []slotProp `json:"slots"`
}

// slotProp is a slot that an owner declares.
type slotProp struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
	Type     string `json:"type"`
}

// slots are the `$` props of the owner whose name is at the source offset
// of the request: the checker's answer at that name's copy in the virtual
// text — the props of a component at its tag, the props of a slot's value at
// the slot's name. Empty when the name has no copy, or no type.
func slots(ctx context.Context, program *rtsx.Program, file *rtsx.SourceFile, params []byte) (any, error) {
	result := slotsResult{Slots: []slotProp{}}
	var p struct {
		Offset int `json:"offset"`
	}
	if file == nil || json.Unmarshal(params, &p) != nil {
		return result, nil
	}
	mapped, ok := mapper.Of(file)
	if !ok || mapped.Map == nil {
		return result, nil
	}
	at, ok := mapped.Map.Output(p.Offset)
	if !ok {
		return result, nil
	}
	checker, done := rtsx.GetChecker(ctx, program, file)
	defer done()
	for _, prop := range rtsx.PropsAt(checker, file, at) {
		if strings.HasPrefix(prop.Name, "$") {
			result.Slots = append(result.Slots, slotProp{Name: prop.Name, Optional: prop.Optional, Type: prop.Type})
		}
	}
	return result, nil
}
