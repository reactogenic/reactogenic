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

// Slot args on an attachment (syntax.md, *Slots → Grammar*).
func TestSlotArgs(t *testing.T) {
	text := `const x = <option slot={$Option} &size &label={o.label} &&value={o.value} &&selected>t</option>;`
	file, attrs := attributes(t, text)
	noErrors(t, file)
	if ref, _ := SlotAttachment(attrs[0].Parent.Parent); ref == nil || rtsx.NodeText(ref) != "$Option" {
		t.Fatalf("attachment not found")
	}
	want := []struct {
		src  string
		name string
		kind ArgKind
	}{
		{`slot={$Option}`, "", NotArg},
		{`&size`, "size", Arg},
		{`&label={o.label}`, "label", Arg},
		{`&&value={o.value}`, "value", ArgProp},
		{`&&selected`, "selected", ArgProp},
	}
	if len(attrs) != len(want) {
		t.Fatalf("want %d attributes, got %d", len(want), len(attrs))
	}
	for i, w := range want {
		name, kind := SlotArg(attrs[i])
		if name != w.name || kind != w.kind {
			t.Errorf("%s: got %q %v", w.src, name, kind)
		}
		start := strings.Index(text, w.src)
		if got := text[rtsx.TokenStart(file, attrs[i]):attrs[i].End()]; got != w.src || rtsx.TokenStart(file, attrs[i]) != start {
			t.Errorf("span: got %q, want %q", got, w.src)
		}
	}
}

func TestSlotArgErrors(t *testing.T) {
	for _, text := range []string{
		`const x = <option slot={$Option} & size />;`, // whitespace after &
		`const x = <option slot={$Option} &{x} />;`,
	} {
		if file := rtsx.ParseRTSX("/test.rtsx", text); len(file.Diagnostics()) == 0 {
			t.Errorf("want a parse error: %s", text)
		}
	}
	if file := rtsx.ParseTSX("/test.tsx", `const x = <option slot={$Option} &size />;`); len(file.Diagnostics()) == 0 {
		t.Error("want a syntax error in .tsx")
	}
}

// The error of a half-typed arg — `&` without a name yet, `& name` — is on
// the arg attribute, so whoever asks the tree what is broken finds it there
// (ide.md, *Tolerance*).
func TestSlotArgErrorFlag(t *testing.T) {
	for _, text := range []string{
		`const x = <span slot={$Icon} &></span>;`,
		`const x = <span slot={$Icon} & size></span>;`,
		`const x = <span slot={$Icon} &&></span>;`,
		`const x = <Match& on={s}>x</Match>;`,
	} {
		file, attrs := attributes(t, text)
		rtsx.Bind(file)
		if len(file.Diagnostics()) == 0 {
			t.Fatalf("want a parse error: %s", text)
		}
		flagged := false
		for _, attr := range attrs {
			if _, kind := SlotArg(attr); kind != NotArg {
				flagged = rtsx.HasParseError(attr)
			}
		}
		if !flagged {
			t.Errorf("the arg attribute carries no parse-error flag: %s", text)
		}
	}
	// A well-formed arg carries none.
	file, attrs := attributes(t, `const x = <span slot={$Icon} &size></span>;`)
	rtsx.Bind(file)
	if rtsx.HasParseError(attrs[1]) {
		t.Error("`&size` is flagged")
	}
}

// BlankText is the scanner's rule on the text itself. After error recovery
// the parser's flag is stale — here on the line breaks around the mismatched
// `</$Icon>` — and a caller that trusted it would take formatting for content.
func TestBlankText(t *testing.T) {
	texts := func(text string) (blank, content, stale int) {
		var visit func(n *rtsx.Node) bool
		visit = func(n *rtsx.Node) bool {
			if n.Kind == rtsx.KindJsxText {
				switch {
				case !BlankText(n):
					content++
				case !n.AsJsxText().ContainsOnlyTriviaWhiteSpaces:
					blank, stale = blank+1, stale+1
				default:
					blank++
				}
			}
			return n.ForEachChild(visit)
		}
		rtsx.ParseRTSX("/test.rtsx", text).AsNode().ForEachChild(visit)
		return
	}
	// Valid: the line breaks are formatting; ` `, `x` and ` y ` are content.
	if blank, content, stale := texts("const a = (\n  <A>\n    <b> {x}</b>x<i> y </i>\n  </A>\n);\n"); blank != 2 || content != 3 || stale != 0 {
		t.Errorf("valid: %d blank, %d content, %d stale; want 2, 3, 0", blank, content, stale)
	}
	// Half-typed: every text but `x` is formatting, whatever the flag says.
	blank, content, stale := texts("const a = (\n  <A>\n    <$B>\n      <$Icn>x</$Icon>\n    </$B>\n  </A>\n);\n")
	if stale == 0 {
		t.Fatal("no stale flag — the case tests nothing")
	}
	if blank != 4 || content != 1 {
		t.Errorf("half-typed: %d blank, %d content; want 4, 1", blank, content)
	}
}
