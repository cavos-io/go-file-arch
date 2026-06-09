package filearch

import (
	"context"
	"errors"

	"github.com/logrusorgru/aurora/v3"

	archmodels "github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/operations/check"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/checker"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/common/path"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/common/yaml/reference"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/holder"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/info"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/resolver"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/project/scanner"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/render/code"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/render/printer"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/schema"
	specassembler "github.com/cavos-io/go-file-arch/internal/archlint/services/spec/assembler"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec/decoder"
	specvalidator "github.com/cavos-io/go-file-arch/internal/archlint/services/spec/validator"
)

type ArchLintOptions struct {
	ProjectPath string
	ArchFile    string
	MaxWarnings int
}

type ArchLintCheckResult struct {
	ModuleName         string
	HasWarnings        bool
	DependencyWarnings []ArchLintDependencyWarning
	MatchWarnings      []ArchLintMatchWarning
	DeepScanWarnings   int
	DocumentNotices    []ArchLintNotice
	OmittedCount       int
}

type ArchLintDependencyWarning struct {
	ComponentName string
	FilePath      string
	ImportPath    string
	Line          int
	Column        int
}

type ArchLintMatchWarning struct {
	FilePath string
}

type ArchLintNotice struct {
	Text   string
	File   string
	Line   int
	Column int
}

func CheckArchLint(ctx context.Context, opts ArchLintOptions) (ArchLintCheckResult, error) {
	if opts.MaxWarnings == 0 {
		opts.MaxWarnings = 512
	}

	out, err := newArchLintCheckOperation().Behave(ctx, archmodels.CmdCheckIn{
		ProjectPath: opts.ProjectPath,
		ArchFile:    opts.ArchFile,
		MaxWarnings: opts.MaxWarnings,
	})
	if err != nil && !errors.Is(err, archmodels.UserSpaceError{}) {
		return ArchLintCheckResult{}, err
	}

	return convertArchLintCheckResult(out), nil
}

func newArchLintCheckOperation() *check.Operation {
	pathResolver := path.NewResolver()
	yamlDecoder := decoder.NewDecoder(reference.NewResolver(), schema.NewProvider())
	specAssembler := specassembler.NewAssembler(
		yamlDecoder,
		specvalidator.NewValidator(pathResolver),
		pathResolver,
	)

	projectFilesResolver := resolver.NewResolver(
		scanner.NewScanner(),
		holder.NewHolder(),
	)
	referenceRender := code.NewRender(
		printer.NewColorPrinter(aurora.NewAurora(false)),
	)

	return check.NewOperation(
		info.NewAssembler(),
		specAssembler,
		checker.NewCompositeChecker(
			checker.NewImport(projectFilesResolver),
			checker.NewDeepScan(projectFilesResolver, referenceRender),
		),
		referenceRender,
		false,
	)
}

func convertArchLintCheckResult(out archmodels.CmdCheckOut) ArchLintCheckResult {
	result := ArchLintCheckResult{
		ModuleName:       out.ModuleName,
		HasWarnings:      out.ArchHasWarnings,
		DeepScanWarnings: len(out.ArchWarningsDeepScan),
		OmittedCount:     out.OmittedCount,
	}

	for _, warning := range out.ArchWarningsDependency {
		result.DependencyWarnings = append(result.DependencyWarnings, ArchLintDependencyWarning{
			ComponentName: warning.ComponentName,
			FilePath:      warning.FileRelativePath,
			ImportPath:    warning.ResolvedImportName,
			Line:          warning.Reference.Line,
			Column:        warning.Reference.Column,
		})
	}
	for _, warning := range out.ArchWarningsMatch {
		result.MatchWarnings = append(result.MatchWarnings, ArchLintMatchWarning{
			FilePath: warning.FileRelativePath,
		})
	}
	for _, notice := range out.DocumentNotices {
		result.DocumentNotices = append(result.DocumentNotices, ArchLintNotice{
			Text:   notice.Text,
			File:   notice.File,
			Line:   notice.Line,
			Column: notice.Column,
		})
	}

	return result
}
