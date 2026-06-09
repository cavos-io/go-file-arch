package filearch

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

type Options struct {
	ConfigPath         string
	ArchLintConfigPath string
	Patterns           []string
	Workdir            string
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

	var messages []string
	if len(diagnostics) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%d architecture violation(s):", len(diagnostics))
		for _, diagnostic := range diagnostics {
			pos := fset.Position(diagnostic.Pos)
			fmt.Fprintf(&b, "\n%s: %s", pos, diagnostic.Message)
		}
		messages = append(messages, b.String())
	}

	if opts.ArchLintConfigPath != "" {
		archLintProjectPath := cfg.baseDir
		if opts.Workdir != "" {
			if abs, err := filepath.Abs(opts.Workdir); err == nil {
				archLintProjectPath = abs
			}
		}
		result, err := CheckArchLint(ctx, ArchLintOptions{
			ProjectPath: archLintProjectPath,
			ArchFile:    opts.ArchLintConfigPath,
		})
		if err != nil {
			return err
		}
		if archLintHasFindings(result) {
			messages = append(messages, formatArchLintFindings(result))
		}
	}

	if len(messages) == 0 {
		return nil
	}

	return errors.New(strings.Join(messages, "\n"))
}

func archLintHasFindings(result ArchLintCheckResult) bool {
	return result.HasWarnings ||
		len(result.DependencyWarnings) > 0 ||
		len(result.MatchWarnings) > 0 ||
		result.DeepScanWarnings > 0 ||
		len(result.DocumentNotices) > 0
}

func formatArchLintFindings(result ArchLintCheckResult) string {
	var b strings.Builder
	total := len(result.DependencyWarnings) + len(result.MatchWarnings) + result.DeepScanWarnings + len(result.DocumentNotices)
	fmt.Fprintf(&b, "%d go-arch-lint violation(s):", total)
	for _, notice := range result.DocumentNotices {
		fmt.Fprintf(&b, "\n%s:%d:%d: %s", notice.File, notice.Line, notice.Column, notice.Text)
	}
	for _, warning := range result.DependencyWarnings {
		fmt.Fprintf(
			&b,
			"\n%s:%d:%d: component %s may not import %s",
			warning.FilePath,
			warning.Line,
			warning.Column,
			warning.ComponentName,
			warning.ImportPath,
		)
	}
	for _, warning := range result.MatchWarnings {
		fmt.Fprintf(&b, "\n%s: file does not match any component", warning.FilePath)
	}
	if result.DeepScanWarnings > 0 {
		fmt.Fprintf(&b, "\n%d deep-scan warning(s)", result.DeepScanWarnings)
	}
	if result.OmittedCount > 0 {
		fmt.Fprintf(&b, "\n%d warning(s) omitted", result.OmittedCount)
	}
	return b.String()
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
