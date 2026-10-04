package build

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// index are the files that make a directory a page, the first that is there
// (builder.md, *Routes*).
var index = []string{"index.rtsx", "index.tsx"}

// findRoutes returns the pages under dir, by pathname: `index.rtsx` (or
// `index.tsx`) of a directory is a page, and its pathname is the directory,
// with a trailing slash. Any other module there is a segment or a module of
// a page. An error: dir is not a directory that can be read.
//
// dir may be a symbolic link: it is walked where it leads, and its files are
// named under dir all the same — as a tsconfig that lists `pages` names
// them. A directory inside it that is a link is not followed (builder.md,
// *Routes*).
func findRoutes(dir string) ([]render.Route, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, &fs.PathError{Op: "read", Path: dir, Err: fs.ErrInvalid}
	}
	var routes []render.Route
	root := real(dir)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, name := range index {
			if info, err := os.Stat(filepath.Join(path, name)); err != nil || !info.Mode().IsRegular() {
				continue
			}
			pathname := "/"
			if rel != "." {
				pathname = "/" + filepath.ToSlash(rel) + "/"
			}
			routes = append(routes, render.Route{Pathname: pathname, File: filepath.ToSlash(filepath.Join(dir, rel, name))})
			break
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(routes, func(a, b render.Route) int { return strings.Compare(a.Pathname, b.Pathname) })
	return routes, nil
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
// one of its own (builder.md, *Routes*: public-conflict): a page's
// `index.html`, anything under `_rg/` — and a file where the build needs a
// directory, or under what the build writes as a file: `public/guide`
// beside the page `/guide/`, `public/_rg`.
func conflicts(opts Options, routes []render.Route, static []string) []report.Report {
	written := map[string]render.Route{} // a page's file → the page
	inside := map[string]render.Route{}  // a directory the build makes → the first page in it
	for _, route := range routes {
		file := output(route.Pathname)
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
			return "The page " + route.Pathname + " is written to `" + output(route.Pathname) + "`: "
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
