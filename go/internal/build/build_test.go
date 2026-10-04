package build

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/mapper"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// The transform is registered once for the process, as the command does.
func TestMain(m *testing.M) {
	mapper.RegisterStrict("test")
	os.Exit(m.Run())
}

// The fixtures are projects of testdata: on disk, inside the repository,
// because their pages render with the repository's React and use the real
// @reactogenic/ui — both resolved from the root's node_modules, as a project
// resolves them.
//
//	site    seven pages that differ in what they use: nothing that opens
//	        (/plain/, /guide/, /guide/more/), a drawer (/), a menu of links
//	        (/links/), a dialog (/dialog/), an action menu with typeahead
//	        that opens a dialog (/actions/); a segment; `public/`
//	served  three pages for what packaging changes of a page: the base in its
//	        links, the elements it adds — and a page the user edits
//	bad     one project, a `--pages` root per mistake
//	types   a project that does not check
//	both    `index.rtsx` and `index.tsx` in one directory
//
// `site` and `types` also have a `tsconfig.solution.json`: a tsconfig that
// only references the project's, as Vite's template lays one out.
func testdata(t testing.TB) string {
	t.Helper()
	repo, _ := filepath.Abs("../../..")
	for _, pkg := range []string{"react-dom", "@reactogenic/ui"} {
		if _, err := os.Stat(filepath.Join(repo, "node_modules", pkg, "package.json")); err != nil {
			t.Fatalf("the fixtures build with the repository's React and @reactogenic/ui, and %s has no node_modules/%s: run `pnpm install` there", repo, pkg)
		}
	}
	dir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// run is `reactogenic build` with args, in testdata.
func run(t testing.TB, args ...string) (stdout, stderr string, status int) {
	t.Helper()
	return runIn(t, testdata(t), args...)
}

// runIn is `reactogenic build` with args, in cwd.
func runIn(t testing.TB, cwd string, args ...string) (stdout, stderr string, status int) {
	t.Helper()
	var out, err bytes.Buffer
	status = Main(args, cwd, &out, &err)
	return out.String(), err.String(), status
}

// scratch is a directory of the test's own that holds copies of fixtures —
// and files of its own, by path — and reaches the repository's packages as a
// project outside it would: through a link to its node_modules. It is where
// a test builds what may go wrong with the project's own directory, and
// what it has to change: nothing a failing test empties is the repository's.
func scratch(t testing.TB, fixtures []string, files map[string]string) string {
	t.Helper()
	data := testdata(t)
	repo, _ := filepath.Abs("../../..")
	work := real(t.TempDir())
	if err := os.Symlink(filepath.Join(repo, "node_modules"), filepath.Join(work, "node_modules")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	for _, name := range fixtures {
		if err := os.CopyFS(filepath.Join(work, name), os.DirFS(filepath.Join(data, name))); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		if err := writeFile(work, name, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	return work
}

// What a project of a test's own starts from: the fixtures' tsconfig, and a
// page.
const (
	tsconfig = `{
  "compilerOptions": {
    "strict": true, "jsx": "react-jsx", "module": "esnext", "moduleResolution": "bundler",
    "target": "es2022", "lib": ["es2022", "dom"], "types": [], "noEmit": true, "skipLibCheck": true
  },
  "include": ["."]
}
`
	cssModule = "declare module \"*.css\";\n"
)

func pageOf(body string) string {
	return "export default function Page() {\n  return <html lang=\"en\"><head><title>t</title></head><body>" + body + "</body></html>;\n}\n"
}

// built runs a build that has to succeed into a new directory, and returns
// the directory and what the command printed.
func buildSite(t testing.TB, args ...string) (out, stdout string) {
	t.Helper()
	out = filepath.Join(t.TempDir(), "dist")
	stdout, stderr, status := run(t, append(args, "--out", out)...)
	if status != 0 || stderr != "" {
		t.Fatalf("build %v: status %d\n%s%s", args, status, stdout, stderr)
	}
	// The one line that names the output directory is the machine's.
	return out, strings.ReplaceAll(stdout, filepath.ToSlash(real(out)), "<out>")
}

// tree is a directory as its files: path from it → content.
func tree(t testing.TB, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// golden compares files with the directory testdata/golden/<name>, file by
// file and as a set; `go test -update` rewrites it.
func golden(t *testing.T, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join("testdata", "golden", filepath.FromSlash(name))
	if *updateGolden {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		for file, content := range files {
			if err := writeFile(dir, file, []byte(content)); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("no golden output %s: run `go test -update`", dir)
	}
	want := tree(t, dir)
	for _, file := range slices.Sorted(maps.Keys(files)) {
		expected, ok := want[file]
		switch {
		case !ok:
			t.Errorf("%s: %s is written and is not in the golden output", name, file)
		case expected != files[file]:
			t.Errorf("%s: %s differs from the golden output:\n--- got\n%s\n--- want\n%s", name, file, files[file], expected)
		}
	}
	for _, file := range slices.Sorted(maps.Keys(want)) {
		if _, ok := files[file]; !ok {
			t.Errorf("%s: %s of the golden output is not written", name, file)
		}
	}
}

// The modes of a build: the golden output of each is the whole of its
// output directory (`golden/<mode>/out` — not `dist`, which the repository
// ignores) and what the command printed (`golden/<mode>/print/stdout.txt`).
var modes = []struct {
	name string
	args []string
}{
	{"auto", []string{"-p", "site", "--report"}},
	{"always", []string{"-p", "site", "--inline", "always"}},
	{"never", []string{"-p", "site", "--inline", "never"}},
	{"control", []string{"-p", "site", "--no-specialize", "--report"}},
	{"base", []string{"-p", "site", "--base", "/docs", "--inline", "never"}},
	{"base-control", []string{"-p", "site", "--base", "/docs/", "--no-specialize"}},
	// The page as it is served (TestServed).
	{"served", []string{"-p", "served", "--inline", "always", "--report"}},
	{"served-base", []string{"-p", "served", "--base", "/docs/", "--inline", "always"}},
	{"served-files", []string{"-p", "served", "--base", "/docs/", "--inline", "never"}},
}

// TestGolden: the whole output of the fixture site, in every mode.
func TestGolden(t *testing.T) {
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			out, stdout := buildSite(t, mode.args...)
			golden(t, mode.name+"/out", tree(t, out))
			golden(t, mode.name+"/print", map[string]string{"stdout.txt": stdout})
		})
	}
}

// TestDeterministic: two builds of one input are the same bytes — the pages,
// the blobs and their names, the report.
func TestDeterministic(t *testing.T) {
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			first, printed := buildSite(t, mode.args...)
			second, again := buildSite(t, mode.args...)
			a, b := tree(t, first), tree(t, second)
			if !maps.Equal(a, b) {
				for _, file := range slices.Sorted(maps.Keys(a)) {
					if a[file] != b[file] {
						t.Errorf("%s differs between two builds:\n%s\n---\n%s", file, a[file], b[file])
					}
				}
				t.Errorf("files: %v and %v", slices.Sorted(maps.Keys(a)), slices.Sorted(maps.Keys(b)))
			}
			if printed != again {
				t.Errorf("printed:\n%s\n---\n%s", printed, again)
			}
		})
	}
}

