package resolver

import (
	"context"
	"regexp"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
)

type (
	projectFilesResolver interface {
		Scan(
			ctx context.Context,
			projectDirectory string,
			moduleName string,
			excludePaths []models.ResolvedPath,
			excludeFileMatchers []*regexp.Regexp,
		) ([]models.ProjectFile, error)
	}

	projectFilesHolder interface {
		HoldProjectFiles(files []models.ProjectFile, components []arch.Component) []models.FileHold
	}
)
