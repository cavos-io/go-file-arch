package filearch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckPathRulesRequiresAllowsAndDeniesDirectEntries(t *testing.T) {
	root := t.TempDir()
	writeInventoryFile(t, root, "go.mod")
	writeInventoryFile(t, root, "tmp.txt")
	writeInventoryFile(t, root, "cmd/main.go")
	writeInventoryFile(t, root, "legacy/keep.txt")

	inventory, err := newRepositoryInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{PathRules: []PathRule{{
		ID:          "root-layout",
		Directories: FileSet{Include: []string{"."}},
		Depth:       "direct",
		Require:     PathSet{Files: []string{"app.go"}},
		Allow: PathSet{
			Files:       []string{"go.mod", "app.go"},
			Directories: []string{"cmd", "legacy"},
		},
		Deny:    PathSet{Directories: []string{"legacy"}},
		Message: "root contract",
	}}}

	got := checkPathRules(cfg, inventory)
	want := []ruleDiagnostic{
		{Path: ".", Line: 1, Column: 1, RuleID: "root-layout", Message: "root contract required file not found: app.go"},
		{Path: "legacy", Line: 1, Column: 1, RuleID: "root-layout", Message: "root contract denied directory: legacy"},
		{Path: "tmp.txt", Line: 1, Column: 1, RuleID: "root-layout", Message: "root contract file is not allowed: tmp.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("checkPathRules() = %#v, want %#v", got, want)
	}
}

func TestCheckPathRulesHonorsRecursiveDepth(t *testing.T) {
	root := t.TempDir()
	writeInventoryFile(t, root, "db/migration/atlas.sum")
	writeInventoryFile(t, root, "db/migration/001.sql")

	inventory, err := newRepositoryInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{PathRules: []PathRule{{
		ID:          "atlas",
		Directories: FileSet{Include: []string{"db"}},
		Depth:       "recursive",
		Require: PathSet{
			Directories: []string{"migration"},
			Files:       []string{"migration/atlas.sum"},
		},
		Message: "atlas contract",
	}}}

	if got := checkPathRules(cfg, inventory); len(got) != 0 {
		t.Fatalf("checkPathRules() = %#v, want no diagnostics", got)
	}
}

func TestLoadConfigRejectsInvalidPathRule(t *testing.T) {
	tests := []struct {
		name   string
		rule   string
		needle string
	}{
		{"severity", "severity: warning\n    require: {files: [go.mod]}", "unsupported severity"},
		{"depth", "depth: descendants\n    require: {files: [go.mod]}", "unsupported depth"},
		{"operation", "depth: direct", "must configure require, allow, or deny"},
		{"glob", "depth: direct\n    require: {files: ['[']}", "invalid glob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfigFixture(t, "version: 1\npathRules:\n  - id: root\n    directories: {include: ['.']}\n    message: root contract\n    "+tt.rule+"\n")
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.needle) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, tt.needle)
			}
		})
	}
}

func writeInventoryFile(t *testing.T, root, relative string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}
