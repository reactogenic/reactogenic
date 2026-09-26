package conformance

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	rtsxHeader = regexp.MustCompile(`^//\s*(\S*)\.rtsx\b`)
	tsxHeader  = regexp.MustCompile(`^//\s*\S*\.tsx\b`)
	passHeader = regexp.MustCompile(`^//\s*after pass (\d+)\b`)
	segmentRe  = regexp.MustCompile(`\s#([A-Za-z][\w-]*)`)
	// An annotation is a `//` comment at the start of a line or after two or
	// more spaces. Inside JSX a `//` is text, not a comment, so annotations
	// are stripped before parsing.
	annotationRe = regexp.MustCompile(`(^\s*|\s{2,})//.*$`)
)

// Spec examples leave out their imports. Elements are recognised by import
// origin, so every example without an import from the package gets this one;
// after pass 2 only `Each` is left of it.
const (
	specPrelude      = "import { Switch, Match, Each } from \"@reactogenic/core\";\n"
	specPreludeAfter = "import { Each } from \"@reactogenic/core\";\n"
)

// specListSlots are the list slots the spec's examples declare.
var specListSlots = []string{"Form.$Field"}

type codeBlock struct {
	anchor string // slug of the nearest heading
	header string // first line, when it is a `//` comment
	body   string // without the header line and without annotations
}

// ExtractSpec turns the `.rtsx` → `.tsx` example pairs of a spec into cases.
//
// A block whose first line is `// .rtsx` (or `// page.rtsx`, or `// .rtsx — …`)
// is an input; the code blocks right after it headed `// .tsx`, `// page.tsx`
// or `// after pass N` are its expected outputs, one case each.
//
// Spec cases check output only: the `// Error: …` annotations in the spec
// mix transpiler errors with type errors that only `reactogenic check`
// reports, so diagnostics are covered by fixtures/ instead.
func ExtractSpec(name, markdown string) []Case {
	blocks := codeBlocks(markdown)
	var cases []Case
	counts := map[string]int{}
	for i := 0; i < len(blocks); i++ {
		m := rtsxHeader.FindStringSubmatch(blocks[i].header)
		if m == nil {
			continue
		}
		in := blocks[i]
		entry := "input.rtsx"
		if m[1] != "" {
			entry = m[1] + ".rtsx"
		}
		body, prelude := in.body, false
		if !strings.Contains(body, `from "@reactogenic/core"`) {
			body, prelude = specPrelude+body, true
		}
		files := map[string]string{entry: body}
		for _, seg := range segmentRe.FindAllStringSubmatch(in.body, -1) {
			files["+"+seg[1]+".rtsx"] = "export default function Segment() {\n  return null;\n}\n"
		}
		for i+1 < len(blocks) {
			out := blocks[i+1]
			pass := 0
			if p := passHeader.FindStringSubmatch(out.header); p != nil {
				pass, _ = strconv.Atoi(p[1])
			} else if !tsxHeader.MatchString(out.header) {
				break
			}
			i++
			counts[in.anchor]++
			want := out.body
			if prelude && (pass == 0 || pass >= 2) {
				want = specPreludeAfter + want // pass 2 dropped Switch and Match
			} else if prelude {
				want = specPrelude + want
			}
			cases = append(cases, Case{
				ID:        fmt.Sprintf("%s#%s/%d", name, in.anchor, counts[in.anchor]),
				Files:     files,
				Entry:     entry,
				UntilPass: pass,
				WantTSX:   want,
				// Spec examples check output only; see the doc comment.
				IgnoreDiagnostics: true,
				ListSlots:         specListSlots,
			})
		}
	}
	return cases
}

func codeBlocks(markdown string) []codeBlock {
	var (
		blocks  []codeBlock
		anchor  string
		slugs   = map[string]int{}
		inBlock bool
		lines   []string
	)
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				blocks = append(blocks, newBlock(anchor, lines))
				lines = nil
			}
			inBlock = !inBlock
			continue
		}
		if inBlock {
			lines = append(lines, line)
			continue
		}
		if h := headingRe.FindStringSubmatch(line); h != nil {
			anchor = slug(h[2], slugs)
		}
	}
	return blocks
}

func newBlock(anchor string, lines []string) codeBlock {
	b := codeBlock{anchor: anchor}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "//") {
		b.header = strings.TrimSpace(lines[0])
		lines = lines[1:]
	}
	for i, line := range lines {
		lines[i] = annotationRe.ReplaceAllString(line, "")
	}
	b.body = strings.Join(lines, "\n") + "\n"
	return b
}

// slug is GitHub's heading anchor: lowercase, punctuation dropped, spaces to
// hyphens, "-1", "-2", … for repeats.
func slug(heading string, seen map[string]int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ' || r == '-':
			b.WriteRune('-')
		case r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 && r != '—' && r != '–' && r != '→':
			b.WriteRune(r)
		}
	}
	s := b.String()
	n := seen[s]
	seen[s] = n + 1
	if n > 0 {
		s = fmt.Sprintf("%s-%d", s, n)
	}
	return s
}
