package project

import (
	"context"

	"github.com/cavos-io/go-file-arch-fixture/core/project/model"
)

type IProjectService interface {
	Find(ctx context.Context, id string) (model.ProjectModel, error)
}

type Service struct {
	repository IProjectRepository
}

func NewProjectService(repository IProjectRepository) IProjectService {
	return &Service{repository: repository}
}

func (service *Service) Find(ctx context.Context, id string) (model.ProjectModel, error) {
	return service.repository.Find(ctx, id)
}
