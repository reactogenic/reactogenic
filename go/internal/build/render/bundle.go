package render

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// The builder's own modules of the render bundle live in a namespace of
// their own: `reactogenic:sandbox`, `reactogenic:jsx`, `reactogenic:react`,
// the generated `reactogenic:entry`. `reactogenic:real/<specifier>` is the
// project's React itself, for the two modules that wrap it.
const (
	namespace   = "reactogenic"
	jsxModule   = namespace + ":jsx"   // React's JSX runtime in the bundle, and its jsxImportSource (builder.md, *The record*)
	reactModule = namespace + ":react" // `react` in the bundle (builder.md, *Shell code in phase 2*)
	real        = "real/"

	// React's static renderer, in the project's react-dom: the build that
	// needs nothing of its host — the public `react-dom/server` entry also
	// loads the streaming renderer, which needs MessageChannel and
	// TextEncoder (research/evaluation.md, Verification, claim 9).
	staticRenderer = "cjs/react-dom-server-legacy.browser.production.js"
)

var (
	//go:embed js/sandbox.js
	sandboxJS string
	//go:embed js/jsx.js
	jsxJS string
	//go:embed js/react.js
	reactJS string
)

// bundle is the render bundle (plan.md, RGP2-010): every variant and React's
// static renderer as one script that defines `__reactogenic_render`.
type bundle struct {
	code      string
	sites     *sourceMap
	files     []*rtsx.SourceFile // the program's files in it: the pages' module closure, by name
	importers map[string][]edge  // who imports a module of the bundle, by its name as the source map has it
}

// edge is one import of the bundle: the importing module, and the specifier
// as it was written.
type edge struct {
	importer, specifier string
}

// variant is how a bundle differs from the one the builder executes: the
// differential test's oracle is the same pages rendered by the public
// `react-dom/server` and by React itself, in Node.
type variant struct {
	renderer string // a module specifier; "": staticRenderer of the project's react-dom
	plain    bool   // nothing of the builder's: React's JSX runtime and `react`, no sandbox, no record
	platform api.Platform
}

