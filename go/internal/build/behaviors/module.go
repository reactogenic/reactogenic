package behaviors

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// module is what has to be known of a behaviour module before a page is
// built with it: `Define` is fixed when a build starts, so its flags are
// read first.
type module struct {
	notFound bool          // the specifier does not resolve
	errors   []api.Message // it resolves and does not build: an import of its own, its syntax
	unread   string        // what the builder could not learn of it — its file, the flags of a file: internal

	file  string   // the file it resolves to, from the project directory, as esbuild's metafile names it
	files []string // file and every file it imports, transitively, sorted
	// flags are the module's and those of the modules it imports: the
	// identifiers of files that start with `RG_` and that nothing binds. nil
	// when it could not be read.
	flags map[string]bool
	// effects is the code that stays in a script which imports the module
	// and uses nothing of it — top-level side effects — one per file.
	effects []effect
}

type effect struct {
	file string // as esbuild's metafile names it
	runs string // the first statement of it that stays
	when string // "", or the flags it stays with: " with every flag on"
}

// read learns a module with a build of its own: an entry that imports it
// for nothing — `import "<specifier>"`. That resolves it, as the page's
// build will; its metafile lists the files it reaches; and with nothing
// used, whatever is left in the output ran at the top level of a module —
// the rule "no top-level side effects", checked where esbuild decides it,
// and without taking an annotation at its word: a package.json's
// `sideEffects: false` would have the whole module dropped here, and is not
// asked when a page mounts it.
//
// A module with flags is built twice more, with every flag on and with
// every flag off, and what is left of those is what counts: a page's build
// defines every flag, so there a constant made of one
// (`const DELAY = RG_SLOW ? 500 : 100`) is a constant, and a statement under
// one (`if (RG_X) …`) runs or does not.
func (o Options) read(spec string) *module {
	m := &module{}
	unused := func(define map[string]string) (string, metafile, []api.Message) {
		return o.esbuild("import "+quote(spec)+";\n", func(b *api.BuildOptions) {
			b.IgnoreAnnotations, b.MinifySyntax, b.Define = true, true, define
		})
	}
	kept, meta, errors := unused(nil)
	for _, e := range errors {
		if e.Location != nil && e.Location.File == stdin && strings.HasPrefix(e.Text, "Could not resolve") {
			m.notFound = true
			return m
		}
	}
	if m.errors = errors; len(errors) > 0 {
		return m
	}
	// A module is known by its file (plan.byFile): one that has none is not
	// taken for another.
	imports := meta.Inputs[stdin].Imports
	if len(imports) != 1 || imports[0].Path == "" {
		m.unread = "esbuild does not say which file it resolves to"
		return m
	}
	m.file, m.files = imports[0].Path, meta.files()
	flags := map[string]bool{}
	for _, file := range m.files {
		loader, ok := loaders[path.Ext(file)]
		if !ok {
			continue // not code: JSON, text
		}
		text, err := os.ReadFile(o.real(file)) // esbuild has just read it
		var names []string
		if err == nil {
			names, err = freeFlags(string(text), loader)
		}
		if err != nil {
			m.unread = fmt.Sprintf("the flags of `%s` cannot be read: %v", file, err)
			return m
		}
		for _, name := range names {
			flags[name] = true
		}
	}
	m.flags = flags
	if len(flags) == 0 {
		m.effects = effects(kept, meta, "")
		return m
	}
	for _, value := range []string{"true", "false"} {
		define := map[string]string{}
		for flag := range flags {
			define[flag] = value
		}
		kept, meta, errors := unused(define)
		if m.errors = errors; len(errors) > 0 {
			return m
		}
		when := " with every flag on"
		if value == "false" {
			when = " with every flag off"
		}
		for _, e := range effects(kept, meta, when) {
			if !slices.ContainsFunc(m.effects, func(had effect) bool { return had.file == e.file }) {
				m.effects = append(m.effects, e)
			}
		}
	}
	slices.SortStableFunc(m.effects, func(a, b effect) int { return strings.Compare(a.file, b.file) })
	return m
}

// effects are the files that left code in a bundle nothing of which is
// used, each with the first line of it: esbuild heads a file's code with
// `// <path>` in a bundle it does not minify.
func effects(bundle string, meta metafile, when string) []effect {
	var all []effect
	bytes := meta.bytes()
	for _, file := range meta.files() {
		if bytes[file] == 0 {
			continue
		}
		e := effect{file: file, when: when}
		if _, after, found := strings.Cut("\n"+bundle, "\n// "+file+"\n"); found {
			e.runs, _, _ = strings.Cut(after, "\n")
		}
		all = append(all, e)
	}
	return all
}

