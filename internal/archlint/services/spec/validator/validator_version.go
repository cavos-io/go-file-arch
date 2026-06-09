package validator

import (
	"fmt"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type validatorVersion struct{}

func newValidatorVersion() *validatorVersion {
	return &validatorVersion{}
}

func (v *validatorVersion) Validate(doc spec.Document) []arch.Notice {
	notices := make([]arch.Notice, 0)

	if doc.Version().Value <= models.SupportedVersionMax && doc.Version().Value >= models.SupportedVersionMin {
		return notices
	}

	notices = append(notices, arch.Notice{
		Notice: fmt.Errorf("version '%d' is not supported, supported: [%d-%d]",
			doc.Version().Value,
			models.SupportedVersionMin,
			models.SupportedVersionMax,
		),
		Ref: doc.Version().Reference,
	})

	return notices
}
