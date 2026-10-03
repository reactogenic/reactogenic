package lsp_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/lsptest"
)

// The real thing, once: `reactogenic lsp --stdio` as a process — flags,
// framing on real pipes, and its exit status (ide.md, *Other editors*).
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

	// process runs the binary as a server. Its stdin is a pipe that stays
	// open after the process is gone, as an editor's does: the process must
	// end by itself.
	var mu sync.Mutex
	stderr := ""
	process := func(args ...string) lsptest.Serve {
		return func(ctx context.Context, in io.Reader, out, log io.Writer, cwd string) error {
			cmd := exec.Command(bin, append([]string{"lsp", "--stdio"}, args...)...)
			var said strings.Builder
			cmd.Dir, cmd.Stdout, cmd.Stderr = cwd, out, &said
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			go func() {
				io.Copy(stdin, in)
				stdin.Close()
			}()
			err = cmd.Run()
			mu.Lock()
			stderr = said.String()
			mu.Unlock()
			return err
		}
	}
	status := func(err error) int {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		if err != nil {
			t.Errorf("the server ended with %v", err)
		}
		return 0
	}
	lastLine := func() string {
		mu.Lock()
		defer mu.Unlock()
		lines := strings.Split(strings.TrimSpace(stderr), "\n")
		return lines[len(lines)-1]
	}

	// Shutdown, exit: status 0 (the client fails the test otherwise). The
	// editor's process id on the command line is accepted, as upstream's.
	t.Run("session", func(t *testing.T) {
		c := lsptest.Start(t, lsptest.Project(t, app), process("--clientProcessId", strconv.Itoa(os.Getpid())))
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
	root := lsptest.Project(t, app)
	t.Run("exit without shutdown", func(t *testing.T) {
		c := lsptest.Connect(t, root, process())
		initialize(c, root)
		c.Send(0, "exit", nil)
		if code := status(c.Ended(10 * time.Second)); code != 1 || lastLine() != "reactogenic lsp: exit without shutdown" {
			t.Errorf("exit status %d, stderr %q", code, lastLine())
		}
	})
	t.Run("input that is not LSP", func(t *testing.T) {
		c := lsptest.Connect(t, root, process())
		c.Write("GET / HTTP/1.1\r\n\r\n")
		if code := status(c.Ended(10 * time.Second)); code != 1 || lastLine() != "reactogenic lsp: a message header without Content-Length" {
			t.Errorf("exit status %d, stderr %q", code, lastLine())
		}
	})
	t.Run("the input ends", func(t *testing.T) {
		c := lsptest.Connect(t, root, process())
		initialize(c, root)
		c.Close()
		if code := status(c.Ended(10 * time.Second)); code != 0 {
			t.Errorf("exit status %d, stderr %q", code, lastLine())
		}
	})
}