// loaders are the files that hold code, by extension: what a flag can be in.
var loaders = map[string]api.Loader{
	".ts": api.LoaderTS, ".mts": api.LoaderTS, ".cts": api.LoaderTS, ".tsx": api.LoaderTSX,
	".js": api.LoaderJS, ".mjs": api.LoaderJS, ".cjs": api.LoaderJS, ".jsx": api.LoaderJSX,
}

// A name that may be a flag: an identifier that starts with `RG_`. The
// classes are those of an identifier's characters, near enough — what is
// taken for a name here is only a candidate (freeFlags).
var identifier = regexp.MustCompile(`[\p{L}\p{Nl}\p{Mn}\p{Mc}\p{Nd}\p{Pc}$\x{200C}\x{200D}]+`)

// freeFlags are the flags of a source text: the identifiers that start with
// `RG_` and that nothing binds — a `declare const` binds nothing — which is
// exactly what `Define` replaces. So esbuild is asked: every such name of the
// text is defined as a marker, and the names whose marker is in what esbuild
// makes of the text are the flags. A name of a comment, of a string, of a
// property or of a constant of the module's own is not replaced.
func freeFlags(text string, loader api.Loader) ([]string, error) {
	if !strings.Contains(text, "RG_") {
		return nil, nil
	}
	var names []string
	define := map[string]string{}
	for _, n := range identifier.FindAllString(text, -1) {
		if strings.HasPrefix(n, "RG_") && define[n] == "" {
			define[n] = fmt.Sprintf("__reactogenic_flag_%d__", len(names))
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	r := api.Transform(text, api.TransformOptions{Loader: loader, Define: define, LogLevel: api.LogLevelSilent})
	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("%s", r.Errors[0].Text)
	}
	var flags []string
	for _, n := range names {
		if strings.Contains(string(r.Code), define[n]) {
			flags = append(flags, n)
		}
	}
	return flags, nil
}

// undefinedFlags are the flags a built script still reads: nothing defined
// them, and each is a ReferenceError when the script runs. The builder finds
// a flag by its name in a module's text; one written otherwise — with an
// escape, `RG_A` — it does not see until the script is built.
func undefinedFlags(js string) []string {
	flags, err := freeFlags(js, api.LoaderJS)
	if err != nil {
		return []string{err.Error()}
	}
	return flags
}

// Cache holds what was read of the behaviour modules of a site, so that a
// module mounted on every page is read once. It may be shared by concurrent
// builds; it does not notice a module that changes.
type Cache struct {
	mu      sync.Mutex
	modules map[string]*cached // by project directory and specifier
}

type cached struct {
	once sync.Once
	*module
}

func (o Options) modules(specs []string) []*module {
	mods := make([]*module, len(specs))
	for i, spec := range specs {
		if o.Cache == nil {
			mods[i] = o.read(spec)
			continue
		}
		o.Cache.mu.Lock()
		if o.Cache.modules == nil {
			o.Cache.modules = map[string]*cached{}
		}
		key := o.root + "\x00" + spec
		c := o.Cache.modules[key]
		if c == nil {
			c = &cached{}
			o.Cache.modules[key] = c
		}
		o.Cache.mu.Unlock()
		c.once.Do(func() { c.module = o.read(spec) })
		mods[i] = c.module
	}
	return mods
}

// sideEffects reports each file of a module that runs code when it is
// imported, at the file, with the code that stays.
func (o Options) sideEffects(page render.Page, m *module) []report.Report {
	var reports []report.Report
	for _, e := range m.effects {
		related := []report.Report{{Severity: report.Message, Message: "mounted on the page " + page.Pathname}}
		if e.runs != "" {
			related = append([]report.Report{{Severity: report.Message, Message: "what runs" + e.when + ": `" + e.runs + "`"}}, related...)
		}
		reports = append(reports, report.Report{
			File: o.path(e.file), Code: "mount-side-effect",
			Message: fmt.Sprintf("A behaviour module cannot run code when it is imported, only when it is mounted: `%s`", path.Base(e.file)),
			Related: related,
		})
	}
	return reports
}

// real is a file of a metafile where it is: esbuild names a file from the
// project directory with its links resolved.
func (o Options) real(file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(o.root, filepath.FromSlash(file))
}

// path is a file of a metafile as a report names it: under the project
// directory, from the directory as the caller names it — the names the other
// reports of the build are in; outside it — a package of the workspace,
// linked into node_modules — where it is: `..` from a link is not `..` from
// what it links to.
func (o Options) path(file string) string {
	if file == ".." || strings.HasPrefix(file, "../") || filepath.IsAbs(file) {
		return filepath.ToSlash(o.real(file))
	}
	return filepath.ToSlash(filepath.Join(o.Dir, filepath.FromSlash(file)))
}
