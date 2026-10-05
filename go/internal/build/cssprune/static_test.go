package cssprune

import (
	"fmt"
	"strings"
)

// The bridge to cascadia, the independent matcher of the soundness test.
//
// cascadia matches a page as it is: it knows no runtime state, no :is(), and
// its :has() is "has a descendant matching". static rewrites a selector of
// ours into selectors it can run that match **at least** what the original
// may ever match: runtime conditions are taken out, :is() is expanded, and a
// condition cascadia has no form for is left out. So if cascadia finds no
// element for static(s), no element can match s.

// static returns the selectors; approx reports that something had to be left
// out or widened beyond runtime state.
func static(s *selector) (alts []string, approx bool) {
	sels, exact := expand(s)
	approx = !exact
	for _, alt := range sels {
		for _, lin := range linear(alt, &approx) {
			text := widen(lin, &approx)
			if lin.comb[0] != 0 { // relative, in @scope: to any element
				text = "* " + string(lin.comb[0]) + " " + text
			}
			alts = append(alts, text)
		}
	}
	return alts, approx
}

// linear rewrites a :has() on the subject as what it is for the question
// asked here — does anything match? `P C:has(> S)` matches some element iff
// `P C > S` does. It is exact for every relative selector, which cascadia's
// own :has() is not.
func linear(s *selector, approx *bool) []*selector {
	last := &s.parts[len(s.parts)-1]
	at := -1
	for i, sm := range last.simples {
		if sm.kind == sHas {
			if at >= 0 {
				return []*selector{s} // two: the same element must have both
			}
			at = i
		}
	}
	if at < 0 {
		return []*selector{s}
	}
	c := compound{tag: last.tag}
	c.simples = append(append(c.simples, last.simples[:at]...), last.simples[at+1:]...)
	var out []*selector
	for _, arg := range last.simples[at].args {
		alts, exact := expand(arg)
		*approx = *approx || !exact
		for _, alt := range alts {
			n := &selector{}
			n.parts = append(append(append(n.parts, s.parts[:len(s.parts)-1]...), c), alt.parts...)
			n.comb = append(append(n.comb, s.comb...), alt.comb...)
			if alt.comb[0] == 0 {
				n.comb[len(s.comb)] = ' '
			}
			out = append(out, n)
		}
	}
	return out
}

// expand removes :is() from the compounds of s: one selector per choice of
// argument. A complex argument that does not stand first keeps its last
// compound only, which widens (exact is false).
func expand(s *selector) (out []*selector, exact bool) {
	for pi := range s.parts {
		part := &s.parts[pi]
		for si := range part.simples {
			if part.simples[si].kind != sIs {
				continue
			}
			exact = true
			for _, arg := range part.simples[si].args {
				n := len(arg.parts) - 1
				last := arg.parts[n]
				c := compound{tag: part.tag}
				c.simples = append(c.simples, part.simples[:si]...)
				c.simples = append(c.simples, part.simples[si+1:]...)
				c.simples = append(c.simples, last.simples...)
				if last.tag != "" && last.tag != "*" {
					if c.tag != "" && c.tag != "*" && c.tag != last.tag {
						continue // two different tags: nothing matches
					}
					c.tag = last.tag
				}
				sel := &selector{}
				switch {
				case pi == 0 && s.comb[0] == 0:
					sel.parts = append(append(sel.parts, arg.parts[:n]...), c)
					sel.comb = append(sel.comb, arg.comb...)
				default:
					exact = exact && n == 0
					sel.parts = append(append(sel.parts, s.parts[:pi]...), c)
					sel.comb = append(sel.comb, s.comb[:pi+1]...)
				}
				sel.parts = append(sel.parts, s.parts[pi+1:]...)
				sel.comb = append(sel.comb, s.comb[pi+1:]...)
				more, e := expand(sel)
				out, exact = append(out, more...), exact && e && arg.known
			}
			return out, exact
		}
	}
	return []*selector{s}, true
}

// hasMaybe: s holds runtime state somewhere.
func hasMaybe(s *selector) bool {
	for _, c := range s.parts {
		for _, sm := range c.simples {
			switch sm.kind {
			case sMaybe:
				return true
			case sAttr:
				if dynamic(sm.name) {
					return true
				}
			case sIs, sNot, sHas:
				for _, a := range sm.args {
					if hasMaybe(a) || sm.kind == sIs && !a.known {
						return true
					}
				}
			}
		}
	}
	return false
}

// notOfMaybe: s holds a :not() with runtime state in an argument, which
// widen leaves out whole.
func notOfMaybe(s *selector) bool {
	for _, c := range s.parts {
		for _, sm := range c.simples {
			for _, a := range sm.args {
				if sm.kind == sNot && hasMaybe(a) || notOfMaybe(a) {
					return true
				}
			}
		}
	}
	return false
}

