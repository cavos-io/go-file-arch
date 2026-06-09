package assembler

import (
	"fmt"

	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
	"github.com/cavos-io/go-file-arch/internal/archlint/services/spec"
)

type (
	assembler interface {
		assemble(spec *arch.Spec, doc spec.Document) error
	}

	specCompositeModifier struct {
		modifiers []assembler
	}
)

func newSpecCompositeAssembler(modifiers []assembler) *specCompositeModifier {
	return &specCompositeModifier{
		modifiers: modifiers,
	}
}

func (s *specCompositeModifier) assemble(spec *arch.Spec, doc spec.Document) error {
	for _, modifier := range s.modifiers {
		err := modifier.assemble(spec, doc)
		if err != nil {
			return fmt.Errorf("failed to assemble spec with '%T' assembler: %w", modifier, err)
		}
	}

	return nil
}