// site is a built site, for the assertions: its files, and its report.
type site struct {
	files  map[string]string
	report Report
}

func load(t *testing.T, args ...string) site {
	t.Helper()
	out, _ := buildSite(t, args...)
	s := site{files: tree(t, out)}
	if err := json.Unmarshal([]byte(s.files[reportFile]), &s.report); err != nil {
		t.Fatalf("%s: %v", reportFile, err)
	}
	return s
}

// page is a page of the report.
func (s site) page(t *testing.T, pathname string) PageReport {
	t.Helper()
	for _, p := range s.report.Pages {
		if p.Pathname == pathname {
			return p
		}
	}
	t.Fatalf("no page %s in the report", pathname)
	return PageReport{}
}

var (
	inlineStyle  = regexp.MustCompile(`(?s)<style>(.*?)</style></head>`)
	inlineScript = regexp.MustCompile(`(?s)<script type="module">(.*?)</script></body></html>$`)
)

// css and js are what a page ships, wherever it is: in the page — at the end
// of its head, at the end of its body — or in the file the page links.
func (s site) css(t *testing.T, pathname string) string {
	t.Helper()
	p := s.page(t, pathname)
	html := s.files[p.Output]
	switch {
	case p.CSS == nil:
		if strings.Contains(html, "<style") || strings.Contains(html, `rel="stylesheet"`) {
			t.Errorf("%s has no CSS in the report, and a stylesheet in the page", pathname)
		}
		return ""
	case p.CSS.Delivery == "file":
		if !strings.Contains(html, `<link rel="stylesheet" href="`+p.CSS.URL+`"></head>`) {
			t.Errorf("%s does not link %s at the end of its head", pathname, p.CSS.URL)
		}
		return s.files[strings.TrimPrefix(p.CSS.URL, s.report.Base)]
	}
	m := inlineStyle.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("%s has no <style> at the end of its head", pathname)
	}
	return m[1]
}

func (s site) js(t *testing.T, pathname string) string {
	t.Helper()
	p := s.page(t, pathname)
	html := s.files[p.Output]
	switch {
	case p.JS == nil:
		if strings.Contains(html, "<script") {
			t.Errorf("%s has no JS in the report, and a <script> in the page", pathname)
		}
		return ""
	case p.JS.Delivery == "file":
		if !strings.HasSuffix(html, `<script type="module" src="`+p.JS.URL+`"></script></body></html>`) {
			t.Errorf("%s does not load %s at the end of its body", pathname, p.JS.URL)
		}
		return s.files[strings.TrimPrefix(p.JS.URL, s.report.Base)]
	}
	m := inlineScript.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("%s has no <script type=module> at the end of its body", pathname)
	}
	return m[1]
}

// What only one behaviour has: a key `menu-keys` handles, the way its
// typeahead reads a key, the event `overlays` listens to, the test
// `invokers` makes of the browser.
const (
	menuKeys  = "ArrowDown"
	typeahead = "startsWith("
	overlays  = "pagehide"
	invokers  = "HTMLButtonElement"
)

// TestShips: what each page of the fixture ships is what it uses (builder.md,
// *Behaviours*, *CSS*) — in every way of delivering it.
func TestShips(t *testing.T) {
	everything := []string{"/", "/actions/", "/dialog/", "/guide/", "/guide/more/", "/links/", "/plain/"}
	for _, inline := range []string{InlineAuto, InlineAlways, InlineNever} {
		t.Run(inline, func(t *testing.T) {
			s := load(t, "-p", "site", "--inline", inline)
			var pathnames []string
			for _, p := range s.report.Pages {
				pathnames = append(pathnames, p.Pathname)
				if html := s.files[p.Output]; !strings.HasPrefix(html, "<!doctype html><html lang=\"en\"><head>") || !strings.HasSuffix(html, "</body></html>") {
					t.Errorf("%s is not a document: %.60s … %s", p.Output, html, html[max(0, len(html)-30):])
				}
			}
			if !slices.Equal(pathnames, everything) {
				t.Fatalf("pages: %q", pathnames)
			}
			// Every page is pruned — the ones with a script too: the
			// `<script>` in them is the builder's (builder.md, *CSS*, *The
			// builder's own elements*). Unpruned, the CSS half of the bet
			// is lost and nothing else fails.
			for _, p := range s.report.Pages {
				if p.Styles == nil || p.Styles.Unpruned || p.Styles.Why != "" {
					t.Errorf("%s is not pruned: %+v", p.Pathname, p.Styles)
				}
			}

			// The script: the behaviours the page mounted, and nothing else.
			scripts := []struct {
				pathname string
				has      []string
			}{
				{"/plain/", nil}, // nothing that opens: no <script>
				{"/guide/", nil},
				{"/guide/more/", nil},
				{"/", []string{overlays}},       // a drawer
				{"/links/", []string{overlays}}, // a menu of links: no keys
				{"/dialog/", []string{overlays, invokers}},
				{"/actions/", []string{overlays, invokers, menuKeys, typeahead}},
			}
			for _, want := range scripts {
				js := s.js(t, want.pathname)
				if (js == "") != (want.has == nil) {
					t.Errorf("%s: script %q", want.pathname, js)
				}
				for _, marker := range []string{overlays, invokers, menuKeys, typeahead} {
					if strings.Contains(js, marker) != slices.Contains(want.has, marker) {
						t.Errorf("%s: its script holds %q: %v, want %v\n%s", want.pathname, marker, !slices.Contains(want.has, marker), slices.Contains(want.has, marker), js)
					}
				}
				if strings.Contains(js, "RG_") || strings.Contains(js, "react") {
					t.Errorf("%s: a flag or React is left in its script:\n%s", want.pathname, js)
				}
				// The report's rows add up to the script.
				total := 0
				for _, m := range s.page(t, want.pathname).Modules {
					total += m.Bytes
				}
				if total != len(js) {
					t.Errorf("%s: the modules of the report add up to %d B, the script is %d B", want.pathname, total, len(js))
				}
			}

			// The CSS: a rule is there only if the page has what it styles.
			sheets := []struct {
				pathname string
				has      []string
			}{
				{"/plain/", []string{".rg-button", ".site-header"}},
				{"/guide/", []string{".site-header", ".prose"}},
				{"/guide/more/", []string{".site-header", ".prose"}},
				{"/", []string{".rg-sidemenu", ".site-header"}},
				{"/links/", []string{".rg-button", ".rg-menu", ".site-header"}},
				{"/dialog/", []string{".rg-button", ".rg-dialog", ".site-header"}},
				{"/actions/", []string{".rg-button", ".rg-dialog", ".rg-menu", ".site-header"}},
			}
			for _, want := range sheets {
				css := s.css(t, want.pathname)
				for _, class := range []string{".rg-button", ".rg-dialog", ".rg-menu", ".rg-sidemenu", ".site-header", ".prose"} {
					if strings.Contains(css, class) != slices.Contains(want.has, class) {
						t.Errorf("%s: its CSS holds %s: %v, want %v\n%s", want.pathname, class, !slices.Contains(want.has, class), slices.Contains(want.has, class), css)
					}
				}
				// No page has a `.never`: the rule goes, and its animation with it.
				if strings.Contains(css, "never") || strings.Contains(css, "/*") {
					t.Errorf("%s: a rule of no page, or a comment, is in its CSS:\n%s", want.pathname, css)
				}
			}
			// Two pages with the same kinds of elements have one sheet.
			if a, b := s.page(t, "/guide/").CSS, s.page(t, "/guide/more/").CSS; a.Blob != b.Blob || a.Pages != 2 {
				t.Errorf("/guide/ and /guide/more/: blobs %+v, %+v", a, b)
			}
		})
	}
}

