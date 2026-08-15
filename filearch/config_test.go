package filearch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"core/user/model",
		"library/logging",
	} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
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

func TestLoadConfigParsesRepositoryContractRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
directoryRules:
  - id: module-files
    directories:
      include: [modules/*]
      exclude: [modules/ignored]
    require:
      files: [metadata.go]
      anyFiles: [feature.go, features/*.go]
    message: module files required
fileContractRules:
  - id: feature-contract
    files:
      include: [modules/*/feature.go]
    require:
      siblingFiles: [feature_test.go]
      declarations:
        - kind: func
          name: NewFeature
          exported: true
          receiver:
            present: false
          parameters:
            first: {name: ctx, type: context.Context}
          returns:
            contains: ["*Feature", {type: error}]
    deny:
      declarations:
        - kind: interface
          exported: true
    message: feature contract required
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.DirectoryRules[0].Require.AnyFiles[1]; got != "features/*.go" {
		t.Fatalf("anyFiles[1] = %q", got)
	}
	selector := cfg.FileContractRules[0].Require.Declarations[0]
	if selector.Exported == nil || !*selector.Exported {
		t.Fatalf("exported = %v, want true", selector.Exported)
	}
	if got := selector.Returns.Contains[0].Type; got != "*Feature" {
		t.Fatalf("returns.contains[0].type = %q", got)
	}
	if got := selector.Returns.Contains[1].Type; got != "error" {
		t.Fatalf("returns.contains[1].type = %q", got)
	}
	if selector.Parameters.First == nil || selector.Parameters.First.Name != "ctx" || selector.Parameters.First.Type != "context.Context" {
		t.Fatalf("parameters.first = %#v", selector.Parameters.First)
	}
}

func TestLoadConfigRejectsInvalidRepositoryContracts(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "empty directory rule",
			yaml: "version: 1\ndirectoryRules:\n- id: bad\n  directories: {include: [modules/*]}\n  message: bad\n",
			want: "must configure files or anyFiles",
		},
		{
			name: "return on struct",
			yaml: "version: 1\nfileContractRules:\n- id: bad\n  files: {include: ['**/*.go']}\n  require:\n    declarations:\n    - kind: struct\n      returns: {contains: [Thing]}\n  message: bad\n",
			want: "returns is only valid for func",
		},
		{
			name: "value on func",
			yaml: "version: 1\nfileContractRules:\n- id: bad\n  files: {include: ['**/*.go']}\n  require:\n    declarations:\n    - kind: func\n      value: {matches: [x]}\n  message: bad\n",
			want: "value is only valid for const",
		},
		{
			name: "invalid regex",
			yaml: "version: 1\nfileContractRules:\n- id: bad\n  files: {include: ['**/*.go']}\n  deny:\n    declarations:\n    - kind: func\n      nameMatches: ['[']\n  message: bad\n",
			want: "invalid nameMatches regex",
		},
		{
			name: "unknown type condition field",
			yaml: "version: 1\nfileContractRules:\n- id: bad\n  files: {include: ['**/*.go']}\n  require:\n    declarations:\n    - kind: func\n      returns:\n        contains: [{typo: error}]\n  message: bad\n",
			want: "field typo not found",
		},
		{
			name: "escaping sibling",
			yaml: "version: 1\nfileContractRules:\n- id: bad\n  files: {include: ['**/*.go']}\n  require: {siblingFiles: [../secret.go]}\n  message: bad\n",
			want: "must not escape",
		},
		{
			name: "duplicate ID",
			yaml: "version: 1\ncontentRules:\n- id: same\n  files: {include: ['**/*.go']}\n  deny: {declarations: [struct]}\n  message: one\nfileContractRules:\n- id: same\n  files: {include: ['**/*.go']}\n  deny: {declarations: [{kind: interface}]}\n  message: two\n",
			want: "duplicate rule id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadConfig() error = %v, want substring %q", err, tt.want)
			}
		})
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

