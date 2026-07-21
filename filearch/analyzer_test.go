package filearch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestStrictFixturePasses(t *testing.T) {
	testdata := analysistest.TestData()
	configPath := filepath.Join(testdata, "go-file-arch.yml")

	err := Run(context.Background(), Options{
		ConfigPath: configPath,
		Workdir:    filepath.Join(testdata, "src"),
		Patterns:   []string{"./..."},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestStrictFixtureReportsViolations(t *testing.T) {
	sourceConfig := filepath.Join(analysistest.TestData(), "go-file-arch.yml")
	data, err := os.ReadFile(sourceConfig)
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	auditfail := filepath.Join(tempDir, "auditfail")
	copyFixtureDir(t, filepath.Join(analysistest.TestData(), "auditfail"), auditfail)
	configPath := filepath.Join(tempDir, "go-file-arch.yml")
	configData := strings.Replace(string(data), "workdir: src", "workdir: auditfail", 1)
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	err = Run(context.Background(), Options{
		ConfigPath: configPath,
		Workdir:    auditfail,
		Patterns:   []string{"./..."},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want strict fixture violations")
	}
	for _, want := range []string{
		"core-repository-interface-only",
		"core-no-struct-outside-model-or-service",
		"dto-structs-in-dto-files",
		"handlers-in-handler-files",
		"handler-methods-in-handler-files",
		"package-metadata",
		"feature-contract",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run() error = %v, want %q", err, want)
		}
	}
}

func copyFixtureDir(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzerRejectsStructInCoreUserRepository(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/repository.go": `package user

type RepositoryData struct { // want ` + "`\\[core-repository-interface-only\\]: core repository files may only define interfaces\\. Move implementations to adapter/\\*\\*\\. Found disallowed struct \"RepositoryData\" \\(struct declarations are denied here\\)\\.` `\\[core-no-struct-outside-model\\]: Structs in core should live under core/\\*\\*/model/\\*\\*\\. Found disallowed struct \"RepositoryData\" \\(struct declarations are denied here\\)\\.`" + `
	ID string
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := writeTestConfig(t, gopath)
	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerRejectsFuncInCoreUserRepository(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/repository.go": `package user

func NewRepository() string { // want ` + "`\\[core-repository-interface-only\\]: core repository files may only define interfaces\\. Move implementations to adapter/\\*\\*\\. Found disallowed func \"NewRepository\" \\(func declarations are denied here\\)\\.`" + `
	return "repository"
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := writeTestConfig(t, gopath)
	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerReportsFileLevelFileNameRule(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"interface/http/user.go": `package http // want ` + "`\\[dto-files-only\\]: interface Go files must be named dto files\\. File \"user\\.go\" does not satisfy required fileName condition\\.`" + `

func HandleUser() {}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
fileNameRules:
  - id: dto-files-only
    files:
      include:
        - interface/**/*.go
    require:
      fileName:
        matches:
          - ".*_dto\\.go$"
    message: "interface Go files must be named dto files."
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "interface/http")
}

func TestAnalyzerReportsFileContractDeclarations(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"modules/one/feature.go": `package one // want "\\[feature-contract\\].*required declaration not found: func NewFeature"

type Client interface{} // want "\\[feature-contract\\].*denied declaration matched: exported interface Client"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
fileContractRules:
  - id: feature-contract
    files:
      include: [modules/*/feature.go]
    require:
      declarations:
        - kind: func
          name: NewFeature
    deny:
      declarations:
        - kind: interface
          exported: true
    message: feature contract
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "modules/one")
}

func TestAnalyzerReportsDependencyRuleViolation(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "adapter/user" // want ` + "`\\[dependencyRules\\.core\\] dependency rule for component core: core packages may not import component adapter via \"adapter/user\"\\. Move dependency behind core interface or adapter implementation\\. detected import component: adapter`" + `

func Use() string {
	return adapter.Name
}
`,
		"adapter/user/repository.go": `package adapter

const Name = "adapter"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
components:
  core:
    in:
      - core/**
  adapter:
    in:
      - adapter/**
dependencyRules:
  core:
    mayDependOn:
      - core
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerMatchesComponentsByPackageDirectory(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "adapter/user" // want ` + "`\\[dependencyRules\\.core\\] dependency rule for component core: core packages may not import component adapter via \"adapter/user\"\\. Move dependency behind core interface or adapter implementation\\. detected import component: adapter`" + `

func Use() string {
	return adapter.Name
}
`,
		"adapter/user/repository.go": `package adapter

const Name = "adapter"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
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
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerAllowsConfiguredDependency(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "core/model"

func Use() string {
	return model.Name
}
`,
		"core/model/user.go": `package model

const Name = "user"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
components:
  core:
    in:
      - core/**
  core_model:
    in:
      - core/model/**
dependencyRules:
  core:
    mayDependOn:
      - core
      - core_model
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerAllowsCommonComponentDependency(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"app/user/service.go": `package user

import "library/logging"

func Use() string {
	return logging.Name
}
`,
		"library/logging/logging.go": `package logging

const Name = "logging"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
components:
  app:
    in:
      - app/**
  library:
    in:
      - library/**
commonComponents:
  - library
dependencyRules:
  app:
    mayDependOn:
      - app
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "app/user")
}

func TestAnalyzerIgnoresExternalImports(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "fmt"

func Use() string {
	return fmt.Sprint("user")
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
components:
  core:
    in:
      - core/**
dependencyRules:
  core:
    mayDependOn:
      - core
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func TestAnalyzerIgnoresUnmappedLocalImports(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "generated/user"

func Use() string {
	return generated.Name
}
`,
		"generated/user/user.go": `package generated

const Name = "generated"
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	configPath := filepath.Join(gopath, "go-file-arch.yml")
	err = os.WriteFile(configPath, []byte(`
version: 1
workdir: src
components:
  core:
    in:
      - core/**
dependencyRules:
  core:
    mayDependOn:
      - core
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, gopath, NewAnalyzer(configPath), "core/user")
}

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "go-file-arch.yml")
	if err := os.WriteFile(path, []byte(`
version: 1
workdir: src

components:
  core:
    in:
      - core/**

dependencyRules:
  core:
    mayDependOn:
      - core

contentRules:
  - id: core-repository-interface-only
    files:
      include:
        - core/**/repository.go
      exclude:
        - "**/*_test.go"
    allow:
      declarations:
        - interface
    deny:
      declarations:
        - struct
        - func
        - var
        - const
    message: "core repository files may only define interfaces. Move implementations to adapter/**."

  - id: core-no-struct-outside-model
    files:
      include:
        - core/**/*.go
      exclude:
        - core/**/model/**/*.go
        - "**/*_test.go"
    deny:
      declarations:
        - struct
    message: "Structs in core should live under core/**/model/**."
`), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}
