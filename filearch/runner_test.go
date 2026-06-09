package filearch

import (
	"context"
	"strings"
	"testing"
)

func TestRunRequiresConfigPath(t *testing.T) {
	err := Run(context.Background(), Options{
		Patterns: []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
}

func TestRunCombinesArchLintCheck(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/project

go 1.26
`)
	writeFile(t, dir, ".go-file-arch.yml", `
version: 1
components:
  core:
    in:
      - core/*
  adapter:
    in:
      - adapter/*
dependencyRules:
  core:
    mayDependOn:
      - core
      - adapter
`)
	writeFile(t, dir, ".go-arch-lint.yml", `
version: 3
components:
  core: { in: core/* }
  adapter: { in: adapter/* }
deps:
  core:
    mayDependOn:
      - core
`)
	writeFile(t, dir, "core/user/service.go", `package user

import "example.com/project/adapter/user"

func Use() string {
	return user.Name
}
`)
	writeFile(t, dir, "adapter/user/repository.go", `package user

const Name = "adapter"
`)

	err := Run(context.Background(), Options{
		ConfigPath:         dir + "/.go-file-arch.yml",
		ArchLintConfigPath: ".go-arch-lint.yml",
		Workdir:            dir,
		Patterns:           []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want go-arch-lint error")
	}
	if !strings.Contains(err.Error(), "go-arch-lint violation") {
		t.Fatalf("Run() error = %v, want go-arch-lint violation", err)
	}
}