// entry is the generated entry (builder.md, *What shell code can ask the
// builder*; plan.md, *The build-time protocol*): `__reactogenic_render`
// renders one document — a variant of a route, named by its path
// (Route.Path) — and returns { html, mounts, components, classes }, or throws; while
// it renders, `__reactogenic_build` is the page's. `pathname` there is the
// route's, in every variant of it (*Routes*).
func entry(routes []Route, v variant) string {
	var b strings.Builder
	if v.plain {
		b.WriteString("import { createElement as root } from \"react\";\n")
		b.WriteString("const start = () => {}, components = () => ({}), owners = () => ({}), swallowed = () => null, calling = () => \"\";\n")
	} else {
		b.WriteString("import \"" + namespace + ":sandbox\";\n")
		b.WriteString("import { calling, components, owners, root, start, swallowed } from \"" + jsxModule + "\";\n")
	}
	b.WriteString("import { renderToStaticMarkup } from \"" + namespace + ":renderer\";\n")
	for i, route := range routes {
		// The namespace, not the default export: a page without one is
		// page-no-default, not a failed bundle.
		fmt.Fprintf(&b, "import * as page%d from %s;\n", i, strconv.Quote(route.File))
	}
	b.WriteString("const pages = {")
	for i, route := range routes {
		fmt.Fprintf(&b, " %s: [page%d, %s],", strconv.Quote(route.Path()), i, strconv.Quote(route.Pathname))
	}
	b.WriteString(` };

function coded(name, message) {
  const error = new Error(message);
  error.name = name;
  return error;
}

// What a value that is not JSON is, for the message.
function kind(value) {
  if (value === undefined) return "undefined";
  if (typeof value === "number") return String(value);
  if (typeof value !== "object") return "a " + typeof value;
  const type = Object.getPrototypeOf(value)?.constructor?.name;
  return type ? "an instance of ` + "`" + `" + type + "` + "`" + `" : "an object";
}

// A mount's data as the builder writes it into the page's script: JSON —
// a plain object of JSON values, and nothing a literal cannot say — with
// its keys sorted, so that the same data is the same text however a use
// site wrote it (builder.md, *Behaviours*, mount-data). A key whose value
// is undefined is left out, as JSON.stringify leaves it.
function json(value, at, within, wrong) {
  switch (typeof value) {
    case "string":
    case "boolean":
      return JSON.stringify(value);
    case "number":
      if (Number.isFinite(value)) return JSON.stringify(value);
      break;
    case "object": {
      if (value === null) return "null";
      if (within.includes(value)) throw wrong(at, "an object that holds itself");
      const inside = [...within, value];
      if (Array.isArray(value)) {
        const items = [];
        for (let i = 0; i < value.length; i++) items.push(json(value[i], at + "[" + i + "]", inside, wrong));
        return "[" + items.join(",") + "]";
      }
      const proto = Object.getPrototypeOf(value);
      if (proto !== Object.prototype && proto !== null) break;
      if (Object.getOwnPropertySymbols(value).length > 0) throw wrong(at, "an object with a symbol for a key");
      const members = [];
      for (const key of Object.keys(value).sort()) {
        const member = /^[A-Za-z_$][\w$]*$/.test(key) ? at + "." + key : at + "[" + JSON.stringify(key) + "]";
        // In a literal, "__proto__" is the object's prototype, not a key.
        if (key === "__proto__") throw wrong(member, "not a key a literal can have");
        if (value[key] !== undefined) members.push(JSON.stringify(key) + ":" + json(value[key], member, inside, wrong));
      }
      return "{" + members.join(",") + "}";
    }
  }
  throw wrong(at, kind(value));
}

function render(path) {
  start();
  if (!Object.hasOwn(pages, path)) throw new Error("no page at " + path);
  const [module, pathname] = pages[path];
  const page = module.default;
  if (typeof page !== "function") throw coded("page-no-default", "The page has no default export that is a component");
  const mounts = [], ids = {}, classes = [], resolved = new Map();
  globalThis.__reactogenic_build = {
    pathname,
    id(prefix) {
      const p = prefix ? String(prefix) : "r";
      // The counter follows the prefix: the first "d1" and the eleventh "d"
      // would be one id.
      if (/[0-9]$/.test(p)) throw coded("shell-error", "useShellId(" + JSON.stringify(p) + "): a prefix cannot end in a digit: the counter follows it");
      ids[p] = (ids[p] || 0) + 1;
      return p + ids[p];
    },
    mount(module, id, flags, data) {
      const mount = { module: String(module), id: id == null ? "" : String(id) };
      if (flags != null) {
        mount.flags = {};
        for (const name of Object.keys(flags)) mount.flags[name] = Boolean(flags[name]);
      }
      if (data != null) {
        const said = "` + "`" + `mount(" + JSON.stringify(mount.module) + ")` + "`" + `: ";
        // A behaviour of the page is called once, for all who mounted it:
        // there is no use site to hand it anything of.
        if (mount.id === "") throw coded("mount-data", said + "a behaviour of the page — mounted without an id — takes no data");
        if (typeof data !== "object" || Array.isArray(data)) throw coded("mount-data", said + "the data is not a plain object of JSON values: it is " + (Array.isArray(data) ? "an array" : kind(data)));
        mount.data = json(data, "data", [], (at, what) => coded("mount-data", said + "the data is not JSON: ` + "`" + `" + at + "` + "`" + ` is " + what));
      }
      mounts.push(mount);
    },
    // What a variants() call resolved (builder.md, *What shell code can ask
    // the builder*): each class once, in the order first resolved, with the
    // components that called. For the report.
    classes(names) {
      const by = calling();
      for (const each of Array.isArray(names) ? names : []) {
        const name = String(each);
        let entry = resolved.get(name);
        if (entry === undefined) {
          entry = { name, by: [] };
          resolved.set(name, entry);
          classes.push(entry);
        }
        if (by !== "" && !entry.by.includes(by)) entry.by.push(by);
      }
    },
  };
  try {
    const html = renderToStaticMarkup(root(page));
    // The render ended, and a component had thrown: a boundary rendered its
    // fallback instead. No page is built around a swallowed error.
    const lost = swallowed();
    if (lost !== null) throw lost;
    return { html, mounts, components: components(), classes };
  } finally {
    delete globalThis.__reactogenic_build;
  }
}

// For a host that cannot read an exception's properties: the page, or what
// was thrown and the components it passed through, as JSON.
function renderJSON(path) {
  try {
    return JSON.stringify({ page: render(path) });
  } catch (thrown) {
    const isError = thrown instanceof Error;
    return JSON.stringify({
      error: {
        name: isError ? String(thrown.name) : "",
        message: isError ? String(thrown.message) : String(thrown),
        stack: isError ? String(thrown.stack) : "",
        ...owners(thrown),
      },
    });
  }
}

globalThis.__reactogenic_render = render;
globalThis.__reactogenic_render_json = renderJSON;
`)
	return b.String()
}

