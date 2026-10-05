package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDocsSite builds the documentation site (site/, plan.md RGP2-040) — the
// one site the phase is measured on — in every mode, and holds it to what the
// plan says of it: four pages — one variant, `index`, of each of its four
// routes: the ten other `.rtsx` files of `pages/` are segments, mounted —
// nothing reported, each page pruned and shipping the behaviours its
// components mounted and no other script. No golden: the site's text is not
// this package's to pin.
func TestDocsSite(t *testing.T) {
	repo, _ := filepath.Abs("../../..")
	config := filepath.Join(repo, "site", "tsconfig.json")
	if _, err := os.Stat(filepath.Join(repo, "site", "node_modules", "@reactogenic", "ui", "package.json")); err != nil {
		t.Fatalf("the docs site builds with the workspace's packages, and site/node_modules has no @reactogenic/ui: run `pnpm install` in %s", repo)
	}
	const (
		ui      = "@reactogenic/ui/behaviors/"
		typeOn  = "RG_MENU_TYPEAHEAD"
		docsOut = "dist"
	)
	// What each page mounts, in render order (components.md, *Behaviours*).
	mounts := map[string][]string{
		"/":               {ui + "overlays", ui + "invokers"},
		"/guide/":         {ui + "overlays"},
		"/reference/cli/": {ui + "overlays"},
		"/syntax/":        {ui + "overlays", ui + "menu-keys", ui + "invokers"},
	}
	for _, mode := range [][]string{
		nil,
		{"--inline", "always"},
		{"--inline", "never"},
		{"--base", "/reactogenic/"},
		{"--no-specialize"},
		{"--no-specialize", "--base", "/reactogenic/"},
	} {
		t.Run(strings.Join(append([]string{"default"}, mode...), " "), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), docsOut)
			stdout, stderr, status := runIn(t, repo, append([]string{"-p", config, "--out", out}, mode...)...)
			if status != 0 || stderr != "" {
				t.Fatalf("status %d\n%s%s", status, stdout, stderr)
			}
			// Nothing reported, not a warning: the one line is the last.
			if !strings.HasPrefix(stdout, "4 pages written to ") || strings.Count(stdout, "\n") != 1 {
				t.Errorf("the build has something to say:\n%s", stdout)
			}
			files := tree(t, out)
			var report Report
			if err := json.Unmarshal([]byte(files[reportFile]), &report); err != nil {
				t.Fatal(err)
			}
			control := slices.Contains(mode, "--no-specialize")
			var pathnames []string
			for _, p := range report.Pages {
				pathnames = append(pathnames, p.Pathname)
				var mounted []string
				for _, m := range p.Mounts {
					mounted = append(mounted, m.Module)
					if (m.Module == ui+"menu-keys") != (m.Flags[typeOn]) {
						t.Errorf("%s: %s with the flags %v", p.Pathname, m.Module, m.Flags)
					}
					// The menu that asked for typeahead says so to its
					// behaviour alone: the mount's data, no attribute.
					if want := map[bool]string{true: `{"typeahead":true}`}[m.Module == ui+"menu-keys"]; strings.Join(strings.Fields(string(m.Data)), "") != want {
						t.Errorf("%s: %s with the data %s, want %q", p.Pathname, m.Module, m.Data, want)
					}
				}
				if !slices.Equal(mounted, mounts[p.Pathname]) {
					t.Errorf("%s mounts %q, want %q", p.Pathname, mounted, mounts[p.Pathname])
				}
				// One script, the builder's: the site has none of its own
				// (plan.md, T6), and no handler.
				html := files[p.Output]
				if p.Variant != "index" || p.Path != p.Pathname || p.Output != strings.TrimPrefix(p.Pathname, "/")+"index.html" || strings.Contains(html, "data-typeahead") {
					t.Errorf("%s: variant %s, written to %s", p.Path, p.Variant, p.Output)
				}
				if n := strings.Count(html, "<script"); n != 1 || !strings.Contains(html, `<script type="module"`) || strings.Contains(html, " onclick=") {
					t.Errorf("%s has %d scripts", p.Output, n)
				}
				if control {
					continue
				}
				// Pruned — with the builder's script in the page, which is
				// not a script of the page's own (builder.md, *CSS*).
				if p.Styles == nil || p.Styles.Unpruned || p.Styles.Why != "" || p.Styles.RulesDropped == 0 {
					t.Errorf("%s is not pruned: %+v", p.Pathname, p.Styles)
				}
			}
			if want := []string{"/", "/guide/", "/reference/cli/", "/syntax/"}; !slices.Equal(pathnames, want) {
				t.Fatalf("pages: %q", pathnames)
			}
			// Four documents and no other: a segment is not built to one.
			var documents []string
			for file := range files {
				if strings.HasSuffix(file, ".html") {
					documents = append(documents, file)
				}
			}
			slices.Sort(documents)
			if want := []string{"guide/index.html", "index.html", "reference/cli/index.html", "syntax/index.html"}; !slices.Equal(documents, want) {
				t.Errorf("documents: %q", documents)
			}
			if _, copied := files["favicon.svg"]; !copied || !slices.Contains(report.Public, "favicon.svg") {
				t.Errorf("public/favicon.svg is not in the output: %q", report.Public)
			}
		})
	}
}
