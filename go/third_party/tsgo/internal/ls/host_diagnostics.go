// Reactogenic: the diagnostics of a content-mapped document, from the host
// of the built-in mapper. Not part of upstream: added by
// go/patches/0006-rtsx-lsp.patch.

package ls

import (
	"context"
	"unicode/utf8"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

// HostDiagnostic is one diagnostic of a content-mapped file as its host
// reports it: in the text the author wrote, in the host's words.
type HostDiagnostic struct {
	// Pos and End are byte offsets in the file's original text.
	Pos, End int
	// Code and Source name a diagnostic that is the host's own, or one of
	// the compiler's that the host reworded. With Code empty the diagnostic
	// has the compiler's Number as its code and the source "ts", which the
	// compiler's quick fixes match on: TS's own, or — without TS — that of
	// a syntax error the host's parse of the original text found.
	Code, Source string
	Number       int32
	// Severity is that of a diagnostic that is not made from one of the
	// compiler's (TS nil).
	Severity lsproto.DiagnosticSeverity
	Message  string
	Related  []HostRelatedInformation
	// TS is the compiler's diagnostic this one was made from; nil for the
	// host's own. Its severity (the user's reportStyleChecksAsWarnings
	// included) and its tags — unnecessary, deprecated — are kept.
	TS *ast.Diagnostic
}

// HostRelatedInformation is a related location: byte offsets in the
// original text of FileName. With FileName empty it has no place of its
// own, and is shown at its diagnostic's.
type HostRelatedInformation struct {
	FileName string
	Pos, End int
	Message  string
}

// HostDiagnostics returns every diagnostic of a content-mapped file of
// program: the complete list, the compiler's included. It is given the
// program rather than the compiler's diagnostics because it decides which
// of them are asked for at all — never the syntactic ones of a virtual text.
type HostDiagnostics func(ctx context.Context, program *compiler.Program, file *ast.SourceFile) []HostDiagnostic

// ProvideHostDiagnostics answers textDocument/diagnostic for a content-mapped
// document with the host's diagnostics, in place of ProvideDiagnostics: the
// compiler's diagnostics reach the host still structured — before
// toLSPDiagnostics would map them, and aggregate those in synthesized code
// at the top of the file. ok is false for a document that is not content
// mapped: ProvideDiagnostics answers for it.
func (l *LanguageService) ProvideHostDiagnostics(ctx context.Context, uri lsproto.DocumentUri, provide HostDiagnostics) (response lsproto.DocumentDiagnosticResponse, ok bool) {
	program, file := l.getProgramAndFile(uri)
	if file.SpanMap() == nil {
		return response, false
	}
	items := []*lsproto.Diagnostic{}
	if !l.UserPreferences().EnableValidation.IsFalse() {
		document := hostScript{fileName: file.OriginalFileName(), text: file.OriginalText()}
		styleChecksAsWarnings := l.UserPreferences().ReportStyleChecksAsWarnings.IsTrue()
		relatedInformation := lsproto.GetClientCapabilities(ctx).TextDocument.Diagnostic.RelatedInformation
		for _, d := range provide(ctx, program, file) {
			items = append(items, l.hostDiagnosticToLSP(ctx, program, document, d, styleChecksAsWarnings, relatedInformation))
		}
	}
	return lsproto.RelatedFullDocumentDiagnosticReportOrUnchangedDocumentDiagnosticReport{
		FullDocumentDiagnosticReport: &lsproto.RelatedFullDocumentDiagnosticReport{Items: items},
	}, true
}

func (l *LanguageService) hostDiagnosticToLSP(ctx context.Context, program *compiler.Program, document hostScript, d HostDiagnostic, styleChecksAsWarnings, relatedInformation bool) *lsproto.Diagnostic {
	result := &lsproto.Diagnostic{Severity: &d.Severity}
	if d.TS != nil {
		// The compiler's diagnostic, converted as upstream converts it — its
		// number as the code, the source "ts", its severity and tags — and
		// then placed and worded by the host.
		result = lsconv.DiagnosticToLSPPull(ctx, l.converters, d.TS, styleChecksAsWarnings)
		result.RelatedInformation = nil
	}
	if d.Code != "" {
		result.Code = &lsproto.IntegerOrString{String: &d.Code}
		result.Source = &d.Source
	} else {
		result.Code = &lsproto.IntegerOrString{Integer: &d.Number}
		result.Source = new("ts")
	}
	result.Message = lsproto.StringOrMarkupContent{String: &d.Message}
	result.Range = l.hostRange(document, d.Pos, d.End)
	if relatedInformation && len(d.Related) > 0 {
		related := make([]*lsproto.DiagnosticRelatedInformation, 0, len(d.Related))
		for _, r := range d.Related {
			location := lsproto.Location{Uri: lsconv.FileNameToDocumentURI(document.fileName), Range: result.Range}
			if r.FileName != "" {
				location = lsproto.Location{Uri: lsconv.FileNameToDocumentURI(r.FileName)}
				if file := program.GetSourceFile(r.FileName); file != nil {
					script := hostScript{fileName: file.OriginalFileName(), text: file.Text()}
					if file.SpanMap() != nil {
						script.text = file.OriginalText()
					}
					location.Range, _ = l.converters.ToLSPRange(script, core.NewTextRange(r.Pos, r.End))
				}
			}
			related = append(related, &lsproto.DiagnosticRelatedInformation{Location: location, Message: r.Message})
		}
		result.RelatedInformation = &related
	}
	return result
}

// hostRange is the range of a span of the document's text. A span of no
// length covers the character after it — also a line break — so that every
// client has something to underline; at the end of the text it stays empty.
func (l *LanguageService) hostRange(document hostScript, pos, end int) lsproto.Range {
	pos = max(0, min(pos, len(document.text)))
	end = max(pos, min(end, len(document.text)))
	if end == pos && pos < len(document.text) {
		_, size := utf8.DecodeRuneInString(document.text[pos:])
		end = pos + size
		if document.text[pos] == '\r' && end < len(document.text) && document.text[end] == '\n' {
			end++
		}
	}
	lspRange, _ := l.converters.ToLSPRange(document, core.NewTextRange(pos, end))
	return lspRange
}

// hostScript is the text a host's positions are in — the original text of
// a content-mapped file, or the text of any other file — as a Script: its
// offsets convert to lines and characters without a span map.
type hostScript struct {
	fileName string
	text     string
}

func (s hostScript) FileName() string         { return s.fileName }
func (s hostScript) OriginalFileName() string { return s.fileName }
func (s hostScript) Text() string             { return s.text }
func (s hostScript) OriginalText() string     { return s.text }
func (hostScript) SpanMap() *spanmap.SpanMap  { return nil }
