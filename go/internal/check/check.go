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
	reports, _, _ := run(configPath, nil)
	return reports
}

// Program is Run for a caller that goes on with what was checked — `build`,
// which renders the pages from it (specs/phase02/builder.md, *The
// pipeline*). It also returns one program: that of the project the files
// are of — the project that owns, as below, the first of them any project
// lists; when none lists one, the project of configPath itself. So a
// tsconfig that only references others (Vite's template) is built through
// them, as it is checked. nil: the tsconfig cannot be read.
func Program(configPath string, files []string) ([]Report, *rtsx.Program) {
	reports, _, program := run(configPath, files)
	return reports, program
}

// run checks the project and, first, the projects it references — a tsconfig
// with `references` and no files of its own (Vite's template) is checked
// through them. It also returns the tsconfig files read, for --watch.
//
// A referenced project's modules are in the referencing program too, read
// from source — and checked there with the referencing project's options.
// So each file is reported once, by its own project: the first that lists
// it, in `files` or through `include`, whichever project is checked first.
// A file that no project lists — one reached through an import only — is
// reported by the first that holds it.
//
// kept is the program of the project that owns the first of files that a
// project lists (Program); every other is let go once it is checked.
func run(configPath string, files []string) (reports []Report, configs []string, kept *rtsx.Program) {
	fs := rtsx.OSFS()
	type loaded struct {
		project     *rtsx.Project // nil: the tsconfig cannot be read
		diagnostics []*rtsx.Diagnostic
	}
	var projects []loaded // a project after those it references
	var load func(config string)
	load = func(config string) {
		if slices.Contains(configs, config) {
			return
		}
		configs = append(configs, config)
		// A referenced tsconfig that is not there is the referencing
		// project's to report (TS6053), once.
		if config != configPath && !fs.FileExists(config) {
			return
		}
		project, diagnostics := rtsx.LoadProject(config, path.Dir(config), fs)
		if project != nil {
			for _, reference := range project.References() {
				load(reference)
			}
		}
		projects = append(projects, loaded{project, diagnostics})
	}
	load(configPath)

	owner := map[string]int{} // a listed file → the first project that lists it
	for i, p := range projects {
		if p.project == nil {
			continue
		}
		for _, file := range p.project.Files() {
			if _, listed := owner[file]; !listed {
				owner[file] = i
			}
		}
	}
	keep := len(projects) - 1 // configPath's own: a project comes after those it references
	for _, file := range files {
		if o, listed := owner[file]; listed {
			keep = o
			break
		}
	}
	reported := map[string]bool{} // the files no project lists, and the reports that have no file
	for i, p := range projects {
		var program *rtsx.Program // one at a time: built, checked, let go
		if p.project != nil {
			program = p.project.Program()
		}
		if i == keep {
			kept = program
		}
		held := map[string]bool{}
		for _, r := range report.Program(program, p.diagnostics) {
			key := r.File
			if key == "" {
				key = "\x00" + r.Code + r.Message
			}
			if o, listed := owner[key]; listed && o != i || !listed && reported[key] {
				continue
			}
			reports = append(reports, r)
			held[key] = true
		}
		if program != nil {
			for _, file := range program.GetSourceFiles() {
				held[file.FileName()] = true
			}
		}
		for key := range held {
			reported[key] = true
		}
	}
	slices.SortStableFunc(reports, func(a, b Report) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	return reports, configs, kept
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
// otherwise `file(line,col): error CODE: message`. A report about a file as a
// whole (line 0: report.Page, the builder's) names the file alone:
// `file - error CODE: message`, `file: error CODE: message`.
func Print(w io.Writer, reports []Report, cwd string, pretty bool, readFile func(string) (string, bool)) {
	for _, r := range reports {
		kind := r.Severity.String()
		name := rel(cwd, r.File)
		switch {
		case r.File == "":
			fmt.Fprintf(w, "%s %s: %s\n", kind, r.Code, r.Message)
		case r.Line == 0 && pretty:
			fmt.Fprintf(w, "%s - %s %s: %s\n", name, kind, r.Code, r.Message)
		case r.Line == 0:
			fmt.Fprintf(w, "%s: %s %s: %s\n", name, kind, r.Code, r.Message)
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
