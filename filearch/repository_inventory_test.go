package filearch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryInventoryIncludesRoot(t *testing.T) {
	inventory, err := newRepositoryInventory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	got := inventory.matchingDirectories(FileSet{Include: []string{"."}})
	if want := []string{"."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("matchingDirectories() = %v, want %v", got, want)
	}
}

func TestRepositoryInventoryQueriesRelativeFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/one/metadata.go", "package one")
	writeFile(t, dir, "modules/one/features/a.go", "package features")
	writeFile(t, dir, "modules/ignored/metadata.go", "package ignored")

	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	dirs := inventory.matchingDirectories(FileSet{
		Include: []string{"modules/*"},
		Exclude: []string{"modules/ignored"},
	})
	if got := strings.Join(dirs, ","); got != "modules/one" {
		t.Fatalf("directories = %q", got)
	}
	if !inventory.hasRelativeFile("modules/one", "features/*.go") {
		t.Fatal("nested relative glob did not match")
	}
	if inventory.hasRelativeFile("modules/one", "missing.go") {
		t.Fatal("missing file matched")
	}
}

func TestRepositoryInventoryDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "hidden.go", "package external")
	if err := os.MkdirAll(filepath.Join(dir, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dir, "modules", "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.hasRelativeFile("modules/linked", "hidden.go") {
		t.Fatal("inventory followed a directory symlink")
	}
}
