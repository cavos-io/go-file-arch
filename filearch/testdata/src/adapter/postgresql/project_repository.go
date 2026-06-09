package postgresql

import (
	"context"

	"github.com/cavos-io/go-file-arch-fixture/core/project/model"
)

type ProjectRepository struct{}

func NewProjectRepository() *ProjectRepository {
	return &ProjectRepository{}
}

func (repository *ProjectRepository) Find(ctx context.Context, id string) (model.ProjectModel, error) {
	return model.ProjectModel{ID: id, Name: "project"}, nil
}
