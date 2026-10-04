package build

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/build/behaviors"
	"github.com/reactogenic/reactogenic/go/internal/build/render"
	"github.com/reactogenic/reactogenic/go/internal/check"
)

// orders are the rules of the `served` fixture's sheet that a page ships:
// each rule there has an `order` of its own.
func orders(css string) []int {
	var out []int
	for _, m := range regexp.MustCompile(`order:(\d+)`).FindAllStringSubmatch(css, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// TestServed: a page's CSS is pruned against the page as it is served
// (builder.md, *CSS*) — with the base in its links, and with the `<style>`
// or `<link>` and the `<script>` packaging puts in it. A rule that matches
// the file that is written is in its sheet; one that only matched the page
// as it was rendered is not.
func TestServed(t *testing.T) {
	everything := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	tests := []struct {
		name  string
		args  []string
		home  []int // "/": `<a href="/guide/">`, a `<p>`, a script
		guide []int // "/guide/": `<a href="/">`, no script
	}{
		// 2: `a[href="/guide/"]`; 5: `[href$="/guide/"]`; 6: `style`; 9:
		// `title + style`; 10: `script`; 12: `p + script`.
		{"no base, inlined", []string{"--inline", "always"}, []int{1, 2, 5, 6, 9, 10, 12}, []int{1, 6, 9}},
		// 7: `link[rel=stylesheet]`; 9: `title + link`; 11: `script[src]`.
		{"no base, files", []string{"--inline", "never"}, []int{1, 2, 5, 7, 9, 10, 11, 12}, []int{1, 7, 9}},
		// 3: `a[href="/docs/guide/"]`; 4: `a[href^="/docs/"]` — and not 2.
		{"a base, inlined", []string{"--base", "/docs/", "--inline", "always"}, []int{1, 3, 4, 5, 6, 9, 10, 12}, []int{1, 4, 6, 9}},
		// 8: `link[href^="/docs/_rg/"]`: the sheet's own URL.
		{"a base, files", []string{"--base", "/docs/", "--inline", "never"}, []int{1, 3, 4, 5, 7, 8, 9, 10, 11, 12}, []int{1, 4, 7, 8, 9}},
		// One page each: nothing is shared, so nothing is a file.
		{"a base, auto", []string{"--base", "/docs/"}, []int{1, 3, 4, 5, 6, 9, 10, 12}, []int{1, 4, 6, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := load(t, append([]string{"-p", "served"}, tt.args...)...)
			for pathname, want := range map[string][]int{"/": tt.home, "/guide/": tt.guide, "/edit/": everything} {
				if got := orders(s.css(t, pathname)); !slices.Equal(got, want) {
					t.Errorf("%s ships the rules %v, want %v\n%s", pathname, got, want, s.files[s.page(t, pathname).Output])
				}
			}
			// The page the user edits has its whole sheet, and the report
			// says why.
			if styles := s.page(t, "/edit/").Styles; styles == nil || !styles.Unpruned || styles.Because != "the page has an element the user edits (`contenteditable`)" || styles.RulesDropped != 0 {
				t.Errorf("/edit/: %+v", styles)
			}
			if styles := s.page(t, "/").Styles; styles == nil || styles.Unpruned || styles.Because != "" || styles.Rules != 15 || styles.RulesDropped != 15-len(tt.home) {
				t.Errorf("/: %+v", styles)
			}
			// The control ships every rule to every page, whatever is in it.
			control := load(t, append([]string{"-p", "served", "--no-specialize"}, tt.args...)...)
			for _, p := range control.report.Pages {
				if got := orders(control.css(t, p.Pathname)); !slices.Equal(got, everything) {
					t.Errorf("the control: %s ships the rules %v", p.Pathname, got)
				}
			}
		})
	}
}

// TestServedGivesUp: a page whose document is still another one after
// `rounds` rounds — a rule selects on the URL of the sheet it is in, which
// carries the sheet's hash — is given its whole sheet: that depends on
// nothing. Forced here: no round is allowed after the guess, and with files
// every page's document is another than the guess.
func TestServedGivesUp(t *testing.T) {
	defer func(n int) { rounds = n }(rounds)
	rounds = 1
	s := load(t, "-p", "served", "--inline", "never")
	for _, p := range s.report.Pages {
		if got := orders(s.css(t, p.Pathname)); len(got) != 15 {
			t.Errorf("%s ships the rules %v", p.Pathname, got)
		}
		if p.Styles == nil || !p.Styles.Unpruned || p.Styles.RulesDropped != 0 || p.Styles.Rules != 15 {
			t.Errorf("%s: %+v", p.Pathname, p.Styles)
		}
	}
	if because := s.page(t, "/").Styles.Because; because != "its rules select on the URL of the stylesheet they are in" {
		t.Errorf("/: because %q", because)
	}
	// The three sheets are one now: a file for the site.
	if len(s.report.Blobs) != 2 || len(s.report.Blobs[0].Pages)+len(s.report.Blobs[1].Pages) != 4 {
		t.Errorf("blobs: %+v", s.report.Blobs)
	}
}

// TestReferences: `-p` is the project as for `check` — a tsconfig that only
// references others is checked, and built, through them (builder.md, the
// flags; *The pipeline*).
func TestReferences(t *testing.T) {
	// The fixture through its solution-style tsconfig: the same site.
	out, _ := buildSite(t, "-p", "site/tsconfig.solution.json", "--report")
	golden(t, "auto/out", tree(t, out))

	// Vite's template: `tsconfig.json` references `tsconfig.app.json`, which
	// lists the pages, and `tsconfig.node.json`, which does not.
	app := strings.Replace(tsconfig, `"include": ["."]`, `"include": ["pages"]`, 1)
	node := strings.Replace(tsconfig, `"include": ["."]`, `"include": ["tool.ts"]`, 1)
	work := scratch(t, nil, map[string]string{
		"tsconfig.json":      `{ "files": [], "references": [{ "path": "./tsconfig.node.json" }, { "path": "./tsconfig.app.json" }] }`,
		"tsconfig.app.json":  app,
		"tsconfig.node.json": node,
		"tool.ts":            "export const port: number = 5173;\n",
		"pages/index.rtsx":   pageOf(`<p>Home</p>`),
	})
	if reports := check.Run(filepath.ToSlash(filepath.Join(work, "tsconfig.json"))); len(reports) != 0 {
		t.Fatalf("check: %+v", reports)
	}
	for _, p := range []string{".", "tsconfig.json", "tsconfig.app.json"} {
		stdout, stderr, status := runIn(t, work, "-p", p)
		if status != 0 || stderr != "" || stdout != "1 page written to dist\n" {
			t.Errorf("-p %s: status %d\n%s%s", p, status, stdout, stderr)
		}
		if html := tree(t, filepath.Join(work, "dist"))["index.html"]; !strings.Contains(html, "<p>Home</p>") {
			t.Errorf("-p %s: the page: %s", p, html)
		}
	}
	// An error of a referenced project that holds no page stops the build,
	// and is printed as `check` prints it.
	if err := os.WriteFile(filepath.Join(work, "tool.ts"), []byte("export const port: number = \"5173\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, status := runIn(t, work)
	if want := "tool.ts(1,14): error TS2322: Type 'string' is not assignable to type 'number'.\n"; status != 1 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, want)
	}
	// A page no project lists is not built unchecked.
	if err := os.WriteFile(filepath.Join(work, "tsconfig.app.json"), []byte(strings.Replace(tsconfig, `"include": ["."]`, `"include": ["tool.ts"]`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "tool.ts"), []byte("export const port: number = 5173;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, status = runIn(t, work)
	if want := "pages/index.rtsx(1,1): error render-bundle: The page is not a module of the project: the tsconfig does not include it\n"; status != 1 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, want)
	}
}

// TestLinked: `pages` and `public` may be symbolic links (builder.md,
// *Routes*): the pages are found through the link, and are the modules the
// program has under whichever name it gave them.
func TestLinked(t *testing.T) {
	files := map[string]string{
		"tsconfig.json":              tsconfig,
		"real-pages/index.rtsx":      pageOf(`<a href="/x.txt">x</a><a href="/guide/">guide</a>`),
		"real-pages/guide/index.tsx": pageOf(`<a href="/">home</a>`),
		"real-public/x.txt":          "x\n",
	}
	built := func(t *testing.T, work string, args ...string) {
		t.Helper()
		stdout, stderr, status := runIn(t, work, args...)
		if status != 0 || stderr != "" || stdout != "2 pages written to dist\n" {
			t.Fatalf("%v: status %d\n%s%s", args, status, stdout, stderr)
		}
		got := tree(t, filepath.Join(work, "dist"))
		if got["x.txt"] != "x\n" || !strings.Contains(got["index.html"], `href="/guide/"`) || !strings.Contains(got["guide/index.html"], `href="/"`) {
			t.Errorf("%v: the output: %v", args, slices.Sorted(maps.Keys(got)))
		}
	}
	link := func(t *testing.T, work, target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(work, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("both are links", func(t *testing.T) {
		work := scratch(t, nil, files)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		built(t, work)
		built(t, work, "--pages", "pages")
		built(t, work, "--pages", filepath.Join(work, "pages"))
		built(t, work, "--pages", "real-pages")
	})
	t.Run("the pages lead elsewhere: public is beside the link", func(t *testing.T) {
		work := scratch(t, nil, map[string]string{
			"site/tsconfig.json":           tsconfig,
			"site/public/x.txt":            "x\n",
			"shared/pages/index.rtsx":      files["real-pages/index.rtsx"],
			"shared/pages/guide/index.tsx": files["real-pages/guide/index.tsx"],
			"shared/public/y.txt":          "not the site's\n",
		})
		link(t, filepath.Join(work, "site"), filepath.Join("..", "shared", "pages"), "pages")
		built(t, filepath.Join(work, "site"))
		if got := tree(t, filepath.Join(work, "site", "dist")); got["y.txt"] != "" {
			t.Errorf("the output: %v", slices.Sorted(maps.Keys(got)))
		}
	})
	t.Run("public alone", func(t *testing.T) {
		work := scratch(t, nil, files)
		if err := os.Rename(filepath.Join(work, "real-pages"), filepath.Join(work, "pages")); err != nil {
			t.Fatal(err)
		}
		link(t, work, "real-public", "public")
		built(t, work)
	})
	t.Run("the project's directory is reached through a link", func(t *testing.T) {
		work := scratch(t, nil, files)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		through := filepath.Join(t.TempDir(), "through")
		if err := os.Symlink(work, through); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, status := runIn(t, through)
		if status != 0 || stderr != "" || stdout != "2 pages written to dist\n" {
			t.Fatalf("status %d\n%s%s", status, stdout, stderr)
		}
	})
	t.Run("the tsconfig lists the pages where they are, not through the link", func(t *testing.T) {
		listed := maps.Clone(files)
		listed["tsconfig.json"] = strings.Replace(tsconfig, `"include": ["."]`, `"include": ["real-pages"]`, 1)
		work := scratch(t, nil, listed)
		link(t, work, "real-pages", "pages")
		link(t, work, "real-public", "public")
		built(t, work)
	})
}

// TestConflicts: a file of `public/` cannot be where the build writes
// (builder.md, *Routes*: public-conflict) — also not as a file where the
// build needs a directory, or under what the build writes as a file. Found
// before anything is written: the write would stop half-way.
func TestConflicts(t *testing.T) {
	routes := []render.Route{{Pathname: "/"}, {Pathname: "/guide/"}, {Pathname: "/guide/more/"}, {Pathname: "/a/b/c/"}}
	opts := Options{Pages: filepath.FromSlash("/site/pages")}
	tests := []struct {
		file string
		says string // "": no conflict
	}{
		{"favicon.svg", ""},
		{"guide/logo.svg", ""},
		{"guide/more/index.htm", ""},
		{"a/b/index.html", ""}, // no page is written there
		{"guides", ""},
		{"_rgb/x.css", ""},
		{"index.html", "The page / is written to `index.html`: a file of `public/` cannot be there"},
		{"guide/more/index.html", "The page /guide/more/ is written to `guide/more/index.html`: a file of `public/` cannot be there"},
		{"_rg/mine.css", "`_rg/` is the builder's: a file of `public/` cannot be there"},
		{"_rg", "`_rg/` is the builder's: a file of `public/` cannot be there"},
		{"guide", "The page /guide/ is written to `guide/index.html`: `guide` is a directory of the output, and cannot be a file of `public/`"},
		{"guide/more", "The page /guide/more/ is written to `guide/more/index.html`: `guide/more` is a directory of the output, and cannot be a file of `public/`"},
		{"a", "The page /a/b/c/ is written to `a/b/c/index.html`: `a` is a directory of the output, and cannot be a file of `public/`"},
		{"a/b", "The page /a/b/c/ is written to `a/b/c/index.html`: `a/b` is a directory of the output, and cannot be a file of `public/`"},
		{"index.html/x.txt", "The page / is written to `index.html`: a file of `public/` cannot be under it"},
		{"guide/index.html/deep/x.txt", "The page /guide/ is written to `guide/index.html`: a file of `public/` cannot be under it"},
	}
	for _, tt := range tests {
		reports := conflicts(opts, routes, []string{tt.file})
		switch {
		case tt.says == "" && len(reports) != 0:
			t.Errorf("%s: %+v", tt.file, reports)
		case tt.says != "" && (len(reports) != 1 || reports[0].Code != "public-conflict" || reports[0].Message != tt.says || reports[0].File != "/site/public/"+tt.file):
			t.Errorf("%s: %+v\nwant: %s", tt.file, reports, tt.says)
		}
	}
}

// TestFailedWrite: the new output is written beside the old one, and takes
// its place when all of it is there (builder.md, *The output directory*) —
// a build that cannot write leaves the last output as it was.
func TestFailedWrite(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file that cannot be read")
	}
	work := scratch(t, []string{"site"}, nil)
	site := filepath.Join(work, "site")
	out := filepath.Join(site, "dist")
	if stdout, stderr, status := runIn(t, site); status != 0 {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	before := tree(t, out)

	// A file of `public/` that cannot be read: found when it is copied.
	secret := filepath.Join(site, "public", "robots.txt")
	if err := os.Chmod(secret, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(secret, 0o644)
	stdout, stderr, status := runIn(t, site)
	if status != 1 || stdout != "" || !strings.HasPrefix(stderr, "reactogenic build: ") || !strings.Contains(stderr, "robots.txt") {
		t.Errorf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
	if after := tree(t, out); !maps.Equal(before, after) {
		t.Errorf("a build that could not write changed the output: %v, was %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	if entries, _ := os.ReadDir(out); len(entries) != len(topLevel(before)) {
		t.Errorf("something is left beside the output: %v", entries)
	}

	// Into a directory that is not there: nothing is left that the next
	// build would not take for its own.
	fresh := filepath.Join(work, "fresh")
	if _, _, status := runIn(t, site, "--out", fresh); status != 1 {
		t.Fatalf("status %d", status)
	}
	if entries, _ := os.ReadDir(fresh); len(entries) != 0 {
		t.Errorf("a build that could not write left %v", entries)
	}
	os.Chmod(secret, 0o644)
	if stdout, stderr, status := runIn(t, site, "--out", fresh); status != 0 {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	if after := tree(t, fresh); !maps.Equal(before, after) {
		t.Errorf("the output: %v, want %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
}

// topLevel are the entries of a tree's root.
func topLevel(files map[string]string) map[string]bool {
	top := map[string]bool{}
	for file := range files {
		name, _, _ := strings.Cut(file, "/")
		top[name] = true
	}
	return top
}

// TestNaming: a file of the report is named by what it is on every machine
// (builder.md, *The report*: files) — the project's own from the project
// directory, a package's by its package, wherever the install put it.
func TestNaming(t *testing.T) {
	root := real(t.TempDir())
	for file, content := range map[string]string{
		"repo/site/site.css":                                                       "",
		"repo/site/node_modules/plain/package.json":                                `{"name": "plain"}`,
		"repo/site/node_modules/plain/dist/esm/package.json":                       `{"type": "module"}`,
		"repo/site/node_modules/plain/dist/esm/a.css":                              "",
		"repo/site/node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/package.json": `{"name": "@s/ui", "version": "1.0.0"}`,
		"repo/site/node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/src/b.ts":     "",
		"repo/packages/ui/package.json":                                            `{"name": "@reactogenic/ui"}`,
		"repo/packages/ui/src/behaviors/overlays.ts":                               "",
		"repo/shared/tokens.css":                                                   "",
		"elsewhere/lib/package.json":                                               `not JSON`,
		"elsewhere/lib/c.css":                                                      "",
	} {
		if err := writeFile(root, file, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	names := naming{dir: filepath.Join(root, "repo", "site"), packages: map[string]string{}}
	for name, want := range map[string]string{
		"site.css":                          "site.css",
		"pages/guide/page.css":              "pages/guide/page.css", // need not be there
		"node_modules/plain/dist/esm/a.css": "plain/dist/esm/a.css",
		"node_modules/.pnpm/@s+ui@1.0.0/node_modules/@s/ui/src/b.ts": "@s/ui/src/b.ts",
		"../packages/ui/src/behaviors/overlays.ts":                   "@reactogenic/ui/src/behaviors/overlays.ts",
		// Of no package: where it is, from the project.
		"../shared/tokens.css":      "../shared/tokens.css",
		"../../elsewhere/lib/c.css": "../../elsewhere/lib/c.css",
		behaviors.Entry:             behaviors.Entry,
		behaviors.Runtime:           behaviors.Runtime,
	} {
		if got := names.of(name); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}

// TestReportNamesNoMachine: a project whose packages are outside it —
// reached through a link, as with a workspace, `npm link` or a store — has a
// report that names no path of the machine (builder.md, *The report*).
func TestReportNamesNoMachine(t *testing.T) {
	work := scratch(t, []string{"site"}, nil)
	stdout, stderr, status := runIn(t, filepath.Join(work, "site"), "--report")
	if status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	report := tree(t, filepath.Join(work, "site", "dist"))[reportFile]
	repo, _ := filepath.Abs("../../..")
	for _, machine := range []string{"../", filepath.ToSlash(work), filepath.ToSlash(repo), "node_modules"} {
		if strings.Contains(report, machine) || strings.Contains(stdout, machine) {
			t.Errorf("the report holds %q", machine)
		}
	}
	for _, name := range []string{`"@reactogenic/ui/src/behaviors/overlays.ts"`, `"@reactogenic/ui/src/dialog.css"`, `"site.css"`} {
		if !strings.Contains(report, name) {
			t.Errorf("the report does not name %s", name)
		}
	}
	// The same bytes as the build of the fixture in the repository.
	golden(t, "auto/out", tree(t, filepath.Join(work, "site", "dist")))
}
