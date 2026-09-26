package syntax

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// Package is the module the framework's elements are imported from.
const Package = "@reactogenic/core"

// FrameworkExport returns the name `Switch`, `Match`, `Each`, … that ident
// refers to when it resolves to a named value import from Package, however it
// is aliased (`import { Switch as Choose }`); "" otherwise. Recognition is by
// import origin, never by name: a local `Switch` is not the framework's.
func FrameworkExport(ident *rtsx.Node) string {
	if ident.Kind != rtsx.KindIdentifier {
		return ""
	}
	decl := valueBinding(ident, rtsx.NodeText(ident))
	if decl == nil || decl.Kind != rtsx.KindImportSpecifier || !fromPackage(importDeclaration(decl)) {
		return ""
	}
	return importedName(decl)
}

func importedName(specifier *rtsx.Node) string {
	if p := specifier.PropertyName(); p != nil {
		return rtsx.NodeText(p)
	}
	return rtsx.NodeText(specifier.Name())
}

func importDeclaration(n *rtsx.Node) *rtsx.Node {
	for n != nil && n.Kind != rtsx.KindImportDeclaration {
		n = n.Parent
	}
	return n
}

func fromPackage(decl *rtsx.Node) bool {
	if decl == nil {
		return false
	}
	var spec *rtsx.Node
	switch decl.Kind {
	case rtsx.KindImportDeclaration:
		spec = decl.AsImportDeclaration().ModuleSpecifier
	case rtsx.KindExportDeclaration:
		spec = decl.AsExportDeclaration().ModuleSpecifier
	}
	return spec != nil && rtsx.NodeText(spec) == Package
}

// compileTimeOnly are the exports that exist only as elements: the compiler
// lowers them away, so there is no value to pass around (flow-as-value).
var compileTimeOnly = map[string]bool{"Switch": true, "Match": true}

// CheckFlowAsValue reports every use of `Switch` or `Match` other than as a
// JSX tag: `const S = Switch`, `as={Match}`, `createElement(Switch, …)`,
// re-exports, and `R.Switch` through a namespace import.
func CheckFlowAsValue(file *rtsx.SourceFile) []Error {
	var errs []Error
	report := func(n *rtsx.Node, name string) {
		errs = append(errs, Error{rtsx.TokenStart(file, n), n.End(), "flow-as-value",
			fmt.Sprintf("`%s` exists only as an element", name)})
	}
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		switch n.Kind {
		case rtsx.KindImportDeclaration:
			return false // the import itself is fine
		case rtsx.KindExportDeclaration:
			if fromPackage(n) {
				clause := n.AsExportDeclaration().ExportClause
				if clause == nil || clause.Kind != rtsx.KindNamedExports {
					report(n, "Switch") // export * re-exports them all
					return false
				}
				for _, spec := range clause.Elements() {
					if name := importedName(spec); compileTimeOnly[name] {
						report(spec, name)
					}
				}
				return false
			}
		case rtsx.KindExportSpecifier: // export { Switch }
			local := n.PropertyName()
			if local == nil {
				local = n.Name()
			}
			if name := FrameworkExport(local); compileTimeOnly[name] {
				report(n, name)
			}
			return false
		case rtsx.KindIdentifier:
			if isTagName(n) || !rtsx.IsValueReference(n) {
				return false
			}
			if name := FrameworkExport(n); compileTimeOnly[name] {
				report(n, name)
			}
			return false
		case rtsx.KindPropertyAccessExpression:
			if name := rtsx.NodeText(n.Name()); compileTimeOnly[name] && isPackageNamespace(n.Expression()) {
				report(n, name)
				return false
			}
			return visit(n.Expression()) // the member name is not a reference
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return errs
}

// isTagName reports whether ident is the tag of a JSX opening, closing or
// self-closing element.
func isTagName(ident *rtsx.Node) bool {
	p := ident.Parent
	switch p.Kind {
	case rtsx.KindJsxOpeningElement, rtsx.KindJsxSelfClosingElement, rtsx.KindJsxClosingElement:
		return p.TagName() == ident
	}
	return false
}

func isPackageNamespace(n *rtsx.Node) bool {
	if n.Kind != rtsx.KindIdentifier {
		return false
	}
	decl := valueBinding(n, rtsx.NodeText(n))
	return decl != nil && decl.Kind == rtsx.KindNamespaceImport && fromPackage(importDeclaration(decl))
}
