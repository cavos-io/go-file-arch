package http

import (
	"context"

	"github.com/cavos-io/go-file-arch-fixture/core/project"
	"github.com/cavos-io/go-file-arch-fixture/interface/http/dto"
)

type ProjectHandler struct {
	service project.IProjectService
}

func NewProjectHandler(service project.IProjectService) *ProjectHandler {
	return &ProjectHandler{service: service}
}

func (handler *ProjectHandler) HandlerFindProject(id string) (dto.ProjectResponse, error) {
	found, err := handler.service.Find(context.Background(), id)
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	return dto.ProjectResponse{ID: found.ID, Name: found.Name}, nil
}
