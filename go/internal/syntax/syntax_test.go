package syntax

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// attributes parses text as .rtsx and returns the attributes of its first
// JSX element.
func attributes(t *testing.T, text string) (*rtsx.SourceFile, []*rtsx.Node) {
	t.Helper()
	file := rtsx.ParseRTSX("/test.rtsx", text)
	var attrs []*rtsx.Node
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxAttributes {
			attrs = n.Properties()
			return true
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return file, attrs
}

func noErrors(t *testing.T, file *rtsx.SourceFile) {
	t.Helper()
	for _, d := range file.Diagnostics() {
		t.Errorf("unexpected parse error at %d: %s", d.Pos(), rtsx.Message(d))
	}
}

// The grammar table of syntax.md, *Slots → Grammar*.
func TestSlotParamsGrammar(t *testing.T) {
	cases := []struct {
		attr   string // the attribute under test, written after `<$X a="1" `
		params bool
		names  []string // bound names, in order; "...rest" for a rest element
	}{
		{`{...rest}`, false, nil},
		{`{ ...rest }`, false, nil},
		{`{ size }`, true, []string{"size"}},
		{`{ size: s }`, true, []string{"s"}},
		{`{ size = "md" }`, true, []string{"size"}},
		{`{ row: { id } }`, true, []string{"{ id }"}},
		{`{}`, true, nil},
		{`{ size, ...rest }`, true, []string{"size", "...rest"}},
		{`{ label: optionLabel, value: optionValue }`, true, []string{"optionLabel", "optionValue"}},
	}
	for _, c := range cases {
		t.Run(c.attr, func(t *testing.T) {
			text := `const x = <$X a="1" ` + c.attr + `>body</$X>;`
			file, attrs := attributes(t, text)
			noErrors(t, file)
			if len(attrs) != 2 {
				t.Fatalf("want 2 attributes, got %d", len(attrs))
			}
			pattern, ok := SlotParams(attrs[1])
			if ok != c.params {
				t.Fatalf("SlotParams = %v, want %v", ok, c.params)
			}
			if !ok {
				if attrs[1].Kind != rtsx.KindJsxSpreadAttribute {
					t.Fatalf("want a spread attribute, got %v", attrs[1].Kind)
				}
				return
			}
			// Exact span: from `{` to after `}`, for the attribute and the pattern.
			start := strings.Index(text, c.attr)
			end := start + len(c.attr)
			for _, n := range []*rtsx.Node{attrs[1], pattern} {
				if got := rtsx.TokenStart(file, n); got != start || n.End() != end {
					t.Errorf("%v span = [%d,%d), want [%d,%d)", n.Kind, got, n.End(), start, end)
				}
			}
			var names []string
			for _, el := range pattern.Elements() {
				name := text[rtsx.TokenStart(file, el.Name()):el.Name().End()]
				if el.AsBindingElement().DotDotDotToken != nil {
					name = "..." + name
				}
				names = append(names, name)
			}
			if strings.Join(names, ",") != strings.Join(c.names, ",") {
				t.Errorf("names = %v, want %v", names, c.names)
			}
		})
	}
}

// Only .rtsx gets the extension: in .tsx, `{` must still be followed by `...`.
func TestSlotParamsOnlyInRTSX(t *testing.T) {
	text := `const x = <$X { size }>body</$X>;`
	if file := rtsx.ParseTSX("/test.tsx", text); len(file.Diagnostics()) == 0 {
		t.Error("want a syntax error in .tsx")
	}
	file, attrs := attributes(t, text)
	noErrors(t, file)
	if _, ok := SlotParams(attrs[0]); !ok {
		t.Error("want slot params in .rtsx")
	}
}

// Params on a component element, next to other attributes and children.
func TestSlotParamsOnComponent(t *testing.T) {
	file, attrs := attributes(t, "const x = (\n  <Each items { item, index }>\n    <p key={item}>{item}</p>\n  </Each>\n);")
	noErrors(t, file)
	if len(attrs) != 2 {
		t.Fatalf("want 2 attributes, got %d", len(attrs))
	}
	if _, ok := SlotParams(attrs[0]); ok {
		t.Error("`items` is not params")
	}
	if _, ok := SlotParams(attrs[1]); !ok {
		t.Error("want params")
	}
}

// Malformed params are ordinary parse errors, and parsing goes on.
func TestSlotParamsErrors(t *testing.T) {
	for _, text := range []string{
		`const x = <$X { size >body</$X>; const y = 1;`,
		`const x = <$X { 1 }>body</$X>;`,
		`const x = <$X { size: }>body</$X>;`,
	} {
		file := rtsx.ParseRTSX("/test.rtsx", text)
		if len(file.Diagnostics()) == 0 {
			t.Errorf("want a parse error: %s", text)
		}
	}
}

// Segment roots: syntax.md, *Segment roots → Grammar*. tsgo needs no patch:
// it already reads `#about-us` as a JsxAttribute named "#about-us".
func TestSegmentRoot(t *testing.T) {
	cases := []struct {
		text string
		want []string // per attribute: the segment name, or "" when not a segment root
	}{
		{`<section #about-us />`, []string{"about-us"}},
		{`<section #AboutUs />`, []string{"AboutUs"}},
		{`<section #about-us></section>`, []string{"about-us"}},
		{`<section {...p} #about-us className="band" />`, []string{"", "about-us", ""}},
		{`<Section #about-us { size } />`, []string{"about-us", ""}},
		{`<Card #counter />`, []string{"counter"}},
		// Not segment roots:
		{`<section id="about-us" />`, []string{""}},
		{`<section about-us />`, []string{""}},
		{`<section #about-us="x" />`, []string{""}},
		{`<section #about:us />`, []string{""}},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			text := "const x = " + c.text + ";"
			file, attrs := attributes(t, text)
			noErrors(t, file)
			if len(attrs) != len(c.want) {
				t.Fatalf("want %d attributes, got %d", len(c.want), len(attrs))
			}
			for i, attr := range attrs {
				name, ok := SegmentRoot(attr)
				if ok != (c.want[i] != "") || name != c.want[i] {
					t.Errorf("attribute %d: SegmentRoot = %q, %v; want %q", i, name, ok, c.want[i])
				}
				if ok {
					start := strings.Index(text, "#"+name)
					if got := rtsx.TokenStart(file, attr); got != start || attr.End() != start+1+len(name) {
						t.Errorf("span = [%d,%d), want [%d,%d)", got, attr.End(), start, start+1+len(name))
					}
				}
			}
		})
	}
}

// No whitespace between `#` and the name: tsgo's scanner rejects a lone `#`.
func TestSegmentRootWhitespace(t *testing.T) {
	file := rtsx.ParseRTSX("/test.rtsx", `const x = <section # about />;`)
	if len(file.Diagnostics()) == 0 {
		t.Fatal("want a parse error for `# about`")
	}
	if msg := rtsx.Message(file.Diagnostics()[0]); msg != "Invalid character." {
		t.Errorf("message = %q", msg)
	}
}
