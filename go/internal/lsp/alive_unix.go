//go:build unix

package lsp

import (
	"errors"
	"os"
	"syscall"
)

const processAliveSupported = true

// isProcessAlive probes the process with signal 0 (as upstream's
// cmd/tsc/isprocessalive_unix.go): nil or EPERM — it exists, though we may
// not signal it; anything else, it is gone.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
