// Package rtsx is Reactogenic's bridge into tsgo's internal packages.
//
// Go allows internal/ to be imported only from inside this module, so
// Reactogenic's transpiler reaches the parser, AST and checker through here.
// Not part of upstream: added by go/patches/0001-rtsx-bridge.patch.
// Keep it thin — aliases and one-line wrappers only.
package rtsx

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

type (
	SourceFile = ast.SourceFile
	Diagnostic = ast.Diagnostic
)

// ParseTSX parses text as a .tsx file.
func ParseTSX(fileName string, text string) *SourceFile {
	opts := ast.SourceFileParseOptions{
		FileName: fileName,
		Path:     tspath.Path(fileName),
	}
	return parser.ParseSourceFile(opts, text, core.ScriptKindTSX)
}
