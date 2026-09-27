// Reactogenic: the .rtsx extensions to JSX attribute parsing.
// Not part of upstream: added by go/patches/0002-rtsx-parser.patch.

package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// isRTSX reports whether the file being parsed is .rtsx. Only .rtsx gets the
// extensions; .tsx keeps TypeScript's grammar.
func (p *Parser) isRTSX() bool {
	return tspath.FileExtensionIs(p.opts.FileName, ".rtsx")
}

func (p *Parser) nextTokenIsDotDotDot() bool {
	p.nextToken()
	return p.token == ast.KindDotDotDotToken
}

// parseJsxSlotParams parses slot params in attribute position:
// `{ size }`, `{ label: l, value = 0 }`, `{ size, ...rest }`, `{}`.
//
// The result is a JsxSpreadAttribute whose expression is an
// ObjectBindingPattern, a shape the .tsx grammar never produces, so no new
// node kind is needed. The checker never sees .rtsx trees: the transpiler
// lowers params away first.
func (p *Parser) parseJsxSlotParams() *ast.Node {
	pos := p.nodePos()
	pattern := p.parseObjectBindingPattern()
	return p.finishNode(p.factory.NewJsxSpreadAttribute(pattern), pos)
}

// isRTSXArgStart: `&` or `&&` starting a slot arg in an .rtsx attribute list.
func (p *Parser) isRTSXArgStart() bool {
	return (p.token == ast.KindAmpersandToken || p.token == ast.KindAmpersandAmpersandToken) && p.isRTSX()
}

// parseJsxArgAttribute parses a slot arg on an attachment: `&size`,
// `&label={x}`, `&&value={x}`. It is stored as a JsxAttribute whose name is
// an identifier spelled with its prefix ("&size", "&&value") — as TypeScript
// already stores `#name` — so no new node kind is needed. The checker never
// sees it: the transpiler lowers args away first.
func (p *Parser) parseJsxArgAttribute() *ast.Node {
	pos := p.nodePos()
	prefix := "&"
	if p.token == ast.KindAmpersandAmpersandToken {
		prefix = "&&"
	}
	prefixEnd := p.scanner.TokenEnd()
	p.nextToken()
	if p.scanner.TokenStart() != prefixEnd || !tokenIsIdentifierOrKeyword(p.token) {
		p.parseErrorAtCurrentToken(diagnostics.Identifier_expected) // `& size`, `&{…}`
	}
	p.scanJsxIdentifier()
	name := p.parseIdentifierNameErrorOnUnicodeEscapeSequence()
	id := p.finishNode(p.factory.NewIdentifier(prefix+name.Text()), pos)
	return p.finishNode(p.factory.NewJsxAttribute(id, p.parseJsxAttributeValue()), pos)
}
