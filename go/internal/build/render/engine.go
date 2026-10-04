package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/quickjs"
)

// engine executes the render bundle (plan.md, RGP2-011): one runtime per
// build. Nothing of the host is in it — no file system, no network, no
// clock: the bundle's sandbox module sees to the clock.
type engine struct {
	vm      *quickjs.VM
	timeout time.Duration
}

// maxDepth bounds the engine's call depth: about one unit per JavaScript
// frame, two for a call through a built-in. The engine's own default is, in
// effect, none — and a function that recurses without end then overflows the
// Go stack (7.7 kB per frame, 1 GB allowed), which no recover catches: the
// process dies. Bounded, it is an `InternalError: stack overflow` of the
// page. React's renderer takes four frames per level of nesting.
const maxDepth = 10_000

// thrown is an exception of shell code, as the engine saw it.
type thrown struct {
	Name    string  `json:"name"`    // an Error's name: a diagnostic code when the builder threw it
	Message string  `json:"message"` // an Error's message; anything else, as a string
	Stack   string  `json:"stack"`   // the engine's, in the bundle's coordinates
	Owners  []owner `json:"owners"`  // the components it passed through, innermost first
	After   bool    `json:"after"`   // no component threw it: Owners are those of the component called last
}

// owner is a component of an exception's stack, and where its element was
// written: a position in the text of a module, as esbuild's jsxDEV gives it
// (1-based, columns in UTF-16 code units). No file: the page itself.
type owner struct {
	Name         string `json:"name"`
	FileName     string `json:"fileName"`
	LineNumber   int    `json:"lineNumber"`
	ColumnNumber int    `json:"columnNumber"`
}

// printed is one call of `console`, and the engine's stack there.
type printed struct {
	Level string `json:"level"`
	Text  string `json:"text"`
	Stack string `json:"stack"`
}

// rendered is the answer of the bundle's `__reactogenic_render_json`.
type rendered struct {
	Page *struct {
		HTML   string `json:"html"`
		Mounts []struct {
			Module string          `json:"module"`
			ID     string          `json:"id"`
			Flags  map[string]bool `json:"flags"`
		} `json:"mounts"`
		Components map[string]int `json:"components"`
	} `json:"page"`
	Error *thrown `json:"error"`
}

// start loads the bundle. What a module throws while it loads — at its top
// level — is the *thrown returned: no page can render then.
func start(code string, timeout time.Duration) (*engine, *thrown, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, nil, err
	}
	if err := vm.SetEvalTimeout(timeout); err != nil {
		vm.Close()
		return nil, nil, err
	}
	vm.SetMaxStackSize(maxDepth)
	e := &engine{vm, timeout}
	if _, err := vm.Eval(code, quickjs.EvalGlobal); err != nil {
		return e, e.exception(err), nil
	}
	return e, nil, nil
}

func (e *engine) close() {
	e.vm.Close()
}

// render renders one page.
func (e *engine) render(pathname string) (*rendered, error) {
	value, err := e.vm.Call("__reactogenic_render_json", pathname)
	if err != nil {
		// Not an exception of the page — those come back as JSON: the
		// engine ended the call.
		return &rendered{Error: e.exception(err)}, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("render: the bundle answered %T", value)
	}
	var r rendered
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return &r, nil
}

// console returns what shell code printed since the last call.
func (e *engine) console() []printed {
	value, err := e.vm.Call("__reactogenic_console")
	text, ok := value.(string)
	if err != nil || !ok {
		return nil // the sandbox module did not load: nothing was collected
	}
	var messages []printed
	json.Unmarshal([]byte(text), &messages)
	return messages
}

// exception reads an error of the engine's.
func (e *engine) exception(err error) *thrown {
	var js *quickjs.Error
	if !errors.As(err, &js) {
		return &thrown{Message: strings.TrimSpace(err.Error())}
	}
	t := &thrown{Name: js.Name, Message: js.Message, Stack: js.Stack}
	if t.Name == "InternalError" && t.Message == "interrupted" { // the timeout's; the stack is where it struck
		t.Name, t.Message = "", fmt.Sprintf("Rendering did not end in %v: a loop without an end?", e.timeout)
	}
	return t
}
