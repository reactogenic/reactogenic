package render

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

// driver runs a render bundle in Node: `node driver.cjs bundle.cjs
// pathname…` prints the answers of `__reactogenic_render_json`, as a JSON
// array. It exits itself: React's server build for Node keeps the process
// alive (research/evaluation.md, Verification, claim 10).
const driver = `const [bundle, ...pathnames] = process.argv.slice(2);
globalThis.console = new Proxy({}, { get: () => () => {} }); // stdout is the answer
require(bundle);
const answers = pathnames.map((pathname) => JSON.parse(globalThis.__reactogenic_render_json(pathname)));
process.stdout.write(JSON.stringify(answers), () => process.exit(0));
`

// node runs a bundle in Node, in a time zone, and returns its answer for each
// route.
func node(t *testing.T, name, zone, code string, routes []Route) []rendered {
	t.Helper()
	bin, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("the differential test renders the fixtures in Node, and there is no `node` on PATH: %v", err)
	}
	dir := t.TempDir()
	script, file := filepath.Join(dir, "driver.cjs"), filepath.Join(dir, name+".cjs")
	if err := os.WriteFile(script, []byte(driver), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{script, file}
	for _, route := range routes {
		args = append(args, route.Pathname)
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "TZ="+zone)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = string(exit.Stderr)
		}
		t.Fatalf("node %s: %v\n%s", name, err, stderr)
	}
	var answers []rendered
	if err := json.Unmarshal(out, &answers); err != nil || len(answers) != len(routes) {
		t.Fatalf("node %s: %v in %.200s", name, err, out)
	}
	return answers
}

// The differential test (plan.md, RGP2-010 and RGP2-011, *Done when*): what
// the embedded engine renders is what React renders in Node — every page of
// the fixtures, the attribute cases of research/evaluation.md among them
// (site/pages/attrs), byte for byte. Three renderings of each page:
//
//   - the engine: the render bundle in modernc.org/quickjs;
//   - the same bundle in Node, in a zone that is not UTC: the engine against
//     V8, and the sandbox's time zone against the machine's;
//   - the oracle in Node, under TZ=UTC: the same pages bundled with the
//     public `react-dom/server` (Node's build of it) and React itself —
//     nothing of the builder's is in it: not its JSX runtime, not its
//     `react`, not its sandbox, not the renderer file it picks and changes.
func TestDifferential(t *testing.T) {
	for _, site := range []string{"site", "bad"} {
		t.Run(site, func(t *testing.T) {
			f := load(t, site)
			bundled, reports := build(f.program, f.routes, f.dir, variant{platform: api.PlatformBrowser})
			if bundled == nil {
				t.Fatalf("no bundle: %q", lines(f.dir, reports))
			}
			oracle, reports := build(f.program, f.routes, f.dir, variant{renderer: "react-dom/server", plain: true, platform: api.PlatformNode})
			if oracle == nil {
				t.Fatalf("no oracle: %q", lines(f.dir, reports))
			}
			for _, builders := range []string{namespace + ":jsx", namespace + ":react", namespace + ":sandbox", staticRenderer} {
				if slices.ContainsFunc(oracle.sites.sources, func(source string) bool { return strings.HasSuffix(source, builders) }) {
					t.Errorf("in the oracle: %s", builders)
				}
			}
			inNode := node(t, "bundle", "Asia/Tokyo", bundled.code, f.routes)
			byReact := node(t, "oracle", "UTC", oracle.code, f.routes)

			e, thrown, err := start(bundled.code, 30*time.Second)
			if err != nil || thrown != nil {
				t.Fatalf("the engine: %v, %+v", err, thrown)
			}
			defer e.close()
			pages := 0
			for i, route := range f.routes {
				inEngine, err := e.render(route.Pathname)
				if err != nil {
					t.Fatal(err)
				}
				if inEngine.Page == nil {
					// A page that throws in the engine throws in V8 — and the
					// same thing, when the builder threw it: the sandbox, the
					// shell rule of the JSX runtime. Each engine names and
					// words its own errors.
					if own := strings.HasPrefix(inEngine.Error.Name, "shell-") || strings.HasPrefix(inEngine.Error.Name, "page-"); inNode[i].Error == nil || own && inNode[i].Error.Name != inEngine.Error.Name {
						t.Errorf("%s: the engine threw %s (%s); Node: %+v", route.Pathname, inEngine.Error.Name, inEngine.Error.Message, inNode[i].Error)
					}
					continue
				}
				pages++
				for _, other := range []struct {
					name string
					got  rendered
				}{{"the bundle in Node", inNode[i]}, {"react-dom/server in Node", byReact[i]}} {
					if other.got.Page == nil {
						t.Errorf("%s: %s threw %+v", route.Pathname, other.name, other.got.Error)
						continue
					}
					if other.got.Page.HTML != inEngine.Page.HTML {
						t.Errorf("%s: HTML differs\n--- the engine\n%s\n--- %s\n%s", route.Pathname, inEngine.Page.HTML, other.name, other.got.Page.HTML)
					}
					if !reflect.DeepEqual(other.got.Page.Mounts, inEngine.Page.Mounts) {
						t.Errorf("%s: mounts differ\n--- the engine\n%+v\n--- %s\n%+v", route.Pathname, inEngine.Page.Mounts, other.name, other.got.Page.Mounts)
					}
				}
				if inNode[i].Page != nil && !reflect.DeepEqual(inNode[i].Page.Components, inEngine.Page.Components) {
					t.Errorf("%s: components differ: the engine %v, Node %v", route.Pathname, inEngine.Page.Components, inNode[i].Page.Components)
				}
			}
			// bad: /console/, and the three pages that are not documents.
			if want := map[string]int{"site": 8, "bad": 4}[site]; pages != want {
				t.Errorf("%d pages compared, want %d", pages, want)
			}
		})
	}
}
