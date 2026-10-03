package transpiler

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
)

// Input is one transpilation: the entry file plus the files it may need
// (segments, containers), keyed by path.
type Input struct {
	Files map[string]string
	Entry string
	// UntilPass stops after the given pass of syntax.md, *Compilation
	// passes*; 0 runs them all.
	UntilPass int
	// ReadFile reads a file that is not in Files — segments next to the
	// entry. nil: only Files exist.
	ReadFile func(path string) (string, bool)
	// Tolerant is for the editor (ide.md, *Tolerance*): the passes run on
	// the parser's recovered tree instead of stopping at a syntax error, and
	// a pass that fails keeps the previous pass's text (Output.Stopped). The
	// builds — Vite, `reactogenic check` — are strict.
	Tolerant bool
}

// readFile looks in Files, then ReadFile.
func (in Input) readFile(p string) (string, bool) {
	if text, ok := in.Files[p]; ok {
		return text, true
	}
	if in.ReadFile != nil {
		return in.ReadFile(p)
	}
	return "", false
}

type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a transpiler error on the .rtsx source. Line and Col are
// 1-based.
type Diagnostic struct {
	File     string
	Line     int
	Col      int
	Span     emit.Span // in the source
	Severity Severity
	Code     string // e.g. "orphan-slot"; "TS1005" for a TypeScript syntax error
	Message  string
}

type Output struct {
	TSX         string
	Diagnostics []Diagnostic
	// Map maps TSX back to the entry's source (diagnostics.md, *Mapping*).
	Map *emit.Map
	// Notes say what the code synthesized for a construct is, so a TS error
	// on it can be reworded in the author's terms (diagnostics.md,
	// *Rewrites*).
	Notes []Note
	// Generated maps each generated name to what the author wrote:
	// `_Div_num` → `#num`, `_on` → `getStatus()`.
	Generated map[string]string
	// Shorthands are the bare names that stand for `name={name}`: one source
	// token, emitted twice — as the prop (or arg key) and as the binding.
	// A rename through one must expand it (ide.md, *Rename*).
	Shorthands []Shorthand
	// SlotGroups: per owner and slot name, the tag names of every slot
	// element that fills it. Only the first is copied into the emitted prop
	// name, so only it answers the editor directly (ide.md, *Span map*).
	SlotGroups []SlotGroup
	// Stopped, in tolerant mode: why the passes ended early, starting with
	// the pass ("pass 2 (flow lowering): …"). TSX is then the last good text
	// — up to the source itself — and still holds unlowered constructs, so
	// TS's diagnostics for it mean nothing.
	Stopped string
	// Dropped: code the author wrote is not in TSX — a `Switch` or `Match`
	// that cannot be lowered and an orphaned slot element are replaced by
	// `null`; a half-typed `$Case` with a body, a conditional child that
	// mixes slot elements with anything else, and a `children` attribute next
	// to a body are left out. What TS says about the file is then not about
	// the author's code: names used only there read as unused (ide.md,
	// *Tolerance*).
	Dropped bool
}

// Shorthand is a bare name with two meanings.
type Shorthand struct {
	Name emit.Span // in the source
	Kind string    // attr: `<Input value />`; arg: `&size`; arg-prop: `&&size`
}

// SlotGroup lists the slot elements of one name under one owner.
type SlotGroup struct {
	Name  string
	Owner emit.Span   // the owner's opening tag, in the source
	Tags  []emit.Span // tag names, opening and closing, in source order; Tags[0] is the copied one
}

// Note marks a source span: what was synthesized for it.
type Note struct {
	Span   emit.Span // in the source
	Kind   string    // slot-prop, slot-body, slot-params, slot-args, slot-arg, slot-conditional, slot-key, slot-entry-key, attachment, no-match, segment, shorthand-true
	Name   string    // the slot (`$Title`), segment (`about-us`) or attribute
	Detail string    // the container tag, the arg name, …
	// Tag is, for slot-conditional, the owner's tag name in the source: the
	// component whose slot may be left NOT_ASSIGNED; for slot-entry-key, the
	// slot element.
	Tag emit.Span
}

