package filearch

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestDeclarationExtractionDistinguishesAliasesFromNamedTypes(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", `package types
type Imported = Other
type StructAlias = struct{ Name string }
type Status string
type Record struct{ Name string }
`, 0)
	if err != nil {
		t.Fatal(err)
	}

	candidates := extractDeclarationCandidates(fset, file)
	for _, selector := range []DeclarationSelector{
		{Kind: "alias", Name: "Imported"},
		{Kind: "alias", Name: "StructAlias"},
		{Kind: "type", Name: "Status"},
		{Kind: "struct", Name: "Record"},
	} {
		if !anyDeclarationMatches(candidates, selector) {
			t.Fatalf("selector %#v did not match %#v", selector, candidates)
		}
	}
}

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
		{Kind: "func", NameMatches: []string{"^New"}, Receiver: ReceiverCondition{Present: testBoolPointer(false)}, Returns: ReturnCondition{Contains: []TypeCondition{{Type: "*Feature"}, {Type: "error"}}}},
		{Kind: "func", Returns: ReturnCondition{Matches: []string{"^missing$", "^bool$"}}},
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

func TestDeclarationSelectorMatchesUnderlyingAndInitialization(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "declarations.go", `package declarations
type Status string
type Handler func() error
var ready = true
var unset bool
var first, second = pair()
func pair() (int, int) { return 0, 0 }
type Service struct{}
func (Service) Run() {}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	candidates := extractDeclarationCandidates(fset, file)

	selectors := []DeclarationSelector{
		{Kind: "type", Name: "Status", Underlying: TypeCondition{Type: "string"}},
		{Kind: "type", Name: "Handler", Underlying: TypeCondition{TypeMatches: []string{`^func\(`}, TypeNotMatches: []string{`string`}}},
		{Kind: "var", Name: "ready", Initialized: testBoolPointer(true)},
		{Kind: "var", Name: "unset", Initialized: testBoolPointer(false)},
		{Kind: "var", Name: "first", Initialized: testBoolPointer(true)},
		{Kind: "var", Name: "second", Initialized: testBoolPointer(true)},
		{Kind: "func", Name: "Run", Receiver: ReceiverCondition{TypeMatches: []string{"Service"}, TypeNotMatches: []string{"Legacy"}}},
	}
	for _, selector := range selectors {
		if !anyDeclarationMatches(candidates, selector) {
			t.Fatalf("selector %#v did not match %#v", selector, candidates)
		}
	}

	if anyDeclarationMatches(candidates, DeclarationSelector{
		Kind: "type", Name: "Status", Underlying: TypeCondition{TypeNotMatches: []string{"string"}},
	}) {
		t.Fatal("negative underlying type pattern matched Status")
	}
	if anyDeclarationMatches(candidates, DeclarationSelector{
		Kind: "func", Name: "Run", Receiver: ReceiverCondition{TypeNotMatches: []string{"Service"}},
	}) {
		t.Fatal("negative receiver type pattern matched Service.Run")
	}
}

func TestSelectorDescriptionIncludesConfiguredConstraints(t *testing.T) {
	description := selectorDescription(DeclarationSelector{
		Kind:           "func",
		NameMatches:    []string{"^New"},
		NameNotMatches: []string{"Legacy"},
		Exported:       testBoolPointer(true),
		Returns: ReturnCondition{
			Contains: []TypeCondition{{Type: "*Feature"}},
			Matches:  []string{"^error$"},
		},
	})
	for _, want := range []string{"exported func", `name matches "^New"`, `name does not match "Legacy"`, `returns contains "*Feature"`, `returns matches "^error$"`} {
		if !strings.Contains(description, want) {
			t.Fatalf("selectorDescription() = %q, want %q", description, want)
		}
	}
}

