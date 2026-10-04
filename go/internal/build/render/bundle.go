package render

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
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
// their own: `reactogenic:sandbox`, `reactogenic:jsx`, the generated
// `reactogenic:entry`.
const (
	namespace = "reactogenic"
	jsxSource = namespace + ":jsx" // the bundle's jsxImportSource (builder.md, *The record*)

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
)

// bundle is the render bundle (plan.md, RGP2-010): every page and React's
// static renderer as one script that defines `__reactogenic_render`.
type bundle struct {
	code  string
	sites *sourceMap
	files []*rtsx.SourceFile // the program's files in it: the pages' module closure, by name
}

// variant is how a bundle differs from the one the builder executes: the
// differential test's oracle is the same pages rendered by the public
// `react-dom/server` and React's own JSX runtime, in Node.
type variant struct {
	renderer string // a module specifier; "": staticRenderer of the project's react-dom
	plainJSX bool   // React's JSX runtime, no record
	platform api.Platform
}

// entry is the generated entry (builder.md, *What shell code can ask the
// builder*; plan.md, *The build-time protocol*): `__reactogenic_render`
// renders one page and returns { html, mounts, components }, or throws;
// while it renders, `__reactogenic_build` is the page's.
func entry(routes []Route, v variant) string {
	var b strings.Builder
	b.WriteString("import \"" + namespace + ":sandbox\";\n")
	b.WriteString("import { renderToStaticMarkup } from \"" + namespace + ":renderer\";\n")
	if v.plainJSX {
		b.WriteString("import { createElement } from \"react\";\n")
		b.WriteString("const jsx = createElement, start = () => {}, components = () => ({}), owners = () => ({});\n")
	} else {
		b.WriteString("import { components, jsx, owners, start } from \"" + jsxSource + "\";\n")
	}
	for i, route := range routes {
		// The namespace, not the default export: a page without one is
		// page-no-default, not a failed bundle.
		fmt.Fprintf(&b, "import * as page%d from %s;\n", i, strconv.Quote(route.File))
	}
	b.WriteString("const pages = {")
	for i, route := range routes {
		fmt.Fprintf(&b, " %s: page%d,", strconv.Quote(route.Pathname), i)
	}
	b.WriteString(` };

function coded(name, message) {
  const error = new Error(message);
  error.name = name;
  return error;
}

function render(pathname) {
  start();
  if (!Object.hasOwn(pages, pathname)) throw new Error("no page at " + pathname);
  const page = pages[pathname].default;
  if (typeof page !== "function") throw coded("page-no-default", "The page has no default export that is a component");
  const mounts = [], ids = {};
  globalThis.__reactogenic_build = {
    pathname,
    id(prefix) {
      const p = prefix ? String(prefix) : "r";
      ids[p] = (ids[p] || 0) + 1;
      return p + ids[p];
    },
    mount(module, id, flags) {
      const mount = { module: String(module), id: id == null ? "" : String(id) };
      if (flags != null) {
        mount.flags = {};
        for (const name of Object.keys(flags)) mount.flags[name] = Boolean(flags[name]);
      }
      mounts.push(mount);
    },
  };
  try {
    return { html: renderToStaticMarkup(jsx(page, {})), mounts, components: components() };
  } finally {
    delete globalThis.__reactogenic_build;
  }
}

// For a host that cannot read an exception's properties: the page, or what
// was thrown and the components it passed through, as JSON.
function renderJSON(pathname) {
  try {
    return JSON.stringify({ page: render(pathname) });
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

// build makes the render bundle of routes. A report is an error of the
// bundle itself — something it cannot resolve or parse — at its position in
// the author's text: no bundle then.
func build(program *rtsx.Program, routes []Route, dir string, v variant) (*bundle, []report.Report) {
	var (
		mu     sync.Mutex // esbuild calls the plugin from several goroutines
		loaded []*rtsx.SourceFile
	)
	// source is the program's module at an esbuild path: a file whose text
	// is code. A declaration file stands for code that only esbuild can find.
	source := func(path string) *rtsx.SourceFile {
		if file := program.GetSourceFile(filepath.ToSlash(path)); file != nil && !file.IsDeclarationFile {
			return file
		}
		return nil
	}
	plugin := api.Plugin{Name: namespace, Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `^` + namespace + `:`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			name := strings.TrimPrefix(args.Path, namespace+":")
			switch name {
			case "jsx/jsx-runtime", "jsx/jsx-dev-runtime":
				name = "jsx"
			case "renderer":
				return resolveRenderer(b, dir, v)
			}
			return api.OnResolveResult{Path: name, Namespace: namespace}, nil
		})
		// A stylesheet is the CSS stage's (builder.md, *CSS*): here it is an
		// empty module.
		b.OnResolve(api.OnResolveOptions{Filter: `\.css$`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: "css", Namespace: namespace}, nil
		})
		// One resolver (builder.md, *The pipeline*): an import that a
		// program file makes goes where the program resolved it — what is
		// built is what was checked. What the program holds no code for (a
		// package's JavaScript, behind its declarations) is esbuild's.
		b.OnResolve(api.OnResolveOptions{Filter: `.*`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if args.Namespace == namespace {
				if source(args.Path) != nil { // the entry's import of a page
					return api.OnResolveResult{Path: filepath.FromSlash(args.Path)}, nil
				}
				return api.OnResolveResult{}, nil
			}
			importer := source(args.Importer)
			if importer == nil {
				return api.OnResolveResult{}, nil
			}
			for _, specifier := range importer.Imports() {
				if specifier.Text() != args.Path {
					continue
				}
				resolved := program.GetResolvedModuleFromModuleSpecifier(importer, specifier)
				if resolved != nil && resolved.ResolvedFileName != "" && source(resolved.ResolvedFileName) != nil {
					return api.OnResolveResult{Path: filepath.FromSlash(resolved.ResolvedFileName)}, nil
				}
			}
			return api.OnResolveResult{}, nil
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
			case "css":
			default:
				return api.OnLoadResult{}, fmt.Errorf("no module %s:%s", namespace, args.Path)
			}
			// The project's React: `react` is resolved from the project.
			return api.OnLoadResult{Contents: &text, Loader: api.LoaderJS, ResolveDir: dir}, nil
		})
		// A program file's text is the program's: for an .rtsx module, its
		// emitted TSX.
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			file := source(args.Path)
			loader, known := loaders[strings.ToLower(filepath.Ext(args.Path))]
			if file == nil || !known {
				return api.OnLoadResult{}, nil
			}
			if f, mapped := mapper.Of(file); mapped && f.Stopped {
				// Its text is its source, not TSX: `build` stops on the
				// file's diagnostics before it gets here.
				return api.OnLoadResult{}, fmt.Errorf("%s was not compiled: it has errors", args.Path)
			}
			mu.Lock()
			loaded = append(loaded, file)
			mu.Unlock()
			text := file.Text()
			return api.OnLoadResult{Contents: &text, Loader: loader, ResolveDir: filepath.Dir(args.Path)}, nil
		})
	}}

	jsx := api.BuildOptions{JSX: api.JSXAutomatic, JSXImportSource: jsxSource, JSXDev: true}
	if v.plainJSX {
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
		LogLevel:        api.LogLevelSilent,
		Plugins:         []api.Plugin{plugin},
	})
	if len(result.Errors) > 0 {
		var reports []report.Report
		for _, message := range result.Errors {
			reports = append(reports, bundleError(program, dir, message))
		}
		return nil, reports
	}
	out := &bundle{files: loaded}
	slices.SortFunc(out.files, func(a, b *rtsx.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	for _, file := range result.OutputFiles {
		if strings.HasSuffix(file.Path, ".map") {
			sites, err := parseSourceMap(file.Contents)
			if err != nil {
				return nil, []report.Report{{Code: "render-bundle", Message: err.Error()}}
			}
			sites.resolve(dir)
			out.sites = sites
		} else {
			out.code = string(file.Contents)
		}
	}
	return out, nil
}

// loaders are esbuild's loaders for the program's files, by extension.
var loaders = map[string]api.Loader{
	".rtsx": api.LoaderTSX, ".tsx": api.LoaderTSX,
	".ts": api.LoaderTS, ".mts": api.LoaderTS, ".cts": api.LoaderTS,
	".jsx": api.LoaderJSX,
	".js":  api.LoaderJS, ".mjs": api.LoaderJS, ".cjs": api.LoaderJS,
	".json": api.LoaderJSON,
}

// resolveRenderer finds React's static renderer in the react-dom that the
// project resolves.
func resolveRenderer(b api.PluginBuild, dir string, v variant) (api.OnResolveResult, error) {
	options := api.ResolveOptions{ResolveDir: dir, Kind: api.ResolveJSImportStatement}
	if v.renderer != "" {
		resolved := b.Resolve(v.renderer, options)
		return api.OnResolveResult{Path: resolved.Path, Errors: resolved.Errors}, nil
	}
	resolved := b.Resolve("react-dom/package.json", options)
	if len(resolved.Errors) > 0 {
		return api.OnResolveResult{}, fmt.Errorf("react-dom is not installed in %s: the builder renders pages with the project's React", dir)
	}
	path := filepath.Join(filepath.Dir(resolved.Path), filepath.FromSlash(staticRenderer))
	if _, err := os.Stat(path); err != nil {
		return api.OnResolveResult{}, fmt.Errorf("this react-dom has no static renderer the builder knows (%s)", path)
	}
	return api.OnResolveResult{Path: path}, nil
}

// resolve makes the map's sources what the program calls its files.
func (m *sourceMap) resolve(dir string) {
	for i, source := range m.sources {
		if !strings.HasPrefix(source, namespace+":") && !filepath.IsAbs(source) {
			m.sources[i] = filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(source)))
		}
	}
}

// bundleError reports an error of esbuild's: render-bundle, where the author
// wrote what it is about.
func bundleError(program *rtsx.Program, dir string, message api.Message) report.Report {
	r := report.Report{Code: "render-bundle", Message: message.Text}
	location := message.Location
	if location == nil || location.Namespace != "" && location.Namespace != "file" {
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
