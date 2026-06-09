package assembler

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type workdirAssembler struct{}

func newWorkdirAssembler() *workdirAssembler {
	return &workdirAssembler{}
}

func (efa *workdirAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	spec.WorkingDirectory = document.WorkingDirectory()

	return nil
}
