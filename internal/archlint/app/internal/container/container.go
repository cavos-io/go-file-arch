package container

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models"
)

type Container struct {
	version    string
	buildTime  string
	commitHash string

	flags models.FlagsRoot
}

func NewContainer(
	version string,
	buildTime string,
	commitHash string,
) *Container {
	return &Container{
		version:    version,
		buildTime:  buildTime,
		commitHash: commitHash,
	}
}
