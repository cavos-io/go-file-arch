package filearch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestDeclarationGroupingRule(t *testing.T) {
	tests := []struct {
		name          string
		declarations  string
		wantViolation bool
	}{
		{name: "zero", declarations: "", wantViolation: false},
		{name: "one separate", declarations: `var ErrOne = errors.New("one")`, wantViolation: false},
		{name: "two separate", declarations: "var ErrOne = errors.New(\"one\")\nvar ErrTwo = errors.New(\"two\")", wantViolation: false},
		{name: "two grouped", declarations: "var (\nErrOne = errors.New(\"one\")\nErrTwo = errors.New(\"two\")\n)", wantViolation: true},
		{name: "three grouped", declarations: "var (\nErrOne = errors.New(\"one\")\nErrTwo = errors.New(\"two\")\nErrThree = errors.New(\"three\")\n)", wantViolation: false},
		{name: "three separate", declarations: "var ErrOne = errors.New(\"one\")\nvar ErrTwo = errors.New(\"two\")\nvar ErrThree = errors.New(\"three\")", wantViolation: true},
		{name: "four split", declarations: "var (\nErrOne = errors.New(\"one\")\nErrTwo = errors.New(\"two\")\n)\nvar (\nErrThree = errors.New(\"three\")\nErrFour = errors.New(\"four\")\n)", wantViolation: true},
		{name: "mixed group", declarations: "var (\nErrOne = errors.New(\"one\")\nErrTwo = errors.New(\"two\")\nErrThree = errors.New(\"three\")\nlookup = map[string]bool{}\n)", wantViolation: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := groupingDiagnostics(t, test.declarations)
			if test.wantViolation && len(diagnostics) == 0 {
				t.Fatal("grouping diagnostics are empty, want error-grouping violation")
			}
			if !test.wantViolation && len(diagnostics) != 0 {
				t.Fatalf("grouping diagnostics = %v, want none", diagnostics)
			}
			for _, diagnostic := range diagnostics {
				if !strings.Contains(diagnostic, "[error-grouping]") {
					t.Fatalf("diagnostic = %q, want error-grouping rule ID", diagnostic)
				}
			}
		})
	}
}

func TestDeclarationGroupingRuleValidation(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string
	}{
		{name: "selector count", rule: "    declaration: {kind: var, count: {min: 1}}\n    separateWhenCount: {max: 2}", want: "declaration count is not allowed"},
		{name: "missing thresholds", rule: "    declaration: {kind: var}", want: "must configure a count threshold"},
		{name: "overlapping thresholds", rule: "    declaration: {kind: var}\n    separateWhenCount: {max: 3}\n    singleGroupWhenCount: {min: 3}", want: "count thresholds overlap"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigFixture(t, "version: 1\ndeclarationGroupingRules:\n  - id: invalid\n    files: {include: [\"**/errors.go\"]}\n"+test.rule+"\n    message: invalid grouping\n")
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}

func groupingDiagnostics(t *testing.T, declarations string) []string {
	t.Helper()
	configPath := writeConfigFixture(t, `
version: 1
declarationGroupingRules:
  - id: error-grouping
    files:
      include: ["**/errors.go"]
    declaration:
      kind: var
      nameMatches: ["^Err[A-Z]"]
    separateWhenCount: {max: 2}
    singleGroupWhenCount: {min: 3}
    message: sentinel grouping required
`)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(filepath.Dir(configPath), "fixture", "errors.go")
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package fixture\nimport \"errors\"\n" + declarations + "\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []string
	pass := &analysis.Pass{
		Fset:  fset,
		Files: []*ast.File{file},
		Report: func(d analysis.Diagnostic) {
			diagnostics = append(diagnostics, d.Message)
		},
	}
	if _, err := runWithConfig(pass, cfg, nil); err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
