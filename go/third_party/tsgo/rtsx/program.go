// Reactogenic's bridge to tsgo's program, file system and checker.
// Not part of upstream: added by go/patches/0003-rtsx-program.patch.
// Keep it thin — aliases and short wrappers only.

package rtsx

import (
	"context"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/wrapvfs"
)

type (
	FS             = vfs.FS
	FSEntries      = vfs.Entries
	FSReplacements = wrapvfs.Replacements
	Program        = compiler.Program
	Checker        = checker.Checker
	Type           = checker.Type
)

// OSFS is the real file system.
func OSFS() FS {
	return osvfs.FS()
}

// WrapFS overrides some methods of fs; nil replacements fall through.
func WrapFS(fs FS, r FSReplacements) FS {
	return wrapvfs.Wrap(fs, r)
}

// NewProgram reads the tsconfig at configPath through fs and builds its
// program, with TypeScript's lib files bundled. It also returns the config's
// parsing diagnostics; the program is nil when the config cannot be read.
func NewProgram(configPath, cwd string, fs FS) (*Program, []*Diagnostic) {
	host := compiler.NewCompilerHost(cwd, bundled.WrapFS(fs), bundled.LibPath(), nil, nil, nil)
	config, diags := tsoptions.GetParsedCommandLineOfConfigFile(configPath, nil, nil, host, nil)
	if config == nil {
		return nil, diags
	}
	program := compiler.NewProgram(compiler.ProgramOptions{Host: host, Config: config})
	return program, diags
}

// AllDiagnostics returns what `tsc --noEmit` reports for p, sorted:
// config, syntactic, program, bind, global and semantic diagnostics.
func AllDiagnostics(p *Program) []*Diagnostic {
	ctx := context.Background()
	return compiler.SortAndDeduplicateDiagnostics(
		compiler.GetDiagnosticsOfAnyProgram(ctx, p, nil, false, p.GetBindDiagnostics, p.GetSemanticDiagnostics))
}

// GetChecker returns p's checker and the func that releases it.
func GetChecker(p *Program) (*Checker, func()) {
	return p.GetTypeChecker(context.Background())
}

// TokenAt is the token at pos in file.
func TokenAt(file *SourceFile, pos int) *Node {
	return astnav.GetTokenAtPosition(file, pos)
}

// FileOf is the source file a node belongs to.
func FileOf(n *Node) *SourceFile {
	return ast.GetSourceFileOfNode(n)
}

// SlotDeclaration answers, for a JSX tag: the declaration of the component
// it refers to, and whether the component's props declare the prop slot, and
// as optional. It reads the props type from the component's call
// signatures.
func SlotDeclaration(c *Checker, tag *Node, slot string) (decl *Node, declared, optional bool) {
	if sym := c.GetSymbolAtLocation(tag); sym != nil {
		if sym.Flags&ast.SymbolFlagsAlias != 0 {
			sym = c.GetAliasedSymbol(sym)
		}
		decl = sym.ValueDeclaration
	}
	for _, sig := range c.GetSignaturesOfType(c.GetTypeAtLocation(tag), checker.SignatureKindCall) {
		if len(sig.Parameters()) == 0 {
			continue
		}
		if prop := c.GetPropertyOfType(c.GetTypeAtPosition(sig, 0), slot); prop != nil {
			return decl, true, prop.Flags&ast.SymbolFlagsOptional != 0
		}
	}
	return decl, false, false
}

// IsError reports whether a diagnostic is an error, not a warning or a
// suggestion.
func IsError(d *Diagnostic) bool {
	return d.Category() == diagnostics.CategoryError
}
