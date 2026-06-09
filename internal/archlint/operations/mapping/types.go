package mapping

import (
	"context"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

type (
	specAssembler interface {
		Assemble(prj common.Project) (arch.Spec, error)
	}

	projectFilesResolver interface {
		ProjectFiles(ctx context.Context, spec arch.Spec) ([]models.FileHold, error)
	}

	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (common.Project, error)
	}
)
