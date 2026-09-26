package transpiler

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// Input is one transpilation: the entry file plus the files it may need
// (segments, containers), keyed by path.
type Input struct {
	Files map[string]string
	Entry string
	// UntilPass stops after the given pass of syntax.md, *Compilation
	// passes*; 0 runs them all.
	UntilPass int
}

type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a transpiler error on the .rtsx source. Line and Col are
// 1-based.
type Diagnostic struct {
	File     string
	Line     int
	Col      int
	Severity Severity
	Code     string // e.g. "orphan-slot"; "TS1005" for a TypeScript syntax error
	Message  string
}

type Output struct {
	TSX         string
	Diagnostics []Diagnostic
	// Map maps TSX back to the entry's source (diagnostics.md, *Mapping*).
	Map *emit.Map
}

// A pass reads its input — the source, or the previous pass's output — and
// returns edits to it (syntax.md, *Compilation passes*).
// A repeating pass runs again on its own output until it returns no edits.
type pass struct {
	n      int
	name   string
	run    func(c *passContext) []emit.Edit
	repeat bool
}

var passes = []pass{
	{0, "checks", checks, false},
	{1, "shorthand props", shorthand, false},
	{2, "flow lowering", flow, true},
	{3, "slot hoisting", nil, false},
	{4, "segment roots", nil, false},
}

// maxRuns bounds a repeating pass; a pass that keeps editing is a bug.
const maxRuns = 1000

// Transpile turns Input.Entry from .rtsx into .tsx. Errors in the source are
// Diagnostics; the error result is for failures of the transpiler itself.
func Transpile(in Input) (Output, error) {
	src, ok := in.Files[in.Entry]
	if !ok {
		return Output{}, fmt.Errorf("transpiler: no file %q", in.Entry)
	}
	var (
		out      Output
		text     = src
		toSource = emit.Identity(len(src))
		names    map[string]bool
		runs     int
	)
	for i := 0; i < len(passes); i++ {
		p := passes[i]
		if in.UntilPass > 0 && p.n > in.UntilPass {
			break
		}
		file := rtsx.ParseRTSX("/"+in.Entry, text)
		if parseErrors := file.Diagnostics(); len(parseErrors) > 0 {
			if p.n > 0 {
				return Output{}, fmt.Errorf("transpiler: pass %d (%s) produced code that does not parse: %s", p.n-1, passes[p.n-1].name, rtsx.Message(parseErrors[0]))
			}
			for _, d := range parseErrors {
				out.add(in.Entry, src, emit.Span{Pos: rtsx.SkipTrivia(src, d.Pos()), End: d.End()}, Error, fmt.Sprintf("TS%d", d.Code()), rtsx.Message(d))
			}
			return out, nil
		}
		if names == nil {
			names = identifiers(file)
		}
		if p.run == nil {
			continue
		}
		rtsx.Bind(file)
		c := &passContext{file: file, text: text, names: names, imports: map[string]bool{},
			report: func(s emit.Span, sev Severity, code, msg string) {
				out.add(in.Entry, src, toSource.Source(s), sev, code, msg)
			}}
		edits := p.run(c)
		if len(edits) == 0 {
			continue
		}
		next, m, err := emit.Apply(text, edits)
		if err != nil {
			return Output{}, fmt.Errorf("transpiler: pass %d (%s): %w", p.n, p.name, err)
		}
		text, toSource = next, m.Then(toSource)
		if p.repeat {
			if runs++; runs > maxRuns {
				return Output{}, fmt.Errorf("transpiler: pass %d (%s) does not settle", p.n, p.name)
			}
			i-- // run it again on its own output
		}
	}
	out.TSX, out.Map = text, toSource
	return out, nil
}

func (o *Output) add(file, src string, s emit.Span, sev Severity, code, msg string) {
	line, col := emit.LineCol(src, s.Pos)
	o.Diagnostics = append(o.Diagnostics, Diagnostic{File: file, Line: line, Col: col, Severity: sev, Code: code, Message: msg})
}

// checks is pass 0: errors reported against what the author wrote.
func checks(c *passContext) []emit.Edit {
	for _, e := range append(syntax.Check(c.file), syntax.CheckFlowAsValue(c.file)...) {
		c.report(emit.Span{Pos: e.Pos, End: e.End}, Error, e.Code, e.Message)
	}
	return nil
}