// direct marks a resolution the plugin asks of esbuild for itself: the
// plugin's own callbacks leave it alone.
type direct struct{}

// build makes the render bundle of routes. A report is an error of the
// bundle itself — something it cannot resolve or parse — at its position in
// the author's text: no bundle then.
func build(program *rtsx.Program, routes []Route, dir string, v variant) (*bundle, []report.Report) {
	var (
		mu       sync.Mutex // esbuild calls the plugin from several goroutines
		loaded   []*rtsx.SourceFile
		missing  = map[string]bool{} // "react", "react-dom": not installed in dir
		renderer string              // the file of React's static renderer, once resolved
		reacts   []string            // the directories of the project's react and react-dom
		once     sync.Once
	)
	plugin := api.Plugin{Name: namespace, Setup: func(b api.PluginBuild) {
		// project resolves one of React's packages as the project does.
		project := func(specifier string) api.ResolveResult {
			resolved := b.Resolve(specifier, api.ResolveOptions{ResolveDir: dir, Kind: api.ResolveJSImportStatement, PluginData: direct{}})
			if len(resolved.Errors) > 0 {
				name, _, _ := strings.Cut(specifier, "/")
				mu.Lock()
				missing[name] = true
				mu.Unlock()
			}
			return resolved
		}
		// reactsOwn: a module of React itself. React's own code gets React.
		reactsOwn := func(importer string) bool {
			once.Do(func() {
				for _, name := range []string{"react", "react-dom"} {
					if resolved := project(name + "/package.json"); len(resolved.Errors) == 0 {
						reacts = append(reacts, filepath.Dir(resolved.Path)+string(filepath.Separator))
					}
				}
			})
			return slices.ContainsFunc(reacts, func(dir string) bool { return strings.HasPrefix(importer, dir) })
		}
		b.OnResolve(api.OnResolveOptions{Filter: `^` + namespace + `:`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			name := strings.TrimPrefix(args.Path, namespace+":")
			switch {
			case name == "jsx/jsx-runtime" || name == "jsx/jsx-dev-runtime":
				name = "jsx"
			case name == "renderer":
				result, err := resolveRenderer(project, dir, v)
				mu.Lock()
				renderer = result.Path
				mu.Unlock()
				return result, err
			case strings.HasPrefix(name, real):
				resolved := project(strings.TrimPrefix(name, real))
				return api.OnResolveResult{Path: resolved.Path, Errors: resolved.Errors}, nil
			}
			return api.OnResolveResult{Path: name, Namespace: namespace}, nil
		})
		// A stylesheet is the CSS stage's (builder.md, *CSS*): here it is an
		// empty module.
		b.OnResolve(api.OnResolveOptions{Filter: `\.css$`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: "css", Namespace: namespace}, nil
		})
		b.OnResolve(api.OnResolveOptions{Filter: `.*`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if _, own := args.PluginData.(direct); own {
				return api.OnResolveResult{}, nil
			}
			// React is the builder's for every module of the bundle —
			// compiled packages, a file with a `@jsxImportSource` pragma,
			// another JSX runtime that ends in React's: shell rule S1 and
			// the record do not depend on who made an element or how a hook
			// was reached (builder.md, *Shell code in phase 2*).
			if !v.plain && !reactsOwn(args.Importer) {
				switch args.Path {
				case "react/jsx-runtime", "react/jsx-dev-runtime":
					return api.OnResolveResult{Path: "jsx", Namespace: namespace}, nil
				case "react":
					return api.OnResolveResult{Path: "react", Namespace: namespace}, nil
				}
			}
			// One resolver (builder.md, *The pipeline*): an import that a
			// program file makes goes where the program resolved it — what is
			// built is what was checked. What the program holds no code for (a
			// package's JavaScript, behind its declarations) is esbuild's.
			if args.Namespace == namespace {
				if Source(program, args.Path) != nil { // the entry's import of a page
					return api.OnResolveResult{Path: filepath.FromSlash(args.Path)}, nil
				}
				return api.OnResolveResult{}, nil
			}
			return api.OnResolveResult{Path: Resolve(program, args.Importer, args.Path)}, nil
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: namespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			var text string
			switch args.Path {
			case "entry":
				text = entry(routes, v)
			case "sandbox":
				text = sandboxJS
			case "jsx":
				text = jsxJS
			case "react":
				text = reactJS
			case "css":
			default:
				return api.OnLoadResult{}, fmt.Errorf("no module %s:%s", namespace, args.Path)
			}
			return api.OnLoadResult{Contents: &text, Loader: api.LoaderJS, ResolveDir: dir}, nil
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			mu.Lock()
			isRenderer := !v.plain && args.Path == renderer
			mu.Unlock()
			if isRenderer {
				text, err := hooked(args.Path)
				return api.OnLoadResult{Contents: &text, Loader: api.LoaderJS, ResolveDir: filepath.Dir(args.Path)}, err
			}
			file, result, err := Load(program, args.Path)
			if file != nil && err == nil {
				mu.Lock()
				loaded = append(loaded, file)
				mu.Unlock()
			}
			return result, err
		})
	}}

	jsx := api.BuildOptions{JSX: api.JSXAutomatic, JSXImportSource: jsxModule, JSXDev: true}
	if v.plain {
		jsx = api.BuildOptions{JSX: api.JSXAutomatic, JSXImportSource: "react"}
	}
	outfile := filepath.Join(dir, "render.js") // never written: it names the output, and the map's sources are relative to it
	result := api.Build(api.BuildOptions{
		EntryPoints:   []string{namespace + ":entry"},
		Bundle:        true,
		Write:         false,
		Outfile:       outfile,
		AbsWorkingDir: dir,
		Format:        api.FormatIIFE,
		Platform:      v.platform,
		Target:        api.ES2023, // the engine's level
		// A tsconfig would override the JSX options below, for the files
		// esbuild resolves by itself: none is read.
		TsconfigRaw:     "{}",
		JSX:             jsx.JSX,
		JSXImportSource: jsx.JSXImportSource,
		JSXDev:          jsx.JSXDev,
		AbsPaths:        api.CodeAbsPath, // the file names of jsxDEV's `source`; a message keeps its short paths
		Define:          map[string]string{"process.env.NODE_ENV": `"production"`},
		KeepNames:       true, // a component's name is its name in the record, whatever the bundle calls it
		Sourcemap:       api.SourceMapExternal,
		SourcesContent:  api.SourcesContentExclude, // the program has the texts
		Metafile:        true,                      // who imports what: where a package that throws as it loads is reported
		LogLevel:        api.LogLevelSilent,
		Plugins:         []api.Plugin{plugin},
	})
	if len(missing) > 0 {
		// One report, whatever the modules of the builder's that import it.
		names := slices.Sorted(maps.Keys(missing))
		verb := "is"
		if len(names) > 1 {
			verb = "are"
		}
		return nil, []report.Report{{Code: "render-bundle", Message: fmt.Sprintf("%s %s not installed in %s: the builder renders pages with the project's React", strings.Join(names, " and "), verb, dir)}}
	}
	if len(result.Errors) > 0 {
		var reports []report.Report
		for _, message := range result.Errors {
			reports = append(reports, bundleError(program, dir, message))
		}
		return nil, reports
	}
	out := &bundle{files: loaded, importers: importers(result.Metafile, dir)}
	slices.SortFunc(out.files, func(a, b *rtsx.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	for _, file := range result.OutputFiles {
		if strings.HasSuffix(file.Path, ".map") {
			sites, err := parseSourceMap(file.Contents)
			if err != nil {
				return nil, []report.Report{{Code: "render-bundle", Message: err.Error()}}
			}
			for i, source := range sites.sources {
				sites.sources[i] = absolute(dir, source)
			}
			out.sites = sites
		} else {
			out.code = string(file.Contents)
		}
	}
	return out, nil
}

// Source is the program's module at an esbuild path: a file whose text is
// code. nil for a file the program does not hold, and for a declaration
// file, which stands for code that only esbuild can find.
//
// Source, Resolve and Load are the one resolver of builder.md (*The
// pipeline*) for an esbuild plugin: the render bundle's, and the CSS
// build's, which reads the same modules for the stylesheets they import.
func Source(program *rtsx.Program, path string) *rtsx.SourceFile {
	if file := program.GetSourceFile(filepath.ToSlash(path)); file != nil && !file.IsDeclarationFile {
		return file
	}
	return nil
}

// Resolve is where an import goes that a program file makes: where the
// program resolved it — what is built is what was checked. "": the importer
// is no program file, or the program holds no code for the import (a
// package's JavaScript, behind its declarations; a stylesheet; an asset) —
// it is esbuild's to resolve.
func Resolve(program *rtsx.Program, importer, specifier string) string {
	file := Source(program, importer)
	if file == nil {
		return ""
	}
	for _, written := range file.Imports() {
		if written.Text() != specifier {
			continue
		}
		resolved := program.GetResolvedModuleFromModuleSpecifier(file, written)
		if resolved == nil || resolved.ResolvedFileName == "" {
			continue
		}
		if Source(program, resolved.ResolvedFileName) != nil {
			return filepath.FromSlash(resolved.ResolvedFileName)
		}
		// Declarations of the project's own — not a package's, which
		// esbuild finds as Node does: the JavaScript is beside them,
		// wherever the specifier (a `paths` alias) pointed.
		if code := declared(resolved.ResolvedFileName); code != "" && !resolved.IsExternalLibraryImport {
			return filepath.FromSlash(code)
		}
	}
	return ""
}

// Load is a program file as esbuild loads it: its text is the program's —
// for an .rtsx module, its emitted TSX. A nil file and an empty result: the
// path is not the program's, or has no loader, and esbuild reads it.
func Load(program *rtsx.Program, path string) (*rtsx.SourceFile, api.OnLoadResult, error) {
	file := Source(program, path)
	loader, known := loaders[strings.ToLower(filepath.Ext(path))]
	if file == nil || !known {
		return nil, api.OnLoadResult{}, nil
	}
	if f, mapped := mapper.Of(file); mapped && f.Stopped {
		// Its text is its source, not TSX: `build` stops on the file's
		// diagnostics before it gets here.
		return file, api.OnLoadResult{}, fmt.Errorf("%s was not compiled: it has errors", path)
	}
	text := file.Text()
	return file, api.OnLoadResult{Contents: &text, Loader: loader, ResolveDir: filepath.Dir(path)}, nil
}

// loaders are esbuild's loaders for the program's files, by extension.
var loaders = map[string]api.Loader{
	".rtsx": api.LoaderTSX, ".tsx": api.LoaderTSX,
	".ts": api.LoaderTS, ".mts": api.LoaderTS, ".cts": api.LoaderTS,
	".jsx": api.LoaderJSX,
	".js":  api.LoaderJS, ".mjs": api.LoaderJS, ".cjs": api.LoaderJS,
	".json": api.LoaderJSON,
}

// declarations are the extensions of a declaration file, and those of the
// JavaScript it declares.
var declarations = []struct {
	extension string
	code      []string
}{
	{".d.ts", []string{".js", ".jsx"}},
	{".d.mts", []string{".mjs"}},
	{".d.cts", []string{".cjs"}},
}

// declared is the JavaScript beside a declaration file: `legacy.js` of
// `legacy.d.ts`. "": there is none.
func declared(name string) string {
	for _, d := range declarations {
		if base, ok := strings.CutSuffix(name, d.extension); ok {
			for _, extension := range d.code {
				if info, err := os.Stat(filepath.FromSlash(base + extension)); err == nil && !info.IsDir() {
					return base + extension
				}
			}
		}
	}
	return ""
}

// resolveRenderer finds React's static renderer in the react-dom that the
// project resolves.
func resolveRenderer(project func(string) api.ResolveResult, dir string, v variant) (api.OnResolveResult, error) {
	if v.renderer != "" {
		resolved := project(v.renderer)
		return api.OnResolveResult{Path: resolved.Path, Errors: resolved.Errors}, nil
	}
	resolved := project("react-dom/package.json")
	if len(resolved.Errors) > 0 {
		return api.OnResolveResult{Errors: resolved.Errors}, nil
	}
	path := filepath.Join(filepath.Dir(resolved.Path), filepath.FromSlash(staticRenderer))
	if _, err := os.Stat(path); err != nil {
		return api.OnResolveResult{}, fmt.Errorf("this react-dom has no static renderer the builder knows (%s)", path)
	}
	return api.OnResolveResult{Path: path}, nil
}

// componentCall is where React's static renderer calls a function component
// (`renderWithHooks`, twice: the call, and the call again after a state
// update during the render).
var componentCall = regexp.MustCompile(`\bComponent\(props, secondArg\)`)

// hooked returns the text of React's static renderer with the one change the
// builder makes to it: a function component is called through the builder's
// runtime (js/jsx.js, call) — which is how a component is counted and an
// exception gets its component stack while every element stays React's own,
// its `type` the component (builder.md, *The record*). A renderer that calls
// its components in another way is an error, not a page without a record.
func hooked(path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !componentCall.Match(text) {
		return "", fmt.Errorf("this react-dom's static renderer is not one the builder knows: it does not call a component as `Component(props, secondArg)` (%s)", path)
	}
	// On one line, and the hook's binding after the last: the file's
	// positions stay what they are.
	return componentCall.ReplaceAllLiteralString(string(text), "__reactogenic_call(Component, props, secondArg)") +
		"\nvar __reactogenic_call = require(\"" + jsxModule + "\").call;\n", nil
}

// absolute makes a name of the source map or the metafile what the program
// calls the file.
func absolute(dir, name string) string {
	if strings.HasPrefix(name, namespace+":") || filepath.IsAbs(name) {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(name)))
}

