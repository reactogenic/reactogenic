package lsptest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

// TextEdit is an LSP text edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit is an LSP workspace edit, in either of its two forms: a
// server chooses by the client's capabilities.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes"`
	DocumentChanges []struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Edits []TextEdit `json:"edits"`
	} `json:"documentChanges"`
}

// byURI is the edit's text edits per document.
func (e WorkspaceEdit) byURI() map[string][]TextEdit {
	out := map[string][]TextEdit{}
	for uri, edits := range e.Changes {
		out[uri] = append(out[uri], edits...)
	}
	for _, change := range e.DocumentChanges {
		out[change.TextDocument.URI] = append(out[change.TextDocument.URI], change.Edits...)
	}
	return out
}

// Edits renders a workspace edit as sorted `file line:col-line:col "text"`
// lines.
func (c *Client) Edits(edit WorkspaceEdit) []string {
	out := []string{}
	for uri, edits := range edit.byURI() {
		for _, e := range edits {
			out = append(out, fmt.Sprintf("%s %s %q", c.Rel(uri), e.Range, e.NewText))
		}
	}
	sort.Strings(out)
	return out
}

// Apply applies a workspace edit as the editor does: to the buffer of an
// open document (the server is told of the change), else to the file.
func (c *Client) Apply(edit WorkspaceEdit) {
	c.t.Helper()
	for uri, edits := range edit.byURI() {
		c.ApplyTo(c.Rel(uri), edits)
	}
}

// ApplyTo applies text edits to one document, as Apply does.
func (c *Client) ApplyTo(rel string, edits []TextEdit) {
	c.t.Helper()
	text := ApplyEdits(c.Text(rel), edits)
	c.mu.Lock()
	_, open := c.docs[rel]
	c.mu.Unlock()
	if open {
		c.Change(rel, text)
	} else {
		c.WriteFile(rel, text)
	}
}

// ApplyEdits applies the edits of one document to its text. Insertions at
// one position land in the order given, as LSP has it.
func ApplyEdits(text string, edits []TextEdit) string {
	order := make([]int, len(edits))
	for i := range order {
		order[i] = i
	}
	// From the end of the text, so that earlier offsets hold.
	sort.Slice(order, func(i, j int) bool {
		a, b := edits[order[i]].Range.Start, edits[order[j]].Range.Start
		if a != b {
			return a.Line > b.Line || a.Line == b.Line && a.Character > b.Character
		}
		return order[i] > order[j]
	})
	for _, i := range order {
		e := edits[i]
		text = text[:offset(text, e.Range.Start)] + e.NewText + text[offset(text, e.Range.End):]
	}
	return text
}

// offset is the byte offset of an LSP position (UTF-16) in text.
func offset(text string, at Position) int {
	start := 0
	for line := 0; line < at.Line; line++ {
		i := strings.IndexByte(text[start:], '\n')
		if i < 0 {
			return len(text)
		}
		start += i + 1
	}
	units := 0
	for i, r := range text[start:] {
		if units >= at.Character || r == '\n' {
			return start + i
		}
		units += len(utf16.Encode([]rune{r}))
	}
	return len(text)
}

// CompletionItem is one completion; Raw is the item as the server sent it.
type CompletionItem struct {
	Label               string          `json:"label"`
	Detail              string          `json:"detail"`
	AdditionalTextEdits []TextEdit      `json:"additionalTextEdits"`
	Raw                 json.RawMessage `json:"-"`
}

// Completion returns the completion items at a position.
func (c *Client) Completion(rel string, at Position) []CompletionItem {
	c.t.Helper()
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	c.Request("textDocument/completion", c.doc(rel, at), &list)
	items := make([]CompletionItem, len(list.Items))
	for i, raw := range list.Items {
		json.Unmarshal(raw, &items[i])
		items[i].Raw = raw
	}
	return items
}

// Resolve resolves a completion item, as the editor does when the item is
// selected: its documentation, detail and additional edits (an auto-import)
// arrive only then.
func (c *Client) Resolve(item CompletionItem) CompletionItem {
	c.t.Helper()
	var raw json.RawMessage
	c.Request("completionItem/resolve", item.Raw, &raw)
	resolved := CompletionItem{Raw: raw}
	json.Unmarshal(raw, &resolved)
	return resolved
}

// Rename asks for the rename of the name at a position: the edit, or the
// server's refusal. A name that cannot be renamed at all is an empty edit.
func (c *Client) Rename(rel string, at Position, newName string) (WorkspaceEdit, error) {
	c.t.Helper()
	var edit WorkspaceEdit
	params := c.doc(rel, at)
	params["newName"] = newName
	err := c.Try("textDocument/rename", params, &edit)
	return edit, err
}

// PrepareRename asks whether the name at a position can be renamed, as the
// editor does before it asks for the new name: the range and the text it
// would show, or the server's refusal.
func (c *Client) PrepareRename(rel string, at Position) (Range, string, error) {
	c.t.Helper()
	var result struct {
		Range       Range  `json:"range"`
		Placeholder string `json:"placeholder"`
	}
	err := c.Try("textDocument/prepareRename", c.doc(rel, at), &result)
	return result.Range, result.Placeholder, err
}
