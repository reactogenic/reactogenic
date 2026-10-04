package build

import (
	"io/fs"
	"os"
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
func findRoutes(dir string) ([]render.Route, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, &fs.PathError{Op: "read", Path: dir, Err: fs.ErrInvalid}
	}
	var routes []render.Route
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		for _, name := range index {
			file := filepath.Join(path, name)
			if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
				continue
			}
			pathname := "/"
			if rel, _ := filepath.Rel(dir, path); rel != "." {
				pathname = "/" + filepath.ToSlash(rel) + "/"
			}
			routes = append(routes, render.Route{Pathname: pathname, File: filepath.ToSlash(file)})
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
// directory that is not there holds none.
func publicFiles(dir string) ([]string, error) {
	var files []string
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		// A link to a file is the file; anything else that is not one — a
		// link to a directory, a socket — is not copied.
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(files)
	return files, err
}

// conflicts reports the files of `public/` that are where the build writes
// one of its own: a page's `index.html`, anything under `_rg/`.
func conflicts(opts Options, routes []render.Route, static []string) []report.Report {
	written := map[string]render.Route{}
	for _, route := range routes {
		written[strings.TrimPrefix(route.Pathname, "/")+"index.html"] = route
	}
	var reports []report.Report
	for _, file := range static {
		name := filepath.ToSlash(filepath.Join(opts.public(), filepath.FromSlash(file)))
		if route, taken := written[file]; taken {
			reports = append(reports, report.Report{File: name, Code: "public-conflict", Message: "The page " + route.Pathname + " is written to `" + file + "`: a file of `public/` cannot be there"})
		} else if strings.HasPrefix(file, assets+"/") {
			reports = append(reports, report.Report{File: name, Code: "public-conflict", Message: "`" + assets + "/` is the builder's: a file of `public/` cannot be there"})
		}
	}
	return reports
}
