// Command reactogenic is the phase 1 CLI: `reactogenic check` (RGP1-070) and
// the stdio server driven by the Vite plugin (RGP1-053).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: reactogenic <check>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		fmt.Fprintln(os.Stderr, "reactogenic check: not implemented yet (RGP1-070)")
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "reactogenic: unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}
