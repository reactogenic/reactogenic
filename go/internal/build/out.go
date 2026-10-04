package build

import (
	"fmt"
	"os"
	"path/filepath"
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

// empty removes what is in the output directory, and makes it if it is not
// there. The directory itself stays — it may be a mount, or a link — and so
// does a `.git` in it: an output that is a checkout of the branch it is
// published from.
func empty(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(out, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
