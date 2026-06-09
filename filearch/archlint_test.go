package filearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckArchLintReportsCopiedDependencyViolation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/project

go 1.26
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

	result, err := CheckArchLint(context.Background(), ArchLintOptions{
		ProjectPath: dir,
		ArchFile:    ".go-arch-lint.yml",
	})
	if err != nil {
		t.Fatalf("CheckArchLint() error = %v", err)
	}
	if !result.HasWarnings {
		t.Fatal("HasWarnings = false, want true")
	}
	if len(result.DependencyWarnings) != 1 {
		t.Fatalf("len(DependencyWarnings) = %d, want 1", len(result.DependencyWarnings))
	}
	warning := result.DependencyWarnings[0]
	if warning.ComponentName != "core" {
		t.Fatalf("ComponentName = %q, want core", warning.ComponentName)
	}
	if warning.ImportPath != "example.com/project/adapter/user" {
		t.Fatalf("ImportPath = %q, want example.com/project/adapter/user", warning.ImportPath)
	}
}

func TestCheckArchLintReportsCopiedMissingComponentNotice(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/project

go 1.26
`)
	writeFile(t, dir, ".go-arch-lint.yml", `
version: 3
components:
  core: { in: core/** }
deps:
  core:
    mayDependOn:
      - core
`)

	result, err := CheckArchLint(context.Background(), ArchLintOptions{
		ProjectPath: dir,
		ArchFile:    ".go-arch-lint.yml",
	})
	if err != nil {
		t.Fatalf("CheckArchLint() error = %v", err)
	}
	if len(result.DocumentNotices) != 1 {
		t.Fatalf("len(DocumentNotices) = %d, want 1", len(result.DocumentNotices))
	}
	if result.DocumentNotices[0].Text == "" {
		t.Fatal("DocumentNotices[0].Text is empty")
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
