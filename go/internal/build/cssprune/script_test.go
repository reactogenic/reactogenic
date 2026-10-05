package cssprune

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
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

// TestPruneBuilderElements: the page is pruned as it is served, with the
// elements packaging wrote in it (builder.md, CSS, *The builder's own
// elements*). Named as the builder's (Options.Builder) they are elements a
// rule may select and nothing more: the `<script>` is not a script of the
// page's own — which would leave every page that mounts a behaviour
// unpruned — and the `<link>` is not a stylesheet the builder did not
// bundle. Not named, or beside an author's, the rules for those still hold.
func TestPruneBuilderElements(t *testing.T) {
	const css = `.a{x:y}.dark .a{x:y}.is-open{x:y}.gone{x:y}script{display:block}link{x:y}style{x:y}:root{--unused:1;--read:2}@keyframes k{to{x:y}}`
	const script = `function m(e){e.classList.toggle("is-open");getComputedStyle(e).getPropertyValue("--read")}`
	// find is the driver's part: which elements of the page it wrote.
	find := func(doc *html.Node, is func(n *html.Node) bool) (found []*html.Node) {
		var walk func(n *html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && is(n) {
				found = append(found, n)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		return found
	}
	builders := func(n *html.Node) bool {
		for _, a := range n.Attr {
			if a.Key == "src" || a.Key == "href" {
				return strings.HasPrefix(a.Val, "/_rg/")
			}
		}
		return (n.Data == "style" || n.Data == "script") && n.FirstChild == nil
	}
	for _, tt := range []struct {
		name, page string
		named      bool // the driver names its elements
		want, why  string
	}{
		{"the builder's script, a file", `<p class="a"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			`.a{x:y}.is-open{x:y}script{display:block}:root{--read:2}`, ""},
		{"the builder's script, inlined: empty in the pruner's page", `<p class="a"></p><script type="module"></script>`, true,
			`.a{x:y}.is-open{x:y}script{display:block}:root{--read:2}`, ""},
		{"the builder's script and its linked sheet", `<head><link rel="stylesheet" href="/_rg/page-1a2b3c4d.css"></head><p class="a"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			`.a{x:y}.is-open{x:y}script{display:block}link{x:y}:root{--read:2}`, ""},
		{"the builder's script and its inlined sheet", `<head><style></style></head><p class="a"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			`.a{x:y}.is-open{x:y}script{display:block}style{x:y}:root{--read:2}`, ""},
		// Not told which is its own, the pruner takes every one for the
		// author's: the safe side, and the whole sheet.
		{"the same script, not named", `<p class="a"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, false,
			css, "the page has a script of its own"},
		{"the same sheet, not named", `<head><link rel="stylesheet" href="/_rg/page-1a2b3c4d.css"></head><p class="a"></p>`, false,
			`.a{x:y}.is-open{x:y}link{x:y}:root{--unused:1;--read:2}@keyframes k{to{x:y}}`, ""},
		// The author's, beside the builder's.
		{"an author's script beside the builder's", `<head><script src="/theme.js"></script></head><p class="a"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			css, "the page has a script of its own"},
		{"an author's inline module beside the builder's", `<p class="a"></p><script type="module">document.documentElement.classList.add("dark")</script><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			css, "the page has a script of its own"},
		{"an author's handler beside the builder's script", `<p class="a" onclick="go()"></p><script type="module" src="/_rg/page-1a2b3c4d.js"></script>`, true,
			css, "the page has a script of its own"},
		{"an author's sheet beside the builder's", `<head><link rel="stylesheet" href="/theme.css"><link rel="stylesheet" href="/_rg/page-1a2b3c4d.css"></head><p class="a"></p>`, true,
			`.a{x:y}.is-open{x:y}link{x:y}:root{--unused:1;--read:2}@keyframes k{to{x:y}}`, ""},
		{"an author's <style> beside the builder's", `<head><style>p{animation:k 1s;color:var(--unused)}</style><style></style></head><p class="a"></p>`, true,
			`.a{x:y}.is-open{x:y}style{x:y}:root{--unused:1;--read:2}@keyframes k{to{x:y}}`, ""},
	} {
		doc := parsePage(t, tt.page)
		opts := Options{Script: script}
		if tt.named {
			if opts.Builder = find(doc, builders); len(opts.Builder) == 0 {
				t.Fatalf("%s: the page has no element of the builder's", tt.name)
			}
		}
		out, stats, err := PruneWith(css, doc, opts)
		if err != nil || out != tt.want || stats.Unpruned != (tt.why != "") || stats.Why != tt.why {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", tt.name, err, out, tt.want, stats)
		}
	}
	// A builder's <style> is not read, whatever is in it: a sheet is not
	// pruned against itself.
	doc := parsePage(t, `<head><style>:root{--unused:1}p{animation:k 1s}</style></head><p class="a"></p>`)
	out, _, err := PruneWith(css, doc, Options{Builder: find(doc, func(n *html.Node) bool { return n.Data == "style" })})
	if want := `.a{x:y}style{x:y}`; err != nil || out != want {
		t.Errorf("the builder's <style> with its sheet in it: %v\n got: %s\nwant: %s", err, out, want)
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
		{"a token list that reflects an attribute", `[rel~=next]{x:y}[title=x]{x:y}`, `function m(e){e.relList.add("next")}`, `[rel~=next]{x:y}`},
		{"a custom property it reads, an animation it names", `:root{--rg-breakpoint:48rem;--unused:1}@keyframes spin{to{x:y}}@keyframes gone{to{x:y}}`,
			`function m(e){matchMedia("(min-width:"+getComputedStyle(e).getPropertyValue("--rg-breakpoint")+")");e.style.animation="spin 1s"}`,
			`:root{--rg-breakpoint:48rem}@keyframes spin{to{x:y}}`},
	} {
		out, stats, err := PruneWith(tt.css, parsePage(t, page), Options{Script: tt.script})
		if err != nil || out != tt.want || stats.Unpruned {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", tt.name, err, out, tt.want, stats)
		}
	}
	// The reason names the API: the first one of the script's text.
	const css = `.a{x:y}.a>b{x:y}.gone{x:y}`
	for name, script := range map[string]string{
		"append":     `function m(e){e.append(document.createElement("b"))}`,
		"innerHTML":  `function m(e){e.innerHTML="<b></b>"}`,
		"after":      `function m(e){e.after(e.cloneNode(!0))}`,
		"remove":     `function m(e){e.firstElementChild.remove()}`,
		"outerText":  `function m(e){e.firstElementChild.outerText="gone"}`,
		"insertRow":  `function m(e){e.insertRow()}`,
		"insertNode": `function m(e){getSelection().getRangeAt(0).insertNode(e)}`,
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
		if want := "the page's script may change the tree: it names `" + name + "`"; err != nil || out != css || !stats.Unpruned || stats.Why != want {
			t.Errorf("%s: %v: pruned: %q %+v", name, err, out, stats)
		}
	}
}

// TestPruneState: state that only a script can write is decided on the page,
// unless the page's script names it (builder.md, CSS, *Runtime state*; the
// owner's ruling on M, decisions.md). Of the attributes that were "maybe"
// whoever could write them, only what the browser writes by itself still
// is: `open`, `hidden`, `style` — and `dir` on a text control, `controls`
// and `loop` on a player. Both ways for
// each of the others: dropped when nothing can write it, kept when the
// script names it — as the attribute, or as the property that reflects it.
func TestPruneState(t *testing.T) {
	// No element of the page has any of the state.
	// (No class here is a word of the scripts below: a class the script names
	// is "maybe" on every element.)
	const page = `<p class="a">x</p><button class="b">b</button><input class="i"><ul class="list"><li class="o">o</li></ul>`
	// What the browser itself writes: never decided, script or no script.
	const browser = `.a[open]{x:y}.a:not([open]){x:y}.a[hidden]{x:y}.a[hidden=until-found]{x:y}.a[style*=width]{x:y}.i[dir=rtl]{x:y}`
	if out, _, err := PruneWith(browser+`.a[dir=rtl]{x:y}.a[controls]{x:y}.gone[open]{x:y}`, parsePage(t, page), Options{}); err != nil || out != browser {
		t.Errorf("what the browser writes: %v\n got: %s\nwant: %s", err, out, browser)
	}
	// … and what it writes on one kind of element, from that element's own
	// menu: `dir` on a text control, `controls` and `loop` on a player.
	const media = `<video class="v" src="/a.webm"></video><audio class="s" controls></audio><textarea class="t"></textarea><p class="a" dir="ltr">x</p>`
	const own = `.v[controls]{x:y}.v[loop]{x:y}.s:not([controls]){x:y}.t[dir=rtl]{x:y}.a[dir=ltr]{x:y}`
	if out, _, err := PruneWith(own+`.a[dir=rtl]{x:y}.a[loop]{x:y}.v[muted]{x:y}.t[controls]{x:y}`, parsePage(t, media), Options{}); err != nil || out != own {
		t.Errorf("what the browser writes on an element of its own: %v\n got: %s\nwant: %s", err, out, own)
	}
	for _, tt := range []struct {
		name string
		css  string   // rules on state the page does not have
		by   []string // scripts that may write it, each naming it its own way
	}{
		{"inert", `.a[inert]{x:y}`, []string{
			`function m(e){e.inert=!0}`, `function m(e){e.toggleAttribute("inert")}`, `function m(e){e.setAttribute("inert","")}`,
		}},
		{"disabled", `.b[disabled]{x:y}`, []string{
			`function m(e){e.disabled=!0}`, `function m(e){e.toggleAttribute("disabled",!0)}`, `function m(e){Object.assign(e,{disabled:!0})}`,
		}},
		{"checked", `.i[checked]{x:y}`, []string{
			`function m(e){e.defaultChecked=!0}`, `function m(e){e.setAttribute("checked","")}`,
			`function m(e){e.checked=!0}`, // the property does not reflect: its name is the attribute's all the same
		}},
		{"selected", `.o[selected]{x:y}`, []string{
			`function m(e){e.defaultSelected=!0}`, `function m(e){e.setAttribute("selected","")}`, `function m(e){e.selected=!0}`,
		}},
		{"value", `.i[value=x]{x:y}.o[value="3"]{x:y}`, []string{
			`function m(e){e.defaultValue="x"}`, `function m(e){e.setAttribute("value","x")}`,
			`function m(e){e.value=3}`, // reflects on a button, an option, a list item, a hidden input …
		}},
		{"aria-expanded", `.b[aria-expanded=true]{x:y}`, []string{
			`function m(e){e.setAttribute("aria-expanded","true")}`, `function m(e){e.ariaExpanded="true"}`,
			`function m(e){e.toggleAttribute("aria-expanded")}`, `function m(e){e.removeAttribute("aria-expanded")}`,
			`function m(e){e["ariaExpanded"]="true"}`,
		}},
		{"aria-disabled", `.a[aria-disabled=true]{x:y}.b:is(.gone,[aria-disabled=true]){x:y}`, []string{
			`function m(e){e.ariaDisabled="true"}`, `function m(e){e.setAttribute("aria-disabled","true")}`,
			// As menu-keys reads it, in a selector: named is named.
			`function m(e){return e.querySelectorAll("[role=menuitem]:not(:disabled, [aria-disabled=true])")}`,
		}},
		{"aria-current", `.o[aria-current]{x:y}.o[aria-current=page]{x:y}.list:has([aria-current]){x:y}`, []string{
			`function m(e){e.ariaCurrent="page"}`, `function m(e){e.setAttribute("aria-current","page")}`,
		}},
		{"aria-controls, by element", `.b[aria-controls]{x:y}`, []string{
			`function m(e,t){e.ariaControlsElements=[t]}`, `function m(e){e.setAttribute("aria-controls","t")}`,
		}},
		{"aria-activedescendant, by element", `.list[aria-activedescendant]{x:y}`, []string{
			`function m(e,t){e.ariaActiveDescendantElement=t}`, `function m(e){e.setAttribute("aria-activedescendant","o1")}`,
		}},
		{"data-state", `.a[data-state=open]{x:y}`, []string{
			`function m(e){e.dataset.state="open"}`, `function m(e){e.setAttribute("data-state","open")}`, `function m(e){delete e.dataset.state}`,
		}},
	} {
		css := tt.css + `.gone{x:y}`
		// Nobody can write it: no script at all, and a script that names
		// none of it. What the page has not, it never has.
		for _, script := range []string{``, `function m(e){e.addEventListener("click",()=>e.focus());e.hidden=!e.open}`} {
			out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
			if err != nil || out != "" || stats.Unpruned {
				t.Errorf("%s, script %q: %v\n kept: %s", tt.name, script, err, out)
			}
		}
		for _, script := range tt.by {
			out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
			if err != nil || out != tt.css || stats.Unpruned {
				t.Errorf("%s, script %q: %v\n got: %s\nwant: %s", tt.name, script, err, out, tt.css)
			}
		}
	}
	// A pseudo-class is "maybe" whoever writes: the ruling leaves those as
	// they were, and a list that holds one is kept for it.
	const pseudo = `.b:is(:disabled,[aria-disabled=true]){x:y}.i:checked{x:y}.b:disabled{x:y}`
	if out, _, err := PruneWith(pseudo+`.i[checked]{x:y}`, parsePage(t, page), Options{}); err != nil || out != pseudo {
		t.Errorf("pseudo-classes: %v\n got: %s\nwant: %s", err, out, pseudo)
	}

	// A page that has the state, and no script: what it has, it has — and
	// keeps. `:not()` of it is "no" there, and a rule on it stays.
	const has = `<p class="a" inert aria-hidden="true" data-state="open">x</p><button class="b" disabled aria-disabled="true" aria-expanded="false">b</button>` +
		`<input class="i" checked value="x"><ul class="list"><li class="o" aria-current="page">o</li></ul>`
	const rules = `.a[inert]{x:y}.a:not([inert]){x:y}.b[disabled]{x:y}.b[aria-disabled=true]{x:y}.b:not([aria-disabled]){x:y}.b[aria-expanded=true]{x:y}.b[aria-expanded=false]{x:y}` +
		`.i[checked]{x:y}.i[value=x]{x:y}.i:not([value]){x:y}.o[aria-current=page]{x:y}.list:not(:has([aria-current])){x:y}.a[data-state=open]{x:y}.a[data-state=closed]{x:y}`
	const stay = `.a[inert]{x:y}.b[disabled]{x:y}.b[aria-disabled=true]{x:y}.b[aria-expanded=false]{x:y}.i[checked]{x:y}.i[value=x]{x:y}.o[aria-current=page]{x:y}.a[data-state=open]{x:y}`
	if out, _, err := PruneWith(rules, parsePage(t, has), Options{}); err != nil || out != stay {
		t.Errorf("a page that has the state: %v\n got: %s\nwant: %s", err, out, stay)
	}
	// … and with a behaviour that takes it away, the negations are back.
	out, _, err := PruneWith(rules, parsePage(t, has), Options{Script: `function m(e){e.removeAttribute("aria-disabled");e.inert=!1;e.ariaExpanded="true";delete e.dataset.state;e.defaultValue=""}`})
	if want := `.a[inert]{x:y}.a:not([inert]){x:y}.b[disabled]{x:y}.b[aria-disabled=true]{x:y}.b:not([aria-disabled]){x:y}.b[aria-expanded=true]{x:y}.b[aria-expanded=false]{x:y}` +
		`.i[checked]{x:y}.i[value=x]{x:y}.i:not([value]){x:y}.o[aria-current=page]{x:y}.a[data-state=open]{x:y}.a[data-state=closed]{x:y}`; err != nil || out != want {
		t.Errorf("a behaviour that takes the state away: %v\n got: %s\nwant: %s", err, out, want)
	}
}

// TestPruneReflects: a property that reflects an attribute names it
// (builder.md, CSS, *The page's script*, the table of names): by the
// attribute's own name whatever its case and hyphens, with `default` before
// it, with `Element` or `Elements` after it, and the token lists.
func TestPruneReflects(t *testing.T) {
	const page = `<p class="a">x</p>`
	for attr, scripts := range map[string][]string{
		"value":                 {"value", "defaultValue"},
		"checked":               {"checked", "defaultChecked"},
		"selected":              {"selected", "defaultSelected"},
		"muted":                 {"muted", "defaultMuted"},
		"disabled":              {"disabled"},
		"inert":                 {"inert"},
		"readonly":              {"readOnly"},
		"tabindex":              {"tabIndex"},
		"aria-expanded":         {"ariaExpanded", `"aria-expanded"`},
		"aria-haspopup":         {"ariaHasPopup"},
		"aria-labelledby":       {"ariaLabelledByElements", `"aria-labelledby"`},
		"aria-describedby":      {"ariaDescribedByElements"},
		"aria-controls":         {"ariaControlsElements"},
		"aria-owns":             {"ariaOwnsElements"},
		"aria-flowto":           {"ariaFlowToElements"},
		"aria-details":          {"ariaDetailsElements"},
		"aria-errormessage":     {"ariaErrorMessageElements"},
		"aria-activedescendant": {"ariaActiveDescendantElement"},
		"popovertarget":         {"popoverTargetElement"},
		"commandfor":            {"commandForElement"},
		"data-state":            {"state", `"data-state"`},
		"for":                   {"htmlFor"},
		"rel":                   {"relList"},
	} {
		css := `.a[` + attr + `]{x:y}`
		if out, _, err := PruneWith(css, parsePage(t, page), Options{Script: `function m(e){e.focus()}`}); err != nil || out != "" {
			t.Errorf("[%s], a script that does not name it: %v: kept %q", attr, err, out)
		}
		for _, name := range scripts {
			script := `function m(e){e.` + name + `=1}`
			if strings.HasPrefix(name, `"`) {
				script = `function m(e){e.setAttribute(` + name + `,"")}`
			}
			if out, _, err := PruneWith(css, parsePage(t, page), Options{Script: script}); err != nil || out != css {
				t.Errorf("[%s], script %q: %v: got %q", attr, script, err, out)
			}
		}
	}
	// Not the other way: another name is another name. (`class` has its own
	// test: TestPruneWithScript.)
	for attr, script := range map[string]string{
		"aria-expanded": `function m(e){e.ariaExpandedBy=1;e.expanded=1;e.aria=1}`,
		"value":         `function m(e){e.valueAsNumber=1;e.defaultValues=1}`,
		"checked":       `function m(e){e.indeterminate=!0}`,
	} {
		if out, _, err := PruneWith(`.a[`+attr+`]{x:y}`, parsePage(t, page), Options{Script: script}); err != nil || out != "" {
			t.Errorf("[%s], script %q: %v: kept %q", attr, script, err, out)
		}
	}
}

// TestPruneScriptRemoves: a behaviour takes away as well as it adds. A class
// or an id the page has is "maybe" once the page's script names it — or may
// write the attribute whole — so its negation is "maybe" too: `:not(.x)` is
// not dropped for an element that has `x` when the page loads (builder.md,
// CSS, *The page's script*: "a name that is in it never decides anything").
func TestPruneScriptRemoves(t *testing.T) {
	const page = `<div id="c9" class="card collapsed list"><p class="body">body</p><span class="item active">item</span><i id="first" class="label">x</i></div>`
	const css = `.card:not(.collapsed)>.body{a:b}.card.collapsed>.body{a:b}.list:not(:has(.active)){a:b}.label:not(#first){a:b}.card:not(.list){a:b}.card:not(#c9){a:b}.gone{a:b}`
	const (
		collapsed = `.card:not(.collapsed)>.body{a:b}`
		always    = `.card.collapsed>.body{a:b}`
		active    = `.list:not(:has(.active)){a:b}`
		first     = `.label:not(#first){a:b}`
		list      = `.card:not(.list){a:b}`
		c9        = `.card:not(#c9){a:b}`
	)
	for _, tt := range []struct{ name, script, want string }{
		{"no script: the page is what it is", ``, always},
		{"a script that names none of it", `function m(e){e.addEventListener("click",()=>e.focus())}`, always},
		// The review's page: the class toggled, the class taken from every
		// item, the id written over.
		{"toggle, and an id written over",
			`function m(e){e.addEventListener("click",()=>{e.classList.toggle("collapsed");for(const t of e.querySelectorAll(".item"))t.classList.toggle("active",!1);e.querySelector("i").id="second"})}m(document.getElementById("c9"));`,
			collapsed + always + active + first + c9},
		{"classList.remove", `function m(e){e.addEventListener("click",()=>e.classList.remove("collapsed"))}`, collapsed + always},
		// `list` may go from the card — and come to any other element: the
		// `<p>`, which has nothing `active`, may be `.list:not(:has(.active))`.
		{"classList.replace", `function m(e){e.classList.replace("list","grid")}`, always + active + list},
		{"an id the script names", `function m(){document.getElementById("first").hidden=!0}`, always + first},
		{"removeAttribute of id", `function m(e){e.removeAttribute("id")}`, always + first + c9},
		// The attribute written whole: every class the page has may go.
		{"className", `function m(e){e.className=""}`, collapsed + always + active + list},
		{"setAttribute of class", `function m(e){e.setAttribute("class","plain")}`, collapsed + always + active + list},
		{"removeAttribute of class", `function m(e){e.removeAttribute("class")}`, collapsed + always + active + list},
		{"classList.value", `function m(e){e.classList.value="plain"}`, collapsed + always + active + list},
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: tt.script})
		if err != nil || out != tt.want || stats.Unpruned {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", tt.name, err, out, tt.want, stats)
		}
	}
}

