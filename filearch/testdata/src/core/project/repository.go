package project

import (
	"context"

	"github.com/cavos-io/go-file-arch-fixture/core/project/model"
)

type IProjectRepository interface {
	Find(ctx context.Context, id string) (model.ProjectModel, error)
}

type IObjectStorageRepository interface {
	Store(ctx context.Context, key string) error
}
