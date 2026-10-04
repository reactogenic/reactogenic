package report

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// rewrite rewords a TS error on synthesized code in the author's terms
// (diagnostics.md, *Rewrites*; RGP1-073). It matches by what was synthesized
// at the error's source span — the transpiler's notes — and by the TS code
// and structured message arguments, never by message text. ok is false when
// no rule matches: the error keeps TS's own message.
func rewrite(d *rtsx.Diagnostic, out transpiler.Output, src string, at emit.Span) (code, message string, related []Report, ok bool) {
	args := d.MessageArgs()
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	// A required slot not filled: TS2741 on the component tag, naming a `$` prop.
	if d.Code() == 2741 && strings.HasPrefix(arg(0), "$") {
		return "missing-slot", fmt.Sprintf("`%s` requires `%s`", strings.TrimSpace(src[at.Pos:at.End]), arg(0)), nil, true
	}
	note := innermostNote(out.Notes, at)
	if note == nil {
		return "", "", nil, false
	}
	switch {
	case note.Kind == "slot-prop" && chainHas(d, 2339, note.Name):
		return "undeclared-slot", fmt.Sprintf("`%s` is not declared in `%s`", note.Name, note.Detail), nil, true
	case note.Kind == "slot-prop" && d.Code() == 2322:
		return "slot-type", fmt.Sprintf("`%s` does not match its declaration in `%s`: %s", note.Name, note.Detail, flatten(d)), nil, true
	case note.Kind == "slot-body" && d.Code() == 2353:
		return "content-not-allowed", fmt.Sprintf("`%s` takes no body", note.Name), nil, true
	case note.Kind == "slot-body" && d.Code() == 2322 && note.Detail == "":
		return "params-required", fmt.Sprintf("`%s` requires params: its body is a function of the attachment's args", note.Name), nil, true
	case note.Kind == "slot-params" && d.Code() == 7031:
		return "no-values", fmt.Sprintf("`%s` provides no values", note.Name), nil, true
	case note.Kind == "no-match" && d.Code() == 2345:
		return "switch-missing-case", "Missing " + cases(arg(0)), nil, true
	case note.Kind == "segment" && d.Code() == 2604:
		return "segment-not-component", fmt.Sprintf("The segment `%s` has no default component", note.Name), nil, true
	case note.Kind == "segment" && d.Code() == 2741:
		return "segment-props", fmt.Sprintf("A segment takes no props: `%s` requires `%s`", note.Name, arg(0)), nil, true
	case note.Kind == "segment" && (chainHas(d, 2339, "id") || chainHas(d, 2339, "children")):
		return "segment-root-props", fmt.Sprintf("`%s` must accept `id` and `children` to be a segment root", note.Detail), nil, true
	case note.Kind == "slot-args" && d.Code() == 2741:
		return "slot-args-missing", fmt.Sprintf("`%s` needs `&%s`", note.Name, arg(0)), nil, true
	case note.Kind == "slot-args" && d.Code() == 2345:
		return "slot-list", fmt.Sprintf("`%s` is a list; a slot is one value", note.Name), nil, true
	case note.Kind == "slot-key" && d.Code() == 2353:
		return "slot-key-no-args", fmt.Sprintf("`%s` has no args to key by: its body is not a function", note.Name), nil, true
	case note.Kind == "slot-entry-key" && d.Code() == 2464:
		return "slot-key-inline", fmt.Sprintf("A key of `%s` is a string or a number; a key function is written inline: `key={(args) => …}`", note.Name), nil, true
	case note.Kind == "slot-arg" && d.Code() == 2322 && arg(1) == "never":
		// Args of a slot whose body is not a function are `never`
		// (NoArgs); an arg of the wrong type keeps TS's message.
		return "slot-no-args", fmt.Sprintf("`%s` takes no args: its body is not a function", note.Name), nil, true
	case note.Kind == "shorthand-true" && d.Code() == 2322:
		// TS's message is accurate; add the likely cause.
		hint := Report{Severity: Message, Message: fmt.Sprintf("No `%s` in scope: a bare attribute is `true`. Did you mean `%s={%s}`?", note.Name, note.Name, note.Name)}
		return fmt.Sprintf("TS%d", d.Code()), flatten(d), []Report{hint}, true
	}
	return "", "", nil, false
}

// innermostNote is the smallest note whose span holds at.
func innermostNote(notes []transpiler.Note, at emit.Span) *transpiler.Note {
	var best *transpiler.Note
	for i := range notes {
		n := &notes[i]
		if n.Span.Pos <= at.Pos && at.End <= n.Span.End && (best == nil || n.Span.Len() < best.Span.Len()) {
			best = n
		}
	}
	return best
}

// chainHas: d or a message in its chain has code with arg as its first
// argument.
func chainHas(d *rtsx.Diagnostic, code int32, arg string) bool {
	if d.Code() == code && len(d.MessageArgs()) > 0 && d.MessageArgs()[0] == arg {
		return true
	}
	for _, next := range d.MessageChain() {
		if chainHas(next, code, arg) {
			return true
		}
	}
	return false
}

// cases prints the leftover type of an exhaustive switch as its missing
// cases: `"b" | "c"` → "`"b"`, `"c"`".
func cases(leftover string) string {
	parts := strings.Split(leftover, " | ")
	for i, p := range parts {
		parts[i] = "`" + p + "`"
	}
	return strings.Join(parts, ", ")
}

// renameGenerated replaces generated names in a message with what the author
// wrote (RGP1-072).
func renameGenerated(message string, generated map[string]string) string {
	for local, written := range generated {
		message = regexp.MustCompile(`\b`+regexp.QuoteMeta(local)+`\b`).ReplaceAllLiteralString(message, written)
	}
	return message
}