// A pass reads its input — the source, or the previous pass's output — and
// returns edits to it (syntax.md, *Compilation passes*).
// A repeating pass runs again on its own output until it returns no edits.
type pass struct {
	n      int
	name   string
	run    func(c *passContext) []emit.Edit
	repeat bool
}

var passes = []pass{
	{0, "checks", checks, false},
	{1, "shorthand props", shorthand, false},
	{2, "flow lowering", flow, true},
	{3, "slot hoisting", slots, true},
	{4, "segment roots", segments, false},
}

// maxRuns bounds a repeating pass; a pass that keeps editing is a bug.
const maxRuns = 1000

// Transpile turns Input.Entry from .rtsx into .tsx. Errors in the source are
// Diagnostics; the error result is for failures of the transpiler itself.
func Transpile(in Input) (Output, error) {
	src, ok := in.Files[in.Entry]
	if !ok {
		return Output{}, fmt.Errorf("transpiler: no file %q", in.Entry)
	}
	var (
		out      Output
		text     = src
		toSource = emit.Identity(len(src))
		names    map[string]bool
		runs     int
		broken   bool // tolerant: the source has syntax errors
		// The source parse and the ranges of its errors: where a half-typed
		// construct is recognised (ide.md, *Tolerance*, rule 3).
		source     *rtsx.SourceFile
		sourceErrs []emit.Span
		writer     pass // the pass that wrote text
		// Text emitted twice (an attachment's fallback) goes through the
		// later runs twice; what is found in it is recorded once.
		seenDiagnostics = map[Diagnostic]bool{}
		seenNotes       = map[Note]bool{}
		seenGroups      = map[string]bool{}
	)
	note := func(n Note) {
		if !seenNotes[n] {
			seenNotes[n] = true
			out.Notes = append(out.Notes, n)
		}
	}
	// stop ends a tolerant run early, keeping the last good text.
	stop := func(p pass, why any) (Output, error) {
		out.TSX, out.Map = text, toSource
		out.Stopped = fmt.Sprintf("pass %d (%s): %v", p.n, p.name, why)
		return out, nil
	}
	for i := 0; i < len(passes); i++ {
		p := passes[i]
		if in.UntilPass > 0 && p.n > in.UntilPass {
			break
		}
		file := rtsx.ParseRTSX("/"+in.Entry, text)
		parseErrors := file.Diagnostics()
		if len(parseErrors) > 0 {
			// Emitted text that does not parse is our bug only when the
			// source did: a broken source may stay broken through a pass.
			if p.n > 0 && !broken {
				return Output{}, fmt.Errorf("transpiler: pass %d (%s) produced code that does not parse: %s", writer.n, writer.name, rtsx.Message(parseErrors[0]))
			}
			if p.n == 0 {
				// Syntax errors are the source parse's, only.
				for _, d := range parseErrors {
					// A zero-length error sits before the trivia: never past its end.
					out.add(in.Entry, src, emit.Span{Pos: min(rtsx.SkipTrivia(src, d.Pos()), d.End()), End: d.End()}, Error, fmt.Sprintf("TS%d", d.Code()), rtsx.Message(d))
				}
				if !in.Tolerant {
					return out, nil
				}
				broken = true
			}
		}
		var errs []emit.Span
		for _, d := range parseErrors {
			errs = append(errs, emit.Span{Pos: d.Pos(), End: d.End()})
		}
		if names == nil {
			names = identifiers(file)
		}
		if p.run == nil {
			continue
		}
		rtsx.Bind(file)
		if p.n == 0 {
			source, sourceErrs = file, errs
		}
		c := &passContext{file: file, text: text, names: names, imports: map[string]bool{}, entry: in.Entry, readFile: in.readFile,
			report: func(s emit.Span, sev Severity, code, msg string) bool {
				at := toSource.Source(s)
				// A half-typed construct: its errors are the parser's. The
				// source parse decides; this pass's own tree may add to it
				// (recovery can re-parent lowered text differently).
				if broken && (underParseError(source, sourceErrs, at) || file != source && underParseError(file, errs, s)) {
					return false
				}
				line, col := emit.LineCol(src, at.Pos)
				d := Diagnostic{File: in.Entry, Line: line, Col: col, Span: at, Severity: sev, Code: code, Message: msg}
				if !seenDiagnostics[d] {
					seenDiagnostics[d] = true
					out.Diagnostics = append(out.Diagnostics, d)
				}
				return true
			},
			note: func(s emit.Span, kind, name, detail string) {
				note(Note{Span: toSource.Source(s), Kind: kind, Name: name, Detail: detail})
			},
			noteTag: func(s, tag emit.Span, kind, name string) {
				note(Note{Span: toSource.Source(s), Kind: kind, Name: name, Tag: toSource.Source(tag)})
			},
			shorthandSite: func(name emit.Span, kind string) {
				out.Shorthands = append(out.Shorthands, Shorthand{Name: toSource.Source(name), Kind: kind})
			},
			slotGroup: func(name string, owner emit.Span, tags []emit.Span) {
				g := SlotGroup{Name: name, Owner: toSource.Source(owner)}
				for _, t := range tags {
					g.Tags = append(g.Tags, toSource.Source(t))
				}
				if key := fmt.Sprint(g); !seenGroups[key] {
					seenGroups[key] = true
					out.SlotGroups = append(out.SlotGroups, g)
				}
			},
			dropped: func() { out.Dropped = true },
			generated: func(local, written string) {
				if out.Generated == nil {
					out.Generated = map[string]string{}
				}
				out.Generated[local] = written
			}}
		var edits []emit.Edit
		if in.Tolerant {
			// The editor's file always keeps a virtual text: a pass that
			// panics — on a recovered tree it may meet shapes it never sees
			// in valid code — ends the passes, and says so (ide.md,
			// *Tolerance*, rule 5).
			var panicked any
			func() {
				defer func() { panicked = recover() }()
				edits = p.run(c)
			}()
			if panicked != nil {
				out.add(in.Entry, src, emit.Span{}, Error, "internal", fmt.Sprintf("pass %d (%s): %v", p.n, p.name, panicked))
				return stop(p, panicked)
			}
		} else {
			edits = p.run(c)
		}
		if len(edits) == 0 {
			continue
		}
		for i := range edits {
			oneCopyAnswers(edits[i].Pieces)
		}
		next, m, err := emit.Apply(text, edits)
		if err != nil && broken {
			return stop(p, err)
		}
		if err != nil {
			return Output{}, fmt.Errorf("transpiler: pass %d (%s): %w", p.n, p.name, err)
		}
		text, toSource, writer = next, m.Then(toSource), p
		if p.repeat {
			if runs++; runs > maxRuns && broken {
				return stop(p, "does not settle")
			}
			if runs > maxRuns {
				return Output{}, fmt.Errorf("transpiler: pass %d (%s) does not settle", p.n, p.name)
			}
			i-- // run it again on its own output
		}
	}
	out.TSX, out.Map = text, toSource
	return out, nil
}

