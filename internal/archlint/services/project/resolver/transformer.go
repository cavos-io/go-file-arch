package resolver

import (
	"regexp"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/common"
)

func refPathToList(list []common.Referable[models.ResolvedPath]) []models.ResolvedPath {
	result := make([]models.ResolvedPath, 0)

	for _, path := range list {
		result = append(result, path.Value)
	}

	return result
}

func refRegExpToList(list []common.Referable[*regexp.Regexp]) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0)

	for _, path := range list {
		result = append(result, path.Value)
	}

	return result
}
