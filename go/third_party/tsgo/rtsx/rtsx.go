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
	"github.com/microsoft/TypeScript/tsc/internal/locale"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

type (
	SourceFile = ast.SourceFile
	Diagnostic = ast.Diagnostic
	Node       = ast.Node
	Kind       = ast.Kind
)

const (
	KindBlock                   = ast.KindBlock
	KindEndOfFile               = ast.KindEndOfFile
	KindExpressionStatement     = ast.KindExpressionStatement
	KindJsxText                 = ast.KindJsxText
	KindParenthesizedExpression = ast.KindParenthesizedExpression
)

// ParseTSX parses text as a .tsx file.
func ParseTSX(fileName string, text string) *SourceFile {
	opts := ast.SourceFileParseOptions{
		FileName: fileName,
		Path:     tspath.Path(fileName),
	}
	return parser.ParseSourceFile(opts, text, core.ScriptKindTSX)
}

// NodeText returns the text of an identifier, literal or JSX text node, and
// "" for any other node.
func NodeText(n *Node) string {
	switch n.Kind {
	case ast.KindJsxText:
		return n.AsJsxText().Text
	case ast.KindIdentifier, ast.KindPrivateIdentifier, ast.KindStringLiteral,
		ast.KindNumericLiteral, ast.KindBigIntLiteral, ast.KindRegularExpressionLiteral,
		ast.KindNoSubstitutionTemplateLiteral, ast.KindTemplateHead,
		ast.KindTemplateMiddle, ast.KindTemplateTail, ast.KindJsxNamespacedName:
		return n.Text()
	}
	return ""
}

// Message returns a diagnostic's message text, in English.
func Message(d *Diagnostic) string {
	return d.Localize(locale.Default)
}
