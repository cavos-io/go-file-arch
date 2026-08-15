package project

import (
	"context"
	"fmt"
)

type IProjectsService interface {
	Find(ctx context.Context, id uint64) error
}

type IProjectRepository interface {
	Find(id uint64, ctx context.Context) error
}

type Service struct{}

func NewProjectService() *Service { return &Service{} }

var Format = fmt.Sprintf
