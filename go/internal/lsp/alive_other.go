//go:build !unix && !windows

package lsp

const processAliveSupported = false

func isProcessAlive(int) bool { return true }
