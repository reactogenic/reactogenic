package lsp

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// renameEdits writes a rename's occurrences in .rtsx files back to their
// sources (ide.md, *Rename*): correct, or refused as a whole — never a
// part. TypeScript found the occurrences in the virtual texts; each is
// given here as it stands there, and the edit is made by what the source has
// at that place:
//
//   - copied text: the edit, at the source position;
//   - a bare name that stands for `name={name}` — an attribute, `&name`,
//     `&&name`: one token, two symbols. The one that is renamed is written
//     out (`size={dim}`, `scale={size}`);
//   - a tag name: the same edit in the element's other tag, and for a slot
//     in every tag of its group — TypeScript has one prop for them all;
//   - a reference that is in no virtual text (the children a segment root
//     overwrites): renamed with its declaration, which the source's own
//     scopes find;
//   - anything that has no place in the source refuses the rename, and the
//     error names it.
//
// Then the edits are tried: applied to each file and transpiled again.
func renameEdits(_ context.Context, request server.RenameRequest) ([]server.RenameEdit, error) {
	byFile := map[string][]server.RenameOccurrence{}
	var order []string
	for _, occurrence := range request.Occurrences {
		name := occurrence.File.OriginalFileName()
		if _, seen := byFile[name]; !seen {
			order = append(order, name)
		}
		byFile[name] = append(byFile[name], occurrence)
	}
	sort.Strings(order)

	// A file whose virtual text is not its lowered source may hold
	// occurrences that nothing found: code left out, a statement lowered
	// from a recovered tree.
	word := wordOf(request.Name)
	for _, file := range request.Files {
		mapped, ok := mapper.Of(file)
		if !ok {
			continue
		}
		if _, touched := byFile[file.OriginalFileName()]; touched || word != nil && word.MatchString(file.OriginalText()) {
			if why := unusable(mapped); why != "" {
				return nil, fmt.Errorf("Rename refused: %s %s, so `%s` in it cannot be found for certain. Fix the file first.", path.Base(file.OriginalFileName()), why, request.Name)
			}
		}
	}

	var out []server.RenameEdit
	for _, name := range order {
		occurrences := byFile[name]
		mapped, ok := mapper.Of(occurrences[0].File)
		if !ok {
			return nil, fmt.Errorf("Rename refused: %s has no transform", path.Base(name))
		}
		if why := unusable(mapped); why != "" {
			return nil, fmt.Errorf("Rename refused: %s %s, so `%s` in it cannot be found for certain. Fix the file first.", path.Base(name), why, request.Name)
		}
		r := &renaming{request: request, fileName: name, src: analyse(occurrences[0].File.OriginalText(), mapped.Output), texts: map[emit.Span]string{}, sites: map[emit.Span]*site{}}
		for _, occurrence := range occurrences {
			if err := r.occurrence(occurrence); err != nil {
				return nil, err
			}
		}
		if err := r.lowered(); err != nil {
			return nil, err
		}
		edits, err := r.edits()
		if err != nil {
			return nil, err
		}
		if !request.Prepare {
			if err := r.try(edits); err != nil {
				return nil, err
			}
		}
		out = append(out, edits...)
	}
	return out, nil
}

// unusable says why a file's virtual text cannot be trusted to hold every
// occurrence; "" when it can.
func unusable(file *mapper.File) string {
	switch {
	case file.Err != nil:
		return "could not be transpiled"
	case file.Stopped:
		return "has an error that keeps part of it out of the transpiled code"
	}
	for _, d := range file.Diagnostics {
		if strings.HasPrefix(d.Code, "TS") {
			return "has a syntax error"
		}
	}
	return ""
}

// wordOf matches name as a whole word; nil for a name that is not one.
func wordOf(name string) *regexp.Regexp {
	if name == "" || !regexp.MustCompile(`^[\w$]+$`).MatchString(name) {
		return nil
	}
	return regexp.MustCompile(`(^|[^\w$])` + regexp.QuoteMeta(name) + `($|[^\w$])`)
}

