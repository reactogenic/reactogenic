package build

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
)

// CheckOut says whether the output directory may be emptied (builder.md,
// *The output directory*): the build removes what is in it, so it has to be
// a directory that is the builder's to empty. An error is why it is not; it
// is a usage error, found before anything is built.
//
// Directories are compared by what they are, not by how they are spelt: a
// symbolic link, or another case of the name on a file system that folds
// it, is the same directory.
func CheckOut(opts Options) error {
	out := opts.Out
	refuse := func(why string) error {
		return fmt.Errorf("--out %s %s", out, why)
	}
	info, err := os.Stat(out)
	exists := err == nil
	switch {
	case os.IsNotExist(err): // made by the build
	case err != nil:
		return refuse("cannot be read: " + err.Error())
	case !info.IsDir():
		return refuse("is not a directory")
	}
	sources := []struct{ dir, what string }{
		{opts.dir(), "the project"},
		{opts.Pages, "the pages"},
		{opts.public(), "`public/`"},
	}
	// What the build would delete.
	for _, source := range sources {
		if exists && under(info, source.dir) {
			return refuse("holds " + source.what + " (" + source.dir + "): the build empties its output directory")
		}
	}
	// What the build would read its own output from.
	for _, source := range sources[1:] {
		if inside, err := os.Stat(source.dir); err == nil && under(inside, out) {
			return refuse("is inside " + source.what + " (" + source.dir + ")")
		}
	}
	if !exists {
		return nil
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return refuse("cannot be read: " + err.Error())
	}
	// Empty, as empty leaves it: a `.git` is not the build's.
	if len(entries) == 0 || len(entries) == 1 && entries[0].Name() == ".git" {
		return nil
	}
	if report, err := os.Stat(filepath.Join(out, filepath.FromSlash(reportFile))); err != nil || !report.Mode().IsRegular() {
		return refuse("is not empty and is not an output of `reactogenic build` (it has no " + reportFile + "): the build empties its output directory — empty it yourself, or name another")
	}
	return nil
}

// under says whether dir is the directory of ancestor, or is below it. dir
// need not be there: what counts is the directory it would be made in.
func under(ancestor os.FileInfo, dir string) bool {
	for {
		if info, err := os.Stat(dir); err == nil && os.SameFile(ancestor, info) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// replace puts a new tree in the place of what the output directory holds,
// and makes the directory if it is not there. tree writes the new one into
// the directory it is given.
//
// The new tree is written beside the old one first — into a directory of
// its own inside out, so on the same file system — and only when all of it
// is there is the old one removed and the new one moved up. A build that
// cannot write — a file of `public/` that cannot be read, a full disk, two
// names the file system takes for one — leaves the last output as it was.
//
// The directory itself stays — it may be a mount, or a link — and so does a
// `.git` in it: an output that is a checkout of the branch it is published
// from. And from the first file written it holds `_rg/report.json`, the old
// one or the new: a build that is killed leaves a directory still known as
// the builder's, which the next build empties (CheckOut).
func replace(out string, report []byte, tree func(dir string) error) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	marked := false
	if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(reportFile))); err != nil {
		// Empty until now (CheckOut): the report marks it as the builder's.
		if err := writeFile(out, reportFile, report); err != nil {
			return err
		}
		marked = true
	}
	stage, err := os.MkdirTemp(out, ".rg-")
	if err == nil {
		err = tree(stage)
	}
	if err != nil {
		os.RemoveAll(stage)
		if marked {
			os.RemoveAll(filepath.Join(out, assets))
		}
		return err
	}

	// The old output goes: everything but `.git`, the new tree, and the
	// report, which the new one takes the place of in one step.
	entries := func(dir string, but ...string) ([]string, error) {
		all, err := os.ReadDir(dir)
		var names []string
		for _, entry := range all {
			if !slices.Contains(but, entry.Name()) {
				names = append(names, entry.Name())
			}
		}
		return names, err
	}
	remove := func(dir string, but ...string) error {
		names, err := entries(dir, but...)
		for _, name := range names {
			if err == nil {
				err = os.RemoveAll(filepath.Join(dir, name))
			}
		}
		return err
	}
	move := func(from, to string, but ...string) error {
		names, err := entries(from, but...)
		for _, name := range names {
			if err == nil {
				err = os.Rename(filepath.Join(from, name), filepath.Join(to, name))
			}
		}
		return err
	}
	if err := remove(out, ".git", filepath.Base(stage), assets); err != nil {
		return err
	}
	if err := remove(filepath.Join(out, assets), path.Base(reportFile)); err != nil {
		return err
	}
	if err := move(filepath.Join(stage, assets), filepath.Join(out, assets)); err != nil {
		return err
	}
	if err := move(stage, out, assets); err != nil {
		return err
	}
	return os.RemoveAll(stage)
}