// importers reads the bundle's imports from esbuild's metafile, by imported
// module.
func importers(metafile, dir string) map[string][]edge {
	var meta struct {
		Inputs map[string]struct {
			Imports []struct {
				Path     string `json:"path"`
				Original string `json:"original"`
			} `json:"imports"`
		} `json:"inputs"`
	}
	if json.Unmarshal([]byte(metafile), &meta) != nil {
		return nil
	}
	out := map[string][]edge{}
	for importer, input := range meta.Inputs {
		for _, imported := range input.Imports {
			name := absolute(dir, imported.Path)
			out[name] = append(out[name], edge{absolute(dir, importer), imported.Original})
		}
	}
	for _, edges := range out { // the metafile is a map: the same answer every time
		slices.SortFunc(edges, func(a, b edge) int { return strings.Compare(a.importer, b.importer) })
	}
	return out
}

// bundleError reports an error of esbuild's: render-bundle, where the author
// wrote what it is about. An error in a module of the builder's own has no
// file.
func bundleError(program *rtsx.Program, dir string, message api.Message) report.Report {
	return Message(program, dir, "render-bundle", message)
}

// Message is a message of an esbuild build that reads the program's files
// (Load), as a report where the author wrote what it is about: in a program
// file esbuild's position is one of the program's text — for an .rtsx
// module, of its emitted TSX — and the report's is the source's. dir is the
// build's working directory, which esbuild names files from. A message about
// a module of the builder's own has no file.
func Message(program *rtsx.Program, dir, code string, message api.Message) report.Report {
	r := report.Report{Code: code, Message: message.Text}
	location := message.Location
	if location == nil || strings.HasPrefix(location.File, namespace+":") || location.Namespace != "" && location.Namespace != "file" {
		return r
	}
	name := location.File
	if !filepath.IsAbs(name) {
		name = filepath.Join(dir, name)
	}
	name = filepath.ToSlash(name)
	r.File, r.Line, r.Col = name, location.Line, location.Column+1
	if file := program.GetSourceFile(name); file != nil {
		// esbuild read the program's text: a line, and a column in bytes.
		pos := offsetAt(file.Text(), location.Line-1, 0) + location.Column
		r.File, r.Span, r.Line, r.Col = position(file, emit.Span{Pos: pos, End: pos + location.Length})
	}
	return r
}