// renaming is the rename of one .rtsx file.
type renaming struct {
	request  server.RenameRequest
	fileName string
	src      *sourceTree
	// texts are the new texts by source span; found, the spans TypeScript
	// reported — the declarations among them are what lowered() looks for.
	texts map[emit.Span]string
	found []emit.Span
	// sites are the bare names renamed, by name span.
	sites map[emit.Span]*site
}

// site is a bare name — `size`, `&size`, `&&size` — and which of its two
// meanings is renamed.
type site struct {
	attr        attribute
	name, value *string // the new text of the prop or arg; of the binding
}

func (r *renaming) refuse(pos int, format string, args ...any) error {
	line, col := emit.LineCol(r.src.text, pos)
	return fmt.Errorf("Rename refused at %s:%d:%d: %s", path.Base(r.fileName), line, col, fmt.Sprintf(format, args...))
}

// set records the new text of a source span; a span cannot have two.
func (r *renaming) set(at emit.Span, text string) error {
	if old, ok := r.texts[at]; ok && old != text {
		return r.refuse(at.Pos, "`%s` would become both `%s` and `%s`.", r.src.slice(at), old, text)
	}
	r.texts[at] = text
	return nil
}

// argProp refuses the rename of the arg or the prop of `&&name`: the one
// token is both, and the rename is of one.
const argProp = "`&&%s` names an arg and a prop at once; only the binding behind it can be renamed. Write the arg and the prop apart first."

func (r *renaming) occurrence(o server.RenameOccurrence) error {
	at := emit.Span{Pos: o.OriginalPos, End: o.OriginalEnd}
	if !o.Exact {
		// Generated text: its origin is the construct that produced it.
		// The prop of `&&name` is such: its name is written for the attribute.
		for _, a := range r.src.attrs {
			if a.arg == syntax.ArgProp && rtsx.TokenStart(r.src.file, a.node) == at.Pos {
				return r.refuse(a.name.Pos, argProp, r.src.slice(a.name))
			}
		}
		construct := r.src.slice(emit.Span{Pos: at.Pos, End: min(at.End, len(r.src.text))})
		return r.refuse(at.Pos, "`%s` is also written by the transform, for `%s`; that has no name in the source to rename.", r.request.Name, firstLine(construct))
	}
	r.found = append(r.found, at)

	// The name of an attribute.
	for _, a := range r.src.attrs {
		if a.name != at {
			continue
		}
		renamesName := o.Node == nil || !rtsx.IsValueReference(o.Node)
		switch {
		case a.arg == syntax.ArgProp && renamesName:
			return r.refuse(at.Pos, argProp, r.src.slice(at))
		case !a.bare:
			return r.set(at, o.NewText)
		}
		s := r.sites[at]
		if s == nil {
			s = &site{attr: a}
			r.sites[at] = s
		}
		text := o.NewText
		if renamesName {
			s.name = &text
		} else {
			s.value = &text
		}
		return nil
	}

	// A tag name: a `$` makes a slot element of it, and nothing else does.
	if i := r.src.tagAt(at); i >= 0 {
		tag := r.src.tags[i]
		old := r.src.slice(tag.span)
		renamed := old[:at.Pos-tag.span.Pos] + o.NewText + old[at.End-tag.span.Pos:]
		if was, is := strings.HasPrefix(old, "$"), strings.HasPrefix(renamed, "$"); was && !is {
			return r.refuse(at.Pos, "`<%s>` is a slot element; the new name must start with `$` too.", old)
		} else if is && !was {
			return r.refuse(at.Pos, "`<%s>` would become a slot element: a tag that starts with `$` is a slot.", old)
		}
		for _, twin := range r.src.twins(at) {
			if err := r.set(twin, o.NewText); err != nil {
				return err
			}
		}
		return r.set(at, o.NewText)
	}

	// The `$X` of `slot={$X}`: the `$` is what attaches.
	for _, ref := range r.src.slotRefs() {
		if ref == at && strings.HasPrefix(r.src.slice(at), "$") != strings.HasPrefix(o.NewText, "$") {
			if strings.HasPrefix(o.NewText, "$") {
				return r.refuse(at.Pos, "`slot={%s}` would attach a slot: a name that starts with `$` there is one.", o.NewText)
			}
			return r.refuse(at.Pos, "`slot={%s}` attaches a slot; the new name must start with `$` too.", r.src.slice(at))
		}
	}
	return r.set(at, o.NewText)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i] + "…"
	}
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}

