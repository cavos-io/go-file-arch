package graph

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

type (
	specAssembler interface {
		Assemble(prj common.Project) (arch.Spec, error)
	}

	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (common.Project, error)
	}
)
