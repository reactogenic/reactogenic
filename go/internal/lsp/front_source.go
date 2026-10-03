package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx/server"

	"github.com/reactogenic/reactogenic/go/internal/emit"
	"github.com/reactogenic/reactogenic/go/internal/mapper"
	"github.com/reactogenic/reactogenic/go/internal/syntax"
	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// What the front does for a request at a position of an open .rtsx document
// (ide.md, *Slots*, *Segments*, *Span map* → slot groups). TypeScript
// answers at the tokens the virtual text has; the source has more names than
// that:
//
//   - every tag of a slot group but the first, and the closing tag of an
//     element that is emitted self-closing: the request is moved to the tag
//     that is copied, and the range of its answer is set back;
//   - a slot tag being typed: the completion is the owner's `$` props;
//   - `#name`: the mounted file, the sibling modules — from the names next
//     to the document, which the server is asked for.

// RequestFailed: a request that is refused, with a message for the user.
const requestFailed = -32803

// positional are the methods that take a position and are moved (rename is
// moved and not awaited: the server writes its whole answer).
var positional = map[string]bool{
	"textDocument/hover": true, "textDocument/definition": true, "textDocument/typeDefinition": true,
	"textDocument/implementation": true, "textDocument/references": true, "textDocument/documentHighlight": true,
	"textDocument/prepareRename": true, "textDocument/rename": true, "textDocument/completion": true,
}

// request is a forwarded request whose answer is rewritten.
type request struct {
	Method string
	uri    string // the document, as the server names it
	src    *sourceTree
	syn    *server.Syntactic
	// moved: asked at from, answered at to.
	moved    bool
	from, to emit.Span
	// pos is the source offset asked about (after the move: in to).
	pos int
	// slotTag is the tag name a completion is in — a slot's — or -1.
	slotTag int
	// segment is the `#name` a definition or completion is on.
	segment     emit.Span
	segmentName string
}

// analysis is the source of an open document, kept until its text changes.
func (f *front) analysis(uri, text string) *sourceTree {
	f.mu.Lock()
	src := f.sources[uri]
	f.mu.Unlock()
	if src != nil && src.text == text {
		return src
	}
	// Of the files around it the transform would learn which segments
	// exist; nothing here depends on that.
	_, file := mapper.Transform("/source/document.rtsx", text, func(string) bool { return false })
	src = analyse(text, file.Output)
	f.mu.Lock()
	if _, open := f.docs[uri]; open {
		f.sources[uri] = src
	}
	f.mu.Unlock()
	return src
}

// position handles a positional request for an open .rtsx document: handled
// when it is answered here; else forward, when not nil, goes to the server in
// its place.
func (f *front) position(msg message, uri, text, encoding string) (handled bool, forward []byte) {
	var params map[string]json.RawMessage
	if json.Unmarshal(msg.Params, &params) != nil {
		return false, nil
	}
	syn := newSyntactic(text, encoding)
	pos, err := syn.Offset(params["position"])
	if err != nil {
		return false, nil // the server says what is wrong with the request
	}
	src := f.analysis(uri, text)
	req := request{Method: msg.Method, uri: uri, src: src, syn: syn, pos: pos, slotTag: -1}

	if at, name, ok := src.segmentAt(pos); ok {
		switch msg.Method {
		case "textDocument/prepareRename", "textDocument/rename":
			// ide.md, *Not in the first release*: `#name` has no virtual
			// token, and its import is generated.
			f.replyError(*msg.ID, requestFailed, "A segment root cannot be renamed: `#"+name+"` mounts the file of that name. Rename the file, then the root.")
			return true, nil
		case "textDocument/definition", "textDocument/completion":
			req.segment, req.segmentName = at, name
			return false, f.forward(msg, req, methodSiblings, map[string]any{"textDocument": map[string]any{"uri": uri}})
		}
		return false, nil
	}

	await := msg.Method == "textDocument/documentHighlight" || msg.Method == "textDocument/references"
	if from, to, ok := src.retarget(pos); ok {
		at := to.Pos + min(pos-from.Pos, to.Len())
		position, err := json.Marshal(syn.Range(at, at).Start)
		if err != nil {
			return false, nil
		}
		params["position"] = position
		req.moved, req.from, req.to, req.pos = true, from, to, at
		await = await || msg.Method != "textDocument/rename"
	}
	if msg.Method == "textDocument/completion" {
		if i := src.tagAt(emit.Span{Pos: pos, End: pos}); i >= 0 && !src.tags[i].closing && src.isSlotTag(src.tags[i]) {
			req.slotTag, await = i, true
		}
	}
	if !await && !req.moved {
		return false, nil
	}
	if !await {
		req = request{} // not awaited
	}
	return false, f.forward(msg, req, msg.Method, params)
}

