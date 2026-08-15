package filearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReturnsViolationError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/violation\n\ngo 1.25\n")
	writeFile(t, dir, ".go-file-arch.yml", `
version: 1
contentRules:
  - id: no-functions
    files: {include: ['**/*.go']}
    deny: {declarations: [func]}
    message: functions denied
`)
	writeFile(t, dir, "feature.go", "package violation\nfunc Feature() {}\n")

	err := Run(context.Background(), Options{ConfigPath: filepath.Join(dir, ".go-file-arch.yml"), Workdir: dir})
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("Run() error = %T %v, want *ViolationError", err, err)
	}
}

func TestRunUsesWorkdirOverrideWithExternalConfig(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "go.mod", "module example.com/workdir\n\ngo 1.25\n")
	writeFile(t, repo, "core/feature.go", "package core\n")

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "policy.yml")
	if err := os.WriteFile(configPath, []byte(`
version: 1
workdir: ignored-by-cli
components:
  core:
    in: [core]
pathRules:
  - id: module-file
    directories: {include: ['.']}
    require: {files: [go.mod]}
    message: module required
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Options{ConfigPath: configPath, Workdir: repo}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunGenericStrictFixtures(t *testing.T) {
	passing := filepath.Join("testdata", "generic-strict")
	if err := Run(context.Background(), Options{
		ConfigPath: filepath.Join(passing, ".architecture.yaml"),
		Workdir:    passing,
	}); err != nil {
		t.Fatalf("passing fixture: %v", err)
	}

	failing := filepath.Join("testdata", "generic-fail")
	err := Run(context.Background(), Options{
		ConfigPath: filepath.Join(failing, ".architecture.yaml"),
		Workdir:    failing,
	})
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("failing fixture error = %T %v, want *ViolationError", err, err)
	}
	const want = `7 architecture violation(s):
core/project:1:1: [domain-layout]: domain layout contract required at least one file matching: model/*_model.go
core/project/service.go:1:1: [domain-name-contract]: captured domain names must agree required declaration not found: interface IProjectService
core/project/service.go:1:1: [domain-name-contract]: captured domain names must agree required sibling file not found: model/project_model.go
core/project/service.go:1:1: [repository-context-contract]: repository methods start with context required declaration not found: interface IProjectRepository
core/project/service.go:5:2: [core-import-boundary]: core imports must be explicit import "fmt" is not allowed; category: standard; target component: <none>
legacy:1:1: [root-layout]: root layout contract denied directory: legacy
rogue.go:1:1: [componentOptions.requireMatch]: file does not match any component`
	if violation.Error() != want {
		t.Fatalf("failing fixture diagnostics:\n%s\n\nwant:\n%s", violation, want)
	}
}

func TestRunRequiresConfigPath(t *testing.T) {
	err := Run(context.Background(), Options{
		Workdir:  t.TempDir(), // empty dir: no config to discover
		Patterns: []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Fatalf("Run() error = %v, want 'config not found'", err)
	}
}

func TestDiscoverConfigPath(t *testing.T) {
	if _, err := discoverConfigPath(t.TempDir()); err == nil {
		t.Fatal("discoverConfigPath() error = nil for empty dir, want error")
	}

	for _, name := range []string{".go-file-arch.yml", ".go-file-arch.yaml"} {
		dir := t.TempDir()
		want := filepath.Join(dir, name)
		if err := os.WriteFile(want, []byte("version: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := discoverConfigPath(dir)
		if err != nil {
			t.Fatalf("discoverConfigPath(%s) error = %v", name, err)
		}
		if got != want {
			t.Fatalf("discoverConfigPath(%s) = %q, want %q", name, got, want)
		}
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

func TestRunReportsMissingSiblingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/contracts\n\ngo 1.25\n")
	writeFile(t, dir, ".go-file-arch.yml", `
version: 1
fileContractRules:
  - id: feature-contract
    files:
      include: [modules/*/feature.go]
    require:
      siblingFiles: [feature_test.go]
    message: feature contract
`)
	writeFile(t, dir, "modules/one/feature.go", "package one\n")

	err := Run(context.Background(), Options{
		ConfigPath: filepath.Join(dir, ".go-file-arch.yml"),
		Workdir:    dir,
		Patterns:   []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want missing sibling violation")
	}
	for _, want := range []string{"modules/one/feature.go", "feature-contract", "required sibling file not found: feature_test.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run() error = %v, want %q", err, want)
		}
	}
}
