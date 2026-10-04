package behaviors

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	file  string   // the file it resolves to, from the project directory, as esbuild's metafile names it
	files []string // file and every file it imports, transitively, sorted
	// flags are the `RG_…` identifiers in the text of files: the module's
	// and those of the modules it imports. nil when it could not be read.
	flags map[string]bool
	// effects are the files that leave code in a script which imports the
	// module and uses nothing of it — top-level side effects — and kept is
	// that script.
	effects []string
	kept    string
}

// A flag is a bare identifier (builder.md, *Behaviours*). The text is not
// parsed: a name that only a comment mentions is defined too, to no effect.
var flagName = regexp.MustCompile(`\bRG_[A-Z0-9_]+\b`)

// read learns a module with one build of its own: an entry that imports it
// for nothing — `import "<specifier>"`. That resolves it, as the page's
// build will; its metafile lists the files it reaches; and with nothing
// used, whatever is left in the output ran at the top level of a module —
// the rule "no top-level side effects", checked where esbuild decides it:
// with the constant folding of the page's build, and without taking an
// annotation at its word — a package.json's `sideEffects: false` would have
// the whole module dropped here, and is not asked when a page mounts it.
func (o Options) read(spec string) *module {
	m := &module{}
	kept, meta, errors := o.esbuild("import "+quote(spec)+";\n", func(b *api.BuildOptions) { b.IgnoreAnnotations, b.MinifySyntax = true, true })
	for _, e := range errors {
		if e.Location != nil && e.Location.File == stdin && strings.HasPrefix(e.Text, "Could not resolve") {
			m.notFound = true
			return m
		}
	}
	if m.errors = errors; len(errors) > 0 {
		return m
	}
	if imports := meta.Inputs[stdin].Imports; len(imports) == 1 {
		m.file = imports[0].Path
	}
	m.files, m.flags, m.kept = meta.files(), map[string]bool{}, kept
	bytes := meta.bytes()
	for _, file := range m.files {
		if bytes[file] > 0 {
			m.effects = append(m.effects, file)
		}
		text, _ := os.ReadFile(filepath.Join(o.Dir, file)) // esbuild has just read it
		for _, name := range flagName.FindAll(text, -1) {
			m.flags[string(name)] = true
		}
	}
	return m
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
		key := o.Dir + "\x00" + spec
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
	for _, file := range m.effects {
		related := []report.Report{{Severity: report.Message, Message: "mounted on the page " + page.Pathname}}
		if code := keptOf(m.kept, file); code != "" {
			related = append([]report.Report{{Severity: report.Message, Message: "what runs: `" + code + "`"}}, related...)
		}
		reports = append(reports, report.Report{
			File: filepath.ToSlash(filepath.Join(o.Dir, file)), Code: "mount-side-effect",
			Message: fmt.Sprintf("A behaviour module cannot run code when it is imported, only when it is mounted: `%s`", filepath.Base(file)),
			Related: related,
		})
	}
	return reports
}

// keptOf is the first line of file's code in an unminified bundle: esbuild
// heads each file's with `// <path>`.
func keptOf(bundle, file string) string {
	_, after, found := strings.Cut("\n"+bundle, "\n// "+file+"\n")
	if !found {
		return ""
	}
	line, _, _ := strings.Cut(after, "\n")
	return line
}
