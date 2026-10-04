package lsp

import (
	"context"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/report"
)

// source is the `source` of the diagnostics that are ours — the
// transpiler's, and TypeScript's reworded in slot terms: what the problem
// matcher of the extension's check task gives the same lines.
const source = "reactogenic"

// diagnostics is the answer to textDocument/diagnostic for an .rtsx document
// (ide.md, *Diagnostics*): the reports of the reporting layer — what
// `reactogenic check` prints for the file, and TypeScript's suggestions —
// in the server's terms. The server makes the LSP diagnostics of them.
//
//   - A transpiler diagnostic, a cross-file rule's and a reworded TS error
//     have their name as the code.
//   - A TS diagnostic that is not reworded keeps its number, and the source
//     "ts": TypeScript's quick fixes find it by them. So does a syntax error
//     of the source, which the transpiler's own parse reports.
func diagnostics(ctx context.Context, program *rtsx.Program, file *rtsx.SourceFile) []server.Diagnostic {
	reports := report.File(ctx, program, file)
	out := make([]server.Diagnostic, 0, len(reports))
	for _, r := range reports {
		d := server.Diagnostic{Pos: r.Span.Pos, End: r.Span.End, Message: r.Message, TS: r.TS}
		if number, ok := tsNumber(r.Code); ok && (r.TS == nil || r.TS.Code() == number) {
			d.Number = number
		} else {
			d.Code, d.Source = r.Code, source
		}
		switch r.Severity {
		case report.Warning:
			d.Severity = server.SeverityWarning
		case report.Suggestion:
			d.Severity = server.SeverityHint
		case report.Message:
			d.Severity = server.SeverityInformation
		default:
			d.Severity = server.SeverityError
		}
		for _, rr := range r.Related {
			d.Related = append(d.Related, server.RelatedInformation{FileName: rr.File, Pos: rr.Span.Pos, End: rr.Span.End, Message: rr.Message})
		}
		out = append(out, d)
	}
	return out
}

// tsNumber is the number of a code that is TypeScript's: "TS2322".
func tsNumber(code string) (int32, bool) {
	digits, ok := strings.CutPrefix(code, "TS")
	if !ok || digits == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(digits, 10, 32)
	return int32(n), err == nil
}
