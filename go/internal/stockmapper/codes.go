package stockmapper

import (
	"strconv"
	"strings"
)

// codes numbers the transpiler's diagnostics: the content-mapper contract
// wants an integer where `reactogenic check` prints a name. The name leads
// the message instead: `error reactogenic101: orphan-slot: …`.
//
// The numbers are stable — append, never renumber: they are what a user
// searches for and what a baseline of a build's errors records. All are
// below 1000, where TypeScript has no codes, so a syntax error of the source
// parse keeps TypeScript's own number (`reactogenic1005: ';' expected.`)
// without meeting one of ours.
var codes = map[string]int32{
	// The transpiler's own failure — a pass that failed, a recovered panic.
	"internal": 1,

	// Slots.
	"orphan-slot":            101,
	"arg-without-slot":       102,
	"params-on-html":         103,
	"duplicate-params":       104,
	"component-name":         105,
	"slot-children-conflict": 106,
	"keyed-slot-mixed":       107,
	"mixed-conditional-slot": 108,

	// Flow control.
	"flow-as-value":             201,
	"flow-attribute":            202,
	"flow-no-subject":           203,
	"case-no-test":              204,
	"case-both":                 205,
	"case-params":               206,
	"case-default-value":        207,
	"case-default-not-last":     208,
	"switch-children":           209,
	"switch-exhaustive-default": 210,
	"switch-dynamic-exhaustive": 211,

	// Segment roots.
	"segment-syntax":    301,
	"segment-id":        302,
	"segment-duplicate": 303,
	"segment-in-loop":   304,
	"segment-children":  305, // a warning: not sent (the contract has errors only)
	"segment-not-found": 306,
	"segment-self":      307,

	// Modules.
	"ambiguous-module": 401,
}

// codeUnknown is a transpiler diagnostic the table does not list yet; a test
// keeps the table complete.
const codeUnknown = 999

// numericCode returns the number of a transpiler diagnostic's code, and
// whether its message should carry the name: a syntax error (`TS1005`) is
// TypeScript's and keeps its number bare.
func numericCode(code string) (number int32, named bool) {
	if n, ok := codes[code]; ok {
		return n, true
	}
	if digits, ok := strings.CutPrefix(code, "TS"); ok {
		if n, err := strconv.ParseInt(digits, 10, 32); err == nil && n >= 1000 {
			return int32(n), false
		}
	}
	return codeUnknown, true
}
