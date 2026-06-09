package decoder

import "github.com/cavos-io/go-file-arch/internal/archlint/models/common"

func castRef[T any](r ref[T]) common.Referable[T] {
	return r.ref
}

func castRefList[T any](r []ref[T]) []common.Referable[T] {
	casted := make([]common.Referable[T], 0, len(r))

	for _, ref := range r {
		casted = append(casted, castRef(ref))
	}

	return casted
}