// forward is msg for the server, with another method or other params; its
// answer is awaited when req names a method.
func (f *front) forward(msg message, req request, method string, params any) []byte {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	body, err := json.Marshal(message{JSONRPC: "2.0", ID: msg.ID, Method: method, Params: raw})
	if err != nil {
		return nil
	}
	if req.Method != "" {
		f.mu.Lock()
		f.pending[string(*msg.ID)] = req
		f.mu.Unlock()
	}
	return body
}

// sourceAnswer rewrites the server's result for a positional request. A
// panic here must not end the process: the server's answer goes on as it is.
func (f *front) sourceAnswer(req request, result json.RawMessage) (rewritten json.RawMessage, changed bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(f.log, "reactogenic lsp: panic rewriting the answer to %s: %v\n%s\n", req.Method, r, debug.Stack())
			rewritten, changed = nil, false
			if req.segment.Len() > 0 {
				// The server was asked something else: its answer is not one.
				rewritten, changed = json.RawMessage("null"), true
			}
		}
	}()
	switch {
	case req.segment.Len() > 0 && req.Method == "textDocument/definition":
		return f.segmentDefinition(req, result)
	case req.segment.Len() > 0:
		return f.segmentCompletion(req, result)
	}
	switch req.Method {
	case "textDocument/completion":
		if req.slotTag >= 0 {
			return f.slotCompletion(req, result)
		}
	case "textDocument/documentHighlight":
		return f.twinHighlights(req, result)
	case "textDocument/references":
		return f.twinReferences(req, result)
	case "textDocument/hover", "textDocument/prepareRename":
		return f.setBack(req, result, "range")
	case "textDocument/definition", "textDocument/typeDefinition", "textDocument/implementation":
		var links []map[string]json.RawMessage
		if json.Unmarshal(result, &links) != nil {
			return nil, false
		}
		changed := false
		for _, link := range links {
			if set, ok := f.setBack(req, link["originSelectionRange"], ""); ok {
				link["originSelectionRange"], changed = set, true
			}
		}
		if !changed {
			return nil, false
		}
		out, err := marshal(links)
		return out, err == nil
	}
	return nil, false
}

// setBack: the answer's own range — the field of that name, or with "" the
// value itself — is the name that answered; it becomes the name asked about.
func (f *front) setBack(req request, value json.RawMessage, field string) (json.RawMessage, bool) {
	if !req.moved || len(value) == 0 {
		return nil, false
	}
	answered, _ := json.Marshal(req.syn.Range(req.to.Pos, req.to.End))
	asked, _ := json.Marshal(req.syn.Range(req.from.Pos, req.from.End))
	if field == "" {
		if !sameJSON(value, answered) {
			return nil, false
		}
		return asked, true
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(value, &object) != nil || !sameJSON(object[field], answered) {
		return nil, false
	}
	object[field] = asked
	out, err := marshal(object)
	return out, err == nil
}

// sameJSON compares two JSON values of the same shape, whatever their
// spacing and key order.
func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	one, _ := json.Marshal(x)
	other, _ := json.Marshal(y)
	return bytes.Equal(one, other)
}

// twinRanges are the ranges of the source names that are the same name as
// the one at the LSP range raw and that TypeScript does not list: the other
// tags of a slot group, an element's other tag.
func (req request) twinRanges(raw json.RawMessage) []server.Range {
	var r struct{ Start, End json.RawMessage }
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	pos, err := req.syn.Offset(r.Start)
	if err != nil {
		return nil
	}
	end, err := req.syn.Offset(r.End)
	if err != nil || end < pos {
		return nil
	}
	var out []server.Range
	for _, twin := range req.src.twins(emit.Span{Pos: pos, End: end}) {
		out = append(out, req.syn.Range(twin.Pos, twin.End))
	}
	return out
}

