package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/quickjs"
)

// engine executes the render bundle (plan.md, RGP2-011) for one page: a
// runtime of its own, so nothing a page leaves behind — a module's state, a
// global, a patched prototype — is there for the next (builder.md, *Shell
// code in phase 2*). Nothing of the host is in it — no file system, no
// network, no clock: the bundle's sandbox module sees to the clock.
type engine struct {
	vm *quickjs.VM
	limits
}

// limits are what ends a page that does not end by itself (Options).
type limits struct {
	timeout time.Duration
	memory  uintptr
}

// maxMemory is the memory of one page's runtime when Options sets none. A
// loop that keeps what it makes took 512 MiB in 1.7 s (darwin-arm64): the
// thirty seconds of the timeout would be some 9 GiB.
const maxMemory = 1 << 30

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
// (1-based, columns in UTF-16 code units) — or, for an element made by
// `createElement`, the engine's stack where it was made — or nothing: an
// element that a package made.
type owner struct {
	Name         string `json:"name"`
	Page         bool   `json:"page"` // the page itself: the root
	FileName     string `json:"fileName"`
	LineNumber   int    `json:"lineNumber"`
	ColumnNumber int    `json:"columnNumber"`
	Stack        string `json:"stack"`
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
			Data   string          `json:"data"` // JSON text, its keys sorted; "": none
		} `json:"mounts"`
		Components map[string]int `json:"components"`
		Classes    []struct {
			Name string   `json:"name"`
			By   []string `json:"by"` // the components that resolved it
		} `json:"classes"`
	} `json:"page"`
	Error *thrown `json:"error"`
}

// start loads the bundle from its text. What a module throws while it loads
// — at its top level — is the *thrown returned: no page can render then.
func start(code string, timeout time.Duration) (*engine, *thrown, error) {
	return boot(limits{timeout, maxMemory}, func(vm *quickjs.VM) error {
		_, err := vm.Eval(code, quickjs.EvalGlobal)
		return err
	})
}

// compile is the bundle as the engine's bytecode: its text is read once, and
// the runtime of each page loads it from there — 2.9 ms a runtime, where
// reading the text again takes 27.6 (the fixture site's bundle, 388 kB, most
// of it React's renderer; darwin-arm64).
func compile(code string) ([]byte, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, err
	}
	defer vm.Close()
	return vm.Compile(code, quickjs.EvalGlobal)
}

// startCompiled is start, from the bundle's bytecode.
func startCompiled(bytecode []byte, within limits) (*engine, *thrown, error) {
	return boot(within, func(vm *quickjs.VM) error {
		_, err := vm.EvalBytecode(bytecode)
		return err
	})
}

// boot makes a runtime and loads the bundle into it.
func boot(within limits, bundle func(*quickjs.VM) error) (*engine, *thrown, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, nil, err
	}
	vm.SetMaxStackSize(maxDepth)
	vm.SetMemoryLimit(within.memory)
	e := &engine{vm, within}
	stop := e.watch()
	err = bundle(vm)
	stop()
	if err != nil {
		return e, e.exception(err), nil
	}
	return e, nil, nil
}

func (e *engine) close() {
	e.vm.Close()
}

// again is how often the watch raises its interrupt once the timeout has
// passed: the engine takes the flag down whenever a call begins, so one
// raised before that is lost, and the next is not.
const again = 10 * time.Millisecond

// watch ends what the engine runs from here on once it has run for the
// timeout; the stop it returns is called when the engine has returned, and
// before it is closed. The time is a Go timer's — the monotonic clock, which
// stands still while the machine sleeps. The engine's own SetEvalTimeout is
// not used: its deadline is a moment of the wall clock, so a page that had
// run for a fraction of a second when the machine went to sleep, or when the
// clock was set, was ended on waking with the timeout's message (plan.md,
// RGP2-071: that was TestMemory's failure, twice in some thirty-five runs).
func (e *engine) watch() (stop func()) {
	done, gone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(gone)
		timer := time.NewTimer(e.timeout)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-timer.C:
				e.vm.Interrupt()
				timer.Reset(again)
			}
		}
	}()
	return func() {
		close(done)
		<-gone // no interrupt is raised on an engine that may be closed
	}
}

// render renders one document: path is its Route.Path.
func (e *engine) render(path string) (*rendered, error) {
	defer e.watch()()
	value, err := e.vm.Call("__reactogenic_render_json", path)
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
	if r.Error != nil {
		e.ended(r.Error)
	}
	return &r, nil
}

// console returns what shell code printed since the last call.
func (e *engine) console() []printed {
	defer e.watch()()
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
	return e.ended(&thrown{Name: js.Name, Message: js.Message, Stack: js.Stack})
}

// ended words what the engine itself ended, by its limits; the stack is
// where the limit struck. Running out of memory is an exception that shell
// code may catch — so it also arrives as a page's own (render).
func (e *engine) ended(t *thrown) *thrown {
	switch {
	case t.Name == "InternalError" && t.Message == "interrupted":
		t.Name, t.Message = "", fmt.Sprintf("Rendering did not end in %v: a loop without an end?", e.timeout)
	case t.Name == "InternalError" && t.Message == "out of memory":
		t.Name, t.Message = "", fmt.Sprintf("Rendering took more than %d MiB of memory: a loop that keeps what it makes?", e.memory>>20)
	}
	return t
}
