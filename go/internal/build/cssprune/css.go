package cssprune

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// The reader: the flat CSS esbuild's public API prints — rules, at-rules with
// blocks and with statements — split into a tree of source slices. Nothing is
// re-printed from tokens: what is kept is kept as its own bytes.
//
// It is strict where esbuild's output is regular: an unbalanced bracket, an
// unterminated string or comment, a rule without a block are errors, not
// something to recover from. A sheet read wrongly would be pruned wrongly.

type kind uint8

const (
	kComment   kind = iota // /* … */ between rules
	kStyle                 // selectors { declarations }
	kGroup                 // @media, @supports, @container, @scope, @starting-style: pruned inside
	kLayer                 // @layer name? { … }: pruned inside, and it orders the cascade
	kKeyframes             // @keyframes name { … }
	kTry                   // @position-try --name { … }
	kAt                    // any other at-rule, block or statement: kept as it is
	kRaw                   // a declaration between rules (@scope may hold them): kept as it is
)

// groups are the at-rules that hold rules and nothing else of their own
// (builder.md, CSS: "pruned inside; dropped when empty").
var groups = map[string]bool{"media": true, "supports": true, "container": true, "scope": true, "starting-style": true}

type rule struct {
	kind    kind
	raw     string  // the rule's source, whole
	name    string  // an at-rule's, lower-cased, without the @
	head    string  // the source before the block: ".a,.b", "@media (width>=50rem)"
	prelude string  // an at-rule's head after the name
	body    string  // the source between the braces
	rules   []*rule // kGroup, kLayer
	decls   []*decl // kStyle
	// opaque: a style rule whose block nests rules. esbuild lowers nesting
	// everywhere but inside @scope; such a rule is kept whole, because `&`
	// takes its specificity from the whole selector list.
	opaque bool
	block  bool // it has a block: `@layer a{}`, not `@layer a;`

	sels []string // kStyle: the selectors kept, as written
	dead bool     // kStyle: no selector may match; kKeyframes: no animation names it; kTry: nothing names it

	// What resolve decided, anew in every round of the fixed point.
	live      bool // it is printed
	statement bool // kLayer: emptied, printed as `@layer name;`
}

type decl struct {
	text   string // as written, trimmed
	name   string // lower-cased; a custom property's as written; "" if it is not `name: value`
	value  string
	custom bool // --x
	dead   bool // a custom property nothing reads
}

// maxDepth bounds bracket nesting, so a hostile input cannot exhaust the stack.
const maxDepth = 128

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}
func isNameChar(c byte) bool { return isNameStart(c) || c >= '0' && c <= '9' || c == '-' }

// validEscape: a backslash that escapes something (not a newline, not the end).
func validEscape(s string, i int) bool {
	return i+1 < len(s) && s[i] == '\\' && s[i+1] != '\n' && s[i+1] != '\r' && s[i+1] != '\f'
}

// identStart reports whether an identifier starts at s[i].
func identStart(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	if s[i] == '-' {
		i++
		return i < len(s) && (s[i] == '-' || isNameStart(s[i]) || validEscape(s, i))
	}
	return isNameStart(s[i]) || validEscape(s, i)
}

// readEscape decodes the escape at s[i] (a valid one) and returns what follows.
func readEscape(s string, i int) (rune, int) {
	i++
	if !isHex(s[i]) {
		r, n := utf8.DecodeRuneInString(s[i:])
		return r, i + n
	}
	var v rune
	j := i
	for ; j < len(s) && j < i+6 && isHex(s[j]); j++ {
		c := s[j]
		switch {
		case c <= '9':
			v = v<<4 | rune(c-'0')
		case c >= 'a':
			v = v<<4 | rune(c-'a'+10)
		default:
			v = v<<4 | rune(c-'A'+10)
		}
	}
	if j < len(s) && isSpace(s[j]) { // one whitespace ends a hex escape
		if s[j] == '\r' && j+1 < len(s) && s[j+1] == '\n' {
			j++
		}
		j++
	}
	if v == 0 || v > utf8.MaxRune || v >= 0xD800 && v <= 0xDFFF {
		v = utf8.RuneError
	}
	return v, j
}

