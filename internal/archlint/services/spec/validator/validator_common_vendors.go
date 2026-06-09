package validator

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type validatorCommonVendors struct {
	utils *utils
}

func newValidatorCommonVendors(
	utils *utils,
) *validatorCommonVendors {
	return &validatorCommonVendors{
		utils: utils,
	}
}

func (v *validatorCommonVendors) Validate(doc spec.Document) []arch.Notice {
	notices := make([]arch.Notice, 0)

	for _, vendorName := range doc.CommonVendors() {
		if err := v.utils.assertKnownVendor(vendorName.Value); err != nil {
			notices = append(notices, arch.Notice{
				Notice: err,
				Ref:    vendorName.Reference,
			})
		}
	}

	return notices
}
