package assembler

import (
	"regexp"

	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type excludeFilesMatcherAssembler struct{}

func newExcludeFilesMatcherAssembler() *excludeFilesMatcherAssembler {
	return &excludeFilesMatcherAssembler{}
}

func (efa *excludeFilesMatcherAssembler) assemble(spec *arch.Spec, yamlSpec spec.Document) error {
	for _, regString := range yamlSpec.ExcludedFilesRegExp() {
		matcher, err := regexp.Compile(regString.Value)
		if err != nil {
			continue
		}

		spec.ExcludeFilesMatcher = append(spec.ExcludeFilesMatcher, common.NewReferable(
			matcher,
			regString.Reference,
		))
	}

	return nil
}
