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
	// Stopped, in tolerant mode: why the passes ended early. TSX is then the
	// last good text — up to the source itself — and still holds unlowered
	// constructs, so TS's diagnostics for it mean nothing.
	Stopped string
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
	)
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
		if parseErrors := file.Diagnostics(); len(parseErrors) > 0 {
			// Emitted text that does not parse is our bug only when the
			// source did: a broken source may stay broken through a pass.
			if p.n > 0 && !broken {
				return Output{}, fmt.Errorf("transpiler: pass %d (%s) produced code that does not parse: %s", p.n-1, passes[p.n-1].name, rtsx.Message(parseErrors[0]))
			}
			if p.n == 0 {
				// Syntax errors are the source parse's, only.
				for _, d := range parseErrors {
					out.add(in.Entry, src, emit.Span{Pos: rtsx.SkipTrivia(src, d.Pos()), End: d.End()}, Error, fmt.Sprintf("TS%d", d.Code()), rtsx.Message(d))
				}
				if !in.Tolerant {
					return out, nil
				}
				broken = true
			}
		}
		if names == nil {
			names = identifiers(file)
		}
		if p.run == nil {
			continue
		}
		rtsx.Bind(file)
		c := &passContext{file: file, text: text, names: names, imports: map[string]bool{}, entry: in.Entry, readFile: in.readFile,
			report: func(s emit.Span, sev Severity, code, msg string) {
				if broken && underParseError(file, s) {
					return // a half-typed construct: its errors are the parser's
				}
				out.add(in.Entry, src, toSource.Source(s), sev, code, msg)
			},
			note: func(s emit.Span, kind, name, detail string) {
				out.Notes = append(out.Notes, Note{Span: toSource.Source(s), Kind: kind, Name: name, Detail: detail})
			},
			noteTag: func(s, tag emit.Span, kind, name string) {
				out.Notes = append(out.Notes, Note{Span: toSource.Source(s), Kind: kind, Name: name, Tag: toSource.Source(tag)})
			},
			shorthandSite: func(name emit.Span, kind string) {
				out.Shorthands = append(out.Shorthands, Shorthand{Name: toSource.Source(name), Kind: kind})
			},
			slotGroup: func(name string, owner emit.Span, tags []emit.Span) {
				g := SlotGroup{Name: name, Owner: toSource.Source(owner)}
				for _, t := range tags {
					g.Tags = append(g.Tags, toSource.Source(t))
				}
				out.SlotGroups = append(out.SlotGroups, g)
			},
			generated: func(local, written string) {
				if out.Generated == nil {
					out.Generated = map[string]string{}
				}
				out.Generated[local] = written
			}}
		var edits []emit.Edit
		if broken {
			// On a recovered tree a pass may meet shapes it never sees in
			// valid code; whatever happens, the file keeps a virtual text.
			var panicked any
			func() {
				defer func() { panicked = recover() }()
				edits = p.run(c)
			}()
			if panicked != nil {
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
		text, toSource = next, m.Then(toSource)
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
// The file must be bound: the binder sets the flag.
func underParseError(file *rtsx.SourceFile, s emit.Span) bool {
	n := rtsx.TokenAt(file, s.Pos)
	for n != nil && (n.End() < s.End || rtsx.TokenStart(file, n) > s.Pos) {
		n = n.Parent
	}
	for node := n; n != nil; n = n.Parent {
		if rtsx.HasParseError(n) {
			return true
		}
		if n != node {
			switch n.Kind {
			case rtsx.KindJsxElement, rtsx.KindJsxSelfClosingElement, rtsx.KindJsxFragment:
				return false
			}
		}
	}
	return false
}

// oneCopyAnswers: text copied several times into one construct (an attachment
// emitted in both branches of its ternary, a Switch subject per case) is
// checked in each place but answers the editor from the first only (ide.md,
// *Span map*). Copies with a mask of their own — the two copies of a
// shorthand — are different symbols and both answer.
func oneCopyAnswers(pieces []emit.Piece) {
	var seen []emit.Span
	for i, p := range pieces {
		if !p.Copied || p.Without != 0 {
			continue
		}
		again := false
		for _, s := range seen {
			if p.From.Pos < s.End && s.Pos < p.From.End {
				again = true
				break
			}
		}
		if again {
			pieces[i] = p.Lacking(emit.AllFeatures)
		} else {
			seen = append(seen, p.From)
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