// TestInline: the rule of `--inline` (builder.md, *Packaging*).
func TestInline(t *testing.T) {
	files := func(s site) []string {
		var out []string
		for file := range s.files {
			if strings.HasPrefix(file, assets+"/") && file != reportFile {
				out = append(out, file)
			}
		}
		return out
	}
	t.Run("always", func(t *testing.T) {
		s := load(t, "-p", "site", "--inline", "always")
		if got := files(s); len(got) != 0 {
			t.Errorf("files: %q", got)
		}
		for _, b := range s.report.Blobs {
			if b.Delivery != "inline" || b.File != "" {
				t.Errorf("blob %+v", b)
			}
		}
	})
	t.Run("never", func(t *testing.T) {
		s := load(t, "-p", "site", "--inline", "never")
		if got := files(s); len(got) != len(s.report.Blobs) {
			t.Errorf("%d files for %d blobs: %q", len(got), len(s.report.Blobs), got)
		}
		for _, b := range s.report.Blobs {
			want := assets + "/page-" + b.Hash + "." + b.Kind
			if b.Delivery != "file" || b.File != want || len(s.files[want]) != b.Size.Raw {
				t.Errorf("blob %+v: want the file %s", b, want)
			}
		}
		for _, p := range s.report.Pages {
			if html := s.files[p.Output]; strings.Contains(html, "<style") || strings.Contains(html, `<script type="module">`) {
				t.Errorf("%s holds a blob", p.Output)
			}
		}
	})
	t.Run("auto", func(t *testing.T) {
		s := load(t, "-p", "site")
		inline, file, shared := 0, 0, 0
		for _, b := range s.report.Blobs {
			// A file when it serves two pages or more and is larger than
			// a request, as it is sent: gzipped.
			want := "inline"
			if len(b.Pages) > 1 && b.Size.Gzip > 250 {
				want = "file"
				file++
			} else {
				inline++
			}
			if b.Delivery != want {
				t.Errorf("blob %s (%d B, %d gzipped, %d pages) is %s, want %s", b.Hash, b.Size.Raw, b.Size.Gzip, len(b.Pages), b.Delivery, want)
			}
			if len(b.Pages) > 1 && want == "inline" {
				shared++
			}
		}
		if inline == 0 || file == 0 || shared == 0 {
			t.Errorf("the fixture has %d inlined blobs (%d of them shared) and %d files: it tests one side of the rule only", inline, shared, file)
		}
	})
}

// TestControl: `--no-specialize` (builder.md, *The control*) — the same
// HTML, one CSS bundle that is not pruned, one script that holds every
// behaviour of the site with every flag on.
func TestControl(t *testing.T) {
	built := load(t, "-p", "site", "--inline", "never")
	control := load(t, "-p", "site", "--no-specialize", "--inline", "never")
	if control.report.Specialize || !built.report.Specialize {
		t.Errorf("specialize: %v, %v", control.report.Specialize, built.report.Specialize)
	}
	if len(control.report.Blobs) != 2 {
		t.Fatalf("the control has %d blobs: %+v", len(control.report.Blobs), control.report.Blobs)
	}
	for _, b := range control.report.Blobs {
		if b.File != assets+"/site-"+b.Hash+"."+b.Kind || len(b.Pages) != len(control.report.Pages) {
			t.Errorf("blob %+v", b)
		}
	}
	for _, p := range control.report.Pages {
		css, js := control.css(t, p.Pathname), control.js(t, p.Pathname)
		// Every component's rules, the page's or not; the rule of no page.
		for _, class := range []string{".rg-button", ".rg-dialog", ".rg-menu", ".rg-sidemenu", ".site-header", ".prose", ".never", "@keyframes never"} {
			if !strings.Contains(css, class) {
				t.Errorf("%s: the control's CSS has no %s", p.Pathname, class)
			}
		}
		for _, marker := range []string{overlays, invokers, menuKeys, typeahead, `"/actions/"`} {
			if !strings.Contains(js, marker) {
				t.Errorf("%s: the control's script has no %s", p.Pathname, marker)
			}
		}
		if p.Modules != nil || p.Styles != nil {
			t.Errorf("%s: the control reports modules or styles", p.Pathname)
		}
		// The same HTML: only the two elements packaging adds differ.
		own := built.page(t, p.Pathname)
		if p.HTML != own.HTML {
			t.Errorf("%s: HTML %+v, and %+v when specialized", p.Pathname, p.HTML, own.HTML)
		}
		// What awareness is worth: never more than the control.
		if own.CSS.Raw >= p.CSS.Raw || own.JS != nil && own.JS.Raw >= p.JS.Raw {
			t.Errorf("%s: CSS %d B against the control's %d B, JS %+v against %d B", p.Pathname, own.CSS.Raw, p.CSS.Raw, own.JS, p.JS.Raw)
		}
	}
}

