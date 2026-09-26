// Package project is a TypeScript project in which every Foo.rtsx is served,
// in memory, as its transpiled Foo.tsx (specs/phase01/diagnostics.md,
// *reactogenic check*; RGP1-050). TypeScript's own include globs and module
// resolution then find the .rtsx files with no help.
package project

import (
	"path"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// Project is one tsconfig's program, with .rtsx files transpiled.
type Project struct {
	Program *rtsx.Program
	// ConfigDiagnostics are the tsconfig's parsing diagnostics.
	ConfigDiagnostics []*rtsx.Diagnostic

	config, cwd string
	disk        rtsx.FS
	outputs     map[string]transpiler.Output // by .rtsx path
	sources     map[string]string            // .rtsx text, by path
	listSlot    func(file, container, slot string) bool
}

// Open builds the program of the tsconfig at configPath (absolute).
//
// List slots are type-directed: the transpiler asks whether a container
// declares a slot as an array. Open builds the program once with no answers,
// asks that program's checker, and rebuilds if an answer changed an output.
// The answer depends only on the container's declaration, which desugaring
// never changes, so one round settles it (decisions.md, RGP1-001).
func Open(configPath string) *Project {
	p := &Project{config: configPath, cwd: path.Dir(configPath), disk: rtsx.OSFS()}
	p.build()
	if p.Program == nil {
		return p
	}
	checker, done := rtsx.GetChecker(p.Program)
	answers := map[string]bool{}
	p.listSlot = func(file, container, slot string) bool {
		key := file + "|" + container + "|" + slot
		if answer, ok := answers[key]; ok {
			return answer
		}
		tag := findTag(p.Program.GetSourceFile(tsxPath(file)), container)
		answers[key] = tag != nil && rtsx.IsListSlot(checker, tag, slot)
		return answers[key]
	}
	changed := false
	for file, out := range p.outputs {
		if next := p.transpile(file); next.TSX != out.TSX {
			changed = true
		}
	}
	done()
	if changed {
		p.build() // reads the settled outputs
	}
	return p
}

// build creates the program over the overlay; outputs already transpiled are
// reused, others are transpiled on first read.
func (p *Project) build() {
	if p.outputs == nil {
		p.outputs = map[string]transpiler.Output{}
		p.sources = map[string]string{}
	}
	fs := rtsx.WrapFS(p.disk, rtsx.FSReplacements{
		FileExists: func(name string) bool {
			_, virtual := p.virtual(name)
			return virtual || p.disk.FileExists(name)
		},
		ReadFile: func(name string) (string, bool) {
			if src, ok := p.virtual(name); ok {
				if out, ok := p.outputs[src]; ok {
					return out.TSX, true
				}
				return p.transpile(src).TSX, true
			}
			return p.disk.ReadFile(name)
		},
		GetAccessibleEntries: func(dir string) rtsx.FSEntries {
			entries := p.disk.GetAccessibleEntries(dir)
			have := map[string]bool{}
			for _, f := range entries.Files {
				have[f] = true
			}
			for _, f := range entries.Files {
				if tsx := strings.TrimSuffix(f, ".rtsx") + ".tsx"; strings.HasSuffix(f, ".rtsx") && !have[tsx] {
					entries.Files = append(entries.Files, tsx)
				}
			}
			return entries
		},
	})
	p.Program, p.ConfigDiagnostics = rtsx.NewProgram(p.config, p.cwd, fs)
}

// virtual returns the .rtsx file behind a Foo.tsx that is not on disk.
func (p *Project) virtual(name string) (string, bool) {
	if !strings.HasSuffix(name, ".tsx") || p.disk.FileExists(name) {
		return "", false
	}
	src := strings.TrimSuffix(name, ".tsx") + ".rtsx"
	return src, p.disk.FileExists(src)
}

func tsxPath(rtsxPath string) string {
	return strings.TrimSuffix(rtsxPath, ".rtsx") + ".tsx"
}

// transpile runs the transpiler on one .rtsx file and keeps the output.
func (p *Project) transpile(file string) transpiler.Output {
	text, ok := p.sources[file]
	if !ok {
		text, _ = p.disk.ReadFile(file)
		p.sources[file] = text
	}
	in := transpiler.Input{Files: map[string]string{file: text}, Entry: file, ReadFile: p.disk.ReadFile}
	if p.listSlot != nil {
		in.ListSlot = func(container, slot string) bool { return p.listSlot(file, container, slot) }
	}
	out, err := transpiler.Transpile(in)
	if err != nil {
		out = transpiler.Output{Diagnostics: []transpiler.Diagnostic{{File: file, Line: 1, Col: 1, Code: "internal", Message: err.Error()}}}
	}
	p.outputs[file] = out
	return out
}

// Output returns the transpiler output of an .rtsx file of the project.
func (p *Project) Output(rtsxPath string) (transpiler.Output, bool) {
	out, ok := p.outputs[rtsxPath]
	return out, ok
}

// Outputs lists the .rtsx files the program read.
func (p *Project) Outputs() map[string]transpiler.Output {
	return p.outputs
}

// Diagnostics returns what `tsc --noEmit` reports on the program, with
// positions in the virtual .tsx files (RGP1-071 maps them back).
func (p *Project) Diagnostics() []*rtsx.Diagnostic {
	if p.Program == nil {
		return p.ConfigDiagnostics
	}
	return append(p.ConfigDiagnostics, rtsx.AllDiagnostics(p.Program)...)
}

// findTag returns the tag name of the first JSX element in file written as
// container: where the list-slot question is asked.
func findTag(file *rtsx.SourceFile, container string) *rtsx.Node {
	if file == nil {
		return nil
	}
	var found *rtsx.Node
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxOpeningElement || n.Kind == rtsx.KindJsxSelfClosingElement {
			tag := n.TagName()
			if strings.TrimSpace(file.Text()[rtsx.TokenStart(file, tag):tag.End()]) == container {
				found = tag
				return true
			}
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return found
}
