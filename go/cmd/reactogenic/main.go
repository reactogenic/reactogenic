// Command reactogenic is the phase 1 CLI: `reactogenic check` (RGP1-070),
// the language server (RGP1-103), the stdio server driven by the Vite
// plugin (RGP1-053) and the content mapper for stock TypeScript 7.1
// (RGP1-112).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/reactogenic/reactogenic/go/internal/check"
	"github.com/reactogenic/reactogenic/go/internal/lsp"
	"github.com/reactogenic/reactogenic/go/internal/server"
	"github.com/reactogenic/reactogenic/go/internal/stockmapper"
)

// version is stamped by the release build (scripts/build-binaries.sh).
var version = "0.0.0-dev"

const usage = `usage: reactogenic check [-p tsconfig.json|dir] [--pretty=false] [--watch]
       reactogenic lsp --stdio
       reactogenic serve
       reactogenic content-mapper
       reactogenic --version

  check   type-check the project, with .rtsx transpiled; errors are reported
          on the .rtsx files (exit status 1 when there are errors)
  lsp     the language server for editors (LSP over stdio)
  serve   transform .rtsx for the Vite plugin: JSON requests on stdin, one
          response per line on stdout
  content-mapper
          experimental: .rtsx for stock TypeScript 7.1. 'tsc --runExternalCode'
          and its language server start it for a tsconfig that lists
          @reactogenic/cli in "contentMappers"; it speaks the content-mapper
          protocol on stdin and stdout`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		os.Exit(runCheck(os.Args[2:]))
	case "serve":
		if err := server.Serve(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "reactogenic serve:", err)
			os.Exit(1)
		}
	case "lsp":
		os.Exit(runLSP(os.Args[2:]))
	case "content-mapper":
		// stdout is the protocol's: everything else goes to stderr, which
		// the host shows under TS_CONTENT_MAPPER_DEBUG.
		if err := stockmapper.Serve(os.Stdin, os.Stdout, stockmapper.Options{Log: os.Stderr}); err != nil {
			fmt.Fprintln(os.Stderr, "reactogenic content-mapper:", err)
			os.Exit(1)
		}
	case "-v", "--version", "version":
		fmt.Println(version)
	case "-h", "--help", "help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "reactogenic: unknown command %q\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
}

func runLSP(args []string) int {
	flags := flag.NewFlagSet("lsp", flag.ContinueOnError)
	stdio := flags.Bool("stdio", false, "speak LSP on stdin and stdout")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*stdio {
		fmt.Fprintln(os.Stderr, "reactogenic lsp: only --stdio is supported")
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := lsp.Serve(context.Background(), os.Stdin, os.Stdout, os.Stderr, filepath.ToSlash(cwd), version); err != nil {
		fmt.Fprintln(os.Stderr, "reactogenic lsp:", err)
		return 1
	}
	return 0
}

func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	projectFlag := flags.String("p", "", "tsconfig.json, or a directory holding one")
	pretty := flags.Bool("pretty", true, "code frames, and `file:line:col` positions")
	watch := flags.Bool("watch", false, "check again whenever a source file changes")
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
	readFile := func(p string) (string, bool) {
		b, err := os.ReadFile(p)
		return string(b), err == nil
	}
	if *watch {
		check.Watch(filepath.ToSlash(config), 300*time.Millisecond, nil, func(reports []check.Report) {
			fmt.Printf("\n[%s] %d error(s). Watching for file changes.\n", time.Now().Format("15:04:05"), check.Errors(reports))
			check.Print(os.Stdout, reports, filepath.ToSlash(cwd), *pretty, readFile)
		})
		return 0
	}
	reports := check.Run(filepath.ToSlash(config))
	check.Print(os.Stdout, reports, filepath.ToSlash(cwd), *pretty, readFile)
	if check.Errors(reports) > 0 {
		return 1
	}
	return 0
}
