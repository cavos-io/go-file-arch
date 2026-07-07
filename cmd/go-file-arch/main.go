package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/cavos-io/go-file-arch/filearch"
)

const usage = `go-file-arch — enforce file-level architecture rules for Go from a YAML config.

Usage:
  go-file-arch [flags] [packages]

Flags:
  -c, --config <path>            path to go-file-arch YAML config (required)
  -a, --arch-lint-config <path>  optional go-arch-lint YAML config to run alongside
  -h, --help                     show this help

Arguments:
  [packages]  Go package patterns to check (default "./...")

Examples:
  go-file-arch -c .go-file-arch.yml ./...
  go-file-arch --config .go-file-arch.yml --arch-lint-config .go-arch-lint.yml ./...

Exit codes:
  0  no violations
  1  violations found, or an error occurred
`

func main() {
	var configPath string
	var archLintConfigPath string

	flag.StringVar(&configPath, "config", "", "path to go-file-arch YAML config")
	flag.StringVar(&configPath, "c", "", "shorthand for --config")
	flag.StringVar(&archLintConfigPath, "arch-lint-config", "", "optional go-arch-lint YAML config to run with content and file-name rules")
	flag.StringVar(&archLintConfigPath, "a", "", "shorthand for --arch-lint-config")

	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	if err := filearch.Run(context.Background(), filearch.Options{
		ConfigPath:         configPath,
		ArchLintConfigPath: archLintConfigPath,
		Patterns:           flag.Args(),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
