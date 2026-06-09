package assembler

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type (
	archDecoder interface {
		Decode(archFile string) (spec.Document, []arch.Notice, error)
	}

	archValidator interface {
		Validate(doc spec.Document, projectDir string) []arch.Notice
	}

	pathResolver interface {
		Resolve(absPath string) (resolvePaths []string, err error)
	}
)
