package grpc

import "github.com/cavos-io/go-file-arch-fixture/core/project"

type ProjectGrpcServer struct {
	service project.IProjectService
}

func NewProjectGrpcServer(service project.IProjectService) *ProjectGrpcServer {
	return &ProjectGrpcServer{service: service}
}
