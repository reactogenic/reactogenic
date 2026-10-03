package stockmapper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The real thing, once: `reactogenic content-mapper` as a process, on the
// disk — its stdout is the protocol and nothing else, and it exits when its
// input ends. (Against the real host: scripts/e2e-stock-mapper.sh.)
func TestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "reactogenic")
	build := exec.Command("go", "build", "-o", bin, "github.com/reactogenic/reactogenic/go/cmd/reactogenic")
	build.Env = append(os.Environ(), "GOTELEMETRY=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.ToSlash(dir)
	for name, text := range map[string]string{"button.rtsx": buttonSource, "page.rtsx": pageSource} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var input bytes.Buffer
	for i, request := range []map[string]any{
		{"method": "initialize", "params": map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}}},
		{"method": "openProject", "params": map[string]any{"configFileName": dir + "/tsconfig.json", "projectHandle": "p:0", "compilerOptions": map[string]any{}}},
		{"method": "transform", "params": transformParams{FileName: dir + "/page.rtsx", Content: pageSource, ProjectHandle: "p:0"}},
		{"method": "closeProject", "params": map[string]any{"projectHandle": "p:0"}},
	} {
		request["jsonrpc"], request["id"] = "2.0", fmt.Sprintf("api%d", i+1)
		body, _ := json.Marshal(request)
		fmt.Fprintf(&input, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	cmd := exec.Command(bin, "content-mapper")
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = &input, &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the mapper exited with %v\n%s", err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Errorf("stderr: %s", stderr.String())
	}

	// Four framed responses, and not a byte more.
	reader, answers := bufio.NewReader(&stdout), map[string]json.RawMessage{}
	for {
		body, err := readMessage(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stdout is not the protocol: %v", err)
		}
		var r response
		if err := json.Unmarshal(body, &r); err != nil || r.Error != nil {
			t.Fatalf("response %s: %v", body, err)
		}
		answers[string(r.ID)] = r.Result
	}
	if len(answers) != 4 || string(answers[`"api2"`]) != "{}" || string(answers[`"api4"`]) != "null" {
		t.Fatalf("answers: %s", answers)
	}
	result := decode(t, answers[`"api3"`])
	spanMap(t, result, pageSource)
	// button.rtsx is on the disk, next to the file.
	if !strings.Contains(result.Text, `from "./button.rtsx";`) || !strings.Contains(result.Text, "size={size}") {
		t.Errorf("virtual text:\n%s", result.Text)
	}
}
