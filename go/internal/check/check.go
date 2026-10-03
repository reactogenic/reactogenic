// Package check is `reactogenic check` (specs/phase01/diagnostics.md): the
// transpiler's errors and TS7's, all reported on the files the author wrote.
//
// The program is the mapped one (specs/phase01/ide.md, *The engine*): every
// .rtsx file is a module under its own name, checked through its emitted
// TSX. The caller registers the transform first — mapper.RegisterStrict: a
// build fails on a syntax error. What is reported is the reporting layer's
// (internal/report), which the language server shares.
package check

import (
	"cmp"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/report"
)

// Report is one diagnostic, positioned in a file the author wrote.
type Report = report.Report

// Run type-checks the project of the tsconfig at configPath (absolute).
func Run(configPath string) []Report {
	reports, _ := run(configPath)
	return reports
}

// run checks the project and, first, the projects it references — a tsconfig
// with `references` and no files of its own (Vite's template) is checked
// through them. A referenced project's modules are in the referencing
// program too, read from source; each file is reported once, by the first
// project that holds it: its own, checked with its own options. It also
// returns the tsconfig files read, for --watch.
func run(configPath string) (reports []Report, configs []string) {
	reported := map[string]bool{} // files, and the reports that have none
	var project func(config string)
	project = func(config string) {
		if slices.Contains(configs, config) {
			return
		}
		configs = append(configs, config)
		program, configDiagnostics := rtsx.NewProgram(config, path.Dir(config), rtsx.OSFS())
		if program != nil {
			for _, reference := range rtsx.ProjectReferences(program) {
				project(reference)
			}
		}
		files := map[string]bool{}
		for _, r := range report.Program(program, configDiagnostics) {
			key := r.File
			if key == "" {
				key = "\x00" + r.Code + r.Message
			}
			if !reported[key] {
				reports = append(reports, r)
				files[key] = true
			}
		}
		if program != nil {
			for _, file := range program.GetSourceFiles() {
				files[file.FileName()] = true
			}
		}
		for file := range files {
			reported[file] = true
		}
	}
	project(configPath)
	slices.SortStableFunc(reports, func(a, b Report) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	return reports, configs
}

// Errors counts the reports that are errors.
func Errors(reports []Report) int {
	n := 0
	for _, r := range reports {
		if r.Severity == report.Error {
			n++
		}
	}
	return n
}

// Print writes reports in tsc's format, paths relative to cwd. pretty adds a
// code frame of the source line: `file:line:col - error CODE: message`;
// otherwise `file(line,col): error CODE: message`.
func Print(w io.Writer, reports []Report, cwd string, pretty bool, readFile func(string) (string, bool)) {
	for _, r := range reports {
		kind := r.Severity.String()
		name := rel(cwd, r.File)
		switch {
		case r.File == "":
			fmt.Fprintf(w, "%s %s: %s\n", kind, r.Code, r.Message)
		case pretty:
			fmt.Fprintf(w, "%s:%d:%d - %s %s: %s\n", name, r.Line, r.Col, kind, r.Code, r.Message)
			frame(w, readFile, r.File, r.Line, r.Col)
		default:
			fmt.Fprintf(w, "%s(%d,%d): %s %s: %s\n", name, r.Line, r.Col, kind, r.Code, r.Message)
		}
		for _, rr := range r.Related {
			if rr.File != "" {
				fmt.Fprintf(w, "  %s:%d:%d - %s\n", rel(cwd, rr.File), rr.Line, rr.Col, rr.Message)
			} else {
				fmt.Fprintf(w, "  %s\n", rr.Message)
			}
		}
	}
	if pretty && len(reports) > 0 {
		fmt.Fprintf(w, "\nFound %d error(s).\n", Errors(reports))
	}
}

func rel(cwd, file string) string {
	if file == "" {
		return ""
	}
	if r, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return file
}

// frame prints the source line of a report with a caret under its column.
func frame(w io.Writer, readFile func(string) (string, bool), file string, line, col int) {
	text, ok := readFile(file)
	if !ok {
		return
	}
	lines := strings.Split(text, "\n")
	if line < 1 || line > len(lines) {
		return
	}
	src := strings.TrimRight(lines[line-1], "\r")
	gutter := fmt.Sprintf("%d", line)
	fmt.Fprintf(w, "\n%s %s\n%s %s^\n\n", gutter, src, strings.Repeat(" ", len(gutter)), strings.Repeat(" ", max(col-1, 0)))
}
