// Package server is the Go side of the Vite plugin (RGP1-053): newline-
// delimited JSON requests on one stream, one JSON response per request on
// the other (specs/phase01/decisions.md, RGP1-001, *Node ↔ Go*).
//
// The transpiler is purely syntactic, so the server keeps no program and no
// state between requests: `transform` reads one file and the segment files
// next to it.
//
// `virtual` (virtual.go) is for a caller that is not a build: the TS server
// plugin (specs/phase01/ide.md, *The `.ts` side*), which runs `serve`
// synchronously, one process per batch of files.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/reactogenic/reactogenic/go/internal/transpiler"
)

// Request is one call: {"id": 1, "method": "transform", "params": {...}}.
type Request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response answers the request with the same id: a result, or an error for
// a failure of the server itself (errors in the source are diagnostics).
type Response struct {
	ID     int    `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// TransformParams: the .rtsx file (absolute path) and its current text.
type TransformParams struct {
	File string `json:"file"`
	Code string `json:"code"`
}

// TransformResult: the TSX, its source map (v3, as JSON) back to the .rtsx,
// and the transpiler's diagnostics.
type TransformResult struct {
	Code        string       `json:"code"`
	Map         string       `json:"map"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Line     int    `json:"line"`
	Col      int    `json:"column"`
	Severity string `json:"severity"` // "error" | "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// Serve answers requests from r on w until r ends or a `close` request.
func Serve(r io.Reader, w io.Writer) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1<<20), 1<<30) // one file per line
	out := json.NewEncoder(w)
	for in.Scan() {
		var req Request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			if err := out.Encode(Response{Error: "bad request: " + err.Error()}); err != nil {
				return err
			}
			continue
		}
		resp := Response{ID: req.ID}
		switch req.Method {
		case "transform":
			var p TransformParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				resp.Error = "bad params: " + err.Error()
				break
			}
			result, err := transform(p)
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case "virtual":
			var p VirtualParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				resp.Error = "bad params: " + err.Error()
				break
			}
			resp.Result = virtualAll(p)
		case "close":
			return out.Encode(resp)
		default:
			resp.Error = fmt.Sprintf("unknown method %q", req.Method)
		}
		if err := out.Encode(resp); err != nil {
			return err
		}
	}
	return in.Err()
}

func transform(p TransformParams) (*TransformResult, error) {
	out, err := transpiler.Transpile(transpiler.Input{
		Files: map[string]string{p.File: p.Code},
		Entry: p.File,
		ReadFile: func(path string) (string, bool) {
			b, err := os.ReadFile(path)
			return string(b), err == nil
		},
	})
	if err != nil {
		return nil, err
	}
	result := &TransformResult{Code: out.TSX, Diagnostics: []Diagnostic{}}
	for _, d := range out.Diagnostics {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: d.Line, Col: d.Col, Severity: d.Severity.String(), Code: d.Code, Message: d.Message})
	}
	if out.Map != nil {
		m, err := out.Map.SourceMapV3(p.File+".tsx", p.File, p.Code, out.TSX)
		if err != nil {
			return nil, err
		}
		result.Map = string(m)
	}
	return result, nil
}
