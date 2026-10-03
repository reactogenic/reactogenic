package lsp_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// The real thing, once: `reactogenic lsp --stdio` as a process — flags,
// framing on real pipes, and a clean exit when the client goes away.
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
	if out, err := exec.Command(bin, "--version").Output(); err != nil || strings.TrimSpace(string(out)) != "0.0.0-dev" {
		t.Errorf("--version: %q, %v", out, err)
	}

	exited := make(chan error, 1)
	process := func(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
		cmd := exec.Command(bin, "lsp", "--stdio")
		var stderr strings.Builder
		cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = cwd, in, out, &stderr
		err := cmd.Run()
		if err != nil {
			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			t.Logf("stderr, last lines:\n%s", strings.Join(lines[max(0, len(lines)-8):], "\n"))
		}
		exited <- err
		return err
	}
	t.Run("session", func(t *testing.T) {
		c := lsptest.Start(t, lsptest.Project(t, app), process)
		if c.Initialized.ServerInfo.Name != "reactogenic" || c.Initialized.ServerInfo.Version != "0.0.0-dev" {
			t.Errorf("server info: %+v", c.Initialized.ServerInfo)
		}
		c.Open("src/page.rtsx")
		if hover := c.Hover("src/page.rtsx", c.At("src/page.rtsx", "<$Label", 1, 3)); !strings.Contains(hover, "The label.") {
			t.Errorf("hover: %q", hover)
		}
		if got := lsptest.Lines(c.Diagnostics("src/page.rtsx")); len(got) != 0 {
			t.Errorf("diagnostics: %q", got)
		}
	})
	// The session's cleanup sent shutdown and exit and closed stdin.
	if err := <-exited; err != nil {
		t.Errorf("the server exited with %v", err)
	}
}
