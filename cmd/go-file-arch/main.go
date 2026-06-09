package main

import (
	"github.com/cavos-io/go-file-arch/filearch"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(filearch.Analyzer)
}
