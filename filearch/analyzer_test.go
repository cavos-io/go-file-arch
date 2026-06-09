package filearch

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	configPath := filepath.Join(testdata, "go-file-arch.yml")

	analysistest.Run(t, testdata, NewAnalyzer(configPath), "core/user", "core/badstruct", "core/badfunc", "interface/http")
}

func TestAnalyzerRejectsStructInCoreUserRepository(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/repository.go": `package user

type RepositoryData struct { // want ` + "`\\[core-repository-interface-only\\]: core repository files may only define interfaces\\. Move implementations to adapter/\\*\\*\\. detected declaration kind: struct` `\\[core-no-struct-outside-model\\]: Structs in core should live under core/\\*\\*/model/\\*\\*\\. detected declaration kind: struct`" + `
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

func NewRepository() string { // want ` + "`\\[core-repository-interface-only\\]: core repository files may only define interfaces\\. Move implementations to adapter/\\*\\*\\. detected declaration kind: func`" + `
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
		"interface/http/user.go": `package http // want ` + "`\\[dto-files-only\\]: interface Go files must be named dto files\\. file name \"user\\.go\" does not satisfy required fileName condition; detected declaration kind: file`" + `

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

func TestAnalyzerReportsDependencyRuleViolation(t *testing.T) {
	gopath, cleanup, err := analysistest.WriteFiles(map[string]string{
		"core/user/service.go": `package user

import "adapter/user" // want ` + "`\\[dependencyRules\\.core\\] dependency rule for component core: core packages may not import component adapter\\. Move dependency behind core interface or adapter implementation\\. detected import component: adapter`" + `

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

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(analysistest.TestData(), "go-file-arch.yml"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "go-file-arch.yml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}