// readName reads name characters and escapes from s[i], decoded.
func readName(s string, i int) (string, int) {
	j := i
	for j < len(s) && isNameChar(s[j]) {
		j++
	}
	if !validEscape(s, j) {
		return s[i:j], j
	}
	var b strings.Builder
	b.WriteString(s[i:j])
	for j < len(s) {
		if isNameChar(s[j]) {
			b.WriteByte(s[j])
			j++
		} else if validEscape(s, j) {
			var r rune
			r, j = readEscape(s, j)
			b.WriteRune(r)
		} else {
			break
		}
	}
	return b.String(), j
}

// readString decodes the string that starts at the quote s[i].
func readString(s string, i int) (string, int, error) {
	q := s[i]
	var b strings.Builder
	for j := i + 1; j < len(s); {
		switch c := s[j]; {
		case c == q:
			return b.String(), j + 1, nil
		case c == '\n' || c == '\r' || c == '\f':
			return "", 0, fmt.Errorf("css: unterminated string")
		case c != '\\':
			b.WriteByte(c)
			j++
		case j+1 >= len(s):
			return "", 0, fmt.Errorf("css: unterminated string")
		case validEscape(s, j):
			var r rune
			r, j = readEscape(s, j)
			b.WriteRune(r)
		default: // an escaped newline continues the string
			j += 2
		}
	}
	return "", 0, fmt.Errorf("css: unterminated string")
}

// rawURL reports whether the ( at s[i] opens an unquoted url(…): its content
// is one token to the next `)`, whatever it holds — `;`, `{`, `/*`.
func rawURL(s string, i int) bool {
	if i < 3 || !strings.EqualFold(s[i-3:i], "url") || i > 3 && (isNameChar(s[i-4]) || s[i-4] == '\\') {
		return false
	}
	for i++; i < len(s) && isSpace(s[i]); i++ {
	}
	return i < len(s) && s[i] != '"' && s[i] != '\''
}

// skip returns the index after what starts at s[i]: a string, a comment, an
// escape, url(…), a bracketed block — or the one byte.
func skip(s string, i, depth int) (int, error) {
	switch c := s[i]; c {
	case '"', '\'':
		_, j, err := readString(s, i)
		return j, err
	case '\\':
		if i+1 >= len(s) {
			return 0, fmt.Errorf("css: a backslash at the end")
		}
		_, n := utf8.DecodeRuneInString(s[i+1:])
		return i + 1 + n, nil
	case '/':
		if !strings.HasPrefix(s[i:], "/*") {
			return i + 1, nil
		}
		end := strings.Index(s[i+2:], "*/")
		if end < 0 {
			return 0, fmt.Errorf("css: unterminated comment")
		}
		return i + 2 + end + 2, nil
	case '(':
		if rawURL(s, i) {
			for j := i + 1; j < len(s); j++ {
				switch s[j] {
				case ')':
					return j + 1, nil
				case '\\':
					j++
				}
			}
			return 0, fmt.Errorf("css: unterminated url()")
		}
		return skipBlock(s, i, ')', depth)
	case '[':
		return skipBlock(s, i, ']', depth)
	case '{':
		return skipBlock(s, i, '}', depth)
	case ')', ']', '}':
		return 0, fmt.Errorf("css: unbalanced %q", c)
	}
	return i + 1, nil
}

