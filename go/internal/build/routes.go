package build

import (
	"cmp"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// extension is what a variant's file ends in: only an `.rtsx` file of a
// route directory is built to a document (builder.md, *Routes*).
const extension = ".rtsx"

// candidates returns every `.rtsx` file under dir as the variant it is when
// nothing of the project mounts or imports it (variants): the route is its
// directory — the pathname, with a trailing slash — and the variant its
// name. Whether a file is one is asked of the program, never of its name
// and never by running it; so a file of any other kind — `server.ts`, a
// stylesheet — is not read here at all. Sorted by route, `index` first. An
// error: dir is not a directory that can be read.
//
// dir may be a symbolic link: it is walked where it leads, and its files are
// named under dir all the same — as a tsconfig that lists `pages` names
// them. A directory inside it that is a link is not followed (builder.md,
// *The project*).
func candidates(dir string) ([]render.Route, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, &fs.PathError{Op: "read", Path: dir, Err: fs.ErrInvalid}
	}
	var routes []render.Route
	root := real(dir)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, ok := strings.CutSuffix(entry.Name(), extension)
		// A link to a file is the file; a directory named as one is none.
		if info, err := os.Stat(path); !ok || name == "" || err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		pathname := "/"
		if rel != "." {
			pathname = "/" + filepath.ToSlash(rel) + "/"
		}
		routes = append(routes, render.Route{Pathname: pathname, Variant: name, File: filepath.ToSlash(filepath.Join(dir, rel, entry.Name()))})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// `index` sorts as the empty name: before every other.
	name := func(r render.Route) string {
		if r.Variant == render.Index {
			return ""
		}
		return r.Variant
	}
	slices.SortFunc(routes, func(a, b render.Route) int {
		return cmp.Or(strings.Compare(a.Pathname, b.Pathname), strings.Compare(name(a), name(b)))
	})
	return routes, nil
}

// variants are the candidates that no module of the program imports
// (builder.md, *Routes*): each is built to a document. Any other `.rtsx`
// file of a route directory is a segment — a segment root `#name` is an
// import in the emitted TSX — or a module of a variant.
//
// The graph is the program's, as it was checked: every import it resolved,
// of every module that is not a declaration — `import`, `export … from`,
// `import()`, and one of types alone. A module that imports itself does not
// make itself a segment. A candidate the program does not hold is imported
// by nothing of it: a variant, which render then refuses (render-bundle).
//
// Files are compared by what they are: the program names a file its
// tsconfig lists as the tsconfig reaches it, and one it imports by where
// the import leads — two names for one file when the pages are behind a
// link (named).
func variants(program *rtsx.Program, candidates []render.Route) []render.Route {
	imported := map[string]bool{}
	for _, file := range program.GetSourceFiles() {
		if file.IsDeclarationFile {
			continue
		}
		var self string
		for _, written := range file.Imports() {
			resolved := program.GetResolvedModuleFromModuleSpecifier(file, written)
			if resolved == nil || !strings.HasSuffix(resolved.ResolvedFileName, extension) {
				continue
			}
			if self == "" {
				self = real(filepath.FromSlash(file.FileName()))
			}
			if is := real(filepath.FromSlash(resolved.ResolvedFileName)); is != self {
				imported[is] = true
			}
		}
	}
	var out []render.Route
	for _, candidate := range candidates {
		if !imported[real(filepath.FromSlash(candidate.File))] {
			out = append(out, candidate)
		}
	}
	return out
}

// publicFiles are the files of the `public` directory, by their path from
// it, with forward slashes, sorted: they are copied as they are (builder.md,
// *Not in phase 2*), and a link to one is a link to a file of the output. A
// directory that is not there holds none. Like the pages, it may be a link.
func publicFiles(dir string) ([]string, error) {
	var files []string
	// A file named `public` is no directory of files.
	if info, err := os.Stat(dir); os.IsNotExist(err) || err == nil && !info.IsDir() {
		return nil, nil
	}
	root := real(dir)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		// A link to a file is the file; anything else that is not one — a
		// link to a directory, a socket — is not copied.
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(files)
	return files, err
}

// conflicts reports the files of `public/` that are where the build writes
// one of its own (builder.md, *Routes*: public-conflict): a variant's
// document — `index.html`, `guest.html` — anything under `_rg/`; and a file
// where the build needs a directory, or under what the build writes as a
// file: `public/guide` beside the route `/guide/`, `public/_rg`.
func conflicts(opts Options, routes []render.Route, static []string) []report.Report {
	written := map[string]render.Route{} // a document → its variant
	inside := map[string]render.Route{}  // a directory the build makes → the first variant in it
	for _, route := range routes {
		file := route.Output()
		written[file] = route
		for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
			if _, had := inside[dir]; !had {
				inside[dir] = route
			}
		}
	}
	var reports []report.Report
	for _, file := range static {
		conflict := func(message string) {
			name := filepath.ToSlash(filepath.Join(opts.public(), filepath.FromSlash(file)))
			reports = append(reports, report.Report{File: name, Code: "public-conflict", Message: message})
		}
		page := func(route render.Route) string {
			return "The page " + route.Path() + " is written to `" + route.Output() + "`: "
		}
		if route, taken := written[file]; taken {
			conflict(page(route) + "a file of `public/` cannot be there")
		} else if file == assets || strings.HasPrefix(file, assets+"/") {
			conflict("`" + assets + "/` is the builder's: a file of `public/` cannot be there")
		} else if route, taken := inside[file]; taken {
			conflict(page(route) + "`" + file + "` is a directory of the output, and cannot be a file of `public/`")
		} else {
			for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
				if route, taken := written[dir]; taken {
					conflict(page(route) + "a file of `public/` cannot be under it")
					break
				}
			}
		}
	}
	return reports
}