// slotRefs are the last names of the references in `slot={…}`: `$X`, the
// `$X` of `props.$X` — and of a plain `slot={name}`, which a `$` would turn
// into one.
func (s *sourceTree) slotRefs() []emit.Span {
	var refs []emit.Span
	for _, a := range s.attrs {
		if a.arg != syntax.NotArg || a.bare || s.slice(a.name) != "slot" {
			continue
		}
		init := a.node.Initializer()
		if init.Kind != rtsx.KindJsxExpression || init.Expression() == nil {
			continue
		}
		switch e := init.Expression(); e.Kind {
		case rtsx.KindIdentifier:
			refs = append(refs, emit.Span{Pos: rtsx.TokenStart(s.file, e), End: e.End()})
		case rtsx.KindPropertyAccessExpression:
			refs = append(refs, emit.Span{Pos: rtsx.TokenStart(s.file, e.Name()), End: e.Name().End()})
		}
	}
	return refs
}

// lowered renames the references that TypeScript cannot see: a name that
// is in no virtual text — an expression in the children that a segment root
// overwrites, the subject of a `Switch` without a case — and that the
// source's own scopes resolve to a declaration this rename renames.
//
// What the scopes cannot decide refuses the rename: a member, a type or a
// key of that name in such code may or may not be the renamed one. (The
// names the passes read are not in question: a tag — renamed with its twin —
// and the name of an attribute.)
func (r *renaming) lowered() error {
	if r.request.Name == "" || len(r.found) == 0 {
		return nil
	}
	declared := map[emit.Span]bool{}
	for _, at := range r.found {
		declared[at] = true
	}
	span := func(n *rtsx.Node) emit.Span { return emit.Span{Pos: rtsx.TokenStart(r.src.file, n), End: n.End()} }
	var refused error
	var visit func(n *rtsx.Node) bool
	visit = func(n *rtsx.Node) bool {
		if n.Kind != rtsx.KindIdentifier || rtsx.NodeText(n) != r.request.Name {
			return n.ForEachChild(visit)
		}
		at := span(n)
		if _, done := r.texts[at]; done || r.src.copied(at) || r.src.tagAt(at) >= 0 && !rtsx.IsValueReference(n) {
			return false
		}
		if n.Parent != nil && (n.Parent.Kind == rtsx.KindJsxAttribute || n.Parent.Kind == rtsx.KindImportSpecifier) && n.Parent.Name() == n {
			return false // an attribute the passes read; the import of a name they lower away
		}
		if !rtsx.IsValueReference(n) {
			refused = r.refuse(at.Pos, "`%s` here is in code that the transform leaves out, where it cannot be told from the name being renamed. Remove that code first.", r.request.Name)
			return true
		}
		if symbol := rtsx.ResolveValue(n, r.request.Name); symbol != nil {
			for _, declaration := range symbol.Declarations {
				if name := declaration.Name(); name != nil && declared[span(name)] {
					r.texts[at] = r.request.NewName
					for _, twin := range r.src.twins(at) {
						r.texts[twin] = r.request.NewName
					}
					break
				}
			}
		}
		return false
	}
	r.src.file.AsNode().ForEachChild(visit)
	return refused
}

