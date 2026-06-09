package ast

import (
	"go/token"

	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

func PositionFromToken(pos token.Position) common.Reference {
	ref := common.NewReferenceSingleLine(
		pos.Filename,
		pos.Line,
		pos.Column,
	)

	if pos.Line == 0 {
		ref.Valid = false
		ref.Line = 0

		return ref
	}

	return ref
}
