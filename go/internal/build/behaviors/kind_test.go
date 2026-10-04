package behaviors

import (
	"slices"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/build/render"
)

// A behaviour is per element or per page (builder.md, *Behaviours*): one
// that a page mounts both ways would be called once with its root and once
// with none — `m0(document.getElementById("m1")); m0();` — and the second
// call throws on `undefined.addEventListener`, which ends the script: no
// mount after it runs.
func TestKind(t *testing.T) {
	opts := project(t)
	const body = `<div id="m1"></div><div id="m2"></div>`
	const message = "Page /k/: `mount(\"@fixture/ui/behaviors/menu-keys\")`: the module is mounted on an element (`m1`) and without one: a behaviour is of an element or of the page"
	for name, mounts := range map[string][]render.Mount{
		"an element first": {mount("overlays", ""), mount("menu-keys", "m1"), mount("menu-keys", "m2"), mount("menu-keys", ""), mount("menu-keys", "")},
		"the page first":   {mount("menu-keys", ""), mount("overlays", ""), mount("menu-keys", "m1")},
		// A module is its file, however it is spelled.
		"two spellings": {mount("menu-keys", "m1"), {Module: "./" + src + "behaviors/menu-keys.ts"}},
	} {
		p := page("/k/", body, mounts...)
		js, modules, reports := Build(p, opts)
		if len(reports) != 1 || reports[0].Code != "mount-kind" || reports[0].Message != message || js != "" || modules != nil {
			t.Errorf("%s: %q %+v", name, js, reports)
		}
		// The control's table would call it both ways too.
		if js, reports := BuildControl([]render.Page{home, p}, opts); len(reports) != 1 || reports[0].Code != "mount-kind" || js != "" {
			t.Errorf("%s, the control: %q %+v", name, js, reports)
		}
	}
	// On every element it is mounted on, or on none: fine either way.
	for _, p := range []render.Page{
		page("/k/", body, mount("menu-keys", "m1"), mount("menu-keys", "m2")),
		page("/k/", body, mount("overlays", ""), mount("overlays", "")),
	} {
		if _, _, got := codes(t, opts, p); !slices.Equal(got, nil) {
			t.Errorf("reports: %v", got)
		}
	}
}
