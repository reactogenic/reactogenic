//go:build !race

package lsp_test

// raceDetector: the tests are built with -race, about ten times slower.
const raceDetector = false
