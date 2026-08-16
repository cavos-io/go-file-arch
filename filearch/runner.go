package filearch

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

var defaultConfigNames = []string{".go-file-arch.yml", ".go-file-arch.yaml"}

type Options struct {
	ConfigPath         string
	ArchLintConfigPath string
	Patterns           []string
	Workdir            string
}

type ViolationError struct {
	Message string
}

func (err *ViolationError) Error() string { return err.Message }

func Run(ctx context.Context, opts Options) error {
	if opts.ConfigPath == "" {
		discovered, err := discoverConfigPath(opts.Workdir)
		if err != nil {
			return err
		}
		opts.ConfigPath = discovered
	}

	cfg, err := loadConfig(opts.ConfigPath, opts.Workdir)
	if err != nil {
		return err
	}
	inventory, err := newRepositoryInventory(cfg.workdirAbs)
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
		Dir:     cfg.workdirAbs,
		Fset:    fset,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
		Tests: true,
	}, patterns...)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
	}
	for _, pkg := range pkgs {
		for _, packageErr := range pkg.Errors {
			if packageErr.Kind == packages.ParseError {
				return fmt.Errorf("parse %s", packageErr.Error())
			}
		}
	}
	if packages.PrintErrors(pkgs) > 0 {
		return errors.New("package loading failed")
	}

	diagnostics := checkDirectoryRules(cfg, inventory)
	diagnostics = append(diagnostics, checkDirectoryNameRules(cfg, inventory)...)
	diagnostics = append(diagnostics, checkPathRules(cfg, inventory)...)
	seenFiles := make(map[string]bool)
	for _, pkg := range pkgs {
		var files []*ast.File
		for i, filename := range pkg.CompiledGoFiles {
			if !fileBelongsToWorkdir(filename, cfg) {
				continue
			}
			if seenFiles[filename] {
				continue
			}
			seenFiles[filename] = true
			files = append(files, pkg.Syntax[i])
		}
		pass := &analysis.Pass{
			Analyzer:   Analyzer,
			Fset:       fset,
			Files:      files,
			Pkg:        pkg.Types,
			TypesInfo:  pkg.TypesInfo,
			TypesSizes: pkg.TypesSizes,
			Report: func(d analysis.Diagnostic) {
				pos := fset.Position(d.Pos)
				diagnostics = append(diagnostics, ruleDiagnostic{
					Path:    diagnosticPath(pos.Filename, cfg),
					Line:    pos.Line,
					Column:  pos.Column,
					Message: d.Message,
				})
			},
		}
		if _, err := runWithConfig(pass, cfg, inventory); err != nil {
			return err
		}
	}

	var messages []string
	if len(diagnostics) > 0 {
		sortRuleDiagnostics(diagnostics)
		var b strings.Builder
		fmt.Fprintf(&b, "%d architecture violation(s):", len(diagnostics))
		for _, diagnostic := range diagnostics {
			message := diagnostic.Message
			if diagnostic.RuleID != "" {
				message = fmt.Sprintf("[%s]: %s", diagnostic.RuleID, message)
			}
			fmt.Fprintf(&b, "\n%s:%d:%d: %s", diagnostic.Path, diagnostic.Line, diagnostic.Column, message)
		}
		messages = append(messages, b.String())
	}

	if opts.ArchLintConfigPath != "" {
		result, err := CheckArchLint(ctx, ArchLintOptions{
			ProjectPath: cfg.workdirAbs,
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

	return &ViolationError{Message: strings.Join(messages, "\n")}
}

func fileBelongsToWorkdir(filename string, cfg *Config) bool {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(cfg.workdirAbs, filename)
	}
	relative, err := filepath.Rel(cfg.workdirAbs, filename)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func diagnosticPath(filename string, cfg *Config) string {
	if rel, err := filepath.Rel(cfg.workdirAbs, filename); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filename)
}

func sortRuleDiagnostics(diagnostics []ruleDiagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Message < b.Message
	})
}

func discoverConfigPath(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	for _, name := range defaultConfigNames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("config not found: set -c/--config or add %s to %q", strings.Join(defaultConfigNames, " or "), dir)
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
