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
	flag.StringVar(&configPath, "config", "", "path to go-file-arch YAML config")
	flag.Parse()

	if err := filearch.Run(context.Background(), filearch.Options{
		ConfigPath: configPath,
		Patterns:   flag.Args(),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