func (o *Output) add(file, src string, s emit.Span, sev Severity, code, msg string) {
	line, col := emit.LineCol(src, s.Pos)
	o.Diagnostics = append(o.Diagnostics, Diagnostic{File: file, Line: line, Col: col, Span: s, Severity: sev, Code: code, Message: msg})
}

// underParseError: the node at s, or the nearest JSX element or fragment
// around it, holds a parse error (ide.md, *Tolerance*). Recovery re-parents
// what follows a half-typed attribute, so the unit of trust is the element.
//
// A node holds an error when the parser flagged it or one below it, or when
// the range of a parse error (errs) lies within it: an unclosed tag leaves
// no flag — recovery rebuilds the element — only its error. Nothing above
// the nearest element counts: a syntax error elsewhere in the file hides
// nothing here. A node with no element around it answers for its statement
// instead, and a diagnostic on the file as a whole (the zero span) has no
// node to be broken.
//
// The file must be bound: the binder sets the flag.
func underParseError(file *rtsx.SourceFile, errs []emit.Span, s emit.Span) bool {
	if s == (emit.Span{}) {
		return false
	}
	start := func(n *rtsx.Node) int {
		if n.Kind == rtsx.KindJsxText {
			return n.Pos() // whitespace text has no token of its own
		}
		return rtsx.TokenStart(file, n)
	}
	n := rtsx.TokenAt(file, s.Pos)
	for n != nil && (n.End() < s.End || start(n) > s.Pos) {
		n = n.Parent
	}
	if n == nil || n == file.AsNode() {
		return false
	}
	holds := func(n *rtsx.Node) bool {
		if rtsx.HasParseError(n) {
			return true
		}
		for _, e := range errs {
			if n.Pos() <= e.Pos && e.End <= n.End() {
				return true
			}
		}
		return false
	}
	if holds(n) {
		return true
	}
	for p := n.Parent; p != nil; p = p.Parent {
		if isJSXElement(p) {
			return holds(p)
		}
	}
	// No element around it: the statement it is written in. Recovery makes
	// the children of an owner whose opening tag is half-typed
	// (`<Button variant=>`) top-level expressions of that statement.
	for p := n; p.Parent != nil; p = p.Parent {
		if p.Parent.Kind != rtsx.KindBlock && p.Parent != file.AsNode() {
			continue
		}
		if rtsx.HasParseError(p) {
			return true
		}
		for _, e := range errs {
			// From its first token: an error at the end of the statement
			// before (an unterminated string) is where this one's trivia starts.
			if rtsx.TokenStart(file, p) <= e.Pos && e.End <= p.End() {
				return true
			}
		}
		return false
	}
	return false
}