// TestPruneClassListRemove: `classList.remove("x")` removes a class, not an
// element — the commonest write a behaviour makes does not cost its page
// the pruning. `remove` of anything else is the DOM's (builder.md, CSS,
// *The page's script*, the tree).
func TestPruneClassListRemove(t *testing.T) {
	const page = `<div class="card collapsed"><p class="body">body</p></div>`
	const css = `.card:not(.collapsed)>.body{a:b}.card>.body{a:b}.gone{a:b}`
	const kept = `.card:not(.collapsed)>.body{a:b}.card>.body{a:b}`
	for name, script := range map[string]string{
		"minified":          `function m(e){e.addEventListener("click",()=>e.classList.remove("collapsed"))}`,
		"as written":        "function m(e) {\n  e.classList . remove(\"collapsed\");\n}",
		"optional chaining": `function m(e){e?.classList?.remove("collapsed")}`,
		"twice":             `function m(e){e.classList.remove("collapsed");e.parentElement.classList.remove("collapsed")}`,
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
		if err != nil || out != kept || stats.Unpruned {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", name, err, out, kept, stats)
		}
	}
	for name, script := range map[string]string{
		"an element's":              `function m(e){e.classList.remove("collapsed");e.firstElementChild.remove()}`,
		"a list held in a variable": `function m(e){const l=e.classList;l.remove("collapsed")}`,
		"another list's":            `function m(e){e.myclassList.remove("collapsed")}`,
		"by its name":               `function m(e){e.classList["remove"]("collapsed")}`,
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
		if err != nil || out != css || !stats.Unpruned || stats.Why != "the page's script may change the tree: it names `remove`" {
			t.Errorf("%s: %v: pruned: %q %+v", name, err, out, stats)
		}
	}
}

