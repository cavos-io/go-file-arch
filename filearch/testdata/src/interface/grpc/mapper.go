package grpc

import "github.com/cavos-io/go-file-arch-fixture/core/project/model"

type ProjectMessage struct {
	Id   string
	Name string
}

func FromProjectModel(project model.ProjectModel) ProjectMessage {
	return ProjectMessage{Id: project.ID, Name: project.Name}
}