func skipBlock(s string, i int, end byte, depth int) (int, error) {
	if depth >= maxDepth {
		return 0, fmt.Errorf("css: brackets nested deeper than %d", maxDepth)
	}
	for j := i + 1; j < len(s); {
		if s[j] == end {
			return j + 1, nil
		}
		var err error
		if j, err = skip(s, j, depth+1); err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("css: unclosed %q", s[i])
}

// scanUntil returns the index of the first byte of stops at the top level of
// s[i:], or len(s).
func scanUntil(s string, i int, stops string, depth int) (int, error) {
	for i < len(s) {
		if strings.IndexByte(stops, s[i]) >= 0 {
			return i, nil
		}
		var err error
		if i, err = skip(s, i, depth); err != nil {
			return 0, err
		}
	}
	return i, nil
}

// trim removes the whitespace around s, keeping the one an escape at the end
// consumes (`.a\ `).
func trim(s string) string {
	i, j := 0, len(s)
	for i < j && isSpace(s[i]) {
		i++
	}
	for j > i && isSpace(s[j-1]) {
		j--
	}
	if j < len(s) {
		n := 0
		for k := j; k > i && s[k-1] == '\\'; k-- {
			n++
		}
		if n%2 == 1 {
			j++
		}
	}
	return s[i:j]
}

// splitList splits at the top-level commas: a selector list, layer names.
func splitList(s string) ([]string, error) {
	var out []string
	for i := 0; ; {
		j, err := scanUntil(s, i, ",", 0)
		if err != nil {
			return nil, err
		}
		out = append(out, trim(s[i:j]))
		if j >= len(s) {
			return out, nil
		}
		i = j + 1
	}
}

// parseRules reads a list of rules: the sheet, or (inBlock) the body of an
// at-rule that holds rules.
func parseRules(s string, inBlock bool, depth int) ([]*rule, error) {
	if depth >= maxDepth {
		return nil, fmt.Errorf("css: at-rules nested deeper than %d", maxDepth)
	}
	var out []*rule
	for i := 0; ; {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			return out, nil
		}
		start := i
		if strings.HasPrefix(s[i:], "/*") {
			j, err := skip(s, i, depth)
			if err != nil {
				return nil, err
			}
			out = append(out, &rule{kind: kComment, raw: s[start:j]})
			i = j
			continue
		}
		at := s[i] == '@'
		// A qualified rule runs to its block. Between the rules of a block a
		// `;` ends a stray declaration; at the top level it is part of the
		// prelude, as a browser reads it.
		stops := "{"
		if at || inBlock {
			stops = "{;"
		}
		if inBlock && !at && customAt(s, i) {
			stops = ";" // a custom property's value may hold a {} block
		}
		j, err := scanUntil(s, i, stops, depth)
		if err != nil {
			return nil, err
		}
		r := &rule{head: trim(s[start:j])}
		if at {
			r.kind = kAt
			var n int
			r.name, n = readName(r.head, 1)
			r.name, r.prelude = strings.ToLower(r.name), trim(r.head[n:])
		}
		if j >= len(s) || s[j] == ';' {
			if r.head == "" {
				i = j + 1
				continue
			}
			if !at && !inBlock {
				return nil, fmt.Errorf("css: a rule without a block: %q", clip(r.head))
			}
			if !at {
				r.kind = kRaw
			}
			r.raw = r.head + ";"
			out = append(out, r)
			i = min(j+1, len(s))
			continue
		}
		end, err := skipBlock(s, j, '}', depth)
		if err != nil {
			return nil, err
		}
		r.raw, r.body, r.block = s[start:end], s[j+1:end-1], true
		i = end
		out = append(out, r)
		switch {
		case r.name == "layer" && r.prelude != "" && !layerName(r.prelude):
			// `@layer a, b{…}`, `@layer 1{…}`: a browser ignores it, block
			// and all. It stays an at-rule kept as it is: as a statement it
			// would order what the block never did.
		case !at:
			r.kind = kStyle
			if r.decls, r.opaque, err = parseDecls(r.body, depth); err != nil {
				return nil, err
			}
		case groups[r.name], r.name == "layer":
			r.kind = kGroup
			if r.name == "layer" {
				r.kind = kLayer
			}
			if r.rules, err = parseRules(r.body, true, depth+1); err != nil {
				return nil, err
			}
		case strings.HasSuffix(r.name, "keyframes"): // and -webkit-keyframes
			r.kind = kKeyframes
		case r.name == "position-try" && tryName(r.prelude) != "":
			r.kind = kTry
		}
	}
}

