package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/microsoft/TypeScript/tsc/rtsx"
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
	// slotTag is the tag name a completion is in — a slot's — or -1; ownerAt,
	// the source offset of the tag that names what the slot is a slot of, or
	// -1. first is the server's completion there, while the owner's slots
	// are asked for (reactogenic/slots); client, the id the client waits on.
	slotTag int
	ownerAt int
	first   json.RawMessage
	client  json.RawMessage
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
	req := request{Method: msg.Method, uri: uri, src: src, syn: syn, pos: pos, slotTag: -1, ownerAt: -1}

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

	tag := src.tagAt(emit.Span{Pos: pos, End: pos})
	if msg.Method == "textDocument/completion" && tag >= 0 && src.tags[tag].closing {
		// The name of a closing tag is not chosen: it is its opening tag's.
		// Moved there, the list would be the props of the element above.
		if _, _, moved := src.retarget(pos); moved || src.isSlotTag(src.tags[tag]) {
			return f.reply(*msg.ID, nil) == nil, nil
		}
		return false, nil
	}
	// The answer is looked at: the other tags of a name are added to its
	// references, in any file; the name prepareRename shows is unquoted.
	await := msg.Method == "textDocument/documentHighlight" || msg.Method == "textDocument/references" || msg.Method == "textDocument/prepareRename"
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
	if msg.Method == "textDocument/completion" && tag >= 0 && src.isSlotTag(src.tags[tag]) {
		req.slotTag, req.ownerAt, await = tag, src.ownerAt(src.tags[tag]), true
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
			return f.slotCompletion(req, result, nil)
		}
	case "textDocument/documentHighlight":
		return f.twinHighlights(req, result)
	case "textDocument/references":
		return f.twinReferences(req, result)
	case "textDocument/hover":
		return f.setBack(req, result, "range")
	case "textDocument/prepareRename":
		set, moved := f.setBack(req, result, "range")
		if moved {
			result = set
		}
		plain, unquoted := unquote(req, result)
		if unquoted {
			result = plain
		}
		return result, moved || unquoted
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
// value itself — is in the name that answered; it becomes the same part of
// the name asked about: the whole of `$Icon`, the `Kit` or the `Panel` of
// `</Kit.Panel>`.
func (f *front) setBack(req request, value json.RawMessage, field string) (json.RawMessage, bool) {
	if !req.moved || len(value) == 0 {
		return nil, false
	}
	if field == "" {
		return req.back(value)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(value, &object) != nil {
		return nil, false
	}
	asked, ok := req.back(object[field])
	if !ok {
		return nil, false
	}
	object[field] = asked
	out, err := marshal(object)
	return out, err == nil
}

// offsets are the source offsets of an LSP range.
func (req request) offsets(raw json.RawMessage) (at emit.Span, ok bool) {
	var r struct{ Start, End json.RawMessage }
	if json.Unmarshal(raw, &r) != nil {
		return at, false
	}
	pos, err := req.syn.Offset(r.Start)
	if err != nil {
		return at, false
	}
	end, err := req.syn.Offset(r.End)
	if err != nil || end < pos {
		return at, false
	}
	return emit.Span{Pos: pos, End: end}, true
}

// back is the range raw, which lies in the name that answered, as the same
// part of the name asked about.
func (req request) back(raw json.RawMessage) (json.RawMessage, bool) {
	at, ok := req.offsets(raw)
	if !ok || !within(at, req.to) {
		return nil, false
	}
	shift := req.from.Pos - req.to.Pos
	out, err := json.Marshal(req.syn.Range(at.Pos+shift, at.End+shift))
	return out, err == nil
}

// unquote: the name that prepareRename shows for a name that is no
// identifier — `$sub-item`, where it is declared, and where a slot element
// under a slot element makes a quoted key of it — comes in quotes, which are
// not in its range: kept in the new name, they would be written into the
// name. It is shown as it is written.
func unquote(req request, result json.RawMessage) (json.RawMessage, bool) {
	var object map[string]json.RawMessage
	var placeholder string
	if json.Unmarshal(result, &object) != nil || json.Unmarshal(object["placeholder"], &placeholder) != nil || len(placeholder) < 3 {
		return nil, false
	}
	quote := placeholder[0]
	at, ok := req.offsets(object["range"])
	if !ok || quote != '"' && quote != '\'' || placeholder[len(placeholder)-1] != quote || req.src.slice(at) != placeholder[1:len(placeholder)-1] {
		return nil, false
	}
	object["placeholder"], _ = marshal(placeholder[1 : len(placeholder)-1])
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
	at, ok := req.offsets(raw)
	if !ok {
		return nil
	}
	var out []server.Range
	for _, twin := range req.src.twins(at) {
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

// twinReferences adds, to the references of a name, the names that are the
// same name in the source of each .rtsx file of the answer — whichever
// document asked: the tags of a slot group after its first, the closing tag
// of an element that is emitted without one. A document that is open is
// read as it is in the editor; any other file, as the server reads it: from
// disk.
func (f *front) twinReferences(req request, result json.RawMessage) (json.RawMessage, bool) {
	var locations []map[string]json.RawMessage
	if json.Unmarshal(result, &locations) != nil {
		return nil, false
	}
	seen := map[string]bool{}
	for _, l := range locations {
		seen[canonical(l["uri"])+canonical(l["range"])] = true
	}
	sources := map[string]*request{} // by the URI of the answer; nil: not ours
	changed := false
	for _, l := range locations[:len(locations):len(locations)] {
		var uri string
		if json.Unmarshal(l["uri"], &uri) != nil {
			continue
		}
		in, read := sources[uri]
		if !read {
			in = f.sourceOf(req, uri)
			sources[uri] = in
		}
		if in == nil {
			continue
		}
		for _, twin := range in.twinRanges(l["range"]) {
			raw, _ := json.Marshal(twin)
			if key := canonical(l["uri"]) + canonical(raw); !seen[key] {
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

// sourceOf is the source of the .rtsx file that an answer names uri — as the
// client names it — for reading ranges of the answer; nil for any other
// file, and for one that cannot be read.
func (f *front) sourceOf(req request, uri string) *request {
	// The requesting document is named as the client names it.
	if uri == f.names.clientName(req.uri) {
		return &req
	}
	name := f.names.serverName(uri)
	f.mu.Lock()
	text, open := f.docs[name]
	encoding := f.encoding
	f.mu.Unlock()
	if !open {
		file, ok := filePath(name)
		if !ok || !isRTSX(file) {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil
		}
		text = string(data)
	}
	return &request{uri: name, src: f.analysis(name, text), syn: newSyntactic(text, encoding)}
}

// filePath is the path of a `file:` URI.
func filePath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // `/c:/…`
	}
	return filepath.FromSlash(p), true
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
// *Slots*): exactly the `$` props of what the slot is a slot of. props are
// those, asked of the checker at the owner's tag (reactogenic/slots);
// result is TypeScript's property completion at the name being typed, which
// has an item — with its documentation — for each prop that is not written
// yet, when the name being typed has a copy in the virtual text at all. An
// item is made here for every other prop: one that is written (a keyed slot
// is filled many times), or all of them (a `<$` under a `Match`, still
// open). The inserted text is the name.
//
// Without props — the owner is not one the checker knows: it is being typed
// too — the list is TypeScript's `$` items and the `$` names written under
// the owner.
func (f *front) slotCompletion(req request, result json.RawMessage, props []slotProp) (json.RawMessage, bool) {
	var list map[string]json.RawMessage
	var items []map[string]json.RawMessage
	if json.Unmarshal(result, &list) == nil && list != nil {
		json.Unmarshal(list["items"], &items)
	} else {
		list = map[string]json.RawMessage{"isIncomplete": json.RawMessage("false")}
		json.Unmarshal(result, &items) // a plain array, or null
	}
	declared := map[string]bool{}
	for _, prop := range props {
		declared[prop.Name] = true
	}
	kept, have := []map[string]json.RawMessage{}, map[string]bool{}
	for _, item := range items {
		var label string
		json.Unmarshal(item["label"], &label)
		name := strings.TrimSuffix(label, "?")
		if !strings.HasPrefix(name, "$") || len(props) > 0 && !declared[name] {
			continue
		}
		raw, _ := json.Marshal(name)
		item["insertText"] = raw
		delete(item, "textEdit")
		delete(item, "insertTextFormat")
		kept, have[name] = append(kept, item), true
	}
	made := func(name, label, detail string) {
		if have[name] {
			return
		}
		have[name] = true
		raw, _ := json.Marshal(name)
		item := map[string]json.RawMessage{
			"label": raw, "insertText": raw, "filterText": raw,
			"kind":     json.RawMessage("5"),    // a field, as TypeScript's
			"sortText": json.RawMessage(`"13"`), // after TypeScript's ("12": optional members)
			"data":     json.RawMessage(`{"` + ours + `":"slot"}`),
		}
		if label != name {
			item["label"], _ = marshal(label)
		}
		if detail != "" {
			item["detail"], _ = marshal(detail)
		}
		kept = append(kept, item)
	}
	for _, prop := range props {
		label := prop.Name
		if prop.Optional {
			label += "?" // as TypeScript labels an optional member
		}
		made(prop.Name, label, prop.Type)
	}
	if len(props) == 0 {
		for _, name := range req.src.slotsWritten(req.src.tags[req.slotTag]) {
			made(name, name, "")
		}
	}
	list["items"], _ = marshal(kept)
	out, err := marshal(list)
	return out, err == nil
}

// askSlots asks the server for the slots of the owner of a slot tag being
// typed, after its completion there came back: that is kept, and the client
// is answered when both are known. false: nothing was asked, and the
// completion is answered as it is.
func (f *front) askSlots(id json.RawMessage, req request, result *json.RawMessage) bool {
	f.mu.Lock()
	server := f.server
	f.asked++
	own, _ := json.Marshal(fmt.Sprintf("%s:%d", methodSlots, f.asked))
	f.mu.Unlock()
	if server == nil {
		return false
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": req.uri}, "offset": req.ownerAt})
	ownID := json.RawMessage(own)
	body, err := json.Marshal(message{JSONRPC: "2.0", ID: &ownID, Method: methodSlots, Params: params})
	if err != nil {
		return false
	}
	req.Method, req.client, req.first = methodSlots, id, json.RawMessage("null")
	if result != nil {
		req.first = *result
	}
	f.mu.Lock()
	f.pending[string(own)] = req
	f.mu.Unlock()
	// Not from here: this goroutine reads what the server writes, and the
	// server may be writing.
	go writeFrame(server, body)
	return true
}

// slotsAnswered is the answer to the client's completion in a slot tag, now
// that the server has said — or failed to say — what the owner's slots are.
func (f *front) slotsAnswered(req request, answer message) (body []byte) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(f.log, "reactogenic lsp: panic answering a slot completion: %v\n%s\n", r, debug.Stack())
			body, _ = marshal(message{JSONRPC: "2.0", ID: &req.client, Result: &req.first})
		}
	}()
	var slots slotsResult
	if answer.Error == nil && answer.Result != nil {
		json.Unmarshal(*answer.Result, &slots)
	}
	result := req.first
	if rewritten, ok := f.slotCompletion(req, req.first, slots.Slots); ok {
		result = rewritten
	}
	body, err := marshal(message{JSONRPC: "2.0", ID: &req.client, Result: &result})
	if err != nil {
		return nil
	}
	return body
}

// isFlow: the element is one the transform lowers away around its children —
// a `Match`, a `Switch`, an `Each`, a `$Case` of a `Switch`. A slot element
// under it is a slot of what is above it.
func (s *sourceTree) isFlow(element *rtsx.Node) bool {
	if element.Kind != rtsx.KindJsxElement {
		return false
	}
	tag := element.AsJsxElement().OpeningElement.TagName()
	switch syntax.FrameworkExport(tag) {
	case "Match", "Switch", "Each":
		return true
	}
	above := owner(element)
	return tag.Kind == rtsx.KindIdentifier && rtsx.NodeText(tag) == "$Case" && above != nil && syntax.FrameworkExport(above.AsJsxElement().OpeningElement.TagName()) == "Switch"
}

// slotOwner is what a slot element is a slot of: the nearest element above
// it that is no flow element — a component, or a slot element.
func (s *sourceTree) slotOwner(element *rtsx.Node) *rtsx.Node {
	above := owner(element)
	for above != nil && s.isFlow(above) {
		above = owner(above)
	}
	return above
}

// underSwitch: the slot element is a child of a `Switch` — a `$Case`, which
// is lowered away: TypeScript is never asked about it.
func underSwitch(element *rtsx.Node) bool {
	above := owner(element)
	return above != nil && syntax.FrameworkExport(above.AsJsxElement().OpeningElement.TagName()) == "Switch"
}

// ownerAt is the source offset of the name that answers for the owner of
// the slot element of tag: the tag of the component, or — the owner being a
// slot element — the tag of its group that is copied. -1 without an owner,
// and under a `Switch`.
func (s *sourceTree) ownerAt(tag tagName) int {
	above := s.slotOwner(tag.element)
	if above == nil || underSwitch(tag.element) {
		return -1
	}
	for _, other := range s.tags {
		if other.element == above && !other.closing {
			if group, _ := s.group(other.span); group != nil {
				return group.Tags[0].Pos
			}
			return other.span.Pos
		}
	}
	return -1
}

// slotsWritten are the `$` names under the owner of the slot element of
// tag, in source order, tag's own aside; under a `Switch`, `$Case`.
func (s *sourceTree) slotsWritten(tag tagName) []string {
	if underSwitch(tag.element) {
		return []string{"$Case"}
	}
	above := s.slotOwner(tag.element)
	if above == nil {
		return nil
	}
	var names []string
	for _, other := range s.tags {
		if !other.closing && other.element != tag.element && s.isSlotTag(other) && !s.isFlow(other.element) && s.slotOwner(other.element) == above {
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