// TestPruneScriptText: a script that writes an element's text — `textContent`,
// `innerText`, `text` — removes the elements it held, and adds none: what an
// element *has* is "maybe" where it was "yes", so `:not(:has(b))` stays; the
// page is still pruned, and so is a page whose script only reads the text,
// as `menu-keys` does (builder.md, CSS, *The page's script*, the text).
func TestPruneScriptText(t *testing.T) {
	const page = `<div id="s9" class="status"><b>Ready</b></div>`
	const css = `.status:not(:has(b)){a:b}.status:has(b){a:b}.status:has(i){a:b}.status>b{a:b}.status:not(:has(i)){a:b}.gone{a:b}`
	const kept = `.status:not(:has(b)){a:b}.status:has(b){a:b}.status>b{a:b}.status:not(:has(i)){a:b}`
	for name, script := range map[string]string{
		"textContent":          `function m(e){e.addEventListener("click",()=>{e.textContent="Copied"})}`,
		"innerText":            `function m(e){e.innerText="Copied"}`,
		"text, of a link":      `function m(e){e.text="Copied"}`,
		"through an object":    `function m(e){Object.assign(e,{textContent:"Copied"})}`,
		"read, as menu-keys":   `function m(e){return e.textContent.trim().toLowerCase()}`,
		"appended to":          `function m(e){e.textContent+="!"}`,
		"by its name":          `function m(e){e["textContent"]=""}`,
		"a nullish assignment": `function m(e){e.textContent??="x"}`,
	} {
		out, stats, err := PruneWith(css, parsePage(t, page), Options{Script: script})
		if err != nil || out != kept || stats.Unpruned {
			t.Errorf("%s: %v\n got: %s\nwant: %s\n%+v", name, err, out, kept, stats)
		}
	}
	// Without such a script what the page has, it has.
	out, _, err := PruneWith(css, parsePage(t, page), Options{Script: `function m(e){e.focus()}`})
	if want := `.status:has(b){a:b}.status>b{a:b}.status:not(:has(i)){a:b}`; err != nil || out != want {
		t.Errorf("no text written: %v\n got: %s\nwant: %s", err, out, want)
	}
}

