// Package project is a TypeScript project in which every Foo.rtsx is served,
// in memory, as its transpiled Foo.tsx (specs/phase01/diagnostics.md,
// *reactogenic check*; RGP1-050). TypeScript's own include globs and module
// resolution then find the .rtsx files with no help.
package project

import (
	"path"
	"strings"
	"sync"

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
	// tsgo parses files in parallel, so the overlay transpiles from many
	// goroutines: mu guards outputs and sources.
	mu      sync.Mutex
	outputs map[string]transpiler.Output // by .rtsx path
	sources map[string]string            // .rtsx text, by path
}

// Open builds the program of the tsconfig at configPath (absolute).
func Open(configPath string) *Project {
	p := &Project{config: configPath, cwd: path.Dir(configPath), disk: rtsx.OSFS()}
	p.build()
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

// transpile runs the transpiler on one .rtsx file, once, and keeps the
// output.
func (p *Project) transpile(file string) transpiler.Output {
	p.mu.Lock()
	defer p.mu.Unlock()
	if out, ok := p.outputs[file]; ok {
		return out
	}
	text, ok := p.sources[file]
	if !ok {
		text, _ = p.disk.ReadFile(file)
		p.sources[file] = text
	}
	out, err := transpiler.Transpile(transpiler.Input{Files: map[string]string{file: text}, Entry: file, ReadFile: p.disk.ReadFile})
	if err != nil {
		out = transpiler.Output{Diagnostics: []transpiler.Diagnostic{{File: file, Line: 1, Col: 1, Code: "internal", Message: err.Error()}}}
	}
	p.outputs[file] = out
	return out
}

// Output returns the transpiler output of an .rtsx file of the project.
func (p *Project) Output(rtsxPath string) (transpiler.Output, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out, ok := p.outputs[rtsxPath]
	return out, ok
}

// Source returns the text of an .rtsx file of the project, as transpiled.
func (p *Project) Source(rtsxPath string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	text, ok := p.sources[rtsxPath]
	return text, ok
}

// SourceOf returns the .rtsx file behind a virtual .tsx of the program.
func (p *Project) SourceOf(tsxPath string) (string, bool) {
	src := strings.TrimSuffix(tsxPath, ".tsx") + ".rtsx"
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.outputs[src]
	return src, ok && strings.HasSuffix(tsxPath, ".tsx")
}

// Outputs lists the .rtsx files the program read.
func (p *Project) Outputs() map[string]transpiler.Output {
	p.mu.Lock()
	defer p.mu.Unlock()
	outputs := make(map[string]transpiler.Output, len(p.outputs))
	for file, out := range p.outputs {
		outputs[file] = out
	}
	return outputs
}

// Diagnostics returns what `tsc --noEmit` reports on the program, with
// positions in the virtual .tsx files (RGP1-071 maps them back).
func (p *Project) Diagnostics() []*rtsx.Diagnostic {
	if p.Program == nil {
		return p.ConfigDiagnostics
	}
	return append(p.ConfigDiagnostics, rtsx.AllDiagnostics(p.Program)...)
}
