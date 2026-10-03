package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		text string
		want []string // "code@text", where text is the reported span
	}{
		// Allowed.
		{`<$X { size }>b</$X>`, nil},
		{`<Each items { item }>b</Each>`, nil},
		{`<Icons.Plus { size } />`, nil},
		{`<section #about-us className="band" />`, nil},
		{`<section {...p} />`, nil},
		// A `$` tag is a slot whatever follows the `$`: a hyphen does not make
		// it an HTML element.
		{`<$sub-item { size }>b</$sub-item>`, nil},
		{`<$icon-start { a } { b } />`, []string{"duplicate-params@{ b }"}},
		// params-on-html
		{`<div { size }>b</div>`, []string{"params-on-html@{ size }"}},
		{`<my-element { size } />`, []string{"params-on-html@{ size }"}},
		{`<svg:rect {} />`, []string{"params-on-html@{}"}},
		// duplicate-params
		{`<$X { a } { b }>b</$X>`, []string{"duplicate-params@{ b }"}},
		// segment-id: a second #name
		{`<section #about #contact />`, []string{"segment-id@#contact"}},
		// segment-syntax
		{`<section #about-us="x" />`, []string{"segment-syntax@#about-us=\"x\""}},
		{`<section #about:us />`, []string{"segment-syntax@#about:us"}},
		// Args need an attachment.
		{`<option slot={$Option} &size &&value={v} />`, nil},
		{`<option &size />`, []string{"arg-without-slot@&size"}},
		{`<option slot="header" &&value={v} />`, []string{"arg-without-slot@&&value={v}"}},
		// One bad attribute does not hide the rest, on this element or others.
		{`<div { a } #x #y><section #p="1" /></div>`, []string{"params-on-html@{ a }", "segment-id@#y", "segment-syntax@#p=\"1\""}},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			text := "const x = " + c.text + ";"
			file := rtsx.ParseRTSX("/test.rtsx", text)
			noErrors(t, file)
			var got []string
			for _, e := range Check(file) {
				got = append(got, fmt.Sprintf("%s@%s", e.Code, text[e.Pos:e.End]))
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
