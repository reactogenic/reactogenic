package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/checktest"
)

// output is what `reactogenic check --pretty=false -p config` prints, with
// the project's directory as `<root>`.
func output(dir, config string) string {
	var b bytes.Buffer
	Print(&b, Run(dir+"/"+config), dir, false, readFile)
	return strings.ReplaceAll(b.String(), dir, "<root>")
}

// goldenText compares text with testdata/golden/<test name>.txt.
func goldenText(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+".txt")
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("no golden output for %s: run `go test ./internal/check -update`", t.Name())
		return
	}
	if got != string(want) {
		t.Errorf("output differs from %s:\n--- got\n%s--- want\n%s", path, got, want)
	}
}

// project writes a project of internal/checktest and returns its directory.
func project(t *testing.T, name string) string {
	t.Helper()
	return writeProject(t, checktest.Get(name).Files)
}

// An .rtsx file is a module of the program under its own name (ide.md, *The
// engine*): the shapes of project that the overlay did not cover, or covered
// differently.
func TestProjects(t *testing.T) {
	// A project with nothing but .rtsx files has inputs: no TS18003.
	t.Run("rtsx-only", func(t *testing.T) {
		dir := project(t, "rtsx-only")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Contains(got, "TS18003") || !strings.Contains(got, "src/a.rtsx(2,14): error TS2322") {
			t.Errorf("got:\n%s", got)
		}
	})

	// Vite's template: the root tsconfig has no files, only references. Each
	// referenced project is checked; the root prints what `-p
	// tsconfig.app.json` and `-p tsconfig.node.json` print.
	t.Run("vite-template", func(t *testing.T) {
		dir := project(t, "vite-template")
		root, app, node := output(dir, "tsconfig.json"), output(dir, "tsconfig.app.json"), output(dir, "tsconfig.node.json")
		goldenText(t, root)
		if node != "" {
			t.Errorf("tsconfig.node.json:\n%s", node)
		}
		if root != app || !strings.Contains(app, "src/page.rtsx(2,9): error TS2322") {
			t.Errorf("the root prints:\n%s`-p tsconfig.app.json` prints:\n%s", root, app)
		}
	})

	// A file of two referenced projects is reported once, by the first.
	t.Run("references-shared-file", func(t *testing.T) {
		dir := project(t, "references-shared-file")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Count(got, "src/page.rtsx") != 2 || strings.Count(got, "orphan-slot") != 1 || strings.Count(got, "test/page.test.tsx") != 1 {
			t.Errorf("got:\n%s", got)
		}
	})

	// An .rtsx module of a referenced project is read from its source — it
	// has no output a build could have left. The referenced project is
	// checked first, and its files are its own to report, once and with its
	// own options: lib is not strict, so `pick`'s untyped parameters are no
	// error, though app, which is strict, has the file in its program too.
	t.Run("cross-project", func(t *testing.T) {
		dir := project(t, "cross-project")
		root, app, lib := output(dir, "tsconfig.json"), output(dir, "app/tsconfig.json"), output(dir, "lib/tsconfig.json")
		goldenText(t, "# -p .\n"+root+"# -p app\n"+app+"# -p lib\n"+lib)
		if want := "lib/src/button.rtsx(2,9): error TS2322: Type 'number' is not assignable to type 'string'.\n"; lib != want {
			t.Errorf("-p lib:\n%s", lib)
		}
		if !strings.HasPrefix(app, "app/src/page.rtsx(3,29): error TS2322") || !strings.HasSuffix(app, lib) || strings.Count(app, "error") != 2 {
			t.Errorf("-p app:\n%s", app)
		}
		if root != app {
			t.Errorf("-p .:\n%s-p app:\n%s", root, app)
		}
	})

	// Who reports a file does not depend on the order of `references`: its
	// own project does — the one that lists it — though a project listed
	// before has the file in its program, through an import, and checks it
	// with other options. A test project listed before the app it tests:
	// the root prints what `-p tsconfig.app.json` prints, either way round.
	t.Run("references-order", func(t *testing.T) {
		// The app is strict, its tests are not: the untyped parameters are
		// the app's errors.
		dir := project(t, "references-order")
		root, app := output(dir, "tsconfig.json"), output(dir, "tsconfig.app.json")
		swapped := output(project(t, "references-order/app-first"), "tsconfig.json")
		// The tests are strict, the app is not: they are nobody's.
		dir = project(t, "references-order/strict-tests")
		strictTests, looseApp := output(dir, "tsconfig.json"), output(dir, "tsconfig.app.json")
		goldenText(t, "# tests that are not strict, before a strict app\n"+root+"# strict tests before an app that is not\n"+strictTests)
		if root != app || root != swapped || strings.Count(root, "src/page.rtsx") != 2 || strings.Count(root, "src/util.ts") != 2 {
			t.Errorf("the root prints:\n%s`-p tsconfig.app.json` prints:\n%swith the app listed first, the root prints:\n%s", root, app, swapped)
		}
		if strictTests != "" || looseApp != "" {
			t.Errorf("strict tests first — the root prints:\n%s`-p tsconfig.app.json` prints:\n%s", strictTests, looseApp)
		}
	})

	// A reference to a project that is not there is one error, the
	// referencing project's (TS6053) — not a second one for the tsconfig
	// that could not be read. The tsconfig `check` is given is another
	// matter: that one is read, and missing.
	t.Run("reference-missing", func(t *testing.T) {
		dir := project(t, "reference-missing")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Count(got, "error") != 2 || !strings.Contains(got, "src/a.rtsx(1,24): error orphan-slot") || !strings.Contains(got, "error TS6053: File '<root>/gone' not found.") {
			t.Errorf("got:\n%s", got)
		}
		if missing := output(dir, "gone/tsconfig.json"); !strings.Contains(missing, "error TS5083") {
			t.Errorf("-p gone/tsconfig.json:\n%s", missing)
		}
	})

	// diagnostics.md, step 1: `include` matches .rtsx as it matches .tsx — a
	// directory or a `*` covers both, a pattern that names an extension
	// covers that extension. An .rtsx file that no pattern matches is in the
	// program when an import reaches it, and is checked then; one that
	// nothing reaches is not, like a .tsx file.
	t.Run("include-extensions", func(t *testing.T) {
		check := func(name string) string { return output(project(t, name), "tsconfig.json") }
		directory, named, onlyTS := check("include-extensions/directory"), check("include-extensions"), check("include-extensions/ts-only")
		goldenText(t, "# \"src/**/*.ts\", \"src/**/*.tsx\", \"src/**/*.rtsx\"\n"+named+"# \"src/**/*.ts\", \"src/**/*.tsx\"\n"+onlyTS)
		if named != directory || strings.Count(named, "src/unreached.rtsx") != 2 || !strings.Contains(named, "src/page.rtsx(1,14): error TS2322") {
			t.Errorf("with .rtsx named:\n%swith the directory:\n%s", named, directory)
		}
		if want := "src/page.rtsx(1,14): error TS2322: Type 'string' is not assignable to type 'number'.\n"; onlyTS != want {
			t.Errorf("without .rtsx named:\n%s", onlyTS)
		}
	})

	// A construct that is an error where it stands and stays as written — an
	// arg on an element without `slot={$X}`, params on an intrinsic element
	// — leaves text that is not TSX. The file reports the transpiler's
	// errors, and nothing of what TS makes of that text (ide.md, *Tolerance*,
	// rule 4); its importers are checked.
	t.Run("unlowered", func(t *testing.T) {
		dir := project(t, "unlowered")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		want := "src/args.rtsx(3,26): error arg-without-slot: `&size` is an arg of a slot attachment; this element has no `slot={$X}`\n" +
			"src/args.rtsx(4,40): error arg-without-slot: `&&value` is an arg of a slot attachment; this element has no `slot={$X}`\n" +
			"src/main.tsx(2,14): error TS2322: Type 'number' is not assignable to type 'string'.\n" +
			"src/params.rtsx(2,23): error params-on-html: Params are only allowed on components and slot elements\n"
		if got != want {
			t.Errorf("got:\n%swant:\n%s", got, want)
		}
	})

	// `check` never emits, so what an emit of declarations would report
	// (TS4094) it reports — in a composite project that emits them too, not
	// only under `noEmit`. The editor shows it there (the reporting layer's
	// per-file form).
	t.Run("declarations", func(t *testing.T) {
		dir := project(t, "declarations")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if !strings.HasPrefix(got, "src/a.rtsx(1,14): error TS4094") || strings.Count(got, "error") != 1 {
			t.Errorf("got:\n%s", got)
		}
	})

	// A `paths` alias finds an .rtsx module, with and without the extension.
	t.Run("paths-alias", func(t *testing.T) {
		dir := project(t, "paths-alias")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if strings.Contains(got, "TS2307") || strings.Count(got, "src/page.rtsx(4,") != 3 {
			t.Errorf("got:\n%s", got)
		}
	})

	// A tsconfig that lists the stock content mapper (ide.md, *Stock
	// TypeScript 7.1*) is checked as one without the entry.
	t.Run("content-mappers-entry", func(t *testing.T) {
		listed := output(project(t, "content-mappers-entry"), "tsconfig.json")
		plain := output(project(t, "content-mappers-entry/without"), "tsconfig.json")
		goldenText(t, listed)
		if listed != plain || !strings.Contains(listed, "undeclared-slot") {
			t.Errorf("with the entry:\n%swithout:\n%s", listed, plain)
		}
	})

	// syntax.md, *Segment files*: the generated import names the .rtsx file,
	// which node16 / nodenext resolution finds as bundler resolution does.
	for _, resolution := range []string{"node16", "nodenext"} {
		t.Run("segment-"+resolution, func(t *testing.T) {
			dir := project(t, "segment-"+resolution)
			got := output(dir, "tsconfig.json")
			goldenText(t, got)
			if want := "src/page.rtsx(2,25): error segment-props: A segment takes no props: `intro` requires `title`\n"; !strings.HasPrefix(got, want) || strings.Contains(got, "TS2307") {
				t.Errorf("got:\n%s", got)
			}
		})
	}

	// A segment that mounts itself, directly and through another file
	// (syntax.md, *Segment roots*): each mount of the loop is reported.
	t.Run("segment-self", func(t *testing.T) {
		dir := project(t, "segment-self")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		for _, want := range []string{
			"src/intro.rtsx(2,24): error segment-self: `#outro` mounts the segment it is written in\n",
			"src/outro.rtsx(2,24): error segment-self: `#intro` mounts the segment it is written in\n",
			"src/outro.rtsx(2,42): error segment-self: `#outro` mounts the segment it is written in\n",
		} {
			if strings.Count(got, want) != 1 {
				t.Errorf("want once %q in:\n%s", want, got)
			}
		}
		if strings.Contains(got, "page.rtsx") {
			t.Errorf("page.rtsx is in no loop:\n%s", got)
		}
	})

	// A syntax error fails the check (ide.md, *Tolerance*: the build paths
	// stay strict): the file reports its syntax errors and nothing else, and
	// the rest of the program is still checked.
	t.Run("syntax-error", func(t *testing.T) {
		dir := project(t, "syntax-error")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		want := "src/broken.rtsx(3,34): error TS17008: JSX element 'span' has no corresponding closing tag.\n" + // not its type error, nor its orphaned slot
			"src/main.tsx(2,14): error TS2322: Type 'number' is not assignable to type 'string'.\n" + // `n` is still exported
			"src/main.tsx(4,14): error TS2322: Type 'string' is not assignable to type 'number'.\n"
		if got != want {
			t.Errorf("got:\n%swant:\n%s", got, want)
		}
	})

	// A syntax error in a .ts / .tsx file is TypeScript's: as in tsc, no
	// types are checked while it stands. The transpiler's errors are
	// reported all the same.
	t.Run("tsx-syntax-error", func(t *testing.T) {
		dir := project(t, "tsx-syntax-error")
		got := output(dir, "tsconfig.json")
		goldenText(t, got)
		if !strings.Contains(got, "src/main.tsx(1,26): error TS17008") || !strings.Contains(got, "src/page.rtsx(2,26): error orphan-slot") || strings.Contains(got, "TS2322") {
			t.Errorf("got:\n%s", got)
		}
	})

	// A warning is printed and does not fail the check.
	t.Run("warning", func(t *testing.T) {
		dir := project(t, "warning")
		reports := Run(dir + "/tsconfig.json")
		golden(t, dir, reports)
		if len(reports) != 1 || reports[0].Code != "segment-children" || Errors(reports) != 0 {
			t.Errorf("got %+v", reports)
		}
	})
}
