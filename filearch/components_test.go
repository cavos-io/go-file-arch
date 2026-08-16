package filearch

import (
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestComponentFilesClassifyRootFile(t *testing.T) {
	cfg := &Config{Components: map[string]Component{
		"composition": {Files: FileSet{Include: []string{"app.go"}}},
	}}

	matches := cfg.componentMatches("app.go")
	if len(matches) != 1 || matches[0].name != "composition" {
		t.Fatalf("componentMatches() = %#v", matches)
	}
}

func TestComponentMatchesReturnsAllOverlapsInStableOrder(t *testing.T) {
	cfg := &Config{Components: map[string]Component{
		"core_model": {Files: FileSet{Include: []string{"core/**/model/*.go"}}},
		"core":       {Files: FileSet{Include: []string{"core/**/*.go"}}},
	}}

	matches := cfg.componentMatches("core/user/model/user_model.go")
	var names []string
	for _, match := range matches {
		names = append(names, match.name)
	}
	if want := []string{"core", "core_model"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("component names = %v, want %v", names, want)
	}
}

func TestTestFileComponentDoesNotOwnProductionPackages(t *testing.T) {
	cfg := &Config{Components: map[string]Component{
		"test_support": {Files: FileSet{Include: []string{"**/*_test.go", "internal/mocks/**/*.go"}}},
		"core": {Files: FileSet{
			Include: []string{"core/**/*.go"},
			Exclude: []string{"**/*_test.go"},
		}},
	}}

	for _, test := range []struct {
		path string
		want []string
	}{
		{path: "core/widget/service_test.go", want: []string{"test_support"}},
		{path: "core/widget/service.go", want: []string{"core"}},
	} {
		matches := cfg.componentMatches(test.path)
		var got []string
		for _, match := range matches {
			got = append(got, match.name)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("componentMatches(%q) = %v, want %v", test.path, got, test.want)
		}
	}

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "core/widget", want: "core"},
		{path: "internal/mocks", want: "test_support"},
	} {
		got, ok := cfg.matchPackageComponent(test.path)
		if !ok || got != test.want {
			t.Fatalf("matchPackageComponent(%q) = %q, %t, want %q, true", test.path, got, ok, test.want)
		}
	}

	if got, ok := cfg.matchPackageComponent("unowned/package"); ok {
		t.Fatalf("matchPackageComponent(unowned/package) = %q, true, want no owner", got)
	}
}

func TestCheckComponentOptionsReportsUnmatchedFile(t *testing.T) {
	diagnostics := componentDiagnostics(t, &Config{
		ComponentOptions: ComponentOptions{RequireMatch: true},
		Components: map[string]Component{
			"core": {Files: FileSet{Include: []string{"core/**/*.go"}}},
		},
	}, "app.go")

	want := []string{"[componentOptions.requireMatch]: file does not match any component"}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %v, want %v", diagnostics, want)
	}
}

func TestCheckComponentOptionsReportsOverlappingFile(t *testing.T) {
	diagnostics := componentDiagnostics(t, &Config{
		ComponentOptions: ComponentOptions{RequireSingleMatch: true},
		Components: map[string]Component{
			"core_model": {Files: FileSet{Include: []string{"core/**/model/*.go"}}},
			"core":       {Files: FileSet{Include: []string{"core/**/*.go"}}},
		},
	}, "core/user/model/user_model.go")

	want := []string{"[componentOptions.requireSingleMatch]: file matches multiple components: core, core_model"}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %v, want %v", diagnostics, want)
	}
}

func componentDiagnostics(t *testing.T, cfg *Config, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, "package fixture", 0)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []string
	pass := &analysis.Pass{
		Fset: fset,
		Report: func(d analysis.Diagnostic) {
			diagnostics = append(diagnostics, d.Message)
		},
	}
	checkComponentOptions(pass, file, cfg, path)
	return diagnostics
}