// TestBase: `--base` (builder.md, *Packaging*) — the page is checked as
// rendered, then every root-relative `href` and the URLs of the build's own
// files get the base; the control's table is keyed with it.
func TestBase(t *testing.T) {
	plain := load(t, "-p", "site", "--inline", "never")
	based := load(t, "-p", "site", "--inline", "never", "--base", "docs")
	if based.report.Base != "/docs/" {
		t.Errorf("base: %q", based.report.Base)
	}
	if !maps.Equal(tree(t, filepath.Join("testdata", "golden", "base", "out")), based.files) && !*updateGolden {
		t.Errorf("`--base docs` and `--base /docs` build different sites")
	}
	href := regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	for _, p := range based.report.Pages {
		html, without := based.files[p.Output], plain.files[p.Output]
		var links []string
		for _, m := range href.FindAllStringSubmatch(html, -1) {
			if link := m[1]; strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "/docs/") {
				t.Errorf("%s: %q is outside the base", p.Output, link)
			} else {
				links = append(links, link)
			}
		}
		if len(links) < 8 {
			t.Errorf("%s: links %q", p.Output, links)
		}
		// Nothing else of the page changes: without the base it is the page
		// of the build that has none.
		if got := strings.ReplaceAll(html, `="/docs/`, `="/`); got != without {
			t.Errorf("%s: with the base taken out it is not the page built without one:\n%s\n---\n%s", p.Output, got, without)
		}
	}
	// As written: a query and a fragment stay, `/guide` is not given a slash,
	// a URL with a scheme and a fragment alone are left alone.
	for file, want := range map[string]string{
		"plain/index.html":      `href="/docs/guide/more/?from=plain#top"`,
		"guide/more/index.html": `<a href="/docs/guide">`,
		"guide/index.html":      `<a href="#intro">`,
		"index.html":            `<a href="https://example.com/">`,
		"links/index.html":      `<a href="/docs/robots.txt">`,
	} {
		if !strings.Contains(based.files[file], want) {
			t.Errorf("%s has no %s:\n%s", file, want, based.files[file])
		}
	}
	// `public/` is under the base because the site is: its files are where
	// they were.
	if based.files["robots.txt"] == "" || based.files["favicon.svg"] == "" {
		t.Errorf("public/ is not copied: %v", slices.Sorted(maps.Keys(based.files)))
	}
	control := load(t, "-p", "site", "--no-specialize", "--base", "/docs/")
	if js := control.js(t, "/actions/"); !strings.Contains(js, `"/docs/actions/"`) || strings.Contains(js, `"/actions/"`) {
		t.Errorf("the control's table is not keyed with the base:\n%s", js)
	}
	// A base given percent-encoded is written as given, and is the
	// directory it encodes: the control's table is keyed by its name, which
	// is what the script makes of `location.pathname` (builder.md, *The
	// control*).
	for _, base := range []string{"/caf%C3%A9/", "/café/"} {
		encoded := load(t, "-p", "site", "--no-specialize", "--base", base, "--inline", "never")
		if js := encoded.js(t, "/actions/"); !strings.Contains(js, `"/café/actions/"`) || strings.Contains(js, "%C3") {
			t.Errorf("--base %s: the control's table is not keyed by the directory's name:\n%s", base, js)
		}
		if html := encoded.files["index.html"]; !strings.Contains(html, `<a href="`+base+`guide/">`) || !strings.Contains(html, `src="`+base+`_rg/site-`) {
			t.Errorf("--base %s: the links of the page:\n%s", base, html)
		}
	}
}