// edits are the file's edits, in source order.
func (r *renaming) edits() ([]server.RenameEdit, error) {
	for at, s := range r.sites {
		old := r.src.slice(at)
		bound := s.attr.arg != syntax.NotArg // an arg is its binding, always
		for _, shorthand := range r.src.out.Shorthands {
			bound = bound || shorthand.Name == at
		}
		var text string
		switch {
		case s.name != nil && s.value != nil:
			text = *s.value // both: the token stays one name
		case s.value != nil:
			text = old + "={" + *s.value + "}"
		case bound:
			text = *s.name + "={" + old + "}"
		default:
			text = *s.name // `true`, under its new name
		}
		if err := r.set(at, text); err != nil {
			return nil, err
		}
	}
	spans := make([]emit.Span, 0, len(r.texts))
	for at := range r.texts {
		spans = append(spans, at)
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Pos < spans[j].Pos })
	edits := make([]server.RenameEdit, 0, len(spans))
	for i, at := range spans {
		if i > 0 && at.Pos < spans[i-1].End {
			return nil, r.refuse(at.Pos, "two edits overlap at `%s`.", r.src.slice(at))
		}
		edits = append(edits, server.RenameEdit{FileName: r.fileName, Pos: at.Pos, End: at.End, NewText: r.texts[at]})
	}
	return edits, nil
}

// try is the post-check (ide.md, *Rename*): the edits are applied to the
// source in memory and the result transpiled. The rename is refused when
// the file gains a syntax error, when a bare attribute flips between its
// binding and `true` — the new name captured it — or when the transpiler
// has a new error for it.
func (r *renaming) try(edits []server.RenameEdit) error {
	var b strings.Builder
	last := 0
	for _, e := range edits {
		b.WriteString(r.src.text[last:e.Pos])
		b.WriteString(e.NewText)
		last = e.End
	}
	b.WriteString(r.src.text[last:])
	after := b.String()
	// before maps an offset of the new text to the old one's; inside an
	// edit's text, to the edit's start.
	before := func(pos int) int {
		shift := 0
		for _, e := range edits {
			start := e.Pos + shift
			switch {
			case pos < start:
				return pos - shift
			case pos < start+len(e.NewText):
				return e.Pos
			}
			shift += len(e.NewText) - (e.End - e.Pos)
		}
		return pos - shift
	}

	none := func(string) bool { return false } // the same siblings for both: none
	_, was := mapper.Transform(r.fileName, r.src.text, none)
	_, is := mapper.Transform(r.fileName, after, none)
	if is.Err != nil && was.Err == nil {
		return fmt.Errorf("Rename refused: %s would no longer transpile (%v).", path.Base(r.fileName), is.Err)
	}
	count := func(file *mapper.File) map[string]int {
		codes := map[string]int{}
		for _, d := range file.Diagnostics {
			if d.Severity == transpiler.Error {
				codes[d.Code]++
			}
		}
		return codes
	}
	had := count(was)
	for _, d := range is.Diagnostics {
		if d.Severity != transpiler.Error {
			continue
		}
		if had[d.Code]--; had[d.Code] < 0 {
			line, col := emit.LineCol(r.src.text, before(d.Span.Pos))
			what := "the file would get an error"
			if strings.HasPrefix(d.Code, "TS") {
				what = "the file would no longer parse"
			}
			return fmt.Errorf("Rename refused at %s:%d:%d: %s (%s: %s).", path.Base(r.fileName), line, col, what, d.Code, d.Message)
		}
	}
	bound := func(file *mapper.File) map[int]bool {
		at := map[int]bool{}
		for _, note := range file.Notes {
			if note.Kind == "shorthand-true" {
				at[note.Span.Pos] = false
			}
		}
		for _, shorthand := range file.Shorthands {
			if shorthand.Kind == "attr" {
				at[shorthand.Name.Pos] = true
			}
		}
		return at
	}
	old := bound(was)
	for pos, now := range bound(is) {
		if then, bare := old[before(pos)]; bare && then != now {
			line, col := emit.LineCol(r.src.text, before(pos))
			if now {
				return fmt.Errorf("Rename refused at %s:%d:%d: the new name would capture a bare attribute there — it means `true` now, and would mean `%s={%s}`.", path.Base(r.fileName), line, col, r.request.NewName, r.request.NewName)
			}
			return fmt.Errorf("Rename refused at %s:%d:%d: a bare attribute there would lose its binding and mean `true`.", path.Base(r.fileName), line, col)
		}
	}
	return nil
}
