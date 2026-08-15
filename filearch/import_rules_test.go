package filearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyImport(t *testing.T) {
	cfg := &Config{
		modulePath: "example.com/project",
		Components: map[string]Component{
			"core": {Files: FileSet{Include: []string{"core/**"}}},
		},
	}
	tests := []struct {
		path      string
		category  string
		component string
	}{
		{"context", "standard", ""},
		{"example.com/project/core/user", "internal", "core"},
		{"example.com/project/internal/hidden", "unmatched-internal", ""},
		{"gorm.io/gorm", "external", ""},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			got := classifyImport(test.path, cfg)
			if got.Category != test.category || got.Component != test.component || got.Path != test.path {
				t.Fatalf("classifyImport(%q) = %#v", test.path, got)
			}
		})
	}
}

func TestClassifyImportMatchesPackageFromFileComponentPatterns(t *testing.T) {
	cfg := &Config{
		modulePath: "example.com/project",
		Components: map[string]Component{
			"composition": {Files: FileSet{Include: []string{"app.go"}}},
			"core": {Files: FileSet{
				Include: []string{"core/**/*.go"},
				Exclude: []string{"core/*/model/**/*.go"},
			}},
			"core_model": {Files: FileSet{Include: []string{"core/*/model/**/*.go"}}},
		},
	}

	for _, test := range []struct {
		path      string
		component string
	}{
		{"example.com/project", "composition"},
		{"example.com/project/core/user", "core"},
		{"example.com/project/core/user/model", "core_model"},
	} {
		t.Run(test.path, func(t *testing.T) {
			got := classifyImport(test.path, cfg)
			if got.Category != importCategoryInternal || got.Component != test.component {
				t.Fatalf("classifyImport(%q) = %#v, want internal component %q", test.path, got, test.component)
			}
		})
	}
}

func TestImportRulesDenyPrecedesAllowlist(t *testing.T) {
	rule := ImportRule{
		Allow: ImportConditions{Categories: []string{"external"}},
		Deny:  ImportConditions{Paths: []string{"gorm.io/gorm"}},
	}
	if !importRuleViolated(rule, importTarget{Path: "gorm.io/gorm", Category: "external"}) {
		t.Fatal("explicit deny did not override matching allow")
	}
	if importRuleViolated(rule, importTarget{Path: "github.com/google/uuid", Category: "external"}) {
		t.Fatal("matching allowlist import was rejected")
	}
	if !importRuleViolated(rule, importTarget{Path: "context", Category: "standard"}) {
		t.Fatal("nonmatching import was not rejected by nonempty allowlist")
	}
}

func TestImportRulesMatchEveryConditionKind(t *testing.T) {
	target := importTarget{Path: "example.com/project/core/user", Category: "internal", Component: "core"}
	conditions := []ImportConditions{
		{Components: []string{"core"}},
		{Paths: []string{"example.com/project/core/user"}},
		{PathPrefixes: []string{"example.com/project/core/"}},
		{PathMatches: []string{`/core/[a-z]+$`}},
		{Categories: []string{"internal"}},
	}
	for _, condition := range conditions {
		if !importConditionsMatch(condition, target) {
			t.Errorf("importConditionsMatch(%#v, %#v) = false", condition, target)
		}
	}
}

func TestImportRulesValidation(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string
	}{
		{"undefined component", "deny: {components: [missing]}", "undefined component"},
		{"invalid category", "deny: {categories: [other]}", "unsupported category"},
		{"invalid regex", "deny: {pathMatches: ['[']}", "invalid pathMatches regex"},
		{"empty conditions", "allow: {}", "must configure allow or deny conditions"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "core"), 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "config.yml")
			data := "version: 1\ncomponents:\n  core:\n    in: [core]\nimportRules:\n- id: imports\n  files: {include: ['**/*.go']}\n  " + test.rule + "\n  message: imports\n"
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}
