package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// bareAttributes parses and binds text, and returns every value-less JSX
// attribute, keyed "name@n" where n counts attributes of that name in order.
func bareAttributes(t *testing.T, text string) (*rtsx.SourceFile, map[string]*rtsx.Node) {
	t.Helper()
	file := rtsx.ParseRTSX("/test.rtsx", text)
	noErrors(t, file)
	rtsx.Bind(file)
	attrs := map[string]*rtsx.Node{}
	count := map[string]int{}
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind == rtsx.KindJsxAttribute && n.Initializer() == nil {
			name := rtsx.NodeText(n.Name())
			count[name]++
			attrs[fmt.Sprintf("%s@%d", name, count[name])] = n
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return file, attrs
}

// The table of syntax.md, *What "in scope" means*, plus slot params.
func TestBinding(t *testing.T) {
	src := `import { value } from "./value";
import type { Kind } from "./kind";
import { type Tone } from "./tone";
declare const ambient: string;
const top = 1;
function helper() {}
class Box {}

export function Field({ onChange }: { onChange: () => void }, [first]: string[]) {
  const local = 2;
  return (
    <Form>
      <Input value onChange local top helper Box first />
      <Input name status window Kind Tone ambient />
      <$Icon size { size, row: { id } }>
        <Icon size id />
      </$Icon>
      <$Row { local }>
        {() => { const local = 3; return <Cell local />; }}
      </$Row>
    </Form>
  );
}
`
	file, attrs := bareAttributes(t, src)
	cases := []struct {
		attr string
		want string // text of the declaration's first line, or "" for none
	}{
		{"value@1", `value`},       // value import
		{"onChange@1", `onChange`}, // destructured parameter
		{"local@1", `local = 2`},   // const
		{"top@1", `top = 1`},       // module const
		{"helper@1", `function helper() {}`},
		{"Box@1", `class Box {}`},
		{"first@1", `first`}, // array-destructured parameter
		{"name@1", ""},       // globals never count: window.name
		{"status@1", ""},     // window.status
		{"window@1", ""},
		{"Kind@1", ""},     // import type
		{"Tone@1", ""},     // import { type … }
		{"ambient@1", ""},  // declare const
		{"size@1", ""},     // the slot element's own attribute: params are not in scope there
		{"size@2", `size`}, // in the body: the param
		{"id@1", `id`},     // nested pattern
		// `<Cell local />` is in a function inside the $Row body that declares
		// its own `local`: the nearer binding wins over the param.
		{"local@2", `local = 3`},
	}

	for _, c := range cases {
		attr, ok := attrs[c.attr]
		if !ok {
			t.Fatalf("no attribute %s", c.attr)
		}
		decl := Binding(attr, strings.Split(c.attr, "@")[0])
		got := ""
		if decl != nil {
			got = strings.SplitN(src[rtsx.TokenStart(file, decl):decl.End()], "\n", 2)[0]
		}
		if got != c.want {
			t.Errorf("%s: binding %q, want %q", c.attr, got, c.want)
		}
	}
}

// A param shadows an outer binding of the same name inside the body only.
func TestParamShadowsOuter(t *testing.T) {
	src := "const size = 1;\nexport const x = (\n  <$Icon size { size }>\n    <Icon size />\n  </$Icon>\n);\n"
	file, attrs := bareAttributes(t, src)
	own := Binding(attrs["size@1"], "size")
	body := Binding(attrs["size@2"], "size")
	if own == nil || !strings.HasPrefix(src[rtsx.TokenStart(file, own):], "size = 1") {
		t.Errorf("the slot element's own attribute should see the outer const")
	}
	if body == nil || body.Kind != rtsx.KindIdentifier || rtsx.TokenStart(file, body) != strings.Index(src, "{ size }")+2 {
		t.Errorf("the body should see the param")
	}
}
