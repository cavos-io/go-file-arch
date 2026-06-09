package validator

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type validatorVendors struct{}

func newValidatorVendors() *validatorVendors {
	return &validatorVendors{}
}

func (v *validatorVendors) Validate(_ spec.Document) []arch.Notice {
	return make([]arch.Notice, 0)
}