// TestDocument: packaging writes into React's HTML and leaves the rest of it
// as it is (html.go).
func TestDocument(t *testing.T) {
	tests := []struct {
		name                  string
		src, base, head, body string
		want                  string
	}{
		{
			"the end of head and of body",
			`<html><head><title>t</title></head><body><p>x</p></body></html>`, "/", "<style>a{}</style>", "<script>1</script>",
			`<html><head><title>t</title><style>a{}</style></head><body><p>x</p><script>1</script></body></html>`,
		},
		{
			"nothing to add",
			`<html><head></head><body></body></html>`, "/", "", "",
			`<html><head></head><body></body></html>`,
		},
		{
			"a closing tag in a script's text, or in a title's, is text",
			`<html><head><title></head></title><script>"</head></body>"</script></head><body></body></html>`, "/", "<style></style>", "<script></script>",
			`<html><head><title></head></title><script>"</head></body>"</script><style></style></head><body><script></script></body></html>`,
		},
		{
			"no head, no body: the browser makes them",
			`<html lang="en"><p>x</p>`, "/", "<style></style>", "<script></script>",
			`<html lang="en"><style></style><p>x</p><script></script>`,
		},
		{
			"the base before every root-relative href",
			`<html><head><link rel="icon" href="/favicon.svg"/></head><body><a class="a" href="/guide/?q=1#top">g</a><a href="/">home</a><area href='/x'><a href=/y>y</a></body></html>`, "/docs/", "", "",
			`<html><head><link rel="icon" href="/docs/favicon.svg"/></head><body><a class="a" href="/docs/guide/?q=1#top">g</a><a href="/docs/">home</a><area href='/docs/x'><a href=/docs/y>y</a></body></html>`,
		},
		{
			"another site, a relative link, a fragment, <base>: as they are",
			`<html><head><base href="/app/"/></head><body><a href="//host/x">1</a><a href="/\host">2</a><a href="https://h/x">3</a><a href="x/">4</a><a href="../x">5</a><a href="#x">6</a><a href="?a=/b">7</a><a href="mailto:a@b">8</a><a href>9</a><a>10</a></body></html>`, "/docs/", "", "",
			`<html><head><base href="/app/"/></head><body><a href="//host/x">1</a><a href="/\host">2</a><a href="https://h/x">3</a><a href="x/">4</a><a href="../x">5</a><a href="#x">6</a><a href="?a=/b">7</a><a href="mailto:a@b">8</a><a href>9</a><a>10</a></body></html>`,
		},
		{
			"as a URL parser reads it: space before, a backslash, a character reference",
			`<html><body><a href=" /a">1</a><a href="\b">2</a><a href="&#47;c&amp;d">3</a><a href=&#x2F;e>4</a><a data-href="/f" HREF="/g" href="/h">5</a></body></html>`, "/docs/", "", "",
			`<html><body><a href=" /docs/a">1</a><a href="/docs\b">2</a><a href="/docs/c&amp;d">3</a><a href="/docs/e">4</a><a data-href="/f" HREF="/docs/g" href="/h">5</a></body></html>`,
		},
		{
			"an href in a script's text is text",
			`<html><body><script>'<a href="/x">'</script><textarea><a href="/y"></textarea></body></html>`, "/docs/", "", "",
			`<html><body><script>'<a href="/x">'</script><textarea><a href="/y"></textarea></body></html>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := document(tt.src, tt.base, tt.head, tt.body); got != tt.want {
				t.Errorf("\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestPack: identical content is one blob, by kind; the rule of `auto`; a
// blob that cannot stand in its element is a file.
func TestPack(t *testing.T) {
	// The rule of `auto` counts what is sent: 1200 B of one rule repeated
	// are under 250 B gzipped, 200 rules that differ are over.
	repeated := strings.Repeat("a{color:red}", 100)
	var rules strings.Builder
	for i := range 200 {
		fmt.Fprintf(&rules, ".c%x{order:%d}", i*40503%65536, i*7919%1000)
	}
	big := rules.String()
	if raw, sent := sizeOf(repeated), sizeOf(big); raw.Gzip > request || sent.Gzip <= request {
		t.Fatalf("the blobs do not straddle the rule: %+v, %+v", raw, sent)
	}
	pages := []built{
		{css: "p{}", js: "a()"},
		{css: "p{}"},
		{css: big, js: "a()"},
		{css: big, js: `"</script>"`},
		{js: "p{}"}, // the same text, another kind
		{css: `a{content:"</STYLE>"}`},
		{},
		{css: repeated},
		{css: repeated},
	}
	type delivery struct {
		kind  string
		pages []int
		file  bool
	}
	tests := []struct {
		inline string
		want   []delivery
	}{
		{InlineAuto, []delivery{{"css", []int{0, 1}, false}, {"js", []int{0, 2}, false}, {"css", []int{2, 3}, true}, {"js", []int{3}, true}, {"js", []int{4}, false}, {"css", []int{5}, true}, {"css", []int{7, 8}, false}}},
		{InlineAlways, []delivery{{"css", []int{0, 1}, false}, {"js", []int{0, 2}, false}, {"css", []int{2, 3}, false}, {"js", []int{3}, true}, {"js", []int{4}, false}, {"css", []int{5}, true}, {"css", []int{7, 8}, false}}},
		{InlineNever, []delivery{{"css", []int{0, 1}, true}, {"js", []int{0, 2}, true}, {"css", []int{2, 3}, true}, {"js", []int{3}, true}, {"js", []int{4}, true}, {"css", []int{5}, true}, {"css", []int{7, 8}, true}}},
	}
	for _, tt := range tests {
		t.Run(tt.inline, func(t *testing.T) {
			blobs, of, err := pack(pages, tt.inline)
			if err != nil {
				t.Fatal(err)
			}
			var got []delivery
			for _, b := range blobs {
				got = append(got, delivery{b.kind, b.pages, b.file})
			}
			if len(got) != len(tt.want) {
				t.Fatalf("blobs: %+v", got)
			}
			for i := range got {
				if got[i].kind != tt.want[i].kind || !slices.Equal(got[i].pages, tt.want[i].pages) || got[i].file != tt.want[i].file {
					t.Errorf("blob %d: %+v, want %+v", i, got[i], tt.want[i])
				}
			}
			if of[0] != [2]int{0, 1} || of[1] != [2]int{0, -1} || of[6] != [2]int{-1, -1} {
				t.Errorf("of: %v", of)
			}
		})
	}
	// A file's name is its kind of build and its content: not its pages.
	blobs, _, _ := pack(pages, InlineNever)
	if got := blobs[0].path(false); got != "_rg/page-"+blobs[0].hash+".css" || len(blobs[0].hash) != 8 {
		t.Errorf("path: %s", got)
	}
	if got := blobs[1].path(true); got != "_rg/site-"+blobs[1].hash+".js" {
		t.Errorf("path: %s", got)
	}
	if got := blobs[0].tag("/docs/", false); got != `<link rel="stylesheet" href="/docs/_rg/page-`+blobs[0].hash+`.css">` {
		t.Errorf("tag: %s", got)
	}
}

// TestNormalBase: `--base` as it may be written.
func TestNormalBase(t *testing.T) {
	for base, want := range map[string]string{
		"": "/", "/": "/", "docs": "/docs/", "/docs": "/docs/", "/docs/": "/docs/", "docs/": "/docs/", "/a/b": "/a/b/", "/v1.2/~me/": "/v1.2/~me/", "/se%C3%B1or": "/se%C3%B1or/",
		"/señor/": "/señor/", "/a%20b%25/": "/a%20b%25/", "/caf%c3%a9": "/caf%c3%a9/",
	} {
		if got, ok := NormalBase(base); !ok || got != want {
			t.Errorf("%q: %q, %v; want %q", base, got, ok, want)
		}
	}
	for _, base := range []string{
		"https://example.com/docs/", "//host/docs", "/docs?x", "/docs#x", "/a//b", "/a b/", `/a"b/`, "/a/../b", "/./", `\docs`, "/a&b/", "/<a>/",
		// A `%` that encodes nothing, or no text: the control's script
		// decodes the pathname, and `decodeURIComponent` throws on it. And
		// what a browser reads as a dot segment.
		"/100%/", "/a%zz/", "/a%2", "/%ff/", "/caf%C3/", "/%2e%2e/", "/a/%2E/b",
	} {
		if got, ok := NormalBase(base); ok {
			t.Errorf("%q is taken for a base: %q", base, got)
		}
	}
}

// TestRoutes: `index.rtsx` or `index.tsx` of a directory is a page; anything
// else is a module of one (builder.md, *Routes*).
func TestRoutes(t *testing.T) {
	dir := t.TempDir()
	for _, file := range []string{
		"index.rtsx", "guide/index.rtsx", "guide/install.rtsx", "guide/index.css", "reference/cli/index.tsx",
		"both/index.rtsx", "both/index.tsx", "empty/readme.md", "señor/index.rtsx", "deep/er/est/index.tsx", "not/index.ts", "not/index.jsx",
	} {
		if err := writeFile(dir, file, nil); err != nil {
			t.Fatal(err)
		}
	}
	// A directory named as a page's file is no page.
	if err := os.MkdirAll(filepath.Join(dir, "dir", "index.rtsx"), 0o755); err != nil {
		t.Fatal(err)
	}
	routes, err := findRoutes(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{
		{"/", "index.rtsx"}, {"/both/", "both/index.rtsx"}, {"/deep/er/est/", "deep/er/est/index.tsx"},
		{"/guide/", "guide/index.rtsx"}, {"/reference/cli/", "reference/cli/index.tsx"}, {"/señor/", "señor/index.rtsx"},
	}
	var got [][2]string
	for _, r := range routes {
		got = append(got, [2]string{r.Pathname, strings.TrimPrefix(r.File, filepath.ToSlash(dir)+"/")})
	}
	if !slices.Equal(got, want) {
		t.Errorf("routes:\n got %q\nwant %q", got, want)
	}
	if _, err := findRoutes(filepath.Join(dir, "index.rtsx")); err == nil {
		t.Error("a file is taken for a directory of pages")
	}
	if _, err := findRoutes(filepath.Join(dir, "nowhere")); err == nil {
		t.Error("a directory that is not there is taken for one of pages")
	}
}

// TestErrors: what stops a build — the diagnostics are printed as `check`
// prints them, the status is 1, and nothing is written.
func TestErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout string // the whole of it
	}{
		{
			"the project does not check",
			[]string{"-p", "types"},
			"types/pages/index.rtsx(2,7): error TS2322: Type 'string' is not assignable to type 'number'.\n",
		},
		{
			"a broken idref stops the build, with the page named",
			[]string{"-p", "bad", "--pages", "bad/idref"},
			"bad/idref/index.rtsx: error idref-not-found: Page /: `for=\"email\"` on `<label>` names no element of the page\n",
		},
		{
			"a link to a page that is not there",
			[]string{"-p", "bad", "--pages", "bad/link"},
			"bad/link/index.rtsx: error link-not-found: Page /: `href=\"/guide/\"` on `<a>` is neither a page nor a file of the output\n",
		},
		{
			"with a base the link is still checked as written",
			[]string{"-p", "bad", "--pages", "bad/link", "--base", "/guide/"},
			"bad/link/index.rtsx: error link-not-found: Page /: `href=\"/guide/\"` on `<a>` is neither a page nor a file of the output\n",
		},
		{
			"a shell error, where it was thrown, with the components",
			[]string{"-p", "bad", "--pages", "bad/shell"},
			"bad/shell/index.rtsx(8,38): error shell-error: no price for enterprise\n" +
				"  bad/shell/index.rtsx:16:7 - in Price\n" +
				"  bad/shell/index.rtsx:12:1 - in ShellPage\n",
		},
		{
			"a stylesheet that is not there, at its import",
			[]string{"-p", "bad", "--pages", "bad/css"},
			"bad/css/index.rtsx(4,8): error css-bundle: Could not resolve \"./missing.css\"\n",
		},
		{
			"a behaviour that is not there, an element that is not on the page",
			[]string{"-p", "bad", "--pages", "bad/mount"},
			"bad/mount/index.rtsx: error mount-not-found: Page /: `mount(\"./behaviors/nope\")`: the module does not resolve from the project directory\n" +
				"bad/mount/index.rtsx: error mount-no-element: Page /: `mount(\"./behaviors/nope\")`: no element of the page has `id=\"w1\"`\n",
		},
		{
			"the same in the control",
			[]string{"-p", "bad", "--pages", "bad/mount", "--no-specialize"},
			"bad/mount/index.rtsx: error mount-not-found: Page /: `mount(\"./behaviors/nope\")`: the module does not resolve from the project directory\n" +
				"bad/mount/index.rtsx: error mount-no-element: Page /: `mount(\"./behaviors/nope\")`: no element of the page has `id=\"w1\"`\n",
		},
		{
			"no page",
			[]string{"-p", "bad", "--pages", "bad/segments"},
			"error pages-not-found: <testdata>/bad/segments holds no page: no `index.rtsx` or `index.tsx`\n",
		},
		{
			"no pages directory: the default is `pages`, next to the tsconfig",
			[]string{"-p", "bad"},
			"error pages-not-found: <testdata>/bad/pages is not a directory\n",
		},
		{
			"a file of public/ where the build writes",
			[]string{"-p", "bad", "--pages", "bad/conflict/pages"},
			"bad/conflict/public/_rg/mine.css: error public-conflict: `_rg/` is the builder's: a file of `public/` cannot be there\n" +
				"bad/conflict/public/guide: error public-conflict: The page /guide/ is written to `guide/index.html`: `guide` is a directory of the output, and cannot be a file of `public/`\n" +
				"bad/conflict/public/index.html: error public-conflict: The page / is written to `index.html`: a file of `public/` cannot be there\n",
		},
		{
			// As `check -p types/tsconfig.solution.json` prints it.
			"the project does not check, through the tsconfig that references it",
			[]string{"-p", "types/tsconfig.solution.json"},
			"types/pages/index.rtsx(2,7): error TS2322: Type 'string' is not assignable to type 'number'.\n",
		},
		{
			"index.rtsx and index.tsx side by side: check's ambiguous-module",
			[]string{"-p", "both"},
			"both/pages/index.rtsx(1,1): error ambiguous-module: `index.tsx` and `index.rtsx` side by side: an import of `./index` is ambiguous\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "dist")
			stdout, stderr, status := run(t, append(tt.args, "--out", out)...)
			stdout = strings.ReplaceAll(stdout, filepath.ToSlash(real(testdata(t))), "<testdata>")
			if status != 1 || stderr != "" || stdout != tt.stdout {
				t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, tt.stdout)
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("a build that stopped wrote %v", slices.Sorted(maps.Keys(tree(t, out))))
			}
		})
	}
}

