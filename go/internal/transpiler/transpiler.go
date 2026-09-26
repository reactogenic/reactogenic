package transpiler

import "errors"

// ErrNotImplemented is returned until the passes exist (M3).
var ErrNotImplemented = errors.New("transpiler: not implemented")

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
	Code     string // e.g. "orphan-slot"
	Message  string
}

type Output struct {
	TSX         string
	Diagnostics []Diagnostic
}

// Transpile turns Input.Entry from .rtsx into .tsx.
func Transpile(in Input) (Output, error) {
	return Output{}, ErrNotImplemented
}
