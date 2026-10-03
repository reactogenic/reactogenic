package lsp

import (
	"context"
	"fmt"
	"time"
)

// watchdogInterval is how often the client's process is probed (upstream's
// cmd/tsc/lsp.go: 5 s).
var watchdogInterval = 5 * time.Second

// parentWatchdog ends the server when the editor's process is gone: a
// server whose pipe ends are shared with another process does not see its
// input end when the editor is killed. As upstream's: with a process id
// given on the command line the watch starts at once; otherwise the result
// is the callback that takes the processId of initialize. nil where a
// process cannot be probed. The reason it stops with is Serve's result.
func parentWatchdog(ctx context.Context, stop context.CancelCauseFunc, clientProcessID int) func(pid int) {
	if !processAliveSupported {
		return nil
	}
	watch := func(pid int) {
		if pid <= 0 {
			return
		}
		go func() {
			ticker := time.NewTicker(watchdogInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !isProcessAlive(pid) {
						stop(fmt.Errorf("the client process %d has exited", pid))
						return
					}
				}
			}
		}()
	}
	if clientProcessID > 0 {
		watch(clientProcessID)
		return nil
	}
	return watch
}