// TestPruneFrames: a document of the site in a frame of the page is of the
// page's origin, and its script reaches the page — `parent.document` — as
// one of the page's own does (builder.md, CSS, *a page with a script of its
// own*): the page is not pruned. A document of another site cannot, nor one
// whose `sandbox` denies it scripts or the origin.
func TestPruneFrames(t *testing.T) {
	const css = `.a{x:y}.lit .a{x:y}.gone{x:y}`
	for name, page := range map[string]string{
		"a file of the site":           `<p class="a"></p><iframe src="/frame.html"></iframe>`,
		"a relative URL":               `<p class="a"></p><iframe src="frame.html?x=1"></iframe>`,
		"another page":                 `<p class="a"></p><iframe src="../guide/"></iframe>`,
		"srcdoc":                       `<p class="a"></p><iframe srcdoc="<script>parent.document.body.classList.add('lit')</script>"></iframe>`,
		"srcdoc beside another site":   `<p class="a"></p><iframe src="https://example.com/" srcdoc="<script>go()</script>"></iframe>`,
		"an object":                    `<p class="a"></p><object data="/drawing.svg" type="image/svg+xml"></object>`,
		"an embed":                     `<p class="a"></p><embed src="/drawing.svg">`,
		"a sandbox that allows both":   `<p class="a"></p><iframe sandbox="allow-scripts ALLOW-SAME-ORIGIN" src="/frame.html"></iframe>`,
		"a URL with spaces around it":  `<p class="a"></p><iframe src="  /frame.html "></iframe>`,
		"a scheme that is not a URL's": `<p class="a"></p><iframe src="1:/frame.html"></iframe>`,
	} {
		out, stats := mustPrune(t, css, parsePage(t, page))
		if out != css || !stats.Unpruned || !strings.HasPrefix(stats.Why, "the page has a document of the site in a frame (`<") {
			t.Errorf("%s: pruned: %q %+v", name, out, stats)
		}
	}
	for name, page := range map[string]string{
		"another site":                `<p class="a"></p><iframe src="https://www.youtube-nocookie.com/embed/x"></iframe>`,
		"another host":                `<p class="a"></p><iframe src="//example.com/frame.html"></iframe>`,
		"another host, backslashes":   `<p class="a"></p><iframe src="/\example.com/frame.html"></iframe>`,
		"a data: URL":                 `<p class="a"></p><iframe src="data:text/html,<p>x</p>"></iframe>`,
		"about:blank":                 `<p class="a"></p><iframe src="about:blank"></iframe>`,
		"no document":                 `<p class="a"></p><iframe></iframe><iframe src=""></iframe><object></object>`,
		"sandboxed":                   `<p class="a"></p><iframe sandbox src="/frame.html"></iframe>`,
		"sandboxed, scripts alone":    `<p class="a"></p><iframe sandbox="allow-scripts allow-forms" srcdoc="<script>go()</script>"></iframe>`,
		"sandboxed, the origin alone": `<p class="a"></p><iframe sandbox="allow-same-origin" src="/frame.html"></iframe>`,
		"an image":                    `<p class="a"></p><img src="/drawing.svg">`,
	} {
		out, stats := mustPrune(t, css, parsePage(t, page))
		if want := `.a{x:y}`; out != want || stats.Unpruned || stats.Why != "" {
			t.Errorf("%s:\n got: %s\nwant: %s\n%+v", name, out, want, stats)
		}
	}
	// A frame's `javascript:` URL is a script of the page's own, as ever.
	if _, stats := mustPrune(t, css, parsePage(t, `<p class="a"></p><iframe src="javascript:go()"></iframe>`)); stats.Why != "the page has a script of its own" {
		t.Errorf("a javascript: frame: %+v", stats)
	}
}

