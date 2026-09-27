package server

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	requests := strings.Join([]string{
		`{"id":1,"method":"transform","params":{"file":"/app/page.rtsx","code":"const value = 1;\nexport const a = <Input value />;\n"}}`,
		`{"id":2,"method":"transform","params":{"file":"/app/bad.rtsx","code":"export const a = <div { size }>x</div>;\n"}}`,
		`{"id":3,"method":"nope"}`,
		`not json`,
		`{"id":4,"method":"close"}`,
		`{"id":5,"method":"transform","params":{"file":"/app/late.rtsx","code":""}}`,
	}, "\n")
	var out strings.Builder
	if err := Serve(strings.NewReader(requests), &out); err != nil {
		t.Fatal(err)
	}
	var responses []map[string]any
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		var r map[string]any
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, r)
	}
	if len(responses) != 5 { // 1, 2, 3, the bad line, close — nothing after close
		t.Fatalf("want 5 responses, got %d: %s", len(responses), out.String())
	}

	r1 := responses[0]["result"].(map[string]any)
	if !strings.Contains(r1["code"].(string), "<Input value={value} />") {
		t.Errorf("transform: %v", r1["code"])
	}
	var sm struct {
		Version int
		Sources []string
	}
	if err := json.Unmarshal([]byte(r1["map"].(string)), &sm); err != nil || sm.Version != 3 || sm.Sources[0] != "/app/page.rtsx" {
		t.Errorf("map: %v %+v", err, sm)
	}
	if d := r1["diagnostics"].([]any); len(d) != 0 {
		t.Errorf("diagnostics: %v", d)
	}

	d2 := responses[1]["result"].(map[string]any)["diagnostics"].([]any)
	if len(d2) != 1 || d2[0].(map[string]any)["code"] != "params-on-html" || d2[0].(map[string]any)["line"] != 1.0 {
		t.Errorf("diagnostics: %v", d2)
	}
	if responses[2]["error"] != `unknown method "nope"` || !strings.HasPrefix(responses[3]["error"].(string), "bad request") {
		t.Errorf("errors: %v %v", responses[2], responses[3])
	}
	if responses[4]["id"] != 4.0 {
		t.Errorf("close: %v", responses[4])
	}
}
