package decoder

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

type (
	yamlSourceCodeReferenceResolver interface {
		Resolve(filePath string, yamlPath string) common.Reference
	}

	jsonSchemaProvider interface {
		Provide(version int) ([]byte, error)
	}
)