// TestFailedBuildKeepsOutput: the output directory is emptied when the build
// writes, not before — a build that stops leaves the last one.
func TestFailedBuildKeepsOutput(t *testing.T) {
	out, _ := buildSite(t, "-p", "site")
	before := tree(t, out)
	if _, _, status := run(t, "-p", "bad", "--pages", "bad/link", "--out", out); status != 1 {
		t.Fatalf("status %d", status)
	}
	if !maps.Equal(before, tree(t, out)) {
		t.Error("a build that stopped changed the output directory")
	}
}

// TestWarning: a warning is printed, and the site is built — with a `url()`
// of its CSS as it is written.
func TestWarning(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dist")
	stdout, stderr, status := run(t, "-p", "bad", "--pages", "bad/warning", "--out", out, "--inline", "always")
	want := "bad/warning/page.css(2,3): warning css-warning: \"widht\" is not a known CSS property\n  Did you mean \"width\" instead?\n1 page written to " + filepath.ToSlash(real(out)) + "\n"
	if status != 0 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout:\n%s\nwant:\n%s", status, stderr, stdout, want)
	}
	if html := tree(t, out)["index.html"]; !strings.Contains(html, "<style>.lead{widht:2px;font-weight:600;background:url(/not/there.png)}</style></head>") {
		t.Errorf("the page: %s", html)
	}
}

