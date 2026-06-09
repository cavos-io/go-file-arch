package filearch

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

type Options struct {
	ConfigPath string
	Patterns   []string
	Workdir    string
}

func Run(ctx context.Context, opts Options) error {
	if opts.ConfigPath == "" {
		return errors.New("missing config path")
	}

	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     opts.Workdir,
		Fset:    fset,
		Mode:    packages.NeedName | packages.NeedFiles,
	}, patterns...)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return errors.New("package loading failed")
	}

	var diagnostics []analysis.Diagnostic
	for _, pkg := range pkgs {
		files, err := parsePackageFiles(fset, pkg.GoFiles)
		if err != nil {
			return err
		}
		pass := &analysis.Pass{
			Analyzer: Analyzer,
			Fset:     fset,
			Files:    files,
			Report: func(d analysis.Diagnostic) {
				diagnostics = append(diagnostics, d)
			},
		}
		if _, err := runWithConfig(pass, cfg); err != nil {
			return err
		}
	}

	if len(diagnostics) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d architecture violation(s):", len(diagnostics))
	for _, diagnostic := range diagnostics {
		pos := fset.Position(diagnostic.Pos)
		fmt.Fprintf(&b, "\n%s: %s", pos, diagnostic.Message)
	}
	return errors.New(b.String())
}

func parsePackageFiles(fset *token.FileSet, filenames []string) ([]*ast.File, error) {
	files := make([]*ast.File, 0, len(filenames))
	for _, filename := range filenames {
		file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filename, err)
		}
		files = append(files, file)
	}
	return files, nil
}
