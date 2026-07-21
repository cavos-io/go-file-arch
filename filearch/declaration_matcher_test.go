package filearch

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestDeclarationSelectorMatchesNamesResultsAndLiterals(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "feature.go", `package feature
const Title, Empty = "v1", ""
const Computed = Title + "-next"
type Feature struct{}
type Client interface{ Do() error }
type FeatureOption func(*Feature)
func NewFeature() (*Feature, error) { return nil, nil }
func helper() bool { return true }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	candidates := extractDeclarationCandidates(fset, file)

	tests := []DeclarationSelector{
		{Kind: "const", Name: "Title", Value: ValueCondition{Equals: testStringPointer("v1")}},
		{Kind: "const", Name: "Empty", Value: ValueCondition{Equals: testStringPointer("")}},
		{Kind: "func", NameMatches: []string{"^New"}, Returns: ReturnCondition{Contains: []string{"*Feature", "error"}}},
		{Kind: "func", Returns: ReturnCondition{Matches: []string{"^bool$"}}},
		{Kind: "interface", Exported: testBoolPointer(true)},
		{Kind: "type", Name: "FeatureOption"},
		{Kind: "func", Name: "helper", Exported: testBoolPointer(false)},
	}
	for _, selector := range tests {
		if !anyDeclarationMatches(candidates, selector) {
			t.Fatalf("selector %#v did not match %#v", selector, candidates)
		}
	}

	if anyDeclarationMatches(candidates, DeclarationSelector{
		Kind:           "struct",
		NameNotMatches: []string{"^Feature$"},
	}) {
		t.Fatal("negative name regex unexpectedly matched Feature")
	}
	if anyDeclarationMatches(candidates, DeclarationSelector{
		Kind:  "const",
		Name:  "Computed",
		Value: ValueCondition{Matches: []string{"next"}},
	}) {
		t.Fatal("non-literal constant unexpectedly matched a literal value condition")
	}
}

func TestSelectorDescriptionIncludesConfiguredConstraints(t *testing.T) {
	description := selectorDescription(DeclarationSelector{
		Kind:           "func",
		NameMatches:    []string{"^New"},
		NameNotMatches: []string{"Legacy"},
		Exported:       testBoolPointer(true),
		Returns: ReturnCondition{
			Contains: []string{"*Feature"},
			Matches:  []string{"^error$"},
		},
	})
	for _, want := range []string{"exported func", `name matches "^New"`, `name does not match "Legacy"`, `returns contains "*Feature"`, `returns matches "^error$"`} {
		if !strings.Contains(description, want) {
			t.Fatalf("selectorDescription() = %q, want %q", description, want)
		}
	}
}

func testStringPointer(value string) *string { return &value }

func testBoolPointer(value bool) *bool { return &value }