func TestFileNameRuleTriggersOnFuncNameRegex(t *testing.T) {
	file := parseTestFile(t, "repository.go", `package adapter

func NewProjectRepository() {}
func helper() {}
`)
	rule := FileNameRule{
		When: FileNameRuleWhen{
			Declarations:  []string{"func"},
			FuncNameRegex: []string{"^New.*Repository$"},
		},
		Require: FileNameRequirement{
			FileName: FileNameCondition{Suffix: "_repository.go"},
		},
	}

	triggers := fileNameRuleTriggers(file, rule)
	if len(triggers) != 1 {
		t.Fatalf("len(triggers) = %d, want 1", len(triggers))
	}
	if triggers[0].kind != "func" {
		t.Fatalf("trigger kind = %q, want func", triggers[0].kind)
	}
}

func TestLoadConfigRejectsUnknownDependencyComponent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestLoadConfigRejectsMissingComponentPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
components:
  adapter:
    in:
      - adapter/**
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not found directories for \"adapter/**\"") {
		t.Fatalf("LoadConfig() error = %v, want missing component path error", err)
	}
}

func TestLoadConfigValidatesComponentPathsRelativeToWorkdir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "core", "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
version: 1
workdir: internal
components:
  core:
    in:
      - core/**
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.workdir != "internal" {
		t.Fatalf("workdir = %q, want internal", cfg.workdir)
	}
	if got := filepath.ToSlash(cfg.workdirAbs); got != filepath.ToSlash(filepath.Join(dir, "internal")) {
		t.Fatalf("workdirAbs = %q, want %q", got, filepath.ToSlash(filepath.Join(dir, "internal")))
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

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := writeConfigFixture(t, "version: 1\nunknownPolicy: true\n")

	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "field unknownPolicy not found") {
		t.Fatalf("LoadConfig() error = %v, want unknown-field error", err)
	}
}

func TestLoadConfigParsesGenericOptions(t *testing.T) {
	path := writeConfigFixture(t, `
version: 1
componentOptions:
  requireMatch: true
  requireSingleMatch: true
components:
  composition:
    files:
      include: [app.go]
templates:
  domain:
    pattern: core/{domain}/service.go
    captures:
      domain: '^[a-z][a-z0-9_]*$'
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ComponentOptions.RequireMatch || !cfg.ComponentOptions.RequireSingleMatch {
		t.Fatalf("ComponentOptions = %#v", cfg.ComponentOptions)
	}
	if cfg.Templates["domain"].Pattern != "core/{domain}/service.go" {
		t.Fatalf("Templates = %#v", cfg.Templates)
	}
}

func TestLoadConfigRejectsUnsupportedSeverity(t *testing.T) {
	path := writeConfigFixture(t, `
version: 1
contentRules:
  - id: no-structs
    severity: warning
    files:
      include: ['**/*.go']
    deny:
      declarations: [struct]
    message: structs are forbidden
`)

	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), `unsupported severity "warning"`) {
		t.Fatalf("LoadConfig() error = %v, want unsupported severity error", err)
	}
}

func TestLoadConfigRejectsDuplicateIDsAcrossNewRuleFamilies(t *testing.T) {
	path := writeConfigFixture(t, `
version: 1
pathRules:
  - id: shared-id
    directories: {include: ['.']}
    require: {files: [go.mod]}
    message: root files
importRules:
  - id: shared-id
    files: {include: ['**/*.go']}
    deny: {categories: [external]}
    message: external imports
`)

	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), `duplicate rule id "shared-id"`) {
		t.Fatalf("LoadConfig() error = %v, want duplicate rule ID error", err)
	}
}

func TestLoadConfigAcceptsFileOnlyComponent(t *testing.T) {
	path := writeConfigFixture(t, `
version: 1
components:
  composition:
    files:
      include: [app.go]
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Components["composition"].Files.Include; !reflect.DeepEqual(got, []string{"app.go"}) {
		t.Fatalf("component files = %v", got)
	}
}

func writeConfigFixture(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func parseTestFile(t *testing.T, name string, src string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file
}
