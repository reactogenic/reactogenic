package cssprune

import (
	"testing"
)

// TestPruneOwnScript: a page that carries a script of its own — one the
// builder did not make — is not pruned (builder.md, CSS): what it writes to
// the page is not known, and `/theme.js` adding `dark` to the root would
// find `.dark .a` gone.
func TestPruneOwnScript(t *testing.T) {
	const css = `.a{x:y}.dark .a{x:y}.gone{x:y}:root{--unused:1}@keyframes k{to{x:y}}`
	for name, page := range map[string]string{
		"a script file":           `<head><script src="/theme.js"></script></head><p class="a"></p>`,
		"an inline script":        `<p class="a"></p><script>document.documentElement.classList.add("dark")</script>`,
		"a module":                `<p class="a"></p><script type="module">import "/theme.js"</script>`,
		"a JavaScript MIME type":  `<p class="a"></p><script type="Text/JavaScript">go()</script>`,
		"an empty type":           `<p class="a"></p><script type="">go()</script>`,
		"in SVG":                  `<p class="a"></p><svg><script>go()</script></svg>`,
		"an event handler":        `<my-el class="a" onclick="this.classList.add('dark')"></my-el>`,
		"a javascript: URL":       `<a class="a" href=" JavaScript:go()">x</a>`,
		"a javascript: URL, form": `<form class="a" action="java&#10;script:go()"></form>`,
	} {
		out, stats := mustPrune(t, css, parsePage(t, page))
		if out != css || !stats.Unpruned || stats.Why != "the page has a script of its own" {
			t.Errorf("%s: pruned: %q %+v", name, out, stats)
		}
	}
	// What does not run is not a script: a data block, a script element
	// with nothing in it, an attribute that only starts like a handler's.
	for name, page := range map[string]string{
		"JSON-LD":         `<p class="a"></p><script type="application/ld+json">{"@type":"dark gone"}</script>`,
		"an import map":   `<p class="a"></p><script type="importmap">{"imports":{}}</script>`,
		"an empty script": `<p class="a"></p><script> </script>`,
		"no handler":      `<p class="a" on="x" data-onclick="gone"></p><a href="/javascript:x">x</a>`,
	} {
		out, stats := mustPrune(t, css, parsePage(t, page))
		if want := `.a{x:y}`; out != want || stats.Unpruned || stats.Why != "" {
			t.Errorf("%s:\n got: %s\nwant: %s\n%+v", name, out, want, stats)
		}
	}
}

// TestPruneOtherSheet: a stylesheet the builder did not bundle — a file of
// `public/`, linked — reads the sheet's custom properties and names its
// animations, and nothing here sees it: they all stay. Rules are still
// pruned: what an element may match does not depend on another sheet.
func TestPruneOtherSheet(t *testing.T) {
	const css = `.a{x:y}.gone{x:y}:root{--theme:1;--b:2}@keyframes spin{to{x:y}}`
	const kept = `.a{x:y}:root{--theme:1;--b:2}@keyframes spin{to{x:y}}`
	for _, tt := range []struct{ name, css, page, want string }{
		{"a linked sheet", css, `<head><link rel="stylesheet" href="/theme.css"></head><p class="a"></p>`, kept},
		{"rel is a list, whatever its case", css, `<head><link REL="Alternate  StyleSheet" href="/theme.css" title="t"></head><p class="a"></p>`, kept},
		{"an @import in a <style>", css, `<head><style>@import url(/theme.css);</style></head><p class="a"></p>`, kept},
		{"an @import of the sheet's own", `@import "theme.css";` + css, `<p class="a"></p>`, `@import "theme.css";` + kept},
		{"another link", css, `<head><link rel="icon" href="/theme.css"></head><p class="a"></p>`, `.a{x:y}`},
		{"a <style> without one", css, `<head><style>p{color:var(--b)}</style></head><p class="a"></p>`, `.a{x:y}:root{--b:2}`},
	} {
		out, stats := mustPrune(t, tt.css, parsePage(t, tt.page))
		if out != tt.want || stats.Unpruned {
			t.Errorf("%s:\n got: %s\nwant: %s\n%+v", tt.name, out, tt.want, stats)
		}
	}
}

