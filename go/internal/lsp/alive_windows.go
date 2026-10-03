//go:build windows

package lsp

import "syscall"

const processAliveSupported = true

// isProcessAlive waits on the process for no time at all (as upstream's
// cmd/tsc/isprocessalive_windows.go): a timeout means it still runs.
func isProcessAlive(pid int) bool {
	const synchronize = 0x00100000
	handle, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(handle) }()
	event, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	const waitTimeout = 258
	return event == waitTimeout
}
