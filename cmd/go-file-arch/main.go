package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cavos-io/go-file-arch/filearch"
)

const usage = `go-file-arch — enforce file-level architecture rules for Go from a YAML config.

Usage:
  go-file-arch [flags] [packages]

Flags:
  -c, --config <path>            path to YAML config (default: discover in workdir)
  -a, --arch-lint-config <path>  optional go-arch-lint YAML config to run alongside
  -C, --workdir <path>           repository directory to analyze
  -h, --help                     show this help

Arguments:
  [packages]  Go package patterns to check (default "./...")

Examples:
  go-file-arch -c .go-file-arch.yml ./...
  go-file-arch -c ../policy.yml -C ./service ./...
  go-file-arch --config .go-file-arch.yml --arch-lint-config .go-arch-lint.yml ./...

Exit codes:
  0  no violations
  1  architecture violations found
  2  invalid configuration or execution failure
`

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	var configPath string
	var archLintConfigPath string
	var workdir string

	flags := flag.NewFlagSet("go-file-arch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&configPath, "config", "", "path to go-file-arch YAML config")
	flags.StringVar(&configPath, "c", "", "shorthand for --config")
	flags.StringVar(&archLintConfigPath, "arch-lint-config", "", "optional go-arch-lint YAML config to run with content and file-name rules")
	flags.StringVar(&archLintConfigPath, "a", "", "shorthand for --arch-lint-config")
	flags.StringVar(&workdir, "workdir", "", "repository directory to analyze")
	flags.StringVar(&workdir, "C", "", "shorthand for --workdir")
	flags.Usage = func() { fmt.Fprint(stdout, usage) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	err := filearch.Run(context.Background(), filearch.Options{
		ConfigPath:         configPath,
		ArchLintConfigPath: archLintConfigPath,
		Workdir:            workdir,
		Patterns:           flags.Args(),
	})
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, err)
	var violation *filearch.ViolationError
	if errors.As(err, &violation) {
		return 1
	}
	return 2
}