// TestPruneWithScript: what the page's built script names is "maybe"
// (builder.md, CSS, *the page's script*): a behaviour writes state the HTML
// does not show — a class, an attribute, a property that reflects one — and
// reads custom properties. A script that changes the tree leaves nothing to
// decide.
func TestPruneWithScript(t *testing.T) {
	const page = `<p class="a" id="i" tabindex="-1" data-v="ghost" title="t"></p>`
	for _, tt := range []struct{ name, css, script, want string }{
		{"no script", `.a{x:y}.dark .a{x:y}[tabindex="0"]{x:y}`, ``, `.a{x:y}`},
		{"a script that writes nothing", `.a{x:y}.dark .a{x:y}[tabindex="0"]{x:y}#late{x:y}`,
			`addEventListener("pagehide",()=>{for(const e of document.querySelectorAll(":popover-open"))e.hidePopover()});`, `.a{x:y}`},
		{"a class", `.dark .a{x:y}.is-open{x:y}.gone{x:y}.sm\:flex{x:y}.md\:flex{x:y}`,
			`function m(e){e.classList.add("dark");e.className="a is-open sm:flex"}`, `.dark .a{x:y}.is-open{x:y}.sm\:flex{x:y}`},
		{"an id", `#late{x:y}#gone{x:y}[id=late]{x:y}`, `function m(e){e.id="late"}`, `#late{x:y}[id=late]{x:y}`},
		{"a property that reflects an attribute", `[tabindex="0"]{x:y}[title=x]{x:y}[lang=en]{x:y}`,
			`function m(e){e.tabIndex=0;e.lang="en"}`, `[tabindex="0"]{x:y}[lang=en]{x:y}`},
		{"dataset", `[data-placement=top]{x:y}[data-foo-bar]{x:y}[data-v=solid]{x:y}`,
			`function m(e){e.dataset.placement="top";e.dataset.fooBar=""}`, `[data-placement=top]{x:y}[data-foo-bar]{x:y}`},
		{"setAttribute", `[data-x=y]{x:y}[data-v=solid]{x:y}[for=i]{x:y}[class~=q]{x:y}`,
			`function m(e){e.setAttribute("data-x","y");e.htmlFor="i";e.classList.toggle("q")}`, `[data-x=y]{x:y}[for=i]{x:y}[class~=q]{x:y}`},
		{"a custom property it reads, an animation it names", `:root{--rg-breakpoint:48rem;--unused:1}@keyframes spin{to{x:y}}@keyframes gone{to{x:y}}`,
			`function m(e){matchMedia("(min-width:"+getComputedStyle(e).getPropertyValue("--rg-breakpoint")+")");e.style.animation="spin 1s"}`,
			`:root{--rg-breakpoint:48rem}@keyframes spin{to{x:y}}`},
	} {
		out, stats, err := PruneWith(tt.css, parsePage(t, page), Options{Script: tt.script})
		if err != nil || out != tt.want || stats.Unpruned {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", tt.name, err, out, tt.want, stats)
		}
	}
	const css = `.a{x:y}.a>b{x:y}.gone{x:y}`
	for name, script := range map[string]string{
		"createElement": `function m(e){e.append(document.createElement("b"))}`,
		"innerHTML":     `function m(e){e.innerHTML="<b></b>"}`,
		"cloneNode":     `function m(e){e.after(e.cloneNode(!0))}`,
		"remove":        `function m(e){e.firstElementChild.remove()}`,
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
		if err != nil || out != css || !stats.Unpruned || stats.Why != "the page's script changes the tree" {
			t.Errorf("%s: %v: pruned: %q %+v", name, err, out, stats)
		}
	}
}
