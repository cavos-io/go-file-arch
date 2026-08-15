package project

import (
	"context"

	"example.com/generic-service/core/project/model"
)

type IProjectService interface {
	Find(ctx context.Context, id uint64) (model.ProjectModel, error)
}

type IProjectRepository interface {
	Find(ctx context.Context, id uint64) (model.ProjectModel, error)
}

type Service struct{}

func NewProjectService() *Service { return &Service{} }

func (service *Service) Find(ctx context.Context, id uint64) (model.ProjectModel, error) {
	return model.ProjectModel{ID: id}, nil
}
