package check

import (
	"context"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

type (
	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (common.Project, error)
	}

	specAssembler interface {
		Assemble(prj common.Project) (arch.Spec, error)
	}

	referenceRender interface {
		SourceCode(ref common.Reference, highlight bool, showPointer bool) []byte
	}

	specChecker interface {
		Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error)
	}
)
