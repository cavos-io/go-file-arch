package filearch

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestPathTemplateCapturesAndExpandsDomain(t *testing.T) {
	cfg := &Config{Templates: map[string]PathTemplate{
		"domain": {
			Pattern:  "core/{domain}/service.go",
			Captures: map[string]string{"domain": "^[a-z][a-z0-9_]*$"},
		},
	}}
	if err := cfg.validateTemplates(); err != nil {
		t.Fatal(err)
	}

	matches := cfg.matchFileSet(
		FileSet{Templates: []string{"domain"}},
		[]string{"core/batch_calls/service.go"},
	)
	if len(matches) != 1 {
		t.Fatalf("matches = %#v", matches)
	}
	wants := map[string]string{
		"{domain}":        "batch_calls",
		"{domain|snake}":  "batch_calls",
		"{domain|camel}":  "batchCalls",
		"{domain|pascal}": "BatchCalls",
	}
	for input, want := range wants {
		got, err := expandTemplate(input, matches[0].Captures)
		if err != nil || got != want {
			t.Fatalf("expandTemplate(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestTemplateValidationRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name     string
		template PathTemplate
		needle   string
	}{
		{"duplicate capture", PathTemplate{Pattern: "{domain}/{domain}", Captures: map[string]string{"domain": ".+"}}, "duplicate capture"},
		{"missing capture", PathTemplate{Pattern: "core/{domain}/service.go"}, "has no constraint"},
		{"invalid regex", PathTemplate{Pattern: "core/{domain}", Captures: map[string]string{"domain": "["}}, "invalid constraint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Templates: map[string]PathTemplate{"domain": tt.template}}
			err := cfg.validateTemplates()
			if err == nil || !strings.Contains(err.Error(), tt.needle) {
				t.Fatalf("validateTemplates() error = %v, want %q", err, tt.needle)
			}
		})
	}
}

func TestExpandTemplateRejectsUnknownTransformAndCapture(t *testing.T) {
	for _, input := range []string{"{domain|title}", "{missing}"} {
		if _, err := expandTemplate(input, map[string]string{"domain": "user"}); err == nil {
			t.Fatalf("expandTemplate(%q) error = nil", input)
		}
	}
}

func TestLoadConfigRejectsInvalidFileContractExpansion(t *testing.T) {
	for _, value := range []string{"{missing}.go", "{domain|title}.go"} {
		t.Run(value, func(t *testing.T) {
			path := writeConfigFixture(t, `
version: 1
templates:
  domain:
    pattern: core/{domain}/service.go
    captures: {domain: '^[a-z][a-z0-9_]*$'}
fileContractRules:
  - id: domain
    files: {templates: [domain]}
    require:
      siblingFiles: ['`+value+`']
    message: domain contract
`)
			_, err := LoadConfig(path)
			if err == nil {
				t.Fatalf("LoadConfig() error = nil for %q", value)
			}
		})
	}
}

func TestFileContractExpandsCapturedSiblingAndDeclaration(t *testing.T) {
	root := t.TempDir()
	writeInventoryFile(t, root, "core/batch_calls/service.go")
	writeInventoryFile(t, root, "core/batch_calls/model/batch_calls_model.go")
	inventory, err := newRepositoryInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Version:    1,
		workdirAbs: root,
		Templates: map[string]PathTemplate{
			"domain": {
				Pattern:  "core/{domain}/service.go",
				Captures: map[string]string{"domain": "^[a-z][a-z0-9_]*$"},
			},
		},
		FileContractRules: []FileContractRule{{
			ID:    "domain-contract",
			Files: FileSet{Templates: []string{"domain"}},
			Require: FileContractRequirement{
				SiblingFiles: []string{"model/{domain}_model.go"},
				Declarations: []DeclarationSelector{
					{Kind: "interface", Name: "I{domain|pascal}Service"},
					{
						Kind: "func",
						Name: "Handle",
						Receiver: ReceiverCondition{
							Present: testBoolPointer(true),
							Pointer: testBoolPointer(true),
							Type:    "{domain|pascal}Handler",
						},
						Parameters: ParameterCondition{
							First: &TypeCondition{Name: "{domain|camel}", Type: "{domain|pascal}Request"},
						},
						Returns: ReturnCondition{
							Contains: []TypeCondition{{Type: "*{domain|pascal}Response"}},
						},
					},
				},
			},
			Message: "domain contract",
		}},
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(root, "core/batch_calls/service.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, `package batch_calls
type IBatchCallService interface{}
type BatchCallsHandler struct{}
type BatchCallsRequest struct{}
type BatchCallsResponse struct{}
func (h *BatchCallsHandler) Handle(batchCalls BatchCallsRequest) *BatchCallsResponse { return nil }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []string
	pass := &analysis.Pass{Fset: fset, Report: func(d analysis.Diagnostic) {
		diagnostics = append(diagnostics, d.Message)
	}}
	checkFileContractRules(pass, file, filename, cfg, inventory)
	want := []string{"[domain-contract]: domain contract required declaration not found: interface IBatchCallsService"}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %v, want %v", diagnostics, want)
	}
}