// tryName is the name of a `@position-try` rule, decoded: its prelude, when
// that is one dashed identifier (`--edge`) and nothing else. "": it is not
// — a browser drops the rule, and here it is an at-rule kept as it is.
func tryName(prelude string) string {
	if !strings.HasPrefix(prelude, "--") {
		return ""
	}
	name, end := readName(prelude, 0)
	if end != len(prelude) || len(name) < 3 {
		return ""
	}
	return name
}

// layerName reports whether s names a layer as every browser reads it:
// identifiers joined by dots — nothing between them — and none a CSS-wide
// keyword. It errs on the side of "no": then a block is kept as it is and a
// statement orders nothing, which at worst leaves one statement too many.
func layerName(s string) bool {
	for i := 0; identStart(s, i); {
		name, j := readName(s, i)
		switch strings.ToLower(name) {
		case "initial", "inherit", "unset", "revert", "revert-layer", "default":
			return false
		}
		if j == len(s) {
			return true
		}
		if s[j] != '.' {
			return false
		}
		i = j + 1
	}
	return false
}

// parseDecls splits a style rule's block into declarations. opaque: the block
// nests a rule or an at-rule, and is not taken apart.
func parseDecls(body string, depth int) (decls []*decl, opaque bool, err error) {
	for i := 0; i < len(body); {
		if isSpace(body[i]) || body[i] == ';' {
			i++
			continue
		}
		if body[i] == '@' {
			return nil, true, nil
		}
		start := i
		j, err := scanUntil(body, i, ";{", depth+1)
		if err != nil {
			return nil, false, err
		}
		d := parseDecl(trim(body[start:j]))
		// A { before the ; is a nested rule — unless this is a custom
		// property, whose value may hold any block: `--json: { "a": 1 }`.
		for j < len(body) && body[j] == '{' {
			if !d.custom {
				return nil, true, nil
			}
			if j, err = skipBlock(body, j, '}', depth+1); err != nil {
				return nil, false, err
			}
			if j, err = scanUntil(body, j, ";{", depth+1); err != nil {
				return nil, false, err
			}
			d = parseDecl(trim(body[start:j]))
		}
		decls = append(decls, d)
		i = j
	}
	return decls, false, nil
}

// customAt reports whether a custom property declaration starts at s[i]:
// `--x:`. Between rules that is how a browser reads it, whatever follows.
func customAt(s string, i int) bool {
	if !strings.HasPrefix(s[i:], "--") {
		return false
	}
	_, j := readName(s, i)
	for j < len(s) && isSpace(s[j]) {
		j++
	}
	return j < len(s) && s[j] == ':'
}

func parseDecl(text string) *decl {
	d := &decl{text: text}
	i := 0
	for i < len(text) { // comments before the name
		if isSpace(text[i]) {
			i++
		} else if strings.HasPrefix(text[i:], "/*") {
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return d
			}
			i += end + 4
		} else {
			break
		}
	}
	if !identStart(text, i) {
		return d
	}
	name, j := readName(text, i)
	for j < len(text) && isSpace(text[j]) {
		j++
	}
	if j >= len(text) || text[j] != ':' {
		return d
	}
	d.name, d.value = name, text[j+1:]
	if d.custom = strings.HasPrefix(name, "--"); !d.custom {
		d.name = strings.ToLower(name)
	}
	return d
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

// words calls f with every identifier (decoded) and every string of a piece
// of CSS: a value, a prelude, a style attribute. Units and hex colours come
// out as identifiers too; a caller looks names up, so extra ones are harmless.
func words(s string, f func(w string)) {
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			w, j, err := readString(s, i)
			if err != nil {
				i++
				continue
			}
			f(w)
			i = j
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return
			}
			i += end + 4
		case c == '(' && rawURL(s, i):
			j, err := skip(s, i, 0)
			if err != nil {
				return
			}
			i = j
		case c >= '0' && c <= '9': // a number: its unit is read next, apart
			i++
		case isNameChar(c) || validEscape(s, i):
			w, j := readName(s, i)
			f(w)
			i = j
		default:
			i++
		}
	}
}
