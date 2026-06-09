package assembler

import (
	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type allowedVendorImportsAssembler struct{}

func newAllowedVendorImportsAssembler() *allowedVendorImportsAssembler {
	return &allowedVendorImportsAssembler{}
}

func (aia *allowedVendorImportsAssembler) assemble(
	yamlDocument spec.Document,
	vendorNames []string,
) ([]common.Referable[models.Glob], error) {
	list := make([]common.Referable[models.Glob], 0)

	allowedVendors := make([]string, 0)
	allowedVendors = append(allowedVendors, vendorNames...)
	for _, vendorName := range yamlDocument.CommonVendors() {
		allowedVendors = append(allowedVendors, vendorName.Value)
	}

	for _, name := range allowedVendors {
		yamlVendor, ok := yamlDocument.Vendors()[name]
		if !ok {
			continue
		}

		for _, vendorIn := range yamlVendor.Value.ImportPaths() {
			list = append(list, common.NewReferable(vendorIn, yamlVendor.Reference))
		}
	}

	return list, nil
}
