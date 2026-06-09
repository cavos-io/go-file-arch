package filearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1

components:
  core:
    in:
      - core/**
  core_model:
    in:
      - core/**/model/**
  library:
    in:
      - library/**

commonComponents:
  - library

dependencyRules:
  core:
    mayDependOn:
      - core
      - core_model
      - library

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
    message: "core repository files may only define interfaces."
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if got := cfg.Components["core"].In[0]; got != "core/**" {
		t.Fatalf("core component pattern = %q", got)
	}
	if got := cfg.CommonComponents[0]; got != "library" {
		t.Fatalf("common component = %q", got)
	}
	if got := cfg.DependencyRules["core"].MayDependOn[2]; got != "library" {
		t.Fatalf("dependency rule = %q", got)
	}
	if len(cfg.ContentRules) != 1 {
		t.Fatalf("len(ContentRules) = %d, want 1", len(cfg.ContentRules))
	}
	rule := cfg.ContentRules[0]
	if rule.ID != "core-repository-interface-only" {
		t.Fatalf("Rule ID = %q", rule.ID)
	}
	if got := rule.Files.Include[0]; got != "core/**/repository.go" {
		t.Fatalf("include = %q", got)
	}
	if got := rule.Deny.Declarations[1]; got != "func" {
		t.Fatalf("deny[1] = %q", got)
	}
}

func TestLoadConfigParsesFileNameRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
fileNameRules:
  - id: dto-file-name
    files:
      include:
        - interface/**/*.go
      exclude:
        - "**/*_test.go"
    when:
      declarations:
        - struct
      typeNameRegex:
        - ".*DTO$"
        - ".*Request$"
    require:
      fileName:
        matches:
          - ".*_dto\\.go$"
          - "request\\.go$"
    message: "DTO/request structs must use DTO file names."
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.FileNameRules) != 1 {
		t.Fatalf("len(FileNameRules) = %d, want 1", len(cfg.FileNameRules))
	}
	rule := cfg.FileNameRules[0]
	if rule.When.Declarations[0] != "struct" {
		t.Fatalf("when declaration = %q", rule.When.Declarations[0])
	}
	if rule.When.TypeNameRegex[1] != ".*Request$" {
		t.Fatalf("typeNameRegex[1] = %q", rule.When.TypeNameRegex[1])
	}
	if rule.Require.FileName.Matches[0] != ".*_dto\\.go$" {
		t.Fatalf("require fileName match = %q", rule.Require.FileName.Matches[0])
	}
}

func TestLoadConfigRejectsMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml"))
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
}

func TestLoadConfigRejectsInvalidDeclarationKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
contentRules:
  - id: bad
    files:
      include:
        - "**/*.go"
    deny:
      declarations:
        - class
    message: bad
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
}

func TestLoadConfigRejectsInvalidFileNameRuleRegex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
fileNameRules:
  - id: bad
    files:
      include:
        - "**/*.go"
    when:
      typeNameRegex:
        - "["
    require:
      fileName:
        matches:
          - ".*\\.go$"
    message: bad
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
}

func TestFileNameAllowedRejectsDeniedName(t *testing.T) {
	rule := FileNameRule{
		Deny: FileNameRequirement{
			FileName: FileNameCondition{
				Matches: []string{".*_bad\\.go$"},
			},
		},
	}

	if fileNameAllowed("user_bad.go", rule) {
		t.Fatal("fileNameAllowed() = true, want false")
	}
	if !fileNameAllowed("user.go", rule) {
		t.Fatal("fileNameAllowed() = false, want true")
	}
}

func TestFileNameAllowedSupportsAllMatcherTypes(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		rule     FileNameRule
		want     bool
	}{
		{
			name:     "require equals",
			fileName: "dto.go",
			rule: FileNameRule{
				Require: FileNameRequirement{
					FileName: FileNameCondition{Equals: "dto.go"},
				},
			},
			want: true,
		},
		{
			name:     "require equals any",
			fileName: "request.go",
			rule: FileNameRule{
				Require: FileNameRequirement{
					FileName: FileNameCondition{EqualsAny: []string{"dto.go", "request.go"}},
				},
			},
			want: true,
		},
		{
			name:     "require prefix",
			fileName: "user_dto.go",
			rule: FileNameRule{
				Require: FileNameRequirement{
					FileName: FileNameCondition{Prefix: "user_"},
				},
			},
			want: true,
		},
		{
			name:     "require suffix",
			fileName: "user_dto.go",
			rule: FileNameRule{
				Require: FileNameRequirement{
					FileName: FileNameCondition{Suffix: "_dto.go"},
				},
			},
			want: true,
		},
		{
			name:     "deny equals any",
			fileName: "helper.go",
			rule: FileNameRule{
				Deny: FileNameRequirement{
					FileName: FileNameCondition{EqualsAny: []string{"helper.go", "misc.go"}},
				},
			},
			want: false,
		},
		{
			name:     "require suffix rejects mismatch",
			fileName: "user.go",
			rule: FileNameRule{
				Require: FileNameRequirement{
					FileName: FileNameCondition{Suffix: "_dto.go"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileNameAllowed(tt.fileName, tt.rule); got != tt.want {
				t.Fatalf("fileNameAllowed(%q) = %v, want %v", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestLoadConfigRejectsUnknownDependencyComponent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
components:
  core:
    in:
      - core/**
dependencyRules:
  core:
    mayDependOn:
      - adapter
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
}

func TestMostSpecificComponentWins(t *testing.T) {
	cfg := &Config{
		Components: map[string]Component{
			"interface": {
				In: []string{"interface/**"},
			},
			"interface_dto": {
				In: []string{"interface/**/dto/**"},
			},
		},
	}

	component, ok := cfg.matchComponent("interface/http/dto/user.go")
	if !ok {
		t.Fatal("matchComponent() ok = false, want true")
	}
	if component != "interface_dto" {
		t.Fatalf("matchComponent() = %q, want interface_dto", component)
	}
}
