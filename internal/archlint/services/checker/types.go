package checker

import (
	"context"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

type (
	projectFilesResolver interface {
		ProjectFiles(ctx context.Context, spec arch.Spec) ([]models.FileHold, error)
	}

	checker interface {
		Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error)
	}

	sourceCodeRenderer interface {
		SourceCode(ref common.Reference, highlight bool, showPointer bool) []byte
	}
)