// oneCopyAnswers: text copied several times into one construct (an attachment
// emitted in both branches of its ternary, a Switch subject per case, a
// slot's previous value in each null branch of a conditional) is checked in
// each place but answers the editor from the first only (ide.md, *Span
// map*). Copies with different masks — the two copies of a shorthand — are
// different symbols and both answer; the same copy again, with the same
// mask, is one more identical copy.
func oneCopyAnswers(pieces []emit.Piece) {
	type masked struct {
		from    emit.Span
		without emit.Features
	}
	var seen []emit.Span
	seenMasked := map[masked]bool{}
	for i, p := range pieces {
		if !p.Copied {
			continue
		}
		again := false
		if p.Without != 0 {
			again = seenMasked[masked{p.From, p.Without}]
			seenMasked[masked{p.From, p.Without}] = true
		} else {
			for _, s := range seen {
				if p.From.Pos < s.End && s.Pos < p.From.End {
					again = true
					break
				}
			}
			if !again {
				seen = append(seen, p.From)
			}
		}
		if again {
			pieces[i] = p.Lacking(emit.AllFeatures)
		}
	}
}

// checks is pass 0: errors reported against what the author wrote.
func checks(c *passContext) []emit.Edit {
	var errs []syntax.Error
	for _, check := range []func(*rtsx.SourceFile) []syntax.Error{
		syntax.Check, syntax.CheckFlowAsValue, syntax.CheckSlotTags, syntax.CheckSegments,
	} {
		errs = append(errs, check(c.file)...)
	}
	for _, e := range errs {
		sev := Error
		if e.Warning {
			sev = Warning
		}
		c.report(emit.Span{Pos: e.Pos, End: e.End}, sev, e.Code, e.Message)
	}
	c.checkSegmentFiles()
	c.checkAmbiguousModule()
	return nil
}
