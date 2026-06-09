package validator

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type Validator struct {
	pathResolver pathResolver
}

func NewValidator(
	pathResolver pathResolver,
) *Validator {
	return &Validator{
		pathResolver: pathResolver,
	}
}

func (v *Validator) Validate(doc spec.Document, projectDir string) []arch.Notice {
	notices := make([]arch.Notice, 0)

	utils := newUtils(v.pathResolver, doc, projectDir)
	validators := []validator{
		newValidatorCommonComponents(utils),
		newValidatorCommonVendors(utils),
		newValidatorComponents(utils),
		newValidatorDeps(utils),
		newValidatorDepsComponents(utils),
		newValidatorDepsVendors(utils),
		newValidatorExcludeFiles(),
		newValidatorVendors(),
		newValidatorVersion(),
		newValidatorWorkDir(utils),
	}

	for _, specValidator := range validators {
		notices = append(notices, specValidator.Validate(doc)...)
	}

	return notices
}
