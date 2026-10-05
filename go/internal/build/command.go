package build

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/reactogenic/reactogenic/go/internal/check"
)

// Usage is the command's line of the CLI's usage text.
const Usage = `reactogenic build [-p tsconfig.json|dir] [--pages dir] [--out dir] [--base path]
                         [--inline auto|always|never] [--no-specialize] [--report]`

// Main is `reactogenic build` (builder.md): args are the command's own, cwd
// the working directory. It returns the exit status, as `check`'s: 0; 1 with
// an error among the diagnostics — or when the output cannot be written; 2
// on a usage error.
//
// Diagnostics go to stdout, as `check --pretty=false` prints them, then the
// byte report with `--report`, then one line that says what was written.
// What is not about the project — a usage error, a failed write — goes to
// stderr.
//
// The caller has registered the .rtsx transform (mapper.RegisterStrict).
func Main(args []string, cwd string, stdout, stderr io.Writer) int {
	opts, printReport, status := options(args, cwd, stderr)
	if status != 0 {
		return status
	}
	if err := CheckOut(opts); err != nil {
		fmt.Fprintf(stderr, "reactogenic build: %v\n", err)
		return 2
	}

	reports, bytes, err := Run(opts)
	here := filepath.ToSlash(real(cwd))
	check.Print(stdout, reports, here, false, func(string) (string, bool) { return "", false })
	if err != nil {
		fmt.Fprintf(stderr, "reactogenic build: %v\n", err)
		return 1
	}
	if bytes == nil {
		return 1
	}
	if printReport {
		bytes.Print(stdout)
	}
	where := opts.Out
	if rel, err := filepath.Rel(real(cwd), opts.Out); err == nil && filepath.IsLocal(rel) {
		where = rel
	}
	plural := "s"
	if len(bytes.Pages) == 1 {
		plural = ""
	}
	fmt.Fprintf(stdout, "%d page%s written to %s\n", len(bytes.Pages), plural, filepath.ToSlash(where))
	return 0
}

// options are the command's flags, resolved (builder.md, the table of
// flags): what Run is given. status is 2 on a usage error, which is said on
// stderr. Nothing is asked of the output directory here: that is CheckOut.
func options(args []string, cwd string, stderr io.Writer) (opts Options, printReport bool, status int) {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: "+Usage)
		flags.PrintDefaults()
	}
	project := flags.String("p", "", "tsconfig.json, or a directory holding one")
	pages := flags.String("pages", "", "the root of the routes (default: pages, next to the tsconfig)")
	out := flags.String("out", "", "the output directory, emptied when the build writes (default: dist, next to the tsconfig)")
	base := flags.String("base", "/", "the path the site is served under")
	inline := flags.String("inline", InlineAuto, "auto, always or never: a page's CSS and JS in the page, or as files")
	noSpecialize := flags.Bool("no-specialize", false, "the control: one unpruned CSS bundle and one script for the site")
	report := flags.Bool("report", false, "print the byte report")
	if err := flags.Parse(args); err != nil {
		return opts, false, 2
	}
	usage := func(format string, a ...any) (Options, bool, int) {
		fmt.Fprintf(stderr, "reactogenic build: "+format+"\n", a...)
		return opts, false, 2
	}
	if flags.NArg() > 0 {
		return usage("unexpected argument %q", flags.Arg(0))
	}
	absolute := func(path string) string {
		if filepath.IsAbs(path) {
			return filepath.Clean(path)
		}
		return filepath.Join(cwd, path)
	}

	opts = Options{Inline: *inline, NoSpecialize: *noSpecialize}
	switch opts.Inline {
	case InlineAuto, InlineAlways, InlineNever:
	default:
		return usage("--inline is auto, always or never, not %q", opts.Inline)
	}
	var ok bool
	if opts.Base, ok = NormalBase(*base); !ok {
		return usage("--base is the path the site is served under (/docs/), not %q", *base)
	}

	// The project, as for `check`. The program names its files from the
	// tsconfig's directory with its links resolved, and so does the build.
	config := filepath.Join(cwd, "tsconfig.json")
	if *project != "" {
		config = absolute(*project)
		if info, err := os.Stat(config); err == nil && info.IsDir() {
			config = filepath.Join(config, "tsconfig.json")
		}
	}
	if info, err := os.Stat(config); err != nil || info.IsDir() {
		return usage("no tsconfig at %s", config)
	}
	opts.Config = real(config)
	// `pages` and `public` may be links, each of its own: they are named as
	// they are found next to each other — `public` is beside the `pages`
	// that was named, not beside what that leads to — and from a directory
	// whose own links are resolved, which is how a tsconfig that lists
	// `pages` names the files in it.
	opts.Pages, opts.Out = filepath.Join(opts.dir(), "pages"), filepath.Join(opts.dir(), "dist")
	if *pages != "" {
		named := absolute(*pages)
		opts.Pages = filepath.Join(real(filepath.Dir(named)), filepath.Base(named))
	}
	opts.Public = filepath.Join(filepath.Dir(opts.Pages), "public")
	if *out != "" {
		opts.Out = real(absolute(*out))
	}
	return opts, *report, 0
}

// real is a path with its symbolic links resolved, as far as it exists: the
// output directory may not be there yet.
func real(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(real(parent), filepath.Base(path))
}
