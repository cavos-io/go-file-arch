package decoder

import "github.com/cavos-io/go-file-arch/internal/archlint/services/spec"

type doc interface {
	spec.Document

	postSetup()
}