// The behaviours of @reactogenic/ui, built as a page's script is — bundled,
// minified, every flag on: none names an API that changes the tree, so a
// page that mounts them is pruned (builder.md, CSS, *The page's script*).
// One that starts to is not a mistake; its pages lose their pruning, and
// this test says which.
func TestDesignSystemScripts(t *testing.T) {
	files, err := filepath.Glob("../../../../packages/ui/src/behaviors/*.ts")
	if err != nil || len(files) == 0 {
		t.Fatalf("no behaviours of @reactogenic/ui: %v", err)
	}
	flag := regexp.MustCompile(`\bRG_[A-Z0-9_]+\b`)
	state := uiState(t)
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		define := map[string]string{}
		for _, name := range flag.FindAllString(string(source), -1) {
			define[name] = "true"
		}
		r := api.Build(api.BuildOptions{
			EntryPoints: []string{file}, Bundle: true, Write: false, Format: api.FormatESModule, Define: define,
			MinifyWhitespace: true, MinifyIdentifiers: true, MinifySyntax: true, LogLevel: api.LogLevelSilent,
		})
		if len(r.Errors) > 0 || len(r.OutputFiles) != 1 {
			t.Fatalf("%s: %v", file, r.Errors)
		}
		names := scriptNames(string(r.OutputFiles[0].Contents))
		if names.tree != "" {
			t.Errorf("%s names `%s`: a page that mounts it is not pruned", filepath.Base(file), names.tree)
		}
		// Nor one that would make every class or id of its pages "maybe".
		if names.writesClass() || names.writesID() {
			t.Errorf("%s may write `class` or `id` whole: classes %v, ids %v", filepath.Base(file), names.writesClass(), names.writesID())
		}
		// What it names is state a selector may be about: none of it a
		// class the CSS convention forbids a script to toggle — and no
		// attribute written through an API that could take a name the
		// reader does not see (`setAttribute("aria-" + state)`).
		for _, name := range []string{
			"classList", "className", "dataset", "setAttribute", "toggleAttribute", "removeAttribute",
			"setAttributeNS", "removeAttributeNS", "setAttributeNode", "setAttributeNodeNS", "removeAttributeNode",
			"setNamedItem", "setNamedItemNS", "removeNamedItem", "removeNamedItemNS",
		} {
			if names.words[name] {
				t.Errorf("%s names `%s`: it writes what the HTML does not show", filepath.Base(file), name)
			}
		}
		// The state the design system's CSS selects on, and each behaviour
		// names: on a page that mounts it those rules are "maybe"
		// (builder.md, CSS, *Runtime state* — state only a script writes is
		// decided on the page unless the page's script names it). So a
		// behaviour that writes `aria-expanded` has to be seen here, by the
		// attribute's name or the property's: one that starts to is not a
		// mistake, and this table says which rules its pages then keep.
		var named []string
		for _, attr := range state {
			if !dynamic(attr) && names.attribute(attr) {
				named = append(named, attr)
			}
		}
		if got, want := strings.Join(named, " "), stateNamed[filepath.Base(file)]; got != want {
			t.Errorf("%s names the state %q, not %q: update stateNamed, and components.md (*CSS convention*)", filepath.Base(file), got, want)
		}
	}
	// The table is about something: the CSS does select on state.
	if !slices.Contains(state, "aria-disabled") || !slices.Contains(state, "aria-current") {
		t.Errorf("the state attributes of @reactogenic/ui's CSS: %v", state)
	}
}

// stateNamed: the attributes of @reactogenic/ui's CSS that each behaviour
// names — but `open`, `hidden` and `style`, which are "maybe" without it.
// `menu-keys` writes none: it reads `aria-disabled`, in the selector of the
// items it moves between — named is named, the safe side.
var stateNamed = map[string]string{
	"invokers.ts":  "",
	"menu-keys.ts": "aria-disabled",
	"overlays.ts":  "",
}

// uiState is every attribute a selector of @reactogenic/ui's CSS is about,
// sorted.
func uiState(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../../../packages/ui/src/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("no CSS of @reactogenic/ui: %v", err)
	}
	seen := map[string]bool{}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`\[\s*([a-zA-Z][\w-]*)\s*(?:[~|^$*]?=[^\]]*)?\]`).FindAllStringSubmatch(string(source), -1) {
			seen[strings.ToLower(m[1])] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}