// TestOut: `--out` is emptied, so it has to be the builder's to empty
// (builder.md, *The output directory*).
//
// Every build here that must be refused is one that would build: were the
// rule to break, it would empty its `--out`. So the project is a copy, in a
// directory of the test's own, and whatever is refused is inside it; what is
// not — the root, the repository's fixture — is asked of CheckOut alone, and
// never built into.
func TestOut(t *testing.T) {
	work := scratch(t, []string{"site"}, map[string]string{
		"stray/notes.txt": "mine",
		"a-file":          "",
		// A directory with an `_rg/` and an `index.html` that the builder
		// did not write: not enough to be taken for an output.
		"lookalike/index.html":            "mine",
		"lookalike/_rg/page-00000000.css": "mine",
	})
	stray := filepath.Join(work, "stray")
	link := filepath.Join(work, "link")
	if err := os.Symlink(filepath.Join(work, "site"), link); err != nil {
		t.Fatal(err)
	}

	type refusal struct{ name, out, why string }
	refused := []refusal{
		{"the project directory", "site", "holds the project"},
		{"an ancestor of it", ".", "holds the project"},
		{"the pages", "site/pages", "holds the pages"},
		{"inside the pages", "site/pages/dist", "is inside the pages"},
		{"public/", "site/public", "holds `public/`"},
		{"inside public/", "site/public/a/b", "is inside `public/`"},
		{"a file", "a-file", "is not a directory"},
		{"a directory of something else", stray, "is not empty and is not an output of `reactogenic build`"},
		{"one that only looks like an output", "lookalike", "is not empty and is not an output of `reactogenic build`"},
		{"the project through a link", link, "holds the project"},
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		// Where the file system folds case, another case is the same directory.
		if _, err := os.Stat(filepath.Join(work, "SITE")); err == nil {
			refused = append(refused, refusal{"the project in another case", "SITE", "holds the project"})
		}
	}
	// The project, and the directories that are not the builder's.
	held := func() map[string]string {
		files := map[string]string{}
		for _, dir := range []string{"site", "stray", "lookalike"} {
			for file, content := range tree(t, filepath.Join(work, dir)) {
				files[dir+"/"+file] = content
			}
		}
		for _, file := range []string{"a-file", "node_modules", "link"} {
			if _, err := os.Lstat(filepath.Join(work, file)); err == nil {
				files[file] = "there"
			}
		}
		return files
	}
	before := held()
	if before["stray/notes.txt"] != "mine" || before["lookalike/index.html"] != "mine" || before["site/tsconfig.json"] == "" || len(before) < 20 {
		t.Fatalf("the test's directory: %v", slices.Sorted(maps.Keys(before)))
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, status := runIn(t, work, "-p", "site", "--out", tt.out)
			if status != 2 || stdout != "" || !strings.HasPrefix(stderr, "reactogenic build: --out ") || !strings.Contains(stderr, tt.why) {
				t.Errorf("status %d, stdout %q, stderr %q; want a refusal that says %q", status, stdout, stderr, tt.why)
			}
		})
	}
	if !maps.Equal(before, held()) {
		t.Fatal("a refused build changed the project, or a directory that is not the builder's")
	}

	// Outside the test's directory: the flags are resolved as the command
	// resolves them, and the rule is asked — nothing is built.
	data := testdata(t)
	for _, tt := range []struct {
		refusal
		cwd string
		p   string
	}{
		{refusal{"the root", string(filepath.Separator), "holds the project"}, work, "site"},
		{refusal{"the root, of the repository's fixture", string(filepath.Separator), "holds the project"}, data, "site"},
		{refusal{"the repository's fixture", "site", "holds the project"}, data, "site"},
		{refusal{"the fixtures' directory", ".", "holds the project"}, data, "site"},
		{refusal{"the repository's fixture, from the copy", filepath.Join(data, "site", "pages"), "is not empty and is not an output of `reactogenic build`"}, work, "site"},
		{refusal{"what holds the test's directory", filepath.Dir(work), "holds the project"}, work, "site"},
	} {
		t.Run("asked: "+tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			opts, _, status := options([]string{"-p", tt.p, "--out", tt.out}, tt.cwd, &stderr)
			if status != 0 {
				t.Fatalf("status %d: %s", status, stderr.String())
			}
			if err := CheckOut(opts); err == nil || !strings.HasPrefix(err.Error(), "--out ") || !strings.Contains(err.Error(), tt.why) {
				t.Errorf("CheckOut: %v; want a refusal that says %q", err, tt.why)
			}
		})
	}

	// The build asks again where it empties: Run is safe for a caller that
	// did not ask, and for a directory that changed while the site was built.
	t.Run("Run refuses what CheckOut refuses", func(t *testing.T) {
		opts, _, status := options([]string{"-p", "site", "--out", stray}, work, io.Discard)
		if status != 0 {
			t.Fatalf("status %d", status)
		}
		reports, written, err := Run(opts)
		if err == nil || written != nil || !strings.Contains(err.Error(), "is not empty and is not an output of `reactogenic build`") {
			t.Errorf("Run: %v, %v, %v", reports, written, err)
		}
		if got := tree(t, stray); len(got) != 1 || got["notes.txt"] != "mine" {
			t.Errorf("Run changed a directory that is not the builder's: %v", got)
		}
	})

	t.Run("an output of the builder is emptied", func(t *testing.T) {
		out, _ := buildSite(t, "-p", "site")
		built := tree(t, out)
		// What an earlier output left, a build that was killed among it.
		for _, file := range []string{"stale.html", "old/index.html", "_rg/page-00000000.css", "_rg/old/x.css", ".rg-1234/index.html", ".git/HEAD"} {
			if err := writeFile(out, file, []byte("stale")); err != nil {
				t.Fatal(err)
			}
		}
		if stdout, stderr, status := run(t, "-p", "site", "--out", out); status != 0 {
			t.Fatalf("status %d\n%s%s", status, stdout, stderr)
		}
		again := tree(t, out)
		// A `.git` is not the build's to remove.
		if again[".git/HEAD"] != "stale" {
			t.Errorf(".git is gone: %v", slices.Sorted(maps.Keys(again)))
		}
		delete(again, ".git/HEAD")
		if !maps.Equal(built, again) {
			t.Errorf("the second build's output: %v, the first's: %v", slices.Sorted(maps.Keys(again)), slices.Sorted(maps.Keys(built)))
		}
	})
	t.Run("an empty directory, and one that is not there", func(t *testing.T) {
		empty := filepath.Join(work, "empty")
		if err := os.MkdirAll(filepath.Join(empty, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, out := range []string{empty, filepath.Join(work, "not", "there", "yet")} {
			if stdout, stderr, status := run(t, "-p", "site", "--out", out); status != 0 {
				t.Errorf("%s: status %d\n%s%s", out, status, stdout, stderr)
			}
			if tree(t, out)[reportFile] == "" {
				t.Errorf("%s: nothing built", out)
			}
		}
	})
}

// TestUsage: a usage error is status 2, on stderr, and nothing is built.
func TestUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		says string
	}{
		{"no tsconfig", []string{"-p", "site/pages"}, "no tsconfig at "},
		{"no such project", []string{"-p", "nowhere/tsconfig.json"}, "no tsconfig at "},
		{"--inline", []string{"-p", "site", "--inline", "sometimes"}, "--inline is auto, always or never"},
		{"--base with a host", []string{"-p", "site", "--base", "https://example.com/docs/"}, "--base is the path the site is served under"},
		{"--base with a % that encodes nothing", []string{"-p", "site", "--base", "/100%/"}, "--base is the path the site is served under"},
		{"an argument", []string{"-p", "site", "pages"}, `unexpected argument "pages"`},
		{"an unknown flag", []string{"-p", "site", "--minify"}, "flag provided but not defined: -minify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, status := run(t, append(tt.args, "--out", filepath.Join(t.TempDir(), "dist"))...)
			if status != 2 || stdout != "" || !strings.Contains(stderr, tt.says) {
				t.Errorf("status %d, stdout %q, stderr %q; want %q", status, stdout, stderr, tt.says)
			}
		})
	}
}

