package syntax

import "github.com/microsoft/TypeScript/tsc/rtsx"

// Binding returns the declaration that name refers to at location, as a
// value, or nil (syntax.md, *Shorthand props → What "in scope" means*).
//
// The module's own scopes come from tsgo's binder: variables, parameters,
// functions, classes, value imports — never globals, ambient declarations or
// type-only imports. Slot params are the one scope the binder cannot know:
// they are found by walking up to the elements whose body holds location.
// The nearer of the two wins, as in any lexical lookup. The file must have
// been bound (rtsx.Bind).
func Binding(location *rtsx.Node, name string) *rtsx.Node {
	decl := valueBinding(location, name)
	param, scope := paramBinding(location, name)
	switch {
	case param == nil:
		return decl
	case decl == nil:
		return param
	case decl.Pos() >= scope.Pos() && decl.End() <= scope.End():
		return decl // declared inside the slot body: nearer than the param
	}
	return param
}

func valueBinding(location *rtsx.Node, name string) *rtsx.Node {
	sym := rtsx.ResolveValue(location, name)
	if sym == nil {
		return nil
	}
	for _, d := range sym.Declarations {
		if !rtsx.IsAmbient(d) && !rtsx.IsTypeOnlyImport(d) {
			return d
		}
	}
	return nil
}

// paramBinding finds the nearest element whose params bind name and whose
// body — its children, not its own attributes — holds location. It returns
// the binding's name node and the element.
func paramBinding(location *rtsx.Node, name string) (*rtsx.Node, *rtsx.Node) {
	for child, n := location, location.Parent; n != nil; child, n = n, n.Parent {
		if n.Kind != rtsx.KindJsxElement || child == n.AsJsxElement().OpeningElement {
			continue
		}
		for _, attr := range n.AsJsxElement().OpeningElement.Attributes().Properties() {
			if pattern, ok := SlotParams(attr); ok {
				if id := boundName(pattern, name); id != nil {
					return id, n
				}
			}
		}
	}
	return nil, nil
}

// boundName returns the identifier in a binding pattern that binds name.
func boundName(pattern *rtsx.Node, name string) *rtsx.Node {
	for _, el := range pattern.Elements() {
		switch target := el.Name(); {
		case target == nil:
		case target.Kind == rtsx.KindIdentifier:
			if rtsx.NodeText(target) == name {
				return target
			}
		default: // a nested pattern: `{ row: { id } }`
			if id := boundName(target, name); id != nil {
				return id
			}
		}
	}
	return nil
}