// twinHighlights adds, to the highlights of a name, the names that are the
// same name in the source. On a tag TypeScript highlights the element's two
// tags, whole; of an element that is rebuilt — it has slots — neither is
// source text, and the answer is empty: its names are highlighted then.
func (f *front) twinHighlights(req request, result json.RawMessage) (json.RawMessage, bool) {
	var highlights []map[string]json.RawMessage
	if json.Unmarshal(result, &highlights) != nil {
		return nil, false
	}
	if i := req.src.tagAt(emit.Span{Pos: req.pos, End: req.pos}); len(highlights) == 0 && i >= 0 {
		name, _ := marshal(req.syn.Range(req.src.tags[i].span.Pos, req.src.tags[i].span.End))
		highlights = append(highlights, map[string]json.RawMessage{"range": name, "kind": json.RawMessage("2")}) // read
		result = nil
	}
	seen := map[string]bool{}
	for _, h := range highlights {
		seen[canonical(h["range"])] = true
	}
	changed := false
	for _, h := range highlights[:len(highlights):len(highlights)] {
		for _, twin := range req.twinRanges(h["range"]) {
			raw, _ := json.Marshal(twin)
			if key := canonical(raw); !seen[key] {
				seen[key], changed = true, true
				more := map[string]json.RawMessage{"range": raw}
				if kind, ok := h["kind"]; ok {
					more["kind"] = kind
				}
				highlights = append(highlights, more)
			}
		}
	}
	if !changed && result != nil {
		return nil, false
	}
	out, err := marshal(highlights)
	return out, err == nil
}

func (f *front) twinReferences(req request, result json.RawMessage) (json.RawMessage, bool) {
	var locations []map[string]json.RawMessage
	if json.Unmarshal(result, &locations) != nil {
		return nil, false
	}
	// The answer names the document as the client does.
	uri, _ := json.Marshal(f.names.clientName(req.uri))
	seen := map[string]bool{}
	for _, l := range locations {
		if sameJSON(l["uri"], uri) {
			seen[canonical(l["range"])] = true
		}
	}
	changed := false
	for _, l := range locations[:len(locations):len(locations)] {
		if !sameJSON(l["uri"], uri) {
			continue
		}
		for _, twin := range req.twinRanges(l["range"]) {
			raw, _ := json.Marshal(twin)
			if key := canonical(raw); !seen[key] {
				seen[key], changed = true, true
				locations = append(locations, map[string]json.RawMessage{"uri": l["uri"], "range": raw})
			}
		}
	}
	if !changed {
		return nil, false
	}
	out, err := marshal(locations)
	return out, err == nil
}

