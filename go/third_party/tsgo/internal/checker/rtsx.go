// Reactogenic: what the host's rename asks of the checker. Not part of
// upstream: added by go/patches/0006-rtsx-lsp.patch.

package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// JsxChildrenPropertyName is the name of the prop that an element's body is
// the value of, for the JSX namespace at location: `children`, unless
// `JSX.ElementChildrenAttribute` names another; "" when a body is no prop.
func (c *Checker) JsxChildrenPropertyName(location *ast.Node) string {
	return c.getJsxElementChildrenPropertyName(c.getJsxNamespaceAt(location))
}
