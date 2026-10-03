package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
)

// An untitled document has no file name, so no extension — and the mapper is
// found by extension: `untitled:Untitled-1` would be parsed as plain
// TypeScript, every JSX element an error. The language id of didOpen decides
// instead (ide.md, *VS Code extension* → Client): a document that is not a
// file and is opened as `rtsx` is served under a name that ends in `.rtsx`,
// and the front translates that name in every message, both ways. Nothing is
// decoded while no such document is open.
type aliases struct {
	mu       sync.Mutex
	toServer map[string]string // the client's URI → the server's
	toClient map[string]string
}

// serverName is the URI the server knows an rtsx document by; "" when it is
// the client's own. The fork drops the `/` of `untitled:/dir/name` in its
// answers, so the alias is written the way the fork writes it back.
func serverName(uri string) string {
	scheme, rest, ok := strings.Cut(uri, ":")
	if !ok || scheme == "file" || strings.ContainsAny(rest, "?#") {
		return ""
	}
	name := uri
	if !strings.HasPrefix(rest, "//") { // no authority
		name = scheme + ":" + strings.TrimLeft(rest, "/")
	}
	if !isRTSX(name) {
		name += ".rtsx"
	}
	if name == uri {
		return ""
	}
	return name
}

// fromClient registers the alias of a document being opened and renames the
// documents of body for the server.
func (a *aliases) fromClient(body []byte) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	var closed string
	if bytes.Contains(body, []byte(`"textDocument/didOpen"`)) || bytes.Contains(body, []byte(`"textDocument/didClose"`)) {
		var msg struct {
			Method string `json:"method"`
			Params struct {
				TextDocument struct {
					URI        string `json:"uri"`
					LanguageID string `json:"languageId"`
				} `json:"textDocument"`
			} `json:"params"`
		}
		if json.Unmarshal(body, &msg) == nil {
			uri := msg.Params.TextDocument.URI
			switch msg.Method {
			case "textDocument/didOpen":
				if name := serverName(uri); name != "" && msg.Params.TextDocument.LanguageID == "rtsx" {
					if a.toServer == nil {
						a.toServer, a.toClient = map[string]string{}, map[string]string{}
					}
					a.toServer[uri], a.toClient[name] = name, uri
				}
			case "textDocument/didClose":
				closed = uri
			}
		}
	}
	body = rename(body, a.toServer)
	if name, ok := a.toServer[closed]; ok {
		delete(a.toServer, closed)
		delete(a.toClient, name)
	}
	return body
}

// fromServer renames the documents of body back for the client.
func (a *aliases) fromServer(body []byte) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return rename(body, a.toClient)
}

// rename replaces every string of the message that is a key of names — a
// value, or a key (`changes` of a workspace edit) — except a document's text.
func rename(body []byte, names map[string]string) []byte {
	found := false
	for name := range names {
		found = found || bytes.Contains(body, []byte(name))
	}
	if !found {
		return body
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber() // ids and versions as written
	var message any
	if decoder.Decode(&message) != nil {
		return body
	}
	changed := false
	var walk func(v any) any
	walk = func(v any) any {
		switch v := v.(type) {
		case string:
			if name, ok := names[v]; ok {
				changed = true
				return name
			}
		case []any:
			for i, item := range v {
				v[i] = walk(item)
			}
		case map[string]any:
			var renamed []string
			for key, value := range v {
				if _, text := value.(string); text && (key == "text" || key == "newText") {
					continue
				}
				v[key] = walk(value)
				if _, ok := names[key]; ok {
					renamed = append(renamed, key)
				}
			}
			for _, key := range renamed {
				v[names[key]], changed = v[key], true
				delete(v, key)
			}
		}
		return v
	}
	message = walk(message)
	if !changed {
		return body
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(message) != nil {
		return body
	}
	return bytes.TrimRight(out.Bytes(), "\n")
}
