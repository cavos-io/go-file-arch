package filearch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckDirectoryNameRulesRequiresSnakeCaseAndDeniesLegacyNames(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		"adapter/organization_manager",
		"adapter/knowledge-manager",
		"internal/testpostgres",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relative)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inventory, err := newRepositoryInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{DirectoryNameRules: []DirectoryNameRule{{
		ID:          "source-directory-name",
		Directories: FileSet{Include: []string{"adapter/**", "internal/**"}},
		Require: DirectoryNameRequirement{DirectoryName: FileNameCondition{
			Matches: []string{`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`},
		}},
		Deny: DirectoryNameRequirement{DirectoryName: FileNameCondition{
			EqualsAny: []string{"testpostgres"},
		}},
		Message: "source directories use snake_case",
	}}}

	got := checkDirectoryNameRules(cfg, inventory)
	want := []ruleDiagnostic{
		{Path: "adapter/knowledge-manager", Line: 1, Column: 1, RuleID: "source-directory-name", Message: `source directories use snake_case directory "knowledge-manager" does not satisfy required directoryName condition`},
		{Path: "internal/testpostgres", Line: 1, Column: 1, RuleID: "source-directory-name", Message: `source directories use snake_case directory "testpostgres" satisfies denied directoryName condition`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("checkDirectoryNameRules() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigParsesDirectoryNameRules(t *testing.T) {
	path := writeConfigFixture(t, `
version: 1
directoryNameRules:
  - id: source-directory-name
    directories:
      include: [adapter/**]
      exclude: [adapter/generated/**]
    require:
      directoryName:
        matches: ['^[a-z][a-z0-9_]*$']
    deny:
      directoryName:
        equalsAny: [legacyname]
    message: source directories use snake_case
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.DirectoryNameRules[0].Require.DirectoryName.Matches[0]; got != "^[a-z][a-z0-9_]*$" {
		t.Fatalf("require match = %q", got)
	}
	if got := cfg.DirectoryNameRules[0].Deny.DirectoryName.EqualsAny[0]; got != "legacyname" {
		t.Fatalf("deny equalsAny = %q", got)
	}
}

func TestLoadConfigRejectsInvalidDirectoryNameRules(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string
	}{
		{"missing id", "directories: {include: [adapter/**]}\n    deny: {directoryName: {equals: legacy}}\n    message: bad", "id is required"},
		{"missing include", "id: bad\n    deny: {directoryName: {equals: legacy}}\n    message: bad", "must include at least one directory pattern"},
		{"empty conditions", "id: bad\n    directories: {include: [adapter/**]}\n    message: bad", "must configure require or deny"},
		{"invalid regex", "id: bad\n    directories: {include: [adapter/**]}\n    require: {directoryName: {matches: ['[']}}\n    message: bad", "invalid regex"},
		{"missing message", "id: bad\n    directories: {include: [adapter/**]}\n    deny: {directoryName: {equals: legacy}}", "message is required"},
		{"unsupported severity", "id: bad\n    severity: warning\n    directories: {include: [adapter/**]}\n    deny: {directoryName: {equals: legacy}}\n    message: bad", "unsupported severity"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigFixture(t, "version: 1\ndirectoryNameRules:\n  - "+test.rule+"\n")
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}
