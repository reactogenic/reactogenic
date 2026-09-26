// Command reactogenic is the phase 1 CLI: `reactogenic check` (RGP1-070) and
// the stdio server driven by the Vite plugin (RGP1-053).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reactogenic/reactogenic/go/internal/check"
)

const usage = `usage: reactogenic check [-p tsconfig.json|dir] [--pretty=false]

  check   type-check the project, with .rtsx transpiled; errors are reported
          on the .rtsx files (exit status 1 when there are errors)`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		os.Exit(runCheck(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "reactogenic: unknown command %q\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
}

func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	projectFlag := flags.String("p", "", "tsconfig.json, or a directory holding one")
	pretty := flags.Bool("pretty", true, "code frames, and `file:line:col` positions")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	config := filepath.Join(cwd, "tsconfig.json")
	if *projectFlag != "" {
		config, _ = filepath.Abs(*projectFlag)
		if info, err := os.Stat(config); err == nil && info.IsDir() {
			config = filepath.Join(config, "tsconfig.json")
		}
	}
	if _, err := os.Stat(config); err != nil {
		fmt.Fprintf(os.Stderr, "reactogenic check: no tsconfig at %s\n", config)
		return 2
	}
	reports := check.Run(filepath.ToSlash(config))
	check.Print(os.Stdout, reports, filepath.ToSlash(cwd), *pretty, func(p string) (string, bool) {
		b, err := os.ReadFile(p)
		return string(b), err == nil
	})
	if check.Errors(reports) > 0 {
		return 1
	}
	return 0
}