// TestBinary runs the built binary — `go build`, then `reactogenic build` as
// a user runs it, in the fixture's directory with the defaults for the
// tsconfig and the pages — and compares what it wrote with the golden output
// of the same build made in this process.
func TestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	data := testdata(t)
	name := "reactogenic"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	compile := exec.Command("go", "build", "-trimpath", "-o", binary, "github.com/reactogenic/reactogenic/go/cmd/reactogenic")
	compile.Dir = data
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	command := func(dir string, args ...string) (stdout, stderr string, status int) {
		t.Helper()
		var out, errs bytes.Buffer
		cmd := exec.Command(binary, args...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, &out, &errs
		if err := cmd.Run(); err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("%v: %v", args, err)
			}
			status = exit.ExitCode()
		}
		return out.String(), errs.String(), status
	}

	out := filepath.Join(t.TempDir(), "dist")
	stdout, stderr, status := command(filepath.Join(data, "site"), "build", "--out", out, "--report")
	if status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	golden(t, "auto/out", tree(t, out))
	golden(t, "auto/print", map[string]string{"stdout.txt": strings.ReplaceAll(stdout, filepath.ToSlash(real(out)), "<out>")})

	// The control, with a base, from another directory.
	out = filepath.Join(t.TempDir(), "dist")
	if stdout, stderr, status := command(data, "build", "-p", "site/tsconfig.json", "--out", out, "--base", "/docs/", "--no-specialize"); status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	}
	golden(t, "base-control/out", tree(t, out))

	// Diagnostics: status 1, on stdout, paths from the working directory.
	stdout, stderr, status = command(data, "build", "-p", "bad", "--pages", "bad/idref", "--out", filepath.Join(t.TempDir(), "dist"))
	if want := "bad/idref/index.rtsx: error idref-not-found: Page /: `for=\"email\"` on `<label>` names no element of the page\n"; status != 1 || stderr != "" || stdout != want {
		t.Errorf("status %d, stderr %q, stdout %q", status, stderr, stdout)
	}
	// A usage error: status 2, on stderr. Asked of a copy of the project: a
	// binary that did not refuse would empty its `--out`.
	work := scratch(t, []string{"site"}, nil)
	if stdout, stderr, status := command(work, "build", "-p", "site", "--out", "site"); status != 2 || stdout != "" || !strings.Contains(stderr, "holds the project") {
		t.Errorf("status %d, stderr %q, stdout %q", status, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(work, "site", "tsconfig.json")); err != nil {
		t.Fatalf("the refused build emptied the project: %v", err)
	}
	// The copy reaches its packages through a link out of the project, and
	// is on another path: its output is the same bytes, the report too.
	out = filepath.Join(work, "site", "dist")
	if stdout, stderr, status := command(filepath.Join(work, "site"), "build", "--report"); status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	} else {
		golden(t, "auto/out", tree(t, out))
		// `dist`, next to the tsconfig, is named from the working directory.
		golden(t, "auto/print", map[string]string{"stdout.txt": strings.Replace(stdout, "written to dist\n", "written to <out>\n", 1)})
	}
	// A tsconfig that only references the project's, as `check` reads it.
	if stdout, stderr, status := command(filepath.Join(work, "site"), "build", "-p", "tsconfig.solution.json", "--report"); status != 0 || stderr != "" {
		t.Fatalf("status %d\n%s%s", status, stdout, stderr)
	} else {
		golden(t, "auto/out", tree(t, out))
	}
	if _, stderr, status := command(data, "build", "--help"); status != 2 || !strings.Contains(stderr, "usage: reactogenic build") {
		t.Errorf("--help: status %d, stderr %q", status, stderr)
	}
	if stdout, _, status := command(data, "--help"); status != 0 || !strings.Contains(stdout, "reactogenic build [-p tsconfig.json|dir]") {
		t.Errorf("the usage does not name build: status %d\n%s", status, stdout)
	}
}
