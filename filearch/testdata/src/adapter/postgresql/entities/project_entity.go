package entities

import "github.com/cavos-io/go-file-arch-fixture/core/project/model"

type ProjectEntity struct {
	ID   string
	Name string
}

func (entity ProjectEntity) ToModel() model.ProjectModel {
	return model.ProjectModel{
		ID:   entity.ID,
		Name: entity.Name,
	}
}

func FromModel(project model.ProjectModel) ProjectEntity {
	return ProjectEntity{
		ID:   project.ID,
		Name: project.Name,
	}
}
