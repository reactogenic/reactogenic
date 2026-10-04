package render

import (
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// stateful are the exports of `react` that shell code cannot use: state,
// effects, refs (builder.md, *Shell code in phase 2*, shell-react). The
// shell is rendered once, at build time, and no React is there to run them
// after.
var stateful = map[string]bool{
	"useState": true, "useReducer": true, "useEffect": true, "useLayoutEffect": true,
	"useInsertionEffect": true, "useRef": true, "useImperativeHandle": true,
	"useSyncExternalStore": true, "useTransition": true, "useDeferredValue": true,
	"useOptimistic": true, "useActionState": true,
}

// shellReact reports the uses of React's state and effects in one module of
// the pages' closure, at the import: `import { useState } from "react"`, and
// — through the default or the namespace import — `React.useState`. It reads
// the text only: nothing runs.
func shellReact(file *rtsx.SourceFile) []report.Report {
	var reports []report.Report
	add := func(name *rtsx.Node) {
		pos := rtsx.TokenStart(file, name)
		r := report.Report{Code: "shell-react", Message: "The shell cannot use React state or effects: `" + rtsx.NodeText(name) + "`"}
		r.File, r.Span, r.Line, r.Col = position(file, emit.Span{Pos: pos, End: name.End()})
		reports = append(reports, r)
	}
	namespaces := map[string]bool{} // `React` of `import React from "react"`, `import * as React from "react"`
	for _, statement := range file.Statements.Nodes {
		if statement.Kind != rtsx.KindImportDeclaration || rtsx.NodeText(statement.ModuleSpecifier()) != "react" {
			continue
		}
		clause := statement.ImportClause()
		if clause == nil || rtsx.IsTypeOnlyImport(clause) {
			continue
		}
		if name := clause.Name(); name != nil {
			namespaces[rtsx.NodeText(name)] = true
		}
		bindings := clause.AsImportClause().NamedBindings
		switch {
		case bindings == nil:
		case bindings.Kind == rtsx.KindNamespaceImport:
			namespaces[rtsx.NodeText(bindings.Name())] = true
		default:
			for _, specifier := range bindings.Elements() {
				if imported := specifier.PropertyNameOrName(); !specifier.IsTypeOnly() && stateful[rtsx.NodeText(imported)] {
					add(imported)
				}
			}
		}
	}
	if len(namespaces) == 0 {
		return reports
	}
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindPropertyAccessExpression && stateful[rtsx.NodeText(n.Name())] {
			if object := n.Expression(); object.Kind == rtsx.KindIdentifier && namespaces[rtsx.NodeText(object)] {
				add(n.Name())
			}
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return reports
}