// widen prints s without its runtime state: a superset of what it matches.
func widen(s *selector, approx *bool) string {
	var b strings.Builder
	for i := range s.parts {
		if i > 0 {
			b.WriteString(" " + string(s.comb[i]) + " ")
		}
		c := &s.parts[i]
		start := b.Len()
		if c.tag != "" && c.tag != "*" {
			b.WriteString(escIdent(c.tag))
		}
		for j := range c.simples {
			sm := &c.simples[j]
			switch sm.kind {
			case sClass:
				b.WriteString("." + escIdent(sm.name))
			case sID:
				b.WriteString("#" + escIdent(sm.name))
			case sAttr:
				switch {
				case dynamic(sm.name):
				case sm.op != 0 && sm.op != '=' && sm.op != '|' && sm.value == "", sm.op == '~' && strings.ContainsAny(sm.value, " \t\n\r\f"):
					// Matches nothing (Selectors, 6.2); cascadia matches everything.
					b.WriteString(":not(*)")
				default:
					b.WriteString(attrText(sm))
				}
			case sRoot:
				b.WriteString(":root")
			case sNot:
				// not(A or B): an argument with runtime state, or one that
				// cannot be said exactly, is left out — which widens.
				var args []string
				for _, a := range sm.args {
					if hasMaybe(a) {
						continue
					}
					texts, ok := exactly(a)
					if !ok {
						*approx = true
						continue
					}
					args = append(args, texts...)
				}
				if len(args) > 0 {
					b.WriteString(":not(" + strings.Join(args, ", ") + ")")
				}
			case sHas:
				var desc, child []string
				none, other := true, false
				for _, a := range sm.args {
					alts, exact := expand(a)
					*approx = *approx || !exact
					for _, alt := range alts {
						none = false
						text := widen(alt, approx)
						switch {
						case alt.comb[0] == '+' || alt.comb[0] == '~':
							other = true
						case alt.comb[0] == '>' && len(alt.parts) == 1:
							child = append(child, text)
						default:
							// cascadia's :has() does not anchor a complex
							// argument at the element: a superset.
							*approx = *approx || len(alt.parts) > 1
							desc = append(desc, text)
						}
					}
				}
				switch {
				case none:
					b.WriteString(":not(*)")
				case other || len(desc) > 0 && len(child) > 0:
					*approx = true
				case len(desc) > 0:
					b.WriteString(":has(" + strings.Join(desc, ", ") + ")")
				default:
					b.WriteString(":haschild(" + strings.Join(child, ", ") + ")")
				}
			}
		}
		if b.Len() == start {
			b.WriteString("*")
		}
	}
	return b.String()
}

// exactly prints an argument of :not() that holds no runtime state, as
// selectors that match exactly what it matches. ok is false if cascadia has
// no form for it.
func exactly(s *selector) (texts []string, ok bool) {
	alts, exact := expand(s)
	if !exact {
		return nil, false
	}
	for _, alt := range alts {
		for i := range alt.parts {
			for _, sm := range alt.parts[i].simples {
				switch sm.kind {
				case sNot:
					for _, a := range sm.args {
						if _, ok := exactly(a); !ok {
							return nil, false
						}
					}
				case sHas:
					kinds := map[bool]bool{}
					for _, a := range sm.args {
						inner, exact := expand(a)
						for _, in := range inner {
							if !exact || len(in.parts) != 1 || in.comb[0] == '+' || in.comb[0] == '~' {
								return nil, false
							}
							if _, ok := exactly(&selector{parts: in.parts, comb: []byte{0}}); !ok {
								return nil, false
							}
							kinds[in.comb[0] == '>'] = true
						}
					}
					if len(kinds) > 1 {
						return nil, false
					}
				}
			}
		}
		approx := false
		text := widen(alt, &approx)
		if approx {
			return nil, false
		}
		texts = append(texts, text)
	}
	return texts, true
}

func attrText(sm *simple) string {
	text := "[" + escIdent(sm.name)
	if sm.op != 0 {
		if sm.op != '=' {
			text += string(sm.op)
		}
		text += `="` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\a `).Replace(sm.value) + `"`
		// cascadia compares values case-sensitively unless told otherwise;
		// HTML does not, for some attributes (match.go).
		if sm.flag == 'i' || sm.flag == 0 && caseInsensitiveValue[sm.name] {
			text += " i"
		}
	}
	return text + "]"
}

func escIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 0x80, r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', i > 0 && (r == '-' || r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, `\%x `, r)
		}
	}
	return b.String()
}