func TestDeclarationSelectorMatchesStructuralContracts(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "repository.go", `package user
import "context"
type BaseRepository interface{}
type Repository interface {
	BaseRepository
	Find(ctx context.Context, id uint64) (*User, error)
}
type Service struct { BaseService }
type User struct{}
func (s *Service) Find(ctx context.Context, id uint64) (*User, error) { return nil, nil }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	candidates := extractDeclarationCandidates(fset, file)

	selectors := []DeclarationSelector{
		{
			Kind: "interface", Name: "Repository",
			Embeds: EmbedCondition{Contains: []TypeCondition{{Type: "BaseRepository"}}},
			Methods: MethodCondition{All: &FunctionCondition{
				Parameters: ParameterCondition{First: &TypeCondition{Type: "context.Context"}},
			}},
		},
		{
			Kind: "func", Name: "Find",
			Receiver: ReceiverCondition{Present: testBoolPointer(true), Pointer: testBoolPointer(true), Type: "Service"},
			Parameters: ParameterCondition{
				Count: CountCondition{Equals: testIntPointer(2)},
				First: &TypeCondition{Name: "ctx", Type: "context.Context"},
			},
			Returns: ReturnCondition{Contains: []TypeCondition{{Type: "error"}}},
			Count:   CountCondition{Equals: testIntPointer(1)},
		},
	}
	for _, selector := range selectors {
		if !anyDeclarationMatches(candidates, selector) {
			t.Fatalf("selector %#v did not match %#v", selector, candidates)
		}
	}

	tooMany := selectors[1]
	tooMany.Count.Equals = testIntPointer(2)
	if anyDeclarationMatches(candidates, tooMany) {
		t.Fatal("exact declaration count unexpectedly matched")
	}
}

func TestDeclarationSelectorMatchesStructFieldsAndTags(t *testing.T) {
	fset := token.NewFileSet()
	source := "package model\n" +
		"import \"net/http\"\n" +
		"type TaggedModel struct {\n" +
		"Name string `json:\"name\" validate:\"required\"`\n" +
		"Count int\n" +
		"}\n" +
		"type PlainModel struct { Name string }\n" +
		"type EmbeddedModel struct { *PlainModel; http.Client }\n"
	file, err := parser.ParseFile(fset, "model.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	candidates := extractDeclarationCandidates(fset, file)

	tagged := DeclarationSelector{
		Kind: "struct",
		Fields: FieldCondition{Contains: []FieldSelector{{
			Name: "Name", Type: "string", TagMatches: []string{`(^|\s)json:`},
		}}},
	}
	if !anyDeclarationMatches(candidates, tagged) {
		t.Fatalf("tagged selector did not match %#v", candidates)
	}

	allPlain := DeclarationSelector{
		Kind: "struct", Name: "PlainModel",
		Fields: FieldCondition{All: &FieldSelector{TagMatches: []string{`^$`}}},
	}
	if !anyDeclarationMatches(candidates, allPlain) {
		t.Fatalf("plain selector did not match %#v", candidates)
	}

	embedded := DeclarationSelector{
		Kind: "struct", Name: "EmbeddedModel",
		Fields: FieldCondition{Contains: []FieldSelector{
			{Name: "PlainModel", Type: "*PlainModel", Exported: testBoolPointer(true)},
			{Name: "Client", Type: "http.Client", Exported: testBoolPointer(true)},
		}},
	}
	if !anyDeclarationMatches(candidates, embedded) {
		t.Fatalf("embedded selector did not match %#v", candidates)
	}
}

func TestDeclarationExtractionNormalizesStructuralCandidates(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "repository.go", `package user
import "context"
type Repository interface {
	BaseRepository
	Find(ctx context.Context, id uint64) (*User, error)
}
type Service struct { BaseService }
func (s *Service) Find(ctx context.Context, id uint64) (*User, error) { return nil, nil }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	candidates := extractDeclarationCandidates(fset, file)

	var repository, service, find *declarationCandidate
	for i := range candidates {
		switch {
		case candidates[i].Kind == "interface" && candidates[i].Name == "Repository":
			repository = &candidates[i]
		case candidates[i].Kind == "struct" && candidates[i].Name == "Service":
			service = &candidates[i]
		case candidates[i].Kind == "func" && candidates[i].Name == "Find":
			find = &candidates[i]
		}
	}
	if repository == nil || len(repository.Embeds) != 1 || repository.Embeds[0].Type != "BaseRepository" {
		t.Fatalf("Repository candidate = %#v", repository)
	}
	if len(repository.Methods) != 1 || repository.Methods[0].Parameters[0] != (typedCandidate{Name: "ctx", Type: "context.Context"}) || repository.Methods[0].Results[1].Type != "error" {
		t.Fatalf("Repository methods = %#v", repository.Methods)
	}
	if service == nil || len(service.Embeds) != 1 || service.Embeds[0].Type != "BaseService" {
		t.Fatalf("Service candidate = %#v", service)
	}
	if find == nil || find.Receiver == nil || !find.Receiver.Pointer || find.Receiver.Type != "Service" {
		t.Fatalf("Find receiver = %#v", find)
	}
	if len(find.Parameters) != 2 || find.Parameters[0] != (typedCandidate{Name: "ctx", Type: "context.Context"}) || len(find.Results) != 2 || find.Results[0].Type != "*User" {
		t.Fatalf("Find signature = %#v", find)
	}
}

func TestDeclarationSelectorValidationRejectsInvalidStructure(t *testing.T) {
	tests := []struct {
		selector DeclarationSelector
		want     string
	}{
		{DeclarationSelector{Kind: "struct", Receiver: ReceiverCondition{Present: testBoolPointer(true)}}, "receiver is only valid for func"},
		{DeclarationSelector{Kind: "func", Receiver: ReceiverCondition{Present: testBoolPointer(false), Pointer: testBoolPointer(true)}}, "receiver.present false"},
		{DeclarationSelector{Kind: "func", Parameters: ParameterCondition{Count: CountCondition{Equals: testIntPointer(-1)}}}, "parameters.count values must be non-negative"},
		{DeclarationSelector{Kind: "interface", Methods: MethodCondition{All: &FunctionCondition{NameMatches: []string{"["}}}}, "methods.all.nameMatches regex"},
		{DeclarationSelector{Kind: "func", Returns: ReturnCondition{Contains: TypeConditionList{{TypeMatches: []string{"["}}}}}, "returns.typeConditions"},
		{DeclarationSelector{Kind: "struct", Fields: FieldCondition{Contains: []FieldSelector{{TagMatches: []string{"["}}}}}, "fields.contains[0].tagMatches regex"},
		{DeclarationSelector{Kind: "struct", Underlying: TypeCondition{Type: "string"}}, "underlying is only valid for type"},
		{DeclarationSelector{Kind: "func", Initialized: testBoolPointer(true)}, "initialized is only valid for var or const"},
		{DeclarationSelector{Kind: "type", Underlying: TypeCondition{TypeMatches: []string{"["}}}, "underlying.typeMatches regex"},
		{DeclarationSelector{Kind: "func", Receiver: ReceiverCondition{TypeNotMatches: []string{"["}}}, "receiver.typeNotMatches regex"},
	}
	for _, test := range tests {
		err := validateDeclarationSelector("bad", test.selector)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("validateDeclarationSelector(%#v) error = %v, want %q", test.selector, err, test.want)
		}
	}
}

func testStringPointer(value string) *string { return &value }

func testBoolPointer(value bool) *bool { return &value }

func testIntPointer(value int) *int { return &value }
