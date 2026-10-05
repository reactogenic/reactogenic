package build

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/report"
)

// styles bundles the stylesheets the pages import (builder.md, *CSS*): one
// esbuild build, in memory, with every page's module as an entry — esbuild
// then gives each entry the CSS its modules import, in import order. The JS
// it makes of the same modules is not looked at.
//
// The sheets are one per route, in their order; with site — the control —
// there is one: everything any page imports, as a build that does not know
// the pages apart would ship it. A sheet is not minified, so that esbuild
// names each source file in it (`/* path/to/file.css */`, from dir) and the
// pruner can count per file; its nesting is lowered, which is the form the
// pruner reads.
//
// The modules are the program's, as in the render bundle: the same resolver
// (render.Resolve), the same texts (render.Load). What only this build sees
// is reported as css-bundle — a stylesheet that is not there, at the import
// that names it — and what esbuild has to say of a stylesheet is passed on
// as a warning (css-warning).
func styles(program *rtsx.Program, dir string, routes []render.Route, site bool) (sheets []string, reports []report.Report) {
	plugin := api.Plugin{Name: "reactogenic-css", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: `.*`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			switch {
			case args.Kind == api.ResolveCSSURLToken:
				// Nothing is processed in phase 2 (builder.md, *Not in
				// phase 2*): a `url()` stays as it is written.
				return api.OnResolveResult{Path: args.Path, External: true}, nil
			case args.Path == "react" || strings.HasPrefix(args.Path, "react/") || args.Path == "react-dom" || strings.HasPrefix(args.Path, "react-dom/"):
				// React holds no stylesheet, and in the render bundle it is
				// the builder's own: it is not read here.
				return api.OnResolveResult{Path: args.Path, External: true}, nil
			case filepath.IsAbs(args.Path) && render.Source(program, args.Path) != nil:
				// A page, as an entry: under the name the program has for
				// it, which is the name its text is loaded by. esbuild
				// would name it by where it is — another file to the
				// program when the pages are reached through a link.
				return api.OnResolveResult{Path: args.Path}, nil
			}
			return api.OnResolveResult{Path: render.Resolve(program, args.Importer, args.Path)}, nil
		})
		b.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			_, result, err := render.Load(program, args.Path)
			return result, err
		})
	}}
	options := api.BuildOptions{
		Bundle:        true,
		Write:         false,
		Outdir:        filepath.Join(dir, ".reactogenic"), // never written: it names the outputs
		AbsWorkingDir: dir,
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		// As the render bundle reads the same files: no tsconfig, React's
		// automatic JSX, and production — which modules a package loads may
		// depend on it.
		TsconfigRaw: "{}",
		JSX:         api.JSXAutomatic,
		Define:      map[string]string{"process.env.NODE_ENV": `"production"`},
		// Lowered and not minified: see above.
		Supported: map[string]bool{"nesting": false},
		LogLevel:  api.LogLevelSilent,
		Plugins:   []api.Plugin{plugin},
	}
	if site {
		var entry strings.Builder
		for _, route := range routes {
			entry.WriteString("import " + strconv.Quote(route.File) + ";\n")
		}
		options.Stdin = &api.StdinOptions{Contents: entry.String(), ResolveDir: dir, Sourcefile: "<site>", Loader: api.LoaderJS}
		sheets = make([]string, 1)
	} else {
		// An output is named by its entry's place among the routes: every
		// page is an `index` of its directory.
		for i, route := range routes {
			options.EntryPointsAdvanced = append(options.EntryPointsAdvanced, api.EntryPoint{InputPath: filepath.FromSlash(route.File), OutputPath: strconv.Itoa(i)})
		}
		sheets = make([]string, len(routes))
	}
	result := api.Build(options)
	for _, message := range result.Errors {
		reports = append(reports, render.Message(program, dir, "css-bundle", message))
	}
	for _, message := range result.Warnings {
		// Of the stylesheets only: what esbuild says of the modules' code
		// is not this build's business — it is compiled to be thrown away.
		if message.Location == nil || !strings.HasSuffix(strings.ToLower(message.Location.File), ".css") {
			continue
		}
		r := render.Message(program, dir, "css-warning", message)
		r.Severity = report.Warning
		for _, note := range message.Notes { // `Did you mean "width" instead?`
			r.Related = append(r.Related, report.Report{Severity: report.Message, Message: note.Text})
		}
		reports = append(reports, r)
	}
	if len(result.Errors) > 0 {
		return nil, reports
	}
	for _, file := range result.OutputFiles {
		name, isCSS := strings.CutSuffix(filepath.Base(file.Path), ".css")
		if !isCSS {
			continue
		}
		i := 0
		if !site {
			var err error
			if i, err = strconv.Atoi(name); err != nil || i < 0 || i >= len(sheets) {
				continue
			}
		}
		sheets[i] = string(file.Contents)
	}
	return sheets, reports
}

// minify is the last step of a page's CSS (builder.md, *CSS*): what the
// pruner kept — byte for byte what esbuild printed, source comments included
// — minified by esbuild. Without the line break esbuild ends a file with: a
// byte of nobody's. What esbuild says here it has said of the bundle.
func minify(css string) string {
	if strings.TrimSpace(css) == "" {
		return ""
	}
	result := api.Transform(css, api.TransformOptions{
		Loader:           api.LoaderCSS,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Supported:        map[string]bool{"nesting": false},
		LogLevel:         api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		// esbuild reads any CSS; were it not to, the sheet is shipped as it is.
		return css
	}
	return strings.TrimSpace(string(result.Code))
}
