package filearch

import (
	"strings"
	"testing"
)

func TestCheckDirectoryRulesReportsAllAndAnyRequirements(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/one/feature.go", "package one")
	writeFile(t, dir, "modules/two/metadata.go", "package two")
	writeFile(t, dir, "modules/ignored/other.go", "package ignored")
	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{DirectoryRules: []DirectoryRule{{
		ID: "module-files",
		Directories: FileSet{
			Include: []string{"modules/*"},
			Exclude: []string{"modules/ignored"},
		},
		Require: DirectoryRequirement{
			Files:    []string{"metadata.go"},
			AnyFiles: []string{"feature.go", "alternate.go"},
		},
		Message: "module contract",
	}}}

	got := checkDirectoryRules(cfg, inventory)
	if len(got) != 2 {
		t.Fatalf("len(diagnostics) = %d, want 2: %#v", len(got), got)
	}
	if got[0].Path != "modules/one" || !strings.Contains(got[0].Message, "metadata.go") {
		t.Fatalf("diagnostic[0] = %#v", got[0])
	}
	if got[1].Path != "modules/two" || !strings.Contains(got[1].Message, "at least one") {
		t.Fatalf("diagnostic[1] = %#v", got[1])
	}
}
