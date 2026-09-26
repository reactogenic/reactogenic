package conformance

import (
	"fmt"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
)

// Canonical parses tsx and prints its AST as an indented tree, so that two
// programs compare equal when they mean the same thing:
//
//   - trivia is gone: whitespace, comments, formatting;
//   - parentheses are unwrapped;
//   - JSX text is cleaned the way React sees it, and dropped when empty;
//   - a block holding a single expression statement is that statement. The
//     spec writes a JSX child shown on its own as `{…}`, which TSX parses as
//     a block; the transpiler emits it at top level as an expression.
//
// It returns the parse errors of tsx as well; a canonical form of code that
// does not parse is not meaningful.
func Canonical(tsx string) (string, []string) {
	file := rtsx.ParseTSX("/canonical.tsx", tsx)
	var errs []string
	for _, d := range file.Diagnostics() {
		line, col := lineCol(tsx, d.Pos())
		errs = append(errs, fmt.Sprintf("%d:%d %s", line, col, rtsx.Message(d)))
	}
	var b strings.Builder
	dump(&b, file.AsNode(), 0)
	return b.String(), errs
}

func dump(b *strings.Builder, n *rtsx.Node, depth int) {
	switch n.Kind {
	case rtsx.KindEndOfFile:
		return
	case rtsx.KindParenthesizedExpression:
		n.ForEachChild(func(c *rtsx.Node) bool { dump(b, c, depth); return false })
		return
	case rtsx.KindBlock:
		if stmts := n.Statements(); len(stmts) == 1 && stmts[0].Kind == rtsx.KindExpressionStatement {
			dump(b, stmts[0], depth)
			return
		}
	}
	text := rtsx.NodeText(n)
	if n.Kind == rtsx.KindJsxText {
		text = cleanJSXText(text)
		if text == "" {
			return
		}
	}
	b.WriteString(strings.Repeat("  ", depth))
	b.WriteString(n.Kind.String())
	if text != "" {
		fmt.Fprintf(b, " %q", text)
	}
	b.WriteByte('\n')
	n.ForEachChild(func(c *rtsx.Node) bool { dump(b, c, depth+1); return false })
}

// cleanJSXText applies React's whitespace rule for JSX text: lines are
// trimmed (except the start of the first and the end of the last), empty
// lines are dropped, and the rest are joined with one space.
func cleanJSXText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var kept []string
	for i, line := range lines {
		if i > 0 {
			line = strings.TrimLeft(line, " \t")
		}
		if i < len(lines)-1 {
			line = strings.TrimRight(line, " \t")
		}
		if line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, " ")
}

// lineCol turns a byte offset into a 1-based line and column, skipping the
// leading trivia a node position may include.
func lineCol(text string, pos int) (int, int) {
	for pos < len(text) && (text[pos] == ' ' || text[pos] == '\t' || text[pos] == '\n' || text[pos] == '\r') {
		pos++
	}
	line, col := 1, 1
	for i := 0; i < pos && i < len(text); i++ {
		if text[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}
