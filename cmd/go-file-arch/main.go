package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/cavos-io/go-file-arch/filearch"
)

func main() {
	var configPath string
	var archLintConfigPath string
	flag.StringVar(&configPath, "config", "", "path to go-file-arch YAML config")
	flag.StringVar(&archLintConfigPath, "arch-lint-config", "", "optional go-arch-lint YAML config to run with content and file-name rules")
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