// marshal is json.Marshal without HTML escapes: an answer that is rewritten
// reads as the server wrote it (`<`, not `\u003c`).
func marshal(v any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

func canonical(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	out, _ := json.Marshal(v)
	return string(out)
}

// ours marks the completion items that are the front's: resolving one
// changes nothing, and the server never sees it.
const ours = "reactogenic"

// slotCompletion is the completion in the name of a slot tag (ide.md,
// *Slots*): TypeScript's property completion at the copied name, of which
// the `$` names are the owner's slots — plus the `$` names already written
// under the same owner, last: TypeScript leaves out a prop that is there,
// and a keyed slot is filled many times. The inserted text is the name.
func (f *front) slotCompletion(req request, result json.RawMessage) (json.RawMessage, bool) {
	var list map[string]json.RawMessage
	var items []map[string]json.RawMessage
	if json.Unmarshal(result, &list) == nil && list != nil {
		json.Unmarshal(list["items"], &items)
	} else {
		list = map[string]json.RawMessage{"isIncomplete": json.RawMessage("false")}
		json.Unmarshal(result, &items) // a plain array, or null
	}
	kept, have := []map[string]json.RawMessage{}, map[string]bool{}
	for _, item := range items {
		var label string
		json.Unmarshal(item["label"], &label)
		name := strings.TrimSuffix(label, "?")
		if !strings.HasPrefix(name, "$") {
			continue
		}
		raw, _ := json.Marshal(name)
		item["insertText"] = raw
		delete(item, "textEdit")
		delete(item, "insertTextFormat")
		kept, have[name] = append(kept, item), true
	}
	tag := req.src.tags[req.slotTag]
	for _, name := range req.src.slotsWritten(tag) {
		if have[name] {
			continue
		}
		have[name] = true
		raw, _ := json.Marshal(name)
		kept = append(kept, map[string]json.RawMessage{
			"label": raw, "insertText": raw, "filterText": raw,
			"kind":     json.RawMessage("5"),    // a field, as TypeScript's
			"sortText": json.RawMessage(`"13"`), // after TypeScript's ("12": optional members)
			"data":     json.RawMessage(`{"` + ours + `":"slot"}`),
		})
	}
	list["items"], _ = marshal(kept)
	out, err := marshal(list)
	return out, err == nil
}

// slotsWritten are the `$` names under the owner of the slot element of
// tag, in source order, tag's own aside — and `$Case` under a `Switch`,
// which is lowered away: TypeScript is never asked about it.
func (s *sourceTree) slotsWritten(tag tagName) []string {
	above := owner(tag.element)
	if above == nil {
		return nil
	}
	var names []string
	if syntax.FrameworkExport(above.AsJsxElement().OpeningElement.TagName()) == "Switch" {
		names = append(names, "$Case")
	}
	for _, other := range s.tags {
		if !other.closing && other.element != tag.element && s.isSlotTag(other) && owner(other.element) == above {
			names = append(names, s.slice(other.span))
		}
	}
	return names
}

// files are the names of a reactogenic/siblings answer.
func files(result json.RawMessage) []string {
	var answer siblingsResult
	json.Unmarshal(result, &answer)
	return answer.Files
}

// sibling is the URI of the file name next to the document uri, as the
// client names it; "" for a document that is not a file.
func sibling(uri, name string) string {
	slash := strings.LastIndexByte(uri, '/')
	if !strings.HasPrefix(uri, "file:") || slash < 0 {
		return ""
	}
	return uri[:slash+1] + url.PathEscape(name)
}

// segmentDefinition: `#about-us` goes to the file it mounts — the first of
// the lookup order that is next to the document (syntax.md, *Segment
// files*).
func (f *front) segmentDefinition(req request, result json.RawMessage) (json.RawMessage, bool) {
	null := json.RawMessage("null")
	names := map[string]bool{}
	for _, name := range files(result) {
		names[name] = true
	}
	for _, extension := range transpiler.SegmentExtensions {
		target := sibling(req.uri, req.segmentName+extension)
		if !names[req.segmentName+extension] || target == "" {
			continue
		}
		top := json.RawMessage(`{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}`)
		var out []byte
		f.mu.Lock()
		links := f.definitionLinks
		f.mu.Unlock()
		if links {
			out, _ = marshal([]map[string]any{{"originSelectionRange": req.syn.Range(req.segment.Pos, req.segment.End), "targetUri": target, "targetRange": top, "targetSelectionRange": top}})
		} else {
			out, _ = marshal([]map[string]any{{"uri": target, "range": top}})
		}
		return out, true
	}
	return null, true
}

var segmentName = regexp.MustCompile(`^[A-Za-z_$][\w$-]*$`)

// segmentCompletion: after `#`, the sibling modules that the file does not
// mount yet (ide.md, *Segments*) — each name once, the file of the lookup
// order's first extension.
func (f *front) segmentCompletion(req request, result json.RawMessage) (json.RawMessage, bool) {
	mounted := req.src.mounted()
	delete(mounted, req.segmentName) // the one being typed
	self := path.Base(req.uri)
	if unescaped, err := url.PathUnescape(self); err == nil {
		self = unescaped
	}
	names := files(result)
	items := []map[string]any{}
	listed := map[string]bool{}
	for _, extension := range transpiler.SegmentExtensions {
		for _, file := range names {
			name, ok := strings.CutSuffix(file, extension)
			if !ok || file == self || strings.HasSuffix(name, ".d") || !segmentName.MatchString(name) || mounted[name] || listed[name] {
				continue
			}
			listed[name] = true
			items = append(items, map[string]any{
				"label": "#" + name, "kind": 17 /* a file */, "detail": file, "filterText": "#" + name, "sortText": name,
				"textEdit": map[string]any{"range": req.syn.Range(req.segment.Pos, req.segment.End), "newText": "#" + name},
				"data":     map[string]string{ours: "segment"},
			})
		}
	}
	out, err := marshal(map[string]any{"isIncomplete": false, "items": items})
	return out, err == nil
}

// isOurs reports whether a completion item — the params of
// completionItem/resolve — is one the front made.
func isOurs(item json.RawMessage) bool {
	var p struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(item, &p) != nil {
		return false
	}
	_, ok := p.Data[ours]
	return ok
}
